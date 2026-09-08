# 12 Scientific Computing — Numerical Methods

STATUS: WORKING TODAY (validated).

## What it demonstrates

- Newton iteration for `sqrt(x)` in floats (stdlib `sqrt` is integers-only)
- trapezoidal integration of `x^2` over `[0,1]` (1000 strips -> ~1/3)
- population mean / variance / standard deviation
- deterministic Monte Carlo pi via a textbook LCG (reproducible on every host)

## Commands

```
karkain check examples/showcase/12_scientific/numerics/main.kark
karkain run   examples/showcase/12_scientific/numerics/main.kark
```

## Expected output (verified)

```
sqrt_2=1.41421
sqrt_25=5
integral_x2_0_1=0.333333
mean=3
variance=2
stddev=1.41421
pi_estimate=3.1692
```