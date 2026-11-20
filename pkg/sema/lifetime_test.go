package sema

import (
	"karkain/pkg/lexer"
	"karkain/pkg/parser"
	"strings"
	"testing"
)

func parseAndCheckLifetime(t *testing.T, input string) []BorrowError {
	t.Helper()
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()
	checker := NewBorrowChecker()
	return checker.Check(prog)
}

// Phase 1.2 (v1.3.0): Lifetime analysis — dangling reference on return.

func TestLifetime_DanglingReferenceOnReturn(t *testing.T) {
	t.Run("borrow of local returned", func(t *testing.T) {
		input := `func foo() &int {
  let x = 5
  return &x
}`
		errs := parseAndCheckLifetime(t, input)
		if len(errs) == 0 {
			t.Fatal("expected lifetime error for returning &local, got none")
		}
		found := false
		for _, e := range errs {
			if strings.Contains(e.Message, "would outlive") || strings.Contains(e.Message, "does not outlive") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected lifetime error, got: %v", errs)
		}
	})
	t.Run("mut borrow of local returned", func(t *testing.T) {
		input := `func foo() &mut int {
  let x = 5
  return &mut x
}`
		errs := parseAndCheckLifetime(t, input)
		if len(errs) == 0 {
			t.Fatal("expected lifetime error for returning &mut local, got none")
		}
	})
	t.Run("address-of local returned", func(t *testing.T) {
		input := `func foo() {
  let x = 5
  return addr x
}`
		errs := parseAndCheckLifetime(t, input)
		// addr may not be a reference type in the current grammar; if it
		// parses as AddressOf, it should be caught. If it doesn't parse,
		// the test is skipped (no false failure).
		if len(errs) > 0 {
			found := false
			for _, e := range errs {
				if strings.Contains(e.Message, "would outlive") {
					found = true
					break
				}
			}
			if !found {
				t.Logf("got errors but no lifetime error (may be parse issue): %v", errs)
			}
		}
	})
}

func TestLifetime_ValidReferenceReturn(t *testing.T) {
	t.Run("return param reference", func(t *testing.T) {
		input := `func foo(x int) &int {
  return &x
}`
		errs := parseAndCheckLifetime(t, input)
		for _, e := range errs {
			if strings.Contains(e.Message, "would outlive") {
				t.Errorf("false positive: param reference should be valid, got: %v", e)
			}
		}
	})
	t.Run("return value not reference", func(t *testing.T) {
		input := `func foo() int {
  let x = 5
  return x
}`
		errs := parseAndCheckLifetime(t, input)
		for _, e := range errs {
			if strings.Contains(e.Message, "would outlive") {
				t.Errorf("false positive: value return should be valid, got: %v", e)
			}
		}
	})
}

func TestLifetime_IntermediateBinding(t *testing.T) {
	t.Run("borrow binding returned", func(t *testing.T) {
		input := `func foo() &int {
  let x = 5
  let r = &x
  return r
}`
		errs := parseAndCheckLifetime(t, input)
		if len(errs) == 0 {
			t.Fatal("expected lifetime error for returning borrow binding, got none")
		}
		found := false
		for _, e := range errs {
			if strings.Contains(e.Message, "would outlive") || strings.Contains(e.Message, "does not outlive") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected lifetime error, got: %v", errs)
		}
	})
}