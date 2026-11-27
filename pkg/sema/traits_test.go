package sema

import (
	"strings"
	"testing"
)

func TestTraitDecl_Completeness(t *testing.T) {
	good := parseTestProg(t, "trait Numeric { fn add(x int) int; }\nimpl Numeric for int { func add(x int) int { return x } }\nfunc main() { print(1) }\n")
	r := NewResolver(good, nil)
	if errs := r.Resolve(); len(errs) != 0 {
		t.Fatalf("valid trait/impl rejected: %v", errs)
	}
	bad := parseTestProg(t, "trait Numeric { fn add(x int) int; }\nimpl Numeric for int { }\nfunc main() { print(1) }\n")
	r = NewResolver(bad, nil)
	errs := r.Resolve()
	if len(errs) == 0 {
		t.Fatal("impl missing a trait method was accepted")
	}
	if !strings.Contains(errs[0].Msg, "missing method 'add'") {
		t.Fatalf("unexpected diagnostic: %v", errs)
	}
}

func TestTraitDecl_UnknownTraitAndArity(t *testing.T) {
	unknown := parseTestProg(t, "impl Nope for int { }\nfunc main() { print(1) }\n")
	r := NewResolver(unknown, nil)
	errs := r.Resolve()
	if len(errs) == 0 || !strings.Contains(errs[0].Msg, "unknown trait 'Nope'") {
		t.Fatalf("unknown trait not reported: %v", errs)
	}
	arity := parseTestProg(t, "trait T { fn f(x int) int; }\nimpl T for int { func f() int { return 0 } }\nfunc main() { print(1) }\n")
	r = NewResolver(arity, nil)
	errs = r.Resolve()
	if len(errs) == 0 || !strings.Contains(errs[0].Msg, "has 0 parameters") || !strings.Contains(errs[0].Msg, "requires 1") {
		t.Fatalf("arity mismatch not reported: %v", errs)
	}
}
