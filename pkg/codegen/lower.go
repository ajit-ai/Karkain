package codegen

import (
	"karkain/pkg/parser"
	"karkain/pkg/ir/ssa"
	"strconv"
	"strings"
)

// Phase 53: AST -> SSA IR lowering.
// Core statements/expressions lower to real SSA instructions; anything not yet
// supported lowers to an explicit OpRawC escape hatch that reuses the legacy
// generator's tested emission, keeping one pipeline with visible seams.

type loopCtx struct {
	breakTarget string
	contTarget  string
	mutated     []string // env keys passed as branch args
}

type fnLowerer struct {
	g        *Generator
	fn       *ssa.Function
	env      map[string]string
	declared map[string]bool
	loops    []loopCtx
	cur      *ssa.Block
}

func mutatingBuiltin(name string) bool {
	switch name {
	case "appendArray", "push", "delete":
		return true
	}
	return false
}

func callableBuiltin(name string) bool {
	switch name {
	case "len", "sqrt", "pow", "hasKey", "readFile", "writeFile",
		"trim", "contains", "split":
		return true
	}
	return false
}

func newFnLowerer(g *Generator, prog *parser.Program, fn *ssa.Function) *fnLowerer {
	declared := make(map[string]bool)
	for _, st := range prog.Statements {
		if fd, ok := st.(*parser.FuncDecl); ok {
			declared[fd.Name] = true
		}
	}
	lf := &fnLowerer{g: g, fn: fn, env: make(map[string]string), declared: declared}
	lf.cur = fn.Entry
	return lf
}

func copyMap(m map[string]string) map[string]string {
	c := make(map[string]string, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

// ---- expression lowering ----

func (l *fnLowerer) materialize(op ssa.Operand, hint string) string {
	if !op.IsCst {
		return op.Reg
	}
	dest := l.fn.NewReg(hint)
	l.cur.Emit(ssa.Instr{Op: ssa.OpConst, Dest: dest, Ty: op.CstKind, Args: []ssa.Operand{op}})
	return dest
}

func (l *fnLowerer) ensureReg(op ssa.Operand, hint string) string {
	return l.materialize(op, hint)
}

func (l *fnLowerer) rawcExpr(e parser.Node) string {
	dest := l.fn.NewReg("raw")
	l.cur.Emit(ssa.Instr{Op: ssa.OpRawC, Dest: dest, Ty: ssa.Value, RawC: l.g.genExpr(e)})
	return dest
}

func (l *fnLowerer) lowerExpr(e parser.Node) ssa.Operand {
	switch n := e.(type) {
	case *parser.IntLiteral:
		v, _ := strconv.ParseInt(n.Value, 10, 64)
		return ssa.ConstInt(v)
	case *parser.Float64Literal:
		v, _ := strconv.ParseFloat(n.Value, 64)
		return ssa.ConstFlt(v)
	case *parser.BoolLiteral:
		return ssa.ConstBool(n.Value)
	case *parser.StringLiteral:
		return ssa.ConstStr(n.Value)
	case *parser.Identifier:
		if _, ok := l.env[n.Name]; ok {
			dest := l.fn.NewReg(n.Name)
			l.cur.Emit(ssa.Instr{Op: ssa.OpLoad, Dest: dest, Ty: ssa.Value, Cell: n.Name})
			return ssa.Reg(dest)
		}
		return ssa.Reg(l.rawcExpr(n))
	case *parser.UnaryExpr:
		if n.Operator == "-" || n.Operator == "!" {
			a := l.ensureReg(l.lowerExpr(n.Operand), "u")
			dest := l.fn.NewReg("neg")
			if n.Operator == "-" {
				dest = l.fn.NewReg("negv")
			} else {
				dest = l.fn.NewReg("notv")
			}
			l.cur.Emit(ssa.Instr{Op: ssa.OpUnOp, Dest: dest, Ty: ssa.Value, Args: []ssa.Operand{ssa.Reg(a)}, OpStr: n.Operator})
			return ssa.Reg(dest)
		}
	case *parser.BinaryExpr:
		if n.Operator == "=" || n.Operator == "&&" || n.Operator == "||" {
			return ssa.Reg(l.rawcExpr(n))
		}
		a := l.ensureReg(l.lowerExpr(n.Left), "lhs")
		b := l.ensureReg(l.lowerExpr(n.Right), "rhs")
		dest := l.fn.NewReg("bin")
		l.cur.Emit(ssa.Instr{Op: ssa.OpBinOp, Dest: dest, Ty: ssa.Value, Args: []ssa.Operand{ssa.Reg(a), ssa.Reg(b)}, OpStr: n.Operator})
		return ssa.Reg(dest)
	case *parser.CallExpr:
		if !n.IsCFunc && !strings.Contains(n.Function, ".") &&
			!mutatingBuiltin(n.Function) &&
			(l.declared[n.Function] || callableBuiltin(n.Function)) {
			args := make([]ssa.Operand, len(n.Args))
			for i, a := range n.Args {
				args[i] = ssa.Reg(l.ensureReg(l.lowerExpr(a), "arg"))
			}
			target := n.Function
			if !l.declared[target] {
				target = "karkain_" + target // runtime builtin naming
			}
			dest := l.fn.NewReg("call")
			l.cur.Emit(ssa.Instr{Op: ssa.OpCall, Dest: dest, Ty: ssa.Value, Args: args, OpStr: target})
			return ssa.Reg(dest)
		}
	}
	return ssa.Reg(l.rawcExpr(e))
}

// ---- statement lowering ----

func (l *fnLowerer) rawcStmt(c string) {
	t := strings.TrimSpace(c)
	if t != "" && !strings.HasSuffix(t, ";") {
		t += ";"
	}
	l.cur.Emit(ssa.Instr{Op: ssa.OpRawC, RawC: t})
}

// collectMutated finds variable names rebound anywhere inside stmts (assignments,
// declarations) conservatively — used for loop header params.
func collectMutated(stmts []parser.Node, out map[string]bool) {
	for _, s := range stmts {
		switch n := s.(type) {
		case *parser.VarDeclStmt:
			out[n.Name] = true
		case *parser.ExprStmt:
			if be, ok := n.Expression.(*parser.BinaryExpr); ok && be.Operator == "=" {
				if id, ok := be.Left.(*parser.Identifier); ok {
					out[id.Name] = true
				}
			}
		case *parser.IfStmt:
			collectMutated(n.Consequence, out)
			collectMutated(n.Alternative, out)
		case *parser.WhileStmt:
			collectMutated(n.Body, out)
		case *parser.BlockStmt:
			collectMutated(n.Statements, out)
		}
	}
}

func (l *fnLowerer) lowerStmts(stmts []parser.Node) {
	for _, s := range stmts {
		l.lowerStmt(s)
		if l.cur.Terminated() {
			return
		}
	}
}

func (l *fnLowerer) branchArgs(keys []string) []ssa.Operand {
	args := make([]ssa.Operand, len(keys))
	for i, k := range keys {
		r, ok := l.env[k]
		if !ok {
			r = l.fn.NewReg(k)
			l.env[k] = r
		}
		args[i] = ssa.Reg(r)
	}
	return args
}

func (l *fnLowerer) storeCell(name string, op ssa.Operand) {
	r := l.materialize(op, name)
	l.cur.Emit(ssa.Instr{Op: ssa.OpStore, Cell: name, Args: []ssa.Operand{ssa.Reg(r)}})
	l.env[name] = r
}

func (l *fnLowerer) lowerStmt(s parser.Node) {
	switch n := s.(type) {
	case *parser.VarDeclStmt:
		if n.IsMatrix {
			l.rawcStmt(l.g.genStatement(s))
			return
		}
		if _, ok := n.Value.(*parser.FuncDecl); ok {
			l.rawcStmt(l.g.genStatement(s))
			return
		}
		l.storeCell(n.Name, l.lowerExpr(n.Value))
	case *parser.ExprStmt:
		if be, ok := n.Expression.(*parser.BinaryExpr); ok && be.Operator == "=" {
			if id, ok2 := be.Left.(*parser.Identifier); ok2 {
				l.storeCell(id.Name, l.lowerExpr(be.Right))
				return
			}
		}
		l.rawcStmt(l.g.genExpressionAsStatement(n.Expression))
	case *parser.PrintStmt:
		if _, ok := n.Value.(*parser.MatrixIndexExpr); ok {
			l.rawcStmt(l.g.genStatement(s))
			return
		}
		v := l.ensureReg(l.lowerExpr(n.Value), "pr")
		l.cur.Emit(ssa.Instr{Op: ssa.OpPrint, Args: []ssa.Operand{ssa.Reg(v)}})
	case *parser.ReturnStmt:
		if n.Value == nil {
			l.cur.Emit(ssa.Instr{Op: ssa.OpRet})
			return
		}
		v := l.ensureReg(l.lowerExpr(n.Value), "rv")
		l.cur.Emit(ssa.Instr{Op: ssa.OpRet, Args: []ssa.Operand{ssa.Reg(v)}})
	case *parser.BreakStmt:
		if len(l.loops) > 0 {
			lp := l.loops[len(l.loops)-1]
			l.cur.Emit(ssa.Instr{Op: ssa.OpJmp, JmpTarget: lp.breakTarget})
		}
	case *parser.ContinueStmt:
		if len(l.loops) > 0 {
			lp := l.loops[len(l.loops)-1]
			l.cur.Emit(ssa.Instr{Op: ssa.OpJmp, JmpTarget: lp.contTarget})
		}
	case *parser.IfStmt:
		l.lowerIf(n)
	case *parser.WhileStmt:
		l.lowerWhile(n)
	default:
		l.rawcStmt(l.g.genStatement(s))
	}
}


func (l *fnLowerer) lowerIf(n *parser.IfStmt) {
	creg := l.ensureReg(l.lowerExpr(n.Condition), "cond")
	thenB := l.fn.NewBlock("then")
	joinB := l.fn.NewBlock("join")
	var elseB *ssa.Block
	elseName := joinB.Name
	if len(n.Alternative) > 0 {
		elseB = l.fn.NewBlock("else")
		elseName = elseB.Name
	}
	l.cur.Emit(ssa.Instr{
		Op: ssa.OpBr, Args: []ssa.Operand{ssa.Reg(creg)},
		ThenTarget: thenB.Name, ElseTarget: elseName,
	})

	l.cur = thenB
	l.lowerStmts(n.Consequence)
	if !l.cur.Terminated() {
		l.cur.Emit(ssa.Instr{Op: ssa.OpJmp, JmpTarget: joinB.Name})
	}

	if elseB != nil {
		l.cur = elseB
		l.lowerStmts(n.Alternative)
		if !l.cur.Terminated() {
			l.cur.Emit(ssa.Instr{Op: ssa.OpJmp, JmpTarget: joinB.Name})
		}
	}
	l.cur = joinB
}

func (l *fnLowerer) lowerWhile(n *parser.WhileStmt) {
	header := l.fn.NewBlock("loopheader")
	bodyB := l.fn.NewBlock("loopbody")
	exitB := l.fn.NewBlock("loopexit")

	l.cur.Emit(ssa.Instr{Op: ssa.OpJmp, JmpTarget: header.Name})
	l.cur = header
	creg := l.ensureReg(l.lowerExpr(n.Condition), "cond")
	header.Emit(ssa.Instr{
		Op: ssa.OpBr, Args: []ssa.Operand{ssa.Reg(creg)},
		ThenTarget: bodyB.Name, ElseTarget: exitB.Name,
	})

	l.loops = append(l.loops, loopCtx{breakTarget: exitB.Name, contTarget: header.Name})
	l.cur = bodyB
	l.lowerStmts(n.Body)
	if !l.cur.Terminated() {
		l.cur.Emit(ssa.Instr{Op: ssa.OpJmp, JmpTarget: header.Name})
	}
	l.loops = l.loops[:len(l.loops)-1]
	l.cur = exitB
}
