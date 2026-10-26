package instance

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ark4660/armstrong/internal/app"
)

func TestSlug(t *testing.T) {
	tests := map[string]string{
		"Fabric 26.3":      "fabric-26.3",
		"  My  Cool Pack ": "my-cool-pack",
		"Ünïcode ✨":        "n-code",
		"...":              "instance",
		"a/b\\c":           "a-b-c",
		"keep_under.score": "keep_under.score",
	}
	for in, want := range tests {
		if got := slug(in); got != want {
			t.Errorf("slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCreateListRenameDuplicateDelete(t *testing.T) {
	s := Store{Root: t.TempDir()}
	a, err := s.Create(Instance{Name: "Fabric 26.3", Minecraft: "26.3", Loader: LoaderFabric})
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Create(Instance{Name: "Fabric 26.3", Minecraft: "26.3", Loader: LoaderFabric})
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != "fabric-26.3" || b.ID != "fabric-26.3-2" {
		t.Fatalf("ids: %s, %s", a.ID, b.ID)
	}
	if a.MemoryMB != DefaultMemoryMB {
		t.Errorf("default memory = %d", a.MemoryMB)
	}
	for _, l := range []string{"forge", LoaderVanilla} {
		if _, err := s.Create(Instance{Name: "x", Minecraft: "26.3", Loader: l}); err == nil {
			t.Errorf("loader %q should be rejected for new instances", l)
		}
	}
	if _, err := s.Create(Instance{Name: "  ", Minecraft: "26.3", Loader: LoaderFabric}); err == nil {
		t.Error("empty name should be rejected")
	}

	// Most recently played first.
	b.LastPlayed = time.Now()
	if err := s.Save(b); err != nil {
		t.Fatal(err)
	}
	list, err := s.List()
	if err != nil || len(list) != 2 || list[0].ID != b.ID {
		t.Fatalf("List = %v, %v", list, err)
	}

	if err := s.Rename(a, "Renamed"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get(a.ID); got.Name != "Renamed" || got.ID != "fabric-26.3" {
		t.Errorf("after rename: %+v", got)
	}

	// Duplicate copies the game dir, symlinks included.
	world := filepath.Join(a.GameDir(), "saves", "World", "level.dat")
	os.MkdirAll(filepath.Dir(world), 0o755)
	os.WriteFile(world, []byte("data"), 0o644)
	os.Symlink("saves", filepath.Join(a.GameDir(), "link"))
	c, err := s.Duplicate(a, "Renamed copy")
	if err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(c.GameDir(), "saves", "World", "level.dat")); err != nil || string(data) != "data" {
		t.Errorf("copied world: %q, %v", data, err)
	}
	if target, err := os.Readlink(filepath.Join(c.GameDir(), "link")); err != nil || target != "saves" {
		t.Errorf("copied symlink: %q, %v", target, err)
	}
	if !c.LastPlayed.IsZero() || c.ID == a.ID {
		t.Errorf("duplicate: %+v", c)
	}

	if err := s.Delete(c); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(c.Dir()); !os.IsNotExist(err) {
		t.Errorf("deleted instance still exists: %v", err)
	}
	if err := s.Delete(&Instance{ID: ".."}); err == nil {
		t.Error("deleting id .. must be refused")
	}
	if _, err := s.Get("../etc"); err == nil {
		t.Error("Get(../etc) must be refused")
	}
}

func TestMigrateProfiles(t *testing.T) {
	paths := app.Paths{Root: t.TempDir()}
	old := filepath.Join(paths.Root, "profiles", "default", "saves")
	os.MkdirAll(old, 0o755)
	os.WriteFile(filepath.Join(old, "w.dat"), []byte("w"), 0o644)
	s := app.Settings{LegacyOfflineName: "Steve", LegacyMinecraft: "26.3", LegacyProfile: "default", LegacyMemoryMB: 6144}

	changed, err := Migrate(paths, &s)
	if err != nil || !changed {
		t.Fatalf("Migrate = %v, %v", changed, err)
	}
	if s.Instance != "default" || s.LegacyProfile != "" || s.LegacyMemoryMB != 0 || s.LegacyOfflineName != "Steve" {
		t.Errorf("settings after migrate: %+v", s)
	}
	inst, err := Store{Root: paths.Instances()}.Get("default")
	if err != nil {
		t.Fatal(err)
	}
	if inst.Minecraft != "26.3" || inst.Loader != LoaderFabric || inst.MemoryMB != 6144 {
		t.Errorf("migrated instance: %+v", inst)
	}
	if _, err := os.Stat(filepath.Join(inst.GameDir(), "saves", "w.dat")); err != nil {
		t.Errorf("world not moved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(paths.Root, "profiles")); !os.IsNotExist(err) {
		t.Errorf("profiles dir should be gone: %v", err)
	}

	// Running again is a no-op.
	if changed, err := Migrate(paths, &s); err != nil || changed {
		t.Errorf("second Migrate = %v, %v", changed, err)
	}
}

func TestMigrateFreshInstall(t *testing.T) {
	paths := app.Paths{Root: t.TempDir()}
	var s app.Settings
	if changed, err := Migrate(paths, &s); err != nil || changed {
		t.Fatalf("Migrate on empty dir = %v, %v", changed, err)
	}
	if list, _ := (Store{Root: paths.Instances()}).List(); len(list) != 0 {
		t.Errorf("fresh install got instances: %v", list)
	}
}
