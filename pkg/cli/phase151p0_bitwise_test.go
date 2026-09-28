package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Phase 151P0 - bitwise and shift operators. See
// docs/audit/PHASE-151P0-BITWISE-BASELINE.md.
//
// Before this increment `&` `|` `^` `<<` `>>` were not implemented, and
// the failure was SILENT on the reference engine: a program containing
// `a & b` compiled, exited 0, and printed a wrong answer. This gate exists
// because that class of defect is invisible to every other test in the
// repository - there is nothing to fail, the program just lies.
//
// The gate therefore asserts CORRECT VALUES, not merely that the two
// engines agree. A byte-identity assertion alone would be satisfied by two
// engines that are identically wrong, which is precisely the state this
// increment was found in.
//
// Every value below is C semantics, because Karkain compiles to C and a
// user who reasons in one must get the same grouping and the same results
// in the other. Two entries pin grouping rules that were WRONG in the first
// draft of this increment and were caught by the probe rather than by
// inspection: "1<<3+1" must be 16 (shifts bind looser than +) and
// "1|2==2" must be 1 (the comparisons bind tighter than |).
var bitwiseGolden = []string{
	// and / or / xor
	"255&15 15",
	"0|0 0",
	"0|7 7",
	"7|0 7",
	"0^0 0",
	"5^3 6",
	"255^0 255",
	"0^255 255",
	"597&21 21",
	// shifts
	"1<<0 1",
	"1<<1 2",
	"1<<4 16",
	"1<<10 1024",
	"1024>>3 128",
	"1>>1 0",
	// shift edges: a count outside the 64-bit width is defined, not UB
	"1<<63 -9223372036854775808",
	"1<<64 0",
	"1<<65 0",
	"1<<(-1) 0",
	"1024>>(-1) 0",
	"1024>>64 0",
	"1024>>63 0",
	// negative values: right shift is arithmetic and sign-preserving
	"-8>>1 -4",
	"-1>>63 -1",
	"-1>>64 0",
	"-1&255 255",
	"-1|0 -1",
	// precedence, the C table
	"1<<3+1 16",
	"1<<(3+1) 16",
	"6&3+1 4",
	"6&(3+1) 4",
	"1|2^3&1 3",
	"(1|2)^(3&1) 2",
	"1|2==2 1",
	"1==2|2 2",
	"1<<2*2 16",
	"1<<2*3 64",
	"2*1<<3 16",
	// combined
	"byte-roundtrip 47",
	"imm64 4822678189205111",
}

// runBitwiseOn runs the testdata corpus under one engine and returns its
// output lines with the engine banner removed.
func runBitwiseOn(t *testing.T, karkain, engine string) []string {
	t.Helper()
	corpus, err := os.ReadFile(filepath.Join(repoRoot(t), "pkg", "cli", "testdata", "phase151p0_bitwise.kark"))
	if err != nil {
		t.Fatalf("reading corpus: %v", err)
	}
	// Staged into a temp dir so the run leaves no generated .c beside the
	// repository's own testdata.
	dir := t.TempDir()
	src := filepath.Join(dir, "gate.kark")
	if err := os.WriteFile(src, corpus, 0o644); err != nil {
		t.Fatalf("writing corpus: %v", err)
	}
	cmd := exec.Command(karkain, "run", src)
	cmd.Env = append(os.Environ(), "KARKAIN_ENGINE="+engine)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s engine: run failed: %v\n%s", engine, err, out)
	}
	var lines []string
	for _, ln := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		ln = strings.TrimRight(ln, "\r")
		// The kcc engine prints a build banner before the program output.
		// It is a driver line, not program stdout, so it is dropped rather
		// than allowed to fail parity for a reason unrelated to the
		// operators.
		if strings.HasPrefix(ln, "[ok]") || strings.HasPrefix(ln, "[kcc]") {
			continue
		}
		lines = append(lines, ln)
	}
	return lines
}

// TestPhase151P0_BitwiseCorrectOnBothEngines is the load-bearing assertion:
// each engine must produce the EXPECTED values. A gate that only compared
// the engines to each other would have passed on the defective code, which
// is the whole reason this increment needed its own gate.
func TestPhase151P0_BitwiseCorrectOnBothEngines(t *testing.T) {
	karkain := phase130Karkain(t)
	for _, engine := range []string{"go", "kcc"} {
		engine := engine
		t.Run(engine, func(t *testing.T) {
			got := runBitwiseOn(t, karkain, engine)
			if len(got) != len(bitwiseGolden) {
				t.Fatalf("%s engine: got %d output lines, want %d\n got: %v",
					engine, len(got), len(bitwiseGolden), got)
			}
			for i := range bitwiseGolden {
				if got[i] != bitwiseGolden[i] {
					t.Errorf("%s engine: line %d: got %q, want %q",
						engine, i+1, got[i], bitwiseGolden[i])
				}
			}
		})
	}
}

// TestPhase151P0_CrossEngineParity pins the two engines byte-identically, so
// "both engines implement the same semantics" is measured, not assumed.
func TestPhase151P0_CrossEngineParity(t *testing.T) {
	karkain := phase130Karkain(t)
	goOut := runBitwiseOn(t, karkain, "go")
	kccOut := runBitwiseOn(t, karkain, "kcc")
	if len(goOut) != len(kccOut) {
		t.Fatalf("engine line counts differ: go=%d kcc=%d\ngo:  %v\nkcc: %v",
			len(goOut), len(kccOut), goOut, kccOut)
	}
	for i := range goOut {
		if goOut[i] != kccOut[i] {
			t.Errorf("line %d differs: go=%q kcc=%q", i+1, goOut[i], kccOut[i])
		}
	}
}

// TestPhase151P0_NonIntegerOperands pins the documented boundary: bitwise has
// no sensible coercion, so a float or string operand yields 0 on both engines
// rather than a truncated or garbage result.
//
// The bool case is the deliberate EXCEPTION and is listed here so the
// asymmetry is explicit rather than accidental: a bool operand is lowered to
// an int before it reaches binary_op, and is accepted, exactly as the
// arithmetic block does. Refusing it would make `true & 1` disagree with
// `true + 1` for no stated reason. An earlier draft of this gate asserted
// `true & 1` was 0 and failed against the reference engine, which is how the
// coercion was found.
func TestPhase151P0_NonIntegerOperands(t *testing.T) {
	karkain := phase130Karkain(t)
	const src = `func p(label, v) {
	print(label + " " + str(v))
}

func main() {
	p("1.5&1", 1.5 & 1)
	p("1|1.5", 1 | 1.5)
	p("true&1", true & 1)
	p("s&1", "a" & 1)
	p("1.5<<1", 1.5 << 1)
}
`
	want := []string{"1.5&1 0", "1|1.5 0", "true&1 1", "s&1 0", "1.5<<1 0"}

	for _, engine := range []string{"go", "kcc"} {
		engine := engine
		t.Run(engine, func(t *testing.T) {
			dir := t.TempDir()
			f := filepath.Join(dir, "nonint.kark")
			if err := os.WriteFile(f, []byte(src), 0o644); err != nil {
				t.Fatalf("writing corpus: %v", err)
			}
			cmd := exec.Command(karkain, "run", f)
					cmd.Env = append(os.Environ(), "KARKAIN_ENGINE="+engine)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%s engine: run failed: %v\n%s", engine, err, out)
			}
			var got []string
			for _, ln := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
				ln = strings.TrimRight(ln, "\r")
				if strings.HasPrefix(ln, "[ok]") || strings.HasPrefix(ln, "[kcc]") {
					continue
				}
				got = append(got, ln)
			}
			if len(got) != len(want) {
				t.Fatalf("%s engine: got %d lines, want %d\n got: %v", engine, len(got), len(want), got)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Errorf("%s engine: line %d: got %q, want %q", engine, i+1, got[i], want[i])
				}
			}
		})
	}
}

// TestPhase151P0_LogicalOperatorsUnaffected guards the neighbours this change
// could plausibly have broken.
//
// The risk is real and specific: `&` was, on the self-hosted engine, emitted
// as the LOGICAL-and token with length 1, so `a & b` silently meant
// `a && b` there. Adding a real bitwise token could easily have gone the
// other way and broken `&&`, and the `<` case had to gain `<<` without
// losing `<-` (the concurrency chanSend keyword) or `<=`.
func TestPhase151P0_LogicalOperatorsUnaffected(t *testing.T) {
	karkain := phase130Karkain(t)
	const src = `func p(label, v) {
	print(label + " " + str(v))
}

func main() {
	p("0&&0", 0 && 0)
	p("1&&1", 1 && 1)
	p("0||0", 0 || 0)
	p("0||1", 0 || 1)
	p("1<2", 1 < 2)
	p("2<=2", 2 <= 2)
	p("3>2", 3 > 2)
	p("3>=4", 3 >= 4)
}
`
	want := []string{
		"0&&0 0", "1&&1 1", "0||0 0", "0||1 1",
		"1<2 1", "2<=2 1", "3>2 1", "3>=4 0",
	}

	for _, engine := range []string{"go", "kcc"} {
		engine := engine
		t.Run(engine, func(t *testing.T) {
			dir := t.TempDir()
			f := filepath.Join(dir, "logical.kark")
			if err := os.WriteFile(f, []byte(src), 0o644); err != nil {
				t.Fatalf("writing corpus: %v", err)
			}
			cmd := exec.Command(karkain, "run", f)
					cmd.Env = append(os.Environ(), "KARKAIN_ENGINE="+engine)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%s engine: run failed: %v\n%s", engine, err, out)
			}
			var got []string
			for _, ln := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
				ln = strings.TrimRight(ln, "\r")
				if strings.HasPrefix(ln, "[ok]") || strings.HasPrefix(ln, "[kcc]") {
					continue
				}
				got = append(got, ln)
			}
			if len(got) != len(want) {
				t.Fatalf("%s engine: got %d lines, want %d\n got: %v", engine, len(got), len(want), got)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Errorf("%s engine: line %d: got %q, want %q", engine, i+1, got[i], want[i])
				}
			}
		})
	}
}
