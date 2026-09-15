# 01 — Fundamentals

A beginner sequence covering the core Karkain language. Every file is
**Runnable**: it compiles and runs byte-identically on the Go front end and
the self-hosted `kcc` engine, and is pinned by
`pkg/cli/phase114_examples_test.go`.

## Learning order

| Step | File                          | Idea                                      |
|------|-------------------------------|-------------------------------------------|
| 1    | `01_hello_world.kark`         | first program, `func main()`, `print`     |
| 2    | `02_variables.kark`           | `let`, reassignment                        |
| 3    | `03_constants.kark`           | `const` immutable bindings                 |
| 4    | `04_functions.kark`           | functions, parameters, returns, recursion |
| 5    | `05_conditionals.kark`        | `if` / `else if` / `else`                  |
| 6    | `06_loops.kark`               | `while` loops, counters, accumulation      |
| 7    | `07_strings.kark`             | concatenation, `len`, indexing, `str()`    |
| 8    | `08_arrays.kark`              | literals, `push`, slices, 2D arrays        |
| 9    | `09_maps.kark`                | map literals, updates, `std.collections`   |
| 10   | `10_structs.kark`             | `type` records, construction, field access |
| 11   | `11_match.kark`               | `match`, `Some`/`None` Options            |
| 12   | `12_casts.kark`               | `int(f)`, `str(n)`, int/float division     |
| 13   | `13_enums.kark`               | `enum` variants, `EnumName.Variant` tags, matching |
| 14   | `14_adt_match.kark`           | ADT-style tagged `match` over enum values |

## Standalone helpers vs. the rest of the corpus

`examples/language_foundation/` holds the older, parity-gated set of larger
programs; `probes/` holds the regression smoke programs. This category is the
deliberately simple, single-concept path.

## Determinism notes

- Output is line-oriented (one `print` per line).
- String arrays print with different bracket styling per engine, so examples
  enumerate string collections element-by-element.
- `match` arms (literal, enum-variant, and `_` wildcard) run byte-identically
  on both engines.

## Run

```
karkain run examples/01-fundamentals/05_conditionals.kark
```

## Verify

```
karkain check examples/01-fundamentals/*.kark
```