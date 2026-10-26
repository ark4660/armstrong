package game

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ark4660/armstrong/internal/app"
	"github.com/ark4660/armstrong/internal/download"
)

const assetsURL = "https://resources.download.minecraft.net/"

// Installer installs everything a version needs into the shared data directory.
type Installer struct {
	Paths app.Paths
	DL    *download.Client

	// OnStep and OnProgress are optional hooks for showing progress.
	OnStep     func(label string)
	OnProgress func(download.Progress)
}

func (in *Installer) step(label string) {
	if in.OnStep != nil {
		in.OnStep(label)
	}
}

func (in *Installer) progress(p download.Progress) {
	if in.OnProgress != nil {
		in.OnProgress(p)
	}
}

const (
	LoaderFabric  = "fabric"
	LoaderVanilla = "vanilla"
)

// Spec says what to install.
type Spec struct {
	Minecraft     string
	Loader        string // LoaderFabric or LoaderVanilla
	LoaderVersion string // empty: latest stable
}

// A loader layers itself on top of vanilla as a version profile that
// inheritsFrom the vanilla version. Vanilla itself has no loader.
type loader interface {
	latest(ctx context.Context, in *Installer, mc string) (string, error)
	profile(ctx context.Context, in *Installer, mc, version string) (*Version, error)
	supports(ctx context.Context, in *Installer) (map[string]bool, error)
}

var loaders = map[string]loader{
	LoaderFabric: fabricLoader{},
}

// Installation is a fully installed, launchable version.
type Installation struct {
	Version       *Version // vanilla, merged with the loader profile if any
	Minecraft     string   // vanilla version id
	Loader        string
	LoaderVersion string
	Java          string
	Classpath     []string
}

// Install installs a Minecraft version with its loader, Java runtime,
// libraries and assets.
func (in *Installer) Install(ctx context.Context, spec Spec) (*Installation, error) {
	in.step("Version metadata")
	mc := spec.Minecraft
	vanilla, err := in.vanilla(ctx, mc)
	if err != nil {
		return nil, err
	}
	v, loaderVersion := vanilla, ""
	if spec.Loader != LoaderVanilla {
		l, ok := loaders[spec.Loader]
		if !ok {
			return nil, fmt.Errorf("unsupported loader %q", spec.Loader)
		}
		loaderVersion = spec.LoaderVersion
		if loaderVersion == "" {
			if loaderVersion, err = l.latest(ctx, in, mc); err != nil {
				return nil, err
			}
		}
		profile, err := l.profile(ctx, in, mc, loaderVersion)
		if err != nil {
			return nil, err
		}
		v = Merge(vanilla, profile)
	}

	java, err := in.InstallJava(ctx, v.JavaVersion.Component)
	if err != nil {
		return nil, err
	}

	env := CurrentEnv()
	var items []download.Item
	var classpath []string
	var natives []nativeJar
	seen := map[string]bool{}
	for _, lib := range v.Libraries {
		if !lib.Rules.Allowed(env) {
			continue
		}
		if lib.Natives != nil {
			it, ok, err := in.nativeItem(lib, env)
			if err != nil {
				return nil, err
			}
			if ok && !seen[it.Path] {
				seen[it.Path] = true
				natives = append(natives, nativeJar{it.Path, lib.Extract.Exclude})
				if it.URL != "" {
					items = append(items, it)
				}
			}
			// Old versions list native jars as their own libraries with no
			// main artifact; they're extracted, never put on the classpath.
			if lib.Downloads.Artifact == nil {
				continue
			}
		}
		it, err := in.libraryItem(lib)
		if err != nil {
			return nil, err
		}
		if seen[it.Path] {
			continue
		}
		seen[it.Path] = true
		classpath = append(classpath, it.Path)
		if it.URL != "" {
			items = append(items, it)
		}
	}
	clientJar := filepath.Join(in.Paths.Versions(), mc, mc+".jar")
	c := vanilla.Downloads.Client
	items = append(items, download.Item{URL: c.URL, Path: clientJar, SHA1: c.SHA1, Size: c.Size})
	classpath = append(classpath, clientJar)

	in.step("Libraries")
	if err := in.DL.Fetch(ctx, items, in.progress); err != nil {
		return nil, fmt.Errorf("libraries: %w", err)
	}

	if len(natives) > 0 {
		in.step("Native libraries")
		if err := extractNatives(natives, in.Paths.Natives(v.ID)); err != nil {
			return nil, err
		}
	}

	in.step("Assets")
	if err := in.installAssets(ctx, v.AssetIndex); err != nil {
		return nil, err
	}

	return &Installation{
		Version: v, Minecraft: mc, Loader: spec.Loader, LoaderVersion: loaderVersion,
		Java: java, Classpath: classpath,
	}, nil
}

// LatestLoader returns the newest stable version of a loader for mc.
func (in *Installer) LatestLoader(ctx context.Context, loaderName, mc string) (string, error) {
	l, ok := loaders[loaderName]
	if !ok {
		return "", fmt.Errorf("unsupported loader %q", loaderName)
	}
	return l.latest(ctx, in, mc)
}

func (in *Installer) manifest(ctx context.Context) (*Manifest, error) {
	var man Manifest
	if err := in.DL.CachedJSON(ctx, manifestURL, filepath.Join(in.Paths.Versions(), "version_manifest_v2.json"), "", &man); err != nil {
		return nil, fmt.Errorf("version manifest: %w", err)
	}
	return &man, nil
}

// OldestSupported is the first Minecraft release GameVersions lists.
const OldestSupported = "1.20"

// GameVersions lists the releases from OldestSupported on that loader
// supports, newest first.
func (in *Installer) GameVersions(ctx context.Context, loaderName string) ([]string, error) {
	man, err := in.manifest(ctx)
	if err != nil {
		return nil, err
	}
	var supported map[string]bool
	if loaderName != LoaderVanilla {
		l, ok := loaders[loaderName]
		if !ok {
			return nil, fmt.Errorf("unsupported loader %q", loaderName)
		}
		if supported, err = l.supports(ctx, in); err != nil {
			return nil, err
		}
	}
	var out []string
	for _, e := range man.Versions { // newest first
		if e.Type != "release" || (supported != nil && !supported[e.ID]) {
			continue
		}
		out = append(out, e.ID)
		if e.ID == OldestSupported {
			break
		}
	}
	return out, nil
}

func (in *Installer) vanilla(ctx context.Context, mc string) (*Version, error) {
	man, err := in.manifest(ctx)
	if err != nil {
		return nil, err
	}
	for _, e := range man.Versions {
		if e.ID == mc {
			var v Version
			if err := in.DL.CachedJSON(ctx, e.URL, filepath.Join(in.Paths.Versions(), mc, mc+".json"), e.SHA1, &v); err != nil {
				return nil, fmt.Errorf("version %s: %w", mc, err)
			}
			return &v, nil
		}
	}
	return nil, fmt.Errorf("unknown Minecraft version %q", mc)
}

func (in *Installer) libraryItem(lib Library) (download.Item, error) {
	if a := lib.Downloads.Artifact; a != nil {
		path := a.Path
		if path == "" {
			p, err := MavenPath(lib.Name)
			if err != nil {
				return download.Item{}, err
			}
			path = p
		}
		return download.Item{URL: a.URL, Path: filepath.Join(in.Paths.Libraries(), filepath.FromSlash(path)), SHA1: a.SHA1, Size: a.Size}, nil
	}
	path, err := MavenPath(lib.Name)
	if err != nil {
		return download.Item{}, err
	}
	it := download.Item{Path: filepath.Join(in.Paths.Libraries(), filepath.FromSlash(path)), SHA1: lib.SHA1, Size: lib.Size}
	if lib.URL != "" {
		it.URL = lib.URL + path
	}
	return it, nil
}

// nativeItem is the native-library jar of lib for this OS, if it has one.
func (in *Installer) nativeItem(lib Library, env Env) (download.Item, bool, error) {
	classifier, ok := lib.Natives[env.OS]
	if !ok {
		return download.Item{}, false, nil
	}
	bits := "64"
	if env.Arch == "x86" {
		bits = "32"
	}
	classifier = strings.ReplaceAll(classifier, "${arch}", bits)
	if a := lib.Downloads.Classifiers[classifier]; a != nil {
		path := a.Path
		if path == "" {
			p, err := MavenPath(lib.Name + ":" + classifier)
			if err != nil {
				return download.Item{}, false, err
			}
			path = p
		}
		return download.Item{URL: a.URL, Path: filepath.Join(in.Paths.Libraries(), filepath.FromSlash(path)), SHA1: a.SHA1, Size: a.Size}, true, nil
	}
	// Maven-style library (a loader profile): the classifier is part of the path.
	path, err := MavenPath(lib.Name + ":" + classifier)
	if err != nil {
		return download.Item{}, false, err
	}
	it := download.Item{Path: filepath.Join(in.Paths.Libraries(), filepath.FromSlash(path))}
	if lib.URL != "" {
		it.URL = lib.URL + path
	}
	return it, true, nil
}

type nativeJar struct {
	path    string
	exclude []string // path prefixes not to extract (META-INF/)
}

// extractNatives unpacks native-library jars into dir, replacing what's there.
func extractNatives(jars []nativeJar, dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, j := range jars {
		if err := extractJar(j, dir); err != nil {
			return fmt.Errorf("natives %s: %w", filepath.Base(j.path), err)
		}
	}
	return nil
}

func extractJar(j nativeJar, dir string) error {
	zr, err := zip.OpenReader(j.path)
	if err != nil {
		return err
	}
	defer zr.Close()
files:
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		for _, ex := range j.exclude {
			if strings.HasPrefix(f.Name, ex) {
				continue files
			}
		}
		// Only plain relative names; never write outside dir.
		name := filepath.FromSlash(f.Name)
		if !filepath.IsLocal(name) {
			return fmt.Errorf("unsafe path %q", f.Name)
		}
		dst := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := writeZipFile(f, dst); err != nil {
			return err
		}
	}
	return nil
}

func writeZipFile(f *zip.File, dst string) error {
	r, err := f.Open()
	if err != nil {
		return err
	}
	defer r.Close()
	w, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, r); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}

type assetObjects struct {
	Objects map[string]struct {
		Hash string `json:"hash"`
		Size int64  `json:"size"`
	} `json:"objects"`
}

func (in *Installer) installAssets(ctx context.Context, idx AssetIndex) error {
	var objs assetObjects
	path := filepath.Join(in.Paths.Assets(), "indexes", idx.ID+".json")
	if err := in.DL.CachedJSON(ctx, idx.URL, path, idx.SHA1, &objs); err != nil {
		return fmt.Errorf("asset index: %w", err)
	}
	seen := map[string]bool{}
	var items []download.Item
	for _, o := range objs.Objects {
		if seen[o.Hash] {
			continue
		}
		seen[o.Hash] = true
		rel := o.Hash[:2] + "/" + o.Hash
		items = append(items, download.Item{
			URL:  assetsURL + rel,
			Path: filepath.Join(in.Paths.Assets(), "objects", filepath.FromSlash(rel)),
			SHA1: o.Hash,
			Size: o.Size,
		})
	}
	if err := in.DL.Fetch(ctx, items, in.progress); err != nil {
		return fmt.Errorf("assets: %w", err)
	}
	return nil
}

// ReadVersion reads an installed vanilla version JSON from disk (no network).
func ReadVersion(paths app.Paths, mc string) (*Version, error) {
	data, err := os.ReadFile(filepath.Join(paths.Versions(), mc, mc+".json"))
	if err != nil {
		return nil, err
	}
	var v Version
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return &v, nil
}
