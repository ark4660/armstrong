// Package update keeps the launcher and the client current. Each GitHub
// release carries release.json (what's in the release, with SHA-256 sums)
// and release.json.sig, an Ed25519 signature made with the release key.
// The launcher only trusts an index whose signature matches the public key
// compiled into it, and only installs files whose sums match the index.
package update

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/ark4660/armstrong/internal/app"
	"github.com/ark4660/armstrong/internal/client"
	"github.com/ark4660/armstrong/internal/download"
)

const FormatVersion = 1

// IndexName and SigName are the release assets the launcher reads.
const (
	IndexName = "release.json"
	SigName   = "release.json.sig"
)

// Release is release.json.
type Release struct {
	Format   int              `json:"format"`
	Version  string           `json:"version"`  // without the leading v
	Launcher map[string]Asset `json:"launcher"` // by platform, e.g. "linux-amd64"
	Client   map[string]Asset `json:"client"`   // client manifests by Minecraft version
	Mod      Asset            `json:"mod"`      // the client mod jar (also listed in the manifests)
}

type Asset struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

// BaseURL is where the latest release's assets are served. It can be
// pointed elsewhere (a test server, a staging repo) with ARMSTRONG_UPDATE_URL.
func BaseURL() string {
	if u := os.Getenv("ARMSTRONG_UPDATE_URL"); u != "" {
		return strings.TrimSuffix(u, "/") + "/"
	}
	return "https://github.com/" + app.Repo + "/releases/latest/download/"
}

// AssetURL is where a release's file is downloaded from.
func AssetURL(version, name string) string {
	if u := os.Getenv("ARMSTRONG_UPDATE_URL"); u != "" {
		return strings.TrimSuffix(u, "/") + "/" + name
	}
	return "https://github.com/" + app.Repo + "/releases/download/v" + version + "/" + name
}

// Disabled reports whether this build should leave updates alone: local
// development builds, or ARMSTRONG_NO_UPDATE=1.
func Disabled() bool {
	return strings.HasSuffix(app.Version, "-dev") || os.Getenv("ARMSTRONG_NO_UPDATE") == "1"
}

// Platform is this machine's key in Release.Launcher.
func Platform() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}

var ErrBadSignature = errors.New("release index signature doesn't match the release key")

// Fetch downloads and verifies the latest release index.
func Fetch(ctx context.Context, dl *download.Client) (*Release, error) {
	data, err := dl.Get(ctx, BaseURL()+IndexName, 1<<20)
	if err != nil {
		return nil, err
	}
	sig, err := dl.Get(ctx, BaseURL()+SigName, 4<<10)
	if err != nil {
		return nil, err
	}
	if err := Verify(data, sig); err != nil {
		return nil, err
	}
	var r Release
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("release index: %w", err)
	}
	if r.Format != FormatVersion {
		return nil, fmt.Errorf("release index format %d isn't supported by this launcher; download the latest from github.com/%s", r.Format, app.Repo)
	}
	return &r, nil
}

// Verify checks a signature (base64, as in release.json.sig) over data.
func Verify(data, sigFile []byte) error {
	pub, err := base64.StdEncoding.DecodeString(PublicKey)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return errors.New("this launcher has no valid release key")
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sigFile)))
	if err != nil || !ed25519.Verify(ed25519.PublicKey(pub), data, sig) {
		return ErrBadSignature
	}
	return nil
}

// Sign signs data with a private key seed (base64) and returns the
// release.json.sig contents.
func Sign(seedB64 string, data []byte) ([]byte, error) {
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(seedB64))
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, errors.New("signing key must be a base64 Ed25519 seed (see `armstrong release keygen`)")
	}
	sig := ed25519.Sign(ed25519.NewKeyFromSeed(seed), data)
	return []byte(base64.StdEncoding.EncodeToString(sig) + "\n"), nil
}

// NewKey makes a signing key: the private seed and the public key, both base64.
func NewKey() (seedB64, publicB64 string, err error) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(priv.Seed()), base64.StdEncoding.EncodeToString(pub), nil
}

// Newer reports whether the release is newer than this launcher.
func (r *Release) Newer() bool {
	return compareVersions(r.Version, app.Version) > 0
}

// FetchAsset downloads an asset and checks its SHA-256.
func FetchAsset(ctx context.Context, dl *download.Client, a Asset, maxBytes int64) ([]byte, error) {
	data, err := dl.Get(ctx, a.URL, maxBytes)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != a.SHA256 {
		return nil, fmt.Errorf("%s: checksum mismatch (got %s, want %s)", a.Name, got, a.SHA256)
	}
	return data, nil
}

// ApplyLauncher replaces the running launcher with the release's build for
// this platform. The new version runs from the next start.
func ApplyLauncher(ctx context.Context, dl *download.Client, r *Release) error {
	a, ok := r.Launcher[Platform()]
	if !ok {
		return fmt.Errorf("release %s has no launcher for %s", r.Version, Platform())
	}
	data, err := FetchAsset(ctx, dl, a, 200<<20)
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return err
	}
	return replaceExecutable(exe, data)
}

// replaceExecutable swaps exe for data. Unix renames over the running file
// (the running process keeps the old one open); Windows can't, but can rename
// the running file aside, so the old one becomes exe.old until next start.
func replaceExecutable(exe string, data []byte) error {
	tmp := exe + ".new"
	if err := os.WriteFile(tmp, data, 0o755); err != nil {
		return fmt.Errorf("can't write next to %s (is it in a protected folder?): %w", exe, err)
	}
	if runtime.GOOS == "windows" {
		old := exe + ".old"
		os.Remove(old)
		if err := os.Rename(exe, old); err != nil {
			os.Remove(tmp)
			return err
		}
	}
	if err := os.Rename(tmp, exe); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// CleanupOld removes what a Windows update left behind; harmless elsewhere.
func CleanupOld() {
	if exe, err := os.Executable(); err == nil {
		os.Remove(exe + ".old")
	}
}

// compareVersions orders "1.2.3" and "1.2.3-dev"-style versions; a
// pre-release sorts before its release.
func compareVersions(a, b string) int {
	na, preA, _ := strings.Cut(strings.TrimPrefix(a, "v"), "-")
	nb, preB, _ := strings.Cut(strings.TrimPrefix(b, "v"), "-")
	pa, pb := strings.Split(na, "."), strings.Split(nb, ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	switch {
	case preA == "" && preB != "":
		return 1
	case preA != "" && preB == "":
		return -1
	}
	return strings.Compare(preA, preB)
}

// RefreshClient stores the release's client manifests (checked against the
// signed index) so the next Play uses the latest client.
func RefreshClient(ctx context.Context, dl *download.Client, root string, r *Release) error {
	for _, a := range r.Client {
		data, err := FetchAsset(ctx, dl, a, 4<<20)
		if err != nil {
			return err
		}
		if _, err := client.SaveCached(root, data); err != nil {
			return fmt.Errorf("%s: %w", a.Name, err)
		}
	}
	return nil
}
