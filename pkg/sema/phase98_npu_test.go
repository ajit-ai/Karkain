package sema

import (
	"strings"
	"testing"

	"karkain/pkg/lexer"
	"karkain/pkg/parser"
)

// Phase 98: the @target(...) function attribute. These tests prove the NPU
// analyzer recognizes supported targets, rejects unknown ones, and leaves
// plain (CPU) functions untouched, plus end-to-end parsing that attaches the
// attribute to the FuncDecl node.

func parsePhase98Prog(t *testing.T, src string) *parser.Program {
	t.Helper()
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.ParseProgram()
	return prog
}

func TestPhase98_NPU_TargetRecognized(t *testing.T) {
	prog := &parser.Program{Statements: []parser.Node{
		&parser.FuncDecl{Name: "matmul", Target: TargetNPU, Line: 1},
	}}
	errs := NewNPUAnalyzer().Analyze(prog)
	if len(errs) != 0 {
		t.Fatalf("expected no errors for @target(npu), got %v", errs)
	}
}

func TestPhase98_NPU_CPUAndEmptyTargetsAccepted(t *testing.T) {
	prog := &parser.Program{Statements: []parser.Node{
		&parser.FuncDecl{Name: "cpu_fn", Target: TargetCPU, Line: 1},
		&parser.FuncDecl{Name: "default_fn", Line: 2}, // no attribute -> default CPU
	}}
	errs := NewNPUAnalyzer().Analyze(prog)
	if len(errs) != 0 {
		t.Fatalf("expected no errors for cpu/default targets, got %v", errs)
	}
}

func TestPhase98_NPU_UnknownTargetRejected(t *testing.T) {
	prog := &parser.Program{Statements: []parser.Node{
		&parser.FuncDecl{Name: "bad", Target: "tpu_turbo", Line: 7},
	}}
	errs := NewNPUAnalyzer().Analyze(prog)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Msg, "unknown execution target") {
		t.Errorf("message = %q, want unknown-execution-target wording", errs[0].Msg)
	}
	if !strings.Contains(errs[0].Msg, "cpu, npu") {
		t.Errorf("message = %q, want supported-target list", errs[0].Msg)
	}
	if errs[0].Line != 7 {
		t.Errorf("error line = %d, want 7", errs[0].Line)
	}
}

func TestPhase98_NPU_PlainProgramUntouched(t *testing.T) {
	prog := parsePhase98Prog(t, "func plain() { return 1 }\nfunc main() { plain() }\n")
	if len(prog.Statements) < 2 {
		t.Fatalf("expected 2 func decls, got %d", len(prog.Statements))
	}
	errs := NewNPUAnalyzer().Analyze(prog)
	if len(errs) != 0 {
		t.Fatalf("expected no errors for a plain program, got %v", errs)
	}
}

func TestPhase98_NPU_ParsedAttributeAttached(t *testing.T) {
	prog := parsePhase98Prog(t, "@target(npu)\nfunc matmul(a int, b int) {\n    return a\n}\n")
	if len(prog.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Statements))
	}
	fn, ok := prog.Statements[0].(*parser.FuncDecl)
	if !ok {
		t.Fatalf("statement is %T, want *parser.FuncDecl", prog.Statements[0])
	}
	if fn.Target != TargetNPU {
		t.Errorf("Target = %q, want %q", fn.Target, TargetNPU)
	}
}

func TestPhase98_NPU_ParsedAttributeInvalidTargetStillAttached(t *testing.T) {
	prog := parsePhase98Prog(t, "@target(fpgax1000)\nfunc f() {}\n")
	if len(prog.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Statements))
	}
	fn := prog.Statements[0].(*parser.FuncDecl)
	if fn.Target != "fpgax1000" {
		t.Errorf("Target = %q, want %q", fn.Target, "fpgax1000")
	}
	errs := NewNPUAnalyzer().Analyze(prog)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
}

func TestPhase98_NPU_AttributeMustPrecedeFunc(t *testing.T) {
	prog := parsePhase98Prog(t, "@target(npu)\nlet x = 1\n")
	// Placement is a parser concern; the program parses and the analyzer must
	// not crash or attach the attribute to the var decl.
	errs := NewNPUAnalyzer().Analyze(prog)
	if len(errs) != 0 {
		t.Fatalf("expected no analyzer errors, got %v", errs)
	}
}

func TestPhase98_NPU_ValidTargetTable(t *testing.T) {
	// Phase 138: "gpu" graduated to a valid target (WGSL kernels); the
	// reject list keeps the never-supported names.
	if !ValidTarget("") || !ValidTarget(TargetCPU) || !ValidTarget(TargetNPU) || !ValidTarget(TargetGPU) {
		t.Errorf("ValidTarget must accept %q, %q, %q and %q", "", TargetCPU, TargetNPU, TargetGPU)
	}
	for _, name := range []string{"tpu", "google", "intel", "foo"} {
		if ValidTarget(name) {
			t.Errorf("ValidTarget(%q) = true, want false", name)
		}
	}
}