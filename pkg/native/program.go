package native

import (
	"fmt"
	"math"
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
// slot per let (two for strings: ptr+len), then the binary-operand scratch
// stack, then the caller-frame extras area, then a fixed 96-byte argument
// spill area (6 args x 16 bytes). Eval scratch is rax/rcx/rdx (+ the
// stack) ONLY, so argument spill slots and not-yet-loaded argument
// registers are never disturbed by nested evaluation. Helpers take fixed
// registers and may clobber anything.
//
// Native calling convention (Phase 148): argument 8-byte units pack in
// order — ints take one unit, strings two (ptr+len). Units 0-5 travel in
// RDI,RSI,RDX,RCX,R8,R9; further units travel in the caller-frame extras
// array whose address reaches the callee in R10 (set with lea just before
// the call; the callee homes stack units at entry, before R10 can die).
// String returns leave (RAX=ptr, RDX=len). rsp never moves during argument
// evaluation: register units stage through the per-arg spill, extras
// through the extras area, and only then do registers reload.

// OS names accepted by CompileProgramForOS (Phase 149). Emission is
// pure Go on every host; only the container, the syscall numbers (or
// kernel32 boundary on Windows) and the _start tail vary per OS.
const (
	OSLinux   = "linux"
	OSWindows = "windows"
	OSMacOS   = "macos"
)

// sysWrite returns the write syscall number for the target OS
// (Linux/macOS raw-syscall path; Windows uses kernel32 instead).
func (b *Builder) sysWrite() uint32 {
	if b.goos == OSMacOS {
		return 0x2000004
	}
	return 1
}

// sysExit returns the exit syscall number for the target OS.
func (b *Builder) sysExit() uint32 {
	if b.goos == OSMacOS {
		return 0x2000001
	}
	return 60
}

// argRegs is the user-function calling convention (System V order).
var argRegs = []Reg{RDI, RSI, RDX, RCX, R8, R9}

// argSpillBytes reserves caller-side spill space for 6 args x (ptr+len).
const argSpillBytes = 96

// maxBinDepth bounds nested binary-expression evaluation. Binary operands
// stage through depth-indexed scratch slots (never push: pushing moves rsp
// and silently shifts every frame-relative slot address — the Phase-147
// `add(20,22) = 40` defect, where the right operand re-read slot a).
// Nesting deeper than this is a loud K145 error, never a silent miscompile.
const maxBinDepth = 64

// addrPatch records an imm64 placeholder resolving to a .rodata address.
type addrPatch struct {
	pos   int
	roOff uint64
}

// iatPatch records a movabs placeholder resolving to a PE IAT slot
// address (Phase 149): kernel32 calls go through the import table.
type iatPatch struct {
	pos   int
	index int
}

// absPatch records a mov-[imm64] placeholder resolving to a PE IAT slot
// address (Phase 149): the loader-independent bootstrap publishes the
// PEB-resolved kernel32 addresses straight into the slots.
type absPatch struct {
	pos   int
	index int
}

// Value kinds (Phase 150A): the native Value model. KindInt is zero so
// every pre-150 default (untyped parameters, missing returns, unknown
// slots) keeps meaning int without touching its logic. Strings and arrays
// occupy two frame slots (ptr+len); ints and floats occupy one slot
// (floats carry f64 bits, converted to XMM only inside arithmetic).
const (
	KindInt = iota
	KindString
	KindFloat
	KindArray
)

// kindUnits returns the 8-byte frame/call slots a value kind occupies.
func kindUnits(k int) int {
	if k == KindString || k == KindArray {
		return 2
	}
	return 1
}

// kindName renders a kind for K145 diagnostics.
func kindName(k int) string {
	switch k {
	case KindString:
		return "string"
	case KindFloat:
		return "float"
	case KindArray:
		return "array"
	default:
		return "int"
	}
}

// Builder lowers one program to .text+.rodata.
type Builder struct {
	e       *Emitter
	goos    string // Phase 149: OSLinux, OSWindows or OSMacOS
	rodata  []byte
	strs    map[string]uint64
	patches []addrPatch
	ipatches []iatPatch // Phase 149: PE import-slot placeholders
	apatches []absPatch // Phase 149: bootstrap IAT publishes
	funcs   map[string]bool
	ftab    map[string]*parser.FuncDecl // Phase 148: name -> declaration (arity, param kinds)
	retKind map[string]int              // Phase 150A: name -> value kind (was bool string-ness)
	slots   map[string]int
	kinds   map[string]int // Phase 150A: name -> value kind (was bool string-ness)
	frame   int
	binTemp int // Phase 147: base offset of the binary-operand scratch stack
	extrasBase int // Phase 148: base offset of the caller-frame extras area
	scanErr error
	uid     int // Phase 148: monotonically increasing label discriminator
	loops   []loopTgt
}

// loopTgt records a loop's jump targets for break/continue (Phase 148).
type loopTgt struct {
	brk  string // break jumps here (past the loop)
	cont string // continue jumps here (condition check / post statement)
}

// fresh returns a program-unique label (Emitter.Mark panics on duplicates,
// so control-flow labels can never be a bare fixed string).
func (b *Builder) fresh(prefix string) string {
	b.uid++
	return fmt.Sprintf("%s$%d", prefix, b.uid)
}

// CompileProgram lowers prog to a linked static Linux executable image
// (the Phase-145 default; CompileProgramForOS selects the OS).
func CompileProgram(prog *parser.Program) ([]byte, error) {
	return CompileProgramForOS(prog, OSLinux)
}

// CompileProgramForOS lowers prog to a linked static executable image
// for the named OS ("linux", "windows", "macos"): one encoder and one
// lowering feed per-OS containers, syscall numbers (or the kernel32
// boundary on Windows) and entry tails. Anything else is a K145 error.
func CompileProgramForOS(prog *parser.Program, osName string) ([]byte, error) {
	b := newBuilder()
	b.goos = osName
	switch osName {
	case OSLinux, OSWindows, OSMacOS:
	default:
		return nil, fmt.Errorf("error[K145]: unsupported native OS '%s' (want linux, windows or macos)", osName)
	}
	for _, stmt := range prog.Statements {
		if fd, ok := stmt.(*parser.FuncDecl); ok {
			if b.funcs[fd.Name] {
				return nil, fmt.Errorf("error[K145]: duplicate function '%s'", fd.Name)
			}
			b.funcs[fd.Name] = true
			b.ftab[fd.Name] = fd
		}
	}
	if !b.funcs["main"] {
		return nil, fmt.Errorf("error[K145]: no 'main' function for native target")
	}
	// Phase 148: string-return inference for the whole unit before any
	// emission (callers need callee kinds; see retKindOf).
	for name := range b.ftab {
		if _, err := b.retKindOf(name); err != nil {
			return nil, err
		}
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
	if b.goos == OSWindows {
		// Alignment first: everything downstream (main's frame math,
		// every kernel32 call) assumes rsp%16==0 here. Then the
		// loader-independent bootstrap fills the IAT before main runs.
		b.e.AndRspNeg16()
		b.emitWinBootstrap()
	}
	b.e.Call("karkain_main")
	if b.goos == OSWindows {
		b.emitWindowsExit()
	} else {
		b.e.MovRegReg(RDI, RAX)
		b.e.MovRegImm32(RAX, b.sysExit())
		b.e.Syscall()
	}
	text := b.e.Bytes()
	textOff := elfHeaderSize + progHeaderSize
	base := uint64(BaseAddr)
	switch b.goos {
	case OSMacOS:
		textOff = machoTextOff
		base = MachoBase
	case OSWindows:
		textOff = peTextOff
		base = PEBaseAddr
	}
	entry := textOff + b.e.labels["_start"]
	// Windows resolves both patch kinds inside LinkPE (rodata against
	// the .text base, IAT slots against .idata); ELF/Mach-O share the
	// rodata-only loop here.
	if b.goos == OSWindows {
		return LinkPE(b, text, b.rodata, textOff, entry)
	}
	roBase := base + uint64(textOff) + uint64(len(text))
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
	switch b.goos {
	case OSMacOS:
		return LinkMachO(out, b.rodata, textOff, entry)
	default:
		return Link(out, b.rodata, textOff, entry)
	}
}
func newBuilder() *Builder {
	return &Builder{e: NewEmitter(), strs: map[string]uint64{}, funcs: map[string]bool{}, ftab: map[string]*parser.FuncDecl{}, retKind: map[string]int{}}
}

// paramKind reports the value kind of parameter i of fd (Phase-46
// annotations `name string` / `name float`; untyped parameters are ints.
// Array parameters have no annotation syntax in 150A: a function whose
// body indexes a parameter is rejected loudly at layout (150B work).
func paramKind(fd *parser.FuncDecl, i int) int {
	if i < len(fd.ParamTypes) {
		switch fd.ParamTypes[i] {
		case "string":
			return KindString
		case "float":
			return KindFloat
		}
	}
	return KindInt
}

// retKindOf infers the value kind a function returns: every `return` in
// its body (descending into control flow) must agree; no returns means
// int (the missing-return-yields-0 rule). Cyclic call graphs that never
// ground out are a loud K145.
func (b *Builder) retKindOf(name string) (int, error) {
	if k, done := b.retKind[name]; done {
		return k, nil
	}
	return b.retKindVisit(name, map[string]bool{})
}

func (b *Builder) retKindVisit(name string, visiting map[string]bool) (int, error) {
	if k, done := b.retKind[name]; done {
		return k, nil
	}
	if visiting[name] {
		return KindInt, fmt.Errorf("error[K145]: cannot determine return kind of recursive function '%s'", name)
	}
	fd, ok := b.ftab[name]
	if !ok {
		return KindInt, fmt.Errorf("error[K145]: undefined function '%s'", name)
	}
	visiting[name] = true
	// Local varDecl map for resolving returned identifiers, seeded with
	// the parameter kinds (untyped parameters are ints).
	vars := map[string]parser.Node{}
	pstr := map[string]int{}
	for i, p := range fd.Params {
		pstr[p] = paramKind(fd, i)
	}
	var collectVars func(stmts []parser.Node)
	collectVars = func(stmts []parser.Node) {
		for _, s := range stmts {
			switch n := s.(type) {
			case *parser.VarDeclStmt:
				vars[n.Name] = n.Value
			case *parser.IfStmt:
				collectVars(n.Consequence)
				collectVars(n.Alternative)
			case *parser.WhileStmt:
				collectVars(n.Body)
			case *parser.ForStmt:
				collectVars(n.Body)
			case *parser.ForInStmt:
				collectVars(n.Body)
			case *parser.BlockStmt:
				collectVars(n.Statements)
			}
		}
	}
	collectVars(fd.Body)
	seen := false
	kind := KindInt
	var walkReturns func(stmts []parser.Node) error
	walkReturns = func(stmts []parser.Node) error {
		for _, s := range stmts {
			switch n := s.(type) {
			case *parser.ReturnStmt:
				if n.Value == nil {
					continue
				}
				k, err := b.retKindOfExpr(n.Value, vars, pstr, visiting)
				if err != nil {
					return err
				}
				if !seen {
					seen, kind = true, k
				} else if kind != k {
					return fmt.Errorf("error[K145]: function '%s' mixes %s and %s returns", name, kindName(kind), kindName(k))
				}
			case *parser.IfStmt:
				if err := walkReturns(n.Consequence); err != nil {
					return err
				}
				if err := walkReturns(n.Alternative); err != nil {
					return err
				}
			case *parser.WhileStmt:
				if err := walkReturns(n.Body); err != nil {
					return err
				}
			case *parser.ForStmt:
				if err := walkReturns(n.Body); err != nil {
					return err
				}
			case *parser.ForInStmt:
				if err := walkReturns(n.Body); err != nil {
					return err
				}
			case *parser.BlockStmt:
				if err := walkReturns(n.Statements); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := walkReturns(fd.Body); err != nil {
		return KindInt, err
	}
	delete(visiting, name)
	b.retKind[name] = kind
	return kind, nil
}

// retKindOfExpr classifies a returned value expression.
func (b *Builder) retKindOfExpr(n parser.Node, vars map[string]parser.Node, pstr map[string]int, visiting map[string]bool) (int, error) {
	switch x := n.(type) {
	case *parser.StringLiteral:
		return KindString, nil
	case *parser.IntLiteral:
		return KindInt, nil
	case *parser.Float64Literal:
		return KindFloat, nil
	case *parser.Identifier:
		if k, isParam := pstr[x.Name]; isParam {
			return k, nil
		}
		v, ok := vars[x.Name]
		if !ok {
			return KindInt, fmt.Errorf("error[K145]: undefined identifier '%s'", x.Name)
		}
		if v == nil {
			return KindInt, nil
		}
		return b.retKindOfExpr(v, vars, pstr, visiting)
	case *parser.BinaryExpr, *parser.UnaryExpr:
		return KindInt, nil
	case *parser.CallExpr:
		return b.retKindVisit(x.Function, visiting)
	default:
		return KindInt, fmt.Errorf("error[K145]: unsupported return expression %T", n)
	}
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

// emitWrite emits the payload write for (RDI=fd, RSI=ptr, RDX=len):
// a raw syscall on Linux/macOS (numbers via sysWrite), the kernel32
// WriteFile sequence on Windows (Phase 149).
func (b *Builder) emitWrite() {
	if b.goos == OSWindows {
		b.emitWinWrite()
		return
	}
	b.e.MovRegImm32(RAX, b.sysWrite())
	b.e.Syscall()
	// NOTE: keep this body as raw emitter calls — the 148C replaceAll
	// that introduced emitWrite once rewrote this branch into infinite
	// self-recursion. Do not route through emitWrite here.
}

// emitWinWrite emits write(1, RSI, RDX) through kernel32 WriteFile.
// Win64 passes the first four args in RCX,RDX,R8,R9 with a 32-byte
// caller shadow; kernel calls preserve RDI/RSI/RBX (callee-saved) but
// may clobber everything else, so ptr/len ride the stack across the
// GetStdHandle call and registers reload after it.
func (b *Builder) emitWinWrite() {
	b.e.PushReg(RSI)
	b.e.PushReg(RDX)
	b.e.SubRsp(32)
	b.e.MovRegImm32(RCX, 0xFFFFFFF5) // STD_OUTPUT_HANDLE
	b.iatCall(IATGetStdHandle)
	b.e.AddRsp(32)
	b.e.PopReg(RDX)
	b.e.PopReg(RSI)
	b.e.MovRegReg(RCX, RAX)
	b.e.MovRegReg(R8, RDX)
	b.e.MovRegReg(RDX, RSI)
	// 48 = 32 shadow + 8 written-dword + 8 pad: Win64 requires
	// rsp%16==8 before the call (40 would flip it to 0 and fault).
	b.e.SubRsp(48)
	b.e.LeaRegStack(R9, 24)
	b.e.XorRegReg(RAX)
	b.e.StoreStack(RAX, 32)
	b.iatCall(IATWriteFile)
	b.e.AddRsp(48)
}

// emitWindowsExit emits the _start tail on Windows: ExitProcess(main's
// value). Win64 requires caller shadow even for a single argument
// (entry alignment itself is fixed by AndRspNeg16 before the main call).
func (b *Builder) emitWindowsExit() {
	b.e.MovRegReg(RCX, RAX)
	b.e.SubRsp(32)
	b.iatCall(IATExitProcess)
}

// emitWinBootstrap emits the loader-independent kernel32 bootstrap
// (Phase 149): the host loader maps the image and runs entry but does
// not snap the IAT on minimal images (proven: slots keep file content
// at runtime), so _start resolves ExitProcess/GetStdHandle/WriteFile
// itself via the PEB and publishes the addresses into the IAT slots
// (writable .idata) before main runs. Standard shellcode technique,
// fully deterministic: PEB (gs:[0x60]) -> Ldr -> module walk matching
// "kernel32.dll" (never positional) -> export table walk per name.
// Clobbers RAX,RCX,RDX,RSI,RDI,RBP,R8-R11 (nothing is live pre-main);
// RSP is never moved. A missing module or export hits Int3: loud,
// never silent wrong-code.
func (b *Builder) emitWinBootstrap() {
	b.e.MovRegGsMem(RAX, 0x60)     // PEB
	b.e.LoadBaseOff(RAX, RAX, 0x18) // PEB->Ldr
	b.e.LoadBaseOff(RAX, RAX, 0x20) // InMemoryOrder head
	b.e.MovRegImm32(R11, 64)        // walk bound (every process has kernel32)
	walkLbl := b.fresh("k32walk")
	nextLbl := b.fresh("k32next")
	foundLbl := b.fresh("k32found")
	failLbl := b.fresh("k32fail")
	b.e.Mark(walkLbl)
	b.e.LoadBaseOff(RAX, RAX, 0) // Flink -> entry links
	b.e.MovRegReg(RBX, RAX)
	b.e.SubRegImm32(RBX, 0x10) // links field -> entry base
	// Phase-149 correction: 64-bit LDR_DATA_TABLE_ENTRY puts DllBase at
	// +0x30 and BaseDllName at +0x58 (Length +0, Buffer +8); +0x28/+0x50
	// is the 32-bit shape and skipped every module into the Int3.
	b.e.MovzxRegMem16(RCX, RBX, 0x58)
	b.e.CmpRegImm32(RCX, 24) // "kernel32.dll" is 24 bytes
	b.e.Jnz(nextLbl)
	b.e.LoadBaseOff(RDX, RBX, 0x60) // BaseDllName.Buffer
	// Phase-149 ground truth: BaseDllName arrives UPPERCASE
	// (KERNEL32.DLL — read live from the PEB), so fold each WCHAR
	// with 0x20 before comparing against lowercase (folding is a
	// no-op for the digits and dots in this alphabet).
	for k, ch := range "kernel32.dll" {
		b.e.MovzxRegMem16(R8, RDX, k*2)
		b.e.OrRegImm8(R8, 0x20)
		b.e.CmpRegImm32(R8, uint32(ch))
		b.e.Jnz(nextLbl)
	}
	b.e.LoadBaseOff(R10, RBX, 0x30) // DllBase -> kept for all resolves
	b.e.Jmp(foundLbl)
	b.e.Mark(nextLbl)
	b.e.DecReg(R11)
	b.e.Jnz(walkLbl)
	b.e.Mark(failLbl)
	b.e.Int3()
	b.e.Mark(foundLbl)
	b.emitWinResolve("ExitProcess", IATExitProcess)
	b.emitWinResolve("GetStdHandle", IATGetStdHandle)
	b.emitWinResolve("WriteFile", IATWriteFile)
}

// emitWinResolve emits one export-table walk resolving a kernel32 name
// (R10 holds the module base): names/ordinals/functions pointers,
// count-bounded candidate loop with unrolled byte compares, resolved
// address published to the IAT slot. Falls into Int3 when exhausted.
func (b *Builder) emitWinResolve(name string, slot int) {
	b.e.LoadBaseOff32(RCX, R10, 0x3C) // e_lfanew
	b.e.MovRegReg(RDX, R10)
	b.e.AddRegReg(RDX, RCX) // RDX = NT headers
	// Phase-149 ground truth: the export directory is at NT+136
	// (COFF 24 + Optional 112 — the +96 shape is the 32-bit header;
	// verified live against kernel32: dir[0] VA 0xA65F0). The old +120
	// read garbage and faulted the first namesPtr load.
	b.e.LoadBaseOff32(RCX, RDX, 136) // export directory RVA
	b.e.MovRegReg(RDX, R10)
	b.e.AddRegReg(RDX, RCX) // RDX = export directory
	b.e.LoadBaseOff32(R9, RDX, 24)  // NumberOfNames
	b.e.LoadBaseOff32(RCX, RDX, 32) // AddressOfNames
	b.e.MovRegReg(RSI, R10)
	b.e.AddRegReg(RSI, RCX) // RSI = namesPtr
	b.e.LoadBaseOff32(RCX, RDX, 36) // AddressOfNameOrdinals
	b.e.MovRegReg(RDI, R10)
	b.e.AddRegReg(RDI, RCX) // RDI = ordPtr
	b.e.LoadBaseOff32(RCX, RDX, 28) // AddressOfFunctions
	b.e.MovRegReg(RBP, R10)
	b.e.AddRegReg(RBP, RCX) // RBP = funcsPtr
	loopLbl := b.fresh("exprt")
	nextLbl := b.fresh("expnext")
	doneLbl := b.fresh("expdone")
	failLbl := b.fresh("expfail")
	b.e.Mark(loopLbl)
	b.e.TestRegReg(R9, R9)
	b.e.Jz(failLbl)
	b.e.LoadBaseOff32(RCX, RSI, 0) // nameRVA
	b.e.MovRegReg(RDX, R10)
	b.e.AddRegReg(RDX, RCX) // RDX = candidate name
	// Export names are ASCII bytes; compare them as aligned pairs
	// (odd tail pairs against the NUL terminator, always mapped).
	for k := 0; k < len(name); k += 2 {
		lo := uint32(name[k])
		var hi uint32
		if k+1 < len(name) {
			hi = uint32(name[k+1])
		}
		b.e.MovzxRegMem16(R8, RDX, k)
		b.e.CmpRegImm32(R8, lo|hi<<8)
		b.e.Jnz(nextLbl)
	}
	b.e.MovzxRegMem16(R8, RDI, 0) // ordinal
	b.e.LoadScaled32(RAX, RBP, R8, 4, 0)
	b.e.MovRegReg(RDX, R10)
	b.e.AddRegReg(RDX, RAX)
	// Phase-149 correction: 48 A3 stores RAX specifically (moffs form
	// is accumulator-only), so the resolved address moves RDX -> RAX
	// first — storing RDX's predecessor (the raw funcRVA) was silently
	// publishing garbage into the slot.
	b.e.MovRegReg(RAX, RDX)
	pos := b.e.StoreAbs64Placeholder()
	b.apatches = append(b.apatches, absPatch{pos: pos, index: slot})
	b.e.Jmp(doneLbl)
	b.e.Mark(nextLbl)
	b.e.AddRegImm32(RSI, 4)
	b.e.AddRegImm32(RDI, 2)
	b.e.DecReg(R9)
	b.e.Jmp(loopLbl)
	b.e.Mark(failLbl)
	b.e.Int3()
	b.e.Mark(doneLbl)
}

// iatCall emits movabs rax, <IAT slot index> + call rax for a kernel32
// import (Phase 149, PE target only). The slot address resolves at link
// time against the .idata base, reusing the imm64 placeholder machinery.
func (b *Builder) iatCall(index int) {
	pos := b.e.imm64Patch(RAX)
	b.ipatches = append(b.ipatches, iatPatch{pos: pos, index: index})
	b.e.CallReg(RAX)
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
	b.e.MovRegImm32(RAX, b.sysWrite())
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
	b.emitWrite()
	b.e.AddRsp(64)
	// Phase 147: `print` terminates the line (matches the C-backend
	// contract the execution goldens pin: "42\n", not "42"). RDI is
	// still 1 from the payload write; syscalls clobber only RCX/R11.
	b.printNewline()
	b.e.Ret()

	b.e.Mark("print_str")
	b.e.MovRegReg(RDX, RSI)
	b.e.MovRegReg(RSI, RDI)
	b.e.MovRegImm32(RDI, 1)
	b.emitWrite()
	b.printNewline()
	b.e.Ret()
}

// printNewline emits write(1, "\n", 1) with RDI already holding 1.
func (b *Builder) printNewline() {
	b.rodataRef(RSI, "\n")
	b.e.MovRegImm32(RDX, 1)
	b.emitWrite()
}

func (b *Builder) layout(fd *parser.FuncDecl) error {
	b.slots = map[string]int{}
	b.kinds = map[string]int{}
	b.scanErr = nil
	next := 0
	for i, p := range fd.Params {
		// Phase 148: string parameters occupy two slots (ptr+len).
		// Phase 150A: array parameters occupy two slots (ptr+len) as
		// well, but no annotation syntax names them yet — a body that
		// indexes a parameter is a loud K145 (150B work).
		b.slots[p] = next
		b.kinds[p] = paramKind(fd, i)
		next += 8 * kindUnits(b.kinds[p])
	}
	next = b.scanLets(fd.Body, next)
	if b.scanErr != nil {
		return b.scanErr
	}
	// Phase 147: binary-operand scratch stack (see maxBinDepth). rsp never
	// moves during expression evaluation, so slot addresses stay stable.
	b.binTemp = next
	next += 8 * maxBinDepth
	// Phase 148: caller-frame extras area for argument units past the six
	// register units (sized by the hungriest call site in this function).
	maxExtra := b.scanMaxExtras(fd.Body)
	b.extrasBase = next
	next += maxExtra * 8
	// Phase 149: Win64 calls fault on misaligned stacks, so the frame
	// is rounded to 16 on Windows (extras sizing can otherwise leave it
	// 8-mod-16; Linux never noticed because syscalls don't care, and its
	// images stay byte-frozen by gating this to Windows).
	if b.goos == OSWindows && next%16 != 0 {
		next += 8
	}
	b.frame = next + argSpillBytes
	return nil
}

// scanMaxExtras returns the largest extras-unit count of any call in a
// statement list: total caller-side 8-byte units (int 1, string 2) minus
// the six register units, floored at 0. It walks every expression
// position so nested calls size the area too; unknown callees are
// ignored here (emission rejects them loudly).
func (b *Builder) scanMaxExtras(stmts []parser.Node) int {
	max := 0
	var walkExpr func(n parser.Node)
	walkExpr = func(n parser.Node) {
		switch x := n.(type) {
		case *parser.CallExpr:
			units := 0
			for _, a := range x.Args {
				if isStr, err := b.isStringExpr(a); err == nil && isStr {
					units += 2
				} else {
					units++
				}
			}
			if e := units - len(argRegs); e > max {
				max = e
			}
			for _, a := range x.Args {
				walkExpr(a)
			}
		case *parser.IndirectCallExpr:
			for _, a := range x.Args {
				walkExpr(a)
			}
			walkExpr(x.Target)
		case *parser.BinaryExpr:
			walkExpr(x.Left)
			walkExpr(x.Right)
		case *parser.UnaryExpr:
			walkExpr(x.Operand)
		case *parser.IndexExpr:
			walkExpr(x.Left)
			walkExpr(x.Index)
		case *parser.ArrayLiteral:
			for _, e := range x.Elements {
				walkExpr(e)
			}
		case *parser.StructLiteral:
			for _, f := range x.Fields {
				walkExpr(f)
			}
		}
	}
	var walkStmts func(stmts []parser.Node)
	walkStmts = func(stmts []parser.Node) {
		for _, s := range stmts {
			switch n := s.(type) {
			case *parser.VarDeclStmt:
				if n.Value != nil {
					walkExpr(n.Value)
				}
			case *parser.ReturnStmt:
				if n.Value != nil {
					walkExpr(n.Value)
				}
			case *parser.ExprStmt:
				walkExpr(n.Expression)
			case *parser.PrintStmt:
				walkExpr(n.Value)
			case *parser.IfStmt:
				walkExpr(n.Condition)
				walkStmts(n.Consequence)
				walkStmts(n.Alternative)
			case *parser.WhileStmt:
				walkExpr(n.Condition)
				walkStmts(n.Body)
			case *parser.ForStmt:
				if n.Init != nil {
					walkStmts([]parser.Node{n.Init})
				}
				if n.Condition != nil {
					walkExpr(n.Condition)
				}
				if n.Post != nil {
					post := n.Post
					if be, ok := post.(*parser.BinaryExpr); ok && be.Operator == "=" {
						post = &parser.ExprStmt{Expression: be, Line: be.Line}
					}
					walkStmts([]parser.Node{post})
				}
				walkStmts(n.Body)
			case *parser.ForInStmt:
				walkExpr(n.Iter)
				walkStmts(n.Body)
			case *parser.BlockStmt:
				walkStmts(n.Statements)
			}
		}
	}
	walkStmts(stmts)
	return max
}
// into control-flow bodies and C-for initializers (Phase 148: loop bodies
// may declare variables; slots are function-wide, first declaration wins
// a slot and shadowing writes through — v1 semantics, documented). The
// first isStringExpr failure is recorded and returned by layout; emission
// re-validates every node anyway, so the diagnostic is identical.
func (b *Builder) scanLets(stmts []parser.Node, next int) int {
	for _, s := range stmts {
		switch n := s.(type) {
		case *parser.VarDeclStmt:
			if _, seen := b.slots[n.Name]; seen {
				continue
			}
			off := next
			next += 8
			k, err := b.exprKind(n.Value)
			if err != nil {
				if b.scanErr == nil {
					b.scanErr = err
				}
				return next
			}
			if kindUnits(k) == 2 {
				next += 8
			}
			if k == KindArray {
				// Phase 150A: array element storage lives in the frame
				// right after the (ptr,len) header, so every array has
				// a fixed compile-time footprint: header + N int slots.
				// Only int-element literals lower in 150A (see exprKind).
				if lit, ok := n.Value.(*parser.ArrayLiteral); ok {
					next += 8 * len(lit.Elements)
				}
			}
			b.slots[n.Name] = off
			b.kinds[n.Name] = k
		case *parser.IfStmt:
			next = b.scanLets(n.Consequence, next)
			next = b.scanLets(n.Alternative, next)
		case *parser.WhileStmt:
			next = b.scanLets(n.Body, next)
		case *parser.ForStmt:
			if n.Init != nil {
				next = b.scanLets([]parser.Node{n.Init}, next)
			}
			next = b.scanLets(n.Body, next)
		case *parser.ForInStmt:
			next = b.scanLets(n.Body, next)
		case *parser.BlockStmt:
			next = b.scanLets(n.Statements, next)
		}
	}
	return next
}

func (b *Builder) emitFunc(fd *parser.FuncDecl) error {
	// Phase 148: main takes no arguments on the native target (there is
	// no argv protocol in v1; _start invokes it bare).
	if fd.Name == "main" && len(fd.Params) > 0 {
		return fmt.Errorf("error[K145]: 'main' takes no arguments on the native target")
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
	// Home the parameters under the native ABI (Phase 148): the first
	// six 8-byte units arrive in RDI,RSI,RDX,RCX,R8,R9 (a string takes
	// two units); further units arrive in the caller-frame extras array
	// addressed by R10. R10 dies at the first call, so stack homing runs
	// here at entry, before anything else.
	unit := 0
	for _, p := range fd.Params {
		units := kindUnits(b.kinds[p])
		for k := 0; k < units; k++ {
			if unit < len(argRegs) {
				b.e.StoreStack(argRegs[unit], b.slots[p]+k*8)
			} else {
				b.e.LoadBaseOff(RAX, R10, (unit-len(argRegs))*8)
				b.e.StoreStack(RAX, b.slots[p]+k*8)
			}
			unit++
		}
	}
	returned := false
	for _, s := range fd.Body {
		done, err := b.emitStmt(s, fd.Name)
		if err != nil {
			return err
		}
		if done {
			returned = true
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

// emitStmt lowers one statement. It returns done=true for a mid-body
// `return` (the caller records it so a missing trailing return still
// yields 0). Phase 148: factored from emitFunc so loop and branch bodies
// reuse the exact straight-line paths (byte-identical for straight-line
// programs by construction).
func (b *Builder) emitStmt(s parser.Node, fname string) (bool, error) {
	switch n := s.(type) {
	case *parser.VarDeclStmt:
		if err := b.emitLet(n); err != nil {
			return false, err
		}
		return false, nil
	case *parser.PrintStmt:
		if err := b.emitPrint(n); err != nil {
			return false, err
		}
		return false, nil
	case *parser.ReturnStmt:
		if err := b.emitReturn(n, fname == "main"); err != nil {
			return false, err
		}
		name := "fn_" + fname
		if fname == "main" {
			name = "karkain_main"
		}
		b.e.Jmp(name + "$ret")
		return true, nil
	case *parser.ExprStmt:
		if err := b.emitExprStmt(n); err != nil {
			return false, err
		}
		return false, nil
	case *parser.IfStmt:
		return false, b.emitIf(n, fname)
	case *parser.WhileStmt:
		return false, b.emitWhile(n, fname)
	case *parser.ForStmt:
		return false, b.emitFor(n, fname)
	case *parser.BreakStmt:
		if len(b.loops) == 0 {
			return false, fmt.Errorf("error[K145]: break outside of a loop")
		}
		b.e.Jmp(b.loops[len(b.loops)-1].brk)
		return false, nil
	case *parser.ContinueStmt:
		if len(b.loops) == 0 {
			return false, fmt.Errorf("error[K145]: continue outside of a loop")
		}
		b.e.Jmp(b.loops[len(b.loops)-1].cont)
		return false, nil
	case *parser.ForInStmt:
		return false, fmt.Errorf("error[K145]: for-in loops are not supported on the native target yet (arrays lower in a later slice; use while or C-style for)")
	default:
		return false, fmt.Errorf("error[K145]: unsupported statement %T in '%s'", s, fname)
	}
}

// emitExprStmt lowers an expression statement: plain value expressions
// evaluate and discard, while `x = <int>` reassigns a slot variable
// (Phase 148: loop counters need reassignment; strings are immutable
// through this form — declare a fresh variable instead).
func (b *Builder) emitExprStmt(n *parser.ExprStmt) error {
	if be, ok := n.Expression.(*parser.BinaryExpr); ok && be.Operator == "=" {
		id, ok := be.Left.(*parser.Identifier)
		if !ok {
			return fmt.Errorf("error[K145]: assignment target must be a variable")
		}
		off, ok := b.slots[id.Name]
		if !ok {
			return fmt.Errorf("error[K145]: undefined identifier '%s'", id.Name)
		}
		if b.kinds[id.Name] == KindString || b.kinds[id.Name] == KindArray {
			return fmt.Errorf("error[K145]: cannot reassign %s '%s' (declare a fresh variable)", kindName(b.kinds[id.Name]), id.Name)
		}
		rk, err := b.exprKind(be.Right)
		if err != nil {
			return err
		}
		if rk != b.kinds[id.Name] {
			return fmt.Errorf("error[K145]: cannot assign a %s to %s variable '%s'", kindName(rk), kindName(b.kinds[id.Name]), id.Name)
		}
		if err := b.emitExpr(be.Right, 0); err != nil {
			return err
		}
		b.e.StoreStack(RAX, off)
		return nil
	}
	return b.emitExpr(n.Expression, 0)
}

// emitCond evaluates an int comparison and jumps to falseLabel when it
// does NOT hold. Only `== != < <= > >=` over int expressions lower in v1
// (operands reuse the depth-indexed scratch evaluator, so rsp stays put);
// anything else is a loud K145, never a miscompiled truthiness test.
func (b *Builder) emitCond(cond parser.Node, falseLabel string) error {
	be, ok := cond.(*parser.BinaryExpr)
	if !ok {
		return fmt.Errorf("error[K145]: condition must be an int comparison (==, !=, <, <=, >, >=)")
	}
	var jump func(string)
	switch be.Operator {
	case "==":
		jump = b.e.Jnz
	case "!=":
		jump = b.e.Jz
	case "<":
		jump = b.e.Jge
	case "<=":
		jump = b.e.Jg
	case ">":
		jump = b.e.Jle
	case ">=":
		jump = b.e.Jl
	default:
		return fmt.Errorf("error[K145]: condition must be an int comparison (==, !=, <, <=, >, >=), found '%s'", be.Operator)
	}
	if isStr, err := b.isStringExpr(be.Left); err != nil {
		return err
	} else if isStr {
		return fmt.Errorf("error[K145]: string comparison is not supported (ints only)")
	}
	if isStr, err := b.isStringExpr(be.Right); err != nil {
		return err
	} else if isStr {
		return fmt.Errorf("error[K145]: string comparison is not supported (ints only)")
	}
	// Phase 150A Step 1: float operands now lower (bits in RAX), so a float
	// comparison must be refused here — CmpRegReg would silently compare the
	// IEEE-754 bit patterns as integers. Float comparison lands in a later
	// 150A step.
	if k, err := b.exprKind(be.Left); err != nil {
		return err
	} else if k == KindFloat {
		return fmt.Errorf("error[K145]: floating-point comparison is not supported yet (arrives in a later Phase 150A step)")
	}
	if k, err := b.exprKind(be.Right); err != nil {
		return err
	} else if k == KindFloat {
		return fmt.Errorf("error[K145]: floating-point comparison is not supported yet (arrives in a later Phase 150A step)")
	}
	if err := b.emitExpr(be.Left, 1); err != nil {
		return err
	}
	b.e.StoreStack(RAX, b.binTemp)
	if err := b.emitExpr(be.Right, 1); err != nil {
		return err
	}
	b.e.MovRegReg(RCX, RAX)
	b.e.LoadStack(RAX, b.binTemp)
	b.e.CmpRegReg(RAX, RCX)
	jump(falseLabel)
	return nil
}

func (b *Builder) emitStmts(stmts []parser.Node, fname string) error {
	for _, s := range stmts {
		if _, err := b.emitStmt(s, fname); err != nil {
			return err
		}
	}
	return nil
}

// emitIf lowers `if cond { consequence } else { alternative }`.
func (b *Builder) emitIf(n *parser.IfStmt, fname string) error {
	elseLabel := b.fresh("ifelse")
	endLabel := b.fresh("ifend")
	if err := b.emitCond(n.Condition, elseLabel); err != nil {
		return err
	}
	if err := b.emitStmts(n.Consequence, fname); err != nil {
		return err
	}
	b.e.Jmp(endLabel)
	b.e.Mark(elseLabel)
	if err := b.emitStmts(n.Alternative, fname); err != nil {
		return err
	}
	b.e.Mark(endLabel)
	return nil
}

// emitWhile lowers `while cond { body }`.
func (b *Builder) emitWhile(n *parser.WhileStmt, fname string) error {
	loopLabel := b.fresh("while")
	endLabel := b.fresh("whileend")
	b.e.Mark(loopLabel)
	b.loops = append(b.loops, loopTgt{brk: endLabel, cont: loopLabel})
	if err := b.emitCond(n.Condition, endLabel); err != nil {
		b.loops = b.loops[:len(b.loops)-1]
		return err
	}
	if err := b.emitStmts(n.Body, fname); err != nil {
		b.loops = b.loops[:len(b.loops)-1]
		return err
	}
	b.loops = b.loops[:len(b.loops)-1]
	b.e.Jmp(loopLabel)
	b.e.Mark(endLabel)
	return nil
}

// emitFor lowers C-style `for (init; cond; post) { body }`. A nil
// condition means always-true; init/post are arbitrary statements
// (typically `let` and reassignment) emitted inline.
func (b *Builder) emitFor(n *parser.ForStmt, fname string) error {
	if n.Init != nil {
		if _, err := b.emitStmt(n.Init, fname); err != nil {
			return err
		}
	}
	loopLabel := b.fresh("for")
	postLabel := b.fresh("forpost")
	endLabel := b.fresh("forend")
	b.e.Mark(loopLabel)
	b.loops = append(b.loops, loopTgt{brk: endLabel, cont: postLabel})
	if n.Condition != nil {
		if err := b.emitCond(n.Condition, endLabel); err != nil {
			b.loops = b.loops[:len(b.loops)-1]
			return err
		}
	}
	if err := b.emitStmts(n.Body, fname); err != nil {
		b.loops = b.loops[:len(b.loops)-1]
		return err
	}
	b.e.Mark(postLabel)
	if n.Post != nil {
		// The parser reads the post clause as a bare expression, so a
		// `i = i + 1` post arrives as BinaryExpr("="), not an ExprStmt —
		// wrap it so the assignment path handles it identically.
		post := n.Post
		if be, ok := post.(*parser.BinaryExpr); ok && be.Operator == "=" {
			post = &parser.ExprStmt{Expression: be, Line: be.Line}
		}
		if _, err := b.emitStmt(post, fname); err != nil {
			b.loops = b.loops[:len(b.loops)-1]
			return err
		}
	}
	b.loops = b.loops[:len(b.loops)-1]
	b.e.Jmp(loopLabel)
	b.e.Mark(endLabel)
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
		if err := b.emitStr(n.Value, 0); err != nil {
			return err
		}
		b.e.StoreStack(RDI, off)
		b.e.StoreStack(RSI, off+8)
		return nil
	}
	if err := b.emitExpr(n.Value, 0); err != nil {
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
		if err := b.emitStr(n.Value, 0); err != nil {
			return err
		}
		b.e.Call("print_str")
		return nil
	}
	// Phase 150A Step 1: print_float is a later 150A step. Without this
	// guard the float bits would reach print_int and print as an integer.
	if k, err := b.exprKind(n.Value); err != nil {
		return err
	} else if k == KindFloat {
		return fmt.Errorf("error[K145]: floating-point printing is not supported yet (arrives in a later Phase 150A step)")
	}
	if err := b.emitExpr(n.Value, 0); err != nil {
		return err
	}
	b.e.MovRegReg(RDI, RAX)
	b.e.Call("print_int")
	return nil
}

func (b *Builder) emitReturn(n *parser.ReturnStmt, isMain bool) error {
	if n.Value == nil {
		b.e.XorRegReg(RAX)
		return nil
	}
	if isStr, err := b.isStringExpr(n.Value); err != nil {
		return err
	} else if isStr {
		// Phase 148: string-return convention is (RAX=ptr, RDX=len).
		// main keeps the int-only rule (exit codes are ints).
		if isMain {
			return fmt.Errorf("error[K145]: string return values are not supported (function must return int)")
		}
		if err := b.emitStr(n.Value, 0); err != nil {
			return err
		}
		b.e.MovRegReg(RAX, RDI)
		b.e.MovRegReg(RDX, RSI)
		return nil
	}
	// Phase 150A Step 1: a non-main float return already leaves the bits in
	// RAX (emitExpr -> emitFloat), which is the one-unit float return
	// convention; main keeps the int-only rule because its value becomes the
	// process exit code.
	if isMain {
		if k, err := b.exprKind(n.Value); err != nil {
			return err
		} else if k == KindFloat {
			return fmt.Errorf("error[K145]: floating-point return values are not supported (function must return int)")
		}
	}
	return b.emitExpr(n.Value, 0)
}

func (b *Builder) isStringExpr(n parser.Node) (bool, error) {
	k, err := b.exprKind(n)
	if err != nil {
		return false, err
	}
	return k == KindString, nil
}

// exprKind classifies a value expression (Phase 150A). Ints and floats
// are distinguished so float arithmetic never silently truncates; arrays
// are fat (ptr+len) values. Binary/unary operators inherit int (float
// operators are checked at emission); calls follow the callee's inferred
// return kind. Anything else is a loud K145.
func (b *Builder) exprKind(n parser.Node) (int, error) {
	switch x := n.(type) {
	case *parser.StringLiteral:
		return KindString, nil
	case *parser.IntLiteral:
		return KindInt, nil
	case *parser.Float64Literal:
		return KindFloat, nil
	case *parser.Identifier:
		k, ok := b.kinds[x.Name]
		if !ok {
			return KindInt, fmt.Errorf("error[K145]: undefined identifier '%s'", x.Name)
		}
		return k, nil
	case *parser.BinaryExpr:
		// Phase 150A: arithmetic inherits float when either operand
		// is float (mixed int+float is rejected loudly at emission);
		// comparisons always yield int.
		switch x.Operator {
		case "+", "-", "*", "/", "%":
			lk, err := b.exprKind(x.Left)
			if err != nil {
				return KindInt, err
			}
			rk, err := b.exprKind(x.Right)
			if err != nil {
				return KindInt, err
			}
			if lk == KindFloat || rk == KindFloat {
				return KindFloat, nil
			}
			return KindInt, nil
		default:
			return KindInt, nil
		}
	case *parser.UnaryExpr:
		return b.exprKind(x.Operand)
	case *parser.CallExpr:
		// Phase 148: kind follows the callee's inferred return kind
		// (unknown callees already fail loudly at emission).
		if k, ok := b.retKind[x.Function]; ok {
			return k, nil
		}
		return KindInt, nil
	case *parser.ArrayLiteral:
		// Phase 150A: int-element literals only (element kinds are
		// checked at emission, where the diagnostic names the index).
		return KindArray, nil
	case *parser.IndexExpr:
		return KindInt, nil
	default:
		return KindInt, fmt.Errorf("error[K145]: unsupported expression %T", n)
	}
}

func (b *Builder) emitExpr(n parser.Node, depth int) error {
	switch x := n.(type) {
	case *parser.IntLiteral:
		b.e.MovRegImm64(RAX, uint64(parseIntLit(x.Value)))
		return nil
	case *parser.Identifier:
		// Phase 150A Step 1: a float identifier carries its IEEE-754 bits
		// in its single slot; a string identifier is two units.
		if b.kinds[x.Name] == KindFloat {
			return b.emitFloat(x, depth)
		}
		if b.kinds[x.Name] != KindInt {
			return fmt.Errorf("error[K145]: %s '%s' in int position", kindName(b.kinds[x.Name]), x.Name)
		}
		off, ok := b.slots[x.Name]
		if !ok {
			return fmt.Errorf("error[K145]: undefined identifier '%s'", x.Name)
		}
		b.e.LoadStack(RAX, off)
		return nil
	case *parser.Float64Literal:
		return b.emitFloat(x, depth)
	case *parser.BinaryExpr:
		return b.emitBinary(x, depth)
	case *parser.UnaryExpr:
		if x.Operator != "-" {
			return fmt.Errorf("error[K145]: unsupported unary operator '%s'", x.Operator)
		}
		// Phase 150A Step 1: float unary minus is a later 150A step.
		// NegReg would negate the bit pattern as an integer, so refuse.
		if k, err := b.exprKind(x.Operand); err != nil {
			return err
		} else if k == KindFloat {
			return fmt.Errorf("error[K145]: floating-point unary minus is not supported yet (arrives in a later Phase 150A step)")
		}
		if err := b.emitExpr(x.Operand, depth); err != nil {
			return err
		}
		b.e.NegReg(RAX)
		return nil
	case *parser.CallExpr:
		return b.emitCallValue(x, depth)
	default:
		return fmt.Errorf("error[K145]: unsupported expression %T", n)
	}
}

// emitFloat lowers a float64 expression, leaving the IEEE-754 bit pattern in
// RAX — the approved Phase-150A representation (float64 -> bits -> RAX), one
// 8-byte unit exactly like an int. Phase 150A Step 1 covers literals and float
// variables only: arithmetic, comparison, unary minus, printing and
// float-returning calls belong to later 150A steps and are refused loudly here
// so a float is never silently reinterpreted as an integer.
func (b *Builder) emitFloat(n parser.Node, depth int) error {
	switch x := n.(type) {
	case *parser.Float64Literal:
		v, err := strconv.ParseFloat(x.Value, 64)
		if err != nil {
			return fmt.Errorf("error[K145]: invalid float literal '%s'", x.Value)
		}
		// The parsed pattern is materialized verbatim — no rounding and no
		// sign-bit normalization — so the bits of 0.0 and -0.0 stay
		// distinct (the parser lexes a leading '-' as TokenMinus, so a
		// negative literal arrives here without its sign; see the
		// unary-minus guard in emitExpr).
		b.e.MovRegImm64(RAX, math.Float64bits(v))
		return nil
	case *parser.Identifier:
		if b.kinds[x.Name] != KindFloat {
			return fmt.Errorf("error[K145]: %s '%s' in float position", kindName(b.kinds[x.Name]), x.Name)
		}
		off, ok := b.slots[x.Name]
		if !ok {
			return fmt.Errorf("error[K145]: undefined identifier '%s'", x.Name)
		}
		// One unit, like the int path: the stored 64-bit pattern is
		// reloaded unchanged and rsp never moves.
		b.e.LoadStack(RAX, off)
		return nil
	default:
		return fmt.Errorf("error[K145]: unsupported float expression %T (literals and float variables only)", n)
	}
}

func (b *Builder) emitStr(n parser.Node, depth int) error {
	switch x := n.(type) {
	case *parser.StringLiteral:
		off := b.internRodata(x.Value)
		b.patches = append(b.patches, addrPatch{pos: b.e.imm64Patch(RDI), roOff: off})
		b.e.MovRegImm32(RSI, uint32(len(x.Value)))
		return nil
	case *parser.Identifier:
		if b.kinds[x.Name] != KindString {
			return fmt.Errorf("error[K145]: %s '%s' in string position", kindName(b.kinds[x.Name]), x.Name)
		}
		off, ok := b.slots[x.Name]
		if !ok {
			return fmt.Errorf("error[K145]: undefined identifier '%s'", x.Name)
		}
		b.e.LoadStack(RDI, off)
		b.e.LoadStack(RSI, off+8)
		return nil
	case *parser.CallExpr:
		// Phase 148: a string-returning call leaves (RAX=ptr, RDX=len);
		// move the pair into the (RDI, RSI) string-value convention.
		if err := b.emitCallValue(x, depth); err != nil {
			return err
		}
		b.e.MovRegReg(RDI, RAX)
		b.e.MovRegReg(RSI, RDX)
		return nil
	default:
		return fmt.Errorf("error[K145]: unsupported string expression %T (literals, variables and string calls only)", n)
	}
}

func (b *Builder) emitBinary(x *parser.BinaryExpr, depth int) error {
	if x.Operator != "+" && x.Operator != "-" && x.Operator != "*" {
		return fmt.Errorf("error[K145]: unsupported operator '%s' (want +, - or *)", x.Operator)
	}
	// Phase 150A Step 1: int arithmetic only. Float operands now lower (bits
	// in RAX), so an unguarded add/sub/mul here would compute on the
	// IEEE-754 bit patterns; float arithmetic is a later 150A step.
	for _, operand := range []parser.Node{x.Left, x.Right} {
		if k, err := b.exprKind(operand); err != nil {
			return err
		} else if k == KindFloat {
			return fmt.Errorf("error[K145]: floating-point arithmetic is not supported yet (arrives in a later Phase 150A step)")
		}
	}
	if depth >= maxBinDepth {
		return fmt.Errorf("error[K145]: expression nesting exceeds %d binary levels", maxBinDepth)
	}
	// Phase 147: stage the left operand through the depth-indexed scratch
	// slot. The old code pushed rax, which moved rsp and shifted every
	// frame-relative slot address — the right operand then re-read the
	// left's slot (`add(20,22)` yielded 40). rsp never moves now.
	if err := b.emitExpr(x.Left, depth+1); err != nil {
		return err
	}
	b.e.StoreStack(RAX, b.binTemp+depth*8)
	if err := b.emitExpr(x.Right, depth+1); err != nil {
		return err
	}
	b.e.MovRegReg(RCX, RAX)
	b.e.LoadStack(RAX, b.binTemp+depth*8)
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

func (b *Builder) emitCallValue(x *parser.CallExpr, depth int) error {
	if x.Module != "" || x.IsCFunc {
		return fmt.Errorf("error[K145]: module-qualified and C-interop calls are not supported (call to '%s')", x.Function)
	}
	fd, ok := b.ftab[x.Function]
	if !ok {
		return fmt.Errorf("error[K145]: undefined function '%s'", x.Function)
	}
	// Phase 148: exact arity (previously unchecked — a short call read
	// uninitialized slots, a long one silently dropped arguments).
	if len(x.Args) != len(fd.Params) {
		return fmt.Errorf("error[K145]: call to '%s' has %d args (want %d)", x.Function, len(x.Args), len(fd.Params))
	}
	// Kind check + unit plan: ints and floats take one unit, strings
	// take two; units 0-5 ride argRegs, further units ride the extras
	// area via R10. Arrays are not passable in 150A (no annotation
	// syntax names them; 150B work) — a loud K145, never a miscompile.
	argKind := make([]int, len(x.Args))
	for i, a := range x.Args {
		got, err := b.exprKind(a)
		if err != nil {
			return err
		}
		if got == KindArray {
			return fmt.Errorf("error[K145]: array argument for parameter '%s' is not supported (call to '%s')", fd.Params[i], x.Function)
		}
		want := paramKind(fd, i)
		if got != want {
			return fmt.Errorf("error[K145]: %s argument for %s parameter '%s' (call to '%s')", kindName(got), kindName(want), fd.Params[i], x.Function)
		}
		argKind[i] = got
	}
	// Evaluate + stage: register units to the per-arg spill, extras units
	// to the caller-frame extras area. rsp never moves (147's rule).
	// Floats ride RAX as f64 bits, exactly like ints.
	unit := 0
	for i, a := range x.Args {
		if argKind[i] == KindString {
			if err := b.emitStr(a, depth); err != nil {
				return err
			}
			b.stageUnit(i, unit, RDI)
			b.stageUnit(i, unit+1, RSI)
			unit += 2
			continue
		}
		if argKind[i] == KindFloat {
			if err := b.emitFloat(a, depth); err != nil {
				return err
			}
		} else if err := b.emitExpr(a, depth); err != nil {
			return err
		}
		b.stageUnit(i, unit, RAX)
		unit++
	}
	// Load the register units back (extras are already home).
	unit = 0
	for i := range x.Args {
		units := kindUnits(argKind[i])
		for k := 0; k < units; k++ {
			if unit < len(argRegs) {
				b.e.LoadStack(argRegs[unit], b.argTemp(i)+k*8)
			}
			unit++
		}
	}
	if unit > len(argRegs) {
		b.e.LeaRegStack(R10, b.extrasBase)
	}
	if x.Function == "main" {
		b.e.Call("karkain_main")
	} else {
		b.e.Call("fn_" + x.Function)
	}
	return nil
}

// stageUnit homes one evaluated unit: unit < 6 goes to the per-arg spill
// (argTemp(i) is 16 bytes: k selects the low/high half), further units go
// to the caller-frame extras area at the callee-visible extras index.
func (b *Builder) stageUnit(arg, unit int, r Reg) {
	if unit < len(argRegs) {
		off := b.argTemp(arg)
		if r == RSI {
			off += 8
		}
		b.e.StoreStack(r, off)
		return
	}
	b.e.StoreStack(r, b.extrasBase+(unit-len(argRegs))*8)
}

func parseIntLit(s string) int64 {
	v, err := strconv.ParseInt(s, 0, 64)
	if err != nil {
		return 0
	}
	return v
}
