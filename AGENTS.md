# Karkain Development Conventions

## Branch Workflow (MANDATORY RULE)

After EVERY Phase completion and successful test run:
1. Commit all changes to `develop` branch
2. Merge `develop` into `main` branch
3. Push both branches to origin

This ensures `main` always reflects the latest working state.
NEVER skip this step. This is a hard rule, not optional.

## Roadmap

See `ROADMAP.md` for the complete development plan (Phases 50–79+).
Current phase: **post-79** — Phases 50–79 complete.
Also completed: **51** — Borrow Checker Lexical Scoping
(scope-stack identity hardened for shadowing, borrow reversion via declaring scope,
use-after-scope-end diagnostics, escape-analysis flag wiring, Groups A–I tests).
Last completed: **79** — Compiler Integrity, IR Architecture & Self-Hosting
Readiness Audit (evidence-based audit: pipeline, dependency map, Math/Tensor/SSA
IR, CPU/GPU/NPU parity, determinism, optimization boundaries, tests,
BUG-1..8 regression, self-hosting readiness; Phase 80 gate = READY WITH
PREREQUISITES; docs under `docs/audit/PHASE-79-*`).
Prior completed: **78** — NPU Optimization (fusion, memory planning, INT8/INT4 quantization, MLIR codegen);
**70-78** Math/Tensor/NPU chain (see below); **52-69** value/SSA/closures/slices/self-hosting/actors/GPU/quantum/stdlib/borrow/optimizer.

### Completed: Math/Tensor/NPU Chain (Phases 71–78)

```
71 (Math IR) → 72 (Tensor IR) → 73 (CPU Backend)
                                      ↓
                                 74 (Autodiff Integration)
                                      ↓
                                 75 (Backend Abstraction)
                                    ↓        ↓
                               76 (GPU)   77 (NPU) → 78 (NPU Opt)
```
All 8 phases built, tested (full suite green), committed and pushed to `main`.
Backends: CPU reference (`pkg/backend/cpu`, embedded C23 runtime, correct GCC E2E),
GPU/WGSL (`pkg/backend/gpu`), NPU abstraction + 5 vendor adapters (`pkg/npu/*`).
NPU optimization passes: fusion, memory planning, quantization, Karkain-owned MLIR dialect.

### Phase Dependency Chain (Math/Tensor/NPU)

```
71 (Math IR) → 72 (Tensor IR) → 73 (CPU Backend)
                                      ↓
                                 74 (Autodiff Integration)
                                      ↓
                                 75 (Backend Abstraction)
                                    ↓        ↓
                               76 (GPU)   77 (NPU) → 78 (NPU Opt)
```

### Key Design Decisions

1. **No Tensor keyword** — NPU/math ops work on existing `array` types
2. **No Google TPU** — NPU targets Intel/Qualcomm/Apple/AMD/Arm only
3. **Math IR is Karkain-owned** — not ONNX, not MLIR, not vendor-specific
4. **Tensor IR is Karkain-owned** — not NumPy, not PyTorch
5. **CPU is reference backend** — correctness oracle before any accelerator
6. **NPU is a backend** — not the foundation, not the language

## Guiding Principles

1. **Maturity over features** — depth, correctness, performance, verification before expansion
2. **Semantic foundations first** — value representation, ownership, lifetimes, IR before features
3. **Working > ambitious** — fix broken fundamentals before adding new capabilities
4. **Incremental verification** — every phase must compile, pass E2E, pass all tests

## Testing

Run the full test suite before committing:
```
go test ./pkg/lexer/... ./pkg/parser/... ./pkg/codegen/... ./pkg/pm/... -count=1
```

All tests must pass before committing.


