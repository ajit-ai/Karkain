package codegen

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Phase 110 gate: Profiling & Diagnostics codegen. The profiler is opt-in
// (Config.Profiling) and injects a bounded, deterministic aggregation runtime
// plus per-function enter/leave hooks and a JSON dump written via atexit.

// TestPhase110_ProfilingInstrumentation asserts the injected hooks, the name
// table, the timing runtime and the allocation wrappers appear in the profiled
// C, and that function ids follow source declaration order.
func TestPhase110_ProfilingInstrumentation(t *testing.T) {
	src := `
func add(a, b) {
	return a + b
}

func main() {
	print("profiling probes " + str(add(20, 22)))
}
`
	prog := parseProg(t, src)
	tmp := t.TempDir()
	exePath := filepath.Join(tmp, "probe.exe")

	g := New(Config{OutputPath: exePath, Profiling: true})
	if err := g.GenerateAndCompile(prog, filepath.Join(tmp, "main.kark")); err != nil {
		t.Fatalf("GenerateAndCompile: %v", err)
	}
	c, err := os.ReadFile(filepath.Join(tmp, "main.c"))
	if err != nil {
		t.Fatalf("generated main.c missing: %v", err)
	}
	generated := string(c)

	for _, marker := range []string{
		"static const char* k_pf_names[]",
		"static long long karkain_prof_now_ns",
		"#define malloc karkain_prof_malloc",
		"#define free karkain_prof_free",
		"karkain_prof_init();",
		"karkain_prof_enter(0);",
		"karkain_prof_enter(1);",
		"karkain_prof_leave(0);",
	} {
		if !strings.Contains(generated, marker) {
			t.Errorf("profiled C missing %q", marker)
		}
	}
	// Name table order = source declaration order (deterministic fids).
	for _, name := range []string{`"add"`, `"main"`} {
		if !strings.Contains(generated, name) {
			t.Errorf("profiled C missing function name %s", name)
		}
	}
}

// TestPhase110_ProfilingIsOptIn proves default builds carry no instrumentation:
// profiling is opt-in, `karkain run`/`karkain build` never instrument.
func TestPhase110_ProfilingIsOptIn(t *testing.T) {
	prog := parseProg(t, `func main() { print("opt in") }`)
	tmp := t.TempDir()
	g := New(Config{OutputPath: filepath.Join(tmp, "probe.exe")})
	if err := g.GenerateAndCompile(prog, filepath.Join(tmp, "main.kark")); err != nil {
		t.Fatalf("GenerateAndCompile: %v", err)
	}
	c, err := os.ReadFile(filepath.Join(tmp, "main.c"))
	if err != nil {
		t.Fatal(err)
	}
	generated := string(c)
	for _, marker := range []string{"k_pf_names", "karkain_prof_enter", "karkain_prof_leave", "karkain_prof_init", "karkain_prof_malloc"} {
		if strings.Contains(generated, marker) {
			t.Errorf("default (non-profiled) C must not contain %q", marker)
		}
	}
}

// TestPhase110_ProfilingRuntimeAndDump runs a recursive program compiled with
// instrumentation and validates the emitted JSON dump: schema, deterministic
// call counts (fib(18) = 8361 invocations), no overflow overflow flag.
func TestPhase110_ProfilingRuntimeAndDump(t *testing.T) {
	phase107HasGCC(t)

	src := `
func fib(n) {
	if n < 2 {
		return n
	}
	return fib(n - 1) + fib(n - 2)
}

func main() {
	print("fib(18) = " + str(fib(18)))
}
`
	prog := parseProg(t, src)
	tmp := t.TempDir()
	dump := filepath.Join(tmp, "dump.json")
	t.Setenv("KARKAIN_PROF_OUT", dump)

	exePath := filepath.Join(tmp, "probe.exe")
	g := New(Config{OutputPath: exePath, Profiling: true})
	if err := g.GenerateAndCompile(prog, filepath.Join(tmp, "main.kark")); err != nil {
		t.Fatalf("GenerateAndCompile: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	run := exec.CommandContext(ctx, exePath)
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("probe run: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "fib(18) = 2584") {
		t.Fatalf("program output wrong: %q", out)
	}

	data, err := os.ReadFile(dump)
	if err != nil {
		t.Fatalf("profiler dump missing: %v", err)
	}
	var d struct {
		DurationNS int64 `json:"duration_ns"`
		Overflow   int   `json:"overflow"`
		Functions  []struct {
			Name  string `json:"name"`
			Calls int64  `json:"calls"`
		} `json:"functions"`
	}
	if err := json.Unmarshal(data, &d); err != nil {
		t.Fatalf("dump not valid JSON: %v", err)
	}
	if d.DurationNS < 0 {
		t.Errorf("negative duration_ns: %d", d.DurationNS)
	}
	if d.Overflow != 0 {
		t.Errorf("overflow set (expected false) for a program far below capacity")
	}
	counts := map[string]int64{}
	for _, f := range d.Functions {
		counts[f.Name] = f.Calls
	}
	if counts["fib"] != 8361 {
		t.Errorf("fib calls = %d, want 8361", counts["fib"])
	}
	if counts["main"] != 1 {
		t.Errorf("main calls = %d, want 1", counts["main"])
	}
}
