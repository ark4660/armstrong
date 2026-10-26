//go:build !windows

package update

// recordInstalledVersion only matters for the Windows installer's entry.
func recordInstalledVersion(dir, version string) {}
