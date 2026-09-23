# Phase 143 BASELINE — Seed-Binary Bootstrap Closure (Go-free)

Date: 2026-09-23. Target: SOVEREIGNTY opener (after 142).

## 1. Bootstrap anatomy (pkg/bootstrap/bootstrap.go)

- Stage 1 (needs Go): `go build ./cmd/karkain` → `karkain-stage1` →
  transpile main.kark → gcc → `karkain-compiler1`.
- Stages 2/3 (already Go-free): seed binary (absolute path) transpiles +
  gcc links; `KARKAIN_ENGINE=go` pin is Go-CLI-side config, inert on the
  native seed. No `go` invocation anywhere in `runCompileStage`.
- Reproducibility: fixed SOURCE_DATE_EPOCH (binutils TimeDateStamp),
  5-min timeouts, progress heartbeats, K127 low-RAM guard on stages 2/3.

## 2. What "Go-free" needs (and doesn't)

- Needs: a SEED (142 downloadable ceremony, or stage-1 built) + gcc.
  The closure mechanism already exists (`RunStage2/3` take any binary).
- Needs PROOF: a gate that builds the seed with Go, removes Go from PATH
  (proving unresolvability), runs stages 2–3, asserts bitwise identity.
- Needs WORDS: bootstrap docs still present Go as the way, not history.
- Needs NOTHING in product code: zero new production paths (mirrors the
  Phase 130 fixtures+gates+docs-only precedent).

## 3. Environment contract for the gate

- `go` present (else nothing to seed from → skip), `gcc` present (else
  skip), K127 RAM threshold met (else skip). Full closure runs on
  capable hosts + CI; dev host skips honestly (currently ~100–600 MiB
  free and falling).
- `t.Setenv("PATH", scrubbed)` gives test-scoped PATH surgery with
  automatic restore; no product env plumbing required.

## 4. Slice plan (as executed)

- **143A**: `TestBootstrap_SeedClosure` + `TestBootstrap_ScrubGoFromPath`
  (new file, no product changes) + CI step.
- **143B**: self-hosted-compiler.md Seed Closure section.
- **143C**: fast unit suites, vet, reports, commit→merge→push.
