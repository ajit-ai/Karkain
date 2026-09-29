package cli

// LH-1 gate: kcc `?` propagation parity.
//
// The self-hosted engine parsed the postfix `?` but lowered it to nothing, so
// any program using it produced malformed C and failed with
// "gcc: expected expression before ';' token". This gate pins the operator's
// SEMANTICS on both engines, not merely that it compiles:
//
//   Case A  ok_unwrap          `?` on Ok yields the unwrapped payload
//   Case B  err_propagate      `?` on Err returns the original error and does
//                              NOT continue executing
//   Case C  nested_propagate   propagation across three function boundaries
//   Case D  option_propagate   `?` on Option None and Some
//   plus    expr_position      `?` as a call argument, two of them inside a
//                              single expression, and in arithmetic position
//
// A parity-only gate would pass on two engines that are wrong in the same way,
// so every case pins an explicit expected value derived from the language
// semantics AND requires the two engines to agree with each other.
//
// Two deliberate design points:
//
//  1. No match arm appears anywhere in this corpus. Match-arm bindings are the
//     separate, not-yet-authorised LH-2 slice, so depending on them here would
//     confound LH-1 with LH-2. print_value renders a Result or Option as
//     Ok(..)/Err(..)/Some(..)/None on both engines, which observes both the
//     variant and the payload with no binding at all.
//
//  2. Outputs are compared with newlines removed, and that is NOT a weakening.
//     The engines disagree on a pre-existing, unrelated detail: Go print_value
//     terminates scalars with a newline, kcc print_value does not. That shows
//     up in a program containing no Result, no Option and no `?` whatsoever --
//     a bare print of an int already differs -- so it cannot be attributed to
//     this operator, and changing it would alter every kcc program's output.
//     Stripping newlines keeps the comparison sound for what these fixtures
//     assert, namely WHICH values were printed and in what order. The
//     CONTINUED sentinels still discriminate, because a present sentinel
//     changes the compacted string.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"karkain/pkg/codegen"
)

// lh1Compact removes carriage returns and line feeds so the two engines can be
// compared despite the unrelated scalar-newline divergence documented above.
func lh1Compact(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == 13 || r == 10 {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// lh1Fixture copies a propagation fixture into a temp dir so generated
// artifacts never land in the repository.
func lh1Fixture(t *testing.T, name string) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(wd, "..", "..", "examples", "propagation", name+".kark")
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

// lh1Cases pins the expected observable for each fixture. The values were
// derived from the language semantics and confirmed against the Go engine,
// which is the reference implementation of `?`.
var lh1Cases = []struct {
	file string
	want string // newline-compacted expected stdout, identical on both engines
	note string
}{
	{"ok_unwrap", "Ok(11)", "Case A: `?` unwraps Ok and yields 5*2+1"},
	{"err_propagate", "Err(too_big)", "Case B: `?` returns the original Err and stops execution"},
	{"nested_propagate", "Ok(6)Err(too_big)", "Case C: three function boundaries, success and error"},
	{"option_propagate", "CONTINUED11NoneSome(14)None", "Case D: Some is unwrapped, None is propagated"},
	{"expr_position", "710Err(nonpos)", "`?` as call arg, nested in one expression, and in arithmetic"},
}

// TestLH1_PropagationParity is the main gate: for every fixture both engines
// must succeed, agree with each other, and equal the pinned expected value.
func TestLH1_PropagationParity(t *testing.T) {
	for _, tc := range lh1Cases {
		t.Run(tc.file, func(t *testing.T) {
			fixture := lh1Fixture(t, tc.file)

			var goOut, goErr bytes.Buffer
			goRes := RunCommand(fixture, codegen.Config{Stdout: &goOut, Stderr: &goErr}, false)
			if goRes.ExitCode != ExitSuccess {
				t.Fatalf("Go engine: exit=%d want %d (stdout=%q stderr=%q)",
					goRes.ExitCode, ExitSuccess, goOut.String(), goErr.String())
			}
			goGot := lh1Compact(goOut.String())
			if goGot != tc.want {
				t.Errorf("Go engine: got %q, want %q (%s)", goGot, tc.want, tc.note)
			}

			kccRes := KCCRunCommand(nil, fixture, codegen.Config{}, false)
			if kccRes.ExitCode != ExitSuccess {
				t.Fatalf("kcc engine: exit=%d want %d (message=%q)",
					kccRes.ExitCode, ExitSuccess, kccRes.Message)
			}
			kccGot := lh1Compact(lastGoldenLine(kccRes.Message))
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

// TestLH1_PropagationStopsExecution is the direct control for Case B. The
// CONTINUED print sits immediately AFTER the propagating statement, so its
// absence proves control flow left the function; its appearing exactly once in
// the Option fixture proves None propagated while the Some path continued.
func TestLH1_PropagationStopsExecution(t *testing.T) {
	errRes := KCCRunCommand(nil, lh1Fixture(t, "err_propagate"), codegen.Config{}, false)
	if errRes.ExitCode != ExitSuccess {
		t.Fatalf("kcc engine: exit=%d (message=%q)", errRes.ExitCode, errRes.Message)
	}
	if body := lastGoldenLine(errRes.Message); strings.Contains(body, "CONTINUED") {
		t.Errorf("`?` did not stop execution: %q", body)
	}

	optRes := KCCRunCommand(nil, lh1Fixture(t, "option_propagate"), codegen.Config{}, false)
	if optRes.ExitCode != ExitSuccess {
		t.Fatalf("kcc engine: exit=%d (message=%q)", optRes.ExitCode, optRes.Message)
	}
	if body := lastGoldenLine(optRes.Message); strings.Count(body, "CONTINUED") != 1 {
		t.Errorf("want exactly one CONTINUED (Some path only), got %d in %q",
			strings.Count(body, "CONTINUED"), body)
	}
}

// TestLH1_OriginalFailureIsEliminated asserts that the exact C compiler
// diagnostic recorded in the audit no longer occurs.
func TestLH1_OriginalFailureIsEliminated(t *testing.T) {
	const original = "expected expression before ';' token"
	res := KCCRunCommand(nil, lh1Fixture(t, "err_propagate"), codegen.Config{}, false)
	if strings.Contains(res.Message, original) {
		t.Fatalf("the original defect is still present: kcc emitted malformed C (%q)", res.Message)
	}
	if res.ExitCode != ExitSuccess {
		t.Fatalf("kcc engine: exit=%d want %d (message=%q)", res.ExitCode, ExitSuccess, res.Message)
	}
}

// TestLH1_GeneratedCPropagates inspects the actual generated C, proving the
// operator reaches the backend as real code rather than as an empty statement:
// the statement expression, the Result/Option tag tests, the propagating
// return and the constructor helpers must all be present.
//
// The kcc binary is driven directly because KCCBuildCommand compiles inside a
// temporary sandbox that it deletes on return, so the emitted C is not
// observable through it. `build` writes the C beside its input, and
// lh1Fixture places that input in a temp dir.
func TestLH1_GeneratedCPropagates(t *testing.T) {
	bin, err := kccBinaryPath(nil)
	if err != nil {
		t.Fatalf("locate kcc: %v", err)
	}
	fixture := lh1Fixture(t, "err_propagate")

	out, err := exec.Command(bin, "build", fixture).CombinedOutput()
	if err != nil {
		t.Fatalf("kcc build: %v (output=%q)", err, string(out))
	}

	src, err := os.ReadFile(strings.TrimSuffix(fixture, ".kark") + ".c23")
	if err != nil {
		t.Fatalf("read generated C: %v (kcc said %q)", err, string(out))
	}
	c := string(src)

	for _, m := range []struct{ what, want string }{
		{"operand evaluated once into a temporary", "({Value _r = "},
		{"Result Err is propagated", "_r.resVal.tag == 1) return _r"},
		{"Option None is propagated", "_r.optVal.tag == 0) return _r"},
		{"Result constructors lower to the runtime helper", "result_err("},
		{"Option constructors lower to the runtime helper", "option_none("},
	} {
		if !strings.Contains(c, m.want) {
			t.Errorf("generated C is missing the %s marker: %s", m.what, m.want)
		}
	}
}

// TestLH1_LoweringIsStillPresent is the structural regression guard this slice
// requires: it fails if the lowering is ever removed again.
//
// An important correction to the audit's description of the defect: the
// construct is NOT carried by the NODE_PROPAGATE AST node on this path. kcc's
// parser lowers postfix `?` to a UnaryExpr whose operator is the question
// mark, so NODE_PROPAGATE (57) and its Propagate type name are dead on the
// lowering path. That is why searching codegen for a NODE_PROPAGATE case finds
// nothing, and why adding such a case would have changed nothing. The guard
// therefore pins the UnaryExpr question-mark case, which is the code that
// actually runs.
func TestLH1_LoweringIsStillPresent(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(wd, "..", "..")

	for _, c := range []struct{ file, what, want string }{
		{filepath.Join("src", "compiler", "codegen.kark"), "UnaryExpr question-mark lowering", "op == \"?\""},
		{filepath.Join("src", "compiler", "codegen.kark"), "ResultOk lowering", "nt == \"ResultOk\""},
		{filepath.Join("src", "compiler", "codegen.kark"), "ResultErr lowering", "nt == \"ResultErr\""},
		{filepath.Join("src", "compiler", "codegen.kark"), "OptionSome lowering", "nt == \"OptionSome\""},
		{filepath.Join("src", "compiler", "codegen.kark"), "OptionNone lowering", "nt == \"OptionNone\""},
		{filepath.Join("src", "compiler", "parser.kark"), "Ok expression form", "kind == TK_OK()"},
		{filepath.Join("src", "compiler", "parser.kark"), "Err expression form", "kind == TK_ERR()"},
		{filepath.Join("src", "compiler", "parser.kark"), "Some expression form", "kind == TK_SOME()"},
		{filepath.Join("src", "compiler", "parser.kark"), "None expression form", "kind == TK_NONE()"},
		{filepath.Join("src", "compiler", "ast.kark"), "ResultOk constructor", "func makeResultOk("},
		{filepath.Join("src", "compiler", "ast.kark"), "ResultErr constructor", "func makeResultErr("},
		{filepath.Join("src", "compiler", "ast.kark"), "OptionSome constructor", "func makeOptionSome("},
		{filepath.Join("src", "compiler", "ast.kark"), "OptionNone constructor", "func makeOptionNone("},
	} {
		b, err := os.ReadFile(filepath.Join(root, c.file))
		if err != nil {
			t.Fatalf("read %s: %v", c.file, err)
		}
		if !strings.Contains(string(b), c.want) {
			t.Errorf("%s: the %s support is gone (expected to find %s)", c.file, c.what, c.want)
		}
	}
}
