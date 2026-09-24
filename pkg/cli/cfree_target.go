package cli

// Phase 148A: `native-x86_64-linux` CLI commands. The C-free machine-code
// backend (`pkg/native`, Phases 145/147) compiles here: emission is pure
// Go so `build` works from any host, while `run` needs linux/amd64 and
// is refused elsewhere with the Phase-111 build-only hint (ExitEnv).
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

func cfreeBuildCommand(prog *parser.Program, sourceFile string, outputPath string, verbose bool) CommandResult {
	if verbose {
		fmt.Println("=== [native-x86_64-linux] BUILD (no C compiler) ===")
	}

	img, err := native.CompileProgram(prog)
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
		out = filepath.Join(buildDir, filepath.Base(base)+".elf")
	}

	if err := os.WriteFile(out, img, 0o755); err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("write image: %v", err)}
	}

	if verbose {
		fmt.Printf("native image: %d bytes → %s\n", len(img), out)
	}
	return CommandResult{ExitCode: ExitSuccess, Message: out}
}

func cfreeRunCommand(prog *parser.Program, sourceFile string, verbose bool) CommandResult {
	if verbose {
		fmt.Println("=== [native-x86_64-linux] RUN (no C compiler) ===")
	}

	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		return CommandResult{ExitCode: ExitEnv, Message: fmt.Sprintf(
			"cannot run a native-x86_64-linux binary on %s/%s: cross-run requires an emulator or a remote target; use `karkain build --target native-x86_64-linux -o <path>` to build only",
			runtime.GOOS, runtime.GOARCH)}
	}

	img, err := native.CompileProgram(prog)
	if err != nil {
		return CommandResult{ExitCode: ExitCompile, Message: fmt.Sprintf("Native Build Error: %v", err)}
	}

	tmp, err := os.CreateTemp("", "karkain-native-*")
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("temp file: %v", err)}
	}
	tmpName := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpName)

	if err := os.WriteFile(tmpName, img, 0o755); err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("write temp: %v", err)}
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
