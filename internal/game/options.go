package game

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ark4660/armstrong/internal/app"
)

// ClientInfo is what the launcher needs to know about a Minecraft version
// that only its client jar says.
type ClientInfo struct {
	// DataVersion ("world_version" in version.json) is recorded in
	// options.txt. 0 before 1.14 or when it can't be read.
	DataVersion int
	// Backends: the game can render with Vulkan as well as OpenGL (26.x),
	// chosen with the preferredGraphicsBackend option or --graphicsBackend.
	Backends bool
}

// ReadClientInfo reads a version's ClientInfo from its installed client jar.
// A missing or unreadable jar gives the zero value.
func ReadClientInfo(paths app.Paths, minecraft string) ClientInfo {
	var info ClientInfo
	zr, err := zip.OpenReader(filepath.Join(paths.Versions(), minecraft, minecraft+".jar"))
	if err != nil {
		return info
	}
	defer zr.Close()
	if f, err := zr.Open("version.json"); err == nil {
		var v struct {
			WorldVersion int `json:"world_version"`
		}
		if json.NewDecoder(f).Decode(&v) == nil {
			info.DataVersion = v.WorldVersion
		}
		f.Close()
	}
	_, err = fs.Stat(zr, "net/minecraft/client/PreferredGraphicsApi.class")
	info.Backends = err == nil
	return info
}

// firstOptions are written to a new game directory's options.txt before its
// first launch: uncapped frame rate without vsync (Dynamic FPS slows the game
// down when it's in the background), a render distance the client stack
// handles well, and no first-run tutorial or onboarding screens. Players
// change them in game like any other setting; existing files are never touched.
var firstOptions = []string{
	"enableVsync:false",
	"maxFps:260",
	"renderDistance:12",
	"simulationDistance:8",
	"onboardAccessibility:false",
	"tutorialStep:none",
	"skipMultiplayerWarning:true",
	"joinedFirstServer:true",
}

// SeedOptions writes the first-launch options.txt if the game directory has
// none. The data version marks the file as current (without it the game
// treats it as old and resets some settings while upgrading it). Versions
// with a Vulkan backend start on Vulkan: on 26.3 it measured about twice the
// frame rate of OpenGL with the client's mods (`armstrong bench`, RTX 4070
// Super), and the game falls back to OpenGL by itself where Vulkan can't start.
func SeedOptions(gameDir string, info ClientInfo) error {
	path := filepath.Join(gameDir, "options.txt")
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(gameDir, 0o755); err != nil {
		return err
	}
	var lines []string
	if info.DataVersion > 0 {
		lines = append(lines, "version:"+strconv.Itoa(info.DataVersion))
	}
	lines = append(lines, firstOptions...)
	if info.Backends {
		lines = append(lines, `preferredGraphicsBackend:"vulkan"`)
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

// BackendArgs are the game arguments that pick the graphics backend for a
// launch. Shader packs (Iris) only work with OpenGL, so with shaders on the
// game is held to OpenGL whatever the player's setting says; otherwise the
// setting decides.
func BackendArgs(info ClientInfo, shaders bool) []string {
	if info.Backends && shaders {
		return []string{"--graphicsBackend", "opengl"}
	}
	return nil
}
