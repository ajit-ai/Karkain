package parser

import (
	"testing"
)

// TestParseSIMDVectorType checks the "[N]T" lane-vector literal syntax in a var
// declaration, plus the trailing @aligned(N) cache-line attribute.
func TestParseSIMDVectorType(t *testing.T) {
	prog := parseSource(t, `
func main() {
    let a [4]f32 = @simd_splat(1.0, 4)
    let b [2]f64 @aligned(64) = @simd_splat(2.0, 2)
    let c [8]i32 = 0
    print(a)
    print(b)
    print(c)
}
`)
	if prog == nil {
		t.Fatal("ParseProgram returned nil")
	}
	var sawA, sawB, sawC bool
	for _, stmt := range prog.Statements {
		fn, ok := stmt.(*FuncDecl)
		if !ok || fn.Name != "main" {
			continue
		}
		for _, st := range fn.Body {
			vd, ok := st.(*VarDeclStmt)
			if !ok {
				continue
			}
			switch vd.Name {
			case "a":
				if !vd.IsSIMD {
					t.Errorf("a: expected IsSIMD=true, got false")
				}
				if l, elem, ok := ParseSIMDVectorType(vd.Type); !ok || l != 4 || elem != "f32" {
					t.Errorf("a: expected [4]f32, got lanes=%d elem=%s ok=%v", l, elem, ok)
				}
				if vd.Align != 0 {
					t.Errorf("a: expected no alignment, got %d", vd.Align)
				}
				sawA = true
			case "b":
				if !vd.IsSIMD {
					t.Errorf("b: expected IsSIMD=true, got false")
				}
				if l, elem, ok := ParseSIMDVectorType(vd.Type); !ok || l != 2 || elem != "f64" {
					t.Errorf("b: expected [2]f64, got lanes=%d elem=%s ok=%v", l, elem, ok)
				}
				if vd.Align != 64 {
					t.Errorf("b: expected Align=64, got %d", vd.Align)
				}
				sawB = true
			case "c":
				if !vd.IsSIMD {
					t.Errorf("c: expected IsSIMD=true, got false")
				}
				if l, elem, ok := ParseSIMDVectorType(vd.Type); !ok || l != 8 || elem != "i32" {
					t.Errorf("c: expected [8]i32, got lanes=%d elem=%s ok=%v", l, elem, ok)
				}
				sawC = true
			}
		}
	}
	if !sawA || !sawB || !sawC {
		t.Errorf("did not observe all SIMD vars: a=%v b=%v c=%v", sawA, sawB, sawC)
	}
}

// TestParseAtomicOp checks the @atomic_* builtins and their memory orderings.
func TestParseAtomicOp(t *testing.T) {
	prog := parseSource(t, `
func main() {
    let x = @atomic_load(p, "relaxed")
    let y = @atomic_fetch_add(p, 1, "acq_rel")
    let z = @atomic_fetch_sub(p, 1)
    let w = @atomic_store(v, 1, "release")
    let c = @atomic_cas(p, 0, 1, "seq_cst")
    print(x)
    print(y)
    print(z)
    print(w)
    print(c)
}
`)
	if prog == nil {
		t.Fatal("ParseProgram returned nil")
	}
	fn := prog.Statements[0].(*FuncDecl)
	expectOrder := map[string]string{
		"load":  "relaxed",
		"y":     "acq_rel",
		"z":     "seq_cst",
		"w":     "release",
		"c":     "seq_cst",
	}
	for _, st := range fn.Body {
		vd, ok := st.(*VarDeclStmt)
		if !ok {
			continue
		}
		at, ok := vd.Value.(*AtomicOp)
		if !ok {
			t.Errorf("%s: expected AtomicOp, got %T", vd.Name, vd.Value)
			continue
		}
		if want, ok := expectOrder[vd.Name]; ok && at.Order != want {
			t.Errorf("%s: expected order %q, got %q", vd.Name, want, at.Order)
		}
	}
}

// TestParseSIMDBuiltinSplat checks @simd_splat parses to a SIMDBuiltinExpr.
func TestParseSIMDBuiltinSplat(t *testing.T) {
	prog := parseSource(t, `
func main() {
    let v [4]f32 = @simd_splat(1.5, 4)
    print(v)
}
`)
	if prog == nil {
		t.Fatal("ParseProgram returned nil")
	}
	fn := prog.Statements[0].(*FuncDecl)
	vd := fn.Body[0].(*VarDeclStmt)
	if !vd.IsSIMD {
		t.Fatalf("expected SIMD var, got IsSIMD=false")
	}
	be, ok := vd.Value.(*SIMDBuiltinExpr)
	if !ok {
		t.Fatalf("expected SIMDBuiltinExpr, got %T", vd.Value)
	}
	if be.Op != "splat" {
		t.Errorf("expected op splat, got %q", be.Op)
	}
	if len(be.Args) != 2 {
		t.Errorf("expected 2 args, got %d", len(be.Args))
	}
}

// TestParseSIMDVectorTypeHelper checks the helper directly, including the
// normalization of float32->f32 and the rejection of unsupported types.
func TestParseSIMDVectorTypeHelper(t *testing.T) {
	cases := []struct {
		in      string
		lanes   int
		elem    string
		wantOK  bool
	}{
		{"[4]f32", 4, "f32", true},
		{"[8]f32", 8, "f32", true},
		{"[2]f64", 2, "f64", true},
		{"[4]float32", 4, "float32", true},
		{"[4]i32", 4, "i32", true},
		{"[4]i64", 4, "i64", true},
		{"[0]f32", 0, "f32", true},
		{"[4]", 0, "", false},
		{"[]f32", 0, "", false},
		{"f32", 0, "", false},
		{"[4]u8", 0, "", false},
		{"[abc]f32", 0, "", false},
	}
	for _, c := range cases {
		lanes, elem, ok := ParseSIMDVectorType(c.in)
		if ok != c.wantOK {
			t.Errorf("%s: ok=%v want %v", c.in, ok, c.wantOK)
			continue
		}
		if ok {
			if lanes != c.lanes || elem != c.elem {
				t.Errorf("%s: got (lanes=%d, elem=%s), want (%d, %s)", c.in, lanes, elem, c.lanes, c.elem)
			}
			if got := LaneCount(c.in); got != c.lanes {
				t.Errorf("LaneCount(%s)=%d want %d", c.in, got, c.lanes)
			}
		}
	}
	// A non-vector type must satisfy nothing of the helper.
	if _, _, ok := ParseSIMDVectorType("int"); ok {
		t.Error("expected 'int' to be rejected")
	}
}

// TestAlignmentAttrOnly checks @aligned is accepted on a plain (non-SIMD) var.
func TestAlignmentAttrOnly(t *testing.T) {
	prog := parseSource(t, `
func main() {
    let counter [4]i32 = 0
    let ok @aligned(128) = true
    print(counter)
    print(ok)
}
`)
	if prog == nil {
		t.Fatal("ParseProgram returned nil")
	}
	fn := prog.Statements[0].(*FuncDecl)
	vd := fn.Body[1].(*VarDeclStmt)
	if vd.Align != 128 {
		t.Errorf("expected Align=128, got %d", vd.Align)
	}
}

// TestParseAtomicOrderingNormalization verifies default ordering is seq_cst.
func TestParseAtomicOrderingNormalization(t *testing.T) {
	prog := parseSource(t, `
func main() {
    let a = @atomic_load(p)
    print(a)
}
`)
	if prog == nil {
		t.Fatal("ParseProgram returned nil")
	}
	fn := prog.Statements[0].(*FuncDecl)
	vd := fn.Body[0].(*VarDeclStmt)
	at, ok := vd.Value.(*AtomicOp)
	if !ok {
		t.Fatalf("expected AtomicOp, got %T", vd.Value)
	}
	if at.Order != "seq_cst" {
		t.Errorf("expected default order seq_cst, got %q", at.Order)
	}
}
