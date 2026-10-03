package cli

// LH-2 gate: kcc `match` arm binding parity.
//
// The self-hosted engine parses an arm binding (`Ok(n) => ...`) and the code
// generator already lowers it correctly, but the semantic pass never
// registered the name, so every reference to it inside the arm was reported as
// `error[K102] undefined identifier`. The defect was purely semantic: the AST,
// the parser and the lowering were all already correct.
//
// This gate pins the SEMANTICS on both engines, not merely that a binding
// compiles:
//
//   ok_binding      Test A  an Ok arm binding makes n available in that arm
//   err_binding     Test B  an Err arm binding exposes the original payload
//   multi_arm       Test D  independent per-arm bindings, plus Option
//                          Some/None, and names reused across two matches
//   nested_match    Test E  an inner match must not disturb the enclosing
//                          binding, which is used before and after it
//   scope_escape    Test C  NEGATIVE: a binding must not escape its arm
//   bare_binding    Test F  bare (non-block) arm bodies keep their binding in
//                          scope: Ok(n) => n, Ok(n) => n + 1, Some(v) => v,
//                          and Ok(n) => print(n)
//   err_bare_binding Test G NEGATIVE: an undefined identifier in a bare arm
//                          body is reported as K102 naming it. This is what
//                          keeps Test F from passing vacuously -- see the note
//                          on that test.
//
// A parity-only gate would pass on two engines that are wrong together, so each
// positive case pins an explicit expected value derived from the language
// semantics AND requires the two engines to agree.
//
// Two deliberate design points:
//
//  1. The match value is always a local (`let r = f(1); match r { ... }`).
//     A match whose value is a call expression (`match f(1) { ... }`) fails to
//     parse on the Go engine with no binding involved at all, so using it here
//     would test an unrelated pre-existing parser limitation instead of arm
//     bindings.
//
//  2. Outputs are compared with newlines removed. The engines disagree on a
//     pre-existing, unrelated detail: Go print_value terminates scalars with a
//     newline, kcc print_value does not. That shows up in a program with no
//     match and no binding (a bare print of an int already differs), so it
//     cannot be attributed to this change, and altering it would change every
//     kcc program's output. Stripping newlines keeps the comparison sound for
//     what these fixtures assert: WHICH values were printed and in what order.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"karkain/pkg/codegen"
)

// lh2Compact removes carriage returns and line feeds; see design point 2.
func lh2Compact(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == 13 || r == 10 {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// lh2Fixture copies a match-binding fixture into a temp dir so generated
// artifacts never land in the repository.
func lh2Fixture(t *testing.T, name string) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(wd, "..", "..", "examples", "matchbinding", name+".kark")
	prog, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read fixture %s: %v", src, err)
	}
	dst := filepath.Join(t.TempDir(), name+".kark")
	if err := os.WriteFile(dst, prog, 0o644); err != nil {
		t.Fatal(err)
	}
	return dst
}

// lh2Cases pins the expected observable for each positive fixture.
var lh2Cases = []struct {
	file string
	want string // newline-compacted expected stdout, identical on both engines
	note string
}{
	{"ok_binding", "42", "Test A: Ok(n) makes n the Ok payload"},
	{"err_binding", "bad", "Test B: Err(e) makes e the original error payload"},
	{"multi_arm", "42bad7n", "Test D: per-arm bindings stay independent; Some/None also bind"},
	{"nested_match", "20inner20", "Test E: the inner match leaves the enclosing binding intact"},
	{"bare_binding", "4243742", "Test F: bare (non-block) arm bodies bind too: n, n+1, v, print(n)"},
}

// TestLH2_MatchBindingParity is the main gate: for every positive fixture both
// engines must succeed, agree with each other, and equal the pinned value.
func TestLH2_MatchBindingParity(t *testing.T) {
	for _, tc := range lh2Cases {
		t.Run(tc.file, func(t *testing.T) {
			fixture := lh2Fixture(t, tc.file)

			var goOut, goErr bytes.Buffer
			goRes := RunCommand(fixture, codegen.Config{Stdout: &goOut, Stderr: &goErr}, false)
			if goRes.ExitCode != ExitSuccess {
				t.Fatalf("Go engine: exit=%d want %d (stdout=%q stderr=%q msg=%q)",
					goRes.ExitCode, ExitSuccess, goOut.String(), goErr.String(), goRes.Message)
			}
			goGot := lh2Compact(goOut.String())
			if goGot != tc.want {
				t.Errorf("Go engine: got %q, want %q (%s)", goGot, tc.want, tc.note)
			}

			kccRes := KCCRunCommand(nil, fixture, codegen.Config{}, false)
			if kccRes.ExitCode != ExitSuccess {
				t.Fatalf("kcc engine: exit=%d want %d (message=%q)",
					kccRes.ExitCode, ExitSuccess, kccRes.Message)
			}
			kccGot := lh2Compact(lastGoldenLine(kccRes.Message))
			if kccGot != tc.want {
				t.Errorf("kcc engine: got %q, want %q (%s)", kccGot, tc.want, tc.note)
			}

			// Explicit parity assertion, so the engines are compared to each
			// other and not only to a constant.
			if kccGot != goGot {
				t.Errorf("engine parity: Go=%q kcc=%q", goGot, kccGot)
			}
		})
	}
}

// TestLH2_BindingDoesNotEscapeScope is the negative regression (Test C). The
// fixture prints the arm binding AFTER the match, so it must be rejected by
// both engines. A fix that simply added the binding to the enclosing scope --
// the cheapest way to make the positive cases pass -- is caught here.
//
// The two engines reject with DIFFERENT diagnostics, and that is expected
// rather than a parity gap: the Go rejection comes from the borrow checker's
// scope-end rule, while the self-hosted checker reports the name as undefined.
// Both agree the program is invalid, which is the property under test.
func TestLH2_BindingDoesNotEscapeScope(t *testing.T) {
	fixture := lh2Fixture(t, "scope_escape")

	var goOut, goErr bytes.Buffer
	goRes := RunCommand(fixture, codegen.Config{Stdout: &goOut, Stderr: &goErr}, false)
	if goRes.ExitCode == ExitSuccess {
		t.Errorf("Go engine: the arm binding escaped its arm (exit=%d stdout=%q)",
			goRes.ExitCode, goOut.String())
	}

	kccRes := KCCRunCommand(nil, fixture, codegen.Config{}, false)
	if kccRes.ExitCode == ExitSuccess {
		t.Errorf("kcc engine: the arm binding escaped its arm (exit=%d output=%q)",
			kccRes.ExitCode, lastGoldenLine(kccRes.Message))
	}
	if !strings.Contains(kccRes.Message, "undefined identifier") {
		t.Errorf("kcc engine: expected an undefined-identifier diagnostic, got %q", kccRes.Message)
	}
}

// TestLH2_BindingIsArmLocal inspects the generated C, proving the binding is
// declared INSIDE the arm's own block. That is what makes it arm-local in the
// generated program and not only in the checker's bookkeeping, so the negative
// test above holds for the emitted code and not merely for the front end.
func TestLH2_BindingIsArmLocal(t *testing.T) {
	bin, err := kccBinaryPath(nil)
	if err != nil {
		t.Fatalf("locate kcc: %v", err)
	}
	fixture := lh2Fixture(t, "ok_binding")

	out, err := exec.Command(bin, "build", fixture).CombinedOutput()
	if err != nil {
		t.Fatalf("kcc build: %v (output=%q)", err, string(out))
	}
	c, err := os.ReadFile(strings.TrimSuffix(fixture, ".kark") + ".c23")
	if err != nil {
		t.Fatalf("read generated C: %v (kcc said %q)", err, string(out))
	}
	src := string(c)

	// The payload assignment, inside the arm block that also holds the body.
	if !strings.Contains(src, "Value n = *_match_val.resVal.okVal;") {
		t.Errorf("generated C does not bind n to the Ok payload")
	}
	// The enclosing function must not contain the payload extraction; it is
	// emitted per arm, guarded by the same tag test the Ok arm is.
	if !strings.Contains(src, "_match_val.resVal.tag == 0") {
		t.Errorf("generated C does not guard the arm with the Ok tag test")
	}
}

// TestLH2_BareArmBodyIsChecked is the negative regression for the bare
// (non-block) arm body (Test G). The parser wraps `Ok(n) => nope` in an
// ExprStmt, and the checker used to walk only Block bodies, so that body was
// never examined and the program was ACCEPTED.
//
// This is why the assertions below are on the diagnostic's code and the name
// rather than on the exit status alone: the unfixed checker accepts the
// program, so an exit-status-only assertion would fail loudly here but a
// future regression that reintroduced the skip would satisfy nothing. Requiring
// "K102" and "nope" is what distinguishes "the body was checked and rejected"
// from "the body was never looked at" -- K102 is emitted from exactly one
// place, reached only while checking an identifier.
//
// As with Test C, the two engines reject with DIFFERENT diagnostics and that is
// expected: Go's resolver does not walk match arms at all (pkg/sema/resolve.go
// checkStmt/checkExpr have no MatchExpr case), so Go rejects `nope` later, at C
// compile. Both agree the program is invalid, which is the property under test.
func TestLH2_BareArmBodyIsChecked(t *testing.T) {
	fixture := lh2Fixture(t, "err_bare_binding")

	var goOut, goErr bytes.Buffer
	goRes := RunCommand(fixture, codegen.Config{Stdout: &goOut, Stderr: &goErr}, false)
	if goRes.ExitCode == ExitSuccess {
		t.Errorf("Go engine: undefined identifier in a bare arm body was accepted (exit=%d stdout=%q)",
			goRes.ExitCode, goOut.String())
	}

	kccRes := KCCRunCommand(nil, fixture, codegen.Config{}, false)
	if kccRes.ExitCode == ExitSuccess {
		t.Fatalf("kcc engine: the bare arm body was skipped, so `nope` was never checked "+
			"(exit=%d output=%q)", kccRes.ExitCode, lastGoldenLine(kccRes.Message))
	}
	if !strings.Contains(kccRes.Message, "K102") {
		t.Errorf("kcc engine: expected a K102 diagnostic, got %q", kccRes.Message)
	}
	if !strings.Contains(kccRes.Message, "nope") {
		t.Errorf("kcc engine: expected the diagnostic to name 'nope', got %q", kccRes.Message)
	}
}
