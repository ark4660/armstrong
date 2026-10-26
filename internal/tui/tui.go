// Package tui is the launcher's terminal interface. It only renders and
// routes input; installing and launching happen in internal/game, driven
// from a background goroutine that reports back with messages.
package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ark4660/armstrong/internal/app"
	"github.com/ark4660/armstrong/internal/auth"
	"github.com/ark4660/armstrong/internal/client"
	"github.com/ark4660/armstrong/internal/download"
	"github.com/ark4660/armstrong/internal/game"
	"github.com/ark4660/armstrong/internal/instance"
	"github.com/ark4660/armstrong/internal/update"
)

type screen int

const (
	screenHome screen = iota
	screenInstances
	screenMods
	screenAccounts
	screenConsole
	screenCount
)

var screenNames = [screenCount]string{"Home", "Instances", "Mods", "Accounts", "Console"}

type state int

const (
	stateIdle       state = iota
	stateInstalling       // downloading / verifying files
	stateStarting         // game process running, not yet at the title screen
	stateRunning          // game reported it finished starting
	stateStopping         // asked the game to quit
)

// inputMode is what the shared text input is editing.
type inputMode int

const (
	inputNone inputMode = iota
	inputOffline
	inputWizardName
	inputRename
	inputDuplicate
)

const memoryStep, memoryMin, memoryMax = 512, 1024, 32768

// Run starts the TUI and blocks until the user quits.
func Run(paths app.Paths) error {
	settings, err := paths.LoadSettings()
	var notice string
	if err != nil {
		notice = "Couldn't read settings, using defaults: " + err.Error()
	}
	changed, err := instance.Migrate(paths, &settings)
	if err != nil {
		notice = "Couldn't migrate old profiles: " + err.Error()
	}
	accounts, accChanged, err := auth.Open(paths, &settings)
	if err != nil {
		return fmt.Errorf("accounts: %w", err)
	}
	if changed || accChanged {
		if err := paths.SaveSettings(settings); err != nil {
			notice = "Couldn't save settings: " + err.Error()
		}
	}
	m := newModel(paths, settings, accounts)
	if notice != "" {
		m.notice = notice
	}
	p := tea.NewProgram(m)
	m.send.fn = p.Send
	_, err = p.Run()
	return err
}

// sender lets background work reach the program. It's a pointer so every
// copy of the model shares it.
type sender struct{ fn func(tea.Msg) }

type model struct {
	paths    app.Paths
	settings app.Settings
	store    instance.Store
	accounts *auth.Accounts
	send     *sender

	width, height int
	styles        styles
	screen        screen

	instances     []*instance.Instance
	instList      picker
	wizard        wizard
	confirmDelete bool
	working       string // a long instance operation in progress (copying, deleting)

	state         state
	step          string
	prog          download.Progress
	bar           progress.Model
	spin          spinner.Model
	cancel        context.CancelFunc
	game          *game.Game
	runningID     string        // instance being played
	loaderVersion string        // resolved loader version of the current/last run
	layer         *client.Layer // client layer of the current/last run
	disabledNote  string        // player mods disabled at the last launch
	lastPlayedID  string        // instance disabledNote is about
	startedIn     time.Duration
	stopped       bool // the user asked the game to stop

	accList       picker
	login         login
	mods          modsState
	confirmRemove bool

	input     textinput.Model
	inputMode inputMode

	console console

	release  *update.Release // newer launcher found at startup, if any
	updating bool

	notice      string // last error or exit summary
	confirmQuit bool
	quitting    bool
}

func newModel(paths app.Paths, s app.Settings, accounts *auth.Accounts) model {
	ti := textinput.New()
	ti.Prompt = ""
	m := model{
		paths:    paths,
		settings: s,
		store:    instance.Store{Root: paths.Instances()},
		accounts: accounts,
		accList:  picker{height: 8},
		mods:     newModsState(),
		send:     &sender{},
		styles:   newStyles(true),
		bar:      progress.New(progress.WithDefaultBlend(), progress.WithoutPercentage()),
		spin:     spinner.New(spinner.WithSpinner(spinner.Dot)),
		input:    ti,
		instList: picker{height: 10},
		console:  newConsole(),
	}
	m.reload()
	m.reloadAccounts()
	if len(m.accList.items) == 0 {
		m.screen = screenAccounts // first run: an account comes first
	}
	return m
}

// Messages from the background update check.
type (
	updateFoundMsg struct{ release *update.Release }
	updateDoneMsg  struct {
		version string
		err     error
	}
)

// checkUpdates fetches the signed release index, refreshes the client
// manifests and reports a newer launcher. Offline or failing: silent.
func (m model) checkUpdates() tea.Cmd {
	if update.Disabled() {
		return nil
	}
	root := m.paths.Root
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		dl := download.New(app.UserAgent())
		r, err := update.Fetch(ctx, dl)
		if err != nil {
			return nil
		}
		update.RefreshClient(ctx, dl, root, r)
		if !r.Newer() {
			return nil
		}
		return updateFoundMsg{r}
	}
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{tea.RequestBackgroundColor, m.checkUpdates()}
	if len(m.accList.items) > 0 && len(m.instances) == 0 {
		// First instance: go straight to the create wizard.
		cmds = append(cmds, func() tea.Msg { return openWizardMsg{} })
	}
	return tea.Batch(cmds...)
}

func (m model) busy() bool { return m.state != stateIdle }

// selected is the instance Play launches, or nil when there are none.
func (m *model) selected() *instance.Instance {
	for _, i := range m.instances {
		if i.ID == m.settings.Instance {
			return i
		}
	}
	if len(m.instances) > 0 {
		return m.instances[0]
	}
	return nil
}

// reload re-reads instances from disk, keeping the selection.
func (m *model) reload() {
	list, err := m.store.List()
	if err != nil {
		m.notice = "Couldn't list instances: " + err.Error()
	}
	m.instances = list
	names := make([]string, len(list))
	cursor := 0
	for i, inst := range list {
		names[i] = inst.Name
		if inst.ID == m.settings.Instance {
			cursor = i
		}
	}
	m.instList.setItems(names)
	m.instList.selectIndex(cursor)
}

func (m *model) selectInstance(id string) {
	if m.settings.Instance != id {
		m.settings.Instance = id
		m.saveSettings()
	}
}

func (m *model) saveSettings() {
	if err := m.paths.SaveSettings(m.settings); err != nil {
		m.notice = "Couldn't save settings: " + err.Error()
	}
}

func (m *model) saveInstance(inst *instance.Instance) {
	if err := m.store.Save(inst); err != nil {
		m.notice = "Couldn't save instance: " + err.Error()
	}
}

// editInput opens the shared text input for mode.
func (m *model) editInput(mode inputMode, value, placeholder string, limit int) {
	m.inputMode = mode
	m.input.Placeholder = placeholder
	m.input.CharLimit = limit
	m.input.SetWidth(limit)
	m.input.SetValue(value)
	m.input.CursorEnd()
	m.input.Focus() // the blink command is unused; the virtual cursor still shows
}

func (m *model) closeInput() {
	m.inputMode = inputNone
	m.input.Blur()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		m.styles = newStyles(msg.IsDark())
		m.console.restyle(m.styles)
		return m, nil

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.bar.SetWidth(min(max(m.width-30, 10), 60))
		m.console.resize(m.width, m.height-3) // header 2 lines, footer 1
		m.instList.height = max(m.height-8, 3)
		m.wizard.version.height = max(m.height-10, 3)
		m.instList.clamp()
		m.wizard.version.clamp()
		m.resizeMods()
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case openWizardMsg, duplicatedMsg, deletedMsg:
		return m.handleInstanceMsg(msg)

	case updateFoundMsg:
		m.release = msg.release
		return m, nil

	case updateDoneMsg:
		m.updating = false
		if msg.err != nil {
			m.notice = "Update failed: " + msg.err.Error()
		} else {
			m.release = nil
			m.notice = "Updated to " + msg.version + ". Restart Armstrong to use it."
		}
		return m, nil

	case loginCodeMsg, loginStepMsg, loginDoneMsg, loginFailedMsg:
		return m.handleLoginMsg(msg)

	case modsOpenedMsg, modsScannedMsg, modsSearchMsg, modsDetailMsg, modsCategoriesMsg,
		modsPlanMsg, modsRemovalMsg, modsUpdatesMsg, modsDoneMsg:
		return m.handleModsMsg(msg)

	case stepMsg:
		m.step = msg.label
		m.prog = download.Progress{}
		return m, nil

	case progressMsg:
		m.prog = msg.p
		return m, nil

	case clientMsg:
		m.layer = msg.layer
		m.disabledNote = ""
		if n := len(msg.disabled); n > 0 {
			names := make([]string, n)
			for i, d := range msg.disabled {
				names[i] = d.ClientMod
			}
			m.disabledNote = fmt.Sprintf("Disabled your own %s: the client includes its own build (renamed to .disabled in your mods folder).", strings.Join(names, ", "))
		}
		return m, nil

	case failedMsg:
		m.state, m.cancel, m.runningID = stateIdle, nil, ""
		if m.quitting {
			return m, tea.Quit
		}
		switch {
		case errors.Is(msg.err, context.Canceled):
		case errors.Is(msg.err, auth.ErrReauth):
			m.notice = msg.err.Error() + " (Accounts → m)"
		default:
			m.notice = "Launch failed: " + msg.err.Error()
		}
		return m, nil

	case startedMsg:
		m.state, m.game, m.loaderVersion = stateStarting, msg.game, msg.loaderVersion
		m.console.clear()
		m.console.append(m.styles, []string{fmt.Sprintf("── %s started (pid %d) ──", m.runningLabel(), msg.game.Cmd.Process.Pid)})
		if m.quitting { // the user quit while we were installing
			m.stopGame()
		}
		return m, nil // the spinner is already ticking since startPlay

	case logMsg:
		m.console.append(m.styles, msg.lines)
		return m, nil

	case readyMsg:
		if m.state == stateStarting {
			m.state = stateRunning
		}
		m.startedIn = msg.after
		return m, nil

	case exitedMsg:
		m.notice = exitSummary(msg, m.stopped)
		m.console.append(m.styles, []string{"── game exited ──"})
		m.state, m.game, m.cancel, m.stopped, m.runningID = stateIdle, nil, nil, false, ""
		if m.quitting {
			return m, tea.Quit
		}
		return m, nil

	case spinner.TickMsg:
		ms := &m.mods
		modsBusy := ms.opening || ms.searching || ms.checking || ms.working != "" || m.updating
		if m.state == stateIdle && m.working == "" && !m.login.active && !modsBusy {
			return m, nil
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	}

	if m.inputMode != inputNone {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

// runningLabel names what's being played, for the console banner.
func (m *model) runningLabel() string {
	for _, i := range m.instances {
		if i.ID == m.runningID {
			return fmt.Sprintf("%s (%s)", i.Name, versionLabel(i, m.loaderVersion))
		}
	}
	return "game"
}

func (m model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	if k == "ctrl+c" {
		return m.quit()
	}
	if m.inputMode != inputNone {
		return m.handleInputKey(msg)
	}
	if m.confirmDelete {
		m.confirmDelete = false
		if k == "y" {
			return m.deleteSelected()
		}
		return m, nil
	}
	if m.confirmRemove {
		m.confirmRemove = false
		if k == "y" {
			return m.removeAccount()
		}
		return m, nil
	}
	if m.login.active {
		return m.handleLoginKey(k)
	}
	if m.screen == screenInstances && m.wizard.active {
		return m.handleWizardKey(k)
	}
	// The mods search box and pending confirmations take every key.
	if m.screen == screenMods && (m.mods.search.Focused() || m.mods.plan != nil || m.mods.removal != nil) {
		return m.handleModsKey(msg)
	}
	if k != "q" {
		m.confirmQuit = false
	}
	switch k {
	case "q":
		return m.quit()
	case "tab":
		return m.switchScreen((m.screen + 1) % screenCount)
	case "shift+tab":
		return m.switchScreen((m.screen + screenCount - 1) % screenCount)
	case "s":
		// On the Mods tab s sorts search results; stop the game from another tab.
		if m.screen != screenMods {
			if m.game != nil {
				m.stopGame()
			}
			return m, nil
		}
	}
	switch m.screen {
	case screenConsole:
		return m, m.console.handleKey(msg)
	case screenInstances:
		return m.handleInstancesKey(k)
	case screenAccounts:
		return m.handleAccountsKey(k)
	case screenMods:
		return m.handleModsKey(msg)
	}
	return m.handleHomeKey(k)
}

func (m model) handleHomeKey(k string) (tea.Model, tea.Cmd) {
	inst := m.selected()
	switch k {
	case "enter", "space":
		return m.startPlay()
	case "i":
		m.screen = screenInstances
	case "m":
		return m.switchScreen(screenMods)
	case "n":
		return m.openWizard()
	case "a":
		m.screen = screenAccounts
	case "U":
		if m.release != nil && !m.updating {
			m.updating = true
			r := m.release
			return m, tea.Batch(func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				defer cancel()
				return updateDoneMsg{version: r.Version, err: update.ApplyLauncher(ctx, download.New(app.UserAgent()), r)}
			}, m.spin.Tick)
		}
	case "+", "=":
		if inst != nil {
			inst.MemoryMB = min(inst.MemoryMB+memoryStep, memoryMax)
			m.saveInstance(inst)
		}
	case "-", "_":
		if inst != nil {
			inst.MemoryMB = max(inst.MemoryMB-memoryStep, memoryMin)
			m.saveInstance(inst)
		}
	}
	return m, nil
}

func (m model) handleInputKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		switch {
		case m.inputMode == inputWizardName:
			m.closeInput()
			m.wizard.step = stepVersion
			return m, nil
		}
		m.closeInput()
		m.notice = ""
		return m, nil
	case "enter":
		return m.submitInput(strings.TrimSpace(m.input.Value()))
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) submitInput(value string) (tea.Model, tea.Cmd) {
	switch m.inputMode {
	case inputOffline:
		if _, err := m.accounts.AddOffline(value); err != nil {
			m.notice = err.Error()
			return m, nil
		}
		m.notice = ""
		m.closeInput()
		return m.accountAdded()
	case inputWizardName:
		return m.finishWizard(value)
	case inputRename:
		inst := m.cursorInstance()
		if inst == nil {
			m.closeInput()
			return m, nil
		}
		if err := m.store.Rename(inst, value); err != nil {
			m.notice = err.Error()
			return m, nil
		}
		m.closeInput()
		m.notice = ""
		m.reload()
		return m, nil
	case inputDuplicate:
		return m.duplicateSelected(value)
	}
	m.closeInput()
	return m, nil
}

func (m model) startPlay() (tea.Model, tea.Cmd) {
	inst := m.selected()
	acc, hasAccount := m.accounts.Active()
	if m.busy() || !hasAccount || inst == nil {
		if !hasAccount {
			m.notice = "Add an account first (Accounts tab)."
		}
		return m, nil
	}
	inst.LastPlayed = time.Now()
	m.saveInstance(inst)
	req := playRequest{spec: inst.Spec(), gameDir: inst.GameDir(), memoryMB: inst.MemoryMB, account: acc, accounts: m.accounts, addons: inst.Addons}

	ctx, cancel := context.WithCancel(context.Background())
	m.state, m.cancel, m.notice, m.step = stateInstalling, cancel, "", "Preparing"
	m.runningID, m.loaderVersion, m.lastPlayedID, m.disabledNote = inst.ID, "", inst.ID, ""
	m.prog = download.Progress{}
	go play(ctx, m.send.fn, m.paths, req)
	return m, m.spin.Tick
}

func (m *model) stopGame() {
	if m.game == nil || m.state == stateStopping {
		return
	}
	m.state, m.stopped = stateStopping, true
	m.game.Stop(10 * time.Second)
}

// quit leaves at once when idle. While the game is running the first press
// asks for confirmation and the second stops the game, then quits once it exits.
func (m model) quit() (tea.Model, tea.Cmd) {
	switch {
	case !m.busy():
		return m, tea.Quit
	case !m.confirmQuit && !m.quitting:
		m.confirmQuit = true
		return m, nil
	}
	m.confirmQuit, m.quitting = false, true
	if m.cancel != nil {
		m.cancel()
	}
	m.stopGame()
	return m, nil
}

func (m model) View() tea.View {
	var body string
	switch m.screen {
	case screenConsole:
		body = m.console.view()
	case screenInstances:
		body = m.instancesView()
	case screenAccounts:
		body = m.accountsView()
	case screenMods:
		body = m.modsView()
	default:
		body = m.homeView()
	}
	content := lipgloss.JoinVertical(lipgloss.Left, m.headerView(), body)
	content = lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.PlaceVertical(max(m.height-1, 0), lipgloss.Top, content),
		m.footerView())

	v := tea.NewView(content)
	v.AltScreen = true
	v.WindowTitle = app.DisplayName
	switch m.state {
	case stateInstalling:
		if m.prog.Total > 0 {
			v.ProgressBar = tea.NewProgressBar(tea.ProgressBarDefault, m.prog.Done*100/m.prog.Total)
		} else {
			v.ProgressBar = tea.NewProgressBar(tea.ProgressBarIndeterminate, 0)
		}
	case stateStarting:
		v.ProgressBar = tea.NewProgressBar(tea.ProgressBarIndeterminate, 0)
	}
	return v
}

func (m model) headerView() string {
	s := m.styles
	left := s.title.Render(app.DisplayName) + " " + s.muted.Render(app.Version)
	if m.updating {
		left += "  " + m.spin.View() + s.warn.Render(" updating…")
	} else if m.release != nil {
		left += "  " + s.warn.Render("update "+m.release.Version+": press U")
	}
	var right string
	for sc := range screenCount {
		if m.screen == sc {
			right += s.tabActive.Render(screenNames[sc])
		} else {
			right += s.tabInactive.Render(screenNames[sc])
		}
	}
	gap := max(m.width-lipgloss.Width(left)-lipgloss.Width(right)-2, 1)
	line := " " + left + strings.Repeat(" ", gap) + right
	return line + "\n" + s.rule.Render(strings.Repeat("─", max(m.width, 0)))
}

type keyHelp struct{ key, desc string }

func (m model) footerView() string {
	s := m.styles
	if m.confirmQuit {
		return " " + s.warn.Render("The game is running. Press q again to stop it and quit, any other key to stay.")
	}
	if m.confirmDelete {
		if inst := m.cursorInstance(); inst != nil {
			return lipgloss.NewStyle().MaxWidth(max(m.width, 1)).Render(" " + s.err.Render(fmt.Sprintf("Delete %q with its worlds, mods and config? y: delete · other key: keep", inst.Name)))
		}
	}
	if m.confirmRemove {
		if acc, ok := m.cursorAccount(); ok {
			return lipgloss.NewStyle().MaxWidth(max(m.width, 1)).Render(" " + s.err.Render(fmt.Sprintf("Sign out and remove %s? y: remove · other key: keep", acc.Label())))
		}
	}
	var keys []keyHelp
	switch {
	case m.inputMode != inputNone:
		return m.renderKeys([]keyHelp{{"enter", "save"}, {"esc", "back"}, {"ctrl+c", "quit"}})
	case m.login.active:
		return m.renderKeys(m.login.keys())
	case m.screen == screenAccounts:
		keys = []keyHelp{{"enter", "use"}, {"m", "Microsoft sign-in"}, {"o", "offline"}, {"d", "remove"}}
	case m.screen == screenMods:
		keys = m.modsKeys()
		if m.mods.plan != nil || m.mods.removal != nil || m.mods.search.Focused() {
			return m.renderKeys(keys)
		}
	case m.screen == screenInstances && m.wizard.active:
		return m.renderKeys(m.wizard.keys())
	case m.screen == screenConsole:
		keys = []keyHelp{{"↑↓", "scroll"}, {"f", "follow"}, {"g/G", "top/end"}, {"y", "copy"}}
	case m.screen == screenInstances:
		keys = []keyHelp{{"enter", "select"}, {"n", "new"}, {"r", "rename"}, {"c", "copy"}, {"d", "delete"}}
	default:
		keys = []keyHelp{{"enter", "play"}, {"i", "instances"}, {"m", "mods"}, {"a", "account"}, {"+/-", "memory"}}
	}
	if m.game != nil && m.screen != screenMods {
		keys = append(keys, keyHelp{"s", "stop game"})
	}
	keys = append(keys, keyHelp{"tab", "switch"}, keyHelp{"q", "quit"})
	return m.renderKeys(keys)
}

func (m model) renderKeys(keys []keyHelp) string {
	s := m.styles
	parts := make([]string, len(keys))
	for i, kh := range keys {
		parts[i] = s.key.Render(kh.key) + " " + s.help.Render(kh.desc)
	}
	return lipgloss.NewStyle().MaxWidth(max(m.width, 1)).Render(" " + strings.Join(parts, s.muted.Render(" · ")))
}

// versionLabel is "Minecraft 26.3 · Fabric 0.20.1"-style text for an
// instance. Without an explicit loader version it shows the client's pinned one.
func versionLabel(i *instance.Instance, resolved string) string {
	if i.Loader == instance.LoaderVanilla {
		return "Minecraft " + i.Minecraft + " · vanilla"
	}
	lv := i.LoaderVersion
	if resolved != "" {
		lv = resolved
	}
	if lv == "" {
		if m, err := client.Load(i.Minecraft); err == nil {
			lv = m.FabricLoader
		}
	}
	return fmt.Sprintf("Minecraft %s · %s %s", i.Minecraft, loaderTitle(i.Loader), lv)
}

func loaderTitle(l string) string {
	switch l {
	case instance.LoaderFabric:
		return "Fabric"
	case instance.LoaderVanilla:
		return "Vanilla"
	}
	return l
}

// switchScreen changes tabs, loading the Mods tab for the selected instance.
func (m model) switchScreen(sc screen) (tea.Model, tea.Cmd) {
	m.screen = sc
	if sc == screenMods {
		return m, m.openMods()
	}
	return m, nil
}
