# Phase 146D BASELINE — Generics v1 Close-Out

Date: 2026-09-25. Parent: `docs/audit/PHASE-146-BASELINE.md` §5 (slice plan
as executed). 146A (Go engine), 146B (stdlib), 146C (kcc parity) are
committed, merged, pushed, CI green.

## 1. Scope (locked — no new language features)

146D is close-out only, exactly the 146-baseline §5 item:

- **Gate consolidation**: one `phase146d_*` gate (or a documented decision
  that the 146/146B/146C gates already cover it) asserting the full
  146A–146B–146C suite green in a single invocation.
- **CI wiring**: Phase 146 gates added to `.github/workflows/ci.yml` as
  named steps (following the Phase 117/122/132 pattern), so `main` proves
  generics on every push.
- **Docs**: SPEC.md generics section + `docs/source/reference/stable-api.rst`
  generics row brought to the post-146C truth (both engines; K115; WASM
  K108 boundary; non-goals below). Sphinx `-W` HTML + linkcheck green.
- **Reports**: Phase 146 final-close record (this track ends with 146D;
  per-slice reports were intentionally deferred to the baseline only).

## 2. Starting position (verified)

- Gates green locally: `phase146` (7 tests), `phase146b` (4), `phase146c`
  (10/10 incl. compiler self-check); full 146A–C legs = 21 tests.
- CI green on `main` through the 146C fix commits; Phase 117/118/119/122
  unblocked behind them.
- KIR pin already refreshed 8986→9574 for the 146 compiler growth
  (deterministic 9574/9574 text/verify); `stable-api.rst` list-table indent
  fixed (CI docs ERROR resolved).
- Open from 146C closure: combined `TestPhase120–145` sweep was
  ENVIRONMENT-LIMITED (interrupted, not failed) on the 4GB host — 146D may
  either close it on a capable host or carry the limitation note forward.

## 3. Non-goals (from 146-baseline §6, restated — NOT defects)

Variance (invariant only), HKT, trait objects/dispatch, generic methods,
const generics, checked constraints, WASM lowering (K108 stays), generic
inference. NOTE on inference: the 146 baseline named it a "147 candidate",
but 147 landed as Native Execution P1 Resolution — inference has NO phase
number and is future work, not 146D scope.

## 4. Exit criteria

- [ ] Consolidated 146D gate green (local + CI named step).
- [ ] SPEC.md + `stable-api.rst` describe the shipped v1 surface honestly.
- [ ] Sphinx `-W` HTML + linkcheck green.
- [ ] AGENTS.md 146D completion record appended.
- [ ] Commit `develop` → merge `main` → push both (hard rule).

## 5. Explicitly out of scope

146D does not change `VERSION` (stays `1.1.0`), cut any tag, touch
bootstrap/seed/native tracks, or start 150+ work. The v1.1.0 tag cut belongs
to phase 152.
