package cli

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Phase 140 — Debugger Integration: `karkain dbg` walks the program under
// gdb and renders Karkain-level backtraces (karkain_dbg trace).
//
// The gate pins the exact Karkain function sequence on a straight-line
// 3-deep program. Frame LOCATIONS are asserted present-and-suffixed (the
// generated-C basename), never exact lines: #line mapping is
// assembly-relative (the known sibling-join phenomenon), so exact lines
// would couple the gate to project layout instead of debugger behavior.

const phase140Prog = `func inner() int {
    return 7
}

func outer() int {
    return inner() + 1
}

func main() {
    print(outer())
}
`

func writePhase140Prog(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "dbgprog.kark")
	if err := os.WriteFile(p, []byte(phase140Prog), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPhase140_DbgTraceSequence(t *testing.T) {
	if findGDB() == "" {
		t.Skip("gdb not found")
	}
	res := DbgCommand(writePhase140Prog(t), false)
	if res.ExitCode != ExitSuccess {
		t.Fatalf("dbg exit = %d, want 0:\n%s", res.ExitCode, res.Message)
	}
	lines := strings.Split(strings.TrimSpace(res.Message), "\n")
	if len(lines) < 4 || lines[0] != "karkain_dbg trace: dbgprog.kark" {
		t.Fatalf("trace header wrong:\n%s", res.Message)
	}
	// Top three frames must be exactly inner <- outer <- main, each with a
	// location in the generated unit.
	wantFuncs := []string{"inner", "outer", "main"}
	for i, want := range wantFuncs {
		fields := strings.Fields(lines[i+1])
		if len(fields) < 3 || fields[0] != "#"+strconv.Itoa(i) || fields[1] != want {
			t.Errorf("frame %d = %q, want #%d %s at <loc>:\n%s", i, lines[i+1], i, want, res.Message)
			continue
		}
		// Location is "<path>:<line>" where path names generated C or
		// Karkain source (the #line-mapped unit); strip the line first.
		loc := fields[3]
		if cut := strings.LastIndex(loc, ":"); cut < 0 {
			t.Errorf("frame %d location %q lacks :line:\n%s", i, loc, res.Message)
		} else if base, num := loc[:cut], loc[cut+1:]; !strings.HasSuffix(base, ".c") && !strings.HasSuffix(base, ".kark") {
			t.Errorf("frame %d location %q names neither generated C nor Karkain source:\n%s", i, loc, res.Message)
		} else if _, err := strconv.Atoi(num); err != nil {
			t.Errorf("frame %d line %q is not numeric:\n%s", i, num, res.Message)
		}
	}
}

func TestPhase140_DbgTraceDeterministic(t *testing.T) {
	if findGDB() == "" {
		t.Skip("gdb not found")
	}
	// Function SEQUENCE is stable across runs (locations carry temp paths,
	// so only the demangled names compare).
	seq := func(t *testing.T) []string {
		t.Helper()
		res := DbgCommand(writePhase140Prog(t), false)
		if res.ExitCode != ExitSuccess {
			t.Fatalf("dbg exit = %d:\n%s", res.ExitCode, res.Message)
		}
		var out []string
		for _, line := range strings.Split(res.Message, "\n")[1:] {
			if f := strings.Fields(line); len(f) >= 2 {
				out = append(out, f[1])
			}
		}
		return out
	}
	a, b := seq(t), seq(t)
	if strings.Join(a, ",") != strings.Join(b, ",") {
		t.Fatalf("trace unstable:\n%v\n%v", a, b)
	}
	if len(a) < 3 || a[0] != "inner" || a[1] != "outer" || a[2] != "main" {
		t.Fatalf("trace head wrong: %v", a)
	}
}

func TestPhase140_DbgMissingFile(t *testing.T) {
	res := DbgCommand(filepath.Join(t.TempDir(), "nope.kark"), false)
	if res.ExitCode != ExitUsage {
		t.Errorf("missing file exit = %d, want %d:\n%s", res.ExitCode, ExitUsage, res.Message)
	}
}

func TestPhase140_DbgSemanticError(t *testing.T) {
	if findGDB() == "" {
		t.Skip("gdb not found")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.kark")
	if err := os.WriteFile(p, []byte("func main() {\n print(nope)\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := DbgCommand(p, false)
	if res.ExitCode != ExitCompile {
		t.Errorf("semantic-error exit = %d, want %d:\n%s", res.ExitCode, ExitCompile, res.Message)
	}
}
