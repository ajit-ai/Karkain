package native

import "fmt"

// Reg is an x86-64 general register, numbered 0-15 (rax..rdi, r8..r15).
type Reg int

const (
	RAX Reg = iota
	RCX
	RDX
	RBX
	RSP
	RBP
	RSI
	RDI
	R8
	R9
	R10
	R11
	R12
	R13
	R14
	R15
)

// XmmReg is an x86-64 SSE/AVX register (xmm0..xmm15).
type XmmReg int

const (
	XMM0 XmmReg = iota
	XMM1
	XMM2
	XMM3
	XMM4
	XMM5
	XMM6
	XMM7
	XMM8
	XMM9
	XMM10
	XMM11
	XMM12
	XMM13
	XMM14
	XMM15
)

func (x XmmReg) low() byte { return byte(int(x) & 7) }
func (x XmmReg) ext() bool { return int(x) > 7 }

func (r Reg) low() byte { return byte(int(r) & 7) }
func (r Reg) ext() bool { return int(r) > 7 }

// patch is a pending rel32 fixup: 4 bytes at off resolve to label-addr when
// Finish runs (rel = target - (off+4)).
type patch struct {
	off   int
	label string
}

// Emitter accumulates .text bytes with label definition + rel32 patching.
type Emitter struct {
	code    []byte
	labels  map[string]int
	patches []patch
}

// NewEmitter creates an empty x86-64 emitter.
func NewEmitter() *Emitter { return &Emitter{labels: map[string]int{}} }

// Bytes returns the emitted machine code with all rel32 patches resolved.
// Every referenced label must be marked first (else Finish panics — a
// backend bug, never user input).
func (e *Emitter) Bytes() []byte {
	out := append([]byte{}, e.code...)
	for _, p := range e.patches {
		addr, ok := e.labels[p.label]
		if !ok {
			panic(fmt.Sprintf("native backend: undefined label %q", p.label))
		}
		rel := int32(addr - (p.off + 4))
		out[p.off] = byte(rel)
		out[p.off+1] = byte(rel >> 8)
		out[p.off+2] = byte(rel >> 16)
		out[p.off+3] = byte(rel >> 24)
	}
	return out
}

// Len returns the current code length (label arithmetic).
func (e *Emitter) Len() int { return len(e.code) }

// Mark defines a label at the current position.
func (e *Emitter) Mark(label string) {
	if _, dup := e.labels[label]; dup {
		panic(fmt.Sprintf("native backend: duplicate label %q", label))
	}
	e.labels[label] = len(e.code)
}

func (e *Emitter) byte(b byte) { e.code = append(e.code, b) }

func (e *Emitter) u32(v uint32) {
	e.code = append(e.code, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}

func (e *Emitter) u64(v uint64) {
	e.u32(uint32(v))
	e.u32(uint32(v >> 32))
}

// rex emits a REX prefix when needed: W for 64-bit operand size, R/B for
// the high bits of the modrm reg/rm (or opcode-reg) fields.
func (e *Emitter) rex(w bool, reg, rm Reg) {
	r := byte(0x40)
	if w {
		r |= 0x08
	}
	if reg.ext() {
		r |= 0x04
	}
	if rm.ext() {
		r |= 0x01
	}
	if r != 0x40 {
		e.byte(r)
	}
}

// modrm emits a ModRM byte: mod(2)|reg(3)|rm(3).
func (e *Emitter) modrm(mod, reg, rm byte) {
	e.byte(mod<<6 | (reg&7)<<3 | (rm & 7))
}

// memRsp emits ModRM+SIB+disp for [rsp+off] (rm=100 selects SIB; SIB 0x24
// is scale-0/index-none/base-rsp). off==0 encodes with no displacement.
func (e *Emitter) memRsp(reg Reg, off int) {
	if off == 0 {
		e.modrm(0, reg.low(), 4)
		e.byte(0x24)
		return
	}
	if off >= -128 && off <= 127 {
		e.modrm(1, reg.low(), 4)
		e.byte(0x24)
		e.byte(byte(int8(off)))
		return
	}
	e.modrm(2, reg.low(), 4)
	e.byte(0x24)
	e.u32(uint32(int32(off)))
}

// MovRegImm32 emits mov r32, imm32 (B8+rd io): the 32-bit result is
// zero-extended into the full r64, which is exactly what every caller
// wants (syscall numbers, fds, counts — all small nonneg constants).
// Phase-147 correction: this previously emitted REX.W + B8+rd with only
// a 32-bit immediate, but per the Intel SDM (vol. 2, MOV) REX.W + B8+rd
// IS mov r64, imm64 (10 bytes) — the CPU consumed the next 4 bytes as
// immediate, desynchronizing the stream at every use site. The REX
// prefix is now emitted only for r8-r15 (REX.B); plain registers get
// the 5-byte form with no prefix.
func (e *Emitter) MovRegImm32(r Reg, v uint32) {
	e.rex(false, 0, r)
	e.byte(0xB8 + r.low())
	e.u32(v)
}

// MovRegImm64 emits mov r64, imm64: REX.W + B8+rd io64.
func (e *Emitter) MovRegImm64(r Reg, v uint64) {
	e.rex(true, 0, r)
	e.byte(0xB8 + r.low())
	e.u64(v)
}

// MovRegReg emits mov r64, r64: REX.W + 89 /r.
func (e *Emitter) MovRegReg(dst, src Reg) {
	e.rex(true, src, dst)
	e.byte(0x89)
	e.modrm(3, src.low(), dst.low())
}

// LoadStack emits mov r64, [rsp+off].
func (e *Emitter) LoadStack(dst Reg, off int) {
	e.rex(true, dst, RSP)
	e.byte(0x8B)
	e.memRsp(dst, off)
}

// StoreStack emits mov [rsp+off], r64.
func (e *Emitter) StoreStack(src Reg, off int) {
	e.rex(true, src, RSP)
	e.byte(0x89)
	e.memRsp(src, off)
}

// AddRegReg emits add r64, r64: REX.W + 01 /r.
func (e *Emitter) AddRegReg(dst, src Reg) {
	e.rex(true, src, dst)
	e.byte(0x01)
	e.modrm(3, src.low(), dst.low())
}

// SubRegReg emits sub r64, r64: REX.W + 29 /r.
func (e *Emitter) SubRegReg(dst, src Reg) {
	e.rex(true, src, dst)
	e.byte(0x29)
	e.modrm(3, src.low(), dst.low())
}

// MulRegReg emits imul r64, r64 (signed): REX.W + 0F AF /r.
func (e *Emitter) MulRegReg(dst, src Reg) {
	e.rex(true, dst, src)
	e.byte(0x0F)
	e.byte(0xAF)
	e.modrm(3, dst.low(), src.low())
}

// AddRegImm32 emits add r64, imm (83 /0 ib when it fits, else 81 /0 id).
func (e *Emitter) AddRegImm32(r Reg, v int32) {
	e.rex(true, 0, r)
	if v >= -128 && v <= 127 {
		e.byte(0x83)
		e.modrm(3, 0, r.low())
		e.byte(byte(int8(v)))
		return
	}
	e.byte(0x81)
	e.modrm(3, 0, r.low())
	e.u32(uint32(v))
}

// MulRegImm32 emits imul r64, imm32 (69 /r id): the signed 32-bit immediate
// form, which sign-extends to the full 64-bit product.
func (e *Emitter) MulRegImm32(dst Reg, v int32) {
	e.byte(0x69)
	e.modrm(3, dst.low(), dst.low())
	e.u32(uint32(v))
}

// SarRegImm emits sar r64, imm8: REX.W + C1 /7 ib (arithmetic, so negative
// counts keep their sign — print_float's log10 estimate needs that).
func (e *Emitter) SarRegImm(r Reg, imm byte) {
	e.rex(true, 0, r)
	e.byte(0xC1)
	e.modrm(3, 7, r.low())
	e.byte(imm)
}

// CmpRegImm64 emits cmp r64, imm32: REX.W + 81 /7 id. Used for frame-local
// bounds tests whose threshold is a signed 32-bit constant.
func (e *Emitter) CmpRegImm64(r Reg, v int32) {
	e.rex(true, 0, r)
	e.byte(0x81)
	e.modrm(3, 7, r.low())
	e.u32(uint32(v))
}

// SubRegImm32 emits sub r64, imm.
func (e *Emitter) SubRegImm32(r Reg, v int32) {
	e.rex(true, 0, r)
	if v >= -128 && v <= 127 {
		e.byte(0x83)
		e.modrm(3, 5, r.low())
		e.byte(byte(int8(v)))
		return
	}
	e.byte(0x81)
	e.modrm(3, 5, r.low())
	e.u32(uint32(v))
}

// SubRsp emits sub rsp, imm (frame allocation).
func (e *Emitter) SubRsp(n int) { e.SubRegImm32(RSP, int32(n)) }

// AddRsp emits add rsp, imm (frame release).
func (e *Emitter) AddRsp(n int) { e.AddRegImm32(RSP, int32(n)) }

// PushReg emits push r64: 50+rd (+REX.B).
func (e *Emitter) PushReg(r Reg) {
	if r.ext() {
		e.byte(0x41)
	}
	e.byte(0x50 + r.low())
}

// PopReg emits pop r64.
func (e *Emitter) PopReg(r Reg) {
	if r.ext() {
		e.byte(0x41)
	}
	e.byte(0x58 + r.low())
}

// XorRegReg emits xor r64, r64 (zero idiom): REX.W + 31 /r.
func (e *Emitter) XorRegReg(r Reg) {
	e.rex(true, r, r)
	e.byte(0x31)
	e.modrm(3, r.low(), r.low())
}

// NegReg emits neg r64 (two's complement): REX.W + F7 /3.
func (e *Emitter) NegReg(r Reg) {
	e.rex(true, 0, r)
	e.byte(0xF7)
	e.modrm(3, 3, r.low())
}

// DecReg emits dec r64: REX.W + FF /1.
func (e *Emitter) DecReg(r Reg) {
	e.rex(true, 0, r)
	e.byte(0xFF)
	e.modrm(3, 1, r.low())
}

// Cqo emits cqo (sign-extend rax into rdx:rax for division): 48 99.
func (e *Emitter) Cqo() {
	e.byte(0x48)
	e.byte(0x99)
}

// DivReg emits div r/m64 (unsigned rdx:rax / r): REX.W + F7 /6.
func (e *Emitter) DivReg(r Reg) {
	e.rex(true, 0, r)
	e.byte(0xF7)
	e.modrm(3, 6, r.low())
}

// TestRegReg emits test r64, r64: REX.W + 85 /r.
func (e *Emitter) TestRegReg(a, b Reg) {
	e.rex(true, b, a)
	e.byte(0x85)
	e.modrm(3, b.low(), a.low())
}

// StoreMem8 emits mov [base], r8 for a single byte. base must be a plain
// base register (rsi/rdi/...; rsp would need a SIB byte this helper does
// not emit). Only legacy low-byte sources (al/cl/dl/bl) are used, so no
// REX prefix is required.
func (e *Emitter) StoreMem8(base, src Reg) {
	e.byte(0x88)
	e.modrm(0, src.low(), base.low())
}

// rel32 records a 4-byte fixup resolving to label at Finish time.
func (e *Emitter) rel32(label string) {
	e.patches = append(e.patches, patch{off: len(e.code), label: label})
	e.u32(0)
}

// imm64Patch emits mov r64, <placeholder> and returns the offset of the 8
// immediate bytes for later PatchImm64. Used for addresses (e.g. .rodata
// VAs) that are only known after text emission completes.
func (e *Emitter) imm64Patch(r Reg) int {
	e.rex(true, 0, r)
	e.byte(0xB8 + r.low())
	pos := len(e.code)
	e.u64(0)
	return pos
}

// PatchImm64 resolves a placeholder recorded by imm64Patch.
func (e *Emitter) PatchImm64(pos int, v uint64) {
	e.code[pos] = byte(v)
	e.code[pos+1] = byte(v >> 8)
	e.code[pos+2] = byte(v >> 16)
	e.code[pos+3] = byte(v >> 24)
	e.code[pos+4] = byte(v >> 32)
	e.code[pos+5] = byte(v >> 40)
	e.code[pos+6] = byte(v >> 48)
	e.code[pos+7] = byte(v >> 56)
}

// Call emits call rel32: E8 cd.
func (e *Emitter) Call(label string) {
	e.byte(0xE8)
	e.rel32(label)
}

// CallReg emits call m64 (FF /2 with a memory ModRM): an indirect call
// through the address STORED at [r].
//
// Phase-149 root-cause fix: this previously emitted FF D0 (modrm 11 010
// 000), which is call r64 — a call to the address IN the register. For
// the IAT sequence the register holds the SLOT address, so every import
// call jumped into the slot bytes themselves (fault RIP == slot address
// in every run) instead of the resolved address stored there. Memory
// indirect (mod != 11) is required; REX.R stays clear because /2 is a
// fixed opcode extension, with REX.B only for extended base regs.
func (e *Emitter) CallReg(r Reg) {
	e.rex(false, 0, r)
	e.byte(0xFF)
	switch {
	case r.low() == 4:
		e.modrm(0, 2, 4)
		e.byte(0x20 | r.low())
	case r.low() == 5:
		e.modrm(1, 2, r.low())
		e.byte(0)
	default:
		e.modrm(0, 2, r.low())
	}
}

// Jmp emits jmp rel32: E9 cd.
func (e *Emitter) Jmp(label string) {
	e.byte(0xE9)
	e.rel32(label)
}

// Jnz emits jnz rel32: 0F 85 cd.
func (e *Emitter) Jnz(label string) {
	e.byte(0x0F)
	e.byte(0x85)
	e.rel32(label)
}

// Phase 148: condition-code jumps (near form only, uniform with Jnz).
// CmpRegReg emits cmp r64, r64: REX.W + 39 /r (compares dst against src).
func (e *Emitter) CmpRegReg(dst, src Reg) {
	e.rex(true, src, dst)
	e.byte(0x39)
	e.modrm(3, src.low(), dst.low())
}

// Jz emits jz rel32: 0F 84 cd.
func (e *Emitter) Jz(label string) {
	e.byte(0x0F)
	e.byte(0x84)
	e.rel32(label)
}

// Jl emits jl rel32: 0F 8C cd (signed less-than).
func (e *Emitter) Jl(label string) {
	e.byte(0x0F)
	e.byte(0x8C)
	e.rel32(label)
}

// Jle emits jle rel32: 0F 8E cd.
func (e *Emitter) Jle(label string) {
	e.byte(0x0F)
	e.byte(0x8E)
	e.rel32(label)
}

// Jg emits jg rel32: 0F 8F cd.
func (e *Emitter) Jg(label string) {
	e.byte(0x0F)
	e.byte(0x8F)
	e.rel32(label)
}

// Jge emits jge rel32: 0F 8D cd.
func (e *Emitter) Jge(label string) {
	e.byte(0x0F)
	e.byte(0x8D)
	e.rel32(label)
}

// Jns emits jns rel32 (jump if sign flag clear): 0F 89 cd.
func (e *Emitter) Jns(label string) {
	e.byte(0x0F)
	e.byte(0x89)
	e.rel32(label)
}

// Ret emits ret: C3.
func (e *Emitter) Ret() { e.byte(0xC3) }

// AndRspNeg16 emits and rsp, -16: REX.W + 83 /4 F0. Used once at the
// Windows _start so entry alignment is guaranteed regardless of what
// the loader provides (Win64 calls fault on misaligned stacks).
func (e *Emitter) AndRspNeg16() {
	e.rex(true, RSP, RSP)
	e.byte(0x83)
	e.modrm(3, 4, RSP.low())
	e.byte(0xF0)
}

// Phase 149: loader-independent bootstrap primitives (PEB walk + export
// resolve for the Windows kernel32 boundary). All golden-pinned below.

// MovRegGsMem emits mov r64, gs:[disp32]: 0x65 + REX.W + 8B /r + SIB
// 0x25 + disp32. Used once: rax = PEB (gs:[0x60] on x86-64 Windows).
func (e *Emitter) MovRegGsMem(dst Reg, disp uint32) {
	e.byte(0x65)
	e.rex(true, dst, RSP)
	e.byte(0x8B)
	e.modrm(0, dst.low(), 4)
	e.byte(0x25)
	e.u32(disp)
}

// MovzxRegMem16 emits movzx r32, word [base+off]: 0F B7 /r + ModRM +
// SIB + disp (no REX.W: 32-bit destination). Used to read UNICODE_STRING
// Length fields and WCHARs while matching DLL/export names.
func (e *Emitter) MovzxRegMem16(dst, base Reg, off int) {
	e.rex(false, dst, base)
	e.byte(0x0F)
	e.byte(0xB7)
	switch {
	case off == 0 && base.low() != 5:
		e.modrm(0, dst.low(), 4)
	case off >= -128 && off <= 127:
		e.modrm(1, dst.low(), 4)
	default:
		e.modrm(2, dst.low(), 4)
	}
	e.byte(0x20 | base.low())
	switch {
	case off == 0 && base.low() != 5:
	case off >= -128 && off <= 127:
		e.byte(byte(int8(off)))
	default:
		e.u32(uint32(int32(off)))
	}
}

// CmpRegImm32 emits cmp r64, imm32: REX.W + 81 /7 io.
func (e *Emitter) CmpRegImm32(r Reg, v uint32) {
	e.rex(true, r, r)
	e.byte(0x81)
	e.modrm(3, 7, r.low())
	e.u32(v)
}

// StoreBaseOff emits mov [base+off], r64: REX.W + 89 /r + ModRM + SIB +
// disp. Used to publish resolved kernel32 addresses into the IAT slots.
func (e *Emitter) StoreBaseOff(src, base Reg, off int) {
	e.rex(true, src, base)
	e.byte(0x89)
	switch {
	case off == 0 && base.low() != 5:
		e.modrm(0, src.low(), 4)
	case off >= -128 && off <= 127:
		e.modrm(1, src.low(), 4)
	default:
		e.modrm(2, src.low(), 4)
	}
	e.byte(0x20 | base.low())
	switch {
	case off == 0 && base.low() != 5:
	case off >= -128 && off <= 127:
		e.byte(byte(int8(off)))
	default:
		e.u32(uint32(int32(off)))
	}
}

// Phase 148: frame-addressing primitives for the extras-pointer ABI
// (arguments past the six register units travel in a caller-frame array
// whose address reaches the callee in R10).

// LeaRegStack emits lea r64, [rsp+off]: REX.W + 8D /r.
func (e *Emitter) LeaRegStack(dst Reg, off int) {
	e.rex(true, dst, RSP)
	e.byte(0x8D)
	e.memRsp(dst, off)
}

// LoadBaseOff emits mov r64, [base+off] for an arbitrary base register:
// REX.W + 8B /r + ModRM + SIB + disp.
func (e *Emitter) LoadBaseOff(dst, base Reg, off int) {
	e.rex(true, dst, base)
	e.byte(0x8B)
	switch {
	case off == 0 && base.low() != 5:
		e.modrm(0, dst.low(), 4)
	case off >= -128 && off <= 127:
		e.modrm(1, dst.low(), 4)
	default:
		e.modrm(2, dst.low(), 4)
	}
	e.byte(0x20 | base.low())
	switch {
	case off == 0 && base.low() != 5:
	case off >= -128 && off <= 127:
		e.byte(byte(int8(off)))
	default:
		e.u32(uint32(int32(off)))
	}
}

// Syscall emits syscall: 0F 05.
func (e *Emitter) Syscall() {
	e.byte(0x0F)
	e.byte(0x05)
}

// Phase 149: loader-independent bootstrap memory forms.

// LoadBaseOff32 emits mov r32, [base+off] (zero-extending): 8B /r +
// ModRM + SIB + disp with no REX.W (REX.B only for extended regs).
// Used for u32 fields (PE headers, export directory, name RVAs).
func (e *Emitter) LoadBaseOff32(dst, base Reg, off int) {
	e.rex(false, dst, base)
	e.byte(0x8B)
	switch {
	case off == 0 && base.low() != 5:
		e.modrm(0, dst.low(), 4)
	case off >= -128 && off <= 127:
		e.modrm(1, dst.low(), 4)
	default:
		e.modrm(2, dst.low(), 4)
	}
	e.byte(0x20 | base.low())
	switch {
	case off == 0 && base.low() != 5:
	case off >= -128 && off <= 127:
		e.byte(byte(int8(off)))
	default:
		e.u32(uint32(int32(off)))
	}
}

// LoadScaled32 emits mov r32, [base+index*scale+disp]: 8B /r + ModRM +
// SIB(scale,index,base) + disp. Used for the export functions table
// ([funcsPtr + ordinal*4]).
func (e *Emitter) LoadScaled32(dst, base, index Reg, scale int, disp int) {
	var sc byte
	switch scale {
	case 1:
		sc = 0
	case 2:
		sc = 1
	case 4:
		sc = 2
	case 8:
		sc = 3
	default:
		panic("native backend: bad scale (want 1, 2, 4 or 8)")
	}
	r := byte(0x40)
	if dst.ext() {
		r |= 0x04 // REX.R extends the modrm reg field (dst here)
	}
	if index.ext() {
		r |= 0x02 // REX.X extends the SIB index field
	}
	if base.ext() {
		r |= 0x01 // REX.B extends the SIB base field
	}
	if r != 0x40 {
		e.byte(r)
	}
	e.byte(0x8B)
	if disp == 0 && base.low() != 5 {
		e.modrm(0, dst.low(), 4)
	} else if disp >= -128 && disp <= 127 {
		e.modrm(1, dst.low(), 4)
	} else {
		e.modrm(2, dst.low(), 4)
	}
	e.byte(sc<<6 | index.low()<<3 | base.low())
	if disp == 0 && base.low() != 5 {
	} else if disp >= -128 && disp <= 127 {
		e.byte(byte(int8(disp)))
	} else {
		e.u32(uint32(int32(disp)))
	}
}

// StoreAbs64Placeholder emits mov [imm64], rax (48 A3 + 8 placeholder
// bytes) and returns the offset of the immediate for link-time
// resolution. Used once per kernel32 import: the bootstrap publishes
// the resolved address straight into the IAT slot.
func (e *Emitter) StoreAbs64Placeholder() int {
	e.rex(true, RAX, RAX)
	e.byte(0xA3)
	pos := len(e.code)
	e.u64(0)
	return pos
}

// Int3 emits int3 (CC): a loud breakpoint fault. The bootstrap uses it
// for the impossible path (a kernel32 export missing on a real host) so
// it can never fail silently into wrong-code execution.
func (e *Emitter) Int3() {
	e.byte(0xCC)
}

// OrRegImm8 emits or r64, imm8: REX.W + 83 /1 ib (REX.B only for
// r8-r15; REX.R stays clear because the /1 lives in the fixed opcode
// extension, not a register field). The bootstrap folds WCHARs with
// 0x20 before comparing DLL names (BaseDllName arrives uppercase,
// e.g. KERNEL32.DLL; folding is a no-op for digits/dots).
func (e *Emitter) OrRegImm8(r Reg, v byte) {
	e.rex(true, RAX, r)
	e.byte(0x83)
	e.modrm(3, 1, r.low())
	e.byte(v)
}

// Phase 150A: SSE2 scalar double-precision float operations.

// MovXmmRegGp emits movq xmm, r64: 66 + REX.W + 0F 6E /r.
func (e *Emitter) MovXmmRegGp(dst XmmReg, src Reg) {
	e.byte(0x66)
	e.rex(true, Reg(dst), src)
	e.byte(0x0F)
	e.byte(0x6E)
	e.modrm(3, dst.low(), src.low())
}

// MovGpRegXmm emits movq r64, xmm: 66 + REX.W + 0F 7E /r.
func (e *Emitter) MovGpRegXmm(dst Reg, src XmmReg) {
	e.byte(0x66)
	e.rex(true, Reg(src), dst)
	e.byte(0x0F)
	e.byte(0x7E)
	e.modrm(3, src.low(), dst.low())
}

// AddsdXmmXmm emits addsd xmm1, xmm2: F2 + (REX) + 0F 58 /r.
func (e *Emitter) AddsdXmmXmm(dst, src XmmReg) {
	e.byte(0xF2)
	e.rex(false, Reg(dst), Reg(src))
	e.byte(0x0F)
	e.byte(0x58)
	e.modrm(3, dst.low(), src.low())
}

// SubsdXmmXmm emits subsd xmm1, xmm2: F2 + (REX) + 0F 5C /r.
func (e *Emitter) SubsdXmmXmm(dst, src XmmReg) {
	e.byte(0xF2)
	e.rex(false, Reg(dst), Reg(src))
	e.byte(0x0F)
	e.byte(0x5C)
	e.modrm(3, dst.low(), src.low())
}

// MulsdXmmXmm emits mulsd xmm1, xmm2: F2 + (REX) + 0F 59 /r.
func (e *Emitter) MulsdXmmXmm(dst, src XmmReg) {
	e.byte(0xF2)
	e.rex(false, Reg(dst), Reg(src))
	e.byte(0x0F)
	e.byte(0x59)
	e.modrm(3, dst.low(), src.low())
}

// DivsdXmmXmm emits divsd xmm1, xmm2: F2 + (REX) + 0F 5E /r.
func (e *Emitter) DivsdXmmXmm(dst, src XmmReg) {
	e.byte(0xF2)
	e.rex(false, Reg(dst), Reg(src))
	e.byte(0x0F)
	e.byte(0x5E)
	e.modrm(3, dst.low(), src.low())
}

// UcomisdXmmXmm emits ucomisd xmm1, xmm2: 66 + (REX) + 0F 2E /r.
func (e *Emitter) UcomisdXmmXmm(dst, src XmmReg) {
	e.byte(0x66)
	e.rex(false, Reg(dst), Reg(src))
	e.byte(0x0F)
	e.byte(0x2E)
	e.modrm(3, dst.low(), src.low())
}

// Cvtsi2sdXmmGp emits cvtsi2sd xmm, r64: F2 + REX.W + 0F 2A /r.
func (e *Emitter) Cvtsi2sdXmmGp(dst XmmReg, src Reg) {
	e.byte(0xF2)
	e.rex(true, Reg(dst), src)
	e.byte(0x0F)
	e.byte(0x2A)
	e.modrm(3, dst.low(), src.low())
}

// Cvttsd2siGpXmm emits cvttsd2si r64, xmm: F2 + REX.W + 0F 2C /r.
func (e *Emitter) Cvttsd2siGpXmm(dst Reg, src XmmReg) {
	e.byte(0xF2)
	e.rex(true, dst, Reg(src))
	e.byte(0x0F)
	e.byte(0x2C)
	e.modrm(3, dst.low(), src.low())
}

// XorpdXmmXmm emits xorpd xmm1, xmm2: 66 + (REX) + 0F 57 /r.
func (e *Emitter) XorpdXmmXmm(dst, src XmmReg) {
	e.byte(0x66)
	e.rex(false, Reg(dst), Reg(src))
	e.byte(0x0F)
	e.byte(0x57)
	e.modrm(3, dst.low(), src.low())
}

// Ja emits ja rel32: 0F 87 cd (jump if above / CF=0 and ZF=0).
func (e *Emitter) Ja(label string) {
	e.byte(0x0F)
	e.byte(0x87)
	e.rel32(label)
}

// Jae emits jae rel32: 0F 83 cd (jump if above or equal / CF=0).
func (e *Emitter) Jae(label string) {
	e.byte(0x0F)
	e.byte(0x83)
	e.rel32(label)
}

// Jb emits jb rel32: 0F 82 cd (jump if below / CF=1).
func (e *Emitter) Jb(label string) {
	e.byte(0x0F)
	e.byte(0x82)
	e.rel32(label)
}

// Jbe emits jbe rel32: 0F 86 cd (jump if below or equal / CF=1 or ZF=1).
func (e *Emitter) Jbe(label string) {
	e.byte(0x0F)
	e.byte(0x86)
	e.rel32(label)
}

// Jp emits jp rel32: 0F 8A cd (jump if parity / PF=1, used for unordered in ucomisd).
func (e *Emitter) Jp(label string) {
	e.byte(0x0F)
	e.byte(0x8A)
	e.rel32(label)
}

// Phase 150A: integer helpers print_float needs to decompose an IEEE-754
// pattern (sign / exponent / mantissa) and to walk decimal digits.

// AndRegReg emits and r64, r64: REX.W + 21 /r.
func (e *Emitter) AndRegReg(dst, src Reg) {
	e.rex(true, src, dst)
	e.byte(0x21)
	e.modrm(3, src.low(), dst.low())
}

// ShrRegImm emits shr r64, imm8: REX.W + C1 /5 ib.
func (e *Emitter) ShrRegImm(r Reg, imm byte) {
	e.rex(true, 0, r)
	e.byte(0xC1)
	e.modrm(3, 5, r.low())
	e.byte(imm)
}

// ShlRegImm emits shl r64, imm8: REX.W + C1 /4 ib.
func (e *Emitter) ShlRegImm(r Reg, imm byte) {
	e.rex(true, 0, r)
	e.byte(0xC1)
	e.modrm(3, 4, r.low())
	e.byte(imm)
}

// StoreMem8Off emits mov byte [base+off], r8: REX + 88 /r + ModRM + SIB +
// disp8. Like StoreBaseOff it always emits a SIB byte so rsp bases (the
// print_float frame) address correctly; off must fit in a signed byte.
func (e *Emitter) StoreMem8Off(src, base Reg, off int) {
	r := byte(0x40)
	if src.ext() {
		r |= 0x04
	}
	if base.ext() {
		r |= 0x01
	}
	if r != 0x40 {
		e.byte(r)
	}
	e.byte(0x88)
	e.modrm(1, src.low(), 4)
	e.byte(0x20 | base.low())
	e.byte(byte(int8(off)))
}

// MovMemDwordImm emits mov r/m64, imm32 (REX.W + C7 /0 + ModRM + SIB +
// disp8 + imm32): the 32-bit immediate is sign-extended, which is exactly
// right for packed little-endian ASCII (the high bytes are zero). off must
// fit in a signed byte.
func (e *Emitter) MovMemDwordImm(base Reg, off int, v uint32) {
	e.byte(0x48) // REX.W
	e.byte(0xC7)
	e.modrm(1, 0, 4)
	e.byte(0x20 | base.low())
	e.byte(byte(int8(off)))
	e.u32(v)
}

// Phase 150A: exactly bounded decimal conversion needs a few additional
// integer forms. Each is hand-checked against the Intel SDM and pinned
// below. They are narrow, purpose-built helpers for print_float's fixed
// 20-limb scratch area, not a general bignum ISA.

// AdcRegReg emits adc r64, r64: REX.W + 11 /r. Adds src plus CF to dst.
func (e *Emitter) AdcRegReg(dst, src Reg) {
	e.rex(true, src, dst)
	e.byte(0x11)
	e.modrm(3, src.low(), dst.low())
}

// SbbRegReg emits sbb r64, r64: REX.W + 19 /r. Subtracts src plus CF from dst.
func (e *Emitter) SbbRegReg(dst, src Reg) {
	e.rex(true, src, dst)
	e.byte(0x19)
	e.modrm(3, src.low(), dst.low())
}

// OrRegReg emits or r64, r64: REX.W + 09 /r. Used for sticky-bit accumulation.
func (e *Emitter) OrRegReg(dst, src Reg) {
	e.rex(true, src, dst)
	e.byte(0x09)
	e.modrm(3, src.low(), dst.low())
}

// BsrReg emits bsr r64, r/m64: REX.W + 0F BD /r. The source must be nonzero;
// a zero source leaves the destination undefined.
func (e *Emitter) BsrReg(dst, src Reg) {
	e.rex(true, dst, src)
	e.byte(0x0F)
	e.byte(0xBD)
	e.modrm(3, dst.low(), src.low())
}

// RolRegImm emits rol r64, imm8: REX.W + C1 /0 ib.
func (e *Emitter) RolRegImm(r Reg, imm byte) {
	e.rex(true, 0, r)
	e.byte(0xC1)
	e.modrm(3, 0, r.low())
	e.byte(imm)
}

// MovzxRegMem8 emits movzx r64, byte [base+off]: REX.W + 0F B6 /r. The SIB
// form matches LoadBaseOff so rsp- and register-based frame pointers both
// address correctly.
func (e *Emitter) MovzxRegMem8(dst, base Reg, off int) {
	e.rex(true, dst, base)
	e.byte(0x0F)
	e.byte(0xB6)
	switch {
	case off == 0 && base.low() != 5:
		e.modrm(0, dst.low(), 4)
	case off >= -128 && off <= 127:
		e.modrm(1, dst.low(), 4)
	default:
		e.modrm(2, dst.low(), 4)
	}
	e.byte(0x20 | base.low())
	switch {
	case off == 0 && base.low() != 5:
	case off >= -128 && off <= 127:
		e.byte(byte(int8(off)))
	default:
		e.u32(uint32(int32(off)))
	}
}

// scaledSIB emits [base+index*scale+disp] addressing for a 64-bit move.
// opcode selects 0x8B (load) or 0x89 (store). rsp cannot be an index
// register; callers use a separate frame pointer for limb arrays.
func (e *Emitter) scaledSIB(dst Reg, opcode byte, base, index Reg, scale int, disp int) {
	if index.low() == 4 {
		panic("native backend: rsp cannot be a SIB index register")
	}
	r := byte(0x48)
	if dst.ext() {
		r |= 0x04
	}
	if index.ext() {
		r |= 0x02
	}
	if base.ext() {
		r |= 0x01
	}
	e.byte(r)
	e.byte(opcode)
	e.sibTail(dst.low(), base, index, scale, disp)
}

// sibTail emits the ModRM+SIB+disp tail shared by every SIB-addressed form.
// split from scaledSIB so the 8-bit and two-byte-opcode variants (movzx
// 0F B6, mov 88) can reuse the register/displacement encoding without
// re-deriving the REX bits.
func (e *Emitter) sibTail(reg byte, base, index Reg, scale int, disp int) {
	var sc byte
	switch scale {
	case 1:
		sc = 0
	case 2:
		sc = 1
	case 4:
		sc = 2
	case 8:
		sc = 3
	default:
		panic("native backend: bad scale (want 1, 2, 4 or 8)")
	}
	if index.low() == 4 {
		panic("native backend: rsp cannot be a SIB index register")
	}
	if disp == 0 && base.low() != 5 {
		e.modrm(0, reg, 4)
	} else if disp >= -128 && disp <= 127 {
		e.modrm(1, reg, 4)
	} else {
		e.modrm(2, reg, 4)
	}
	e.byte(sc<<6 | index.low()<<3 | base.low())
	if disp == 0 && base.low() != 5 {
	} else if disp >= -128 && disp <= 127 {
		e.byte(byte(int8(disp)))
	} else {
		e.u32(uint32(int32(disp)))
	}
}

// IncReg emits inc r64 (REX.W + FF /0): the byte-copy index step.
func (e *Emitter) IncReg(r Reg) {
	e.rex(true, 0, r)
	e.byte(0xFF)
	e.modrm(0, 0, r.low())
}

// LoadScaled8 emits movzx r64, byte [base+index*scale+disp]: REX.W + 0F B6
// /r + SIB. Phase 150B: the byte-wise copy behind string concatenation,
// where the length is a runtime value so a constant-offset load cannot work.
func (e *Emitter) LoadScaled8(dst, base, index Reg, scale int, disp int) {
	e.rex(true, dst, base)
	e.byte(0x0F)
	e.byte(0xB6)
	e.sibTail(dst.low(), base, index, scale, disp)
}

// StoreScaled8 emits mov [base+index*scale+disp], r8: REX + 88 /r + SIB.
// The mirror of LoadScaled8, for the destination half of the same copy.
func (e *Emitter) StoreScaled8(src, base, index Reg, scale int, disp int) {
	e.rex(false, src, base)
	e.byte(0x88)
	e.sibTail(src.low(), base, index, scale, disp)
}

// LoadScaled64 emits mov r64, [base+index*scale+disp]: REX.W + 8B /r.
func (e *Emitter) LoadScaled64(dst, base, index Reg, scale int, disp int) {
	e.scaledSIB(dst, 0x8B, base, index, scale, disp)
}

// StoreScaled64 emits mov [base+index*scale+disp], r64: REX.W + 89 /r.
func (e *Emitter) StoreScaled64(src, base, index Reg, scale int, disp int) {
	e.scaledSIB(src, 0x89, base, index, scale, disp)
}

// CmpMemReg emits cmp [base+off], r64: REX.W + 39 /r + ModRM + SIB +
// disp. Compares frame memory against a register without loading it,
// so for-in bounds checks read the header length in place (one
// instruction instead of load + cmp) and never disturb RAX/RCX.
func (e *Emitter) CmpMemReg(base Reg, off int, src Reg) {
	e.rex(true, src, base)
	e.byte(0x39)
	switch {
	case off == 0 && base.low() != 5:
		e.modrm(0, src.low(), 4)
	case off >= -128 && off <= 127:
		e.modrm(1, src.low(), 4)
	default:
		e.modrm(2, src.low(), 4)
	}
	e.byte(0x20 | base.low())
	switch {
	case off == 0 && base.low() != 5:
	case off >= -128 && off <= 127:
		e.byte(byte(int8(off)))
	default:
		e.u32(uint32(int32(off)))
	}
}

// IncMem emits inc qword [base+off]: REX.W + FF /0 + ModRM + SIB +
// disp. The for-in index slot increments in place — no load/add/store
// round trip, and RAX stays free for the element load below.
func (e *Emitter) IncMem(base Reg, off int) {
	e.rex(true, 0, base)
	e.byte(0xFF)
	switch {
	case off == 0 && base.low() != 5:
		e.modrm(0, 0, 4)
	case off >= -128 && off <= 127:
		e.modrm(1, 0, 4)
	default:
		e.modrm(2, 0, 4)
	}
	e.byte(0x20 | base.low())
	switch {
	case off == 0 && base.low() != 5:
	case off >= -128 && off <= 127:
		e.byte(byte(int8(off)))
	default:
		e.u32(uint32(int32(off)))
	}
}

