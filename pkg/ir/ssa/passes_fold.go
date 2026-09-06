package ssa

// FoldConstPass — Phase 94: constant folding pass.
//
// Evaluates pure operations over constants at compile time:
//
//	binop const OP const → const
//	unop OP const        → const
//
// Also performs algebraic identity simplification: x+0, x*1, x&&true, x||false.

type FoldConstPass struct{}

func (FoldConstPass) Name() string { return "fold-const" }

func (FoldConstPass) Run(fn *Function) bool {
	changed := false
	// Track registers that hold constants for cross-instruction propagation.
	constRegs := make(map[string]Operand)
	for _, p := range fn.Params {
		_ = p // params are not constants
	}

	for _, b := range fn.Blocks {
		for i := range b.Instrs {
			in := &b.Instrs[i]
			// Replace register args with known constants.
			resolveArgs(in, constRegs)
			if foldInstr(in) {
				changed = true
			}
			// Track newly folded constants.
			if in.Op == OpConst && in.Dest != "" && len(in.Args) > 0 && in.Args[0].IsCst {
				constRegs[in.Dest] = in.Args[0]
			}
		}
	}
	return changed
}

func resolveArgs(in *Instr, constRegs map[string]Operand) {
	for j := range in.Args {
		if !in.Args[j].IsCst && in.Args[j].Reg != "" {
			if c, ok := constRegs[in.Args[j].Reg]; ok {
				in.Args[j] = c
			}
		}
	}
}

func foldInstr(in *Instr) bool {
	switch in.Op {
	case OpBinOp:
		if len(in.Args) == 2 && in.Args[0].IsCst && in.Args[1].IsCst {
			if out, ok := evalBinOp(in.OpStr, in.Args[0], in.Args[1]); ok {
				*in = Instr{Op: OpConst, Dest: in.Dest, Ty: out.CstKind, Args: []Operand{out}}
				return true
			}
		}
		if out, ok := algebraicSimplify(*in); ok {
			*in = out
			return true
		}
	case OpUnOp:
		if len(in.Args) == 1 && in.Args[0].IsCst {
			if out, ok := evalUnOp(in.OpStr, in.Args[0]); ok {
				*in = Instr{Op: OpConst, Dest: in.Dest, Ty: out.CstKind, Args: []Operand{out}}
				return true
			}
		}
	}
	return false
}

func algebraicSimplify(in Instr) (Instr, bool) {
	if len(in.Args) != 2 || !in.Args[1].IsCst {
		return Instr{}, false
	}
	a := in.Args[0]
	b := in.Args[1]
	switch in.Ty {
	case I:
		switch in.OpStr {
		case "+":
			if b.IntVal == 0 {
				return Instr{Op: in.Op, Dest: in.Dest, Ty: in.Ty, Args: []Operand{a}, OpStr: in.OpStr}, true
			}
		case "*":
			if b.IntVal == 1 {
				return Instr{Op: in.Op, Dest: in.Dest, Ty: in.Ty, Args: []Operand{a}, OpStr: in.OpStr}, true
			}
			if b.IntVal == 0 {
				return Instr{Op: OpConst, Dest: in.Dest, Ty: I, Args: []Operand{ConstInt(0)}}, true
			}
		case "-":
			if b.IntVal == 0 {
				return Instr{Op: in.Op, Dest: in.Dest, Ty: in.Ty, Args: []Operand{a}, OpStr: in.OpStr}, true
			}
		case "/":
			if b.IntVal == 1 {
				return Instr{Op: in.Op, Dest: in.Dest, Ty: in.Ty, Args: []Operand{a}, OpStr: in.OpStr}, true
			}
		}
	case F:
		switch in.OpStr {
		case "+":
			if b.FltVal == 0 {
				return Instr{Op: in.Op, Dest: in.Dest, Ty: in.Ty, Args: []Operand{a}, OpStr: in.OpStr}, true
			}
		case "*":
			if b.FltVal == 1 {
				return Instr{Op: in.Op, Dest: in.Dest, Ty: in.Ty, Args: []Operand{a}, OpStr: in.OpStr}, true
			}
		case "-":
			if b.FltVal == 0 {
				return Instr{Op: in.Op, Dest: in.Dest, Ty: in.Ty, Args: []Operand{a}, OpStr: in.OpStr}, true
			}
		}
	case Bool:
		switch in.OpStr {
		case "&&":
			if b.BoolVal {
				return Instr{Op: in.Op, Dest: in.Dest, Ty: in.Ty, Args: []Operand{a}, OpStr: in.OpStr}, true
			}
			return Instr{Op: OpConst, Dest: in.Dest, Ty: Bool, Args: []Operand{ConstBool(false)}}, true
		case "||":
			if !b.BoolVal {
				return Instr{Op: in.Op, Dest: in.Dest, Ty: in.Ty, Args: []Operand{a}, OpStr: in.OpStr}, true
			}
			return Instr{Op: OpConst, Dest: in.Dest, Ty: Bool, Args: []Operand{ConstBool(true)}}, true
		}
	}
	return Instr{}, false
}

func evalBinOp(op string, a, b Operand) (Operand, bool) {
	switch a.CstKind {
	case I:
		return evalIntBinOp(op, a.IntVal, b.IntVal)
	case F:
		return evalFltBinOp(op, a.FltVal, b.FltVal)
	case Bool:
		return evalBoolBinOp(op, a.BoolVal, b.BoolVal)
	}
	return Operand{}, false
}

func evalIntBinOp(op string, x, y int64) (Operand, bool) {
	switch op {
	case "+":
		return ConstInt(x + y), true
	case "-":
		return ConstInt(x - y), true
	case "*":
		return ConstInt(x * y), true
	case "/":
		if y != 0 {
			return ConstInt(x / y), true
		}
	case "%":
		if y != 0 {
			return ConstInt(x % y), true
		}
	case "==":
		return ConstBool(x == y), true
	case "!=":
		return ConstBool(x != y), true
	case "<":
		return ConstBool(x < y), true
	case ">":
		return ConstBool(x > y), true
	case "<=":
		return ConstBool(x <= y), true
	case ">=":
		return ConstBool(x >= y), true
	}
	return Operand{}, false
}

func evalFltBinOp(op string, x, y float64) (Operand, bool) {
	switch op {
	case "+":
		return ConstFlt(x + y), true
	case "-":
		return ConstFlt(x - y), true
	case "*":
		return ConstFlt(x * y), true
	case "/":
		if y != 0 {
			return ConstFlt(x / y), true
		}
	case "==":
		return ConstBool(x == y), true
	case "!=":
		return ConstBool(x != y), true
	case "<":
		return ConstBool(x < y), true
	case ">":
		return ConstBool(x > y), true
	case "<=":
		return ConstBool(x <= y), true
	case ">=":
		return ConstBool(x >= y), true
	}
	return Operand{}, false
}

func evalBoolBinOp(op string, x, y bool) (Operand, bool) {
	switch op {
	case "&&":
		return ConstBool(x && y), true
	case "||":
		return ConstBool(x || y), true
	}
	return Operand{}, false
}

func evalUnOp(op string, a Operand) (Operand, bool) {
	switch op {
	case "-":
		switch a.CstKind {
		case I:
			return ConstInt(-a.IntVal), true
		case F:
			return ConstFlt(-a.FltVal), true
		}
	case "!":
		if a.CstKind == Bool {
			return ConstBool(!a.BoolVal), true
		}
	}
	return Operand{}, false
}
