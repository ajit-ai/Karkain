# Phase 115 — Developer Preview Readiness: Final Report

## Status: PASS — committed to `develop`, merged to `main`, pushed

## Mission

Make Karkain honestly obtainable, buildable and usable by an external
developer; label every capability truthfully; validate a fresh checkout;
harden the error experience; align all version identity to the Developer
Preview label; re-run the full regression; and publish the phase under
**`🚀 Karkain Developer Preview (v0.115.0)`** — never "1.0".

## Deliverables (the 13 acceptance items)

1. **Obtainable** — canonical remote `https://github.com/ajit-ai/Karkain.git`
   documented in README/CONTRIBUTING; `VERSION` file at the root.
2. **Buildable** — `go build ./...`, bootstrap `go build ./cmd/karkain`,
   self-hosted kcc rebuild (staleness path) all verified on this host.
3. **Usable** — `karkain run/check/build/test` demonstrated from a fresh
   local clone by `TestPhase115_FreshCheckout`.
4. **Honest capability labels** — README What-Works/Planned/Experimental
   matrix; docs status vocabulary; per-example `// Status:`/`// Engine:`
   headers; SPEC version history corrected.
5. **Baseline validation** — full regression below.
6. **Fresh-checkout simulation** — automated gate clones the public tree,
   builds, and runs/checks/tests a hello program.
7. **Error-experience audit** — before/after table + friendly typo hint and
   missing-file diagnostic (see Readiness report).
8. **Documentation updated** — README, docs index, developer-preview page,
   status pages, strict Sphinx + linkcheck.
9. **Non-applicable/experimental features labeled** — concurrency
   Experimental (Go engine), quantum/networking/database/web Planned,
   wasm/cross-cross-toolchain boundaries footnoted.
10. **Release notes** — `docs/source/release-notes.rst` (v0.115.0, explicit
    "intentionally no v1.0.0 claim", Phase 91 correction).
11. **Contribution guide + code of conduct** — `CONTRIBUTING.md`,
    `CODE_OF_CONDUCT.md` (Contributor Covenant 2.1).
12. **License** — `LICENSE` (MIT, 2025-2026 The Karkain Contributors).
13. **CI review + feedback + public status** — `ci.yml` vet step,
    phase-114/115 gate steps, extra required doc pages; GitHub Issues as the
    feedback channel; Developer Preview surfaced in README/docs/AGENTS/CI.

## What changed

### Repository integrity (new files)
- `LICENSE`, `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, `VERSION`
- `docs/release/developer-preview-checklist.md` (16-section gate)
- `docs/audit/PHASE-115-DEVELOPER-PREVIEW-READINESS.md` (this audit)

### Rewritten / updated
- `README.md` — honest Developer Preview status, quick start, capabilities,
  CLI, cross-compilation, structure, roadmap, feedback, license.
- `AGENTS.md`, `ROADMAP.md` (header + Phase-91-superseded banner),
  `SPEC.md` (version + history table).
- `docs/source/{index,release-notes}.rst`,
  `docs/source/development/developer-preview.rst`,
  `docs/source/tools/cli.rst` (version example),
  `docs/source/conf.py` (`linkcheck_ignore`, version default).
- `examples/showcase/README.md` toolchain line.

### Compiler + CLI
- `cmd/karkain/main.go` — version string + friendly missing-file check.
- `pkg/cli/commands.go` — version string + `suggestCommand`/`editDistance`
  "Did you mean" hint in `ValidateKarFile`.
- `src/compiler/main.kark` — self-hosted version strings (3 sites).
- `src/compiler/codegen.kark` — generated-C headers (6 sites).
- `pkg/codegen/{dwarf,qasm,qec,qir,openpulse,quantum_opt}.go` — producer /
  generated headers now v0.115.0.
- `scripts/release-all.{sh,ps1}` — default version v0.115.0.

### Tests + CI
- `pkg/cli/phase115_developer_preview_test.go` — the phase gate (6 tests).
- `pkg/bootstrap/args_test.go`, `pkg/cli/phase88_test.go`,
  `pkg/cli/phase90_test.go` — version assertions -> v0.115.0.
- `.github/workflows/ci.yml` — vet/phase-114/115 gates + doc pages.

## Gates

See `PHASE-115-DEVELOPER-PREVIEW-READINESS.md` for the full table. Summary:
phase-115 gate 6/6 PASS (92.9 s), phase-114 both-engine corpus PASS
(Go 99.8 s batch, kcc parity 181.0 s), phase-88/90/99 version+self-hosted
gates PASS, `pkg/codegen` full PASS, bootstrap version PASS, whole-tree
`go build`/`go vet` PASS, strict Sphinx PASS, `linkcheck` PASS.

## Honesty notes

- The Phase 91 "KARRKAIN 1.0 — RELEASE READY" record is superseded and every
  forward-facing document says so; historical audit reports are retained and
  marked superseded, never rewritten.
- One prior environment artifact surfaced and was removed: a stale root
  `karkain.exe` built earlier with `-ldflags -X main.versionString=…v1.0.0…`
  that was shadowing `go build` in the phase-88/90 gates.

## Boundaries

- Public visibility of the GitHub web/API pages cannot be verified from this
  host (bot 404 for the repo root too); git access is verified. This is an
  environment limitation, not a code defect.
- `linkcheck_ignore` covers the two canonical GitHub URLs for the same reason.

## Evidence

- Gate log tail + before/after error-probe output: Readiness report.
- Fresh-checkout: `TestPhase115_FreshCheckout` local-clone run.
- Version identity: `karkain --version` ->
  `Karkain Compiler v0.115.0 (windows/amd64, Developer Preview Build)`.

## Revision bookmark

| Event | Commit |
|-------|--------|
| Phase 115 code + docs + gates committed (develop) | TBD |
| Merge to main | TBD |
| Report identifier backfill (develop/main) | TBD |