package ir

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
)

// ============================================================
// Phase 35: Binary IR Bytecode Format & Serializer
// Karkain Module Bytecode (.kbc) Format
// ============================================================

// Magic bytes: "KRK\x01"
var magicBytes = [4]byte{0x4B, 0x52, 0x4B, 0x01}

const (
	BytecodeVersion uint32 = 1
	MaxStringLen    uint32 = 65535
)

// Section types
const (
	SectionHeader   uint8 = 0x01
	SectionSymbols  uint8 = 0x02
	SectionCode     uint8 = 0x03
	SectionData     uint8 = 0x04
	SectionDebug    uint8 = 0x05
	SectionQuantum  uint8 = 0x06
	SectionGPU      uint8 = 0x07
	SectionTensor   uint8 = 0x08
	SectionEOF      uint8 = 0xFF
)

// ============================================================
// Opcodes
// ============================================================

type Opcode uint8

// CPU Opcodes
const (
	OpNop Opcode = iota
	OpConstInt
	OpConstFloat
	OpConstBool
	OpConstString
	OpLoad
	OpStore
	OpAdd
	OpSub
	OpMul
	OpDiv
	OpMod
	OpNeg
	OpNot
	OpAnd
	OpOr
	OpEq
	OpNeq
	OpLt
	OpLe
	OpGt
	OpGe
	OpJump
	OpJumpIf
	OpJumpIfNot
	OpCall
	OpReturn
	OpPop
	OpDup
	OpPrint
	OpCast
	OpIndex
	OpField
	OpNewStruct
	OpNewArray
	OpArrayPush
	OpArrayLen
	OpMakeSlice
	OpAssert
)

// GPU Opcodes
const (
	OpGPUKernel Opcode = 0x40 + iota
	OpGPUBufferAlloc
	OpGPUBufferFree
	OpGPUBufferWrite
	OpGPUBufferRead
	OpGPUDispatch
	OpGPUSync
	OpGPUBuildKernel
	OpGPUSetArg
)

// Quantum Opcodes
const (
	OpQPUH Opcode = 0x80 + iota
	OpQPUX
	OpQPUY
	OpQPUZ
	OpQPUXor
	OpQPUYor
	OpQPUZor
	OpQPURx
	OpQPURy
	OpQPURz
	OpQPUCCX
	OpQPUMeasure
	OpQPUCircuit
	OpQPUQubit
	OpQPUClear
	OpQPUBarrier
	OpQPUSnapshot
)

// Tensor Opcodes
const (
	OpTensorCreate Opcode = 0xA0 + iota
	OpTensorAdd
	OpTensorMul
	OpTensorMatMul
	OpTensorReshape
	OpTensorSlice
	OpTensorSum
	OpTensorGrad
	OpTensorBackward
)

// ============================================================
// Bytecode Module
// ============================================================

// Module represents a complete compiled Karkain module
type Module struct {
	Header     ModuleHeader
	Symbols    SymbolTable
	Functions  []Function
	Kernels    []GPUKernel
	Circuits   []QuantumCircuit
	Tensors    []TensorGraph
	Sections   []Section
}

// ModuleHeader is the file header
type ModuleHeader struct {
	Magic    [4]byte
	Version  uint32
	Flags    uint32
	EntryIdx uint32 // index of main entry function
	NumSyms  uint32
	NumFuncs uint32
	NumKerns uint32
	NumQPU   uint32
	NumTen   uint32
	OffsetSym uint32
}

// Section is a generic section container
type Section struct {
	Type   uint8
	Offset uint32
	Size   uint32
}

// ============================================================
// Symbol Table
// ============================================================

type SymbolKind uint8

const (
	SymFunc SymbolKind = iota
	SymVar
	SymType
	SymStruct
	SymKernel
	SymCircuit
	SymMacro
	SymImport
	SymExport
)

type SymbolEntry struct {
	Name       string
	Kind       SymbolKind
	Idx        uint32 // index into the relevant table
	IsExported bool
	IsPublic   bool
}

type SymbolTable struct {
	Entries []SymbolEntry
}

// ============================================================
// IR Functions
// ============================================================

type Function struct {
	Name       string
	NumParams  uint16
	NumLocals  uint16
	Params     []ParamInfo
	Locals     []LocalInfo
	Instructions []Instruction
}

type ParamInfo struct {
	Name string
	Type TypeDesc
}

type LocalInfo struct {
	Name string
	Type TypeDesc
	Index uint16
}

type TypeDesc struct {
	Kind    TypeKind
	Name    string
	Size    uint32
	ElemType *TypeDesc // for arrays/slices
}

type TypeKind uint8

const (
	TypeInt TypeKind = iota
	TypeFloat
	TypeBool
	TypeString
	TypeVoid
	TypeArray
	TypeSlice
	TypeStruct
	TypeQubit
	TypeBit
	TypeTensor
	TypeGPUBuffer
	TypePointer
	TypeFunc
)

type Instruction struct {
	Op   Opcode
	Arg0 uint16
	Arg1 uint16
	Arg2 uint16
	Imm  int64  // immediate value
}

// ============================================================
// GPU Kernels
// ============================================================

type GPUKernel struct {
	Name       string
	SourceLang string // "wgsl", "cuda", "spirv"
	Source     string
	NumBuffers uint16
	WorkGroup  [3]uint32
	Params     []ParamInfo
	Instructions []Instruction
}

// ============================================================
// Quantum Circuits
// ============================================================

type QuantumCircuit struct {
	Name      string
	NumQubits uint16
	NumBits   uint16
	Gates     []QuantumGateIR
	Params    []ParamInfo
}

type QuantumGateIR struct {
	Op      Opcode // OpQPUH, OpQPUX, etc.
	Qubits  []uint16
	Angle   float64 // for parameterized gates
	Control uint16  // control qubit for controlled gates
}

// ============================================================
// Tensor Graphs
// ============================================================

type TensorGraph struct {
	Name      string
	NumInputs uint16
	Nodes     []TensorNode
	Edges     []TensorEdge
}

type TensorNode struct {
	Op     Opcode
	Name   string
	Shape  []int32
	Inputs []uint32
}

type TensorEdge struct {
	From uint32
	To   uint32
}

// ============================================================
// Writer — Serialization
// ============================================================

type Writer struct {
	buf  []byte
	pos  int
}

// NewWriter creates a new bytecode writer
func NewWriter() *Writer {
	return &Writer{buf: make([]byte, 0, 4096)}
}

func (w *Writer) WriteByteVal(b byte) {
	w.buf = append(w.buf, b)
}

func (w *Writer) WriteBytes(data []byte) {
	w.buf = append(w.buf, data...)
}

func (w *Writer) WriteUint16(v uint16) {
	w.buf = append(w.buf, byte(v), byte(v>>8))
}

func (w *Writer) WriteUint32(v uint32) {
	w.buf = append(w.buf, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}

func (w *Writer) WriteInt64(v int64) {
	w.buf = append(w.buf, byte(v), byte(v>>8), byte(v>>16), byte(v>>24),
		byte(v>>32), byte(v>>40), byte(v>>48), byte(v>>56))
}

func (w *Writer) WriteFloat64(v float64) {
	w.WriteUint64(math.Float64bits(v))
}

func (w *Writer) WriteUint64(v uint64) {
	w.buf = append(w.buf, byte(v), byte(v>>8), byte(v>>16), byte(v>>24),
		byte(v>>32), byte(v>>40), byte(v>>48), byte(v>>56))
}

func (w *Writer) WriteString(s string) {
	if len(s) > int(MaxStringLen) {
		s = s[:MaxStringLen]
	}
	w.WriteUint16(uint16(len(s)))
	w.buf = append(w.buf, s...)
}

func (w *Writer) Bytes() []byte {
	return w.buf
}

func (w *Writer) Len() int {
	return len(w.buf)
}

// ============================================================
// Reader — Deserialization
// ============================================================

type Reader struct {
	data []byte
	pos  int
}

// NewReader creates a new bytecode reader
func NewReader(data []byte) *Reader {
	return &Reader{data: data}
}

func (r *Reader) ReadByte() (byte, error) {
	if r.pos >= len(r.data) {
		return 0, io.ErrUnexpectedEOF
	}
	b := r.data[r.pos]
	r.pos++
	return b, nil
}

func (r *Reader) ReadBytes(n int) ([]byte, error) {
	if r.pos+n > len(r.data) {
		return nil, io.ErrUnexpectedEOF
	}
	data := r.data[r.pos : r.pos+n]
	r.pos += n
	return data, nil
}

func (r *Reader) ReadUint16() (uint16, error) {
	b, err := r.ReadBytes(2)
	if err != nil {
		return 0, err
	}
	return uint16(b[0]) | uint16(b[1])<<8, nil
}

func (r *Reader) ReadUint32() (uint32, error) {
	b, err := r.ReadBytes(4)
	if err != nil {
		return 0, err
	}
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24, nil
}

func (r *Reader) ReadInt64() (int64, error) {
	b, err := r.ReadBytes(8)
	if err != nil {
		return 0, err
	}
	return int64(b[0]) | int64(b[1])<<8 | int64(b[2])<<16 | int64(b[3])<<24 |
		int64(b[4])<<32 | int64(b[5])<<40 | int64(b[6])<<48 | int64(b[7])<<56, nil
}

func (r *Reader) ReadFloat64() (float64, error) {
	v, err := r.ReadUint64()
	if err != nil {
		return 0, err
	}
	return math.Float64frombits(v), nil
}

func (r *Reader) ReadUint64() (uint64, error) {
	b, err := r.ReadBytes(8)
	if err != nil {
		return 0, err
	}
	return uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 |
		uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56, nil
}

func (r *Reader) ReadString() (string, error) {
	n, err := r.ReadUint16()
	if err != nil {
		return "", err
	}
	b, err := r.ReadBytes(int(n))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ============================================================
// Serialization: Module → Bytes
// ============================================================

// SerializeModule writes a Module to binary format
func SerializeModule(mod *Module) ([]byte, error) {
	w := NewWriter()

	// Reserve space for header (will be filled later)
	headerStart := w.Len()
	for i := 0; i < 40; i++ { // header size: 10 uint32 fields
		w.WriteByteVal(0)
	}

	// Serialize symbols
	symStart := uint32(w.Len())
	writeSymbolTable(w, &mod.Symbols)

	// Serialize functions
	w.WriteUint32(uint32(len(mod.Functions)))
	for i := range mod.Functions {
		writeFunction(w, &mod.Functions[i])
	}

	// Serialize GPU kernels
	w.WriteUint32(uint32(len(mod.Kernels)))
	for i := range mod.Kernels {
		writeGPUKernel(w, &mod.Kernels[i])
	}

	// Serialize quantum circuits
	w.WriteUint32(uint32(len(mod.Circuits)))
	for i := range mod.Circuits {
		writeQuantumCircuit(w, &mod.Circuits[i])
	}

	// Serialize tensor graphs
	w.WriteUint32(uint32(len(mod.Tensors)))
	for i := range mod.Tensors {
		writeTensorGraph(w, &mod.Tensors[i])
	}

	// EOF marker
	w.WriteByteVal(SectionEOF)

	// Write header at reserved position
	data := w.Bytes()
	copy(data[headerStart:], magicBytes[:])
	binary.LittleEndian.PutUint32(data[headerStart+4:], BytecodeVersion)
	binary.LittleEndian.PutUint32(data[headerStart+8:], mod.Header.Flags)
	binary.LittleEndian.PutUint32(data[headerStart+12:], mod.Header.EntryIdx)
	binary.LittleEndian.PutUint32(data[headerStart+16:], uint32(len(mod.Symbols.Entries)))
	binary.LittleEndian.PutUint32(data[headerStart+20:], uint32(len(mod.Functions)))
	binary.LittleEndian.PutUint32(data[headerStart+24:], uint32(len(mod.Kernels)))
	binary.LittleEndian.PutUint32(data[headerStart+28:], uint32(len(mod.Circuits)))
	binary.LittleEndian.PutUint32(data[headerStart+32:], uint32(len(mod.Tensors)))

	// Fix up section offsets
	binary.LittleEndian.PutUint32(data[headerStart+36:], symStart)

	return data, nil
}

func writeSymbolTable(w *Writer, st *SymbolTable) {
	w.WriteUint32(uint32(len(st.Entries)))
	for _, e := range st.Entries {
		w.WriteString(e.Name)
		w.WriteByteVal(byte(e.Kind))
		w.WriteUint32(e.Idx)
		w.WriteByteVal(boolToByte(e.IsExported))
		w.WriteByteVal(boolToByte(e.IsPublic))
	}
}

func writeFunction(w *Writer, f *Function) {
	w.WriteString(f.Name)
	w.WriteUint16(f.NumParams)
	w.WriteUint16(f.NumLocals)

	// Params
	for _, p := range f.Params {
		w.WriteString(p.Name)
		writeTypeDesc(w, &p.Type)
	}

	// Locals
	for _, l := range f.Locals {
		w.WriteString(l.Name)
		writeTypeDesc(w, &l.Type)
		w.WriteUint16(l.Index)
	}

	// Instructions
	w.WriteUint32(uint32(len(f.Instructions)))
	for _, inst := range f.Instructions {
		w.WriteByteVal(byte(inst.Op))
		w.WriteUint16(inst.Arg0)
		w.WriteUint16(inst.Arg1)
		w.WriteUint16(inst.Arg2)
		w.WriteInt64(inst.Imm)
	}
}

func writeGPUKernel(w *Writer, k *GPUKernel) {
	w.WriteString(k.Name)
	w.WriteString(k.SourceLang)
	w.WriteString(k.Source)
	w.WriteUint16(k.NumBuffers)
	for _, d := range k.WorkGroup {
		w.WriteUint32(d)
	}
	w.WriteUint16(uint16(len(k.Params)))
	for _, p := range k.Params {
		w.WriteString(p.Name)
		writeTypeDesc(w, &p.Type)
	}
	w.WriteUint32(uint32(len(k.Instructions)))
	for _, inst := range k.Instructions {
		w.WriteByteVal(byte(inst.Op))
		w.WriteUint16(inst.Arg0)
		w.WriteUint16(inst.Arg1)
		w.WriteUint16(inst.Arg2)
		w.WriteInt64(inst.Imm)
	}
}

func writeQuantumCircuit(w *Writer, qc *QuantumCircuit) {
	w.WriteString(qc.Name)
	w.WriteUint16(qc.NumQubits)
	w.WriteUint16(qc.NumBits)
	w.WriteUint16(uint16(len(qc.Params)))
	for _, p := range qc.Params {
		w.WriteString(p.Name)
		writeTypeDesc(w, &p.Type)
	}
	w.WriteUint32(uint32(len(qc.Gates)))
	for _, g := range qc.Gates {
		w.WriteByteVal(byte(g.Op))
		w.WriteUint16(uint16(len(g.Qubits)))
		for _, q := range g.Qubits {
			w.WriteUint16(q)
		}
		w.WriteFloat64(g.Angle)
		w.WriteUint16(g.Control)
	}
}

func writeTensorGraph(w *Writer, tg *TensorGraph) {
	w.WriteString(tg.Name)
	w.WriteUint16(tg.NumInputs)
	w.WriteUint32(uint32(len(tg.Nodes)))
	for _, n := range tg.Nodes {
		w.WriteByteVal(byte(n.Op))
		w.WriteString(n.Name)
		w.WriteUint32(uint32(len(n.Shape)))
		for _, s := range n.Shape {
			w.WriteInt32(s)
		}
		w.WriteUint32(uint32(len(n.Inputs)))
		for _, inp := range n.Inputs {
			w.WriteUint32(inp)
		}
	}
	w.WriteUint32(uint32(len(tg.Edges)))
	for _, e := range tg.Edges {
		w.WriteUint32(e.From)
		w.WriteUint32(e.To)
	}
}

func (w *Writer) WriteInt32(v int32) {
	w.buf = append(w.buf, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}

func writeTypeDesc(w *Writer, td *TypeDesc) {
	w.WriteByteVal(byte(td.Kind))
	w.WriteString(td.Name)
	w.WriteUint32(td.Size)
	if td.ElemType != nil {
		w.WriteByteVal(1)
		writeTypeDesc(w, td.ElemType)
	} else {
		w.WriteByteVal(0)
	}
}

func boolToByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

// ============================================================
// Deserialization: Bytes → Module
// ============================================================

// DeserializeModule reads a Module from binary format
func DeserializeModule(data []byte) (*Module, error) {
	r := NewReader(data)
	mod := &Module{}

	// Validate header
	if len(data) < 40 {
		return nil, errors.New("kbc: file too small for header")
	}

	var magic [4]byte
	for i := 0; i < 4; i++ {
		b, _ := r.ReadByte()
		magic[i] = b
	}
	if magic != magicBytes {
		return nil, fmt.Errorf("kbc: invalid magic bytes %v, expected %v", magic, magicBytes)
	}

	version, _ := r.ReadUint32()
	if version > BytecodeVersion {
		return nil, fmt.Errorf("kbc: unsupported version %d (max %d)", version, BytecodeVersion)
	}
	mod.Header.Version = version

	mod.Header.Flags, _ = r.ReadUint32()
	mod.Header.EntryIdx, _ = r.ReadUint32()
	mod.Header.NumSyms, _ = r.ReadUint32()
	mod.Header.NumFuncs, _ = r.ReadUint32()
	mod.Header.NumKerns, _ = r.ReadUint32()
	mod.Header.NumQPU, _ = r.ReadUint32()
	mod.Header.NumTen, _ = r.ReadUint32()
	mod.Header.OffsetSym, _ = r.ReadUint32()

	// Skip to symbol section
	r.pos = int(mod.Header.OffsetSym)

	// Read symbol table
	mod.Symbols, _ = readSymbolTable(r)

	// Read functions
	mod.Functions, _ = readFunctions(r)

	// Read GPU kernels
	mod.Kernels, _ = readGPUKernels(r)

	// Read quantum circuits
	mod.Circuits, _ = readQuantumCircuits(r)

	// Read tensor graphs
	mod.Tensors, _ = readTensorGraphs(r)

	return mod, nil
}

func readSymbolTable(r *Reader) (SymbolTable, error) {
	st := SymbolTable{}
	n, _ := r.ReadUint32()
	st.Entries = make([]SymbolEntry, n)
	for i := uint32(0); i < n; i++ {
		name, _ := r.ReadString()
		kind, _ := r.ReadByte()
		idx, _ := r.ReadUint32()
		exp, _ := r.ReadByte()
		pub, _ := r.ReadByte()
		st.Entries[i] = SymbolEntry{
			Name:       name,
			Kind:       SymbolKind(kind),
			Idx:        idx,
			IsExported: exp == 1,
			IsPublic:   pub == 1,
		}
	}
	return st, nil
}

func readFunctions(r *Reader) ([]Function, error) {
	n, _ := r.ReadUint32()
	funcs := make([]Function, n)
	for i := uint32(0); i < n; i++ {
		f := Function{}
		f.Name, _ = r.ReadString()
		f.NumParams, _ = r.ReadUint16()
		f.NumLocals, _ = r.ReadUint16()

		f.Params = make([]ParamInfo, f.NumParams)
		for j := uint16(0); j < f.NumParams; j++ {
			f.Params[j].Name, _ = r.ReadString()
			f.Params[j].Type, _ = readTypeDesc(r)
		}

		f.Locals = make([]LocalInfo, f.NumLocals)
		for j := uint16(0); j < f.NumLocals; j++ {
			f.Locals[j].Name, _ = r.ReadString()
			f.Locals[j].Type, _ = readTypeDesc(r)
			f.Locals[j].Index, _ = r.ReadUint16()
		}

		numInst, _ := r.ReadUint32()
		f.Instructions = make([]Instruction, numInst)
		for j := uint32(0); j < numInst; j++ {
			op, _ := r.ReadByte()
			arg0, _ := r.ReadUint16()
			arg1, _ := r.ReadUint16()
			arg2, _ := r.ReadUint16()
			imm, _ := r.ReadInt64()
			f.Instructions[j] = Instruction{Op: Opcode(op), Arg0: arg0, Arg1: arg1, Arg2: arg2, Imm: imm}
		}
		funcs[i] = f
	}
	return funcs, nil
}

func readGPUKernels(r *Reader) ([]GPUKernel, error) {
	n, _ := r.ReadUint32()
	kernels := make([]GPUKernel, n)
	for i := uint32(0); i < n; i++ {
		k := GPUKernel{}
		k.Name, _ = r.ReadString()
		k.SourceLang, _ = r.ReadString()
		k.Source, _ = r.ReadString()
		k.NumBuffers, _ = r.ReadUint16()
		for j := 0; j < 3; j++ {
			k.WorkGroup[j], _ = r.ReadUint32()
		}
		numParams, _ := r.ReadUint16()
		k.Params = make([]ParamInfo, numParams)
		for j := uint16(0); j < numParams; j++ {
			k.Params[j].Name, _ = r.ReadString()
			k.Params[j].Type, _ = readTypeDesc(r)
		}
		numInst, _ := r.ReadUint32()
		k.Instructions = make([]Instruction, numInst)
		for j := uint32(0); j < numInst; j++ {
			op, _ := r.ReadByte()
			arg0, _ := r.ReadUint16()
			arg1, _ := r.ReadUint16()
			arg2, _ := r.ReadUint16()
			imm, _ := r.ReadInt64()
			k.Instructions[j] = Instruction{Op: Opcode(op), Arg0: arg0, Arg1: arg1, Arg2: arg2, Imm: imm}
		}
		kernels[i] = k
	}
	return kernels, nil
}

func readQuantumCircuits(r *Reader) ([]QuantumCircuit, error) {
	n, _ := r.ReadUint32()
	circuits := make([]QuantumCircuit, n)
	for i := uint32(0); i < n; i++ {
		qc := QuantumCircuit{}
		qc.Name, _ = r.ReadString()
		qc.NumQubits, _ = r.ReadUint16()
		qc.NumBits, _ = r.ReadUint16()
		numParams, _ := r.ReadUint16()
		qc.Params = make([]ParamInfo, numParams)
		for j := uint16(0); j < numParams; j++ {
			qc.Params[j].Name, _ = r.ReadString()
			qc.Params[j].Type, _ = readTypeDesc(r)
		}
		numGates, _ := r.ReadUint32()
		qc.Gates = make([]QuantumGateIR, numGates)
		for j := uint32(0); j < numGates; j++ {
			op, _ := r.ReadByte()
			numQ, _ := r.ReadUint16()
			qubits := make([]uint16, numQ)
			for k := uint16(0); k < numQ; k++ {
				qubits[k], _ = r.ReadUint16()
			}
			angle, _ := r.ReadFloat64()
			ctrl, _ := r.ReadUint16()
			qc.Gates[j] = QuantumGateIR{Op: Opcode(op), Qubits: qubits, Angle: angle, Control: ctrl}
		}
		circuits[i] = qc
	}
	return circuits, nil
}

func readTensorGraphs(r *Reader) ([]TensorGraph, error) {
	n, _ := r.ReadUint32()
	graphs := make([]TensorGraph, n)
	for i := uint32(0); i < n; i++ {
		tg := TensorGraph{}
		tg.Name, _ = r.ReadString()
		tg.NumInputs, _ = r.ReadUint16()
		numNodes, _ := r.ReadUint32()
		tg.Nodes = make([]TensorNode, numNodes)
		for j := uint32(0); j < numNodes; j++ {
			n2 := TensorNode{}
			op, _ := r.ReadByte()
			n2.Op = Opcode(op)
			n2.Name, _ = r.ReadString()
			numShape, _ := r.ReadUint32()
			n2.Shape = make([]int32, numShape)
			for k := uint32(0); k < numShape; k++ {
				v, _ := r.ReadUint32()
				n2.Shape = append(n2.Shape, int32(v))
			}
			numInputs, _ := r.ReadUint32()
			n2.Inputs = make([]uint32, numInputs)
			for k := uint32(0); k < numInputs; k++ {
				n2.Inputs[k], _ = r.ReadUint32()
			}
			tg.Nodes[j] = n2
		}
		numEdges, _ := r.ReadUint32()
		tg.Edges = make([]TensorEdge, numEdges)
		for j := uint32(0); j < numEdges; j++ {
			tg.Edges[j].From, _ = r.ReadUint32()
			tg.Edges[j].To, _ = r.ReadUint32()
		}
		graphs[i] = tg
	}
	return graphs, nil
}

func readTypeDesc(r *Reader) (TypeDesc, error) {
	td := TypeDesc{}
	kind, _ := r.ReadByte()
	td.Kind = TypeKind(kind)
	td.Name, _ = r.ReadString()
	td.Size, _ = r.ReadUint32()
	hasElem, _ := r.ReadByte()
	if hasElem == 1 {
		elem, _ := readTypeDesc(r)
		td.ElemType = &elem
	}
	return td, nil
}

// ============================================================
// Opcode Name Lookup
// ============================================================

func OpcodeName(op Opcode) string {
	switch {
	case op <= OpAssert:
		return cpuOpcodeName(op)
	case op >= OpGPUKernel && op <= OpGPUSetArg:
		return gpuOpcodeName(op)
	case op >= OpQPUH && op <= OpQPUSnapshot:
		return qpuOpcodeName(op)
	case op >= OpTensorCreate && op <= OpTensorBackward:
		return tensorOpcodeName(op)
	default:
		return fmt.Sprintf("Unknown(0x%02X)", uint8(op))
	}
}

func cpuOpcodeName(op Opcode) string {
	names := [...]string{
		"nop", "const_int", "const_float", "const_bool", "const_string",
		"load", "store", "add", "sub", "mul", "div", "mod", "neg", "not",
		"and", "or", "eq", "neq", "lt", "le", "gt", "ge", "jump", "jump_if",
		"jump_if_not", "call", "return", "pop", "dup", "print", "cast",
		"index", "field", "new_struct", "new_array", "array_push", "array_len",
		"make_slice", "assert",
	}
	if int(op) < len(names) {
		return names[op]
	}
	return "unknown_cpu"
}

func gpuOpcodeName(op Opcode) string {
	names := [...]string{
		"gpu_kernel", "gpu_buf_alloc", "gpu_buf_free", "gpu_buf_write",
		"gpu_buf_read", "gpu_dispatch", "gpu_sync", "gpu_build_kernel", "gpu_set_arg",
	}
	idx := int(op) - int(OpGPUKernel)
	if idx < len(names) {
		return names[idx]
	}
	return "unknown_gpu"
}

func qpuOpcodeName(op Opcode) string {
	names := [...]string{
		"qpu_h", "qpu_x", "qpu_y", "qpu_z", "qpu_xor", "qpu_yor", "qpu_zor",
		"qpu_rx", "qpu_ry", "qpu_rz", "qpu_ccx", "qpu_measure", "qpu_circuit",
		"qpu_qubit", "qpu_clear", "qpu_barrier", "qpu_snapshot",
	}
	idx := int(op) - int(OpQPUH)
	if idx >= 0 && idx < len(names) {
		return names[idx]
	}
	return "unknown_qpu"
}

func tensorOpcodeName(op Opcode) string {
	names := [...]string{
		"tensor_create", "tensor_add", "tensor_mul", "tensor_matmul",
		"tensor_reshape", "tensor_slice", "tensor_sum", "tensor_grad",
		"tensor_backward",
	}
	idx := int(op) - int(OpTensorCreate)
	if idx >= 0 && idx < len(names) {
		return names[idx]
	}
	return "unknown_tensor"
}

// ============================================================
// Human-Readable Dump
// ============================================================

// DumpModule produces a human-readable dump of the module
func DumpModule(mod *Module) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("=== Karkain Bytecode Module (v%d) ===\n", mod.Header.Version))
	b.WriteString(fmt.Sprintf("Functions: %d, Kernels: %d, Circuits: %d, Tensors: %d\n",
		len(mod.Functions), len(mod.Kernels), len(mod.Circuits), len(mod.Tensors)))
	b.WriteString(fmt.Sprintf("Symbols: %d\n\n", len(mod.Symbols.Entries)))

	for i, f := range mod.Functions {
		b.WriteString(fmt.Sprintf("--- Function %d: %s (params=%d, locals=%d) ---\n",
			i, f.Name, f.NumParams, f.NumLocals))
		for j, inst := range f.Instructions {
			b.WriteString(fmt.Sprintf("  [%3d] %-16s arg0=%d arg1=%d arg2=%d imm=%d\n",
				j, OpcodeName(inst.Op), inst.Arg0, inst.Arg1, inst.Arg2, inst.Imm))
		}
		b.WriteString("\n")
	}

	for i, k := range mod.Kernels {
		b.WriteString(fmt.Sprintf("--- GPU Kernel %d: %s (lang=%s, bufs=%d) ---\n",
			i, k.Name, k.SourceLang, k.NumBuffers))
		for j, inst := range k.Instructions {
			b.WriteString(fmt.Sprintf("  [%3d] %-16s arg0=%d arg1=%d arg2=%d imm=%d\n",
				j, OpcodeName(inst.Op), inst.Arg0, inst.Arg1, inst.Arg2, inst.Imm))
		}
		b.WriteString("\n")
	}

	for i, qc := range mod.Circuits {
		b.WriteString(fmt.Sprintf("--- Quantum Circuit %d: %s (qubits=%d, gates=%d) ---\n",
			i, qc.Name, qc.NumQubits, len(qc.Gates)))
		for j, g := range qc.Gates {
			b.WriteString(fmt.Sprintf("  [%3d] %s qubits=%v angle=%.4f ctrl=%d\n",
				j, OpcodeName(g.Op), g.Qubits, g.Angle, g.Control))
		}
		b.WriteString("\n")
	}

	return b.String()
}
