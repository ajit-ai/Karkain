package sema

import (
	"fmt"

	"karkain/pkg/parser"
)

// Execution targets recognized by the @target(...) function attribute.
//
// Phase 98 implements `cpu` (the default, pre-existing pipeline) and `npu`
// (NPU dispatch with an always-available CPU fallback). Phase 138 adds `gpu`
// (GPU/WGSL compute kernels with compile-only guarantee). The architecture leaves
// room for future targets (dsp, ipu, dpu, neuromorphic) without changing
// the language syntax: a new target is added by extending TargetNames and the
// NPU/codegen dispatch layer — never by grammar changes.
const (
	TargetCPU = "cpu"
	TargetNPU = "npu"
	TargetGPU = "gpu" // Phase 138: GPU/WGSL compute kernels
)

// TargetNames is the set of supported execution target names.
var TargetNames = []string{TargetCPU, TargetNPU, TargetGPU}

// ValidTarget reports whether name is a supported execution target. The empty
// string means "no explicit target" (the default CPU pipeline), which is valid.
func ValidTarget(name string) bool {
	switch name {
	case "", TargetCPU, TargetNPU, TargetGPU:
		return true
	}
	return false
}

// NPUError is a semantic error raised for an invalid @target(...) attribute.
type NPUError struct {
	Line int
	Col  int
	Msg  string
}

func (e NPUError) Error() string { return e.Msg }

// NPUAnalyzer performs static semantic analysis of @target(...) function
// attributes: it validates the target name and flags malformed/unsupported
// declarations (Phase 98). Placement validation happens in the parser
// (@target(...) may only prefix a func declaration).
type NPUAnalyzer struct {
	errors []NPUError
}

// NewNPUAnalyzer creates a target-attribute analyzer.
func NewNPUAnalyzer() *NPUAnalyzer {
	return &NPUAnalyzer{}
}

// Analyze validates every @target(...) function attribute in the program and
// returns the collected semantic errors. Programs without the attribute are
// untouched: plain functions keep the normal CPU pipeline.
func (a *NPUAnalyzer) Analyze(prog *parser.Program) []NPUError {
	a.errors = nil
	for _, stmt := range prog.Statements {
		fn, ok := stmt.(*parser.FuncDecl)
		if !ok || fn.Target == "" {
			continue
		}
		a.validate(fn)
	}
	return a.errors
}

// validate checks a single targeted function.
func (a *NPUAnalyzer) validate(fn *parser.FuncDecl) {
	if ValidTarget(fn.Target) {
		return
	}
	a.errors = append(a.errors, NPUError{
		Line: fn.Line,
		Col:  fn.Col,
		Msg:  fmt.Sprintf("@target(%s): unknown execution target; supported targets: cpu, npu, gpu", fn.Target),
	})
}
