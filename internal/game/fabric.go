package game

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

// Fabric's own intermediary mappings start at 1.14. Older versions run the
// same loader on Legacy Fabric's mappings, from Legacy Fabric's meta server;
// its profiles have the same format.
const (
	fabricMeta       = "https://meta.fabricmc.net/v2"
	legacyFabricMeta = "https://meta.legacyfabric.net/v2"
)

// IsLegacyFabric reports whether mc is older than 1.14, so its Fabric loader,
// API and mods come from Legacy Fabric.
func IsLegacyFabric(mc string) bool {
	parts := strings.Split(mc, ".")
	if len(parts) < 2 || parts[0] != "1" {
		return false
	}
	minor, err := strconv.Atoi(parts[1])
	return err == nil && minor < 14
}

// ModrinthLoader is the Modrinth loader tag of Fabric mods for mc.
func ModrinthLoader(mc string) string {
	if IsLegacyFabric(mc) {
		return "legacy-fabric"
	}
	return "fabric"
}

// metaFor returns the meta server and cache folder for mc.
func metaFor(mc string) (base, cache string) {
	if IsLegacyFabric(mc) {
		return legacyFabricMeta, "legacy-fabric"
	}
	return fabricMeta, "fabric"
}

type fabricLoaderEntry struct {
	Loader struct {
		Version string `json:"version"`
		Stable  bool   `json:"stable"`
	} `json:"loader"`
}

type fabricLoader struct{}

// latest returns the newest stable Fabric loader for mc.
func (fabricLoader) latest(ctx context.Context, in *Installer, mc string) (string, error) {
	var entries []fabricLoaderEntry
	base, cache := metaFor(mc)
	u := base + "/versions/loader/" + url.PathEscape(mc)
	path := filepath.Join(in.Paths.Versions(), cache, "loaders-"+mc+".json")
	if err := in.DL.CachedJSON(ctx, u, path, "", &entries); err != nil {
		return "", fmt.Errorf("fabric loaders: %w", err)
	}
	for _, e := range entries { // newest first
		if e.Loader.Stable {
			return e.Loader.Version, nil
		}
	}
	if len(entries) > 0 {
		return entries[0].Loader.Version, nil
	}
	return "", fmt.Errorf("fabric doesn't support Minecraft %s", mc)
}

// profile fetches Fabric's launcher profile for mc + loader. It inherits
// from the vanilla version and only adds libraries, mainClass and args.
func (fabricLoader) profile(ctx context.Context, in *Installer, mc, loader string) (*Version, error) {
	id := "fabric-loader-" + loader + "-" + mc
	base, _ := metaFor(mc)
	u := base + "/versions/loader/" + url.PathEscape(mc) + "/" + url.PathEscape(loader) + "/profile/json"
	var v Version
	if err := in.DL.CachedJSON(ctx, u, filepath.Join(in.Paths.Versions(), id, id+".json"), "", &v); err != nil {
		return nil, fmt.Errorf("fabric profile: %w", err)
	}
	if v.InheritsFrom != mc {
		return nil, fmt.Errorf("fabric profile %s inherits from %q, want %q", v.ID, v.InheritsFrom, mc)
	}
	if IsLegacyFabric(mc) {
		// These versions ask for Java 8. Legacy Fabric runs on current Java,
		// and so does the client mod, which shares its source with the
		// modern builds.
		v.JavaVersion.Component, v.JavaVersion.MajorVersion = legacyJava, 21
	}
	return &v, nil
}

// legacyJava is the Mojang Java runtime Legacy Fabric versions run on.
const legacyJava = "java-runtime-delta"

// supports returns the Minecraft versions Fabric or Legacy Fabric has
// intermediary mappings for.
func (fabricLoader) supports(ctx context.Context, in *Installer) (map[string]bool, error) {
	out := map[string]bool{}
	for _, m := range []struct{ base, cache string }{{fabricMeta, "fabric"}, {legacyFabricMeta, "legacy-fabric"}} {
		var entries []struct {
			Version string `json:"version"`
		}
		path := filepath.Join(in.Paths.Versions(), m.cache, "game.json")
		if err := in.DL.CachedJSON(ctx, m.base+"/versions/game", path, "", &entries); err != nil {
			return nil, fmt.Errorf("fabric game versions: %w", err)
		}
		for _, e := range entries {
			if IsLegacyFabric(e.Version) == (m.cache == "legacy-fabric") {
				out[e.Version] = true
			}
		}
	}
	return out, nil
}
