# Karkain Conformance Corpus

Karkain-native language conformance tests. Each `*_test.kark` file declares
`test_`-prefixed functions (KTF-001 convention) that assert on the language
itself, using the builtin `assert` / `assert_eq` / `assert_ne` checks.

This corpus is the language-level regression net: it exercises the same
constructs through the real front end, SSA IR emit and C runtime that all
normal programs use.

## Running

```
karkain test conformance/
```

Every test must pass before a phase is considered complete. CI runs this
corpus on every push.

## Conventions

- Each file is self-contained: helper functions live in the same file and are
  available to its tests (the test runner synthesizes a mini-program from the
  file's non-test declarations plus the selected test function).
- Files only use constructs that are actually implemented and verified. A new
  feature adding a conformance file here proves it end-to-end.
- Negative/diagnostics behavior is covered separately by the KTF-002 compile
  corpus (`karkain test --compile`) and the Go-level semantic tests.

## Coverage

| # | File | Coverage |
|---|------|----------|
| 001 | `001_arithmetic_test.kark` | int/float arithmetic, modulo, precedence, negatives |
| 002 | `002_control_flow_test.kark` | if/else/else-if, while, nested loops, break/continue, bool ops |
| 003 | `003_functions_test.kark` | functions, params, recursion, array param mutation |
| 004 | `004_strings_test.kark` | length, indexing, concatenation |
| 005 | `005_arrays_test.kark` | literals, indexing, push, len, slices, 2D arrays |
| 006 | `006_structs_test.kark` | struct declarations, field access |
| 007 | `007_maps_test.kark` | map literal, index get/set |
| 008 | `008_algorithms_test.kark` | factorial, fibonacci, gcd, primality, sort |
| 009 | `009_phase81_regression_test.kark` | unparenthesized `if`, `%` modulo on int+float |