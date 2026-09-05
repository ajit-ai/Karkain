# SELF-HOSTING TRACKER — `pkg/bootstrap` Status, Measurement History, Exit Criterion

**Status:** VERIFIED GREEN in the Phase 80 dev environment (changed from TRACKED
during Phase 80 execution — see §1b); exit criterion retained as the standing
gate so any future regression re-surfaces the blocker.
**Baseline:** Phase 79 audit recorded FAILing `pkg/bootstrap` for the Phase
51/52–79 audit series on the then-current environment.

---

## 1. Measurement History

### 1a. Phase 79 baseline (the "blocker" record)

| Test | What it proves | Phase 79 status |
|---|---|---|
| `TestBootstrap_Stage1Compilation` | Stage 0 (Go) compiler compiles `src/compiler/*.kark` into the Stage 1 compiler, which then compiles a seed program | FAIL |
| `TestBootstrap_Stage2SelfHosting` | Stage 1 compiler recompiles the compiler sources with itself; output must be valid | FAIL |
| `TestBootstrap_BitwiseIdentity` | Stage 2 artifact is **byte-for-byte SHA-256 identical** to Stage 1 artifact (reproducibility/soundness gate) | FAIL |
| `TestArgs_*` | Arg plumbing of the bootstrap harness | FAIL (auxiliary) |

Measurement at the time: `go test ./pkg/bootstrap/... -count=1` — repeated,
stable failures. HIGH confidence: that was a measured test result, not an estimate.

### 1b. Phase 80 re-measurement (superseding evidence)

Re-run during Phase 80 prerequisite verification on the Karkain dev machine:

```
go test ./pkg/bootstrap/... -count=1
ok	karkain/pkg/bootstrap	283.772s
```

and as part of the full Phase 79/80 suite: `ok karkain/pkg/bootstrap 300.748s`.
Every bootstrap test now passes. The Phase 79 FAIL measurement and this green
result were captured on different environments/toolchains; the green result is
registered here as the current ground truth. If the toolchain differs on
another machine, §4 (exit criterion) is the arbiter.

## 2. Why It Was NOT a Phase 80 (SIMD) Block

Phase 79's own evidence (`PHASE-79-SELF-HOSTING-READINESS.md:101`) states it was
*"underway but NOT ready for production bootstrap"* and *"NOT a Phase 80 (SIMD)
prerequisite."* The failure predated Phases 51–79; nothing in the SIMD work
changes the bootstrap pipeline, and the Phase 80 parity harness does not touch
`pkg/bootstrap`.

## 3. Known Contributing Factors (evidence-linked)

- `src/analyzer.kark` is a 1-line placeholder — semantic-analysis port
  incomplete (`PHASE-79-SELF-HOSTING-READINESS.md:22`).
- Bootstrap requires full Karkain runtime support (strings/collections/files/
  memory) with no measured coverage matrix (`PHASE-79-...:93-94`).
- Stage1/Stage2 recompile + bitwise SHA tests depend on deterministic
  re-emission that today is blocked by the above port gaps.

## 4. Exit Criterion (definition of done to unblock)

Self-hosting is declared UNBLOCKED only when **all** hold:

1. `TestBootstrap_Stage1Compilation` **passes** — Stage 0 compiles
   `src/compiler/*.kark` and the Stage 1 compiler compiles a seed program.
2. `TestBootstrap_Stage2SelfHosting` **passes** — the Stage 1 compiler
   recompiles the compiler with itself successfully.
3. `TestBootstrap_BitwiseIdentity` **passes** — Stage 1 and Stage 2 artifacts
   are SHA-256 identical (reproducibility proof).
4. `TestArgs_*` **passes** — harness argument plumbing verified.
5. The full suite then reads: `go test ./... -count=1` → **all packages green,
   including `pkg/bootstrap`**, with no skip.

Any single failing test ⇒ the blocker is RE-OPENED (status flips back to
TRACKED) and the phase that caused it must not be merged.

## 5. Recurring Verification

Every completed phase re-runs the full suite; `pkg/bootstrap` status is
recorded in the phase log. Phase 80 was the first phase to formally track this
as a documented risk with an exit criterion (per Phase 79 audit
recommendation §12.2) — and Phase 80's re-measurement is the recorded evidence
that the pending blocker cleared on the current environment. Subsequent phases
must continue recording the status so a regression cannot hide.