package target

import (
	"strings"
	"testing"
)

// v1.2.30 platform expansion, first slice: FreeBSD and Android enter the
// canonical target model; ARM64 was already modeled. This pins parse/normalize,
// data-layout facts and the honest ABI limits — it does NOT claim an emitter
// or a working cross linker for either new target.
func TestPhaseV1230_NewTargetsParseAndNormalize(t *testing.T) {
	cases := []struct {
		input string
		arch  Arch
		os    OS
		want  string
	}{
		{"x86_64-freebsd", ArchX8664, OSFreeBSD, "x86_64-freebsd"},
		{"aarch64-freebsd", ArchAArch64, OSFreeBSD, "aarch64-freebsd"},
		{"x86_64-unknown-freebsd", ArchX8664, OSFreeBSD, "x86_64-freebsd"},
		{"aarch64-android", ArchAArch64, OSAndroid, "aarch64-android"},
		{"aarch64-linux-android", ArchAArch64, OSAndroid, "aarch64-android"},
	}
	for _, c := range cases {
		tg, err := Parse(c.input)
		if err != nil {
			t.Errorf("Parse(%q): %v", c.input, err)
			continue
		}
		if tg.Arch != c.arch || tg.OS != c.os || tg.String() != c.want {
			t.Errorf("Parse(%q) = %v/%v/%q, want %v/%v/%q", c.input, tg.Arch, tg.OS, tg.String(), c.arch, c.os, c.want)
		}
		if !IsSupported(tg) {
			t.Errorf("%s parsed but is not in SupportedTargets", c.input)
		}
	}
}

func TestPhaseV1230_NewTargetFeaturesAreELF(t *testing.T) {
	for _, input := range []string{"x86_64-freebsd", "aarch64-android"} {
		tg, err := Parse(input)
		if err != nil {
			t.Fatal(err)
		}
		f := FeaturesOf(tg)
		if f.ObjFormat != "ELF" || f.ExeFormat != "ELF" || f.PointerWidth != 64 {
			t.Errorf("%s features not 64-bit ELF: %+v", input, f)
		}
		if !strings.Contains(f.LinkerRequirement, "clang") {
			t.Errorf("%s must require an explicit clang --target, got %q", input, f.LinkerRequirement)
		}
	}
}

func TestPhaseV1230_AbiLimitsAreHonest(t *testing.T) {
	for _, input := range []string{
		"x86_64-freebsd-msvc", "aarch64-android-msvc", "aarch64-android-gnu", "riscv64-freebsd",
	} {
		if _, err := Parse(input); err == nil {
			t.Errorf("Parse(%q) should reject an unsupported ABI", input)
		}
	}
}
