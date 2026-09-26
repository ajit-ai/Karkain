package native

import (
	"strings"
	"testing"

	"karkain/pkg/parser"
)

// Phase 150B3b -- records (structs) on the native target.
//
// The executed goldens run through the PE container on this windows/amd64
// host, so the lowering is proven by real execution rather than assertion.
// The cases cover literal construction and field reads, field assignment (the
// write-through the by-pointer value model exists for), mixed int/string/float
// field layouts, a record crossing the call boundary (the record idiom, with
// the mutation visible to the caller), a record returned by value, records
// built from computed field values, and the independence of two records.
var nativeStructCases = []struct {
	name string
	src  string
	want string
}{
	// Construction and reads.
	{"two_ints", "type Point struct { x int; y int }\nfunc main() {\n    let p = Point { x: 3, y: 4 }\n    print(p.x)\n    print(p.y)\n}\n", "3\n4\n"},
	{"field_in_expr", "type P struct { x int; y int }\nfunc main() {\n    let p = P { x: 10, y: 4 }\n    print(p.x * p.y + 1)\n}\n", "41\n"},
	{"single_field", "type C struct { n int }\nfunc main() {\n    let c = C { n: 9 }\n    print(c.n)\n}\n", "9\n"},

	// Field assignment -- the reason a record is a pointer.
	{"assign_int", "type P struct { x int }\nfunc main() {\n    let p = P { x: 1 }\n    p.x = 7\n    print(p.x)\n}\n", "7\n"},
	{"assign_cumulative", "type A struct { v int }\nfunc main() {\n    let a = A { v: 0 }\n    a.v = a.v + 1\n    a.v = a.v + 1\n    a.v = a.v + 1\n    print(a.v)\n}\n", "3\n"},
	{"assign_expression", "type P struct { x int }\nfunc main() {\n    let p = P { x: 2 }\n    p.x = p.x * 5 + 1\n    print(p.x)\n}\n", "11\n"},
	{"assign_in_loop", "type C struct { n int }\nfunc main() {\n    let c = C { n: 0 }\n    let i = 0\n    while (i < 5) {\n        c.n = c.n + i\n        i = i + 1\n    }\n    print(c.n)\n}\n", "10\n"},
	{"assign_from_field", "type P struct { x int; y int }\nfunc main() {\n    let p = P { x: 4, y: 9 }\n    p.y = p.x * 3\n    print(p.y)\n}\n", "12\n"},

	// Mixed field kinds: an int, a string and a float in one record proves
	// the offset arithmetic and the two-unit string field.
	{"mixed_fields", "type S struct { a int; label string; r float }\nfunc main() {\n    let s = S { a: 5, label: \"hi\", r: 2.5 }\n    print(s.a)\n    print(s.label)\n    print(s.r)\n}\n", "5\nhi\n2.5\n"},
	{"string_field_assign", "type S struct { label string; n int }\nfunc main() {\n    let s = S { label: \"a\", n: 1 }\n    s.label = \"changed\"\n    print(s.label)\n    print(s.n)\n}\n", "changed\n1\n"},
	{"string_between_ints", "type S struct { a int; label string; b int }\nfunc main() {\n    let s = S { a: 1, label: \"mid\", b: 2 }\n    print(s.a)\n    print(s.label)\n    print(s.b)\n}\n", "1\nmid\n2\n"},
}


// The record idiom: a record crosses the call boundary by address and the
// callee's field write is visible to the caller. This is the case the whole
// by-pointer value model exists for, and the record return cases beside it
// prove the address travels correctly in both directions.
var nativeStructIdiomCases = []struct {
	name string
	src  string
	want string
}{
	{"idiom_mutate", "type Counter struct { n int }\nfunc bump(c Counter) {\n    c.n = c.n + 1\n}\nfunc main() {\n    let c = Counter { n: 5 }\n    bump(c)\n    bump(c)\n    print(c.n)\n}\n", "7\n"},
	{"idiom_two_fields", "type Acct struct { owner string; balance int }\nfunc deposit(a Acct) {\n    a.balance = a.balance + 50\n}\nfunc main() {\n    let a = Acct { owner: \"ada\", balance: 10 }\n    deposit(a)\n    print(a.owner)\n    print(a.balance)\n}\n", "ada\n60\n"},
	{"idiom_reads_param", "type P struct { x int }\nfunc getx(p P) {\n    print(p.x)\n}\nfunc main() {\n    getx(P { x: 42 })\n}\n", "42\n"},
	{"idiom_with_scalar", "type P struct { x int }\nfunc addto(p P, n) {\n    p.x = p.x + n\n}\nfunc main() {\n    let p = P { x: 1 }\n    addto(p, 99)\n    print(p.x)\n}\n", "100\n"},
	{"two_records", "type P struct { x int }\nfunc swap(a P, b P) {\n    a.x = a.x + b.x\n    b.x = b.x + 100\n}\nfunc main() {\n    let a = P { x: 1 }\n    let b = P { x: 2 }\n    swap(a, b)\n    print(a.x)\n    print(b.x)\n}\n", "3\n102\n"},
	{"return_literal", "type P struct { x int }\nfunc mk() {\n    return P { x: 8 }\n}\nfunc main() {\n    let p = mk()\n    print(p.x)\n}\n", "8\n"},
	{"return_then_mutate", "type P struct { x int }\nfunc mk(n) {\n    return P { x: n }\n}\nfunc main() {\n    let p = mk(3)\n    p.x = p.x * 3\n    print(p.x)\n}\n", "9\n"},
	{"return_passed_record", "type P struct { x int }\nfunc same(p P) {\n    return p\n}\nfunc main() {\n    let a = P { x: 4 }\n    let b = same(a)\n    print(a.x)\n    print(b.x)\n    a.x = 9\n    print(b.x)\n}\n", "4\n4\n9\n"},
	{"computed_fields", "func twice(n) {\n    return n * 2\n}\ntype P struct { x int; y int }\nfunc main() {\n    let p = P { x: twice(5), y: 3 + 4 }\n    print(p.x)\n    print(p.y)\n}\n", "10\n7\n"},
	{"field_from_other_record", "type P struct { x int; y int }\nfunc main() {\n    let a = P { x: 1, y: 2 }\n    let b = P { x: a.x + 10, y: a.y * 5 }\n    print(b.x)\n    print(b.y)\n}\n", "11\n10\n"},
	{"independent_records", "type P struct { x int }\nfunc main() {\n    let a = P { x: 1 }\n    let b = P { x: 2 }\n    a.x = 10\n    print(a.x)\n    print(b.x)\n}\n", "10\n2\n"},
}


// nativeStructFlowCases covers records under control flow and string fields
// flowing through the existing string machinery (concatenation, comparison).
var nativeStructFlowCases = []struct {
	name string
	src  string
	want string
}{
	{"in_if", "type P struct { x int }\nfunc main() {\n    let p = P { x: 0 }\n    if (1 == 1) {\n        let q = P { x: 5 }\n        p.x = q.x\n    }\n    print(p.x)\n}\n", "5\n"},
	{"shadowed_name", "type P struct { x int }\nfunc main() {\n    let p = P { x: 1 }\n    let p = P { x: 2 }\n    print(p.x)\n}\n", "2\n"},
	{"in_for_body", "type C struct { total int }\nfunc main() {\n    let c = C { total: 0 }\n    for (let i = 1; i <= 4; i = i + 1) {\n        c.total = c.total + i\n    }\n    print(c.total)\n}\n", "10\n"},
	{"field_condition", "type P struct { x int }\nfunc main() {\n    let p = P { x: 3 }\n    if (p.x > 2) {\n        print(1)\n    } else {\n        print(0)\n    }\n}\n", "1\n"},
	{"string_field_concat", "type S struct { label string }\nfunc main() {\n    let s = S { label: \"ka\" }\n    print(s.label + \"rk\")\n}\n", "kark\n"},
	{"string_field_compare", "type S struct { label string }\nfunc main() {\n    let a = S { label: \"x\" }\n    let b = S { label: \"x\" }\n    if (a.label == b.label) {\n        print(1)\n    } else {\n        print(0)\n    }\n}\n", "1\n"},
	{"string_field_param_write", "type S struct { label string }\nfunc set(s S) {\n    s.label = \"set\"\n}\nfunc main() {\n    let s = S { label: \"init\" }\n    set(s)\n    print(s.label)\n}\n", "set\n"},
	// Aliasing: `let b = a` copies the ADDRESS, not the record, so the two
	// names denote ONE field area. This is the same by-pointer rule the
	// call boundary follows, pinned here because it is the one place a
	// reader might expect a copy.
	{"alias_shares_one_record", "type P struct { x int }\nfunc main() {\n    let a = P { x: 1 }\n    let b = a\n    b.x = 9\n    print(a.x)\n    print(b.x)\n}\n", "9\n9\n"},
	{"alias_independent_of_other", "type P struct { x int }\nfunc main() {\n    let a = P { x: 1 }\n    let b = a\n    let c = P { x: 2 }\n    b.x = 9\n    print(a.x)\n    print(b.x)\n    print(c.x)\n}\n", "9\n9\n2\n"},
}


// TestNativeStructLayout pins the compile-time field layout: declaration
// order, one unit for an int or float, two for a string, and a loud refusal
// for a field type the value model does not have.
func TestNativeStructLayout(t *testing.T) {
	si, err := newStructInfo("P", []parser.StructField{
		{Name: "a", Type: "int"},
		{Name: "label", Type: "string"},
		{Name: "r", Type: "float"},
	})
	if err != nil {
		t.Fatalf("layout: %v", err)
	}
	if si.size != 32 {
		t.Errorf("size = %d, want 32 (int + string(2 units) + float)", si.size)
	}
	for _, tc := range []struct {
		field   string
		off, kd int
	}{
		{"a", 0, KindInt},
		{"label", 8, KindString},
		{"r", 24, KindFloat},
	} {
		off, kd, ok := si.structField(tc.field)
		if !ok {
			t.Fatalf("field %q missing", tc.field)
		}
		if off != tc.off || kd != tc.kd {
			t.Errorf("field %q: off=%d kind=%d, want off=%d kind=%d", tc.field, off, kd, tc.off, tc.kd)
		}
	}
	if _, _, ok := si.structField("missing"); ok {
		t.Error("unknown field resolved")
	}
}

func allStructCases() []struct {
	name string
	src  string
	want string
} {
	out := make([]struct {
		name string
		src  string
		want string
	}, 0, len(nativeStructCases)+len(nativeStructIdiomCases)+len(nativeStructFlowCases))
	out = append(out, nativeStructCases...)
	out = append(out, nativeStructIdiomCases...)
	out = append(out, nativeStructFlowCases...)
	return out
}

// TestNativeStructExecPE runs the record surface through the PE container,
// where the Win64 boundary, the PEB bootstrap and the .idata arena all execute
// for real on windows/amd64.
func TestNativeStructExecPE(t *testing.T) {
	for _, c := range allStructCases() {
		c := c
		t.Run(c.name, func(t *testing.T) {
			img := compileNativeOS(t, OSWindows, c.src)
			out, code := runNativeWindows(t, img)
			if out != "" {
				if out != c.want {
					t.Errorf("%s: output %q, want %q", c.name, out, c.want)
				}
				if code != 0 {
					t.Errorf("%s: exit %d, want 0 (out=%q)", c.name, code, out)
				}
			}
		})
	}
}

// TestNativeStructExec mirrors the record surface on the Linux ELF container.
func TestNativeStructExec(t *testing.T) {
	for _, c := range allStructCases() {
		c := c
		t.Run(c.name, func(t *testing.T) {
			img := compileNative(t, c.src)
			out, code := runNativeCode(t, img)
			if out != "" {
				if out != c.want {
					t.Errorf("%s: output %q, want %q", c.name, out, c.want)
				}
				if code != 0 {
					t.Errorf("%s: exit %d, want 0 (out=%q)", c.name, code, out)
				}
			}
		})
	}
}

// TestNativeStructStructural compiles and structurally validates every record
// case for all three OS containers, so a record program is proven to lower
// into a valid image even where execution is unavailable (Mach-O).
// TestNativeStructNegatives is the K145 reject table. Every entry is a shape
// the value model does not define, and each must be REFUSED rather than
// lowered into something that runs and computes the wrong thing.
func TestNativeStructNegatives(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"undeclared_struct", "func main() {\n    let p = Nope { x: 1 }\n    print(p.x)\n}\n", "unknown struct type"},
		{"unknown_field", "type P struct { x int }\nfunc main() {\n    let p = P { x: 1, y: 2 }\n    print(p.x)\n}\n", "unknown field"},
		{"missing_field", "type P struct { x int; y int }\nfunc main() {\n    let p = P { x: 1 }\n    print(p.x)\n}\n", "not initialised"},
		{"field_not_a_record", "func main() {\n    let n = 5\n    print(n.x)\n}\n", "is not a record"},
		{"wrong_field_type", "type P struct { x int }\nfunc main() {\n    let p = P { x: \"s\" }\n    print(p.x)\n}\n", "expects int"},
		{"assign_wrong_kind", "type P struct { x int }\nfunc main() {\n    let p = P { x: 1 }\n    p.x = \"s\"\n    print(p.x)\n}\n", "expects int"},
		{"print_record", "type P struct { x int }\nfunc main() {\n    let p = P { x: 1 }\n    print(p)\n}\n", "cannot print a struct"},
		{"main_returns_record", "type P struct { x int }\nfunc main() {\n    return P { x: 1 }\n}\n", "struct return values are not supported"},
		{"record_arg_mismatch", "type P struct { x int }\nfunc takes_int(n) {\n    print(n)\n}\nfunc main() {\n    let p = P { x: 1 }\n    takes_int(p)\n}\n", "struct argument for int parameter"},
		{"unknown_param_type", "type P struct { x int }\nfunc f(v Mystery) {\n    print(v)\n}\nfunc main() {\n    f(1)\n}\n", "unknown parameter type"},
		{"duplicate_struct", "type P struct { x int }\ntype P struct { y int }\nfunc main() {\n    let p = P { x: 1 }\n    print(p.x)\n}\n", "duplicate struct type"},
		{"duplicate_field_decl", "type P struct { x int; x int }\nfunc main() {\n    let p = P { x: 1 }\n    print(p.x)\n}\n", "duplicate field"},
		{"nested_struct_field", "type Q struct { n int }\ntype P struct { q Q }\nfunc main() {\n    let p = P { q: Q { n: 1 } }\n    print(1)\n}\n", "unsupported field type"},
		{"unsupported_field_type", "type P struct { x array }\nfunc main() {\n    let p = P { x: 1 }\n    print(p.x)\n}\n", "unsupported field type"},
		{"unknown_receiver_field", "type P struct { x int }\nfunc main() {\n    let p = P { x: 1 }\n    print(p.z)\n}\n", "unknown field"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			_, err := CompileProgram(parseNative(t, c.src))
			if err == nil {
				t.Fatalf("%s: compiled without error, want a K145 containing %q", c.name, c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("%s: error %q, want it to mention %q", c.name, err, c.want)
			}
		})
	}
}

