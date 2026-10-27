package game

import (
	"bufio"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// JVM presets: the garbage collector and flags the launcher adds before the
// player's own JVM arguments. Chosen with `armstrong bench` on 26.3 (RTX 4070
// Super, 2026-10-27, three runs per scene):
//
//   - default: G1 with the heap committed up front (-Xms = -Xmx). Against
//     Java's defaults: 1% lows +6 to +14%, GC pause time -37 to -58%, fewer
//     stutters, same average fps. Tuned G1 flags (pause target, new-size and
//     region size) and -XX:+AlwaysPreTouch measured no better and are left out.
//   - zgc: no GC pauses at all and better 0.1% lows, but nearly twice the heap
//     and a worse p99 with many entities, so it's a choice, not the default.
//   - none: only -Xmx, like before presets existed.
const (
	PresetDefault = "default"
	PresetZGC     = "zgc"
	PresetNone    = "none"
)

var Presets = []string{PresetDefault, PresetZGC, PresetNone}

// JVMFlags returns the flags for a preset on a Java major version with a
// maximum heap of memoryMB. Unknown presets fall back to the default.
func JVMFlags(preset string, javaMajor, memoryMB int) []string {
	var flags []string
	switch preset {
	case PresetNone:
		return nil
	case PresetZGC:
		flags = append(flags, "-XX:+UseZGC")
		if javaMajor >= 21 && javaMajor < 23 {
			flags = append(flags, "-XX:+ZGenerational") // the default from 23 on
		}
	default:
		// G1 is Java's default only on machines it considers servers; say so.
		flags = append(flags, "-XX:+UseG1GC")
		if memoryMB > 0 {
			flags = append(flags, "-Xms"+strconv.Itoa(memoryMB)+"M")
		}
	}
	if javaMajor >= 25 {
		// 8-byte object headers instead of 12: 5-10% less heap and allocation.
		// A product flag (not experimental) from Java 25 (JEP 519).
		flags = append(flags, "-XX:+UseCompactObjectHeaders")
	}
	return flags
}

// TunedJVMArgs are the preset's flags for a Java runtime followed by the
// player's own arguments, which win where they overlap (the JVM takes the
// last of a repeated flag). A player who picks a garbage collector gets no
// preset collector flags: two collectors stop the JVM from starting.
func TunedJVMArgs(java, preset string, memoryMB int, user []string) []string {
	flags := JVMFlags(preset, JavaMajor(java), memoryMB)
	if slices.ContainsFunc(user, isGCChoice) {
		flags = slices.DeleteFunc(flags, func(f string) bool {
			return isGCChoice(f) || f == "-XX:+ZGenerational"
		})
	}
	return append(flags, user...)
}

func isGCChoice(arg string) bool {
	return strings.HasPrefix(arg, "-XX:+Use") && strings.HasSuffix(arg, "GC")
}

// JavaMajor reads a Java runtime's major version from the release file next
// to its bin folder (JAVA_VERSION="21.0.7"). 0 when it can't tell.
func JavaMajor(java string) int {
	root := filepath.Dir(filepath.Dir(java))
	f, err := os.Open(filepath.Join(root, "release"))
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		v, ok := strings.CutPrefix(sc.Text(), "JAVA_VERSION=")
		if !ok {
			continue
		}
		v = strings.Trim(v, `"`)
		v = strings.TrimPrefix(v, "1.") // 1.8.0_x
		major, _, _ := strings.Cut(v, ".")
		n, _ := strconv.Atoi(major)
		return n
	}
	return 0
}
