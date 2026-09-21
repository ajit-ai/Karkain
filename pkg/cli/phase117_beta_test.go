package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Phase 117 â€” Beta 1 Readiness & Hardening gate.
//
// This suite locks in the Beta contract: cross-engine parity of the stable
// language core (unparenthesized `while`, module-qualified stdlib calls),
// semantic build/run gating on BOTH engines (clean K00x/K1xx diagnostics +
// ExitCompile instead of raw compiler noise), numeric error-code
// documentation through `karkain explain`, the exit-code contract across
// commands and engines, and the Beta repository artifacts (fresh-checkout
// script, status/beta.rst, audit report).
//
// E2E invocations exercise the real binary and real dispatch; kcc-engine
// paths run with the working directory inside the repository so the
// self-hosted compiler is discoverable (kccRepoRoot).

// runKarkain runs the karkain binary with the optional env overrides in dir.
func runKarkain(t *testing.T, bin, dir string, env []string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			code = -1
		}
	}
	return string(out), code
}

// TestPhase117_WhileParensParity proves both engines accept the while loop in
// parenthesized AND unparenthesized form and produce identical output. This
// was a documented Go-vs-kcc divergence that Phase 117 removed.
func TestPhase117_WhileParensParity(t *testing.T) {
	bin := buildPreviewBinary(t)
	skipped := false
	work := filepath.Join(repoRoot(t), "phase117-while-scratch")
	if err := os.MkdirAll(work, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(work) })

	if _, err := exec.LookPath("gcc"); err != nil {
		skipped = true
	}

	programs := map[string]string{
		"parens.kark": `func main() {
    let n = 10
    let acc = 0
    while (acc < n) {
        acc = acc + 1
    }
    print(acc)
}
`,
		"bare.kark": `func main() {
    let n = 10
    let acc = 0
    while acc < n {
        acc = acc + 1
    }
    print(acc)
}
`,
	}
	want := "10"
	for name, src := range programs {
		file := filepath.Join(work, name)
		if err := os.WriteFile(file, []byte(src), 0644); err != nil {
			t.Fatal(err)
		}

		goOut, goCode := runKarkain(t, bin, work, []string{"KARKAIN_ENGINE=go"}, "run", file, "--engine", "go")
		if goCode != 0 {
			t.Errorf("%s go engine: exit %d, want 0\n%s", name, goCode, goOut)
		} else if !strings.Contains(goOut, want) {
			t.Errorf("%s go engine output does not contain %q:\n%s", name, want, goOut)
		}

		if skipped {
			continue
		}
		kccOut, kccCode := runKarkain(t, bin, work, []string{"KARKAIN_ENGINE=kcc"}, "run", file, "--engine", "kcc")
		if kccCode != 0 {
			t.Errorf("%s kcc engine: exit %d, want 0\n%s", name, kccCode, kccOut)
			continue
		}
		if got := strings.TrimSpace(stripKCCBuildBanner(t, kccOut)); got != strings.TrimSpace(goOut) {
			t.Errorf("%s kcc output != go output:\n kcc: %q\n  go: %q", name, got, strings.TrimSpace(goOut))
		}
	}
}

// TestPhase117_SemanticBuildRunGating proves `karkain build` and `karkain run`
// reject a semantically invalid program (undefined identifier) on BOTH engines
// with a clean error[K00x]/error[K1xx] diagnostic and ExitCompile(3) â€” the
// same contract as `karkain check` â€” instead of raw codegen/gcc noise with an
// unrelated exit code.
func TestPhase117_SemanticBuildRunGating(t *testing.T) {
	bin := buildPreviewBinary(t)
	work := t.TempDir()
	file := filepath.Join(work, "bad.kark")
	src := "func main() {\n    let t = undefined_symbol_zzz\n    print(t)\n}\n"
	if err := os.WriteFile(file, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	type tc struct {
		args     []string
		engine   string
		wantCode string
	}
	cases := []tc{
		{[]string{"build", file, "--engine", "go"}, "go", "error[K002]"},
		{[]string{"run", file, "--engine", "go"}, "go", "error[K002]"},
		{[]string{"check", file, "--engine", "go"}, "go", "error[K002]"},
	}

	skip := false
	if _, err := exec.LookPath("gcc"); err != nil {
		skip = true
	}
	if !skip {
		cases = append(cases,
			tc{[]string{"build", file, "--engine", "kcc"}, "kcc", "error[K102]"},
			tc{[]string{"run", file, "--engine", "kcc"}, "kcc", "error[K102]"},
			tc{[]string{"check", file, "--engine", "kcc"}, "kcc", "error[K102]"},
		)
	}

	for _, c := range cases {
		dir := work
		if c.engine == "kcc" {
			// The self-hosted engine must be discoverable from the working
			// directory (kccRepoRoot walks up from cwd to find src/compiler).
			dir = repoRoot(t)
		}
		out, code := runKarkain(t, bin, dir, []string{"KARKAIN_ENGINE=" + c.engine}, c.args...)
		if code != ExitCompile {
			t.Errorf("engine=%s args=%v: exit %d, want %d\n%s", c.engine, c.args, code, ExitCompile, out)
		}
		if !strings.Contains(out, c.wantCode) {
			t.Errorf("engine=%s args=%v: missing %q\n%s", c.engine, c.args, c.wantCode, out)
		}
	}
}

// TestPhase117_ExplainNumericCodes proves `karkain explain` documents the
// numeric error-code namespace of both engines (Go K001-K008/K100 and the
// self-hosted K101-K113) and that `--list` includes them.
func TestPhase117_ExplainNumericCodes(t *testing.T) {
	bin := buildPreviewBinary(t)
	work := t.TempDir()
	for _, code := range []string{"K001", "K002", "K003", "K100", "K101", "K102", "K103", "K106", "K107", "K108", "K109", "K112", "K113", "K114"} {
		out, err := runBin(t, bin, work, "explain", code)
		if err != nil {
			t.Errorf("explain %s failed: %v\n%s", code, err, out)
			continue
		}
		if !strings.Contains(out, code+"\n") && !strings.Contains(out, code+"  ") {
			t.Errorf("explain %s does not mention its code:\n%s", code, out)
		}
	}

	out, err := runBin(t, bin, work, "explain", "--list")
	if err != nil {
		t.Fatalf("explain --list failed: %v\n%s", err, out)
	}
	for _, want := range []string{"K001", "K002", "K100", "K101", "K102", "K112", "K113", "K114", "E-K-SYN", "E-PKG-LOCK"} {
		if !strings.Contains(out, want) {
			t.Errorf("explain --list missing %q", want)
		}
	}
}

// TestPhase117_ExitCodeContract verifies the documented exit-code table end to
// end: 0 clean run, 3 parse/semantic failure, 2 bad usage, 4 failing tests.
func TestPhase117_ExitCodeContract(t *testing.T) {
	bin := buildPreviewBinary(t)
	work := t.TempDir()

	good := filepath.Join(work, "good.kark")
	writeFile(t, good, "func main() { print(42) }\n")
	out, code := runKarkain(t, bin, work, nil, "run", good, "--engine", "go")
	if code != 0 || !strings.Contains(out, "42") {
		t.Errorf("clean run: exit %d want 0, contains 42: %v\n%s", code, strings.Contains(out, "42"), out)
	}

	parseErr := filepath.Join(work, "syntax.kark")
	writeFile(t, parseErr, "func main() { let = 42 }\n")
	_, code = runKarkain(t, bin, work, nil, "run", parseErr, "--engine", "go")
	if code != ExitCompile {
		t.Errorf("parse-error run: exit %d, want %d", code, ExitCompile)
	}

	_, code = runKarkain(t, bin, work, nil, "run", filepath.Join(work, "missing.kark"), "--engine", "go")
	if code != ExitUsage {
		t.Errorf("missing-file run: exit %d, want %d", code, ExitUsage)
	}

	failTest := filepath.Join(work, "fail_test.kark")
	writeFile(t, failTest, "func test_bad() { assert_eq(1, 2) }\n")
	_, code = runKarkain(t, bin, work, nil, "test", failTest, "--engine", "go")
	if code != ExitTest {
		t.Errorf("failing test: exit %d, want %d", code, ExitTest)
	}

	passTest := filepath.Join(work, "pass_test.kark")
	writeFile(t, passTest, "func test_good() { assert_eq(2, 2) }\n")
	_, code = runKarkain(t, bin, work, nil, "test", passTest, "--engine", "go")
	if code != 0 {
		t.Errorf("passing test: exit %d, want 0", code)
	}
}

// TestPhase117_StdlibBothEngines verifies an `import std.*` program produces
// identical output on the Go engine and the self-hosted engine.
func TestPhase117_StdlibBothEngines(t *testing.T) {
	bin := buildPreviewBinary(t)
	work := filepath.Join(repoRoot(t), "phase117-stdlib-scratch")
	if err := os.MkdirAll(work, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(work) })

	src := `import std.string
import std.encoding

func main() {
    print(str_to_upper(str_trim("  beta 1  ")))
    print(str_starts_with("karkain", "kar"))
    print(str_len("abcd"))
    print(hex_encode("ab"))
    print(base64_decode(base64_encode("hello")))
}
`
	file := filepath.Join(work, "stdlib.kark")
	if err := os.WriteFile(file, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	goOut, goCode := runKarkain(t, bin, work, []string{"KARKAIN_ENGINE=go"}, "run", file, "--engine", "go")
	if goCode != 0 {
		t.Fatalf("stdlib go engine: exit %d\n%s", goCode, goOut)
	}

	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("gcc not available; skipping self-hosted engine check")
	}
	kccOut, kccCode := runKarkain(t, bin, work, []string{"KARKAIN_ENGINE=kcc"}, "run", file, "--engine", "kcc")
	if kccCode != 0 {
		t.Fatalf("stdlib kcc engine: exit %d\n%s", kccCode, kccOut)
	}
	if got := strings.TrimSpace(stripKCCBuildBanner(t, kccOut)); got != strings.TrimSpace(goOut) {
		t.Errorf("stdlib kcc output != go output:\n kcc: %q\n  go: %q", got, strings.TrimSpace(goOut))
	}
}

// TestPhase117_TestRunnerKCCParity proves the test runner exit-code contract on
// the default self-hosted engine: failing tests exit 4 (ExitTest), passing
// tests exit 0.
func TestPhase117_TestRunnerKCCParity(t *testing.T) {
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("gcc not available; skipping kcc test-runner check")
	}
	bin := buildPreviewBinary(t)
	work := filepath.Join(repoRoot(t), "phase117-test-scratch")
	if err := os.MkdirAll(work, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(work) })

	failFile := filepath.Join(work, "fail_test.kark")
	writeFile(t, failFile, "func test_fail() { assert_eq(1, 99) }\n")
	out, code := runKarkain(t, bin, work, []string{"KARKAIN_ENGINE=kcc"}, "test", failFile, "--engine", "kcc")
	if code != ExitTest {
		t.Errorf("kcc failing test: exit %d, want %d\n%s", code, ExitTest, out)
	}
	if !strings.Contains(out, "1 failed") {
		t.Errorf("kcc failing test summary missing '1 failed':\n%s", out)
	}

	passFile := filepath.Join(work, "pass_test.kark")
	writeFile(t, passFile, "func test_pass() { assert_eq(1, 1) }\n")
	out, code = runKarkain(t, bin, work, []string{"KARKAIN_ENGINE=kcc"}, "test", passFile, "--engine", "kcc")
	if code != 0 {
		t.Errorf("kcc passing test: exit %d, want 0\n%s", code, out)
	}
	if !strings.Contains(out, "1 passed") {
		t.Errorf("kcc passing test summary missing '1 passed':\n%s", out)
	}
}

// TestPhase117_TargetMatrix verifies `karkain target` advertises the host
// triple and the supported target matrix.
func TestPhase117_TargetMatrix(t *testing.T) {
	bin := buildPreviewBinary(t)
	work := t.TempDir()
	out, err := runBin(t, bin, work, "target")
	if err != nil {
		t.Fatalf("karkain target failed: %v\n%s", err, out)
	}
	for _, want := range []string{"host", "native", "c23", "wasm32-wasi", "native-link"} {
		if !strings.Contains(out, want) {
			t.Errorf("karkain target missing %q:\n%s", want, out)
		}
	}
}

// TestPhase117_BetaArtifacts verifies the Beta-1 repository deliverables that
// CI depends on: the fresh-checkout script, the status page and this phase's
// audit report.
func TestPhase117_BetaArtifacts(t *testing.T) {
	root := repoRoot(t)
	for _, p := range []string{
		"scripts/beta-fresh-checkout.ps1",
		"docs/source/status/beta.rst",
		"docs/audit/PHASE-117-BETA-1-READINESS.md",
		"pkg/cli/phase117_stdlib_edge_test.go",
	} {
		data, err := os.ReadFile(filepath.Join(root, p))
		if err != nil {
			t.Errorf("missing Beta-1 artifact %s: %v", p, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("Beta-1 artifact %s is empty", p)
		}
	}
	status, err := os.ReadFile(filepath.Join(root, "docs", "source", "status", "beta.rst"))
	if err == nil && !strings.Contains(string(status), "Beta 1") {
		t.Errorf("status/beta.rst does not mention Beta 1")
	}
}

// TestPhase117_DebugAndProfile verifies the two opt-in diagnostics commands:
// `karkain debug` emits deterministic `karkain:<file>:enter <func>` trace
// lines on stderr while program stdout passes through untouched, and a plain
// `karkain run` is NOT instrumented; `karkain prof` produces the deterministic
// text report (function table + call graph) with program output passing through.
func TestPhase117_DebugAndProfile(t *testing.T) {
	bin := buildPreviewBinary(t)
	work := t.TempDir()

	prog := filepath.Join(work, "diag.kark")
	writeFile(t, prog, "func fib(n int) int {\n    if n < 2 { return n }\n    return fib(n-1) + fib(n-2)\n}\nfunc main() { print(fib(10)) }\n")

	runOut, runCode := runKarkain(t, bin, work, nil, "run", prog, "--engine", "go")
	if runCode != 0 {
		t.Fatalf("baseline run: exit %d\n%s", runCode, runOut)
	}
	if !strings.Contains(runOut, "55") {
		t.Errorf("baseline run missing program output 55:\n%s", runOut)
	}
	if strings.Contains(runOut, "karkain:diag.kark:enter") {
		t.Errorf("plain run must not be instrumented:\n%s", runOut)
	}

	debugOut, debugCode := runKarkain(t, bin, work, nil, "debug", prog)
	if debugCode != 0 {
		t.Fatalf("debug: exit %d\n%s", debugCode, debugOut)
	}
	if !strings.Contains(debugOut, "55") {
		t.Errorf("debug run lost program stdout:\n%s", debugOut)
	}
	if !strings.Contains(debugOut, "karkain:diag.kark:enter fib") || !strings.Contains(debugOut, "karkain:diag.kark:leave main") {
		t.Errorf("debug missing deterministic trace lines:\n%s", debugOut)
	}

	profOut, profCode := runKarkain(t, bin, work, nil, "prof", prog)
	if profCode != 0 {
		t.Fatalf("prof: exit %d\n%s", profCode, profOut)
	}
	for _, want := range []string{"Functions (by total time):", "Call graph:", "Allocation:", "55"} {
		if !strings.Contains(profOut, want) {
			t.Errorf("prof output missing %q:\n%s", want, profOut)
		}
	}
}
