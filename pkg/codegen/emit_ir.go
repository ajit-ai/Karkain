package codegen

import (
	"fmt"
	"karkain/pkg/ir/ssa"
	"karkain/pkg/parser"
	"sort"
	"strconv"
	"strings"
)

// Phase 53: SSA -> C23 emission and pipeline integration.

// unescapeKarkain translates the raw escape sequences stored in Karkain string
// literals into their actual byte values. The lexer intentionally keeps escapes
// un-unescaped in the token/AST value (e.g. "\n" is stored as the two characters
// backslash+'n'), so codegen must decode them before quoting so that the emitted C
// string literal denotes the right value. Without this, a literal "\n" would be
// emitted as backslash+'n' instead of a real newline.
func unescapeKarkain(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' || i+1 >= len(s) {
			b.WriteByte(c)
			continue
		}
		i++
		switch s[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case '0':
			b.WriteByte(0)
		case '\\':
			b.WriteByte('\\')
		case '"':
			b.WriteByte('"')
		case '\'':
			b.WriteByte('\'')
		default:
			b.WriteByte('\\')
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func sanitizeC(id string) string {
	var b strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

func blockParamC(block, param string) string {
	return sanitizeC(block) + "__" + sanitizeC(param)
}

func operandC(op ssa.Operand) string {
	if !op.IsCst {
		return sanitizeC(op.Reg)
	}
	switch op.CstKind {
	case ssa.I:
		return "make_int(" + strconv.FormatInt(op.IntVal, 10) + ")"
	case ssa.F:
		return "mk_float(" + strconv.FormatFloat(op.FltVal, 'g', -1, 64) + ")"
	case ssa.Bool:
		if op.BoolVal {
			return "make_bool(1)"
		}
		return "make_bool(0)"
	case ssa.Str:
		return "make_string(" + strconv.Quote(unescapeKarkain(op.StrVal)) + ")"
	}
	return "mk_nil()"
}

func (g *Generator) genExpressionAsStatement(e parser.Node) string {
	t := strings.TrimSpace(g.genExpr(e))
	if t != "" && !strings.HasSuffix(t, ";") {
		t += ";"
	}
	return t
}

func (g *Generator) emitSSAFunction(fn *ssa.Function, paramNames []string) string {
	var sb strings.Builder
	isMain := fn.Name == "main"

	// Karkain argument API: getArgs() is an intrinsic whose body is supplied by
	// the codegen, returning the argc/argv captured by the generated C entry
	// point. Both the Go bootstrap backend and the self-hosted emitC11 backend
	// must emit this same canonical implementation for parity. The function is
	// still declared in Karkain source (so the parser/self-hosted driver can
	// dispatch on it), but the lowered placeholder body is replaced here.
	if fn.Name == "getArgs" {
		sb.WriteString("Value getArgs(void) {\n")
		sb.WriteString("\tValue _args = make_array();\n")
		sb.WriteString("\tint _i;\n")
		sb.WriteString("\tfor (_i = 0; _i < _karkain_gargc; _i++) {\n")
		sb.WriteString("\t\tarray_push(&_args, make_string(_karkain_gargv[_i]));\n")
		sb.WriteString("\t}\n")
		sb.WriteString("\treturn _args;\n")
		sb.WriteString("}\n")
		return sb.String()
	}

	if isMain {
		// Generated C entry point receives real argv so Karkain's getArgs() can
		// expose the actual process arguments (not a placeholder). Entry
		// parameter names (_karkain_argc/_karkain_argv) and the backing globals
		// (_karkain_gargc/_karkain_gargv) are distinct to avoid colliding with
		// Karkain identifiers such as "argc"/"argv" and to avoid self-assignment.
		sb.WriteString("int main(int _karkain_argc, char** _karkain_argv) {\n")
		sb.WriteString("\t_karkain_gargc = _karkain_argc; _karkain_gargv = _karkain_argv;\n")
	} else {
		params := make([]string, len(fn.Params))
		for i, p := range fn.Params {
			params[i] = "Value " + sanitizeC(p.Name)
		}
		fmt.Fprintf(&sb, "Value %s(%s) {\n", userFuncC(fn.Name), strings.Join(params, ", "))
	}
	// Function parameters are bound directly by the signature — no copies needed.
	// Their names serve as the variable cells.

	blocks := make(map[string]*ssa.Block, len(fn.Blocks))
	decls := map[string]bool{}
	for _, b := range fn.Blocks {
		blocks[b.Name] = b
		for _, bp := range b.Params {
			decls[blockParamC(b.Name, bp.Name)] = true
		}
		for _, in := range b.Instrs {
			if in.Dest != "" {
				decls[sanitizeC(in.Dest)] = true
			}
			if in.Cell != "" {
				decls[sanitizeC(in.Cell)] = true
			}
		}
	}
	names := make([]string, 0, len(decls))
	for d := range decls {
		names = append(names, d)
	}
	sort.Strings(names)
	for _, d := range names {
		fmt.Fprintf(&sb, "\tValue %s;\n", d)
	}

	emitInstr := func(in ssa.Instr, b *ssa.Block) {
		dest := ""
		if in.Dest != "" {
			dest = sanitizeC(in.Dest)
		}
		arg := func(i int) string { return operandC(in.Args[i]) }
		switch in.Op {
		case ssa.OpConst:
			fmt.Fprintf(&sb, "\t%s = %s;\n", dest, arg(0))
		case ssa.OpBinOp:
			fmt.Fprintf(&sb, "\t%s = binary_op(%s, %q, %s);\n", dest, arg(0), in.OpStr, arg(1))
		case ssa.OpUnOp:
			fn2 := "karkain_negate"
			if in.OpStr == "!" {
				fn2 = "karkain_not"
			}
			fmt.Fprintf(&sb, "\t%s = %s(%s);\n", dest, fn2, arg(0))
		case ssa.OpCall:
			args := make([]string, len(in.Args))
			for i := range in.Args {
				args[i] = arg(i)
			}
			fmt.Fprintf(&sb, "\t%s = %s(%s);\n", dest, sanitizeC(in.OpStr), strings.Join(args, ", "))
		case ssa.OpCallVoid:
			args := make([]string, len(in.Args))
			for i := range in.Args {
				args[i] = arg(i)
			}
			fmt.Fprintf(&sb, "\t%s(%s);\n", sanitizeC(in.OpStr), strings.Join(args, ", "))
		case ssa.OpBr:
			targets := []struct {
				name string
				args []ssa.Operand
			}{{in.ThenTarget, nil}, {in.ElseTarget, nil}}
			if len(in.BranchArgs) == 2 {
				targets[0].args = in.BranchArgs[0]
				targets[1].args = in.BranchArgs[1]
			}
			for i, t := range targets {
				tb := blocks[t.name]
				for j, bp := range tb.Params {
					v := "mk_nil()"
					if j < len(t.args) {
						v = operandC(t.args[j])
					}
					fmt.Fprintf(&sb, "\t%s = %s;\n", blockParamC(tb.Name, bp.Name), v)
				}
				_ = i
			}
			fmt.Fprintf(&sb, "\tif (is_truthy(%s)) goto L_%s;\n\tgoto L_%s;\n", arg(0), sanitizeC(in.ThenTarget), sanitizeC(in.ElseTarget))
		case ssa.OpJmp:
			tb := blocks[in.JmpTarget]
			if len(in.BranchArgs) == 1 {
				for j, bp := range tb.Params {
					v := "mk_nil()"
					if j < len(in.BranchArgs[0]) {
						v = operandC(in.BranchArgs[0][j])
					}
					fmt.Fprintf(&sb, "\t%s = %s;\n", blockParamC(tb.Name, bp.Name), v)
				}
			}
			fmt.Fprintf(&sb, "\tgoto L_%s;\n", sanitizeC(in.JmpTarget))
		case ssa.OpRet:
			if isMain {
				sb.WriteString("\treturn 0;\n")
			} else if len(in.Args) == 0 {
				sb.WriteString("\treturn mk_nil();\n")
			} else {
				fmt.Fprintf(&sb, "\treturn %s;\n", arg(0))
			}
		case ssa.OpIndexGet:
			fmt.Fprintf(&sb, "\t%s = array_or_string_get(%s, %s);\n", dest, arg(0), arg(1))
		case ssa.OpIndexSet:
			fmt.Fprintf(&sb, "\tindex_set(&%s, %s, %s);\n", arg(0), arg(1), arg(2))
		case ssa.OpPrint:
			fmt.Fprintf(&sb, "\tprint_value(%s);\n", arg(0))
		case ssa.OpLoad:
			fmt.Fprintf(&sb, "\t%s = %s;\n", dest, sanitizeC(in.Cell))
		case ssa.OpStore:
			fmt.Fprintf(&sb, "\t%s = %s;\n", sanitizeC(in.Cell), arg(0))
		case ssa.OpRawC:
			if dest != "" {
				fmt.Fprintf(&sb, "\t%s = (%s);\n", dest, in.RawC)
			} else {
				fmt.Fprintf(&sb, "\t%s\n", in.RawC)
			}
		}
	}

	for bi, b := range fn.Blocks {
		if bi > 0 {
			fmt.Fprintf(&sb, "L_%s:\n", sanitizeC(b.Name))
		}
		for _, in := range b.Instrs {
			emitInstr(in, b)
		}
	}
	sb.WriteString("}\n")
	return sb.String()
}

// emitFunctionViaIR lowers one function to SSA, optimizes, verifies, emits C.
// Any failure returns ok=false and the caller falls back to legacy emission.
func (g *Generator) emitFunctionViaIR(prog *parser.Program, fn *parser.FuncDecl) (out string, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			out, ok = "", false
		}
	}()

	mod := &ssa.Module{}
	sfn := mod.NewFunction(fn.Name)
	for _, p := range fn.Params {
		sfn.Params = append(sfn.Params, ssa.BlockParam{Name: p, Ty: ssa.Value})
	}
	lf := newFnLowerer(g, prog, sfn)
	lf.lowerStmts(fn.Body)
	if !lf.cur.Terminated() {
		lf.cur.Emit(ssa.Instr{Op: ssa.OpRet})
	}
	mod.FoldConstants()
	mod.EliminateDeadCode()
	if err := mod.Verify(); err != nil {
		return "", false
	}
	return g.emitSSAFunction(sfn, fn.Params), true
}
