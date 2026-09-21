package cli

// Phase 133 — First-class fn values: capture-mutation completion is not
// enough; closures must also FLOW (params, arrays, returns) and be INVOKED
// through values on BOTH engines.
//
// Karkain-owned design (no imported closure semantics):
//   - `IndirectCallExpr` for computed callees (`ops[0](5)`); identifier
//     callees keep static dispatch paths untouched.
//   - `TYPE_FUNC` Value cells `{canonical wrapper, heap env}`; per-binding
//     heap envs (never a shared static) + cell dispatch for direct calls
//     (except self-reference) so rebinding/recursion stay correct.
//   - Captures stay by-ref; returning a closure that captures locals is
//     rejected at check time (Go error[K002] / kcc error[K114], exit 3)
//     because the value would dangle. Non-capturing closures return fine.
//   - Calling a non-function value or wrong arity is a file:line runtime
//     error (exit 1) on both engines — never a C-compile failure.
//   - WASM rejects indirect calls with error[K108] (documented boundary);
//     `func(T) R` full signatures stay Planned (bare `fn` suffices).

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// phase133Cases returns the Phase 133 corpus goldens, measured live on both
// engines (byte-identical stdout).
func phase133Cases(t *testing.T) []phase102Case {
	t.Helper()
	dir := filepath.Join(repoRoot(t), "examples", "closures")
	return []phase102Case{
		// Higher-order params + fn-typed aliases dispatch through cells.
		{filepath.Join(dir, "02_higher_order.kark"), "11\n22\n6\n"},
		// Array-stored closures invoked through index expressions; a nested
		// closure invoked within its defining frame.
		{filepath.Join(dir, "03_array_call.kark"), "15\n10\n20\n"},
		// Pure factory returns (null env) + loop-built closures each keep
		// their own heap env. Loop-body lets share one C slot across
		// iterations, so post-loop reads observe the last value (21/21/21)
		// identically on both engines — documented slot-reuse semantics.
		{filepath.Join(dir, "04_factory.kark"), "101\n102\n21\n21\n21\n"},
		// Recursion through the cell: sibling calls after a recursive call
		// returns read the current frame's env (static-holder dispatch
		// would read the innermost one and print 0).
		{filepath.Join(dir, "05_recursion.kark"), "300\n"},
	}
}

// TestPhase133_GoldensGoEngine runs the corpus through the real binary on
// the Go engine and asserts byte-exact golden stdout.
func TestPhase133_GoldensGoEngine(t *testing.T) {
	karkain := phase130Karkain(t)
	runPhase102Cases(t, karkain, "go", phase133Cases(t))
}

// TestPhase133_GoldensKCC runs the same corpus through the kcc engine for
// byte-identical parity. On low-RAM hosts the kcc self-build is cleanly
// aborted by the Phase 127 guard (error[K127]); that subtest skips.
func TestPhase133_GoldensKCC(t *testing.T) {
	karkain := phase130Karkain(t)
	for _, c := range phase133Cases(t) {
		cmd := exec.Command(karkain, "run", c.file, "--engine", "kcc")
		cmd.Env = append(os.Environ(), "KARKAIN_ENGINE=kcc")
		out, err := cmd.CombinedOutput()
		if err != nil {
			msg := string(out)
			if strings.Contains(msg, "error[K127]") {
				t.Skipf("low-RAM host: kcc self-build guarded (error[K127]) — parity exercised on CI: %s", filepath.Base(c.file))
			}
			t.Fatalf("%s (engine=kcc): exit err %v\n%s", filepath.Base(c.file), err, msg)
		}
		got := stripKCCBuildBanner(t, string(out))
		got = strings.ReplaceAll(got, "\r\n", "\n")
		if got != c.want {
			t.Errorf("%s (engine=kcc): mismatch\nwant:\n%q\ngot:\n%q", filepath.Base(c.file), c.want, got)
		}
	}
}

// TestPhase133_CheckClean proves both checkers accept the corpus (exit 0):
// syntax, resolution, and escape rules agree before any code runs.
func TestPhase133_CheckClean(t *testing.T) {
	karkain := phase130Karkain(t)
	for _, c := range phase133Cases(t) {
		for _, engine := range []string{"go", "kcc"} {
			cmd := exec.Command(karkain, "check", c.file, "--engine", engine)
			cmd.Env = append(os.Environ(), "KARKAIN_ENGINE="+engine)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("%s: check --engine %s failed:\n%s", filepath.Base(c.file), engine, string(out))
			}
		}
	}
}

// TestPhase133_EscapeRejection pins the honest boundary: returning a closure
// that captures function-local state is rejected at check time on BOTH
// engines (Go error[K002], kcc error[K114], exit 3) instead of running to a
// silent wrong result. Both the direct-lambda and alias shapes are covered.
func TestPhase133_EscapeRejection(t *testing.T) {
	karkain := phase130Karkain(t)
	cases := map[string]string{
		"direct": "func mkadder(n int) fn {\n\treturn fn(x int) int { return x + n }\n}\nfunc main() {\n\tprint(1)\n}\n",
		"alias":  "func mk(n int) fn {\n\tlet f = fn(x int) int { return x + n }\n\treturn f\n}\nfunc main() {\n\tprint(1)\n}\n",
	}
	wantCode := map[string]string{"go": "error[K002]", "kcc": "error[K114]"}
	for name, src := range cases {
		probe := filepath.Join(t.TempDir(), name+".kark")
		if err := os.WriteFile(probe, []byte(src), 0644); err != nil {
			t.Fatalf("writing probe: %v", err)
		}
		for _, engine := range []string{"go", "kcc"} {
			cmd := exec.Command(karkain, "check", probe, "--engine", engine)
			cmd.Env = append(os.Environ(), "KARKAIN_ENGINE="+engine)
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Errorf("%s engine=%s: escaping factory should be rejected, but check passed", name, engine)
				continue
			}
			if !strings.Contains(string(out), wantCode[engine]) {
				t.Errorf("%s engine=%s: want %s, got:\n%s", name, engine, wantCode[engine], string(out))
			}
		}
	}
}

// TestPhase133_PureFactoryRuns proves the sound side of the boundary: a
// factory returning a NON-capturing closure runs identically on both
// engines (null env, nothing to dangle).
func TestPhase133_PureFactoryRuns(t *testing.T) {
	karkain := phase130Karkain(t)
	probe := filepath.Join(t.TempDir(), "pure.kark")
	src := "func mk() fn {\n\treturn fn(x int) int { return x + 1 }\n}\nfunc main() {\n\tlet inc = mk()\n\tprint(inc(10))\n\tprint(inc(20))\n}\n"
	if err := os.WriteFile(probe, []byte(src), 0644); err != nil {
		t.Fatalf("writing probe: %v", err)
	}
	for _, engine := range []string{"go", "kcc"} {
		cmd := exec.Command(karkain, "run", probe, "--engine", engine)
		cmd.Env = append(os.Environ(), "KARKAIN_ENGINE="+engine)
		out, err := cmd.CombinedOutput()
		if err != nil {
			msg := string(out)
			if engine == "kcc" && strings.Contains(msg, "error[K127]") {
				t.Skipf("low-RAM host: kcc self-build guarded (error[K127])")
			}
			t.Fatalf("engine=%s: pure factory should run:\n%s", engine, msg)
		}
		got := stripKCCBuildBanner(t, string(out))
		if strings.TrimSpace(strings.ReplaceAll(got, "\r\n", "\n")) != "11\n21" {
			t.Errorf("engine=%s: golden mismatch: want 11/21, got %q", engine, got)
		}
	}
}

// TestPhase133_WasmBoundary pins the documented WASM boundary in Karkain
// words: indirect calls are rejected with error[K108] naming the language
// feature, never a Go AST type name.
func TestPhase133_WasmBoundary(t *testing.T) {
	karkain := phase130Karkain(t)
	probe := filepath.Join(t.TempDir(), "indirect.kark")
	src := "func main() {\n\tlet ops = [1]\n\tprint(ops[0](5))\n}\n"
	if err := os.WriteFile(probe, []byte(src), 0644); err != nil {
		t.Fatalf("writing probe: %v", err)
	}
	build := exec.Command(karkain, "build", probe, "--target", "wasm32-wasi")
	out, err := build.CombinedOutput()
	if err == nil {
		t.Fatalf("wasm build of indirect call should be rejected, but succeeded:\n%s", string(out))
	}
	msg := string(out)
	if !strings.Contains(msg, "K108") {
		t.Fatalf("want error[K108], got:\n%s", msg)
	}
	if !strings.Contains(msg, "first-class function values (indirect calls)") {
		t.Fatalf("diagnostic must name the Karkain feature, got:\n%s", msg)
	}
	if strings.Contains(msg, "parser.IndirectCallExpr") {
		t.Fatalf("diagnostic leaks a Go type name, got:\n%s", msg)
	}
}

// TestPhase133_RuntimeErrors pins the runtime contract on both engines:
// calling a non-function value and arity mismatch are file:line runtime
// errors (exit 1), never C-compile failures.
func TestPhase133_RuntimeErrors(t *testing.T) {
	karkain := phase130Karkain(t)
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"nonfn", "func main() {\n\tlet x = 5\n\tprint(x(1))\n}\n", "called non-function value"},
		{"arity", "func main() {\n\tlet f = fn(a int, b int) int { return a + b }\n\tprint(f(1))\n}\n", "wrong number of arguments"},
	}
	for _, c := range cases {
		probe := filepath.Join(t.TempDir(), c.name+".kark")
		if err := os.WriteFile(probe, []byte(c.src), 0644); err != nil {
			t.Fatalf("writing probe: %v", err)
		}
		for _, engine := range []string{"go", "kcc"} {
			run := exec.Command(karkain, "run", probe, "--engine", engine)
			run.Env = append(os.Environ(), "KARKAIN_ENGINE="+engine)
			out, err := run.CombinedOutput()
			if err == nil {
				t.Errorf("%s engine=%s: should fail at runtime, but ran:\n%s", c.name, engine, string(out))
				continue
			}
			msg := string(out)
			if engine == "kcc" && strings.Contains(msg, "error[K127]") {
				t.Skipf("low-RAM host: kcc self-build guarded (error[K127])")
			}
			if !strings.Contains(msg, c.want) {
				t.Errorf("%s engine=%s: want %q in output, got:\n%s", c.name, engine, c.want, msg)
			}
			if !strings.Contains(msg, "runtime error:") {
				t.Errorf("%s engine=%s: want Phase-100 runtime-error diagnostics, got:\n%s", c.name, engine, msg)
			}
		}
	}
}
