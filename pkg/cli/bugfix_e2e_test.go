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

// TestAlgo_ChainedIndex2DTable verifies the algorithm-suite fixes end-to-end:
// (1) `dp[i][j] = v` in statement position must parse as one nested IndexExpr
// (not split into `dp[i]` + a misparsed `[j] = v` ArrayLiteral assignment) and
// actually mutate the shared heap row; and (2) user functions named `min`/`max`
// must compile on Windows where <windows.h> defines min/max macros.
func TestAlgo_ChainedIndex2DTable(t *testing.T) {
	out := buildRunCapture(t, t.TempDir(), "knap.kark", `
func max(a, b) {
    if (a >= b) {
        return a
    }
    return b
}
func knapsack(weights, values, capacity) {
    let n = len(weights)
    let dp = []
    let i = 0
    while (i <= n) {
        let row = []
        let j = 0
        while (j <= capacity) {
            push(row, 0)
            j = j + 1
        }
        push(dp, row)
        i = i + 1
    }
    i = 1
    while (i <= n) {
        let j = 0
        while (j <= capacity) {
            if (weights[i - 1] <= j) {
                let withItem = dp[i - 1][j - weights[i - 1]] + values[i - 1]
                dp[i][j] = max(withItem, dp[i - 1][j])
            } else {
                dp[i][j] = dp[i - 1][j]
            }
            j = j + 1
        }
        i = i + 1
    }
    return dp[n][capacity]
}
func main() {
    print(knapsack([1, 3, 4, 5], [1, 4, 5, 7], 7))
}
`)
	if !strings.Contains(out, "9") {
		t.Errorf("expected knapsack result 9, got %q", out)
	}
}

// TestAlgo_StringCharIndex verifies the string indexing fix: s[i] must route
// through array_or_string_get so Karkain strings support character access.
func TestAlgo_StringCharIndex(t *testing.T) {
	out := buildRunCapture(t, t.TempDir(), "sidx.kark", `
func main() {
    let s = "hello"
    print(s[1])
    print(s[4])
}
`)
	if !strings.Contains(out, "e") || !strings.Contains(out, "o") {
		t.Errorf("expected char access s[1]='e' and s[4]='o', got %q", out)
	}
}
