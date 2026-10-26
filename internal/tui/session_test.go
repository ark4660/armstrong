package tui

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestExitSummary(t *testing.T) {
	exitErr := exec.Command("sh", "-c", "exit 3").Run()
	tests := []struct {
		name    string
		msg     exitedMsg
		stopped bool
		want    string
	}{
		{"clean exit", exitedMsg{}, false, ""},
		{"stopped by user", exitedMsg{err: exitErr}, true, ""},
		{"exit code", exitedMsg{err: exitErr}, false, "Game exited with code 3"},
		{"other error", exitedMsg{err: errors.New("boom")}, false, "Game exited: boom"},
		{"failure marker", exitedMsg{failure: "Mod loading failed"}, false, "Game failed: Mod loading failed"},
		{"crash wins", exitedMsg{failure: "x", crashes: []string{"a.txt", "b.txt"}}, true, "Game crashed: b.txt"},
	}
	for _, tt := range tests {
		got := exitSummary(tt.msg, tt.stopped)
		if (tt.want == "") != (got == "") || !strings.HasPrefix(got, tt.want) {
			t.Errorf("%s: got %q, want prefix %q", tt.name, got, tt.want)
		}
	}
}
