package ssa

// Phase 53: optimization passes — backward-compatible module-level API.
// Delegates to the Phase 94 individual pass implementations.

// isTerminator returns true if the instruction is a block terminator.
func isTerminator(in Instr) bool {
	switch in.Op {
	case OpBr, OpJmp, OpRet:
		return true
	}
	return false
}

// FoldConstants folds pure ops over constants (delegates to FoldConstPass).
func (m *Module) FoldConstants() {
	p := FoldConstPass{}
	for _, fn := range m.Functions {
		p.Run(fn)
	}
}

// EliminateDeadCode removes dead instructions (delegates to DCESimplePass).
func (m *Module) EliminateDeadCode() {
	p := DCESimplePass{}
	for _, fn := range m.Functions {
		p.Run(fn)
	}
}
