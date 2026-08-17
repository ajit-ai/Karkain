package jit

import (
	"math"
	"testing"

	"karkain/pkg/ir"
)

func buildTestModule(funcs []ir.Function, symbols []ir.SymbolEntry) *ir.Module {
	return &ir.Module{
		Header: ir.ModuleHeader{
			Version:  ir.BytecodeVersion,
			EntryIdx: 0,
		},
		Symbols: ir.SymbolTable{Entries: symbols},
		Functions: funcs,
		Circuits:  []ir.QuantumCircuit{},
		Kernels:   []ir.GPUKernel{},
		Tensors:   []ir.TensorGraph{},
	}
}

// ============================================================
// TestJIT_BytecodeExecution — end-to-end CPU execution
// ============================================================

func TestJIT_BasicArithmetic(t *testing.T) {
	mod := buildTestModule(
		[]ir.Function{
			{
				Name:       "main",
				NumParams:  0,
				NumLocals:  1,
				Instructions: []ir.Instruction{
					{Op: ir.OpConstInt, Imm: 3},
					{Op: ir.OpConstInt, Imm: 4},
					{Op: ir.OpAdd},
					{Op: ir.OpStore, Arg0: 0},
					{Op: ir.OpLoad, Arg0: 0},
					{Op: ir.OpReturn},
				},
			},
		},
		[]ir.SymbolEntry{{Name: "main", Kind: ir.SymFunc, Idx: 0, IsExported: true}},
	)

	engine := NewEngine(DefaultConfig())
	ctx, err := engine.ExecuteModule(mod)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	result := ctx.Stack[ctx.Sp-1]
	if result.Kind != VKInt || result.Int != 7 {
		t.Errorf("expected 7, got %v", result)
	}
}

func TestJIT_SubMulDiv(t *testing.T) {
	mod := buildTestModule(
		[]ir.Function{
			{
				Name:      "main",
				NumParams: 0, NumLocals: 0,
				Instructions: []ir.Instruction{
					{Op: ir.OpConstInt, Imm: 10},
					{Op: ir.OpConstInt, Imm: 3},
					{Op: ir.OpSub},
					{Op: ir.OpConstInt, Imm: 2},
					{Op: ir.OpMul},
					{Op: ir.OpConstInt, Imm: 7},
					{Op: ir.OpDiv},
					{Op: ir.OpReturn},
				},
			},
		},
		[]ir.SymbolEntry{{Name: "main", Kind: ir.SymFunc, Idx: 0}},
	)

	engine := NewEngine(DefaultConfig())
	ctx, err := engine.ExecuteModule(mod)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	result := ctx.Stack[ctx.Sp-1]
	// (10 - 3) = 7; 7 * 2 = 14; 14 / 7 = 2
	if result.Kind != VKInt || result.Int != 2 {
		t.Errorf("expected 2, got %v", result)
	}
}

func TestJIT_Modulo(t *testing.T) {
	mod := buildTestModule(
		[]ir.Function{
			{
				Name:      "main",
				NumParams: 0, NumLocals: 0,
				Instructions: []ir.Instruction{
					{Op: ir.OpConstInt, Imm: 17},
					{Op: ir.OpConstInt, Imm: 5},
					{Op: ir.OpMod},
					{Op: ir.OpReturn},
				},
			},
		},
		[]ir.SymbolEntry{{Name: "main", Kind: ir.SymFunc, Idx: 0}},
	)

	engine := NewEngine(DefaultConfig())
	ctx, err := engine.ExecuteModule(mod)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	result := ctx.Stack[ctx.Sp-1]
	if result.Kind != VKInt || result.Int != 2 {
		t.Errorf("expected 2, got %v", result)
	}
}

func TestJIT_Negation(t *testing.T) {
	mod := buildTestModule(
		[]ir.Function{
			{
				Name:      "main",
				NumParams: 0, NumLocals: 0,
				Instructions: []ir.Instruction{
					{Op: ir.OpConstInt, Imm: 5},
					{Op: ir.OpNeg},
					{Op: ir.OpReturn},
				},
			},
		},
		[]ir.SymbolEntry{{Name: "main", Kind: ir.SymFunc, Idx: 0}},
	)

	engine := NewEngine(DefaultConfig())
	ctx, err := engine.ExecuteModule(mod)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	result := ctx.Stack[ctx.Sp-1]
	if result.Kind != VKInt || result.Int != -5 {
		t.Errorf("expected -5, got %v", result)
	}
}

func TestJIT_Comparisons(t *testing.T) {
	tests := []struct {
		op   ir.Opcode
		a, b int64
		want bool
	}{
		{ir.OpEq, 5, 5, true},
		{ir.OpEq, 5, 3, false},
		{ir.OpNeq, 5, 3, true},
		{ir.OpLt, 3, 5, true},
		{ir.OpLt, 5, 3, false},
		{ir.OpLe, 5, 5, true},
		{ir.OpLe, 5, 3, false},
		{ir.OpGt, 5, 3, true},
		{ir.OpGt, 3, 5, false},
		{ir.OpGe, 5, 5, true},
		{ir.OpGe, 3, 5, false},
	}

	for _, tt := range tests {
		mod := buildTestModule(
			[]ir.Function{
				{
					Name:      "main",
					NumParams: 0, NumLocals: 0,
					Instructions: []ir.Instruction{
						{Op: ir.OpConstInt, Imm: tt.a},
						{Op: ir.OpConstInt, Imm: tt.b},
						{Op: tt.op},
						{Op: ir.OpReturn},
					},
				},
			},
			[]ir.SymbolEntry{{Name: "main", Kind: ir.SymFunc, Idx: 0}},
		)

		engine := NewEngine(DefaultConfig())
		ctx, err := engine.ExecuteModule(mod)
		if err != nil {
			t.Fatalf("op %d: execute failed: %v", tt.op, err)
		}

		result := ctx.Stack[ctx.Sp-1]
		if result.Kind != VKBool || result.Bool != tt.want {
			t.Errorf("op %d: expected %v, got %v", tt.op, tt.want, result.Bool)
		}
	}
}

func TestJIT_JumpConditional(t *testing.T) {
	// if (5 > 3) { x = 10 } else { x = 20 }
	// return x
	mod := buildTestModule(
		[]ir.Function{
			{
				Name:      "main",
				NumParams: 0, NumLocals: 1,
				Instructions: []ir.Instruction{
					{Op: ir.OpConstInt, Imm: 5},
					{Op: ir.OpConstInt, Imm: 3},
					{Op: ir.OpGt},
					{Op: ir.OpJumpIfNot, Imm: 7}, // skip to else at instruction 7
					{Op: ir.OpConstInt, Imm: 10},
					{Op: ir.OpStore, Arg0: 0},
					{Op: ir.OpJump, Imm: 9}, // skip else, go to end
					{Op: ir.OpConstInt, Imm: 20}, // else branch
					{Op: ir.OpStore, Arg0: 0},
					{Op: ir.OpLoad, Arg0: 0},
					{Op: ir.OpReturn},
				},
			},
		},
		[]ir.SymbolEntry{{Name: "main", Kind: ir.SymFunc, Idx: 0}},
	)

	engine := NewEngine(DefaultConfig())
	ctx, err := engine.ExecuteModule(mod)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	result := ctx.Stack[ctx.Sp-1]
	if result.Kind != VKInt || result.Int != 10 {
		t.Errorf("expected 10 (true branch), got %v", result)
	}
}

func TestJIT_JumpIfNot(t *testing.T) {
	// if (3 > 5) { x = 10 } else { x = 20 }
	// return x
	mod := buildTestModule(
		[]ir.Function{
			{
				Name:      "main",
				NumParams: 0, NumLocals: 1,
				Instructions: []ir.Instruction{
					{Op: ir.OpConstInt, Imm: 3},
					{Op: ir.OpConstInt, Imm: 5},
					{Op: ir.OpGt},
					{Op: ir.OpJumpIfNot, Imm: 7}, // skip to else at instruction 7
					{Op: ir.OpConstInt, Imm: 10},
					{Op: ir.OpStore, Arg0: 0},
					{Op: ir.OpJump, Imm: 9}, // skip else, go to end
					{Op: ir.OpConstInt, Imm: 20}, // else branch
					{Op: ir.OpStore, Arg0: 0},
					{Op: ir.OpLoad, Arg0: 0},
					{Op: ir.OpReturn},
				},
			},
		},
		[]ir.SymbolEntry{{Name: "main", Kind: ir.SymFunc, Idx: 0}},
	)

	engine := NewEngine(DefaultConfig())
	ctx, err := engine.ExecuteModule(mod)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	result := ctx.Stack[ctx.Sp-1]
	if result.Kind != VKInt || result.Int != 20 {
		t.Errorf("expected 20 (false branch), got %v", result)
	}
}

func TestJIT_FunctionCall(t *testing.T) {
	// main calls add(3, 4) → 7
	addFn := ir.Function{
		Name:      "add",
		NumParams: 2,
		NumLocals: 0,
		Params: []ir.ParamInfo{
			{Name: "a", Type: ir.TypeDesc{Kind: ir.TypeInt, Name: "int"}},
			{Name: "b", Type: ir.TypeDesc{Kind: ir.TypeInt, Name: "int"}},
		},
		Instructions: []ir.Instruction{
			{Op: ir.OpLoad, Arg0: 0},
			{Op: ir.OpLoad, Arg0: 1},
			{Op: ir.OpAdd},
			{Op: ir.OpReturn},
		},
	}

	mainFn := ir.Function{
		Name:      "main",
		NumParams: 0,
		NumLocals: 1,
		Instructions: []ir.Instruction{
			{Op: ir.OpConstInt, Imm: 3},
			{Op: ir.OpConstInt, Imm: 4},
			{Op: ir.OpCall, Arg0: 2, Arg1: 1}, // 2 args, symbol index 1 = "add"
			{Op: ir.OpStore, Arg0: 0},
			{Op: ir.OpLoad, Arg0: 0},
			{Op: ir.OpReturn},
		},
	}

	mod := buildTestModule(
		[]ir.Function{mainFn, addFn},
		[]ir.SymbolEntry{
			{Name: "main", Kind: ir.SymFunc, Idx: 0},
			{Name: "add", Kind: ir.SymFunc, Idx: 1},
		},
	)

	engine := NewEngine(DefaultConfig())
	ctx, err := engine.ExecuteModule(mod)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	result := ctx.Stack[ctx.Sp-1]
	if result.Kind != VKInt || result.Int != 7 {
		t.Errorf("expected 7, got %v", result)
	}
}

func TestJIT_Print(t *testing.T) {
	mod := buildTestModule(
		[]ir.Function{
			{
				Name:      "main",
				NumParams: 0, NumLocals: 0,
				Instructions: []ir.Instruction{
					{Op: ir.OpConstInt, Imm: 42},
					{Op: ir.OpPrint},
					{Op: ir.OpNop},
					{Op: ir.OpReturn},
				},
			},
		},
		[]ir.SymbolEntry{{Name: "main", Kind: ir.SymFunc, Idx: 0}},
	)

	engine := NewEngine(DefaultConfig())
	ctx, err := engine.ExecuteModule(mod)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	if len(ctx.Outputs) != 1 || ctx.Outputs[0] != "42" {
		t.Errorf("expected output ['42'], got %v", ctx.Outputs)
	}
}

func TestJIT_ArrayOperations(t *testing.T) {
	mod := buildTestModule(
		[]ir.Function{
			{
				Name:      "main",
				NumParams: 0, NumLocals: 1,
				Instructions: []ir.Instruction{
					{Op: ir.OpConstInt, Imm: 3},
					{Op: ir.OpNewArray},
					{Op: ir.OpStore, Arg0: 0},
					{Op: ir.OpLoad, Arg0: 0},
					{Op: ir.OpArrayLen},
					{Op: ir.OpReturn},
				},
			},
		},
		[]ir.SymbolEntry{{Name: "main", Kind: ir.SymFunc, Idx: 0}},
	)

	engine := NewEngine(DefaultConfig())
	ctx, err := engine.ExecuteModule(mod)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	result := ctx.Stack[ctx.Sp-1]
	if result.Kind != VKInt || result.Int != 3 {
		t.Errorf("expected array length 3, got %v", result)
	}
}

func TestJIT_DupAndPop(t *testing.T) {
	mod := buildTestModule(
		[]ir.Function{
			{
				Name:      "main",
				NumParams: 0, NumLocals: 0,
				Instructions: []ir.Instruction{
					{Op: ir.OpConstInt, Imm: 99},
					{Op: ir.OpDup},
					{Op: ir.OpPop},
					{Op: ir.OpReturn},
				},
			},
		},
		[]ir.SymbolEntry{{Name: "main", Kind: ir.SymFunc, Idx: 0}},
	)

	engine := NewEngine(DefaultConfig())
	ctx, err := engine.ExecuteModule(mod)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	result := ctx.Stack[ctx.Sp-1]
	if result.Kind != VKInt || result.Int != 99 {
		t.Errorf("expected 99, got %v", result)
	}
}

func TestJIT_CallDepthExceeded(t *testing.T) {
	// Recursive function that hits call depth limit
	recursiveFn := ir.Function{
		Name:      "recurse",
		NumParams: 0,
		NumLocals: 0,
		Instructions: []ir.Instruction{
			{Op: ir.OpCall, Arg0: 0, Arg1: 0},
			{Op: ir.OpReturn},
		},
	}

	mod := buildTestModule(
		[]ir.Function{recursiveFn},
		[]ir.SymbolEntry{{Name: "recurse", Kind: ir.SymFunc, Idx: 0}},
	)

	cfg := DefaultConfig()
	cfg.MaxCallDepth = 5
	engine := NewEngine(cfg)
	_, err := engine.ExecuteModule(mod)
	if err == nil {
		t.Fatal("expected call depth error")
	}
}

func TestJIT_AssertPass(t *testing.T) {
	mod := buildTestModule(
		[]ir.Function{
			{
				Name:      "main",
				NumParams: 0, NumLocals: 0,
				Instructions: []ir.Instruction{
					{Op: ir.OpConstBool, Imm: 1},
					{Op: ir.OpAssert},
					{Op: ir.OpReturn},
				},
			},
		},
		[]ir.SymbolEntry{{Name: "main", Kind: ir.SymFunc, Idx: 0}},
	)

	engine := NewEngine(DefaultConfig())
	_, err := engine.ExecuteModule(mod)
	if err != nil {
		t.Fatalf("assert should pass: %v", err)
	}
}

func TestJIT_AssertFail(t *testing.T) {
	mod := buildTestModule(
		[]ir.Function{
			{
				Name:      "main",
				NumParams: 0, NumLocals: 0,
				Instructions: []ir.Instruction{
					{Op: ir.OpConstBool, Imm: 0},
					{Op: ir.OpAssert},
					{Op: ir.OpReturn},
				},
			},
		},
		[]ir.SymbolEntry{{Name: "main", Kind: ir.SymFunc, Idx: 0}},
	)

	engine := NewEngine(DefaultConfig())
	_, err := engine.ExecuteModule(mod)
	if err == nil {
		t.Fatal("expected assertion failure")
	}
}

func TestJIT_FloatArithmetic(t *testing.T) {
	mod := buildTestModule(
		[]ir.Function{
			{
				Name:      "main",
				NumParams: 0, NumLocals: 0,
				Instructions: []ir.Instruction{
					{Op: ir.OpConstFloat, Imm: int64(math.Float64bits(2.5))},
					{Op: ir.OpConstFloat, Imm: int64(math.Float64bits(3.5))},
					{Op: ir.OpMul},
					{Op: ir.OpReturn},
				},
			},
		},
		[]ir.SymbolEntry{{Name: "main", Kind: ir.SymFunc, Idx: 0}},
	)

	engine := NewEngine(DefaultConfig())
	ctx, err := engine.ExecuteModule(mod)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	result := ctx.Stack[ctx.Sp-1]
	if result.Kind != VKFloat || result.Float != 8.75 {
		t.Errorf("expected 8.75, got %v", result)
	}
}

func TestJIT_AndOrLogic(t *testing.T) {
	mod := buildTestModule(
		[]ir.Function{
			{
				Name:      "main",
				NumParams: 0, NumLocals: 0,
				Instructions: []ir.Instruction{
					{Op: ir.OpConstBool, Imm: 1},
					{Op: ir.OpConstBool, Imm: 0},
					{Op: ir.OpAnd},
					{Op: ir.OpConstBool, Imm: 1},
					{Op: ir.OpConstBool, Imm: 0},
					{Op: ir.OpOr},
					{Op: ir.OpReturn},
				},
			},
		},
		[]ir.SymbolEntry{{Name: "main", Kind: ir.SymFunc, Idx: 0}},
	)

	engine := NewEngine(DefaultConfig())
	ctx, err := engine.ExecuteModule(mod)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	// Stack: false (and result), true (or result)
	result := ctx.Stack[ctx.Sp-1]
	if result.Kind != VKBool || result.Bool != true {
		t.Errorf("expected true for or result, got %v", result)
	}
}

func TestJIT_Cast(t *testing.T) {
	mod := buildTestModule(
		[]ir.Function{
			{
				Name:      "main",
				NumParams: 0, NumLocals: 0,
				Instructions: []ir.Instruction{
					{Op: ir.OpConstFloat, Imm: int64(math.Float64bits(3.7))},
					{Op: ir.OpCast, Arg0: uint16(ir.TypeInt)},
					{Op: ir.OpReturn},
				},
			},
		},
		[]ir.SymbolEntry{{Name: "main", Kind: ir.SymFunc, Idx: 0}},
	)

	engine := NewEngine(DefaultConfig())
	ctx, err := engine.ExecuteModule(mod)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	result := ctx.Stack[ctx.Sp-1]
	if result.Kind != VKInt || result.Int != 3 {
		t.Errorf("expected 3, got %v", result)
	}
}

// ============================================================
// TestJIT_HeterogeneousDispatch — hybrid CPU/GPU/QPU/Tensor
// ============================================================

func TestJIT_HeterogeneousDispatch(t *testing.T) {
	gpuDispatched := false
	qpuCircuit := false
	tensorBackward := false

	cfg := DefaultConfig()
	cfg.OnGPUDispatch = func(ctx *ExecContext, kern *ir.GPUKernel, args []Value) ([]Value, error) {
		gpuDispatched = true
		return nil, nil
	}
	cfg.OnQPUExecute = func(ctx *ExecContext, circ *ir.QuantumCircuit, args []Value) ([]Value, error) {
		qpuCircuit = true
		return nil, nil
	}
	cfg.OnTensorExec = func(ctx *ExecContext, graph *ir.TensorGraph, args []Value) ([]Value, error) {
		tensorBackward = true
		return nil, nil
	}

	mod := &ir.Module{
		Header: ir.ModuleHeader{Version: ir.BytecodeVersion, EntryIdx: 0},
		Symbols: ir.SymbolTable{Entries: []ir.SymbolEntry{
			{Name: "main", Kind: ir.SymFunc, Idx: 0},
		}},
		Functions: []ir.Function{
			{
				Name:      "main",
				NumParams: 0, NumLocals: 0,
				Instructions: []ir.Instruction{
					// CPU: compute 2+2
					{Op: ir.OpConstInt, Imm: 2},
					{Op: ir.OpConstInt, Imm: 2},
					{Op: ir.OpAdd},
					{Op: ir.OpPop},
					// GPU: buffer alloc + dispatch
					{Op: ir.OpGPUBufferAlloc, Arg0: 0, Imm: 1024},
					{Op: ir.OpGPUDispatch, Arg0: 64, Arg1: 1, Arg2: 1},
					{Op: ir.OpGPUSync},
					// QPU: execute a circuit (triggers OnQPUExecute)
					{Op: ir.OpQPUCircuit, Imm: 0},
					// Tensor: backward pass (triggers OnTensorExec)
					{Op: ir.OpTensorCreate, Imm: 4},
					{Op: ir.OpTensorBackward},
					{Op: ir.OpReturn},
				},
			},
		},
		Kernels: []ir.GPUKernel{
			{Name: "vec_add", SourceLang: "wgsl", Source: "@compute @workgroup_size(64)"},
		},
		Circuits: []ir.QuantumCircuit{
			{Name: "bell", NumQubits: 2, NumBits: 2},
		},
		Tensors: []ir.TensorGraph{
			{Name: "forward", NumInputs: 2},
		},
	}

	engine := NewEngine(cfg)
	_, err := engine.ExecuteModule(mod)
	if err != nil {
		t.Fatalf("heterogeneous execution failed: %v", err)
	}

	if !gpuDispatched {
		t.Error("GPU dispatch was not called")
	}
	if !qpuCircuit {
		t.Error("QPU circuit execution was not called")
	}
	if !tensorBackward {
		t.Error("Tensor backward pass was not called")
	}
}

// ============================================================
// TestJIT_QuantumSimulation — quantum gate simulation
// ============================================================

func TestJIT_HadamardGate(t *testing.T) {
	mod := &ir.Module{
		Header: ir.ModuleHeader{Version: ir.BytecodeVersion, EntryIdx: 0},
		Symbols: ir.SymbolTable{Entries: []ir.SymbolEntry{
			{Name: "main", Kind: ir.SymFunc, Idx: 0},
		}},
		Functions: []ir.Function{
			{
				Name: "main", NumParams: 0, NumLocals: 0,
				Instructions: []ir.Instruction{
					{Op: ir.OpQPUH, Arg0: 0},
					{Op: ir.OpQPUMeasure, Arg0: 0},
					{Op: ir.OpReturn},
				},
			},
		},
		Circuits: []ir.QuantumCircuit{
			{Name: "test", NumQubits: 1, NumBits: 1},
		},
	}

	engine := NewEngine(DefaultConfig())
	ctx, err := engine.ExecuteModule(mod)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	result := ctx.Stack[ctx.Sp-1]
	if result.Kind != VKInt {
		t.Fatalf("expected int result, got %v", result)
	}
	// Measurement of H|0> is 0 or 1 with ~50/50 probability
	if result.Int != 0 && result.Int != 1 {
		t.Errorf("expected 0 or 1, got %d", result.Int)
	}
}

func TestJIT_PauliXGate(t *testing.T) {
	mod := &ir.Module{
		Header: ir.ModuleHeader{Version: ir.BytecodeVersion, EntryIdx: 0},
		Symbols: ir.SymbolTable{Entries: []ir.SymbolEntry{
			{Name: "main", Kind: ir.SymFunc, Idx: 0},
		}},
		Functions: []ir.Function{
			{
				Name: "main", NumParams: 0, NumLocals: 0,
				Instructions: []ir.Instruction{
					{Op: ir.OpQPUX, Arg0: 0}, // X|0> = |1>
					{Op: ir.OpQPUMeasure, Arg0: 0},
					{Op: ir.OpReturn},
				},
			},
		},
		Circuits: []ir.QuantumCircuit{
			{Name: "test", NumQubits: 1, NumBits: 1},
		},
	}

	engine := NewEngine(DefaultConfig())
	ctx, err := engine.ExecuteModule(mod)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	result := ctx.Stack[ctx.Sp-1]
	if result.Kind != VKInt || result.Int != 1 {
		t.Errorf("expected 1 (X|0>=|1>), got %v", result)
	}
}

func TestJIT_GPUBufferAlloc(t *testing.T) {
	mod := &ir.Module{
		Header: ir.ModuleHeader{Version: ir.BytecodeVersion, EntryIdx: 0},
		Symbols: ir.SymbolTable{Entries: []ir.SymbolEntry{
			{Name: "main", Kind: ir.SymFunc, Idx: 0},
		}},
		Functions: []ir.Function{
			{
				Name: "main", NumParams: 0, NumLocals: 0,
				Instructions: []ir.Instruction{
					{Op: ir.OpGPUBufferAlloc, Arg0: 1, Imm: 256},
					{Op: ir.OpGPUBufferRead, Arg0: 1},
					{Op: ir.OpReturn},
				},
			},
		},
	}

	engine := NewEngine(DefaultConfig())
	ctx, err := engine.ExecuteModule(mod)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	// Buffer read returns 0 for empty buffer
	result := ctx.Stack[ctx.Sp-1]
	if result.Kind != VKInt || result.Int != 0 {
		t.Errorf("expected 0, got %v", result)
	}

	// Verify buffer was created
	if _, ok := ctx.GPUBuffers[1]; !ok {
		t.Error("GPU buffer 1 not created")
	}
}

func TestJIT_TensorCreate(t *testing.T) {
	mod := &ir.Module{
		Header: ir.ModuleHeader{Version: ir.BytecodeVersion, EntryIdx: 0},
		Symbols: ir.SymbolTable{Entries: []ir.SymbolEntry{
			{Name: "main", Kind: ir.SymFunc, Idx: 0},
		}},
		Functions: []ir.Function{
			{
				Name: "main", NumParams: 0, NumLocals: 0,
				Instructions: []ir.Instruction{
					{Op: ir.OpTensorCreate, Imm: 4},
					{Op: ir.OpArrayLen},
					{Op: ir.OpReturn},
				},
			},
		},
	}

	engine := NewEngine(DefaultConfig())
	ctx, err := engine.ExecuteModule(mod)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	result := ctx.Stack[ctx.Sp-1]
	if result.Kind != VKInt || result.Int != 4 {
		t.Errorf("expected tensor size 4, got %v", result)
	}
}

// ============================================================
// TestJIT_SerializationRoundTrip
// ============================================================

func TestJIT_SerializationRoundTrip(t *testing.T) {
	mod := buildTestModule(
		[]ir.Function{
			{
				Name: "main", NumParams: 0, NumLocals: 0,
				Instructions: []ir.Instruction{
					{Op: ir.OpConstInt, Imm: 21},
					{Op: ir.OpConstInt, Imm: 2},
					{Op: ir.OpMul},
					{Op: ir.OpReturn},
				},
			},
		},
		[]ir.SymbolEntry{{Name: "main", Kind: ir.SymFunc, Idx: 0}},
	)

	data, err := ir.SerializeModule(mod)
	if err != nil {
		t.Fatalf("serialize failed: %v", err)
	}

	engine := NewEngine(DefaultConfig())
	ctx, err := engine.ExecuteBytes(data)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	result := ctx.Stack[ctx.Sp-1]
	if result.Kind != VKInt || result.Int != 42 {
		t.Errorf("expected 42, got %v", result)
	}
}

// ============================================================
// TestJIT_FFIIntegration
// ============================================================

func TestJIT_FFIIntegration(t *testing.T) {
	ffiCalled := false
	ffiResult := Value{Kind: VKInt, Int: 42}

	cfg := DefaultConfig()
	cfg.OnFFICall = func(ctx *ExecContext, name string, args []Value) ([]Value, error) {
		if name == "test_func" {
			ffiCalled = true
			return []Value{ffiResult}, nil
		}
		return nil, nil
	}

	// Set up FFI library with the symbol
	ffiLib := &FFILib{
		Name:    "test",
		Symbols: make(map[string]*FFISymbol),
	}
	ffiLib.RegisterSymbol("test_func", 0, CTypeI32, nil)

	cfg.FFILib = ffiLib

	// main calls test_func — test_func is NOT in Functions, only in symbols/FFI
	mainFn := ir.Function{
		Name: "main", NumParams: 0, NumLocals: 0,
		Instructions: []ir.Instruction{
			{Op: ir.OpCall, Arg0: 0, Arg1: 1},
			{Op: ir.OpReturn},
		},
	}

	mod := &ir.Module{
		Header: ir.ModuleHeader{Version: ir.BytecodeVersion, EntryIdx: 0},
		Symbols: ir.SymbolTable{Entries: []ir.SymbolEntry{
			{Name: "main", Kind: ir.SymFunc, Idx: 0},
			{Name: "test_func", Kind: ir.SymFunc, Idx: 1},
		}},
		Functions: []ir.Function{mainFn},
	}

	engine := NewEngine(cfg)
	ctx, err := engine.ExecuteModule(mod)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	if !ffiCalled {
		t.Error("FFI callback was not invoked")
	}

	result := ctx.Stack[ctx.Sp-1]
	if result.Kind != VKInt || result.Int != 42 {
		t.Errorf("expected FFI result 42, got %v", result)
	}
}
