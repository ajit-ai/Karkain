# Phase 95 — Self-Hosted kcc Owns the Core Pipeline

## Objective
Make the self-hosted compiler (`kcc`, bootstrapped from `src/compiler`) the
primary engine for the core pipeline — **lex + parse + sema + C codegen** — and
route `karkain check/build/run` through it, with the Go front end retained as
the fallback for the LSP, test runner and exotic backends.

## What changed

### CLI engine (new: `pkg/cli/kcc_engine.go`)
- `EngineKind` (`EngineGo` / `EngineKCC`) + `EngineFromEnv`/`EngineFlag` resolve
  the engine from `KARKAIN_ENGINE` or `--engine go|kcc`.
- `kccBinaryPath` resolves the self-hosted binary: `KARKAIN_KCC` env wins,
  then `<repoRoot>/kcc.exe`, building it on demand. A **staleness check**
  (`kccStale`) rebuilds whenever any `src/compiler/*.kark` is newer than the
  binary — this prevents silently-stale-engine output (the root cause of a
  bootstrap regression during this phase).
- `buildKCC` bootstraps stage-1 via the Go compiler into C23, links with gcc.
- `KCCCheckCommand`, `KCCBuildCommand` (C23→gcc native link or `--compile-only`),
  `KCCRunCommand` (temp-sandboxed so artifacts never sit next to source).

### CLI wiring (`cmd/karkain/main.go`)
- `--engine go|kcc` flag (both `--engine=x` and `--engine x` forms) plus the
  `KARKAIN_ENGINE` env default.
- `run`/`build`/`transpile`/`check` dispatch through kcc when the kcc engine is
  active (`native` and `c23` targets for build; exotic targets still Go).

### kcc parity fixes (self-hosted `src/compiler`)
The self-hosted compiler now reproduces the Go front end's exact runtime
behavior on both pinned corpora:

- **parser.kark**
  - `psIsTypeToken` helper: type keywords lex as `TK_INT(140)`/`TK_FLOAT64(141)`/
    `TK_STRING(142)`/`TK_BOOL(143)`, not `TK_IDENT` — fixed `func f() int { }`
    returning an empty body.
  - Unparenthesized `if x < y {` disambiguation via `psNoStructAtBrace`
    (`state[6]`, mirrored from Go).
  - Map literal parsing in `parsePrimary` + `parseMapLit` (`{}`, `{k: v}`).
- **ast.kark**
  - `makeSlice`/`sliceLeft`/`sliceStart`/`sliceEnd`/`sliceLine`.
- **codegen.kark**
  - Slice emission (`karkain_slice`).
  - `=` sends Index (ident vs nested) and Member through `index_set`/`map_set`
    with GNU statement-expression temps (Phase-52 mirror).
  - `karkain_appendArray` takes `Value*` **and returns `Value`** (matches Go's
    `Value karkain_appendArray(Value*, Value)`); the `push`/`appendArray` call
    branch emits `&ident` or a temp.
  - assert/assert_eq/assert_ne runtime + call emission (KTF-001 parity).
  - `array_get` now routes `TYPE_MAP` to `map_get`.
  - Struct decl emits a comment (fields are maps at runtime, Go parity); struct
    and map literals emit `make_map()` + `map_set`.

### Parity gate (new: `pkg/cli/phase95_parity_test.go`)
- `TestPhase95_ProbesParity`: kcc must reproduce all 12 golden probe outputs.
- `TestPhase95_ConformanceParity`: all 11 conformance files (59 assertions)
  run green through kcc with synthesized per-file drivers.
- `TestPhase95_EngineSelection`: `--engine` parsing contract.

## Verification
- `pkg/cli` full suite green (363s).
- `pkg/lexer`, `pkg/parser`, `pkg/sema`, `pkg/codegen`, `pkg/pm` all green.
- `TestBootstrap_BitwiseIdentity` passes — **stage2 == stage3 bitwise identical**
  (kcc rebuilt to fix the `void` karkain_appendArray emission).
- `go vet ./pkg/cli/ ./cmd/karkain/` clean.

## Result
The self-hosted compiler is now the co-equal primary engine for the core
pipeline. `karkain run/build/check --engine=kcc` (or `KARKAIN_ENGINE=kcc`)
compiles through kcc and reproduces the exact Go-runtime behavior of both
probe and conformance corpora. The Go front end remains for LSP/IDE tooling,
the native test runner and non-C23 backends.

Committed on `develop` (`c81884d`), fast-forward merged to `main`, both pushed.
