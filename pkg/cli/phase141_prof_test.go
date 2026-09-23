package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// Phase 141C: Value-cell allocation accuracy. The Phase 110 malloc/free
// wrappers never saw container constructions (make_string/make_array/
// make_map run inside preamble helpers); Phase 141 redirects those call
// sites in generated user code through a cell counter. This gate profiles
// a container-heavy program and pins cells > 0 in JSON and text, plus the
// exact per-construct counts (3 strings + 2 arrays + 1 map = 6 cells).

const phase141ProfSrc = `
func main() {
    let a = "alpha"
    let b = "beta"
    let xs = [1, 2, 3]
    let ys = [4, 5]
    let m = {"k": 1}
    print(a + b)
    print(len(xs) + len(ys) + len(m))
}
`

func TestPhase141_ProfCellsJSON(t *testing.T) {
	skipIfNoCompiler(t)
	f := writePhase110Fixture(t, phase141ProfSrc)
	out := captureStdout(t, func() {
		res := ProfCommand(f, "json", "", "go", "", false)
		if res.ExitCode != ExitSuccess {
			t.Fatalf("ProfCommand failed: [%d] %s", res.ExitCode, res.Message)
		}
	})
	idx := strings.Index(out, "{")
	if idx < 0 {
		t.Fatalf("no json document in captured output:\n%s", out)
	}
	var p Profile
	if err := json.Unmarshal([]byte(out[idx:]), &p); err != nil {
		t.Fatalf("json output is not a valid profile: %v\n%s", err, out)
	}
	if p.Allocation.Cells != 6 {
		t.Errorf("cells = %d, want 6 (3 strings + 2 arrays + 1 map)", p.Allocation.Cells)
	}
	if !strings.Contains(out[idx:], `"cells": 6`) {
		t.Errorf("raw dump lacks \"cells\": 6:\n%s", out)
	}
}

func TestPhase141_ProfCellsText(t *testing.T) {
	skipIfNoCompiler(t)
	f := writePhase110Fixture(t, phase141ProfSrc)
	out := captureStdout(t, func() {
		res := ProfCommand(f, "text", "", "go", "", false)
		if res.ExitCode != ExitSuccess {
			t.Fatalf("ProfCommand failed: [%d] %s", res.ExitCode, res.Message)
		}
	})
	if !strings.Contains(out, "6 value cell(s)") {
		t.Errorf("text report lacks cell count:\n%s", out)
	}
}
