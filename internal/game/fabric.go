package game

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
)

const fabricMeta = "https://meta.fabricmc.net/v2"

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
	u := fabricMeta + "/versions/loader/" + url.PathEscape(mc)
	path := filepath.Join(in.Paths.Versions(), "fabric", "loaders-"+mc+".json")
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
	u := fabricMeta + "/versions/loader/" + url.PathEscape(mc) + "/" + url.PathEscape(loader) + "/profile/json"
	var v Version
	if err := in.DL.CachedJSON(ctx, u, filepath.Join(in.Paths.Versions(), id, id+".json"), "", &v); err != nil {
		return nil, fmt.Errorf("fabric profile: %w", err)
	}
	if v.InheritsFrom != mc {
		return nil, fmt.Errorf("fabric profile %s inherits from %q, want %q", v.ID, v.InheritsFrom, mc)
	}
	return &v, nil
}

// supports returns the Minecraft versions Fabric has intermediary mappings for.
func (fabricLoader) supports(ctx context.Context, in *Installer) (map[string]bool, error) {
	var entries []struct {
		Version string `json:"version"`
	}
	path := filepath.Join(in.Paths.Versions(), "fabric", "game.json")
	if err := in.DL.CachedJSON(ctx, fabricMeta+"/versions/game", path, "", &entries); err != nil {
		return nil, fmt.Errorf("fabric game versions: %w", err)
	}
	out := make(map[string]bool, len(entries))
	for _, e := range entries {
		out[e.Version] = true
	}
	return out, nil
}
