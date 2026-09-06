package ssa

import (
	"testing"
)

// ─── FoldConstPass Tests ─────────────────────────────────────────

func TestFoldConst_IntArithmetic(t *testing.T) {
	fn := NewFunction("fold_int")
	a := fn.NewReg("a")
	b := fn.NewReg("b")
	fn.Entry.Emit(Instr{Op: OpBinOp, Dest: a, Ty: I, Args: []Operand{ConstInt(3), ConstInt(4)}, OpStr: "+"})
	fn.Entry.Emit(Instr{Op: OpBinOp, Dest: b, Ty: I, Args: []Operand{Reg(a), ConstInt(2)}, OpStr: "*"})
	fn.Entry.Emit(Instr{Op: OpRet, Args: []Operand{Reg(b)}})

	FoldConstPass{}.Run(fn)

	// 3+4=7, then 7*2=14
	if fn.Entry.Instrs[0].Op != OpConst || fn.Entry.Instrs[0].Args[0].IntVal != 7 {
		t.Errorf("expected 3+4 folded to 7, got %v", fn.Entry.Instrs[0])
	}
	if fn.Entry.Instrs[1].Op != OpConst || fn.Entry.Instrs[1].Args[0].IntVal != 14 {
		t.Errorf("expected 7*2 folded to 14, got %v", fn.Entry.Instrs[1])
	}
}

func TestFoldConst_FltArithmetic(t *testing.T) {
	fn := NewFunction("fold_flt")
	a := fn.NewReg("a")
	fn.Entry.Emit(Instr{Op: OpBinOp, Dest: a, Ty: F, Args: []Operand{ConstFlt(2.5), ConstFlt(3.5)}, OpStr: "+"})
	fn.Entry.Emit(Instr{Op: OpRet, Args: []Operand{Reg(a)}})

	FoldConstPass{}.Run(fn)

	if fn.Entry.Instrs[0].Op != OpConst || fn.Entry.Instrs[0].Args[0].FltVal != 6.0 {
		t.Errorf("expected 2.5+3.5 folded to 6.0, got %v", fn.Entry.Instrs[0])
	}
}

func TestFoldConst_BoolLogic(t *testing.T) {
	fn := NewFunction("fold_bool")
	a := fn.NewReg("a")
	b := fn.NewReg("b")
	fn.Entry.Emit(Instr{Op: OpBinOp, Dest: a, Ty: Bool, Args: []Operand{ConstBool(true), ConstBool(false)}, OpStr: "&&"})
	fn.Entry.Emit(Instr{Op: OpBinOp, Dest: b, Ty: Bool, Args: []Operand{ConstBool(true), ConstBool(false)}, OpStr: "||"})
	fn.Entry.Emit(Instr{Op: OpRet, Args: []Operand{Reg(a)}})

	FoldConstPass{}.Run(fn)

	if fn.Entry.Instrs[0].Op != OpConst || fn.Entry.Instrs[0].Args[0].BoolVal != false {
		t.Errorf("expected true&&false=false, got %v", fn.Entry.Instrs[0])
	}
	if fn.Entry.Instrs[1].Op != OpConst || fn.Entry.Instrs[1].Args[0].BoolVal != true {
		t.Errorf("expected true||false=true, got %v", fn.Entry.Instrs[1])
	}
}

func TestFoldConst_UnaryNeg(t *testing.T) {
	fn := NewFunction("fold_neg")
	a := fn.NewReg("a")
	fn.Entry.Emit(Instr{Op: OpUnOp, Dest: a, Ty: I, Args: []Operand{ConstInt(42)}, OpStr: "-"})
	fn.Entry.Emit(Instr{Op: OpRet, Args: []Operand{Reg(a)}})

	FoldConstPass{}.Run(fn)

	if fn.Entry.Instrs[0].Op != OpConst || fn.Entry.Instrs[0].Args[0].IntVal != -42 {
		t.Errorf("expected -42 folded, got %v", fn.Entry.Instrs[0])
	}
}

func TestFoldConst_UnaryNot(t *testing.T) {
	fn := NewFunction("fold_not")
	a := fn.NewReg("a")
	fn.Entry.Emit(Instr{Op: OpUnOp, Dest: a, Ty: Bool, Args: []Operand{ConstBool(true)}, OpStr: "!"})
	fn.Entry.Emit(Instr{Op: OpRet, Args: []Operand{Reg(a)}})

	FoldConstPass{}.Run(fn)

	if fn.Entry.Instrs[0].Op != OpConst || fn.Entry.Instrs[0].Args[0].BoolVal != false {
		t.Errorf("expected !true=false, got %v", fn.Entry.Instrs[0])
	}
}

func TestFoldConst_AlgebraicIdentities(t *testing.T) {
	fn := NewFunction("fold_id")
	fn.Params = []BlockParam{{Name: "x_0", Ty: I}}
	x := fn.Params[0].Name
	a := fn.NewReg("a")
	b := fn.NewReg("b")
	c := fn.NewReg("c")
	// x + 0
	fn.Entry.Emit(Instr{Op: OpBinOp, Dest: a, Ty: I, Args: []Operand{Reg(x), ConstInt(0)}, OpStr: "+"})
	// x * 1
	fn.Entry.Emit(Instr{Op: OpBinOp, Dest: b, Ty: I, Args: []Operand{Reg(x), ConstInt(1)}, OpStr: "*"})
	// x * 0
	fn.Entry.Emit(Instr{Op: OpBinOp, Dest: c, Ty: I, Args: []Operand{Reg(x), ConstInt(0)}, OpStr: "*"})
	fn.Entry.Emit(Instr{Op: OpRet, Args: []Operand{Reg(a)}})

	FoldConstPass{}.Run(fn)

	// x + 0 → x (same op with one arg)
	if fn.Entry.Instrs[0].Op != OpBinOp || len(fn.Entry.Instrs[0].Args) != 1 {
		t.Errorf("expected x+0 simplified to x, got %v", fn.Entry.Instrs[0])
	}
	// x * 1 → x
	if fn.Entry.Instrs[1].Op != OpBinOp || len(fn.Entry.Instrs[1].Args) != 1 {
		t.Errorf("expected x*1 simplified to x, got %v", fn.Entry.Instrs[1])
	}
	// x * 0 → 0
	if fn.Entry.Instrs[2].Op != OpConst || fn.Entry.Instrs[2].Args[0].IntVal != 0 {
		t.Errorf("expected x*0=0, got %v", fn.Entry.Instrs[2])
	}
}

// ─── DCESimplePass Tests ─────────────────────────────────────────

func TestDCE_RemovesDead(t *testing.T) {
	fn := NewFunction("dce")
	dead := fn.NewReg("dead")
	live := fn.NewReg("live")
	fn.Entry.Emit(Instr{Op: OpConst, Dest: dead, Ty: I, Args: []Operand{ConstInt(1)}})
	fn.Entry.Emit(Instr{Op: OpConst, Dest: live, Ty: I, Args: []Operand{ConstInt(2)}})
	fn.Entry.Emit(Instr{Op: OpPrint, Args: []Operand{Reg(live)}})
	fn.Entry.Emit(Instr{Op: OpRet})

	DCESimplePass{}.Run(fn)

	if len(fn.Entry.Instrs) != 3 {
		t.Fatalf("expected 3 instrs (const, print, ret), got %d", len(fn.Entry.Instrs))
	}
}

func TestDCE_CascadingDead(t *testing.T) {
	fn := NewFunction("dce_chain")
	a := fn.NewReg("a")
	b := fn.NewReg("b") // uses a
	c := fn.NewReg("c") // uses b
	live := fn.NewReg("live")
	fn.Entry.Emit(Instr{Op: OpConst, Dest: a, Ty: I, Args: []Operand{ConstInt(1)}})
	fn.Entry.Emit(Instr{Op: OpBinOp, Dest: b, Ty: I, Args: []Operand{Reg(a), ConstInt(2)}, OpStr: "+"})
	fn.Entry.Emit(Instr{Op: OpBinOp, Dest: c, Ty: I, Args: []Operand{Reg(b), ConstInt(3)}, OpStr: "+"})
	fn.Entry.Emit(Instr{Op: OpConst, Dest: live, Ty: I, Args: []Operand{ConstInt(99)}})
	fn.Entry.Emit(Instr{Op: OpPrint, Args: []Operand{Reg(live)}})
	fn.Entry.Emit(Instr{Op: OpRet})

	DCESimplePass{}.Run(fn)

	if len(fn.Entry.Instrs) != 3 {
		t.Fatalf("expected 3 instrs (const, print, ret), got %d:\n%s",
			len(fn.Entry.Instrs), formatFn(fn))
	}
}

func TestDCE_KeepsSideEffects(t *testing.T) {
	fn := NewFunction("dce_side")
	dead := fn.NewReg("dead")
	unused := fn.NewReg("unused")
	fn.Entry.Emit(Instr{Op: OpConst, Dest: dead, Ty: I, Args: []Operand{ConstInt(1)}})
	fn.Entry.Emit(Instr{Op: OpConst, Dest: unused, Ty: I, Args: []Operand{ConstInt(99)}})
	fn.Entry.Emit(Instr{Op: OpCallVoid, OpStr: "external_func", Args: []Operand{Reg(dead)}})
	fn.Entry.Emit(Instr{Op: OpRet})

	DCESimplePass{}.Run(fn)

	// dead is used by the call, unused is dead. Expect: const, call, ret = 3
	if len(fn.Entry.Instrs) != 3 {
		t.Fatalf("expected 3 instrs (const, call, ret), got %d", len(fn.Entry.Instrs))
	}
	if fn.Entry.Instrs[1].Op != OpCallVoid {
		t.Errorf("expected call to survive, got %v", fn.Entry.Instrs[1])
	}
}

// ─── CSEPass Tests ───────────────────────────────────────────────

func TestCSE_DuplicateBinOps(t *testing.T) {
	fn := NewFunction("cse_dup")
	fn.Params = []BlockParam{{Name: "x_0", Ty: I}}
	x := fn.Params[0].Name
	a := fn.NewReg("a")
	b := fn.NewReg("b") // same as a
	fn.Entry.Emit(Instr{Op: OpBinOp, Dest: a, Ty: I, Args: []Operand{Reg(x), ConstInt(5)}, OpStr: "+"})
	fn.Entry.Emit(Instr{Op: OpBinOp, Dest: b, Ty: I, Args: []Operand{Reg(x), ConstInt(5)}, OpStr: "+"})
	fn.Entry.Emit(Instr{Op: OpPrint, Args: []Operand{Reg(b)}})
	fn.Entry.Emit(Instr{Op: OpRet})

	CSEPass{}.Run(fn)

	// The second binop should be eliminated, and b's uses rewritten to a.
	if len(fn.Entry.Instrs) != 3 {
		t.Fatalf("expected 3 instrs after CSE, got %d:\n%s",
			len(fn.Entry.Instrs), formatFn(fn))
	}
}

func TestCSE_DifferentOps(t *testing.T) {
	fn := NewFunction("cse_diff")
	fn.Params = []BlockParam{{Name: "x_0", Ty: I}}
	x := fn.Params[0].Name
	a := fn.NewReg("a")
	b := fn.NewReg("b") // different op
	fn.Entry.Emit(Instr{Op: OpBinOp, Dest: a, Ty: I, Args: []Operand{Reg(x), ConstInt(5)}, OpStr: "+"})
	fn.Entry.Emit(Instr{Op: OpBinOp, Dest: b, Ty: I, Args: []Operand{Reg(x), ConstInt(5)}, OpStr: "*"})
	fn.Entry.Emit(Instr{Op: OpPrint, Args: []Operand{Reg(b)}})
	fn.Entry.Emit(Instr{Op: OpRet})

	changed := CSEPass{}.Run(fn)

	if changed {
		t.Error("CSE should not change anything for different operators")
	}
	if len(fn.Entry.Instrs) != 4 {
		t.Errorf("expected 4 instrs unchanged, got %d", len(fn.Entry.Instrs))
	}
}

// ─── Mem2RegPass Tests ───────────────────────────────────────────

func TestMem2Reg_SimpleStoreLoad(t *testing.T) {
	fn := NewFunction("m2r")
	fn.Entry.Emit(Instr{Op: OpStore, Cell: "x", Args: []Operand{ConstInt(42)}})
	fn.Entry.Emit(Instr{Op: OpLoad, Dest: "r", Cell: "x", Ty: I})
	fn.Entry.Emit(Instr{Op: OpPrint, Args: []Operand{Reg("r")}})
	fn.Entry.Emit(Instr{Op: OpRet})

	changed := Mem2RegPass{}.Run(fn)

	if !changed {
		t.Fatal("expected mem2reg to make changes")
	}
	// Store and load should be removed.
	for _, in := range fn.Entry.Instrs {
		if in.Op == OpStore || in.Op == OpLoad {
			t.Errorf("store/load should be removed, got %v", in)
		}
	}
}

func TestMem2Reg_MultipleLoads(t *testing.T) {
	fn := NewFunction("m2r_multi")
	fn.Entry.Emit(Instr{Op: OpStore, Cell: "x", Args: []Operand{ConstInt(10)}})
	fn.Entry.Emit(Instr{Op: OpLoad, Dest: "r1", Cell: "x", Ty: I})
	fn.Entry.Emit(Instr{Op: OpLoad, Dest: "r2", Cell: "x", Ty: I})
	fn.Entry.Emit(Instr{Op: OpPrint, Args: []Operand{Reg("r1")}})
	fn.Entry.Emit(Instr{Op: OpPrint, Args: []Operand{Reg("r2")}})
	fn.Entry.Emit(Instr{Op: OpRet})

	changed := Mem2RegPass{}.Run(fn)

	if !changed {
		t.Fatal("expected mem2reg to make changes")
	}
	for _, in := range fn.Entry.Instrs {
		if in.Op == OpStore || in.Op == OpLoad {
			t.Errorf("store/load should be removed, got %v", in)
		}
	}
}

func TestMem2Reg_MultipleStoresNotPromoted(t *testing.T) {
	fn := NewFunction("m2r_multi_store")
	b1 := fn.NewBlock("b1")
	b2 := fn.NewBlock("b2")

	fn.Entry.Emit(Instr{Op: OpStore, Cell: "x", Args: []Operand{ConstInt(1)}})
	fn.Entry.Emit(Instr{Op: OpJmp, JmpTarget: b1.Name})
	b1.Emit(Instr{Op: OpStore, Cell: "x", Args: []Operand{ConstInt(2)}})
	b1.Emit(Instr{Op: OpJmp, JmpTarget: b2.Name})
	b2.Emit(Instr{Op: OpLoad, Dest: "r", Cell: "x", Ty: I})
	b2.Emit(Instr{Op: OpPrint, Args: []Operand{Reg("r")}})
	b2.Emit(Instr{Op: OpRet})

	changed := Mem2RegPass{}.Run(fn)

	// Multiple stores → not promotable.
	if changed {
		t.Error("mem2reg should not promote variables with multiple stores")
	}
}

// ─── LICMPass Tests ──────────────────────────────────────────────

func TestLICM_HoistsConstFromLoop(t *testing.T) {
	fn := NewFunction("licm")
	loopHeader := fn.NewBlock("hdr")
	loopBody := fn.NewBlock("bdy")
	loopExit := fn.NewBlock("ext")

	// entry → hdr
	fn.Entry.Emit(Instr{Op: OpJmp, JmpTarget: loopHeader.Name})

	// hdr: test condition
	loopHeader.Emit(Instr{
		Op:          OpBr,
		Args:        []Operand{ConstBool(true)},
		ThenTarget:  loopBody.Name,
		ElseTarget:  loopExit.Name,
		BranchArgs:  [][]Operand{{}, {}},
	})

	// bdy: loop-invariant computation + back-edge to hdr
	hoistable := fn.NewReg("hoist")
	loopBody.Emit(Instr{Op: OpBinOp, Dest: hoistable, Ty: I, Args: []Operand{ConstInt(3), ConstInt(4)}, OpStr: "*"})
	loopBody.Emit(Instr{Op: OpJmp, JmpTarget: loopHeader.Name})

	loopExit.Emit(Instr{Op: OpRet})

	changed := LICMPass{}.Run(fn)

	if !changed {
		t.Fatal("expected LICM to hoist loop-invariant code")
	}
	// The constant computation should be in the header (entry side), not the body.
	foundInEntry := false
	for _, in := range fn.Entry.Instrs {
		if in.Op == OpBinOp && in.Dest == hoistable {
			foundInEntry = true
		}
	}
	// Check header too — it might be inserted before the terminator
	for _, in := range loopHeader.Instrs {
		if in.Op == OpBinOp && in.Dest == hoistable {
			foundInEntry = true
		}
	}
	if !foundInEntry {
		t.Error("expected loop-invariant instruction to be hoisted out of loop body")
	}
}

// ─── Pipeline Tests ──────────────────────────────────────────────

func TestPipeline_Run(t *testing.T) {
	fn := NewFunction("pipe")
	a := fn.NewReg("a")
	b := fn.NewReg("b") // dead
	c := fn.NewReg("c")
	fn.Entry.Emit(Instr{Op: OpConst, Dest: a, Ty: I, Args: []Operand{ConstInt(3)}, OpStr: ""})
	fn.Entry.Emit(Instr{Op: OpConst, Dest: b, Ty: I, Args: []Operand{ConstInt(4)}, OpStr: ""})
	fn.Entry.Emit(Instr{Op: OpBinOp, Dest: c, Ty: I, Args: []Operand{Reg(a), ConstInt(5)}, OpStr: "+"})
	fn.Entry.Emit(Instr{Op: OpPrint, Args: []Operand{Reg(c)}})
	fn.Entry.Emit(Instr{Op: OpRet})

	changed := NewPipeline().Run(fn)

	if changed == 0 {
		t.Error("expected pipeline to make changes")
	}
	// After folding: 3+5=8, after DCE: dead const removed.
	for _, in := range fn.Entry.Instrs {
		if in.Op == OpConst && in.Dest == "b_1" {
			t.Error("dead constant should be eliminated")
		}
	}
}

func TestPipeline_Stats(t *testing.T) {
	fn := NewFunction("stats")
	a := fn.NewReg("a")
	b := fn.NewReg("b")
	fn.Entry.Emit(Instr{Op: OpBinOp, Dest: a, Ty: I, Args: []Operand{ConstInt(2), ConstInt(3)}, OpStr: "+"})
	fn.Entry.Emit(Instr{Op: OpBinOp, Dest: b, Ty: I, Args: []Operand{Reg(a), ConstInt(1)}, OpStr: "*"})
	fn.Entry.Emit(Instr{Op: OpPrint, Args: []Operand{Reg(b)}})
	fn.Entry.Emit(Instr{Op: OpRet})

	stats := NewPipeline().RunWithStats(fn)

	if len(stats) == 0 {
		t.Fatal("expected stats")
	}
	// At least fold-const should have run.
	found := false
	for _, s := range stats {
		if s.Name == "fold-const" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected fold-const pass in stats")
	}
}

func TestPipeline_RunModule(t *testing.T) {
	m := &Module{}
	fn1 := m.NewFunction("f1")
	a := fn1.NewReg("a")
	fn1.Entry.Emit(Instr{Op: OpConst, Dest: a, Ty: I, Args: []Operand{ConstInt(10)}})
	fn1.Entry.Emit(Instr{Op: OpPrint, Args: []Operand{Reg(a)}})
	fn1.Entry.Emit(Instr{Op: OpRet})

	fn2 := m.NewFunction("f2")
	b := fn2.NewReg("b")
	fn2.Entry.Emit(Instr{Op: OpConst, Dest: b, Ty: I, Args: []Operand{ConstInt(20)}})
	fn2.Entry.Emit(Instr{Op: OpPrint, Args: []Operand{Reg(b)}})
	fn2.Entry.Emit(Instr{Op: OpRet})

	changed := NewPipeline().RunModule(m)

	if changed == 0 {
		t.Error("expected module pipeline to make changes")
	}
}

// ─── Benchmark ───────────────────────────────────────────────────

func BenchmarkPipeline(b *testing.B) {
	for i := 0; i < b.N; i++ {
		fn := NewFunction("bench")
		// Build a function with many foldable + dead instructions.
		regs := make([]string, 20)
		for j := 0; j < 20; j++ {
			r := fn.NewReg("v")
			regs[j] = r
			fn.Entry.Emit(Instr{Op: OpConst, Dest: r, Ty: I, Args: []Operand{ConstInt(int64(j))}})
		}
		// Dead computation chain
		for j := 0; j < 19; j++ {
			r := fn.NewReg("d")
			fn.Entry.Emit(Instr{Op: OpBinOp, Dest: r, Ty: I, Args: []Operand{Reg(regs[j]), Reg(regs[j+1])}, OpStr: "+"})
		}
		// Live output
		fn.Entry.Emit(Instr{Op: OpPrint, Args: []Operand{Reg(regs[0])}})
		fn.Entry.Emit(Instr{Op: OpRet})

		NewPipeline().Run(fn)
	}
}

func BenchmarkFoldConst(b *testing.B) {
	for i := 0; i < b.N; i++ {
		fn := NewFunction("bench_fold")
		for j := 0; j < 100; j++ {
			r := fn.NewReg("v")
			fn.Entry.Emit(Instr{Op: OpBinOp, Dest: r, Ty: I, Args: []Operand{ConstInt(int64(j)), ConstInt(int64(j + 1))}, OpStr: "*"})
		}
		fn.Entry.Emit(Instr{Op: OpRet})
		FoldConstPass{}.Run(fn)
	}
}

func BenchmarkDCE(b *testing.B) {
	for i := 0; i < b.N; i++ {
		fn := NewFunction("bench_dce")
		for j := 0; j < 100; j++ {
			r := fn.NewReg("d")
			fn.Entry.Emit(Instr{Op: OpConst, Dest: r, Ty: I, Args: []Operand{ConstInt(int64(j))}})
		}
		fn.Entry.Emit(Instr{Op: OpRet})
		DCESimplePass{}.Run(fn)
	}
}

// ─── Helper ──────────────────────────────────────────────────────

func formatFn(fn *Function) string {
	m := &Module{Functions: []*Function{fn}}
	return m.Format()
}

func TestPassNames(t *testing.T) {
	passes := []Pass{
		&FoldConstPass{},
		&DCESimplePass{},
		&CSEPass{},
		&Mem2RegPass{},
		&LICMPass{},
		&SAROPass{},
	}
	for _, p := range passes {
		if p.Name() == "" {
			t.Errorf("pass %T has empty name", p)
		}
	}
}
