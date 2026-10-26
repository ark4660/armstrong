package cosmetics

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ark4660/armstrong/internal/download"
	"github.com/ark4660/armstrong/internal/update"
)

func writePNG(t *testing.T, path string, w, h int) {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIndexRoundTrip(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(t.TempDir(), "cape.png")
	writePNG(t, src, 128, 64)
	seed, pub, err := update.NewKey()
	if err != nil {
		t.Fatal(err)
	}

	idx, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.AddCape(dir, "armstrong", "Armstrong", src); err != nil {
		t.Fatal(err)
	}
	idx.Badges["dev"] = Badge{Name: "Developer", Symbol: "◆", Color: "#5EF38C"}
	uuid := "069a79f444e94726a5befca90e38aaf5"
	if err := idx.Give(uuid, "Notch", "armstrong", "dev"); err != nil {
		t.Fatal(err)
	}
	if err := idx.Give(uuid, "Notch", "nope", ""); err == nil {
		t.Error("an unknown cape was accepted")
	}
	if err := idx.Save(dir, seed); err != nil {
		t.Fatal(err)
	}

	// The signature verifies with the matching public key.
	data, _ := os.ReadFile(filepath.Join(dir, IndexFile))
	sig, _ := os.ReadFile(filepath.Join(dir, SigFile))
	old := update.PublicKey
	update.PublicKey = pub
	defer func() { update.PublicKey = old }()
	if err := update.Verify(data, sig); err != nil {
		t.Errorf("signature: %v", err)
	}

	back, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if p := back.Players[uuid]; p.Cape != "armstrong" || p.Badge != "dev" {
		t.Errorf("player after reload: %+v", p)
	}

	// Taking both away removes the player.
	if err := back.Give(uuid, "Notch", "-", "-"); err != nil {
		t.Fatal(err)
	}
	if _, ok := back.Players[uuid]; ok {
		t.Error("player with nothing left is still listed")
	}

	// A cape file edited after it was added fails validation.
	writePNG(t, filepath.Join(dir, "capes", "armstrong.png"), 64, 32)
	if err := back.Validate(dir); err == nil {
		t.Error("a changed cape file passed validation")
	}
}

func TestCheckCapePNG(t *testing.T) {
	for _, tc := range []struct {
		w, h int
		ok   bool
	}{{64, 32, true}, {128, 64, true}, {1024, 512, true}, {64, 64, false}, {100, 50, false}, {2048, 1024, false}} {
		var buf bytes.Buffer
		png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, tc.w, tc.h)))
		if err := CheckCapePNG(buf.Bytes()); (err == nil) != tc.ok {
			t.Errorf("%dx%d: err=%v, want ok=%v", tc.w, tc.h, err, tc.ok)
		}
	}
	if CheckCapePNG([]byte("not a png")) == nil {
		t.Error("garbage accepted")
	}
}

func TestResolve(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/Notch" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"id":"069a79f444e94726a5befca90e38aaf5","name":"Notch"}`))
	}))
	defer srv.Close()
	old := profileURL
	profileURL = srv.URL + "/"
	defer func() { profileURL = old }()

	dl := download.New("test")
	dl.Retries = 0
	uuid, name, err := Resolve(context.Background(), dl, "Notch")
	if err != nil || uuid != "069a79f444e94726a5befca90e38aaf5" || name != "Notch" {
		t.Errorf("Resolve(Notch) = %s %s %v", uuid, name, err)
	}
	if uuid, _, err := Resolve(context.Background(), dl, "069A79F4-44E9-4726-A5BE-FCA90E38AAF5"); err != nil || uuid != "069a79f444e94726a5befca90e38aaf5" {
		t.Errorf("Resolve(uuid) = %s %v", uuid, err)
	}
	if _, _, err := Resolve(context.Background(), dl, "Nobody"); err == nil {
		t.Error("unknown name resolved")
	}
	if _, _, err := Resolve(context.Background(), dl, "bad name!"); err == nil {
		t.Error("invalid name accepted")
	}
}
