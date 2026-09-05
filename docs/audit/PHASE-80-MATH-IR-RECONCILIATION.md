# PHASE 80 — Math IR Reconciliation (Decision Record)

**Phase:** 80 (SIMD / Vector Execution Architecture)
**Status:** COMPLETE
**Prerequisite addressed:** Phase 79 audit §12.1 ("Reconcile Math IR").

---

## 1. Problem

The Phase 79 audit found `karkain/pkg/math` (the Math IR) is a dead leaf:
**imported by no package** in the module (verified three ways:
`go list -deps ./...` shows only the self-node; `grep '"karkain/pkg/math"'`
finds only the Phase 79 audit docs; no `.go` source imports it). This leaves
the ownership of math semantics ambiguous and its optimizer/evaluator
unreachable from any product path.

## 2. Options Considered

| Option | Verdict |
|---|---|
| A. Wire Math IR into the compiler pipeline | REJECTED for Phase 80 — high churn, risks destabilizing the stable lexer→parser→sema→SSA→C23 path for zero immediate value; Tensor IR already owns the shared backend-facing graph |
| B. Deprecate / delete `pkg/math` | REJECTED — it is a working, tested, Karkain-owned semantic reference (evaluator `Eval`, optimizer `Optimize`, validator `Validate`, builder + printer); deleting capability is regressive |
| C. Mark as **reference caller-free evaluator** | **CHOSEN** — zero-risk, authoritative, keeps the code honest |

## 3. Decision

`pkg/math` is the **reference caller-free evaluator**: the standalone,
dependency-free oracle for Karkain scalar/math semantics. It is intentionally
not wired into the pipeline and has no importers; Tensor IR is the single
shared graph the execution backends consume. `pkg/math` remains fully tested
and available for two future uses:

1. **Semantic source for vector/SIMD op reference values** (differential
   parity oracles), and
2. **Karkain-owned scalar lowering target** if a future phase introduces a
   math-level optimization pipeline.

Recorded in `pkg/math/ir.go` package doc comment (2026-09-05).

## 4. Evidence

- `go list -deps ./...` → `karkain/pkg/math` appears exactly once (self node).
- No `pkg/math` importer in the entire module.
- `pkg/math` tests remain green in the full suite.

## 5. Follow-up Options (NOT in Phase 80 scope)

- Wire `pkg/math` as the scalar semantic oracle for a future vector-IR
  validation pass.
- Adopt `pkg/math` nodes as the Karkain-owned MLIR-dialect lowering source.