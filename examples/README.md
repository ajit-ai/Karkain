# Karkain by Example

A curated, executable corpus that demonstrates working Karkain for real
programming tasks. Every example in the 15 canonical categories below carries
an honest tri-state status header at the top of the file:

```
// Status: Runnable | Experimental | Planned
// Engine: both | go | kcc | planned
```

## Status model

| Status       | Meaning                                                              |
|--------------|----------------------------------------------------------------------|
| `Runnable`   | Compiles and runs today through the declared engine(s).              |
| `Experimental` | Runs on one engine only (kcc or Go); cross-engine parity pending.    |
| `Planned`    | Specified and documented, but intentionally NOT runnable yet.        |

`Engine: both` means the example produces byte-identical output through the
Go front end AND the self-hosted kcc engine. Planned categories contain no
runnable code — they document the roadmap honestly and are never executed.

Runnable and Experimental examples are pinned with byte-identical golden
output in the corpus gate (`TestPhase114_CorpusExamples_*`,
`pkg/cli/phase114_examples_test.go`), so the corpus can never silently drift.
Planned examples are deliberately excluded from that gate.

## Categories

| Dir | Category                  | Examples | Runnable | Experimental | Planned |
|-----|---------------------------|----------|----------|--------------|---------|
| 01  | Fundamentals              | 12       | 12       | 0            | 0       |
| 02  | Algorithms                | 13       | 13       | 0            | 0       |
| 03  | Systems                   | 1        | 1        | 0            | 0       |
| 04  | Networking                | 0        | –        | –            | planned |
| 05  | Data                      | 4        | 4        | 0            | 0       |
| 06  | Database                  | 0        | –        | –            | planned |
| 07  | Web                       | 0        | –        | –            | planned |
| 08  | Concurrency               | 2        | 0        | 2 (go)       | 0       |
| 09  | AI                        | 2        | 2        | 0            | 0       |
| 10  | Machine Learning          | 2        | 2        | 0            | 0       |
| 11  | Quantum                   | 0        | –        | –            | planned |
| 12  | Scientific Computing      | 4        | 4        | 0            | 0       |
| 13  | Finance                   | 4        | 4        | 0            | 0       |
| 14  | Security                  | 4        | 4        | 0            | 0       |
| 15  | Developer Tools           | 2        | 1        | 0            | 0 (+1 test-mode) |

Totals: **50** `.kark` files (49 corpus programs + 1 test-mode file), of
which **49** are golden-pinned (`47` both-engine + `2` Go-engine
Experimental). Four categories — networking, database, web, quantum — are
documented-but-planned with no runnable code.

The test-mode file `15-developer-tools/02_assertions_test.kark` is run with
`karkain test examples/15-developer-tools/02_assertions_test.kark`
(assertions, `[ok]`, exit 4).

## Running examples

```bash
# both-engine examples (default engine: kcc)
karkain run examples/01-fundamentals/01_hello_world.kark

# Go engine variant
karkain run --engine go examples/02-algorithms/04_bubble_sort.kark

# test-mode example
karkain test examples/15-developer-tools/02_assertions_test.kark
```

Each file's `// Run:` header shows the exact command. For full documentation
of the corpus (per-category deep dives in Sphinx) see
`docs/source/examples/` and `docs/source/reference/example-matrix.rst`.

## Verification

`scripts/verify-examples.ps1` walks the corpus, checks headers, and runs
every Runnable example on both engines (exit-code-based). The Go-side gate in
`pkg/cli/phase114_examples_test.go` pins byte-identical golden output.

## Conventions

- Extensions are always `.kark` — never `.kar`.
- Category directories are dash-named: `NN-category-name`.
- Every category directory has a `README.md` describing its status and
  contents (planned categories document intent and constraints).
- Everything in this corpus is honest: no example is `Runnable` unless it
  actually runs, and no feature is advertised as Stable that is not.

See `EXAMPLES.md` for the machine-readable file inventory.