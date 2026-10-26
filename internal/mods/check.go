package mods

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/ark4660/armstrong/internal/modmeta"
)

// Unit is one mod's metadata: a jar's own fabric.mod.json, or one of its
// nested jars. Checks run on units because nested mods count too.
type Unit struct {
	ID        string       `json:"id"`
	Version   string       `json:"version"`
	Name      string       `json:"name,omitempty"`
	Provides  []string     `json:"provides,omitempty"`
	Depends   modmeta.Deps `json:"depends,omitempty"`
	Breaks    modmeta.Deps `json:"breaks,omitempty"`
	Conflicts modmeta.Deps `json:"conflicts,omitempty"`
}

func (u Unit) title() string {
	if u.Name != "" {
		return u.Name
	}
	return u.ID
}

// UnitsOf flattens a jar's metadata: the jar's mod first, then nested mods.
func UnitsOf(m *modmeta.FabricMod) []Unit {
	if m == nil {
		return nil
	}
	out := []Unit{{ID: m.ID, Version: m.Version, Name: m.Name, Provides: m.Provides, Depends: m.Depends, Breaks: m.Breaks, Conflicts: m.Conflicts}}
	for _, n := range m.Nested {
		out = append(out, UnitsOf(n)...)
	}
	return out
}

// Where a provider in the environment comes from.
const (
	FromGame      = "game"      // minecraft, fabricloader, java
	FromClient    = "client"    // the Armstrong client layer
	FromInstalled = "installed" // the player's mods folder
	FromPlanned   = "planned"   // about to be installed
)

type provider struct {
	unit   Unit
	origin string
	owner  string // display name of the jar it's in
	key    string // identifies the jar, so a jar isn't checked against itself
}

// Env is everything a mod could depend on or clash with.
type Env struct {
	byID      map[string][]*provider
	providers []*provider
}

func NewEnv(minecraft, loader, java string) *Env {
	e := &Env{byID: map[string][]*provider{}}
	for _, u := range []Unit{{ID: "minecraft", Version: minecraft, Name: "Minecraft"}, {ID: "fabricloader", Version: loader, Name: "Fabric Loader"}, {ID: "java", Version: java, Name: "Java"}} {
		e.addUnit(u, FromGame, u.Name, "game:"+u.ID)
	}
	return e
}

// Add puts a jar's units in the environment. key identifies the jar.
func (e *Env) Add(units []Unit, origin, owner, key string) {
	for _, u := range units {
		e.addUnit(u, origin, owner, key)
	}
}

func (e *Env) addUnit(u Unit, origin, owner, key string) {
	p := &provider{unit: u, origin: origin, owner: owner, key: key}
	e.providers = append(e.providers, p)
	for _, id := range append([]string{u.ID}, u.Provides...) {
		e.byID[id] = append(e.byID[id], p)
	}
}

// Clone copies the environment so a plan can add to it without touching the base.
func (e *Env) Clone() *Env {
	c := &Env{byID: make(map[string][]*provider, len(e.byID)), providers: slices.Clone(e.providers)}
	for id, ps := range e.byID {
		c.byID[id] = slices.Clone(ps)
	}
	return c
}

// Provided returns who provides an id (the client layer and game first).
func (e *Env) Provided(id string) (origin, owner, version string, ok bool) {
	ps := e.byID[id]
	if len(ps) == 0 {
		return "", "", "", false
	}
	return ps[0].origin, ps[0].owner, ps[0].unit.Version, true
}

type Severity int

const (
	Warning Severity = iota
	Error
)

type Problem struct {
	Severity Severity
	Text     string
}

func (p Problem) String() string { return p.Text }

// Check tests a jar (its units, identified by key) against the environment,
// in both directions: what it needs and breaks, and who else needs or breaks it.
func (e *Env) Check(units []Unit, owner, key string) []Problem {
	var out []Problem
	add := func(s Severity, format string, args ...any) {
		out = append(out, Problem{Severity: s, Text: fmt.Sprintf(format, args...)})
	}
	self := map[string]Unit{}
	for _, u := range units {
		for _, id := range append([]string{u.ID}, u.Provides...) {
			self[id] = u
		}
	}
	others := func(id string) []*provider {
		var ps []*provider
		for _, p := range e.byID[id] {
			if p.key != key {
				ps = append(ps, p)
			}
		}
		return ps
	}

	for _, u := range units {
		for _, id := range sortedKeys(u.Depends) {
			preds := u.Depends[id]
			if mine, ok := self[id]; ok {
				if Match(mine.Version, preds) == No {
					add(Error, "%s needs %s %s, but it bundles %s", u.title(), id, joinPreds(preds), mine.Version)
				}
				continue
			}
			ps := others(id)
			if len(ps) == 0 {
				add(Error, "%s needs %s %s, which isn't installed", u.title(), id, joinPreds(preds))
				continue
			}
			if !slices.ContainsFunc(ps, func(p *provider) bool { return Match(p.unit.Version, preds) != No }) {
				p := ps[0]
				if id == "minecraft" {
					add(Error, "%s is made for Minecraft %s, not %s", owner, joinPreds(preds), p.unit.Version)
				} else {
					add(Error, "%s needs %s %s, but %s has %s", u.title(), p.unit.title(), joinPreds(preds), describe(p), p.unit.Version)
				}
			}
		}
		for _, id := range sortedKeys(u.Breaks) {
			for _, p := range others(id) {
				if Match(p.unit.Version, u.Breaks[id]) == Yes {
					add(Error, "%s doesn't work with %s %s (%s)", owner, p.unit.title(), p.unit.Version, describe(p))
				}
			}
		}
		for _, id := range sortedKeys(u.Conflicts) {
			for _, p := range others(id) {
				if Match(p.unit.Version, u.Conflicts[id]) == Yes {
					add(Warning, "%s may not work well with %s (%s)", owner, p.unit.title(), describe(p))
				}
			}
		}
	}

	// The other direction: mods already there that break, conflict with or
	// need a different version of anything this jar provides.
	for _, p := range e.providers {
		if p.key == key {
			continue
		}
		for id, preds := range p.unit.Breaks {
			if u, ok := self[id]; ok && Match(u.Version, preds) == Yes {
				add(Error, "%s (%s) doesn't work with %s %s", p.unit.title(), describe(p), u.title(), u.Version)
			}
		}
		for id, preds := range p.unit.Conflicts {
			if u, ok := self[id]; ok && Match(u.Version, preds) == Yes {
				add(Warning, "%s (%s) may not work well with %s", p.unit.title(), describe(p), u.title())
			}
		}
		for id, preds := range p.unit.Depends {
			if u, ok := self[id]; ok && Match(u.Version, preds) == No && !e.satisfiedElsewhere(id, preds, key) {
				add(Error, "%s (%s) needs %s %s, not %s", p.unit.title(), describe(p), u.title(), joinPreds(preds), u.Version)
			}
		}
	}
	return dedupe(out)
}

// satisfiedElsewhere reports whether another jar also provides id at a fitting version.
func (e *Env) satisfiedElsewhere(id string, preds []string, key string) bool {
	for _, p := range e.byID[id] {
		if p.key != key && Match(p.unit.Version, preds) != No {
			return true
		}
	}
	return false
}

func describe(p *provider) string {
	switch p.origin {
	case FromClient:
		return "part of the Armstrong client"
	case FromInstalled:
		return "installed: " + p.owner
	case FromPlanned:
		return "being installed: " + p.owner
	}
	return "the game"
}

func joinPreds(preds []string) string {
	if len(preds) == 1 && preds[0] == "*" {
		return ""
	}
	return strings.Join(preds, " or ")
}

func sortedKeys(d modmeta.Deps) []string {
	keys := make([]string, 0, len(d))
	for k := range d {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func dedupe(ps []Problem) []Problem {
	seen := map[string]bool{}
	var out []Problem
	for _, p := range ps {
		if !seen[p.Text] {
			seen[p.Text] = true
			out = append(out, p)
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].Severity > out[b].Severity })
	return out
}

// HasErrors reports whether any problem blocks the game from starting.
func HasErrors(ps []Problem) bool {
	return slices.ContainsFunc(ps, func(p Problem) bool { return p.Severity == Error })
}
