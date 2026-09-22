package target

import (
	"errors"
	"strings"
	"testing"
)

func TestParseValid(t *testing.T) {
	cases := []struct {
		in   string
		want Target
	}{
		{"x86_64-windows", Target{Arch: ArchX8664, OS: OSWindows, Env: EnvGNU}},
		{"x86_64-pc-windows-msvc", Target{Arch: ArchX8664, OS: OSWindows, Env: EnvMSVC}},
		{"x86_64-unknown-windows-msvc", Target{Arch: ArchX8664, OS: OSWindows, Env: EnvMSVC}},
		{"x86_64-windows-gnu", Target{Arch: ArchX8664, OS: OSWindows, Env: EnvGNU}},
		{"x86_64-linux", Target{Arch: ArchX8664, OS: OSLinux, Env: EnvGNU}},
		{"x86_64-unknown-linux-gnu", Target{Arch: ArchX8664, OS: OSLinux, Env: EnvGNU}},
		{"x86_64-pc-linux-gnu", Target{Arch: ArchX8664, OS: OSLinux, Env: EnvGNU}},
		{"aarch64-linux", Target{Arch: ArchAArch64, OS: OSLinux, Env: EnvGNU}},
		{"aarch64-unknown-linux-gnu", Target{Arch: ArchAArch64, OS: OSLinux, Env: EnvGNU}},
		{"aarch64-linux-gnu", Target{Arch: ArchAArch64, OS: OSLinux, Env: EnvGNU}},
		{"aarch64-windows", Target{Arch: ArchAArch64, OS: OSWindows, Env: EnvGNU}},
		{"aarch64-pc-windows-msvc", Target{Arch: ArchAArch64, OS: OSWindows, Env: EnvMSVC}},
		{"riscv64-linux", Target{Arch: ArchRiscv64, OS: OSLinux, Env: EnvGNU}},
		{"riscv64-unknown-linux-gnu", Target{Arch: ArchRiscv64, OS: OSLinux, Env: EnvGNU}},
		{"x86_64-macos", Target{Arch: ArchX8664, OS: OSMacOS}},
		{"x86_64-apple-macosx", Target{Arch: ArchX8664, OS: OSMacOS}},
		{"x86_64-darwin", Target{Arch: ArchX8664, OS: OSMacOS}},
		{"aarch64-macos", Target{Arch: ArchAArch64, OS: OSMacOS}},
		{"aarch64-apple-macosx", Target{Arch: ArchAArch64, OS: OSMacOS}},
		{"wasm32-wasi", Target{Arch: ArchWasm32, OS: OSWasi}},
	}
	for _, c := range cases {
		got, err := Parse(c.in)
		if err != nil {
			t.Errorf("Parse(%q): unexpected error %v", c.in, err)
			continue
		}
		if got.Arch != c.want.Arch || got.OS != c.want.OS || got.Env != c.want.Env {
			t.Errorf("Parse(%q) = %+v, want arch=%v os=%v env=%v", c.in, got, c.want.Arch, c.want.OS, c.want.Env)
		}
	}
}

func TestParseCanonicalStrings(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"x86_64-windows", "x86_64-windows"},
		{"x86_64-pc-windows-msvc", "x86_64-windows"},
		{"x86_64-unknown-windows-msvc", "x86_64-windows"},
		{"x86_64-linux", "x86_64-linux"},
		{"x86_64-unknown-linux-gnu", "x86_64-linux"},
		{"aarch64-unknown-linux-gnu", "aarch64-linux"},
		{"aarch64-windows", "aarch64-windows"},
		{"riscv64-unknown-linux-gnu", "riscv64-linux"},
		{"x86_64-apple-macosx", "x86_64-macos"},
		{"x86_64-darwin", "x86_64-macos"},
		{"aarch64-apple-macosx", "aarch64-macos"},
		{"wasm32-wasi", "wasm32-wasi"},
	}
	for _, c := range cases {
		got, err := Parse(c.in)
		if err != nil {
			t.Errorf("Parse(%q): %v", c.in, err)
			continue
		}
		if got.String() != c.want {
			t.Errorf("Parse(%q).String() = %q, want %q", c.in, got.String(), c.want)
		}
	}
}

func TestParseInvalid(t *testing.T) {
	cases := []struct {
		in   string
		kind string
		want string
	}{
		{"", "malformed", "empty target"},
		{"unknown-platform", "unknown-arch", "x86_64"},
		{"s390x-linux", "unknown-arch", "x86_64"},
		{"x86_64", "malformed", "got 1 component"},
		{"x86_64-openbsd", "unknown-os", "windows"},
		{"x86_64-linux-macabi", "unknown-env", "gnu, msvc"},
		{"x86_64-linux-msvc", "unsupported-abi", "only valid for Windows"},
		{"x86_64-linux-musl", "unsupported-abi", "musl"},
		{"aarch64-linux-musl", "unsupported-abi", "musl"},
		{"riscv64-windows", "unsupported-abi", "only modeled for Linux"},
		{"riscv64-macos", "unsupported-abi", "only modeled for Linux"},
		{"wasm32-macos", "unsupported-abi", "only supports the x86_64 and aarch64"},
		{"x86_64-macos-gnu", "unsupported-abi", "macOS builds use clang"},
		{"x86_64-macos-msvc", "unsupported-abi", "macOS builds use clang"},
		{"wasm32-linux", "unknown-os", "only supports the wasi OS"},
		{"wasm32-windows", "unknown-os", "only supports the wasi OS"},
		{"x86_64-wasi", "unknown-os", "only supports the wasm32"},
		{"x86_64-unknown-linux-musl-gnu-x", "malformed", "got 6 component"},
	}
	for _, c := range cases {
		got, err := Parse(c.in)
		if err == nil {
			t.Errorf("Parse(%q) succeeded with %+v, want %s error", c.in, got, c.kind)
			continue
		}
		var pe *ParseError
		if !errors.As(err, &pe) {
			t.Errorf("Parse(%q) error %T, want *ParseError", c.in, err)
			continue
		}
		if pe.Kind != c.kind {
			t.Errorf("Parse(%q) error kind = %q, want %q (%v)", c.in, pe.Kind, c.kind, err)
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("Parse(%q) error %q should mention %q", c.in, err.Error(), c.want)
		}
	}
}

func TestHostIsThisMachine(t *testing.T) {
	h := Host()
	if h.Arch == ArchUnknown || h.OS == OSUnknown {
		t.Fatalf("Host() = %+v has unmodeled arch/os", h)
	}
	if got, err := Parse(h.String()); err != nil || !got.Equal(h) {
		t.Errorf("Host() %v does not round-trip through Parse", h)
	}
}

func TestIsSupported(t *testing.T) {
	// Phase 139: aarch64-windows, riscv64-linux and both macOS triples join
	// the modeled set; riscv64-windows stays unparseable (unsupported-abi)
	// so it cannot reach IsSupported at all.
	for _, in := range []string{"x86_64-windows", "x86_64-linux", "aarch64-linux", "wasm32-wasi",
		"aarch64-windows", "riscv64-linux", "x86_64-macos", "aarch64-macos"} {
		tg, err := Parse(in)
		if err != nil {
			t.Fatalf("Parse(%q): %v", in, err)
		}
		if !IsSupported(tg) {
			t.Errorf("IsSupported(%q) = false, want true", in)
		}
	}
	for _, in := range []string{"x86_64-openbsd"} {
		tg, err := Parse(in)
		if err == nil {
			t.Errorf("Parse(%q) succeeded with %+v, want unknown-os error", in, tg)
			continue
		}
		if IsSupported(tg) {
			t.Errorf("IsSupported(%q) = true, want false", in)
		}
	}
}

func TestFeaturesLayout(t *testing.T) {
	cases := []struct {
		in             string
		ptr            int
		obj, exe       string
		endian         string
		runtimeVariant string
	}{
		{"x86_64-windows", 64, "PE/COFF", "PE/COFF", "little-endian", "windows"},
		{"aarch64-windows", 64, "PE/COFF", "PE/COFF", "little-endian", "windows"},
		{"x86_64-linux", 64, "ELF", "ELF", "little-endian", "linux-gnu"},
		{"aarch64-linux", 64, "ELF", "ELF", "little-endian", "linux-gnu"},
		{"riscv64-linux", 64, "ELF", "ELF", "little-endian", "linux-gnu"},
		{"x86_64-macos", 64, "Mach-O", "Mach-O", "little-endian", "macos"},
		{"aarch64-macos", 64, "Mach-O", "Mach-O", "little-endian", "macos"},
		{"wasm32-wasi", 32, "WASM", "WASM", "little-endian", "wasi"},
	}
	for _, c := range cases {
		tg, err := Parse(c.in)
		if err != nil {
			t.Fatalf("Parse(%q): %v", c.in, err)
		}
		f := FeaturesOf(tg)
		if f.PointerWidth != c.ptr || f.ObjFormat != c.obj || f.ExeFormat != c.exe ||
			f.Endianness != c.endian || f.RuntimeVariant != c.runtimeVariant {
			t.Errorf("FeaturesOf(%q) = %+v, want ptr=%d obj=%s exe=%s endian=%s runtime=%s",
				c.in, f, c.ptr, c.obj, c.exe, c.endian, c.runtimeVariant)
		}
	}
}

func TestDescribeListsFeatures(t *testing.T) {
	d := Describe(mustParse(t, "x86_64-linux"))
	for _, want := range []string{"x86_64-linux", "64-bit", "ELF"} {
		if !strings.Contains(d, want) {
			t.Errorf("Describe output missing %q: %s", want, d)
		}
	}
}

func mustParse(t *testing.T, s string) Target {
	t.Helper()
	tg, err := Parse(s)
	if err != nil {
		t.Fatalf("Parse(%q): %v", s, err)
	}
	return tg
}
