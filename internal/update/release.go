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
//  1. build the client mod jar
//  2. StampManifests: put the jar (at its release URL) into every client manifest
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

// StampManifests rewrites each manifest in dir for this release: the client
// version, and the Armstrong mod jar at its download URL. Copies go to dist
// as client-<mc>.json, next to the jar.
func StampManifests(dir, dist, version, jar string) error {
	data, err := os.ReadFile(jar)
	if err != nil {
		return err
	}
	s1 := sha1.Sum(data)
	s512 := sha512.Sum512(data)
	jarName := "armstrong-" + version + ".jar"
	mod := client.Mod{
		ID: client.ModID, Name: "Armstrong", Version: version, File: jarName,
		URL: AssetURL(version, jarName), SHA1: hex.EncodeToString(s1[:]), SHA512: hex.EncodeToString(s512[:]), Size: int64(len(data)),
	}
	if err := os.MkdirAll(dist, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dist, jarName), data, 0o644); err != nil {
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
	r := &Release{Format: FormatVersion, Version: version, Launcher: map[string]Asset{}, Client: map[string]Asset{}}
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
			r.Mod = a
		case strings.HasPrefix(name, "armstrong-"):
			platform := strings.TrimSuffix(strings.TrimPrefix(name, "armstrong-"), ".exe")
			r.Launcher[platform] = a
		case strings.HasPrefix(name, "client-") && strings.HasSuffix(name, ".json"):
			r.Client[strings.TrimSuffix(strings.TrimPrefix(name, "client-"), ".json")] = a
		}
	}
	if len(r.Launcher) == 0 || len(r.Client) == 0 || r.Mod.Name == "" {
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
