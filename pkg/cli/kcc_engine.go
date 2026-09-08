package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"karkain/pkg/codegen"
)

// EngineKind selects which front end powers the check/build/run commands.
// Phase 95: the self-hosted compiler (kcc, bootstrapped from src/compiler) is
// the primary engine for lex + parse + C codegen. The Go front end remains the
// fallback for the LSP, test runner and tooling that still depend on parse ASTs.
type EngineKind int

const (
	// EngineGo uses the in-tree Go front end (pkg/lexer + pkg/parser + pkg/codegen).
	EngineGo EngineKind = iota
	// EngineKCC uses the self-hosted compiler binary produced from src/compiler.
	EngineKCC
)

// EngineFromEnv resolves the engine selection. KARKAIN_ENGINE=kcc selects the
// self-hosted engine; anything else (and no value) keeps the Go engine.
func EngineFromEnv() EngineKind {
	if v := os.Getenv("KARKAIN_ENGINE"); v == "kcc" {
		return EngineKCC
	}
	return EngineGo
}

// EngineFlag recomputes the engine after a CLI --engine flag was parsed.
// Valid values: "go" and "kcc". Returns the resolved kind or an error for an
// unrecognized value.
func EngineFlag(v string) (EngineKind, error) {
	switch v {
	case "go":
		return EngineGo, nil
	case "kcc":
		return EngineKCC, nil
	default:
		return EngineGo, fmt.Errorf("unknown engine %q (supported: go, kcc)", v)
	}
}

// kccRepoRoot locates the Karkain repository root (the directory containing
// cmd/karkain and src/compiler). It walks upward from the working directory so
// the CLI can find src/compiler when invoked from a subdirectory.
func kccRepoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "src", "compiler", "main.kark")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
dir = parent
	}
}

// kccBinaryPath resolves the self-hosted compiler binary. Resolution order:
//  1. KARKAIN_KCC environment variable (explicit path, never built)
//  2. <kccRepoRoot>/kcc[.exe] â€” built on demand when src/compiler is present
//
// When building is required but impossible (no repo, no gcc), an error is
// returned so callers can fall back or report the missing engine.
func kccBinaryPath(w io.Writer) (string, error) {
	if env := os.Getenv("KARKAIN_KCC"); env != "" {
		if _, err := os.Stat(env); err != nil {
			return "", fmt.Errorf("KARKAIN_KCC points to missing binary %q", env)
		}
		return env, nil
	}
	root := kccRepoRoot()
	if root == "" {
		return "", fmt.Errorf("cannot locate src/compiler to build the self-hosted engine (set KARKAIN_KCC)")
	}
bin := filepath.Join(root, "kcc.exe")
	if _, err := os.Stat(bin); err == nil && !kccStale(root, bin) {
		return bin, nil
	}
	return buildKCC(root, w)
}

// kccStale reports whether the on-disk kcc binary predates any of the
// self-hosted sources it was compiled from. A stale binary silently produces
// incorrect output, so the engine rebuilds whenever a src/compiler/*.kark
// source is newer than the binary.
func kccStale(root, bin string) bool {
	binInfo, err := os.Stat(bin)
	if err != nil {
		return true
	}
	srcDir := filepath.Join(root, "src", "compiler")
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return true
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".kark") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return true
		}
		if info.ModTime().After(binInfo.ModTime()) {
			return true
		}
	}
	return false
}

// buildKCC bootstraps the self-hosted compiler: it compiles src/compiler via
// the Go bootstrap compiler into C23, then links the stage-1 binary with gcc.
// Produces <root>/kcc.exe (gitignored build artifact).
func buildKCC(root string, w io.Writer) (string, error) {
	karkain, err := bootstrapKarkainBinary(root, w)
	if err != nil {
		return "", err
	}
	if _, err := exec.LookPath("gcc"); err != nil {
		return "", fmt.Errorf("gcc not available to build the self-hosted engine")
	}

	srcDir := filepath.Join(root, "src", "compiler")
	if _, err := os.Stat(filepath.Join(srcDir, "main.kark")); err != nil {
		return "", fmt.Errorf("src/compiler not found (not in a Karkain repo?)")
	}

	buildCmd := exec.Command(karkain, "build", filepath.Join(srcDir, "main.kark"))
	buildCmd.Dir = srcDir
	if out, err := buildCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("stage-1 build failed: %v\n%s", err, string(out))
	}

	stage1 := filepath.Join(srcDir, "main.c")
	bin := filepath.Join(root, "kcc.exe")
	linkCmd := exec.Command("gcc", "-std=c99", "-x", "c", stage1, "-o", bin, "-lgmp")
	linkCmd.Dir = srcDir
	if out, err := linkCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("stage-1 link failed: %v\n%s", err, string(out))
	}
	if w != nil {
		fmt.Fprintf(w, "bootstrapped self-hosted engine: %s\n", bin)
	}
	return bin, nil
}

// runKCC executes the self-hosted compiler binary with the given arguments and
// returns its combined output (stdout + stderr).
func runKCC(bin string, args ...string) (string, int) {
	return runKCCDir(bin, "", args...)
}

// runKCCDir is runKCC with a working directory (used to sandbox kcc's
// synthesized test-driver artifacts).
func runKCCDir(bin, dir string, args ...string) (string, int) {
	cmd := exec.Command(bin, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			code = -1
		}
	}
	return strings.TrimSpace(string(out)), code
}

// KCCCheckCommand routes `karkain check` through the self-hosted engine. The
// corpus contract is the same as the Go path: parsing had better be clean.
func KCCCheckCommand(w io.Writer, file string, verbose bool) CommandResult {
	bin, err := kccBinaryPath(w)
	if err != nil {
		return CommandResult{ExitCode: ExitEnv, Message: err.Error()}
	}
	out, code := runKCC(bin, "check", file)
	if code == 0 && strings.Contains(out, "[ok]") {
		return CommandResult{ExitCode: ExitSuccess, Message: out}
	}
	return CommandResult{ExitCode: ExitCompile, Message: out}
}

// KCCBuildCommand routes `karkain build` through the self-hosted engine. It
// compiles the Karkain source to C23 and, unless compile-only, links a native
// executable with gcc. The C artifact is written next to the source (the kcc
// contract) so `--compile-only` mirrors the Go CLI's behavior.
func KCCBuildCommand(w io.Writer, file, outputPath string, cfg codegen.Config, verbose bool) CommandResult {
	bin, err := kccBinaryPath(w)
	if err != nil {
		return CommandResult{ExitCode: ExitEnv, Message: err.Error()}
	}
	out, code := runKCC(bin, "build", file, "--target", "c23")
	if code != 0 || !strings.Contains(out, "[ok]") {
		return CommandResult{ExitCode: ExitCompile, Message: out}
	}

	c23 := replaceExt(file, ".c23")
	if cfg.CompileOnly {
		return CommandResult{ExitCode: ExitSuccess, Message: out}
	}

	if _, err := exec.LookPath("gcc"); err != nil {
		return CommandResult{ExitCode: ExitEnv, Message: "gcc not available to link the self-hosted engine output"}
	}
	exe := outputPath
	if exe == "" {
		base := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
		exe = filepath.Join(filepath.Dir(file), base)
		if filepath.Ext(exe) == "" {
			exe += ".exe"
		}
	}
	linkCmd := exec.Command("gcc", "-std=c99", "-x", "c", c23, "-o", exe, "-lgmp", "-lm")
	if out, err := linkCmd.CombinedOutput(); err != nil {
		return CommandResult{ExitCode: ExitEnv, Message: fmt.Sprintf("gcc link failed: %v\n%s", err, string(out))}
	}
	if verbose && w != nil {
		fmt.Fprintf(w, "linked %s\n", exe)
	}
	return CommandResult{ExitCode: ExitSuccess, Message: out}
}

// KCCRunCommand routes `karkain run` through the self-hosted engine: build to
// C23, link with gcc, then execute the native binary and surface its output.
func KCCRunCommand(w io.Writer, file string, cfg codegen.Config, verbose bool) CommandResult {
	bin, err := kccBinaryPath(w)
	if err != nil {
		return CommandResult{ExitCode: ExitEnv, Message: err.Error()}
	}
base := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))

	// Work in a temp sandbox so build artifacts never sit next to the source.
	sandbox, err := os.MkdirTemp("", "karkain-kcc-run")
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: err.Error()}
	}
	defer func() {
		for i := 0; i < 25; i++ {
			if err := os.RemoveAll(sandbox); err == nil {
				return
			}
			os.RemoveAll(sandbox)
		}
	}()
	copyPath := filepath.Join(sandbox, base+".kark")
	data, err := os.ReadFile(file)
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: err.Error()}
	}
	if err := os.WriteFile(copyPath, data, 0o644); err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: err.Error()}
	}

	out, code := runKCC(bin, "build", copyPath, "--target", "c23")
	if code != 0 || !strings.Contains(out, "[ok]") {
		return CommandResult{ExitCode: ExitCompile, Message: out}
	}
	if _, err := exec.LookPath("gcc"); err != nil {
		return CommandResult{ExitCode: ExitEnv, Message: "gcc not available to link the self-hosted engine output"}
	}
	c23 := replaceExt(copyPath, ".c23")
	exe := filepath.Join(sandbox, base+".exe")
	linkCmd := exec.Command("gcc", "-std=c99", "-x", "c", c23, "-o", exe, "-lgmp", "-lm")
	if lout, err := linkCmd.CombinedOutput(); err != nil {
		return CommandResult{ExitCode: ExitEnv, Message: fmt.Sprintf("gcc link failed: %v\n%s", err, string(lout))}
	}
	runCmd := exec.Command(exe)
	runOut, err := runCmd.CombinedOutput()
	msg := out
	if len(runOut) > 0 {
		if msg != "" {
			msg += "\n"
		}
		msg += strings.TrimSpace(string(runOut))
	}
if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: msg}
	}
	return CommandResult{ExitCode: ExitSuccess, Message: msg}
}

// kccSummaryRe matches the compact result line emitted by the self-hosted test
// runner (identical format to Go's summaryLine): "N passed; M failed; S skipped; T total".
var kccSummaryRe = regexp.MustCompile(`(\d+) passed;\s*(\d+) failed;\s*(\d+) skipped;\s*(\d+) total`)

// KCCTestCommand routes `karkain test` through the self-hosted engine. kcc
// discovers *_test.kark files, synthesizes drivers and reports per-test
// PASS/FAIL plus the aggregate summary. Because the kcc process itself exits 0
// (generated main always returns 0), the Go wrapper maps the parsed summary to
// the ExitTest(4) exit code, exactly like TestCommandFiltered. The kcc process
// runs in a temp sandbox so its synthesized driver artifacts never land in the
// user's working directory.
func KCCTestCommand(w io.Writer, testPath, filter string) CommandResult {
	bin, err := kccBinaryPath(w)
	if err != nil {
		return CommandResult{ExitCode: ExitEnv, Message: err.Error()}
	}
	abs, err := filepath.Abs(testPath)
	if err != nil {
		abs = testPath
	}
	sandbox, err := os.MkdirTemp("", "karkain-kcc-test")
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: err.Error()}
	}
	defer func() {
		for i := 0; i < 25; i++ {
			if err := os.RemoveAll(sandbox); err == nil {
				return
			}
			os.RemoveAll(sandbox)
		}
	}()

	args := []string{"test", abs}
	if filter != "" {
		args = append(args, filter)
	}
	out, _ := runKCCDir(bin, sandbox, args...)
	out = strings.TrimSpace(out)

	// No test files / clean directory: mirror TestCommandFiltered.
	if strings.Contains(out, "No test files found.") {
		return CommandResult{ExitCode: ExitSuccess, Message: out}
	}
	m := kccSummaryRe.FindStringSubmatch(out)
	if m == nil {
		return CommandResult{ExitCode: ExitFailure, Message: out}
	}
	failed, _ := strconv.Atoi(m[2])
	if failed > 0 {
		return CommandResult{ExitCode: ExitTest, Message: out}
	}
	return CommandResult{ExitCode: ExitSuccess, Message: out}
}

// replaceExt swaps a path's extension for a new one while preserving the base.
func replaceExt(path, newExt string) string {
	return strings.TrimSuffix(path, filepath.Ext(path)) + newExt
}

// bootstrapKarkainBinary returns the Go bootstrap compiler binary, building
// root/karkain.exe on demand (mirrors buildBootstrap in phase88_test.go).
func bootstrapKarkainBinary(root string, w io.Writer) (string, error) {
	bin := filepath.Join(root, "karkain.exe")
	if _, err := os.Stat(bin); err == nil {
		return bin, nil
	}
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/karkain/")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to build bootstrap compiler: %v\n%s", err, string(out))
	}
	if w != nil {
		fmt.Fprintf(w, "built bootstrap compiler: %s\n", bin)
	}
	return bin, nil
}
