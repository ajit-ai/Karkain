package native

import (
	"bytes"
	"testing"
)

// Phase 145A: golden machine-code pins for every encoder primitive. These
// are hand-verified against the Intel SDM (vol. 2) — a wrong byte here is
// a wrong program everywhere, so each encoding is asserted exactly.

func hexOf(t *testing.T, build func(e *Emitter)) []byte {
	t.Helper()
	e := NewEmitter()
	build(e)
	return e.Bytes()
}

func want(t *testing.T, name string, got []byte, want ...byte) {
	t.Helper()
	if !bytes.Equal(got, want) {
		t.Errorf("%s = % x, want % x", name, got, want)
	}
}

func TestEmitMov(t *testing.T) {
	want(t, "mov rax,1", hexOf(t, func(e *Emitter) { e.MovRegImm32(RAX, 1) }), 0xB8, 0x01, 0x00, 0x00, 0x00)
	want(t, "mov r8,1", hexOf(t, func(e *Emitter) { e.MovRegImm32(R8, 1) }), 0x41, 0xB8, 0x01, 0x00, 0x00, 0x00)
	want(t, "mov rax,imm64", hexOf(t, func(e *Emitter) { e.MovRegImm64(RAX, 0x1122334455667788) }),
		0x48, 0xB8, 0x88, 0x77, 0x66, 0x55, 0x44, 0x33, 0x22, 0x11)
	want(t, "mov rax,rbx", hexOf(t, func(e *Emitter) { e.MovRegReg(RAX, RBX) }), 0x48, 0x89, 0xD8)
	want(t, "mov r9,r8", hexOf(t, func(e *Emitter) { e.MovRegReg(R9, R8) }), 0x4D, 0x89, 0xC1)
	want(t, "mov rax,[rsp]", hexOf(t, func(e *Emitter) { e.LoadStack(RAX, 0) }), 0x48, 0x8B, 0x04, 0x24)
	want(t, "mov rax,[rsp+16]", hexOf(t, func(e *Emitter) { e.LoadStack(RAX, 16) }), 0x48, 0x8B, 0x44, 0x24, 0x10)
	want(t, "mov rax,[rsp+300]", hexOf(t, func(e *Emitter) { e.LoadStack(RAX, 300) }),
		0x48, 0x8B, 0x84, 0x24, 0x2C, 0x01, 0x00, 0x00)
	want(t, "mov [rsp+8],rcx", hexOf(t, func(e *Emitter) { e.StoreStack(RCX, 8) }), 0x48, 0x89, 0x4C, 0x24, 0x08)
	want(t, "mov [rsi],dl", hexOf(t, func(e *Emitter) { e.StoreMem8(RSI, RDX) }), 0x88, 0x16)
}

func TestEmitArith(t *testing.T) {
	want(t, "add rax,rbx", hexOf(t, func(e *Emitter) { e.AddRegReg(RAX, RBX) }), 0x48, 0x01, 0xD8)
	want(t, "sub r10,r11", hexOf(t, func(e *Emitter) { e.SubRegReg(R10, R11) }), 0x4D, 0x29, 0xDA)
	want(t, "imul rcx,rdx", hexOf(t, func(e *Emitter) { e.MulRegReg(RCX, RDX) }), 0x48, 0x0F, 0xAF, 0xCA)
	want(t, "add rax,5", hexOf(t, func(e *Emitter) { e.AddRegImm32(RAX, 5) }), 0x48, 0x83, 0xC0, 0x05)
	want(t, "add rax,1000", hexOf(t, func(e *Emitter) { e.AddRegImm32(RAX, 1000) }),
		0x48, 0x81, 0xC0, 0xE8, 0x03, 0x00, 0x00)
	want(t, "sub rsp,40", hexOf(t, func(e *Emitter) { e.SubRsp(40) }), 0x48, 0x83, 0xEC, 0x28)
	want(t, "xor rax,rax", hexOf(t, func(e *Emitter) { e.XorRegReg(RAX) }), 0x48, 0x31, 0xC0)
	want(t, "neg rax", hexOf(t, func(e *Emitter) { e.NegReg(RAX) }), 0x48, 0xF7, 0xD8)
	want(t, "dec rsi", hexOf(t, func(e *Emitter) { e.DecReg(RSI) }), 0x48, 0xFF, 0xCE)
	want(t, "cqo", hexOf(t, func(e *Emitter) { e.Cqo() }), 0x48, 0x99)
	want(t, "div rcx", hexOf(t, func(e *Emitter) { e.DivReg(RCX) }), 0x48, 0xF7, 0xF1)
	want(t, "test rax,rax", hexOf(t, func(e *Emitter) { e.TestRegReg(RAX, RAX) }), 0x48, 0x85, 0xC0)
	want(t, "push rbx", hexOf(t, func(e *Emitter) { e.PushReg(RBX) }), 0x53)
	want(t, "pop r15", hexOf(t, func(e *Emitter) { e.PopReg(R15) }), 0x41, 0x5F)
}

func TestEmitControl(t *testing.T) {
	e := NewEmitter()
	e.Call("fn")
	e.Jmp("end")
	e.Mark("fn")
	e.Ret()
	e.Mark("end")
	e.Jnz("fn")
	e.Jns("end")
	e.Syscall()
	got := e.Bytes()
	// Layout: call@0(5) jmp@5(5) fn@10:ret end@11:jnz(6) jns(6) syscall(2).
	// rel = target - disp_end (RIP of the following instruction):
	// call fn: 10-5 = 5; jmp end: 11-10 = 1;
	// jnz fn: 10-17 = -7; jns end: 11-23 = -12.
	want(t, "control", got,
		0xE8, 0x05, 0x00, 0x00, 0x00,
		0xE9, 0x01, 0x00, 0x00, 0x00,
		0xC3,
		0x0F, 0x85, 0xF9, 0xFF, 0xFF, 0xFF,
		0x0F, 0x89, 0xF4, 0xFF, 0xFF, 0xFF,
		0x0F, 0x05)
}

// Phase 148: golden pins for cmp + the signed condition-code jumps.
// cmp rax,rcx is REX.W + 39 C8; each jcc is the 0F 8x near form with a
// zero displacement (self-target, so rel = 0 - 6 = -6 = F AF FF FF).
func TestEmitCondJumps(t *testing.T) {
	want(t, "cmp rax,rcx", hexOf(t, func(e *Emitter) { e.CmpRegReg(RAX, RCX) }), 0x48, 0x39, 0xC8)
	want(t, "cmp r9,r8", hexOf(t, func(e *Emitter) { e.CmpRegReg(R9, R8) }), 0x4D, 0x39, 0xC1)
	back := func(build func(e *Emitter)) []byte {
		e := NewEmitter()
		e.Mark("here")
		build(e)
		return e.Bytes()
	}
	want(t, "jz", back(func(e *Emitter) { e.Jz("here") }), 0x0F, 0x84, 0xFA, 0xFF, 0xFF, 0xFF)
	want(t, "jl", back(func(e *Emitter) { e.Jl("here") }), 0x0F, 0x8C, 0xFA, 0xFF, 0xFF, 0xFF)
	want(t, "jle", back(func(e *Emitter) { e.Jle("here") }), 0x0F, 0x8E, 0xFA, 0xFF, 0xFF, 0xFF)
	want(t, "jg", back(func(e *Emitter) { e.Jg("here") }), 0x0F, 0x8F, 0xFA, 0xFF, 0xFF, 0xFF)
	want(t, "jge", back(func(e *Emitter) { e.Jge("here") }), 0x0F, 0x8D, 0xFA, 0xFF, 0xFF, 0xFF)
}
