package tui

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/ark4660/armstrong/internal/app"
	"github.com/ark4660/armstrong/internal/auth"
	"github.com/ark4660/armstrong/internal/client"
	"github.com/ark4660/armstrong/internal/download"
	"github.com/ark4660/armstrong/internal/game"
)

// Messages sent from the play pipeline back into Update.
type (
	stepMsg     struct{ label string }
	progressMsg struct{ p download.Progress }
	failedMsg   struct{ err error }
	clientMsg   struct {
		layer    *client.Layer
		disabled []client.Disabled
	}
	startedMsg struct {
		game          *game.Game
		loaderVersion string // empty for vanilla
	}
	logMsg    struct{ lines []string }
	readyMsg  struct{ after time.Duration }
	exitedMsg struct {
		err     error
		failure string   // failure marker seen in the log, if any
		crashes []string // crash reports written during this run
	}
)

// playRequest is a snapshot of what to launch, so the pipeline never shares
// state with the model.
type playRequest struct {
	spec     game.Spec
	gameDir  string
	memoryMB int
	preset   string   // JVM preset
	jvmArgs  []string // the player's own JVM arguments
	account  auth.Account
	accounts *auth.Accounts
	addons   []string
}

// play installs and launches the game in the background. Everything it learns
// goes back through send; it never touches the model.
func play(ctx context.Context, send func(tea.Msg), paths app.Paths, req playRequest) {
	var lastProgress time.Time
	onProgress := func(p download.Progress) {
		// Called once per file; thousands of assets would flood Update.
		if p.Done < p.Total && time.Since(lastProgress) < 50*time.Millisecond {
			return
		}
		lastProgress = time.Now()
		send(progressMsg{p})
	}
	dl := download.New(app.UserAgent())

	// Sign in first: Microsoft tokens are refreshed on every launch.
	send(stepMsg{"Signing in as " + req.account.Name})
	sess, err := req.accounts.Session(ctx, req.account)
	if err != nil {
		send(failedMsg{err})
		return
	}

	// Every Fabric instance runs with the client layer.
	spec := req.spec
	var layer *client.Layer
	if spec.Loader == game.LoaderFabric {
		m, err := client.Load(spec.Minecraft)
		if err != nil {
			send(failedMsg{err})
			return
		}
		if spec.LoaderVersion == "" {
			spec.LoaderVersion = m.FabricLoader
		}
		send(stepMsg{"Client " + m.Version})
		var disabled []client.Disabled
		layer, disabled, err = client.Prepare(ctx, dl, paths, spec.Minecraft, req.addons, req.gameDir, onProgress)
		if err != nil {
			send(failedMsg{err})
			return
		}
		send(clientMsg{layer: layer, disabled: disabled})
	}

	in := &game.Installer{
		Paths:      paths,
		DL:         dl,
		OnStep:     func(label string) { send(stepMsg{label}) },
		OnProgress: onProgress,
	}
	inst, err := in.Install(ctx, spec)
	if err != nil {
		send(failedMsg{err})
		return
	}

	send(stepMsg{"Starting"})
	gameDir := req.gameDir
	info := game.ReadClientInfo(paths, inst.Minecraft)
	if err := game.SeedOptions(gameDir, info); err != nil {
		send(failedMsg{err})
		return
	}
	opts := game.LaunchOptions{
		Paths:    paths,
		GameDir:  gameDir,
		Player:   game.PlayerFromSession(sess),
		MemoryMB: req.memoryMB,
		JVMArgs:  game.TunedJVMArgs(inst.Java, req.preset, req.memoryMB, req.jvmArgs),
		GameArgs: game.BackendArgs(info, layer != nil && layer.Has(client.ShadersModID)),
	}
	if layer != nil {
		opts.AddMods = layer.Files
	}
	cmd, err := game.Command(inst, opts)
	if err != nil {
		send(failedMsg{err})
		return
	}
	if ctx.Err() != nil {
		send(failedMsg{ctx.Err()})
		return
	}
	crashesBefore := game.CrashReports(gameDir)
	start := time.Now()
	g, err := game.Start(cmd)
	if err != nil {
		send(failedMsg{err})
		return
	}
	send(startedMsg{game: g, loaderVersion: inst.LoaderVersion})

	// Forward log lines in batches so a chatty game doesn't redraw per line.
	var (
		batch   []string
		failure string
		ready   bool
		tick    = time.NewTicker(100 * time.Millisecond)
	)
	defer tick.Stop()
	flush := func() {
		if len(batch) > 0 {
			send(logMsg{batch})
			batch = nil
		}
	}
	for lines := g.Lines; lines != nil; {
		select {
		case line, ok := <-lines:
			if !ok {
				lines = nil
				break
			}
			batch = append(batch, line)
			if failure == "" {
				failure = game.FailureMarker(line)
			}
			if !ready && strings.Contains(line, game.StartedMarker) {
				ready = true
				flush()
				send(readyMsg{time.Since(start)})
			}
		case <-tick.C:
			flush()
		}
	}
	flush()
	err = <-g.Done
	send(exitedMsg{err: err, failure: failure, crashes: game.NewCrashReports(gameDir, crashesBefore)})
}

// exitSummary describes how a game run ended, or "" for a clean exit.
func exitSummary(m exitedMsg, stoppedByUser bool) string {
	switch {
	case len(m.crashes) > 0:
		return "Game crashed: " + m.crashes[len(m.crashes)-1]
	case m.failure != "":
		return "Game failed: " + m.failure + " (see Console)"
	case m.err != nil && !stoppedByUser:
		var ee interface{ ExitCode() int }
		if errors.As(m.err, &ee) {
			return "Game exited with code " + strconv.Itoa(ee.ExitCode()) + " (see Console)"
		}
		return "Game exited: " + m.err.Error()
	}
	return ""
}
