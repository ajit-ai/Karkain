package codegen

import (
	"strings"
	"testing"
)

// Phase 130 gate — capture MUTATION on the Go engine.
//
// Phase 121 pinned capture analysis and codegen markers for read-only captures
// (`let base` read inside the closure). Phase 130 completes the story by pinning
// the WRITE-THROUGH path: a closure body that ASSIGNS to a captured `var`
// declares the alias such that the assignment goes through the env pointer to
// the enclosing binding — and the enclosing scope observes the mutation after
// the call returns (examples/closures/00_capture_mutation.kark golden:
// 1/2/2/3/3).

// TestPhase130_CaptureMutationCodegenMarkers pins the emitted C for a closure
// that mutates a captured var: the alias is used on the left-hand side of an
// assignment inside the closure body, and the binding site wires the env field
// to the enclosing variable address.
func TestPhase130_CaptureMutationCodegenMarkers(t *testing.T) {
	src := `
func main() {
	var counter = 0
	let bump = fn() {
		counter = counter + 1
		return counter
	}
	println(bump())
	println(bump())
	println(counter)
}
`
	generated, _ := compileCOnly(t, src)
	for _, marker := range []string{
		// The capture set is the same as Phase 121: an env typedef with a
		// Value* field per captured name (captured as a POINTER, so mutations
		// and reads both go through the alias to the enclosing var).
		"typedef struct {\n\tValue* karkain_cap_counter;\n} ClosureEnv_bump;",
		"static ClosureEnv_bump* _genv_bump;",
		"Value karkain_user_bump(ClosureEnv_bump* _env);",
		"#define counter (*_env->karkain_cap_counter)",
		// The mutation writes THROUGH the alias — this is the Phase 130 signal.
		"counter = binary_op(counter, \"+\", make_int(1));",
		"#undef counter",
		// Binding site: env field holds the enclosing variable's address.
		"{ static ClosureEnv_bump _e; _e.karkain_cap_counter = &counter; _genv_bump = &_e; }",
		"karkain_user_bump(_genv_bump)",
	} {
		if !strings.Contains(generated, marker) {
			t.Errorf("generated C missing %q", marker)
		}
	}
}

// TestPhase130_NestedMutationCodegenMarkers pins a nested closure that mutates
// an outer-local-in-enclosing var: the inner lambda's env field for that name
// copies the pointer from the OUTER env (the outer closure captured it by-ref
// at its own binding site), proving mutation stays aliased across nesting.
func TestPhase130_NestedMutationCodegenMarkers(t *testing.T) {
	src := `
func main() {
	var log = 0
	let outer = fn() {
		let inner = fn() {
			log = log + 1
			return log
		}
		return inner()
	}
	println(outer())
	println(log)
}
`
	generated, _ := compileCOnly(t, src)
	// Inner closure captures `log` through the outer env pointer chain.
	for _, marker := range []string{
		"typedef struct {\n\tValue* karkain_cap_log;\n} ClosureEnv_inner;",
		"#define log (*_env->karkain_cap_log)",
		"log = binary_op(log, \"+\", make_int(1));",
		// inner binding inside outer: log is an outer CAPTURE (pointer copy).
		"{ static ClosureEnv_inner _e; _e.karkain_cap_log = _env->karkain_cap_log; _genv_inner = &_e; }",
		"karkain_user_inner(_genv_inner)",
	} {
		if !strings.Contains(generated, marker) {
			t.Errorf("generated C missing %q", marker)
		}
	}
}
