package mods

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/ark4660/armstrong/internal/client"
	"github.com/ark4660/armstrong/internal/download"
	"github.com/ark4660/armstrong/internal/game"
	"github.com/ark4660/armstrong/internal/modmeta"
)

const disabledSuffix = ".disabled"

// maxCandidates is how many versions of a project are tried, newest first,
// before giving up (the newest often needs a newer dependency than you have).
const maxCandidates = 10

// Manager handles one instance's mods folder.
type Manager struct {
	ModsDir   string
	IndexPath string // the instance's mods.json (outside the game folder)
	CacheDir  string // downloaded jars by sha1, shared by instances

	Minecraft, Loader, Java string
	Layer                   *client.Layer // the client layer the instance runs with
	Source                  Source
	DL                      *download.Client

	mu sync.Mutex // guards the index file

	baseOnce sync.Once // the client layer's units, read once
	base     *Env
	baseErr  error
}

// Installed is a jar in the mods folder.
type Installed struct {
	File       string // name on disk (ends in .disabled when off)
	Key        string // name without .disabled
	Enabled    bool
	Units      []Unit // nil if it isn't a Fabric mod
	SHA1       string
	Source     *Ref
	Explicit   bool     // the player chose it (not just a dependency)
	RequiredBy []string // projects that needed it
	Problems   []Problem
}

func (i *Installed) Name() string {
	if len(i.Units) > 0 {
		return i.Units[0].title()
	}
	return i.Key
}

func (i *Installed) Version() string {
	if len(i.Units) > 0 {
		return i.Units[0].Version
	}
	return ""
}

// Ref says where a file came from.
type Ref struct {
	Source    string `json:"source"`
	Project   string `json:"project"`
	VersionID string `json:"versionId"`
	Version   string `json:"version"`
	Channel   string `json:"channel,omitempty"`
}

type indexEntry struct {
	SHA1       string   `json:"sha1"`
	Size       int64    `json:"size"`
	ModTime    int64    `json:"mtime"`
	Units      []Unit   `json:"units,omitempty"`
	NotAMod    bool     `json:"notAMod,omitempty"`
	Source     *Ref     `json:"source,omitempty"`
	Unknown    bool     `json:"unknown,omitempty"` // looked up, no site knows it
	Explicit   bool     `json:"explicit,omitempty"`
	RequiredBy []string `json:"requiredBy,omitempty"`
}

type index struct {
	Version int                    `json:"version"`
	Files   map[string]*indexEntry `json:"files"` // by key (file name without .disabled)
}

func (m *Manager) loadIndex() *index {
	idx := &index{Version: 1, Files: map[string]*indexEntry{}}
	data, err := os.ReadFile(m.IndexPath)
	if err == nil {
		if json.Unmarshal(data, idx) != nil || idx.Files == nil {
			idx = &index{Version: 1, Files: map[string]*indexEntry{}}
		}
	}
	return idx
}

func (m *Manager) saveIndex(idx *index) error {
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	tmp := m.IndexPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, m.IndexPath)
}

// Scan lists the mods folder, reading metadata only for files that changed
// since the last scan (by size and modification time).
func (m *Manager) Scan() ([]*Installed, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.loadIndex()
	entries, err := os.ReadDir(m.ModsDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	seen := map[string]bool{}
	var out []*Installed
	for _, e := range entries {
		name := e.Name()
		key, enabled := strings.TrimSuffix(name, disabledSuffix), !strings.HasSuffix(name, disabledSuffix)
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(key), ".jar") || seen[key] {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		seen[key] = true
		ie := idx.Files[key]
		if ie == nil || ie.Size != info.Size() || ie.ModTime != info.ModTime().UnixNano() {
			fresh, err := readEntry(filepath.Join(m.ModsDir, name), info)
			if err != nil {
				continue
			}
			if ie != nil && ie.SHA1 == fresh.SHA1 { // same file, touched: keep what we knew
				fresh.Source, fresh.Unknown, fresh.Explicit, fresh.RequiredBy = ie.Source, ie.Unknown, ie.Explicit, ie.RequiredBy
			} else if ie == nil {
				fresh.Explicit = true // put there by the player
			}
			ie = fresh
			idx.Files[key] = ie
		}
		out = append(out, &Installed{File: name, Key: key, Enabled: enabled, Units: ie.Units, SHA1: ie.SHA1,
			Source: ie.Source, Explicit: ie.Explicit, RequiredBy: ie.RequiredBy})
	}
	for key := range idx.Files {
		if !seen[key] {
			delete(idx.Files, key)
		}
	}
	if err := m.saveIndex(idx); err != nil {
		return nil, err
	}
	sort.Slice(out, func(a, b int) bool { return strings.ToLower(out[a].Name()) < strings.ToLower(out[b].Name()) })
	m.validate(out)
	return out, nil
}

func readEntry(path string, info fs.FileInfo) (*indexEntry, error) {
	sum, err := fileSHA1(path)
	if err != nil {
		return nil, err
	}
	ie := &indexEntry{SHA1: sum, Size: info.Size(), ModTime: info.ModTime().UnixNano()}
	meta, err := modmeta.ReadJar(path)
	if err != nil {
		ie.NotAMod = true
	} else {
		ie.Units = UnitsOf(meta)
	}
	return ie, nil
}

func fileSHA1(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// BaseEnv is the game plus the client layer, and the add-ons the player
// could switch on. The layer's jars are read once per Manager (Fabric API
// alone nests dozens of mods); callers get their own copy.
func (m *Manager) BaseEnv() (*Env, error) {
	m.baseOnce.Do(func() {
		env := NewEnv(m.Minecraft, m.Loader, m.Java)
		if m.Layer != nil {
			for i, mod := range m.Layer.Mods {
				meta, err := modmeta.ReadJar(m.Layer.Files[i])
				if err != nil {
					m.baseErr = fmt.Errorf("client mod %s: %w", mod.Name, err)
					return
				}
				env.Add(UnitsOf(meta), FromClient, mod.Name, "client:"+mod.ID)
			}
			for _, mod := range m.Layer.Manifest.Mods {
				if mod.Optional && !m.Layer.Has(mod.ID) {
					env.AddAddon(mod)
				}
			}
		}
		m.base = env
	})
	if m.baseErr != nil {
		return nil, m.baseErr
	}
	return m.base.Clone(), nil
}

// envWith is the base plus every enabled installed mod except skip (a key).
func (m *Manager) envWith(installed []*Installed, skip string) (*Env, error) {
	env, err := m.BaseEnv()
	if err != nil {
		return nil, err
	}
	for _, i := range installed {
		if i.Enabled && i.Key != skip && i.Units != nil {
			env.Add(i.Units, FromInstalled, i.Name(), "file:"+i.Key)
		}
	}
	return env, nil
}

// validate fills each enabled mod's problems: missing dependencies, wrong
// versions and incompatibilities, as Fabric would report them at startup.
func (m *Manager) validate(installed []*Installed) {
	env, err := m.envWith(installed, "")
	if err != nil {
		return
	}
	for _, i := range installed {
		if i.Enabled && i.Units != nil {
			i.Problems = env.Check(i.Units, i.Name(), "file:"+i.Key)
		}
	}
}

// Identify asks the source which project unidentified files belong to.
func (m *Manager) Identify(ctx context.Context) error {
	m.mu.Lock()
	idx := m.loadIndex()
	var hashes []string
	for _, ie := range idx.Files {
		if ie.Source == nil && !ie.Unknown && !ie.NotAMod {
			hashes = append(hashes, ie.SHA1)
		}
	}
	m.mu.Unlock()
	if len(hashes) == 0 {
		return nil
	}
	found, err := m.Source.Identify(ctx, hashes)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	idx = m.loadIndex()
	for _, ie := range idx.Files {
		if ie.Source != nil || ie.Unknown || ie.NotAMod || !slices.Contains(hashes, ie.SHA1) {
			continue
		}
		if v, ok := found[ie.SHA1]; ok {
			ie.Source = &Ref{Source: m.Source.Name(), Project: v.ProjectID, VersionID: v.ID, Version: v.Number, Channel: v.Channel}
		} else {
			ie.Unknown = true
		}
	}
	return m.saveIndex(idx)
}

// SetEnabled turns a mod on or off by renaming it.
func (m *Manager) SetEnabled(i *Installed, on bool) error {
	if i.Enabled == on {
		return nil
	}
	from := filepath.Join(m.ModsDir, i.File)
	to := filepath.Join(m.ModsDir, i.Key)
	if !on {
		to += disabledSuffix
	}
	return os.Rename(from, to)
}

// ErrInClient means the project is already part of the client layer.
type ErrInClient struct {
	Name  string
	Addon bool   // an optional add-on the player can switch on instead
	Mod   string // set when Name is built into this client mod (Sodium provides Indium)
}

func (e *ErrInClient) Error() string {
	switch {
	case e.Mod != "":
		return e.Name + " is built into " + e.Mod + ", which is part of the Armstrong client"
	case e.Addon:
		return e.Name + " is an Armstrong add-on: switch it on under Client add-ons instead"
	}
	return e.Name + " is already part of the Armstrong client"
}

// ErrInstalled means the mod is already in the mods folder under another file.
type ErrInstalled struct{ Name, File string }

func (e *ErrInstalled) Error() string { return e.Name + " is already installed (" + e.File + ")" }

// ErrNoFit means no tried version of a project fits this instance.
type ErrNoFit struct {
	Project  string
	Tried    int
	Untried  int       // older versions left out of the automatic choice
	Problems []Problem // why the newest candidate didn't fit
}

func (e *ErrNoFit) Error() string {
	if e.Tried == 0 {
		return fmt.Sprintf("%s has no version for this Minecraft version and loader", e.Project)
	}
	msg := fmt.Sprintf("no version of %s fits this instance (tried %d)", e.Project, e.Tried)
	if len(e.Problems) > 0 {
		msg += ": " + e.Problems[0].Text
	}
	return msg
}

// Step is one jar a plan installs.
type Step struct {
	Version   Version
	Units     []Unit
	Path      string // downloaded jar in the cache
	Requested bool   // false: a dependency
	NeededBy  string // project id that needs it
	Replaces  string // key of the installed jar it updates
}

func (s Step) Name() string {
	if len(s.Units) > 0 {
		return s.Units[0].title()
	}
	return s.Version.File.Name
}

// Plan is what installing (or updating) a project would do.
type Plan struct {
	Steps    []Step
	Warnings []Problem
	Notes    []string    // e.g. why the newest version wasn't picked
	edges    [][2]string // {installed key, project id that now needs it}
}

type planner struct {
	m         *Manager
	ctx       context.Context
	installed []*Installed
	base      *Env // game + client + installed (minus what's being replaced)
	plan      *Plan
	replace   map[string]string // project id → installed key being replaced
	progress  func(string)      // may be nil

	seen *lookups
}

// lookups are answers that hold for every plan against the same mods folder:
// what the site says, and dependencies that didn't fit. Without them every
// candidate of a mod resolves its dependencies again from scratch; checking
// several versions of a mod in a row shares them too.
type lookups struct {
	versions map[string][]Version
	version  map[string]*Version
	failed   map[string]error // by failKey
}

func newLookups() *lookups {
	return &lookups{versions: map[string][]Version{}, version: map[string]*Version{}, failed: map[string]error{}}
}

// PlanInstall works out how to install a project: a version that fits, and
// its required dependencies, recursively. Nothing is written until Commit.
// progress, if not nil, hears which file is being checked.
func (m *Manager) PlanInstall(ctx context.Context, project string, progress func(string)) (*Plan, error) {
	installed, err := m.scanWithout(project)
	if err != nil {
		return nil, err
	}
	return m.plan(ctx, installed, project, "", "", progress)
}

// PlanInstallVersion is PlanInstall for one chosen version instead of the
// newest that fits.
func (m *Manager) PlanInstallVersion(ctx context.Context, project, versionID string, progress func(string)) (*Plan, error) {
	installed, err := m.scanWithout(project)
	if err != nil {
		return nil, err
	}
	return m.plan(ctx, installed, project, versionID, "", progress)
}

// CheckVersions works out, version by version, whether each would install,
// without installing anything: report gets the plan or why it doesn't fit.
// What the site says and dependencies that didn't fit are shared between
// them. It stops early (with the context's error) when ctx ends.
func (m *Manager) CheckVersions(ctx context.Context, project string, versionIDs []string, report func(versionID string, plan *Plan, err error)) error {
	installed, err := m.scanWithout(project)
	if err != nil {
		return err
	}
	// One listing gives every version's details, instead of a request per version.
	seen := newLookups()
	vs, err := m.Source.Versions(ctx, project, m.Minecraft, game.ModrinthLoader(m.Minecraft))
	if err != nil {
		return err
	}
	seen.versions[project] = vs
	for i := range vs {
		seen.version[vs[i].ID] = &vs[i]
	}
	for _, id := range versionIDs {
		plan, err := m.planWith(ctx, installed, project, id, "", nil, seen)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		report(id, plan, err)
	}
	return nil
}

// Versions lists every version of a project for this instance's Minecraft
// version and loader, newest first.
func (m *Manager) Versions(ctx context.Context, project string) ([]Version, error) {
	vs, err := m.Source.Versions(ctx, project, m.Minecraft, game.ModrinthLoader(m.Minecraft))
	if err != nil {
		return nil, err
	}
	sort.SliceStable(vs, func(a, b int) bool { return vs[a].Published > vs[b].Published })
	return vs, nil
}

// scanWithout scans the mods folder, failing if the project is already in it.
func (m *Manager) scanWithout(project string) ([]*Installed, error) {
	installed, err := m.Scan()
	if err != nil {
		return nil, err
	}
	for _, i := range installed {
		if i.Source != nil && i.Source.Project == project {
			return nil, fmt.Errorf("%s is already installed", i.Name())
		}
	}
	return installed, nil
}

// PlanUpdate works out how to replace an installed mod with a newer version.
func (m *Manager) PlanUpdate(ctx context.Context, i *Installed, versionID string, progress func(string)) (*Plan, error) {
	if i.Source == nil {
		return nil, fmt.Errorf("%s isn't from a known source", i.Name())
	}
	installed, err := m.Scan()
	if err != nil {
		return nil, err
	}
	return m.plan(ctx, installed, i.Source.Project, versionID, i.Key, progress)
}

func (m *Manager) plan(ctx context.Context, installed []*Installed, project, versionID, replaceKey string, progress func(string)) (*Plan, error) {
	return m.planWith(ctx, installed, project, versionID, replaceKey, progress, newLookups())
}

func (m *Manager) planWith(ctx context.Context, installed []*Installed, project, versionID, replaceKey string, progress func(string), seen *lookups) (*Plan, error) {
	base, err := m.envWith(installed, replaceKey)
	if err != nil {
		return nil, err
	}
	p := &planner{m: m, ctx: ctx, installed: installed, base: base, plan: &Plan{}, replace: map[string]string{}, progress: progress, seen: seen}
	if replaceKey != "" {
		p.replace[project] = replaceKey
	}
	if err := p.want(project, versionID, "", true); err != nil {
		return nil, err
	}
	return p.plan, nil
}

// env is the base plus every planned step.
func (p *planner) env() *Env {
	env := p.base.Clone()
	for _, s := range p.plan.Steps {
		env.Add(s.Units, FromPlanned, s.Name(), "plan:"+s.Version.ProjectID)
	}
	return env
}

func (p *planner) installedProject(project string) *Installed {
	if _, replacing := p.replace[project]; replacing {
		return nil
	}
	for _, i := range p.installed {
		if i.Source != nil && i.Source.Project == project {
			return i
		}
	}
	return nil
}

func (p *planner) clientMod(project string) *client.Mod {
	if p.m.Layer == nil {
		return nil
	}
	for _, mod := range p.m.Layer.Manifest.Mods {
		if mod.Project == project {
			return &mod
		}
	}
	return nil
}

// want makes sure a project is (or will be) installed.
func (p *planner) want(project, versionID, neededBy string, requested bool) error {
	if slices.ContainsFunc(p.plan.Steps, func(s Step) bool { return s.Version.ProjectID == project }) {
		return nil
	}
	if mod := p.clientMod(project); mod != nil {
		enabled := p.m.Layer.Has(mod.ID)
		if requested || !enabled {
			return &ErrInClient{Name: mod.Name, Addon: mod.Optional && !enabled}
		}
		return nil // a dependency the client already provides
	}
	if i := p.installedProject(project); i != nil {
		if neededBy != "" {
			p.plan.edges = append(p.plan.edges, [2]string{i.Key, neededBy})
		}
		return nil
	}
	// A dependency that didn't fit one candidate of the mod needing it won't
	// fit the next candidate either: the plan around it is the same.
	key := p.failKey(project, versionID, neededBy)
	if err, ok := p.seen.failed[key]; ok && neededBy != "" {
		return err
	}
	err := p.resolve(project, versionID, neededBy, requested)
	if err != nil && neededBy != "" && p.ctx.Err() == nil {
		p.seen.failed[key] = err
	}
	return err
}

// resolve tries a project's candidate versions, newest first, until one fits.
func (p *planner) resolve(project, versionID, neededBy string, requested bool) error {
	var cands []Version
	total := 1
	if versionID != "" {
		v, err := p.lookupVersion(versionID)
		if err != nil {
			return err
		}
		cands = []Version{*v}
	} else {
		vs, err := p.lookupVersions(project)
		if err != nil {
			return err
		}
		total = len(vs)
		cands = candidates(vs, maxCandidates)
	}

	fail := &ErrNoFit{Project: project, Tried: len(cands), Untried: total - len(cands)}
	for ci, v := range cands {
		if err := p.ctx.Err(); err != nil {
			return err
		}
		p.say("%s (%d of %d)", v.File.Name, ci+1, len(cands))
		problems, err := p.try(v, neededBy, requested)
		if err != nil {
			var inClient *ErrInClient
			var installed *ErrInstalled
			if errors.As(err, &inClient) || errors.As(err, &installed) || p.ctx.Err() != nil {
				return err
			}
			problems = []Problem{{Severity: Error, Text: err.Error()}}
		}
		if problems == nil {
			if ci > 0 && requested && len(fail.Problems) > 0 {
				p.plan.Notes = append(p.plan.Notes, fmt.Sprintf("Using %s instead of the newest %s: %s", v.Number, cands[0].Number, fail.Problems[0].Text))
			}
			return nil
		}
		if ci == 0 {
			fail.Problems = problems
		}
	}
	if len(cands) > 0 && len(fail.Problems) > 0 {
		if name := p.nameOf(cands[0]); name != "" {
			fail.Project = name
		}
	}
	if fail.Project == project { // still an id: ask the site for the name
		if title, err := p.m.Source.Title(p.ctx, project); err == nil && title != "" {
			fail.Project = title
		}
	}
	return fail
}

// failKey identifies a dependency lookup: the project, a pinned version,
// and what's planned so far apart from the mod that needs it.
func (p *planner) failKey(project, versionID, neededBy string) string {
	var b strings.Builder
	b.WriteString(project + "@" + versionID)
	for _, s := range p.plan.Steps {
		if s.Version.ProjectID != neededBy {
			b.WriteString("|" + s.Version.ID)
		}
	}
	return b.String()
}

func (p *planner) lookupVersions(project string) ([]Version, error) {
	if vs, ok := p.seen.versions[project]; ok {
		return vs, nil
	}
	vs, err := p.m.Source.Versions(p.ctx, project, p.m.Minecraft, game.ModrinthLoader(p.m.Minecraft))
	if err != nil {
		return nil, err
	}
	p.seen.versions[project] = vs
	return vs, nil
}

func (p *planner) lookupVersion(id string) (*Version, error) {
	if v, ok := p.seen.version[id]; ok {
		return v, nil
	}
	v, err := p.m.Source.Version(p.ctx, id)
	if err != nil {
		return nil, err
	}
	p.seen.version[id] = v
	return v, nil
}

func (p *planner) say(format string, args ...any) {
	if p.progress != nil {
		p.progress(fmt.Sprintf(format, args...))
	}
}

// nameOf is a version's mod name if its metadata was already read (a version
// rejected on what the site says isn't looked into just for this).
func (p *planner) nameOf(v Version) string {
	if units, err := p.m.cachedUnits(v); err == nil && len(units) > 0 {
		return units[0].title()
	}
	return ""
}

// try plans one candidate version. It returns nil problems on success; on
// failure the plan is left as it was. Cheap checks come first: what the site
// says, then the jar's own requirements, and only then its dependencies,
// each of which may mean more downloads.
func (p *planner) try(v Version, neededBy string, requested bool) ([]Problem, error) {
	// Some requirements exist only on the site: a dependency pinned to an
	// exact version. If that's installed at an older version, this one doesn't
	// fit (Iris pins a Sodium build this way). An older pin is left to the
	// metadata check, which is what Fabric itself enforces.
	for _, d := range v.Deps {
		if d.Kind != "required" || d.VersionID == "" {
			continue
		}
		if i := p.installedProject(d.ProjectID); i != nil && i.Source.VersionID != d.VersionID {
			pinned, err1 := p.lookupVersion(d.VersionID)
			have, err2 := p.lookupVersion(i.Source.VersionID)
			if err1 == nil && err2 == nil && pinned.Published > have.Published {
				return []Problem{{Severity: Error, Text: fmt.Sprintf("needs %s %s, you have %s (update it first)", i.Name(), pinned.Number, i.Source.Version)}}, nil
			}
		}
	}
	if problems := p.siteProblems(v); problems != nil {
		return problems, nil
	}

	units, err := p.m.versionUnits(p.ctx, v)
	if errors.Is(err, modmeta.ErrNotAMod) {
		return []Problem{{Severity: Error, Text: v.File.Name + " isn't a Fabric mod"}}, nil
	}
	if err != nil {
		return nil, err
	}
	path := p.m.jarPath(v)
	if p.m.Layer != nil {
		ids := p.m.Layer.IDs()
		if name, ok := ids[units[0].ID]; ok {
			if !requested {
				return nil, nil // a dependency the client already provides
			}
			if !p.m.Layer.Has(units[0].ID) {
				// Provided by a client mod under another id.
				return nil, &ErrInClient{Name: units[0].title(), Mod: name}
			}
			return nil, &ErrInClient{Name: name}
		}
	}
	// Already installed under another file (copied in by hand, or from
	// another site): count it as there rather than adding a second copy.
	if origin, owner, _, ok := p.base.Provided(units[0].ID); ok && origin == FromInstalled {
		if requested {
			return nil, &ErrInstalled{Name: units[0].title(), File: owner}
		}
		for _, i := range p.installed {
			if i.Units != nil && i.Units[0].ID == units[0].ID && neededBy != "" {
				p.plan.edges = append(p.plan.edges, [2]string{i.Key, neededBy})
			}
		}
		return nil, nil
	}

	snapshot := len(p.plan.Steps)
	edges := len(p.plan.edges)
	rollback := func() {
		p.plan.Steps = p.plan.Steps[:snapshot]
		p.plan.edges = p.plan.edges[:edges]
	}
	step := Step{Version: v, Units: units, Path: path, Requested: requested, NeededBy: neededBy, Replaces: p.replace[v.ProjectID]}
	p.plan.Steps = append(p.plan.Steps, step)

	// Its own requirements (Minecraft, loader, what's installed) before its
	// dependencies: a version that can't load anyway isn't worth resolving.
	// Dependencies it's missing are what the next step looks for.
	if problems := withoutMissing(p.env().Check(units, step.Name(), "plan:"+v.ProjectID)); HasErrors(problems) {
		rollback()
		return problems, nil
	}

	for _, d := range v.Deps {
		if d.Kind != "required" {
			continue
		}
		if err := p.want(d.ProjectID, d.VersionID, v.ProjectID, false); err != nil {
			if p.ctx.Err() != nil {
				rollback()
				return nil, p.ctx.Err()
			}
			// Multi-loader uploads often list a dependency that only exists
			// for other loaders (KotlinLangForge for Zoomify). With no build
			// for this loader at all it can't be meant for us; if the jar
			// really needs something, its fabric.mod.json check below says so.
			var noFit *ErrNoFit
			if errors.As(err, &noFit) && noFit.Tried == 0 {
				continue
			}
			rollback()
			return []Problem{{Severity: Error, Text: "dependency: " + err.Error()}}, nil
		}
	}

	problems := p.env().Check(units, step.Name(), "plan:"+v.ProjectID)
	if HasErrors(problems) {
		rollback()
		return problems, nil
	}
	p.plan.Warnings = append(p.plan.Warnings, problems...)
	return nil, nil
}

// siteProblems are what a version's page says against it: a dependency that's
// a client add-on the player hasn't switched on, or a mod it doesn't work
// with. Checked before the jar is downloaded.
func (p *planner) siteProblems(v Version) []Problem {
	for _, d := range v.Deps {
		switch d.Kind {
		case "required":
			if mod := p.clientMod(d.ProjectID); mod != nil && !p.m.Layer.Has(mod.ID) {
				return []Problem{{Severity: Error, Text: addonNeeded(mod.Name)}}
			}
		case "incompatible":
			if i := p.installedProject(d.ProjectID); i != nil {
				return []Problem{{Severity: Error, Text: fmt.Sprintf("doesn't work with %s (installed)", i.Name())}}
			}
			if mod := p.clientMod(d.ProjectID); mod != nil && p.m.Layer.Has(mod.ID) {
				return []Problem{{Severity: Error, Text: fmt.Sprintf("doesn't work with %s (part of the Armstrong client)", mod.Name)}}
			}
		}
	}
	return nil
}

// addonNeeded says a mod needs a client add-on that's switched off.
func addonNeeded(name string) string {
	return "needs " + name + ", an Armstrong add-on: switch it on under Client add-ons"
}

// jarPath is where a version's jar is kept in the cache (by sha1).
func (m *Manager) jarPath(v Version) string {
	return filepath.Join(m.CacheDir, v.File.SHA1+".jar")
}

// fetch downloads a version's jar into the cache and returns its path.
func (m *Manager) fetch(ctx context.Context, v Version) (string, error) {
	if v.File.SHA1 == "" {
		return "", fmt.Errorf("%s has no checksum", v.File.Name)
	}
	path := m.jarPath(v)
	err := m.DL.Fetch(ctx, []download.Item{{URL: v.File.URL, Path: path, SHA1: v.File.SHA1, Size: v.File.Size}}, nil)
	return path, err
}

// unitsFile is a version's metadata kept next to the jar cache, so looking
// at a version again needs no network.
type unitsFile struct {
	Units   []Unit `json:"units,omitempty"`
	NotAMod bool   `json:"notAMod,omitempty"`
}

func (m *Manager) unitsPath(v Version) string {
	return filepath.Join(m.CacheDir, v.File.SHA1+".units.json")
}

// versionUnits reads what a version's jar holds, downloading as little as it
// can: the jar if it's already in the cache, else metadata kept from an
// earlier look, else just the jar's zip index and fabric.mod.json files over
// HTTP range requests (a 30 MB mod costs well under 1 MB this way). The jar
// itself is downloaded only when the site can't serve ranges; installing
// downloads it in Commit. Errors wrap modmeta.ErrNotAMod for a jar that
// isn't a Fabric mod.
func (m *Manager) versionUnits(ctx context.Context, v Version) ([]Unit, error) {
	if v.File.SHA1 == "" {
		return nil, fmt.Errorf("%s has no checksum", v.File.Name)
	}
	if units, err := m.cachedUnits(v); err == nil || errors.Is(err, modmeta.ErrNotAMod) {
		return units, err
	}
	meta, err := modmeta.Read(m.DL.RangeReader(ctx, v.File.URL, v.File.Size), v.File.Size)
	switch {
	case ctx.Err() != nil:
		return nil, ctx.Err()
	case err != nil && !errors.Is(err, modmeta.ErrNotAMod):
		// No ranges, or a jar that can't be read in pieces: download it.
		path, err := m.fetch(ctx, v)
		if err != nil {
			return nil, err
		}
		return readUnits(path)
	}
	var file unitsFile
	if err == nil {
		file.Units = UnitsOf(meta)
	} else {
		file.NotAMod = true
	}
	if data, merr := json.Marshal(file); merr == nil && os.MkdirAll(m.CacheDir, 0o755) == nil {
		os.WriteFile(m.unitsPath(v), data, 0o644) // best effort: it's only a cache
	}
	if file.NotAMod {
		return nil, modmeta.ErrNotAMod
	}
	return file.Units, nil
}

// cachedUnits is a version's metadata from the cache only (no network).
func (m *Manager) cachedUnits(v Version) ([]Unit, error) {
	if v.File.SHA1 == "" {
		return nil, fs.ErrNotExist
	}
	if jar := m.jarPath(v); download.Valid(jar, v.File.SHA1, v.File.Size) {
		return readUnits(jar)
	}
	data, err := os.ReadFile(m.unitsPath(v))
	if err != nil {
		return nil, err
	}
	var file unitsFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	if file.NotAMod {
		return nil, modmeta.ErrNotAMod
	}
	if len(file.Units) == 0 {
		return nil, fs.ErrNotExist
	}
	return file.Units, nil
}

func readUnits(jar string) ([]Unit, error) {
	meta, err := modmeta.ReadJar(jar)
	if err != nil {
		if errors.Is(err, modmeta.ErrNotAMod) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %v", modmeta.ErrNotAMod, err)
	}
	return UnitsOf(meta), nil
}

// Commit carries out a plan: downloads its jars (checking a plan reads only
// their metadata), copies them into the mods folder, removes the versions
// they replace, and records who needs what. Nothing in the mods folder
// changes unless every download worked.
func (m *Manager) Commit(ctx context.Context, plan *Plan) error {
	var items []download.Item
	for _, s := range plan.Steps {
		items = append(items, download.Item{URL: s.Version.File.URL, Path: s.Path, SHA1: s.Version.File.SHA1, Size: s.Version.File.Size})
	}
	if err := m.DL.Fetch(ctx, items, nil); err != nil {
		return fmt.Errorf("downloading: %w", err)
	}
	if err := os.MkdirAll(m.ModsDir, 0o755); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.loadIndex()
	for _, s := range plan.Steps {
		if err := safeName(s.Version.File.Name); err != nil {
			return err
		}
		key := s.Version.File.Name
		var keepRequiredBy []string
		explicit := s.Requested
		if s.Replaces != "" {
			if old := idx.Files[s.Replaces]; old != nil {
				keepRequiredBy, explicit = old.RequiredBy, old.Explicit
			}
			os.Remove(filepath.Join(m.ModsDir, s.Replaces))
			os.Remove(filepath.Join(m.ModsDir, s.Replaces+disabledSuffix))
			delete(idx.Files, s.Replaces)
		}
		dst := filepath.Join(m.ModsDir, key)
		if err := copyFile(s.Path, dst); err != nil {
			return err
		}
		info, err := os.Stat(dst)
		if err != nil {
			return err
		}
		ie := &indexEntry{
			SHA1: s.Version.File.SHA1, Size: info.Size(), ModTime: info.ModTime().UnixNano(), Units: s.Units,
			Source:   &Ref{Source: m.Source.Name(), Project: s.Version.ProjectID, VersionID: s.Version.ID, Version: s.Version.Number, Channel: s.Version.Channel},
			Explicit: explicit, RequiredBy: keepRequiredBy,
		}
		if s.NeededBy != "" && !slices.Contains(ie.RequiredBy, s.NeededBy) {
			ie.RequiredBy = append(ie.RequiredBy, s.NeededBy)
		}
		idx.Files[key] = ie
	}
	for _, e := range plan.edges {
		if ie := idx.Files[e[0]]; ie != nil && !slices.Contains(ie.RequiredBy, e[1]) {
			ie.RequiredBy = append(ie.RequiredBy, e[1])
		}
	}
	return m.saveIndex(idx)
}

func safeName(name string) error {
	if name == "" || name != filepath.Base(name) || strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
		return fmt.Errorf("unsafe file name %q", name)
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

// Removal is what removing a mod would do.
type Removal struct {
	Remove     []*Installed // the mod, then dependencies nothing else needs
	Dependents []string     // installed mods that need the mod
}

// PlanRemove works out a removal: dependencies that were installed only for
// this mod go too; a dependency something else still needs stays.
func (m *Manager) PlanRemove(target *Installed) (*Removal, error) {
	installed, err := m.Scan()
	if err != nil {
		return nil, err
	}
	r := &Removal{}
	gone := map[string]bool{}
	var remove func(i *Installed)
	remove = func(i *Installed) {
		if gone[i.Key] {
			return
		}
		gone[i.Key] = true
		r.Remove = append(r.Remove, i)
		if i.Source == nil {
			return
		}
		for _, dep := range installed {
			if gone[dep.Key] || dep.Explicit || !slices.Contains(dep.RequiredBy, i.Source.Project) {
				continue
			}
			stillNeeded := slices.ContainsFunc(dep.RequiredBy, func(project string) bool {
				return project != i.Source.Project && slices.ContainsFunc(installed, func(o *Installed) bool {
					return !gone[o.Key] && o.Source != nil && o.Source.Project == project
				})
			})
			if !stillNeeded {
				remove(dep)
			}
		}
	}
	for _, i := range installed {
		if i.Key == target.Key {
			remove(i)
		}
	}
	if len(r.Remove) == 0 {
		return nil, fmt.Errorf("%s isn't installed", target.Name())
	}

	// Mods that stay but declare a dependency on something being removed.
	removedIDs := map[string]bool{}
	for _, i := range r.Remove {
		for _, u := range i.Units {
			removedIDs[u.ID] = true
			for _, p := range u.Provides {
				removedIDs[p] = true
			}
		}
	}
	for _, o := range installed {
		if gone[o.Key] || !o.Enabled {
			continue
		}
		for _, u := range o.Units {
			if slices.ContainsFunc(sortedKeys(u.Depends), func(id string) bool { return removedIDs[id] }) {
				r.Dependents = append(r.Dependents, o.Name())
				break
			}
		}
	}
	return r, nil
}

// CommitRemove deletes the files of a removal.
func (m *Manager) CommitRemove(r *Removal) error {
	for _, i := range r.Remove {
		if err := os.Remove(filepath.Join(m.ModsDir, i.File)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	_, err := m.Scan() // drops the index entries
	return err
}

// Update is a newer version of an installed mod.
type Update struct {
	Installed *Installed
	Version   Version
}

// CheckUpdates finds newer versions of identified mods. Mods installed from a
// release only get releases; beta or alpha installs may move to newer betas.
func (m *Manager) CheckUpdates(ctx context.Context, installed []*Installed) ([]Update, error) {
	var stable, any []string
	byHash := map[string]*Installed{}
	for _, i := range installed {
		if i.Source == nil || i.Source.Source != m.Source.Name() {
			continue
		}
		byHash[i.SHA1] = i
		if i.Source.Channel == "" || i.Source.Channel == "release" {
			stable = append(stable, i.SHA1)
		} else {
			any = append(any, i.SHA1)
		}
	}
	var out []Update
	for _, batch := range []struct {
		hashes   []string
		channels []string
	}{{stable, []string{"release"}}, {any, []string{"release", "beta", "alpha"}}} {
		latest, err := m.Source.Latest(ctx, batch.hashes, m.Minecraft, game.ModrinthLoader(m.Minecraft), batch.channels)
		if err != nil {
			return nil, err
		}
		for hash, v := range latest {
			i := byHash[hash]
			if i != nil && v.ID != i.Source.VersionID && v.File.SHA1 != i.SHA1 {
				out = append(out, Update{Installed: i, Version: v})
			}
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Installed.Name() < out[b].Installed.Name() })
	return out, nil
}
