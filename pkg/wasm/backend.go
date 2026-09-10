package wasm

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"karkain/pkg/parser"
)

// statics interns dynamic strings (string literals, error kinds, the source
// file basename) into the fixed pool region that begins at poolBase.
type statics struct {
	pool []byte
	seen map[string]int
}

func newStatics() *statics {
	return &statics{seen: map[string]int{}}
}

// intern places s in the pool (if not already present) and returns its
// linear-memory offset.
func (s *statics) intern(str string) int {
	if off, ok := s.seen[str]; ok {
		return off
	}
	off := poolBase + len(s.pool)
	s.pool = append(s.pool, str...)
	s.seen[str] = off
	return off
}

// dataBytes returns the full initializer for the data segment at staticBase:
// the fixed static region followed by all interned pool strings.
func (s *statics) dataBytes() []byte {
	out := append([]byte{}, staticBytes()...)
	return append(out, s.pool...)
}

// gen lowers an AST program to a binary wasm module.
type gen struct {
	st       *statics
	mb       *ModuleBuilder
	rt       RuntimeFunc
	ko       kindOffsets
	fileBase string

	e        *Emitter
	scopes   []map[string]int // innermost last; each map is name->local index
	loops    []loopInfo
	funcIdx  map[string]int
	mainIdx  int
	mainSeen bool
	temps    []int
}

type loopInfo struct {
	brkBase  int
	contBase int
}

func CompileProgram(prog *parser.Program, srcFile string) ([]byte, error) {
	g := &gen{
		st:      newStatics(),
		mb:      &ModuleBuilder{MemoryMin: 2, MemoryMax: -1},
		funcIdx: map[string]int{},
		mainIdx: -1,
	}
	g.fileBase = filepath.Base(srcFile)
	g.mb.Globals = []Global{{
		Type: I32, Mut: true,
		Init: func() []byte {
			var b []byte
			b = append(b, 0x41)
			b = appendSleb(b, heapBase)
			b = append(b, 0x0b)
			return b
		}(),
	}}

	fnames := collectFuncNames(prog)
	if err := g.scanUnsupported(prog, fnames); err != nil {
		return nil, err
	}

	rt, ko := addRuntime(g.mb, g.st, g.fileBase)
	g.rt, g.ko = rt, ko

	// Emit user functions in source order. Index them first so recursive
	// calls resolve before any body is emitted.
	var userFuncs []*parser.FuncDecl
	for _, stmt := range prog.Statements {
		if fd, ok := stmt.(*parser.FuncDecl); ok {
			userFuncs = append(userFuncs, fd)
		}
	}
	base := len(g.mb.Codes)
	for i, fd := range userFuncs {
		// each runtime body occupies one function index; fd_write import
		// holds index 0, so the first user fn is base+1.
		g.funcIdx[fd.Name] = base + i + 1
	}
	for _, fd := range userFuncs {
		g.emitFunc(fd)
	}
	if mainIdx, ok := g.funcIdx["main"]; ok {
		g.mainIdx = mainIdx
		g.mainSeen = true
	}
	if !g.mainSeen {
		return nil, fmt.Errorf("error K108: no 'main' function found for target wasm32-wasi")
	}

	// Emit _start: calls main and drops the result.
	startType := FuncType{}
	g.mb.FuncTypes = append(g.mb.FuncTypes, g.mb.AddType(startType))
	startIdx := g.emitStart(g.mainIdx)

	g.mb.Exports = append(g.mb.Exports,
		Export{Name: "_start", Kind: ExportFunc, Idx: startIdx},
		Export{Name: "memory", Kind: ExportMemory, Idx: 0},
	)
	g.mb.Data = append(g.mb.Data, DataSeg{Offset: staticBase, Bytes: g.st.dataBytes()})

	return g.mb.Encode(), nil
}

func collectFuncNames(prog *parser.Program) map[string]bool {
	out := map[string]bool{}
	for _, stmt := range prog.Statements {
		if fd, ok := stmt.(*parser.FuncDecl); ok {
			out[fd.Name] = true
		}
	}
	return out
}

// emitStart builds the _start function body.
func (g *gen) emitStart(mainIdx int) int {
	e := NewEmitter()
	e.Call(mainIdx)
	e.Drop()
	e.End()
	g.mb.Codes = append(g.mb.Codes, Code{Body: e.Bytes()})
	return len(g.mb.Codes)
}

// --- function emission ---

func (g *gen) emitFunc(fd *parser.FuncDecl) int {
	g.e = NewEmitter()
	g.scopes = []map[string]int{{}}

	// params: one i64 local each
	np := len(fd.Params)
	g.e.ParamCount = np
	g.declareParams(fd)

	for _, stmt := range fd.Body {
		g.genStmt(stmt)
	}
	// fallback value so every path leaves an i64 for the return
	g.e.I64Const(0)
	g.e.End()

	ft := FuncType{Params: repeatI64(np), Results: []ValueType{I64}}
	g.mb.FuncTypes = append(g.mb.FuncTypes, g.mb.AddType(ft))
	g.mb.Codes = append(g.mb.Codes, Code{Body: g.e.Bytes(), Locals: g.currentLocals()})
	return len(g.mb.Codes)
}

func repeatI64(n int) []ValueType {
	out := make([]ValueType, n)
	for i := range out {
		out[i] = I64
	}
	return out
}

// declareParams binds parameter names to their initial local indices.
func (g *gen) declareParams(fd *parser.FuncDecl) {
	for i, name := range fd.Params {
		g.scope()[name] = i
	}
}

// currentLocals returns the extra local slots (beyond the params) declared
// so far. All user-function locals are i64.
func (g *gen) currentLocals() []ValueType {
	return g.e.ExtraLocals
}

func (g *gen) scope() map[string]int { return g.scopes[len(g.scopes)-1] }

// allocLocal allocates an i64 local slot and returns its index.
func (g *gen) allocLocal() int {
	g.e.ExtraLocals = append(g.e.ExtraLocals, I64)
	return g.e.ParamCount + len(g.e.ExtraLocals) - 1
}

// allocTemp allocates a scratch i64 local (used by short-circuit evaluation).
func (g *gen) allocTemp() int { return g.allocLocal() }

func (g *gen) lookup(name string) int {
	for i := len(g.scopes) - 1; i >= 0; i-- {
		if idx, ok := g.scopes[i][name]; ok {
			return idx
		}
	}
	return -1
}

// --- statements ---

func (g *gen) genStmt(n parser.Node) {
	switch s := n.(type) {
	case *parser.VarDeclStmt:
		g.genVarDecl(s)
	case *parser.ExprStmt:
		g.genExpr(s.Expression)
		g.e.Drop()
	case *parser.PrintStmt:
		g.genExpr(s.Value)
		g.e.Call(g.rt.PrintValue)
	case *parser.ReturnStmt:
		if s.Value != nil {
			g.genExpr(s.Value)
		} else {
			g.e.I64Const(0)
		}
		g.e.Return()
	case *parser.IfStmt:
		g.genIf(s)
	case *parser.WhileStmt:
		g.genWhile(s)
	case *parser.ForStmt:
		g.genFor(s)
	case *parser.ForInStmt:
		g.genForIn(s)
	case *parser.BlockStmt:
		g.pushScope()
		for _, inner := range s.Statements {
			g.genStmt(inner)
		}
		g.popScope()
	case *parser.BreakStmt:
		g.genBreak()
	case *parser.ContinueStmt:
		g.genContinue()
	default:
		panic(fmt.Sprintf("wasm backend: unexpected statement %T", n))
	}
}

func (g *gen) pushScope() { g.scopes = append(g.scopes, map[string]int{}) }
func (g *gen) popScope()  { g.scopes = g.scopes[:len(g.scopes)-1] }

func (g *gen) genVarDecl(s *parser.VarDeclStmt) {
	idx := g.allocLocal()
	g.scope()[s.Name] = idx
	g.genExpr(s.Value)
	g.e.LocalSet(idx)
}

func (g *gen) genIf(s *parser.IfStmt) {
	g.genExpr(s.Condition)
	g.e.Call(g.rt.IsTruthy)
	g.e.BeginIf(noResult)
	g.pushScope()
	for _, stmt := range s.Consequence {
		g.genStmt(stmt)
	}
	g.popScope()
	if len(s.Alternative) > 0 {
		g.e.Else()
		g.pushScope()
		for _, stmt := range s.Alternative {
			g.genStmt(stmt)
		}
		g.popScope()
	}
	g.e.Close()
}

func (g *gen) genWhile(s *parser.WhileStmt) {
	g.e.BeginBlock(noResult)
	g.loops = append(g.loops, loopInfo{brkBase: g.e.frameDepth()})
	g.e.BeginLoop(noResult)
	contBase := g.e.frameDepth()
	g.loops[len(g.loops)-1].contBase = contBase
	g.genExpr(s.Condition)
	g.e.Call(g.rt.IsTruthy)
	g.e.I32Eqz()
	g.e.BrIf(1)
	g.pushScope()
	for _, stmt := range s.Body {
		g.genStmt(stmt)
	}
	g.popScope()
	g.e.Br(0)
	g.e.Close()
	g.e.Close()
	g.loops = g.loops[:len(g.loops)-1]
}

func (g *gen) genFor(s *parser.ForStmt) {
	g.pushScope()
	if s.Init != nil {
		g.genStmt(s.Init)
	}
	g.e.BeginBlock(noResult)
	g.loops = append(g.loops, loopInfo{brkBase: g.e.frameDepth()})
	g.e.BeginLoop(noResult)
	g.loops[len(g.loops)-1].contBase = g.e.frameDepth()
	if s.Condition != nil {
		g.genExpr(s.Condition)
		g.e.Call(g.rt.IsTruthy)
		g.e.I32Eqz()
		g.e.BrIf(1)
	} else {
		g.e.I32Const(1)
		g.e.I32Eqz()
		g.e.BrIf(1)
	}
	for _, stmt := range s.Body {
		g.genStmt(stmt)
	}
	if s.Post != nil {
		g.genExpr(s.Post)
		g.e.Drop()
	}
	g.e.Br(0)
	g.e.Close()
	g.e.Close()
	g.loops = g.loops[:len(g.loops)-1]
	g.popScope()
}

func (g *gen) genForIn(s *parser.ForInStmt) {
	// for x in arr: index runs over the array; element fetched via rt_get.
	arr := g.allocTemp()
	idxLoc := g.allocTemp()
	g.genExpr(s.Iter)
	g.e.LocalSet(arr)

	g.e.I64Const(0)
	g.e.LocalSet(idxLoc)

	g.e.BeginBlock(noResult)
	g.loops = append(g.loops, loopInfo{brkBase: g.e.frameDepth()})
	g.e.BeginLoop(noResult)
	g.loops[len(g.loops)-1].contBase = g.e.frameDepth()

	// exit when idx >= len(arr)
	g.e.LocalGet(idxLoc)
	g.e.LocalGet(arr)
	g.e.Call(g.rt.Len)
	g.e.I64LtS()
	g.e.I32Eqz()
	g.e.BrIf(1)

	// element = rt_get(arr, idx, file, line)
	element := g.allocTemp()
	g.e.LocalGet(arr)
	g.e.LocalGet(idxLoc)
	g.e.I32Const(g.ko.filePtr)
	g.e.I32Const(g.ko.fileLen)
	g.e.I32Const(s.Line)
	g.e.Call(g.rt.Get)
	g.e.LocalSet(element)

	g.pushScope()
	g.scope()[s.VarName] = element
	for _, stmt := range s.Body {
		g.genStmt(stmt)
	}
	g.popScope()

	// idx += 2
	g.e.LocalGet(idxLoc)
	g.e.I64Const(2)
	g.e.I64Add()
	g.e.LocalSet(idxLoc)

	g.e.Br(0)
	g.e.Close()
	g.e.Close()
	g.loops = g.loops[:len(g.loops)-1]
}

func (g *gen) genBreak() {
	if len(g.loops) == 0 {
		panic("wasm backend: break outside loop")
	}
	li := g.loops[len(g.loops)-1]
	g.e.Br(g.e.frameDepth() - li.brkBase)
}

func (g *gen) genContinue() {
	if len(g.loops) == 0 {
		panic("wasm backend: continue outside loop")
	}
	li := g.loops[len(g.loops)-1]
	g.e.Br(g.e.frameDepth() - li.contBase)
}

// --- expressions ---

func (g *gen) genExpr(n parser.Node) {
	switch x := n.(type) {
	case *parser.StringLiteral:
		off := g.st.intern(x.Value)
		g.e.I32Const(off)
		g.e.I32Const(len(x.Value))
		g.e.Call(g.rt.MakeString)
	case *parser.IntLiteral:
		v, err := strconv.ParseInt(x.Value, 0, 64)
		if err != nil {
			v = 0
		}
		g.e.I64Const(v << 1)
	case *parser.BoolLiteral:
		if x.Value {
			g.e.I64Const(2)
		} else {
			g.e.I64Const(0)
		}
	case *parser.Identifier:
		idx := g.lookup(x.Name)
		if idx < 0 {
			panic(fmt.Sprintf("wasm backend: undefined identifier %q", x.Name))
		}
		g.e.LocalGet(idx)
	case *parser.ArrayLiteral:
		g.e.I32Const(len(x.Elements))
		g.e.Call(g.rt.MakeArray)
		// store each element
		tmp := g.allocTemp()
		g.e.LocalSet(tmp)
		for i, el := range x.Elements {
			g.e.LocalGet(tmp)
			g.e.I64Const(int64(i) << 1)
			g.genExpr(el)
			g.e.I32Const(g.ko.filePtr)
			g.e.I32Const(g.ko.fileLen)
			g.e.I32Const(x.Line)
			g.e.Call(g.rt.Set)
			g.e.Drop()
		}
		g.e.LocalGet(tmp)
	case *parser.UnaryExpr:
		g.genUnary(x)
	case *parser.BinaryExpr:
		g.genBinary(x)
	case *parser.IndexExpr:
		g.genExpr(x.Left)
		g.genExpr(x.Index)
		g.e.I32Const(g.ko.filePtr)
		g.e.I32Const(g.ko.fileLen)
		g.e.I32Const(x.Line)
		g.e.Call(g.rt.Get)
	case *parser.CallExpr:
		g.genCall(x)
	case *parser.FuncRefExpr:
		g.e.Call(g.funcIdx[x.Name])
	default:
		panic(fmt.Sprintf("wasm backend: unexpected expression %T", n))
	}
}

func (g *gen) genUnary(x *parser.UnaryExpr) {
	switch x.Operator {
	case "-":
		g.genExpr(x.Operand)
		g.e.Call(g.rt.Neg)
	case "!":
		g.genExpr(x.Operand)
		g.e.Call(g.rt.Not)
	default:
		g.genExpr(x.Operand)
	}
}

func (g *gen) genBinary(x *parser.BinaryExpr) {
	switch x.Operator {
	case "=":
		g.genAssign(x)
	default:
		g.genArith(x)
	}
}

// genAssign emits `lhs = rhs`, leaving the assigned value (or, for index
// assignment, the container) on the stack.
func (g *gen) genAssign(x *parser.BinaryExpr) {
	if id, ok := x.Left.(*parser.Identifier); ok {
		idx := g.lookup(id.Name)
		if idx < 0 {
			panic(fmt.Sprintf("wasm backend: undefined identifier %q", id.Name))
		}
		g.genExpr(x.Right)
		g.e.LocalTee(idx)
		return
	}
	if ix, ok := x.Left.(*parser.IndexExpr); ok {
		g.genExpr(ix.Left)
		g.genExpr(ix.Index)
		g.genExpr(x.Right)
		g.e.I32Const(g.ko.filePtr)
		g.e.I32Const(g.ko.fileLen)
		g.e.I32Const(ix.Line)
		g.e.Call(g.rt.Set)
		return
	}
	panic(fmt.Sprintf("wasm backend: unsupported assignment target %T", x.Left))
}

func (g *gen) genArith(x *parser.BinaryExpr) {
	switch x.Operator {
	case "&&", "||":
		g.genShortCircuit(x)
		return
	}
	g.genExpr(x.Left)
	g.genExpr(x.Right)
	switch x.Operator {
	case "+":
		g.e.Call(g.rt.Add)
	case "-":
		g.e.Call(g.rt.Sub)
	case "*":
		g.e.Call(g.rt.Mul)
	case "/":
		g.rtArgs(x.Line)
		g.e.Call(g.rt.Div)
	case "%":
		g.rtArgs(x.Line)
		g.e.Call(g.rt.Mod)
	case "==":
		g.e.Call(g.rt.Eq)
	case "!=":
		g.e.Call(g.rt.Ne)
	case "<":
		g.e.Call(g.rt.Lt)
	case ">":
		g.e.Call(g.rt.Gt)
	case "<=":
		g.e.Call(g.rt.Le)
	case ">=":
		g.e.Call(g.rt.Ge)
	case "+=", "-=", "*=", "/=", "%=":
		g.genCompound(x)
	default:
		panic(fmt.Sprintf("wasm backend: unsupported operator %q", x.Operator))
	}
}

// rtArgs pushes the file/line triple for checked operations.
func (g *gen) rtArgs(line int) {
	g.e.I32Const(g.ko.filePtr)
	g.e.I32Const(g.ko.fileLen)
	g.e.I32Const(line)
}

// genCompound emits `lhs op= rhs`. lhs must be an identifier; an i64 copy of
// the result is left on the stack.
func (g *gen) genCompound(x *parser.BinaryExpr) {
	id, ok := x.Left.(*parser.Identifier)
	if !ok {
		panic(fmt.Sprintf("wasm backend: compound assignment requires identifier, got %T", x.Left))
	}
	idx := g.lookup(id.Name)
	if idx < 0 {
		panic(fmt.Sprintf("wasm backend: undefined identifier %q", id.Name))
	}
	g.e.LocalGet(idx)
	g.genExpr(x.Right)
	switch x.Operator {
	case "+=":
		g.e.Call(g.rt.Add)
	case "-=":
		g.e.Call(g.rt.Sub)
	case "*=":
		g.e.Call(g.rt.Mul)
	case "/=":
		g.rtArgs(x.Line)
		g.e.Call(g.rt.Div)
	case "%=":
		g.rtArgs(x.Line)
		g.e.Call(g.rt.Mod)
	}
	g.e.LocalTee(idx)
}

// genShortCircuit emits &&/|| leaving an int Value (0 or 2) on the stack.
func (g *gen) genShortCircuit(x *parser.BinaryExpr) {
	g.genExpr(x.Left)
	g.e.Call(g.rt.IsTruthy)
	if x.Operator == "&&" {
		g.e.BeginIf(I64)
		g.genExpr(x.Right)
		g.e.Call(g.rt.IsTruthy)
		g.e.I64ExtendI32U()
		g.e.I64Const(1)
		g.e.I64Shl()
		g.e.Else()
		g.e.I64Const(0)
		g.e.Close()
	} else { // ||
		g.e.BeginIf(I64)
		g.e.I64Const(2)
		g.e.Else()
		g.genExpr(x.Right)
		g.e.Call(g.rt.IsTruthy)
		g.e.I64ExtendI32U()
		g.e.I64Const(1)
		g.e.I64Shl()
		g.e.Close()
	}
}

func (g *gen) genCall(x *parser.CallExpr) {
	if idx, ok := g.funcIdx[x.Function]; ok {
		for _, a := range x.Args {
			g.genExpr(a)
		}
		g.e.Call(idx)
		return
	}
	switch x.Function {
	case "len":
		g.genExpr(x.Args[0])
		g.e.Call(g.rt.Len)
	default:
		panic(fmt.Sprintf("wasm backend: unknown function %q", x.Function))
	}
}

// --- unsupported feature scan ---

func (g *gen) scanUnsupported(prog *parser.Program, fnames map[string]bool) error {
	var errs []string
	add := func(feature string) {
		errs = append(errs, fmt.Sprintf("error K108: feature '%s' is not supported for target wasm32-wasi", feature))
	}

	checkCall := func(c *parser.CallExpr) {
		if c.Module != "" {
			add("module-qualified calls")
			return
		}
		if c.IsCFunc {
			add("C-interop")
			return
		}
		if fnames[c.Function] {
			return
		}
		switch c.Function {
		case "len":
			return
		case "assert", "assert_eq", "assert_ne":
			add("assertions")
		case "spawn", "join", "send", "receive", "channel", "actor", "actorSend", "actorState", "setActorState", "actorStop", "wait_all", "chanSend", "chanClose":
			add("concurrency")
		case "getArgs":
			add("getArgs")
		case "sqrt", "pow", "abs", "fabs", "floor", "ceil", "round", "sin", "cos", "tan":
			add("math builtins")
		default:
			add("builtin '" + c.Function + "'")
		}
	}

	var walk func(n parser.Node)
	walkExpr := func(n parser.Node) {
		if n == nil {
			return
		}
		walk(n)
	}
	walk = func(n parser.Node) {
		switch x := n.(type) {
		case *parser.FuncDecl:
			if x.Target != "" && x.Target != "cpu" {
				add("@target(" + x.Target + ")")
			}
			if len(x.GenericParams) > 0 {
				add("generics")
			}
			for _, t := range x.ParamTypes {
				if t != "" && t != "int" && t != "string" && t != "array" && t != "bool" {
					add("type annotation '" + t + "'")
				}
			}
			for _, s := range x.Body {
				walk(s)
			}
		case *parser.VarDeclStmt:
			if x.IsMatrix || x.IsSIMD {
				add("matrices/SIMD")
			}
			if x.Type != "" && x.Type != "int" && x.Type != "string" && x.Type != "array" && x.Type != "bool" {
				add("type annotation '" + x.Type + "'")
			}
			walkExpr(x.Value)
		case *parser.BlockStmt:
			for _, s := range x.Statements {
				walk(s)
			}
		case *parser.IfStmt:
			walkExpr(x.Condition)
			for _, s := range x.Consequence {
				walk(s)
			}
			for _, s := range x.Alternative {
				walk(s)
			}
		case *parser.WhileStmt:
			walkExpr(x.Condition)
			for _, s := range x.Body {
				walk(s)
			}
		case *parser.ForStmt:
			walkExpr(x.Init)
			walkExpr(x.Condition)
			walkExpr(x.Post)
			for _, s := range x.Body {
				walk(s)
			}
		case *parser.ForInStmt:
			walkExpr(x.Iter)
			for _, s := range x.Body {
				walk(s)
			}
		case *parser.PrintStmt:
			walkExpr(x.Value)
		case *parser.ReturnStmt:
			walkExpr(x.Value)
		case *parser.ExprStmt:
			walkExpr(x.Expression)
		case *parser.Float64Literal:
			add("floats")
		case *parser.BigIntLiteral, *parser.BigFloatLiteral:
			add("bignums")
		case *parser.MapLiteral:
			add("maps")
		case *parser.SliceExpr:
			add("slices")
		case *parser.LambdaExpr:
			add("closures")
		case *parser.FuncRefExpr:
			add("function references")
		case *parser.StructLiteral:
			add("structs")
		case *parser.BorrowExpr, *parser.MoveExpr, *parser.RawAccessExpr:
			add("references/@raw")
		case *parser.SpawnExpr:
			add("concurrency")
			for _, a := range x.Args {
				walkExpr(a)
			}
		case *parser.SendExpr:
			add("concurrency")
			walkExpr(x.Channel)
			walkExpr(x.Message)
			walkExpr(x.Timeout)
		case *parser.ReceiveStmt:
			add("concurrency")
			walkExpr(x.Channel)
		case *parser.BinaryExpr:
			walkExpr(x.Left)
			walkExpr(x.Right)
		case *parser.UnaryExpr:
			walkExpr(x.Operand)
		case *parser.IndexExpr:
			walkExpr(x.Left)
			walkExpr(x.Index)
		case *parser.ArrayLiteral:
			for _, el := range x.Elements {
				walkExpr(el)
			}
		case *parser.CallExpr:
			checkCall(x)
			for _, a := range x.Args {
				walkExpr(a)
			}
		case *parser.Identifier, *parser.StringLiteral, *parser.IntLiteral, *parser.BoolLiteral:
			// always supported
		default:
			add("construct '" + fmt.Sprintf("%T", n) + "'")
		}
	}

	for _, stmt := range prog.Statements {
		walk(stmt)
	}
	for _, ci := range prog.CImports {
		_ = ci
		add("C-interop")
	}
	if len(prog.Imports) > 0 {
		add("modules/imports")
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}