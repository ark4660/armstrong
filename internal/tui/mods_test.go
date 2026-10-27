package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/ark4660/armstrong/internal/mods"
)

func key(s string) tea.KeyPressMsg {
	if s == "esc" {
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	}
	return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
}

// checking is a model on the Browse tab with an install check running.
func checking(cancelled *bool) model {
	m := model{mods: newModsState(), styles: newStyles(true)}
	m.mods.mgr = &mods.Manager{}
	m.mods.view = modsBrowse
	m.mods.checkSeq = 7
	m.mods.working = "Checking Distant Horizons"
	m.mods.cancel = func() { *cancelled = true }
	return m
}

func TestModsCheckBlocksKeysAndEscCancels(t *testing.T) {
	var cancelled bool
	m := checking(&cancelled)

	// Other keys do nothing while the check runs: no tab switch, no second check.
	for _, k := range []string{"i", "d", "b", "/"} {
		next, _ := m.handleModsKey(key(k))
		m = next.(model)
	}
	if m.mods.view != modsBrowse || m.mods.working == "" || cancelled {
		t.Fatalf("a key got through during the check: view %d, working %q, cancelled %v", m.mods.view, m.mods.working, cancelled)
	}
	if got := m.modsKeys(); len(got) != 1 || got[0].key != "esc" {
		t.Errorf("footer during a check = %v, want only esc", got)
	}

	// Progress shows next to the spinner.
	next, _ := m.handleModsMsg(modsProgressMsg{seq: 7, text: "DistantHorizons-3.3.3.jar (2 of 9)"})
	m = next.(model)
	if m.mods.progress != "DistantHorizons-3.3.3.jar (2 of 9)" {
		t.Errorf("progress = %q", m.mods.progress)
	}

	// Esc cancels; Esc again (no check running) goes back to Installed as usual.
	next, _ = m.handleModsKey(key("esc"))
	m = next.(model)
	if !cancelled || m.mods.working != "" || m.mods.notice != "Cancelled." || m.mods.view != modsBrowse {
		t.Fatalf("after esc: cancelled %v, working %q, notice %q, view %d", cancelled, m.mods.working, m.mods.notice, m.mods.view)
	}

	// The cancelled check's late answer and progress are dropped.
	next, _ = m.handleModsMsg(modsProgressMsg{seq: 7, text: "late"})
	m = next.(model)
	next, _ = m.handleModsMsg(modsPlanMsg{seq: 7, title: "Distant Horizons", plan: &mods.Plan{}})
	m = next.(model)
	if m.mods.plan != nil || m.mods.progress != "" {
		t.Errorf("a cancelled check's result was used: plan %v, progress %q", m.mods.plan, m.mods.progress)
	}
}

func TestModsCheckAnswerForCurrentCheck(t *testing.T) {
	var cancelled bool
	m := checking(&cancelled)
	next, _ := m.handleModsMsg(modsPlanMsg{seq: 7, title: "Distant Horizons", err: errors.New("boom")})
	m = next.(model)
	if m.mods.working != "" || m.mods.cancel != nil || m.mods.notice == "" {
		t.Errorf("after the answer: working %q, cancel set %v, notice %q", m.mods.working, m.mods.cancel != nil, m.mods.notice)
	}
}

func TestModsChangeBeingWrittenIgnoresEsc(t *testing.T) {
	m := model{mods: newModsState(), styles: newStyles(true)}
	m.mods.mgr = &mods.Manager{}
	m.mods.working = "Installing Distant Horizons" // no cancel: files are being written
	next, _ := m.handleModsKey(key("esc"))
	if got := next.(model).mods; got.working == "" || got.notice != "" {
		t.Errorf("esc interrupted a commit: working %q, notice %q", got.working, got.notice)
	}
}

// withVersions is a model showing a version list of 12 versions.
func withVersions() model {
	m := model{mods: newModsState(), styles: newStyles(true), width: 140, height: 40, send: &sender{fn: func(tea.Msg) {}}}
	m.mods.mgr = &mods.Manager{}
	m.mods.view = modsBrowse
	m.mods.versions = &versionsState{project: "P-dh", title: "Distant Horizons", loading: true,
		checks: map[string]versionCheck{}, picker: picker{height: 20}}
	var vs []mods.Version
	for i := 12; i >= 1; i-- {
		vs = append(vs, mods.Version{ID: fmt.Sprintf("v%d", i), Number: fmt.Sprintf("3.%d", i), Channel: "release", Published: "2026-10-01T00:00:00Z"})
	}
	next, _ := m.handleModsMsg(modsVersionsMsg{project: "P-dh", versions: vs})
	return next.(model)
}

func TestVersionListChecks(t *testing.T) {
	m := withVersions()
	if v := m.mods.versions; v.loading || len(v.list) != 12 {
		t.Fatalf("list not loaded: %+v", v)
	}

	// c checks the highlighted version only.
	next, cmd := m.handleModsKey(key("c"))
	m = next.(model)
	if cmd == nil || m.mods.working != "Checking Distant Horizons 3.12" || m.mods.cancel == nil {
		t.Fatalf("c: working %q", m.mods.working)
	}
	seq := m.mods.checkSeq
	next, _ = m.handleModsMsg(modsVersionCheckMsg{seq: seq, id: "v12", check: versionCheck{text: "needs Fabric API 0.120"}})
	m = next.(model)
	next, _ = m.handleModsMsg(modsVersionCheckMsg{seq: seq, done: true})
	m = next.(model)
	if m.mods.working != "" || m.mods.versions.checks["v12"].fits {
		t.Fatalf("after the check: working %q, checks %v", m.mods.working, m.mods.versions.checks)
	}

	// C checks a batch from the cursor down; Esc stops it and keeps what was found.
	next, _ = m.handleModsKey(key("down"))
	m = next.(model)
	next, _ = m.handleModsKey(key("C"))
	m = next.(model)
	if m.mods.working != "Checking 10 versions of Distant Horizons" || m.mods.progress != "1 of 10" {
		t.Fatalf("C: working %q, progress %q", m.mods.working, m.mods.progress)
	}
	seq = m.mods.checkSeq
	next, _ = m.handleModsMsg(modsVersionCheckMsg{seq: seq, id: "v11", check: versionCheck{fits: true, text: "fits"}})
	m = next.(model)
	next, _ = m.handleModsKey(key("esc"))
	m = next.(model)
	next, _ = m.handleModsMsg(modsVersionCheckMsg{seq: seq, id: "v10", check: versionCheck{fits: true, text: "late"}})
	m = next.(model)
	if m.mods.versions == nil || m.mods.notice != "Cancelled." || !m.mods.versions.checks["v11"].fits {
		t.Fatalf("after esc: versions %v, notice %q", m.mods.versions, m.mods.notice)
	}
	if _, late := m.mods.versions.checks["v10"]; late {
		t.Error("a check that arrived after cancelling was kept")
	}

	view := m.versionsView()
	for _, want := range []string{"Versions of Distant Horizons", "✗ needs Fabric API 0.120", "✓ fits", "not checked"} {
		if !strings.Contains(view, want) {
			t.Errorf("view lacks %q:\n%s", want, view)
		}
	}

	// Esc with nothing running goes back to the search.
	next, _ = m.handleModsKey(key("esc"))
	if next.(model).mods.versions != nil {
		t.Error("esc didn't close the version list")
	}
}

func TestInstallErrorPointsToOlderVersions(t *testing.T) {
	err := &mods.ErrNoFit{Project: "Old", Tried: 10, Untried: 4, Problems: []mods.Problem{{Severity: mods.Error, Text: "Old is made for Minecraft 1.20.1, not 26.3"}}}
	if got := installError("Old", err); !strings.Contains(got, "4 older version(s) weren't tried: press v") {
		t.Errorf("installError = %q", got)
	}
}
