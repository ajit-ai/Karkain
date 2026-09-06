# Karkain Examples — Developer Probes

Deterministic print-based smoke programs, one per construct area. Each probe
has a **golden output** enforced by `pkg/cli/probes_corpus_test.go`, so any
regression in parsing, semantics or codegen changes a probe's output and fails
the suite.

Run from the repo root:

```
karkain run examples/probes/hello/main.kark
```

## Probes and golden outputs

| Probe | File | Constructs | Golden output |
|-------|------|-----------|---------------|
| hello | `hello/main.kark` | basic print | `hello world` |
| strings | `strings/main.kark` | len, char index | `11` `h` `w` |
| strings_concat | `strings_concat/main.kark` | string concat | `foobar` `foobarfoo` |
| control_flow | `control_flow/main.kark` | nested while, else-if chains | `12` `-1` `0` `1` `2` |
| functions | `functions/main.kark` | recursion, param mutation | `55` `12` |
| structs | `structs/main.kark` | struct declaration, field access | `Ana` `25` |
| arrays | `arrays/main.kark` | slices, 2D arrays, push | `3` `30` `50` `5` `99` |
| maps | `maps/main.kark` | map literal, index get/set | `30` `Ana` |
| math | `math/main.kark` | int div/mod, floats | `3` `2` `-1` `3.5` `10` `1` |
| algorithms | `algorithms/main.kark` | bubble sort, primality | `11` `25` `90` `1` `0` |
| phase81 | `phase81/main.kark` | unparenthesized if, modulo | `1`..`10` `1` `-1` `1` |
| namespace | `namespace/main.kark` | user builtin shadowing (`sqrt`, `readFile`) | `777` `virtual` `1` `2` |

## Conventions

- Self-contained single-file programs (`func main()`), module-scope free.
- Only verified constructs with pinned outputs — the test asserts exact stdout.
- Determinism relies on `print` emitting one value per line.