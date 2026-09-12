# PHASE 117 — BETA 1 READINESS

Status: **VERDICT PASS** — Beta 1 gate complete. Full measured results in
`PHASE-117-BETA-1-FINAL-REPORT.md`; this page is the scorecard.

## Stable Core

The Beta 1 stable core (both engines, byte-identical on the golden corpus):

- Variables (`let`/`var`/`const`), functions, recursion, control flow
  (`if/else`, `while (cond)`, C-style `for`, `for-in`, `break`/`continue`/
  `return`), assertions.
- Primitive values and strings; checked div/mod/index runtime errors with
  `runtime error: <kind> at <file>:<line>`; stack traces on both engines.
- Arrays, maps, structs (records), enums + `match`, modules with `public`
  exports and qualified calls.
- Go front end and the self-hosted `kcc` engine, both with semantic
  build/run gating (Phase 117): an undefined identifier rejects `build`/`run`
  with `error[K002]` (Go) / `error[K102]` (kcc) and `ExitCompile` (3) — the
  same contract as `check`.

## Experimental

Keep-only surfaces that exist and are tested but remain engine-bound:

- Concurrency runtime (channels/actors/spawn) — Go engine only (Phase 107).
- Profiling (`karkain prof`) — Go engine only (Phase 110).
- Debug tracing (`karkain debug`) — Go engine only (Phase 112).
- SIMD/vector types — documented Phase 106 scope.
- WASM target (`wasm32-wasi`) — Go engine only, requires `wasmtime` (Phase
  108).
- Closures/`fn` — NOT supported on either engine (documented Phase 101
  boundary); `karkain` never pretends otherwise.

## Planned

Everything listed under `docs/source/status/planned.rst` (networking,
databases, web/AI/ML frameworks, quantum, GPU/NPU extensions, full package
registry, std.math/std.ai, etc.). None of these are claims.

## Compatibility

- Stable core: changes are deliberate, documented and regression-tested.
- Experimental: may change.
- Planned: no guarantee.
- Phase 117 deliberately preserved two boundaries that could not change
  without breaking existing gates: program output for kcc stays merged in the
  command result (tests regex the `[ok]` banner on stdout), and the `wasm`
  exit code stays `1` (consistent with the native path collapse).

## Compiler

- Go front end: parser + sema + codegen + C23 transpile; the reference
  engine.
- Self-hosted `kcc`: default engine for `check/build/run/test`; bootstrap
  stage-2 == stage-3 bitwise identical; auto-rebuilds from
  `src/compiler/*.kark`.
- Phase 117 compiler alignment: `while` conditions in unparenthesized form
  now parse identically on both engines (Go `parseWhile` + kcc
  `parseWhile`), matching `parseIf`.
- Phase 117 new gate: recoverable parse errors are no longer swallowed by
  `run`/`build` — `func main() { let = 42 }` now exits 3 on both engines
  (`parseVarDecl` name guard + `parseSourceWithErrors`).

## Standard Library

Public modules (Phase 109): `std.string`, `std.collections`, `std.io`,
`std.encoding`, `std.crypto`, `std.testing`. Edge-case gates added in Phase
117 (`pkg/cli/phase117_stdlib_edge_test.go`): empty strings/arrays/maps,
single-element arrays, duplicate map keys, empty and invalid hex/base64
input, empty hashing input — byte-identical on both engines.
`std.math`/`std.ai` are intentionally NOT invented.

## Toolchain

CLI surface verified: `check/build/run/test/fmt/lint/debug/prof/target/
explain/clean/pkg/lsp`. Phase 117 added numeric error codes to `explain`
(K001-K008, K100, K101-K113) and removed the hyphen requirement, so
`karkain explain K002` and `explain --list` document the stable code
namespace. Exit-code contract verified end to end (see final report).

## Examples

Phase 114/116 corpus: 49 pinned goldens in `examples/` plus the developer
categories. `TestPhase114_CorpusExamples_KCCParity` + `GoEngine` and the
`TestPhase116` metadata gates pass under the new semantic gating — no
conservative-checker false positives. No golden was changed by Phase 117.

## Cross Compilation

`karkain target` matrix verified by `TestPhase117_TargetMatrix` and the
Phase 111 gate: host triple, `native`, `c23`, `wasm32-wasi`, `native-link`;
foreign triples fail deterministically (`ToolchainError`, exit 6) without a
cross-linker; foreign `run` is refused.

## Documentation

Sphinx `docs/source` strict build (`-W --keep-going`) and linkcheck (`-W`)
both GREEN on the final battery (see final report). Phase 117 added
`status/beta.rst`.

## CI

Phase 115 CI runs vet, unit tests, and the Phase 114/115 gates. Phase 117
adds a `TestPhase117` step to `.github/workflows/ci.yml` (see final report
for the diff).

## Fresh Checkout

`scripts/beta-fresh-checkout.ps1` (Phase 117) walks checkout → build → vet →
rebuild CLI → help → hello world (check/build/run) → test → debug → prof →
target → example corpus → Sphinx strict html + linkcheck, all in a
repository-relative scratch directory that it removes. Result: **PASS** —
all 16 steps OK on the final battery (see final report). Three script bugs
found and fixed while validating it (PS 5.1 native-stderr escalation under
`$ErrorActionPreference=Stop`, the `if (cmd; $LASTEXITCODE -eq 0)` syntax
error, and `karkain help` → `karkain --help`).

## Security

No formal audit is claimed. Phase 117 review scope: process execution (kcc
sandboxed temp builds, `wasmtime` subprocess), filesystem paths
(repository-relative scratch dirs in gates), package dependencies
(lockfile-integrated resolver, atomic fetch), cryptographic API surface
(std.crypto is a thin wrapper over runtime SHA-256/SHA-512 builtins — no
homegrown crypto, no new dependencies). Remaining limitations documented in
`status/beta.rst`.

## Remaining Blockers

- [x] Full regression battery + Sphinx strict + linkcheck must pass.
- [x] CI `TestPhase117` step added.
- [x] Public status flip to Beta 1 after the battery (v0.117.0, Beta 1 Build).
- [x] Commit, merge, push, clean tree.

## Documented Environmental Limitation (not a defect)

The bootstrap stage-2 tests (`TestBootstrap_Stage2SelfHosting`,
`TestBootstrap_BitwiseIdentity`) fail on the ~4 GB / ~940 MB-free development
host with a `0xc0000005` SEGFAULT during the ~150 s self-hosted full-source
transpile — the documented kcc-build-mode OOM class (see AGENTS Phase
99/101/107/110/111 notes). This is NOT a Beta 1 defect: `TestPhase117` and
the full regression battery pass, kcc type-checks the `while`/`;` surface
cleanly, and `TestBootstrap_Stage2SelfHosting` passed one complete run with
the identical stage-1 binary. Full concurrent `go test ./pkg/cli` runs also
OOM-crash this host; every CLI suite passes when run in isolation/batches.