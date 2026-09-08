# 09 AI — Vector / Matrix Math

STATUS: WORKING TODAY (validated).

## What it demonstrates

- `dot`, matrix multiply (2D arrays of arrays), integer-arithmetic
- float `cosine` similarity with a Newton-method float square root
  (the stdlib `sqrt` is integers-only)
- 1-nearest-neighbor lookup keeping the HIGHEST cosine score
- the `@target(cpu)` function attribute (Phase 98)

## Commands

```
karkain check examples/showcase/09_ai/matrix/main.kark
karkain run   examples/showcase/09_ai/matrix/main.kark
```

## Expected output (verified)

```
dot=32
matmul[0][1]=22
matmul[1][0]=43
cosine=0.974632
nearest_q0=1
nearest_q1=2
```