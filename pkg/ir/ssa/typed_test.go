package ssa

import (
	"fmt"
	"testing"
)

// ─── Typed SSA Tests ─────────────────────────────────────────────

func TestTyped_Registry(t *testing.T) {
	r := NewTypeRegistry()

	tests := []struct {
		input    string
		expected Type
	}{
		{"void", Void},
		{"i8", I},
		{"i16", I},
		{"i32", I},
		{"i64", I},
		{"f16", F},
		{"f32", F},
		{"f64", F},
		{"bool", Bool},
		{"str", Str},
		{"string", Str},
		{"ptr", Ptr},
		{"unknown", Value},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := r.ResolveType(tt.input)
			if result != tt.expected {
				t.Errorf("ResolveType(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTyped_TypeBits(t *testing.T) {
	tests := []struct {
		input    Type
		expected int
	}{
		{I, 64},
		{F, 64},
		{Bool, 1},
		{Ptr, 64},
		{Str, 0},
		{Value, 0},
		{Void, 0},
	}

	for _, tt := range tests {
		t.Run(tt.input.String(), func(t *testing.T) {
			result := TypeBits(tt.input)
			if result != tt.expected {
				t.Errorf("TypeBits(%v) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTyped_TypePromote(t *testing.T) {
	tests := []struct {
		a, b     Type
		expected Type
	}{
		{I, I, I},
		{F, F, F},
		{I, F, F},
		{F, I, F},
		{I, Value, Value},
		{Value, I, Value},
		{Bool, I, Value},
	}

	for _, tt := range tests {
		name := tt.a.String() + "+" + tt.b.String()
		t.Run(name, func(t *testing.T) {
			result := TypePromote(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("TypePromote(%v, %v) = %v, want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestTyped_TypeIsCompatible(t *testing.T) {
	tests := []struct {
		a, b     Type
		expected bool
	}{
		{I, I, true},
		{F, F, true},
		{I, F, false},
		{I, Value, true},
		{Value, F, true},
		{Bool, Bool, true},
		{Bool, I, false},
	}

	for _, tt := range tests {
		name := tt.a.String() + "+" + tt.b.String()
		t.Run(name, func(t *testing.T) {
			result := TypeIsCompatible(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("TypeIsCompatible(%v, %v) = %v, want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

// ─── Dominance Tests ─────────────────────────────────────────────

func TestDomTree_Diamond(t *testing.T) {
	fn := NewFunction("diamond")
	left := fn.NewBlock("left")
	right := fn.NewBlock("right")
	merge := fn.NewBlock("merge")

	fn.Entry.Emit(Instr{
		Op:          OpBr,
		Args:        []Operand{ConstBool(true)},
		ThenTarget:  left.Name,
		ElseTarget:  right.Name,
		BranchArgs:  [][]Operand{{}, {}},
	})
	left.Emit(Instr{Op: OpJmp, JmpTarget: merge.Name})
	right.Emit(Instr{Op: OpJmp, JmpTarget: merge.Name})
	merge.Emit(Instr{Op: OpRet})

	dt := BuildDominatorTree(fn)

	// Entry dominates all blocks
	if !dt.Dominates("entry", left.Name) {
		t.Error("entry should dominate left")
	}
	if !dt.Dominates("entry", right.Name) {
		t.Error("entry should dominate right")
	}
	if !dt.Dominates("entry", merge.Name) {
		t.Error("entry should dominate merge")
	}

	// Left and right do NOT dominate merge (there's an alternative path)
	if dt.Dominates(left.Name, merge.Name) {
		t.Error("left should NOT dominate merge")
	}
	if dt.Dominates(right.Name, merge.Name) {
		t.Error("right should NOT dominate merge")
	}

	// Left and right do NOT dominate each other
	if dt.Dominates(left.Name, right.Name) {
		t.Error("left should NOT dominate right")
	}
	if dt.Dominates(right.Name, left.Name) {
		t.Error("right should NOT dominate left")
	}
}

func TestDomTree_LinearChain(t *testing.T) {
	fn := NewFunction("linear")
	b1 := fn.NewBlock("b1")
	b2 := fn.NewBlock("b2")
	b3 := fn.NewBlock("b3")

	fn.Entry.Emit(Instr{Op: OpJmp, JmpTarget: b1.Name})
	b1.Emit(Instr{Op: OpJmp, JmpTarget: b2.Name})
	b2.Emit(Instr{Op: OpJmp, JmpTarget: b3.Name})
	b3.Emit(Instr{Op: OpRet})

	dt := BuildDominatorTree(fn)

	// In a linear chain, each block dominates all subsequent blocks
	if !dt.Dominates("entry", b1.Name) {
		t.Error("entry should dominate b1")
	}
	if !dt.Dominates("entry", b2.Name) {
		t.Error("entry should dominate b2")
	}
	if !dt.Dominates("entry", b3.Name) {
		t.Error("entry should dominate b3")
	}
	if !dt.Dominates(b1.Name, b2.Name) {
		t.Error("b1 should dominate b2")
	}
	if !dt.Dominates(b1.Name, b3.Name) {
		t.Error("b1 should dominate b3")
	}
	if !dt.Dominates(b2.Name, b3.Name) {
		t.Error("b2 should dominate b3")
	}
}

func TestDomTree_Loop(t *testing.T) {
	fn := NewFunction("loop")
	loopBody := fn.NewBlock("body")
	loopEnd := fn.NewBlock("end")

	fn.Entry.Emit(Instr{Op: OpJmp, JmpTarget: loopBody.Name})
	loopBody.Emit(Instr{
		Op:          OpBr,
		Args:        []Operand{ConstBool(true)},
		ThenTarget:  loopBody.Name,
		ElseTarget:  loopEnd.Name,
		BranchArgs:  [][]Operand{{}, {}},
	})
	loopEnd.Emit(Instr{Op: OpRet})

	dt := BuildDominatorTree(fn)

	if !dt.Dominates("entry", loopBody.Name) {
		t.Error("entry should dominate body")
	}
	if !dt.Dominates("entry", loopEnd.Name) {
		t.Error("entry should dominate end")
	}
	if !dt.Dominates(loopBody.Name, loopEnd.Name) {
		t.Error("body should dominate end")
	}
}

func TestDomTree_StrictlyDominates(t *testing.T) {
	fn := NewFunction("diamond")
	left := fn.NewBlock("left")
	right := fn.NewBlock("right")
	merge := fn.NewBlock("merge")

	fn.Entry.Emit(Instr{
		Op:          OpBr,
		Args:        []Operand{ConstBool(true)},
		ThenTarget:  left.Name,
		ElseTarget:  right.Name,
		BranchArgs:  [][]Operand{{}, {}},
	})
	left.Emit(Instr{Op: OpJmp, JmpTarget: merge.Name})
	right.Emit(Instr{Op: OpJmp, JmpTarget: merge.Name})
	merge.Emit(Instr{Op: OpRet})

	dt := BuildDominatorTree(fn)

	// A block should not strictly dominate itself
	if dt.StrictlyDominates("entry", "entry") {
		t.Error("entry should not strictly dominate itself")
	}

	// But entry should strictly dominate left
	if !dt.StrictlyDominates("entry", left.Name) {
		t.Error("entry should strictly dominate left")
	}
}

func TestDomTree_DomTreeDepth(t *testing.T) {
	fn := NewFunction("depth")
	b1 := fn.NewBlock("b1")
	b2 := fn.NewBlock("b2")

	fn.Entry.Emit(Instr{Op: OpJmp, JmpTarget: b1.Name})
	b1.Emit(Instr{Op: OpJmp, JmpTarget: b2.Name})
	b2.Emit(Instr{Op: OpRet})

	dt := BuildDominatorTree(fn)

	if dt.DomTreeDepth("entry") != 0 {
		t.Errorf("entry depth = %d, want 0", dt.DomTreeDepth("entry"))
	}
	if dt.DomTreeDepth(b1.Name) != 1 {
		t.Errorf("%s depth = %d, want 1", b1.Name, dt.DomTreeDepth(b1.Name))
	}
	if dt.DomTreeDepth(b2.Name) != 2 {
		t.Errorf("%s depth = %d, want 2", b2.Name, dt.DomTreeDepth(b2.Name))
	}
}

func TestDomTree_ComplexCFG(t *testing.T) {
	fn := NewFunction("complex")
	a := fn.NewBlock("a")
	b := fn.NewBlock("b")
	c := fn.NewBlock("c")
	d := fn.NewBlock("d")
	e := fn.NewBlock("e")

	// entry -> a, entry -> c
	fn.Entry.Emit(Instr{
		Op:          OpBr,
		Args:        []Operand{ConstBool(true)},
		ThenTarget:  a.Name,
		ElseTarget:  c.Name,
		BranchArgs:  [][]Operand{{}, {}},
	})
	a.Emit(Instr{Op: OpJmp, JmpTarget: b.Name})
	b.Emit(Instr{Op: OpJmp, JmpTarget: d.Name})
	c.Emit(Instr{Op: OpJmp, JmpTarget: d.Name})
	d.Emit(Instr{Op: OpJmp, JmpTarget: e.Name})
	e.Emit(Instr{Op: OpRet})

	dt := BuildDominatorTree(fn)

	// Entry dominates everything
	if !dt.Dominates("entry", a.Name) {
		t.Error("entry should dominate a")
	}
	if !dt.Dominates("entry", d.Name) {
		t.Error("entry should dominate d")
	}
	if !dt.Dominates("entry", e.Name) {
		t.Error("entry should dominate e")
	}

	// A dominates b (only path to b goes through a)
	if !dt.Dominates(a.Name, b.Name) {
		t.Error("a should dominate b")
	}

	// D dominates e (only path to e goes through d)
	if !dt.Dominates(d.Name, e.Name) {
		t.Error("d should dominate e")
	}
}

func TestDomTree_PrintIDom(t *testing.T) {
	fn := NewFunction("test")
	left := fn.NewBlock("left")
	right := fn.NewBlock("right")
	merge := fn.NewBlock("merge")

	fn.Entry.Emit(Instr{
		Op:          OpBr,
		Args:        []Operand{ConstBool(true)},
		ThenTarget:  left.Name,
		ElseTarget:  right.Name,
		BranchArgs:  [][]Operand{{}, {}},
	})
	left.Emit(Instr{Op: OpJmp, JmpTarget: merge.Name})
	right.Emit(Instr{Op: OpJmp, JmpTarget: merge.Name})
	merge.Emit(Instr{Op: OpRet})

	dt := BuildDominatorTree(fn)
	t.Logf("IDom map: %v", dt.IDom)
	t.Logf("DomTree:\n%s", dt.String())

	for _, b := range fn.Blocks {
		t.Logf("Block %s: idom=%s, depth=%d", b.Name, dt.IDom[b.Name], dt.DomTreeDepth(b.Name))
	}
}

// ─── Liveness Tests ──────────────────────────────────────────────

func TestLiveness_SimpleDefUse(t *testing.T) {
	fn := NewFunction("defuse")
	b := fn.Entry

	b.Emit(Instr{Op: OpConst, Dest: "x", Ty: I, Args: []Operand{ConstInt(42)}})
	b.Emit(Instr{Op: OpPrint, Args: []Operand{Reg("x")}})
	b.Emit(Instr{Op: OpRet})

	li := ComputeLiveness(fn)

	if li.TotalRegs != 1 {
		t.Errorf("expected 1 register, got %d", li.TotalRegs)
	}

	if !li.LiveIn["entry"]["x"] {
		t.Error("x should be live at entry")
	}
}

func TestLiveness_NoLiveRegisters(t *testing.T) {
	fn := NewFunction("empty")
	fn.Entry.Emit(Instr{Op: OpRet})

	li := ComputeLiveness(fn)

	if li.TotalRegs != 0 {
		t.Errorf("expected 0 registers, got %d", li.TotalRegs)
	}
}

func TestLiveness_MultipleRegs(t *testing.T) {
	fn := NewFunction("multi")
	b := fn.Entry

	b.Emit(Instr{Op: OpConst, Dest: "a", Ty: I, Args: []Operand{ConstInt(1)}})
	b.Emit(Instr{Op: OpConst, Dest: "b", Ty: I, Args: []Operand{ConstInt(2)}})
	b.Emit(Instr{Op: OpBinOp, Dest: "c", Ty: I, OpStr: "+", Args: []Operand{Reg("a"), Reg("b")}})
	b.Emit(Instr{Op: OpPrint, Args: []Operand{Reg("c")}})
	b.Emit(Instr{Op: OpRet})

	li := ComputeLiveness(fn)

	if li.TotalRegs != 3 {
		t.Errorf("expected 3 registers, got %d", li.TotalRegs)
	}
}

func TestLiveness_CrossBlock(t *testing.T) {
	fn := NewFunction("cross")
	b1 := fn.NewBlock("b1")

	fn.Entry.Emit(Instr{Op: OpJmp, JmpTarget: b1.Name})
	b1.Emit(Instr{Op: OpConst, Dest: "x", Ty: I, Args: []Operand{ConstInt(42)}})
	b1.Emit(Instr{Op: OpPrint, Args: []Operand{Reg("x")}})
	b1.Emit(Instr{Op: OpRet})

	li := ComputeLiveness(fn)

	if !li.LiveIn[b1.Name]["x"] {
		t.Error("x should be live at b1 entry")
	}
}

func TestLiveness_DefBeforeUse(t *testing.T) {
	fn := NewFunction("defbeforeuse")
	b := fn.Entry

	b.Emit(Instr{Op: OpConst, Dest: "x", Ty: I, Args: []Operand{ConstInt(1)}})
	b.Emit(Instr{Op: OpBinOp, Dest: "y", Ty: I, OpStr: "+", Args: []Operand{Reg("x"), ConstInt(2)}})
	b.Emit(Instr{Op: OpPrint, Args: []Operand{Reg("y")}})
	b.Emit(Instr{Op: OpRet})

	li := ComputeLiveness(fn)

	if li.TotalRegs != 2 {
		t.Errorf("expected 2 registers, got %d", li.TotalRegs)
	}
}

// ─── Dominance Frontier Tests ────────────────────────────────────

func TestDomTree_DominanceFrontier(t *testing.T) {
	fn := NewFunction("diamond")
	left := fn.NewBlock("left")
	right := fn.NewBlock("right")
	merge := fn.NewBlock("merge")

	fn.Entry.Emit(Instr{
		Op:          OpBr,
		Args:        []Operand{ConstBool(true)},
		ThenTarget:  left.Name,
		ElseTarget:  right.Name,
		BranchArgs:  [][]Operand{{}, {}},
	})
	left.Emit(Instr{Op: OpJmp, JmpTarget: merge.Name})
	right.Emit(Instr{Op: OpJmp, JmpTarget: merge.Name})
	merge.Emit(Instr{Op: OpRet})

	dt := BuildDominatorTree(fn)

	// Entry's dominance frontier should be empty
	df := dt.DominanceFrontier(fn, "entry")
	if len(df) != 0 {
		t.Errorf("entry dominance frontier should be empty, got %v", df)
	}

	// Left's dominance frontier should include merge
	df = dt.DominanceFrontier(fn, left.Name)
	if !df[merge.Name] {
		t.Errorf("left dominance frontier should include merge, got %v", df)
	}

	// Right's dominance frontier should include merge
	df = dt.DominanceFrontier(fn, right.Name)
	if !df[merge.Name] {
		t.Errorf("right dominance frontier should include merge, got %v", df)
	}
}

// ─── SSA Verifier ────────────────────────────────────────────────

func VerifySSA(fn *Function) []string {
	var errors []string
	blockNames := make(map[string]bool)
	for _, b := range fn.Blocks {
		blockNames[b.Name] = true
	}

	defined := make(map[string]bool)
	for _, p := range fn.Params {
		defined[p.Name] = true
	}

	for _, b := range fn.Blocks {
		for _, p := range b.Params {
			defined[p.Name] = true
		}

		for _, in := range b.Instrs {
			for _, arg := range in.Args {
				if !arg.IsCst && arg.Reg != "" && !defined[arg.Reg] {
					errors = append(errors, fmt.Sprintf("block %s: use of undefined register %%%s", b.Name, arg.Reg))
				}
			}

			if in.Op == OpBr {
				if !blockNames[in.ThenTarget] {
					errors = append(errors, fmt.Sprintf("block %s: branch to undefined block %s", b.Name, in.ThenTarget))
				}
				if !blockNames[in.ElseTarget] {
					errors = append(errors, fmt.Sprintf("block %s: branch to undefined block %s", b.Name, in.ElseTarget))
				}
			}
			if in.Op == OpJmp {
				if !blockNames[in.JmpTarget] {
					errors = append(errors, fmt.Sprintf("block %s: jump to undefined block %s", b.Name, in.JmpTarget))
				}
			}

			if in.Dest != "" {
				defined[in.Dest] = true
			}
		}

		if len(b.Instrs) > 0 && !b.Terminated() {
			errors = append(errors, fmt.Sprintf("block %s: not terminated", b.Name))
		}
	}

	return errors
}

func TestSSAVerifier_ValidProgram(t *testing.T) {
	fn := NewFunction("valid")
	left := fn.NewBlock("left")
	right := fn.NewBlock("right")
	merge := fn.NewBlock("merge")

	fn.Entry.Emit(Instr{
		Op:          OpBr,
		Args:        []Operand{ConstBool(true)},
		ThenTarget:  left.Name,
		ElseTarget:  right.Name,
		BranchArgs:  [][]Operand{{}, {}},
	})
	left.Emit(Instr{Op: OpJmp, JmpTarget: merge.Name})
	right.Emit(Instr{Op: OpJmp, JmpTarget: merge.Name})
	merge.Emit(Instr{Op: OpRet})

	errs := VerifySSA(fn)
	if len(errs) > 0 {
		t.Errorf("SSA verifier found errors: %v", errs)
	}
}

func TestSSAVerifier_UndefinedReg(t *testing.T) {
	fn := NewFunction("undef")
	fn.Entry.Emit(Instr{
		Op:   OpPrint,
		Args: []Operand{Reg("nonexistent")},
	})
	fn.Entry.Emit(Instr{Op: OpRet})

	errs := VerifySSA(fn)
	if len(errs) == 0 {
		t.Error("expected errors for undefined register")
	}
}

func TestSSAVerifier_UnterminatedBlock(t *testing.T) {
	fn := NewFunction("unterm")
	fn.Entry.Emit(Instr{Op: OpConst, Dest: "x", Ty: I, Args: []Operand{ConstInt(1)}})

	errs := VerifySSA(fn)
	if len(errs) == 0 {
		t.Error("expected errors for unterminated block")
	}
}

func TestSSAVerifier_UndefinedBlockTarget(t *testing.T) {
	fn := NewFunction("badtarget")
	fn.Entry.Emit(Instr{
		Op:          OpBr,
		Args:        []Operand{ConstBool(true)},
		ThenTarget:  "nonexistent",
		ElseTarget:  "also_nonexistent",
		BranchArgs:  [][]Operand{{}, {}},
	})

	errs := VerifySSA(fn)
	if len(errs) == 0 {
		t.Error("expected errors for undefined block target")
	}
}
