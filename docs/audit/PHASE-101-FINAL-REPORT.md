# Phase 101 — Libc-Free Runtime Foundation — Final Report

**Status:** COMPLETE
**Date:** 2026-09-09
**Branch:** `develop` -> `main` (committed and merged)

## Objective

Make the Karkain runtime progressively independent of libc and libm by establishing
Karkain-owned memory allocation (`runtime/freestanding/`) and a raw operating-system
interface, while keeping every existing program compiling and running on both the Go
engine and the self-hosted `kcc` engine.

## What Was Implemented

### 1. Karkain Arena Allocator (`runtime/freestanding/karkain_memory.c`)
Runtime-owned page/arena allocation with alloc/reset/release operations. Replaces
libc `malloc`/`free` for the freestanding paths that link against it.

### 2. Raw Syscall Layer (`runtime/freestanding/karkain_platform.h`, `karkain_platform.c`)
Small Karkain-owned OS abstraction (`karkain_platform_*`) providing the operations the
runtime requires without routing them through libc. Isolated by platform so additional
targets can be added later.

### 3. libc Independence
The freestanding runtime compiles and links with **no libc**. I/O (`karkain_io.c`),
string ops (`karkain_string.c`), and the runtime shell (`karkain_runtime.c`) build on
the platform layer only. A gate test (`pkg/runtime/phase101_freestanding_test.go`)
compiles these with `gcc` freestanding-style and runs a hello program successfully.

### 4. libm Independence
Only the math primitives the runtime actually requires are provided
(`runtime/freestanding/karkain_math.c`). No general math library was built.

### 5. GMP / BigInt Boundary
GMP remains a **temporary, isolated boundary** for the compiler-runtime bootstrap; the
runtime architecture admits a Karkain-owned BigInt replacement (deferred to Phase 109)
without redesign. The boundary is documented in the freestanding layer's doc comments.

### 6. Runtime Compatibility — the Phase 100 debugger registry (frame stack)
- Runtime frame stack (`karkain_frame_enter/leave`, `karkain_set_line`,
  `KARKAIN_MAX_FRAMES`) with stack-trace reporting on runtime errors, mirrored in both
  engines. Verified byte-identical diagnostics on `examples/runtime_errors/stack_chain.kark`
  (frames `inner:2 / outer:5 / main:9`).
- Go side: `pkg/codegen/codegen.go` legacy emitter + `pkg/ir/ssa/ssa.go`
  (`Function.Line`) + `pkg/codegen/emit_ir.go`.
- Self-hosted side: `src/compiler/codegen.kark` (frame helpers, `FuncDecl` prologue,
  return wrapper `{ Value _karkain_fret = ...; karkain_frame_leave(); return _karkain_fret; }`,
  epilogue `karkain_frame_leave()`).

### Runtime Compatibility blocker fixed
A fresh bootstrap revealed a latent self-hosted-crash: `kcc check` raised
`array index out of range` in the checker's `getNodeType` for any program containing a
**bare `return`** (e.g. the compiler's own `if (x) { return }`), because `parseReturn`
yields an empty value node and `checkStmt`/`checkExpr` dereferenced it. Fixed with
empty-node guards in `src/compiler/checker.kark`. `parseFunc` func-token line fix in
`src/compiler/parser.kark` was also restored (working-tree reset had dropped it) and
keeps ancestor frame line parity.

## Tests Performed and Results

| Test | Result |
|------|--------|
| `kcc check` on all 9 self-hosted compiler sources | PASS (exit 0, `[ok]`) |
| `go test ./pkg/cli/ -run 'TestPhase99\|TestPhase100\|TestPhase101' -count=1` | PASS (104.6s; includes `CompilerSourcesTypeCheck`, runtime-error parity, stack-chain parity) |
| `go test ./pkg/codegen/ -run TestPhase101 -count=1` | PASS |
| `go test ./pkg/runtime/ -run TestPhase101 -count=1` (freestanding gate) | PASS |
| kcc vs Go stack-chain parity (`stack_chain.kark`) | PASS (identical diagnostics) |
| Bare-`return` program check + run via kcc | PASS |

## Remaining Direct libc / libm / GMP Dependencies (intentional)

- **GMP**: retained as the isolated BigInt kernel for the compiler-runtime bootstrap;
  intended to be replaced by a Karkain-owned BigInt implementation (Phase 109).
- **libc/libm**: no longer required by the freestanding runtime; host `gcc` still links
  `-lm -lgmp` for the self-hosted compiler bootstrap and the codegen-embedded runtime
  remains host-bound until a later phase moves it onto `runtime/` layers.

## Known Pre-Existing Limitation (NOT a Phase 101 blocker)

Closure (`fn`/lambda) code generation is broken in **both** engines in this tree
(Go: `ClosureEnv_*` typedef is never emitted; kcc: no lambda support exists in
`codegen.kark`). No conformance/probe/corpus file uses lambdas, so no targeted gate
covers it. It predates Phase 101, is documented, and is out of scope for this phase.

## Commit Hash

(Included in the Phase 101 commit on `develop` merged into `main`.)