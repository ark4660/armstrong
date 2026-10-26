package game

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/ark4660/armstrong/internal/download"
)

const javaRuntimesURL = "https://launchermeta.mojang.com/v1/products/java-runtime/2ec0cc96c44e5a76b9c8b7c39df7210883d12871/all.json"

type runtimeEntry struct {
	Manifest struct {
		SHA1 string `json:"sha1"`
		URL  string `json:"url"`
	} `json:"manifest"`
	Version struct {
		Name string `json:"name"`
	} `json:"version"`
}

type runtimeManifest struct {
	Files map[string]struct {
		Type       string `json:"type"` // "file", "directory", "link"
		Executable bool   `json:"executable"`
		Target     string `json:"target"`
		Downloads  struct {
			Raw download.Item `json:"raw"`
		} `json:"downloads"`
	} `json:"files"`
}

// javaPlatform is Mojang's key for this OS/architecture in the runtime list.
func javaPlatform() (string, error) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "linux/amd64":
		return "linux", nil
	case "linux/386":
		return "linux-i386", nil
	case "darwin/amd64":
		return "mac-os", nil
	case "darwin/arm64":
		return "mac-os-arm64", nil
	case "windows/amd64":
		return "windows-x64", nil
	case "windows/arm64":
		return "windows-arm64", nil
	case "windows/386":
		return "windows-x86", nil
	}
	return "", fmt.Errorf("Mojang has no Java runtime for %s/%s", runtime.GOOS, runtime.GOARCH)
}

// InstallJava installs Mojang's Java runtime component (e.g. java-runtime-epsilon)
// and returns the path of the java executable.
func (in *Installer) InstallJava(ctx context.Context, component string) (string, error) {
	platform, err := javaPlatform()
	if err != nil {
		return "", err
	}
	var all map[string]map[string][]runtimeEntry
	if err := in.DL.CachedJSON(ctx, javaRuntimesURL, filepath.Join(in.Paths.Runtimes(), "all.json"), "", &all); err != nil {
		return "", fmt.Errorf("java runtime list: %w", err)
	}
	entries := all[platform][component]
	if len(entries) == 0 {
		return "", fmt.Errorf("no %s runtime for %s", component, platform)
	}
	entry := entries[0]

	var man runtimeManifest
	manPath := filepath.Join(in.Paths.Runtimes(), "manifests", component+".json")
	if err := in.DL.CachedJSON(ctx, entry.Manifest.URL, manPath, entry.Manifest.SHA1, &man); err != nil {
		return "", fmt.Errorf("java runtime manifest: %w", err)
	}

	root := filepath.Join(in.Paths.Runtimes(), component)
	var items []download.Item
	for name, f := range man.Files {
		path := filepath.Join(root, filepath.FromSlash(name))
		switch f.Type {
		case "directory":
			if err := os.MkdirAll(path, 0o755); err != nil {
				return "", err
			}
		case "file":
			it := f.Downloads.Raw
			it.Path = path
			it.Executable = f.Executable
			items = append(items, it)
		}
	}
	in.step(fmt.Sprintf("Java %s", entry.Version.Name))
	if err := in.DL.Fetch(ctx, items, in.progress); err != nil {
		return "", fmt.Errorf("java runtime: %w", err)
	}
	for name, f := range man.Files {
		path := filepath.Join(root, filepath.FromSlash(name))
		switch {
		case f.Type == "file" && f.Executable && runtime.GOOS != "windows":
			if err := os.Chmod(path, 0o755); err != nil {
				return "", err
			}
		case f.Type == "link" && runtime.GOOS != "windows":
			if _, err := os.Lstat(path); err == nil {
				continue
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return "", err
			}
			if err := os.Symlink(f.Target, path); err != nil {
				return "", err
			}
		}
	}

	switch runtime.GOOS {
	case "windows":
		return filepath.Join(root, "bin", "javaw.exe"), nil
	case "darwin":
		return filepath.Join(root, "jre.bundle", "Contents", "Home", "bin", "java"), nil
	}
	return filepath.Join(root, "bin", "java"), nil
}
