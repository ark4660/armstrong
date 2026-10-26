package client

import (
	"archive/zip"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ark4660/armstrong/internal/app"
	"github.com/ark4660/armstrong/internal/download"
	"github.com/ark4660/armstrong/internal/game"
	"github.com/ark4660/armstrong/internal/modrinth"
)

func TestBundledManifests(t *testing.T) {
	all, err := Bundled()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) == 0 {
		t.Fatal("no bundled manifests")
	}
	for mc, m := range all {
		ids := map[string]bool{}
		for _, mod := range m.Mods {
			if mod.ID == "" || mod.SHA1 == "" || mod.Size <= 0 || !strings.HasPrefix(mod.URL, "https://") {
				t.Errorf("%s: incomplete entry %+v", mc, mod)
			}
			if err := safeFileName(mod.File); err != nil {
				t.Errorf("%s: %v", mc, err)
			}
			if ids[mod.ID] {
				t.Errorf("%s: duplicate mod %s", mc, mod.ID)
			}
			ids[mod.ID] = true
		}
		for _, mod := range m.Mods {
			for _, r := range mod.Requires {
				if !ids[r] {
					t.Errorf("%s: %s requires %s, which isn't in the manifest", mc, mod.ID, r)
				}
			}
		}
		core := map[string]bool{}
		for _, mod := range m.Enabled(nil) {
			core[mod.ID] = true
		}
		wantCore := []string{"fabric-api", "sodium", "lithium"}
		if game.IsLegacyFabric(mc) {
			wantCore = []string{"legacy-fabric-api", "phosphor-legacy"}
		}
		for _, want := range wantCore {
			if !core[want] {
				t.Errorf("%s: core is missing %s", mc, want)
			}
		}
		if m.FabricLoader == "" {
			t.Errorf("%s: no pinned Fabric loader", mc)
		}
	}
}

func testManifest() *Manifest {
	return &Manifest{Minecraft: "26.3", Mods: []Mod{
		{ID: "fabric-api", Name: "Fabric API", File: "fapi.jar"},
		{ID: "sodium", Name: "Sodium", Provides: []string{"sodium_alias"}, File: "sodium.jar"},
		{ID: "iris", Slug: "iris", Optional: true, Requires: []string{"sodium"}, File: "iris.jar"},
		{ID: "moreculling", Slug: "moreculling", Optional: true, Requires: []string{"cloth-config"}, File: "mc.jar"},
		{ID: "cloth-config", Optional: true, File: "cloth.jar"},
	}}
}

func ids(mods []Mod) []string {
	var out []string
	for _, m := range mods {
		out = append(out, m.ID)
	}
	return out
}

func TestEnabled(t *testing.T) {
	m := testManifest()
	if got := ids(m.Enabled(nil)); !reflect.DeepEqual(got, []string{"fabric-api", "sodium"}) {
		t.Errorf("core = %v", got)
	}
	got := ids(m.Enabled([]string{"moreculling"}))
	if want := []string{"fabric-api", "sodium", "moreculling", "cloth-config"}; !reflect.DeepEqual(got, want) {
		t.Errorf("with moreculling = %v, want %v", got, want)
	}
}

func jar(t *testing.T, path, modJSON string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, _ := zw.Create("fabric.mod.json")
	w.Write([]byte(modJSON))
	zw.Close()
	f.Close()
}

func TestDisableDuplicates(t *testing.T) {
	dir := t.TempDir()
	jar(t, filepath.Join(dir, "my-sodium.jar"), `{"id":"sodium","version":"0.9.3"}`)
	jar(t, filepath.Join(dir, "alias.jar"), `{"id":"other","provides":["sodium_alias"]}`)
	jar(t, filepath.Join(dir, "modmenu.jar"), `{"id":"modmenu"}`)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hi"), 0o644)
	os.WriteFile(filepath.Join(dir, "my-sodium.jar.disabled"), []byte("older"), 0o644)

	layer := &Layer{Mods: testManifest().Enabled(nil)}
	got, err := DisableDuplicates(dir, layer)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("disabled = %+v", got)
	}
	for _, f := range []string{"modmenu.jar", "notes.txt", "my-sodium.jar.disabled", "my-sodium.jar.2.disabled", "alias.jar.disabled"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "my-sodium.jar")); !os.IsNotExist(err) {
		t.Error("my-sodium.jar should have been renamed")
	}
	if none, err := DisableDuplicates(filepath.Join(dir, "missing"), layer); err != nil || none != nil {
		t.Errorf("missing mods dir: %v, %v", none, err)
	}
}

func TestSyncDownloadsAndCleans(t *testing.T) {
	body := []byte("jar bytes")
	sum := sha1.Sum(body)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(body) }))
	defer srv.Close()

	paths := app.Paths{Root: t.TempDir()}
	m := &Manifest{Minecraft: "26.3", Mods: []Mod{
		{ID: "a", File: "a.jar", URL: srv.URL + "/a", SHA1: hex.EncodeToString(sum[:]), Size: int64(len(body))},
		{ID: "b", File: "b.jar", URL: srv.URL + "/b", SHA1: hex.EncodeToString(sum[:]), Size: int64(len(body)), Optional: true},
	}}
	dir := Dir(paths, "26.3")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "stale.jar"), []byte("old"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.jar"), []byte("another instance's add-on"), 0o644)

	layer, err := Sync(context.Background(), download.New("test"), paths, m, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(layer.Files, []string{filepath.Join(dir, "a.jar")}) {
		t.Errorf("files = %v", layer.Files)
	}
	if _, err := os.Stat(filepath.Join(dir, "stale.jar")); !os.IsNotExist(err) {
		t.Error("stale.jar should be removed")
	}
	if _, err := os.Stat(filepath.Join(dir, "b.jar")); err != nil {
		t.Error("b.jar is in the manifest and must be kept")
	}

	m.Mods[0].File = "../escape.jar"
	if _, err := Sync(context.Background(), download.New("test"), paths, m, nil, nil); err == nil {
		t.Error("unsafe file name must be rejected")
	}
}

func TestVersionLess(t *testing.T) {
	order := []string{"1.20", "1.20.1", "1.21", "1.21.11", "26.1", "26.1.2", "26.3"}
	for i := 1; i < len(order); i++ {
		if !versionLess(order[i-1], order[i]) || versionLess(order[i], order[i-1]) {
			t.Errorf("%s should sort before %s", order[i-1], order[i])
		}
	}
}

func TestCandidatesAndPins(t *testing.T) {
	vs := []modrinth.Version{
		{ID: "a1", VersionType: "alpha", DatePublished: "2026-10-20"},
		{ID: "b1", VersionType: "beta", DatePublished: "2026-10-15"},
		{ID: "r1", VersionType: "release", DatePublished: "2026-09-15"},
		{ID: "r2", VersionType: "release", DatePublished: "2026-10-01"},
	}
	var got []string
	for _, v := range candidates(vs) {
		got = append(got, v.ID)
	}
	if !reflect.DeepEqual(got, []string{"r2", "r1", "b1"}) {
		t.Errorf("candidates = %v", got)
	}

	r := &resolver{chosen: map[string]*choice{"sodium": {version: &modrinth.Version{ID: "s-092"}}}}
	pinsOther := &modrinth.Version{Dependencies: []modrinth.Dependency{{ProjectID: "sodium", VersionID: "s-093", Type: "required"}}}
	pinsSame := &modrinth.Version{Dependencies: []modrinth.Dependency{{ProjectID: "sodium", VersionID: "s-092", Type: "required"}}}
	if !r.pinsConflict(pinsOther) || r.pinsConflict(pinsSame) {
		t.Error("pinsConflict misjudged an exact-version dependency")
	}
}
