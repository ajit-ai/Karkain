package cli

// Phase 151A Step 9h - records (structs) through the real AST driver.
//
// Records are the last big gap in the native value model: a record is ONE 8-byte
// unit whose value IS the address of a compile-time-sized field area reserved in
// the frame, so field reads and writes go through a base the driver RELOADS from
// the frame before every access (never carried across an expression), and nothing
// is allocated -- which is exactly what keeps every pre-9h image byte-identical.
//
// WHY THIS GATE EXISTS EVEN THOUGH THE CODE LANDED FIRST. The three commits that
// shipped record lowering (7d6db10, f64d394, 82bb1f3) touched ONLY
// src/compiler/native_value.kark -- no gate, no CI entry. That is the exact class
// the repo's "a gate is a test file PLUS a CI entry" companion rule exists to
// prevent: production lowering with nothing that fails when it regresses. This
// file closes that gap, and it also pins the one real defect the lowering had.
//
// THE DEFECT THIS GATE PINS (the 9h-DEFECT-FIX). natNativePlan previously opened
// with `let tbl = natStructTable(stmts)`, rebuilding the struct table from MAIN'S
// BODY. Top-level `type X struct` declarations live at PROGRAM level, never in
// main's body, so the rebuild found nothing, natStructSizeOf returned -1, the
// record's field area reserved ZERO bytes, and the next local aliased the first
// field. Measured pre-fix: `Point{x,y}` + `let n = 5` printed 5/4/5 (n aliased
// p.x) and every record frame was 624 regardless of field area. The fix makes the
// planner use the table natNativeProgram built from the PROGRAM statements. The
// `record_and_scalar` case below is the load-bearing one: it asserts 3/4/5 AND a
// 640-byte frame, and BOTH layers fail on the unfixed planner (which would give
// 624 and 5/4/5), so the frame layer and the execution layer each independently
// catch the regression.
//
// WHAT IS PROVEN, by EXECUTING the kcc-produced PE and comparing exact bytes:
//   1. int fields construct and read back;
//   2. int field assignment (literal and from a local);
//   3. a string field between two ints stores/reads TWO units (ptr, len);
//   4. a string field can be reassigned;
//   5. a record and a scalar coexist -- the anti-aliasing / WIP-fix layer;
//   6. two independent records do not share a field area.
//
// NOT IN THIS SLICE, each refused BY NAME (not mis-lowered): printing a record as
// a whole, an unknown field or type, a nested-struct field, assigning a wrong kind
// to a field, aliasing a record with `let b = a`, and integer arithmetic on record
// fields. Each refusal is asserted, so narrowing the subset stays visible.

import (
	"os"
	"strings"
	"testing"
)

// nat9hCase is one record program and the exact observable result it must produce.
// localBytes is the locals-region size, derived independently in the test from the
// value model's documented rule: 8 bytes for a record's value slot PLUS 8 bytes
// per field unit (int/float 1 unit, string 2), then 8 per scalar local. Zero means
// the frame is not asserted for this case.
type nat9hCase struct {
	name       string
	src        string
	wantOut    []byte
	wantExit   int
	localBytes int
}

var nat9hCases = []nat9hCase{
	{
		// The smallest record: one int field. localBytes = 8 (slot) + 8 (field) = 16.
		name:    "single_field",
		src:     "type C struct { n int }\nfunc main() {\n    let c = C { n: 9 }\n    print(c.n)\n}\n",
		wantOut: []byte("9\n"), wantExit: 0, localBytes: 16,
	},
	{
		// Two int fields. localBytes = 8 + 16 = 24. Proves fields are laid out in
		// declaration order at 8-byte strides, not collapsed.
		name:    "two_ints",
		src:     "type Point struct { x int; y int }\nfunc main() {\n    let p = Point { x: 3, y: 4 }\n    print(p.x)\n    print(p.y)\n}\n",
		wantOut: []byte("3\n4\n"), wantExit: 0, localBytes: 24,
	},
	{
		// Int field assignment from a literal. localBytes = 8 + 8 = 16.
		name:    "assign_int",
		src:     "type P struct { x int }\nfunc main() {\n    let p = P { x: 1 }\n    p.x = 7\n    print(p.x)\n}\n",
		wantOut: []byte("7\n"), wantExit: 0, localBytes: 16,
	},
	{
		// Int field assignment from a LOCAL. localBytes = 16 (record) + 8 (v) = 24.
		name:    "assign_from_local",
		src:     "type P struct { x int }\nfunc main() {\n    let p = P { x: 1 }\n    let v = 9\n    p.x = v\n    print(p.x)\n}\n",
		wantOut: []byte("9\n"), wantExit: 0, localBytes: 24,
	},
	{
		// A string field BETWEEN two ints. This is the sharpest layout case: the
		// string's length unit sits at its slot+8, exactly where an int field's own
		// value or the next field would sit if the two-unit string were sized as
		// one. localBytes = 8 + (8 + 16 + 8) = 40.
		name:    "string_between_ints",
		src:     "type S struct { a int; label string; b int }\nfunc main() {\n    let s = S { a: 1, label: \"mid\", b: 2 }\n    print(s.a)\n    print(s.label)\n    print(s.b)\n}\n",
		wantOut: []byte("1\nmid\n2\n"), wantExit: 0, localBytes: 40,
	},
	{
		// A string field reassigned to a new literal. localBytes = 8 + (16 + 8) = 32.
		name:    "string_field_assign",
		src:     "type S struct { label string; n int }\nfunc main() {\n    let s = S { label: \"a\", n: 1 }\n    s.label = \"changed\"\n    print(s.label)\n    print(s.n)\n}\n",
		wantOut: []byte("changed\n1\n"), wantExit: 0, localBytes: 32,
	},
	{
		// THE WIP-FIX CASE. A record and a scalar in one program. Pre-fix the field
		// area was zero bytes, so `n` aliased `p.x` and this printed 5/4/5 with a
		// 624-byte frame. Fixed it prints 3/4/5 with a 640-byte frame. Both the
		// stdout layer and the frame layer below assert the fixed numbers.
		// localBytes = 24 (Point) + 8 (n) = 32.
		name:    "record_and_scalar",
		src:     "type Point struct { x int; y int }\nfunc main() {\n    let p = Point { x: 3, y: 4 }\n    let n = 5\n    print(p.x)\n    print(p.y)\n    print(n)\n}\n",
		wantOut: []byte("3\n4\n5\n"), wantExit: 0, localBytes: 32,
	},
	{
		// Two independent records. Proves the second record's field area does not
		// overlap the first. localBytes = 16 + 16 = 32.
		name:    "two_records",
		src:     "type P struct { x int }\nfunc main() {\n    let a = P { x: 1 }\n    let b = P { x: 2 }\n    print(a.x)\n    print(b.x)\n}\n",
		wantOut: []byte("1\n2\n"), wantExit: 0, localBytes: 32,
	},
}

// nat9hRefusals are shapes outside the supported subset. Each must be refused BY
// NAME with the named diagnostic, must NOT produce an image, and (for the driver's
// own K145 refusals) must say it is not handed to the Go engine. `code` names the
// diagnostic because one case is caught EARLIER, by kcc's type checker (K106), and
// asserting K145 for it would be a wrong expectation that teaches readers to
// distrust the gate.
var nat9hRefusals = []struct {
	name string
	src  string
	want string
	code string
}{
	{
		// Printing a record as a whole. A record has no scalar reading -- printing
		// one would render its ADDRESS as a decimal, so it is refused by name.
		name: "print_whole_record",
		src:  "type P struct { x int }\nfunc main() {\n    let p = P { x: 1 }\n    print(p)\n}\n",
		want: "cannot print `p` as a whole",
		code: "error[K145]",
	},
	{
		// An unknown field in a literal.
		name: "unknown_field_in_literal",
		src:  "type P struct { x int }\nfunc main() {\n    let p = P { x: 1, y: 2 }\n    print(p.x)\n}\n",
		want: "record `P` has no field `y`",
		code: "error[K145]",
	},
	{
		// An unknown record TYPE. Caught by kcc's own checker (K106), before the
		// native driver runs -- a stronger outcome than a driver refusal.
		name: "unknown_type",
		src:  "type P struct { x int }\nfunc main() {\n    let p = Q { x: 1 }\n    print(p.x)\n}\n",
		want: "undefined type 'Q'",
		code: "error[K106]",
	},
	{
		// A nested-struct field. Records of records are not lowered.
		name: "nested_struct_field",
		src:  "type Q struct { n int }\ntype P struct { q Q }\nfunc main() {\n    let p = P { q: Q { n: 1 } }\n    print(p.q)\n}\n",
		want: "must be initialised with an integer literal",
		code: "error[K145]",
	},
	{
		// An unknown field on a read.
		name: "unknown_field_read",
		src:  "type P struct { x int }\nfunc main() {\n    let p = P { x: 1 }\n    print(p.z)\n}\n",
		want: "record `P` has no field `z`",
		code: "error[K145]",
	},
	{
		// Assigning to a field the record does not have.
		name: "assign_unknown_field",
		src:  "type P struct { x int }\nfunc main() {\n    let p = P { x: 1 }\n    p.q = 2\n    print(p.x)\n}\n",
		want: "record `P` has no field `q`",
		code: "error[K145]",
	},
	{
		// Assigning a string to an int field.
		name: "assign_string_to_int_field",
		src:  "type P struct { x int }\nfunc main() {\n    let p = P { x: 1 }\n    p.x = \"s\"\n    print(p.x)\n}\n",
		want: "field `x` is int",
		code: "error[K145]",
	},
	{
		// Aliasing a record with `let b = a`. A record alias would have to share one
		// field area; the driver binds only literals, so this is refused by name
		// rather than silently copying or sharing the wrong thing.
		name: "record_alias",
		src:  "type P struct { x int }\nfunc main() {\n    let a = P { x: 1 }\n    let b = a\n    print(a.x)\n}\n",
		want: "initialised from a non-integer expression",
		code: "error[K145]",
	},
	{
		// Integer arithmetic on record fields (p.x * p.y + 1). SAFE -- it is
		// refused, never mis-lowered -- but the DIAGNOSTIC IS IMPRECISE: it
		// misroutes to the string-concatenation refusal because the concat-site
		// scanner treats any `+` as a potential concat without first checking the
		// operand kinds. That is a documented diagnostic-quality gap for a follow-up
		// slice, NOT a miscompile, so this case asserts only that it is refused with
		// K145 rather than pinning the misleading message as if it were correct.
		name: "field_arithmetic",
		src:  "type P struct { x int; y int }\nfunc main() {\n    let p = P { x: 10, y: 4 }\n    print(p.x * p.y + 1)\n}\n",
		want: "error[K145]",
		code: "error[K145]",
	},
}

// TestPhase151A9H_RecordProgramsExecuteWithExactBytes is layer 1: the primary
// claim. Each program is compiled by kcc, written to disk as a PE, EXECUTED, and
// its stdout compared byte for byte. Nothing is trimmed.
func TestPhase151A9H_RecordProgramsExecuteWithExactBytes(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("PE images only execute on Windows; the driver itself is cross-platform")
	}
	for _, c := range nat9hCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			img := nat9dImage(t, c.src)
			got, code := nat9dRun(t, img)
			if !nat9cBytesEq(got, c.wantOut) {
				t.Errorf("stdout = %q (% x), want %q (% x)\n  source: %s\n"+
					"  compared as EXACT BYTES: a missing or extra trailing newline "+
					"is a real defect, and trimming would hide it",
					got, got, c.wantOut, c.wantOut, c.src)
			}
			if code != c.wantExit {
				t.Errorf("exit = %d, want %d\n  source: %s", code, c.wantExit, c.src)
			}
		})
	}
}

// TestPhase151A9H_RecordsProduceDistinctImages is layer 2: the anti-hard-coding
// layer. It compares IMAGES, not stdout, because a stdout comparison can be
// satisfied by choosing the right expected value once. A driver that emitted a
// fixed field value instead of reading the literal would produce the SAME IMAGE
// for different field values, so distinctness fails where a single stdout case
// might pass.
func TestPhase151A9H_RecordsProduceDistinctImages(t *testing.T) {
	variants := []struct{ name, src string }{
		{"x1", "type P struct { x int }\nfunc main() {\n    let p = P { x: 1 }\n    print(p.x)\n}\n"},
		{"x2", "type P struct { x int }\nfunc main() {\n    let p = P { x: 2 }\n    print(p.x)\n}\n"},
		{"x3", "type P struct { x int }\nfunc main() {\n    let p = P { x: 3 }\n    print(p.x)\n}\n"},
		{"two_fields", "type P struct { x int; y int }\nfunc main() {\n    let p = P { x: 1, y: 2 }\n    print(p.x)\n}\n"},
	}
	var prior []byte
	var priorName string
	for _, v := range variants {
		v := v
		t.Run(v.name, func(t *testing.T) {
			img := nat9dImage(t, v.src)
			if prior != nil && nat9cBytesEq(img, prior) {
				t.Fatalf("image for %s is byte-identical to %s.\n  The driver must "+
					"read the field values and the field count out of the AST; "+
					"identical images mean one of them is not reaching the emitter.\n"+
					"  source: %s", v.name, priorName, v.src)
			}
			prior, priorName = img, v.name
		})
	}
}

// TestPhase151A9H_FrameFollowsTheValueModelRule is layer 3, and it is the layer
// that pins the 9h-DEFECT-FIX directly.
//
// A record local owns 8 bytes for its value slot PLUS 8 bytes per field unit, so a
// two-field record must need MORE frame than a one-field record, and a record plus
// a scalar must need more than the record alone. The unfixed planner reserved ZERO
// field bytes, so it computed the one-field frame (624) for EVERY record program;
// asserting the correct per-shape frame is what makes that regression fail here
// even before a single byte of stdout is compared.
func TestPhase151A9H_FrameFollowsTheValueModelRule(t *testing.T) {
	for _, c := range nat9hCases {
		if c.localBytes == 0 {
			continue
		}
		c := c
		t.Run(c.name, func(t *testing.T) {
			img := nat9dImage(t, c.src)
			got := nat9dFrameSubRsp(t, img)
			want := nat9dExpectFrame(c.localBytes)
			if !nat9dContainsInt32(got, want) {
				t.Errorf("no `sub rsp, %d` in the image; got %v.\n  want %d for "+
					"localBytes %d.\n  A record is 8 bytes for its value slot plus "+
					"8 per field unit (a string field is TWO units); sizing it "+
					"without the field area lets the next local alias the first "+
					"field, which is the 9h-DEFECT-FIX.\n  source: %s",
					want, got, want, c.localBytes, c.src)
			}
		})
	}
}

// TestPhase151A9H_OutOfSubsetShapesAreRefusedByName is layer 4: the refusal layer.
//
// nat9hRefusals describes nine shapes the record lowering does NOT support. Until
// this test existed the table was written but never exercised, so it documented an
// intent without protecting it -- the same "production code with no gate" class this
// file's header says it was opened to close.
//
// THE EXPECTATIONS ARE MEASURED, NOT INVENTED. Every case was driven through kcc
// and its diagnostic text recorded before a single assertion below was written, so
// a refusal that shifts to a different message fails instead of quiet-passing on a
// hope. Two measured facts shape the assertions:
//
//   - eight cases are refused by the native DRIVER with error[K145], and each of
//     those messages carries the long "151A Step 9h-2a native driver lowers ..."
//     enumeration plus "not handed to the Go engine";
//   - `unknown_type` is refused EARLIER by kcc's own type checker with error[K106]
//     ("use of undefined type 'Q' in struct literal"). That is a strictly stronger
//     outcome -- the program never reached the native lowering at all -- so it is
//     deliberately exempt from the Go-engine assertion below.
//
// A bare error code is not enough to distinguish a refusal from the failures this
// layer exists to catch, so each case also has to:
//
//   - NOT be hex. A "refusal" followed by hex bytes is a program that was silently
//     mis-lowered into a real image while also emitting a diagnostic -- the worst
//     outcome, because it looks like a refusal and runs like a miscompile.
//   - be TEXT. nat9cBuild already fails a crash that produces no output, which is
//     what separates a refusal from a driver crash or a killed process.
//   - NAME the offending construct, so a refusal firing for the wrong reason fails.
//
// NOTE on `field_arithmetic`: its measured diagnostic is the chained-concatenation
// refusal, which is imprecise for a program that is really about record fields.
// That is a documented diagnostic-quality gap in the file's own comment and is NOT
// pinned here as if it were correct -- this case asserts the K145 refusal only.
func TestPhase151A9H_OutOfSubsetShapesAreRefusedByName(t *testing.T) {
	for _, r := range nat9hRefusals {
		r := r
		t.Run(r.name, func(t *testing.T) {
			out := nat9cBuild(t, r.src)

			// A refusal must be TEXT, never an image. This is checked first so a
			// silently mis-lowered program is reported as such rather than being
			// waved through by the string checks below.
			if isHex9d(strings.TrimSpace(out)) {
				t.Fatalf("a REFUSED program also produced an image:\n%.200s", out)
			}

			if !strings.Contains(out, r.code) {
				t.Fatalf("expected %s, got:\n%.400s", r.code, out)
			}
			if !strings.Contains(out, r.want) {
				t.Errorf("refusal does not name the offending construct %q:\n%.400s",
					r.want, out)
			}
			// "not handed to the Go engine" is what distinguishes a driver refusal
			// from a silent fallback into the Go backend. It is asserted only for
			// the driver's OWN refusals: a case kcc's checker rejects earlier
			// carries that checker's diagnostic instead and has no reason to
			// mention the engine, because the program never reached the lowering.
			if r.code == "error[K145]" && !strings.Contains(out, "not handed to the Go engine") {
				t.Errorf("refusal does not say the construct is not handed to the Go "+
					"engine; an unexplained refusal invites a fallback:\n%.300s", out)
			}
		})
	}
}
