package game

import (
	"fmt"
	"strings"
)

// coords is a parsed Maven coordinate: group:artifact:version[:classifier][@ext].
type coords struct {
	group, artifact, version, classifier, ext string
}

func parseCoords(name string) (coords, error) {
	c := coords{ext: "jar"}
	if i := strings.LastIndexByte(name, '@'); i >= 0 {
		name, c.ext = name[:i], name[i+1:]
	}
	parts := strings.Split(name, ":")
	if len(parts) < 3 || len(parts) > 4 {
		return coords{}, fmt.Errorf("bad maven coordinate %q", name)
	}
	c.group, c.artifact, c.version = parts[0], parts[1], parts[2]
	if len(parts) == 4 {
		c.classifier = parts[3]
	}
	return c, nil
}

// MavenPath turns a coordinate into its repository path,
// e.g. net.fabricmc:fabric-loader:0.20.1 → net/fabricmc/fabric-loader/0.20.1/fabric-loader-0.20.1.jar.
func MavenPath(name string) (string, error) {
	c, err := parseCoords(name)
	if err != nil {
		return "", err
	}
	file := c.artifact + "-" + c.version
	if c.classifier != "" {
		file += "-" + c.classifier
	}
	file += "." + c.ext
	return strings.ReplaceAll(c.group, ".", "/") + "/" + c.artifact + "/" + c.version + "/" + file, nil
}

// libraryKey identifies a library regardless of version, so a loader's copy
// can replace the vanilla one.
func libraryKey(name string) string {
	c, err := parseCoords(name)
	if err != nil {
		return name
	}
	return c.group + ":" + c.artifact + ":" + c.classifier
}
