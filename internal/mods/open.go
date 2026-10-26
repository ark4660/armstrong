package mods

import (
	"context"
	"path/filepath"
	"strconv"

	"github.com/ark4660/armstrong/internal/app"
	"github.com/ark4660/armstrong/internal/client"
	"github.com/ark4660/armstrong/internal/download"
	"github.com/ark4660/armstrong/internal/game"
	"github.com/ark4660/armstrong/internal/instance"
	"github.com/ark4660/armstrong/internal/modrinth"
)

// Open prepares a manager for an instance: it syncs the instance's client
// layer (usually already downloaded) so checks know what the client brings.
func Open(ctx context.Context, paths app.Paths, inst *instance.Instance, dl *download.Client) (*Manager, error) {
	man, err := client.Load(inst.Minecraft)
	if err != nil {
		return nil, err
	}
	layer, err := client.Sync(ctx, dl, paths, man, inst.Addons, nil)
	if err != nil {
		return nil, err
	}
	loader := inst.LoaderVersion
	if loader == "" {
		loader = man.FabricLoader
	}
	java := "25"
	if v, err := game.ReadVersion(paths, inst.Minecraft); err == nil && v.JavaVersion.MajorVersion > 0 {
		java = strconv.Itoa(v.JavaVersion.MajorVersion)
	}
	return &Manager{
		ModsDir:   filepath.Join(inst.GameDir(), "mods"),
		IndexPath: filepath.Join(inst.Dir(), "mods.json"),
		CacheDir:  filepath.Join(paths.Root, "cache", "mods"),
		Minecraft: inst.Minecraft, Loader: loader, Java: java,
		Layer:  layer,
		Source: Modrinth{Client: &modrinth.Client{DL: dl}},
		DL:     dl,
	}, nil
}
