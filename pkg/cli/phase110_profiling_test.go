package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Phase 110 gate: Profiling & Diagnostics CLI through the real pipeline.
// karkain prof is opt-in, Go-engine only (kcc/WASM are explicit boundaries),
// and produces deterministic schema-valid reports (text/json/folded).

const phase110FixtureSrc = `
func add(a, b) {
	return a + b
}

func double(x) {
	return x * 2
}

func main() {
	print("p110 = " + str(add(double(21), add(1, 2))))
}
`

func writePhase110Fixture(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	f := filepath.Join(dir, "main.kark")
	if err := os.WriteFile(f, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestPhase110_ProfCommand_Text(t *testing.T) {
	skipIfNoCompiler(t)
	f := writePhase110Fixture(t, phase110FixtureSrc)
	out := captureStdout(t, func() {
		res := ProfCommand(f, "text", "", "go", "", false)
		if res.ExitCode != ExitSuccess {
			t.Fatalf("ProfCommand failed: [%d] %s", res.ExitCode, res.Message)
		}
	})
	for _, want := range []string{
		"Profile of main.kark",
		"engine: go",
		"p110 = 45",
		"Call graph:",
		"main -> add x2",
		"main -> double x1",
		"Allocation:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("text report missing %q\n%s", want, out)
		}
	}
}

func TestPhase110_ProfCommand_JSON(t *testing.T) {
	skipIfNoCompiler(t)
	f := writePhase110Fixture(t, phase110FixtureSrc)
	out := captureStdout(t, func() {
		res := ProfCommand(f, "json", "", "go", "", false)
		if res.ExitCode != ExitSuccess {
			t.Fatalf("ProfCommand failed: [%d] %s", res.ExitCode, res.Message)
		}
	})
	var p Profile
	// The program's stdout passes through and precedes the report; the JSON
	// document starts at the first '{'.
	idx := strings.Index(out, "{")
	if idx < 0 {
		t.Fatalf("no json document in captured output:\n%s", out)
	}
	if err := json.Unmarshal([]byte(out[idx:]), &p); err != nil {
		t.Fatalf("json output is not a valid profile: %v\n%s", err, out)
	}
	if p.Schema != "karkain-profile-v1" || p.Version != 1 {
		t.Errorf("unexpected schema/version: %s/%d", p.Schema, p.Version)
	}
	if p.Program != "main.kark" || p.Engine != "go" {
		t.Errorf("unexpected program/engine: %s/%s", p.Program, p.Engine)
	}
	if p.Overflow {
		t.Errorf("overflow must be false for a small program")
	}
	if p.Allocation.Count < 0 || p.Allocation.Bytes < 0 || p.Allocation.PeakBytes < 0 {
		t.Errorf("allocation metrics must be non-negative: %+v", p.Allocation)
	}

	counts := map[string]int64{}
	for _, fn := range p.Functions {
		counts[fn.Name] = fn.Calls
	}
	if counts["add"] != 2 {
		t.Errorf("add calls = %d, want 2", counts["add"])
	}
	if counts["double"] != 1 {
		t.Errorf("double calls = %d, want 1", counts["double"])
	}
	if counts["main"] != 1 {
		t.Errorf("main calls = %d, want 1", counts["main"])
	}

	edges := map[string]int64{}
	for _, c := range p.Calls {
		edges[c.Caller+"->"+c.Callee] += c.Count
	}
	if edges["main->add"] != 2 {
		t.Errorf("main->add count = %d, want 2", edges["main->add"])
	}
	if edges["main->double"] != 1 {
		t.Errorf("main->double count = %d, want 1", edges["main->double"])
	}
	if len(p.Folded) == 0 {
		t.Errorf("no folded stacks in json report")
	}
}

func TestPhase110_ProfCommand_Folded(t *testing.T) {
	skipIfNoCompiler(t)
	f := writePhase110Fixture(t, phase110FixtureSrc)
	out := captureStdout(t, func() {
		res := ProfCommand(f, "folded", "", "go", "", false)
		if res.ExitCode != ExitSuccess {
			t.Fatalf("ProfCommand failed: [%d] %s", res.ExitCode, res.Message)
		}
	})
	paths := map[string]bool{}
	seenNumbers := false
	for _, ln := range strings.Split(strings.TrimSpace(out), "\n") {
		parts := strings.Fields(ln)
		if len(parts) != 2 {
			continue // program stdout passes through (by design); skip it
		}
		if !allDigits(parts[1]) {
			continue // same: not a folded "path ns" line
		}
		paths[parts[0]] = true
		seenNumbers = true
	}
	if !seenNumbers {
		t.Fatalf("no folded entries at all:\n%s", out)
	}
	for _, want := range []string{"main", "main;double", "main;add"} {
		if !paths[want] {
			t.Errorf("folded output missing stack %q (have %v)", want, paths)
		}
	}
}

func TestPhase110_ProfCommand_OutputFile(t *testing.T) {
	skipIfNoCompiler(t)
	f := writePhase110Fixture(t, phase110FixtureSrc)
	report := filepath.Join(t.TempDir(), "report.json")
	res := ProfCommand(f, "json", report, "go", "", false)
	if res.ExitCode != ExitSuccess {
		t.Fatalf("ProfCommand failed: [%d] %s", res.ExitCode, res.Message)
	}
	data, err := os.ReadFile(report)
	if err != nil {
		t.Fatalf("report file missing: %v", err)
	}
	if !strings.Contains(string(data), `"schema": "karkain-profile-v1"`) {
		t.Errorf("report file does not contain a karkain-profile-v1 document")
	}
}

// TestPhase110_ProfCommand_Boundaries pins the Phase 110 boundary contract:
// no kcc profiling, no WASM profiling (explicit rejection, no silent native
// fallback), bad formats and missing files are usage errors.
func TestPhase110_ProfCommand_Boundaries(t *testing.T) {
	f := writePhase110Fixture(t, `func main() { print("x") }`)

	res := ProfCommand(f, "text", "", "kcc", "", false)
	if res.ExitCode != ExitFailure {
		t.Errorf("kcc engine: exit %d, want ExitFailure", res.ExitCode)
	} else if !strings.Contains(res.Message, "Go engine only") {
		t.Errorf("kcc rejection message unexpected: %q", res.Message)
	}

	res = ProfCommand(f, "text", "", "go", "wasm32-wasi", false)
	if res.ExitCode != ExitFailure {
		t.Errorf("wasm target: exit %d, want ExitFailure", res.ExitCode)
	} else if !strings.Contains(res.Message, "no silent fallback") {
		t.Errorf("wasm rejection message unexpected: %q", res.Message)
	}

	res = ProfCommand(f, "bogus", "", "go", "", false)
	if res.ExitCode != ExitUsage {
		t.Errorf("bad format: exit %d, want ExitUsage", res.ExitCode)
	}

	res = ProfCommand(filepath.Join(t.TempDir(), "missing.kark"), "text", "", "go", "", false)
	if res.ExitCode != ExitFailure {
		t.Errorf("missing file: exit %d, want ExitFailure", res.ExitCode)
	}
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}
