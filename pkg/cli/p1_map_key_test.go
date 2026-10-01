package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// P1 - map key equality on the default (self-hosted kcc) engine.
//
// THE DEFECT THIS PINS
//
// kcc stored map keys as `char**`, so a key was only ever a C string.
// map_get, map_set, karkain_hasKey and karkain_delete each coerced the key
// through `k.strVal` and compared with strcmp. An integer key has
// strVal == NULL, so it collapsed onto the empty string: every integer key
// matched the SAME entry (the last one stored) and len(m) under-counted.
//
// The failure mode is why this needs a dedicated gate: it is SILENT. The
// program printed plausible values and exited 0. Nothing in the existing corpus
// could see it, because every map literal there has STRING keys
// (examples/01-fundamentals/09_maps.kark) -- and a string-keyed map is exactly
// the case the broken comparison happened to get right.
//
// WHY A DIFFERENTIAL AND NOT A GOLDEN
//
// The defect was a DISAGREEMENT between the two engines, so a golden pinned to
// either engine alone would have kept passing while the defect lived. Both
// halves are therefore required:
//   - kcc output == Go reference output (the disagreement is gone), AND
//   - that shared output == the expected lines (both are not wrong the same
//     way, which a cross-engine comparison alone cannot rule out).

// mapKeyCase is one program plus the exact stdout both engines must produce.
type mapKeyCase struct {
	name string
	src  string
	want []string
}

var mapKeyCases = []mapKeyCase{
	{
		// The original collision: two DIFFERENT integer keys used to read back
		// the same value and the map reported length 1.
		name: "integer_keys_do_not_collide",
		src: "func main() {\n" +
			"    let m = { 1: \"one\", 2: \"two\" }\n" +
			"    print(m[1])\n" +
			"    print(m[2])\n" +
			"    print(len(m))\n" +
			"}\n",
		want: []string{"one", "two", "2"},
	},
	{
		// Insertion and overwrite through map_set, which had the same
		// strcmp coercion.
		name: "integer_keys_set_and_overwrite",
		src: "func main() {\n" +
			"    let m = {}\n" +
			"    m[1] = \"one\"\n" +
			"    m[2] = \"two\"\n" +
			"    print(m[1])\n" +
			"    print(m[2])\n" +
			"    m[2] = \"TWO\"\n" +
			"    print(m[2])\n" +
			"    print(len(m))\n" +
			"}\n",
		want: []string{"one", "two", "TWO", "2"},
	},
	{
		// karkain_hasKey had the identical coercion, so every integer key
		// reported the same answer.
		name: "hasKey_on_integer_keys",
		src: "func main() {\n" +
			"    let m = { 1: \"one\", 2: \"two\" }\n" +
			"    print(hasKey(m, 1))\n" +
			"    print(hasKey(m, 2))\n" +
			"    print(hasKey(m, 9))\n" +
			"}\n",
		want: []string{"1", "1", "0"},
	},
	{
		// PRESERVATION: string keys are the case the old code got right, and
		// they are what structs are built from, so a regression here would be
		// far more damaging than the bug being fixed.
		name: "string_keys_still_work",
		src: "func main() {\n" +
			"    let m = { \"one\": 100, \"two\": 200 }\n" +
			"    print(m[\"one\"])\n" +
			"    print(m[\"two\"])\n" +
			"    print(len(m))\n" +
			"    print(hasKey(m, \"one\"))\n" +
			"    print(hasKey(m, \"zz\"))\n" +
			"}\n",
		want: []string{"100", "200", "2", "1", "0"},
	},
	{
		// delete also read keys as char*, and it shifts the key array, so it
		// must keep working with typed keys after the representation change.
		name: "delete_on_mixed_keys",
		src: "func main() {\n" +
			"    let m = {}\n" +
			"    m[1] = \"one\"\n" +
			"    m[\"one\"] = 100\n" +
			"    print(len(m))\n" +
			"    delete(m, 1)\n" +
			"    print(hasKey(m, 1))\n" +
			"    print(hasKey(m, \"one\"))\n" +
			"    print(m[\"one\"])\n" +
			"    print(len(m))\n" +
			"}\n",
		want: []string{"2", "0", "1", "100", "1"},
	},
	{
		// A struct IS a map with string keys, so this is the highest-volume
		// consumer of the code path being changed.
		name: "struct_fields_are_string_keyed_maps",
		src: "type Point struct { x int; y int }\n" +
			"\n" +
			"func main() {\n" +
			"    let p = Point{ x: 3, y: 4 }\n" +
			"    print(p.x)\n" +
			"    print(p.y)\n" +
			"    p.x = 10\n" +
			"    print(p.x)\n" +
			"    print(p.y)\n" +
			"}\n",
		want: []string{"3", "4", "10", "4"},
	},
}

// runMapKeyCase runs one fixture through the real CLI on a named engine and
// returns its stdout lines. stderr is deliberately NOT merged: the defect being
// pinned produced wrong VALUES on stdout, and merging stderr would let a
// diagnostic stand in for a missing line.
func runMapKeyCase(t *testing.T, bin, src, engine string) []string {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "main.kark")
	// BOM-free: the lexer rejects a UTF-8 BOM.
	if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	cmd := exec.Command(bin, "run", file, "--engine", engine)
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

// TestP1_MapKeyEquality_DefaultEngineMatchesReference is the P1 regression
// gate. It names the self-hosted engine explicitly, because "the default
// engine" is a configuration that can change; kcc is the engine that had the
// defect and is the one that must not regress.
func TestP1_MapKeyEquality_DefaultEngineMatchesReference(t *testing.T) {
	hasGCC(t)
	bin := phase130Karkain(t)
	// The child runs with cwd set to a temp dir so fixtures never touch the
	// repo, which means it cannot locate src/compiler to build the engine
	// itself. Point it at a prebuilt kcc, the same way the other engine-driving
	// gates in this package do.
	selectKCCEngine(t, phase95KCC(t))

	for _, tc := range mapKeyCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := runMapKeyCase(t, bin, tc.src, "kcc")

			want := strings.Join(tc.want, "\n")
			if have := strings.Join(got, "\n"); have != want {
				t.Errorf("self-hosted kcc output:`n have = %q`n want = %q", have, want)
			}

			// The differential: the two engines must agree. Comparing against
			// the reference engine is what makes this a parity assertion rather
			// than a self-consistent one.
			ref := runMapKeyCase(t, bin, tc.src, "go")
			if strings.Join(got, "\n") != strings.Join(ref, "\n") {
				t.Errorf("engine disagreement on the same source:`n   kcc = %q`n    go = %q",
					strings.Join(got, "\n"), strings.Join(ref, "\n"))
			}
		})
	}
}

// TestP1_MapKeyEquality_NotVacuous states the defect as a precondition, so the
// gate above cannot silently become a test of nothing.
//
// It asserts that the fixture actually exercises the defect: with an
// integer-keyed map, m[1], m[2] and m[3] must be DISTINCT values. Under the old
// strcmp-only comparison they collapsed onto one entry, so this assertion is
// what distinguishes "the map works" from "the map was never asked a question
// with two different answers".
func TestP1_MapKeyEquality_NotVacuous(t *testing.T) {
	hasGCC(t)
	bin := phase130Karkain(t)
	selectKCCEngine(t, phase95KCC(t))

	src := "func main() {\n" +
		"    let m = { 1: \"one\", 2: \"two\", 3: \"three\" }\n" +
		"    print(m[1])\n" +
		"    print(m[2])\n" +
		"    print(m[3])\n" +
		"}\n"
	got := runMapKeyCase(t, bin, src, "kcc")
	want := []string{"one", "two", "three"}
	if len(got) != len(want) {
		t.Fatalf("got %d lines %v, want %d %v", len(got), got, len(want), want)
	}
	seen := map[string]bool{}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d: got %q, want %q (keys collapsed?)", i, got[i], want[i])
		}
		if seen[got[i]] {
			t.Fatalf("line %d repeats an earlier value %q: distinct keys must read back distinctly", i, got[i])
		}
		seen[got[i]] = true
	}
}
