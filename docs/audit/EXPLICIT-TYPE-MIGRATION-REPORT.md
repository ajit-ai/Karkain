# Karkain — Repository-Wide Type-First Migration Inventory

**Date:** 2026-11-19
**Version:** `1.2.0` (unchanged); tag `v1.2.0` unchanged.

## Discovery and reconciliation

| Metric | Count |
|---|---:|
| Tracked `.kark` files discovered | **303** |
| Untracked `.kark` files | 0 |
| Files changed (cumulative, preserved from prior passes) | **156** |
| Files added / removed / golden files deleted | 0 / 0 / 0 |
| Genuine legacy `let` declarations at start | 3,163 |
| Declarations converted in this assignment | **3,189** |
| Genuine legacy `let` declarations at end | **987** |

## Conversions by pass

- Pass 1 (literals, typed symbols, explicit signatures, builtins): 1,797 declarations / 61 files.
- Pass 2 (function returns derived from `return` statements): 1,290 declarations / 72 files.
- Pass 3 (empty arrays/maps from `push`/index assignment, same-file identifiers): 102 declarations / 19 files.
- Eight unsafe inferences were reverted after checker diagnostics (exact names in the final report).

## Final classification (exact locations in `EXPLICIT-TYPE-MIGRATION-REMAINING-LET.md`)

| Category | Count | Area |
|---|---:|---|
| Self-hosted compiler internals | 535 | `src/compiler/**`, `compiler/**` |
| Call-result / expression | 412 | examples, stdlib, tests, conformance |
| Negative fixtures | 40 | deliberately invalid syntax fixtures |

## Verification

- `go build ./...`, focused `go vet`, `go test ./pkg/sema -run TestTypeFirst`: PASS.
- Acceptance A–E and typed MAP: PASS on both Go and self-hosted KCC engines.
- Self-hosted compiler rebuilt from migrated sources: PASS.
- Differential execution vs HEAD: 3/3 exact stdout matches.

The zero-legacy acceptance condition is **not** met; the residual declarations are
recorded with exact file:line locations rather than guessed types.
