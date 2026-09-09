package codegen

import (
	"strings"
	"testing"

	"karkain/pkg/parser"
)

// Phase 100 gate: the runtime failure model. Every generated program must carry
// the runtime-error reporting foundation, and division/modulo/index sites must
// route through the checked helpers with the source file and line embedded so a
// failing program reports a source-located diagnostic instead of silently
// returning 0.

// TestPhase100_HeaderRuntimeErrorFoundation verifies the generated C header
// always includes the runtime-error reporter and the four checked helpers.
func TestPhase100_HeaderRuntimeErrorFoundation(t *testing.T) {
	g := New(Config{})
	h := g.generateCHeader()
	for _, fn := range []string{
		"karkain_runtime_error",
		"karkain_checked_div",
		"karkain_checked_mod",
		"karkain_checked_get",
		"karkain_checked_set",
		"runtime error: ",
	} {
		if !strings.Contains(h, fn) {
			t.Errorf("generated header missing %q", fn)
		}
	}
}

// TestPhase100_CheckedDivisionEmission verifies `/`, `%` and the `mod` builtin
// route through the checked wrappers carrying file and line, while other binary
// operators keep the single authoritative binary_op path.
func TestPhase100_CheckedDivisionEmission(t *testing.T) {
	g := New(Config{})
	g.sourceFile = "probe.kark"

	div := g.genExpr(&parser.BinaryExpr{
		Left:     &parser.IntLiteral{Value: "10"},
		Operator: "/",
		Right:    &parser.IntLiteral{Value: "0"},
		Line:     7,
	})
	if !strings.Contains(div, `karkain_checked_div(make_int(10), make_int(0), "probe.kark", 7)`) {
		t.Errorf("unexpected division emission: %s", div)
	}

	mod := g.genExpr(&parser.BinaryExpr{
		Left:     &parser.IntLiteral{Value: "10"},
		Operator: "%",
		Right:    &parser.IntLiteral{Value: "0"},
		Line:     8,
	})
	if !strings.Contains(mod, `karkain_checked_mod(make_int(10), make_int(0), "probe.kark", 8)`) {
		t.Errorf("unexpected modulo emission: %s", mod)
	}

	modBuiltin := g.genExpr(&parser.CallExpr{
		Function: "mod",
		Args:     []parser.Node{&parser.IntLiteral{Value: "10"}, &parser.IntLiteral{Value: "0"}},
		Line:     9,
	})
	if !strings.Contains(modBuiltin, `karkain_checked_mod(make_int(10), make_int(0), "probe.kark", 9)`) {
		t.Errorf("unexpected mod builtin emission: %s", modBuiltin)
	}

	add := g.genExpr(&parser.BinaryExpr{
		Left:     &parser.IntLiteral{Value: "1"},
		Operator: "+",
		Right:    &parser.IntLiteral{Value: "2"},
		Line:     3,
	})
	if !strings.Contains(add, `binary_op(make_int(1), "+", make_int(2))`) {
		t.Errorf("unexpected addition emission: %s", add)
	}
}

// TestPhase100_CheckedIndexEmission verifies array/string index reads and index
// writes route through the checked helpers with file and line embedded.
func TestPhase100_CheckedIndexEmission(t *testing.T) {
	g := New(Config{})
	g.sourceFile = "probe.kark"

	read := g.genExpr(&parser.IndexExpr{
		Left:  &parser.Identifier{Name: "xs"},
		Index: &parser.IntLiteral{Value: "7"},
		Line:  5,
	})
	if !strings.Contains(read, `karkain_checked_get(xs, make_int(7), "probe.kark", 5)`) {
		t.Errorf("unexpected index read emission: %s", read)
	}

	write := g.genExpr(&parser.BinaryExpr{
		Left: &parser.IndexExpr{
			Left:  &parser.Identifier{Name: "xs"},
			Index: &parser.IntLiteral{Value: "7"},
		},
		Operator: "=",
		Right:    &parser.IntLiteral{Value: "99"},
		Line:     6,
	})
	if !strings.Contains(write, `karkain_checked_set(&xs, make_int(7), make_int(99), "probe.kark", 6)`) {
		t.Errorf("unexpected index write emission: %s", write)
	}
}
