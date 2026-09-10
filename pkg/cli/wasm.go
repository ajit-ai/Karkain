package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"karkain/pkg/parser"
	"karkain/pkg/wasm"
)

func wasmBuildCommand(prog *parser.Program, sourceFile string, outputPath string, verbose bool) CommandResult {
	if verbose {
		fmt.Println("=== [wasm32-wasi] BUILD ===")
	}

	bin, err := wasm.CompileProgram(prog, sourceFile)
	if err != nil {
		return CommandResult{ExitCode: ExitCompile, Message: fmt.Sprintf("WASM Build Error: %v", err)}
	}

	out := outputPath
	if out == "" {
		base := strings.TrimSuffix(sourceFile, filepath.Ext(sourceFile))
		out = base + ".wasm"
	}

	if err := os.WriteFile(out, bin, 0o755); err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("write .wasm: %v", err)}
	}

	if verbose {
		fmt.Printf("wasm module: %d bytes → %s\n", len(bin), out)
	}
	return CommandResult{ExitCode: ExitSuccess, Message: out}
}

func wasmRunCommand(prog *parser.Program, sourceFile string, outputPath string, verbose bool) CommandResult {
	if verbose {
		fmt.Println("=== [wasm32-wasi] RUN ===")
	}

	bin, err := wasm.CompileProgram(prog, sourceFile)
	if err != nil {
		return CommandResult{ExitCode: ExitCompile, Message: fmt.Sprintf("WASM Build Error: %v", err)}
	}

	wt := findWasmtime()
	if wt == "" {
		return CommandResult{ExitCode: ExitEnv, Message: "wasmtime not found: install from https://wasmtime.dev"}
	}

	tmp, err := os.CreateTemp("", "karkain-*.wasm")
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("temp file: %v", err)}
	}
	tmpName := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpName)

	if err := os.WriteFile(tmpName, bin, 0o755); err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("write temp: %v", err)}
	}

	cmd := exec.Command(wt, "run", "--dir", ".", tmpName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("wasmtime: %v", err)}
	}
	return CommandResult{ExitCode: ExitSuccess}
}

func findWasmtime() string {
	if p, err := exec.LookPath("wasmtime"); err == nil {
		return p
	}
	home, _ := os.UserHomeDir()
	if home == "" {
		return ""
	}
	candidates := []string{
		filepath.Join(home, "bin", "wasmtime.exe"),
		filepath.Join(home, "bin", "wasmtime"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
