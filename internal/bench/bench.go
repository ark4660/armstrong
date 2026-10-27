package bench

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"text/tabwriter"
)

// Scenes the client mod knows (client-mod/.../bench/Bench.java).
var Scenes = []string{"static", "fly", "spin", "entities", "menu"}

// Options are the game settings every run uses, so results only differ by
// what's being compared. extra lines (key:value) override them. dataVersion
// (game.DataVersion) keeps the game from upgrading the file, which would
// reset some settings (the graphics API, for one).
func Options(dataVersion, renderDistance int, extra []string) string {
	var lines []string
	if dataVersion > 0 {
		lines = append(lines, fmt.Sprintf("version:%d", dataVersion))
	}
	lines = append(lines,
		"enableVsync:false",
		"maxFps:260", // unlimited
		"inactivityFpsLimit:\"minimized\"",
		fmt.Sprintf("renderDistance:%d", renderDistance),
		"simulationDistance:8",
		"guiScale:2",
		"pauseOnLostFocus:false",
		"onboardAccessibility:false",
		"tutorialStep:none",
		"joinedFirstServer:true",
		"skipMultiplayerWarning:true",
		"soundCategory_master:0.0",
		"difficulty:0", // new worlds in 1.8.9 take it from here
		"narrator:0",
	)
	set := map[string]int{}
	for i, l := range lines {
		k, _, _ := strings.Cut(l, ":")
		set[k] = i
	}
	for _, l := range extra {
		k, _, _ := strings.Cut(l, ":")
		if i, ok := set[k]; ok {
			lines[i] = l
		} else {
			lines = append(lines, l)
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

// ModConfigs are config files written before each run: Dynamic FPS would
// drop an unfocused window to 1 fps and spoil the run.
var ModConfigs = map[string]string{
	"config/dynamic_fps.json": `{"enabled": false}` + "\n",
}

// Result is what the client mod writes, plus what the launcher adds.
type Result struct {
	Label     string `json:"label"`
	Scene     string `json:"scene"`
	Minecraft string `json:"minecraft"`
	Renderer  string `json:"renderer"`
	Backend   string `json:"backend"`
	Entities  int    `json:"entities"`
	LaunchMs  int64  `json:"launchToExitMs"`
	Frames    Frames `json:"frames"`
	GC        []GC   `json:"gc"`
	Memory    Memory `json:"memory"`
	Startup   struct {
		ReadyMs   int64 `json:"readyMs"`
		InWorldMs int64 `json:"inWorldMs"`
	} `json:"startup"`
	JVM struct {
		Version string   `json:"version"`
		Args    []string `json:"args"`
	} `json:"jvm"`
	Mods map[string]string `json:"mods"`
}

type Frames struct {
	Count      int     `json:"count"`
	AvgFps     float64 `json:"avgFps"`
	Low1Fps    float64 `json:"low1Fps"`
	Low01Fps   float64 `json:"low01Fps"`
	P50Ms      float64 `json:"p50Ms"`
	P99Ms      float64 `json:"p99Ms"`
	P999Ms     float64 `json:"p999Ms"`
	MaxMs      float64 `json:"maxMs"`
	StdDevMs   float64 `json:"stdDevMs"`
	Stutters   int     `json:"stutters"`
	Unfocused  int     `json:"unfocused"`
	AllocKBPer float64 `json:"allocKBPerFrame"`
}

type GC struct {
	Name   string `json:"name"`
	Count  int64  `json:"count"`
	TimeMs int64  `json:"timeMs"`
}

type Memory struct {
	HeapAfterLastGcMB int64 `json:"heapAfterLastGcMB"`
	HeapCommittedMB   int64 `json:"heapCommittedMB"`
}

// GCPauses sums collector time that pauses the game. ZGC also reports its
// concurrent cycles, which don't stop the game; those are left out.
func (r *Result) GCPauses() (count, ms int64) {
	for _, g := range r.GC {
		if strings.Contains(g.Name, "Cycles") {
			continue
		}
		count += g.Count
		ms += g.TimeMs
	}
	return
}

// Load reads result files; directories are searched for *.json.
func Load(paths []string) ([]*Result, error) {
	var out []*Result
	for _, p := range paths {
		files := []string{p}
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			files, _ = filepath.Glob(filepath.Join(p, "*.json"))
		}
		for _, f := range files {
			data, err := os.ReadFile(f)
			if err != nil {
				return nil, err
			}
			var r Result
			if err := json.Unmarshal(data, &r); err != nil {
				return nil, fmt.Errorf("%s: %w", f, err)
			}
			if r.Label == "" {
				r.Label = filepath.Base(filepath.Dir(f))
			}
			out = append(out, &r)
		}
	}
	return out, nil
}

// group is the runs of one label and scene, averaged.
type group struct {
	label, scene string
	runs         []*Result
}

func (g *group) mean(f func(*Result) float64) float64 {
	var s float64
	for _, r := range g.runs {
		s += f(r)
	}
	return s / float64(len(g.runs))
}

// Compare prints one table per scene: a row per label (in the order first
// seen), and its change against the first label.
func Compare(w io.Writer, results []*Result) {
	var labels []string
	groups := map[[2]string]*group{}
	scenes := map[string]bool{}
	for _, r := range results {
		key := [2]string{r.Label, r.Scene}
		g, ok := groups[key]
		if !ok {
			g = &group{label: r.Label, scene: r.Scene}
			groups[key] = g
			if !slices.Contains(labels, r.Label) {
				labels = append(labels, r.Label)
			}
		}
		g.runs = append(g.runs, r)
		scenes[r.Scene] = true
	}
	var sceneList []string
	for _, s := range Scenes {
		if scenes[s] {
			sceneList = append(sceneList, s)
		}
	}
	for s := range scenes {
		if !slices.Contains(sceneList, s) {
			sceneList = append(sceneList, s)
		}
	}
	sort.SliceStable(sceneList, func(a, b int) bool { return sceneIndex(sceneList[a]) < sceneIndex(sceneList[b]) })

	metrics := []struct {
		name   string
		f      func(*Result) float64
		higher bool // higher is better
	}{
		{"avg fps", func(r *Result) float64 { return r.Frames.AvgFps }, true},
		{"1% low", func(r *Result) float64 { return r.Frames.Low1Fps }, true},
		{"0.1% low", func(r *Result) float64 { return r.Frames.Low01Fps }, true},
		{"p99 ms", func(r *Result) float64 { return r.Frames.P99Ms }, false},
		{"max ms", func(r *Result) float64 { return r.Frames.MaxMs }, false},
		{"stutters", func(r *Result) float64 { return float64(r.Frames.Stutters) }, false},
		{"gc ms", func(r *Result) float64 { _, ms := r.GCPauses(); return float64(ms) }, false},
		{"KB/frame", func(r *Result) float64 { return r.Frames.AllocKBPer }, false},
		{"heap MB", func(r *Result) float64 { return float64(r.Memory.HeapAfterLastGcMB) }, false},
		{"to world s", func(r *Result) float64 { return float64(r.Startup.InWorldMs) / 1000 }, false},
	}
	for _, scene := range sceneList {
		fmt.Fprintf(w, "\n== %s\n", scene)
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', tabwriter.AlignRight)
		fmt.Fprint(tw, "label\truns\t")
		for _, m := range metrics {
			fmt.Fprintf(tw, "%s\t", m.name)
		}
		fmt.Fprintln(tw)
		var base *group
		for _, l := range labels {
			g := groups[[2]string{l, scene}]
			if g == nil {
				continue
			}
			fmt.Fprintf(tw, "%s\t%d\t", l, len(g.runs))
			for _, m := range metrics {
				v := g.mean(m.f)
				cell := trim(v)
				if base != nil {
					if b := base.mean(m.f); b != 0 {
						cell += fmt.Sprintf(" (%+.0f%%)", (v-b)/b*100)
					}
				}
				fmt.Fprintf(tw, "%s\t", cell)
			}
			fmt.Fprintln(tw)
			if base == nil {
				base = g
			}
		}
		tw.Flush()
		for _, l := range labels {
			if g := groups[[2]string{l, scene}]; g != nil {
				for _, r := range g.runs {
					if r.Frames.Count > 0 && r.Frames.Unfocused*20 > r.Frames.Count {
						fmt.Fprintf(w, "warning: %s/%s: the window was unfocused for %d of %d frames\n", l, scene, r.Frames.Unfocused, r.Frames.Count)
					}
				}
			}
		}
	}
}

func trim(v float64) string {
	switch {
	case v >= 100:
		return fmt.Sprintf("%.0f", v)
	case v >= 10:
		return fmt.Sprintf("%.1f", v)
	}
	return fmt.Sprintf("%.2f", v)
}

func sceneIndex(s string) int {
	for i, x := range Scenes {
		if x == s {
			return i
		}
	}
	return len(Scenes)
}
