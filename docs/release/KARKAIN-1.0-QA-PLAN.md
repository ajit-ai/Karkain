# Karkain 1.0.0 — QA Plan

The master QA gate is **`scripts/qa/run-full-qa.ps1`** (Windows/PowerShell
5.1+, PowerShell Core cross-platform). It orchestrates the *existing* test
machinery rather than duplicating it: every stage it runs is a pre-existing
suite or script. A stage failure is a failure of the gate (non-zero exit);
stage names are printed so the failing stage is identifiable.

## Stages

| Stage | What it runs | Failure = |
|-------|--------------|-----------|
| `build` | `go build ./...` | Go build break |
| `vet` | `go vet ./...` | static-analysis finding |
| `units` | `go test` over all non-CLI packages (`-count=1 -p 1`) | unit regression |
| `cligates` | phase 114/115/116/117/118 CLI gates (monolithic, gcc-gated) | corpus/parity/identity regression |
| `conformance` | `karkain test conformance/` + `fmt --check` on conformance files | conformance regression |
| `probes` | the probes corpus gate | deterministic probe regression |
| `examples` | `scripts/verify-examples.ps1` (49/0/5 contract) | example run regression |
| `docs` | Sphinx `html -W --keep-going` + `linkcheck -W` | doc-tree/link regression |
| `fresh` | `scripts/beta-fresh-checkout.ps1` (fresh-checkout gate) | checkout regression |
| `install` | `scripts/install.ps1` + `scripts/verify-install.ps1` | install regression |
| `journey` | `scripts/verify-rc-journey.ps1` (11-step external journey) | journey regression |

## Execution model

- Stages run in order and abort at the first failure (deterministic).
- `-Stage <name>` runs a single stage; `-Skip <name,...>` skips stages
  (documented for constrained hosts, e.g. the ~4 GB OOM class where the
  monolithic gate must run in isolation).
- No build artifacts are committed by the gate (temp dirs are cleaned).
- Exit code: `0` = all stages green; non-zero = the failing stage is printed
  last.

## Evidence

Per-phase/audit reports record the `go test` timing and the raw pass/fail
lines; the Phase 119 battery results are in
`docs/audit/PHASE-119-LANGUAGE-QA-FINAL-REPORT.md`.