package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ark4660/armstrong/internal/app"
	"github.com/ark4660/armstrong/internal/bench"
	"github.com/ark4660/armstrong/internal/client"
	"github.com/ark4660/armstrong/internal/download"
	"github.com/ark4660/armstrong/internal/game"
	"github.com/ark4660/armstrong/internal/instance"
)

// Warm-up per scene when -warmup isn't given: long enough for the chunks in
// view to load, except fly, which is about loading them.
var benchWarmup = map[string]float64{"static": 15, "spin": 10, "entities": 15, "menu": 10, "fly": 3}

type listFlag []string

func (l *listFlag) String() string     { return strings.Join(*l, " ") }
func (l *listFlag) Set(v string) error { *l = append(*l, v); return nil }

// cmdBench runs benchmark scenes in a scratch game directory and writes one
// result per scene and run; `bench compare` prints them side by side.
func cmdBench(ctx context.Context, paths app.Paths, args []string) error {
	if len(args) > 0 && args[0] == "compare" {
		if len(args) < 2 {
			return errors.New("usage: bench compare RESULT_DIR_OR_FILE...")
		}
		results, err := bench.Load(args[1:])
		if err != nil {
			return err
		}
		bench.Compare(os.Stdout, results)
		return nil
	}

	fs := flag.NewFlagSet("bench", flag.ExitOnError)
	mc := fs.String("mc", instance.DefaultMinecraft, "Minecraft version")
	scenes := fs.String("scenes", "static,fly,entities", "scenes to run: "+strings.Join(bench.Scenes, ","))
	runs := fs.Int("runs", 1, "runs of every scene (interleaved)")
	label := fs.String("label", "", "name of this configuration (default: date and time)")
	out := fs.String("o", "", "results folder (default: <data>/bench/results/<label>)")
	warmup := fs.Float64("warmup", 0, "seconds in the world before measuring (default: per scene)")
	duration := fs.Float64("duration", 20, "seconds measured")
	addons := fs.String("addons", "", "client add-ons to enable, comma-separated")
	without := fs.String("without", "", "client mods to leave out, comma-separated mod ids")
	only := fs.String("only", "", "keep only these client mods (plus Fabric API and Armstrong)")
	noModules := fs.Bool("no-modules", false, "switch off every Armstrong module (the mod only measures)")
	preset := fs.String("preset", game.PresetDefault, "JVM preset: "+strings.Join(game.Presets, ", "))
	mem := fs.Int("mem", instance.DefaultMemoryMB, "max heap in MB")
	var jvm, env, opts listFlag
	fs.Var(&jvm, "jvm", "extra JVM argument (repeatable, or space-separated)")
	fs.Var(&env, "env", "environment variable KEY=value (repeatable)")
	fs.Var(&opts, "opt", "options.txt line key:value (repeatable)")
	wrapper := fs.String("wrapper", "", "run java through this command, like gamemoderun")
	rd := fs.Int("rd", 12, "render distance")
	size := fs.String("size", "1600x900", "window size")
	timeout := fs.Duration("timeout", 4*time.Minute, "give up on a run after this long")
	fs.Parse(args)

	sceneList := strings.Split(*scenes, ",")
	for _, s := range sceneList {
		if !slices.Contains(bench.Scenes, s) {
			return fmt.Errorf("unknown scene %q (have %s)", s, strings.Join(bench.Scenes, ", "))
		}
	}
	width, height, ok := strings.Cut(*size, "x")
	if !ok {
		return fmt.Errorf("bad -size %q, want WIDTHxHEIGHT", *size)
	}
	if *label == "" {
		*label = time.Now().Format("2006-01-02-150405")
	}
	if *out == "" {
		*out = filepath.Join(paths.Root, "bench", "results", *label)
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		return err
	}
	gameDir := filepath.Join(paths.Root, "bench", "game")

	m, err := client.Load(*mc)
	if err != nil {
		return err
	}
	spec := game.Spec{Minecraft: *mc, Loader: game.LoaderFabric, LoaderVersion: m.FabricLoader}
	pp := &progressPrinter{label: "Client " + m.Version}
	layer, _, err := client.Prepare(ctx, download.New(app.UserAgent()), paths, *mc, splitList(*addons), gameDir, pp.progress)
	pp.finish()
	if err != nil {
		return err
	}
	if !layer.Has(client.ModID) {
		return fmt.Errorf("the %s mod isn't in the Minecraft %s client layer: build it with `client build -mc %s`", client.ModID, *mc, *mc)
	}
	drop := splitList(*without)
	if keep := splitList(*only); len(keep) > 0 {
		keep = append(keep, client.ModID, "fabric-api", "legacy-fabric-api")
		for _, mod := range layer.Mods {
			if !slices.Contains(keep, mod.ID) {
				drop = append(drop, mod.ID)
			}
		}
	}
	layer = layer.Without(drop)

	inst, err := install(ctx, paths, spec)
	if err != nil {
		return err
	}
	var jvmArgs []string
	jvmArgs = append(jvmArgs, game.JVMFlags(*preset, game.JavaMajor(inst.Java), *mem)...)
	for _, j := range jvm {
		jvmArgs = append(jvmArgs, strings.Fields(j)...)
	}
	player := game.OfflinePlayer("Bench")

	var mods []string
	for _, mod := range layer.Mods {
		mods = append(mods, mod.ID)
	}
	fmt.Printf("Benchmark %q: Minecraft %s, %d client mods, preset %s, results in %s\n", *label, *mc, len(mods), *preset, *out)
	fmt.Println("The game window opens for each run: keep it focused and leave the mouse alone.")

	var files []string
	for run := 1; run <= *runs; run++ {
		for _, scene := range sceneList {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			w := benchWarmup[scene]
			if *warmup > 0 {
				w = *warmup
			}
			file := filepath.Join(*out, fmt.Sprintf("%s-%d.json", scene, run))
			props := []string{
				"-Darmstrong.bench.scene=" + scene,
				"-Darmstrong.bench.warmup=" + strconv.FormatFloat(w, 'f', -1, 64),
				"-Darmstrong.bench.duration=" + strconv.FormatFloat(*duration, 'f', -1, 64),
				"-Darmstrong.bench.out=" + file,
				"-Darmstrong.bench.world=" + bench.WorldName,
			}
			if *noModules {
				props = append(props, "-Darmstrong.bench.noModules=true")
			}
			gameArgs := []string{"--width", width, "--height", height}
			if err := prepareBenchDir(gameDir, game.ReadClientInfo(paths, *mc).DataVersion, *rd, opts); err != nil {
				return err
			}
			opt := game.LaunchOptions{
				Paths:    paths,
				GameDir:  gameDir,
				Player:   player,
				MemoryMB: *mem,
				JVMArgs:  append(slices.Clone(jvmArgs), props...),
				AddMods:  layer.Files,
				GameArgs: gameArgs,
				Env:      env,
				Wrapper:  strings.Fields(*wrapper),
			}
			r, err := runBench(ctx, inst, opt, file, *timeout)
			if err != nil {
				return fmt.Errorf("%s run %d: %w", scene, run, err)
			}
			r.Label = *label
			if err := rewriteResult(file, *label, r.LaunchMs); err != nil {
				return err
			}
			files = append(files, file)
			pauses, pauseMs := r.GCPauses()
			fmt.Printf("  %-8s run %d: %6.0f fps avg, %6.0f 1%% low, %5.0f 0.1%% low, p99 %.2f ms, %d stutters, %d GC pauses (%d ms), %.1f KB/frame\n",
				scene, run, r.Frames.AvgFps, r.Frames.Low1Fps, r.Frames.Low01Fps, r.Frames.P99Ms, r.Frames.Stutters, pauses, pauseMs, r.Frames.AllocKBPer)
			if r.Frames.Unfocused*20 > r.Frames.Count {
				fmt.Printf("  warning: the window was unfocused for %d of %d frames; this run isn't comparable\n", r.Frames.Unfocused, r.Frames.Count)
			}
		}
	}
	results, err := bench.Load(files)
	if err != nil {
		return err
	}
	bench.Compare(os.Stdout, results)
	fmt.Printf("\nCompare with another configuration: %s bench compare %s <other results folder>\n", app.Name, *out)
	return nil
}

// prepareBenchDir resets the scratch game dir: no old world, fixed options
// and mod configs.
func prepareBenchDir(gameDir string, dataVersion, rd int, extra []string) error {
	if err := bench.ResetWorld(gameDir); err != nil {
		return err
	}
	if err := os.MkdirAll(gameDir, 0o755); err != nil { // the first run on a machine
		return err
	}
	if err := os.WriteFile(filepath.Join(gameDir, "options.txt"), []byte(bench.Options(dataVersion, rd, extra)), 0o644); err != nil {
		return err
	}
	for name, content := range bench.ModConfigs {
		path := filepath.Join(gameDir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
	}
	// The module settings start from defaults every run.
	os.Remove(filepath.Join(gameDir, "config", "armstrong.json"))
	return nil
}

// runBench runs one scene and returns its result once the game has quit.
func runBench(ctx context.Context, inst *game.Installation, opt game.LaunchOptions, file string, timeout time.Duration) (*bench.Result, error) {
	os.Remove(file)
	cmd, err := game.Command(inst, opt)
	if err != nil {
		return nil, err
	}
	crashesBefore := game.CrashReports(opt.GameDir)
	start := time.Now()
	g, err := game.Start(cmd)
	if err != nil {
		return nil, err
	}
	deadline := time.After(timeout)
	var tail []string
	failure := ""
	for lines := g.Lines; lines != nil; {
		select {
		case line, ok := <-lines:
			if !ok {
				lines = nil
				break
			}
			tail = append(tail, line)
			if len(tail) > 40 {
				tail = tail[1:]
			}
			if f := game.FailureMarker(line); f != "" && failure == "" {
				failure = f
			}
		case <-deadline:
			g.Stop(10 * time.Second)
			deadline = nil
			failure = "timed out"
		case <-ctx.Done():
			g.Stop(10 * time.Second)
			return nil, ctx.Err()
		}
	}
	<-g.Done
	elapsed := time.Since(start)
	data, err := os.ReadFile(file)
	if err != nil {
		if failure == "" {
			failure = "the game quit without writing a result"
		}
		if crashes := game.NewCrashReports(opt.GameDir, crashesBefore); len(crashes) > 0 {
			failure += fmt.Sprintf(" (crash reports: %v)", crashes)
		}
		fmt.Fprintln(os.Stderr, "---- last log lines ----")
		for _, l := range tail {
			fmt.Fprintln(os.Stderr, l)
		}
		return nil, errors.New(failure)
	}
	var r bench.Result
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	r.LaunchMs = elapsed.Milliseconds()
	return &r, nil
}

// rewriteResult adds the launcher's fields to the mod's result file.
func rewriteResult(file, label string, launchMs int64) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	raw["label"] = label
	raw["launchToExitMs"] = launchMs
	raw["launcher"] = app.Name + " " + app.Version
	data, err = json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(file, append(data, '\n'), 0o644)
}

func splitList(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}
