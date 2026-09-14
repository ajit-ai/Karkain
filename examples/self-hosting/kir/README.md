# Self-Hosting: KIR emitter

Demonstrates the first compiler component written entirely in Karkain:
the KIR v1 text emitter (`src/compiler/kir.kark`), owned by the
self-hosted compiler and gated by Phase 120.

## Run

```
karkain kir examples/self-hosting/kir/main.kark
karkain run examples/self-hosting/kir/main.kark
```

`karkain kir` renders the program as deterministic KIR text (26 lines,
ending in `[ok] kir text: 26 lines`). `karkain run` prints
`15 / 20 / 1 / 15` identically on the Go and kcc engines.

The file exercises the full KIR surface: struct and enum declarations,
function recursion, array/map/struct literals, for-in, if/else with the
space-form `print`, and while loops.