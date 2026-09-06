package sema

import (
	"strings"
	"testing"
)

// Phase 83 — warning infrastructure: unused-variable findings are conservative,
// deterministic, span-accurate and never fire on writes-only constructs that
// the walker cannot see.

func TestUnusedVars_Detected(t *testing.T) {
	src := "func main() {\n  let count = 10\n  println(\"hi\")\n}\n"
	prog := parseTestProg(t, src)
	warns := UnusedVars(prog)

	if len(warns) != 1 {
		t.Fatalf("expected 1 warning, got %d (%+v)", len(warns), warns)
	}
	w := warns[0]
	if !strings.Contains(w.Msg, "unused variable `count`") {
		t.Errorf("unexpected message: %s", w.Msg)
	}
	// "  let count = 10": 'c' is byte column 6 (0-based), 'count' ends at 11.
	if w.Line != 2 {
		t.Errorf("expected line 2, got %d", w.Line)
	}
	if w.Col != 6 {
		t.Errorf("expected col 6, got %d", w.Col)
	}
	if w.EndCol != 11 {
		t.Errorf("expected endCol 11, got %d", w.EndCol)
	}
}

func TestUnusedVars_SeveralInOrder(t *testing.T) {
	src := "func main() {\n  let a = 1\n  let b = 2\n  println(\"x\")\n}\n"
	warns := UnusedVars(parseTestProg(t, src))
	if len(warns) != 2 {
		t.Fatalf("expected 2 warnings, got %d (%+v)", len(warns), warns)
	}
	if !strings.Contains(warns[0].Msg, "`a`") || !strings.Contains(warns[1].Msg, "`b`") {
		t.Errorf("warnings not ordered by declaration: %+v", warns)
	}
	if !(warns[0].Line < warns[1].Line) {
		t.Errorf("warnings must sort by line")
	}
}

func TestUnusedVars_ReadsCountAsUse(t *testing.T) {
	// Every declaration below is read in at least one value position.
	src := "func main() {\n" +
		"  let direct = 1\n" +
		"  println(direct)\n" +
		"  let inIf = 2\n" +
		"  if inIf > 0 {\n" +
		"    print(inIf)\n" +
		"  }\n" +
		"  let inWhile = 3\n" +
		"  while inWhile < 10 {\n" +
		"    print(inWhile)\n" +
		"  }\n" +
		"  let inArr = 4\n" +
		"  let arr = [1, inArr]\n" +
		"  print(len(arr))\n" +
		"}\n"
	if warns := UnusedVars(parseTestProg(t, src)); len(warns) != 0 {
		t.Fatalf("expected no warnings, got %+v", warns)
	}
}

func TestUnusedVars_IndexedWriteStillUses(t *testing.T) {
	// Assignment to an element reads the base array.
	src := "func main() {\n  let arr = [1, 2]\n  arr[0] = 9\n  print(arr[0])\n}\n"
	if warns := UnusedVars(parseTestProg(t, src)); len(warns) != 0 {
		t.Fatalf("expected no warnings, got %+v", warns)
	}
}

func TestUnusedVars_WriteOnlyIsUnused(t *testing.T) {
	// `count` is assigned inside main but never read — still reported.
	src := "func main() {\n  let count = 0\n  count = 5\n}\n"
	warns := UnusedVars(parseTestProg(t, src))
	if len(warns) != 1 || !strings.Contains(warns[0].Msg, "`count`") {
		t.Fatalf("expected write-only warning, got %+v", warns)
	}
}

func TestUnusedVars_ExemptsParamsAndLoopVars(t *testing.T) {
	src := "func helper(x) {\n  return 5\n}\n" +
		"func main() {\n" +
		"  let arr = [1, 2]\n" +
		"  for v in arr {\n" +
		"    print(\"loop\")\n" +
		"  }\n" +
		"}\n"
	// `x` (unused param) and `v` (unread loop var) must not warn; `arr` is read
	// by the iteration expression.
	if warns := UnusedVars(parseTestProg(t, src)); len(warns) != 0 {
		t.Fatalf("expected no warnings, got %+v", warns)
	}
}

func TestUnusedVars_ConservativeOnUnknown(t *testing.T) {
	// Named-function declarations (let f = fn...) carry nested bodies the
	// walker treats as opaque; the whole function's findings are suppressed
	// rather than risk a false positive.
	src := "func main() {\n  let unused_here = 1\n  let f = fn(x) { return x }\n  print(f(1))\n}\n"
	if warns := UnusedVars(parseTestProg(t, src)); len(warns) != 0 {
		t.Fatalf("expected suppression on unknown constructs, got %+v", warns)
	}
}

func TestUnusedVars_UnusedInsideLoopDeclared(t *testing.T) {
	// A declaration inside the loop body is tracked per token span.
	src := "func main() {\n  for (let i = 0; i < 3; i = i + 1) {\n    let inner = i\n    print(\"x\")\n  }\n}\n"
	warns := UnusedVars(parseTestProg(t, src))
	if len(warns) != 1 || !strings.Contains(warns[0].Msg, "`inner`") {
		t.Fatalf("expected inner unused warning, got %+v", warns)
	}
	if warns[0].Line != 3 {
		t.Errorf("expected line 3, got %d", warns[0].Line)
	}
}