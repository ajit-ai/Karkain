package hir

import (
	"fmt"
	"strings"
)

// Format returns a human-readable string representation of the HIR Module.
func (m *Module) Format() string {
	var b strings.Builder

	for _, imp := range m.Imports {
		fmt.Fprintf(&b, "import %s\n", imp)
	}
	if len(m.Imports) > 0 {
		b.WriteString("\n")
	}

	for _, s := range m.Structs {
		b.WriteString(s.Format())
		b.WriteString("\n")
	}

	for _, e := range m.Enums {
		b.WriteString(e.Format())
		b.WriteString("\n")
	}

	for _, t := range m.Traits {
		b.WriteString(t.Format())
		b.WriteString("\n")
	}

	for _, fn := range m.Functions {
		b.WriteString(fn.Format())
		b.WriteString("\n")
	}

	return b.String()
}

func (s *StructDecl) Format() string {
	var b strings.Builder
	if s.Public {
		b.WriteString("pub ")
	}
	fmt.Fprintf(&b, "struct %s", s.Name)
	if len(s.Generic) > 0 {
		b.WriteString("[")
		for i, g := range s.Generic {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(g.Name)
		}
		b.WriteString("]")
	}
	b.WriteString(" {\n")
	for _, f := range s.Fields {
		fmt.Fprintf(&b, "    %s: %s\n", f.Name, f.Type)
	}
	b.WriteString("}")
	return b.String()
}

func (e *EnumDecl) Format() string {
	var b strings.Builder
	if e.Public {
		b.WriteString("pub ")
	}
	fmt.Fprintf(&b, "enum %s {\n", e.Name)
	for _, v := range e.Variants {
		if v.Payload != nil {
			fmt.Fprintf(&b, "    %s(%s)\n", v.Name, v.Payload)
		} else {
			fmt.Fprintf(&b, "    %s\n", v.Name)
		}
	}
	b.WriteString("}")
	return b.String()
}

func (t *TraitDecl) Format() string {
	var b strings.Builder
	fmt.Fprintf(&b, "trait %s {\n", t.Name)
	for _, m := range t.Methods {
		fmt.Fprintf(&b, "    fn %s() -> %s\n", m.Name, m.ReturnType)
	}
	b.WriteString("}")
	return b.String()
}

func (f *Function) Format() string {
	var b strings.Builder
	if f.Public {
		b.WriteString("pub ")
	}
	fmt.Fprintf(&b, "fn %s(", f.Name)
	for i, p := range f.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%s: %s", p.Name, p.Type)
	}
	b.WriteString(")")
	if f.ReturnType != nil && f.ReturnType.Kind != TypeVoid {
		fmt.Fprintf(&b, " -> %s", f.ReturnType)
	}
	b.WriteString(" {\n")
	for _, s := range f.Body {
		formatStmt(&b, s, 1)
	}
	b.WriteString("}")
	return b.String()
}

func formatStmt(b *strings.Builder, s Stmt, indent int) {
	prefix := strings.Repeat("    ", indent)

	switch s.Kind {
	case StmtReturn:
		fmt.Fprintf(b, "%sreturn %s\n", prefix, formatExpr(s.ReturnVal))
	case StmtVarDecl:
		mut := ""
		if s.VarIsMutable {
			mut = "mut "
		}
		if s.VarType != nil && s.VarType.Kind != TypeUnknown {
			fmt.Fprintf(b, "%slet %s%s: %s = %s\n", prefix, mut, s.VarName, s.VarType, formatExpr(s.VarValue))
		} else {
			fmt.Fprintf(b, "%slet %s%s = %s\n", prefix, mut, s.VarName, formatExpr(s.VarValue))
		}
	case StmtAssign:
		fmt.Fprintf(b, "%s%s = %s\n", prefix, formatExpr(s.AssignTarget), formatExpr(s.AssignValue))
	case StmtIf:
		fmt.Fprintf(b, "%sif %s {\n", prefix, formatExpr(s.IfCond))
		for _, st := range s.IfThen {
			formatStmt(b, st, indent+1)
		}
		if len(s.IfElse) > 0 {
			fmt.Fprintf(b, "%s} else {\n", prefix)
			for _, st := range s.IfElse {
				formatStmt(b, st, indent+1)
			}
		}
		fmt.Fprintf(b, "%s}\n", prefix)
	case StmtWhile:
		fmt.Fprintf(b, "%swhile %s {\n", prefix, formatExpr(s.WhileCond))
		for _, st := range s.WhileBody {
			formatStmt(b, st, indent+1)
		}
		fmt.Fprintf(b, "%s}\n", prefix)
	case StmtFor:
		initStr := ""
		if s.ForInit != nil {
			initStr = formatStmtInline(*s.ForInit)
		}
		fmt.Fprintf(b, "%sfor %s; %s; {\n", prefix,
			initStr,
			formatExpr(s.ForCond))
		for _, st := range s.ForBody {
			formatStmt(b, st, indent+1)
		}
		fmt.Fprintf(b, "%s}\n", prefix)
	case StmtForIn:
		fmt.Fprintf(b, "%sfor %s in %s {\n", prefix, s.ForInVar, formatExpr(s.ForInIter))
		for _, st := range s.ForInBody {
			formatStmt(b, st, indent+1)
		}
		fmt.Fprintf(b, "%s}\n", prefix)
	case StmtBreak:
		fmt.Fprintf(b, "%sbreak\n", prefix)
	case StmtContinue:
		fmt.Fprintf(b, "%scontinue\n", prefix)
	case StmtPrint:
		fmt.Fprintf(b, "%sprint %s\n", prefix, formatExpr(s.PrintVal))
	case StmtBlock:
		fmt.Fprintf(b, "%s{\n", prefix)
		for _, st := range s.BlockStmts {
			formatStmt(b, st, indent+1)
		}
		fmt.Fprintf(b, "%s}\n", prefix)
	case StmtMatch:
		fmt.Fprintf(b, "%smatch %s {\n", prefix, formatExpr(s.MatchValue))
		for _, arm := range s.MatchArms {
			fmt.Fprintf(b, "%s    %s => {\n", prefix, formatExpr(arm.Pattern.Value))
			for _, st := range arm.Body {
				formatStmt(b, st, indent+2)
			}
			fmt.Fprintf(b, "%s    }\n", prefix)
		}
		fmt.Fprintf(b, "%s}\n", prefix)
	case StmtReceive:
		fmt.Fprintf(b, "%sreceive %s -> %s\n", prefix, formatExpr(s.RecvChannel), s.RecvVarName)
	case StmtQubitAssign:
		if s.QubitInit {
			fmt.Fprintf(b, "%squbit %s[%d]\n", prefix, s.QubitName, s.QubitSize)
		} else {
			fmt.Fprintf(b, "%s%s = qubit[%d]\n", prefix, s.QubitName, s.QubitSize)
		}
	case StmtExpr:
		fmt.Fprintf(b, "%s%s\n", prefix, formatExpr(s.ExprStmt))
	}
}

func formatStmtInline(s Stmt) string {
	switch s.Kind {
	case StmtVarDecl:
		if s.VarType != nil && s.VarType.Kind != TypeUnknown {
			return fmt.Sprintf("let %s: %s = %s", s.VarName, s.VarType, formatExpr(s.VarValue))
		}
		return fmt.Sprintf("let %s = %s", s.VarName, formatExpr(s.VarValue))
	case StmtAssign:
		return fmt.Sprintf("%s = %s", formatExpr(s.AssignTarget), formatExpr(s.AssignValue))
	case StmtExpr:
		return formatExpr(s.ExprStmt)
	default:
		return ""
	}
}

func formatExpr(e *Expr) string {
	if e == nil {
		return "<?>"
	}
	switch e.Kind {
	case ExprInt:
		return e.IntVal
	case ExprFloat:
		return e.FloatVal
	case ExprString:
		return fmt.Sprintf("%q", e.StringVal)
	case ExprBool:
		if e.BoolVal {
			return "true"
		}
		return "false"
	case ExprIdent:
		return e.IdentName
	case ExprBinary:
		return fmt.Sprintf("(%s %s %s)", formatExpr(e.BinLeft), e.BinOp, formatExpr(e.BinRight))
	case ExprUnary:
		return fmt.Sprintf("(%s%s)", e.UnOp, formatExpr(e.UnOperand))
	case ExprCall:
		args := make([]string, len(e.CallArgs))
		for i, a := range e.CallArgs {
			args[i] = formatExpr(a)
		}
		prefix := ""
		if e.CallIsC {
			prefix = "c:"
		}
		return fmt.Sprintf("%s%s(%s)", prefix, e.CallFunc, strings.Join(args, ", "))
	case ExprIndex:
		return fmt.Sprintf("%s[%s]", formatExpr(e.IndexTarget), formatExpr(e.IndexKey))
	case ExprSlice:
		return fmt.Sprintf("%s[%s..%s]", formatExpr(e.SliceTarget), formatExpr(e.SliceStart), formatExpr(e.SliceEnd))
	case ExprArrayLit:
		elems := make([]string, len(e.ArrayElems))
		for i, el := range e.ArrayElems {
			elems[i] = formatExpr(el)
		}
		return fmt.Sprintf("[%s]", strings.Join(elems, ", "))
	case ExprMapLit:
		pairs := make([]string, len(e.MapKeys))
		for i, k := range e.MapKeys {
			pairs[i] = fmt.Sprintf("%s: %s", formatExpr(k), formatExpr(e.MapValues[i]))
		}
		return fmt.Sprintf("{%s}", strings.Join(pairs, ", "))
	case ExprStructLit:
		return fmt.Sprintf("%s{...}", e.StructTypeName)
	case ExprDot:
		return fmt.Sprintf("%s.%s", formatExpr(e.DotLeft), e.DotRight)
	case ExprLambda:
		params := make([]string, len(e.LambdaParams))
		for i, p := range e.LambdaParams {
			params[i] = fmt.Sprintf("%s: %s", p.Name, p.Type)
		}
		return fmt.Sprintf("fn(%s) { ... }", strings.Join(params, ", "))
	case ExprFuncRef:
		return e.FuncRefName
	case ExprAddressOf:
		return fmt.Sprintf("&%s", formatExpr(e.MemOperand))
	case ExprDereference:
		return fmt.Sprintf("*%s", formatExpr(e.MemOperand))
	case ExprBorrow:
		return fmt.Sprintf("&%s", formatExpr(e.MemOperand))
	case ExprMove:
		return fmt.Sprintf("move %s", formatExpr(e.MemOperand))
	case ExprOptionSome:
		return fmt.Sprintf("Some(%s)", formatExpr(e.OptVal))
	case ExprOptionNone:
		return "None"
	case ExprResultOk:
		return fmt.Sprintf("Ok(%s)", formatExpr(e.ResVal))
	case ExprResultErr:
		return fmt.Sprintf("Err(%s)", formatExpr(e.ResErr))
	case ExprPropagate:
		return fmt.Sprintf("%s?", formatExpr(e.PropagateOperand))
	case ExprEnumVariant:
		if e.VariantVal != nil && e.VariantVal.Kind != ExprInt {
			return fmt.Sprintf("%s::%s(%s)", e.EnumName, e.Variant, formatExpr(e.VariantVal))
		}
		return fmt.Sprintf("%s::%s", e.EnumName, e.Variant)
	case ExprGateApply:
		return fmt.Sprintf("gate %s(%s)", e.QGate, formatExpr(e.QTarget))
	case ExprMeasure:
		return fmt.Sprintf("measure %s", formatExpr(e.QMeasure))
	case ExprSpawn:
		return fmt.Sprintf("spawn %s()", e.SpawnActor)
	case ExprSend:
		return fmt.Sprintf("send %s -> %s", formatExpr(e.SendChan), formatExpr(e.SendMsg))
	case ExprChannelDecl:
		return fmt.Sprintf("channel[%s]", e.ChElemType)
	case ExprGlobalId:
		return fmt.Sprintf("global_id(%d)", e.GPUDim)
	case ExprTensorOp:
		return fmt.Sprintf("tensor.%s()", e.TensorOp)
	case ExprTensorShapeOf:
		return fmt.Sprintf("shape(%s)", formatExpr(e.TensorSrc))
	case ExprAwait:
		return fmt.Sprintf("await %s", formatExpr(e.AwaitOperand))
	case ExprYield:
		return fmt.Sprintf("yield %s", formatExpr(e.YieldVal))
	case ExprAsync:
		return "async { ... }"
	case ExprComptime:
		return fmt.Sprintf("comptime %s", formatExpr(e.ComptimeExpr))
	default:
		return "<?>"
	}
}
