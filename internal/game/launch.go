package game

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ark4660/armstrong/internal/app"
	"github.com/ark4660/armstrong/internal/auth"
)

// Player is who the game runs as.
type Player struct {
	Name        string
	UUID        string // without dashes
	AccessToken string
	UserType    string // "msa" or "legacy" (offline)
	XUID        string
}

// PlayerFromSession is the player an auth session launches as.
func PlayerFromSession(s *auth.Session) Player {
	return Player{Name: s.Name, UUID: s.UUID, AccessToken: s.AccessToken, UserType: s.UserType, XUID: s.XUID}
}

// OfflinePlayer is a player without a Microsoft account. The game can't
// reach Mojang's services this way (multiplayer needs offline-mode servers).
func OfflinePlayer(name string) Player {
	return Player{Name: name, UUID: auth.OfflineUUID(name), AccessToken: "0", UserType: "legacy", XUID: "0"}
}

type LaunchOptions struct {
	Paths    app.Paths
	GameDir  string
	Player   Player
	MemoryMB int
	JVMArgs  []string // extra user JVM arguments
	AddMods  []string // mod jars loaded from outside the mods folder (the client layer)
	Server   string   // host[:port] to join on start; empty for none
}

// Game is a running game process. Lines carries its combined stdout/stderr
// and is closed when the output ends; Done then receives the exit result.
type Game struct {
	Cmd   *exec.Cmd
	Lines <-chan string
	Done  <-chan error
}

// Command builds the java command line for an installation.
func Command(inst *Installation, opt LaunchOptions) (*exec.Cmd, error) {
	v := inst.Version
	natives := opt.Paths.Natives(v.ID)
	for _, dir := range []string{opt.GameDir, natives} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	env := CurrentEnv()
	if opt.Server != "" {
		env.Features["is_quick_play_multiplayer"] = true
	}
	vars := map[string]string{
		"auth_player_name":     opt.Player.Name,
		"auth_uuid":            opt.Player.UUID,
		"auth_access_token":    opt.Player.AccessToken,
		"auth_xuid":            opt.Player.XUID,
		"user_type":            opt.Player.UserType,
		"clientid":             "",
		"version_name":         v.ID,
		"version_type":         v.Type,
		"game_directory":       opt.GameDir,
		"assets_root":          opt.Paths.Assets(),
		"assets_index_name":    v.AssetIndex.ID,
		"natives_directory":    natives,
		"library_directory":    opt.Paths.Libraries(),
		"classpath":            strings.Join(inst.Classpath, string(os.PathListSeparator)),
		"classpath_separator":  string(os.PathListSeparator),
		"launcher_name":        app.Name,
		"launcher_version":     app.Version,
		"quickPlayMultiplayer": opt.Server,
	}

	var args []string
	if opt.MemoryMB > 0 {
		args = append(args, "-Xmx"+strconv.Itoa(opt.MemoryMB)+"M")
	}
	args = append(args, opt.JVMArgs...)
	if len(opt.AddMods) > 0 {
		args = append(args, "-Dfabric.addMods="+strings.Join(opt.AddMods, string(os.PathListSeparator)))
	}
	args = append(args, expand(v.Arguments.JVM, env, vars)...)
	args = append(args, v.MainClass)
	args = append(args, expand(v.Arguments.Game, env, vars)...)

	cmd := exec.Command(inst.Java, args...)
	cmd.Dir = opt.GameDir
	return cmd, nil
}

var placeholder = regexp.MustCompile(`\$\{([A-Za-z0-9_]+)\}`)

// expand filters arguments by rules and substitutes ${...} variables.
// Unknown variables are left as they are.
func expand(list []Argument, env Env, vars map[string]string) []string {
	var out []string
	for _, a := range list {
		if !a.Rules.Allowed(env) {
			continue
		}
		for _, s := range a.Values {
			out = append(out, placeholder.ReplaceAllStringFunc(s, func(m string) string {
				if v, ok := vars[m[2:len(m)-1]]; ok {
					return v
				}
				return m
			}))
		}
	}
	return out
}

// Start runs the command and streams its output line by line.
func Start(cmd *exec.Cmd) (*Game, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	cmd.Stdout, cmd.Stderr = w, w
	if err := cmd.Start(); err != nil {
		r.Close()
		w.Close()
		return nil, fmt.Errorf("start %s: %w", filepath.Base(cmd.Path), err)
	}
	w.Close() // the child has its own copy

	lines := make(chan string, 256)
	done := make(chan error, 1)
	go func() {
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			lines <- sc.Text()
		}
		r.Close()
		close(lines)
		done <- cmd.Wait()
	}()
	return &Game{Cmd: cmd, Lines: lines, Done: done}, nil
}

// Stop asks the game to quit (SIGTERM, so it shuts down cleanly and flushes
// its logs) and kills it if it's still running after grace. Windows has no
// SIGTERM for console-less processes, so there it kills right away. The
// caller still drains Lines and waits on Done.
func (g *Game) Stop(grace time.Duration) {
	if runtime.GOOS == "windows" || g.Cmd.Process.Signal(syscall.SIGTERM) != nil {
		g.Cmd.Process.Kill()
		return
	}
	// Kill after the process has exited is a no-op (os.ErrProcessDone).
	time.AfterFunc(grace, func() { g.Cmd.Process.Kill() })
}
