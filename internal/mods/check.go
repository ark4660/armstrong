package mods

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/ark4660/armstrong/internal/client"
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
	nested bool   // bundled inside the jar (jar-in-jar), not the jar's own mod
}

// Env is everything a mod could depend on or clash with.
type Env struct {
	byID      map[string][]*provider
	providers []*provider
	addons    map[string]string // id → name of client add-ons that are switched off
}

func NewEnv(minecraft, loader, java string) *Env {
	e := &Env{byID: map[string][]*provider{}}
	for _, u := range []Unit{{ID: "minecraft", Version: minecraft, Name: "Minecraft"}, {ID: "fabricloader", Version: loader, Name: "Fabric Loader"}, {ID: "java", Version: java, Name: "Java"}} {
		e.addUnit(u, FromGame, u.Name, "game:"+u.ID, false)
	}
	return e
}

// Add puts a jar's units in the environment (its own mod first, then the
// ones it bundles, as UnitsOf orders them). key identifies the jar.
func (e *Env) Add(units []Unit, origin, owner, key string) {
	for i, u := range units {
		e.addUnit(u, origin, owner, key, i > 0)
	}
}

func (e *Env) addUnit(u Unit, origin, owner, key string, nested bool) {
	p := &provider{unit: u, origin: origin, owner: owner, key: key, nested: nested}
	e.providers = append(e.providers, p)
	for _, id := range append([]string{u.ID}, u.Provides...) {
		e.byID[id] = append(e.byID[id], p)
	}
}

// AddAddon notes a client add-on the player could switch on, so a mod that
// needs it is told so instead of that it isn't installed.
func (e *Env) AddAddon(mod client.Mod) {
	if e.addons == nil {
		e.addons = map[string]string{}
	}
	for _, id := range append([]string{mod.ID}, mod.Provides...) {
		e.addons[id] = mod.Name
	}
}

// Clone copies the environment so a plan can add to it without touching the base.
func (e *Env) Clone() *Env {
	c := &Env{byID: make(map[string][]*provider, len(e.byID)), providers: slices.Clone(e.providers), addons: e.addons}
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
	missing  bool // a dependency nothing provides (yet)
}

func (p Problem) String() string { return p.Text }

// Check tests a jar (its units, identified by key) against the environment,
// in both directions: what it needs and breaks, and who else needs or breaks it.
func (e *Env) Check(units []Unit, owner, key string) []Problem {
	var out []Problem
	add := func(s Severity, format string, args ...any) {
		out = append(out, Problem{Severity: s, Text: fmt.Sprintf(format, args...)})
	}
	// Bundled copies that Fabric won't load (a same or newer copy is
	// elsewhere) don't count: not what they provide, need or break.
	var live []Unit
	for i, u := range units {
		if i > 0 && e.shadowed(u, key) {
			continue
		}
		live = append(live, u)
		if i > 0 {
			for _, p := range e.byID[u.ID] {
				if p.key != key && p.origin == FromClient && p.unit.ID == u.ID && !newerOrSame(p.unit.Version, u.Version) {
					add(Warning, "%s bundles %s %s, newer than the Armstrong client's %s: Fabric will load the bundled one",
						owner, u.title(), u.Version, p.unit.Version)
				}
			}
		}
	}
	self := map[string]Unit{}
	for _, u := range live {
		for _, id := range append([]string{u.ID}, u.Provides...) {
			self[id] = u
		}
	}
	others := func(id string) []*provider {
		var ps []*provider
		for _, p := range e.byID[id] {
			if p.key != key && !e.inactive(p) {
				ps = append(ps, p)
			}
		}
		return ps
	}

	for _, u := range live {
		for _, id := range sortedKeys(u.Depends) {
			preds := u.Depends[id]
			if mine, ok := self[id]; ok {
				if Match(mine.Version, preds) == No {
					add(Error, "%s needs %s, but it bundles %s", u.title(), need(id, preds), mine.Version)
				}
				continue
			}
			ps := others(id)
			if len(ps) == 0 {
				if name, ok := e.addons[id]; ok {
					add(Error, "%s %s", u.title(), addonNeeded(name))
				} else {
					add(Error, "%s needs %s, which isn't installed", u.title(), need(id, preds))
				}
				out[len(out)-1].missing = true
				continue
			}
			if !slices.ContainsFunc(ps, func(p *provider) bool { return Match(p.unit.Version, preds) != No }) {
				p := ps[0]
				if id == "minecraft" {
					add(Error, "%s is made for Minecraft %s, not %s", owner, joinPreds(preds), p.unit.Version)
				} else {
					add(Error, "%s needs %s, but %s has %s", u.title(), need(p.unit.title(), preds), describe(p), p.unit.Version)
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
		if p.key == key || e.inactive(p) {
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
				add(Error, "%s (%s) needs %s, not %s", p.unit.title(), describe(p), need(u.title(), preds), u.Version)
			}
		}
	}
	return dedupe(out)
}

// shadowed reports whether a bundled unit of the jar identified by key won't
// load: Fabric loads one copy of each mod, the newest, so a bundled copy with
// a same or newer copy anywhere else (the client's Fabric API, say) is ignored.
// Of equal bundled copies one has to stay: the one from the jar whose key
// sorts first.
func (e *Env) shadowed(u Unit, key string) bool {
	for _, p := range e.byID[u.ID] {
		if p.key == key || p.unit.ID != u.ID {
			continue
		}
		c, ok := compareLoose(p.unit.Version, u.Version)
		if ok && (c > 0 || c == 0 && (!p.nested || p.key < key)) {
			return true
		}
	}
	return false
}

// inactive reports whether a provider in the environment is a bundled copy
// that Fabric won't load.
func (e *Env) inactive(p *provider) bool {
	return p.nested && e.shadowed(p.unit, p.key)
}

// newerOrSame reports whether version a is at least b (see compareLoose).
func newerOrSame(a, b string) bool {
	c, ok := compareLoose(a, b)
	return ok && c >= 0
}

// compareLoose compares versions the way Fabric picks among copies; versions
// that aren't semantic only compare equal to themselves.
func compareLoose(a, b string) (int, bool) {
	if c, ok := CompareVersions(a, b); ok {
		return c, true
	}
	return 0, a == b
}

// satisfiedElsewhere reports whether another jar also provides id at a fitting version.
func (e *Env) satisfiedElsewhere(id string, preds []string, key string) bool {
	for _, p := range e.byID[id] {
		if p.key != key && !e.inactive(p) && Match(p.unit.Version, preds) != No {
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

// need names a dependency with its version range, if it has one.
func need(name string, preds []string) string {
	if r := joinPreds(preds); r != "" {
		return name + " " + r
	}
	return name
}

// withoutMissing drops missing-dependency problems: the planner looks for
// those dependencies next.
func withoutMissing(ps []Problem) []Problem {
	return slices.DeleteFunc(slices.Clone(ps), func(p Problem) bool { return p.missing })
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
