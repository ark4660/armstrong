package mods

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ark4660/armstrong/internal/client"
	"github.com/ark4660/armstrong/internal/download"
)

func jarBytes(t *testing.T, modJSON string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("fabric.mod.json")
	w.Write([]byte(modJSON))
	zw.Close()
	return buf.Bytes()
}

// fakeSite serves jars and answers like a mod site.
type fakeSite struct {
	srv      *httptest.Server
	jars     map[string][]byte // file name → bytes
	versions map[string][]Version
	byID     map[string]Version
}

func newFakeSite(t *testing.T) *fakeSite {
	f := &fakeSite{jars: map[string][]byte{}, versions: map[string][]Version{}, byID: map[string]Version{}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, ok := f.jars[strings.TrimPrefix(r.URL.Path, "/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(data)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

// add publishes a version of project whose jar has the given fabric.mod.json.
func (f *fakeSite) add(t *testing.T, project, id, number, channel, published, modJSON string, deps ...Dep) Version {
	data := jarBytes(t, modJSON)
	sum := sha1.Sum(data)
	name := fmt.Sprintf("%s-%s.jar", project, number)
	f.jars[name] = data
	v := Version{ID: id, ProjectID: project, Number: number, Channel: channel, Published: published,
		File: File{URL: f.srv.URL + "/" + name, Name: name, SHA1: hex.EncodeToString(sum[:]), Size: int64(len(data))}, Deps: deps}
	f.versions[project] = append(f.versions[project], v)
	f.byID[id] = v
	return v
}

func (f *fakeSite) Name() string { return "Fake" }
func (f *fakeSite) Versions(_ context.Context, project, _, _ string) ([]Version, error) {
	return f.versions[project], nil
}
func (f *fakeSite) Version(_ context.Context, id string) (*Version, error) {
	v, ok := f.byID[id]
	if !ok {
		return nil, errors.New("no such version")
	}
	return &v, nil
}
func (f *fakeSite) Identify(_ context.Context, hashes []string) (map[string]Version, error) {
	out := map[string]Version{}
	for _, vs := range f.versions {
		for _, v := range vs {
			for _, h := range hashes {
				if h == v.File.SHA1 {
					out[h] = v
				}
			}
		}
	}
	return out, nil
}
func (f *fakeSite) Latest(_ context.Context, hashes []string, _, _ string, _ []string) (map[string]Version, error) {
	out := map[string]Version{}
	for _, vs := range f.versions {
		var newest Version
		has := false
		for _, v := range vs {
			for _, h := range hashes {
				if h == v.File.SHA1 {
					has = true
				}
			}
			if v.Published > newest.Published {
				newest = v
			}
		}
		if has {
			for _, v := range vs {
				for _, h := range hashes {
					if h == v.File.SHA1 {
						out[h] = newest
					}
				}
			}
		}
	}
	return out, nil
}

// clientLayer builds a client layer with Sodium (which breaks "optifabric")
// and an optional Iris add-on that isn't enabled.
func clientLayer(t *testing.T) *client.Layer {
	dir := t.TempDir()
	sodium := filepath.Join(dir, "sodium.jar")
	os.WriteFile(sodium, jarBytes(t, `{"id":"sodium","version":"0.9.2+mc26.3","name":"Sodium","breaks":{"optifabric":"*"}}`), 0o644)
	fapi := filepath.Join(dir, "fabric-api.jar")
	os.WriteFile(fapi, jarBytes(t, `{"id":"fabric-api","version":"0.165.0+26.3","name":"Fabric API","provides":["fabric"]}`), 0o644)
	m := &client.Manifest{Minecraft: "26.3", Mods: []client.Mod{
		{ID: "fabric-api", Name: "Fabric API", Project: "P-fapi"},
		{ID: "sodium", Name: "Sodium", Project: "P-sodium"},
		{ID: "iris", Name: "Iris Shaders", Project: "P-iris", Optional: true},
	}}
	return &client.Layer{Manifest: m, Mods: m.Mods[:2], Files: []string{fapi, sodium}}
}

func newManager(t *testing.T, site *fakeSite) *Manager {
	root := t.TempDir()
	return &Manager{
		ModsDir: filepath.Join(root, "minecraft", "mods"), IndexPath: filepath.Join(root, "mods.json"),
		CacheDir: filepath.Join(root, "cache"), Minecraft: "26.3", Loader: "0.20.1", Java: "25",
		Layer: clientLayer(t), Source: site, DL: download.New("test"),
	}
}

func stepNames(p *Plan) []string {
	var out []string
	for _, s := range p.Steps {
		out = append(out, s.Name())
	}
	return out
}

func TestInstallWithDependenciesAndFallback(t *testing.T) {
	site := newFakeSite(t)
	site.add(t, "P-lib", "lib-1", "1.0.0", "release", "2026-09-01", `{"id":"lib","version":"1.0.0","name":"Lib","depends":{"fabric":"*"}}`)
	// The newest map needs a newer loader than the instance has; the older one fits.
	site.add(t, "P-map", "map-2", "2.0.0", "release", "2026-10-01", `{"id":"map","version":"2.0.0","name":"Map","depends":{"fabricloader":">=0.99","lib":">=1.0"}}`, Dep{ProjectID: "P-lib", Kind: "required"})
	site.add(t, "P-map", "map-1", "1.5.0", "release", "2026-09-15", `{"id":"map","version":"1.5.0","name":"Map","depends":{"fabricloader":">=0.20","minecraft":"~26.3","lib":">=1.0"}}`, Dep{ProjectID: "P-lib", Kind: "required"})
	m := newManager(t, site)
	ctx := context.Background()

	plan, err := m.PlanInstall(ctx, "P-map")
	if err != nil {
		t.Fatal(err)
	}
	if got := stepNames(plan); len(got) != 2 || plan.Steps[0].Version.ID != "map-1" || got[1] != "Lib" {
		t.Fatalf("plan = %v (%s)", got, plan.Steps[0].Version.ID)
	}
	if err := m.Commit(plan); err != nil {
		t.Fatal(err)
	}
	installed, err := m.Scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(installed) != 2 {
		t.Fatalf("installed = %d", len(installed))
	}
	for _, i := range installed {
		if len(i.Problems) > 0 {
			t.Errorf("%s problems: %v", i.Name(), i.Problems)
		}
		if i.Name() == "Lib" && (i.Explicit || len(i.RequiredBy) != 1) {
			t.Errorf("Lib should be a dependency of Map: %+v", i)
		}
	}

	// Removing Map takes Lib with it, since nothing else needs Lib.
	var mapMod *Installed
	for _, i := range installed {
		if i.Name() == "Map" {
			mapMod = i
		}
	}
	r, err := m.PlanRemove(mapMod)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Remove) != 2 {
		t.Fatalf("removal = %d mods", len(r.Remove))
	}
	if err := m.CommitRemove(r); err != nil {
		t.Fatal(err)
	}
	if left, _ := m.Scan(); len(left) != 0 {
		t.Errorf("left after remove: %d", len(left))
	}
}

func TestBlockedInstalls(t *testing.T) {
	site := newFakeSite(t)
	site.add(t, "P-optifabric", "of-1", "1.0", "release", "2026-01-01", `{"id":"optifabric","version":"1.0","name":"OptiFabric"}`)
	site.add(t, "P-old", "old-1", "1.0", "release", "2026-01-01", `{"id":"oldmod","version":"1.0","name":"Old Mod","depends":{"minecraft":"1.21.1"}}`)
	site.add(t, "P-breaker", "br-1", "1.0", "release", "2026-01-01", `{"id":"breaker","version":"1.0","name":"Breaker","breaks":{"sodium":"<1.0"}}`)
	m := newManager(t, site)
	ctx := context.Background()

	var noFit *ErrNoFit
	if _, err := m.PlanInstall(ctx, "P-optifabric"); !errors.As(err, &noFit) || !strings.Contains(err.Error(), "Sodium") {
		t.Errorf("OptiFabric should be blocked by Sodium's breaks: %v", err)
	}
	if _, err := m.PlanInstall(ctx, "P-old"); err == nil || !strings.Contains(err.Error(), "made for Minecraft 1.21.1") {
		t.Errorf("wrong Minecraft version should be blocked: %v", err)
	}
	if _, err := m.PlanInstall(ctx, "P-breaker"); err == nil || !strings.Contains(err.Error(), "doesn't work with Sodium") {
		t.Errorf("a mod breaking a client mod should be blocked: %v", err)
	}

	var inClient *ErrInClient
	if _, err := m.PlanInstall(ctx, "P-sodium"); !errors.As(err, &inClient) || inClient.Addon {
		t.Errorf("Sodium is in the client: %v", err)
	}
	if _, err := m.PlanInstall(ctx, "P-iris"); !errors.As(err, &inClient) || !inClient.Addon {
		t.Errorf("Iris should point to the add-on: %v", err)
	}
}

func TestReverseChecksAndValidation(t *testing.T) {
	site := newFakeSite(t)
	site.add(t, "P-a", "a-1", "1.0", "release", "2026-01-01", `{"id":"a","version":"1.0","name":"A","breaks":{"b":"*"}}`)
	site.add(t, "P-b", "b-1", "1.0", "release", "2026-01-01", `{"id":"b","version":"1.0","name":"B"}`)
	site.add(t, "P-c", "c-1", "1.0", "release", "2026-01-01", `{"id":"c","version":"1.0","name":"C","depends":{"b":"*"}}`)
	m := newManager(t, site)
	ctx := context.Background()

	plan, err := m.PlanInstall(ctx, "P-a")
	if err != nil {
		t.Fatal(err)
	}
	m.Commit(plan)
	// B is declared broken by the installed A: blocked from the other side.
	if _, err := m.PlanInstall(ctx, "P-b"); err == nil || !strings.Contains(err.Error(), "A (installed") {
		t.Errorf("B should be blocked by installed A: %v", err)
	}

	// C needs B; with A removed and C added by hand, validation reports it.
	installed, _ := m.Scan()
	r, _ := m.PlanRemove(installed[0])
	m.CommitRemove(r)
	os.WriteFile(filepath.Join(m.ModsDir, "c.jar"), site.jars["P-c-1.0.jar"], 0o644)
	installed, _ = m.Scan()
	if len(installed) != 1 || !HasErrors(installed[0].Problems) || !strings.Contains(installed[0].Problems[0].Text, "needs b") {
		t.Errorf("C should report missing B: %+v", installed[0].Problems)
	}
	if err := m.Identify(ctx); err != nil {
		t.Fatal(err)
	}
	installed, _ = m.Scan()
	if installed[0].Source == nil || installed[0].Source.Project != "P-c" {
		t.Errorf("Identify should find C's project: %+v", installed[0].Source)
	}
}

func TestPinnedDependencyAndUpdates(t *testing.T) {
	site := newFakeSite(t)
	site.add(t, "P-lib", "lib-1", "1.0", "release", "2026-08-01", `{"id":"lib","version":"1.0","name":"Lib"}`)
	site.add(t, "P-lib", "lib-2", "2.0", "release", "2026-09-01", `{"id":"lib","version":"2.0","name":"Lib"}`)
	site.add(t, "P-user", "u-1", "1.0", "release", "2026-07-01", `{"id":"user","version":"1.0","name":"User"}`, Dep{ProjectID: "P-lib", Kind: "required"})
	m := newManager(t, site)
	ctx := context.Background()

	// Install Lib 1.0 explicitly.
	plan, err := m.plan(ctx, nil, "P-lib", "lib-1", "")
	if err != nil {
		t.Fatal(err)
	}
	m.Commit(plan)

	// A mod pinning Lib 2.0 doesn't fit while Lib 1.0 is installed.
	site.add(t, "P-pinner", "p-1", "1.0", "release", "2026-10-01", `{"id":"pinner","version":"1.0","name":"Pinner"}`, Dep{ProjectID: "P-lib", VersionID: "lib-2", Kind: "required"})
	if _, err := m.PlanInstall(ctx, "P-pinner"); err == nil || !strings.Contains(err.Error(), "update it first") {
		t.Errorf("pinned newer dependency should block: %v", err)
	}

	installed, _ := m.Scan()
	updates, err := m.CheckUpdates(ctx, installed)
	if err != nil || len(updates) != 1 || updates[0].Version.ID != "lib-2" {
		t.Fatalf("updates = %+v, %v", updates, err)
	}
	up, err := m.PlanUpdate(ctx, updates[0].Installed, updates[0].Version.ID)
	if err != nil {
		t.Fatal(err)
	}
	m.Commit(up)
	installed, _ = m.Scan()
	if len(installed) != 1 || installed[0].Version() != "2.0" {
		t.Errorf("after update: %+v", installed)
	}
}

func TestSkipsDependencyWithoutBuildForLoader(t *testing.T) {
	site := newFakeSite(t)
	// "P-forgeonly" has no versions here, like a Forge-only library listed by a universal jar.
	site.add(t, "P-zoom", "z-2", "2.0", "release", "2026-10-08", `{"id":"zoom","version":"2.0","name":"Zoom"}`, Dep{ProjectID: "P-forgeonly", Kind: "required"})
	site.add(t, "P-zoom", "z-1", "1.0", "release", "2026-09-01", `{"id":"zoom","version":"1.0","name":"Zoom"}`)
	m := newManager(t, site)
	plan, err := m.PlanInstall(context.Background(), "P-zoom")
	if err != nil || len(plan.Steps) != 1 || plan.Steps[0].Version.ID != "z-2" {
		t.Fatalf("plan = %+v, %v", plan, err)
	}

	// But a dependency the jar itself declares is still enforced.
	site.add(t, "P-needy", "n-1", "1.0", "release", "2026-10-08", `{"id":"needy","version":"1.0","name":"Needy","depends":{"forgeonly":"*"}}`, Dep{ProjectID: "P-forgeonly", Kind: "required"})
	if _, err := m.PlanInstall(context.Background(), "P-needy"); err == nil || !strings.Contains(err.Error(), "needs forgeonly") {
		t.Errorf("a dependency in fabric.mod.json must still block: %v", err)
	}
}
