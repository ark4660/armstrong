// Package modrinth is a small client for the Modrinth v2 API
// (https://api.modrinth.com/v2, no key, ~300 requests per minute).
package modrinth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/ark4660/armstrong/internal/download"
)

const API = "https://api.modrinth.com/v2"

type Client struct {
	DL *download.Client
}

type Project struct {
	ID    string `json:"id"`
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

type Version struct {
	ID            string       `json:"id"`
	ProjectID     string       `json:"project_id"`
	Name          string       `json:"name"`
	VersionNumber string       `json:"version_number"`
	VersionType   string       `json:"version_type"` // release, beta, alpha
	DatePublished string       `json:"date_published"`
	GameVersions  []string     `json:"game_versions"`
	Loaders       []string     `json:"loaders"`
	Files         []File       `json:"files"`
	Dependencies  []Dependency `json:"dependencies"`
}

type File struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
	Primary  bool   `json:"primary"`
	Size     int64  `json:"size"`
	Hashes   struct {
		SHA1   string `json:"sha1"`
		SHA512 string `json:"sha512"`
	} `json:"hashes"`
}

type Dependency struct {
	ProjectID string `json:"project_id"`
	VersionID string `json:"version_id"`      // set when the dependency pins an exact version
	Type      string `json:"dependency_type"` // required, optional, incompatible, embedded
}

// PrimaryFile is the version's primary file, else its first.
func (v *Version) PrimaryFile() (File, bool) {
	for _, f := range v.Files {
		if f.Primary {
			return f, true
		}
	}
	if len(v.Files) > 0 {
		return v.Files[0], true
	}
	return File{}, false
}

func (c *Client) Project(ctx context.Context, idOrSlug string) (*Project, error) {
	var p Project
	if err := c.DL.GetJSON(ctx, API+"/project/"+url.PathEscape(idOrSlug), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// Versions lists a project's versions for a loader and game version, newest first.
func (c *Client) Versions(ctx context.Context, idOrSlug, loader, gameVersion string) ([]Version, error) {
	q := url.Values{}
	q.Set("loaders", jsonList(loader))
	q.Set("game_versions", jsonList(gameVersion))
	var vs []Version
	err := c.DL.GetJSON(ctx, API+"/project/"+url.PathEscape(idOrSlug)+"/version?"+q.Encode(), &vs)
	if err != nil {
		return nil, fmt.Errorf("modrinth versions of %s: %w", idOrSlug, err)
	}
	return vs, nil
}

func (c *Client) Version(ctx context.Context, id string) (*Version, error) {
	var v Version
	if err := c.DL.GetJSON(ctx, API+"/version/"+url.PathEscape(id), &v); err != nil {
		return nil, err
	}
	return &v, nil
}

func jsonList(s string) string {
	b, _ := json.Marshal([]string{s})
	return string(b)
}

// SearchQuery is a mod search for one Minecraft version and loader.
type SearchQuery struct {
	Text      string
	Minecraft string
	Loader    string
	Category  string // empty: any
	Index     string // relevance, downloads, follows, newest, updated
	Offset    int
	Limit     int
}

type SearchHit struct {
	ProjectID    string   `json:"project_id"`
	Slug         string   `json:"slug"`
	Title        string   `json:"title"`
	Author       string   `json:"author"`
	Description  string   `json:"description"`
	Downloads    int      `json:"downloads"`
	Follows      int      `json:"follows"`
	Categories   []string `json:"display_categories"`
	DateModified string   `json:"date_modified"`
}

type SearchResult struct {
	Hits      []SearchHit `json:"hits"`
	Offset    int         `json:"offset"`
	TotalHits int         `json:"total_hits"`
}

// Search finds client-usable mods (server-only ones are left out).
func (c *Client) Search(ctx context.Context, q SearchQuery) (*SearchResult, error) {
	facets := [][]string{
		{"project_type:mod"},
		{"categories:" + q.Loader},
		{"versions:" + q.Minecraft},
		{"environment!=server_only"},
		{"environment!=dedicated_server_only"},
	}
	if q.Category != "" {
		facets = append(facets, []string{"categories:" + q.Category})
	}
	f, _ := json.Marshal(facets)
	v := url.Values{}
	v.Set("query", q.Text)
	v.Set("facets", string(f))
	if q.Index != "" {
		v.Set("index", q.Index)
	}
	v.Set("offset", fmt.Sprint(q.Offset))
	limit := q.Limit
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	v.Set("limit", fmt.Sprint(limit))
	var r SearchResult
	if err := c.DL.GetJSON(ctx, API+"/search?"+v.Encode(), &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// ProjectDetails is a project page: the long description is Markdown.
type ProjectDetails struct {
	Project
	Description string   `json:"description"`
	Body        string   `json:"body"`
	Downloads   int      `json:"downloads"`
	Followers   int      `json:"followers"`
	Categories  []string `json:"categories"`
	Updated     string   `json:"updated"`
	License     struct {
		ID string `json:"id"`
	} `json:"license"`
	SourceURL string `json:"source_url"`
}

func (c *Client) ProjectDetails(ctx context.Context, idOrSlug string) (*ProjectDetails, error) {
	var p ProjectDetails
	if err := c.DL.GetJSON(ctx, API+"/project/"+url.PathEscape(idOrSlug), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// VersionsByHash looks files up by sha1: which version (and project) they are.
func (c *Client) VersionsByHash(ctx context.Context, sha1s []string) (map[string]Version, error) {
	out := map[string]Version{}
	if len(sha1s) == 0 {
		return out, nil
	}
	err := c.DL.PostJSON(ctx, API+"/version_files", map[string]any{"hashes": sha1s, "algorithm": "sha1"}, &out)
	return out, err
}

// LatestByHash returns, per sha1, the newest version of that file's project
// for the loader and game version, limited to the given release channels.
func (c *Client) LatestByHash(ctx context.Context, sha1s []string, loader, gameVersion string, types []string) (map[string]Version, error) {
	out := map[string]Version{}
	if len(sha1s) == 0 {
		return out, nil
	}
	body := map[string]any{
		"hashes": sha1s, "algorithm": "sha1",
		"loaders": []string{loader}, "game_versions": []string{gameVersion},
	}
	if len(types) > 0 {
		body["version_types"] = types
	}
	err := c.DL.PostJSON(ctx, API+"/version_files/update", body, &out)
	return out, err
}

// Categories lists the mod categories search can filter by.
func (c *Client) Categories(ctx context.Context) ([]string, error) {
	var tags []struct {
		Name        string `json:"name"`
		ProjectType string `json:"project_type"`
		Header      string `json:"header"`
	}
	if err := c.DL.GetJSON(ctx, API+"/tag/category", &tags); err != nil {
		return nil, err
	}
	var out []string
	for _, t := range tags {
		if t.ProjectType == "mod" && t.Header == "categories" {
			out = append(out, t.Name)
		}
	}
	return out, nil
}
