package client

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sort"

	"github.com/ark4660/armstrong/internal/download"
	"github.com/ark4660/armstrong/internal/game"
	"github.com/ark4660/armstrong/internal/modmeta"
	"github.com/ark4660/armstrong/internal/modrinth"
)

// ResolveRequest says what a client release should contain.
type ResolveRequest struct {
	Client       string
	Version      string
	Minecraft    string
	FabricLoader string
	Core         []string // Modrinth slugs or ids, always enabled
	Optional     []string // add-ons
	// IfAvailable are core mods left out of Minecraft versions they have
	// no usable build for, instead of failing the whole manifest.
	IfAvailable []string
	// AllowAlpha may resolve to an alpha build when they have no release or
	// beta for the version (projects that only publish alphas).
	AllowAlpha []string
}

// maxCandidates is how many versions of a project are tried, newest first.
const maxCandidates = 10

type resolver struct {
	mr     *modrinth.Client
	dl     *download.Client
	tmp    string
	mc     string
	chosen map[string]*choice // by Modrinth project id
	order  []string

	allowAlpha []string
}

// errNoVersion: a project has no usable build for the Minecraft version.
var errNoVersion = errors.New("no usable version")

type choice struct {
	version  *modrinth.Version
	optional bool
	requires []string // project ids
}

// Resolve picks a version of every requested project (and their required
// dependencies) for the Minecraft version and builds a manifest. It prefers
// releases over betas, never picks alphas, and honours dependencies pinned to
// an exact version on Modrinth (Iris pins a Sodium build, for example). Every
// jar is downloaded into tmp to read its fabric.mod.json.
func Resolve(ctx context.Context, mr *modrinth.Client, dl *download.Client, tmp string, req ResolveRequest) (*Manifest, error) {
	r := &resolver{mr: mr, dl: dl, tmp: tmp, mc: req.Minecraft, chosen: map[string]*choice{}, allowAlpha: req.AllowAlpha}
	for _, slug := range req.IfAvailable {
		if _, err := r.resolve(ctx, slug, false); err != nil && !errors.Is(err, errNoVersion) {
			return nil, err
		}
	}
	for _, slug := range req.Core {
		if _, err := r.resolve(ctx, slug, false); err != nil {
			return nil, err
		}
	}
	for _, slug := range req.Optional {
		if _, err := r.resolve(ctx, slug, true); err != nil {
			return nil, err
		}
	}

	m := &Manifest{
		FormatVersion: FormatVersion,
		Client:        req.Client,
		Version:       req.Version,
		Minecraft:     req.Minecraft,
		FabricLoader:  req.FabricLoader,
	}
	modIDs := map[string]string{} // project id → fabric mod id
	for _, pid := range r.order {
		c := r.chosen[pid]
		f, _ := c.version.PrimaryFile()
		path := filepath.Join(tmp, f.Filename)
		if err := dl.Fetch(ctx, []download.Item{{URL: f.URL, Path: path, SHA1: f.Hashes.SHA1, Size: f.Size}}, nil); err != nil {
			return nil, err
		}
		meta, err := modmeta.ReadJar(path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.Filename, err)
		}
		if meta.Environment == "server" {
			return nil, fmt.Errorf("%s is a server-only mod", meta.ID)
		}
		modIDs[pid] = meta.ID
		proj, err := mr.Project(ctx, pid)
		if err != nil {
			return nil, err
		}
		name := proj.Title
		if name == "" {
			name = meta.ID
		}
		m.Mods = append(m.Mods, Mod{
			ID: meta.ID, Provides: meta.Provides, Name: name, Version: meta.Version,
			Optional: c.optional,
			Project:  pid, Slug: proj.Slug, VersionID: c.version.ID,
			File: f.Filename, URL: f.URL, SHA1: f.Hashes.SHA1, SHA512: f.Hashes.SHA512, Size: f.Size,
		})
	}
	for i := range m.Mods {
		for _, dep := range r.chosen[m.Mods[i].Project].requires {
			m.Mods[i].Requires = append(m.Mods[i].Requires, modIDs[dep])
		}
		sort.Strings(m.Mods[i].Requires)
	}
	return m, nil
}

// resolve chooses a version of a project and, recursively, of its required
// dependencies. A project already chosen as core stays core.
func (r *resolver) resolve(ctx context.Context, idOrSlug string, optional bool) (string, error) {
	for pid, c := range r.chosen {
		if pid == idOrSlug || c.version.ProjectID == idOrSlug {
			if !optional {
				r.markCore(pid)
			}
			return pid, nil
		}
	}
	versions, err := r.mr.Versions(ctx, idOrSlug, game.ModrinthLoader(r.mc), r.mc)
	if err != nil {
		return "", err
	}
	alpha := slices.Contains(r.allowAlpha, idOrSlug)
	for _, v := range candidates(versions, alpha) {
		if r.pinsConflict(v) {
			continue
		}
		return r.choose(ctx, v, optional)
	}
	return "", fmt.Errorf("%w: no Fabric build of %s for Minecraft %s", errNoVersion, idOrSlug, r.mc)
}

func (r *resolver) choose(ctx context.Context, v *modrinth.Version, optional bool) (string, error) {
	pid := v.ProjectID
	if c, ok := r.chosen[pid]; ok {
		if c.version.ID != v.ID {
			return "", fmt.Errorf("project %s needed at two versions (%s and %s)", pid, c.version.VersionNumber, v.VersionNumber)
		}
		if !optional {
			r.markCore(pid)
		}
		return pid, nil
	}
	if _, ok := v.PrimaryFile(); !ok {
		return "", fmt.Errorf("%s %s has no files", pid, v.VersionNumber)
	}
	c := &choice{version: v, optional: optional}
	r.chosen[pid] = c
	r.order = append(r.order, pid)
	for _, d := range v.Dependencies {
		if d.Type != "required" {
			continue
		}
		var depID string
		var err error
		if d.VersionID != "" {
			pinned, ferr := r.mr.Version(ctx, d.VersionID)
			if ferr != nil {
				return "", ferr
			}
			depID, err = r.choose(ctx, pinned, optional)
		} else {
			depID, err = r.resolve(ctx, d.ProjectID, optional)
		}
		if err != nil {
			return "", fmt.Errorf("dependency of %s: %w", v.VersionNumber, err)
		}
		c.requires = append(c.requires, depID)
	}
	return pid, nil
}

// markCore makes a project and what it requires part of the core.
func (r *resolver) markCore(pid string) {
	c := r.chosen[pid]
	if c == nil || !c.optional {
		return
	}
	c.optional = false
	for _, dep := range c.requires {
		r.markCore(dep)
	}
}

// pinsConflict reports whether v pins a dependency to a version other than
// the one already chosen.
func (r *resolver) pinsConflict(v *modrinth.Version) bool {
	for _, d := range v.Dependencies {
		if d.Type != "required" || d.VersionID == "" {
			continue
		}
		if c, ok := r.chosen[d.ProjectID]; ok && c.version.ID != d.VersionID {
			return true
		}
	}
	return false
}

// candidates orders versions: releases newest first, then betas. Alphas
// only when allowed, and only if there's nothing else.
func candidates(vs []modrinth.Version, alpha bool) []*modrinth.Version {
	var rel, beta, alphas []*modrinth.Version
	for i := range vs {
		switch vs[i].VersionType {
		case "release":
			rel = append(rel, &vs[i])
		case "beta":
			beta = append(beta, &vs[i])
		case "alpha":
			alphas = append(alphas, &vs[i])
		}
	}
	byDate := func(s []*modrinth.Version) {
		sort.SliceStable(s, func(a, b int) bool { return s[a].DatePublished > s[b].DatePublished })
	}
	byDate(rel)
	byDate(beta)
	out := append(rel, beta...)
	if len(out) == 0 && alpha {
		byDate(alphas)
		out = alphas
	}
	if len(out) > maxCandidates {
		out = out[:maxCandidates]
	}
	return out
}
