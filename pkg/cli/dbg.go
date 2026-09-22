package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"karkain/pkg/codegen"
)

// Live-debugger walk, Phase 140.
//
// `karkain dbg <file.kark>` builds the program with debug symbols (the Go
// engine's `-g` path — the self-hosted kcc engine is not yet trace-aware,
// the same explicit boundary as Phase 112 tracing) and walks it under gdb
// in batch mode: a breakpoint regex over the deterministic `karkain_user_*`
// namespace, then repeated backtrace/continue pairs, keeping the deepest
// observed stack. Frames demangle to Karkain function names with honest C
// file:line locations (no `#line` directives are emitted anywhere — the
// determinism gates forbid the byte surface — so locations name generated
// C, and the Karkain content is the function sequence).
//
// gdb missing → ExitEnv with an install hint, never a fake trace. The walk
// succeeding is exit 0 even when the inferior itself exits nonzero (a
// debugger reports; it does not judge). lldb stays a documented
// alternative: same batch shape, unwired on this host.

// dbgMaxSteps bounds the breakpoint walk (run + bt/continue pairs). It caps
// the deepest observable chain; straight-line programs well under it are
// exact, deeper ones report the deepest window observed.
const dbgMaxSteps = 10

// dbgTimeout bounds the whole gdb walk so a hanging inferior cannot hang
// the command.
const dbgTimeout = 120 * time.Second

// dbgFrame is one parsed backtrace frame.
type dbgFrame struct {
	Func string // Karkain-demangled function name (karkain_user_ stripped)
	At   string // "file:line" when gdb reported a location, else ""
}

// dbgBtLine matches gdb batch backtrace lines:
// `#0  karkain_user_inner () at /tmp/x/prog.c:123`
// `#1  0x7fff... in karkain_user_outer () at /tmp/x/prog.c:110`
// Location-less frames (shared-lib entries without debuginfo) match with an
// empty location group.
var dbgBtLine = regexp.MustCompile(`^#(\d+)\s+(?:0x[0-9a-fA-F]+\s+in\s+)?(\S+?)(?:\s*\(.*\))?(?:\s+at\s+(\S+))?\s*$`)

// demangleDbgFunc strips the deterministic user namespace; C runtime frames
// (main, _start, __libc_start_main, ...) pass through untouched.
func demangleDbgFunc(fn string) string {
	return strings.TrimPrefix(fn, "karkain_user_")
}

// findGDB locates a runnable gdb binary.
func findGDB() string {
	if p, err := exec.LookPath("gdb"); err == nil {
		return p
	}
	return ""
}

// DbgCommand implements `karkain dbg <file.kark>`.
func DbgCommand(targetFile string, verbose bool) CommandResult {
	if err := ValidateKarFile(targetFile); err != nil {
		return CommandResult{ExitCode: ExitUsage, Message: err.Error()}
	}
	if _, err := os.Stat(targetFile); err != nil {
		return CommandResult{ExitCode: ExitUsage, Message: fmt.Sprintf("cannot open '%s': %v", targetFile, err)}
	}
	gdb := findGDB()
	if gdb == "" {
		return CommandResult{ExitCode: ExitEnv,
			Message: "gdb not found on PATH: `karkain dbg` needs GDB (install Cypress/MinGW gdb on Windows, the gdb package on Linux) — no trace was produced"}
	}

	sandbox, err := os.MkdirTemp("", "karkain-dbg")
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: err.Error()}
	}
	defer os.RemoveAll(sandbox)

	// Stage the source so the beside-source .c artifact (historical CLI
	// behavior) lands in the sandbox, never next to user code.
	raw, err := os.ReadFile(targetFile)
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: err.Error()}
	}
	staged := filepath.Join(sandbox, filepath.Base(targetFile))
	if err := os.WriteFile(staged, raw, 0o644); err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: err.Error()}
	}
	exe := filepath.Join(sandbox, "dbgprog")
	if os.PathSeparator == '\\' {
		exe += ".exe"
	}
	// Compile-only -g build through the Go engine (mirrors RunCommand's
	// gates — syntax, borrow, semantic — but never executes: execution is
	// gdb's job). Plain BuildCommand cannot be reused: a native (non-triple)
	// build transpiles without linking, and linking is exactly what -g
	// debugging needs.
	sourceText, srcMap, err := resolveSourcesCheck(staged)
	if err != nil {
		return sourceLoadResult(err)
	}
	prog, errDiags := parseSourceWithErrors(staged, sourceText, srcMap, verbose)
	if prog == nil {
		return CommandResult{ExitCode: ExitCompile, Message: "Parse failed"}
	}
	if len(errDiags) > 0 {
		renderDiagnostics(sourceText, errDiags)
		return CommandResult{ExitCode: ExitCompile, Message: checkStageMessage(checkStageSyntax, len(errDiags))}
	}
	if errs := runBorrowCheck(prog); len(errs) > 0 {
		msg := "Borrow check failed:\n"
		for _, e := range errs {
			msg += "  " + e.Message + "\n"
		}
		return CommandResult{ExitCode: ExitCompile, Message: msg}
	}
	if errDiags := runSemanticPreflight(staged, sourceText, srcMap, prog); len(errDiags) > 0 {
		renderDiagnostics(sourceText, errDiags)
		return CommandResult{ExitCode: ExitCompile, Message: checkStageMessage(checkStageFor(errDiags), len(errDiags))}
	}
	cfg := codegen.NewConfig()
	cfg.Debug = true
	cfg.CompileOnly = false
	cfg.OutputPath = exe
	cfg.Verbose = verbose
	// One retry on gcc failure: Windows file locks (AV scanner) and loaded
	// toolchains flake single compiles transiently (the Phase 114 parity
	// harness retries for the same reason). A persistent failure still
	// surfaces deterministically below.
	var buildErr error
	for attempt := 0; attempt < 2; attempt++ {
		if buildErr = codegen.New(cfg).GenerateAndCompile(prog, staged); buildErr == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if buildErr != nil {
		return CommandResult{ExitCode: classifyCompileError(buildErr), Message: "debug build failed: " + buildErr.Error()}
	}

	ctx, cancel := context.WithTimeout(context.Background(), dbgTimeout)
	defer cancel()
	args := []string{"--batch",
		"-ex", "set pagination off",
		"-ex", "set confirm off",
		"-ex", `rbreak ^karkain_user_`,
		"-ex", "run",
	}
	for i := 0; i < dbgMaxSteps; i++ {
		args = append(args, "-ex", "bt", "-ex", "continue")
	}
	args = append(args, "--args", exe)
	cmd := exec.CommandContext(ctx, gdb, args...)
	cmd.Dir = sandbox
	rawOut, err := cmd.CombinedOutput()
	out := string(rawOut)
	if ctx.Err() == context.DeadlineExceeded {
		return CommandResult{ExitCode: ExitFailure,
			Message: "gdb walk timed out after 120s (inferior hung); no trace produced"}
	}
	// gdb batch exits nonzero only for gdb-level failures (missing exe,
	// unreadable symbols); an inferior that exits or signals still yields
	// its backtraces, which is the product. Distinguish by output: with no
	// parseable frame at all, the walk itself failed.
	best := deepestDbgWalk(out)
	if len(best) == 0 {
		msg := "gdb walk produced no backtrace frames"
		if err != nil {
			msg += fmt.Sprintf(" (gdb: %v)", err)
		}
		return CommandResult{ExitCode: ExitFailure, Message: msg + "\n" + out}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "karkain_dbg trace: %s\n", filepath.Base(targetFile))
	for i, f := range best {
		if f.At != "" {
			fmt.Fprintf(&sb, "#%d %s at %s\n", i, f.Func, f.At)
		} else {
			fmt.Fprintf(&sb, "#%d %s\n", i, f.Func)
		}
	}
	return CommandResult{ExitCode: ExitSuccess, Message: strings.TrimRight(sb.String(), "\n")}
}

// deepestDbgWalk splits gdb batch output into per-bt blocks (each `#0 ...`
// line starts a block) and returns the longest block — the deepest observed
// stack. For straight-line call programs that is the exact call chain.
func deepestDbgWalk(out string) []dbgFrame {
	var best []dbgFrame
	var cur []dbgFrame
	flush := func() {
		if len(cur) > len(best) {
			best = cur
		}
		cur = nil
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		m := dbgBtLine.FindStringSubmatch(line)
		if m == nil || m[2] == "" {
			continue
		}
		if m[1] == "0" {
			flush()
		}
		cur = append(cur, dbgFrame{Func: demangleDbgFunc(m[2]), At: m[3]})
	}
	flush()
	return best
}
