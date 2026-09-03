# Karkain Math IR + Tensor IR + NPU Architecture

**Date:** 2026-09-03
**Status:** DESIGN — locked for implementation

---

## Design Principles

1. **Karkain Independence** — Math IR and Tensor IR are Karkain-owned, not ONNX/MLIR/vendor-specific
2. **No Tensor keyword** — NPU/math ops work on existing `array` types
3. **No Google TPU** — NPU targets Intel/Qualcomm/Apple/AMD/Arm only
4. **CPU is reference backend** — correctness oracle before any accelerator
5. **NPU is a backend** — not the foundation, not the language
6. **No dead code** — every component must have a caller and tests

---

## Architecture

```
                         KARKAIN
                            |
                            v
                   Language Semantics
                            |
                            v
                  Mathematical Semantics
                            |
                            v
                       MATH IR (Karkain-owned)
                            |
                +-----------+-----------+
                |           |           |
             Algebra     Calculus    Numerical
                |           |           |
                +-----------+-----------+
                            |
                            v
                       TENSOR IR (Karkain-owned)
                            |
             +--------------+--------------+
             |              |              |
           Scalar         Matrix         Tensor
             |              |              |
             +--------------+--------------+
                            |
                       Autodiff
                            |
                            v
                   Execution Planner
                            |
          +-----------------+-----------------+
          |                 |                 |
         CPU               GPU               NPU
          |                 |                 |
       Native/SIMD        WGSL          NPU Adapter
          |                 |                 |
          +-----------------+-----------------+
                            |
                            v
                         RESULT
```

---

## Phase 71: Math IR Foundation

### What is Math IR?

Math IR is a Karkain-owned intermediate representation for mathematical computation.
It is NOT NumPy, NOT MLIR, NOT SymPy, NOT a vendor format.

### Math IR Node Types

```text
Constants:     Integer, Float, Boolean, Complex, Rational
Variables:     Variable, Parameter, Temporary
Arithmetic:    Add, Subtract, Multiply, Divide, Modulo, Negate, Power
Functions:     sqrt, exp, log, sin, cos, tan, asin, acos, atan, abs, min, max, floor, ceil
Comparison:    Equal, NotEqual, Less, Greater, LessEqual, GreaterEqual
Logic:         And, Or, Not
Calculus:      Derivative, Integral (future)
Algebraic:     Summation, Product, Equation (future)
```

### Math IR Metadata

Each node carries:
```text
numeric_type    — int, float, complex
precision       — bit width
exactness       — exact vs approximate
shape           — for tensor-aware nodes
rank            — dimensionality
constant_status — compile-time known?
differentiable  — can we differentiate?
purity          — side-effect free?
```

### Key Files (to create)

```text
pkg/math/ir.go          — Math IR data model, node types
pkg/math/builder.go     — Builder/factory API
pkg/math/printer.go     — Debug/human-readable output
pkg/math/validate.go    — Well-formedness checks
pkg/math/optimize.go    — Safe algebraic optimizations
pkg/math/eval.go        — Reference CPU evaluator
pkg/math/ir_test.go     — Unit tests
```

---

## Phase 72: Tensor IR Foundation

### What is Tensor IR?

Tensor IR is a Karkain-owned intermediate representation for tensor computation.
It sits ABOVE the SSA IR and BELOW the execution backends.

### Tensor IR Hierarchy

```text
Scalar    — rank 0, single value
Vector    — rank 1, 1D array
Matrix    — rank 2, 2D array
Tensor    — rank N, N-dimensional array
```

### Tensor IR Metadata

```text
element_type  — i32, i64, f32, f64, complex
shape         — [1024, 768] or symbolic
rank          — number of dimensions
layout        — row-major, column-major, strided (extensible)
device        — cpu, gpu, npu
memory        — ownership, lifetime, alignment
```

### Shape System

Support four shape categories:
```text
Static Shape   — known at compile time: [32, 64]
Dynamic Shape  — some dims unknown: [32, ?]
Symbolic Shape — symbolic variables: [N, M]
Unknown Shape  — fully dynamic: [?]
```

### Core Tensor Operations

```text
Create          — allocate tensor with shape
Constant        — constant tensor from values
Load            — read element
Store           — write element

Add             — element-wise add
Subtract        — element-wise subtract
Multiply        — element-wise multiply
Divide          — element-wise divide

MatMul          — matrix multiply
Transpose       — permute dimensions

Reshape         — change shape
Broadcast       — expand dimensions
Slice           — extract sub-tensor
Concat          — concatenate tensors
Reduce          — reduce along axis

Elementwise     — apply function to each element
Unary           — single-operand ops
Binary          — two-operand ops
```

### Key Files (to create)

```text
pkg/tensor/ir.go        — Tensor IR data model
pkg/tensor/types.go     — TensorType, Shape, ElementType
pkg/tensor/ops.go       — Operation definitions
pkg/tensor/shape.go     — Shape validation, broadcasting
pkg/tensor/builder.go   — Builder API
pkg/tensor/lower.go     — Lowering to SSA IR
pkg/tensor/ir_test.go   — Unit tests
```

---

## Phase 73: CPU Reference Backend

### Purpose

CPU backend is the **correctness oracle**. Every tensor operation must produce
the same result on CPU as on any accelerator.

### CPU Backend Architecture

```text
Tensor IR
    |
    v
CPU Backend
    |
    v
C23 Runtime Calls
    |
    v
libc + BLAS (optional)
    |
    v
Correct Result
```

### CPU Runtime Operations

```c
// Tensor creation
Tensor* tensor_create(int ndim, int* shape, int dtype);
void tensor_destroy(Tensor* t);

// Element access
Value tensor_get(Tensor* t, int* indices);
void tensor_set(Tensor* t, int* indices, Value v);

// Core operations
Tensor* tensor_add(Tensor* a, Tensor* b);
Tensor* tensor_mul(Tensor* a, Tensor* b);
Tensor* tensor_matmul(Tensor* a, Tensor* b);
Tensor* tensor_relu(Tensor* x);
Tensor* tensor_softmax(Tensor* x, int axis);
Tensor* tensor_reshape(Tensor* t, int* new_shape);
Tensor* tensor_transpose(Tensor* t, int* axes);
Tensor* tensor_slice(Tensor* t, int* starts, int* stops);
Tensor* tensor_concat(Tensor** tensors, int n, int axis);
Tensor* tensor_reduce_sum(Tensor* t, int axis);

// Broadcasting
Tensor* tensor_broadcast(Tensor* t, int* target_shape);
```

### Key Files (to create)

```text
pkg/backend/cpu/runtime.go    — CPU backend codegen
pkg/backend/cpu/ops.go        — Operation implementations
pkg/backend/cpu/broadcast.go  — Broadcasting logic
pkg/backend/cpu/tensor.c      — C runtime (embedded)
pkg/backend/cpu/cpu_test.go   — Correctness tests
```

---

## Phase 74: Autodiff Integration

### Existing Autodiff (verified in audit)

```text
pkg/sema/autodiff.go (648 lines) — WORKS, NOT CONNECTED
  - ADGraph, ADNode DAG
  - ShapeChecker for matmul, relu, softmax, conv2d, transpose
  - BuildBackwardPass() — reverse-mode AD
  - Gradient generators for all 5 ops
```

### Integration Target

```text
Math/Tensor IR
       |
       v
Autodiff Transformation
       |
       v
Gradient IR
       |
       v
CPU Backend (reference)
       |
       v
GPU Backend (optional)
       |
       v
NPU Backend (optional)
```

### Key Files (to modify/create)

```text
pkg/sema/autodiff.go      — EXTEND: work on Tensor IR, not AST
pkg/tensor/gradient.go    — CREATE: gradient propagation through Tensor IR
pkg/backend/cpu/grad.go   — CREATE: CPU gradient execution
pkg/tensor/grad_test.go   — CREATE: gradient correctness tests
```

---

## Phase 75: Backend Abstraction

### Backend Interface

```go
type Backend interface {
    Name() string
    Capabilities() Capabilities
    Supports(op Operation, dtype DataType, shape Shape) bool
    Execute(program *Program) Result
}

type Capabilities struct {
    SupportedDtypes  []DataType
    MaxRank          int
    MaxDimensions    []int
    SupportedOps     []Operation
    MemoryLimit      int64
    AlignmentReqs    int
    SupportedLayouts []Layout
}
```

### Backend Selection

```text
Operation arrives
    |
    v
Check NPU capabilities
    |
    +--- NPU supports it? --> NPU backend
    |
    +--- GPU supports it? --> GPU backend
    |
    +--- CPU always works --> CPU backend (fallback)
```

### Key Files (to create)

```text
pkg/backend/backend.go      — Backend interface
pkg/backend/planner.go      — Execution planner
pkg/backend/dispatch.go     — Backend dispatch logic
pkg/backend/planner_test.go — Unit tests
```

---

## Phase 76: GPU/WGSL Integration

### Existing WGSL Codegen (verified in audit)

```text
pkg/codegen/wgsl.go (167 lines) — CONNECTED, WORKS for kernel decl
pkg/codegen/tensor_wgsl.go (213 lines) — matmul/relu kernels WORK, graph gen STUB
```

### Integration Target

```text
Tensor IR
    |
    v
GPU Backend (implements Backend interface)
    |
    v
WGSL Codegen (existing tensor_wgsl.go)
    |
    v
WebGPU Runtime
```

### Key Files (to modify/create)

```text
pkg/backend/gpu/wgsl_backend.go  — CREATE: GPU backend implementing Backend interface
pkg/codegen/tensor_wgsl.go       — EXTEND: complete graph generation (currently stub)
pkg/backend/gpu/gpu_test.go      — CREATE: GPU backend tests
```

---

## Phase 77: NPU Abstraction + Backends

### NPU Backend Interface

```go
type NPUBackend interface {
    Backend
    DetectHardware() []NPUDevice
    Compile(model *Program) NPUProgram
    Execute(program NPUProgram, inputs map[string]Tensor) Result
}

type NPUDevice struct {
    Name        string
    Vendor      string  // "intel", "qualcomm", "apple", "amd", "arm"
    Capabilities Capabilities
    Memory      int64
}

type NPUProgram struct {
    Vendor    string
    Blob      []byte  // compiled binary/context
    Metadata  map[string]interface{}
}
```

### Vendor Adapters

```text
Intel NPU    — pkg/npu/intel/openvino.go    — OpenVINO runtime
Qualcomm NPU — pkg/npu/qualcomm/qnn.go      — QNN/HTP backend
Apple NPU    — pkg/npu/apple/coreml.go       — CoreML framework
AMD NPU      — pkg/npu/amd/vitisai.go        — VitisAI EP
Arm NPU      — pkg/npu/arm/ethos.go          — Ethos-U compiler
```

### NPU Fallback

```text
Tensor Operation
      |
      v
Can NPU execute?
      |
   +--+--+
   |     |
  YES    NO
   |     |
  NPU   CPU/GPU
```

No Karkain program becomes invalid because NPU is unavailable.

### Key Files (to create)

```text
pkg/npu/npu_backend.go      — NPUBackend interface
pkg/npu/npu_device.go       — Device detection
pkg/npu/npu_capabilities.go — Capability model
pkg/npu/intel/openvino.go   — Intel adapter
pkg/npu/qualcomm/qnn.go     — Qualcomm adapter
pkg/npu/apple/coreml.go     — Apple adapter
pkg/npu/npu_test.go         — Unit tests (with mock backend)
```

---

## Phase 78: NPU Optimization

### Operator Fusion

Fuse multiple operations into single NPU kernel:

```text
Conv2d + ReLU + BatchNorm  -->  FusedConv
MatMul + Add + ReLU        -->  FusedLinear
```

### Memory Planning

```text
DDR <--> On-chip (TCM/scratchpad/L1) data movement
Double buffering for pipeline parallelism
Memory layout optimization (NCHW vs NHWC)
```

### Quantization

```text
FP32 --> INT8 (per-channel scales)
FP32 --> INT4 (weight-only)
Mixed precision dispatch
Calibration dataset support
```

### MLIR Codegen (for kernel-level control)

```text
Tensor IR
    |
    v
MLIR Dialect (Karkain-owned)
    |
    v
Vendor-specific MLIR lowering
    |
    v
Vendor binary / context blob
```

### Key Files (to create)

```text
pkg/npu/fusion.go           — Operator fusion passes
pkg/npu/memory.go           — Memory planning
pkg/npu/quantize.go         — Quantization algorithms
pkg/npu/mlir_dialect.go     — MLIR dialect definition
pkg/npu/mlir_lowering.go    — MLIR lowering passes
pkg/npu/opt_test.go         — Optimization tests
```

---

## File Structure Summary

```
karkain/
├── pkg/math/                    # Phase 71: Math IR
│   ├── ir.go
│   ├── builder.go
│   ├── printer.go
│   ├── validate.go
│   ├── optimize.go
│   ├── eval.go
│   └── ir_test.go
├── pkg/tensor/                  # Phase 72: Tensor IR
│   ├── ir.go
│   ├── types.go
│   ├── ops.go
│   ├── shape.go
│   ├── builder.go
│   ├── lower.go
│   ├── gradient.go             # Phase 74: autodiff integration
│   └── ir_test.go
├── pkg/backend/                 # Phase 73+75: Backends
│   ├── backend.go              # Interface
│   ├── planner.go              # Execution planner
│   ├── dispatch.go             # Backend dispatch
│   ├── cpu/                    # Phase 73: CPU reference
│   │   ├── runtime.go
│   │   ├── ops.go
│   │   ├── broadcast.go
│   │   ├── tensor.c
│   │   ├── grad.go             # Phase 74: gradient execution
│   │   └── cpu_test.go
│   └── gpu/                    # Phase 76: GPU/WGSL
│       └── wgsl_backend.go
├── pkg/npu/                     # Phase 77+78: NPU
│   ├── npu_backend.go
│   ├── npu_device.go
│   ├── npu_capabilities.go
│   ├── fusion.go               # Phase 78
│   ├── memory.go               # Phase 78
│   ├── quantize.go             # Phase 78
│   ├── mlir_dialect.go         # Phase 78
│   ├── mlir_lowering.go        # Phase 78
│   ├── intel/openvino.go       # Phase 77
│   ├── qualcomm/qnn.go         # Phase 77
│   ├── apple/coreml.go         # Phase 77
│   └── npu_test.go
└── docs/
    ├── KARKAIN-MATH-REPOSITORY-ASSESSMENT.md
    ├── KARKAIN-MATH-ARCHITECTURE.md
    └── KARKAIN-NPU-ARCHITECTURE.md
```

---

## Success Criteria

| Phase | Metric | Target |
|-------|--------|--------|
| 71 | Math IR nodes, builder, printer, optimizer, evaluator | All tests pass |
| 72 | Tensor IR, shape validation, core ops, lowering to SSA | All tests pass |
| 73 | CPU backend executes tensor ops correctly | Matches expected results |
| 74 | Autodiff produces correct gradients through Tensor IR | Matches hand-computed derivatives |
| 75 | Backend abstraction dispatches to CPU/GPU/NPU | All backends implement interface |
| 76 | GPU backend generates working WGSL from Tensor IR | matmul runs on GPU |
| 77 | NPU backend detects hardware, compiles model, executes | Intel/Qualcomm adapters work |
| 78 | Fusion, memory planning, quantization improve performance | 1.5-3x speedup over unfused |

---

## Constraints

1. **Self-hosting cannot break** — all changes through Go codegen first
2. **No external deps in core** — Math IR and Tensor IR are Karkain-owned
3. **No dead code** — every component has caller + tests
4. **Deterministic builds** — all optimization passes are deterministic
5. **CPU first** — reference backend before any accelerator
6. **Graceful fallback** — no NPU = CPU execution, never an error
