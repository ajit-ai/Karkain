package hir

import (
	"karkain/pkg/parser"
)

// BuildHIR converts a parser AST Program into a HIR Module.
// This is the first stage of the compilation pipeline after parsing.
func BuildHIR(prog *parser.Program) *Module {
	m := &Module{}

	for _, imp := range prog.Imports {
		m.Imports = append(m.Imports, imp.Name)
	}

	for _, node := range prog.Statements {
		switch n := node.(type) {
		case *parser.FuncDecl:
			m.Functions = append(m.Functions, lowerFuncDecl(n))
		case *parser.StructDeclStmt:
			m.Structs = append(m.Structs, lowerStructDecl(n))
		case *parser.EnumDecl:
			m.Enums = append(m.Enums, lowerEnumDecl(n))
		case *parser.TraitDeclStmt:
			m.Traits = append(m.Traits, lowerTraitDecl(n))
		case *parser.ImplDeclStmt:
			m.Impls = append(m.Impls, lowerImplDecl(n))
		case *parser.VarDeclStmt:
			m.Globals = append(m.Globals, lowerGlobalVar(n))
		}
	}

	return m
}

// ─── Function lowering ───────────────────────────────────────────

func lowerFuncDecl(f *parser.FuncDecl) *Function {
	fn := &Function{
		Name:       f.Name,
		Public:     f.Public,
		Line:       f.Line,
		ReturnType: &Type{Kind: TypeUnknown},
	}

	for i, p := range f.Params {
		pt := &Type{Kind: TypeUnknown}
		if i < len(f.ParamTypes) {
			pt = parseTypeStr(f.ParamTypes[i])
		}
		fn.Params = append(fn.Params, &Param{Name: p, Type: pt})
	}

	for _, g := range f.GenericParams {
		fn.Generic = append(fn.Generic, &GenericParam{
			Name:        g.Name,
			Constraints: g.Constraints,
		})
	}

	fn.Captures = f.Captures
	fn.Body = lowerBlock(f.Body)

	return fn
}

// ─── Statement lowering ──────────────────────────────────────────

func lowerBlock(nodes []parser.Node) []Stmt {
	var stmts []Stmt
	for _, n := range nodes {
		stmts = append(stmts, lowerStmt(n))
	}
	return stmts
}

func lowerStmt(node parser.Node) Stmt {
	switch n := node.(type) {
	case *parser.ReturnStmt:
		return Stmt{
			Kind:      StmtReturn,
			Line:      n.Line,
			ReturnVal: lowerExpr(n.Value),
		}
	case *parser.VarDeclStmt:
		return Stmt{
			Kind:         StmtVarDecl,
			Line:         n.Line,
			VarName:      n.Name,
			VarType:      parseTypeStr(n.Type),
			VarValue:     lowerExpr(n.Value),
			VarIsMutable: false,
		}
	case *parser.IfStmt:
		s := Stmt{
			Kind:   StmtIf,
			Line:   n.Line,
			IfCond: lowerExpr(n.Condition),
			IfThen: lowerBlock(n.Consequence),
		}
		if len(n.Alternative) > 0 {
			s.IfElse = lowerBlock(n.Alternative)
		}
		return s
	case *parser.WhileStmt:
		return Stmt{
			Kind:      StmtWhile,
			Line:      n.Line,
			WhileCond: lowerExpr(n.Condition),
			WhileBody: lowerBlock(n.Body),
		}
	case *parser.ForStmt:
		s := Stmt{
			Kind:    StmtFor,
			Line:    n.Line,
			ForCond: lowerExpr(n.Condition),
			ForBody: lowerBlock(n.Body),
		}
		if n.Init != nil {
			init := lowerStmt(n.Init)
			s.ForInit = &init
		}
		if n.Post != nil {
			post := lowerStmt(n.Post)
			s.ForPost = &post
		}
		return s
	case *parser.ForInStmt:
		return Stmt{
			Kind:         StmtForIn,
			Line:         n.Line,
			ForInVar:     n.VarName,
			ForInKeyName: n.KeyName,
			ForInIter:    lowerExpr(n.Iter),
			ForInBody:    lowerBlock(n.Body),
		}
	case *parser.BreakStmt:
		return Stmt{Kind: StmtBreak, Line: n.Line}
	case *parser.ContinueStmt:
		return Stmt{Kind: StmtContinue, Line: n.Line}
	case *parser.PrintStmt:
		return Stmt{
			Kind:     StmtPrint,
			Line:     n.Line,
			PrintVal: lowerExpr(n.Value),
		}
	case *parser.ExprStmt:
		return Stmt{
			Kind:     StmtExpr,
			Line:     n.Line,
			ExprStmt: lowerExpr(n.Expression),
		}
	case *parser.BlockStmt:
		return Stmt{
			Kind:       StmtBlock,
			BlockStmts: lowerBlock(n.Statements),
		}
	case *parser.MatchExpr:
		s := Stmt{
			Kind:       StmtMatch,
			Line:       n.Line,
			MatchValue: lowerExpr(n.Value),
		}
		for _, arm := range n.Arms {
			s.MatchArms = append(s.MatchArms, lowerMatchArm(&arm))
		}
		return s
	case *parser.ReceiveStmt:
		return Stmt{
			Kind:        StmtReceive,
			Line:        n.Line,
			RecvChannel: lowerExpr(n.Channel),
			RecvVarName: n.VarName,
		}
	case *parser.QubitAssignStmt:
		return Stmt{
			Kind:       StmtQubitAssign,
			Line:       n.Line,
			QubitName:  n.Name,
			QubitSize:  n.Size,
			QubitInit:  n.Init,
		}
	default:
		return Stmt{
			Kind:     StmtExpr,
			Line:     parser.GetLine(node),
			ExprStmt: lowerExpr(node),
		}
	}
}

func lowerMatchArm(arm *parser.MatchArm) *MatchArm {
	return &MatchArm{
		Pattern: lowerMatchPattern(&arm.Pattern),
		Body:    []Stmt{lowerStmt(arm.Body)},
	}
}

func lowerMatchPattern(p *parser.MatchPattern) *MatchPattern {
	mp := &MatchPattern{
		Kind:    p.Type,
		Binding: p.Binding,
	}
	if p.Value != nil {
		mp.Value = lowerExpr(p.Value)
	}
	return mp
}

// ─── Expression lowering ─────────────────────────────────────────

func lowerExpr(node parser.Node) *Expr {
	if node == nil {
		return &Expr{Kind: ExprInt}
	}

	switch n := node.(type) {
	case *parser.IntLiteral:
		return &Expr{Kind: ExprInt, IntVal: n.Value, Line: n.Line}
	case *parser.Float64Literal:
		return &Expr{Kind: ExprFloat, FloatVal: n.Value, Line: n.Line}
	case *parser.StringLiteral:
		return &Expr{Kind: ExprString, StringVal: n.Value, Line: n.Line}
	case *parser.BoolLiteral:
		return &Expr{Kind: ExprBool, BoolVal: n.Value, Line: n.Line}
	case *parser.Identifier:
		return &Expr{Kind: ExprIdent, IdentName: n.Name, Line: n.Line}
	case *parser.BinaryExpr:
		return &Expr{
			Kind:     ExprBinary,
			BinOp:    n.Operator,
			BinLeft:  lowerExpr(n.Left),
			BinRight: lowerExpr(n.Right),
			Line:     n.Line,
		}
	case *parser.UnaryExpr:
		return &Expr{
			Kind:      ExprUnary,
			UnOp:      n.Operator,
			UnOperand: lowerExpr(n.Operand),
			Line:      n.Line,
		}
	case *parser.CallExpr:
		e := &Expr{
			Kind:     ExprCall,
			CallFunc: n.Function,
			CallIsC:  n.IsCFunc,
			Line:     n.Line,
		}
		for _, a := range n.Args {
			e.CallArgs = append(e.CallArgs, lowerExpr(a))
		}
		return e
	case *parser.ArrayLiteral:
		e := &Expr{Kind: ExprArrayLit, Line: n.Line}
		for _, el := range n.Elements {
			e.ArrayElems = append(e.ArrayElems, lowerExpr(el))
		}
		return e
	case *parser.MapLiteral:
		e := &Expr{Kind: ExprMapLit, Line: n.Line}
		for i, k := range n.Keys {
			e.MapKeys = append(e.MapKeys, lowerExpr(k))
			if i < len(n.Values) {
				e.MapValues = append(e.MapValues, lowerExpr(n.Values[i]))
			}
		}
		return e
	case *parser.IndexExpr:
		return &Expr{
			Kind:        ExprIndex,
			IndexTarget: lowerExpr(n.Left),
			IndexKey:    lowerExpr(n.Index),
			Line:        n.Line,
		}
	case *parser.SliceExpr:
		return &Expr{
			Kind:        ExprSlice,
			SliceTarget: lowerExpr(n.Target),
			SliceStart:  lowerExpr(n.Start),
			SliceEnd:    lowerExpr(n.End),
			Line:        n.Line,
		}
	case *parser.StructLiteral:
		e := &Expr{
			Kind:           ExprStructLit,
			StructTypeName: n.TypeName,
			Line:           n.Line,
		}
		for _, f := range n.Fields {
			e.StructFields = append(e.StructFields, lowerExpr(f))
		}
		return e
	case *parser.DotExpr:
		return &Expr{
			Kind:      ExprDot,
			DotLeft:   lowerExpr(n.Left),
			DotRight:  n.Right,
			Line:      n.Line,
		}
	case *parser.LambdaExpr:
		e := &Expr{
			Kind:           ExprLambda,
			Line:           n.Line,
			LambdaCaptures: n.Captures,
		}
		for i, p := range n.Params {
			pt := &Type{Kind: TypeUnknown}
			if i < len(n.ParamTypes) {
				pt = parseTypeStr(n.ParamTypes[i])
			}
			e.LambdaParams = append(e.LambdaParams, &Param{Name: p, Type: pt})
		}
		e.LambdaBody = lowerBlock(n.Body)
		return e
	case *parser.FuncRefExpr:
		return &Expr{Kind: ExprFuncRef, FuncRefName: n.Name, Line: n.Line}
	case *parser.AddressOf:
		return &Expr{Kind: ExprAddressOf, MemOperand: lowerExpr(n.Operand), Line: n.Line}
	case *parser.Dereference:
		return &Expr{Kind: ExprDereference, MemOperand: lowerExpr(n.Operand), Line: n.Line}
	case *parser.BorrowExpr:
		return &Expr{Kind: ExprBorrow, MemOperand: lowerExpr(n.Operand), MemMutable: n.Mutable, Line: n.Line}
	case *parser.MoveExpr:
		return &Expr{Kind: ExprMove, MemOperand: lowerExpr(n.Operand), Line: n.Line}
	case *parser.AllocExpr:
		return &Expr{Kind: ExprAlloc, MemType: parseTypeStr(n.Type), MemCount: lowerExpr(n.Count), Line: n.Line}
	case *parser.FreeExpr:
		return &Expr{Kind: ExprFree, MemOperand: lowerExpr(n.Ptr), Line: n.Line}
	case *parser.OptionSomeExpr:
		return &Expr{Kind: ExprOptionSome, OptVal: lowerExpr(n.Value), Line: n.Line}
	case *parser.OptionNoneExpr:
		return &Expr{Kind: ExprOptionNone, Line: n.Line}
	case *parser.ResultOkExpr:
		return &Expr{Kind: ExprResultOk, ResVal: lowerExpr(n.Value), Line: n.Line}
	case *parser.ResultErrExpr:
		return &Expr{Kind: ExprResultErr, ResErr: lowerExpr(n.Error), Line: n.Line}
	case *parser.PropagateExpr:
		return &Expr{Kind: ExprPropagate, PropagateOperand: lowerExpr(n.Operand), Line: n.Line}
	case *parser.EnumVariantExpr:
		return &Expr{
			Kind:        ExprEnumVariant,
			EnumName:    n.EnumName,
			Variant:     n.Variant,
			VariantVal:  lowerExpr(n.Value),
			Line:        n.Line,
		}
	case *parser.SIMDBuiltinExpr:
		e := &Expr{Kind: ExprSIMDBuiltin, SIMDOp: n.Op, Line: n.Line}
		for _, a := range n.Args {
			e.SIMDArgs = append(e.SIMDArgs, lowerExpr(a))
		}
		return e
	case *parser.AtomicOp:
		e := &Expr{Kind: ExprAtomicOp, AtomicOp: n.Op, AtomicOrder: n.Order, Line: n.Line}
		for _, a := range n.Args {
			e.AtomicArgs = append(e.AtomicArgs, lowerExpr(a))
		}
		return e
	case *parser.GateApplyStmt:
		e := &Expr{
			Kind:    ExprGateApply,
			QGate:   n.Gate,
			QTarget: lowerExpr(n.Target),
			Line:    n.Line,
		}
		if n.Control != nil {
			e.QControl = lowerExpr(n.Control)
		}
		for _, p := range n.Params {
			e.QParams = append(e.QParams, lowerExpr(p))
		}
		return e
	case *parser.MeasureExpr:
		return &Expr{Kind: ExprMeasure, QMeasure: lowerExpr(n.Qubit), Line: n.Line}
	case *parser.SpawnExpr:
		e := &Expr{Kind: ExprSpawn, SpawnActor: n.ActorName, Line: n.Line}
		for _, a := range n.Args {
			e.SpawnArgs = append(e.SpawnArgs, lowerExpr(a))
		}
		return e
	case *parser.SendExpr:
		return &Expr{
			Kind:     ExprSend,
			SendChan: lowerExpr(n.Channel),
			SendMsg:  lowerExpr(n.Message),
			SendSync: n.IsSync,
			Line:     n.Line,
		}
	case *parser.ChDeclExpr:
		return &Expr{
			Kind:       ExprChannelDecl,
			ChElemType: parseTypeStr(n.ElementType),
			ChBufSize:  lowerExpr(n.BufferSize),
			Line:       n.Line,
		}
	case *parser.GlobalIdExpr:
		return &Expr{Kind: ExprGlobalId, GPUDim: n.Dimension, Line: n.Line}
	case *parser.TensorOpExpr:
		e := &Expr{Kind: ExprTensorOp, TensorOp: n.Op, Line: n.Line}
		for _, a := range n.Args {
			e.TensorArgs = append(e.TensorArgs, lowerExpr(a))
		}
		e.TensorAttrs = make(map[string]*Expr)
		for k, v := range n.Attrs {
			e.TensorAttrs[k] = lowerExpr(v)
		}
		return e
	case *parser.TensorShapeOfExpr:
		return &Expr{Kind: ExprTensorShapeOf, TensorSrc: lowerExpr(n.Operand), Line: n.Line}
	case *parser.TensorIndexExpr:
		e := &Expr{Kind: ExprTensorIndex, TensorSrc: lowerExpr(n.Source), Line: n.Line}
		for _, idx := range n.Indices {
			e.TensorIdx = append(e.TensorIdx, lowerExpr(idx))
		}
		return e
	case *parser.QPUOpExpr:
		e := &Expr{Kind: ExprQPUOp, QPUOp: n.Op, QAngle: lowerExpr(n.Angle), Line: n.Line}
		for _, a := range n.Args {
			e.QPUArgs = append(e.QPUArgs, lowerExpr(a))
		}
		return e
	case *parser.AwaitExpr:
		return &Expr{
			Kind:          ExprAwait,
			AwaitOperand:  lowerExpr(n.Operand),
			AwaitTimeout:  lowerExpr(n.Timeout),
			Line:          n.Line,
		}
	case *parser.YieldExpr:
		return &Expr{Kind: ExprYield, YieldVal: lowerExpr(n.Value), Line: n.Line}
	case *parser.ChSendExpr:
		return &Expr{
			Kind:     ExprChSend,
			SendChan: lowerExpr(n.Channel),
			SendMsg:  lowerExpr(n.Value),
			Line:     n.Line,
		}
	case *parser.ChRecvExpr:
		return &Expr{Kind: ExprChRecv, SendChan: lowerExpr(n.Channel), Line: n.Line}
	case *parser.AsyncExpr:
		return &Expr{Kind: ExprAsync, AsyncBody: lowerBlock(n.Body), Line: n.Line}
	case *parser.AwaitAllExpr:
		e := &Expr{Kind: ExprAwaitAll, Line: n.Line}
		for _, f := range n.Futures {
			e.AwaitFutures = append(e.AwaitFutures, lowerExpr(f))
		}
		return e
	case *parser.GreenSpawnExpr:
		e := &Expr{Kind: ExprGreenSpawn, GreenFunc: n.Function, Line: n.Line}
		for _, a := range n.Args {
			e.GreenArgs = append(e.GreenArgs, lowerExpr(a))
		}
		return e
	case *parser.ReflectTypeExpr:
		return &Expr{Kind: ExprReflectType, ReflectType: lowerExpr(n.TypeExpr), Line: n.Line}
	case *parser.DeriveExpr:
		e := &Expr{Kind: ExprDerive, DeriveTrait: n.Trait, Line: n.Line}
		e.DeriveTarget = lowerExpr(n.Target)
		for _, a := range n.Args {
			e.DeriveArgs = append(e.DeriveArgs, lowerExpr(a))
		}
		return e
	case *parser.TagExpr:
		return &Expr{
			Kind:       ExprTag,
			TagTarget:  lowerExpr(n.Target),
			TagName:    n.TagName,
			TagValue:   n.TagValue,
			Line:       n.Line,
		}
	case *parser.ComptimeExpr:
		return &Expr{Kind: ExprComptime, ComptimeExpr: lowerExpr(n.Expr), Line: n.Line}
	default:
		return &Expr{Kind: ExprInt, Line: parser.GetLine(node)}
	}
}

// ─── Declaration lowering ────────────────────────────────────────

func lowerStructDecl(s *parser.StructDeclStmt) *StructDecl {
	sd := &StructDecl{
		Name:   s.Name,
		Public: s.Public,
		Line:   s.Line,
	}
	for _, f := range s.Fields {
		sd.Fields = append(sd.Fields, &Field{
			Name: f.Name,
			Type: parseTypeStr(f.Type),
		})
	}
	for _, g := range s.GenericParams {
		sd.Generic = append(sd.Generic, &GenericParam{
			Name:        g.Name,
			Constraints: g.Constraints,
		})
	}
	return sd
}

func lowerEnumDecl(e *parser.EnumDecl) *EnumDecl {
	ed := &EnumDecl{
		Name:   e.Name,
		Public: e.Public,
		Line:   e.Line,
	}
	for _, v := range e.Variants {
		ed.Variants = append(ed.Variants, &Variant{
			Name:    v.Name,
			Payload: parseTypeStr(v.Payload),
		})
	}
	return ed
}

func lowerTraitDecl(t *parser.TraitDeclStmt) *TraitDecl {
	td := &TraitDecl{
		Name: t.Name,
		Line: t.Line,
	}
	for _, m := range t.Methods {
		td.Methods = append(td.Methods, &FuncSignature{
			Name:       m.Name,
			ReturnType: parseTypeStr(m.ReturnType),
		})
	}
	return td
}

func lowerImplDecl(i *parser.ImplDeclStmt) *ImplDecl {
	id := &ImplDecl{
		TraitName: i.TraitName,
		ForType:   i.ForType,
		Line:      i.Line,
	}
	for _, m := range i.Methods {
		method := m
		id.Methods = append(id.Methods, lowerFuncDecl(&method))
	}
	return id
}

func lowerGlobalVar(v *parser.VarDeclStmt) *GlobalVar {
	return &GlobalVar{
		Name:   v.Name,
		Type:   parseTypeStr(v.Type),
		Value:  lowerExpr(v.Value),
		Public: false,
		Line:   v.Line,
	}
}

// ─── Type resolution helpers ─────────────────────────────────────

func parseTypeStr(s string) *Type {
	if s == "" {
		return &Type{Kind: TypeUnknown}
	}

	// Handle pointer types
	if len(s) > 0 && s[0] == '*' {
		return &Type{Kind: TypePtr, Elem: parseTypeStr(s[1:])}
	}

	// Handle array types
	if len(s) > 2 && s[len(s)-2:] == "[]" {
		return &Type{Kind: TypeArray, Elem: parseTypeStr(s[:len(s)-2])}
	}

	switch s {
	case "int":
		return &Type{Kind: TypeInt}
	case "float64":
		return &Type{Kind: TypeFloat}
	case "bool":
		return &Type{Kind: TypeBool}
	case "string":
		return &Type{Kind: TypeString}
	case "void":
		return &Type{Kind: TypeVoid}
	default:
		return &Type{Kind: TypeStruct, Name: s}
	}
}
