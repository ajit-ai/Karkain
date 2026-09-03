package npu

import (
	"fmt"
	"strings"
)

// Karkain-owned MLIR dialect string. This is NOT MLIR from LLVM proper — it is
// a textual dialect Karkain defines for kernel-level control, consumed only by
// internal lowering and (optionally) emitted to a real MLIR/LLVM pipeline.
const KarkainMLIRDialect = "karkain"

// MLIROp is a dialect operation signature.
type MLIROp struct {
	Name    string   // e.g. "karkain.matmul"
	Operands []string // SSA value names
	Result  string    // SSA result name
	Attrs   map[string]string // static attributes
}

// MLIRModule models a list of dialect ops (function-scoped).
type MLIRModule struct {
	FuncName string
	Ops      []*MLIROp
	Results  []string
}

// NewMLIRModule creates an empty module for the given function.
func NewMLIRModule(name string) *MLIRModule {
	return &MLIRModule{FuncName: name}
}

// Add appends an op.
func (m *MLIRModule) Add(op *MLIROp) {
	m.Ops = append(m.Ops, op)
}

// AddResult registers a function result name.
func (m *MLIRModule) AddResult(name string) {
	m.Results = append(m.Results, name)
}

// Emit renders the module as MLIR dialect text.
func (m *MLIRModule) Emit() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("module {\n  func.func @%s(", m.FuncName))
	for i, r := range m.Results {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("%" + r)
	}
	sb.WriteString(") {\n")
	for _, op := range m.Ops {
		var attrs string
		if len(op.Attrs) > 0 {
			var parts []string
			for k, v := range op.Attrs {
				parts = append(parts, k+" = "+v)
			}
			attrs = " {" + strings.Join(parts, ", ") + "}"
		}
		sb.WriteString(fmt.Sprintf("    %%_%s = %s(", op.Result, op.Name))
		for i, opd := range op.Operands {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString("%" + opd)
		}
		sb.WriteString(") : ()" + attrs + "\n")
	}
	sb.WriteString("  }\n}\n")
	return sb.String()
}
