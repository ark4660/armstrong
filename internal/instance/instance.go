// Package instance manages instances: separate game setups, each with its
// own Minecraft version, loader, settings and game directory (mods, config,
// saves). Libraries, assets and Java runtimes stay shared in the data dir.
//
// Layout: instances/<id>/instance.json and instances/<id>/minecraft/.
// The id is the directory name and never changes; Name is what users see.
package instance

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ark4660/armstrong/internal/game"
)

const (
	LoaderFabric  = game.LoaderFabric
	LoaderVanilla = game.LoaderVanilla
)

// Loaders lists the loaders new instances can use. Every instance runs the
// client, which is a Fabric mod; vanilla remains a valid game.Spec for tests.
var Loaders = []string{LoaderFabric}

const DefaultMemoryMB = 4096

type Instance struct {
	ID            string    `json:"-"`
	Name          string    `json:"name"`
	Minecraft     string    `json:"minecraft"`
	Loader        string    `json:"loader"`
	LoaderVersion string    `json:"loaderVersion,omitempty"` // empty: latest stable
	MemoryMB      int       `json:"memoryMB"`
	JVMArgs       []string  `json:"jvmArgs,omitempty"`
	JVMPreset     string    `json:"jvmPreset,omitempty"` // empty: game.PresetDefault
	Addons        []string  `json:"addons,omitempty"`    // optional client mods the player enabled
	Created       time.Time `json:"created"`
	LastPlayed    time.Time `json:"lastPlayed,omitzero"`

	dir string
}

func (i *Instance) Dir() string { return i.dir }

// Preset is the instance's JVM preset (see game.Presets).
func (i *Instance) Preset() string {
	if i.JVMPreset == "" {
		return game.PresetDefault
	}
	return i.JVMPreset
}
func (i *Instance) GameDir() string { return filepath.Join(i.dir, "minecraft") }

// Spec is what has to be installed to play this instance.
func (i *Instance) Spec() game.Spec {
	return game.Spec{Minecraft: i.Minecraft, Loader: i.Loader, LoaderVersion: i.LoaderVersion}
}

// Store is the instances directory.
type Store struct {
	Root string
}

func (s Store) file(id string) string { return filepath.Join(s.Root, id, "instance.json") }

// List returns all instances, most recently played first, then by name.
// Directories without a readable instance.json are skipped.
func (s Store) List() ([]*Instance, error) {
	entries, err := os.ReadDir(s.Root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []*Instance
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if inst, err := s.Get(e.Name()); err == nil {
			out = append(out, inst)
		}
	}
	sort.Slice(out, func(a, b int) bool {
		if !out[a].LastPlayed.Equal(out[b].LastPlayed) {
			return out[a].LastPlayed.After(out[b].LastPlayed)
		}
		return strings.ToLower(out[a].Name) < strings.ToLower(out[b].Name)
	})
	return out, nil
}

func (s Store) Get(id string) (*Instance, error) {
	if !validID(id) {
		return nil, fmt.Errorf("invalid instance id %q", id)
	}
	data, err := os.ReadFile(s.file(id))
	if err != nil {
		return nil, err
	}
	var inst Instance
	if err := json.Unmarshal(data, &inst); err != nil {
		return nil, fmt.Errorf("instance %s: %w", id, err)
	}
	inst.ID, inst.dir = id, filepath.Join(s.Root, id)
	if inst.Name == "" {
		inst.Name = id
	}
	if inst.MemoryMB <= 0 {
		inst.MemoryMB = DefaultMemoryMB
	}
	return &inst, nil
}

// Save writes instance.json atomically.
func (s Store) Save(inst *Instance) error {
	if inst.ID == "" || inst.dir == "" {
		return errors.New("instance has no id; use Create")
	}
	data, err := json.MarshalIndent(inst, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(inst.GameDir(), 0o755); err != nil {
		return err
	}
	tmp := s.file(inst.ID) + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.file(inst.ID))
}

// Create makes a new instance from a template (Name, Minecraft, Loader, ...),
// choosing a free id derived from the name.
func (s Store) Create(tmpl Instance) (*Instance, error) {
	if err := ValidateName(tmpl.Name); err != nil {
		return nil, err
	}
	if tmpl.Minecraft == "" {
		return nil, errors.New("no Minecraft version chosen")
	}
	if !knownLoader(tmpl.Loader) {
		return nil, fmt.Errorf("unsupported loader %q", tmpl.Loader)
	}
	inst := tmpl
	inst.Name = strings.TrimSpace(inst.Name)
	if inst.MemoryMB <= 0 {
		inst.MemoryMB = DefaultMemoryMB
	}
	if inst.Created.IsZero() {
		inst.Created = time.Now()
	}
	id, err := s.reserve(slug(inst.Name))
	if err != nil {
		return nil, err
	}
	inst.ID, inst.dir = id, filepath.Join(s.Root, id)
	if err := s.Save(&inst); err != nil {
		os.RemoveAll(inst.dir)
		return nil, err
	}
	return &inst, nil
}

// reserve creates the directory for the first free id based on base.
func (s Store) reserve(base string) (string, error) {
	if err := os.MkdirAll(s.Root, 0o755); err != nil {
		return "", err
	}
	for n := 1; n < 1000; n++ {
		id := base
		if n > 1 {
			id = base + "-" + strconv.Itoa(n)
		}
		err := os.Mkdir(filepath.Join(s.Root, id), 0o755)
		if err == nil {
			return id, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return "", err
		}
	}
	return "", fmt.Errorf("no free instance id for %q", base)
}

func (s Store) Rename(inst *Instance, name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	old := inst.Name
	inst.Name = strings.TrimSpace(name)
	if err := s.Save(inst); err != nil {
		inst.Name = old
		return err
	}
	return nil
}

// Duplicate copies an instance, game directory included, under a new name.
func (s Store) Duplicate(src *Instance, name string) (*Instance, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}
	id, err := s.reserve(slug(name))
	if err != nil {
		return nil, err
	}
	dst := *src
	dst.ID, dst.dir = id, filepath.Join(s.Root, id)
	dst.Name = strings.TrimSpace(name)
	dst.Created, dst.LastPlayed = time.Now(), time.Time{}
	if err := copyTree(src.GameDir(), dst.GameDir()); err != nil {
		os.RemoveAll(dst.dir)
		return nil, err
	}
	if err := s.Save(&dst); err != nil {
		os.RemoveAll(dst.dir)
		return nil, err
	}
	return &dst, nil
}

// Delete removes an instance and everything in it, saves included.
func (s Store) Delete(inst *Instance) error {
	if !validID(inst.ID) {
		return fmt.Errorf("invalid instance id %q", inst.ID)
	}
	return os.RemoveAll(filepath.Join(s.Root, inst.ID))
}

func ValidateName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("instance name can't be empty")
	}
	if len([]rune(name)) > 40 {
		return errors.New("instance name is too long (40 characters max)")
	}
	return nil
}

func knownLoader(l string) bool {
	for _, k := range Loaders {
		if l == k {
			return true
		}
	}
	return false
}

// slug turns a display name into a directory-safe id: "Fabric 26.3" → "fabric-26.3".
func slug(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.' || r == '_':
			b.WriteRune(r)
			dash = false
		case !dash && b.Len() > 0:
			b.WriteByte('-')
			dash = true
		}
	}
	s := strings.Trim(b.String(), "-.")
	if s == "" {
		return "instance"
	}
	return s
}

// validID guards against ids that would escape the instances directory.
func validID(id string) bool {
	return id != "" && id != "." && id != ".." && !strings.ContainsAny(id, `/\`) && filepath.Base(id) == id
}

// copyTree copies a directory recursively, keeping symlinks as links.
// A missing source is an empty tree.
func copyTree(src, dst string) error {
	if _, err := os.Stat(src); errors.Is(err, fs.ErrNotExist) {
		return os.MkdirAll(dst, 0o755)
	}
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o755)
		case d.Type()&fs.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		case d.Type().IsRegular():
			return copyFile(path, target)
		}
		return nil // sockets, devices: skip
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	st, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, st.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
