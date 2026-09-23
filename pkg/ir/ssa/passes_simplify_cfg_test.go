package ssa

import "testing"

// Phase 141: constant-branch folding + unreachable-block deletion.

func TestSimplifyCFG_FoldsConstantBranch(t *testing.T) {
	fn := NewFunction("br")
	thenB := fn.NewBlock("then")
	elseB := fn.NewBlock("else")
	joinB := fn.NewBlock("join")

	v := fn.NewReg("v")
	fn.Entry.Emit(Instr{Op: OpConst, Dest: v, Ty: I, Args: []Operand{ConstInt(1)}})
	fn.Entry.Emit(Instr{
		Op: OpBr, Args: []Operand{{IsCst: true, CstKind: Bool, BoolVal: false}},
		ThenTarget: thenB.Name, ElseTarget: elseB.Name,
		BranchArgs: [][]Operand{{}, {}},
	})
	thenB.Emit(Instr{Op: OpConst, Dest: fn.NewReg("t"), Ty: I, Args: []Operand{ConstInt(9)}})
	thenB.Emit(Instr{Op: OpJmp, JmpTarget: joinB.Name})
	elseB.Emit(Instr{Op: OpJmp, JmpTarget: joinB.Name})
	joinB.Emit(Instr{Op: OpRet})

	changed := SimplifyCFGPass{}.Run(fn)
	if !changed {
		t.Fatal("expected changes")
	}
	// Entry must end in a jmp to the else block now.
	last := fn.Entry.Instrs[len(fn.Entry.Instrs)-1]
	if last.Op != OpJmp || last.JmpTarget != elseB.Name {
		t.Fatalf("entry ends in %+v, want jmp else", last)
	}
	// The then block is unreachable and must be gone.
	for _, b := range fn.Blocks {
		if b.Name == thenB.Name {
			t.Fatalf("dead block %q survives", b.Name)
		}
	}
	if err := (&Module{Functions: []*Function{fn}}).Verify(); err != nil {
		t.Fatalf("verify after simplify: %v", err)
	}
}

func TestSimplifyCFG_KeepsLoops(t *testing.T) {
	fn := NewFunction("loop")
	hdr := fn.NewBlock("hdr")
	bdy := fn.NewBlock("bdy")
	ext := fn.NewBlock("ext")

	fn.Entry.Emit(Instr{Op: OpJmp, JmpTarget: hdr.Name})
	// Non-constant condition: nothing folds, nothing is deleted.
	c := fn.NewReg("c")
	hdr.Emit(Instr{Op: OpConst, Dest: fn.NewReg("one"), Ty: I, Args: []Operand{ConstInt(1)}})
	hdr.Emit(Instr{
		Op: OpBr, Args: []Operand{{Reg: c}},
		ThenTarget: bdy.Name, ElseTarget: ext.Name,
		BranchArgs: [][]Operand{{}, {}},
	})
	bdy.Emit(Instr{Op: OpJmp, JmpTarget: hdr.Name})
	ext.Emit(Instr{Op: OpRet})

	changedLoop := SimplifyCFGPass{}.Run(fn)
	if changedLoop {
		t.Fatal("expected no changes for a live loop")
	}
	if len(fn.Blocks) != 4 {
		t.Fatalf("blocks = %d, want 4", len(fn.Blocks))
	}
}

func TestSimplifyCFG_IntCondition(t *testing.T) {
	fn := NewFunction("intbr")
	thenB := fn.NewBlock("then")
	elseB := fn.NewBlock("else")
	fn.Entry.Emit(Instr{
		Op: OpBr, Args: []Operand{ConstInt(2)},
		ThenTarget: thenB.Name, ElseTarget: elseB.Name,
		BranchArgs: [][]Operand{{}, {}},
	})
	thenB.Emit(Instr{Op: OpRet})
	elseB.Emit(Instr{Op: OpRet})

	changed := SimplifyCFGPass{}.Run(fn)
	if !changed {
		t.Fatal("expected changes")
	}
	last := fn.Entry.Instrs[len(fn.Entry.Instrs)-1]
	if last.Op != OpJmp || last.JmpTarget != thenB.Name {
		t.Fatalf("nonzero int must take then, got %+v", last)
	}
}
