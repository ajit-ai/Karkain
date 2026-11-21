package sema

import (
	"strings"
	"testing"
)

// Phase 1.3 corrective -- the compile-time bounds checker's array-size table
// is keyed by FUNCTION as well as by variable name, and a size declared in a
// block is restored when that block ends.
//
// The first version was keyed by name alone. It rejected the compiler's own
// sources: a `Value s = [[], []]` in one function made `s[14]` in a DIFFERENT
// function -- where `s` was a parameter -- look out of range. The fix is in
// pkg/sema/bounds_check.go; this gate keeps the two properties it bought.
// It lives in pkg/sema because the checker is a resolver pass and emits no
// bytes; an end-to-end CLI golden could not distinguish "wrong diagnostic"
// from "correct rejection" without reimplementing the table.

func boundsErrors(errs []ResolveError) []string {
	var out []string
	for _, e := range errs {
		if strings.Contains(e.Msg, "out of range (length") {
			out = append(out, e.Msg)
		}
	}
	return out
}

func TestPhase13_BoundsTableIsFunctionScoped(t *testing.T) {
	src := "func a() {\n" +
		"    let s = [1, 2]\n" +
		"    print(s[9])\n" +
		"}\n" +
		"func b(s int) {\n" +
		"    print(s[9])\n" +
		"}\n" +
		"func main() {\n" +
		"    a()\n" +
		"    b(1)\n" +
		"}\n"

	errs := NewResolver(parseTestProg(t, src), nil).Resolve()
	got := boundsErrors(errs)
	if len(got) != 1 {
		t.Fatalf("want exactly one bounds diagnostic (from a), got %d: %v\nall errors: %s",
			len(got), got, errLines(errs))
	}
	if !strings.Contains(got[0], "length 2") {
		t.Errorf("the diagnostic should report the length of a's literal: %s", got[0])
	}
}

func TestPhase13_ReassignmentForgetsLiteralSize(t *testing.T) {
	src := "func main() {\n" +
		"    let fns = []\n" +
		"    fns = appendArray(fns, 7)\n" +
		"    print(fns[0])\n" +
		"}\n"

	errs := NewResolver(parseTestProg(t, src), nil).Resolve()
	if got := boundsErrors(errs); len(got) != 0 {
		t.Fatalf("appendArray assignment should remove the provable literal size, got: %v\nall errors: %s",
			got, errLines(errs))
	}
}

func TestPhase13_LoopReassignmentSurvivesBlockRestore(t *testing.T) {
	// Exact shape from examples/closures/04_factory.kark that failed CI:
	// an empty literal is grown inside a while body, then indexed after it.
	src := "func main() {\n" +
		"    let fns = []\n" +
		"    int i = 0\n" +
		"    while (i < 3) {\n" +
		"        fns = appendArray(fns, i)\n" +
		"        i = i + 1\n" +
		"    }\n" +
		"    print(fns[0])\n" +
		"}\n"

	errs := NewResolver(parseTestProg(t, src), nil).Resolve()
	if got := boundsErrors(errs); len(got) != 0 {
		t.Fatalf("loop reassignment invalidation must survive block restore, got: %v\nall errors: %s",
			got, errLines(errs))
	}
}

func TestPhase13_BlockDeclarationDoesNotOutliveItsBlock(t *testing.T) {
	src := "func main() {\n" +
		"    let s = [1, 2]\n" +
		"    if (true) {\n" +
		"        let s = [1, 2, 3, 4, 5, 6]\n" +
		"        print(s[5])\n" +
		"    }\n" +
		"    print(s[3])\n" +
		"}\n"

	errs := NewResolver(parseTestProg(t, src), nil).Resolve()
	got := boundsErrors(errs)
	// The inner s[5] is in range. The outer s[3] is NOT: only the inner
	// declaration has six elements, and it must not leak past the block.
	if len(got) != 1 {
		t.Fatalf("want exactly one bounds diagnostic (the outer s[3]), got %d: %v\nall errors: %s",
			len(got), got, errLines(errs))
	}
	if !strings.Contains(got[0], "length 2") {
		t.Errorf("the outer binding should still be length 2, got: %s", got[0])
	}
	if strings.Contains(got[0], "length 6") {
		t.Errorf("the inner block's size leaked out of its scope: %s", got[0])
	}
}
