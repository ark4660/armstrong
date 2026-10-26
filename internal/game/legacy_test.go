package game

import (
	"archive/zip"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ark4660/armstrong/internal/app"
)

func TestIsLegacyFabric(t *testing.T) {
	tests := map[string]bool{
		"1.8.9": true, "1.12.2": true, "1.13.2": true,
		"1.14": false, "1.21.1": false, "26.3": false, "24w14a": false,
	}
	for mc, want := range tests {
		if got := IsLegacyFabric(mc); got != want {
			t.Errorf("IsLegacyFabric(%q) = %v, want %v", mc, got, want)
		}
	}
	if ModrinthLoader("1.8.9") != "legacy-fabric" || ModrinthLoader("1.21.1") != "fabric" {
		t.Error("ModrinthLoader picks the wrong tag")
	}
}

func TestLegacyCommand(t *testing.T) {
	root := t.TempDir()
	inst := &Installation{
		Version: &Version{
			ID:                 "fabric-loader-0.20.1-1.8.9",
			MainClass:          "Knot",
			MinecraftArguments: "--username ${auth_player_name} --userProperties ${user_properties}",
		},
		Java:      "java",
		Classpath: []string{"a.jar", "b.jar"},
	}
	cmd, err := Command(inst, LaunchOptions{
		Paths: app.Paths{Root: root}, GameDir: filepath.Join(root, "game"),
		Player: OfflinePlayer("Steve"), Server: "mc.example.org:25570",
	})
	if err != nil {
		t.Fatal(err)
	}
	args := cmd.Args[1:]
	natives := "-Djava.library.path=" + filepath.Join(root, "natives", inst.Version.ID)
	for _, want := range []string{natives, "-cp", "Knot", "Steve", "{}", "--server", "mc.example.org", "--port", "25570"} {
		if !slices.Contains(args, want) {
			t.Errorf("args %q lack %q", args, want)
		}
	}
	if slices.Index(args, "-cp") > slices.Index(args, "Knot") {
		t.Error("classpath must come before the main class")
	}
}

func writeZip(t *testing.T, path string, files ...string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for _, name := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(name))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
}

func TestExtractNatives(t *testing.T) {
	dir := t.TempDir()
	jar := filepath.Join(dir, "natives.jar")
	writeZip(t, jar, "META-INF/MANIFEST.MF", "liblwjgl.so", "sub/libopenal.so")
	out := filepath.Join(dir, "out")
	if err := extractNatives([]nativeJar{{jar, []string{"META-INF/"}}}, out); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"liblwjgl.so", "sub/libopenal.so"} {
		if _, err := os.Stat(filepath.Join(out, f)); err != nil {
			t.Errorf("%s not extracted: %v", f, err)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "META-INF")); err == nil {
		t.Error("excluded META-INF was extracted")
	}

	evil := filepath.Join(dir, "evil.jar")
	writeZip(t, evil, "../escape.so")
	if err := extractNatives([]nativeJar{{evil, nil}}, out); err == nil {
		t.Error("a path outside the natives folder was accepted")
	}
}

func TestNativeItem(t *testing.T) {
	in := &Installer{Paths: app.Paths{Root: "/data"}}
	lib := Library{
		Name:    "org.lwjgl.lwjgl:lwjgl-platform:2.9.4+legacyfabric.17",
		URL:     "https://maven.legacyfabric.net/",
		Natives: map[string]string{"linux": "natives-linux", "windows": "natives-windows-${arch}"},
	}
	it, ok, err := in.nativeItem(lib, Env{OS: "linux", Arch: "x86_64"})
	want := "https://maven.legacyfabric.net/org/lwjgl/lwjgl/lwjgl-platform/2.9.4+legacyfabric.17/lwjgl-platform-2.9.4+legacyfabric.17-natives-linux.jar"
	if err != nil || !ok || it.URL != want {
		t.Errorf("linux: %+v %v %v", it, ok, err)
	}
	it, _, _ = in.nativeItem(lib, Env{OS: "windows", Arch: "x86"})
	if filepath.Base(it.Path) != "lwjgl-platform-2.9.4+legacyfabric.17-natives-windows-32.jar" {
		t.Errorf("windows x86: %s", it.Path)
	}
	if _, ok, _ := in.nativeItem(lib, Env{OS: "osx"}); ok {
		t.Error("osx has no natives here")
	}
}
