# Karkain C ABI & Runtime Boundary Specification

Authoritative record of the distinct C runtimes in the Karkain toolchain and
the ABI contract each one honors. It exists to **prevent silent drift and
mis-linking**: the three runtimes serve different purposes, must NOT be merged,
and must NOT be linked against one another.

## 1. The three runtimes (do not merge)

| # | Runtime | Location | Purpose | C std (gcc flag) | Linked into |
|---|---------|----------|---------|------------------|-------------|
| 1 | **Codegen scalar preamble** | `pkg/codegen/codegen.go::generateCHeader()` | Native scalar/atom/SIMD/quantum/socket runtime **embedded into every generated program** | `-std=c2x` | any binary produced by `build`/`run` via the compiler |
| 2 | **CPU tensor reference** | `pkg/backend/cpu/runtime_c.go::TensorCRuntime` | **Tensor IR reference oracle** for the Math/Tensor/NPU chain — correctness reference for every accelerator | `-std=c2x` | temp `kernel-*.c` programs run by the CPU backend |
| 3 | **Self-hosting `Value` runtime (legacy)** | `src/compiler/runtime.c` | Pointer-based dynamic `Value` type system for the Karkain-written compiler | `gcc -c99` (stale) | **nothing active** — legacy / superseded |

### Why they must stay separate

- **ABIs differ.** #1 holds scalar/quantum values by value and exposes
  `Complex`, `QuantumRegister`, socket + SIMD surfaces. #2 holds `Tensor`
  structs with refcounts and broadcasting. #3 holds a tagged-union `Value*`.
  They share no common type or calling convention.
- **Linking them conflicts.** The generated C already defines every runtime
  helper by value (see #1). Linking #3's `main.c` output would produce
  duplicate-symbol errors. The bootstrap `compileWithGCC` therefore deliberately
  does **not** link `runtime.c`; the parameter that once suggested otherwise has
  been removed (see `pkg/bootstrap/bootstrap.go`).

## 2. The codegen scalar preamble contract (#1)

Emitted verbatim at the top of every generated C program (gcc without `-x c`
input dispatch issues, `-std=c2x`). Responsibilities:

- **Includes:** `stdio, stdlib, string, stdint, stdalign, math, time, gmp`,
  plus `stdatomic`, SIMD (`immintrin`/portable vector), and platform
  socket/winsock headers.
- **Quantum types:** `Complex`, `QuantumRegister` and their helpers
  (`complex_add/mul/scale`, `qreg_init`, ...).
- **Ownership:** helpers are value/stack oriented; the program is a single
  translation unit with no external runtime dependency beyond libc, libm, gmp.

## 3. The CPU tensor reference contract (#2)

`pkg/backend/cpu` compiles a small C program (unique temp files) per
`Execute`, embedding `TensorCRuntime`, then gcc `-std=c2x -O2 -lm`.
Responsibilities:

- **`Tensor`** struct with shape/strides/dtype/refcounted data.
- Ops: elementwise add/sub/mul/div, matmul, relu, sigmoid, tanh, softmax,
  transpose, reshape, reduce_sum, create/copy. Row-major, rank ≤ 8.
- Because it is the **oracle**, GPUs/NPUs must match its numeric behavior; it
  is the reference for golden/differential testing.

## 4. The self-hosting runtime (#3): status

`src/compiler/runtime.c` was used by early bootstrap stages to give the
Karkain-written compiler a dynamic `Value` layer. Current generated code
(`main.kark` → C) emits a **by-value** runtime via #1 instead. As a result #3
is no longer linked at any stage and is retained only as historical evidence.
It is **not** a live ABI surface.

## 5. Determinism guarantees (relevant to ABI)

- gcc embeds a PE `TimeDateStamp` from the wall clock unless
  `SOURCE_DATE_EPOCH` is set; the bootstrap sets `reproducibleEpoch =
  "1072915200"` for bitwise-identical stage2/stage3 binaries.
- Karkain codegen is deterministic across lineage (stage2-C == stage3-C).

## 6. Contract for future changes

1. Never link #1 and #3 together. If #3 is ever revived, it must be re-based
   to the by-value ABI of #1 or re-architected; do not mix pointer and by-value
   helpers.
2. Any new accelerator's numerics must be checked against #2 (the CPU oracle).
3. Any change to the embedded header (#1) must regenerate the self-hosted
   compiler (stage2/3) and re-run the full bootstrap suite to keep the cycle
   green.