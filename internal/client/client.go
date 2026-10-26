// Package client manages the client layer: the mods every instance runs
// with (our client mod and the core performance stack). They live outside
// the player's mods folder, in a launcher-managed folder per Minecraft
// version, and are loaded with Fabric's -Dfabric.addMods. Players can't
// remove them by accident, and client updates never touch the player's files.
package client

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/ark4660/armstrong/internal/app"
	"github.com/ark4660/armstrong/internal/download"
	"github.com/ark4660/armstrong/internal/modmeta"
)

// Manifest describes one client release for one Minecraft version.
type Manifest struct {
	FormatVersion int    `json:"formatVersion"`
	Client        string `json:"client"`
	Version       string `json:"version"` // client release
	Minecraft     string `json:"minecraft"`
	FabricLoader  string `json:"fabricLoader"` // pinned so every player runs the same build
	Mods          []Mod  `json:"mods"`
}

// Mod is one jar of the client layer.
type Mod struct {
	ID       string   `json:"id"`                 // fabric.mod.json id
	Provides []string `json:"provides,omitempty"` // other ids it answers to
	Name     string   `json:"name"`
	Version  string   `json:"version"`
	Optional bool     `json:"optional,omitempty"` // an add-on the player can enable
	Requires []string `json:"requires,omitempty"` // ids of client mods it needs

	Project   string `json:"project,omitempty"`   // Modrinth project id
	Slug      string `json:"slug,omitempty"`      // Modrinth project slug
	VersionID string `json:"versionId,omitempty"` // Modrinth version id
	File      string `json:"file"`
	URL       string `json:"url"`
	SHA1      string `json:"sha1"`
	SHA512    string `json:"sha512,omitempty"`
	Size      int64  `json:"size"`
}

const FormatVersion = 1

// ModID is Armstrong's own client mod.
const ModID = "armstrong"

// IsReadyLine reports whether a log line is the client mod's
// "Armstrong <version> ready with N modules" (the boot test waits for it).
func IsReadyLine(line string) bool {
	return strings.Contains(line, "]: Armstrong ") && strings.Contains(line, " ready with ")
}

//go:embed manifests
var bundled embed.FS

// Bundled returns the manifests compiled into the launcher, by Minecraft version.
func Bundled() (map[string]*Manifest, error) {
	out := map[string]*Manifest{}
	entries, err := fs.ReadDir(bundled, "manifests")
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := fs.ReadFile(bundled, "manifests/"+e.Name())
		if err != nil {
			return nil, err
		}
		var m Manifest
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("bundled manifest %s: %w", e.Name(), err)
		}
		if m.FormatVersion != FormatVersion {
			return nil, fmt.Errorf("bundled manifest %s: format %d, want %d", e.Name(), m.FormatVersion, FormatVersion)
		}
		out[m.Minecraft] = &m
	}
	return out, nil
}

// dataRoot is the launcher's data folder, where updated manifests are kept
// (set by Init; empty means built-in manifests only).
var dataRoot string

// Init tells the package where downloaded manifests live.
func Init(paths app.Paths) {
	dataRoot = paths.Root
}

// CachedPath is where the latest verified manifest for mc is kept.
func CachedPath(root, mc string) string {
	return filepath.Join(root, "client", mc, "manifest.json")
}

// SaveCached stores a verified manifest (from a signed release) for later launches.
func SaveCached(root string, data []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m.FormatVersion != FormatVersion || m.Minecraft == "" {
		return nil, fmt.Errorf("manifest for %q has format %d, want %d", m.Minecraft, m.FormatVersion, FormatVersion)
	}
	path := CachedPath(root, m.Minecraft)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return nil, err
	}
	return &m, os.Rename(tmp, path)
}

func cached(mc string) *Manifest {
	if dataRoot == "" {
		return nil
	}
	data, err := os.ReadFile(CachedPath(dataRoot, mc))
	if err != nil {
		return nil
	}
	var m Manifest
	if json.Unmarshal(data, &m) != nil || m.FormatVersion != FormatVersion || m.Minecraft != mc {
		return nil
	}
	return &m
}

// SupportedVersions lists the Minecraft versions with a client release, newest first.
func SupportedVersions() []string {
	all, err := Bundled()
	if err != nil {
		return nil
	}
	if dataRoot != "" {
		dirs, _ := os.ReadDir(filepath.Join(dataRoot, "client"))
		for _, d := range dirs {
			if m := cached(d.Name()); m != nil {
				all[m.Minecraft] = m
			}
		}
	}
	out := make([]string, 0, len(all))
	for mc := range all {
		out = append(out, mc)
	}
	sort.Slice(out, func(a, b int) bool { return versionLess(out[b], out[a]) })
	return out
}

// Load returns the client manifest for a Minecraft version: the latest one
// downloaded from a signed release, or the built-in one if that's newer or
// nothing was downloaded yet.
func Load(mc string) (*Manifest, error) {
	all, err := Bundled()
	if err != nil {
		return nil, err
	}
	b, hasBundled := all[mc]
	c := cached(mc)
	switch {
	case c != nil && (!hasBundled || !releaseLess(c.Version, b.Version)):
		return c, nil
	case hasBundled:
		return b, nil
	}
	return nil, fmt.Errorf("the client isn't available for Minecraft %s yet", mc)
}

// releaseLess orders client versions like "0.1.0-dev" < "0.1.0" < "0.2.0".
func releaseLess(a, b string) bool {
	na, preA, _ := strings.Cut(a, "-")
	nb, preB, _ := strings.Cut(b, "-")
	if na != nb {
		return versionLess(na, nb)
	}
	return preA != "" && preB == ""
}

// Enabled returns the core mods plus the named add-ons (by mod id, Modrinth
// slug or project id) and everything they require.
func (m *Manifest) Enabled(addons []string) []Mod {
	byID := map[string]Mod{}
	for _, mod := range m.Mods {
		byID[mod.ID] = mod
	}
	want := map[string]bool{}
	var add func(id string)
	add = func(id string) {
		if want[id] {
			return
		}
		mod, ok := byID[id]
		if !ok {
			return
		}
		want[id] = true
		for _, r := range mod.Requires {
			add(r)
		}
	}
	for _, mod := range m.Mods {
		if !mod.Optional || slices.Contains(addons, mod.ID) || slices.Contains(addons, mod.Slug) || (mod.Project != "" && slices.Contains(addons, mod.Project)) {
			add(mod.ID)
		}
	}
	var out []Mod
	for _, mod := range m.Mods { // keep manifest order
		if want[mod.ID] {
			out = append(out, mod)
		}
	}
	return out
}

// Layer is a synced client layer, ready to pass to Fabric.
type Layer struct {
	Manifest *Manifest
	Mods     []Mod
	Files    []string // jar paths for -Dfabric.addMods
}

// IDs returns every mod id the layer provides.
func (l *Layer) IDs() map[string]string {
	out := map[string]string{}
	for _, m := range l.Mods {
		out[m.ID] = m.Name
		for _, p := range m.Provides {
			out[p] = m.Name
		}
	}
	return out
}

// Dir is where a Minecraft version's client jars live.
func Dir(paths app.Paths, mc string) string {
	return filepath.Join(paths.Root, "client", mc, "mods")
}

// DevDir holds locally built client jars (`armstrong client build`). They
// join the layer as they are, until the client mod ships in releases.
func DevDir(paths app.Paths, mc string) string {
	return filepath.Join(paths.Root, "client", mc, "dev")
}

// Sync downloads the enabled mods into the managed folder and removes jars
// no manifest entry names any more.
func Sync(ctx context.Context, dl *download.Client, paths app.Paths, m *Manifest, addons []string, onProgress func(download.Progress)) (*Layer, error) {
	dir := Dir(paths, m.Minecraft)
	layer := &Layer{Manifest: m, Mods: m.Enabled(addons)}
	var items []download.Item
	for _, mod := range layer.Mods {
		if err := safeFileName(mod.File); err != nil {
			return nil, err
		}
		path := filepath.Join(dir, mod.File)
		items = append(items, download.Item{URL: mod.URL, Path: path, SHA1: mod.SHA1, Size: mod.Size})
		layer.Files = append(layer.Files, path)
	}
	if err := dl.Fetch(ctx, items, onProgress); err != nil {
		return nil, fmt.Errorf("client mods: %w", err)
	}
	if err := addDevJars(layer, DevDir(paths, m.Minecraft)); err != nil {
		return nil, err
	}

	keep := map[string]bool{}
	for _, mod := range m.Mods { // all entries, so other instances' add-ons survive
		keep[mod.File] = true
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !e.IsDir() && !keep[e.Name()] {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
	return layer, nil
}

// safeFileName rejects names that would escape the managed folder.
func safeFileName(name string) error {
	if name == "" || name != filepath.Base(name) || strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
		return fmt.Errorf("unsafe file name %q in client manifest", name)
	}
	return nil
}

// Disabled is a player's mod that was switched off because the client
// already provides it. Fabric doesn't refuse duplicates: it silently loads
// the newest version (checked with Loader 0.20.1), so a player's copy would
// replace the client's tested build and break pins like Iris → Sodium.
type Disabled struct {
	File      string // file name, now with .disabled appended
	ModID     string
	ClientMod string // the client's copy
}

// DisableDuplicates renames the player's jars that provide a mod id the
// client layer already provides, so the client's build is the one that loads.
func DisableDuplicates(modsDir string, layer *Layer) ([]Disabled, error) {
	ids := layer.IDs()
	entries, err := os.ReadDir(modsDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Disabled
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".jar") {
			continue
		}
		path := filepath.Join(modsDir, e.Name())
		mod, err := modmeta.ReadJar(path)
		if err != nil {
			continue // not a Fabric mod: Fabric will complain itself if it matters
		}
		for _, id := range mod.IDs() {
			name, dup := ids[id]
			if !dup {
				continue
			}
			target := path + ".disabled"
			for n := 2; fileExists(target); n++ {
				target = fmt.Sprintf("%s.%d.disabled", path, n)
			}
			if err := os.Rename(path, target); err != nil {
				return out, err
			}
			out = append(out, Disabled{File: filepath.Base(target), ModID: id, ClientMod: name})
			break
		}
	}
	return out, nil
}

func fileExists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// versionLess orders Minecraft versions like "1.21.11" < "26.1" < "26.3".
func versionLess(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			fmt.Sscanf(pa[i], "%d", &x)
		}
		if i < len(pb) {
			fmt.Sscanf(pb[i], "%d", &y)
		}
		if x != y {
			return x < y
		}
	}
	return false
}

// Prepare loads the manifest for mc, syncs the layer with the given add-ons
// and disables the player's duplicates in gameDir/mods.
func Prepare(ctx context.Context, dl *download.Client, paths app.Paths, mc string, addons []string, gameDir string, onProgress func(download.Progress)) (*Layer, []Disabled, error) {
	m, err := Load(mc)
	if err != nil {
		return nil, nil, err
	}
	layer, err := Sync(ctx, dl, paths, m, addons, onProgress)
	if err != nil {
		return nil, nil, err
	}
	disabled, err := DisableDuplicates(filepath.Join(gameDir, "mods"), layer)
	if err != nil {
		return nil, nil, fmt.Errorf("checking your mods: %w", err)
	}
	return layer, disabled, nil
}

// addDevJars adds locally built jars to the layer. A dev jar replaces a
// manifest entry with the same mod id, so a local build wins over a release.
func addDevJars(layer *Layer, dir string) error {
	jars, err := filepath.Glob(filepath.Join(dir, "*.jar"))
	if err != nil {
		return err
	}
	for _, path := range jars {
		meta, err := modmeta.ReadJar(path)
		if err != nil {
			return fmt.Errorf("dev jar %s: %w", filepath.Base(path), err)
		}
		for i, mod := range layer.Mods {
			if mod.ID == meta.ID {
				layer.Mods = slices.Delete(layer.Mods, i, i+1)
				layer.Files = slices.Delete(layer.Files, i, i+1)
				break
			}
		}
		layer.Mods = append(layer.Mods, Mod{ID: meta.ID, Provides: meta.Provides, Name: meta.Name, Version: meta.Version, File: filepath.Base(path)})
		layer.Files = append(layer.Files, path)
	}
	return nil
}

// Has reports whether the layer includes a mod id.
func (l *Layer) Has(id string) bool {
	return slices.ContainsFunc(l.Mods, func(m Mod) bool { return m.ID == id })
}
