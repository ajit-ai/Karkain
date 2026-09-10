# Phase 105 — Error Recovery & Incremental Compilation — Final Report

## Objective

Two pillars, per the Phase 105 implementation prompt (§27 order, §28
completion criteria):

1. **Multi-error reporting** — recover every recoverable diagnostic in one
   invocation (no crash, no duplicates, no artificial errors) at both the
   syntax and semantic/type-check stages, surfaced by the CLI, exit 3.
2. **Incremental compilation** — dependency-aware, content-hashed caching of
   the whole-assembly build (generated C + linked executable), with
   invalidation on source/interface/config/compiler-change, a `karkain clean`
   purge path, cache safety on failed builds, and no behavioural change to the
   normal `check`/`build`/`run`/`test` commands.

Per `docs/ROADMAP-PRODUCTION.md` Tier 3 (Phases 104–109).

## Delivered — Part 1 (Multi-error reporting)

- **Engine-agnostic project-wide syntax preflight** — `pkg/cli/multierror.go`:
  `projectSyntaxDiagnostics(targetFile)` parses every unit file of the target
  project *independently* (project/dependency sources, with sibling fallback,
  non-root `func main` files excluded, root-module file last — mirroring the
  assembler's ordering) and collects every `CodeSyntax` diagnostic with
  per-file path, line, column and excerpt. No assembly is attempted when any
  file has recoverable errors.
- **Both engine paths**: `CheckCommandFormatted` (Go engine) and
  `KCCCheckCommand` (default kcc engine) run the preflight before
  lex/parse/sema; kcc's self-hosted parser is more permissive than the Go
  parser and previously reported `[ok]` on the multi-error fixture — the
  preflight now gives kcc the identical Go-computed diagnostic set. JSON-aware
  output (`--format=json`) emits the full `karkain-diagnostics-v1` array.
- **Semantic/type-check collection** — the resolver already aggregates
  resolve-stage diagnostics; `examples/phase105_errors/semantic.kark` proves
  three independent resolve errors surface together in one invocation
  (`missing_one`, `missing_two`, `undefined_type_call`). Later-stage arity
  diagnostics are intentionally out of the resolve stage (documented in the
  fixture).
- **Parser recovery reality (empirical, documented)**: the Go parser cleanly
  accumulates a handful of top-level recoverable forms — unknown `@attr` and
  `public <non-decl>` — plus certain expression-shape errors. Body-level
  mistakes are otherwise silently tolerated and some forms cascade. The gate
  fixture therefore uses six clean, non-cascading, non-duplicate recoverable
  errors (see gate), exceeding the five-diagnostic requirement while remaining
  honest about parser recovery boundaries.

## Delivered — Part 2 (Incremental compilation)

- **Content-addressed, dependency-aware cache engine** — new package
  `pkg/compiler` (`incremental.go`):
  - `ContentHash` (sha256) per unit file; `ModuleInterface` hashes *parameter
    type lists of `public` functions, struct/enum headers, top-level var
    headers and imports* — function bodies, locals and comments are excluded,
    so a body-only edit never invalidates dependents.
  - `PlanBuild(prev, files, key)` → per-module status `compiled` / `reused` /
    `invalidated`: content changed or first-seen → compiled; compiler-identity
    (`key`) change → invalidated; content unchanged but a dependency's
    *interface* differs from the persisted snapshot → invalidated; else
    reused. Interface hashes are memoized by content hash, so a no-op rebuild
    requires **zero reparsing**.
  - `Cache` with atomic writes (temp + rename), `manifest.json` + one C +
    one exe artifact per project; `CompilerKey` folds Go runtime/GOOS/GOARCH,
    target, debug flag and the detected C-compiler identity, so config or
    toolchain changes invalidate everything.
  - Project hash over the ordered per-file content hashes keys the whole
    assembly; **failed builds never `Store`**, so the last-good snapshot is
    always intact.
  - Decided design note: this is a **whole-assembly cache** (generated C +
    executable), NOT per-module `.o` TU splitting — per-module object
    granularity is explicitly deferred as post-105 work (documented in the
    package header). The dependency/interface machinery here is exactly what
    that later split needs.
- **CLI incremental build** — `pkg/cli/incremental.go`:
  `BuildCommandIncremental(targetFile, outputPath, cfg, verbose, cacheDir)`:
  a no-op fast path replays the cached executable (pure file hashing + copy),
  otherwise runs the full parse → borrow → GPU-shader-emit → C-codegen → gcc
  pipeline producing a persisted executable, then caches generated C + exe.
  `unitFiles` mirrors the assembler's dependency ordering. `karkain build
  --incremental` / `--incremental-cache <dir>` flags wire both engine
  branches in `cmd/karkain/main.go` (default cache dir `<root>/.karkain-cache`).
  The `--target=native-link` boundary is documented (no caching).
- **`karkain clean`** — `pkg/cli/clean.go` removes `.karkain-cache`
  directories during the artifact walk.

## Acceptance

### Multi-error gate (5-error requirement, exit 3)

Fixture `examples/phase105_errors/multierr.kark` — **six** recoverable parse
errors in one invocation, no crash, no duplicates:

```
error[K002] unexpected token '1' ...
error[K002] unexpected token '2' ...
error[K002] unknown attribute '@bad_attr_alpha'
error[K002] unknown attribute '@bad_attr_beta'
error[K002] unknown attribute '@bad_attr_gamma'
error[K002] unexpected token ')' ...
```

Semantic gate `examples/phase105_errors/semantic.kark` — three resolve
diagnostics in one invocation (`missing_one`, `missing_two`,
`undefined_type_call`). Positive control passes `[ok]`.

Gate: `pkg/cli/phase105_multierror_test.go` — **5 tests**: syntactic multi-error
through the Go engine, CLI exit-3 check, the same preflight through the default
kcc path, semantic multi-error, and a positive control. All PASS.

### Incremental gate (8 cache-correctness scenarios)

Plan-level (`pkg/compiler/incremental_test.go`, **10 tests**): clean build
compiles the unit with deterministic project hash; no-op rebuild reuses
everything with zero reparse; leaf body change recompiles only the leaf while
its dependents stay reused; interface change invalidates dependents; unrelated
module change recompiles only that module; interface hash provably ignores
bodies; config/compiler-key change invalidates all; failed build never poisons
the cache; Store/Load atomic round-trip; adding a file changes the project
hash.

E2E (`pkg/cli/phase105_incremental_test.go`, **3 tests** through real
`generateAndCompile` + gcc + executed natives): clean build → correct output +
`3 compiled`; no-op → `3 reused`, identical executable output; leaf body change
→ exactly `1 compiled` with unchanged behaviour; config change → all
`3 invalidated`; `clean` removes the cache; post-clean rebuild reproduces
**byte-identical generated C** and **identical stdout**; interface change and
failed-build-no-poison verified end-to-end.

## Performance (measured, this host)

`examples/phase105` (3 modules): clean = **2790 ms**, no-op = **101 ms**
(~27×), leaf body change = **2425 ms**, clean-again = **2608 ms**. The no-op
path is pure hashing + executable replay.

## Regression suite

Low-memory pattern on the ~4GB host (`GOMAXPROCS=1 GOGC=60`, packages run
individually, the documented pattern): `go vet` clean; `go build ./...` clean;
`pkg/compiler`, `pkg/parser`, `pkg/sema`, `pkg/pm`, `pkg/codegen` all green;
**full `pkg/cli`** (including the 59-assertion conformance corpus, probes, and
all Phase 97–104 gates) **PASS** (1066.8 s). Compiler engines unchanged on
their own paths; `check`/`build`/`run`/`test` behaviour outside the preflight
and `--incremental` is untouched. The documented environmental kcc stage-2
bootstrapping OOM persists (known Phase 99+ class, unrelated to this work).

## Files

- New: `pkg/cli/multierror.go`, `pkg/cli/incremental.go`,
  `pkg/compiler/incremental.go`, `pkg/cli/phase105_multierror_test.go`,
  `pkg/cli/phase105_incremental_test.go`, `pkg/compiler/incremental_test.go`,
  `examples/phase105/{main,math,strings}.kark`,
  `examples/phase105_errors/{multierr,semantic}.kark`.
- Modified: `pkg/cli/check.go`, `pkg/cli/checker.go`,
  `pkg/cli/kcc_engine.go` (preflight + srcMap attribution),
  `pkg/cli/clean.go` (`.karkain-cache`), `cmd/karkain/main.go`
  (`--incremental` / `--incremental-cache`).

## Future (explicitly NOT Phase 105 scope)

Per-module `.o` TU splitting on top of the interface machinery (post-105), and
any Phase 106–115 work, are intentionally out of scope for this phase.

## Release

Per AGENTS.md: all changes committed on `develop`, merged into `main`, both
pushed to `origin`.