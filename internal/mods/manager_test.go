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
	"sync"
	"testing"
	"time"

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

	mu           sync.Mutex
	downloads    map[string]int // jar file name → times served
	versionLists map[string]int // project → Versions calls
	ranges       bool           // serve byte ranges, like Modrinth's CDN
	bytesSent    int64
}

type countingWriter struct {
	http.ResponseWriter
	site *fakeSite
}

func (c *countingWriter) Write(p []byte) (int, error) {
	c.site.mu.Lock()
	c.site.bytesSent += int64(len(p))
	c.site.mu.Unlock()
	return c.ResponseWriter.Write(p)
}

func (f *fakeSite) served(name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.downloads[name]
}

func newFakeSite(t *testing.T) *fakeSite {
	f := &fakeSite{jars: map[string][]byte{}, versions: map[string][]Version{}, byID: map[string]Version{},
		downloads: map[string]int{}, versionLists: map[string]int{}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		f.mu.Lock()
		f.downloads[name]++
		f.mu.Unlock()
		data, ok := f.jars[name]
		if !ok {
			http.NotFound(w, r)
			return
		}
		cw := &countingWriter{ResponseWriter: w, site: f}
		if f.ranges {
			http.ServeContent(cw, r, name, time.Time{}, bytes.NewReader(data))
			return
		}
		cw.Write(data)
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
	f.mu.Lock()
	f.versionLists[project]++
	f.mu.Unlock()
	return f.versions[project], nil
}
func (f *fakeSite) Version(_ context.Context, id string) (*Version, error) {
	v, ok := f.byID[id]
	if !ok {
		return nil, errors.New("no such version")
	}
	return &v, nil
}
func (f *fakeSite) Title(_ context.Context, project string) (string, error) {
	return strings.TrimPrefix(project, "P-"), nil
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

// clientLayer builds a client layer with Sodium (which breaks "optifabric"
// and provides "indium") and an optional Iris add-on that isn't enabled.
func clientLayer(t *testing.T) *client.Layer {
	dir := t.TempDir()
	sodium := filepath.Join(dir, "sodium.jar")
	os.WriteFile(sodium, jarBytes(t, `{"id":"sodium","version":"0.9.2+mc26.3","name":"Sodium","provides":["indium"],"breaks":{"optifabric":"*"}}`), 0o644)
	fapi := filepath.Join(dir, "fabric-api.jar")
	os.WriteFile(fapi, jarBytes(t, `{"id":"fabric-api","version":"0.165.0+26.3","name":"Fabric API","provides":["fabric"]}`), 0o644)
	m := &client.Manifest{Minecraft: "26.3", Mods: []client.Mod{
		{ID: "fabric-api", Name: "Fabric API", Project: "P-fapi"},
		{ID: "sodium", Name: "Sodium", Project: "P-sodium", Provides: []string{"indium"}},
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

	plan, err := m.PlanInstall(ctx, "P-map", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := stepNames(plan); len(got) != 2 || plan.Steps[0].Version.ID != "map-1" || got[1] != "Lib" {
		t.Fatalf("plan = %v (%s)", got, plan.Steps[0].Version.ID)
	}
	if err := m.Commit(ctx, plan); err != nil {
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
	if _, err := m.PlanInstall(ctx, "P-optifabric", nil); !errors.As(err, &noFit) || !strings.Contains(err.Error(), "Sodium") {
		t.Errorf("OptiFabric should be blocked by Sodium's breaks: %v", err)
	}
	if _, err := m.PlanInstall(ctx, "P-old", nil); err == nil || !strings.Contains(err.Error(), "made for Minecraft 1.21.1") {
		t.Errorf("wrong Minecraft version should be blocked: %v", err)
	}
	if _, err := m.PlanInstall(ctx, "P-breaker", nil); err == nil || !strings.Contains(err.Error(), "doesn't work with Sodium") {
		t.Errorf("a mod breaking a client mod should be blocked: %v", err)
	}

	var inClient *ErrInClient
	if _, err := m.PlanInstall(ctx, "P-sodium", nil); !errors.As(err, &inClient) || inClient.Addon {
		t.Errorf("Sodium is in the client: %v", err)
	}
	if _, err := m.PlanInstall(ctx, "P-iris", nil); !errors.As(err, &inClient) || !inClient.Addon {
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

	plan, err := m.PlanInstall(ctx, "P-a", nil)
	if err != nil {
		t.Fatal(err)
	}
	m.Commit(ctx, plan)
	// B is declared broken by the installed A: blocked from the other side.
	if _, err := m.PlanInstall(ctx, "P-b", nil); err == nil || !strings.Contains(err.Error(), "A (installed") {
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
	plan, err := m.plan(ctx, nil, "P-lib", "lib-1", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	m.Commit(ctx, plan)

	// A mod pinning Lib 2.0 doesn't fit while Lib 1.0 is installed.
	site.add(t, "P-pinner", "p-1", "1.0", "release", "2026-10-01", `{"id":"pinner","version":"1.0","name":"Pinner"}`, Dep{ProjectID: "P-lib", VersionID: "lib-2", Kind: "required"})
	if _, err := m.PlanInstall(ctx, "P-pinner", nil); err == nil || !strings.Contains(err.Error(), "update it first") {
		t.Errorf("pinned newer dependency should block: %v", err)
	}

	installed, _ := m.Scan()
	updates, err := m.CheckUpdates(ctx, installed)
	if err != nil || len(updates) != 1 || updates[0].Version.ID != "lib-2" {
		t.Fatalf("updates = %+v, %v", updates, err)
	}
	up, err := m.PlanUpdate(ctx, updates[0].Installed, updates[0].Version.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	m.Commit(ctx, up)
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
	plan, err := m.PlanInstall(context.Background(), "P-zoom", nil)
	if err != nil || len(plan.Steps) != 1 || plan.Steps[0].Version.ID != "z-2" {
		t.Fatalf("plan = %+v, %v", plan, err)
	}

	// But a dependency the jar itself declares is still enforced.
	site.add(t, "P-needy", "n-1", "1.0", "release", "2026-10-08", `{"id":"needy","version":"1.0","name":"Needy","depends":{"forgeonly":"*"}}`, Dep{ProjectID: "P-forgeonly", Kind: "required"})
	if _, err := m.PlanInstall(context.Background(), "P-needy", nil); err == nil || !strings.Contains(err.Error(), "needs forgeonly") {
		t.Errorf("a dependency in fabric.mod.json must still block: %v", err)
	}

	// A project with no build at all is named, not shown by its id.
	if _, err := m.PlanInstall(context.Background(), "P-forgeonly", nil); err == nil || !strings.HasPrefix(err.Error(), "forgeonly has no version") {
		t.Errorf("no build: %v", err)
	}
}

func TestFailingDependencyResolvedOnce(t *testing.T) {
	site := newFakeSite(t)
	// No version of Lib fits (made for another Minecraft), so no version of App can.
	for i := 1; i <= 3; i++ {
		site.add(t, "P-lib", fmt.Sprintf("lib-%d", i), fmt.Sprintf("%d.0", i), "release", fmt.Sprintf("2026-09-0%d", i),
			fmt.Sprintf(`{"id":"lib","version":"%d.0","name":"Lib","depends":{"minecraft":"1.21.1"}}`, i))
	}
	for i := 1; i <= 4; i++ {
		site.add(t, "P-app", fmt.Sprintf("app-%d", i), fmt.Sprintf("%d.0", i), "release", fmt.Sprintf("2026-10-0%d", i),
			fmt.Sprintf(`{"id":"app","version":"%d.0","name":"App","depends":{"lib":"*"}}`, i), Dep{ProjectID: "P-lib", Kind: "required"})
	}
	m := newManager(t, site)
	var checked []string
	_, err := m.PlanInstall(context.Background(), "P-app", func(s string) { checked = append(checked, s) })
	var noFit *ErrNoFit
	if !errors.As(err, &noFit) || !strings.Contains(err.Error(), "made for Minecraft 1.21.1") {
		t.Fatalf("err = %v", err)
	}
	libChecks := 0
	for _, c := range checked {
		if strings.HasPrefix(c, "P-lib-") {
			libChecks++
		}
	}
	// 4 App candidates, but Lib's 3 versions are looked at once.
	if libChecks != 3 || site.versionLists["P-lib"] != 1 {
		t.Errorf("Lib checked %d times, listed %d times; want 3 and 1 (progress: %v)", libChecks, site.versionLists["P-lib"], checked)
	}
}

func TestAddonDependencies(t *testing.T) {
	site := newFakeSite(t)
	// The site says it needs Iris, a client add-on that's switched off.
	site.add(t, "P-shaderpack", "sp-1", "1.0", "release", "2026-10-01", `{"id":"shaderthing","version":"1.0","name":"Shader Thing","depends":{"iris":"*"}}`,
		Dep{ProjectID: "P-iris", Kind: "required"})
	// Only the jar says so.
	site.add(t, "P-quiet", "q-1", "1.0", "release", "2026-10-01", `{"id":"quiet","version":"1.0","name":"Quiet","depends":{"iris":">=1.0"}}`)
	site.add(t, "P-lonely", "l-1", "1.0", "release", "2026-10-01", `{"id":"lonely","version":"1.0","name":"Lonely","depends":{"nothing":"*"}}`)
	m := newManager(t, site)
	ctx := context.Background()

	_, err := m.PlanInstall(ctx, "P-shaderpack", nil)
	if err == nil || !strings.Contains(err.Error(), "needs Iris Shaders, an Armstrong add-on: switch it on") {
		t.Errorf("shader pack: %v", err)
	}
	if !strings.Contains(err.Error(), "no version of shaderpack fits") {
		t.Errorf("the project's name should come from the site when its jar wasn't downloaded: %v", err)
	}
	if n := site.served("P-shaderpack-1.0.jar"); n != 0 {
		t.Errorf("the jar was downloaded %d times; the site already said it can't fit", n)
	}
	if _, err := m.PlanInstall(ctx, "P-quiet", nil); err == nil || !strings.Contains(err.Error(), "Quiet needs Iris Shaders, an Armstrong add-on") {
		t.Errorf("quiet: %v", err)
	}
	if _, err := m.PlanInstall(ctx, "P-lonely", nil); err == nil || !strings.Contains(err.Error(), "needs nothing, which isn't installed") {
		t.Errorf("lonely (no stray space before the comma): %v", err)
	}
}

func TestProvidedByClientMod(t *testing.T) {
	site := newFakeSite(t)
	site.add(t, "P-indium", "in-1", "1.0", "release", "2026-10-01", `{"id":"indium","version":"1.0","name":"Indium"}`)
	site.add(t, "P-fancy", "f-1", "1.0", "release", "2026-10-01", `{"id":"fancy","version":"1.0","name":"Fancy","depends":{"indium":"*"}}`,
		Dep{ProjectID: "P-indium", Kind: "required"})
	m := newManager(t, site)
	ctx := context.Background()

	var inClient *ErrInClient
	if _, err := m.PlanInstall(ctx, "P-indium", nil); !errors.As(err, &inClient) || err.Error() != "Indium is built into Sodium, which is part of the Armstrong client" {
		t.Errorf("Indium: %v", err)
	}
	plan, err := m.PlanInstall(ctx, "P-fancy", nil)
	if err != nil || len(plan.Steps) != 1 {
		t.Fatalf("a dependency Sodium provides counts as there: %v %v", plan, err)
	}
}

func TestOwnRequirementsBeforeDependencies(t *testing.T) {
	site := newFakeSite(t)
	site.add(t, "P-big", "big-1", "1.0", "release", "2026-10-01", `{"id":"big","version":"1.0","name":"Big"}`)
	site.add(t, "P-oldie", "o-1", "1.0", "release", "2026-10-01", `{"id":"oldie","version":"1.0","name":"Oldie","depends":{"minecraft":"1.21.1","big":"*"}}`,
		Dep{ProjectID: "P-big", Kind: "required"})
	m := newManager(t, site)
	if _, err := m.PlanInstall(context.Background(), "P-oldie", nil); err == nil || !strings.Contains(err.Error(), "made for Minecraft 1.21.1") {
		t.Fatalf("err = %v", err)
	}
	if site.versionLists["P-big"] != 0 || site.served("P-big-1.0.jar") != 0 {
		t.Errorf("a dependency was looked up for a version that can't load anyway")
	}
}

func TestPlanCancelled(t *testing.T) {
	site := newFakeSite(t)
	site.add(t, "P-a", "a-1", "1.0", "release", "2026-10-01", `{"id":"a","version":"1.0","name":"A"}`)
	m := newManager(t, site)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := m.PlanInstall(ctx, "P-a", nil); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestChooseAndCheckVersions(t *testing.T) {
	site := newFakeSite(t)
	site.add(t, "P-lib", "lib-1", "1.0", "release", "2026-08-01", `{"id":"lib","version":"1.0","name":"Lib"}`)
	// Newest first: 3.0 needs a newer loader, 2.0 fits, 1.0 is for another Minecraft.
	site.add(t, "P-tool", "t-1", "1.0", "release", "2026-08-01", `{"id":"tool","version":"1.0","name":"Tool","depends":{"minecraft":"1.21.1"}}`)
	site.add(t, "P-tool", "t-3", "3.0", "beta", "2026-10-01", `{"id":"tool","version":"3.0","name":"Tool","depends":{"fabricloader":">=0.99","lib":"*"}}`,
		Dep{ProjectID: "P-lib", Kind: "required"})
	site.add(t, "P-tool", "t-2", "2.0", "release", "2026-09-01", `{"id":"tool","version":"2.0","name":"Tool","depends":{"lib":"*"}}`,
		Dep{ProjectID: "P-lib", Kind: "required"})
	m := newManager(t, site)
	ctx := context.Background()

	vs, err := m.Versions(ctx, "P-tool")
	if err != nil || len(vs) != 3 || vs[0].ID != "t-3" || vs[1].ID != "t-2" || vs[2].ID != "t-1" {
		t.Fatalf("versions = %v, %v; want newest first", vs, err)
	}

	// The default is the newest that fits; a chosen version is planned as is.
	plan, err := m.PlanInstall(ctx, "P-tool", nil)
	if err != nil || plan.Steps[0].Version.ID != "t-2" {
		t.Fatalf("default install: %v %v", plan, err)
	}
	if _, err := m.PlanInstallVersion(ctx, "P-tool", "t-1", nil); err == nil || !strings.Contains(err.Error(), "made for Minecraft 1.21.1") {
		t.Errorf("chosen 1.0: %v", err)
	}

	site.versionLists["P-lib"] = 0
	got := map[string]string{}
	err = m.CheckVersions(ctx, "P-tool", []string{"t-3", "t-2", "t-1"}, func(id string, p *Plan, err error) {
		if err != nil {
			got[id] = err.Error()
		} else {
			got[id] = fmt.Sprintf("fits, %d jars", len(p.Steps))
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got["t-3"], "Fabric Loader >=0.99") || got["t-2"] != "fits, 2 jars" || !strings.Contains(got["t-1"], "made for Minecraft 1.21.1") {
		t.Errorf("checks = %v", got)
	}
	if site.versionLists["P-lib"] > 1 {
		t.Errorf("Lib's versions were listed %d times across the checks; want them shared", site.versionLists["P-lib"])
	}

	ctx2, cancel := context.WithCancel(ctx)
	cancel()
	if err := m.CheckVersions(ctx2, "P-tool", []string{"t-2"}, func(string, *Plan, error) { t.Error("reported after cancel") }); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled check: %v", err)
	}
}

func TestNoFitCountsUntriedVersions(t *testing.T) {
	site := newFakeSite(t)
	for i := 1; i <= maxCandidates+3; i++ {
		site.add(t, "P-old", fmt.Sprintf("o-%d", i), fmt.Sprintf("%d.0", i), "release", fmt.Sprintf("2026-%02d-01", (i%12)+1)+fmt.Sprintf("T00:00:%02d", i),
			fmt.Sprintf(`{"id":"old","version":"%d.0","name":"Old","depends":{"minecraft":"1.21.1"}}`, i))
	}
	m := newManager(t, site)
	var noFit *ErrNoFit
	if _, err := m.PlanInstall(context.Background(), "P-old", nil); !errors.As(err, &noFit) || noFit.Tried != maxCandidates || noFit.Untried != 3 {
		t.Errorf("err = %v (%+v)", err, noFit)
	}
}

// bigJar is a jar with fabric.mod.json and a large class file, like a real mod.
func bigJar(t *testing.T, modJSON string, size int) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("fabric.mod.json")
	w.Write([]byte(modJSON))
	w, _ = zw.CreateHeader(&zip.FileHeader{Name: "com/example/Big.class", Method: zip.Store})
	filler := make([]byte, size)
	for i := range filler {
		filler[i] = byte(i * 31)
	}
	w.Write(filler)
	zw.Close()
	return buf.Bytes()
}

func TestCheckingReadsOnlyMetadata(t *testing.T) {
	site := newFakeSite(t)
	site.ranges = true
	v := site.add(t, "P-huge", "h-1", "1.0", "release", "2026-10-01", `{"id":"huge","version":"1.0","name":"Huge"}`)
	data := bigJar(t, `{"id":"huge","version":"1.0","name":"Huge"}`, 8<<20)
	sum := sha1.Sum(data)
	site.jars[v.File.Name] = data
	v.File.SHA1, v.File.Size = hex.EncodeToString(sum[:]), int64(len(data))
	site.versions["P-huge"] = []Version{v}
	site.byID["h-1"] = v

	m := newManager(t, site)
	ctx := context.Background()
	plan, err := m.PlanInstall(ctx, "P-huge", nil)
	if err != nil || len(plan.Steps) != 1 {
		t.Fatalf("plan = %v, %v", plan, err)
	}
	if site.bytesSent > int64(len(data))/20 {
		t.Errorf("planning sent %d of the jar's %d bytes; want just its index and fabric.mod.json", site.bytesSent, len(data))
	}
	if _, err := os.Stat(plan.Steps[0].Path); err == nil {
		t.Error("the jar was downloaded while planning")
	}

	// Looking again needs no network: the metadata was kept.
	site.bytesSent = 0
	if _, err := m.PlanInstall(ctx, "P-huge", nil); err != nil || site.bytesSent != 0 {
		t.Errorf("second plan: err %v, %d bytes sent", err, site.bytesSent)
	}

	// Installing downloads the jar (checked against its sha1).
	if err := m.Commit(ctx, plan); err != nil {
		t.Fatal(err)
	}
	installed, _ := m.Scan()
	if len(installed) != 1 || installed[0].Name() != "Huge" || installed[0].SHA1 != v.File.SHA1 {
		t.Errorf("installed = %+v", installed)
	}
}

// jarWithNested is a jar whose fabric.mod.json lists one bundled jar (stored,
// the usual jar-in-jar layout).
func jarWithNested(t *testing.T, modJSON, nestedJSON string) []byte {
	inner := jarBytes(t, nestedJSON)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("fabric.mod.json")
	w.Write([]byte(modJSON))
	w, _ = zw.CreateHeader(&zip.FileHeader{Name: "META-INF/jars/bundled.jar", Method: zip.Store})
	w.Write(inner)
	zw.Close()
	return buf.Bytes()
}

// The Distant Horizons case on 1.21.11: it bundles Fabric API 0.139.4, the
// client has 0.141.6 and Sodium breaks Fabric API below 0.140. Fabric loads
// the client's copy, so the install must go through.
func TestInstallModBundlingOlderFabricAPI(t *testing.T) {
	site := newFakeSite(t)
	data := jarWithNested(t,
		`{"id":"distanthorizons","version":"3.3.3","name":"Distant Horizons","depends":{"minecraft":"1.21.11"},"jars":[{"file":"META-INF/jars/bundled.jar"}]}`,
		`{"id":"fabric-api","version":"0.139.4+1.21.11","name":"Fabric API"}`)
	v := site.add(t, "P-dh", "dh-1", "3.3.3", "release", "2026-10-01", `{}`)
	sum := sha1.Sum(data)
	site.jars[v.File.Name] = data
	v.File.SHA1, v.File.Size = hex.EncodeToString(sum[:]), int64(len(data))
	site.versions["P-dh"] = []Version{v}

	dir := t.TempDir()
	fapi := filepath.Join(dir, "fabric-api.jar")
	os.WriteFile(fapi, jarBytes(t, `{"id":"fabric-api","version":"0.141.6+1.21.11","name":"Fabric API"}`), 0o644)
	sodium := filepath.Join(dir, "sodium.jar")
	os.WriteFile(sodium, jarBytes(t, `{"id":"sodium","version":"0.8.14+mc1.21.11","name":"Sodium","breaks":{"fabric-api":"<0.140.0"}}`), 0o644)
	man := &client.Manifest{Minecraft: "1.21.11", Mods: []client.Mod{{ID: "fabric-api", Name: "Fabric API", Project: "P-fapi"}, {ID: "sodium", Name: "Sodium", Project: "P-sodium"}}}
	m := newManager(t, site)
	m.Minecraft, m.Layer = "1.21.11", &client.Layer{Manifest: man, Mods: man.Mods, Files: []string{fapi, sodium}}

	plan, err := m.PlanInstall(context.Background(), "P-dh", nil)
	if err != nil {
		t.Fatalf("Distant Horizons should install: %v", err)
	}
	if len(plan.Steps) != 1 || len(plan.Warnings) != 0 {
		t.Errorf("plan = %v, warnings %v", stepNames(plan), plan.Warnings)
	}
}
