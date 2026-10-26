package instance

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ark4660/armstrong/internal/app"
)

// DefaultMinecraft is used for instances migrated from the old single-profile
// layout, which only ever installed this version.
const DefaultMinecraft = "26.3"

// Migrate converts the pre-instance layout (profiles/<name>/ game dirs and
// one global version/memory in settings.json) into instances. It only acts
// when no instances exist yet. It reports whether settings changed and need saving.
func Migrate(paths app.Paths, s *app.Settings) (bool, error) {
	store := Store{Root: paths.Instances()}
	legacy := s.LegacyMinecraft != "" || s.LegacyProfile != "" || s.LegacyLoader != "" || s.LegacyMemoryMB != 0
	clear := func() {
		s.LegacyMinecraft, s.LegacyLoader, s.LegacyProfile, s.LegacyMemoryMB = "", "", "", 0
	}

	existing, err := store.List()
	if err != nil {
		return false, err
	}
	if len(existing) > 0 {
		clear()
		return legacy, nil
	}

	tmpl := Instance{
		Minecraft:     s.LegacyMinecraft,
		Loader:        LoaderFabric,
		LoaderVersion: s.LegacyLoader,
		MemoryMB:      s.LegacyMemoryMB,
	}
	if tmpl.Minecraft == "" {
		tmpl.Minecraft = DefaultMinecraft
	}

	profiles := filepath.Join(paths.Root, "profiles")
	entries, err := os.ReadDir(profiles)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	var first string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		tmpl.Name = e.Name()
		inst, err := store.Create(tmpl)
		if err != nil {
			return false, fmt.Errorf("migrating profile %s: %w", e.Name(), err)
		}
		// Create made an empty game dir; replace it with the old profile.
		if err := os.Remove(inst.GameDir()); err != nil {
			return false, err
		}
		if err := os.Rename(filepath.Join(profiles, e.Name()), inst.GameDir()); err != nil {
			return false, fmt.Errorf("migrating profile %s: %w", e.Name(), err)
		}
		if first == "" || e.Name() == s.LegacyProfile {
			first = inst.ID
		}
	}
	if first == "" && legacy {
		tmpl.Name = "Default"
		inst, err := store.Create(tmpl)
		if err != nil {
			return false, err
		}
		first = inst.ID
	}
	os.Remove(profiles) // only succeeds if now empty

	if first == "" {
		return legacy, nil
	}
	if s.Instance == "" {
		s.Instance = first
	}
	clear()
	return true, nil
}
