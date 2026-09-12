# Phase 114 — Complete Example Corpus: Final Report

## Status: PASS — committed as `develop` commit, merged to `main`, pushed

## Mission

Populate the 15-category example framework established by Phase 113 with a
real, organized, tested, honest `.kark` corpus, pin it behind an automated
gate, document every category in Sphinx, and publish it as the official
demonstration layer.

## Result

The nearest the docs could realistically present is the honest
Developer Preview vocabulary the whole corpus now lives by:

- **46 `.kark` files** across 15 categories
- **43 Runnable directly** (byte-identical on both engines via `karkain run`)
- **1 test-mode** file (validated through `karkain test`)
- **2 Experimental** (concurrency — Go engine only, documented boundary)
- **4 Planned** categories (networking, database, web, quantum — README only)

Every Runnable example is pinned to a golden output on **both engines**
(Go front end + self-hosted `kcc`) by the new gate
`pkg/cli/phase114_examples_test.go`.

## Categories delivered

| # | Category | Files | Status |
|---|----------|-------|--------|
| 01 | Fundamentals | 12 | Runnable · both engines |
| 02 | Algorithms | 10 | Runnable · both engines |
| 03 | Systems | 1 | Runnable · both engines |
| 04 | Networking | 0 | Planned (README only) |
| 05 | Data | 3 | Runnable · both engines |
| 06 | Database | 0 | Planned (README only) |
| 07 | Web | 0 | Planned (README only) |
| 08 | Concurrency | 2 | Experimental · Go engine |
| 09 | AI | 2 | Runnable · both engines |
| 10 | Machine Learning | 2 | Runnable · both engines |
| 11 | Quantum | 0 | Planned / infrastructure-only |
| 12 | Scientific Computing | 4 | Runnable · both engines |
| 13 | Finance | 4 | Runnable · both engines |
| 14 | Security | 4 | Runnable · both engines |
| 15 | Developer Tools | 2 | Runnable · both engines / test-mode |

## Honesty rules honored

- **No invented APIs.** AI/ML examples are hand-rolled arithmetic in ordinary
  Karkain; there is no `std.ai`. Quantum is classified as *Planned /
  infrastructure-only* because the lexer/parser do not wire
  `circuit`/`qubit`/`qpu` (verified empirically with `karkain check`).
- **Documented parity gaps.** Wildcard match arms, qualified enum variants
  and function values are excluded from Runnable examples (verified broken);
  string-array printing differs across engines and is avoided; float math
  uses only `+ - * /` because the float builtins are unreliable.
- **Deterministic.** Floats print identically at 6 significant figures on
  both engines (e.g. `1.41421`, `0.333333`).
- **Byte-identical NIST/RFC vectors.** `sha256("abc") =
  ba7816bf…`, `sha256("") = e3b0c442…`, `sha512("abc") = ddaf35a1…`, RFC 4648
  hex/base64 round-trips — all verified on both engines.

## Verify script

`scripts/verify-examples.ps1` classifies each `.kark` by its
`// Status: Runnable|Experimental|Planned` + `// Engine:` header comment and
runs the real CLI: **45 passed, 0 failed, 5 skipped** (4 planned + 1
test-mode).

## Gate coverage

`pkg/cli/phase114_examples_test.go`:

- `TestPhase114_CorpusExamples_GoEngine` — all 45 files through the Go front
  end, pinned goldens.
- `TestPhase114_CorpusExamples_KCCParity` — 43 both-engine files through the
  self-hosted kcc engine, byte-identical parity + pinned goldens.
- `TestPhase114_CorpusTestFiles` — `*_test.kark` through the real
  `karkain test` runner (`3 passed`).
- `TestPhase114_CorpusCoverage` — inventory completeness guard.

Measured: Go leg ≈ 102 s, kcc leg ≈ 124 s on this host.

## Documentation

- All 16 Sphinx example pages rewritten/updated
  (`docs/source/examples/*.rst`) with real corpus status, per-file tables and
  excerpts from the checked-in sources.
- New `docs/source/reference/example-matrix.rst` (capability matrix),
  included in the Reference toctree.
- `examples/EXAMPLES.md` authoritative inventory.
- Per-category READMEs (all 15).

## Known corrections during the phase

- `sha256`/`sha512` return hex strings; `hex_encode(sha256(...))` double-encodes.
- Earlier stdlib function-name list in planning notes (`map_keys_of`,
  `map_contains`) corrected to the real surface (`map_keys`, `map_contains_key`).
- Previous EXAMPLES total (44) corrected to 46 files (45 run golden-pinned +
  1 test-mode).

## Regression status

- `go build ./...`, targeted suites green; `pkg/cli` Phase 114 gates green.
- Strict Sphinx build: **0 warnings, 0 errors**; linkcheck clean.

## Artifacts

- Gate: `pkg/cli/phase114_examples_test.go`
- Verifier: `scripts/verify-examples.ps1`
- Inventory: `examples/EXAMPLES.md`
- Docs: `docs/source/examples/*.rst`, `docs/source/reference/example-matrix.rst`
- Reports per category: README files under each `examples/NN_name/`