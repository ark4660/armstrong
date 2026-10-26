package game

import "runtime"

// Rules decide whether a library or argument applies. With no rules it
// applies; otherwise it's disallowed unless the last matching rule allows it.
type Rules []Rule

type Rule struct {
	Action   string          `json:"action"`
	OS       *OSRule         `json:"os,omitempty"`
	Features map[string]bool `json:"features,omitempty"`
}

type OSRule struct {
	Name string `json:"name,omitempty"`
	Arch string `json:"arch,omitempty"`
	// Version (a regex on the OS version) is only used by old versions for
	// macOS 10.5 and Windows 10 workarounds; it's ignored here.
}

// Env is what rules are evaluated against.
type Env struct {
	OS       string // "linux", "osx", "windows"
	Arch     string // "x86" for 32-bit, "x86_64", "arm64"
	Features map[string]bool
}

func CurrentEnv() Env {
	os := runtime.GOOS
	if os == "darwin" {
		os = "osx"
	}
	arch := runtime.GOARCH
	switch arch {
	case "386":
		arch = "x86"
	case "amd64":
		arch = "x86_64"
	}
	return Env{OS: os, Arch: arch, Features: map[string]bool{}}
}

func (rs Rules) Allowed(env Env) bool {
	if len(rs) == 0 {
		return true
	}
	allowed := false
	for _, r := range rs {
		if r.matches(env) {
			allowed = r.Action == "allow"
		}
	}
	return allowed
}

func (r Rule) matches(env Env) bool {
	if r.OS != nil {
		if r.OS.Name != "" && r.OS.Name != env.OS {
			return false
		}
		if r.OS.Arch != "" && r.OS.Arch != env.Arch {
			return false
		}
	}
	for name, want := range r.Features {
		if env.Features[name] != want {
			return false
		}
	}
	return true
}
