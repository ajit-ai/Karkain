package ssa

import (
	"fmt"
	"strings"
)

// Type — Phase 53: IR register types. Value is the dynamic Karkain value.
type Type uint8

const (
	Void    Type = iota
	I            // integer (i64 semantics)
	F            // float64
	Bool
	Str
	Value // dynamic boxed Karkain value
	Ptr   // raw pointer (alloc/free, references &T)
)

func (t Type) String() string {
	switch t {
	case Void:
		return "void"
	case I:
		return "i64"
	case F:
		return "f64"
	case Bool:
		return "bool"
	case Str:
		return "str"
	case Value:
		return "value"
	case Ptr:
		return "ptr"
	}
	return "?"
}

// Operand — either a register reference or a constant.
type Operand struct {
	Reg   string // empty means constant
	IsCst bool
	// Constant payload (valid when IsCst):
	CstKind Type   // I, F, Bool, Str
	IntVal  int64
	FltVal  float64
	BoolVal bool
	StrVal  string
}

func Reg(name string) Operand { return Operand{Reg: name} }

func ConstInt(v int64) Operand  { return Operand{IsCst: true, CstKind: I, IntVal: v} }
func ConstFlt(v float64) Operand { return Operand{IsCst: true, CstKind: F, FltVal: v} }
func ConstBool(v bool) Operand  { return Operand{IsCst: true, CstKind: Bool, BoolVal: v} }
func ConstStr(v string) Operand { return Operand{IsCst: true, CstKind: Str, StrVal: v} }

func (o Operand) String() string {
	if !o.IsCst {
		return "%" + o.Reg
	}
	switch o.CstKind {
	case I:
		return fmt.Sprintf("%d", o.IntVal)
	case F:
		return fmt.Sprintf("%g", o.FltVal)
	case Bool:
		if o.BoolVal {
			return "true"
		}
		return "false"
	case Str:
		return fmt.Sprintf("%q", o.StrVal)
	}
	return "?"
}

// OpKind — Phase 53: IR instruction opcodes.
type OpKind uint8

const (
	OpConst OpKind = iota
	OpBinOp          // %d = binop %a OP %b
	OpUnOp           // %d = unop OP %a ("-" or "!")
	OpCall           // %d = call name(%a...)
	OpCallVoid       // call name(%a...)
	OpBr             // br %cond ? ^then : ^else   (block args via BranchArgs)
	OpJmp            // jmp ^target                (block args via BranchArgs)
	OpRet            // ret %v (or ret void when Dest=="" and no Args)
	OpIndexGet       // %d = index_get %arr, %i
	OpIndexSet       // index_set %arr, %i, %v
	OpPrint          // print %v
	OpRawC           // ESCAPE HATCH: opaque pre-rendered C expression (dest) or statement (no dest)
)

// Instr — one SSA instruction inside a block.
type Instr struct {
	Op     OpKind
	Dest   string // register written, "" if none
	Ty     Type   // type of Dest (or result)
	Args   []Operand
	OpStr  string   // binop operator / call target name
	RawC   string   // OpRawC payload
	// Branch payloads:
	ThenTarget, ElseTarget string      // OpBr
	JmpTarget              string      // OpJmp
	BranchArgs             [][]Operand // parallel to targets: args passed to block params
}

// BlockParam — SSA block parameter (replaces phi nodes).
type BlockParam struct {
	Name string
	Ty   Type
}

// Block — a basic block: params, instructions, terminator last.
type Block struct {
	Name   string
	Params []BlockParam
	Instrs []Instr // last must be a terminator (br/jmp/ret)
}

// Function — SSA function over basic blocks.
type Function struct {
	Name       string
	Params     []BlockParam
	ReturnType Type
	Blocks     []*Block
	Entry      *Block
	regCount   int
	blockCount int
}

func NewFunction(name string) *Function {
	f := &Function{Name: name}
	f.Entry = f.NewBlock("entry")
	return f
}

func (f *Function) NewBlock(hint string) *Block {
	name := fmt.Sprintf("%s%d", hint, f.blockCount)
	if f.blockCount == 0 && hint == "entry" {
		name = "entry"
	}
	f.blockCount++
	b := &Block{Name: name}
	f.Blocks = append(f.Blocks, b)
	return b
}

// NewReg allocates a fresh SSA register name.
func (f *Function) NewReg(hint string) string {
	name := fmt.Sprintf("%s_%d", hint, f.regCount)
	f.regCount++
	return name
}

func (b *Block) Terminated() bool {
	if len(b.Instrs) == 0 {
		return false
	}
	switch b.Instrs[len(b.Instrs)-1].Op {
	case OpBr, OpJmp, OpRet:
		return true
	}
	return false
}

func (b *Block) Emit(i Instr) { b.Instrs = append(b.Instrs, i) }

// Module — collection of functions.
type Module struct {
	Functions []*Function
}

func (m *Module) NewFunction(name string) *Function {
	f := NewFunction(name)
	m.Functions = append(m.Functions, f)
	return f
}

// BinOp validation table.
var validBinOps = map[string]bool{
	"+": true, "-": true, "*": true, "/": true, "%": true,
	"==": true, "!=": true, "<": true, ">": true, "<=": true, ">=": true,
	"&&": true, "||": true,
}

// Format renders the module in readable text form.
func (m *Module) Format() string {
	var sb strings.Builder
	for _, fn := range m.Functions {
		fmt.Fprintf(&sb, "func @%s(", fn.Name)
		for i, p := range fn.Params {
			if i > 0 {
				sb.WriteString(", ")
			}
			fmt.Fprintf(&sb, "%%%s: %s", p.Name, p.Ty)
		}
		fmt.Fprintf(&sb, ") -> %s {\n", fn.ReturnType)
		for _, b := range fn.Blocks {
			fmt.Fprintf(&sb, "%s:", b.Name)
			for _, p := range b.Params {
				fmt.Fprintf(&sb, " %%%s: %s", p.Name, p.Ty)
			}
			sb.WriteString("\n")
			for _, in := range b.Instrs {
				sb.WriteString("\t")
				sb.WriteString(formatInstr(in))
				sb.WriteString("\n")
			}
		}
		sb.WriteString("}\n")
	}
	return sb.String()
}

func formatInstr(in Instr) string {
	switch in.Op {
	case OpConst:
		cst := Operand{IsCst: true, CstKind: in.Ty, IntVal: constInt(in), FltVal: constFlt(in), BoolVal: constBool(in), StrVal: in.RawC}
		return fmt.Sprintf("%%%-8s = const %s : %s", in.Dest, cst.String(), in.Ty)
	case OpBinOp:
		return fmt.Sprintf("%%%-8s = binop %s %s, %s : %s", in.Dest, in.OpStr, in.Args[0].String(), in.Args[1].String(), in.Ty)
	case OpUnOp:
		return fmt.Sprintf("%%%-8s = unop %s%s : %s", in.Dest, in.OpStr, in.Args[0].String(), in.Ty)
	case OpCall:
		args := formatOperands(in.Args)
		return fmt.Sprintf("%%%-8s = call %s(%s) : %s", in.Dest, in.OpStr, args, in.Ty)
	case OpCallVoid:
		return fmt.Sprintf("call %s(%s)", in.OpStr, formatOperands(in.Args))
	case OpBr:
		targs := ""
		if len(in.BranchArgs) > 0 {
			targs = ", " + formatOperandGroups(in.BranchArgs)
		}
		return fmt.Sprintf("br %s, ^%s, ^%s%s", in.Args[0].String(), in.ThenTarget, in.ElseTarget, targs)
	case OpJmp:
		targs := ""
		if len(in.BranchArgs) > 0 {
			targs = ", " + formatOperandGroups(in.BranchArgs[:1])
		}
		return fmt.Sprintf("jmp ^%s%s", in.JmpTarget, targs)
	case OpRet:
		if len(in.Args) == 0 {
			return "ret void"
		}
		return "ret " + in.Args[0].String()
	case OpIndexGet:
		return fmt.Sprintf("%%%-8s = index_get %s[%s] : %s", in.Dest, in.Args[0].String(), in.Args[1].String(), in.Ty)
	case OpIndexSet:
		return fmt.Sprintf("index_set %s[%s] = %s", in.Args[0].String(), in.Args[1].String(), in.Args[2].String())
	case OpPrint:
		return "print " + in.Args[0].String()
	case OpRawC:
		if in.Dest != "" {
			return fmt.Sprintf("%%%-8s = rawc {%s}", in.Dest, in.RawC)
		}
		return fmt.Sprintf("rawc_stmt {%s}", strings.ReplaceAll(in.RawC, "\n", " "))
	}
	return "?instr"
}

func formatOperands(args []Operand) string {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = a.String()
	}
	return strings.Join(parts, ", ")
}

func formatOperandGroups(groups [][]Operand) string {
	parts := make([]string, len(groups))
	for i, g := range groups {
		parts[i] = "[" + formatOperands(g) + "]"
	}
	return strings.Join(parts, ", ")
}

func constInt(in Instr) int64 {
	if len(in.Args) > 0 {
		return in.Args[0].IntVal
	}
	return 0
}
func constFlt(in Instr) float64 {
	if len(in.Args) > 0 {
		return in.Args[0].FltVal
	}
	return 0
}
func constBool(in Instr) bool {
	if len(in.Args) > 0 {
		return in.Args[0].BoolVal
	}
	return false
}

