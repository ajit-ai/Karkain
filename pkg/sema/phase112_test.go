package sema

import (
	"strings"
	"testing"
)

// Phase 112 gate: function-scope `const` declarations must (a) resolve clean,
// (b) reject reassignment with a "cannot reassign constant" diagnostic, and
// (c) leave regular let/var reassignment untouched — on BOTH the vanilla
// resolver path and the macro-expansion path used by AnalyzeSource.

func TestPhase112_ConstDeclaresAndResolves(t *testing.T) {
	src := "func main() {\n  const MAX = 100\n  let y = MAX + 1\n  println(y)\n}\n"
	prog := parseTestProg(t, src)
	r := NewResolver(prog, nil)
	errs := r.Resolve()
	if len(errs) > 0 {
		t.Fatalf("const declaration should resolve clean, got: %s", errLines(errs))
	}
}

func TestPhase112_ConstReassignmentRejected(t *testing.T) {
	src := "func main() {\n  const MAX = 100\n  MAX = 200\n}\n"
	prog := parseTestProg(t, src)
	r := NewResolver(prog, nil)
	errs := r.Resolve()
	found := false
	for _, e := range errs {
		if strings.Contains(e.Msg, "cannot reassign constant 'MAX'") {
			found = true
			if e.Line != 3 {
				t.Errorf("expected line 3 for reassignment diagnostic, got %d", e.Line)
			}
		}
	}
	if !found {
		t.Fatalf("expected const reassignment error, got: %s", errLines(errs))
	}
}

func TestPhase112_ConstReassignmentRejectedAfterMacroExpansion(t *testing.T) {
	// AnalyzeSource runs ApplyMacroExpansion before the resolver; the
	// Const flag must survive the expander or reassignment goes undetected.
	src := "func main() {\n  const MAX = 100\n  MAX = 200\n}\n"
	prog := macroParseTestProg(t, src)
	r := NewResolver(prog, nil)
	errs := r.Resolve()
	for _, e := range errs {
		if strings.Contains(e.Msg, "cannot reassign constant 'MAX'") {
			return
		}
	}
	t.Fatalf("expected const reassignment error after macro expansion, got: %s", errLines(errs))
}

func TestPhase112_LetReassignmentStillAllowed(t *testing.T) {
	src := "func main() {\n  let n = 1\n  n = 2\n  println(n)\n}\n"
	prog := parseTestProg(t, src)
	r := NewResolver(prog, nil)
	errs := r.Resolve()
	for _, e := range errs {
		if strings.Contains(e.Msg, "cannot reassign") {
			t.Fatalf("let reassignment wrongly rejected: %s", errLines(errs))
		}
	}
}

func TestPhase112_FloatBuiltinKnown(t *testing.T) {
	src := "func main() {\n  let f = float(3)\n  println(f)\n}\n"
	prog := parseTestProg(t, src)
	r := NewResolver(prog, nil)
	errs := r.Resolve()
	for _, e := range errs {
		if strings.Contains(e.Msg, "not defined") || strings.Contains(e.Msg, "undefined") {
			t.Fatalf("float builtin flagged as undefined: %s", errLines(errs))
		}
	}
}