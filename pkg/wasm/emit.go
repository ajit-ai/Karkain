package wasm

// WASM instruction opcodes used by the backend.
const (
	opUnreachable  = 0x00
	opNop          = 0x01
	opBlock        = 0x02
	opLoop         = 0x03
	opIf           = 0x04
	opElse         = 0x05
	opBr           = 0x0c
	opBrIf         = 0x0d
	opReturn       = 0x0f
	opCall         = 0x10
	opDrop         = 0x1a
	opLocalGet     = 0x20
	opLocalSet     = 0x21
	opLocalTee     = 0x22
	opGlobalGet    = 0x23
	opGlobalSet    = 0x24
	opI32Load      = 0x28
	opI64Load      = 0x29
	opI32Load8U    = 0x2d
	opI32Store     = 0x36
	opI64Store     = 0x37
	opI32Store8    = 0x3a
	opMemorySize   = 0x3f
	opMemoryGrow   = 0x40
	opI32Const     = 0x41
	opI64Const     = 0x42
	opI32Eqz       = 0x45
	opI32Eq        = 0x46
	opI32Ne        = 0x47
	opI32LtS       = 0x48
	opI32GtS       = 0x4a
	opI32LeS       = 0x4c
	opI32GeS       = 0x4e
	opI32LtU       = 0x49
	opI32GtU       = 0x4b
	opI32LeU       = 0x4d
	opI32GeU       = 0x4f
	opI64Eqz       = 0x50
	opI64Eq        = 0x51
	opI64Ne        = 0x52
	opI64LtS       = 0x53
	opI64GtS       = 0x55
	opI64LeS       = 0x57
	opI64GeS       = 0x59
	opI64Add       = 0x7c
	opI64Sub       = 0x7d
	opI64Mul       = 0x7e
	opI64DivS      = 0x7f
	opI64RemS      = 0x81
	opI32WrapI64   = 0xa7
	opI64ExtendI32S = 0xac
	opI64ExtendI32U = 0xad
	opI64Shl       = 0x86
	opI64ShrS      = 0x87
	opI32And       = 0x71
	opI32Or        = 0x72
	opI64Xor       = 0x85
	opI64And       = 0x83
	opI64Or        = 0x84
	opI32Add       = 0x6a
	opI32Sub       = 0x6b
	opI32ShrS      = 0x75
	opI32ShrU      = 0x76
	opEnd          = 0x0b
)

// Emitter accumulates instruction bytes with structured control-flow framing.
type Emitter struct {
	buf []byte
	// frame stack: labels for br (loop = frame with label, block/if labelled too)
	labels []int
	// depth of if frames without else yet, to validate else/end matching
	ifStack []bool
	// ParamCount is the number of i64 parameter locals (indices 0..n-1).
	ParamCount int
	// ExtraLocals are additional i64 local slots allocated during emission.
	ExtraLocals []ValueType
}

func NewEmitter() *Emitter { return &Emitter{} }

func (e *Emitter) Bytes() []byte { return e.buf }

// --- leaf instructions ---

func (e *Emitter) Unreachable() { e.buf = append(e.buf, opUnreachable) }
func (e *Emitter) Nop()         { e.buf = append(e.buf, opNop) }
func (e *Emitter) Return()      { e.buf = append(e.buf, opReturn) }
func (e *Emitter) Drop()        { e.buf = append(e.buf, opDrop) }
func (e *Emitter) End()         { e.buf = append(e.buf, opEnd) }

func (e *Emitter) LocalGet(idx int) { e.buf = append(e.buf, opLocalGet); e.buf = appendUleb(e.buf, uint64(idx)) }
func (e *Emitter) LocalSet(idx int) { e.buf = append(e.buf, opLocalSet); e.buf = appendUleb(e.buf, uint64(idx)) }
func (e *Emitter) LocalTee(idx int) { e.buf = append(e.buf, opLocalTee); e.buf = appendUleb(e.buf, uint64(idx)) }
func (e *Emitter) GlobalGet(idx int) { e.buf = append(e.buf, opGlobalGet); e.buf = appendUleb(e.buf, uint64(idx)) }
func (e *Emitter) GlobalSet(idx int) { e.buf = append(e.buf, opGlobalSet); e.buf = appendUleb(e.buf, uint64(idx)) }

func (e *Emitter) I32Const(v int) { e.buf = append(e.buf, opI32Const); e.buf = appendSleb(e.buf, int64(v)) }
func (e *Emitter) I64Const(v int64) { e.buf = append(e.buf, opI64Const); e.buf = appendSleb(e.buf, v) }

func (e *Emitter) I32Eqz() { e.buf = append(e.buf, opI32Eqz) }
func (e *Emitter) I32Eq()  { e.buf = append(e.buf, opI32Eq) }
func (e *Emitter) I32Ne()  { e.buf = append(e.buf, opI32Ne) }
func (e *Emitter) I32LtS() { e.buf = append(e.buf, opI32LtS) }
func (e *Emitter) I32GtS() { e.buf = append(e.buf, opI32GtS) }
func (e *Emitter) I32LeS() { e.buf = append(e.buf, opI32LeS) }
func (e *Emitter) I32GeS() { e.buf = append(e.buf, opI32GeS) }
func (e *Emitter) I32LtU() { e.buf = append(e.buf, opI32LtU) }
func (e *Emitter) I32GtU() { e.buf = append(e.buf, opI32GtU) }
func (e *Emitter) I32LeU() { e.buf = append(e.buf, opI32LeU) }
func (e *Emitter) I32GeU() { e.buf = append(e.buf, opI32GeU) }
func (e *Emitter) I32ShrU() { e.buf = append(e.buf, opI32ShrU) }
func (e *Emitter) I32And() { e.buf = append(e.buf, opI32And) }
func (e *Emitter) I32Or()  { e.buf = append(e.buf, opI32Or) }
func (e *Emitter) I32Add() { e.buf = append(e.buf, opI32Add) }
func (e *Emitter) I32Sub() { e.buf = append(e.buf, opI32Sub) }

func (e *Emitter) I64Eqz() { e.buf = append(e.buf, opI64Eqz) }
func (e *Emitter) I64Eq()  { e.buf = append(e.buf, opI64Eq) }
func (e *Emitter) I64Ne()  { e.buf = append(e.buf, opI64Ne) }
func (e *Emitter) I64LtS() { e.buf = append(e.buf, opI64LtS) }
func (e *Emitter) I64GtS() { e.buf = append(e.buf, opI64GtS) }
func (e *Emitter) I64LeS() { e.buf = append(e.buf, opI64LeS) }
func (e *Emitter) I64GeS() { e.buf = append(e.buf, opI64GeS) }
func (e *Emitter) I64Add() { e.buf = append(e.buf, opI64Add) }
func (e *Emitter) I64Sub() { e.buf = append(e.buf, opI64Sub) }
func (e *Emitter) I64Mul() { e.buf = append(e.buf, opI64Mul) }
func (e *Emitter) I64DivS() { e.buf = append(e.buf, opI64DivS) }
func (e *Emitter) I64RemS() { e.buf = append(e.buf, opI64RemS) }
func (e *Emitter) I64Shl()  { e.buf = append(e.buf, opI64Shl) }
func (e *Emitter) I64ShrS() { e.buf = append(e.buf, opI64ShrS) }
func (e *Emitter) I64Xor()  { e.buf = append(e.buf, opI64Xor) }
func (e *Emitter) I64And()  { e.buf = append(e.buf, opI64And) }
func (e *Emitter) I64Or()   { e.buf = append(e.buf, opI64Or) }

func (e *Emitter) I32WrapI64()    { e.buf = append(e.buf, opI32WrapI64) }
func (e *Emitter) I64ExtendI32S() { e.buf = append(e.buf, opI64ExtendI32S) }
func (e *Emitter) I64ExtendI32U() { e.buf = append(e.buf, opI64ExtendI32U) }

func (e *Emitter) I32Load(align, offset int) { e.buf = append(e.buf, opI32Load); e.memArg(align, offset) }
func (e *Emitter) I64Load(align, offset int) { e.buf = append(e.buf, opI64Load); e.memArg(align, offset) }
func (e *Emitter) I32Store(align, offset int) { e.buf = append(e.buf, opI32Store); e.memArg(align, offset) }
func (e *Emitter) I64Store(align, offset int) { e.buf = append(e.buf, opI64Store); e.memArg(align, offset) }
func (e *Emitter) I32Load8U(offset int)      { e.buf = append(e.buf, opI32Load8U); e.memArg(1, offset) }
func (e *Emitter) I32Store8(offset int)      { e.buf = append(e.buf, opI32Store8); e.memArg(1, offset) }

// memArg writes the memarg (align log2, offset uleb).
func (e *Emitter) memArg(align, offset int) {
	l2 := 0
	for (1 << (l2 + 1)) <= align && l2 < 31 {
		l2++
	}
	e.buf = appendUleb(e.buf, uint64(l2))
	e.buf = appendUleb(e.buf, uint64(offset))
}

func (e *Emitter) MemorySize() { e.buf = append(e.buf, opMemorySize, 0x00) }
func (e *Emitter) MemoryGrow() { e.buf = append(e.buf, opMemoryGrow, 0x00) }

func (e *Emitter) Call(funcIdx int) { e.buf = append(e.buf, opCall); e.buf = appendUleb(e.buf, uint64(funcIdx)) }

func (e *Emitter) Br(label int)    { e.buf = append(e.buf, opBr); e.buf = appendUleb(e.buf, uint64(label)) }
func (e *Emitter) BrIf(label int)  { e.buf = append(e.buf, opBrIf); e.buf = appendUleb(e.buf, uint64(label)) }

// --- structured control flow ---
// Labels are depths; a fresh frame starts at depth 0 relative to its own scope.

type blockType struct {
	// result type; empty (0x40) for param-less/no-result blocks
	val ValueType
	has  bool
}

func (e *Emitter) frameDepth() int { return len(e.labels) }

// BeginBlock pushes a labelled block frame.
func (e *Emitter) BeginBlock(result ValueType) {
	e.buf = append(e.buf, opBlock)
	e.emitBlockType(result)
	e.labels = append(e.labels, len(e.labels))
	e.ifStack = append(e.ifStack, false)
}

// BeginLoop pushes a loop frame (continue target).
func (e *Emitter) BeginLoop(result ValueType) {
	e.buf = append(e.buf, opLoop)
	e.emitBlockType(result)
	e.labels = append(e.labels, len(e.labels))
	e.ifStack = append(e.ifStack, false)
}

// BeginIf emits `if blocktype`. The condition must already be on the stack.
func (e *Emitter) BeginIf(result ValueType) {
	e.buf = append(e.buf, opIf)
	e.emitBlockType(result)
	e.labels = append(e.labels, len(e.labels))
	e.ifStack = append(e.ifStack, true)
}

// Else emits the else marker for the innermost if frame.
func (e *Emitter) Else() {
	e.buf = append(e.buf, opElse)
}

// Close emits `end` for the innermost frame with structural checking.
func (e *Emitter) Close() {
	e.buf = append(e.buf, opEnd)
	if len(e.labels) == 0 {
		panic("wasm emitter: Close with no open frames")
	}
	e.labels = e.labels[:len(e.labels)-1]
	e.ifStack = e.ifStack[:len(e.ifStack)-1]
}

// ifOpen reports whether the current frame is an if (so Else may be emitted).
func (e *Emitter) ifOpen() bool {
	if len(e.ifStack) == 0 {
		return false
	}
	return e.ifStack[len(e.ifStack)-1]
}

func (e *Emitter) emitBlockType(result ValueType) {
	if result == I64 {
		e.buf = append(e.buf, byte(I64))
	} else if result == I32 {
		e.buf = append(e.buf, byte(I32))
	} else {
		e.buf = append(e.buf, 0x40) // empty block type
	}
}