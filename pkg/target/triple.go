// Package target implements Karkain's own target-triple model (Phase 111).
//
// A target names the platform a program is compiled FOR, independently of the
// platform the toolchain runs ON (the host). Karkain owns this model: it uses
// conventional LLVM-style terminology where practical (x86_64-pc-windows-msvc,
// x86_64-unknown-linux-gnu) but parses, validates and normalizes targets on its
// own terms and never delegates triples to an external compiler framework.
package target

import (
	"fmt"
	"runtime"
	"strings"
)

// Arch is a CPU architecture Karkain can target.
type Arch int

const (
	ArchUnknown Arch = iota
	ArchX8664        // AMD64 / EM_X86_64
	ArchAArch64      // ARM64 / EM_AARCH64
	ArchWasm32       // WebAssembly 32-bit
)

// OS is an operating-system family Karkain can target.
type OS int

const (
	OSUnknown OS = iota
	OSWindows
	OSLinux
	OSWasi
)

// Env is the optional environment/ABI component of a triple.
type Env int

const (
	EnvUnknown Env = iota
	EnvGNU         // GNU/Linux (glibc), MinGW-w64 (mingw)
	EnvMSVC        // Windows MSVC toolchain ABI
	EnvMusl        // musl libc (recognized, not yet buildable)
)

func (a Arch) String() string {
	switch a {
	case ArchX8664:
		return "x86_64"
	case ArchAArch64:
		return "aarch64"
	case ArchWasm32:
		return "wasm32"
	}
	return "unknown"
}

func (o OS) String() string {
	switch o {
	case OSWindows:
		return "windows"
	case OSLinux:
		return "linux"
	case OSWasi:
		return "wasi"
	}
	return "unknown"
}

func (e Env) String() string {
	switch e {
	case EnvGNU:
		return "gnu"
	case EnvMSVC:
		return "msvc"
	case EnvMusl:
		return "musl"
	}
	return ""
}

// defaultEnv returns the environment/ABI Karkain assumes when a target does
// not name one explicitly. Karkain's own native pipeline links through
// MinGW-w64 gcc on Windows and glibc (GNU) gcc on Linux.
func defaultEnv(o OS) Env {
	switch o {
	case OSWindows, OSLinux:
		return EnvGNU
	}
	return EnvUnknown
}

// Target is a parsed, validated, normalized Karkain target triple.
type Target struct {
	Arch   Arch
	OS     OS
	Env    Env
	Vendor string // informational only; always dropped from Canonical()
}

// String returns the canonical short form, e.g. "x86_64-windows".
func (t Target) String() string {
	if t.Arch == ArchWasm32 && t.OS == OSWasi {
		return "wasm32-wasi"
	}
	return t.Arch.String() + "-" + t.OS.String()
}

// Equal reports whether two targets describe the same platform (vendor and
// environment/ABI are part of the platform identity).
func (t Target) Equal(o Target) bool {
	return t.Arch == o.Arch && t.OS == o.OS && t.Env == o.Env
}

// SameMachine reports whether two targets run on the same machine kind
// (architecture + OS). The environment/ABI component is a toolchain flavor,
// not a machine; cross-run restrictions compare machines, not triples, so
// `--target x86_64-pc-windows-msvc` may still run on a GNU-toolchain Windows
// host while `--target x86_64-linux` can never run on it.
func (t Target) SameMachine(o Target) bool {
	return t.Arch == o.Arch && t.OS == o.OS
}

// IsHost reports whether the target is the host platform Karkain runs on.
func (t Target) IsHost() bool {
	return t.Equal(Host())
}

// Host returns the platform the running Karkain toolchain itself executes on.
// Host detection is used ONLY to (a) derive the implicit `native` target and
// (b) decide whether cross-run is possible. It is never a substitute for an
// explicit target configuration.
func Host() Target {
	switch runtime.GOARCH {
	case "amd64":
		switch runtime.GOOS {
		case "windows":
			return Target{Arch: ArchX8664, OS: OSWindows, Env: defaultEnv(OSWindows)}
		case "linux":
			return Target{Arch: ArchX8664, OS: OSLinux, Env: defaultEnv(OSLinux)}
		}
	case "arm64":
		switch runtime.GOOS {
		case "windows":
			return Target{Arch: ArchAArch64, OS: OSWindows, Env: defaultEnv(OSWindows)}
		case "linux":
			return Target{Arch: ArchAArch64, OS: OSLinux, Env: defaultEnv(OSLinux)}
		}
	}
	return Target{Arch: archFromString(runtime.GOARCH), OS: osFromString(runtime.GOOS)}
}

func archFromString(s string) Arch {
	switch s {
	case "x86_64", "amd64":
		return ArchX8664
	case "aarch64", "arm64":
		return ArchAArch64
	case "wasm32":
		return ArchWasm32
	}
	return ArchUnknown
}

func osFromString(s string) OS {
	switch s {
	case "windows":
		return OSWindows
	case "linux":
		return OSLinux
	case "wasi":
		return OSWasi
	}
	return OSUnknown
}

func envFromString(s string) Env {
	switch s {
	case "gnu":
		return EnvGNU
	case "msvc":
		return EnvMSVC
	case "musl":
		return EnvMusl
	}
	return EnvUnknown
}

// ParseError describes why a target string could not be accepted.
type ParseError struct {
	Input string
	Kind  string // one of: malformed, unknown-arch, unknown-os, unknown-env
	Value string // the offending component
}

func (e *ParseError) Error() string {
	switch e.Kind {
	case "unknown-arch":
		return fmt.Sprintf("unsupported architecture '%s' in target '%s' (supported architectures: x86_64, aarch64, wasm32)", e.Value, e.Input)
	case "unknown-os":
		return fmt.Sprintf("unsupported operating system '%s' in target '%s' (supported operating systems: windows, linux, wasi)", e.Value, e.Input)
	case "unknown-env":
		return fmt.Sprintf("unsupported environment/ABI '%s' in target '%s' (supported environments: gnu, msvc)", e.Value, e.Input)
	case "unsupported-abi":
		return fmt.Sprintf("unsupported ABI in target '%s': %s", e.Input, e.Value)
	default:
		if e.Value != "" {
			return fmt.Sprintf("malformed target triple '%s' (%s)", e.Input, e.Value)
		}
		return fmt.Sprintf("malformed target triple '%s'", e.Input)
	}
}

// Parse parses and normalizes a target string. Accepted forms (vendor and env
// are optional; canonical output drops vendor):
//
//	x86_64-windows
//	x86_64-linux
//	aarch64-linux
//	x86_64-pc-windows-msvc
//	x86_64-unknown-linux-gnu
//	aarch64-unknown-linux-gnu
//	wasm32-wasi
func Parse(s string) (Target, error) {
	if s == "" {
		return Target{}, &ParseError{Input: s, Kind: "malformed", Value: "empty target"}
	}
	parts := strings.Split(s, "-")
	if n := len(parts); n < 2 || n > 4 {
		return Target{}, &ParseError{Input: s, Kind: "malformed",
			Value: fmt.Sprintf("expected '<arch>-<os>' with optional '<vendor>[-<env>]' components, got %d component(s)", n)}
	}

	var t Target
	t.Arch = archFromString(parts[0])
	if t.Arch == ArchUnknown {
		return Target{}, &ParseError{Input: s, Kind: "unknown-arch", Value: parts[0]}
	}

	switch len(parts) {
	case 2:
		t.OS = osFromString(parts[1])
		if t.OS == OSUnknown {
			return Target{}, &ParseError{Input: s, Kind: "unknown-os", Value: parts[1]}
		}
	case 3:
		// "<arch>-<vendor>-<os>" or "<arch>-<os>-<env>" are both valid;
		// disambiguate by whether the last component names an OS.
		if osFromString(parts[2]) != OSUnknown {
			t.Vendor = parts[1]
			t.OS = osFromString(parts[2])
		} else {
			t.OS = osFromString(parts[1])
			if t.OS == OSUnknown {
				return Target{}, &ParseError{Input: s, Kind: "unknown-os", Value: parts[1]}
			}
			t.Env = envFromString(parts[2])
			if t.Env == EnvUnknown {
				return Target{}, &ParseError{Input: s, Kind: "unknown-env", Value: parts[2]}
			}
		}
	case 4:
		t.Vendor = parts[1]
		t.OS = osFromString(parts[2])
		if t.OS == OSUnknown {
			return Target{}, &ParseError{Input: s, Kind: "unknown-os", Value: parts[2]}
		}
		t.Env = envFromString(parts[3])
		if t.Env == EnvUnknown {
			return Target{}, &ParseError{Input: s, Kind: "unknown-env", Value: parts[3]}
		}
	}

	// Default the environment/ABI when the triple does not name one, then run
	// the ABI sanity checks.
	if t.Env == EnvUnknown {
		t.Env = defaultEnv(t.OS)
	}

	// ABI sanity: combinations that make no sense for any real backend.
	if t.Env == EnvMusl {
		return Target{}, &ParseError{Input: s, Kind: "unsupported-abi",
			Value: "the 'musl' ABI is recognized but not yet supported by any Karkain backend"}
	}
	if t.Env == EnvMSVC && t.OS == OSLinux {
		return Target{}, &ParseError{Input: s, Kind: "unsupported-abi",
			Value: "the 'msvc' ABI is only valid for Windows targets"}
	}
	if t.Env == EnvGNU && t.OS == OSWasi {
		return Target{}, &ParseError{Input: s, Kind: "unsupported-abi",
			Value: "the 'gnu' ABI is not valid for WASI targets"}
	}
	if t.OS == OSWasi && t.Arch != ArchWasm32 {
		return Target{}, &ParseError{Input: s, Kind: "unknown-os",
			Value: "the 'wasi' OS currently only supports the wasm32 architecture"}
	}
	if t.Arch == ArchWasm32 && t.OS != OSWasi {
		return Target{}, &ParseError{Input: s, Kind: "unknown-os",
			Value: "the 'wasm32' architecture currently only supports the wasi OS"}
	}
	return t, nil
}

// SupportedArchs and SupportedOSes are the accepted arch/OS names used in
// diagnostics.
func SupportedArchs() string { return "x86_64, aarch64, wasm32" }
func SupportedOSes() string  { return "windows, linux, wasi" }
