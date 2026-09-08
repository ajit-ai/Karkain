# Phase 96 — Self-Hosted kcc Owns the Test Runner

## Objective

Make the self-hosted compiler (`kcc` from `src/compiler`) the primary engine
for `karkain test`. The self-hosted test runner must reproduce the Go CLI's
runner contract (discovery, per-test execution, filtering, exit-code mapping)
so that the parity gate can prove kcc and Go agree on the same test corpus.

## Deliverables

### Self-hosted runner in `src/compiler/main.kark`

- `runTests(testPath, filter)` — top-level driver. Mirrors
  `TestCommandFiltered` flow:
  - `collectTestFiles` → recursive `*_test.kark` discovery (mirrors Go
    `findTestFiles`), sorted; single-file fallback; empty directory reports
    `No test files found.`
  - per-file: read → tokenize → parse → error report → `collectTestNames`
    (top-level `test_*` funcs) → `applyFilter` (`--filter` substring,
    matching `pkg/testing.Filter`)
  - fast path: one synthesized driver `main` calling every selected test;
    clean exit → all PASS; nonzero exit → per-test drivers to isolate
    failures
  - `compileRunDriver` — parse/generate(`c23`)/`gcc -std=c99 -x c
    -o karkain_test` compile + run, deterministic temp artifacts
    (`karkain_test.c`, `karkain_test`, `karkain_test.exe`), all cleaned up
  - Go-parity summary:
    `=== Test Summary: N tests, P passed, F failed, S skipped ===`
    and `N passed; F failed; 0 skipped; N total` (parsed by the Go wrapper)
- `sortStrings` — deterministic lexicographic insertion sort for stable,
  Go-parity discovery output.

### Go wrapper in `pkg/cli/kcc_engine.go`

- `kccSummaryRe` — matches `(\d+) passed;\s*(\d+) failed;\s*(\d+) skipped;\s*(\d+) total`
- `KCCTestCommand(w, testPath, filter)`:
  - locates the kcc binary (`kccBinaryPath`, staleness-aware rebuild)
  - runs kcc in a temp sandbox (`runKCCDir`) so synthesized driver
    artifacts never land in the user's working directory
  - `No test files found.` → `ExitSuccess`
  - `failed > 0` → `ExitTest(4)`; otherwise `ExitSuccess`
- `runKCCDir` — `runKCC` with an optional working directory.

### CLI routing in `cmd/karkain/main.go`

- `karkain test --engine=kcc` / `KARKAIN_ENGINE=kcc` → `KCCTestCommand`;
  default remains the Go runner (`TestCommandFiltered`). Phase 95 kept kcc
  behind `--engine` for `check/build/run`; phase 96 extends the same contract
  to `test`.

## Root Causes Fixed

1. **`INT==BOOL` makes `values_equal` return 0.** `endsWith` returns
   `TYPE_INT` on its normal path (from a string `==`), so
   `endsWith(name, "_test.kark") == true` lowered to
   `binary_op(..., "==", make_bool(1))` → runtime `binary_op ==`
   falls through to `values_equal`, which returns `make_int(0)` when the
   operand types differ. Discovery therefore matched zero files and the Go
   runner's `findTestFiles` behavior (all `*_test.kark`) was not reproduced.
   Fix: bare truthiness (`if (endsWith(...))`) — `is_truthy` handles both
   `TYPE_INT` and `TYPE_BOOL`. The SSA emitter already normalizes
   `Ident == true` to `make_int(1)` (why `loadSourceWithSiblings` worked),
   but a call-result `== true` was left as `make_bool(1)`.

2. **Windows `system("./exe")` fails.** `cmd` rejects the `./` prefix
   (`'.' is not recognized`). kcc resolved the artifact to
   `karkain_test.exe` then ran `system("./karkain_test.exe")`. Fix: run a
   bare name when the artifact ends in `.exe`, keep `./` only on POSIX-style
   bare names.

3. **Empty directory misclassified as a single file.** `listFiles` returns an
   empty array for both a real empty directory and a single file. kcc treated
   the path as a single file and emitted a misleading read error. Fix: adopt
   the path as a single file only when it has a `_test.kark` suffix or reads
   back as source (`readFile(path) != ""`); a directory yields `""` on
   Windows `fopen`. Empty directories now report `No test files found.`
   exactly like Go.

## Parity Gate — `pkg/cli/phase96_parity_test.go`

| Test | Asserts |
|------|---------|
| `TestPhase96_KCCConformanceParity` | kcc `test` over `conformance/` — 59 assertions, same per-test PASS lines + summary as Go |
| `TestPhase96_KCCFailingTest` | a driver with an intentional `assert_eq` failure → kcc isolates the failing test (one PASS, one FAIL), summary and `ExitTest(4)` |
| `TestPhase96_KCCSingleFileAndFilter` | single-file path used as-is; `--filter` runs only matching `test_*` |
| `TestPhase96_KCCEmptyDirectory` | empty directory → `No test files found.` + `ExitSuccess` |

## Verification

- `karkain test --engine=kcc conformance` — 11 files, 59/59 PASS, `0 failed`,
  identical summary format to Go.
- `TestPhase96_*` — 4/4 pass.
- Conformance parity (Go engine) — 59/59.
- `TestBootstrap_BitwiseIdentity` — stage2 == stage3 bitwise identical
  (stage2/stage3 SHA `f39111a2…`), unchanged from phase 95.
- Full regression: `pkg/lexer`, `pkg/parser`, `pkg/codegen`, `pkg/pm`,
  `pkg/cli` (including `TestPhase95_*`, conformance corpus, probes corpus,
  namespace/array-return regression) — GREEN.
- `go build ./...`, `go vet ./pkg/cli/ ./pkg/bootstrap/` — clean.

## Files Changed

- `src/compiler/main.kark` — self-hosted test runner (`runTests`,
  `collectTestFiles`, `collectTestNames`, `applyFilter`, `buildCallList`,
  `compileRunDriver`, `sortStrings`); `test` command now accepts `[filter]`.
- `pkg/cli/kcc_engine.go` — `KCCTestCommand`, `kccSummaryRe`, `runKCCDir`.
- `cmd/karkain/main.go` — `test` routing to kcc when `engine == EngineKCC`.
- `pkg/cli/phase96_parity_test.go` — new phase-96 parity gate.
- `docs/audit/PHASE-96-FINAL-REPORT.md`, `AGENTS.md`, `ROADMAP.md`.

## Status

**COMPLETE.** Phase-96 parity gate green; bootstrap identity preserved; full
regression suite green; committed to `develop` and merged to `main`.