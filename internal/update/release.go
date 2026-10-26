package update

import (
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ark4660/armstrong/internal/client"
)

// Release building: run by the release workflow, in this order:
//  1. build the client mod jars (one per Minecraft version)
//  2. StampManifests: put each jar (at its release URL) into its version's manifest
//  3. build the launchers (they embed the stamped manifests)
//  4. WriteIndex: release.json with every file's sum, signed

// LauncherName is a launcher binary's asset name for a platform.
func LauncherName(platform string) string {
	name := "armstrong-" + platform
	if strings.HasPrefix(platform, "windows-") {
		name += ".exe"
	}
	return name
}

// ModJarName is the release asset name of the client mod built for a
// Minecraft version: armstrong-0.3.0-mc1.21.1.jar. (No "+": GitHub may
// rename release files with unusual characters.)
func ModJarName(version, minecraft string) string {
	return "armstrong-" + version + "-mc" + minecraft + ".jar"
}

// StampManifests rewrites each manifest in dir for this release: the client
// version, and the Armstrong mod jar built for its Minecraft version, at its
// download URL. jarPattern is the built jar's path with {mc} for the version.
// Copies of the jars and manifests (as client-<mc>.json) go to dist.
func StampManifests(dir, dist, version, jarPattern string) error {
	if err := os.MkdirAll(dist, 0o755); err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil || len(files) == 0 {
		return fmt.Errorf("no manifests in %s", dir)
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		var m client.Manifest
		if err := json.Unmarshal(raw, &m); err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
		mod, err := stampJar(dist, version, m.Minecraft, strings.ReplaceAll(jarPattern, "{mc}", m.Minecraft))
		if err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
		m.Version = version
		var mods []client.Mod
		for _, x := range m.Mods {
			if x.ID != client.ModID {
				mods = append(mods, x)
			}
		}
		m.Mods = append([]client.Mod{mod}, mods...)
		out, err := json.MarshalIndent(&m, "", "  ")
		if err != nil {
			return err
		}
		out = append(out, '\n')
		if err := os.WriteFile(f, out, 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dist, "client-"+m.Minecraft+".json"), out, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// WriteIndex describes everything in dist in release.json and signs it.
func WriteIndex(dist, version, signingKey string) (*Release, error) {
	r := &Release{Format: FormatVersion, Version: version, Launcher: map[string]Asset{}, Client: map[string]Asset{}, Mods: map[string]Asset{}}
	entries, err := os.ReadDir(dist)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		a, err := describe(dist, version, name)
		if err != nil {
			return nil, err
		}
		switch {
		case strings.HasPrefix(name, "armstrong-") && strings.HasSuffix(name, ".jar"):
			// armstrong-<version>-mc<minecraft>.jar
			i := strings.LastIndex(name, "-mc")
			if i < 0 {
				return nil, fmt.Errorf("%s: not named like %s", name, ModJarName(version, "<minecraft>"))
			}
			mc := strings.TrimSuffix(name[i+len("-mc"):], ".jar")
			r.Mods[mc] = a
		case strings.HasPrefix(name, "armstrong-"):
			platform := strings.TrimSuffix(strings.TrimPrefix(name, "armstrong-"), ".exe")
			r.Launcher[platform] = a
		case strings.HasPrefix(name, "client-") && strings.HasSuffix(name, ".json"):
			r.Client[strings.TrimSuffix(strings.TrimPrefix(name, "client-"), ".json")] = a
		}
	}
	if len(r.Launcher) == 0 || len(r.Client) == 0 || len(r.Mods) == 0 {
		return nil, fmt.Errorf("%s needs launchers, client manifests and the mod jar", dist)
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')
	sig, err := Sign(signingKey, data)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dist, IndexName), data, 0o644); err != nil {
		return nil, err
	}
	return r, os.WriteFile(filepath.Join(dist, SigName), sig, 0o644)
}

func describe(dist, version, name string) (Asset, error) {
	data, err := os.ReadFile(filepath.Join(dist, name))
	if err != nil {
		return Asset{}, err
	}
	sum := sha256.Sum256(data)
	return Asset{Name: name, URL: AssetURL(version, name), SHA256: hex.EncodeToString(sum[:]), Size: int64(len(data))}, nil
}

// stampJar copies the mod jar built for one Minecraft version into dist and
// returns its manifest entry.
func stampJar(dist, version, minecraft, jar string) (client.Mod, error) {
	data, err := os.ReadFile(jar)
	if err != nil {
		return client.Mod{}, err
	}
	s1 := sha1.Sum(data)
	s512 := sha512.Sum512(data)
	name := ModJarName(version, minecraft)
	if err := os.WriteFile(filepath.Join(dist, name), data, 0o644); err != nil {
		return client.Mod{}, err
	}
	return client.Mod{
		ID: client.ModID, Name: "Armstrong", Version: version, File: name,
		URL: AssetURL(version, name), SHA1: hex.EncodeToString(s1[:]), SHA512: hex.EncodeToString(s512[:]), Size: int64(len(data)),
	}, nil
}
