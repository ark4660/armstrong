package bench

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestOptionsOverride(t *testing.T) {
	got := Options(5023, 8, []string{"maxFps:120", `preferredGraphicsBackend:"vulkan"`})
	for _, want := range []string{"renderDistance:8\n", "maxFps:120\n", `preferredGraphicsBackend:"vulkan"` + "\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("options lack %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "maxFps:260") {
		t.Error("override kept the default too")
	}
}

func TestLoadAndCompare(t *testing.T) {
	dir := t.TempDir()
	write := func(label, name string, fps float64) {
		os.MkdirAll(filepath.Join(dir, label), 0o755)
		data := `{"scene":"static","frames":{"count":1000,"avgFps":` + strconv.FormatFloat(fps, 'f', -1, 64) + `,"low1Fps":100},
			"gc":[{"name":"G1 Young Generation","count":2,"timeMs":10},{"name":"ZGC Minor Cycles","count":5,"timeMs":99}]}`
		os.WriteFile(filepath.Join(dir, label, name), []byte(data), 0o644)
	}
	write("a", "static-1.json", 1000)
	write("a", "static-2.json", 1200)
	write("b", "static-1.json", 1650)
	results, err := Load([]string{filepath.Join(dir, "a"), filepath.Join(dir, "b")})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 || results[0].Label != "a" {
		t.Fatalf("results = %+v", results)
	}
	if n, ms := results[0].GCPauses(); n != 2 || ms != 10 {
		t.Errorf("GCPauses = %d, %d; concurrent cycles must not count", n, ms)
	}
	var out bytes.Buffer
	Compare(&out, results)
	if !strings.Contains(out.String(), "1100") || !strings.Contains(out.String(), "1650 (+50%)") {
		t.Errorf("compare output:\n%s", out.String())
	}
}
