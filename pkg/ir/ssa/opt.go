package ssa

// Phase 53: optimization passes — constant folding, then dead code elimination.

// FoldConstants folds pure ops over constants:
//
//	const OP const → const   (arith, comparison on ints/floats/bools)
//	unop !true / -5    → const
func (m *Module) FoldConstants() {
	for _, fn := range m.Functions {
		for _, b := range fn.Blocks {
			foldBlock(b)
		}
	}
}

func foldBlock(b *Block) {
	for i := range b.Instrs {
		in := &b.Instrs[i]
		switch in.Op {
		case OpBinOp:
			if len(in.Args) == 2 && in.Args[0].IsCst && in.Args[1].IsCst {
				if out, ok := foldBinary(in.OpStr, in.Args[0], in.Args[1]); ok {
					replaceWithConst(in, out)
				}
			}
		case OpUnOp:
			if len(in.Args) == 1 && in.Args[0].IsCst {
				if out, ok := foldUnary(in.OpStr, in.Args[0]); ok {
					replaceWithConst(in, out)
				}
			}
		case OpConst:
			// normalize: keep payload in Args[0] for uniform access
			if len(in.Args) == 0 {
				in.Args = []Operand{{IsCst: true, CstKind: in.Ty}}
			}
		}
	}
}

func replaceWithConst(in *Instr, o Operand) {
	in.Op = OpConst
	in.Ty = o.CstKind
	in.Args = []Operand{o}
	in.OpStr = ""
	in.RawC = ""
}

func foldBinary(op string, a, b Operand) (Operand, bool) {
	if op == "&&" || op == "||" {
		return foldLogical(op, a, b)
	}
	switch a.CstKind {
	case I:
		x, y := a.IntVal, b.IntVal
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
	case F:
		x, y := a.FltVal, b.FltVal
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
	}
	return Operand{}, false
}

func foldLogical(op string, a, b Operand) (Operand, bool) {
	if a.CstKind != Bool || b.CstKind != Bool {
		return Operand{}, false
	}
	switch op {
	case "&&":
		return ConstBool(a.BoolVal && b.BoolVal), true
	case "||":
		return ConstBool(a.BoolVal || b.BoolVal), true
	}
	return Operand{}, false
}

func foldUnary(op string, a Operand) (Operand, bool) {
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

// EliminateDeadCode removes instructions whose destinations are never read.
// Registers are function-scoped; side-effecting ops (calls, print, index_set,
// rawc statements, terminators) are always kept.
func (m *Module) EliminateDeadCode() {
	for _, fn := range m.Functions {
		dceFunction(fn)
	}
}

func dceFunction(fn *Function) {
	used := make(map[string]bool)
	collectUses := func(o Operand) {
		if !o.IsCst && o.Reg != "" {
			used[o.Reg] = true
		}
	}
	// Iterate to fixpoint (use chains can cascade).
	for changed := true; changed; {
		changed = false
		for k := range used {
			delete(used, k)
		}
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				for _, a := range in.Args {
					collectUses(a)
				}
				for _, g := range in.BranchArgs {
					for _, a := range g {
						collectUses(a)
					}
				}
			}
		}
		for _, b := range fn.Blocks {
			keep := b.Instrs[:0]
			for _, in := range b.Instrs {
				sideEffect := in.Dest == "" || used[in.Dest]
				switch in.Op {
				case OpCallVoid, OpIndexSet, OpPrint:
					sideEffect = true
				case OpCall:
					sideEffect = true // may have effects even when result unused
				case OpRawC:
					sideEffect = true // may have arbitrary effects
				}
				if sideEffect || isTerminator(in) {
					keep = append(keep, in)
				} else if in.Dest != "" && !used[in.Dest] {
					changed = true // dropped a dead def; recompute uses
				} else {
					keep = append(keep, in)
				}
			}
			b.Instrs = keep
		}
	}
}

func isTerminator(in Instr) bool {
	switch in.Op {
	case OpBr, OpJmp, OpRet:
		return true
	}
	return false
}

