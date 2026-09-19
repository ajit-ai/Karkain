package cli

// Phase 130 â€” Closures / fn values: capture-mutation completion + corpus.
//
// The Go engine's let-bound closure machinery (Phase 54/121) already handles
// zero-capture closures, free-variable captures (by-ref env alias),
// nested closures (pointer copy of the outer env), multiple sibling closures
// and capture MUTATION: a closure body may assign to a captured `var`, writing
// THROUGH the env alias, and the change is immediately visible back in the
// enclosing binding after the call returns.
//
// This gate pins the Phase 130 closure corpus (examples/closures/) with exact
// golden stdout on the Go engine, and drives the kcc engine for byte-identical
// parity when the host can build the self-hosted compiler (CI / larger hosts).
// On low-RAM hosts the kcc self-build is cleanly aborted by the Phase 127 guard
// (error[K127]); that subtest skips rather than flaking.
//
// First-class function values (closures stored in arrays, higher-order
// function-typed params and calls such as `ops[0](5)`) are NOT yet supported:
// the parser rejects the call shape at `ops[0](` with a K001 parse error. That
// boundary is documented here and in docs/audit/PHASE-130-FINAL-REPORT.md; it
// is intentionally not silently codegen'd.
//
// A second boundary (root-caused in Phase 130): a closure VARIABLE
// (`let f = fn...`) desugars to a generated function with no runtime Value
// cell, so another closure cannot FREE-CAPTURE it (`let g = fn... { ... f(...) ...
// }`). `karkain check` accepts the program on both engines (the identifier
// resolves), but `karkain run` fails at C compile because the env init takes
// `&f` of a name that has no C declaration â€” identically on both engines
// (parity preserved). Free-variable capture works for plain variables only.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// phase130Once builds the karkain CLI exactly once for all Phase 130 subtests.
// On the ~4GB test host the Go toolchain itself OOMs when several
// go-build/gcc pipelines run under pressure, so a single shared binary keeps
// the gate host-friendly (the documented Phase 127 environmental class).
var phase130Once sync.Once
var phase130Bin string
var phase130Err error

func phase130Karkain(t *testing.T) string {
	t.Helper()
	phase130Once.Do(func() {
		phase130Err = buildKarkainErr(t, &phase130Bin)
	})
	if phase130Err != nil {
		t.Fatalf("building karkain: %v", phase130Err)
	}
	return phase130Bin
}

func buildKarkainErr(t *testing.T, binOut *string) error {
	t.Helper()
	dir, err := os.MkdirTemp("", "phase130-karkain")
	if err != nil {
		return err
	}
	bin := filepath.Join(dir, "karkain")
	if isWindows() {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, "karkain/cmd/karkain")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}
	_ = out
	*binOut = bin
	return nil
}

// phase130ClosureCases returns the Phase 130 closure corpus goldens.
func phase130ClosureCases(t *testing.T) []phase102Case {
	t.Helper()
	dir := filepath.Join(repoRoot(t), "examples", "closures")
	return []phase102Case{
		// var captured by-ref: bump mutates counter through the env alias;
		// enclosing counter reflects every increment immediately.
		{filepath.Join(dir, "00_capture_mutation.kark"), "1\n2\n2\n3\n3\n"},
		// nested closures: inner captures through the outer env (base) plus an
		// outer local (m); outer(a) = (base+m+a)+(base+m+1) = 27+a, so
		// outer(5) = 32 and outer(1) = 28.
		{filepath.Join(dir, "01_nested.kark"), "32\n28\n"},
	}
}

// TestPhase130_ClosureGoldensGoEngine runs the closure corpus through the real
// binary on the Go engine and asserts byte-exact stdout (CRLF-normalized).
func TestPhase130_ClosureGoldensGoEngine(t *testing.T) {
	karkain := phase130Karkain(t)
	runPhase102Cases(t, karkain, "go", phase130ClosureCases(t))
}

// TestPhase130_ClosureGoldensKCC runs the closure corpus through the kcc
// engine for byte-identical parity. On hosts where the self-hosted compiler
// cannot be built (the documented Phase 127 low-RAM class, error[K127]) the
// subtest skips with a note; on CI / larger hosts it exercises the real gate.
func TestPhase130_ClosureGoldensKCC(t *testing.T) {
	karkain := phase130Karkain(t)
	for _, c := range phase130ClosureCases(t) {
		cmd := exec.Command(karkain, "run", c.file, "--engine", "kcc")
		cmd.Env = append(os.Environ(), "KARKAIN_ENGINE=kcc")
		out, err := cmd.CombinedOutput()
		if err != nil {
			msg := string(out)
			if strings.Contains(msg, "error[K127]") {
				t.Skipf("low-RAM host: kcc self-build guarded (error[K127]) â€” parity exercised on CI: %s", filepath.Base(c.file))
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

// TestPhase130_FirstClassBoundary pins the deliberately-unsupported first-class
// function-value shape: calling a closure stored in a container. The parser
// rejects `ops[0](5)` at the `(` with a K001 parse error (exit 3) â€” it is NOT
// codegen'd into broken bytes. This documents the Phase 130 boundary.
func TestPhase130_FirstClassBoundary(t *testing.T) {
	karkain := phase130Karkain(t)
	probe := filepath.Join(t.TempDir(), "probe.kark")
	src := "func main() {\n\tlet base = 10\n\tlet ops = [fn(x int) int { return x + base }, fn(x int) int { return x * 2 }]\n\tprintln(ops[0](5))\n}\n"
	if err := os.WriteFile(probe, []byte(src), 0644); err != nil {
		t.Fatalf("writing probe: %v", err)
	}
	cmd := exec.Command(karkain, "run", probe, "--engine", "go")
	cmd.Env = append(os.Environ(), "KARKAIN_ENGINE=go")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("first-class call should be rejected, but program ran:\n%s", string(out))
	}
	msg := string(out)
	if !strings.Contains(msg, "unexpected token ')'") {
		t.Fatalf("expected the first-class boundary parse error, got:\n%s", msg)
	}
}

// TestPhase130_ClosureVarCaptureBoundary pins the closure-variable capture
// boundary: `check` accepts the program on the Go engine (identifier resolves),
// but `run` fails at C compilation because the env init takes the address of a
// closure binding that has no C variable (both engines emit the identical
// desugaring â€” Go `main.c` and the kcc `main.c23` both contain
// `_e.karkain_cap_apply = &apply;` with no `apply` declaration â€” so this is a
// documented parity boundary, not an engine drift).
func TestPhase130_ClosureVarCaptureBoundary(t *testing.T) {
	karkain := phase130Karkain(t)
	probe := filepath.Join(t.TempDir(), "probe.kark")
	src := "func main() {\n\tlet base = 10\n\tlet apply = fn(x int) int { return x + base }\n\tlet twice = fn(n int) int { return apply(apply(n)) }\n\tprintln(twice(1))\n}\n"
	if err := os.WriteFile(probe, []byte(src), 0644); err != nil {
		t.Fatalf("writing probe: %v", err)
	}
	cmd := exec.Command(karkain, "check", probe, "--engine", "go")
	cmd.Env = append(os.Environ(), "KARKAIN_ENGINE=go")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("check should accept the program (boundary is at codegen), but failed:\n%s", string(out))
	}
	run := exec.Command(karkain, "run", probe, "--engine", "go")
	run.Env = append(os.Environ(), "KARKAIN_ENGINE=go")
	if out, err := run.CombinedOutput(); err == nil {
		t.Fatalf("run should fail at C compile (capture of a closure variable), but succeeded:\n%s", string(out))
	} else if !strings.Contains(string(out), "C compilation failed") {
		t.Fatalf("expected the closure-var capture C compile failure, got:\n%s", string(out))
	}
}
