package update

import (
	"testing"

	"golang.org/x/sys/windows/registry"
)

func TestRecordInstalledVersion(t *testing.T) {
	uninstallKey = `Software\ArmstrongTest\Uninstall\Armstrong`
	defer registry.DeleteKey(registry.CURRENT_USER, uninstallKey)
	k, _, err := registry.CreateKey(registry.CURRENT_USER, uninstallKey, registry.ALL_ACCESS)
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	k.SetStringValue("InstallLocation", `C:\Users\x\AppData\Local\Programs\Armstrong`)
	k.SetStringValue("DisplayVersion", "0.2.1")
	version := func() string { v, _, _ := k.GetStringValue("DisplayVersion"); return v }

	recordInstalledVersion(`C:\Somewhere\Else`, "0.3.0")
	if v := version(); v != "0.2.1" {
		t.Fatalf("another copy changed the entry to %s", v)
	}
	recordInstalledVersion(`c:\users\x\appdata\local\programs\armstrong\`, "0.3.0")
	if v := version(); v != "0.3.0" {
		t.Fatalf("DisplayVersion = %s, want 0.3.0", v)
	}
}
