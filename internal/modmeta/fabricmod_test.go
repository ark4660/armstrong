package modmeta

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func makeJar(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write(data)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestReadWithNestedJars(t *testing.T) {
	inner := makeJar(t, map[string][]byte{
		"fabric.mod.json": []byte(`{"id":"inner","version":"2.0"}`),
	})
	outer := makeJar(t, map[string][]byte{
		"fabric.mod.json": []byte("\xef\xbb\xbf" + `{
			"schemaVersion": 1, "id": "outer", "version": "1.0", "environment": "client",
			"provides": ["outer_alias"],
			"depends": {"fabricloader": ">=0.15", "minecraft": ["26.3", "26.2"]},
			"breaks": {"optifabric": "*"},
			"jars": [{"file": "META-INF/jars/inner.jar"}, {"file": "META-INF/jars/missing.jar"}]
		}`),
		"META-INF/jars/inner.jar": inner,
	})
	mod, err := Read(bytes.NewReader(outer), int64(len(outer)))
	if err != nil {
		t.Fatal(err)
	}
	if mod.ID != "outer" || mod.Environment != "client" {
		t.Errorf("mod = %+v", mod)
	}
	if !reflect.DeepEqual(mod.IDs(), []string{"outer", "outer_alias"}) {
		t.Errorf("IDs = %v", mod.IDs())
	}
	if !reflect.DeepEqual(mod.Depends["minecraft"], []string{"26.3", "26.2"}) || !reflect.DeepEqual(mod.Depends["fabricloader"], []string{">=0.15"}) {
		t.Errorf("depends = %v", mod.Depends)
	}
	if len(mod.Breaks["optifabric"]) != 1 {
		t.Errorf("breaks = %v", mod.Breaks)
	}
	if len(mod.Nested) != 1 || mod.Nested[0].ID != "inner" {
		t.Errorf("nested = %+v", mod.Nested)
	}
}

func TestReadNotAMod(t *testing.T) {
	jar := makeJar(t, map[string][]byte{"META-INF/MANIFEST.MF": []byte("Manifest-Version: 1.0\n")})
	if _, err := Read(bytes.NewReader(jar), int64(len(jar))); !errors.Is(err, ErrNotAMod) {
		t.Errorf("err = %v, want ErrNotAMod", err)
	}
}

// TestRealJars reads every jar in ARMSTRONG_TEST_MODS, if set: a real mods
// folder is the best check that the parser copes with what's out there.
func TestRealJars(t *testing.T) {
	dir := os.Getenv("ARMSTRONG_TEST_MODS")
	if dir == "" {
		t.Skip("ARMSTRONG_TEST_MODS not set")
	}
	jars, _ := filepath.Glob(filepath.Join(dir, "*.jar"))
	if len(jars) == 0 {
		t.Fatalf("no jars in %s", dir)
	}
	for _, j := range jars {
		mod, err := ReadJar(j)
		if err != nil {
			t.Errorf("%s: %v", filepath.Base(j), err)
			continue
		}
		t.Logf("%-40s %s %s (%d nested)", filepath.Base(j), mod.ID, mod.Version, len(mod.Nested))
	}
}

// A nested jar stored uncompressed (the usual jar-in-jar layout) is read in
// place from the outer jar.
func TestReadStoredNestedJar(t *testing.T) {
	inner := makeJar(t, map[string][]byte{
		"fabric.mod.json": []byte(`{"id":"inner","version":"2.0"}`),
	})
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("fabric.mod.json")
	w.Write([]byte(`{"id":"outer","version":"1.0","jars":[{"file":"META-INF/jars/inner.jar"}]}`))
	w, _ = zw.CreateHeader(&zip.FileHeader{Name: "META-INF/jars/inner.jar", Method: zip.Store})
	w.Write(inner)
	zw.Close()
	mod, err := Read(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(mod.Nested) != 1 || mod.Nested[0].ID != "inner" {
		t.Fatalf("nested = %+v", mod.Nested)
	}
}
