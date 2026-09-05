# Phase 82 — Language Conformance, Examples & Developer Tooling Foundation

## Scope

Complete the developer-facing foundations the roadmap gate calls for: a
language-level **conformance corpus**, an **examples/probes corpus** with
golden outputs, expansion of the **algorithm corpus**, a stable **toolchain
contract** for editors (`check --format=json`, `fmt`, real `lsp`, `ide info`),
a **VS Code extension** shell, an **IDE integration** doc, and **CI**
coverage for the new surfaces.

## Delivered

### 1. Conformance corpus (`conformance/`)

Karkain-native assert-based tests (KTF-001 convention `func test_*`):
9 files, **48 tests**, all green through the real front end + SSA IR + C
runtime. Self-contained files (helpers in-file) via the runner's own-scope
model (`testFileOwnScope`).

Coverage: arithmetic + modulo signs, floats, precedence, control flow
(if/else/else-if, while, break/continue, boolean ops), functions + recursion,
strings, arrays (slices, 2D, push), structs, maps (incl. nested), classic
algorithms, and the Phase 81 regression set (unparenthesized `if`, `%`).

Gates:
- `pkg/cli/conformance_test.go` — runs the corpus (exit 0, 0 failed) and
  guards corpus erosion via parse + declaration counts.
- CI: `karkain test conformance/` + per-file `fmt --check` on every push.
- All 9 corpus files are formatter-canonical (`fmt --check` clean).

### 2. Examples probes corpus (`examples/probes/`)

11 deterministic print-based smoke programs (`hello`, `strings`,
`strings_concat`, `control_flow`, `functions`, `structs`, `arrays`, `maps`,
`math`, `algorithms`, `phase81`) with **pinned golden outputs** enforced by
`pkg/cli/probes_corpus_test.go`. A probe whose stdout changes fails CI — this
is the "basic smoke" safety net beyond a single-file E2E pass.

### 3. Algorithm corpus expansion (`examples/algorithms/`)

21 algorithms now: added factorial, fibonacci, gcd, lcm, sieve, power,
absolute_value — all with exact expected outputs in
`TestAlgorithmCorpus_RunsEveryProgram` and coverage-enforcing guard.
Notable finding: user functions that collide with C library symbols (`abs`)
fail at gcc; the corpus uses collision-free names and the limitation is
documented in the toolchain contract.

### 4. Toolchain contract (`karkain check --format=json`, `fmt`)

- Parser threads real token columns (`Parser.ErrorCols`) so parse diagnostics
  carry true 1-based columns; structured `Diagnostic` type +
  `karkain-diagnostics-v1` JSON schema (`pkg/diagnostics`).
- `check --format=json` emits a pure JSON diagnostic array; valid files emit
  `[]`; exit codes preserved.
- `karkain fmt` canonicalizes formatting token-level (idempotent, comment and
  string-safe, semantics-preserving — proven by re-running formatted sources).
- Both covered by `pkg/cli/phase82_cli_test.go` and
  `pkg/cli/check_test.go`-style integration tests.

### 5. IDE integration

- `karkain ide info` emits the machine contract (language, project detection,
  argv-ready toolchain commands, diagnostics schema, exit codes);
  `pkg/cli/toolchain_info.go` + tests.
- `karkain lsp` / `language-server` serve the real LSP server
  (`pkg/lsp/server.go`), replacing the former inline mock.
- VS Code extension rebuilt at `editors/vscode`: fixed `engines` object, real
  `extension.js` registering check/compile/run/format commands + a document
  formatting provider, corrected TextMate grammar scopes, settings
  (`compilerPath`, `formatOnSave`), README. Validated by
  `pkg/cli/vscode_extension_test.go` and `node --check`.

### 6. CI

`.github/workflows/ci.yml` test job now additionally runs the conformance
corpus and its format check after the existing E2E loop.

## Evidence

| Surface | Gate | Result |
|---------|------|--------|
| Conformance corpus | `karkain test conformance/` | 48 tests, 0 failed |
| Conformance Go gate | `TestConformanceCorpus_*` | PASS |
| Probes corpus | `TestProbesCorpus_RunsEveryProbe` | 11 golden probes PASS |
| Algorithm corpus | `TestAlgorithmCorpus_*` | 21 programs PASS |
| CLI contract | `TestCheckJSONFormat`, fmt tests, `TestToolchainInfoCommand` | PASS |
| VS Code extension | `TestVSCodeExtension_*` + `node --check` | PASS |
| LSP | `pkg/lsp/lsp_test.go` (pre-existing) | PASS |

## IMPLEMENTED vs PLANNED

**IMPLEMENTED:** conformance corpus; probes corpus w/ goldens; algorithm
expansion; structured diagnostics (`E-K-*`, real line+column, JSON schema);
`fmt` + `--check`; `ide info`; real LSP wiring; VS Code extension shell;
toolchain/IDE docs; CI coverage for corpus + format.

**PLANNED (next phases):** LSP diagnostics/sync into an editor with full
semantic features; Problems-panel + semantic highlights in the VS Code
extension; true column numbers for name-resolution errors; compiler symbol
namespacing (C-library collision fix); `fmt` style-grade formatting; conformance
growth for language fundamentals added by later phases.

## Known limitations (documented)

- `E-K-RES` errors are line-anchored (column 1).
- C-library symbol collisions (`abs`) are a live compiler limitation
  (`docs/audit/PHASE-82-TOOLCHAIN-CONTRACT.md`).
- `while`/`for` continue to require parenthesized conditions (only
  unparenthesized `if` is supported after Phase 81).
- Returning arrays from functions fails at the C stage — not part of Phase 82
  scope (probes/algorithms avoid the pattern).

## Next steps

1. LSP: wire diagnostics + document sync; then completion/hover.
2. VS Code: consume the LSP; surface diagnostics in the Problems panel.
3. Compiler: symbol namespacing to remove C collisions; fix `E-K-RES`
   columns; allow array returns.
4. Grow the conformance corpus with every future language feature.