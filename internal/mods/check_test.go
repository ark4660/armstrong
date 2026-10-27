package mods

import (
	"strings"
	"testing"

	"github.com/ark4660/armstrong/internal/modmeta"
)

// clientEnv is a 1.21.11 client: Fabric API 0.141.6 (bundling fabric-api-base)
// and Sodium, which breaks Fabric API older than 0.140.
func clientEnv() *Env {
	e := NewEnv("1.21.11", "0.20.1", "21")
	e.Add([]Unit{{ID: "fabric-api", Version: "0.141.6+1.21.11", Name: "Fabric API"}, {ID: "fabric-api-base", Version: "1.0.5"}}, FromClient, "Fabric API", "client:fabric-api")
	e.Add([]Unit{{ID: "sodium", Version: "0.8.14+mc1.21.11", Name: "Sodium", Breaks: modmeta.Deps{"fabric-api": {"<0.140.0"}}}}, FromClient, "Sodium", "client:sodium")
	return e
}

func errorsIn(ps []Problem) []string {
	var out []string
	for _, p := range ps {
		if p.Severity == Error {
			out = append(out, p.Text)
		}
	}
	return out
}

// Distant Horizons bundles an older Fabric API. Fabric loads only the newest
// copy (the client's), so the bundled one doesn't count: not against Sodium,
// not for satisfying anything.
func TestBundledOlderCopyIsIgnored(t *testing.T) {
	e := clientEnv()
	dh := []Unit{
		{ID: "distanthorizons", Version: "3.3.3", Name: "Distant Horizons", Depends: modmeta.Deps{"fabric-api-base": {"*"}, "minecraft": {"1.21.11"}}},
		{ID: "fabric-api", Version: "0.139.4+1.21.11", Name: "Fabric API"},
		{ID: "fabric-api-base", Version: "1.0.4"},
	}
	e.Add(dh, FromPlanned, "Distant Horizons", "plan:dh")
	if errs := errorsIn(e.Check(dh, "Distant Horizons", "plan:dh")); len(errs) > 0 {
		t.Errorf("errors for a bundled copy Fabric won't load: %v", errs)
	}
	// And the client's Sodium isn't reported against it either.
	sodium := []Unit{e.byID["sodium"][0].unit}
	if errs := errorsIn(e.Check(sodium, "Sodium", "client:sodium")); len(errs) > 0 {
		t.Errorf("Sodium reported against the bundled copy: %v", errs)
	}
}

func TestBundledNewerCopyWarns(t *testing.T) {
	e := clientEnv()
	mod := []Unit{{ID: "fresh", Version: "1.0", Name: "Fresh"}, {ID: "fabric-api", Version: "0.142.0+1.21.11", Name: "Fabric API"}}
	e.Add(mod, FromPlanned, "Fresh", "plan:fresh")
	ps := e.Check(mod, "Fresh", "plan:fresh")
	if errs := errorsIn(ps); len(errs) > 0 {
		t.Errorf("errors: %v", errs)
	}
	if len(ps) != 1 || !strings.Contains(ps[0].Text, "Fresh bundles Fabric API 0.142.0+1.21.11, newer than the Armstrong client's 0.141.6+1.21.11") {
		t.Errorf("problems = %v, want one warning about the newer bundled copy", ps)
	}
}

// Two mods bundling the same library: Fabric loads one copy, so it's there.
func TestEqualBundledCopiesStillProvide(t *testing.T) {
	e := clientEnv()
	a := []Unit{{ID: "a", Version: "1.0", Name: "A", Depends: modmeta.Deps{"lib": {">=1.0"}}}, {ID: "lib", Version: "1.0", Name: "Lib"}}
	b := []Unit{{ID: "b", Version: "1.0", Name: "B", Depends: modmeta.Deps{"lib": {"*"}}}, {ID: "lib", Version: "1.0", Name: "Lib"}}
	e.Add(a, FromInstalled, "A", "file:a.jar")
	e.Add(b, FromPlanned, "B", "plan:b")
	for _, c := range []struct {
		units []Unit
		owner string
		key   string
	}{{a, "A", "file:a.jar"}, {b, "B", "plan:b"}} {
		if errs := errorsIn(e.Check(c.units, c.owner, c.key)); len(errs) > 0 {
			t.Errorf("%s: %v", c.owner, errs)
		}
	}
}

// A bundled copy that does load is checked like any other mod.
func TestBundledCopyThatLoadsIsChecked(t *testing.T) {
	e := NewEnv("1.21.11", "0.20.1", "21")
	e.Add([]Unit{{ID: "sodium", Version: "0.8.14", Name: "Sodium", Breaks: modmeta.Deps{"oldlib": {"<2.0"}}}}, FromClient, "Sodium", "client:sodium")
	mod := []Unit{{ID: "user", Version: "1.0", Name: "User"}, {ID: "oldlib", Version: "1.5", Name: "Old Lib"}}
	e.Add(mod, FromPlanned, "User", "plan:user")
	if errs := errorsIn(e.Check(mod, "User", "plan:user")); len(errs) != 1 || !strings.Contains(errs[0], "Sodium (part of the Armstrong client) doesn't work with Old Lib 1.5") {
		t.Errorf("errors = %v, want Sodium's breaks against the only Old Lib", errs)
	}
}
