# 12 — Scientific Computing

Numerical methods in plain Karkain with deterministic output. No
arbitrary-precision or dimensional-analysis systems exist, so the honest
arithmetic is IEEE-style `double` through the generated C runtime.

| Example                          | Status   | Idea                                    |
|----------------------------------|----------|-----------------------------------------|
| `01_sqrt_newton.kark`            | Runnable | Newton-Raphson root iteration           |
| `02_numerical_integration.kark`  | Runnable | trapezoid rule, convergence to 1/3      |
| `03_statistics.kark`             | Runnable | mean / variance / stddev                |
| `04_matrix_multiply.kark`        | Runnable | 3x3 integer matrix product              |

## Honest notes

- `sqrt`/`pow`/`abs` builtins do not reliably handle float arguments on this
  pipeline, so iterative methods are used instead — which is both correct
  and more instructive.
- Floats print at 6 significant figures, identically on both engines.
- No GMP/big-number surface in the language (documented boundary).

## Run

```
karkain run examples/12-scientific-computing/02_numerical_integration.kark
```