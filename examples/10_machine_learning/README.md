# 10 — Machine Learning

Runnable ML training loops in plain Karkain on synthetic data. Produces exact
closed-form and convergent solutions without a framework, and demonstrates
what the language can do today.

| Example                          | Status   | Engine | Idea                             |
|----------------------------------|----------|--------|----------------------------------|
| `01_linear_regression.kark`      | Runnable | both   | OLS closed form, plain floats    |
| `02_gradient_descent.kark`       | Runnable | both   | minimize convex loss directly    |

## Honest status

There is no ML framework in the stdlib. `pkg/ir/tensor`, GPU/WGSL and NPU
machinery are compiler-internal (Phases 71-78) and not yet reachable from
`.kark`. The intended progression:

1. runnable algorithmic examples (this category) — **today**
2. tensor-typed language surface — future phase
3. autodiff + optimization passes — future phase
4. GPU/NPU accelerated training — future phase

## Run

```
karkain run examples/10_machine_learning/01_linear_regression.kark
```