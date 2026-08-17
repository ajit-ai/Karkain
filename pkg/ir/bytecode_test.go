package ir

import (
	"bytes"
	"testing"
)

func TestIR_BytecodeSerialization(t *testing.T) {
	// Build a test module with CPU functions, GPU kernels, and quantum circuits
	mod := &Module{
		Header: ModuleHeader{
			Flags:    0x01,
			EntryIdx: 0,
		},
		Symbols: SymbolTable{
			Entries: []SymbolEntry{
				{Name: "main", Kind: SymFunc, Idx: 0, IsExported: true, IsPublic: true},
				{Name: "add", Kind: SymFunc, Idx: 1, IsExported: true, IsPublic: false},
				{Name: "buf", Kind: SymVar, Idx: 0, IsExported: false, IsPublic: false},
			},
		},
		Functions: []Function{
			{
				Name:      "main",
				NumParams: 0,
				NumLocals: 2,
				Params:    []ParamInfo{},
				Locals: []LocalInfo{
					{Name: "x", Type: TypeDesc{Kind: TypeInt, Name: "int", Size: 8}, Index: 0},
					{Name: "y", Type: TypeDesc{Kind: TypeInt, Name: "int", Size: 8}, Index: 1},
				},
				Instructions: []Instruction{
					{Op: OpConstInt, Imm: 42},
					{Op: OpStore, Arg0: 0},
					{Op: OpConstInt, Imm: 58},
					{Op: OpStore, Arg0: 1},
					{Op: OpLoad, Arg0: 0},
					{Op: OpLoad, Arg0: 1},
					{Op: OpAdd},
					{Op: OpReturn},
				},
			},
			{
				Name:      "add",
				NumParams: 2,
				NumLocals: 0,
				Params: []ParamInfo{
					{Name: "a", Type: TypeDesc{Kind: TypeInt, Name: "int", Size: 8}},
					{Name: "b", Type: TypeDesc{Kind: TypeInt, Name: "int", Size: 8}},
				},
				Instructions: []Instruction{
					{Op: OpLoad, Arg0: 0},
					{Op: OpLoad, Arg0: 1},
					{Op: OpAdd},
					{Op: OpReturn},
				},
			},
		},
		Kernels: []GPUKernel{
			{
				Name:       "add_vectors",
				SourceLang: "wgsl",
				Source:     "@group(0) @binding(0) var<storage,read> a: array<f32>;",
				NumBuffers: 3,
				WorkGroup:  [3]uint32{256, 1, 1},
				Params: []ParamInfo{
					{Name: "n", Type: TypeDesc{Kind: TypeInt, Name: "u32", Size: 4}},
				},
				Instructions: []Instruction{
					{Op: OpGPUBuildKernel, Imm: 0},
					{Op: OpGPUDispatch, Arg0: 256, Arg1: 1, Arg2: 1},
					{Op: OpGPUSync},
				},
			},
		},
		Circuits: []QuantumCircuit{
			{
				Name:      "bell_pair",
				NumQubits: 2,
				NumBits:   2,
				Params:    []ParamInfo{},
				Gates: []QuantumGateIR{
					{Op: OpQPUH, Qubits: []uint16{0}},
					{Op: OpQPUCCX, Qubits: []uint16{0, 1}},
					{Op: OpQPUMeasure, Qubits: []uint16{0}},
					{Op: OpQPUMeasure, Qubits: []uint16{1}},
				},
			},
			{
				Name:      "rotation_test",
				NumQubits: 1,
				NumBits:   1,
				Params:    []ParamInfo{{Name: "theta", Type: TypeDesc{Kind: TypeFloat, Name: "f64", Size: 8}}},
				Gates: []QuantumGateIR{
					{Op: OpQPURx, Qubits: []uint16{0}, Angle: 1.5708},
					{Op: OpQPUH, Qubits: []uint16{0}},
				},
			},
		},
		Tensors: []TensorGraph{
			{
				Name:      "vqe_energy",
				NumInputs: 2,
				Nodes: []TensorNode{
					{Op: OpTensorCreate, Name: "params", Shape: []int32{4}, Inputs: nil},
					{Op: OpTensorCreate, Name: "hamiltonian", Shape: []int32{4, 4}, Inputs: nil},
					{Op: OpTensorMatMul, Name: "expectation", Shape: nil, Inputs: []uint32{0, 1}},
				},
				Edges: []TensorEdge{
					{From: 0, To: 2},
					{From: 1, To: 2},
				},
			},
		},
	}

	// Serialize
	data, err := SerializeModule(mod)
	if err != nil {
		t.Fatalf("serialize failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("serialized data is empty")
	}

	// Validate magic bytes
	if data[0] != 0x4B || data[1] != 0x52 || data[2] != 0x4B || data[3] != 0x01 {
		t.Errorf("invalid magic bytes: %v", data[:4])
	}

	// Deserialize
	mod2, err := DeserializeModule(data)
	if err != nil {
		t.Fatalf("deserialize failed: %v", err)
	}

	// Verify round-trip
	if mod2.Header.Version != BytecodeVersion {
		t.Errorf("version mismatch: %d vs %d", mod2.Header.Version, BytecodeVersion)
	}

	if len(mod2.Symbols.Entries) != 3 {
		t.Errorf("expected 3 symbols, got %d", len(mod2.Symbols.Entries))
	}
	if mod2.Symbols.Entries[0].Name != "main" {
		t.Errorf("expected symbol 'main', got '%s'", mod2.Symbols.Entries[0].Name)
	}

	if len(mod2.Functions) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(mod2.Functions))
	}
	if mod2.Functions[0].Name != "main" {
		t.Errorf("expected function 'main', got '%s'", mod2.Functions[0].Name)
	}
	if len(mod2.Functions[0].Instructions) != 8 {
		t.Errorf("expected 8 instructions in main, got %d", len(mod2.Functions[0].Instructions))
	}
	if mod2.Functions[0].Instructions[0].Imm != 42 {
		t.Errorf("expected imm=42 for first instruction, got %d", mod2.Functions[0].Instructions[0].Imm)
	}

	if len(mod2.Kernels) != 1 {
		t.Fatalf("expected 1 kernel, got %d", len(mod2.Kernels))
	}
	if mod2.Kernels[0].Name != "add_vectors" {
		t.Errorf("expected kernel 'add_vectors', got '%s'", mod2.Kernels[0].Name)
	}
	if mod2.Kernels[0].SourceLang != "wgsl" {
		t.Errorf("expected lang 'wgsl', got '%s'", mod2.Kernels[0].SourceLang)
	}
	if mod2.Kernels[0].WorkGroup[0] != 256 {
		t.Errorf("expected workgroup[0]=256, got %d", mod2.Kernels[0].WorkGroup[0])
	}

	if len(mod2.Circuits) != 2 {
		t.Fatalf("expected 2 circuits, got %d", len(mod2.Circuits))
	}
	if mod2.Circuits[0].Name != "bell_pair" {
		t.Errorf("expected circuit 'bell_pair', got '%s'", mod2.Circuits[0].Name)
	}
	if mod2.Circuits[0].NumQubits != 2 {
		t.Errorf("expected 2 qubits, got %d", mod2.Circuits[0].NumQubits)
	}
	if len(mod2.Circuits[0].Gates) != 4 {
		t.Errorf("expected 4 gates, got %d", len(mod2.Circuits[0].Gates))
	}
	if mod2.Circuits[0].Gates[0].Op != OpQPUH {
		t.Errorf("expected first gate to be H")
	}
	if mod2.Circuits[1].Gates[0].Angle != 1.5708 {
		t.Errorf("expected angle 1.5708, got %f", mod2.Circuits[1].Gates[0].Angle)
	}

	if len(mod2.Tensors) != 1 {
		t.Fatalf("expected 1 tensor graph, got %d", len(mod2.Tensors))
	}
	if mod2.Tensors[0].Name != "vqe_energy" {
		t.Errorf("expected tensor 'vqe_energy', got '%s'", mod2.Tensors[0].Name)
	}
	if len(mod2.Tensors[0].Nodes) != 3 {
		t.Errorf("expected 3 tensor nodes, got %d", len(mod2.Tensors[0].Nodes))
	}
}

func TestIR_RoundTripBinary(t *testing.T) {
	mod := &Module{
		Header: ModuleHeader{Flags: 0},
		Symbols: SymbolTable{Entries: []SymbolEntry{
			{Name: "test_func", Kind: SymFunc, Idx: 0, IsExported: true},
		}},
		Functions: []Function{{
			Name: "test_func", NumParams: 1, NumLocals: 1,
			Params: []ParamInfo{{Name: "val", Type: TypeDesc{Kind: TypeFloat, Name: "f64", Size: 8}}},
			Locals: []LocalInfo{{Name: "tmp", Type: TypeDesc{Kind: TypeFloat, Name: "f64", Size: 8}, Index: 0}},
			Instructions: []Instruction{
				{Op: OpLoad, Arg0: 0},
				{Op: OpConstFloat, Imm: 314159265358979}, // π approximation
				{Op: OpMul},
				{Op: OpReturn},
			},
		}},
	}

	data1, _ := SerializeModule(mod)
	mod2, _ := DeserializeModule(data1)
	data2, _ := SerializeModule(mod2)

	if !bytes.Equal(data1, data2) {
		t.Error("double round-trip should produce identical bytes")
	}
}

func TestIR_OpcodeNames(t *testing.T) {
	tests := []struct {
		op   Opcode
		want string
	}{
		{OpNop, "nop"},
		{OpConstInt, "const_int"},
		{OpAdd, "add"},
		{OpReturn, "return"},
		{OpGPUKernel, "gpu_kernel"},
		{OpGPUDispatch, "gpu_dispatch"},
		{OpQPUH, "qpu_h"},
		{OpQPUCCX, "qpu_ccx"},
		{OpQPUMeasure, "qpu_measure"},
		{OpTensorCreate, "tensor_create"},
		{OpTensorMatMul, "tensor_matmul"},
	}
	for _, tt := range tests {
		got := OpcodeName(tt.op)
		if got != tt.want {
			t.Errorf("OpcodeName(%d) = %q, want %q", tt.op, got, tt.want)
		}
	}
}

func TestIR_DumpModule(t *testing.T) {
	mod := &Module{
		Header:  ModuleHeader{Version: BytecodeVersion},
		Symbols: SymbolTable{Entries: []SymbolEntry{{Name: "f", Kind: SymFunc}}},
		Functions: []Function{{
			Name: "f",
			Instructions: []Instruction{
				{Op: OpConstInt, Imm: 42},
				{Op: OpReturn},
			},
		}},
	}

	dump := DumpModule(mod)
	if !bytes.Contains([]byte(dump), []byte("Function 0: f")) {
		t.Error("dump should contain function name")
	}
	if !bytes.Contains([]byte(dump), []byte("const_int")) {
		t.Error("dump should contain opcode name")
	}
}
