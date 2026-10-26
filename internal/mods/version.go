// Package mods manages the player's own mods in an instance: what's
// installed, whether it fits together with the client layer, installing
// from a source (Modrinth for now) with dependencies, and removing.
package mods

import (
	"strconv"
	"strings"
)

// version is a Fabric-style semantic version: dot-separated numbers, an
// optional pre-release after "-", and build metadata after "+" (ignored when
// comparing). Non-numeric versions don't parse; see Match.
type version struct {
	core []int
	pre  []string // nil: a release
}

func parseVersion(s string) (version, bool) {
	if i := strings.IndexByte(s, '+'); i >= 0 {
		s = s[:i]
	}
	var v version
	if i := strings.IndexByte(s, '-'); i >= 0 {
		v.pre = strings.Split(s[i+1:], ".")
		s = s[:i]
	}
	if s == "" {
		return version{}, false
	}
	for _, part := range strings.Split(s, ".") {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return version{}, false
		}
		v.core = append(v.core, n)
	}
	return v, true
}

func compareCore(a, b []int) int {
	for i := 0; i < len(a) || i < len(b); i++ {
		var x, y int
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

// compare orders versions by semver rules: core first, then a release
// beats a pre-release, then pre-release identifiers (numbers < words).
func compare(a, b version) int {
	if c := compareCore(a.core, b.core); c != 0 {
		return c
	}
	switch {
	case a.pre == nil && b.pre == nil:
		return 0
	case a.pre == nil:
		return 1
	case b.pre == nil:
		return -1
	}
	for i := 0; i < len(a.pre) && i < len(b.pre); i++ {
		x, y := a.pre[i], b.pre[i]
		xn, xerr := strconv.Atoi(x)
		yn, yerr := strconv.Atoi(y)
		switch {
		case xerr == nil && yerr == nil:
			if xn != yn {
				if xn < yn {
					return -1
				}
				return 1
			}
		case xerr == nil:
			return -1
		case yerr == nil:
			return 1
		default:
			if c := strings.Compare(x, y); c != 0 {
				return c
			}
		}
	}
	switch {
	case len(a.pre) < len(b.pre):
		return -1
	case len(a.pre) > len(b.pre):
		return 1
	}
	return 0
}

// CompareVersions orders two version strings; ok is false if either isn't
// a semantic version.
func CompareVersions(a, b string) (int, bool) {
	va, okA := parseVersion(a)
	vb, okB := parseVersion(b)
	if !okA || !okB {
		return 0, false
	}
	return compare(va, vb), true
}

// Result of matching a version against fabric.mod.json predicates.
type Result int

const (
	No Result = iota
	Yes
	Unsure // the version or a predicate isn't semantic, so Fabric itself decides
)

// Match checks a version against a dependency's predicates: any of the
// strings may match (OR); within a string, space-separated terms must all
// match (AND). Terms: "*", "1.2.3" or "=1.2.3", ">=", "<=", ">", "<",
// "~1.2" (same minor), "^1.2" (same major), and x-ranges like "1.2.x".
func Match(v string, predicates []string) Result {
	if len(predicates) == 0 {
		return Yes
	}
	best := No
	for _, p := range predicates {
		switch matchAll(v, p) {
		case Yes:
			return Yes
		case Unsure:
			best = Unsure
		}
	}
	return best
}

func matchAll(v, predicate string) Result {
	terms := strings.Fields(predicate)
	if len(terms) == 0 {
		return Yes
	}
	out := Yes
	for _, t := range terms {
		switch matchTerm(v, t) {
		case No:
			return No
		case Unsure:
			out = Unsure
		}
	}
	return out
}

func matchTerm(v, term string) Result {
	if term == "*" {
		return Yes
	}
	op := ""
	for _, o := range []string{">=", "<=", ">", "<", "=", "~", "^"} {
		if strings.HasPrefix(term, o) {
			op, term = o, term[len(o):]
			break
		}
	}
	have, ok := parseVersion(v)

	// x-ranges: "1.2.x" means any 1.2.*.
	if prefix, wild := xRangePrefix(term); wild && (op == "" || op == "=") {
		if !ok {
			return Unsure
		}
		for i, n := range prefix {
			if i >= len(have.core) || have.core[i] != n {
				return No
			}
		}
		return Yes
	}

	want, wantOK := parseVersion(term)
	if !ok || !wantOK {
		if op == "" || op == "=" {
			if v == term {
				return Yes
			}
			if !ok && !wantOK {
				return No
			}
		}
		return Unsure
	}
	c := compare(have, want)
	switch op {
	case "", "=":
		return yes(c == 0)
	case ">=":
		return yes(c >= 0)
	case "<=":
		return yes(c <= 0)
	case ">":
		return yes(c > 0)
	case "<":
		return yes(c < 0)
	case "~", "^":
		if c < 0 {
			return No
		}
		// Upper bound: next minor for ~ (next major if only one part), next major for ^.
		idx := 0
		if op == "~" && len(want.core) >= 2 {
			idx = 1
		}
		upper := append([]int{}, want.core[:idx+1]...)
		upper[idx]++
		return yes(compareCore(have.core, upper) < 0)
	}
	return Unsure
}

func yes(b bool) Result {
	if b {
		return Yes
	}
	return No
}

// xRangePrefix parses "1.2.x" / "1.2.*" / "1.X" into its fixed prefix.
func xRangePrefix(s string) ([]int, bool) {
	parts := strings.Split(s, ".")
	last := parts[len(parts)-1]
	if len(parts) < 2 || (last != "x" && last != "X" && last != "*") {
		return nil, false
	}
	var prefix []int
	for _, p := range parts[:len(parts)-1] {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, false
		}
		prefix = append(prefix, n)
	}
	return prefix, true
}
