# Profiling examples (`karkain prof`)

Each file is a self-contained program that the profiler compiles, runs once with
compiler-inserted instrumentation, and reports on:

- `basic.kark` — minimal two-function program showing the function table and
  `main -> add` / `main -> square` call graph.
- `recursion.kark` — `fib(18)` (8361 recursive invocations) showing aggregated
  per-function inclusive totals, the `fib -> fib` recursion edge, and flame-graph
  (folded) stacks.
- `hotspot.kark` — a compute-bound loop where `countOdds` visibly dominates the
  profile and `isOdd` is the innermost call.

## Usage

```
karkain prof examples/profiling/basic.kark
karkain prof --format json examples/profiling/basic.kark
karkain prof --format folded examples/profiling/recursion.kark
karkain prof --output prof.json --json examples/profiling/hotspot.kark
```

Formats:

- `text` (default) — function table sorted by inclusive time, call graph,
  allocation line.
- `json` — `karkain-profile-v1` document: `functions` (name, calls,
  total/self/min/max/avg ns), `calls` (caller -> callee count + inclusive ns),
  `folded` stacks, `allocation` (count/bytes/peak), `overflow`, `duration_ns`.
- `folded` — `path;path ns` flame-graph stacks, one per line.

## Boundaries (Phase 110)

- Profiling is opt-in: `karkain run` / `karkain build` never instrument the
  program.
- Go engine only. `--engine kcc` and `--target wasm32-wasi` are rejected with an
  explicit unsupported/deferred diagnostic (no silent native fallback).
- Single-threaded: programs using `spawn`/`channel`/`actor` are out of scope for
  the profiler instrumentation.
- Allocation metrics count `malloc`/`free` emitted in the generated user code
  (raw pointer `alloc`/`free`); string/array/map/struct helpers allocate inside
  the uninstrumented C preamble and are not counted.