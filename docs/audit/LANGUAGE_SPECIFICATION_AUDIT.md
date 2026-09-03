# KARKAIN LANGUAGE SPECIFICATION AUDIT

Determine whether Karkain has authoritative specs and whether implementation converges with them.

## What exists
| Doc | Authoritative? | Covers |
|-----|----------------|--------|
| SPEC.md (v0.14.0) | YES — single source of truth | syntax, tokens, keywords, types, operators, memory model sketch, quantum, macros, generics, KPM, C interop |
| ROADMAP.md | Living plan | phases, gap register (G1-G15), bug register (BUG-1..8), execution order |
| README.md | Good | architecture diagram, feature matrix, CLI reference |
| AGENTS.md | Operating rule | branch workflow, test command |

## Specification gaps (implementation exists WITHOUT spec coverage)
| Topic | Implementation | SPEC coverage |
|-------|---------------|---------------|
| Match/pattern-matching semantics | parser.go:977 | partial |
| Macros (quote/unquote/comptime, @derive) | macro.go | section 8 |
| Actor/coroutine concurrency | pkg/runtime | section 4.5/7 |
| GPU kernels (kernel/device/global_id/barrier) | codegen/gpu*,wgsl | section (partial) |
| Quantum (qreg/gate/measure, no-cloning) | codegen/qasm,qir,sema/quantum | section 4.4/7.3 |
| Memory model (ownership/borrow lifetimes) | borrow_checker.go | section 5 (sketch) |
| ABI / stability | — | MISSING |
| FFI rules | ffi.go | section 11 |

## Conflicts (implementation ↔ spec ↔ tests ↔ roadmap)
- **Parser generics**: SPEC section 9 declares monomorphized generics as a feature, but the
  parser does NOT parse `<T>` syntax (parser.go:95). Impl behind spec.
- **Borrow checker**: SPEC section 5 implies ownership/borrowing; implementation has the
  "moves never expire" (BUG-5) defect and `BorrowError.Line==0`. Behavior behind spec.
- **Match/Result**: SPEC implies Option/Result; BUG-2 (match arms compile to `1`) and BUG-4
  (`?` no-op) mean behavior does not match spec. Impl broken vs spec.
- **GPU/quantum**: SPEC L466 states emitters "not conformance-tested"; codegen has them but
  no end-to-end hardware verification. Honest partial.
- **Extension `.kark`**: Spec, README, CLI, tests updated to `.kark` (consistent after rename).

## Convergence requirement
The implementation and specification MUST eventually converge (SPEC section principles). The
largest divergences are generics (not parsed), match/Result/error (`?` no-op), and borrow
lifetimes — i.e. exactly the CRITICAL gaps (C2-C5). Priority: bring codegen+TypeSystem up to
SPEC, or explicitly downgrade SPEC for features that are not V1.

## Recommendation
Adopt a CONFORMANCE test matrix that links each SPEC section to tests (Phase 69). Until then,
mark the divergent features (generics syntax, match correctness, `?`, borrow lifetimes) as
NOT spec-conformant in the gap register so nothing is falsely claimed.
