package wasm

import (
	"fmt"
)

// Karkain runtime value model in WASM memory.
//
// A Value is an i64:
//   - bit 0 == 0: unboxed integer; the value is v >> 1 (matches make_int semantics).
//   - bit 0 == 1: boxed heap cell pointer | 1; the cell lives in linear memory.
//
// Heap cell layout (8-byte aligned):
//   +0  i32 type  (TYPE_INT=0, TYPE_FLOAT64=1, TYPE_STRING=2, TYPE_ARRAY=3,
//                  TYPE_MAP=4, TYPE_BOOL=5 — same order as the C runtime)
//   +4  i32 length (string: byte length; array: element count; bool: 0/1)
//   +8  data (string bytes, or array elements stored as i64 value slots)
//
// This mirrors the tagged C `Value` semantics so native/WASM parity holds for
// the supported program subset.

const (
	typeInt    = 0
	typeFloat  = 1
	typeString = 2
	typeArray  = 3
	typeMap    = 4
	typeBool   = 5
)

// Fixed linear-memory layout (all offsets are bytes).
const (
	scratchNwritten   = 0    // 4 bytes: fd_write result slot
	scratchIOVec      = 8    // 8 bytes: iovs {ptr, len}
	scratchDigits     = 16   // 128 bytes: decimal conversion scratch
	scratchDigitsEnd  = 144
	offWasiArgc       = 160  // 4 bytes: argc cell written by args_sizes_get
	offWasiArgvBufLen = 164  // 4 bytes: argv buffer size cell
	staticBase        = 0x200
	poolBase          = 0x230
	heapBase          = 0x10000
	heapGlobal        = 0    // global index
	wasiArgcGlobal    = 1    // global index: argc gathered at _start
	wasiArgvGlobal    = 2    // global index: argv pointer array gathered at _start
)

// Fixed static string offsets inside the static region.
const (
	offNewline    = staticBase + 0x00
	offTrue       = staticBase + 0x01
	offFalse      = staticBase + 0x05
	offLBrack     = staticBase + 0x0a
	offRBrack     = staticBase + 0x0b
	offSep        = staticBase + 0x0c
	offQuote      = staticBase + 0x0e
	offErrPrefix  = staticBase + 0x0f
	offErrAt      = staticBase + 0x1e
	offErrColon   = staticBase + 0x22
)

func staticBytes() []byte {
	sz := poolBase - staticBase
	b := make([]byte, sz)
	put := func(field []byte, off int) {
		copy(b[off:], field)
	}
	put([]byte{'\n'}, 0x00)
	put([]byte("true"), 0x01)
	put([]byte("false"), 0x05)
	put([]byte{'['}, 0x0a)
	put([]byte{']'}, 0x0b)
	put([]byte(", "), 0x0c)
	put([]byte{'"'}, 0x0e)
	put([]byte("runtime error: "), 0x0f)
	put([]byte(" at "), 0x1e)
	put([]byte{':'}, 0x22)
	return b
}

// RuntimeFunc holds the function indices of the embedded runtime helpers.
type RuntimeFunc struct {
	FDWrite     int
	ArgsSizesGet int
	ArgsGet     int
	ProcExit    int
	Alloc       int
	WriteRaw    int
	WriteI64    int
	PrintValue  int
	RTError     int
	Add         int
	Sub         int
	Mul         int
	Div         int
	Mod         int
	Eq          int
	Ne          int
	Lt          int
	Gt          int
	Le          int
	Ge          int
	IsTruthy    int
	Not         int
	Neg         int
	Len         int
	Get         int
	Set         int
	MakeString  int
	MakeArray   int
	GetArgs     int
	Start       int
	UserBase    int
}

// runtimeConsts returns the fixed runtime function indices. The four imports
// (fd_write, args_sizes_get, args_get, proc_exit) always occupy indices 0-3;
// module-defined runtime bodies follow from index 4, then user functions.
func runtimeConsts() RuntimeFunc {
	return RuntimeFunc{
		FDWrite:     0,
		ArgsSizesGet: 1,
		ArgsGet:     2,
		ProcExit:    3,
		Alloc:       4,
		WriteRaw:    5,
		WriteI64:    6,
		PrintValue:  7,
		RTError:     8,
		Add:         9,
		Sub:         10,
		Mul:         11,
		Div:         12,
		Mod:         13,
		Eq:          14,
		Ne:          15,
		Lt:          16,
		Gt:          17,
		Le:          18,
		Ge:          19,
		IsTruthy:    20,
		Not:         21,
		Neg:         22,
		Len:         23,
		Get:         24,
		Set:         25,
		MakeString:  26,
		MakeArray:   27,
		GetArgs:     28,
		Start:       29,
		UserBase:    30,
	}
}

// kindOffsets fixes the interned offsets for runtime error kinds + source file.
type kindOffsets struct {
	div0Kind  int
	div0Len   int
	mod0Kind  int
	mod0Len   int
	strIdx    int
	strIdxLen int
	arrIdx    int
	arrIdxLen int
	filePtr   int
	fileLen   int
}

// addRuntime adds the runtime function bodies and the fd_write import to the
// module builder, returning the fixed indices after the import.
func addRuntime(mb *ModuleBuilder, st *statics, srcFileBase string) (RuntimeFunc, kindOffsets) {
	rt := runtimeConsts()

	// Type section entries. Imported function types are added first; the
	// import order fixes the function indices 0-3 (fd_write, args_sizes_get,
	// args_get, proc_exit) which runtimeConsts depends on.
	tFdWrite := mb.AddType(FuncType{Params: []ValueType{I32, I32, I32, I32}, Results: []ValueType{I32}})
	tArgsSizesGet := mb.AddType(FuncType{Params: []ValueType{I32, I32}, Results: []ValueType{I32}})
	tArgsGet := mb.AddType(FuncType{Params: []ValueType{I32, I32}, Results: []ValueType{I32}})
	tProcExit := mb.AddType(FuncType{Params: []ValueType{I32}})
	mb.Imports = append(mb.Imports,
		Import{
			Module: "wasi_snapshot_preview1", Field: "fd_write",
			Kind: ImportFunc, TypeIdx: tFdWrite,
		},
		Import{
			Module: "wasi_snapshot_preview1", Field: "args_sizes_get",
			Kind: ImportFunc, TypeIdx: tArgsSizesGet,
		},
		Import{
			Module: "wasi_snapshot_preview1", Field: "args_get",
			Kind: ImportFunc, TypeIdx: tArgsGet,
		},
		Import{
			Module: "wasi_snapshot_preview1", Field: "proc_exit",
			Kind: ImportFunc, TypeIdx: tProcExit,
		},
	)

	addType := func(ft FuncType) int { return mb.AddType(ft) }

	// 1 alloc (i32)->i32
	tAlloc := addType(FuncType{Params: []ValueType{I32}, Results: []ValueType{I32}})
	// 2 write_raw (i32,i32,i32)->()
	tWriteRaw := addType(FuncType{Params: []ValueType{I32, I32, I32}})
	// 3 write_i64 (i32,i64)->()
	tWriteI64 := addType(FuncType{Params: []ValueType{I32, I64}})
	// 4 print_value (i64)->()
	tPrintV := addType(FuncType{Params: []ValueType{I64}})
	// 5 rt_error (i32,i32,i32,i32,i32)->()
	tRTErr := addType(FuncType{Params: []ValueType{I32, I32, I32, I32, I32}})
	// 6 arith (i64,i64)->i64
	tArith := addType(FuncType{Params: []ValueType{I64, I64}, Results: []ValueType{I64}})
	// 7 checked (i64,i64,i32,i32,i32)->i64
	tChecked := addType(FuncType{Params: []ValueType{I64, I64, I32, I32, I32}, Results: []ValueType{I64}})
	// 8 set (i64,i64,i64,i32,i32,i32)->i64
	tSet := addType(FuncType{Params: []ValueType{I64, I64, I64, I32, I32, I32}, Results: []ValueType{I64}})
	// 9 is_truthy (i64)->i32
	tTruthy := addType(FuncType{Params: []ValueType{I64}, Results: []ValueType{I32}})
	// 10 one (i64)->i64
	tOne := addType(FuncType{Params: []ValueType{I64}, Results: []ValueType{I64}})
	// 11 mk_string (i32,i32)->i64
	tMkStr := addType(FuncType{Params: []ValueType{I32, I32}, Results: []ValueType{I64}})
	// 12 mk_array (i32)->i64
	tMkArr := addType(FuncType{Params: []ValueType{I32}, Results: []ValueType{I64}})
	// 13 start ()->()
	tStart := addType(FuncType{})
	_ = tStart

	ko := kindOffsets{
		div0Kind:  st.intern("integer division by zero"),
		mod0Kind:  st.intern("integer modulo by zero"),
		strIdx:    st.intern("string index out of range"),
		arrIdx:    st.intern("array index out of range"),
		filePtr:   st.intern(srcFileBase),
	}
	ko.div0Len = len("integer division by zero")
	ko.mod0Len = len("integer modulo by zero")
	ko.strIdxLen = len("string index out of range")
	ko.arrIdxLen = len("array index out of range")
	ko.fileLen = len(srcFileBase)

	// --- alloc ---
	mb.FuncTypes = append(mb.FuncTypes, tAlloc)
	mb.addCode("alloc", runtimeLocals("alloc"), func(e *Emitter) {
		e.LocalGet(p0)
		e.I32Const(7)
		e.I32Add()
		e.I32Const(-8)
		e.I32And()
		e.LocalSet(p1)
		e.GlobalGet(heapGlobal)
		e.LocalSet(p2)
		e.LocalGet(p2)
		e.LocalGet(p1)
		e.I32Add()
		e.LocalSet(p3)
		e.I32Const(65535)
		e.LocalGet(p3)
		e.I32Add()
		e.I32Const(16)
		e.I32ShrU()
		e.LocalSet(p4)
		e.MemorySize()
		e.LocalSet(p5)
		e.LocalGet(p4)
		e.LocalGet(p5)
		e.I32GtU()
		e.BeginIf(noResult)
		e.MemorySize()
		e.LocalSet(p4)
		e.LocalGet(p4)
		e.LocalGet(p5)
		e.I32Sub()
		e.MemoryGrow()
		e.I32Const(-1)
		e.I32Eq()
		e.BeginIf(noResult)
		e.Unreachable()
		e.Close()
		e.Close()
		e.GlobalGet(heapGlobal)
		e.LocalGet(p1)
		e.I32Add()
		e.GlobalSet(heapGlobal)
		e.LocalGet(p2)
	})

	// --- write_raw (fd, ptr, len) -> () ---
	mb.FuncTypes = append(mb.FuncTypes, tWriteRaw)
	mb.addCode("write_raw", runtimeLocals("write_raw"), func(e *Emitter) {
		e.I32Const(scratchIOVec)
		e.LocalGet(p1)
		e.I32Store(4, 0)
		e.I32Const(scratchIOVec + 4)
		e.LocalGet(p2)
		e.I32Store(4, 0)
		e.LocalGet(p0)
		e.I32Const(scratchIOVec)
		e.I32Const(1)
		e.I32Const(scratchNwritten)
		e.Call(rt.FDWrite)
		e.Drop()
	})

	// --- write_i64 (fd, val) -> () ---
	mb.FuncTypes = append(mb.FuncTypes, tWriteI64)
	mb.addCode("write_i64", runtimeLocals("write_i64"), func(e *Emitter) {
		e.LocalGet(p1)
		e.I64Eqz()
		e.BeginIf(noResult)
		e.I32Const(scratchDigits)
		e.I32Const(48)
		e.I32Store8(0)
		e.LocalGet(p0)
		e.I32Const(scratchDigits)
		e.I32Const(1)
		e.Call(rt.WriteRaw)
		e.Return()
		e.Close()
		// neg = v < 0
		e.LocalGet(p1)
		e.I64Const(0)
		e.I64LtS()
		e.LocalSet(p2)
		e.LocalGet(p1)
		e.LocalSet(p3) // u = v
		e.LocalGet(p2)
		e.BeginIf(noResult)
		e.I64Const(0)
		e.LocalGet(p3)
		e.I64Sub()
		e.LocalSet(p3)
		e.Close()
		e.I32Const(scratchDigitsEnd)
		e.LocalSet(p4) // pos
		// while u != 0
		e.BeginBlock(noResult)
		e.BeginLoop(noResult)
		e.LocalGet(p3)
		e.I64Eqz()
		e.BrIf(1)
		e.LocalGet(p4)
		e.I32Const(1)
		e.I32Sub()
		e.LocalTee(p4)
		e.LocalGet(p3)
		e.I64Const(10)
		e.I64RemS()
		e.I64Const(48)
		e.I64Add()
		e.I32WrapI64()
		e.I32Store8(0)
		e.LocalGet(p3)
		e.I64Const(10)
		e.I64DivS()
		e.LocalSet(p3)
		e.Br(0)
		e.Close()
		e.Close()
		// sign
		e.LocalGet(p2)
		e.BeginIf(noResult)
		e.LocalGet(p4)
		e.I32Const(1)
		e.I32Sub()
		e.LocalTee(p4)
		e.I32Const(45)
		e.I32Store8(0)
		e.Close()
		// len = end - pos
		e.LocalGet(p0)
		e.LocalGet(p4)
		e.I32Const(scratchDigitsEnd)
		e.LocalGet(p4)
		e.I32Sub()
		e.Call(rt.WriteRaw)
	})

	// --- print_value (v i64) -> () ---
	mb.FuncTypes = append(mb.FuncTypes, tPrintV)
	mb.addCode("print_value", runtimeLocals("print_value"), func(e *Emitter) {
		// unboxed int
		e.LocalGet(p0)
		e.I64Const(1)
		e.I64And()
		e.I64Eqz()
		e.BeginIf(noResult)
		e.I32Const(1)
		e.LocalGet(p0)
		e.I64Const(1)
		e.I64ShrS()
		e.Call(rt.WriteI64)
		e.I32Const(1)
		e.I32Const(offNewline)
		e.I32Const(1)
		e.Call(rt.WriteRaw)
		e.Return()
		e.Close()
		// boxed
		e.LocalGet(p0)
		e.I64Const(1)
		e.I64ShrS()
		e.I32WrapI64()
		e.LocalSet(p1) // ptr
		// string
		e.LocalGet(p1)
		e.I32Load(4, 0)
		e.I32Const(typeString)
		e.I32Eq()
		e.BeginIf(noResult)
		e.I32Const(1)
		e.LocalGet(p1)
		e.I32Const(8)
		e.I32Add()
		e.LocalGet(p1)
		e.I32Load(4, 4)
		e.Call(rt.WriteRaw)
		e.I32Const(1)
		e.I32Const(offNewline)
		e.I32Const(1)
		e.Call(rt.WriteRaw)
		e.Return()
		e.Close()
		// array
		e.LocalGet(p1)
		e.I32Load(4, 0)
		e.I32Const(typeArray)
		e.I32Eq()
		e.BeginIf(noResult)
		e.LocalGet(p1)
		e.I32Load(4, 4)
		e.LocalSet(p2) // len
		e.I32Const(1)
		e.I32Const(offLBrack)
		e.I32Const(1)
		e.Call(rt.WriteRaw)
		e.I32Const(0)
		e.LocalSet(p3) // i
		e.BeginBlock(noResult)
		e.BeginLoop(noResult)
		e.LocalGet(p3)
		e.LocalGet(p2)
		e.I32GeS()
		e.BrIf(1)
		e.LocalGet(p3)
		e.I32Const(0)
		e.I32GtS()
		e.BeginIf(noResult)
		e.I32Const(1)
		e.I32Const(offSep)
		e.I32Const(2)
		e.Call(rt.WriteRaw)
		e.Close()
		// addr = ptr+8+i*8
		e.LocalGet(p1)
		e.I64ExtendI32U()
		e.I64Const(8)
		e.I64Add()
		e.LocalGet(p3)
		e.I64ExtendI32U()
		e.I64Const(8)
		e.I64Mul()
		e.I64Add()
		e.I32WrapI64()
		e.LocalSet(p4) // addr
		// item
		e.LocalGet(p4)
		e.I64Load(8, 0)
		e.LocalSet(p5) // item
		e.LocalGet(p5)
		e.I64Const(1)
		e.I64And()
		e.I64Eqz()
		e.BeginIf(noResult)
		e.I32Const(1)
		e.LocalGet(p5)
		e.I64Const(1)
		e.I64ShrS()
		e.Call(rt.WriteI64)
		e.Else()
		e.LocalGet(p5)
		e.I64Const(1)
		e.I64ShrS()
		e.I32WrapI64()
		e.LocalSet(p6) // itemPtr
		e.LocalGet(p6)
		e.I32Load(4, 0)
		e.I32Const(typeString)
		e.I32Eq()
		e.BeginIf(noResult)
		e.I32Const(1)
		e.I32Const(offQuote)
		e.I32Const(1)
		e.Call(rt.WriteRaw)
		e.I32Const(1)
		e.LocalGet(p6)
		e.I32Const(8)
		e.I32Add()
		e.LocalGet(p6)
		e.I32Load(4, 4)
		e.Call(rt.WriteRaw)
		e.I32Const(1)
		e.I32Const(offQuote)
		e.I32Const(1)
		e.Call(rt.WriteRaw)
		e.Close()
		e.Close()
		e.LocalGet(p3)
		e.I32Const(1)
		e.I32Add()
		e.LocalSet(p3)
		e.Br(0)
		e.Close()
		e.Close()
		e.I32Const(1)
		e.I32Const(offRBrack)
		e.I32Const(1)
		e.Call(rt.WriteRaw)
		e.I32Const(1)
		e.I32Const(offNewline)
		e.I32Const(1)
		e.Call(rt.WriteRaw)
		e.Return()
		e.Close()
		// bool
		e.LocalGet(p1)
		e.I32Load(4, 0)
		e.I32Const(typeBool)
		e.I32Eq()
		e.BeginIf(noResult)
		e.LocalGet(p1)
		e.I32Load(4, 4)
		e.LocalSet(p2)
		e.LocalGet(p2)
		e.BeginIf(noResult)
		e.I32Const(1)
		e.I32Const(offTrue)
		e.I32Const(4)
		e.Call(rt.WriteRaw)
		e.Else()
		e.I32Const(1)
		e.I32Const(offFalse)
		e.I32Const(5)
		e.Call(rt.WriteRaw)
		e.Close()
		e.I32Const(1)
		e.I32Const(offNewline)
		e.I32Const(1)
		e.Call(rt.WriteRaw)
		e.Return()
		e.Close()
		// fallback
		e.I32Const(1)
		e.I32Const(offNewline)
		e.I32Const(1)
		e.Call(rt.WriteRaw)
	})

	// --- rt_error (kind, kindLen, file, fileLen, line) -> () ---
	mb.FuncTypes = append(mb.FuncTypes, tRTErr)
	mb.addCode("rt_error", runtimeLocals("rt_error"), func(e *Emitter) {
		e.I32Const(2)
		e.I32Const(offErrPrefix)
		e.I32Const(15)
		e.Call(rt.WriteRaw)
		e.I32Const(2)
		e.LocalGet(p0)
		e.LocalGet(p1)
		e.Call(rt.WriteRaw)
		e.I32Const(2)
		e.I32Const(offErrAt)
		e.I32Const(4)
		e.Call(rt.WriteRaw)
		e.I32Const(2)
		e.LocalGet(p2)
		e.LocalGet(p3)
		e.Call(rt.WriteRaw)
		e.I32Const(2)
		e.I32Const(offErrColon)
		e.I32Const(1)
		e.Call(rt.WriteRaw)
		e.I32Const(2)
		e.LocalGet(p4)
		e.I64ExtendI32S()
		e.Call(rt.WriteI64)
		e.I32Const(2)
		e.I32Const(offNewline)
		e.I32Const(1)
		e.Call(rt.WriteRaw)
		// Deterministic process exit code (native parity): a Karkain runtime
		// error terminates the program with exit status 1.
		e.I32Const(1)
		e.Call(rt.ProcExit)
	})

	// --- rt_add/rt_sub/rt_mul ---
	for _, nm := range []struct {
		name string
		op   func(*Emitter)
	}{{"rt_add", func(e *Emitter) { e.I64Add() }}, {"rt_sub", func(e *Emitter) { e.I64Sub() }}, {"rt_mul", func(e *Emitter) { e.I64Mul() }}} {
		mb.FuncTypes = append(mb.FuncTypes, tArith)
		name := nm.name
		mb.addCode(name, nil, func(e *Emitter) {
			e.LocalGet(p0)
			e.I64Const(1)
			e.I64ShrS()
			e.LocalGet(p1)
			e.I64Const(1)
			e.I64ShrS()
			nm.op(e)
			e.I64Const(1)
			e.I64Shl()
		})
	}

	// --- rt_div / rt_mod ---
	mb.FuncTypes = append(mb.FuncTypes, tChecked)
	mb.addCode("rt_div", runtimeLocals("rt_div"), func(e *Emitter) {
		e.LocalGet(p1)
		e.I64Const(1)
		e.I64ShrS()
		e.LocalSet(p5) // dv
		e.LocalGet(p5)
		e.I64Eqz()
		e.BeginIf(noResult)
		e.I32Const(ko.div0Kind)
		e.I32Const(ko.div0Len)
		e.I32Const(ko.filePtr)
		e.I32Const(ko.fileLen)
		e.LocalGet(p4)
		e.Call(rt.RTError)
		e.Close()
		e.LocalGet(p0)
		e.I64Const(1)
		e.I64ShrS()
		e.LocalGet(p5)
		e.I64DivS()
		e.I64Const(1)
		e.I64Shl()
	})
	mb.FuncTypes = append(mb.FuncTypes, tChecked)
	mb.addCode("rt_mod", runtimeLocals("rt_mod"), func(e *Emitter) {
		e.LocalGet(p1)
		e.I64Const(1)
		e.I64ShrS()
		e.LocalSet(p5)
		e.LocalGet(p5)
		e.I64Eqz()
		e.BeginIf(noResult)
		e.I32Const(ko.mod0Kind)
		e.I32Const(ko.mod0Len)
		e.I32Const(ko.filePtr)
		e.I32Const(ko.fileLen)
		e.LocalGet(p4)
		e.Call(rt.RTError)
		e.Close()
		e.LocalGet(p0)
		e.I64Const(1)
		e.I64ShrS()
		e.LocalGet(p5)
		e.I64RemS()
		e.I64Const(1)
		e.I64Shl()
	})

	// --- rt_eq / rt_ne (value equality: containers compare content) ---
	eqLocals := []ValueType{I32, I32, I32, I32, I32, I64}
	emitEq := func(e *Emitter) {
		// p2=cellA p3=cellB p4=tagA p5=n p6=i p7=acc
		e.I64Const(2)
		e.LocalSet(p7)
		// both unboxed?
		e.LocalGet(p0)
		e.I64Const(1)
		e.I64And()
		e.LocalGet(p1)
		e.I64Const(1)
		e.I64And()
		e.I64Or()
		e.I64Eqz()
		e.BeginIf(noResult)
		e.LocalGet(p0)
		e.LocalGet(p1)
		e.I64Eq()
		e.I64ExtendI32U()
		e.I64Const(1)
		e.I64Shl()
		e.LocalSet(p7)
		e.Else()
		// at least one boxed: both boxed?
		e.LocalGet(p0)
		e.I64Const(1)
		e.I64And()
		e.LocalGet(p1)
		e.I64Const(1)
		e.I64And()
		e.I64And()
		e.I64Eqz()
		e.BeginIf(noResult)
		e.I64Const(0)
		e.LocalSet(p7)
		e.Else()
		// deep compare
		e.LocalGet(p0)
		e.I64Const(1)
		e.I64ShrS()
		e.I32WrapI64()
		e.LocalSet(p2)
		e.LocalGet(p1)
		e.I64Const(1)
		e.I64ShrS()
		e.I32WrapI64()
		e.LocalSet(p3)
		e.LocalGet(p2)
		e.I32Load(4, 0)
		e.LocalSet(p4)
		e.LocalGet(p2)
		e.I32Load(4, 4)
		e.LocalSet(p5)
		// tag mismatch → 0
		e.LocalGet(p3)
		e.I32Load(4, 0)
		e.LocalGet(p4)
		e.I32Ne()
		e.BeginIf(noResult)
		e.I64Const(0)
		e.LocalSet(p7)
		e.Close()
		// unsupported tag → 0
		e.LocalGet(p4)
		e.I32Const(2)
		e.I32Ne()
		e.LocalGet(p4)
		e.I32Const(3)
		e.I32Ne()
		e.I32And()
		e.BeginIf(noResult)
		e.I64Const(0)
		e.LocalSet(p7)
		e.Close()
		// length mismatch → 0
		e.LocalGet(p3)
		e.I32Load(4, 4)
		e.LocalGet(p5)
		e.I32Ne()
		e.BeginIf(noResult)
		e.I64Const(0)
		e.LocalSet(p7)
		e.Close()
		// string: byte compare
		e.LocalGet(p4)
		e.I32Const(2)
		e.I32Eq()
		e.BeginIf(noResult)
		e.I32Const(0)
		e.LocalSet(p6)
		e.BeginBlock(noResult)
		e.BeginLoop(noResult)
		e.LocalGet(p6)
		e.LocalGet(p5)
		e.I32GeS()
		e.BrIf(1)
		e.LocalGet(p2)
		e.LocalGet(p6)
		e.I32Add()
		e.I32Const(8)
		e.I32Add()
		e.I32Load8U(0)
		e.LocalGet(p3)
		e.LocalGet(p6)
		e.I32Add()
		e.I32Const(8)
		e.I32Add()
		e.I32Load8U(0)
		e.I32Eq()
		e.I32Eqz()
		e.BeginIf(noResult)
		e.I64Const(0)
		e.LocalSet(p7)
		e.Close()
		e.LocalGet(p6)
		e.I32Const(1)
		e.I32Add()
		e.LocalSet(p6)
		e.Br(0)
		e.Close()
		e.Close()
		e.Else()
		// array: recursive element compare
		e.LocalGet(p4)
		e.I32Const(3)
		e.I32Eq()
		e.BeginIf(noResult)
		e.I32Const(0)
		e.LocalSet(p6)
		e.BeginBlock(noResult)
		e.BeginLoop(noResult)
		e.LocalGet(p6)
		e.LocalGet(p5)
		e.I32GeS()
		e.BrIf(1)
		e.LocalGet(p2)
		e.LocalGet(p6)
		e.I64ExtendI32U()
		e.I64Const(8)
		e.I64Mul()
		e.I32WrapI64()
		e.I32Add()
		e.I32Const(8)
		e.I32Add()
		e.I64Load(8, 0)
		e.LocalGet(p3)
		e.LocalGet(p6)
		e.I64ExtendI32U()
		e.I64Const(8)
		e.I64Mul()
		e.I32WrapI64()
		e.I32Add()
		e.I32Const(8)
		e.I32Add()
		e.I64Load(8, 0)
		e.Call(rt.Eq)
		e.I64Eqz()
		e.BeginIf(noResult)
		e.I64Const(0)
		e.LocalSet(p7)
		e.Close()
		e.LocalGet(p6)
		e.I32Const(1)
		e.I32Add()
		e.LocalSet(p6)
		e.Br(0)
		e.Close()
		e.Close()
		e.Close()
		e.Close()
		// both-boxed end
		e.Close()
		// both-unboxed / mixed end
		e.Close()
		e.LocalGet(p7)
	}
	mb.FuncTypes = append(mb.FuncTypes, tArith)
	mb.addCode("rt_eq", eqLocals, emitEq)
	mb.FuncTypes = append(mb.FuncTypes, tArith)
	mb.addCode("rt_ne", eqLocals, func(e *Emitter) {
		emitEq(e)
		e.I64Eqz()
		e.I64ExtendI32U()
		e.I64Const(1)
		e.I64Shl()
	})
	ops := []struct {
		name string
		op   func(*Emitter)
	}{
		{"rt_lt", func(e *Emitter) { e.I64LtS() }},
		{"rt_gt", func(e *Emitter) { e.I64GtS() }},
		{"rt_le", func(e *Emitter) { e.I64LeS() }},
		{"rt_ge", func(e *Emitter) { e.I64GeS() }},
	}
	for _, o := range ops {
		mb.FuncTypes = append(mb.FuncTypes, tArith)
		mb.addCode(o.name, nil, func(e *Emitter) {
			e.LocalGet(p0)
			e.I64Const(1)
			e.I64ShrS()
			e.LocalGet(p1)
			e.I64Const(1)
			e.I64ShrS()
			o.op(e)
			e.I64ExtendI32U()
			e.I64Const(1)
			e.I64Shl()
		})
	}

	// --- is_truthy (v)->i32 ---
	mb.FuncTypes = append(mb.FuncTypes, tTruthy)
	mb.addCode("is_truthy", runtimeLocals("is_truthy"), func(e *Emitter) {
		e.LocalGet(p0)
		e.I64Const(1)
		e.I64And()
		e.I64Eqz()
		e.BeginIf(noResult)
		e.LocalGet(p0)
		e.I64Eqz()
		e.I32Eqz()
		e.Return()
		e.Close()
		e.LocalGet(p0)
		e.I64Const(1)
		e.I64ShrS()
		e.I32WrapI64()
		e.LocalSet(p1)
		e.LocalGet(p1)
		e.I32Load(4, 0)
		e.LocalSet(p2)
		e.LocalGet(p2)
		e.I32Const(typeString)
		e.I32Eq()
		e.BeginIf(noResult)
		e.LocalGet(p1)
		e.I32Load(4, 4)
		e.I32Const(0)
		e.I32GtS()
		e.Return()
		e.Close()
		e.LocalGet(p2)
		e.I32Const(typeArray)
		e.I32Eq()
		e.BeginIf(noResult)
		e.LocalGet(p1)
		e.I32Load(4, 4)
		e.I32Const(0)
		e.I32GtS()
		e.Return()
		e.Close()
		e.LocalGet(p2)
		e.I32Const(typeBool)
		e.I32Eq()
		e.BeginIf(noResult)
		e.LocalGet(p1)
		e.I32Load(4, 4)
		e.I32Eqz()
		e.I32Eqz()
		e.Return()
		e.Close()
		e.I32Const(0)
	})

	// --- rt_not (v)->i64 ---
	mb.FuncTypes = append(mb.FuncTypes, tOne)
	mb.addCode("rt_not", runtimeLocals("rt_not"), func(e *Emitter) {
		e.LocalGet(p0)
		e.Call(rt.IsTruthy)
		e.I32Eqz()
		e.I64ExtendI32U()
		e.I64Const(1)
		e.I64Shl()
	})

	// --- rt_neg (v)->i64: -v for ints, else 0 ---
	mb.FuncTypes = append(mb.FuncTypes, tOne)
	mb.addCode("rt_neg", runtimeLocals("rt_neg"), func(e *Emitter) {
		e.LocalGet(p0)
		e.I64Const(1)
		e.I64ShrS()
		e.I64Const(0)
		e.I64Sub()
		e.I64Const(1)
		e.I64Shl()
	})

	// --- rt_len (v)->i64 ---
	mb.FuncTypes = append(mb.FuncTypes, tOne)
	mb.addCode("rt_len", runtimeLocals("rt_len"), func(e *Emitter) {
		e.LocalGet(p0)
		e.I64Const(1)
		e.I64And()
		e.I64Eqz()
		e.BeginIf(noResult)
		e.I64Const(0)
		e.Return()
		e.Close()
		e.LocalGet(p0)
		e.I64Const(1)
		e.I64ShrS()
		e.I32WrapI64()
		e.LocalSet(p1)
		e.LocalGet(p1)
		e.I32Load(4, 0)
		e.LocalSet(p2)
		e.LocalGet(p2)
		e.I32Const(typeString)
		e.I32Eq()
		e.BeginIf(noResult)
		e.LocalGet(p1)
		e.I32Load(4, 4)
		e.I64ExtendI32U()
		e.I64Const(1)
		e.I64Shl()
		e.Return()
		e.Close()
		e.LocalGet(p2)
		e.I32Const(typeArray)
		e.I32Eq()
		e.BeginIf(noResult)
		e.LocalGet(p1)
		e.I32Load(4, 4)
		e.I64ExtendI32U()
		e.I64Const(1)
		e.I64Shl()
		e.Return()
		e.Close()
		e.I64Const(0)
	})

	// --- rt_get (container, idx, filePtr, fileLen, line) -> i64 ---
	mb.FuncTypes = append(mb.FuncTypes, tChecked)
	mb.addCode("rt_get", runtimeLocals("rt_get"), func(e *Emitter) {
		e.LocalGet(p0)
		e.I64Const(1)
		e.I64And()
		e.I64Eqz()
		e.BeginIf(noResult)
		e.I64Const(0)
		e.Return()
		e.Close()
		e.LocalGet(p0)
		e.I64Const(1)
		e.I64ShrS()
		e.I32WrapI64()
		e.LocalSet(p5)
		e.LocalGet(p5)
		e.I32Load(4, 0)
		e.LocalSet(p6)
		// i = idx>>1 if unboxed else -1
		e.LocalGet(p1)
		e.I64Const(1)
		e.I64And()
		e.I64Eqz()
		e.BeginIf(I64)
		e.LocalGet(p1)
		e.I64Const(1)
		e.I64ShrS()
		e.Else()
		e.I64Const(-1)
		e.Close()
		e.LocalSet(p7)
		// string
		e.LocalGet(p6)
		e.I32Const(typeString)
		e.I32Eq()
		e.BeginIf(noResult)
		e.LocalGet(p5)
		e.I32Load(4, 4)
		e.I64ExtendI32U()
		e.LocalSet(p9) // n
		e.LocalGet(p7)
		e.I64Const(0)
		e.I64LtS()
		e.BeginIf(noResult)
		e.I32Const(ko.strIdx)
		e.I32Const(ko.strIdxLen)
		e.LocalGet(p2)
		e.LocalGet(p3)
		e.LocalGet(p4) // line
		e.Call(rt.RTError)
		e.Close()
		e.LocalGet(p7)
		e.LocalGet(p9)
		e.I64GeS()
		e.BeginIf(noResult)
		e.I32Const(ko.strIdx)
		e.I32Const(ko.strIdxLen)
		e.LocalGet(p2)
		e.LocalGet(p3)
		e.LocalGet(p4)
		e.Call(rt.RTError)
		e.Close()
		e.LocalGet(p5)
		e.I64ExtendI32U()
		e.I64Const(8)
		e.I64Add()
		e.LocalGet(p7)
		e.I64Add()
		e.I32WrapI64()
		e.I32Load8U(0)
		e.I64ExtendI32U()
		e.I64Const(1)
		e.I64Shl()
		e.Return()
		e.Close()
		// array
		e.LocalGet(p6)
		e.I32Const(typeArray)
		e.I32Eq()
		e.BeginIf(noResult)
		e.LocalGet(p5)
		e.I32Load(4, 4)
		e.I64ExtendI32U()
		e.LocalSet(p9)
		e.LocalGet(p7)
		e.I64Const(0)
		e.I64LtS()
		e.BeginIf(noResult)
		e.I32Const(ko.arrIdx)
		e.I32Const(ko.arrIdxLen)
		e.LocalGet(p2)
		e.LocalGet(p3)
		e.LocalGet(p4) // line
		e.Call(rt.RTError)
		e.Close()
		e.LocalGet(p7)
		e.LocalGet(p9)
		e.I64GeS()
		e.BeginIf(noResult)
		e.I32Const(ko.arrIdx)
		e.I32Const(ko.arrIdxLen)
		e.LocalGet(p2)
		e.LocalGet(p3)
		e.LocalGet(p4)
		e.Call(rt.RTError)
		e.Close()
		e.LocalGet(p5)
		e.I64ExtendI32U()
		e.I64Const(8)
		e.I64Add()
		e.LocalGet(p7)
		e.I64Const(8)
		e.I64Mul()
		e.I64Add()
		e.I32WrapI64()
		e.I64Load(8, 0)
		e.Return()
		e.Close()
		e.I64Const(0)
	})

	// --- rt_set (container, idx, val, filePtr, fileLen, line) -> i64 ---
	mb.FuncTypes = append(mb.FuncTypes, tSet)
	mb.addCode("rt_set", runtimeLocals("rt_set"), func(e *Emitter) {
		e.LocalGet(p0)
		e.I64Const(1)
		e.I64And()
		e.I64Eqz()
		e.BeginIf(noResult)
		e.I64Const(0)
		e.Return()
		e.Close()
		e.LocalGet(p0)
		e.I64Const(1)
		e.I64ShrS()
		e.I32WrapI64()
		e.LocalSet(p6)
		e.LocalGet(p6)
		e.I32Load(4, 0)
		e.I32Const(typeArray)
		e.I32Eq()
		e.I32Eqz()
		e.BeginIf(noResult)
		e.I64Const(0)
		e.Return()
		e.Close()
		e.LocalGet(p6)
		e.I32Load(4, 4)
		e.I64ExtendI32U()
		e.LocalSet(p8)
		e.LocalGet(p1)
		e.I64Const(1)
		e.I64And()
		e.I64Eqz()
		e.BeginIf(I64)
		e.LocalGet(p1)
		e.I64Const(1)
		e.I64ShrS()
		e.Else()
		e.I64Const(-1)
		e.Close()
		e.LocalSet(p7)
		e.LocalGet(p7)
		e.I64Const(0)
		e.I64LtS()
		e.BeginIf(noResult)
		e.I32Const(ko.arrIdx)
		e.I32Const(ko.arrIdxLen)
		e.LocalGet(p3)
		e.LocalGet(p4)
		e.LocalGet(p5) // line
		e.Call(rt.RTError)
		e.Close()
		e.LocalGet(p7)
		e.LocalGet(p8)
		e.I64GeS()
		e.BeginIf(noResult)
		e.I32Const(ko.arrIdx)
		e.I32Const(ko.arrIdxLen)
		e.LocalGet(p3)
		e.LocalGet(p4)
		e.LocalGet(p5)
		e.Call(rt.RTError)
		e.Close()
		e.LocalGet(p6)
		e.I64ExtendI32U()
		e.I64Const(8)
		e.I64Add()
		e.LocalGet(p7)
		e.I64Const(8)
		e.I64Mul()
		e.I64Add()
		e.I32WrapI64()
		e.LocalGet(p2)
		e.I64Store(8, 0)
		e.LocalGet(p0)
	})

	// --- mk_string (ptr, len) -> i64 ---
	mb.FuncTypes = append(mb.FuncTypes, tMkStr)
	mb.addCode("mk_string", runtimeLocals("mk_string"), func(e *Emitter) {
		e.I32Const(8)
		e.LocalGet(p1)
		e.I32Add()
		e.I32Const(7)
		e.I32Add()
		e.I32Const(-8)
		e.I32And()
		e.LocalSet(p2)
		e.LocalGet(p2)
		e.Call(rt.Alloc)
		e.LocalSet(p3)
		e.LocalGet(p3)
		e.I32Const(typeString)
		e.I32Store(4, 0)
		e.LocalGet(p3)
		e.I32Const(4)
		e.I32Add()
		e.LocalGet(p1)
		e.I32Store(4, 0)
		e.I32Const(0)
		e.LocalSet(p4)
		e.BeginBlock(noResult)
		e.BeginLoop(noResult)
		e.LocalGet(p4)
		e.LocalGet(p1)
		e.I32GeS()
		e.BrIf(1)
		e.LocalGet(p3)
		e.LocalGet(p4)
		e.I32Const(8)
		e.I32Add()
		e.I32Add()
		e.LocalGet(p0)
		e.LocalGet(p4)
		e.I32Add()
		e.I32Load8U(0)
		e.I32Store8(0)
		e.LocalGet(p4)
		e.I32Const(1)
		e.I32Add()
		e.LocalSet(p4)
		e.Br(0)
		e.Close()
		e.Close()
		e.LocalGet(p3)
		e.I64ExtendI32U()
		e.I64Const(1)
		e.I64Shl()
		e.I64Const(1)
		e.I64Or()
	})

	// --- mk_array (len) -> i64 ---
	mb.FuncTypes = append(mb.FuncTypes, tMkArr)
	mb.addCode("mk_array", runtimeLocals("mk_array"), func(e *Emitter) {
		e.I32Const(8)
		e.LocalGet(p0)
		e.I64ExtendI32U()
		e.I64Const(8)
		e.I64Mul()
		e.I32WrapI64()
		e.I32Add()
		e.I32Const(7)
		e.I32Add()
		e.I32Const(-8)
		e.I32And()
		e.LocalSet(p1)
		e.LocalGet(p1)
		e.Call(rt.Alloc)
		e.LocalSet(p2)
		e.LocalGet(p2)
		e.I32Const(typeArray)
		e.I32Store(4, 0)
		e.LocalGet(p2)
		e.I32Const(4)
		e.I32Add()
		e.LocalGet(p0)
		e.I32Store(4, 0)
		e.LocalGet(p2)
		e.I64ExtendI32U()
		e.I64Const(1)
		e.I64Shl()
		e.I64Const(1)
		e.I64Or()
	})

	// --- get_args () -> i64 ---
	// Materializes the Karkain argument array from the argc/argv gathered at
	// _start (stored in globals wasiArgcGlobal/wasiArgvGlobal). Each argument
	// becomes a boxed string; the result is the same array-of-strings Value the
	// native/C engine produces for getArgs().
	tGetArgs := addType(FuncType{Results: []ValueType{I64}})
	mb.FuncTypes = append(mb.FuncTypes, tGetArgs)
	mb.addCode("get_args", runtimeLocals("get_args"), func(e *Emitter) {
		// cell = mk_array(argc)
		e.GlobalGet(wasiArgcGlobal)
		e.Call(rt.MakeArray)
		e.LocalSet(p2) // p2 i64: array Value
		e.LocalGet(p2)
		e.I64Const(1)
		e.I64ShrS()
		e.I32WrapI64()
		e.LocalSet(p1) // p1 i32: array cell ptr
		e.I32Const(0)
		e.LocalSet(p0) // p0 i32: i
		e.BeginBlock(noResult)
		e.BeginLoop(noResult)
		e.LocalGet(p0)
		e.GlobalGet(wasiArgcGlobal)
		e.I32GeS()
		e.BrIf(1)
		// addr = cell + 8 + i*8
		e.LocalGet(p1)
		e.I32Const(8)
		e.I32Add()
		e.LocalGet(p0)
		e.I64ExtendI32U()
		e.I64Const(8)
		e.I64Mul()
		e.I32WrapI64()
		e.I32Add()
		e.LocalSet(p3)
		// str ptr = argv[i]
		e.GlobalGet(wasiArgvGlobal)
		e.LocalGet(p0)
		e.I64ExtendI32U()
		e.I64Const(4)
		e.I64Mul()
		e.I32WrapI64()
		e.I32Add()
		e.I32Load(4, 0)
		e.LocalSet(p4)
		// len = strlen(str)
		e.I32Const(0)
		e.LocalSet(p5)
		e.BeginBlock(noResult)
		e.BeginLoop(noResult)
		e.LocalGet(p4)
		e.LocalGet(p5)
		e.I32Add()
		e.I32Load8U(0)
		e.I32Eqz()
		e.BrIf(1)
		e.LocalGet(p5)
		e.I32Const(1)
		e.I32Add()
		e.LocalSet(p5)
		e.Br(0)
		e.Close()
		e.Close()
		// *(addr) = mk_string(str, len)
		e.LocalGet(p3)
		e.LocalGet(p4)
		e.LocalGet(p5)
		e.Call(rt.MakeString)
		e.I64Store(8, 0)
		e.LocalGet(p0)
		e.I32Const(1)
		e.I32Add()
		e.LocalSet(p0)
		e.Br(0)
		e.Close()
		e.Close()
		e.LocalGet(p2)
	})

	return rt, ko
}

// addCode appends a function body to the module with the given extra locals.
func (mb *ModuleBuilder) addCode(name string, locals []ValueType, emit func(*Emitter)) {
	e := NewEmitter()
	emit(e)
	e.End()
	mb.Codes = append(mb.Codes, Code{Body: e.Bytes(), Locals: locals})
}

// local indices used by the runtime assembly
const (
	p0 = 0
	p1 = 1
	p2 = 2
	p3 = 3
	p4 = 4
	p5 = 5
	p6 = 6
	p7 = 7
	p8 = 8
	p9 = 9
)

var noResult ValueType

// runtimeLocals declares the extra locals each runtime function requires,
// keyed by the names passed to addCode. Default: none.
var defaultLocals = map[string][]ValueType{}

func runtimeLocals(name string) []ValueType {
	switch name {
	case "alloc":
		return []ValueType{I32, I32, I32, I32, I32, I32}
	case "write_i64":
		return []ValueType{I32, I64, I32}
	case "print_value":
		return []ValueType{I32, I32, I32, I32, I64, I32}
	case "rt_div", "rt_mod":
		return []ValueType{I64}
	case "rt_get":
		return []ValueType{I32, I32, I64, I64, I64}
	case "rt_set":
		return []ValueType{I32, I64, I64, I64}
	case "mk_string":
		return []ValueType{I32, I32, I32, I32}
	case "mk_array":
		return []ValueType{I32, I32}
	case "get_args":
		// p0 i32: i, p1 i32: array cell, p2 i64: array Value, p3/p4/p5: scratch
		return []ValueType{I32, I32, I64, I32, I32, I32}
	case "is_truthy":
		return []ValueType{I32, I32}
	case "rt_len":
		return []ValueType{I32, I32}
	case "rt_eq", "rt_ne", "rt_lt", "rt_gt", "rt_le", "rt_ge":
		return nil
	}
	if l, ok := defaultLocals[name]; ok {
		return l
	}
	return nil
}

func (r RuntimeFunc) String() string {
	return fmt.Sprintf("fd_write=%d alloc=%d start=%d", r.FDWrite, r.Alloc, r.Start)
}