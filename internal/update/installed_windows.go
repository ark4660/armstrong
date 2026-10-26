package update

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// The entry ArmstrongSetup.exe writes for Settings → Installed apps.
var uninstallKey = `Software\Microsoft\Windows\CurrentVersion\Uninstall\Armstrong`

// recordInstalledVersion keeps Installed apps showing the running version
// after a self-update, which the installer never sees. Only the entry for
// this copy is touched: one pointing at another folder is left alone.
func recordInstalledVersion(dir, version string) {
	k, err := registry.OpenKey(registry.CURRENT_USER, uninstallKey, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return // not installed with the installer
	}
	defer k.Close()
	loc, _, err := k.GetStringValue("InstallLocation")
	if err != nil || !strings.EqualFold(filepath.Clean(loc), filepath.Clean(dir)) {
		return
	}
	k.SetStringValue("DisplayVersion", version)
}
