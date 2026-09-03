# KARKAIN UNIFIED EXECUTION MODEL — FEASIBILITY STUDY

Concept (FUTURE differentiator): one abstraction unifies Function / Task / Parallel / Remote /
GPU execution via a single call form with optional qualifiers:
```
fn calculate() { }          // plain function
task fn calculate() { }     // async task
parallel fn calculate() { } // data-parallel
gpu fn calculate() { }      // GPU kernel
remote fn calculate() { }   // network-remote
```
This is a FEASIBILITY STUDY ONLY. Do NOT implement now.

## 1. Is it architecturally viable?
YES, but only after the core compiler is sound. The pieces exist as separate sub-systems today:
- plain function: fully implemented (parser, SSA, codegen).
- gpu: `kernel` keyword + WGSL/OpenCL/SPIR-V emitters exist (not conformance-tested).
- async/task: coroutine runtime + async/await/yield tokens exist (standalone).
- parallel: no explicit support yet; needs threads+atomics (Phase 70).
- remote: actor TCP mesh + RPC C runtime exist (standalone).

## 2. Which abstractions can be unified?
A single `fn` with a qualifier (task/parallel/gpu/remote) that lowers to a shared calling
convention and IR is viable. The semantic cores (borrow checker, type checker) would be shared.

## 3. Which should remain separate?
- `gpu` and `remote` have distinct execution models (device kernels vs network) and safe to keep
  as distinct sub-models with a unified surface.
- `task` vs `thread` share a scheduler core but differ in ownership.

## 4. Compiler changes required
- Type checker extension for effect/qualifier + return/argument constraints per execution model.
- Unified IR passes aware of device/task/remote; ownership rules per model (e.g. GPU buffers,
  remote-serializable types).

## 5. Runtime changes required
- Unified scheduler + device dispatch table (the JIT engine's callback dispatch is a partial seed).
- Capability gate: `gpu`/`remote` require capabilities/FFI-safety boundaries.

## 6. What should be implemented first?
Follow the ordering: (0) fix parser OOM + type/sema foundation → (1) memory model soundness →
(2) atomics + threads → (3) wire existing actor/coroutine into pipeline → (4) GPU/quantum
conformance → THEN (5) unify surfaces. Do NOT attempt unification before steps 0-2.

## Decision
Viable, valuable, and aligns with Karkain's "one language, all hardware" differentiator — but it
is a P6/RESEARCH feature. Keep `gpu`/`remote`/`task` as separate models with a shared core until the
core is Mature (score ≥4). Re-evaluate unification at Phase 57+64 completion.
