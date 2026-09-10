package codegen

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"

	"karkain/pkg/parser"
)

// Phase 106: SIMD & Vector Types — the real elementwise layer on top of the
// Phase 70 lane-vector skeleton. A "[N]f32"-style variable annotation selects
// one of the karkain_* lane types below; arithmetic then lowers to a matching
// karkain_simd_* runtime helper that is, on x86-64, an actual SIMD operation
// (SSE for 128-bit widths, AVX for 256-bit widths when the program uses them
// and the generated C is compiled with -mavx). On targets without the wide ISA
// the same helpers compile to scalar loops / GNU vector extensions, so
// correctness never depends on the hardware.

// simdElem is one of the supported element types.
type simdElem string

const (
	elemF32 simdElem = "f32"
	elemF64 simdElem = "f64"
	elemI32 simdElem = "i32"
	elemI64 simdElem = "i64"
)

// simdLanesForType canonicalizes an element name (accepts both "f32" and
// "float32", matching parser.ParseSIMDVectorType's element strings).
func simdLanesForType(elem string) simdElem {
	switch elem {
	case "float32", "f32":
		return elemF32
	case "float64", "f64":
		return elemF64
	case "int32", "i32":
		return elemI32
	case "int64", "i64":
		return elemI64
	}
	return ""
}

// simdSupportedWidth describes one lane/element combo the front end can emit.
type simdSupportedWidth struct {
	elem  simdElem
	lanes int
}

// simdSupported lists every [N]T vector width with a native lane type. These
// are exactly the widths simdVectorCType maps to karkain_* types.
var simdSupported = []simdSupportedWidth{
	{elemF32, 4}, {elemF32, 8},
	{elemF64, 2}, {elemF64, 4},
	{elemI32, 4}, {elemI32, 8},
	{elemI64, 2}, {elemI64, 4},
}

// simdSupportedLane reports whether (elem, lanes) has a native lane type.
func simdSupportedLane(elem string, lanes int) bool {
	canon := simdLanesForType(elem)
	if canon == "" {
		return false
	}
	for _, w := range simdSupported {
		if w.lanes == lanes && w.elem == canon {
			return true
		}
	}
	return false
}

// simdWideLane reports whether a width needs 256-bit AVX on x86 (Go then asks
// the C compiler to add -mavx). 128-bit widths are baseline SSE.
func simdWideLane(lanes int, elem string) bool {
	switch simdLanesForType(elem) {
	case elemF32, elemI32:
		return lanes >= 8
	case elemF64, elemI64:
		return lanes >= 4
	}
	return false
}

// simdTyName returns the Karkain lane type name for a supported width, e.g.
// ("f32", 8) -> "karkain_f32x8".
func simdTyName(elem string, lanes int) string {
	return fmt.Sprintf("karkain_%sx%d", simdLanesForType(elem), lanes)
}

// simdOpName returns the runtime helper name for an elementwise op on a
// supported width, e.g. ("add", "f32", 8) -> "karkain_simd_add_f32x8".
func simdOpName(op, elem string, lanes int) string {
	return fmt.Sprintf("karkain_simd_%s_%sx%d", op, simdLanesForType(elem), lanes)
}

// simdCElem maps an element name to its C scalar type.
func simdCElem(elem string) string {
	switch simdLanesForType(elem) {
	case elemF32:
		return "float"
	case elemF64:
		return "double"
	case elemI32:
		return "int"
	case elemI64:
		return "long long"
	}
	return "float"
}

// simdScalarCast wraps a scalar expression in a cast to the lane element type.
func simdScalarCast(elem, expr string) string {
	return fmt.Sprintf("((%s)(%s))", simdCElem(elem), expr)
}

// simdElemHint infers the element type of a bare scalar splat seed.
func simdElemHint(n parser.Node) (string, bool) {
	switch n.(type) {
	case *parser.Float64Literal:
		return "f32", true
	case *parser.IntLiteral:
		return "i32", true
	}
	return "", false
}

// simdSeedC returns the raw C scalar for a splat seed. Numeric literals carry
// their value directly (a float literal is a C float constant, not a Value);
// any other expression evaluates to a Value whose float lane is the seed.
func simdSeedC(n parser.Node, g *Generator) string {
	switch v := n.(type) {
	case *parser.IntLiteral:
		return v.Value
	case *parser.Float64Literal:
		return v.Value
	default:
		return fmt.Sprintf("(%s).floatVal", g.genExpr(n))
	}
}

// simdVectorCType maps a "[N]f32"-style lane-vector type to its C type.
//
//	[4]f32 -> karkain_f32x4 (SSE)
//	[8]f32 -> karkain_f32x8 (AVX)
//	[2]f64 -> karkain_f64x2 (SSE2)
//	[4]f64 -> karkain_f64x4 (AVX)
//
// Unknown combos fall back to an aligned scalar array so code still compiles.
func simdVectorCType(typeStr string) string {
	lanes, elem, ok := parser.ParseSIMDVectorType(typeStr)
	if !ok || lanes <= 0 {
		return "void*"
	}
	if simdSupportedLane(elem, lanes) {
		return simdTyName(elem, lanes)
	}
	switch simdLanesForType(elem) {
	case elemF32:
		return fmt.Sprintf("float[%d]", lanes)
	case elemF64:
		return fmt.Sprintf("double[%d]", lanes)
	case elemI32:
		return fmt.Sprintf("int32_t[%d]", lanes)
	case elemI64:
		return fmt.Sprintf("int64_t[%d]", lanes)
	}
	return fmt.Sprintf("%s[%d]", elem, lanes)
}

// simdLanes resolves a SIMDBuiltinExpr to a supported (lanes, elem) when the
// operands are lane-typed. splat infers the width from its seed and lane
// literal; everything else looks up the first operand's declared vector type.
func (g *Generator) simdLanes(se *parser.SIMDBuiltinExpr) (int, string, bool) {
	if se == nil {
		return 0, "", false
	}
	for _, op := range []string{"add", "sub", "mul", "div", "sum"} {
		if se.Op == op {
			if len(se.Args) < 1 {
				return 0, "", false
			}
			id, ok := se.Args[0].(*parser.Identifier)
			if !ok {
				return 0, "", false
			}
			t, ok := g.simdVars[id.Name]
			if !ok {
				return 0, "", false
			}
			lanes, elem, ok2 := parser.ParseSIMDVectorType(t)
			if !ok2 || !simdSupportedLane(elem, lanes) {
				return 0, "", false
			}
			return lanes, elem, true
		}
	}
	if se.Op == "splat" {
		if len(se.Args) < 2 {
			return 0, "", false
		}
		li, ok := se.Args[1].(*parser.IntLiteral)
		if !ok {
			return 0, "", false
		}
		lanes, err := strconv.Atoi(li.Value)
		if err != nil || lanes <= 0 {
			return 0, "", false
		}
		elem, ok := simdElemHint(se.Args[0])
		if !ok || !simdSupportedLane(elem, lanes) {
			return 0, "", false
		}
		return lanes, elem, true
	}
	return 0, "", false
}

// genSIMDTyped emits a SIMD op against a known (lanes, elem) width. The result
// is a raw C expression of the lane type — or, for sum, a Value (make_float /
// make_int) so the vector can round-trip into the scalar Value world.
func (g *Generator) genSIMDTyped(se *parser.SIMDBuiltinExpr, lanes int, elem string) string {
	if simdWideLane(lanes, elem) {
		g.simdNeedsAVX = true
	}
	switch se.Op {
	case "splat":
		if len(se.Args) < 1 {
			return "make_int(0)"
		}
		return fmt.Sprintf("%s(%s)", simdOpName("splat", elem, lanes),
			simdScalarCast(elem, simdSeedC(se.Args[0], g)))
	case "sum":
		if len(se.Args) < 1 {
			return "make_int(0)"
		}
		fn := fmt.Sprintf("%s(%s)", simdOpName("sum", elem, lanes), g.genExpr(se.Args[0]))
		if elem == "i32" || elem == "i64" {
			return fmt.Sprintf("make_int(%s)", fn)
		}
		return fmt.Sprintf("make_float(%s)", fn)
	case "add", "sub", "mul", "div":
		if len(se.Args) < 2 {
			return "make_int(0)"
		}
		return fmt.Sprintf("%s(%s, %s)", simdOpName(se.Op, elem, lanes),
			g.genExpr(se.Args[0]), g.genExpr(se.Args[1]))
	}
	return g.genSIMDExpr(se)
}

// genSIMDDecl emits a fixed-lane SIMD variable declaration. It records the
// declared vector type so later operands resolve their lane width, and
// substitutes a lane-typed initializer when the value is a SIMD builtin.
func (g *Generator) genSIMDDecl(vd *parser.VarDeclStmt) string {
	g.simdVars[vd.Name] = vd.Type
	align := ""
	if vd.Align > 0 {
		align = fmt.Sprintf(" _Alignas(%d)", vd.Align)
	}
	cType := simdVectorCType(vd.Type)
	lanes, elem, ok := parser.ParseSIMDVectorType(vd.Type)
	if !ok || lanes <= 0 || !simdSupportedLane(elem, lanes) {
		return fmt.Sprintf("\t%s%s %s = %s;\n", cType, align, vd.Name, g.genExpr(vd.Value))
	}
	if simdWideLane(lanes, elem) {
		g.simdNeedsAVX = true
	}
	val := g.genExpr(vd.Value)
	if se, okv := vd.Value.(*parser.SIMDBuiltinExpr); okv {
		val = g.genSIMDTyped(se, lanes, elem)
	}
	return fmt.Sprintf("\t%s%s %s = %s;\n", cType, align, vd.Name, val)
}

// appendAVXFlags adds -mavx to a GCC/Clang flag list when the program uses a
// 256-bit lane width. MSVC needs /arch:AVX and is skipped (SIMD on MSVC is out
// of scope for Phase 106). x86 host only: -mavx is meaningless on other ISAs.
func (g *Generator) appendAVXFlags(flags []string) []string {
	if g.simdNeedsAVX && (runtime.GOARCH == "amd64" || runtime.GOARCH == "386") {
		flags = append(flags, "-mavx")
	}
	return flags
}

// simdRuntimeC emits the portable C runtime that backs the karkain_simd_* ops.
// Helpers only use GNU-friendly vector operators, compound literals and union
// puns, so one body serves the x86 intrinsics typedefs and the ARM GNU-vector
// typedefs alike. 128-bit widths are SSE baseline on x86; the 256-bit widths
// are AVX when -mavx is passed and fall back to GNU vector_size(32) types
// otherwise. Integer mul/div have no native ISA instruction family, so those
// helpers reduce lane-wise (correct on every target).
func simdRuntimeC() string {
	var b strings.Builder
	b.WriteString(`
// ============================================================
// Phase 106: SIMD & Vector Types runtime
//   Karkain-owned lane-vector layer. "[N]f32"-style variable annotations
//   select one of the karkain_* lane types below; elementwise arithmetic
//   calls the matching karkain_simd_* helper. On x86-64 the 128-bit widths
//   are SSE (baseline) and the 256-bit widths are AVX (active when the
//   generated C is compiled with -mavx). Other targets use GNU vector
//   extensions; integer multiply/divide and unsupported ISAs reduce through
//   scalar loops, so correctness never depends on the hardware.
#if defined(__x86_64__) || defined(__i386__) || defined(_M_X64) || defined(_M_IX86)
#include <immintrin.h>
typedef __m128  karkain_f32x4;
typedef __m128d karkain_f64x2;
#if defined(__AVX__) || defined(__AVX2__)
typedef __m256  karkain_f32x8;
typedef __m256d karkain_f64x4;
#else
typedef float   karkain_f32x8 __attribute__((vector_size(32)));
typedef double  karkain_f64x4 __attribute__((vector_size(32)));
#endif
#else
typedef float   karkain_f32x4 __attribute__((vector_size(16)));
typedef double  karkain_f64x2 __attribute__((vector_size(16)));
typedef float   karkain_f32x8 __attribute__((vector_size(32)));
typedef double  karkain_f64x4 __attribute__((vector_size(32)));
#endif
typedef int        karkain_i32x4 __attribute__((vector_size(16)));
typedef int        karkain_i32x8 __attribute__((vector_size(32)));
typedef long long  karkain_i64x2 __attribute__((vector_size(16)));
typedef long long  karkain_i64x4 __attribute__((vector_size(32)));
`)
	for _, w := range simdSupported {
		ct := simdCElem(string(w.elem))
		ty := simdTyName(string(w.elem), w.lanes)
		tag := string(w.elem)
		lanes := w.lanes
		seeds := strings.TrimSuffix(strings.Repeat("v, ", lanes), ", ")

		fmt.Fprintf(&b, "\nstatic inline %s karkain_simd_splat_%sx%d(%s v) {\n", ty, tag, lanes, ct)
		fmt.Fprintf(&b, "    return (%s){%s};\n}\n", ty, seeds)

		for _, op := range []string{"add", "sub"} {
			sym := "+"
			if op == "sub" {
				sym = "-"
			}
			fmt.Fprintf(&b, "static inline %s karkain_simd_%s_%sx%d(%s a, %s b) {\n    return a %s b;\n}\n", ty, op, tag, lanes, ty, ty, sym)
		}

		intMulDiv := tag == "i32" || tag == "i64"
		for _, op := range []string{"mul", "div"} {
			sym := "*"
			if op == "div" {
				sym = "/"
			}
			fmt.Fprintf(&b, "static inline %s karkain_simd_%s_%sx%d(%s a, %s b) {\n", ty, op, tag, lanes, ty, ty)
			if intMulDiv {
				fmt.Fprintf(&b, "    union { %s v; %s x[%d]; } ua, ub, ur;\n", ty, ct, lanes)
				fmt.Fprintf(&b, "    ua.v = a; ub.v = b;\n")
				fmt.Fprintf(&b, "    for (int _i = 0; _i < %d; _i++) ur.x[_i] = ua.x[_i] %s ub.x[_i];\n", lanes, sym)
				fmt.Fprintf(&b, "    return ur.v;\n}\n")
			} else {
				fmt.Fprintf(&b, "    return a %s b;\n}\n", sym)
			}
		}

		fmt.Fprintf(&b, "static inline %s karkain_simd_sum_%sx%d(%s v) {\n", ct, tag, lanes, ty)
		fmt.Fprintf(&b, "    union { %s v; %s a[%d]; } u;\n", ty, ct, lanes)
		fmt.Fprintf(&b, "    u.v = v; %s s = u.a[0];\n", ct)
		fmt.Fprintf(&b, "    for (int _i = 1; _i < %d; _i++) s += u.a[_i];\n", lanes)
		fmt.Fprintf(&b, "    return s;\n}\n")
	}
	return b.String()
}