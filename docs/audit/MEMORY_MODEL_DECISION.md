# KARKAIN MEMORY MODEL DECISION

Architectural review of the memory model. Non-destructive — this recommends a direction and
does NOT implement it. The goal is "small core, safe by default, fast by design."

## What already exists (evidence)
- **Ownership + borrowing** (Rust-style) is the foundation already built: `pkg/sema/borrow_checker.go`
  — lexical scope stack, shadowing, `&x`/`&mut x` (TokenAmp), `move(x)` (TokenMove), `raw` (TokenRaw),
  linear types (`linear` keyword). Wired into Run/Build (`commands.go:47,86`).
- **Manual allocation** primitives: `alloc` / `free` / `addr` keywords; C backend maps to `karkain_alloc`.
- **Escape analysis** foundation: `pkg/parser/escape.go`, `MarkVarEscaping`, capture analysis.
- **No GC** exists anywhere. No `defer`/RAII (ROADMAP G8).
- **Known defect**: moves never expire on scope exit (BUG-5); `BorrowError.Line` always 0.

## The three candidate models (per prompt)

### MODEL A — Garbage Collected
- Complexity: runtime GC (tracing) + write barriers. High.
- Performance: pause/throughput overhead; contradicts "fastest" goal.
- Fit: does NOT match the existing ownership architecture. Would require a large rewrite of
  sema and codegen.
- Verdict: **REJECT.** No GC. Directly contradicts Pillar 3 (FAST) and existing Rust-style foundation.

### MODEL B — Full Ownership & Borrowing
- Complexity: complete borrow checker (fix BUG-5), lifetimes, move semantics, drop/RAII.
- Performance: zero runtime overhead beyond manual; ideal for "fastest" and systems.
- Fit: matches existing implementation (Phase 51 borrow checker, move/borrow tokens).
- Cost: developer learning curve; battles with borrow checker.
- Verdict: **ADOPT as the primary model.** Finish what's already built to production soundness.

### MODEL C — Hybrid Deterministic (ownership primary + opt-in GC/ARC for specific types)
- Complexity: two models to implement/maintain; subtle interaction bugs.
- Performance: retains zero-cost for ownership; GC only where opted in.
- Fit: extensible, but heavier than B and risks violating Pillar 1 (SIMPLE) / Rule 8
  ("do not add multiple competing programming models unnecessarily").
- Verdict: **DEFER until Model B is complete and proven.** If ergonomics demand it later,
  revisit as a narrow opt-in (e.g. ARC-like `shared` types), per Capability/Effect gating — never
  as a default.

## Recommendation
**Primary: Model B — full ownership/borrowing with deterministic, non-GC memory.**
Add, in dependency order:
1. Fix BUG-5 (borrows/moves expire with scope) — soundness prerequisite.
2. Populate `BorrowError.Line` — usable diagnostics.
3. Add `defer`/RAII (G8) for resource destruction (a small, high-value addition that fits
   ownership, unlike GC).
4. Complete escape analysis → stack allocation for non-escaping values (already foundation'd).
5. Define the `unsafe` boundary explicitly for `raw`/FFI (P2 systems capability).

## Decisions made in this document
- NO garbage collection will be introduced (rejects Model A, defers Model C default).
- Ownership/borrow is the committed model; the existing work must be completed to production
  soundness before expansion (Phase 63).

## Rationale mapped to prompt constraints
- "Do not copy Rust automatically" → we keep Karkain's own token set (`move`, `raw`, `linear`,
  `addr`, `packed`) and don't import Rust lifetimes/borrow syntax wholesale; we complete the
  existing checker rather than copying Rust's.
- Rule 5 (no GC automatically) → honored: GC rejected.
- Rule 8 (no competing models) → honored: one primary model (ownership); Model C deferred.
