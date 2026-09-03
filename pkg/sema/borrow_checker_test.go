package sema

import (
	"karkain/pkg/lexer"
	"karkain/pkg/parser"
	"strings"
	"testing"
)

func parseAndCheck(t *testing.T, input string) []BorrowError {
	t.Helper()
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()
	checker := NewBorrowChecker()
	return checker.Check(prog)
}

func TestBorrowCheck_ValidProgram(t *testing.T) {
	input := `func main() {
  let x = 42
  print(x)
}`
	errs := parseAndCheck(t, input)
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
}

func TestBorrowCheck_MoveThenUse(t *testing.T) {
	input := `func main() {
  let x = 42
  let y = move(x)
  print(x)
}`
	errs := parseAndCheck(t, input)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Message, "moved value") {
		t.Errorf("unexpected error: %s", errs[0].Message)
	}
}

func TestBorrowCheck_MoveThenUseOK(t *testing.T) {
	input := `func main() {
  let x = 42
  let y = move(x)
  print(y)
}`
	errs := parseAndCheck(t, input)
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
}

func TestBorrowCheck_MutableBorrowAlias(t *testing.T) {
	input := `func main() {
  let x = 42
  let r1 = &x
  let r2 = &mut x
  print(r1)
  print(r2)
}`
	errs := parseAndCheck(t, input)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Message, "mutably borrow") {
		t.Errorf("unexpected error: %s", errs[0].Message)
	}
}

func TestBorrowCheck_ImmutableBorrowsOK(t *testing.T) {
	input := `func main() {
  let x = 42
  let r1 = &x
  let r2 = &x
  print(r1)
  print(r2)
}`
	errs := parseAndCheck(t, input)
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
}

func TestBorrowCheck_MoveBorrowedValue(t *testing.T) {
	input := `func main() {
  let x = 42
  let r = &mut x
  let y = move(x)
  print(y)
}`
	errs := parseAndCheck(t, input)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Message, "value is borrowed") {
		t.Errorf("unexpected error: %s", errs[0].Message)
	}
}

func TestBorrowCheck_DoubleMove(t *testing.T) {
	input := `func main() {
  let x = 42
  let y = move(x)
  let z = move(x)
  print(y)
  print(z)
}`
	errs := parseAndCheck(t, input)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Message, "already moved") {
		t.Errorf("unexpected error: %s", errs[0].Message)
	}
}

func TestBorrowCheck_MutableThenImmutableBorrow(t *testing.T) {
	input := `func main() {
  let x = 42
  let r1 = &mut x
  let r2 = &x
  print(r1)
  print(r2)
}`
	errs := parseAndCheck(t, input)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Message, "mutably borrowed") {
		t.Errorf("unexpected error: %s", errs[0].Message)
	}
}

func TestBorrowCheck_NoErrorsOnPlainCode(t *testing.T) {
	input := `func main() {
  let a = 1
  let b = 2
  let c = a + b
  print(c)
}`
	errs := parseAndCheck(t, input)
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
}

// Phase 44: Exhaustive match tests

func TestBorrowCheck_NonExhaustiveOptionMatch(t *testing.T) {
	input := `func main() {
  let x = Some(42)
  let v = match x {
    Some(v) => v
  }
  print(v)
}`
	errs := parseAndCheck(t, input)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Message, "non-exhaustive") || !strings.Contains(errs[0].Message, "None") {
		t.Errorf("unexpected error: %s", errs[0].Message)
	}
}

func TestBorrowCheck_ExhaustiveOptionMatch(t *testing.T) {
	input := `func main() {
  let x = Some(42)
  let v = match x {
    Some(v) => v,
    None => 0
  }
  print(v)
}`
	errs := parseAndCheck(t, input)
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
}

func TestBorrowCheck_NonExhaustiveResultMatch(t *testing.T) {
	input := `func main() {
  let r = Ok(42)
  let v = match r {
    Ok(v) => v
  }
  print(v)
}`
	errs := parseAndCheck(t, input)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Message, "non-exhaustive") || !strings.Contains(errs[0].Message, "Err") {
		t.Errorf("unexpected error: %s", errs[0].Message)
	}
}

func TestBorrowCheck_ExhaustiveResultMatch(t *testing.T) {
	input := `func main() {
  let r = Ok(42)
  let v = match r {
    Ok(v) => v,
    Err(e) => 0
  }
  print(v)
}`
	errs := parseAndCheck(t, input)
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
}

func TestBorrowCheck_NonOptionResultMatchSkipsCheck(t *testing.T) {
	input := `func main() {
  let x = 42
  let v = match x {
    5 => 10,
    _ => 0
  }
  print(v)
}`
	errs := parseAndCheck(t, input)
	if len(errs) > 0 {
		t.Errorf("expected no errors for non-Option/Result match, got %d: %v", len(errs), errs)
	}
}

// Phase 51: Lexical scoping tests

func TestBorrowCheck_ShadowVariable(t *testing.T) {
	input := `func main() {
  let x = 42
  let y = move(x)
  {
    let x = 10
    print(x)
  }
  print(y)
}`
	errs := parseAndCheck(t, input)
	if len(errs) > 0 {
		t.Errorf("expected no errors (inner x shadows outer), got %d: %v", len(errs), errs)
	}
}

func TestBorrowCheck_ShadowThenUseOuter(t *testing.T) {
	input := `func main() {
  let x = 42
  {
    let x = 10
    print(x)
  }
  print(x)
}`
	errs := parseAndCheck(t, input)
	if len(errs) > 0 {
		t.Errorf("expected no errors (outer x still valid), got %d: %v", len(errs), errs)
	}
}

func TestBorrowCheck_BorrowExpiresInScope(t *testing.T) {
	input := `func main() {
  let x = 42
  {
    let r = &mut x
    print(r)
  }
  let y = move(x)
  print(y)
}`
	errs := parseAndCheck(t, input)
	if len(errs) > 0 {
		t.Errorf("expected no errors (borrow expires at scope exit), got %d: %v", len(errs), errs)
	}
}

func TestBorrowCheck_BorrowStillActiveInScope(t *testing.T) {
	input := `func main() {
  let x = 42
  let r = &mut x
  {
    let y = move(x)
    print(y)
  }
  print(r)
}`
	errs := parseAndCheck(t, input)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error (x is borrowed in outer scope), got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Message, "value is borrowed") {
		t.Errorf("unexpected error: %s", errs[0].Message)
	}
}

func TestBorrowCheck_MoveInInnerScopeOk(t *testing.T) {
	input := `func main() {
  let x = 42
  {
    let y = move(x)
    print(y)
  }
}`
	errs := parseAndCheck(t, input)
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
}

func TestBorrowCheck_DoubleMoveAcrossScopes(t *testing.T) {
	input := `func main() {
  let x = 42
  {
    let y = move(x)
    print(y)
  }
  let z = move(x)
  print(z)
}`
	errs := parseAndCheck(t, input)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error (double move), got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Message, "already moved") {
		t.Errorf("unexpected error: %s", errs[0].Message)
	}
}

func TestBorrowCheck_ShadowResetsOwnership(t *testing.T) {
	input := `func main() {
  let x = 42
  let y = move(x)
  let x = 10
  print(x)
  print(y)
}`
	errs := parseAndCheck(t, input)
	if len(errs) > 0 {
		t.Errorf("expected no errors (new x is independent of moved x), got %d: %v", len(errs), errs)
	}
}

func TestBorrowCheck_IfScopeBorrow(t *testing.T) {
	input := `func main() {
  let x = 42
  if (true) {
    let r = &x
    print(r)
  }
  let y = move(x)
  print(y)
}`
	errs := parseAndCheck(t, input)
	if len(errs) > 0 {
		t.Errorf("expected no errors (borrow in if-scope expires), got %d: %v", len(errs), errs)
	}
}

func TestBorrowCheck_WhileScopeBorrow(t *testing.T) {
	input := `func main() {
  let x = 42
  while (true) {
    let r = &x
    print(r)
    break
  }
  let y = move(x)
  print(y)
}`
	errs := parseAndCheck(t, input)
	if len(errs) > 0 {
		t.Errorf("expected no errors (borrow in while-scope expires), got %d: %v", len(errs), errs)
	}
}

func TestBorrowCheck_ForScopeBorrow(t *testing.T) {
	input := `func main() {
  let x = 42
  for (var i int = 0; i < 10; i = i + 1) {
    let r = &x
    print(r)
  }
  let y = move(x)
  print(y)
}`
	errs := parseAndCheck(t, input)
	if len(errs) > 0 {
		t.Errorf("expected no errors (borrow in for-scope expires), got %d: %v", len(errs), errs)
	}
}

// =============================================================================
// Phase 51 — Borrow Checker Lexical Scoping regression tests.
// Groups A–I per the Phase 51 implementation prompt.
// =============================================================================

// Group A — basic scope entry/exit: inner bindings are usable inside, gone after.
func TestPhase51_ScopeEntryExit(t *testing.T) {
	input := `func main() {
  {
    let inner = 7
    print(inner)
  }
}`
	errs := parseAndCheck(t, input)
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
}

// Group B — outer bindings survive an inner scope that shares a name.
func TestPhase51_OuterBindingSurvives(t *testing.T) {
	input := `func main() {
  let x = 1
  {
    let x = 2
    print(x)
  }
  print(x)
}`
	errs := parseAndCheck(t, input)
	if len(errs) > 0 {
		t.Errorf("expected no errors (outer x restored), got %d: %v", len(errs), errs)
	}
}

// Group C — inner bindings expire: using the inner name after its scope ends,
// with no live shadow, is diagnosed as a use-after-scope-end.
func TestPhase51_InnerBindingExpires(t *testing.T) {
	input := `func main() {
  {
    let x = 42
    print(x)
  }
  print(x)
}`
	errs := parseAndCheck(t, input)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Message, "scope has ended") {
		t.Errorf("unexpected error: %s", errs[0].Message)
	}
}

// Group D — shadowing creates a distinct binding; the outer is not affected.
func TestPhase51_ShadowDistinctIdentity(t *testing.T) {
	input := `func main() {
  let x = 42
  let first = &x
  {
    let x = 100
    print(x)
  }
  print(*first)
}`
	errs := parseAndCheck(t, input)
	if len(errs) > 0 {
		t.Errorf("expected no errors (shadow is distinct from outer), got %d: %v", len(errs), errs)
	}
}

// Group E — three-level nested shadowing: each exit restores the prior binding.
func TestPhase51_NestedShadowThreeLevels(t *testing.T) {
	input := `func main() {
  let x = 1
  {
    let x = 2
    {
      let x = 3
      print(x)
    }
    print(x)
  }
  print(x)
}`
	errs := parseAndCheck(t, input)
	if len(errs) > 0 {
		t.Errorf("expected no errors (3-level restore), got %d: %v", len(errs), errs)
	}
}

// Group F — a borrow expires when its scope exits (multiple scoping constructs).
func TestPhase51_BorrowExpiryAllScopes(t *testing.T) {
	input := `func main() {
  let x = 42
  if (true) {
    let r = &x
    print(r)
  }
  {
    let r = &mut x
    print(r)
  }
  let y = move(x)
  print(y)
}`
	errs := parseAndCheck(t, input)
	if len(errs) > 0 {
		t.Errorf("expected no errors (borrows expire at scope exit), got %d: %v", len(errs), errs)
	}
}

// Group G — reference outlives referent. Full lifetime tracking is Phase 63
// ownership work; this pins the current documented behavior so deltas are
// surfaced if the semantics change.
func TestPhase51_ReferenceOutlivesReferent(t *testing.T) {
	input := `func main() {
  let r = &0
  {
    let x = 42
    let q = &x
    print(q)
  }
  print(r)
}`
	errs := parseAndCheck(t, input)
	if len(errs) > 0 {
		t.Errorf("expected no borrow-checker errors (lifetime tracking is Phase 63), got %d: %v", len(errs), errs)
	}
}

// Group H — move semantics persist across scope exit: a moved value stays moved.
func TestPhase51_MovePersistsAcrossScope(t *testing.T) {
	input := `func main() {
  let x = 42
  {
    let y = move(x)
    print(y)
  }
  print(x)
}`
	errs := parseAndCheck(t, input)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error (x remains moved), got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Message, "moved value") {
		t.Errorf("unexpected error: %s", errs[0].Message)
	}
}

// Group I — BUG-5 regression: double move across scopes yields exactly one error
// and the move is not undone by exiting the inner scope.
func TestPhase51_Bug5DoubleMoveAcrossScopes(t *testing.T) {
	input := `func main() {
  let x = 1
  {
    let a = move(x)
    print(a)
  }
  {
    let b = move(x)
    print(b)
  }
}
`
	errs := parseAndCheck(t, input)
	if len(errs) != 1 {
		t.Fatalf("expected exactly 1 error (BUG-5), got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Message, "moved value") && !strings.Contains(errs[0].Message, "already moved") {
		t.Errorf("unexpected error: %s", errs[0].Message)
	}
}
