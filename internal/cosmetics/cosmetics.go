// Package cosmetics manages the cosmetics list: which players have which
// cape and name-tag badge. The list (cosmetics/cosmetics.json in the
// repository, with its capes) is signed with the cosmetics key and served
// from GitHub; every Armstrong client downloads it, checks the signature and
// draws the cosmetics on the players it lists. Only Armstrong players see them.
package cosmetics

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ark4660/armstrong/internal/update"
)

const FormatVersion = 1

// Index is cosmetics.json.
type Index struct {
	FormatVersion int               `json:"formatVersion"`
	Capes         map[string]Cape   `json:"capes"`
	Badges        map[string]Badge  `json:"badges"`
	Players       map[string]Player `json:"players"` // by UUID without dashes
}

// Cape is a cape texture in capes/<id>.png.
type Cape struct {
	Name   string `json:"name"`
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
}

// Badge is a symbol shown before a player's name, in name tags and the tab list.
type Badge struct {
	Name   string `json:"name"`
	Symbol string `json:"symbol"`
	Color  string `json:"color"` // #RRGGBB
}

// Player is what one player has. Name is only there to make the file readable.
type Player struct {
	Name  string `json:"name"`
	Cape  string `json:"cape,omitempty"`
	Badge string `json:"badge,omitempty"`
}

// Files in the cosmetics folder.
const (
	IndexFile = "cosmetics.json"
	SigFile   = "cosmetics.json.sig"
)

var (
	idPattern    = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)
	colorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	uuidPattern  = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

// Load reads the index from dir; a missing file is an empty index.
func Load(dir string) (*Index, error) {
	idx := &Index{FormatVersion: FormatVersion, Capes: map[string]Cape{}, Badges: map[string]Badge{}, Players: map[string]Player{}}
	data, err := os.ReadFile(filepath.Join(dir, IndexFile))
	if os.IsNotExist(err) {
		return idx, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, idx); err != nil {
		return nil, fmt.Errorf("%s: %w", IndexFile, err)
	}
	if idx.FormatVersion != FormatVersion {
		return nil, fmt.Errorf("%s: format %d, want %d", IndexFile, idx.FormatVersion, FormatVersion)
	}
	if idx.Capes == nil {
		idx.Capes = map[string]Cape{}
	}
	if idx.Badges == nil {
		idx.Badges = map[string]Badge{}
	}
	if idx.Players == nil {
		idx.Players = map[string]Player{}
	}
	return idx, nil
}

// Validate checks that every reference resolves and every cape file matches its hash.
func (idx *Index) Validate(dir string) error {
	for id, c := range idx.Capes {
		if !idPattern.MatchString(id) {
			return fmt.Errorf("cape id %q: use a-z, 0-9, - and _", id)
		}
		if c.File != "capes/"+id+".png" {
			return fmt.Errorf("cape %s: file must be capes/%s.png", id, id)
		}
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(c.File)))
		if err != nil {
			return fmt.Errorf("cape %s: %w", id, err)
		}
		if sum := sha256.Sum256(data); hex.EncodeToString(sum[:]) != c.SHA256 {
			return fmt.Errorf("cape %s: %s changed since it was added (run add-cape again)", id, c.File)
		}
	}
	for id, b := range idx.Badges {
		if !idPattern.MatchString(id) {
			return fmt.Errorf("badge id %q: use a-z, 0-9, - and _", id)
		}
		if !colorPattern.MatchString(b.Color) || b.Symbol == "" {
			return fmt.Errorf("badge %s: needs a symbol and a #RRGGBB colour", id)
		}
	}
	for uuid, p := range idx.Players {
		if !uuidPattern.MatchString(uuid) {
			return fmt.Errorf("player %q: not a UUID without dashes", uuid)
		}
		if _, ok := idx.Capes[p.Cape]; p.Cape != "" && !ok {
			return fmt.Errorf("player %s: no cape %q", p.Name, p.Cape)
		}
		if _, ok := idx.Badges[p.Badge]; p.Badge != "" && !ok {
			return fmt.Errorf("player %s: no badge %q", p.Name, p.Badge)
		}
	}
	return nil
}

// Save validates the index, writes it and signs it with the cosmetics key.
func (idx *Index) Save(dir, keySeed string) error {
	if err := idx.Validate(dir); err != nil {
		return err
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	sig, err := update.Sign(keySeed, data)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, IndexFile), data, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, SigFile), sig, 0o644)
}

// AddCape copies a PNG into capes/<id>.png and records it. Capes use the
// vanilla layout: 2:1, 64x32 or a multiple of it (HD), at most 1024 wide.
func (idx *Index) AddCape(dir, id, name, pngPath string) error {
	if !idPattern.MatchString(id) {
		return fmt.Errorf("cape id %q: use a-z, 0-9, - and _", id)
	}
	data, err := os.ReadFile(pngPath)
	if err != nil {
		return err
	}
	if err := CheckCapePNG(data); err != nil {
		return err
	}
	file := "capes/" + id + ".png"
	if err := os.MkdirAll(filepath.Join(dir, "capes"), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(file)), data, 0o644); err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	idx.Capes[id] = Cape{Name: name, File: file, SHA256: hex.EncodeToString(sum[:])}
	return nil
}

// CheckCapePNG checks that data is a PNG with the cape layout's proportions.
func CheckCapePNG(data []byte) error {
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("not a PNG: %w", err)
	}
	if _, err := png.Decode(bytes.NewReader(data)); err != nil {
		return fmt.Errorf("broken PNG: %w", err)
	}
	if cfg.Width != 2*cfg.Height || cfg.Width%64 != 0 || cfg.Width > 1024 {
		return fmt.Errorf("cape is %dx%d: it must be 64x32 or a multiple (128x64, 256x128...), at most 1024 wide", cfg.Width, cfg.Height)
	}
	return nil
}

// Give sets a player's cape and/or badge ("" leaves one unchanged, "-" removes it).
func (idx *Index) Give(uuid, name, cape, badge string) error {
	p := idx.Players[uuid]
	p.Name = name
	switch cape {
	case "":
	case "-":
		p.Cape = ""
	default:
		if _, ok := idx.Capes[cape]; !ok {
			return fmt.Errorf("no cape %q (have: %s)", cape, strings.Join(keys(idx.Capes), ", "))
		}
		p.Cape = cape
	}
	switch badge {
	case "":
	case "-":
		p.Badge = ""
	default:
		if _, ok := idx.Badges[badge]; !ok {
			return fmt.Errorf("no badge %q (have: %s)", badge, strings.Join(keys(idx.Badges), ", "))
		}
		p.Badge = badge
	}
	if p.Cape == "" && p.Badge == "" {
		delete(idx.Players, uuid)
	} else {
		idx.Players[uuid] = p
	}
	return nil
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
