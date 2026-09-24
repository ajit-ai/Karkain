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
