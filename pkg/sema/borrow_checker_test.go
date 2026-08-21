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
