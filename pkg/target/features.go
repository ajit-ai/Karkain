package target

import "fmt"

// Features is the Phase-111 target data layout/runtime model. It describes the
// ABI facts Karkain-derived code and the build pipeline must honour for a
// target; it never guesses from the host.
type Features struct {
	Target            Target
	PointerWidth      int    // bits (32 or 64)
	Endianness        string // "little-endian"
	ObjFormat         string // "ELF", "PE/COFF", "WASM"
	ExeFormat         string // "ELF", "PE/COFF", "WASM"
	ABI               string // human-readable ABI/calling-convention name
	RuntimeVariant    string // runtime selection key ("windows", "linux-gnu", "wasi")
	LinkerRequirement string // what an external C toolchain must provide
}

// FeaturesOf returns the target data model for t. Unknown targets yield the
// zero Features (callers must validate the target before building).
func FeaturesOf(t Target) Features {
	f := Features{Target: t}
	switch t.Arch {
	case ArchX8664, ArchAArch64, ArchRiscv64:
		f.PointerWidth = 64
		f.Endianness = "little-endian"
	case ArchWasm32:
		f.PointerWidth = 32
		f.Endianness = "little-endian"
		f.ObjFormat = "WASM"
		f.ExeFormat = "WASM"
		f.RuntimeVariant = "wasi"
		f.ABI = "WebAssembly/WASI"
		f.LinkerRequirement = "none (Karkain's wasm backend emits the module directly)"
		return f
	}

	switch t.OS {
	case OSWindows:
		f.ObjFormat = "PE/COFF"
		f.ExeFormat = "PE/COFF"
		f.RuntimeVariant = "windows"
		if t.Arch == ArchX8664 {
			f.ABI = "Microsoft x64 / x64 or Win64 calling convention"
		} else {
			f.ABI = "AArch64 Windows / AACPS64"
		}
		f.LinkerRequirement = fmt.Sprintf("MinGW-w64 gcc for %s (e.g. %s-gcc) or clang with --target=%s",
			t.Arch, MingwTriple(t), t)
	case OSLinux:
		f.ObjFormat = "ELF"
		f.ExeFormat = "ELF"
		f.RuntimeVariant = "linux-gnu"
		if t.Env == EnvMusl {
			f.ABI = "musl libc"
		} else {
			f.ABI = "System V " + t.Arch.String() + " / glibc (GNU)"
		}
		f.LinkerRequirement = fmt.Sprintf("%s-linux-gnu-gcc or clang with --target=%s-unknown-linux-gnu",
			t.Arch, t.Arch)
	case OSMacOS:
		// Phase 139: modeled for search/error paths; linked only via clang
		// with an explicit --target (no GNU/MSVC toolchain exists for macOS).
		f.ObjFormat = "Mach-O"
		f.ExeFormat = "Mach-O"
		f.RuntimeVariant = "macos"
		f.ABI = "System V " + t.Arch.String() + " / Apple"
		f.LinkerRequirement = fmt.Sprintf("clang with --target=%s-apple-macosx", t.Arch)
	}
	return f
}

// MingwTriple returns the conventional MinGW-w64 compiler prefix for a
// Windows target (used only in diagnostics and toolchain probing).
func MingwTriple(t Target) string {
	if t.Arch == ArchX8664 {
		return "x86_64-w64-mingw32"
	}
	return "aarch64-w64-mingw32"
}

// SupportedTargets returns the closed set of targets the Karkain toolchain
// models. Availability of an actual cross toolchain for a given target is a
// host build-time question answered by ToolchainFor (pkg/cli); a compiled
// artifact is only ever produced for targets whose linker Karkain can find.
func SupportedTargets() []Target {
	env := EnvGNU
	if Host().OS == OSWindows {
		env = EnvMSVC
	}
	return []Target{
		{Arch: ArchX8664, OS: OSWindows, Env: env},
		{Arch: ArchAArch64, OS: OSWindows, Env: EnvGNU},
		{Arch: ArchX8664, OS: OSLinux, Env: EnvGNU},
		{Arch: ArchAArch64, OS: OSLinux, Env: EnvGNU},
		{Arch: ArchRiscv64, OS: OSLinux, Env: EnvGNU},
		{Arch: ArchX8664, OS: OSMacOS},
		{Arch: ArchAArch64, OS: OSMacOS},
		{Arch: ArchWasm32, OS: OSWasi},
	}
}

// IsSupported reports whether t is one of the model's supported targets.
func IsSupported(t Target) bool {
	for _, s := range SupportedTargets() {
		if s.Arch == t.Arch && s.OS == t.OS {
			return true
		}
	}
	return false
}

// ToolchainError reports that no C toolchain on the host can produce an
// artifact for the requested target. It is the Phase-111 "missing target
// linker/toolchain" diagnostic: never a silent host fallback and never a
// wrong-architecture binary.
type ToolchainError struct {
	Target   Target
	Host     Target
	Searched []string
}

func (e *ToolchainError) Error() string {
	msg := fmt.Sprintf("no cross-linker available for target '%s' on host '%s'", e.Target, e.Host)
	if len(e.Searched) > 0 {
		msg += " (searched: " + firstStrings(3, e.Searched)
		if len(e.Searched) > 3 {
			msg += fmt.Sprintf(", and %d more", len(e.Searched)-3)
		}
		msg += ")"
	}
	msg += fmt.Sprintf("; install a C cross-toolchain for %s (see `karkain target`) and ensure it is on PATH — the host toolchain can neither link nor execute %s output", e.Target, e.Target.OS)
	return msg
}

func firstStrings(n int, s []string) string {
	out := ""
	for i := 0; i < n && i < len(s); i++ {
		if i > 0 {
			out += ", "
		}
		out += "'" + s[i] + "'"
	}
	return out
}

// Describe returns a one-line human summary of a target's features.
func Describe(t Target) string {
	f := FeaturesOf(t)
	return fmt.Sprintf("%-18s %d-bit %s; objects %s / executables %s; %s",
		t, f.PointerWidth, f.Endianness, f.ObjFormat, f.ExeFormat, f.ABI)
}
