package jit

import (
	"fmt"
	"math"
	"math/cmplx"
	"os"
	"strings"

	"karkain/pkg/ir"
)

// ============================================================
// Phase 36: Unified Multi-Target JIT Execution Engine
// Stack-based VM that executes .kbc bytecode modules
// ============================================================

// JITConfig configures the JIT engine behavior
type JITConfig struct {
	MaxStack     int
	MaxCallDepth int
	Verbose      bool
	OnGPUDispatch func(ctx *ExecContext, kern *ir.GPUKernel, args []Value) ([]Value, error)
	OnQPUExecute  func(ctx *ExecContext, circ *ir.QuantumCircuit, args []Value) ([]Value, error)
	OnTensorExec  func(ctx *ExecContext, graph *ir.TensorGraph, args []Value) ([]Value, error)
	OnPrint       func(args []Value)
	OnFFICall     func(ctx *ExecContext, name string, args []Value) ([]Value, error)
	FFILib        *FFILib
}

func DefaultConfig() *JITConfig {
	return &JITConfig{
		MaxStack:     1 << 20,
		MaxCallDepth: 1024,
	}
}

// Value represents a runtime value in the VM
type Value struct {
	Kind   ValueKind
	Int    int64
	Float  float64
	Bool   bool
	Str    string
	Ptr    uintptr
	Array  []Value
}

type ValueKind int

const (
	VKNone ValueKind = iota
	VKInt
	VKFloat
	VKBool
	VKString
	VKPtr
	VKArray
	VKFunc
)

func (v Value) String() string {
	switch v.Kind {
	case VKInt:
		return fmt.Sprintf("%d", v.Int)
	case VKFloat:
		return fmt.Sprintf("%g", v.Float)
	case VKBool:
		if v.Bool {
			return "true"
		}
		return "false"
	case VKString:
		return v.Str
	case VKPtr:
		return fmt.Sprintf("ptr(%d)", v.Ptr)
	case VKArray:
		return fmt.Sprintf("array[%d]", len(v.Array))
	case VKFunc:
		return "func"
	default:
		return "none"
	}
}

// ============================================================
// Execution Context
// ============================================================

// ExecContext holds per-thread execution state
type ExecContext struct {
	Module      *ir.Module
	Config      *JITConfig
	Stack       []Value
	Sp          int // stack pointer
	CallStack   []callFrame
	CallDepth   int
	Globals     map[string]Value
	Heap        []byte
	HeapPtr     uintptr
	Outputs     []string
	Circuits    map[string]*CircuitState
	GPUBuffers  map[uint32]*GPUBuf
	FuncTable   map[string]*ir.Function
	TensorGraphs map[string]*TensorState
	FFILib      *FFILib
	Error       error
}

type callFrame struct {
	Func       *ir.Function
	ReturnAddr int
	Locals     []Value
}

// CircuitState tracks quantum circuit execution state
type CircuitState struct {
	Circuit    *ir.QuantumCircuit
	StateVec   []complex128
	NumQubits int
}

// GPUBuf represents a simulated GPU buffer
type GPUBuf struct {
	ID    uint32
	Size  uint32
	Data  []byte
}

// TensorState tracks tensor graph execution
type TensorState struct {
	Graph    *ir.TensorGraph
	Values   []Value
	Grad     []Value
}

// ============================================================
// JIT Engine
// ============================================================

// Engine is the main JIT execution engine
type Engine struct {
	Config *JITConfig
}

// NewEngine creates a new JIT engine
func NewEngine(cfg *JITConfig) *Engine {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Engine{Config: cfg}
}

// ExecuteModule loads and executes a bytecode module
func (e *Engine) ExecuteModule(mod *ir.Module) (*ExecContext, error) {
	ctx := &ExecContext{
		Module:       mod,
		Config:       e.Config,
		Stack:        make([]Value, e.Config.MaxStack),
		Sp:           0,
		CallStack:    make([]callFrame, e.Config.MaxCallDepth),
		CallDepth:    0,
		Globals:      make(map[string]Value),
		Heap:         make([]byte, 64*1024),
		HeapPtr:      0,
		Circuits:     make(map[string]*CircuitState),
		GPUBuffers:   make(map[uint32]*GPUBuf),
		FuncTable:    make(map[string]*ir.Function),
		TensorGraphs: make(map[string]*TensorState),
		FFILib:       e.Config.FFILib,
	}

	for i := range mod.Functions {
		ctx.FuncTable[mod.Functions[i].Name] = &mod.Functions[i]
	}

	// Register built-in kernel/circuit/tensor names
	for i := range mod.Kernels {
		ctx.GPUBuffers[uint32(i)] = &GPUBuf{ID: uint32(i)}
	}
	for i := range mod.Circuits {
		qc := mod.Circuits[i]
		state := &CircuitState{
			Circuit:    &qc,
			NumQubits: int(qc.NumQubits),
			StateVec:   make([]complex128, 1<<qc.NumQubits),
		}
		state.StateVec[0] = 1.0
		ctx.Circuits[qc.Name] = state
	}
	for i := range mod.Tensors {
		tg := mod.Tensors[i]
		ctx.TensorGraphs[tg.Name] = &TensorState{
			Graph:  &tg,
			Values: make([]Value, len(tg.Nodes)),
			Grad:   make([]Value, len(tg.Nodes)),
		}
	}

	// Find and execute entry point
	entryFunc := mod.Header.EntryIdx
	if entryFunc >= uint32(len(mod.Functions)) {
		return nil, fmt.Errorf("jit: entry point index %d out of range (have %d functions)", entryFunc, len(mod.Functions))
	}

	err := e.executeFunction(ctx, &mod.Functions[entryFunc], nil)
	if err != nil {
		return nil, err
	}

	return ctx, nil
}

// ExecuteBytes deserializes and executes bytecode from raw bytes
func (e *Engine) ExecuteBytes(data []byte) (*ExecContext, error) {
	mod, err := ir.DeserializeModule(data)
	if err != nil {
		return nil, fmt.Errorf("jit: deserialize failed: %w", err)
	}
	return e.ExecuteModule(mod)
}

// ExecuteFile loads a .kbc file and executes it
func (e *Engine) ExecuteFile(path string) (*ExecContext, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("jit: read file failed: %w", err)
	}
	return e.ExecuteBytes(data)
}

// ============================================================
// Function Execution
// ============================================================

func (e *Engine) executeFunction(ctx *ExecContext, fn *ir.Function, args []Value) error {
	if ctx.CallDepth >= e.Config.MaxCallDepth {
		return fmt.Errorf("jit: call depth exceeded for %s", fn.Name)
	}

	prevSp := ctx.Sp
	prevDepth := ctx.CallDepth

	// Create local frame — must be large enough for both locals and params
	localSize := int(fn.NumLocals)
	if int(fn.NumParams) > localSize {
		localSize = int(fn.NumParams)
	}
	locals := make([]Value, localSize)
	for i := uint16(0); i < fn.NumParams && int(i) < len(args); i++ {
		locals[i] = args[i]
	}

	ctx.CallStack[ctx.CallDepth] = callFrame{
		Func:       fn,
		ReturnAddr: -1,
		Locals:     locals,
	}
	ctx.CallDepth++

	// Execute instructions
	retVal := Value{Kind: VKNone}
	var execErr error
	ip := 0

	for ip < len(fn.Instructions) {
		inst := fn.Instructions[ip]

		if e.Config.Verbose {
			fmt.Fprintf(os.Stderr, "  [%d] %s arg0=%d arg1=%d imm=%d sp=%d\n",
				ip, ir.OpcodeName(inst.Op), inst.Arg0, inst.Arg1, inst.Imm, ctx.Sp)
		}

		switch {
		// === CPU Opcodes ===
		case inst.Op == ir.OpNop:
			// no-op

		case inst.Op == ir.OpConstInt:
			e.push(ctx, Value{Kind: VKInt, Int: inst.Imm})

		case inst.Op == ir.OpConstFloat:
			e.push(ctx, Value{Kind: VKFloat, Float: math.Float64frombits(uint64(inst.Imm))})

		case inst.Op == ir.OpConstBool:
			e.push(ctx, Value{Kind: VKBool, Bool: inst.Imm != 0})

		case inst.Op == ir.OpConstString:
			// imm holds the string table index; use imm directly as index
			strVal := Value{Kind: VKString, Str: fmt.Sprintf("const_%d", inst.Imm)}
			e.push(ctx, strVal)

		case inst.Op == ir.OpLoad:
			if int(inst.Arg0) < len(locals) {
				e.push(ctx, locals[inst.Arg0])
			} else {
				e.push(ctx, Value{Kind: VKNone})
			}

		case inst.Op == ir.OpStore:
			val := e.pop(ctx)
			if int(inst.Arg0) < len(locals) {
				locals[inst.Arg0] = val
			}

		case inst.Op == ir.OpAdd:
			b := e.pop(ctx)
			a := e.pop(ctx)
			e.push(ctx, addValues(a, b))

		case inst.Op == ir.OpSub:
			b := e.pop(ctx)
			a := e.pop(ctx)
			e.push(ctx, subValues(a, b))

		case inst.Op == ir.OpMul:
			b := e.pop(ctx)
			a := e.pop(ctx)
			e.push(ctx, mulValues(a, b))

		case inst.Op == ir.OpDiv:
			b := e.pop(ctx)
			a := e.pop(ctx)
			e.push(ctx, divValues(a, b))

		case inst.Op == ir.OpMod:
			b := e.pop(ctx)
			a := e.pop(ctx)
			if b.Int == 0 {
				execErr = fmt.Errorf("jit: division by zero in mod")
				break
			}
			e.push(ctx, Value{Kind: VKInt, Int: a.Int % b.Int})

		case inst.Op == ir.OpNeg:
			a := e.pop(ctx)
			if a.Kind == VKInt {
				e.push(ctx, Value{Kind: VKInt, Int: -a.Int})
			} else {
				e.push(ctx, Value{Kind: VKFloat, Float: -a.Float})
			}

		case inst.Op == ir.OpNot:
			a := e.pop(ctx)
			e.push(ctx, Value{Kind: VKBool, Bool: !a.Bool})

		case inst.Op == ir.OpAnd:
			b := e.pop(ctx)
			a := e.pop(ctx)
			e.push(ctx, Value{Kind: VKBool, Bool: a.Bool && b.Bool})

		case inst.Op == ir.OpOr:
			b := e.pop(ctx)
			a := e.pop(ctx)
			e.push(ctx, Value{Kind: VKBool, Bool: a.Bool || b.Bool})

		case inst.Op == ir.OpEq:
			b := e.pop(ctx)
			a := e.pop(ctx)
			e.push(ctx, Value{Kind: VKBool, Bool: valuesEqual(a, b)})

		case inst.Op == ir.OpNeq:
			b := e.pop(ctx)
			a := e.pop(ctx)
			e.push(ctx, Value{Kind: VKBool, Bool: !valuesEqual(a, b)})

		case inst.Op == ir.OpLt:
			b := e.pop(ctx)
			a := e.pop(ctx)
			e.push(ctx, Value{Kind: VKBool, Bool: valuesLT(a, b)})

		case inst.Op == ir.OpLe:
			b := e.pop(ctx)
			a := e.pop(ctx)
			e.push(ctx, Value{Kind: VKBool, Bool: valuesLT(a, b) || valuesEqual(a, b)})

		case inst.Op == ir.OpGt:
			b := e.pop(ctx)
			a := e.pop(ctx)
			e.push(ctx, Value{Kind: VKBool, Bool: !valuesLT(a, b) && !valuesEqual(a, b)})

		case inst.Op == ir.OpGe:
			b := e.pop(ctx)
			a := e.pop(ctx)
			e.push(ctx, Value{Kind: VKBool, Bool: !valuesLT(a, b)})

		case inst.Op == ir.OpJump:
			ip = int(inst.Imm)
			continue

		case inst.Op == ir.OpJumpIf:
			cond := e.pop(ctx)
			if cond.Bool {
				ip = int(inst.Imm)
				continue
			}

		case inst.Op == ir.OpJumpIfNot:
			cond := e.pop(ctx)
			if !cond.Bool {
				ip = int(inst.Imm)
				continue
			}

		case inst.Op == ir.OpCall:
			fnName := resolveFuncName(ctx, inst)

			// Pop arguments (arg0 = number of args)
			numArgs := int(inst.Arg0)
			args := make([]Value, numArgs)
			for i := numArgs - 1; i >= 0; i-- {
				args[i] = e.pop(ctx)
			}

			// Check for FFI calls first
			if ctx.FFILib != nil && ctx.FFILib.HasSymbol(fnName) {
				if ctx.Config.OnFFICall != nil {
					ffiResults, ffiErr := ctx.Config.OnFFICall(ctx, fnName, args)
					if ffiErr != nil {
						execErr = fmt.Errorf("jit: FFI call %s failed: %w", fnName, ffiErr)
						break
					}
					for _, r := range ffiResults {
						e.push(ctx, r)
					}
					ip++
					continue
				}
			}

			callee, ok := ctx.FuncTable[fnName]
			if !ok {
				execErr = fmt.Errorf("jit: undefined function '%s'", fnName)
				break
			}

			err := e.executeFunction(ctx, callee, args)
			if err != nil {
				execErr = err
				break
			}

		case inst.Op == ir.OpReturn:
			if ctx.Sp > 0 {
				retVal = e.pop(ctx)
			}
			goto done

		case inst.Op == ir.OpPop:
			e.pop(ctx)

		case inst.Op == ir.OpDup:
			val := e.peek(ctx)
			e.push(ctx, val)

		case inst.Op == ir.OpPrint:
			val := e.pop(ctx)
			if e.Config.OnPrint != nil {
				e.Config.OnPrint([]Value{val})
			} else {
				ctx.Outputs = append(ctx.Outputs, val.String())
			}

		case inst.Op == ir.OpCast:
			src := e.pop(ctx)
			targetType := ir.TypeKind(inst.Arg0)
			e.push(ctx, castValue(src, targetType))

		case inst.Op == ir.OpNewArray:
			size := e.pop(ctx)
			arr := make([]Value, size.Int)
			e.push(ctx, Value{Kind: VKArray, Array: arr})

		case inst.Op == ir.OpArrayPush:
			val := e.pop(ctx)
			arr := e.pop(ctx)
			arr.Array = append(arr.Array, val)
			e.push(ctx, arr)

		case inst.Op == ir.OpArrayLen:
			arr := e.pop(ctx)
			e.push(ctx, Value{Kind: VKInt, Int: int64(len(arr.Array))})

		case inst.Op == ir.OpIndex:
			idx := e.pop(ctx)
			arr := e.pop(ctx)
			if int(idx.Int) < len(arr.Array) {
				e.push(ctx, arr.Array[idx.Int])
			} else {
				execErr = fmt.Errorf("jit: index %d out of bounds (len=%d)", idx.Int, len(arr.Array))
				break
			}

		case inst.Op == ir.OpAssert:
			cond := e.pop(ctx)
			if !cond.Bool {
				msg := "assertion failed"
				if ctx.Sp > 0 {
					msgVal := e.pop(ctx)
					if msgVal.Kind == VKString {
						msg = msgVal.Str
					}
				}
				execErr = fmt.Errorf("jit: %s", msg)
				break
			}

		// === GPU Opcodes ===
		case inst.Op >= ir.OpGPUKernel && inst.Op <= ir.OpGPUSetArg:
			execErr = e.executeGPUOp(ctx, inst, ip)
			if execErr != nil {
				break
			}

		// === Quantum Opcodes ===
		case inst.Op >= ir.OpQPUH && inst.Op <= ir.OpQPUSnapshot:
			execErr = e.executeQPUOp(ctx, inst)
			if execErr != nil {
				break
			}

		// === Tensor Opcodes ===
		case inst.Op >= ir.OpTensorCreate && inst.Op <= ir.OpTensorBackward:
			execErr = e.executeTensorOp(ctx, inst)
			if execErr != nil {
				break
			}

		default:
			execErr = fmt.Errorf("jit: unknown opcode 0x%02X at instruction %d", uint8(inst.Op), ip)
		}

		if execErr != nil {
			break
		}
		ip++
	}

done:
	ctx.CallDepth--
	_ = prevDepth
	_ = prevSp

	if execErr != nil {
		return execErr
	}

	// Push return value if any
	if retVal.Kind != VKNone {
		e.push(ctx, retVal)
	}

	return nil
}

// ============================================================
// Stack Operations
// ============================================================

func (e *Engine) push(ctx *ExecContext, v Value) {
	if ctx.Sp >= e.Config.MaxStack {
		return
	}
	ctx.Stack[ctx.Sp] = v
	ctx.Sp++
}

func (e *Engine) pop(ctx *ExecContext) Value {
	if ctx.Sp <= 0 {
		return Value{Kind: VKNone}
	}
	ctx.Sp--
	v := ctx.Stack[ctx.Sp]
	ctx.Stack[ctx.Sp] = Value{Kind: VKNone}
	return v
}

func (e *Engine) peek(ctx *ExecContext) Value {
	if ctx.Sp <= 0 {
		return Value{Kind: VKNone}
	}
	return ctx.Stack[ctx.Sp-1]
}

// ============================================================
// GPU Opcode Execution
// ============================================================

func (e *Engine) executeGPUOp(ctx *ExecContext, inst ir.Instruction, ip int) error {
	switch inst.Op {
	case ir.OpGPUBuildKernel:
		// Build a GPU kernel from source
		kernIdx := inst.Imm
		if int(kernIdx) < len(ctx.Module.Kernels) {
			kern := &ctx.Module.Kernels[kernIdx]
			if e.Config.OnGPUDispatch != nil {
				_, err := e.Config.OnGPUDispatch(ctx, kern, nil)
				return err
			}
		}

	case ir.OpGPUBufferAlloc:
		bufID := uint32(inst.Arg0)
		size := uint32(inst.Imm)
		ctx.GPUBuffers[bufID] = &GPUBuf{
			ID:   bufID,
			Size: size,
			Data: make([]byte, size),
		}

	case ir.OpGPUBufferFree:
		bufID := uint32(inst.Arg0)
		delete(ctx.GPUBuffers, bufID)

	case ir.OpGPUBufferWrite:
		bufID := uint32(inst.Arg0)
		if buf, ok := ctx.GPUBuffers[bufID]; ok {
			val := e.pop(ctx)
			buf.Data = append(buf.Data, byte(val.Int))
		}

	case ir.OpGPUBufferRead:
		bufID := uint32(inst.Arg0)
		if buf, ok := ctx.GPUBuffers[bufID]; ok && len(buf.Data) > 0 {
			e.push(ctx, Value{Kind: VKInt, Int: int64(buf.Data[0])})
		} else {
			e.push(ctx, Value{Kind: VKInt, Int: 0})
		}

	case ir.OpGPUDispatch:
		x, y, z := inst.Arg0, inst.Arg1, inst.Arg2
		_ = x
		_ = y
		_ = z
		// Dispatch is handled via callback
		if e.Config.OnGPUDispatch != nil {
			_, err := e.Config.OnGPUDispatch(ctx, nil, nil)
			return err
		}

	case ir.OpGPUSync:
		// GPU sync — no-op in simulation

	case ir.OpGPUSetArg:
		_ = e.pop(ctx)
	}

	return nil
}

// ============================================================
// Quantum Opcode Execution
// ============================================================

func (e *Engine) executeQPUOp(ctx *ExecContext, inst ir.Instruction) error {
	qubitIdx := int(inst.Arg0)

	switch inst.Op {
	case ir.OpQPUH:
		// Hadamard gate simulation on state vector
		e.applyHadamard(ctx, qubitIdx)

	case ir.OpQPUX:
		e.applyPauliX(ctx, qubitIdx)

	case ir.OpQPUY:
		e.applyPauliY(ctx, qubitIdx)

	case ir.OpQPUZ:
		e.applyPauliZ(ctx, qubitIdx)

	case ir.OpQPURx:
		angle := math.Float64frombits(uint64(inst.Imm))
		e.applyRx(ctx, qubitIdx, angle)

	case ir.OpQPURy:
		angle := math.Float64frombits(uint64(inst.Imm))
		e.applyRy(ctx, qubitIdx, angle)

	case ir.OpQPURz:
		angle := math.Float64frombits(uint64(inst.Imm))
		e.applyRz(ctx, qubitIdx, angle)

	case ir.OpQPUCCX:
		// Toffoli: control0=Arg0, control1=Arg1, target=Arg2
		ctrl0 := int(inst.Arg0)
		ctrl1 := int(inst.Arg1)
		target := int(inst.Arg2)
		e.applyToffoli(ctx, ctrl0, ctrl1, target)

	case ir.OpQPUMeasure:
		prob := e.getQubitProbability(ctx, qubitIdx)
		outcome := 0
		if prob < 0.5 {
			outcome = 0
		} else {
			outcome = 1
		}
		e.push(ctx, Value{Kind: VKInt, Int: int64(outcome)})

	case ir.OpQPUCircuit:
		// Execute a named circuit
		circIdx := inst.Imm
		if int(circIdx) < len(ctx.Module.Circuits) {
			circ := &ctx.Module.Circuits[circIdx]
			if e.Config.OnQPUExecute != nil {
				_, err := e.Config.OnQPUExecute(ctx, circ, nil)
				return err
			}
		}

	case ir.OpQPUQubit:
		e.push(ctx, Value{Kind: VKInt, Int: int64(qubitIdx)})

	case ir.OpQPUClear:
		// Reset all qubits
		for i := range ctx.Circuits {
			sv := ctx.Circuits[i].StateVec
			for j := range sv {
				sv[j] = 0
			}
			sv[0] = 1.0
		}

	case ir.OpQPUBarrier:
		// No-op in simulation

	case ir.OpQPUSnapshot:
		e.push(ctx, Value{Kind: VKInt, Int: 0})
	}

	return nil
}

// findCircuitState finds the first available circuit state
func (e *Engine) findCircuitState(ctx *ExecContext) *CircuitState {
	for _, cs := range ctx.Circuits {
		return cs
	}
	return nil
}

func (e *Engine) applyHadamard(ctx *ExecContext, qubit int) {
	cs := e.findCircuitState(ctx)
	if cs == nil {
		return
	}
	sv := cs.StateVec
	n := len(sv)
	h := 1.0 / math.Sqrt(2.0)
	ch := complex(h, 0)
	step := 1 << qubit
	for i := 0; i < n; i += 2 * step {
		for j := i; j < i+step; j++ {
			a := sv[j]
			b := sv[j+step]
			sv[j] = ch * (a + b)
			sv[j+step] = ch * (a - b)
		}
	}
}

func (e *Engine) applyPauliX(ctx *ExecContext, qubit int) {
	cs := e.findCircuitState(ctx)
	if cs == nil {
		return
	}
	sv := cs.StateVec
	n := len(sv)
	step := 1 << qubit
	for i := 0; i < n; i += 2 * step {
		for j := i; j < i+step; j++ {
			sv[j], sv[j+step] = sv[j+step], sv[j]
		}
	}
}

func (e *Engine) applyPauliY(ctx *ExecContext, qubit int) {
	cs := e.findCircuitState(ctx)
	if cs == nil {
		return
	}
	sv := cs.StateVec
	n := len(sv)
	step := 1 << qubit
	for i := 0; i < n; i += 2*step {
		for j := i; j < i+step; j++ {
			a := sv[j]
			b := sv[j+step]
			sv[j] = complex(0, -1) * b
			sv[j+step] = complex(0, 1) * a
		}
	}
}

func (e *Engine) applyPauliZ(ctx *ExecContext, qubit int) {
	cs := e.findCircuitState(ctx)
	if cs == nil {
		return
	}
	sv := cs.StateVec
	n := len(sv)
	step := 1 << qubit
	for i := 0; i < n; i += 2 * step {
		for j := i + step; j < i+2*step; j++ {
			sv[j] = -sv[j]
		}
	}
}

func (e *Engine) applyRx(ctx *ExecContext, qubit int, angle float64) {
	cs := e.findCircuitState(ctx)
	if cs == nil {
		return
	}
	c := complex(math.Cos(angle/2), 0)
	s := complex(math.Sin(angle/2), 0)
	sv := cs.StateVec
	n := len(sv)
	step := 1 << qubit
	for i := 0; i < n; i += 2 * step {
		for j := i; j < i+step; j++ {
			a := sv[j]
			b := sv[j+step]
			sv[j] = c*a + complex(0, -1)*s*b
			sv[j+step] = complex(0, -1)*s*a + c*b
		}
	}
}

func (e *Engine) applyRy(ctx *ExecContext, qubit int, angle float64) {
	cs := e.findCircuitState(ctx)
	if cs == nil {
		return
	}
	c := complex(math.Cos(angle/2), 0)
	s := complex(math.Sin(angle/2), 0)
	sv := cs.StateVec
	n := len(sv)
	step := 1 << qubit
	for i := 0; i < n; i += 2 * step {
		for j := i; j < i+step; j++ {
			a := sv[j]
			b := sv[j+step]
			sv[j] = c*a - s*b
			sv[j+step] = s*a + c*b
		}
	}
}

func (e *Engine) applyRz(ctx *ExecContext, qubit int, angle float64) {
	cs := e.findCircuitState(ctx)
	if cs == nil {
		return
	}
	e0 := cmplx.Exp(complex(0, -angle/2))
	e1 := cmplx.Exp(complex(0, angle/2))
	sv := cs.StateVec
	n := len(sv)
	step := 1 << qubit
	for i := 0; i < n; i += 2 * step {
		for j := i; j < i+step; j++ {
			sv[j] *= e0
		}
		for j := i + step; j < i+2*step; j++ {
			sv[j] *= e1
		}
	}
}

func (e *Engine) applyToffoli(ctx *ExecContext, ctrl0, ctrl1, target int) {
	cs := e.findCircuitState(ctx)
	if cs == nil {
		return
	}
	sv := cs.StateVec
	n := len(sv)
	for i := 0; i < n; i++ {
		bit0 := (i >> ctrl0) & 1
		bit1 := (i >> ctrl1) & 1
		if bit0 == 1 && bit1 == 1 {
			j := i ^ (1 << target)
			if i < j {
				sv[i], sv[j] = sv[j], sv[i]
			}
		}
	}
}

func (e *Engine) getQubitProbability(ctx *ExecContext, qubit int) float64 {
	cs := e.findCircuitState(ctx)
	if cs == nil {
		return 0
	}
	sv := cs.StateVec
	n := len(sv)
	prob := 0.0
	for i := 0; i < n; i++ {
		if ((i >> qubit) & 1) == 1 {
			prob += cmplx.Abs(sv[i]) * cmplx.Abs(sv[i])
		}
	}
	return prob
}

// ============================================================
// Tensor Opcode Execution
// ============================================================

func (e *Engine) executeTensorOp(ctx *ExecContext, inst ir.Instruction) error {
	switch inst.Op {
	case ir.OpTensorCreate:
		size := inst.Imm
		vals := make([]Value, size)
		for i := range vals {
			vals[i] = Value{Kind: VKFloat, Float: 0}
		}
		e.push(ctx, Value{Kind: VKArray, Array: vals})

	case ir.OpTensorAdd:
		b := e.pop(ctx)
		a := e.pop(ctx)
		if len(a.Array) == len(b.Array) {
			result := make([]Value, len(a.Array))
			for i := range result {
				result[i] = addValues(a.Array[i], b.Array[i])
			}
			e.push(ctx, Value{Kind: VKArray, Array: result})
		} else {
			e.push(ctx, a)
		}

	case ir.OpTensorMul:
		b := e.pop(ctx)
		a := e.pop(ctx)
		if len(a.Array) == len(b.Array) {
			result := make([]Value, len(a.Array))
			for i := range result {
				result[i] = mulValues(a.Array[i], b.Array[i])
			}
			e.push(ctx, Value{Kind: VKArray, Array: result})
		} else {
			e.push(ctx, a)
		}

	case ir.OpTensorMatMul:
		e.pop(ctx) // b
		e.pop(ctx) // a
		e.push(ctx, Value{Kind: VKArray, Array: []Value{{Kind: VKFloat, Float: 0}}})

	case ir.OpTensorReshape:
		e.pop(ctx) // new shape
		// tensor stays on stack

	case ir.OpTensorSlice:
		e.pop(ctx) // index
		tensor := e.pop(ctx)
		if len(tensor.Array) > 0 {
			e.push(ctx, tensor.Array[0])
		} else {
			e.push(ctx, Value{Kind: VKFloat, Float: 0})
		}

	case ir.OpTensorSum:
		tensor := e.pop(ctx)
		sum := 0.0
		for _, v := range tensor.Array {
			if v.Kind == VKFloat {
				sum += v.Float
			} else if v.Kind == VKInt {
				sum += float64(v.Int)
			}
		}
		e.push(ctx, Value{Kind: VKFloat, Float: sum})

	case ir.OpTensorGrad:
		e.pop(ctx) // target
		e.push(ctx, Value{Kind: VKArray, Array: []Value{}})

	case ir.OpTensorBackward:
		tensor := e.pop(ctx)
		if e.Config.OnTensorExec != nil {
			// Find matching tensor graph
			for _, tg := range ctx.TensorGraphs {
				_, err := e.Config.OnTensorExec(ctx, tg.Graph, []Value{tensor})
				if err != nil {
					return err
				}
				break
			}
		}
	}

	return nil
}

// ============================================================
// Value Operations
// ============================================================

func addValues(a, b Value) Value {
	if a.Kind == VKInt && b.Kind == VKInt {
		return Value{Kind: VKInt, Int: a.Int + b.Int}
	}
	if a.Kind == VKFloat && b.Kind == VKFloat {
		return Value{Kind: VKFloat, Float: a.Float + b.Float}
	}
	if a.Kind == VKInt && b.Kind == VKFloat {
		return Value{Kind: VKFloat, Float: float64(a.Int) + b.Float}
	}
	if a.Kind == VKFloat && b.Kind == VKInt {
		return Value{Kind: VKFloat, Float: a.Float + float64(b.Int)}
	}
	return Value{Kind: VKNone}
}

func subValues(a, b Value) Value {
	if a.Kind == VKInt && b.Kind == VKInt {
		return Value{Kind: VKInt, Int: a.Int - b.Int}
	}
	if a.Kind == VKFloat && b.Kind == VKFloat {
		return Value{Kind: VKFloat, Float: a.Float - b.Float}
	}
	if a.Kind == VKInt && b.Kind == VKFloat {
		return Value{Kind: VKFloat, Float: float64(a.Int) - b.Float}
	}
	if a.Kind == VKFloat && b.Kind == VKInt {
		return Value{Kind: VKFloat, Float: a.Float - float64(b.Int)}
	}
	return Value{Kind: VKNone}
}

func mulValues(a, b Value) Value {
	if a.Kind == VKInt && b.Kind == VKInt {
		return Value{Kind: VKInt, Int: a.Int * b.Int}
	}
	if a.Kind == VKFloat && b.Kind == VKFloat {
		return Value{Kind: VKFloat, Float: a.Float * b.Float}
	}
	if a.Kind == VKInt && b.Kind == VKFloat {
		return Value{Kind: VKFloat, Float: float64(a.Int) * b.Float}
	}
	if a.Kind == VKFloat && b.Kind == VKInt {
		return Value{Kind: VKFloat, Float: a.Float * float64(b.Int)}
	}
	return Value{Kind: VKNone}
}

func divValues(a, b Value) Value {
	if a.Kind == VKInt && b.Kind == VKInt {
		if b.Int == 0 {
			return Value{Kind: VKInt, Int: 0}
		}
		return Value{Kind: VKInt, Int: a.Int / b.Int}
	}
	if a.Kind == VKFloat && b.Kind == VKFloat {
		if b.Float == 0 {
			return Value{Kind: VKFloat, Float: 0}
		}
		return Value{Kind: VKFloat, Float: a.Float / b.Float}
	}
	if a.Kind == VKInt && b.Kind == VKFloat {
		if b.Float == 0 {
			return Value{Kind: VKFloat, Float: 0}
		}
		return Value{Kind: VKFloat, Float: float64(a.Int) / b.Float}
	}
	if a.Kind == VKFloat && b.Kind == VKInt {
		if b.Int == 0 {
			return Value{Kind: VKFloat, Float: 0}
		}
		return Value{Kind: VKFloat, Float: a.Float / float64(b.Int)}
	}
	return Value{Kind: VKNone}
}

func valuesEqual(a, b Value) bool {
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case VKInt:
		return a.Int == b.Int
	case VKFloat:
		return a.Float == b.Float
	case VKBool:
		return a.Bool == b.Bool
	case VKString:
		return a.Str == b.Str
	default:
		return false
	}
}

func valuesLT(a, b Value) bool {
	if a.Kind == VKInt && b.Kind == VKInt {
		return a.Int < b.Int
	}
	if a.Kind == VKFloat && b.Kind == VKFloat {
		return a.Float < b.Float
	}
	if a.Kind == VKInt && b.Kind == VKFloat {
		return float64(a.Int) < b.Float
	}
	if a.Kind == VKFloat && b.Kind == VKInt {
		return a.Float < float64(b.Int)
	}
	return false
}

func castValue(v Value, targetType ir.TypeKind) Value {
	switch targetType {
	case ir.TypeInt:
		if v.Kind == VKFloat {
			return Value{Kind: VKInt, Int: int64(v.Float)}
		}
		return v
	case ir.TypeFloat:
		if v.Kind == VKInt {
			return Value{Kind: VKFloat, Float: float64(v.Int)}
		}
		return v
	case ir.TypeBool:
		if v.Kind == VKInt {
			return Value{Kind: VKBool, Bool: v.Int != 0}
		}
		return v
	default:
		return v
	}
}

// ============================================================
// Helpers
// ============================================================

func resolveFuncName(ctx *ExecContext, inst ir.Instruction) string {
	fnIdx := int(inst.Arg1)
	if fnIdx < len(ctx.Module.Symbols.Entries) {
		sym := ctx.Module.Symbols.Entries[fnIdx]
		return sym.Name
	}
	// Fallback: search function table by index
	if fnIdx < len(ctx.Module.Functions) {
		return ctx.Module.Functions[fnIdx].Name
	}
	// Use imm as direct function name hash
	for name, fn := range ctx.FuncTable {
		for i := range fn.Instructions {
			_ = i
		}
		_ = name
	}
	return fmt.Sprintf("fn_%d", fnIdx)
}

// GetOutputs returns all printed outputs from execution
func (ctx *ExecContext) GetOutputs() []string {
	return ctx.Outputs
}

// GetGlobals returns global variable values
func (ctx *ExecContext) GetGlobals() map[string]Value {
	return ctx.Globals
}

// ============================================================
// Execution Summary
// ============================================================

// ExecSummary provides a human-readable summary of execution results
type ExecSummary struct {
	InstructionsExecuted int
	FunctionsCalled      int
	GPUDispatches        int
	QPUOperations        int
	TensorOps            int
	Output               string
	Error                error
}

// Summarize produces a summary of the execution context
func Summarize(ctx *ExecContext) ExecSummary {
	summary := ExecSummary{
		Output: strings.Join(ctx.Outputs, "\n"),
	}
	return summary
}
