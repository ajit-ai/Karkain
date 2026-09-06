# Phase 83 — Final Report

**Phase**: 83 — Compiler Symbol Namespacing, True Diagnostic Spans & LSP↔CLI Pipeline Sharing

**Status**: COMPLETE

**Branch**: `develop` (merged to `main` per AGENTS.md workflow)

**Scope**: the reset of the Phase-82 "next steps": C-library collision fix,
`E-K-RES` true columns, array returns, LSP diagnostics + document sync, and
conformance-corpus growth.

---

## 1. Executive Summary

Phase 83 closes the two front-end correctness gaps the Phase-82 closeout listed
as "planned next phases" and makes the editor integration honest:

1. **Compiler symbol namespacing** — user-defined functions now live in a
   deterministic `karkain_user_*` C namespace, so user code can no longer
   collide with C library symbols (`abs`) *or* with the runtime's `karkain_*`
   helpers. As a bonus, user functions that shadow language builtins now
   correctly *win* over the builtin (user-defined `sqrt`/`readFile`/`pow`/…).
2. **`E-K-RES` true columns** — token columns are 0-based byte offsets from the
   lexer, threaded through the parser into per-node span metadata via
   `source.LineIndex`, and reported by `check --format=json` as true
   `column`/`endColumn` with an optional `excerpt`. The v1 "column always 1"
   limitation is *retired*.
3. **LSP↔CLI shared pipeline** — the LSP no longer runs a second, divergent
   front-end. Both `karkain check` and the editor server call one driver
   (`cli.AnalyzeSource`); didChange invalidates and republishes diagnostics,
   verified by real-time-sync tests.
4. **Array returns** — functions may build and return arrays by value
   (literals, loop-built ranges, nested 2D grids, empty arrays), covered by a
   new conformance file.
5. **Corpus discipline honored** — 11 new native tests (2 conformance files),
   1 new pinned-golden probe, and the resolution layer gained a
   conservatively-guarded bare-identifier check.

| Surface | Delivered | Gate |
|---------|-----------|------|
| Symbol namespacing | `userFuncC` in C23 codegen (Go + self-hosted `codegen.kark` mirror) | `conformance/011` (5 tests), `probes/namespace` golden via `TestProbesCorpus_RunsEveryProbe` |
| Builtin shadowing | user `sqrt`/`readFile`/`pow`/`trim` shadow runtime helpers, user wins | `conformance/011` |
| Array returns | build/grow/return arrays, feed functions, index/iterate/mutate, nested 2D | `conformance/010` (6 tests) |
| `E-K-RES` spans | lexer 0-based cols, parser `Col`/`EndCol`, `endColumn`+`excerpt` in JSON | `TestResolver_UndefinedIdentifier*`, `TestLSP_SharedCanonicalDiagnostics` |
| Shared CLI/LSP pipeline | `cli.AnalyzeSource` single-driver | `TestLSP_SharedCanonicalDiagnostics`, `TestLSP_RealTimeSync` |
| `pkg/source` | `LineIndex` (byte↔line/col), UTF-16 cols, excerpts | `pkg/source` suite |

---

## 2. Deliverables

### 2.1 Compiler symbol namespacing (`userFuncC`)

- `pkg/codegen/codegen.go` defines the injective mapping
  `userFuncC(name)`: `main` and `getArgs` keep canonical C names; every other
  user function is emitted as `karkain_user_<name>`.
- Applied at every user-fn symbol site in the C23 path: the forward-declaration
  pass (pre-scans the statement list into `Generator.userFuncs`), `genFuncDecl`,
  the SSA function emitter (`emit_ir.go`), the direct-expression emitter
  (`genExpr`), closure-capable calls, and the `lower.go` SSA call lowering
  (`declared → userFuncC`, runtime → `karkain_*`).
- **Builtin shadowing semantics**: `g.userFuncs[n.Function]` is consulted
  before the builtin branches (`len`, `sqrt`, `readFile`, `trim`, `abs`, …), so
  a user function that shadows a builtin dispatches to the user symbol and the
  builtin is restored for everyone else. The runtime helpers themselves are
  never renamed, so they keep working when not shadowed.
- **Self-hosted parity**: `src/compiler/codegen.kark` mirrors the mapping in
  `emitC11FuncSymbol`/`registerUserFunc`/`hasUserFunc` and applies it in the
  forward-declaration and function-emission passes, so the Go backend and the
  self-hosted compiler emit byte-identical symbol names.
- **Regression coverage**: `conformance/011_namespace_test.kark` (user
  `sqrt`/`trim`/`pow`/`readFile` shadow runtime helpers while `str`/`len`/
  `assert_eq` keep working, forward references resolve after shadowing) and
  `examples/probes/namespace` (pinned golden `777 virtual 1 2`).

### 2.2 `E-K-RES` true columns + span-aware diagnostics

- `pkg/source` (new): `LineIndex` maps byte offsets ↔ 1-based line / 0-based
  byte column (LF/CRLF/lone-CR aware), converts to LSP UTF-16 columns, and
  produces diagnostic `excerpt`s. Tested in `pkg/source/source_test.go`.
- Lexer (`pkg/lexer/lexer.go`): token columns are now exact 0-based byte
  offsets within the line (was 1-based approximate). Two-char operators report
  the column of their first character. `TestTokenLineCol` updated + extended
  (`a == 1` → `==` at col 2).
- Parser: `FuncDecl`, `Identifier`, `CallExpr` carry `Col`/`EndCol`
  (0-based); statement-position idents/calls and dot-chains record end-span.
  `macro.go` clone constructors forward the new fields so spans survive
  macro expansion (the Phase 81 line-fix analog for columns).
- Resolver: `ResolveError` gains `Col`/`EndCol`; the duplicate-definition,
  undefined-function, private-visibility, and new undefined-identifier paths
  all report them.
- Diagnostics contract (`karkain-diagnostics-v1`, additive): optional
  `endColumn` (1-based, just past the token) and `excerpt` (trimmed, ≤80-rune
  source line). `checker.go` (`pkg/cli`) converts 0-based resolver spans to the
  1-based wire columns.
- **New undefined-identifier check**: a bare identifier in value position in a
  function body that is not a local, parameter, loop variable, implicit
  assignment target, function, type, or builtin is now `undefined identifier
  'x'`. Guards are conservative (struct field keys, map keys, enum names,
  method receivers, top-level patterns never fire).

### 2.3 LSP↔CLI shared pipeline + document sync

- `pkg/cli/check.go` + `pkg/cli/checker.go`: extraction of the canonical
  front-end driver `AnalyzeSource(file, src, srcMap)` running lex→parse→macro→
  resolve→kernel-analyser with the same stage short-circuit semantics the CLI
  had, thread-safe, no I/O, no printing.
- `pkg/lsp/handler.go`: `runDiagnostics` is now a 20-line adapter over
  `cli.AnalyzeSource`; the previous divergent lex→parse→actor-check→
  coroutine-check path and the fragile `parseErrorLocation` string-regex are
  gone. Diagnostics are published with true 0-based LSP ranges
  (`line−1`, `column−1` .. `endColumn−1`).
- Document sync (`didChange`) invalidates and republishes diagnostics; LSP
  tests assert: canonical `undefined identifier` spans, `undefined function`
  statement-position spans, real-time invalidation on error → clear on fix,
  and document version tracking.

### 2.4 Array returns

- `conformance/010_array_returns_test.kark`: return of array literals,
  loop-built ranges, mutation of returned arrays, passing returned arrays to
  helper functions, nested 2D grids, and empty-array returns — all through the
  real front end, SSA/C23 path, and C runtime.

### 2.5 Corpora growth

- Conformance corpus: 48 → **59 tests** across 11 files.
- Probes: 11 → 12 (new `namespace` golden enforced by
  `TestProbesCorpus_RunsEveryProbe`).

---

## 3. Verification

| Gate | Command / Test | Result |
|------|----------------|--------|
| Conformance corpus | `karkain test conformance/` | **59 passed, 0 failed** |
| Format gate (LF-normalized; CI-equivalent) | `karkain fmt --check` per file | **11/11 clean** |
| Probes corpus | `TestProbesCorpus_RunsEveryProbe` (12 goldens) | PASS |
| New LSP shared-pipeline tests | `TestLSP_SharedCanonicalDiagnostics`, `_UndefinedFunctionSpans`, `_RealTimeSync` | PASS |
| New resolver tests | `TestResolver_UndefinedIdentifier*` (+ span-through-macro) | PASS |
| Repo-mandated unit suites | `go test ./pkg/lexer/... ./pkg/parser/... ./pkg/codegen/... ./pkg/pm/...` (+ source, diagnostics, sema, lsp) | PASS |
| Whole `pkg/cli` suite | `go test ./pkg/cli/ -count=1` | PASS (480 s) |
| Static checks | `go vet ./...`; `go build ./...` | CLEAN |

New/changed Go files are gofmt-clean modulo the repo-wide classic doc-comment
style that predates Go 1.19 (82 pre-existing files share it; untouched).

**Known transients**: two Windows-only gcc file-lock flakes observed during
corpus re-runs (a conformance test edge and one probe compile); both pass on
immediate re-run and are the pre-existing `compileRunSource` retry-covered
condition documented since Phase 82. ✓ The `fmt --check` gate only passes
locally for LF-normalized copies because `core.autocrlf=true` checks files out
as CRLF; CI (Linux, LF checkout) is unaffected.

---

## 4. IMPLEMENTED vs PLANNED

**IMPLEMENTED**
- Deterministic `karkain_user_*` C namespace for user functions (Go + self-host
  parity); user-function shadowing of builtins (user wins).
- True `E-K-RES` columns: lexer 0-based columns, parser span metadata surviving
  macro expansion, resolver `Col`/`EndCol`, JSON `endColumn` + `excerpt`.
- The undefined-identifier name-resolution check (conservative, preserved
  negative cases).
- LSP↔CLI single-pipeline `AnalyzeSource`; real-time `didChange` sync tests.
- `pkg/source` position/line-index/excerpt package.
- Array-return semantics conformance file; new namespace probe; corpus growth.

**PLANNED (next phases)**
- LSP completion/hover/go-to-definition beyond the (pre-existing) keyword
  completion; VS Code shifting check output to the Problems panel and
  consuming LSP diagnostics.
- Style-grade formatter above token canonicalization.
- `E-K-RES` → module-level UTF-16/CRLF awareness in the LSP (byte columns are
  correct for LF; UTF-16 conversion helper is in `pkg/source` for when the LSP
  wire needs it).
- Further conformance growth (generics, quantum kernels, actor primitives).

---

## 5. Known Limitations (honest, post-Phase-83 state)

| Limitation | Where documented |
|-----------|------------------|
| `while`/`for` still require parenthesized conditions (only `if` unparenthesized) | Toolchain contract |
| `fmt` is contract-grade canonicalization, not a style engine | Toolchain contract |
| LSP completion is keyword/full-symbol index only; hover + definition pending | IDE integration doc |
| `karkain check` from inside a module reads sibling-scope declarations (module context), which can surprise single-file users | Toolchain contract, sibling-join commit |
| Full-suite runs can flake on parallel gcc compile (Windows only) | This report, §3 |

*Resolved by this phase:* `E-K-RES` column=1; user/C-symbol collisions; array
returns; LSP/CLI diagnostic divergence.

---

## 6. Deviations & Decisions

1. **`main`/`getArgs` keep canonical C names** — the C entry point and the
   runtime argument intrinsic must not move namespace; every other user symbol
   is namespaced.
2. **User-defined builtin shadowing adopted as semantics** — a user `func
   sqrt` shadows the runtime builtin rather than erroring, because lexically
   the definition site is unambiguous; the registry is populated by a pre-pass
   so forward references resolve (mirrors the Phase-40 forward-declaration
   pass).
3. **Columns are 0-based bytes internally, 1-based on the wire** — the lexer
   and parser use 0-based byte columns (natural spans); the diagnostics
   contract remains 1-based for textual column semantics; the LSP adapts its
   own 0-based ranges. `pkg/source` holds the one true byte↔line/col model.
4. **`AnalyzeSource` lives in `pkg/cli`** — despite being LSP-consumed, so the
   shared driver and the `karkain check` surface can never drift; no new
   internal layer was introduced (avoids phase scope creep into new packages
   with their own purity claims).
5. **Excerpts are trimmed and truncated** (`≤80` runes) to keep the wire
   compact; `omitempty` keeps backward compat with v1 consumers.
6. **Blanket byte-column gaps in non-LF editors** remain a documented v1.1
   item; the UTF-16 conversion primitive ships now so the LSP can adopt it
   without re-engineering positions later.

---

## 7. Metrics

- Files touched: 21 (16 modified, 5 new — `conformance/010`, `conformance/011`,
  `examples/probes/namespace/main.kark`, `pkg/cli/checker.go`, `pkg/source/*`).
- Conformance corpus: 48 → 59 tests (11 files); probes 11 → 12.
- New Go test files/functions: `pkg/source/source_test.go`, 3 LSP sync/span
  tests, 3 resolver span/guard tests, extended lexer column test.
- Docs: this report, toolchain-contract updates, probes README row.
- Verified gates: full corpus, 12-probe golden set, whole `pkg/cli` suite,
  repo-mandated suites, vet, build.

---

## 8. Next Steps

1. **LSP depth**: wire `pkg/source.UTF16Col` into the LSP spans; add hover and
   go-to-definition for user symbols (the symbol index already exists); move
   the VS Code extension's diagnostics to the Problems panel via a real LSP
   client.
2. **Echo system**: style-grade formatter; REPL; manifest-aware
   build/run/check for `karkain pkg` (deferred since Phase 82).
3. **Correctness**: unparenthesized `while`/`for` parity with `if`; further
   CLI-visible span accuracy (kernel-sema diagnostics are still line-1/col-1
   in one path).
4. **Corpus discipline**: every future phase adds its conformance file(s); the
   corpus is the regression net.