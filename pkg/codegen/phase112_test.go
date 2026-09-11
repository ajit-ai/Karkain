package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"karkain/pkg/parser"
)

// Phase 112 gate: the float() conversion builtin must lower to the
// karkain_float C helper for every primitive input kind (float passthrough,
// int widening, string parse) and the generated header must define the helper.
// Separately, the Config.Trace flag must gate the debug execution trace
// (Phase 112: karkain debug) so default builds carry no instrumentation.

func TestPhase112_HeaderHasFloatHelper(t *testing.T) {
	g := New(Config{})
	h := g.generateCHeader()
	if !strings.Contains(h, "karkain_float") {
		t.Error("generated header missing karkain_float helper")
	}
}

func TestPhase112_FloatBuiltinEmission(t *testing.T) {
	g := New(Config{})
	g.sourceFile = "probe.kark"

	// float(int literal) lowers to karkain_float(make_int(...)).
	out := g.genExpr(&parser.CallExpr{
		Function: "float",
		Args:     []parser.Node{&parser.IntLiteral{Value: "3"}},
		Line:     5,
	})
	want := "karkain_float(make_int(3))"
	if !strings.Contains(out, want) {
		t.Errorf("float(int) emission = %s; want %s", out, want)
	}

	// float(float literal) is the passthrough path.
	out = g.genExpr(&parser.CallExpr{
		Function: "float",
		Args:     []parser.Node{&parser.Float64Literal{Value: "2.5"}},
		Line:     6,
	})
	if !strings.Contains(out, "karkain_float(make_float(2.5))") {
		t.Errorf("float(float) emission = %s", out)
	}

	// float(string) is the string-parse path.
	out = g.genExpr(&parser.CallExpr{
		Function: "float",
		Args:     []parser.Node{&parser.StringLiteral{Value: "4.25"}},
		Line:     7,
	})
	if !strings.Contains(out, `karkain_float(make_string("4.25"))`) {
		t.Errorf("float(string) emission = %s", out)
	}
}

func TestPhase112_TraceGatedByConfig(t *testing.T) {
	prog := parseProg(t, `func main() { print("trace probe") }`)
	tmp := t.TempDir()

	g := New(Config{OutputPath: filepath.Join(tmp, "probe.exe")})
	if err := g.GenerateAndCompile(prog, filepath.Join(tmp, "main.kark")); err != nil {
		t.Fatalf("default GenerateAndCompile: %v", err)
	}
	c, err := os.ReadFile(filepath.Join(tmp, "main.c"))
	if err != nil {
		t.Fatalf("default main.c missing: %v", err)
	}
	if strings.Contains(string(c), "#define KARKAIN_TRACE 1") {
		t.Error("default build must not enable KARKAIN_TRACE (trace is opt-in)")
	}

	g = New(Config{OutputPath: filepath.Join(tmp, "probe2.exe"), Trace: true})
	if err := g.GenerateAndCompile(prog, filepath.Join(tmp, "main.kark")); err != nil {
		t.Fatalf("tracing GenerateAndCompile: %v", err)
	}
	c, err = os.ReadFile(filepath.Join(tmp, "main.c"))
	if err != nil {
		t.Fatalf("tracing main.c missing: %v", err)
	}
	trace := string(c)
	for _, marker := range []string{
		"#define KARKAIN_TRACE 1",
		"karkain_frame_enter",
		"karkain_frame_leave",
	} {
		if !strings.Contains(trace, marker) {
			t.Errorf("tracing C missing %q", marker)
		}
	}
	if !strings.Contains(trace, "karkain:%s:enter %s") || !strings.Contains(trace, "karkain:%s:leave %s") {
		t.Error("tracing C missing frame trace fprintf hooks")
	}
}