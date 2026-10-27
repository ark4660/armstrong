package game

import (
	"archive/zip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ark4660/armstrong/internal/app"
)

func TestJavaMajor(t *testing.T) {
	for release, want := range map[string]int{
		`JAVA_VERSION="25.0.1"`:   25,
		`JAVA_VERSION="21.0.7"`:   21,
		`JAVA_VERSION="1.8.0_51"`: 8,
	} {
		root := t.TempDir()
		os.MkdirAll(filepath.Join(root, "bin"), 0o755)
		os.WriteFile(filepath.Join(root, "release"), []byte("IMPLEMENTOR=\"x\"\n"+release+"\n"), 0o644)
		if got := JavaMajor(filepath.Join(root, "bin", "java")); got != want {
			t.Errorf("%s: JavaMajor = %d, want %d", release, got, want)
		}
	}
	if got := JavaMajor(filepath.Join(t.TempDir(), "bin", "java")); got != 0 {
		t.Errorf("no release file: JavaMajor = %d, want 0", got)
	}
}

func TestJVMFlags(t *testing.T) {
	if f := JVMFlags(PresetNone, 25, 4096); f != nil {
		t.Errorf("none: %q", f)
	}
	if f := JVMFlags(PresetDefault, 25, 4096); !slices.Contains(f, "-XX:+UseG1GC") || !slices.Contains(f, "-Xms4096M") || !slices.Contains(f, "-XX:+UseCompactObjectHeaders") {
		t.Errorf("default on 25: %q", f)
	}
	if f := JVMFlags(PresetDefault, 21, 4096); slices.Contains(f, "-XX:+UseCompactObjectHeaders") {
		t.Errorf("compact headers need Java 25: %q", f)
	}
	if f := JVMFlags(PresetZGC, 21, 4096); !slices.Contains(f, "-XX:+ZGenerational") {
		t.Errorf("zgc on 21 needs generational mode: %q", f)
	}
	if f := JVMFlags(PresetZGC, 25, 4096); slices.Contains(f, "-XX:+ZGenerational") {
		t.Errorf("zgc on 25 is generational already: %q", f)
	}
}

func TestTunedJVMArgsLetsThePlayerPickTheGC(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "release"), []byte(`JAVA_VERSION="25.0.1"`+"\n"), 0o644)
	java := filepath.Join(root, "bin", "java")

	args := TunedJVMArgs(java, PresetDefault, 4096, []string{"-XX:+UseZGC"})
	if slices.Contains(args, "-XX:+UseG1GC") {
		t.Errorf("preset GC flags kept next to the player's: %q", args)
	}
	if !slices.Contains(args, "-XX:+UseCompactObjectHeaders") || args[len(args)-1] != "-XX:+UseZGC" {
		t.Errorf("args = %q", args)
	}
	if args := TunedJVMArgs(java, PresetDefault, 4096, []string{"-Dfoo=1"}); !slices.Contains(args, "-XX:+UseG1GC") {
		t.Errorf("no GC chosen by the player: %q", args)
	}
}

func TestCommandExtras(t *testing.T) {
	root := t.TempDir()
	inst := &Installation{Version: &Version{ID: "x", MainClass: "Main"}, Java: "java"}
	cmd, err := Command(inst, LaunchOptions{
		Paths: app.Paths{Root: root}, GameDir: filepath.Join(root, "game"), Player: OfflinePlayer("Steve"),
		GameArgs: []string{"--width", "800"}, Env: []string{"FOO=1"}, Wrapper: []string{"nice", "-n", "5"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"nice", "-n", "5", "java"}; !slices.Equal(cmd.Args[:4], want) {
		t.Errorf("args start %q, want %q", cmd.Args[:4], want)
	}
	if n := len(cmd.Args); cmd.Args[n-2] != "--width" || cmd.Args[n-1] != "800" {
		t.Errorf("game args not last: %q", cmd.Args)
	}
	if !slices.Contains(cmd.Env, "FOO=1") {
		t.Error("env not passed")
	}
}

func TestSeedOptionsOnlyOnce(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "game")
	if err := SeedOptions(dir, ClientInfo{DataVersion: 5023, Backends: true}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "options.txt"))
	if !strings.HasPrefix(string(data), "version:5023\n") || !strings.Contains(string(data), "enableVsync:false\n") ||
		!strings.Contains(string(data), `preferredGraphicsBackend:"vulkan"`) {
		t.Errorf("options.txt = %q", data)
	}
	os.WriteFile(filepath.Join(dir, "options.txt"), []byte("renderDistance:4\n"), 0o644)
	if err := SeedOptions(dir, ClientInfo{DataVersion: 5023, Backends: true}); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "options.txt")); string(data) != "renderDistance:4\n" {
		t.Errorf("existing options overwritten: %q", data)
	}
}

func TestSeedOptionsOldVersion(t *testing.T) {
	dir := t.TempDir()
	if err := SeedOptions(dir, ClientInfo{}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "options.txt"))
	if strings.Contains(string(data), "version:") || strings.Contains(string(data), "preferredGraphicsBackend") {
		t.Errorf("options.txt for a version without data version or backends: %q", data)
	}
}

func TestBackendArgs(t *testing.T) {
	modern := ClientInfo{DataVersion: 5023, Backends: true}
	if got := BackendArgs(modern, true); !slices.Equal(got, []string{"--graphicsBackend", "opengl"}) {
		t.Errorf("shaders on 26.x: %q", got)
	}
	if got := BackendArgs(modern, false); got != nil {
		t.Errorf("no shaders: %q", got)
	}
	if got := BackendArgs(ClientInfo{}, true); got != nil {
		t.Errorf("version without backends: %q", got)
	}
}

func TestReadClientInfo(t *testing.T) {
	root := t.TempDir()
	paths := app.Paths{Root: root}
	jar := filepath.Join(paths.Versions(), "26.3", "26.3.jar")
	os.MkdirAll(filepath.Dir(jar), 0o755)
	f, _ := os.Create(jar)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("version.json")
	w.Write([]byte(`{"id":"26.3","world_version":5023}`))
	zw.Create("net/minecraft/client/PreferredGraphicsApi.class")
	zw.Close()
	f.Close()
	if got := ReadClientInfo(paths, "26.3"); got != (ClientInfo{DataVersion: 5023, Backends: true}) {
		t.Errorf("ReadClientInfo = %+v", got)
	}
	if got := ReadClientInfo(paths, "1.8.9"); got != (ClientInfo{}) {
		t.Errorf("missing jar: %+v", got)
	}
}
