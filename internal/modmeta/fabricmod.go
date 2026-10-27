// Package modmeta reads mod metadata from jars. Only Fabric's
// fabric.mod.json is supported.
package modmeta

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
)

// ErrNotAMod means the jar has no fabric.mod.json (a library, or a mod for
// another loader).
var ErrNotAMod = errors.New("no fabric.mod.json")

// FabricMod is the subset of fabric.mod.json the launcher uses.
type FabricMod struct {
	ID          string   `json:"id"`
	Version     string   `json:"version"`
	Name        string   `json:"name"`
	Environment string   `json:"environment"` // "*", "client", "server"; empty means "*"
	Provides    []string `json:"provides"`
	Depends     Deps     `json:"depends"`
	Recommends  Deps     `json:"recommends"`
	Breaks      Deps     `json:"breaks"`
	Conflicts   Deps     `json:"conflicts"`
	Jars        []struct {
		File string `json:"file"`
	} `json:"jars"`

	// Nested holds the mods bundled in Jars, recursively.
	Nested []*FabricMod `json:"-"`
}

// Deps maps a mod id to version predicates. In the file a value is a string
// or an array of strings (any of which may match).
type Deps map[string][]string

func (d *Deps) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	out := make(Deps, len(raw))
	for id, v := range raw {
		var one string
		if err := json.Unmarshal(v, &one); err == nil {
			out[id] = []string{one}
			continue
		}
		var many []string
		if err := json.Unmarshal(v, &many); err != nil {
			return fmt.Errorf("dependency %q: %w", id, err)
		}
		out[id] = many
	}
	*d = out
	return nil
}

// IDs returns the mod's id and everything it provides.
func (m *FabricMod) IDs() []string {
	return append([]string{m.ID}, m.Provides...)
}

// maxNested bounds how much of a nested jar is read into memory.
const maxNested = 64 << 20

// ReadJar reads fabric.mod.json from a jar file, including nested jars.
func ReadJar(file string) (*FabricMod, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	return Read(f, st.Size())
}

// Read reads fabric.mod.json from a jar held in r.
func Read(r io.ReaderAt, size int64) (*FabricMod, error) {
	return read(r, size, 0)
}

func read(r io.ReaderAt, size int64, depth int) (*FabricMod, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}
	var mod *FabricMod
	for _, f := range zr.File {
		if f.Name == "fabric.mod.json" {
			if mod, err = parse(f); err != nil {
				return nil, err
			}
			break
		}
	}
	if mod == nil {
		return nil, ErrNotAMod
	}
	if depth >= 4 {
		return mod, nil
	}
	for _, j := range mod.Jars {
		f := findFile(zr, j.File)
		if f == nil || f.UncompressedSize64 > maxNested {
			continue // a broken reference shouldn't hide the outer mod
		}
		// Nested jars are usually stored uncompressed: read them in place, so
		// a remote jar (download.RangeReader) only fetches their index too.
		var inner io.ReaderAt
		if off, err := f.DataOffset(); err == nil && f.Method == zip.Store {
			inner = io.NewSectionReader(r, off, int64(f.UncompressedSize64))
		} else {
			data, err := readAll(f)
			if err != nil {
				continue
			}
			inner = bytes.NewReader(data)
		}
		if nested, err := read(inner, int64(f.UncompressedSize64), depth+1); err == nil {
			mod.Nested = append(mod.Nested, nested)
		}
	}
	return mod, nil
}

func parse(f *zip.File) (*FabricMod, error) {
	data, err := readAll(f)
	if err != nil {
		return nil, err
	}
	// Some jars start the file with a UTF-8 byte order mark.
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	var mod FabricMod
	if err := json.Unmarshal(data, &mod); err != nil {
		return nil, fmt.Errorf("fabric.mod.json: %w", err)
	}
	if mod.ID == "" {
		return nil, errors.New("fabric.mod.json has no id")
	}
	return &mod, nil
}

func findFile(zr *zip.Reader, name string) *zip.File {
	name = path.Clean(name)
	for _, f := range zr.File {
		if path.Clean(f.Name) == name {
			return f
		}
	}
	return nil
}

func readAll(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, maxNested))
}
