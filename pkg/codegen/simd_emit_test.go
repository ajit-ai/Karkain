package codegen

import (
	"strings"
	"testing"

	"karkain/pkg/parser"
)

// Phase 106 gate: SIMD & Vector Types. The unit tests here pin the emitted OPS
// surface and the -mavx wiring; pkg/cli/phase106_simd_test.go proves the whole
// pipeline (gcc compile, run, AVX2 instruction emission).

// TestPhase106_OpNameNaming pins the deterministic karkain_simd_* helper names.
func TestPhase106_OpNameNaming(t *testing.T) {
	if got := simdOpName("add", "f32", 8); got != "karkain_simd_add_f32x8" {
		t.Errorf("simdOpName(add,f32,8) = %q", got)
	}
	if got := simdOpName("sum", "i32", 4); got != "karkain_simd_sum_i32x4" {
		t.Errorf("simdOpName(sum,i32,4) = %q", got)
	}
	if got := simdTyName("f64", 4); got != "karkain_f64x4" {
		t.Errorf("simdTyName(f64,4) = %q", got)
	}
}

// TestPhase106_WideLaneDetection pins which widths need AVX on x86.
func TestPhase106_WideLaneDetection(t *testing.T) {
	cases := []struct {
		elem string
		lanes int
		want bool
	}{
		{"f32", 4, false}, {"f32", 8, true},
		{"f64", 2, false}, {"f64", 4, true},
		{"i32", 4, false}, {"i32", 8, true},
		{"i64", 2, false}, {"i64", 4, true},
		{"f32", 16, true}, // unknown width still reaches -mavx via wide rule
		{"i32", 2, false},
	}
	for _, c := range cases {
		if got := simdWideLane(c.lanes, c.elem); got != c.want {
			t.Errorf("simdWideLane(%d,%s) = %v, want %v", c.lanes, c.elem, got, c.want)
		}
	}
}

// TestPhase106_TypedAddEmission checks a lane-typed @simd_add lowers to the
// karkain_simd_add_* helper (not the scalar binary_op), and the AVX flag fires
// for the 256-bit width.
func TestPhase106_TypedAddEmission(t *testing.T) {
	g := New(Config{})
	g.simdVars["a"] = "[8]f32"
	g.simdVars["b"] = "[8]f32"
	out := g.genSIMDExpr(&parser.SIMDBuiltinExpr{
		Op:   "add",
		Args: []parser.Node{&parser.Identifier{Name: "a"}, &parser.Identifier{Name: "b"}},
		Line: 3,
	})
	if want := "karkain_simd_add_f32x8(a, b)"; !strings.Contains(out, want) {
		t.Errorf("typed add should emit %q, got: %s", want, out)
	}
	if !g.simdNeedsAVX {
		t.Error("f32x8 add should demand -mavx")
	}
	if !hasFlag(g.appendAVXFlags(nil), "-mavx") {
		t.Error("appendAVXFlags should add -mavx when simdNeedsAVX")
	}
}

// TestPhase106_SseWidthNoAVX checks 128-bit widths never demand -mavx.
func TestPhase106_SseWidthNoAVX(t *testing.T) {
	g := New(Config{})
	g.simdVars["a"] = "[4]f32"
	g.simdVars["b"] = "[4]f32"
	g.genSIMDExpr(&parser.SIMDBuiltinExpr{
		Op:   "mul",
		Args: []parser.Node{&parser.Identifier{Name: "a"}, &parser.Identifier{Name: "b"}},
	})
	if g.simdNeedsAVX {
		t.Error("f32x4 mul must not demand -mavx")
	}
	if hasFlag(g.appendAVXFlags(nil), "-mavx") {
		t.Error("appendAVXFlags must not add -mavx for SSE-only width")
	}
}

// TestPhase106_SumToValue checks @simd_sum wraps the reduce in a Value factory,
// matching the element (float vs int).
func TestPhase106_SumToValue(t *testing.T) {
	g := New(Config{})
	g.simdVars["v"] = "[8]f32"
	out := g.genSIMDExpr(&parser.SIMDBuiltinExpr{Op: "sum", Args: []parser.Node{&parser.Identifier{Name: "v"}}})
	if !strings.Contains(out, "make_float(karkain_simd_sum_f32x8(v))") {
		t.Errorf("f32 sum should wrap in make_float, got: %s", out)
	}

	g2 := New(Config{})
	g2.simdVars["w"] = "[4]i32"
	out2 := g2.genSIMDExpr(&parser.SIMDBuiltinExpr{Op: "sum", Args: []parser.Node{&parser.Identifier{Name: "w"}}})
	if !strings.Contains(out2, "make_int(karkain_simd_sum_i32x4(w))") {
		t.Errorf("i32 sum should wrap in make_int, got: %s", out2)
	}
}

// TestPhase106_SplatInference checks splat width inference from (seed, lanes).
func TestPhase106_SplatInference(t *testing.T) {
	g := New(Config{})
	out := g.genSIMDExpr(&parser.SIMDBuiltinExpr{
		Op:   "splat",
		Args: []parser.Node{&parser.Float64Literal{Value: "1.5"}, &parser.IntLiteral{Value: "8"}},
	})
	if want := "karkain_simd_splat_f32x8(((float)(1.5)))"; !strings.Contains(out, want) {
		t.Errorf("float splat(8 lanes) should emit %q, got: %s", want, out)
	}
	if !g.simdNeedsAVX {
		t.Error("f32x8 splat should demand -mavx")
	}
}

// TestPhase106_ScalarFallbackPreserved checks plain-Value operands keep the
// Phase 70 scalar binary_op fallback.
func TestPhase106_ScalarFallbackPreserved(t *testing.T) {
	g := New(Config{}) // no simdVars registered -> scalar path
	out := g.genSIMDExpr(&parser.SIMDBuiltinExpr{
		Op:   "add",
		Args: []parser.Node{&parser.IntLiteral{Value: "1"}, &parser.IntLiteral{Value: "2"}},
	})
	if !strings.Contains(out, `binary_op(make_int(1), "+", make_int(2))`) {
		t.Errorf("scalar operands should keep binary_op fallback, got: %s", out)
	}
	if g.simdNeedsAVX {
		t.Error("scalar fallback must not demand -mavx")
	}
}

// TestPhase106_SIMDVarDeclTyped checks the [N]f32 declaration is emitted with a
// lane type and the width recorded for later operand resolution.
func TestPhase106_SIMDVarDeclTyped(t *testing.T) {
	g := New(Config{})
	vd := &parser.VarDeclStmt{
		Name: "a", IsSIMD: true, Type: "[8]f32",
		Value: &parser.SIMDBuiltinExpr{Op: "splat", Args: []parser.Node{&parser.Float64Literal{Value: "2.0"}, &parser.IntLiteral{Value: "8"}}},
	}
	out := g.genStatement(vd)
	if want := "karkain_f32x8 a = karkain_simd_splat_f32x8(((float)(2.0)))"; !strings.Contains(out, want) {
		t.Errorf("SIMD decl should emit %q, got:\n%s", want, out)
	}
	if got, ok := g.simdVars["a"]; !ok || got != "[8]f32" {
		t.Errorf("simdVars[a] = %q (ok=%v), want [8]f32", got, ok)
	}
	if !g.simdNeedsAVX {
		t.Error("8-lane decl should demand -mavx")
	}
}

// TestPhase106_IntVecMulScalarLoop checks integer multiply lowers lane-wise
// (no native integer vector multiply on every ISA).
func TestPhase106_IntVecMulScalarLoop(t *testing.T) {
	rt := simdRuntimeC()
	for _, want := range []string{
		"karkain_simd_mul_i32x8",       // helper present
		"ur.x[_i] = ua.x[_i] * ub.x[_i]", // scalar-loop body
	} {
		if !strings.Contains(rt, want) {
			t.Errorf("int mul runtime missing %q", want)
		}
	}
}

// TestPhase106_RuntimeContents checks the emitted runtime defines every lane
// type and the float vector helpers use GNU-style operators (the AVX gate).
func TestPhase106_RuntimeContents(t *testing.T) {
	rt := simdRuntimeC()
	for _, ty := range []string{
		"karkain_f32x4", "karkain_f32x8", "karkain_f64x2", "karkain_f64x4",
		"karkain_i32x4", "karkain_i32x8", "karkain_i64x2", "karkain_i64x4",
	} {
		if !strings.Contains(rt, ty) {
			t.Errorf("runtime missing lane type %s", ty)
		}
	}
	for _, want := range []string{
		"karkain_simd_splat_f32x8",
		"karkain_simd_add_f32x8",
		"karkain_simd_sub_f32x8",
		"karkain_simd_mul_f32x8",
		"karkain_simd_div_f32x8",
		"karkain_simd_sum_f32x8",
	} {
		if !strings.Contains(rt, want) {
			t.Errorf("runtime missing %s", want)
		}
	}
}

// TestPhase106_IdentifierLookupInGenExpr checks bare identifiers of SIMD vars
// emit as raw C names (the lane type flows through expression operands).
func TestPhase106_IdentifierLookupInGenExpr(t *testing.T) {
	g := New(Config{})
	g.simdVars["a"] = "[8]f32"
	out := g.genExpr(&parser.Identifier{Name: "a"})
	if out != "a" {
		t.Errorf("identifier of a SIMD var should emit the raw C name, got %q", out)
	}
}

func hasFlag(flags []string, want string) bool {
	for _, f := range flags {
		if f == want {
			return true
		}
	}
	return false
}