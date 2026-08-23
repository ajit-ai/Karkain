package ssa

import (
	"strings"
	"testing"
)

func buildTestModule() *Module {
	m := &Module{}
	fn := m.NewFunction("add")
	fn.Params = []BlockParam{{Name: "x_0", Ty: Value}, {Name: "y_1", Ty: Value}}
	x := fn.Params[0].Name
	y := fn.Params[1].Name
	sum := fn.NewReg("sum")
	fn.Entry.Emit(Instr{Op: OpBinOp, Dest: sum, Ty: Value, Args: []Operand{Reg(x), Reg(y)}, OpStr: "+"})
	fn.Entry.Emit(Instr{Op: OpRet, Args: []Operand{Reg(sum)}})
	return m
}

func TestFormat(t *testing.T) {
	m := buildTestModule()
	out := m.Format()
	for _, want := range []string{"func @add(", "%x_0: value", "binop + %x_0, %y_1", "ret %sum_0"} {
		if !strings.Contains(out, want) {
			t.Errorf("format output missing %q:\n%s", want, out)
		}
	}
}

func TestVerifyOK(t *testing.T) {
	m := buildTestModule()
	if err := m.Verify(); err != nil {
		t.Fatalf("expected valid module, got: %v", err)
	}
}

func TestVerifyMissingTerminator(t *testing.T) {
	m := buildTestModule()
	m.Functions[0].Entry.Instrs = m.Functions[0].Entry.Instrs[:0] // drop ret
	m.Functions[0].Entry.Emit(Instr{Op: OpConst, Dest: "c", Ty: I, Args: []Operand{ConstInt(1)}})
	if err := m.Verify(); err == nil {
		t.Fatal("expected verify failure for missing terminator")
	}
}

func TestVerifySSAViolation(t *testing.T) {
	m := &Module{}
	fn := m.NewFunction("f")
	fn.Entry.Emit(Instr{Op: OpConst, Dest: "dup", Ty: I})
	fn.Entry.Emit(Instr{Op: OpConst, Dest: "dup", Ty: I}) // double write
	fn.Entry.Emit(Instr{Op: OpRet})
	if err := m.Verify(); err == nil {
		t.Fatal("expected SSA violation error")
	} else if !strings.Contains(err.Error(), "SSA") {
		t.Errorf("expected SSA violation message, got: %v", err)
	}
}

func TestConstantFolding(t *testing.T) {
	m := &Module{}
	fn := m.NewFunction("fold")
	a := fn.NewReg("a")
	b := fn.NewReg("b")
	c := fn.NewReg("c")
	fn.Entry.Emit(Instr{Op: OpBinOp, Dest: a, Ty: I, Args: []Operand{ConstInt(6), ConstInt(7)}, OpStr: "*"})
	fn.Entry.Emit(Instr{Op: OpBinOp, Dest: b, Ty: Bool, Args: []Operand{ConstBool(true), ConstBool(false)}, OpStr: "&&"})
	fn.Entry.Emit(Instr{Op: OpUnOp, Dest: c, Ty: I, Args: []Operand{ConstInt(42)}, OpStr: "-"})
	fn.Entry.Emit(Instr{Op: OpRet, Args: []Operand{Reg(a)}})

	m.FoldConstants()

	instrs := fn.Entry.Instrs
	if instrs[0].Op != OpConst || instrs[0].Args[0].IntVal != 42 {
		t.Errorf("expected 6*7 folded to 42, got %v", instrs[0])
	}
	if instrs[1].Op != OpConst || instrs[1].Args[0].CstKind != Bool || instrs[1].Args[0].BoolVal != false {
		t.Errorf("expected true&&false folded to false, got %v", instrs[1])
	}
	if instrs[2].Op != OpConst || instrs[2].Args[0].IntVal != -42 {
		t.Errorf("expected -42 folded, got %v", instrs[2])
	}
}

func TestDeadCodeElimination(t *testing.T) {
	m := &Module{}
	fn := m.NewFunction("dce")
	dead1 := fn.NewReg("dead1")
	dead2 := fn.NewReg("dead2") // feeds dead1
	live := fn.NewReg("live")
	fn.Entry.Emit(Instr{Op: OpConst, Dest: dead2, Ty: I, Args: []Operand{ConstInt(1)}})
	fn.Entry.Emit(Instr{Op: OpBinOp, Dest: dead1, Ty: I, Args: []Operand{Reg(dead2), ConstInt(2)}, OpStr: "+"})
	fn.Entry.Emit(Instr{Op: OpConst, Dest: live, Ty: I, Args: []Operand{ConstInt(3)}})
	fn.Entry.Emit(Instr{Op: OpPrint, Args: []Operand{Reg(live)}})
	fn.Entry.Emit(Instr{Op: OpRet})

	m.EliminateDeadCode()

	if len(fn.Entry.Instrs) != 3 {
		t.Fatalf("expected 3 surviving instrs (const, print, ret), got %d:\n%s",
			len(fn.Entry.Instrs), m.Format())
	}
}

func TestBranchAndBlockParams(t *testing.T) {
	m := &Module{}
	fn := m.NewFunction("abs")
	fn.Params = []BlockParam{{Name: "x_0", Ty: Value}}
	x := fn.Params[0].Name

	thenB := fn.NewBlock("then")
	elseB := fn.NewBlock("else")
	exitB := fn.NewBlock("exit")
	exitB.Params = []BlockParam{{Name: "result", Ty: Value}}

	neg := fn.NewReg("neg")
	fn.Entry.Emit(Instr{Op: OpBinOp, Dest: neg, Ty: Bool, Args: []Operand{Reg(x), ConstInt(0)}, OpStr: "<"})
	fn.Entry.Emit(Instr{
		Op:         OpBr,
		Args:       []Operand{Reg(neg)},
		ThenTarget: thenB.Name,
		ElseTarget: elseB.Name,
		BranchArgs: [][]Operand{{}, {}},
	})

	negv := fn.NewReg("negv")
	thenB.Emit(Instr{Op: OpUnOp, Dest: negv, Ty: Value, Args: []Operand{Reg(x)}, OpStr: "-"})
	thenB.Emit(Instr{Op: OpJmp, JmpTarget: exitB.Name, BranchArgs: [][]Operand{{Reg(negv)}}})

	elseB.Emit(Instr{Op: OpJmp, JmpTarget: exitB.Name, BranchArgs: [][]Operand{{Reg(x)}}})

	exitB.Emit(Instr{Op: OpRet, Args: []Operand{Reg("result")}})

	if err := m.Verify(); err != nil {
		t.Fatalf("expected valid CFG: %v", err)
	}
	out := m.Format()
	if !strings.Contains(out, "exit:") && !strings.Contains(out, "exit1:") {
		t.Logf("blocks rendered as: %s", out)
	}
}

