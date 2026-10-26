package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ark4660/armstrong/internal/app"
	"github.com/ark4660/armstrong/internal/client"
	"github.com/ark4660/armstrong/internal/instance"
)

type (
	openWizardMsg struct{}
	duplicatedMsg struct {
		inst *instance.Instance
		err  error
	}
	deletedMsg struct {
		id  string
		err error
	}
)

const (
	stepVersion = iota
	stepName
)

// wizard is the "new instance" flow: version → name. Every instance runs
// the client on Fabric, so only versions with a client release are offered.
type wizard struct {
	active  bool
	step    int
	version picker
}

func (w *wizard) selectedVersion() string {
	if i := w.version.selected(); i >= 0 {
		return w.version.items[i]
	}
	return ""
}

func (w *wizard) keys() []keyHelp {
	return []keyHelp{{"↑↓", "choose"}, {"type", "filter"}, {"enter", "next"}, {"esc", "cancel"}}
}

func (m model) openWizard() (tea.Model, tea.Cmd) {
	m.screen = screenInstances
	m.confirmDelete = false
	m.wizard = wizard{
		active:  true,
		version: picker{height: max(m.height-12, 3), filterable: true},
	}
	m.wizard.version.setItems(client.SupportedVersions())
	return m, nil
}

func (m model) handleWizardKey(k string) (tea.Model, tea.Cmd) {
	w := &m.wizard
	switch k {
	case "esc":
		if w.version.filter != "" {
			w.version.filter = ""
			w.version.clamp()
		} else {
			w.active = false
		}
		return m, nil
	case "enter":
		mc := w.selectedVersion()
		if mc == "" {
			return m, nil
		}
		w.step = stepName
		m.editInput(inputWizardName, "Minecraft "+mc, "", 40)
		return m, nil
	}
	w.version.handleKey(k)
	return m, nil
}

func (m model) finishWizard(name string) (tea.Model, tea.Cmd) {
	w := &m.wizard
	inst, err := m.store.Create(instance.Instance{
		Name:      name,
		Minecraft: w.selectedVersion(),
		Loader:    instance.LoaderFabric,
	})
	if err != nil {
		m.notice = err.Error()
		return m, nil
	}
	m.closeInput()
	w.active, m.notice = false, ""
	m.selectInstance(inst.ID)
	m.reload()
	m.screen = screenHome
	return m, nil
}

func (m model) handleInstanceMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case openWizardMsg:
		return m.openWizard()
	case duplicatedMsg:
		m.working = ""
		if msg.err != nil {
			m.notice = "Copy failed: " + msg.err.Error()
			return m, nil
		}
		m.reload()
		for i, inst := range m.instances {
			if inst.ID == msg.inst.ID {
				m.instList.selectIndex(i)
			}
		}
	case deletedMsg:
		m.working = ""
		if msg.err != nil {
			m.notice = "Delete failed: " + msg.err.Error()
		}
		if m.settings.Instance == msg.id {
			m.settings.Instance = ""
			m.saveSettings()
		}
		m.reload()
	}
	return m, nil
}

// cursorInstance is the instance under the cursor on the Instances screen.
func (m *model) cursorInstance() *instance.Instance {
	if i := m.instList.selected(); i >= 0 && i < len(m.instances) {
		return m.instances[i]
	}
	return nil
}

func (m model) handleInstancesKey(k string) (tea.Model, tea.Cmd) {
	if m.working != "" {
		return m, nil
	}
	inst := m.cursorInstance()
	switch k {
	case "n":
		return m.openWizard()
	case "enter":
		if inst != nil {
			m.selectInstance(inst.ID)
			m.screen = screenHome
		}
	case "r":
		if inst != nil {
			m.editInput(inputRename, inst.Name, "", 40)
		}
	case "c":
		switch {
		case inst == nil:
		case inst.ID == m.runningID:
			m.notice = "Stop the game before copying this instance."
		default:
			m.editInput(inputDuplicate, inst.Name+" copy", "", 40)
		}
	case "d":
		switch {
		case inst == nil:
		case inst.ID == m.runningID:
			m.notice = "Stop the game before deleting this instance."
		default:
			m.confirmDelete = true
		}
	default:
		m.instList.handleKey(k)
	}
	return m, nil
}

func (m model) duplicateSelected(name string) (tea.Model, tea.Cmd) {
	src := m.cursorInstance()
	if src == nil {
		m.closeInput()
		return m, nil
	}
	if err := instance.ValidateName(name); err != nil {
		m.notice = err.Error()
		return m, nil
	}
	m.closeInput()
	m.notice, m.working = "", "Copying "+src.Name
	store, send := m.store, m.send.fn
	go func() {
		inst, err := store.Duplicate(src, name)
		send(duplicatedMsg{inst: inst, err: err})
	}()
	return m, m.spin.Tick
}

func (m model) deleteSelected() (tea.Model, tea.Cmd) {
	inst := m.cursorInstance()
	if inst == nil || inst.ID == m.runningID {
		return m, nil
	}
	m.notice, m.working = "", "Deleting "+inst.Name
	store, send := m.store, m.send.fn
	go func() {
		send(deletedMsg{id: inst.ID, err: store.Delete(inst)})
	}()
	return m, m.spin.Tick
}

func (m model) instancesView() string {
	if m.wizard.active {
		return m.wizardView()
	}
	s := m.styles
	lines := []string{""}
	if len(m.instances) == 0 {
		lines = append(lines, "  No instances yet. Press "+s.key.Render("n")+" to create one.")
	}
	sel := m.selected()
	nameW := 4
	for _, inst := range m.instances {
		nameW = max(nameW, lipgloss.Width(inst.Name))
	}
	nameW = min(nameW, 40)
	rows := m.instList.view(func(i int, cursor bool) string {
		inst := m.instances[i]
		mark := "  "
		if sel != nil && inst.ID == sel.ID {
			mark = s.accent.Render("▶ ")
		}
		name := lipgloss.NewStyle().Width(nameW + 2).Render(inst.Name)
		if cursor {
			name = s.tabActive.UnsetPadding().Width(nameW + 2).Render(inst.Name)
		}
		detail := s.muted.Render(versionLabel(inst, "") + " · " + lastPlayed(inst.LastPlayed))
		if inst.ID == m.runningID {
			detail += " " + s.ok.Render("● running")
		}
		pointer := "  "
		if cursor {
			pointer = s.key.Render("› ")
		}
		return pointer + mark + name + detail
	})
	lines = append(lines, rows...)

	switch m.inputMode {
	case inputRename:
		lines = append(lines, "", "  "+s.label.Render("Rename")+s.accent.Render("› ")+m.input.View())
	case inputDuplicate:
		lines = append(lines, "", "  "+s.label.Render("Copy as")+s.accent.Render("› ")+m.input.View())
	}
	if m.working != "" {
		lines = append(lines, "", "  "+m.spin.View()+" "+m.working)
	}
	if m.notice != "" {
		lines = append(lines, "", "  "+s.err.Width(max(m.width-4, 20)).Render(m.notice))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m model) wizardView() string {
	s := m.styles
	w := &m.wizard
	step := func(n int, title, value string) string {
		num := s.muted.Render(fmt.Sprintf("%d ", n+1))
		t := s.muted.Render(title)
		if w.step == n {
			num, t = s.key.Render(fmt.Sprintf("%d ", n+1)), s.value.Bold(true).Render(title)
		}
		return "  " + num + lipgloss.NewStyle().Width(10).Render(t) + s.value.Render(value)
	}
	versionVal := ""
	if w.step > stepVersion {
		versionVal = w.selectedVersion()
	}
	lines := []string{"", "  " + s.title.Render("New instance") + s.muted.Render("  runs "+app.DisplayName+" on Fabric"), "",
		step(stepVersion, "Version", versionVal),
		step(stepName, "Name", ""),
		"",
	}

	switch w.step {
	case stepVersion:
		filter := s.muted.Render("type to filter")
		if w.version.filter != "" {
			filter = s.accent.Render("filter: ") + w.version.filter
		}
		lines = append(lines, "    "+filter, "")
		if len(w.version.matches()) == 0 {
			lines = append(lines, "    "+s.muted.Render("no matching versions"))
		}
		lines = append(lines, w.version.view(func(i int, cursor bool) string {
			label := "Minecraft " + w.version.items[i]
			if m, err := client.Load(w.version.items[i]); err == nil {
				label += s.muted.Render(fmt.Sprintf("  client %s · Fabric %s", m.Version, m.FabricLoader))
			}
			if cursor {
				return "    " + s.key.Render("› ") + s.value.Bold(true).Render(label)
			}
			return "      " + label
		})...)
	case stepName:
		lines = append(lines, "    "+s.accent.Render("› ")+m.input.View())
	}
	if m.notice != "" {
		lines = append(lines, "", "  "+s.err.Width(max(m.width-4, 20)).Render(m.notice))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func lastPlayed(t time.Time) string {
	if t.IsZero() {
		return "never played"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "played just now"
	case d < time.Hour:
		return fmt.Sprintf("played %d min ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("played %d h ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("played %d days ago", int(d.Hours()/24))
	}
	return "played " + t.Format("2 Jan 2006")
}
