package native

import (
	"fmt"
	"strconv"

	"karkain/pkg/parser"
)

// Straight-line program lowering, Phase 145.
//
// Supported surface (gate-pinned, everything else is a deterministic
// K145 error): top-level `func` declarations (≤6 params; int params and
// int returns only), `let` bindings of ints and strings, int/string
// literals, `+ - *` on ints, unary minus, calls, `print(expr)` of an int
// or string, and `return` (main's value becomes the process exit code;
// a missing trailing return yields 0). Slot kinds are tracked so a
// string never flows into int code: that is a K145 error, never a
// miscompile. Types are otherwise trusted from the source — callers
// validate first; the gate fixtures are fixed programs.
//
// Values live in stack slots ([rsp+off], 8 bytes each): params, then one
// slot per let (two for strings: ptr+len), then a fixed 96-byte argument
// spill area (6 args x 16 bytes). Eval scratch is rax/rcx/rdx (+ the
// stack) ONLY, so argument spill slots and not-yet-loaded argument
// registers are never disturbed by nested evaluation. Helpers take fixed
// registers and may clobber anything.

// sys_write / sys_exit numbers (Linux x86-64).
const (
	sysWrite = 1
	sysExit  = 60
)

// argRegs is the user-function calling convention (System V order).
var argRegs = []Reg{RDI, RSI, RDX, RCX, R8, R9}

// argSpillBytes reserves caller-side spill space for 6 args x (ptr+len).
const argSpillBytes = 96

// addrPatch records an imm64 placeholder resolving to a .rodata address.
type addrPatch struct {
	pos   int
	roOff uint64
}

// Builder lowers one program to .text+.rodata.
type Builder struct {
	e       *Emitter
	rodata  []byte
	strs    map[string]uint64
	patches []addrPatch
	funcs   map[string]bool
	slots   map[string]int
	kinds   map[string]bool
	frame   int
}

// CompileProgram lowers prog to a linked static executable image. Only
// main is the entry; every other top-level func is emitted as a callable
// unit. Anything outside the v1 surface is an error naming the construct
// (error code K145); nothing is silently dropped.
func CompileProgram(prog *parser.Program) ([]byte, error) {
	b := newBuilder()
	for _, stmt := range prog.Statements {
		if fd, ok := stmt.(*parser.FuncDecl); ok {
			if b.funcs[fd.Name] {
				return nil, fmt.Errorf("error[K145]: duplicate function '%s'", fd.Name)
			}
			b.funcs[fd.Name] = true
		}
	}
	if !b.funcs["main"] {
		return nil, fmt.Errorf("error[K145]: no 'main' function for native target")
	}
	b.emitHelpers()
	for _, stmt := range prog.Statements {
		if fd, ok := stmt.(*parser.FuncDecl); ok {
			if err := b.emitFunc(fd); err != nil {
				return nil, err
			}
		}
	}
	b.e.Mark("_start")
	b.e.Call("karkain_main")
	b.e.MovRegReg(RDI, RAX)
	b.e.MovRegImm32(RAX, sysExit)
	b.e.Syscall()
	text := b.e.Bytes()
	const textOff = elfHeaderSize + progHeaderSize
	roBase := uint64(BaseAddr + textOff + len(text))
	out := append([]byte{}, text...)
	for _, p := range b.patches {
		v := roBase + p.roOff
		out[p.pos] = byte(v)
		out[p.pos+1] = byte(v >> 8)
		out[p.pos+2] = byte(v >> 16)
		out[p.pos+3] = byte(v >> 24)
		out[p.pos+4] = byte(v >> 32)
		out[p.pos+5] = byte(v >> 40)
		out[p.pos+6] = byte(v >> 48)
		out[p.pos+7] = byte(v >> 56)
	}
	entry := textOff + b.e.labels["_start"]
	return Link(out, b.rodata, textOff, entry)
}

func newBuilder() *Builder {
	return &Builder{e: NewEmitter(), strs: map[string]uint64{}, funcs: map[string]bool{}}
}

func (b *Builder) internRodata(s string) uint64 {
	if off, ok := b.strs[s]; ok {
		return off
	}
	off := uint64(len(b.rodata))
	b.rodata = append(b.rodata, s...)
	b.strs[s] = off
	return off
}

func (b *Builder) rodataRef(r Reg, s string) {
	b.patches = append(b.patches, addrPatch{pos: b.e.imm64Patch(r), roOff: b.internRodata(s)})
}

func (b *Builder) emitHelpers() {
	b.e.Mark("print_int")
	b.e.SubRsp(64)
	b.e.MovRegReg(RAX, RDI)
	b.e.MovRegReg(RSI, RSP)
	b.e.AddRegImm32(RSI, 64)
	b.e.TestRegReg(RAX, RAX)
	b.e.Jns("print_int_pos")
	b.e.NegReg(RAX)
	b.e.PushReg(RAX)
	b.e.PushReg(RSI)
	b.e.MovRegImm32(RDX, 1)
	b.e.MovRegImm32(RAX, sysWrite)
	b.e.MovRegImm32(RDI, 1)
	b.rodataRef(RSI, "-")
	b.e.Syscall()
	b.e.PopReg(RSI)
	b.e.PopReg(RAX)
	b.e.Mark("print_int_pos")
	b.e.MovRegImm32(RCX, 10)
	b.e.Mark("print_int_loop")
	b.e.Cqo()
	b.e.DivReg(RCX)
	b.e.AddRegImm32(RDX, '0')
	b.e.DecReg(RSI)
	b.e.StoreMem8(RSI, RDX)
	b.e.TestRegReg(RAX, RAX)
	b.e.Jnz("print_int_loop")
	b.e.MovRegReg(RBX, RSP)
	b.e.AddRegImm32(RBX, 64)
	b.e.MovRegReg(RDX, RBX)
	b.e.SubRegReg(RDX, RSI)
	b.e.MovRegImm32(RDI, 1)
	b.e.MovRegImm32(RAX, sysWrite)
	b.e.Syscall()
	b.e.AddRsp(64)
	b.e.Ret()

	b.e.Mark("print_str")
	b.e.MovRegReg(RDX, RSI)
	b.e.MovRegReg(RSI, RDI)
	b.e.MovRegImm32(RDI, 1)
	b.e.MovRegImm32(RAX, sysWrite)
	b.e.Syscall()
	b.e.Ret()
}

func (b *Builder) layout(fd *parser.FuncDecl) error {
	b.slots = map[string]int{}
	b.kinds = map[string]bool{}
	next := 0
	for _, p := range fd.Params {
		b.slots[p] = next
		b.kinds[p] = false
		next += 8
	}
	for _, s := range fd.Body {
		n, ok := s.(*parser.VarDeclStmt)
		if !ok {
			continue
		}
		off := next
		next += 8
		isStr, err := b.isStringExpr(n.Value)
		if err != nil {
			return err
		}
		if isStr {
			next += 8
		}
		b.slots[n.Name] = off
		b.kinds[n.Name] = isStr
	}
	b.frame = next + argSpillBytes
	return nil
}

func (b *Builder) emitFunc(fd *parser.FuncDecl) error {
	if len(fd.Params) > len(argRegs) {
		return fmt.Errorf("error[K145]: function '%s' has %d params (max %d)", fd.Name, len(fd.Params), len(argRegs))
	}
	name := "fn_" + fd.Name
	if fd.Name == "main" {
		name = "karkain_main"
	}
	b.e.Mark(name)
	if err := b.layout(fd); err != nil {
		return err
	}
	if b.frame > 0 {
		b.e.SubRsp(b.frame)
	}
	for i, p := range fd.Params {
		b.e.StoreStack(argRegs[i], b.slots[p])
	}
	returned := false
	for _, s := range fd.Body {
		switch n := s.(type) {
		case *parser.VarDeclStmt:
			if err := b.emitLet(n); err != nil {
				return err
			}
		case *parser.PrintStmt:
			if err := b.emitPrint(n); err != nil {
				return err
			}
		case *parser.ReturnStmt:
			if err := b.emitReturn(n); err != nil {
				return err
			}
			b.e.Jmp(name + "$ret")
			returned = true
		case *parser.ExprStmt:
			if err := b.emitExpr(n.Expression); err != nil {
				return err
			}
		case *parser.IfStmt, *parser.WhileStmt, *parser.ForStmt, *parser.ForInStmt:
			return fmt.Errorf("error[K145]: control flow is not supported (straight-line code and calls only)")
		default:
			return fmt.Errorf("error[K145]: unsupported statement %T in '%s'", s, fd.Name)
		}
	}
	if !returned {
		b.e.XorRegReg(RAX)
	}
	b.e.Mark(name + "$ret")
	if b.frame > 0 {
		b.e.AddRsp(b.frame)
	}
	b.e.Ret()
	return nil
}

func (b *Builder) argTemp(i int) int {
	return b.frame - argSpillBytes + i*16
}

func (b *Builder) emitLet(n *parser.VarDeclStmt) error {
	isStr, err := b.isStringExpr(n.Value)
	if err != nil {
		return err
	}
	off := b.slots[n.Name]
	if isStr {
		if err := b.emitStr(n.Value); err != nil {
			return err
		}
		b.e.StoreStack(RDI, off)
		b.e.StoreStack(RSI, off+8)
		return nil
	}
	if err := b.emitExpr(n.Value); err != nil {
		return err
	}
	b.e.StoreStack(RAX, off)
	return nil
}

func (b *Builder) emitPrint(n *parser.PrintStmt) error {
	isStr, err := b.isStringExpr(n.Value)
	if err != nil {
		return err
	}
	if isStr {
		if err := b.emitStr(n.Value); err != nil {
			return err
		}
		b.e.Call("print_str")
		return nil
	}
	if err := b.emitExpr(n.Value); err != nil {
		return err
	}
	b.e.MovRegReg(RDI, RAX)
	b.e.Call("print_int")
	return nil
}

func (b *Builder) emitReturn(n *parser.ReturnStmt) error {
	if n.Value == nil {
		b.e.XorRegReg(RAX)
		return nil
	}
	if isStr, err := b.isStringExpr(n.Value); err != nil {
		return err
	} else if isStr {
		return fmt.Errorf("error[K145]: string return values are not supported (function must return int)")
	}
	return b.emitExpr(n.Value)
}

func (b *Builder) isStringExpr(n parser.Node) (bool, error) {
	switch x := n.(type) {
	case *parser.StringLiteral:
		return true, nil
	case *parser.IntLiteral:
		return false, nil
	case *parser.Float64Literal:
		return false, fmt.Errorf("error[K145]: floating-point numbers are not supported (ints and strings only)")
	case *parser.Identifier:
		isStr, ok := b.kinds[x.Name]
		if !ok {
			return false, fmt.Errorf("error[K145]: undefined identifier '%s'", x.Name)
		}
		return isStr, nil
	case *parser.BinaryExpr:
		return false, nil
	case *parser.UnaryExpr:
		return false, nil
	case *parser.CallExpr:
		return false, nil
	default:
		return false, fmt.Errorf("error[K145]: unsupported expression %T", n)
	}
}

func (b *Builder) emitExpr(n parser.Node) error {
	switch x := n.(type) {
	case *parser.IntLiteral:
		b.e.MovRegImm64(RAX, uint64(parseIntLit(x.Value)))
		return nil
	case *parser.Identifier:
		if b.kinds[x.Name] {
			return fmt.Errorf("error[K145]: string '%s' in int position", x.Name)
		}
		off, ok := b.slots[x.Name]
		if !ok {
			return fmt.Errorf("error[K145]: undefined identifier '%s'", x.Name)
		}
		b.e.LoadStack(RAX, off)
		return nil
	case *parser.BinaryExpr:
		return b.emitBinary(x)
	case *parser.UnaryExpr:
		if x.Operator != "-" {
			return fmt.Errorf("error[K145]: unsupported unary operator '%s'", x.Operator)
		}
		if err := b.emitExpr(x.Operand); err != nil {
			return err
		}
		b.e.NegReg(RAX)
		return nil
	case *parser.CallExpr:
		return b.emitCallValue(x)
	default:
		return fmt.Errorf("error[K145]: unsupported expression %T", n)
	}
}

func (b *Builder) emitStr(n parser.Node) error {
	switch x := n.(type) {
	case *parser.StringLiteral:
		off := b.internRodata(x.Value)
		b.patches = append(b.patches, addrPatch{pos: b.e.imm64Patch(RDI), roOff: off})
		b.e.MovRegImm32(RSI, uint32(len(x.Value)))
		return nil
	case *parser.Identifier:
		if !b.kinds[x.Name] {
			return fmt.Errorf("error[K145]: int '%s' in string position", x.Name)
		}
		off, ok := b.slots[x.Name]
		if !ok {
			return fmt.Errorf("error[K145]: undefined identifier '%s'", x.Name)
		}
		b.e.LoadStack(RDI, off)
		b.e.LoadStack(RSI, off+8)
		return nil
	default:
		return fmt.Errorf("error[K145]: unsupported string expression %T (literals and variables only)", n)
	}
}

func (b *Builder) emitBinary(x *parser.BinaryExpr) error {
	if x.Operator != "+" && x.Operator != "-" && x.Operator != "*" {
		return fmt.Errorf("error[K145]: unsupported operator '%s' (want +, - or *)", x.Operator)
	}
	if err := b.emitExpr(x.Left); err != nil {
		return err
	}
	b.e.PushReg(RAX)
	if err := b.emitExpr(x.Right); err != nil {
		return err
	}
	b.e.MovRegReg(RCX, RAX)
	b.e.PopReg(RAX)
	switch x.Operator {
	case "+":
		b.e.AddRegReg(RAX, RCX)
	case "-":
		b.e.SubRegReg(RAX, RCX)
	case "*":
		b.e.MulRegReg(RAX, RCX)
	}
	return nil
}

func (b *Builder) emitCallValue(x *parser.CallExpr) error {
	if x.Module != "" || x.IsCFunc {
		return fmt.Errorf("error[K145]: module-qualified and C-interop calls are not supported (call to '%s')", x.Function)
	}
	if !b.funcs[x.Function] {
		return fmt.Errorf("error[K145]: undefined function '%s'", x.Function)
	}
	if len(x.Args) > len(argRegs) {
		return fmt.Errorf("error[K145]: call to '%s' has %d args (max %d)", x.Function, len(x.Args), len(argRegs))
	}
	for i, a := range x.Args {
		if isStr, err := b.isStringExpr(a); err != nil {
			return err
		} else if isStr {
			return fmt.Errorf("error[K145]: string arguments are not supported (call to '%s')", x.Function)
		}
		if err := b.emitExpr(a); err != nil {
			return err
		}
		b.e.StoreStack(RAX, b.argTemp(i))
	}
	for i := range x.Args {
		b.e.LoadStack(argRegs[i], b.argTemp(i))
	}
	if x.Function == "main" {
		b.e.Call("karkain_main")
	} else {
		b.e.Call("fn_" + x.Function)
	}
	return nil
}

func parseIntLit(s string) int64 {
	v, err := strconv.ParseInt(s, 0, 64)
	if err != nil {
		return 0
	}
	return v
}
