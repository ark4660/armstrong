package app

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// Settings are the launcher's global choices. Per-instance choices (version,
// loader, memory) live in each instance's instance.json.
type Settings struct {
	Instance string `json:"instance,omitempty"` // selected instance id

	// Player name from before accounts existed; auth.Open turns it into an
	// offline account and clears it.
	LegacyOfflineName string `json:"offlineName,omitempty"`

	// Pre-instance settings (0.1 had one global profile). Read only so
	// instance.Migrate can convert them; they're cleared before saving.
	LegacyMinecraft string `json:"minecraft,omitempty"`
	LegacyLoader    string `json:"loader,omitempty"`
	LegacyProfile   string `json:"profile,omitempty"`
	LegacyMemoryMB  int    `json:"memoryMB,omitempty"`
}

func (p Paths) settingsFile() string { return filepath.Join(p.Root, "settings.json") }

// LoadSettings reads settings.json; a missing file gives empty settings.
func (p Paths) LoadSettings() (Settings, error) {
	var s Settings
	data, err := os.ReadFile(p.settingsFile())
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return Settings{}, err
	}
	return s, nil
}

// SaveSettings writes settings.json atomically.
func (p Paths) SaveSettings(s Settings) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(p.Root, 0o755); err != nil {
		return err
	}
	tmp := p.settingsFile() + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p.settingsFile())
}
