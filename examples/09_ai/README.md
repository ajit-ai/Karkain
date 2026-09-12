# 09 — AI

Real, runnable AI fundamentals expressed in **plain Karkain** — no framework
API exists or is claimed. These demonstrate the algorithms (classification,
learning loops) on synthetic data using arrays, maps and arithmetic.

| Example                       | Status   | Engine | Idea                                 |
|-------------------------------|----------|--------|--------------------------------------|
| `01_nearest_neighbor.kark`    | Runnable | both   | Manhattan-distance classification    |
| `02_linear_classifier.kark`   | Runnable | both   | perceptron-style weight updates      |

## Honest status of the AI ecosystem

Karkain has **no** `std.ai` / `tensor()` / `infer()` module surface today.
Tensor-type language syntax (Phase 27 AST/HIR) exists in the compiler
internals, but the lexer/parser do not wire the `tensor` keyword into
runnable programs, and no `.kark`-callable AI API is shipped. Planned
roadmap:

- tensor-typed language surface (runnable) — future phase
- autodiff (`dn`) over tensor ops — future phase
- `@target(npu)` dispatch already exists for functions and falls back to the
  CPU oracle when no vendor adapter is available (Phase 98).
- hardware acceleration via GPU/NPU backends — future phases

## Run

```
karkain run examples/09_ai/01_nearest_neighbor.kark
```