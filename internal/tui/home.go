package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/ark4660/armstrong/internal/app"
	"github.com/ark4660/armstrong/internal/auth"
	"github.com/ark4660/armstrong/internal/client"
	"github.com/ark4660/armstrong/internal/game"
	"github.com/ark4660/armstrong/internal/instance"
)

func (m model) homeView() string {
	s := m.styles
	row := func(label, value string) string {
		return "  " + s.label.Render(label) + value
	}

	account := s.warn.Render("none") + s.muted.Render(" (press a to add one)")
	if acc, ok := m.accounts.Active(); ok {
		account = s.value.Render(acc.Name)
		if acc.Type == auth.TypeMicrosoft {
			account += s.ok.Render(" Microsoft")
		} else {
			account += s.muted.Render(" offline")
		}
	}

	inst := m.selected()
	logo := lipgloss.NewStyle().MarginLeft(2).Render(m.logoView(m.width, m.height))
	lines := []string{"", logo, ""}
	if inst == nil {
		lines = append(lines,
			row("Account", account),
			"",
			"  No instances yet. Press "+s.key.Render("n")+" to create one.",
		)
	} else {
		resolved := ""
		if inst.ID == m.runningID {
			resolved = m.loaderVersion
		}
		lines = append(lines,
			"  "+s.title.Render(inst.Name)+s.muted.Render("  "+lastPlayed(inst.LastPlayed)),
			"",
			row("Version", s.value.Render(versionLabel(inst, resolved))),
			row("Client", m.clientLabel(inst)),
			row("Account", account),
			row("Memory", s.value.Render(fmt.Sprintf("%d MB", inst.MemoryMB))),
			row("JVM", s.value.Render(inst.Preset())+s.muted.Render("  "+presetNote[inst.Preset()])),
			row("Folder", s.muted.Render(tildePath(inst.GameDir()))),
			"",
			lipgloss.NewStyle().MarginLeft(2).Render(m.playButton(inst.ID)),
			"",
		)
	}
	lines = append(lines, m.statusLines()...)
	if m.disabledNote != "" && inst != nil && inst.ID == m.lastPlayedID {
		lines = append(lines, "", "  "+s.warn.Width(max(m.width-4, 20)).Render(m.disabledNote))
	}
	if m.notice != "" {
		style := s.err
		if strings.HasPrefix(m.notice, "Couldn't read") {
			style = s.warn
		}
		lines = append(lines, "", "  "+style.Width(max(m.width-4, 20)).Render(m.notice))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// presetNote explains each JVM preset in a few words on the home screen.
var presetNote = map[string]string{
	game.PresetDefault: "G1, heap reserved up front",
	game.PresetZGC:     "no GC pauses, more memory",
	game.PresetNone:    "Java's defaults",
}

func (m model) playButton(id string) string {
	s := m.styles
	label := "▶ Play"
	_, hasAccount := m.accounts.Active()
	dim := !hasAccount
	if m.busy() {
		if id == m.runningID {
			switch m.state {
			case stateInstalling, stateStarting:
				label = "Launching…"
			case stateRunning:
				label = "● Running"
			case stateStopping:
				label = "Stopping…"
			}
		} else {
			dim = true // one game at a time
		}
	}
	if dim {
		return s.button.Foreground(s.muted.GetForeground()).BorderForeground(s.muted.GetForeground()).Render(label)
	}
	return s.button.Render(label)
}

func (m model) statusLines() []string {
	s := m.styles
	running := ""
	if sel := m.selected(); m.busy() && (sel == nil || sel.ID != m.runningID) {
		running = s.muted.Render(" · " + m.runningLabel())
	}
	switch m.state {
	case stateInstalling:
		line := "  " + m.spin.View() + " " + s.value.Render(m.step) + running
		if m.prog.Total == 0 {
			return []string{line}
		}
		pct := float64(m.prog.Done) / float64(m.prog.Total)
		counts := s.muted.Render(fmt.Sprintf(" %d/%d", m.prog.Done, m.prog.Total))
		if m.prog.Bytes > 0 {
			counts += s.muted.Render(fmt.Sprintf(" · %.1f MB", float64(m.prog.Bytes)/1e6))
		}
		return []string{line, "  " + m.bar.ViewAs(pct) + counts}
	case stateStarting:
		return []string{"  " + m.spin.View() + " " + s.value.Render("Starting the game") + running + s.muted.Render("  (tab: console)")}
	case stateRunning:
		return []string{"  " + s.ok.Render("Game running") + running + s.muted.Render(fmt.Sprintf(" · started in %s", m.startedIn.Round(100*time.Millisecond)))}
	case stateStopping:
		return []string{"  " + m.spin.View() + " " + s.value.Render("Stopping the game") + running}
	}
	return nil
}

// tildePath shortens a path under the home directory to ~/....
func tildePath(p string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	if rel, err := filepath.Rel(home, p); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.Join("~", rel)
	}
	return p
}

// clientLabel describes the client layer an instance runs with.
func (m model) clientLabel(inst *instance.Instance) string {
	s := m.styles
	if inst.Loader != instance.LoaderFabric {
		return s.muted.Render("none (vanilla instance)")
	}
	man, err := client.Load(inst.Minecraft)
	if err != nil {
		return s.err.Render(err.Error())
	}
	core, addons := 0, 0
	for _, mod := range man.Enabled(inst.Addons) {
		if mod.Optional {
			addons++
		} else {
			core++
		}
	}
	text := fmt.Sprintf("%s %s · %d core mods", app.DisplayName, man.Version, core)
	if addons > 0 {
		text += fmt.Sprintf(" + %d add-ons", addons)
	}
	return s.value.Render(text)
}
