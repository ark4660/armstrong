package tui

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ark4660/armstrong/internal/app"
	"github.com/ark4660/armstrong/internal/client"
	"github.com/ark4660/armstrong/internal/download"
	"github.com/ark4660/armstrong/internal/game"
	"github.com/ark4660/armstrong/internal/instance"
	"github.com/ark4660/armstrong/internal/modrinth"
	"github.com/ark4660/armstrong/internal/mods"
)

const (
	modsInstalled = iota
	modsBrowse
)

var sortIndexes = []struct{ id, label string }{
	{"relevance", "Relevance"}, {"downloads", "Downloads"}, {"follows", "Follows"},
	{"newest", "Newest"}, {"updated", "Recently updated"},
}

const searchPage = 30

// Messages from mods work running in the background.
type (
	modsOpenedMsg struct {
		instID    string
		mgr       *mods.Manager
		installed []*mods.Installed
		err       error
	}
	modsScannedMsg struct {
		installed []*mods.Installed
		err       error
	}
	modsSearchMsg struct {
		seq    int
		offset int
		result *modrinth.SearchResult
		err    error
	}
	modsDetailMsg struct {
		project string
		details *modrinth.ProjectDetails
	}
	modsCategoriesMsg struct{ categories []string }
	modsPlanMsg       struct {
		seq   int
		title string
		plan  *mods.Plan
		err   error
	}
	modsProgressMsg struct {
		seq  int
		text string
	}
	modsVersionsMsg struct {
		project  string
		versions []mods.Version
		err      error
	}
	// modsVersionCheckMsg is one checked version, or (done) the end of a check.
	modsVersionCheckMsg struct {
		seq   int
		id    string
		check versionCheck
		done  bool
		err   error
	}
	modsRemovalMsg struct {
		removal *mods.Removal
		err     error
	}
	modsUpdatesMsg struct {
		updates []mods.Update
		err     error
	}
	modsDoneMsg struct {
		text string
		err  error
	}
)

// modRow is one line of the Installed view.
type modRow struct {
	kind      int // rowInstalled, rowAddon, rowClient, rowHeader
	installed *mods.Installed
	addon     client.Mod
	label     string
}

const (
	rowHeader = iota
	rowInstalled
	rowAddon
	rowClient
)

type modsState struct {
	instID  string
	mgr     *mods.Manager
	opening bool
	err     string
	view    int

	installed []*mods.Installed
	rows      []modRow
	list      picker
	updates   map[string]mods.Update // by installed key
	checking  bool

	search     textinput.Model
	searching  bool
	searchSeq  int
	results    []modrinth.SearchHit
	total      int
	sortIdx    int
	categories []string
	catIdx     int // 0: any
	blist      picker
	details    map[string]*modrinth.ProjectDetails
	showDetail bool // narrow terminals: details replace the list

	versions *versionsState // a project's version list, over Browse

	plan      *mods.Plan
	planTitle string
	removal   *mods.Removal
	working   string
	progress  string             // what the running check is looking at
	cancel    context.CancelFunc // stops the running check; nil while a change is written
	checkSeq  int                // results of older or cancelled checks are dropped
	notice    string
}

// versionsState is every version of one project for the instance (v on a
// search result), and what checking them found.
type versionsState struct {
	project, title string
	loading        bool
	list           []mods.Version
	checks         map[string]versionCheck // by version id
	picker         picker
}

type versionCheck struct {
	fits bool
	text string // why it doesn't fit, or what it would add
}

// checkBatch is how many versions C checks at once: each check downloads the
// version's jar, so "all of them" could mean gigabytes for a long-lived mod.
const checkBatch = 10

func newModsState() modsState {
	ti := textinput.New()
	ti.Prompt = "/ "
	ti.Placeholder = "search Modrinth"
	ti.CharLimit = 80
	return modsState{
		list:    picker{height: 10},
		blist:   picker{height: 10},
		search:  ti,
		details: map[string]*modrinth.ProjectDetails{},
		updates: map[string]mods.Update{},
	}
}

// openMods (re)loads the Mods tab for the selected instance when needed.
func (m *model) openMods() tea.Cmd {
	inst := m.selected()
	if inst == nil || inst.Loader != instance.LoaderFabric {
		m.mods.instID, m.mods.mgr = "", nil
		return nil
	}
	if m.mods.instID == inst.ID && (m.mods.mgr != nil || m.mods.opening) {
		return nil
	}
	saved := m.mods
	if saved.cancel != nil {
		saved.cancel()
	}
	m.mods = newModsState()
	m.mods.checkSeq = saved.checkSeq
	m.mods.search.SetValue(saved.search.Value())
	m.mods.categories, m.mods.view, m.mods.notice = saved.categories, saved.view, saved.notice
	m.mods.instID, m.mods.opening = inst.ID, true
	m.resizeMods()
	paths, instCopy := m.paths, *inst
	cmds := []tea.Cmd{func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		mgr, err := mods.Open(ctx, paths, &instCopy, download.New(app.UserAgent()))
		if err != nil {
			return modsOpenedMsg{instID: instCopy.ID, err: err}
		}
		mgr.Identify(ctx) // best effort: unknown files stay "local file"
		installed, err := mgr.Scan()
		return modsOpenedMsg{instID: instCopy.ID, mgr: mgr, installed: installed, err: err}
	}, m.spin.Tick}
	if m.mods.categories == nil {
		cmds = append(cmds, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cats, _ := (&modrinth.Client{DL: download.New(app.UserAgent())}).Categories(ctx)
			return modsCategoriesMsg{cats}
		})
	}
	if m.mods.view == modsBrowse {
		cmds = append(cmds, m.runSearch(0))
	}
	return tea.Batch(cmds...)
}

func (m *model) resizeMods() {
	// header 2, tab title 3, footer 1, and room for a status line and notice below
	h := max(m.height-13, 3)
	m.mods.list.height = h
	m.mods.blist.height = max(h-2, 3)
	m.mods.list.clamp()
	m.mods.blist.clamp()
	if v := m.mods.versions; v != nil {
		v.picker.height = m.mods.blist.height
		v.picker.clamp()
	}
	m.mods.search.SetWidth(max(min(m.width/2, 50), 20))
}

func (m *model) modsLocked() bool { return m.mods.instID != "" && m.mods.instID == m.runningID }

func (m *model) rebuildModRows() {
	s := &m.mods
	var rows []modRow
	rows = append(rows, modRow{kind: rowHeader, label: fmt.Sprintf("Your mods (%d)", len(s.installed))})
	for _, i := range s.installed {
		rows = append(rows, modRow{kind: rowInstalled, installed: i})
	}
	if s.mgr != nil && s.mgr.Layer != nil {
		var addons []client.Mod
		for _, mod := range s.mgr.Layer.Manifest.Mods {
			if mod.Optional && !slices.ContainsFunc(s.mgr.Layer.Manifest.Mods, func(o client.Mod) bool { return slices.Contains(o.Requires, mod.ID) && o.Optional }) {
				addons = append(addons, mod) // dependencies of add-ons (Cloth Config) come along on their own
			}
		}
		rows = append(rows, modRow{kind: rowHeader, label: "Client add-ons"})
		for _, a := range addons {
			rows = append(rows, modRow{kind: rowAddon, addon: a})
		}
		rows = append(rows, modRow{kind: rowHeader, label: "Armstrong client (always on)"})
		for _, c := range s.mgr.Layer.Mods {
			if !c.Optional {
				rows = append(rows, modRow{kind: rowClient, addon: c})
			}
		}
	}
	s.rows = rows
	labels := make([]string, len(rows))
	for i := range rows {
		labels[i] = fmt.Sprint(i)
	}
	cursor := s.list.cursor
	s.list.setItems(labels)
	s.list.cursor = cursor
	if len(rows) > 1 && cursor == 0 {
		s.list.cursor = 1 // skip the first header
	}
	s.list.clamp()
}

func (m *model) cursorRow() *modRow {
	if i := m.mods.list.selected(); i >= 0 && i < len(m.mods.rows) {
		return &m.mods.rows[i]
	}
	return nil
}

func (m *model) rescanMods() tea.Cmd {
	mgr := m.mods.mgr
	if mgr == nil {
		return nil
	}
	return func() tea.Msg {
		installed, err := mgr.Scan()
		return modsScannedMsg{installed, err}
	}
}

func (m *model) runSearch(offset int) tea.Cmd {
	inst := m.selected()
	if inst == nil {
		return nil
	}
	m.mods.searchSeq++
	m.mods.searching = true
	q := modrinth.SearchQuery{
		Text: m.mods.search.Value(), Minecraft: inst.Minecraft, Loader: game.ModrinthLoader(inst.Minecraft),
		Index: sortIndexes[m.mods.sortIdx].id, Offset: offset, Limit: searchPage,
	}
	if m.mods.catIdx > 0 && m.mods.catIdx <= len(m.mods.categories) {
		q.Category = m.mods.categories[m.mods.catIdx-1]
	}
	seq := m.mods.searchSeq
	return tea.Batch(func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		r, err := (&modrinth.Client{DL: download.New(app.UserAgent())}).Search(ctx, q)
		return modsSearchMsg{seq: seq, offset: offset, result: r, err: err}
	}, m.spin.Tick)
}

func (m *model) loadDetail() tea.Cmd {
	hit := m.cursorHit()
	if hit == nil {
		return nil
	}
	if _, ok := m.mods.details[hit.ProjectID]; ok {
		return nil
	}
	m.mods.details[hit.ProjectID] = nil // loading
	id := hit.ProjectID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		d, _ := (&modrinth.Client{DL: download.New(app.UserAgent())}).ProjectDetails(ctx, id)
		return modsDetailMsg{project: id, details: d}
	}
}

func (m *model) cursorHit() *modrinth.SearchHit {
	if i := m.mods.blist.selected(); i >= 0 && i < len(m.mods.results) {
		return &m.mods.results[i]
	}
	return nil
}

func (m model) handleModsMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	s := &m.mods
	switch msg := msg.(type) {
	case modsOpenedMsg:
		if msg.instID != s.instID {
			return m, nil
		}
		s.opening = false
		if msg.err != nil {
			s.err = msg.err.Error()
			return m, nil
		}
		s.mgr, s.installed, s.err = msg.mgr, msg.installed, ""
		m.rebuildModRows()
	case modsScannedMsg:
		if msg.err != nil {
			s.notice = msg.err.Error()
		} else {
			s.installed = msg.installed
			m.rebuildModRows()
		}
	case modsCategoriesMsg:
		s.categories = msg.categories
	case modsSearchMsg:
		if msg.seq != s.searchSeq {
			return m, nil // an older search
		}
		s.searching = false
		if msg.err != nil {
			s.notice = "Search failed: " + msg.err.Error()
			return m, nil
		}
		if msg.offset == 0 {
			s.results = nil
			s.blist.selectIndex(0)
		}
		s.results = append(s.results, msg.result.Hits...)
		s.total = msg.result.TotalHits
		names := make([]string, len(s.results))
		for i, h := range s.results {
			names[i] = h.Title
		}
		cursor := s.blist.cursor
		s.blist.setItems(names)
		s.blist.cursor = cursor
		s.blist.clamp()
		return m, m.loadDetail()
	case modsDetailMsg:
		s.details[msg.project] = msg.details
	case modsVersionsMsg:
		v := s.versions
		if v == nil || v.project != msg.project {
			return m, nil
		}
		v.loading = false
		if msg.err != nil {
			s.versions, s.notice = nil, "Couldn't list versions: "+msg.err.Error()
			return m, nil
		}
		v.list = msg.versions
		names := make([]string, len(v.list))
		for i, x := range v.list {
			names[i] = x.Number
		}
		v.picker.setItems(names)
	case modsVersionCheckMsg:
		if msg.seq != s.checkSeq {
			return m, nil
		}
		if !msg.done {
			if s.versions != nil {
				s.versions.checks[msg.id] = msg.check
			}
			return m, nil
		}
		s.working, s.progress, s.cancel = "", "", nil
		if msg.err != nil && !errors.Is(msg.err, context.Canceled) {
			s.notice = "Couldn't check: " + msg.err.Error()
		}
	case modsProgressMsg:
		if msg.seq == s.checkSeq && s.working != "" {
			s.progress = msg.text
		}
	case modsPlanMsg:
		if msg.seq != s.checkSeq {
			return m, nil // cancelled, or from an instance that's no longer open
		}
		s.working, s.progress, s.cancel = "", "", nil
		if msg.err != nil {
			s.notice = installError(msg.title, msg.err)
			return m, nil
		}
		s.plan, s.planTitle = msg.plan, msg.title
	case modsRemovalMsg:
		s.working = ""
		if msg.err != nil {
			s.notice = msg.err.Error()
			return m, nil
		}
		s.removal = msg.removal
	case modsUpdatesMsg:
		s.checking = false
		if msg.err != nil {
			s.notice = "Update check failed: " + msg.err.Error()
			return m, nil
		}
		s.updates = map[string]mods.Update{}
		for _, u := range msg.updates {
			s.updates[u.Installed.Key] = u
		}
		if len(msg.updates) == 0 {
			s.notice = "Everything is up to date."
		} else {
			s.notice = fmt.Sprintf("%d update(s) available: press u on a mod to update it.", len(msg.updates))
		}
	case modsDoneMsg:
		s.working = ""
		if msg.err != nil {
			s.notice = msg.err.Error()
		} else {
			s.notice = msg.text
			s.versions = nil // back to the search after installing a chosen version
		}
		return m, m.rescanMods()
	}
	return m, nil
}

func installError(title string, err error) string {
	var inClient *mods.ErrInClient
	var noFit *mods.ErrNoFit
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Sprintf("Checking %s took too long. Modrinth may be slow right now: try again in a moment.", title)
	case errors.As(err, &inClient):
		return err.Error() + "."
	case errors.As(err, &noFit) && noFit.Tried > 0 && len(noFit.Problems) > 0:
		msg := fmt.Sprintf("Can't install %s: %s", title, noFit.Problems[0].Text)
		if noFit.Untried > 0 {
			msg += fmt.Sprintf(". %d older version(s) weren't tried: press v to check them.", noFit.Untried)
		}
		return msg
	case errors.As(err, &noFit):
		return fmt.Sprintf("%s has no version for this Minecraft version on Fabric.", title)
	}
	return fmt.Sprintf("Can't install %s: %s", title, err.Error())
}

func (m model) handleModsKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := &m.mods
	k := msg.String()

	// A pending install or removal waits for an answer.
	if s.plan != nil || s.removal != nil {
		switch k {
		case "y", "enter":
			return m.commitModsChange()
		case "n", "esc":
			s.plan, s.removal = nil, nil
		}
		return m, nil
	}
	if s.mgr == nil {
		return m, nil
	}
	// One thing at a time: while a check runs only Esc (cancel) works, and
	// nothing does while a change is being written.
	if s.working != "" {
		if k == "esc" && s.cancel != nil {
			s.cancel()
			s.checkSeq++
			s.working, s.progress, s.cancel = "", "", nil
			s.notice = "Cancelled."
		}
		return m, nil
	}

	if s.versions != nil {
		return m.handleVersionsKey(k)
	}
	if s.view == modsBrowse && s.search.Focused() {
		switch k {
		case "enter", "down":
			s.search.Blur()
			s.showDetail = false
			return m, m.runSearch(0)
		case "esc":
			s.search.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		s.search, cmd = s.search.Update(msg)
		return m, cmd
	}

	if s.view == modsInstalled {
		return m.handleInstalledKey(k)
	}
	return m.handleBrowseKey(k)
}

func (m model) handleInstalledKey(k string) (tea.Model, tea.Cmd) {
	s := &m.mods
	s.notice = "" // a new action replaces the last message
	row := m.cursorRow()
	switch k {
	case "b", "/":
		s.view = modsBrowse
		s.notice = ""
		if k == "/" || s.results == nil {
			s.search.Focus()
		}
		if s.results == nil {
			return m, m.runSearch(0)
		}
		return m, nil
	case "r":
		return m, m.rescanMods()
	case "U":
		s.checking, s.notice = true, ""
		mgr, installed := s.mgr, s.installed
		return m, tea.Batch(func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			ups, err := mgr.CheckUpdates(ctx, installed)
			return modsUpdatesMsg{ups, err}
		}, m.spin.Tick)
	}
	if row == nil {
		m.mods.list.handleKey(k)
		return m, nil
	}
	if m.modsLocked() && slices.Contains([]string{"space", "enter", "d", "u"}, k) {
		s.notice = "Stop the game before changing this instance's mods."
		return m, nil
	}
	switch k {
	case "space", "enter":
		switch row.kind {
		case rowInstalled:
			if err := s.mgr.SetEnabled(row.installed, !row.installed.Enabled); err != nil {
				s.notice = err.Error()
			}
			return m, m.rescanMods()
		case rowAddon:
			return m.toggleAddon(row.addon)
		case rowClient:
			s.notice = row.addon.Name + " is part of the Armstrong client and can't be turned off."
		}
	case "d", "delete":
		if row.kind == rowInstalled {
			s.working = "Checking what else would go"
			mgr, target := s.mgr, row.installed
			return m, tea.Batch(func() tea.Msg {
				r, err := mgr.PlanRemove(target)
				return modsRemovalMsg{r, err}
			}, m.spin.Tick)
		}
	case "u":
		if row.kind == rowInstalled {
			u, ok := s.updates[row.installed.Key]
			if !ok {
				s.notice = "No update known for " + row.installed.Name() + " (press U to check)."
				return m, nil
			}
			mgr, title := s.mgr, row.installed.Name()
			return m.startCheck("Checking "+title+" "+u.Version.Number, title, 2*time.Minute, func(ctx context.Context, progress func(string)) (*mods.Plan, error) {
				return mgr.PlanUpdate(ctx, u.Installed, u.Version.ID, progress)
			})
		}
	default:
		m.mods.list.handleKey(k)
		// Headers aren't selectable: step over them.
		if r := m.cursorRow(); r != nil && r.kind == rowHeader {
			if k == "up" || k == "k" {
				m.mods.list.handleKey("up")
			} else {
				m.mods.list.handleKey("down")
			}
		}
	}
	return m, nil
}

func (m model) toggleAddon(a client.Mod) (tea.Model, tea.Cmd) {
	inst := m.selected()
	if inst == nil {
		return m, nil
	}
	on := slices.ContainsFunc(inst.Addons, func(x string) bool { return x == a.ID || x == a.Slug })
	if on {
		inst.Addons = slices.DeleteFunc(inst.Addons, func(x string) bool { return x == a.ID || x == a.Slug })
	} else {
		inst.Addons = append(inst.Addons, a.ID)
	}
	m.saveInstance(inst)
	m.mods.notice = fmt.Sprintf("%s %s: takes effect next time you play.", a.Name, map[bool]string{true: "off", false: "on"}[on])
	m.mods.instID = "" // reopen with the new layer
	return m, m.openMods()
}

func (m model) handleBrowseKey(k string) (tea.Model, tea.Cmd) {
	s := &m.mods
	s.notice = "" // a new action replaces the last message
	switch k {
	case "esc":
		if s.showDetail {
			s.showDetail = false
		} else {
			s.view = modsInstalled
		}
		s.notice = ""
		return m, nil
	case "/":
		s.search.Focus()
		return m, nil
	case "s":
		s.sortIdx = (s.sortIdx + 1) % len(sortIndexes)
		return m, m.runSearch(0)
	case "c":
		s.catIdx = (s.catIdx + 1) % (len(s.categories) + 1)
		return m, m.runSearch(0)
	case "enter":
		if m.width < 110 {
			s.showDetail = !s.showDetail
		}
		return m, m.loadDetail()
	case "v":
		hit := m.cursorHit()
		if hit == nil {
			return m, nil
		}
		s.versions = &versionsState{project: hit.ProjectID, title: hit.Title, loading: true,
			checks: map[string]versionCheck{}, picker: picker{height: s.blist.height}}
		mgr, project := s.mgr, hit.ProjectID
		return m, tea.Batch(func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			vs, err := mgr.Versions(ctx, project)
			return modsVersionsMsg{project: project, versions: vs, err: err}
		}, m.spin.Tick)
	case "i":
		hit := m.cursorHit()
		if hit == nil {
			return m, nil
		}
		if m.modsLocked() {
			s.notice = "Stop the game before changing this instance's mods."
			return m, nil
		}
		mgr, project, title := s.mgr, hit.ProjectID, hit.Title
		return m.startCheck("Checking "+title, title, 3*time.Minute, func(ctx context.Context, progress func(string)) (*mods.Plan, error) {
			return mgr.PlanInstall(ctx, project, progress)
		})
	}
	before := s.blist.cursor
	s.blist.handleKey(k)
	var cmds []tea.Cmd
	if s.blist.cursor != before {
		cmds = append(cmds, m.loadDetail())
	}
	// Near the end: fetch the next page.
	if !s.searching && len(s.results) < s.total && s.blist.cursor >= len(s.results)-5 {
		cmds = append(cmds, m.runSearch(len(s.results)))
	}
	return m, tea.Batch(cmds...)
}

// startCheck works out an install or update plan in the background. Esc
// cancels it; progress and results of a cancelled check are dropped.
func (m model) startCheck(label, title string, timeout time.Duration, run func(ctx context.Context, progress func(string)) (*mods.Plan, error)) (tea.Model, tea.Cmd) {
	s := &m.mods
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	s.checkSeq++
	s.working, s.progress, s.notice, s.cancel = label, "", "", cancel
	seq, send := s.checkSeq, m.send.fn
	return m, tea.Batch(func() tea.Msg {
		defer cancel()
		plan, err := run(ctx, func(text string) { send(modsProgressMsg{seq: seq, text: text}) })
		return modsPlanMsg{seq: seq, title: title, plan: plan, err: err}
	}, m.spin.Tick)
}

// handleVersionsKey drives a project's version list: install a chosen
// version, or check whether versions would fit without installing them.
func (m model) handleVersionsKey(k string) (tea.Model, tea.Cmd) {
	s := &m.mods
	v := s.versions
	s.notice = ""
	if k == "esc" {
		s.versions = nil
		return m, nil
	}
	i := v.picker.selected()
	if v.loading || i < 0 {
		v.picker.handleKey(k)
		return m, nil
	}
	chosen := v.list[i]
	switch k {
	case "enter", "i":
		if m.modsLocked() {
			s.notice = "Stop the game before changing this instance's mods."
			return m, nil
		}
		mgr, project, title := s.mgr, v.project, v.title+" "+chosen.Number
		return m.startCheck("Checking "+title, title, 3*time.Minute, func(ctx context.Context, progress func(string)) (*mods.Plan, error) {
			return mgr.PlanInstallVersion(ctx, project, chosen.ID, progress)
		})
	case "c":
		return m.checkVersions([]mods.Version{chosen})
	case "C":
		return m.checkVersions(v.list[i:min(i+checkBatch, len(v.list))])
	}
	v.picker.handleKey(k)
	return m, nil
}

// checkVersions works out in the background whether each version would
// install, filling in the list as answers come. Esc stops it; what's
// checked by then stays.
func (m model) checkVersions(vs []mods.Version) (tea.Model, tea.Cmd) {
	s := &m.mods
	v := s.versions
	ids := make([]string, len(vs))
	for i, x := range vs {
		ids[i] = x.ID
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	s.checkSeq++
	s.working, s.progress, s.notice, s.cancel = "Checking "+v.title+" "+vs[0].Number, "", "", cancel
	if len(vs) > 1 {
		s.working = fmt.Sprintf("Checking %d versions of %s", len(vs), v.title)
		s.progress = fmt.Sprintf("1 of %d", len(vs))
	}
	seq, send, mgr, project := s.checkSeq, m.send.fn, s.mgr, v.project
	return m, tea.Batch(func() tea.Msg {
		defer cancel()
		done := 0
		err := mgr.CheckVersions(ctx, project, ids, func(id string, plan *mods.Plan, err error) {
			done++
			send(modsVersionCheckMsg{seq: seq, id: id, check: describeCheck(plan, err)})
			if done < len(ids) {
				send(modsProgressMsg{seq: seq, text: fmt.Sprintf("%d of %d", done+1, len(ids))})
			}
		})
		return modsVersionCheckMsg{seq: seq, done: true, err: err}
	}, m.spin.Tick)
}

// describeCheck sums up one version's check for its row in the list.
func describeCheck(plan *mods.Plan, err error) versionCheck {
	var noFit *mods.ErrNoFit
	switch {
	case err == nil:
		text := "fits"
		if n := len(plan.Steps) - 1; n > 0 {
			text += fmt.Sprintf(", adds %d dependenc%s", n, map[bool]string{true: "y", false: "ies"}[n == 1])
		}
		if len(plan.Warnings) > 0 {
			text += " (" + plan.Warnings[0].Text + ")"
		}
		return versionCheck{fits: true, text: text}
	case errors.As(err, &noFit) && len(noFit.Problems) > 0:
		return versionCheck{text: noFit.Problems[0].Text}
	}
	return versionCheck{text: err.Error()}
}

func (m model) versionsView() string {
	st := m.styles
	v := m.mods.versions
	inst := m.selected()
	head := "  " + st.title.Render("Versions of "+v.title)
	if inst != nil {
		head += st.muted.Render("  for Minecraft " + inst.Minecraft + " · Fabric")
	}
	if v.loading {
		return lipgloss.JoinVertical(lipgloss.Left, head, "", "  "+m.spin.View()+" Loading versions…")
	}
	if len(v.list) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left, head, "", "  "+st.muted.Render("No versions for this Minecraft version and loader."))
	}
	head += st.muted.Render(fmt.Sprintf("  (%d, newest first)", len(v.list)))
	w := max(m.width-4, 20)
	rows := v.picker.view(func(i int, cursor bool) string {
		x := v.list[i]
		pointer, number := "  ", x.Number
		if cursor {
			pointer, number = st.key.Render("› "), st.value.Bold(true).Render(x.Number)
		}
		date := x.Published
		if len(date) > 10 {
			date = date[:10]
		}
		info := st.muted.Render(fmt.Sprintf("  %-7s %s  %5.1f MB  ", x.Channel, date, float64(x.File.Size)/(1<<20)))
		status := st.muted.Render("not checked")
		if c, ok := v.checks[x.ID]; ok {
			if c.fits {
				status = st.ok.Render("✓ " + c.text)
			} else {
				status = st.err.Render("✗ " + c.text)
			}
		}
		return lipgloss.NewStyle().MaxWidth(w).Render(pointer + number + info + status)
	})
	hint := st.muted.Render(fmt.Sprintf("i on the search list installs the newest version that fits. Checking downloads each version's jar, so C checks %d at a time.", checkBatch))
	return lipgloss.JoinVertical(lipgloss.Left, head, "", strings.Join(rows, "\n"), "", "  "+lipgloss.NewStyle().Width(max(m.width-6, 20)).Render(hint))
}

func (m model) commitModsChange() (tea.Model, tea.Cmd) {
	s := &m.mods
	mgr := s.mgr
	if plan := s.plan; plan != nil {
		title := s.planTitle
		s.plan, s.working = nil, "Installing "+title
		return m, tea.Batch(func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			err := mgr.Commit(ctx, plan)
			text := fmt.Sprintf("Installed %s", title)
			if n := len(plan.Steps) - 1; n > 0 {
				text += fmt.Sprintf(" and %d dependenc%s", n, map[bool]string{true: "y", false: "ies"}[n == 1])
			}
			return modsDoneMsg{text: text + ".", err: err}
		}, m.spin.Tick)
	}
	if r := s.removal; r != nil {
		s.removal, s.working = nil, "Removing"
		return m, tea.Batch(func() tea.Msg {
			err := mgr.CommitRemove(r)
			return modsDoneMsg{text: fmt.Sprintf("Removed %d mod(s).", len(r.Remove)), err: err}
		}, m.spin.Tick)
	}
	return m, nil
}

func (m model) modsKeys() []keyHelp {
	s := &m.mods
	switch {
	case s.working != "" && s.cancel != nil:
		return []keyHelp{{"esc", "cancel"}}
	case s.working != "":
		return nil
	case s.plan != nil:
		return []keyHelp{{"y", "install"}, {"n", "cancel"}}
	case s.versions != nil && s.removal == nil:
		return []keyHelp{{"enter", "install this version"}, {"c", "check it"}, {"C", fmt.Sprintf("check %d", checkBatch)}, {"esc", "back"}}
	case s.removal != nil:
		return []keyHelp{{"y", "remove"}, {"n", "cancel"}}
	case s.view == modsBrowse && s.search.Focused():
		return []keyHelp{{"enter", "search"}, {"esc", "done"}}
	case s.view == modsBrowse:
		return []keyHelp{{"/", "search"}, {"i", "install"}, {"v", "versions"}, {"enter", "details"}, {"s", "sort"}, {"c", "category"}, {"esc", "installed"}}
	}
	return []keyHelp{{"space", "on/off"}, {"d", "remove"}, {"U", "check updates"}, {"u", "update"}, {"b", "browse"}}
}

func (m model) modsView() string {
	st := m.styles
	s := &m.mods
	inst := m.selected()
	lines := []string{""}
	switch {
	case inst == nil:
		return "\n  No instance selected."
	case inst.Loader != instance.LoaderFabric:
		return "\n  " + inst.Name + " is a vanilla instance: it can't have mods."
	case s.err != "":
		return "\n  " + st.err.Render("Couldn't load mods: "+s.err)
	case s.mgr == nil:
		return "\n  " + m.spin.View() + " Loading " + inst.Name + "'s mods…"
	}

	tab := func(label string, v int) string {
		if s.view == v {
			return st.tabActive.Render(label)
		}
		return st.tabInactive.Render(label)
	}
	head := "  " + st.title.Render(inst.Name) + st.muted.Render("  Minecraft "+inst.Minecraft+" · Fabric") + "   " + tab("Installed", modsInstalled) + tab("Browse Modrinth", modsBrowse)
	lines = append(lines, head, "")

	var body string
	switch {
	case s.plan != nil:
		body = m.planView()
	case s.removal != nil:
		body = m.removalView()
	case s.versions != nil:
		body = m.versionsView()
	case s.view == modsBrowse:
		body = m.browseView()
	default:
		body = m.installedView()
	}
	lines = append(lines, body)
	if s.working != "" || s.checking {
		w := s.working
		if w == "" {
			w = "Checking for updates"
		}
		line := "  " + m.spin.View() + " " + w + "…"
		if s.progress != "" {
			line += st.muted.Render("  " + s.progress)
		}
		lines = append(lines, "", line)
	}
	if s.notice != "" {
		lines = append(lines, "", "  "+st.warn.Width(max(m.width-4, 20)).Render(s.notice))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m model) installedView() string {
	st := m.styles
	s := &m.mods
	inst := m.selected()
	w := max(m.width-4, 20)
	rows := s.list.view(func(i int, cursor bool) string {
		r := s.rows[i]
		pointer := "  "
		if cursor {
			pointer = st.key.Render("› ")
		}
		switch r.kind {
		case rowHeader:
			return "  " + st.muted.Bold(true).Render(r.label)
		case rowInstalled:
			x := r.installed
			box := st.ok.Render("[✓]")
			if !x.Enabled {
				box = st.muted.Render("[ ]")
			}
			name := x.Name()
			if cursor {
				name = st.value.Bold(true).Render(name)
			}
			extra := st.muted.Render("  " + x.Version())
			if x.Source == nil {
				extra += st.muted.Render(" · local file")
			}
			if !x.Explicit && len(x.RequiredBy) > 0 {
				extra += st.muted.Render(" · dependency")
			}
			if !x.Enabled && len(x.Units) > 0 && s.mgr.Layer != nil {
				if _, inClient := s.mgr.Layer.IDs()[x.Units[0].ID]; inClient {
					extra += st.muted.Render(" · off: the client includes it")
				}
			}
			if _, ok := s.updates[x.Key]; ok {
				extra += st.accent.Render(" · update available")
			}
			if mods.HasErrors(x.Problems) {
				extra += st.err.Render(" · ! " + x.Problems[0].Text)
			}
			return lipgloss.NewStyle().MaxWidth(w).Render(pointer + box + " " + name + extra)
		case rowAddon:
			on := slices.ContainsFunc(inst.Addons, func(a string) bool { return a == r.addon.ID || a == r.addon.Slug })
			box := st.muted.Render("[ ]")
			if on {
				box = st.ok.Render("[✓]")
			}
			name := r.addon.Name
			if cursor {
				name = st.value.Bold(true).Render(name)
			}
			return pointer + box + " " + name + st.muted.Render("  "+r.addon.Version)
		case rowClient:
			name := r.addon.Name
			if cursor {
				name = st.value.Render(name)
			} else {
				name = st.muted.Render(name)
			}
			return pointer + st.muted.Render(" ■ ") + name + st.muted.Render("  "+r.addon.Version)
		}
		return ""
	})
	if len(s.installed) == 0 {
		rows = slices.Insert(rows, min(1, len(rows)), "    "+st.muted.Render("none yet: press b to browse Modrinth"))
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m model) browseView() string {
	st := m.styles
	s := &m.mods
	cat := "any"
	if s.catIdx > 0 && s.catIdx <= len(s.categories) {
		cat = s.categories[s.catIdx-1]
	}
	status := fmt.Sprintf("%d results · sort: %s · category: %s", s.total, sortIndexes[s.sortIdx].label, cat)
	if s.searching {
		status = m.spin.View() + " searching… · " + status
	}
	top := "  " + s.search.View() + "   " + st.muted.Render(status)

	wide := m.width >= 110
	listW := m.width - 4
	if wide {
		listW = m.width * 11 / 20
	}
	var list string
	if !wide && s.showDetail {
		list = m.detailView(m.width - 6)
	} else {
		rows := s.blist.view(func(i int, cursor bool) string {
			h := s.results[i]
			pointer := "  "
			title := h.Title
			if cursor {
				pointer = st.key.Render("› ")
				title = st.value.Bold(true).Render(title)
			}
			meta := st.muted.Render(fmt.Sprintf("  %s · %s", compactNumber(h.Downloads), h.Author))
			return lipgloss.NewStyle().MaxWidth(listW).Render(pointer + title + meta)
		})
		if len(s.results) == 0 && !s.searching {
			rows = []string{"  " + st.muted.Render("No mods found. Try another search (/), sort (s) or category (c).")}
		}
		list = lipgloss.JoinVertical(lipgloss.Left, rows...)
	}
	if wide {
		detail := lipgloss.NewStyle().Width(m.width - listW - 6).PaddingLeft(2).
			BorderLeft(true).BorderStyle(lipgloss.NormalBorder()).BorderForeground(st.muted.GetForeground()).
			Render(m.detailView(m.width - listW - 9))
		list = lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.NewStyle().Width(listW).Render(list), detail)
	}
	return lipgloss.JoinVertical(lipgloss.Left, top, "", list)
}

func (m model) detailView(width int) string {
	st := m.styles
	s := &m.mods
	h := m.cursorHit()
	if h == nil {
		return ""
	}
	lines := []string{
		st.title.Render(h.Title) + st.muted.Render("  by "+h.Author),
		st.muted.Render(fmt.Sprintf("%s downloads · %s follows · %s", compactNumber(h.Downloads), compactNumber(h.Follows), strings.Join(h.Categories, ", "))),
		"",
		lipgloss.NewStyle().Width(width).Render(h.Description),
	}
	if d := s.details[h.ProjectID]; d != nil {
		body := plainText(d.Body)
		maxLines := max(m.height-18, 3)
		wrapped := strings.Split(lipgloss.NewStyle().Width(width).Render(body), "\n")
		if len(wrapped) > maxLines {
			wrapped = append(wrapped[:maxLines], st.muted.Render("…"))
		}
		lines = append(lines, "", st.muted.Render(strings.Join(wrapped, "\n")))
	}
	lines = append(lines, "", st.key.Render("i")+st.muted.Render(" install the newest that fits · ")+st.key.Render("v")+st.muted.Render(" pick a version · modrinth.com/mod/"+h.Slug))
	return strings.Join(lines, "\n")
}

func (m model) planView() string {
	st := m.styles
	p := m.mods.plan
	lines := []string{"  " + st.title.Render("Install "+m.mods.planTitle+"?"), ""}
	for _, s := range p.Steps {
		why := st.muted.Render("  dependency")
		if s.Requested {
			why = ""
		}
		if s.Replaces != "" {
			why = st.muted.Render("  replaces " + s.Replaces)
		}
		lines = append(lines, "    "+st.ok.Render("+ ")+s.Name()+st.muted.Render("  "+s.Version.Number)+why)
	}
	for _, n := range p.Notes {
		lines = append(lines, "", "  "+st.muted.Width(max(m.width-6, 20)).Render(n))
	}
	for _, w := range p.Warnings {
		lines = append(lines, "  "+st.warn.Render("! "+w.Text))
	}
	lines = append(lines, "", "  "+st.muted.Render("Checked against your mods and the Armstrong client. y installs, n cancels."))
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m model) removalView() string {
	st := m.styles
	r := m.mods.removal
	lines := []string{"  " + st.title.Render("Remove "+r.Remove[0].Name()+"?"), ""}
	for i, x := range r.Remove {
		why := ""
		if i > 0 {
			why = st.muted.Render("  no longer needed")
		}
		lines = append(lines, "    "+st.err.Render("- ")+x.Name()+why)
	}
	for _, d := range r.Dependents {
		lines = append(lines, "", "  "+st.warn.Render("! "+d+" needs it and won't start without it."))
	}
	lines = append(lines, "", "  "+st.muted.Render("y removes, n cancels."))
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

var (
	mdImage   = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	mdLink    = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	htmlTag   = regexp.MustCompile(`<[^>]+>`)
	mdMarks   = regexp.MustCompile("(?m)^#{1,6}\\s*|[*_`]{1,3}|^>\\s?")
	blankRuns = regexp.MustCompile(`\n{3,}`)
)

// plainText turns a Markdown project page into readable plain text.
func plainText(md string) string {
	s := mdImage.ReplaceAllString(md, "")
	s = mdLink.ReplaceAllString(s, "$1")
	s = htmlTag.ReplaceAllString(s, "")
	s = mdMarks.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "\r", "")
	return strings.TrimSpace(blankRuns.ReplaceAllString(s, "\n\n"))
}

func compactNumber(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	}
	return fmt.Sprint(n)
}
