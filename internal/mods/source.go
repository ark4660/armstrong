package mods

import (
	"context"
	"sort"

	"github.com/ark4660/armstrong/internal/modrinth"
)

// Source is a mod site. Modrinth is the only one today; CurseForge can be
// added behind the same interface once there's an API key.
type Source interface {
	Name() string
	// Versions of a project for a Minecraft version and loader, any order.
	Versions(ctx context.Context, project, minecraft, loader string) ([]Version, error)
	Version(ctx context.Context, id string) (*Version, error)
	// Title is a project's display name.
	Title(ctx context.Context, project string) (string, error)
	// Identify maps sha1s of local files to the versions they are.
	Identify(ctx context.Context, sha1s []string) (map[string]Version, error)
	// Latest maps sha1s to the newest version of the same project.
	Latest(ctx context.Context, sha1s []string, minecraft, loader string, channels []string) (map[string]Version, error)
}

// Version is one downloadable version of a project.
type Version struct {
	ID        string
	ProjectID string
	Number    string
	Channel   string // release, beta, alpha
	Published string // RFC 3339; sorts as text
	File      File
	Deps      []Dep
}

type File struct {
	URL, Name, SHA1 string
	Size            int64
}

type Dep struct {
	ProjectID string
	VersionID string // pins an exact version when set
	Kind      string // required, optional, incompatible, embedded
}

// candidates orders versions for installing: releases newest first, then
// betas, then alphas.
func candidates(vs []Version, max int) []Version {
	rank := map[string]int{"release": 0, "beta": 1, "alpha": 2}
	out := append([]Version{}, vs...)
	sort.SliceStable(out, func(a, b int) bool {
		ra, rb := rank[out[a].Channel], rank[out[b].Channel]
		if ra != rb {
			return ra < rb
		}
		return out[a].Published > out[b].Published
	})
	if len(out) > max {
		out = out[:max]
	}
	return out
}

// Modrinth adapts the Modrinth client to Source.
type Modrinth struct {
	Client *modrinth.Client
}

func (Modrinth) Name() string { return "Modrinth" }

func (s Modrinth) Versions(ctx context.Context, project, minecraft, loader string) ([]Version, error) {
	vs, err := s.Client.Versions(ctx, project, loader, minecraft)
	if err != nil {
		return nil, err
	}
	out := make([]Version, 0, len(vs))
	for _, v := range vs {
		if c, ok := convert(v); ok {
			out = append(out, c)
		}
	}
	return out, nil
}

func (s Modrinth) Version(ctx context.Context, id string) (*Version, error) {
	v, err := s.Client.Version(ctx, id)
	if err != nil {
		return nil, err
	}
	c, ok := convert(*v)
	if !ok {
		return nil, errNoFile(v.VersionNumber)
	}
	return &c, nil
}

func (s Modrinth) Title(ctx context.Context, project string) (string, error) {
	p, err := s.Client.Project(ctx, project)
	if err != nil {
		return "", err
	}
	return p.Title, nil
}

func (s Modrinth) Identify(ctx context.Context, sha1s []string) (map[string]Version, error) {
	vs, err := s.Client.VersionsByHash(ctx, sha1s)
	return convertMap(vs), err
}

func (s Modrinth) Latest(ctx context.Context, sha1s []string, minecraft, loader string, channels []string) (map[string]Version, error) {
	vs, err := s.Client.LatestByHash(ctx, sha1s, loader, minecraft, channels)
	return convertMap(vs), err
}

func convertMap(vs map[string]modrinth.Version) map[string]Version {
	out := make(map[string]Version, len(vs))
	for hash, v := range vs {
		if c, ok := convert(v); ok {
			out[hash] = c
		}
	}
	return out
}

func convert(v modrinth.Version) (Version, bool) {
	f, ok := v.PrimaryFile()
	if !ok {
		return Version{}, false
	}
	out := Version{
		ID: v.ID, ProjectID: v.ProjectID, Number: v.VersionNumber, Channel: v.VersionType, Published: v.DatePublished,
		File: File{URL: f.URL, Name: f.Filename, SHA1: f.Hashes.SHA1, Size: f.Size},
	}
	for _, d := range v.Dependencies {
		out.Deps = append(out.Deps, Dep{ProjectID: d.ProjectID, VersionID: d.VersionID, Kind: d.Type})
	}
	return out, true
}

type errNoFile string

func (e errNoFile) Error() string { return "version " + string(e) + " has no files" }
