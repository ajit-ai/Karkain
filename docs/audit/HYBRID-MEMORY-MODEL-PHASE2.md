# Karkain Hybrid Memory Model — Phase 2 Design (Reference Counting)

**Status: SLICE 1 COMPLETE (runtime foundation).** Phase 2 adds shared ownership via
reference counting. This document covers slice 1: the Karkain-owned RC runtime
primitives. Compiler integration (automatic retain/release insertion, an `Rc[T]`
surface, borrow-checker integration) is the follow-up slice. No GC, no cycle
collection, no weak references in this slice.

## Scope

Slice 1 delivers:
- `runtime/rc/c/karkain_rc.{h,c}`: `karkain_rc_new`, `karkain_rc_retain`,
  `karkain_rc_release`, `karkain_rc_strong_count`, `karkain_rc_payload`.
- Atomic (thread-safe) refcount using the same GNU-atomic model as the
  concurrency runtime.
- A direct C-toolchain gate `pkg/runtime/phase_v1224_rc_test.go`.

## Out of scope (later slices)

- `Rc[T]`/`Arc[T]` language surface and codegen.
- Automatic retain/release insertion at scope boundaries.
- Cycle collection, weak references, `Arc`.
- Borrow-checker integration.

## Rationale

Phase 1 (v1.2.23) covers ~80% of memory bugs. Phase 2 covers shared mutable
state / cyclic structures. Starting with the runtime primitives keeps the
change small, testable and honest; the compiler-facing surface follows once the
runtime is proven.