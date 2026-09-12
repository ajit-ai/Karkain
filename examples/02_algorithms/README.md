# 02 — Algorithms

Practical, runnable algorithms demonstrating Karkain's array, string, map and
recursion idioms. Every file is pinned by `pkg/cli/phase114_examples_test.go`
and produces byte-identical output on both engines.

## Examples

| File                    | Idea                                              |
|-------------------------|---------------------------------------------------|
| `01_linear_search.kark` | unsorted scan, index or -1                        |
| `02_binary_search.kark` | halving search on sorted data                     |
| `03_min_max.kark`       | rolling comparison                                |
| `04_bubble_sort.kark`   | adjacent-swap sort, copy semantics                |
| `05_frequency_count.kark` | map-backed tally with `std.collections`        |
| `06_fibonacci.kark`     | iterative Fibonacci                               |
| `07_factorial.kark`     | iterative factorial                               |
| `08_gcd.kark`           | Euclid recursion, lcm                             |
| `09_prime_sieve.kark`   | sieve of Eratosthenes                             |
| `10_palindrome.kark`    | string reversal + equality                        |

## Related corpora

The larger per-program set under `examples/algorithms/` (21 programs) adds
sorting variants, searching, dynamic programming and graph basics, validated
by `pkg/cli/algorithm_corpus_test.go` and the kcc parity suite.

## Run

```
karkain run examples/02_algorithms/04_bubble_sort.kark
```

## Verify

```
karkain check examples/02_algorithms/*.kark
```