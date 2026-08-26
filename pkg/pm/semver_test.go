package pm

import (
	"testing"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input       string
		major, minor, patch int
		preRelease  string
		build       string
		wantErr     bool
	}{
		{"1.2.3", 1, 2, 3, "", "", false},
		{"0.0.1", 0, 0, 1, "", "", false},
		{"10.20.30", 10, 20, 30, "", "", false},
		{"v1.2.3", 1, 2, 3, "", "", false},
		{"1.2.3-alpha", 1, 2, 3, "alpha", "", false},
		{"1.2.3-alpha.1", 1, 2, 3, "alpha.1", "", false},
		{"1.2.3+build.123", 1, 2, 3, "", "build.123", false},
		{"1.2.3-alpha.1+build.123", 1, 2, 3, "alpha.1", "build.123", false},
		{"v2.0.0-rc.1", 2, 0, 0, "rc.1", "", false},
		{"1.2.3-beta.1+build.456", 1, 2, 3, "beta.1", "build.456", false},
		{"not-a-version", 0, 0, 0, "", "", true},
		{"1.2", 0, 0, 0, "", "", true},
		{"", 0, 0, 0, "", "", true},
		{"1.2.3.4", 0, 0, 0, "", "", true},
		{"v1", 0, 0, 0, "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			v, err := ParseVersion(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseVersion(%q) succeeded, want error", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseVersion(%q) error: %v", tt.input, err)
				return
			}
			if v.Major != tt.major || v.Minor != tt.minor || v.Patch != tt.patch {
				t.Errorf("ParseVersion(%q) = %d.%d.%d, want %d.%d.%d", tt.input, v.Major, v.Minor, v.Patch, tt.major, tt.minor, tt.patch)
			}
			if v.PreRelease != tt.preRelease {
				t.Errorf("ParseVersion(%q).PreRelease = %q, want %q", tt.input, v.PreRelease, tt.preRelease)
			}
			if v.BuildMetadata != tt.build {
				t.Errorf("ParseVersion(%q).BuildMetadata = %q, want %q", tt.input, v.BuildMetadata, tt.build)
			}
		})
	}
}

func TestVersionString(t *testing.T) {
	tests := []struct {
		v    Version
		want string
	}{
		{Version{1, 2, 3, "", ""}, "1.2.3"},
		{Version{1, 2, 3, "alpha", ""}, "1.2.3-alpha"},
		{Version{1, 2, 3, "", "build.1"}, "1.2.3+build.1"},
		{Version{1, 2, 3, "rc.1", "build.1"}, "1.2.3-rc.1+build.1"},
	}
	for _, tt := range tests {
		got := tt.v.String()
		if got != tt.want {
			t.Errorf("Version.String() = %q, want %q", got, tt.want)
		}
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "2.0.0", -1},
		{"2.0.0", "1.0.0", 1},
		{"1.1.0", "1.0.0", 1},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0-alpha", "1.0.0", -1},
		{"1.0.0", "1.0.0-alpha", 1},
		{"1.0.0-alpha", "1.0.0-alpha", 0},
		{"1.0.0-alpha", "1.0.0-beta", -1},
		{"1.0.0-beta", "1.0.0-alpha", 1},
		{"1.0.0-alpha.1", "1.0.0-alpha.2", -1},
		{"1.0.0-alpha.1", "1.0.0-alpha", 1},
		{"1.0.0-rc.1", "1.0.0", -1},
		{"1.0.0-rc.1", "1.0.0-beta.1", 1},
	}
	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			a, _ := ParseVersion(tt.a)
			b, _ := ParseVersion(tt.b)
			got := Compare(a, b)
			if got != tt.want {
				t.Errorf("Compare(%s, %s) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestSatisfiesExact(t *testing.T) {
	v, _ := ParseVersion("1.2.3")
	ok, err := Satisfies(v, "1.2.3")
	if err != nil || !ok {
		t.Errorf("1.2.3 should satisfy 1.2.3")
	}
	ok, _ = Satisfies(v, "1.2.4")
	if ok {
		t.Errorf("1.2.3 should not satisfy 1.2.4")
	}
}

func TestSatisfiesCaret(t *testing.T) {
	v123, _ := ParseVersion("1.2.3")
	v130, _ := ParseVersion("1.3.0")
	v200, _ := ParseVersion("2.0.0")
	v023, _ := ParseVersion("0.2.3")
	v030, _ := ParseVersion("0.3.0")
	v003, _ := ParseVersion("0.0.3")
	v004, _ := ParseVersion("0.0.4")

	tests := []struct {
		name       string
		v          Version
		constraint string
		want       bool
	}{
		{"^1.2.3 matches 1.2.3", v123, "^1.2.3", true},
		{"^1.2.3 matches 1.3.0", v130, "^1.2.3", true},
		{"^1.2.3 rejects 2.0.0", v200, "^1.2.3", false},
		{"^0.2.3 matches 0.2.3", v023, "^0.2.3", true},
		{"^0.2.3 rejects 0.3.0", v030, "^0.2.3", false},
		{"^0.0.3 matches 0.0.3", v003, "^0.0.3", true},
		{"^0.0.3 rejects 0.0.4", v004, "^0.0.3", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Satisfies(tt.v, tt.constraint)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("%s satisfies %q = %v, want %v", tt.v, tt.constraint, got, tt.want)
			}
		})
	}
}

func TestSatisfiesTilde(t *testing.T) {
	v123, _ := ParseVersion("1.2.3")
	v124, _ := ParseVersion("1.2.4")
	v130, _ := ParseVersion("1.3.0")
	v200, _ := ParseVersion("2.0.0")

	tests := []struct {
		name       string
		v          Version
		constraint string
		want       bool
	}{
		{"~1.2.3 matches 1.2.3", v123, "~1.2.3", true},
		{"~1.2.3 matches 1.2.4", v124, "~1.2.3", true},
		{"~1.2.3 rejects 1.3.0", v130, "~1.2.3", false},
		{"~1.2.3 rejects 2.0.0", v200, "~1.2.3", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Satisfies(tt.v, tt.constraint)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("%s satisfies %q = %v, want %v", tt.v, tt.constraint, got, tt.want)
			}
		})
	}
}

func TestSatisfiesRange(t *testing.T) {
	v150, _ := ParseVersion("1.5.0")
	v200, _ := ParseVersion("2.0.0")
	v090, _ := ParseVersion("0.9.0")
	v100, _ := ParseVersion("1.0.0")

	tests := []struct {
		name       string
		v          Version
		constraint string
		want       bool
	}{
		{"1.5.0 in >=1.0.0, <2.0.0", v150, ">=1.0.0, <2.0.0", true},
		{"2.0.0 in >=1.0.0, <2.0.0", v200, ">=1.0.0, <2.0.0", false},
		{"0.9.0 in >=1.0.0, <2.0.0", v090, ">=1.0.0, <2.0.0", false},
		{"1.0.0 in >=1.0.0, <2.0.0", v100, ">=1.0.0, <2.0.0", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Satisfies(tt.v, tt.constraint)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("%s satisfies %q = %v, want %v", tt.v, tt.constraint, got, tt.want)
			}
		})
	}
}

func TestSatisfiesWildcard(t *testing.T) {
	v123, _ := ParseVersion("1.2.3")
	v129, _ := ParseVersion("1.2.9")
	v130, _ := ParseVersion("1.3.0")
	v100, _ := ParseVersion("1.0.0")
	v200, _ := ParseVersion("2.0.0")

	tests := []struct {
		name       string
		v          Version
		constraint string
		want       bool
	}{
		{"1.2.x matches 1.2.3", v123, "1.2.x", true},
		{"1.2.x matches 1.2.9", v129, "1.2.x", true},
		{"1.2.x rejects 1.3.0", v130, "1.2.x", false},
		{"1.* matches 1.0.0", v100, "1.*", true},
		{"1.* matches 1.2.3", v123, "1.*", true},
		{"1.* rejects 2.0.0", v200, "1.*", false},
		{"* matches anything", v200, "*", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Satisfies(tt.v, tt.constraint)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("%s satisfies %q = %v, want %v", tt.v, tt.constraint, got, tt.want)
			}
		})
	}
}

func TestSatisfiesPreRelease(t *testing.T) {
	v100a, _ := ParseVersion("1.0.0-alpha")
	v100b, _ := ParseVersion("1.0.0-beta")
	v100rc, _ := ParseVersion("1.0.0-rc.1")
	v100, _ := ParseVersion("1.0.0")

	ok, _ := Satisfies(v100a, ">=1.0.0-alpha, <1.0.0")
	if !ok {
		t.Error("1.0.0-alpha should satisfy >=1.0.0-alpha, <1.0.0")
	}
	ok, _ = Satisfies(v100, ">=1.0.0-alpha, <1.0.0")
	if ok {
		t.Error("1.0.0 should not satisfy <1.0.0")
	}
	ok, _ = Satisfies(v100a, "~1.0.0-alpha")
	if !ok {
		t.Error("1.0.0-alpha should satisfy ~1.0.0-alpha")
	}
	ok, _ = Satisfies(v100b, "~1.0.0-alpha")
	if !ok {
		t.Error("1.0.0-beta should satisfy ~1.0.0-alpha (alpha < beta, both < 1.1.0)")
	}
	_ = v100rc
}

func TestLatestSatisfying(t *testing.T) {
	vs, _ := ParseVersions([]string{
		"1.0.0", "1.1.0", "1.2.0", "2.0.0", "1.2.0-beta.1",
	})
	v, err := LatestSatisfying(vs, ">=1.0.0, <2.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.String() != "1.2.0" {
		t.Errorf("LatestSatisfying = %s, want 1.2.0", v)
	}

	v, err = LatestSatisfying(vs, "^1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.String() != "1.2.0" {
		t.Errorf("LatestSatisfying(^1.0.0) = %s, want 1.2.0", v)
	}

	_, err = LatestSatisfying(vs, ">=3.0.0")
	if err == nil {
		t.Error("expected error for unsatisfiable constraint")
	}
}

func TestSatisfiesZeroVersions(t *testing.T) {
	v010, _ := ParseVersion("0.1.0")
	v011, _ := ParseVersion("0.1.1")
	v020, _ := ParseVersion("0.2.0")

	ok, _ := Satisfies(v010, "^0.1.0")
	if !ok {
		t.Error("0.1.0 should satisfy ^0.1.0")
	}
	ok, _ = Satisfies(v011, "^0.1.0")
	if !ok {
		t.Error("0.1.1 should satisfy ^0.1.0")
	}
	ok, _ = Satisfies(v020, "^0.1.0")
	if ok {
		t.Error("0.2.0 should not satisfy ^0.1.0")
	}
}
