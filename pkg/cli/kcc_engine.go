package cli

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"karkain/pkg/codegen"
	"karkain/pkg/pm"
)

// winsockLibFlag returns "-lws2_32" on Windows hosts so native programs linking
// the net builtins resolve Winsock symbols (MinGW gcc ignores
// #pragma comment(lib, ...)). On POSIX hosts Winsock is not used and the flag
// is empty so links never reference a nonexistent library.
func winsockLibFlag() string {
	if runtime.GOOS == "windows" {
		return "-lws2_32"
	}
	return ""
}

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

// EngineFromEnv resolves the engine selection. Phase 97: the self-hosted
// engine (kcc) is the DEFAULT engine. KARKAIN_ENGINE=go explicitly selects the
// Go front end; anything else (and no value) uses kcc. The parity gates
// (phase95/phase96) prove kcc reproduces the conformance corpus, probe goldens
// and the test runner, so routing the core pipeline through kcc by default is
// safe while keeping the Go engine reachable.
func EngineFromEnv() EngineKind {
	if v := os.Getenv("KARKAIN_ENGINE"); v == "go" || v == "Go" || v == "GO" {
		return EngineGo
	}
	return EngineKCC
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

// kccTargetPreflight validates every @target(...) attribute in the assembled
// source with the Go front-end analyzer before the self-hosted engine runs.
// kcc preserves the attribute as an inert C comment (Phase 98) and cannot
// reject an unknown target by itself, so the Go NPU analyzer is the semantic
// authority here, just as it is for `karkain check` on the Go engine. When it
// reports, the diagnostics have been rendered and the caller must terminate
// with ExitCompile (the check path) so `@target(unknown)` never silently
// compiles into an inert comment.
func kccTargetPreflight(file, src string) (bool, int) {
	diags := npuTargetDiagnostics(file, src)
	if len(diags) == 0 {
		return false, 0
	}
	renderDiagnostics(src, diags)
	return true, len(diags)
}

// kccStagedPreflight runs npuTargetDiagnostics over every .kark file staged in
// a sandbox directory (used by the self-hosted test runner for mirrored test
// dirs), returning the aggregate failure count so the caller can stop before
// kcc compiles inert @target comments.
func kccStagedPreflight(dir string) (bool, int) {
	bad := 0
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".kark") {
			return nil
		}
		b, _ := os.ReadFile(path)
		if bFailed, n := kccTargetPreflight(path, string(b)); bFailed {
			bad += n
		}
		return nil
	})
	return bad > 0, bad
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
	// Bootstrap contract: stage-1 must emit src/compiler/main.c through the Go
	// front end (pkg/bootstrap forceGoEngine). Pinning the engine here prevents
	// a default-kcc re-entry that would otherwise recurse through buildKCC when
	// the on-disk kcc.exe is stale (the very condition this rebuild is fixing).
	buildCmd.Env = append(os.Environ(), "KARKAIN_ENGINE=go")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("stage-1 build failed: %v\n%s", err, string(out))
	}

	stage1 := filepath.Join(srcDir, "main.c")
	bin := filepath.Join(root, "kcc.exe")
	linkCmd := exec.Command("gcc", "-std=c99", "-x", "c", "-D_POSIX_C_SOURCE=200809L", stage1, "-o", bin, "-lgmp", "-lm", winsockLibFlag())
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

// kccCheckPreflight runs the self-hosted engine's checker over an assembled
// source file. When the program is semantically invalid, the checker's
// diagnostics (error[K1xx] lines) are returned so `karkain build`/`karkain run`
// reject it with the same clean report and ExitCompile the check path uses —
// never raw gcc noise about an undeclared identifier. Returns ok=true (with
// the [ok] confirmation) when the checker is clean.
func kccCheckPreflight(bin, checkFile string) (ok bool, out string) {
	out, code := runKCC(bin, "check", checkFile)
	if code == 0 && strings.Contains(out, "[ok]") {
		return true, out
	}
	return false, out
}

// KCCCheckCommand routes `karkain check` through the self-hosted engine. The
// corpus contract is the same as the Go path: parsing had better be clean.
// Phase 97: the source is assembled (manifest dependencies + siblings) into a
// temp file so kcc check validates the full project, matching the Go check
// pipeline (resolveSourcesCheck).
func KCCCheckCommand(w io.Writer, file string, verbose bool) CommandResult {
	// Phase 105: run the Go-side multi-file syntax preflight before handing to
	// the self-hosted engine, so BOTH engine paths surface every recoverable
	// syntax error in one invocation with precise per-file spans. Consistent
	// with kccTargetPreflight, which already runs Go-side for both engines.
	if errDiags := projectSyntaxDiagnostics(file); len(errDiags) > 0 {
		renderDiagnostics("", errDiags)
		return CommandResult{ExitCode: ExitCompile, Message: checkStageMessage(checkStageSyntax, len(errDiags))}
	}
	bin, err := kccBinaryPath(w)
	if err != nil {
		return CommandResult{ExitCode: ExitEnv, Message: err.Error()}
	}
	prog, err := kccAssembleSource(file)
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: err.Error()}
	}
	if bad, n := kccTargetPreflight(file, prog); bad {
		return CommandResult{ExitCode: ExitCompile, Message: checkStageMessage(checkStageSema, n)}
	}
	sandbox, err := os.MkdirTemp("", "karkain-kcc-check")
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
	// Phase 122: flat projects are staged (root + siblings + stdlib tree) so the
	// self-hosted engine's own assembler (assembleProject) composes the input;
	// everything else keeps the legacy single assembled file.
	checkFile, err := kccStageInput(file, sandbox)
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: err.Error()}
	}
	out, code := runKCC(bin, "check", checkFile)
	if code == 0 && strings.Contains(out, "[ok]") {
		return CommandResult{ExitCode: ExitSuccess, Message: out}
	}
	return CommandResult{ExitCode: ExitCompile, Message: out}
}

// KCCBuildCommand routes `karkain build` through the self-hosted engine. It
// compiles the Karkain source to C23 and, unless compile-only, links a native
// executable with gcc. Phase 97: the Go-side assembler (resolveSources) pulls in
// manifest dependencies and sibling modules so the self-hosted engine compiles
// the full project, not just the root file. Assembly happens in a temp sandbox
// (as KCCRunCommand does) so kcc only ever loads the single assembled file and
// build artifacts never sit next to the source.
func KCCBuildCommand(w io.Writer, file, outputPath string, cfg codegen.Config, verbose bool) CommandResult {
	bin, err := kccBinaryPath(w)
	if err != nil {
		return CommandResult{ExitCode: ExitEnv, Message: err.Error()}
	}
	prog, err := kccAssembleSource(file)
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: err.Error()}
	}
	if bad, n := kccTargetPreflight(file, prog); bad {
		return CommandResult{ExitCode: ExitCompile, Message: checkStageMessage(checkStageSema, n)}
	}

	sandbox, err := os.MkdirTemp("", "karkain-kcc-build")
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
	base := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	// Phase 122: same staging decision as KCCCheckCommand — the self-hosted
	// engine assembles flat projects itself inside the sandbox.
	srcFile, err := kccStageInput(file, sandbox)
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: err.Error()}
	}

	// Phase 117: same self-hosted checker gate as KCCRunCommand.
	if ok, chkOut := kccCheckPreflight(bin, srcFile); !ok {
		return CommandResult{ExitCode: ExitCompile, Message: chkOut}
	}

	out, code := runKCC(bin, "build", srcFile, "--target", "c23")
	if code != 0 || !strings.Contains(out, "[ok]") {
		return CommandResult{ExitCode: ExitCompile, Message: out}
	}
	// kcc writes the C23 artifact next to the assembled source, in the sandbox.
	artifact := replaceExt(srcFile, ".c23")

	if cfg.CompileOnly {
		// Surface the C artifact next to the source like the classic kcc
		// contract (src/<base>.c23) so --compile-only callers can consume it.
		userC23 := replaceExt(file, ".c23")
		if data, rerr := os.ReadFile(artifact); rerr == nil {
			_ = os.WriteFile(userC23, data, 0o644)
		}
		return CommandResult{ExitCode: ExitSuccess, Message: out}
	}

	if _, err := exec.LookPath("gcc"); err != nil {
		return CommandResult{ExitCode: ExitEnv, Message: "gcc not available to link the self-hosted engine output"}
	}
	exe := outputPath
	if exe == "" {
		exe = filepath.Join(filepath.Dir(file), base)
		if filepath.Ext(exe) == "" {
			exe += ".exe"
		}
	}
	// Phase 137: -std=c2x like the Go reference backend (the embedded
	// concurrency runtime needs C11 atomics on this link path too).
	linkCmd := exec.Command("gcc", "-std=c2x", "-x", "c", "-D_POSIX_C_SOURCE=200809L", artifact, "-o", exe, "-lgmp", "-lm", winsockLibFlag())
	if lout, err := linkCmd.CombinedOutput(); err != nil {
		return CommandResult{ExitCode: ExitEnv, Message: fmt.Sprintf("gcc link failed: %v\n%s", err, string(lout))}
	}
	if verbose && w != nil {
		fmt.Fprintf(w, "linked %s\n", exe)
	}
	return CommandResult{ExitCode: ExitSuccess, Message: out}
}

// kccAssembleSource returns the fully assembled Karkain source for a target
// file, mirroring the Go engine's module-aware loader (resolveSourcesRun):
// import-driven module assembly first (stdlib/local/registered modules via the
// module graph), falling back to the classic paths (project manifest
// dependencies + same-directory sibling modules upstream, root last) when the
// entry file declares no imports. Module import declarations
// (`import std.string`, `import math`) are then stripped from the text because
// the self-hosted parser only understands bare module names — `import math` —
// and chokes on the dotted stdlib names; all imported units are already
// present in the assembled text, so the declarations are pure surface syntax
// for the entry file. C import blocks (`import "C" { ... }`) are preserved.
func kccAssembleSource(file string) (string, error) {
	text, err := resolveSourcesRun(file)
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(`(?m)^[ \t]*import[ \t]+[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z0-9_]+)*[ \t]*\r?$`)
	return re.ReplaceAllString(text, ""), nil
}

// ---- Phase 122: flat project detection and sandbox mirroring ----

// dirHasKark reports whether dir contains at least one .kark entry.
func dirHasKark(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".kark") {
			return true
		}
	}
	return false
}

// findKarkainStdlib walks upward from startDir looking for the Karkain standard
// library tree (a directory containing stdlib/string/string.kark). Returns ""
// when no stdlib tree is an ancestor.
func findKarkainStdlib(startDir string) string {
	dir := startDir
	for {
		cand := filepath.Join(dir, "stdlib", "string", "string.kark")
		if info, err := os.Stat(cand); err == nil && !info.IsDir() {
			return filepath.Join(dir, "stdlib")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// copyFileContents copies a file's bytes to dstPath.
func copyFileContents(srcPath, dstPath string) error {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	return os.WriteFile(dstPath, data, 0o644)
}

// copyKarkDir copies every *.kark file from srcDir into dstDir (created),
// skipping subdirectories.
func copyKarkDir(srcDir, dstDir string) error {
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".kark") {
			continue
		}
		if err := copyFileContents(filepath.Join(srcDir, e.Name()), filepath.Join(dstDir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// flatAssemblyEligible reports whether a target's project is FLAT: compilable
// by the self-hosted engine's own assembler (assembleProject in
// src/compiler/main.kark) with no Go-side module-graph assistance. A
// project is flat when it is not a manifest/workspace project (karkain.toml
// present), its root parses, and every declared import is either a
// standard-library name (std.*, with a stdlib tree available) or resolves to a
// sibling <name>.kark file or <name>/ directory of .kark files. Everything
// else keeps the legacy Go module-aware assembly.
func flatAssemblyEligible(targetFile string) (bool, error) {
	root := effectiveRootFile(targetFile)
	if proj, err := pm.FindProjectRoot(filepath.Dir(root)); err == nil {
		if _, serr := os.Stat(filepath.Join(proj, pm.ManifestFile)); serr == nil {
			return false, nil
		}
	}
	data, err := os.ReadFile(root)
	if err != nil {
		return false, err
	}
	prog := parseProgramStrict(string(data))
	if prog == nil {
		return false, nil
	}
	if len(prog.Imports) == 0 {
		return true, nil
	}
	dir := filepath.Dir(root)
	for _, imp := range prog.Imports {
		if strings.HasPrefix(imp.Name, "std.") {
			if findKarkainStdlib(dir) == "" {
				return false, nil
			}
			continue
		}
		if info, serr := os.Stat(filepath.Join(dir, imp.Name+".kark")); serr == nil && !info.IsDir() {
			continue
		}
		if dirHasKark(filepath.Join(dir, imp.Name)) {
			continue
		}
		return false, nil
	}
	return true, nil
}

// kccMirrorFlat stages a flat project's sources into the sandbox so the
// self-hosted engine's own assembler (assembleProject) composes the input
// text: the root file plus every sibling .kark, sibling module directories
// containing .kark files, and the standard-library tree under sandbox/lib are
// mirrored verbatim, then kcc is run on the staged root. Returns the staged
// root path. The file set is mirrored in sorted order so the assembly is
// deterministic and byte-identical to the legacy Go sibling join.
func kccMirrorFlat(targetFile, sandbox string) (string, error) {
	root := effectiveRootFile(targetFile)
	dir := filepath.Dir(root)
	base := filepath.Base(root)
	stagedRoot := filepath.Join(sandbox, base)
	if err := copyFileContents(root, stagedRoot); err != nil {
		return "", err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		if name == base {
			continue
		}
		srcPath := filepath.Join(dir, name)
		if strings.HasSuffix(name, ".kark") {
			if err := copyFileContents(srcPath, filepath.Join(sandbox, name)); err != nil {
				return "", err
			}
			continue
		}
		if info, serr := os.Stat(srcPath); serr == nil && info.IsDir() && dirHasKark(srcPath) {
			if err := copyKarkDir(srcPath, filepath.Join(sandbox, name)); err != nil {
				return "", err
			}
		}
	}
	if sb := findKarkainStdlib(dir); sb != "" {
		subs, rerr := os.ReadDir(sb)
		if rerr == nil {
			for _, sub := range subs {
				if !sub.IsDir() {
					continue
				}
				subName := sub.Name()
				if _, serr := os.Stat(filepath.Join(sb, subName, subName+".kark")); serr == nil {
					if err := copyKarkDir(filepath.Join(sb, subName), filepath.Join(sandbox, "lib", subName)); err != nil {
						return "", err
					}
				}
			}
		}
	}
	return stagedRoot, nil
}

// kccStageInput composes the file the self-hosted engine will compile inside a
// sandbox and returns its absolute path. For flat projects the root file is
// mirrored so kcc's own assembleProject builds the assembly from the staged
// project; otherwise the legacy Go module-aware assembler writes the single
// assembled file.
func kccStageInput(file, sandbox string) (string, error) {
	flat, err := flatAssemblyEligible(file)
	if err != nil {
		return "", err
	}
	if flat {
		return kccMirrorFlat(file, sandbox)
	}
	prog, err := kccAssembleSource(file)
	if err != nil {
		return "", err
	}
	base := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	checkFile := filepath.Join(sandbox, base+".kark")
	if err := os.WriteFile(checkFile, []byte(prog), 0o644); err != nil {
		return "", err
	}
	return checkFile, nil
}

// kccTestSource returns the dependency-aware content for a test file: the
// project module scope (projectModuleSources — local/workspace/registry/git
// dependency sources + sibling modules without `func main`) prepended to the
// original file. Flat/non-project files return their verbatim content.
func kccTestSource(testFile string) (string, error) {
	orig, err := os.ReadFile(testFile)
	if err != nil {
		return "", err
	}
	modSrc, err := projectModuleSources(testFile)
	if err != nil {
		return "", err
	}
	if modSrc == "" {
		return string(orig), nil
	}
	return modSrc + "\n\n" + string(orig), nil
}

// kccMirrorTestDir recursively copies a test directory into dst, prepending the
// project module scope to every *_test.kark (dependency-aware test drivers).
// Non-test files are copied verbatim so the compiler/toolchain layout is
// preserved. Returns an error only when the source cannot be read.
func kccMirrorTestDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := os.MkdirAll(dstPath, 0o755); err != nil {
				return err
			}
			if err := kccMirrorTestDir(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}
		if strings.HasSuffix(e.Name(), "_test.kark") {
			prog, perr := kccTestSource(srcPath)
			if perr != nil {
				return perr
			}
			if werr := os.WriteFile(dstPath, []byte(prog), 0o644); werr != nil {
				return werr
			}
			continue
		}
		data, rerr := os.ReadFile(srcPath)
		if rerr != nil {
			return rerr
		}
		if werr := os.WriteFile(dstPath, data, 0o644); werr != nil {
			return werr
		}
	}
	return nil
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
	// Phase 122: same staging decision as KCCCheckCommand — the self-hosted
	// engine assembles flat projects itself inside the sandbox.
	copyPath, err := kccStageInput(file, sandbox)
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: err.Error()}
	}

	// Phase 117: gate on the self-hosted checker (like KCCCheckCommand) so a
	// semantically invalid program fails here with error[K1xx] + ExitCompile
	// instead of surfacing as a confusing gcc link failure exit code.
	if ok, chkOut := kccCheckPreflight(bin, copyPath); !ok {
		return CommandResult{ExitCode: ExitCompile, Message: chkOut}
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
	// Phase 137: -std=c2x like the Go reference backend (the embedded
	// concurrency runtime needs C11 atomics on this link path too).
	linkCmd := exec.Command("gcc", "-std=c2x", "-x", "c", "-D_POSIX_C_SOURCE=200809L", c23, "-o", exe, "-lgmp", "-lm", winsockLibFlag())
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
// user's working directory. Phase 97: when the test files live inside a Karkain
// project, each test source is prefixed with the project module scope
// (projectModuleSources — manifest dependencies + sibling modules, no `func
// main`) so the self-hosted test drivers can exercise project code, matching
// the Go test pipeline. Flat/non-project directories are copied verbatim and
// behave exactly as before.
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

	target := abs
	if info, serr := os.Stat(abs); serr == nil {
		if info.IsDir() {
			// Mirror the directory into the sandbox, injecting the project
			// module scope into each *_test.kark.
			if err := kccMirrorTestDir(abs, sandbox); err != nil {
				return CommandResult{ExitCode: ExitFailure, Message: err.Error()}
			}
			target = sandbox
		} else {
			prog, aerr := kccTestSource(abs)
			if aerr != nil {
				return CommandResult{ExitCode: ExitFailure, Message: aerr.Error()}
			}
			staged := filepath.Join(sandbox, filepath.Base(abs))
			if werr := os.WriteFile(staged, []byte(prog), 0o644); werr != nil {
				return CommandResult{ExitCode: ExitFailure, Message: werr.Error()}
			}
			target = staged
		}
	}

	// Preflight every staged test source: kcc treats @target(...) as inert C
	// comments, so invalid targets are rejected here before any test runs.
	if bad, n := kccStagedPreflight(sandbox); bad {
		return CommandResult{ExitCode: ExitCompile, Message: checkStageMessage(checkStageSema, n)}
	}

	args := []string{"test", target}
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
