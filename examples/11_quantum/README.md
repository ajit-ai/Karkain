# 11 — Quantum

**Status: Planned (infrastructure in progress)**

Karkain's quantum story is explicitly *not* MicroQuantum. The architecture
boundary is:

```
Karkain (.kark)
   |
   v  (future: Karkain quantum interface/integration)
Karkain quantum interface
   |
   v
MicroQuantum / future quantum backends
```

Nothing in this category reimplements MicroQuantum.

## What exists today

Quantum *machinery* exists inside the compiler as infrastructure only:

| Piece | Location | Status |
|-------|----------|--------|
| Parser AST: `CircuitDecl`, `QubitAssignStmt`, `QPUOpExpr`, `MeasureExpr` | `pkg/parser/ast.go` | defined |
| HIR quantum nodes + types | `pkg/ir/hir` | defined |
| Quantum safety analyzer (no-cloning, lifecycle) | `pkg/sema/quantum_*.go` | unit-tested |
| OpenQASM 3 generator | `pkg/codegen/qasm.go` | generator only |
| OpenPulse generator | `pkg/codegen/openpulse.go` | generator only |
| QEC/surface-code generators | `pkg/codegen/qec.go` | generator only |
| WGSL simulator bridge | `pkg/sema/quantum_sim_bridge.go` | validator |

**Verified fact:** the lexer and parser do **not** tokenize `circuit`, `qubit`,
`qpu` or `measure` — `karkain check` rejects a `circuit` declaration with
`parse error`. Quantum syntax is therefore NOT runnable end-to-end, on either
engine, today.

## Roadmap

- wire `circuit`/`qubit`/`qpu` through the parser (smallest real foundation)
- quantum simulator runtime for generated C
- examples: Bell state, GHZ, Deutsch-Jozsa, Grover (as they become runnable)
- backend integration surface toward MicroQuantum — preserving the boundary

## Run

Nothing to run yet. See `docs/source/examples/quantum.rst`.