package cli

// Phase 148A/150D: the three `native-x86_64-*` CLI targets. The C-free
// machine-code backend (`pkg/native`, Phases 145/147/149) compiles here:
// emission is pure Go so `build` works from any host for any of the three
// containers, while `run` needs the host to match the target's OS and is
// refused elsewhere with the Phase-111 build-only hint (ExitEnv).
// kcc auto-routes exotic targets to these Go commands (main.go), so no
// kcc changes; kcc parity is Phase 151.

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"karkain/pkg/native"
	"karkain/pkg/parser"
)

// nativeImageExt is the default output extension per container. It is part of
// the user-visible contract: `build` with no -o writes
// <source dir>/build/<name><ext>, and a .exe is what makes the Windows image
// directly runnable on windows/amd64.
func nativeImageExt(osName string) string {
	switch osName {
	case native.OSWindows:
		return ".exe"
	case native.OSMacOS:
		return ".macho"
	default:
		return ".elf"
	}
}

// nativeHostCanRun reports whether a target image can be executed here. The
// check is the target's OS against the HOST, deliberately not a name match:
// that is what stops a Linux ELF from being handed to the Windows loader and
// reported back as a mysterious crash.
func nativeHostCanRun(osName string) bool {
	if runtime.GOARCH != "amd64" {
		return false
	}
	switch osName {
	case native.OSWindows:
		return runtime.GOOS == "windows"
	case native.OSMacOS:
		return runtime.GOOS == "darwin"
	default:
		return runtime.GOOS == "linux"
	}
}

func cfreeBuildCommand(prog *parser.Program, sourceFile string, outputPath string, verbose bool) CommandResult {
	return cfreeBuildForOS(prog, sourceFile, outputPath, native.OSLinux, NativeLinuxTarget, verbose)
}

// cfreeBuildForOS is the target-parameterised build. The target NAME travels
// with the OS so the verbose banner and every refusal names the thing the
// user actually typed instead of a fixed Linux string.
func cfreeBuildForOS(prog *parser.Program, sourceFile string, outputPath string, osName string, targetName string, verbose bool) CommandResult {
	if verbose {
		fmt.Printf("=== [%s] BUILD (no C compiler) ===\n", targetName)
	}

	img, err := native.CompileProgramForOS(prog, osName)
	if err != nil {
		return CommandResult{ExitCode: ExitCompile, Message: fmt.Sprintf("Native Build Error: %v", err)}
	}

	out := outputPath
	if out == "" {
		base := strings.TrimSuffix(sourceFile, filepath.Ext(sourceFile))
		buildDir := filepath.Join(filepath.Dir(sourceFile), "build")
		if err := os.MkdirAll(buildDir, 0o755); err != nil {
			return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("create build dir: %v", err)}
		}
		out = filepath.Join(buildDir, filepath.Base(base)+nativeImageExt(osName))
	}

	if err := os.WriteFile(out, img, 0o755); err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("write image: %v", err)}
	}
	// Same CreateTemp lesson as run: ensure the bit on rebuilds, where
	// WriteFile preserves the existing file's mode.
	if err := os.Chmod(out, 0o755); err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("chmod image: %v", err)}
	}

	if verbose {
		fmt.Printf("native image: %d bytes -> %s\n", len(img), out)
	}
	return CommandResult{ExitCode: ExitSuccess, Message: out}
}

func cfreeRunCommand(prog *parser.Program, sourceFile string, verbose bool) CommandResult {
	return cfreeRunForOS(prog, sourceFile, native.OSLinux, NativeLinuxTarget, verbose)
}

func cfreeRunForOS(prog *parser.Program, sourceFile string, osName string, targetName string, verbose bool) CommandResult {
	if verbose {
		fmt.Printf("=== [%s] RUN (no C compiler) ===\n", targetName)
	}

	if !nativeHostCanRun(osName) {
		return CommandResult{ExitCode: ExitEnv, Message: fmt.Sprintf(
			"cannot run a %s binary on %s/%s: cross-run requires an emulator or a remote target; use `karkain build --target %s -o <path>` to build only",
			targetName, runtime.GOOS, runtime.GOARCH, targetName)}
	}

	img, err := native.CompileProgramForOS(prog, osName)
	if err != nil {
		return CommandResult{ExitCode: ExitCompile, Message: fmt.Sprintf("Native Build Error: %v", err)}
	}

	tmp, err := os.CreateTemp("", "karkain-native-*"+nativeImageExt(osName))
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("temp file: %v", err)}
	}
	tmpName := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpName)

	if err := os.WriteFile(tmpName, img, 0o755); err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("write temp: %v", err)}
	}
	// os.WriteFile does not chmod an existing file: CreateTemp above made
	// it 0600, so set the executable bit explicitly (Linux runs it).
	if err := os.Chmod(tmpName, 0o755); err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("chmod temp: %v", err)}
	}

	cmd := exec.Command(tmpName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		// Phase-100 contract (mirrors RunCommand): a program that fails
		// at runtime is a program failure (ExitFailure), not a
		// compilation failure.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("Execution Error: %v", err)}
		}
		return CommandResult{ExitCode: classifyCompileError(err), Message: fmt.Sprintf("Execution Error: %v", err)}
	}
	return CommandResult{ExitCode: ExitSuccess, Message: ""}
}
