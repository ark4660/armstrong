package update

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ark4660/armstrong/internal/app"
	"github.com/ark4660/armstrong/internal/client"
	"github.com/ark4660/armstrong/internal/download"
)

// fakeRelease builds a release folder like the workflow does, signed with a
// fresh key that the test makes the launcher trust.
func fakeRelease(t *testing.T, version string) (dist string, srv *httptest.Server) {
	t.Helper()
	dist = t.TempDir()
	srv = httptest.NewServer(http.FileServer(http.Dir(dist)))
	t.Cleanup(srv.Close)
	t.Setenv("ARMSTRONG_UPDATE_URL", srv.URL)

	seed, pub, err := NewKey()
	if err != nil {
		t.Fatal(err)
	}
	old := PublicKey
	PublicKey = pub
	t.Cleanup(func() { PublicKey = old })

	manifests := t.TempDir()
	bundled, _ := os.ReadFile("../client/manifests/26.3.json")
	os.WriteFile(filepath.Join(manifests, "26.3.json"), bundled, 0o644)
	jars := t.TempDir()
	os.WriteFile(filepath.Join(jars, "mod-26.3.jar"), []byte("client mod bytes"), 0o644)
	if err := StampManifests(manifests, dist, version, filepath.Join(jars, "mod-{mc}.jar")); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"linux-amd64", "darwin-arm64", "windows-amd64"} {
		os.WriteFile(filepath.Join(dist, LauncherName(p)), []byte("launcher "+version+" "+p), 0o755)
	}
	if _, err := WriteIndex(dist, version, seed); err != nil {
		t.Fatal(err)
	}
	return dist, srv
}

func TestReleaseRoundTrip(t *testing.T) {
	dist, _ := fakeRelease(t, "9.9.9")
	ctx := context.Background()
	dl := download.New("test")

	r, err := Fetch(ctx, dl)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Newer() || len(r.Launcher) != 3 || r.Client["26.3"].Name != "client-26.3.json" || r.Mods["26.3"].Name != "armstrong-9.9.9-mc26.3.jar" {
		t.Fatalf("release = %+v", r)
	}

	// The client manifest lands in the cache with the mod jar first.
	root := t.TempDir()
	if err := RefreshClient(ctx, dl, root, r); err != nil {
		t.Fatal(err)
	}
	client.Init(app.Paths{Root: root})
	m, err := client.Load("26.3")
	if err != nil || m.Version != "9.9.9" || m.Mods[0].ID != client.ModID || !strings.HasSuffix(m.Mods[0].URL, "/armstrong-9.9.9-mc26.3.jar") {
		t.Fatalf("cached manifest = %+v, %v", m, err)
	}

	// Replacing a launcher binary.
	exe := filepath.Join(t.TempDir(), "armstrong")
	os.WriteFile(exe, []byte("old"), 0o755)
	data, err := FetchAsset(ctx, dl, r.Launcher["linux-amd64"], 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if err := replaceExecutable(exe, data); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "launcher 9.9.9 linux-amd64" {
		t.Errorf("replaced binary = %q", got)
	}

	// A tampered file fails its checksum.
	os.WriteFile(filepath.Join(dist, "armstrong-linux-amd64"), []byte("evil"), 0o755)
	if _, err := FetchAsset(ctx, dl, r.Launcher["linux-amd64"], 1<<20); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Errorf("tampered binary: %v", err)
	}

	// A tampered index fails its signature.
	idx, _ := os.ReadFile(filepath.Join(dist, IndexName))
	os.WriteFile(filepath.Join(dist, IndexName), []byte(strings.Replace(string(idx), "9.9.9", "9.9.8", 1)), 0o644)
	if _, err := Fetch(ctx, dl); !errors.Is(err, ErrBadSignature) {
		t.Errorf("tampered index: %v", err)
	}
}

func TestWrongKeyRejected(t *testing.T) {
	fakeRelease(t, "1.0.0")
	_, otherPub, _ := NewKey()
	PublicKey = otherPub // a launcher that trusts a different key
	if _, err := Fetch(context.Background(), download.New("test")); !errors.Is(err, ErrBadSignature) {
		t.Errorf("release signed by another key: %v", err)
	}
}

func TestCompareVersions(t *testing.T) {
	ordered := []string{"0.1.0-dev", "0.1.0", "0.1.1", "0.2.0", "1.0.0", "1.10.0"}
	for i := 1; i < len(ordered); i++ {
		if compareVersions(ordered[i-1], ordered[i]) >= 0 || compareVersions(ordered[i], ordered[i-1]) <= 0 {
			t.Errorf("%s should be < %s", ordered[i-1], ordered[i])
		}
	}
	if compareVersions("v0.2.0", "0.2.0") != 0 {
		t.Error("a leading v must be ignored")
	}
}
