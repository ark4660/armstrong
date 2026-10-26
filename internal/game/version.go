// Package game installs and launches Minecraft: version metadata, libraries,
// assets, Mojang's Java runtimes, the Fabric loader and the launch command.
package game

import (
	"encoding/json"
	"fmt"
)

const manifestURL = "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json"

// Manifest is Mojang's version list.
type Manifest struct {
	Latest struct {
		Release  string `json:"release"`
		Snapshot string `json:"snapshot"`
	} `json:"latest"`
	Versions []ManifestEntry `json:"versions"`
}

type ManifestEntry struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	URL  string `json:"url"`
	SHA1 string `json:"sha1"`
}

// Version is a version JSON: Mojang's, or a loader profile that inheritsFrom one.
type Version struct {
	ID           string    `json:"id"`
	InheritsFrom string    `json:"inheritsFrom,omitempty"`
	Type         string    `json:"type"`
	MainClass    string    `json:"mainClass"`
	Arguments    Arguments `json:"arguments"`
	// MinecraftArguments is the pre-1.13 form of the game arguments: one
	// space-separated string, and no JVM arguments at all.
	MinecraftArguments string     `json:"minecraftArguments,omitempty"`
	Libraries          []Library  `json:"libraries"`
	AssetIndex         AssetIndex `json:"assetIndex"`
	Assets             string     `json:"assets"`
	Downloads          struct {
		Client Artifact `json:"client"`
	} `json:"downloads"`
	JavaVersion struct {
		Component    string `json:"component"`
		MajorVersion int    `json:"majorVersion"`
	} `json:"javaVersion"`
}

type Arguments struct {
	Game []Argument `json:"game"`
	JVM  []Argument `json:"jvm"`
}

// Argument is either a plain string or {rules, value} where value is a
// string or a list of strings.
type Argument struct {
	Rules  Rules
	Values []string
}

func (a *Argument) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		a.Values = []string{s}
		return nil
	}
	var obj struct {
		Rules Rules           `json:"rules"`
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("argument: %w", err)
	}
	a.Rules = obj.Rules
	if err := json.Unmarshal(obj.Value, &s); err == nil {
		a.Values = []string{s}
		return nil
	}
	return json.Unmarshal(obj.Value, &a.Values)
}

func (a Argument) MarshalJSON() ([]byte, error) {
	if len(a.Rules) == 0 && len(a.Values) == 1 {
		return json.Marshal(a.Values[0])
	}
	return json.Marshal(struct {
		Rules Rules    `json:"rules,omitempty"`
		Value []string `json:"value"`
	}{a.Rules, a.Values})
}

// Library covers both formats: Mojang's (downloads.artifact) and Fabric's
// (maven name + repository url + hashes).
type Library struct {
	Name      string `json:"name"`
	Downloads struct {
		Artifact    *Artifact            `json:"artifact,omitempty"`
		Classifiers map[string]*Artifact `json:"classifiers,omitempty"`
	} `json:"downloads"`
	Rules Rules `json:"rules,omitempty"`

	// Natives maps an OS to the classifier of its native-library jar, which
	// is extracted into the natives folder (pre-1.19 versions). "${arch}" in a
	// classifier stands for 32 or 64.
	Natives map[string]string `json:"natives,omitempty"`
	Extract struct {
		Exclude []string `json:"exclude,omitempty"`
	} `json:"extract,omitempty"`

	URL  string `json:"url,omitempty"`
	SHA1 string `json:"sha1,omitempty"`
	Size int64  `json:"size,omitempty"`
}

type Artifact struct {
	Path string `json:"path,omitempty"`
	SHA1 string `json:"sha1"`
	Size int64  `json:"size"`
	URL  string `json:"url"`
}

type AssetIndex struct {
	ID   string `json:"id"`
	SHA1 string `json:"sha1"`
	Size int64  `json:"size"`
	URL  string `json:"url"`
}

// Merge applies a loader profile (child) on top of the vanilla version
// (parent): the child's id and mainClass win, arguments are concatenated, and
// child libraries replace parent libraries with the same group:artifact:classifier.
func Merge(parent, child *Version) *Version {
	out := *parent
	out.ID = child.ID
	out.InheritsFrom = ""
	if child.MainClass != "" {
		out.MainClass = child.MainClass
	}
	if child.Type != "" {
		out.Type = child.Type
	}
	if child.MinecraftArguments != "" {
		out.MinecraftArguments = child.MinecraftArguments
	}
	if child.JavaVersion.Component != "" {
		out.JavaVersion = child.JavaVersion
	}
	out.Arguments.JVM = append(append([]Argument{}, parent.Arguments.JVM...), child.Arguments.JVM...)
	out.Arguments.Game = append(append([]Argument{}, parent.Arguments.Game...), child.Arguments.Game...)

	overridden := map[string]bool{}
	for _, l := range child.Libraries {
		overridden[libraryKey(l.Name)] = true
	}
	out.Libraries = nil
	for _, l := range parent.Libraries {
		if !overridden[libraryKey(l.Name)] {
			out.Libraries = append(out.Libraries, l)
		}
	}
	out.Libraries = append(out.Libraries, child.Libraries...)
	return &out
}
