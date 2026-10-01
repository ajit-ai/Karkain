package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Phase 151 - truth-value and equality semantics in the self-hosted engine.
//
// WHAT WAS INCOMPLETE (both proven by execution before this change, with the
// Go reference run first):
//
//  1. `is_truthy` had no TYPE_MAP, TYPE_OPTION or TYPE_RESULT case, so the
//     chain fell through to `return 0`. A POPULATED MAP WAS THEREFORE FALSY:
//     `if (m) {...}` never ran its body when m held entries. Some(5) and Ok(7)
//     were falsy for the same reason.
//  2. `values_equal` had no TYPE_MAP case either, so it fell through to
//     `return make_int(0)`. Two identical maps compared UNEQUAL: `m == m` was
//     0 and `m != m` was 1, on a plain .kark program, silently.
//
// Both are fixed by mirroring the reference in pkg/codegen/codegen.go
// (is_truthy at codegen.go:2121, values_equal at codegen.go:1242) rather than
// by inventing a rule of our own.
//
// WHY THIS IS THE CONSISTENCY FIX
//
// Map key matching already routes through values_equal -- map_get, map_set,
// karkain_hasKey and karkain_delete all call it -- so before this change a map
// used as a key could never match even itself. Completing values_equal is what
// makes one value-equality rule apply everywhere the language needs it.
//
// DELIBERATELY NOT DONE
//
// values_equal still returns 0 for TYPE_OPTION and TYPE_RESULT, because the
// reference returns 0 for them too. Handling them here would invent semantics
// and create a NEW cross-engine difference, which is the opposite of the goal.
//
// RESULTS ARE PROBED THROUGH A CONDITION
//
// Cases use `if (cond) { print(1) } else { print(0) }` rather than printing a
// comparison directly, for a measured reason: the engines disagree on how a
// boolean PRINTS (print(1 == 1) is `true` on Go, `1` on kcc). The truth value
// is identical; only the rendering differs. Probing through a condition
// asserts the semantics this slice is about and stays comparable across both
// engines.

var truthCases = []struct {
	name string
	src  string
	want []string
}{
	{
		name: "scalar_truth_values",
		src: "func main() {\n" +
			"    if (0) { print(1) } else { print(0) }\n" +
			"    if (5) { print(1) } else { print(0) }\n" +
			"    if (-3) { print(1) } else { print(0) }\n" +
			"    if (0.0) { print(1) } else { print(0) }\n" +
			"    if (1.5) { print(1) } else { print(0) }\n" +
			"    if (\"\") { print(1) } else { print(0) }\n" +
			"    if (\"x\") { print(1) } else { print(0) }\n" +
			"    if (true) { print(1) } else { print(0) }\n" +
			"    if (false) { print(1) } else { print(0) }\n" +
			"}\n",
		want: []string{"0", "1", "1", "0", "1", "0", "1", "1", "0"},
	},
	{
		// THE REGRESSION THIS SLICE FIXES: a populated map used to be falsy.
		name: "collection_truth_values",
		src: "func main() {\n" +
			"    if ([]) { print(1) } else { print(0) }\n" +
			"    if ([1]) { print(1) } else { print(0) }\n" +
			"    if ({}) { print(1) } else { print(0) }\n" +
			"    let m = { \"a\": 1 }\n" +
			"    if (m) { print(1) } else { print(0) }\n" +
			"}\n",
		want: []string{"0", "1", "0", "1"},
	},
	{
		// Some and Ok are truthy, None and Err are falsy. All four were
		// falsy before, because is_truthy had no case for either type.
		name: "option_and_result_truth_values",
		src: "func main() {\n" +
			"    if (Some(5)) { print(1) } else { print(0) }\n" +
			"    if (None) { print(1) } else { print(0) }\n" +
			"    if (Ok(7)) { print(1) } else { print(0) }\n" +
			"    if (Err(9)) { print(1) } else { print(0) }\n" +
			"}\n",
		want: []string{"1", "0", "1", "0"},
	},
	{
		// A map driving a loop. `while` consults the same is_truthy, and this
		// is where the old `return 0` meant the body never ran at all.
		name: "map_drives_while_loop",
		src: "func main() {\n" +
			"    let m = { \"go\": 1 }\n" +
			"    let n = 0\n" +
			"    while (m) {\n" +
			"        print(n)\n" +
			"        n = n + 1\n" +
			"        delete(m, \"go\")\n" +
			"    }\n" +
			"    print(n)\n" +
			"}\n",
		want: []string{"0", "1"},
	},
	{
		// a == a and a != b, the two cases named for this feature.
		name: "equality_and_inequality_of_scalars",
		src: "func main() {\n" +
			"    let a = 5\n" +
			"    let b = 5\n" +
			"    let c = 6\n" +
			"    if (a == a) { print(1) } else { print(0) }\n" +
			"    if (a == b) { print(1) } else { print(0) }\n" +
			"    if (a == c) { print(1) } else { print(0) }\n" +
			"    if (a != a) { print(1) } else { print(0) }\n" +
			"    if (a != c) { print(1) } else { print(0) }\n" +
			"}\n",
		want: []string{"1", "1", "0", "0", "1"},
	},
	{
		// THE OTHER REGRESSION: identical maps compared unequal, so `m == m`
		// was 0 and `m != m` was 1.
		name: "map_equality_reflexive",
		src: "func main() {\n" +
			"    let m = { \"a\": 1, \"b\": 2 }\n" +
			"    print(m == m)\n" +
			"    print(m != m)\n" +
			"}\n",
		want: []string{"1", "0"},
	},
	{
		// Equality of maps is by CONTENT and independent of insertion order,
		// matching the reference. A different-length map is unequal, and
		// `!=` is exactly the negation.
		name: "map_equality_by_content",
		src: "func main() {\n" +
			"    let m1 = { \"a\": 1, \"b\": 2 }\n" +
			"    let m2 = { \"b\": 2, \"a\": 1 }\n" +
			"    let m3 = { \"a\": 1 }\n" +
			"    let m4 = { \"a\": 9 }\n" +
			"    print(m1 == m2)\n" +
			"    print(m1 == m3)\n" +
			"    print(m1 != m3)\n" +
			"    print(m1 == m4)\n" +
			"}\n",
		want: []string{"1", "0", "1", "0"},
	},
	{
		// Arrays remain ORDERED, which is the pre-existing correct behaviour
		// and must not be changed by completing map equality.
		name: "array_equality_is_ordered",
		src: "func main() {\n" +
			"    let a = [1, 2]\n" +
			"    let b = [2, 1]\n" +
			"    print(a == a)\n" +
			"    print(a == b)\n" +
			"}\n",
		want: []string{"1", "0"},
	},
	{
		// An int and a string are never equal: the type tag differs before any
		// payload comparison, so this holds regardless of the value 1.
		//
		// NOTE: this case deliberately does NOT cover int-vs-float. Measured on
		// the current tree, `i == f` with both operands bound to variables
		// yields 1 on BOTH engines, while the literal `1 == 1.0` yields
		// `false` on Go and `1` on kcc. That is an int/float promotion question,
		// not part of the map/option/result truth and equality gaps this slice
		// closed, and its ground truth is not established here. Pinning an
		// expectation for it would encode a guess as a requirement.
		name: "int_and_string_are_unequal",
		src: "func main() {\n" +
			"    let i = 1\n" +
			"    let s = \"1\"\n" +
			"    if (i == s) { print(1) } else { print(0) }\n" +
			"    if (s == s) { print(1) } else { print(0) }\n" +
			"}\n",
		want: []string{"0", "1"},
	},
}

// runTruthCase runs one fixture through the real CLI on a named engine and
// returns its stdout lines. stderr is deliberately NOT merged: this slice is
// about VALUES, and a diagnostic must not be able to stand in for a line.
func runTruthCase(t *testing.T, bin, src, engine string) []string {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "main.kark")
	// BOM-free: the lexer rejects a UTF-8 BOM.
	if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	cmd := exec.Command(bin, "run", file, "--engine", engine)
	// The child needs a cwd it can find src/compiler from, or KARKAIN_KCC must
	// point at a prebuilt engine; see the KARKAIN_KCC selection in the test.
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("--engine %s failed: %v\nstdout:\n%s\nstderr:\n%s", engine, err, stdout.String(), stderr.String())
	}
	var lines []string
	for _, ln := range strings.Split(strings.TrimRight(stdout.String(), "\r\n"), "\n") {
		lines = append(lines, strings.TrimRight(ln, "\r"))
	}
	return lines
}

// TestPhase151_TruthAndEqualitySemantics is the gate for this slice. It names
// the self-hosted engine explicitly, because "the default engine" is a
// configuration that can change.
func TestPhase151_TruthAndEqualitySemantics(t *testing.T) {
	hasGCC(t)
	bin := phase130Karkain(t)
	// The child runs with cwd set to a temp dir so fixtures never touch the
	// repo, which means it cannot locate src/compiler to build the engine
	// itself. Point it at a prebuilt kcc, as the other engine-driving gates here.
	selectKCCEngine(t, phase95KCC(t))

	for _, tc := range truthCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := runTruthCase(t, bin, tc.src, "kcc")

			want := strings.Join(tc.want, "\n")
			if have := strings.Join(got, "\n"); have != want {
				t.Errorf("self-hosted kcc output:\n have = %q\n want = %q", have, want)
			}

			// The differential against the Go reference. Both defects were
			// ENGINE DISAGREEMENTS that exited 0, so a golden pinned to either
			// engine alone would have kept passing while they lived.
			ref := runTruthCase(t, bin, tc.src, "go")
			if strings.Join(got, "\n") != strings.Join(ref, "\n") {
				t.Errorf("engine disagreement on the same source:\n   kcc = %q\n    go = %q",
					strings.Join(got, "\n"), strings.Join(ref, "\n"))
			}
		})
	}
}

// TestPhase151_TruthSemanticsAreNotVacuous guards the gate above against
// testing nothing.
//
// Each of the two defects this slice fixed was a case where the values were
// PLAUSIBLE and the exit code was 0, so "no crash" is not evidence. This
// asserts the discriminating property directly: a populated map must be
// truthy, an empty one must not, and a map must equal itself while differing
// from a map with different contents. If the is_truthy/values_equal chains ever
// fall through to their defaults again, these fail.
func TestPhase151_TruthSemanticsAreNotVacuous(t *testing.T) {
	hasGCC(t)
	bin := phase130Karkain(t)
	selectKCCEngine(t, phase95KCC(t))

	src := "func main() {\n" +
		"    let empty = {}\n" +
		"    let full = { \"a\": 1 }\n" +
		"    let same = { \"a\": 1 }\n" +
		"    let other = { \"a\": 2 }\n" +
		"    if (empty) { print(\"empty-truthy\") } else { print(\"empty-falsy\") }\n" +
		"    if (full) { print(\"full-truthy\") } else { print(\"full-falsy\") }\n" +
		"    if (full == same) { print(\"eq-same\") } else { print(\"ne-same\") }\n" +
		"    if (full == other) { print(\"eq-other\") } else { print(\"ne-other\") }\n" +
		"}\n"
	want := []string{"empty-falsy", "full-truthy", "eq-same", "ne-other"}

	got := runTruthCase(t, bin, src, "kcc")
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("truth/equality discriminators:\n have = %q\n want = %q",
			strings.Join(got, ","), strings.Join(want, ","))
	}
	ref := runTruthCase(t, bin, src, "go")
	if strings.Join(got, ",") != strings.Join(ref, ",") {
		t.Errorf("engine disagreement on the discriminators:\n   kcc = %q\n    go = %q",
			strings.Join(got, ","), strings.Join(ref, ","))
	}
}
