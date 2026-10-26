package game

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestMavenPath(t *testing.T) {
	tests := map[string]string{
		"net.fabricmc:fabric-loader:0.20.1":       "net/fabricmc/fabric-loader/0.20.1/fabric-loader-0.20.1.jar",
		"com.mojang:jtracy:1.14.38:natives-linux": "com/mojang/jtracy/1.14.38/jtracy-1.14.38-natives-linux.jar",
		"org.example:thing:1.0@zip":               "org/example/thing/1.0/thing-1.0.zip",
		"org.example:thing:1.0:sources@txt":       "org/example/thing/1.0/thing-1.0-sources.txt",
	}
	for in, want := range tests {
		got, err := MavenPath(in)
		if err != nil || got != want {
			t.Errorf("MavenPath(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := MavenPath("bad"); err == nil {
		t.Error("MavenPath(bad) should fail")
	}
}

func TestRules(t *testing.T) {
	linux := Env{OS: "linux", Arch: "x86_64", Features: map[string]bool{}}
	tests := []struct {
		rules string
		want  bool
	}{
		{`[]`, true},
		{`[{"action":"allow","os":{"name":"osx"}}]`, false},
		{`[{"action":"allow","os":{"name":"linux"}}]`, true},
		{`[{"action":"allow"},{"action":"disallow","os":{"name":"linux"}}]`, false},
		{`[{"action":"allow","os":{"arch":"x86"}}]`, false},
		{`[{"action":"allow","features":{"is_demo_user":true}}]`, false},
	}
	for _, tt := range tests {
		var rs Rules
		if err := json.Unmarshal([]byte(tt.rules), &rs); err != nil {
			t.Fatal(err)
		}
		if got := rs.Allowed(linux); got != tt.want {
			t.Errorf("%s: got %v, want %v", tt.rules, got, tt.want)
		}
	}
}

func TestArgumentsAndExpand(t *testing.T) {
	var args []Argument
	err := json.Unmarshal([]byte(`["--username","${auth_player_name}",
		{"rules":[{"action":"allow","os":{"name":"osx"}}],"value":["-XstartOnFirstThread"]},
		{"rules":[{"action":"allow","features":{"is_quick_play_multiplayer":true}}],"value":["--quickPlayMultiplayer","${quickPlayMultiplayer}"]},
		"-Dx=${unknown}"]`), &args)
	if err != nil {
		t.Fatal(err)
	}
	env := Env{OS: "linux", Arch: "x86_64", Features: map[string]bool{"is_quick_play_multiplayer": true}}
	got := expand(args, env, map[string]string{"auth_player_name": "Steve", "quickPlayMultiplayer": "mc.example.org"})
	want := []string{"--username", "Steve", "--quickPlayMultiplayer", "mc.example.org", "-Dx=${unknown}"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMergeReplacesLibraries(t *testing.T) {
	parent := &Version{ID: "26.3", MainClass: "net.minecraft.client.main.Main", Libraries: []Library{
		{Name: "org.ow2.asm:asm:9.6"}, {Name: "com.mojang:brigadier:1.0"},
	}}
	child := &Version{ID: "fabric-loader-x-26.3", InheritsFrom: "26.3", MainClass: "Knot", Libraries: []Library{
		{Name: "org.ow2.asm:asm:9.10.1"}, {Name: "net.fabricmc:fabric-loader:0.20.1"},
	}}
	m := Merge(parent, child)
	var names []string
	for _, l := range m.Libraries {
		names = append(names, l.Name)
	}
	want := []string{"com.mojang:brigadier:1.0", "org.ow2.asm:asm:9.10.1", "net.fabricmc:fabric-loader:0.20.1"}
	if m.ID != child.ID || m.MainClass != "Knot" || !reflect.DeepEqual(names, want) {
		t.Errorf("merge: id=%s main=%s libs=%v", m.ID, m.MainClass, names)
	}
}
