// Package app holds the launcher's identity and on-disk layout.
package app

import (
	"os"
	"path/filepath"
	"runtime"
)

const (
	// Name is the technical id: binary, data folder, User-Agent, mod id.
	Name = "armstrong"
	// DisplayName is what players see.
	DisplayName = "Armstrong"
	// Repo hosts the source and the releases the launcher updates from.
	Repo = "ark4660/armstrong"
)

// Version is stamped by release builds:
// go build -ldflags "-X github.com/ark4660/armstrong/internal/app.Version=0.2.0".
// Local builds stay "dev", which never updates itself.
var Version = "0.1.0-dev"

// legacyName is the working name before the rename; its data folder is
// moved over once (see DefaultPaths).
const legacyName = "perfclient"

// UserAgent identifies the launcher to Mojang, Fabric and Modrinth.
// It must never contain personal data such as the user's email.
func UserAgent() string {
	return Name + "/" + Version + " (github.com/ark4660/" + Name + ")"
}

// Paths is the launcher's data directory. Libraries, assets, Java runtimes
// and version files are shared; every instance gets its own game directory.
type Paths struct {
	Root string
}

// DefaultPaths resolves the data directory: $ARMSTRONG_HOME if set,
// otherwise the platform's per-user data location. A data folder left by the
// old working name is renamed to the new one the first time.
func DefaultPaths() (Paths, error) {
	if dir := os.Getenv("ARMSTRONG_HOME"); dir != "" {
		return Paths{Root: dir}, nil
	}
	var base string
	switch runtime.GOOS {
	case "windows", "darwin":
		dir, err := os.UserConfigDir() // %AppData%, ~/Library/Application Support
		if err != nil {
			return Paths{}, err
		}
		base = dir
	default:
		if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
			base = dir
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return Paths{}, err
			}
			base = filepath.Join(home, ".local", "share")
		}
	}
	root := filepath.Join(base, Name)
	if _, err := os.Stat(root); os.IsNotExist(err) {
		if st, err := os.Stat(filepath.Join(base, legacyName)); err == nil && st.IsDir() {
			// Best effort: if the rename fails we just start with a fresh folder.
			os.Rename(filepath.Join(base, legacyName), root)
		}
	}
	return Paths{Root: root}, nil
}

func (p Paths) Versions() string  { return filepath.Join(p.Root, "versions") }
func (p Paths) Libraries() string { return filepath.Join(p.Root, "libraries") }
func (p Paths) Assets() string    { return filepath.Join(p.Root, "assets") }
func (p Paths) Runtimes() string  { return filepath.Join(p.Root, "runtimes") }
func (p Paths) Natives(versionID string) string {
	return filepath.Join(p.Root, "natives", versionID)
}
func (p Paths) Instances() string { return filepath.Join(p.Root, "instances") }
