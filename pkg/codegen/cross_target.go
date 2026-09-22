package codegen

import (
	"fmt"
	"os"
	"os/exec"

	"karkain/pkg/target"
)

// selectedTarget resolves the Config.Target value into a concrete target
// model. The legacy aliases `native`, `c23` and `native-link` compile for the
// host; `wasm32-wasi` names the dedicated Karkain-owned WASM backend (kept
// here only for direct codegen callers — the CLI routes it to pkg/wasm before
// codegen); conventional triples parse through the target model.
func (g *Generator) selectedTarget() (target.Target, error) {
	switch g.cfg.Target {
	case "", "native", "c23", "native-link":
		return target.Host(), nil
	case "wasm32-wasi":
		return target.Parse("wasm32-wasi")
	default:
		return target.Parse(g.cfg.Target)
	}
}

// isHostMachine reports whether the target machine equals the host machine
// (architecture + OS). A same-machine build may use the host C toolchain.
func isHostMachine(tg target.Target) bool {
	return tg.SameMachine(target.Host())
}

// crossToolchainError builds the Phase-111 "missing target linker/toolchain"
// diagnostic listing what was searched so a user knows exactly what to install.
func crossToolchainError(tg target.Target, searched ...string) error {
	return &target.ToolchainError{Target: tg, Host: target.Host(), Searched: searched}
}

// detectCompilerForTarget chooses the C toolchain and flags for the selected
// target. Same-machine targets keep the historical host probing (CC override,
// then gcc, clang, MSVC cl on Windows). Cross targets probe conventional
// triple-prefixed cross compilers first, then clang with an explicit
// --target; if none is on PATH a ToolchainError is returned — never a silent
// fallback to the host toolchain and never a wrong-architecture binary.
func (g *Generator) detectCompilerForTarget(cFile, exeFile string) (string, []string, error) {
	tg, err := g.selectedTarget()
	if err != nil {
		return "", nil, err
	}

	// wasm32-wasi direct codegen path (the CLI routes wasm builds to pkg/wasm
	// before codegen; this preserves the historical clang-wasi emission).
	if tg.OS == target.OSWasi {
		return g.detectWasiCompiler(cFile, exeFile)
	}

	switch tg.OS {
	case target.OSWindows:
		if isHostMachine(tg) {
			return g.detectHostCompiler(cFile, exeFile)
		}
		return g.detectWindowsCrossCompiler(tg, cFile, exeFile)
	case target.OSLinux:
		if isHostMachine(tg) {
			return g.detectHostCompiler(cFile, exeFile)
		}
		return g.detectLinuxCrossCompiler(tg, cFile, exeFile)
	case target.OSMacOS:
		// Phase 139: macOS has no GNU/MSVC toolchain by design (the target
		// model rejects those ABIs at parse); the only producer is clang
		// with an explicit --target. Same-machine macOS hosts use it
		// directly, foreign hosts probe PATH for it.
		return g.detectMacOSCrossCompiler(tg, cFile, exeFile)
	}
	return "", nil, fmt.Errorf("no C toolchain rules for target '%s'", tg)
}

// detectWasiCompiler preserves the historical clang --target=wasm32-wasi path
// for direct codegen callers.
func (g *Generator) detectWasiCompiler(cFile, exeFile string) (string, []string, error) {
	if _, err := exec.LookPath("clang"); err == nil {
		return "clang", []string{cFile, "-o", exeFile, "--target=wasm32-wasi", "-O2"}, nil
	}
	if cc := os.Getenv("CC"); cc != "" {
		return cc, []string{cFile, "-o", exeFile, "--target=wasm32-wasi", "-O2"}, nil
	}
	return "", nil, crossToolchainError(target.Target{Arch: target.ArchWasm32, OS: target.OSWasi},
		"clang", "$CC")
}

// detectMacOSCrossCompiler produces a Mach-O binary for a macOS target using
// clang with an explicit --target (the only supported producer). CC override
// is honored first (historical host-probing rule). Without clang the result
// is a ToolchainError listing exactly what was searched — never a silent
// host-toolchain fallback and never a wrong-OS binary.
func (g *Generator) detectMacOSCrossCompiler(tg target.Target, cFile, exeFile string) (string, []string, error) {
	if cc := os.Getenv("CC"); cc != "" {
		return cc, []string{cFile, "-o", exeFile, "-std=c2x", "-O0", "-Wno-psabi", "-lgmp", "-D_POSIX_C_SOURCE=200809L", "-lm"}, nil
	}
	clangTarget := tg.Arch.String() + "-apple-macosx"
	searched := []string{"clang --target=" + clangTarget}
	if _, err := exec.LookPath("clang"); err == nil {
		flags := []string{cFile, "-o", exeFile, "--target=" + clangTarget, "-std=c2x", "-O0", "-Wno-psabi", "-lgmp", "-D_POSIX_C_SOURCE=200809L", "-lm"}
		if g.cfg.Debug {
			flags = append(flags, "-g")
		}
		return "clang", flags, nil
	}
	return "", nil, crossToolchainError(tg, searched...)
}

// detectHostCompiler retains the exact historical host probing: CC override,
// then gcc (Windows vs POSIX variants), clang, then MSVC cl.exe on Windows.
func (g *Generator) detectHostCompiler(cFile, exeFile string) (string, []string, error) {
	if cc := os.Getenv("CC"); cc != "" {
		return cc, g.hostNativeFlags(cFile, exeFile), nil
	}
	if _, err := exec.LookPath("gcc"); err == nil {
		return "gcc", g.hostNativeFlags(cFile, exeFile), nil
	}
	if _, err := exec.LookPath("clang"); err == nil {
		return "clang", g.hostNativeFlags(cFile, exeFile), nil
	}
	if os.Getenv("GOOS") == "windows" {
		if _, err := exec.LookPath("cl"); err == nil {
			return "cl", []string{cFile, "/Fe:" + exeFile, "/nologo", "/O0"}, nil
		}
	}
	return "", nil, crossToolchainError(target.Host(), "gcc", "clang", "cl.exe")
}

// hostNativeFlags are the historical native compile flags: -std=c2x -O0 -lgmp,
// with -mconsole when targeting Windows (host Windows builds; the POSIX branch
// is used verbatim when the host itself is POSIX). Winsock symbols (net
// builtins) resolve through an explicit -lws2_32 on Windows targets because
// MinGW gcc ignores #pragma comment(lib, ...).
func (g *Generator) hostNativeFlags(cFile, exeFile string) []string {
	flags := []string{cFile, "-o", exeFile, "-std=c2x", "-O0", "-Wno-psabi", "-lgmp"}
	if target.Host().OS == target.OSWindows {
		flags = []string{cFile, "-o", exeFile, "-mconsole", "-std=c2x", "-O0", "-Wno-psabi", "-lgmp", "-lws2_32"}
	} else {
		flags = append(flags, "-D_POSIX_C_SOURCE=200809L", "-lm")
	}
	if g.cfg.Debug {
		flags = append(flags, "-g")
	}
	return g.appendAVXFlags(flags)
}

// detectWindowsCrossCompiler produces a PE/COFF binary for a Windows target
// from a non-Windows host (or a different Windows architecture) using a MinGW
// cross gcc or clang --target.
func (g *Generator) detectWindowsCrossCompiler(tg target.Target, cFile, exeFile string) (string, []string, error) {
	if cc := os.Getenv("CC"); cc != "" {
		return cc, []string{cFile, "-o", exeFile, "-mconsole", "-std=c2x", "-O0", "-Wno-psabi", "-lgmp", "-lws2_32"}, nil
	}
	prefix := target.MingwTriple(tg)
	searched := []string{prefix + "-gcc"}
	if _, err := exec.LookPath(prefix + "-gcc"); err == nil {
		flags := []string{cFile, "-o", exeFile, "-mconsole", "-std=c2x", "-O0", "-Wno-psabi", "-lgmp", "-lws2_32"}
		if g.cfg.Debug {
			flags = append(flags, "-g")
		}
		return prefix + "-gcc", g.appendAVXFlags(flags), nil
	}
	if _, err := exec.LookPath("clang"); err == nil {
		searched = append(searched, "clang --target="+tg.String())
		return "clang", []string{cFile, "-o", exeFile, "--target=" + target.MingwTriple(tg), "-std=c2x", "-O0", "-Wno-psabi", "-lgmp", "-lws2_32"}, nil
	}
	return "", nil, crossToolchainError(tg, searched...)
}

// detectLinuxCrossCompiler produces an ELF binary for a Linux target from a
// non-Linux host (or a different Linux architecture) using a triple-prefixed
// GNU cross gcc or clang --target.
func (g *Generator) detectLinuxCrossCompiler(tg target.Target, cFile, exeFile string) (string, []string, error) {
	if cc := os.Getenv("CC"); cc != "" {
		return cc, []string{cFile, "-o", exeFile, "-std=c2x", "-O0", "-Wno-psabi", "-lgmp", "-D_POSIX_C_SOURCE=200809L", "-lm"}, nil
	}
	arch := tg.Arch.String()
	names := []string{arch + "-linux-gnu-gcc", arch + "-pc-linux-gnu-gcc", arch + "-unknown-linux-gnu-gcc"}
	var searched []string
	for _, n := range names {
		searched = append(searched, n)
		if _, err := exec.LookPath(n); err == nil {
			flags := []string{cFile, "-o", exeFile, "-std=c2x", "-O0", "-Wno-psabi", "-lgmp", "-D_POSIX_C_SOURCE=200809L", "-lm"}
			if g.cfg.Debug {
				flags = append(flags, "-g")
			}
			return n, g.appendAVXFlags(flags), nil
		}
	}
	if _, err := exec.LookPath("clang"); err == nil {
		clangTarget := arch + "-unknown-linux-gnu"
		searched = append(searched, "clang --target="+clangTarget)
		return "clang", []string{cFile, "-o", exeFile, "--target=" + clangTarget, "-std=c2x", "-O0", "-Wno-psabi", "-lgmp"}, nil
	}
	return "", nil, crossToolchainError(tg, searched...)
}
