package mods

import "testing"

func TestCompareVersions(t *testing.T) {
	ordered := []string{
		"0.9.2-alpha", "0.9.2-alpha.1", "0.9.2-alpha.beta", "0.9.2-beta", "0.9.2-beta.2", "0.9.2-beta.11",
		"0.9.2-rc.1", "0.9.2", "0.9.3", "0.10", "1.0.0", "26.3",
	}
	for i := 1; i < len(ordered); i++ {
		if c, ok := CompareVersions(ordered[i-1], ordered[i]); !ok || c >= 0 {
			t.Errorf("%s should be < %s (got %d, %v)", ordered[i-1], ordered[i], c, ok)
		}
	}
	if c, _ := CompareVersions("0.9.2+mc26.3", "0.9.2"); c != 0 {
		t.Error("build metadata must be ignored")
	}
	if c, _ := CompareVersions("1.2", "1.2.0"); c != 0 {
		t.Error("missing components count as 0")
	}
	if _, ok := CompareVersions("mc26.3-0.9.2-fabric", "1.0"); ok {
		t.Error("non-semantic versions must not parse")
	}
}

func TestMatch(t *testing.T) {
	tests := []struct {
		v     string
		preds []string
		want  Result
	}{
		{"26.3", []string{"~26.3"}, Yes},
		{"26.3.2", []string{"~26.3"}, Yes},
		{"26.4", []string{"~26.3"}, No},
		{"0.20.1", []string{">=0.20.1"}, Yes},
		{"0.19.9", []string{">=0.20.1"}, No},
		{"1.21.1", []string{"1.21.x"}, Yes},
		{"1.22", []string{"1.21.x"}, No},
		{"2.5.0", []string{"^2.1"}, Yes},
		{"3.0.0", []string{"^2.1"}, No},
		{"1.5", []string{">=1.0 <2.0"}, Yes},
		{"2.0", []string{">=1.0 <2.0"}, No},
		{"26.2", []string{"26.3", "26.2"}, Yes}, // OR
		{"1.0.0", []string{"*"}, Yes},
		{"0.9.2+mc26.3", []string{"<=1.11.2"}, Yes},     // Sodium vs an old Iris break
		{"2.0.2-1.21.2", []string{"<2.0.2-1.21.2"}, No}, // pre-release bound
		{"1.6.0-beta.1", []string{"<1.6.0-beta.2"}, Yes},
		{"1.6.0", []string{"<1.6.0-beta.2"}, No},
		{"custom-build", []string{">=1.0"}, Unsure},
		{"custom-build", []string{"custom-build"}, Yes},
		{"1.0", nil, Yes},
	}
	for _, tt := range tests {
		if got := Match(tt.v, tt.preds); got != tt.want {
			t.Errorf("Match(%q, %q) = %v, want %v", tt.v, tt.preds, got, tt.want)
		}
	}
}
