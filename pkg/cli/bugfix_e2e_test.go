package cli

import (
	"strings"
	"testing"
)

// TestBugFix_OptionPropagation verifies BUG-4: the `?` operator now actively
// propagates `None` (early-returns from the enclosing function) and unwraps
// the payload of `Some(v)` — it is no longer a no-op for Option values.
func TestBugFix_OptionPropagation(t *testing.T) {
	out := buildRunCapture(t, t.TempDir(), "prop.kark", `
func inner() {
    let v = f()?
    print("unreached")
}
func f() { return None }
func main() {
    print("before")
    inner()
    print("after")
}
`)
	if !strings.Contains(out, "before") {
		t.Errorf("expected 'before' printed, got %q", out)
	}
	if strings.Contains(out, "unreached") {
		t.Errorf("None must propagate (unreached body must not run), got %q", out)
	}
	if !strings.Contains(out, "after") {
		t.Errorf("main must continue after propagation, got %q", out)
	}
}

// TestBugFix_OptionUnwrap verifies BUG-4: `?` unwraps the Some payload.
func TestBugFix_OptionUnwrap(t *testing.T) {
	out := buildRunCapture(t, t.TempDir(), "unwrap.kark", `
func inner() {
    let v = f()?
    print(v)
}
func f() { return Some(42) }
func main() {
    inner()
}
`)
	if !strings.Contains(out, "42") {
		t.Errorf("expected Some payload 42 to be unwrapped, got %q", out)
	}
}

// TestBugFix_EnumPayloadConstructor verifies BUG-7: a payload enum variant
// compiles (constructor named {Enum}_make_{Variant}) and matches by tag.
func TestBugFix_EnumPayloadConstructor(t *testing.T) {
	out := buildRunCapture(t, t.TempDir(), "enum.kark", `
enum Color { Red(int), Blue(string) }
func main() {
    let c = Color.Red(42)
    match c {
      Color.Red => print(1),
      Color.Blue => print(2),
      _ => print(9)
    }
}
`)
	if !strings.Contains(out, "1") {
		t.Errorf("expected Red arm to match, got %q", out)
	}
}

// TestBugFix_TypedColonDecl verifies BUG-8: `let n: int = 7` places the type
// annotation in the declaration instead of mis-parsing `int` as the left
// operand of an assignment.
func TestBugFix_TypedColonDecl(t *testing.T) {
	out := buildRunCapture(t, t.TempDir(), "typed.kark", `
func main() {
    let n: int = 7
    print(n)
}
`)
	if !strings.Contains(out, "7") {
		t.Errorf("expected typed decl value 7, got %q", out)
	}
}
