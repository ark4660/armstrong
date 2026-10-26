package game

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// StartedMarker is logged once the client has finished starting up.
const StartedMarker = "Sound engine started"

var failureMarkers = []string{
	"Mod loading failed",
	"Incompatible mods found",
	"Mod resolution failed",
	"---- Minecraft Crash Report ----",
	"Could not find or load main class",
}

// FailureMarker returns the failure a log line reports, or "".
func FailureMarker(line string) string {
	for _, m := range failureMarkers {
		if strings.Contains(line, m) {
			return m
		}
	}
	return ""
}

// CrashReports snapshots the crash-reports folder of a game directory, so
// NewCrashReports can tell which reports a run wrote.
func CrashReports(gameDir string) map[string]bool {
	out := map[string]bool{}
	entries, _ := os.ReadDir(filepath.Join(gameDir, "crash-reports"))
	for _, e := range entries {
		out[e.Name()] = true
	}
	return out
}

// NewCrashReports returns the paths of reports that aren't in before, sorted.
func NewCrashReports(gameDir string, before map[string]bool) []string {
	var out []string
	for name := range CrashReports(gameDir) {
		if !before[name] {
			out = append(out, filepath.Join(gameDir, "crash-reports", name))
		}
	}
	sort.Strings(out)
	return out
}
