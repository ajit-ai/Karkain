# Workstream B completion report - maturity scorecard and governance

**Date:** 2026-10-15
**Branch:** `develop`
**Scope:** documentation only. No compiler behaviour, gate, CI step or release
state was changed by this workstream.

This report answers the requirement "identify each original requirement and mark
it complete, partial, blocked, or not started, with supporting evidence" for
Workstream B of the master plan. Workstream statuses for A-E as a whole are in
the master completion report.

---

## 1. Required deliverables

| # | Requirement | Status | Evidence |
|---|-------------|--------|----------|
| B-1 | `docs/audit/KARKAIN_MATURITY_SCORECARD.md` | **complete** | Rewritten: 332 lines, 30 dimensions, scoring model, coverage matrix, gates, benchmark policy, limitations, traceability (sections 2-8) |
| B-2 | `ROADMAP.md` | **complete** | `Language Quality & Gates (2026-10)` section. Corrected in this pass: it described the scorecard as "groups A-D" after the restructure, and anchored the evidence at `main` = `f66863d`; both now read four categories and `develop` = `cc44c95` |
| B-3 | `docs/audit/KARKAIN-VERSION-PLAN.md` | **complete** | Pointer row added, then corrected in this pass to name the four categories and the current commit |
| B-4 | Completion report | **complete** | This document |
| B-5 | Separate index document "after checking existing conventions" | **not started, deliberately** | No index document was created: `AGENTS.md` and `KARKAIN-VERSION-PLAN.md` already hold the authoritative pointers, and a third index would create the competing source of truth the master plan forbids. Workstream C is where a new control entry point becomes justified |

## 2. Scorecard requirements

| # | Requirement | Status | Evidence |
|---|-------------|--------|----------|
| B-6 | Retain the 30-dimension structure | **complete** | Exactly 30 rows: OM1-OM7 (7), AR1-AR9 (9), CE1-CE8 (8), EX1-EX6 (6), verified mechanically |
| B-7 | Map explicitly to the four required categories | **complete** | Operational maturity, Architecture, Compiler engineering, Ecosystem are the section headings (2.1-2.4) |
| B-8 | Explicit coverage matrix | **complete** | Section 3 maps each required area to its dimensions and states depth, including "Weak" verdicts |
| B-9 | Traceability to the requested taxonomy | **complete** | Section 8 maps all 30 previous rows (A1-A10, B1-B8, C1-C7, D1-D5) to their new identifiers; none dropped silently |
| B-10 | Performance and reproducible benchmarks | **complete** | OM6 + CE8, scored 2 with the reason stated: one benchmark function, no baseline, no threshold |
| B-11 | Binary size and build characteristics | **complete** | CE8, scored 2: build characteristics measured (105/134/150D); no size budget exists |
| B-12 | Undefined behaviour and safety boundaries | **complete** | CE7 + OM5; defined answers for shift edges, division, index bounds, arena exhaustion |
| B-13 | Integer and floating-point semantics | **complete** | CE4 + AR8, with the asymmetry stated: strong on the C path, absent on the native path |
| B-14 | ABI and platform compatibility | **complete** | CE4; calls ABI documented, Mach-O execution NE, no arm64 |
| B-15 | Fuzzing and parser/compiler robustness | **complete** | OM7, scored 2: no-progress guard and multi-error recovery exist, zero fuzz targets |
| B-16 | Security and supply-chain practices | **complete** | OM5, scored 2: policy docs and checksums; no SBOM, signing or enforced model |
| B-17 | Conformance and oracle-based parity | **complete** | CE3 + EX5 + AR7, scored 4 |
| B-18 | Testing and CI enforcement | **complete** | OM1 + EX5, scored 4 |
| B-19 | Documentation, onboarding, contributor experience | **complete** | OM3 (3) + OM4 (2) |
| B-20 | Release reproducibility and operational recovery | **complete** | OM2 (3) + OM4 (2) |
| B-21 | Do not inflate scores; each rating links to evidence or states evidence is missing | **complete** | Six scoring rules in section 1, including "a two-feature dimension takes the lower score" and "planned is not implemented" (154 scores 3 for delivery, not for having a baseline) |
| B-22 | Distinguish defined / executed locally / CI-enforced | **complete** | Section 5 has a column per state; Gates A and E are reported as **"Not re-run"**, not as green |
| B-23 | Do not claim Gates A-F passed without actual results | **complete** | Section 5 records the real results (D2 7/7 shapes, `KnownNonParity` PASS, `TestPhase151` family 232.9 s, KIR `text=14050 verify=14050` at 908.3 s, D5 3/3) and states the full QA battery was not run |
| B-24 | Document stale KIR pins, unclassified crashes, missing measurements, unverified behaviour | **complete** | Section 6 items 1-7, each with evidence and its named decisive next test |

## 3. Benchmark and conformance policy

| # | Requirement | Status | Evidence |
|---|-------------|--------|----------|
| B-25 | Reproducible benchmark inputs and environment | **complete** | 4.1: host class, toolchain versions, memory environment, repeat policy, input identity |
| B-26 | Baselines and measurement methodology | **complete** | 4.2: a baseline is a recorded measurement of a named revision; unbased metrics are labelled; absolute figures preferred over ratios |
| B-27 | No invented numerical performance targets | **complete** | 4.2 states the rule; the scorecard publishes no throughput, time or size target |
| B-28 | Correctness checks before performance comparisons | **complete** | 4.3, with the repository-specific prerequisites named |
| B-29 | Go-oracle comparisons | **complete** | 4.4 defines byte identity / structural equivalence / semantic equivalence and what each is admissible as |
| B-30 | Byte identity versus semantic equivalence criteria | **complete** | 4.5, including the Phase 9b precedent that structural validity did not imply behavioural correctness |
| B-31 | Platform-specific expectations | **complete** | 4.6 table; NE is never reported as passing |
| B-32 | Regression thresholds and evidence retention | **complete** | 4.7; timing is never golden-tested (Phase 110 precedent); every claim keeps its figure |
| B-33 | supported / experimental / deferred / blocked rules | **complete** | 4.8 vocabulary with in-force examples |
| B-34 | Distinguish structural checks, execution tests, semantic conformance | **complete** | 4.4 table and 4.5 |

## 4. Documentation validation

| # | Requirement | Status | Evidence |
|---|-------------|--------|----------|
| B-35 | Run the configured documentation checks | **complete** | `python -m sphinx -b html docs/source docs/build/html -W --keep-going -E` -> **build succeeded, zero warnings**. The first invocation was cached ("no targets are out of date"), so it was repeated with `-E` to force a fresh-environment build; a cached pass would have been weak evidence |
| B-36 | Validate internal links and document references | **complete** | Every backticked path-like reference in the three modified governance documents was extracted and resolved repo-wide. Four non-file references remain and all are legitimate: `checksums.txt` (a release artifact), `vX.Y.Z-CHECKLIST.md`, `std.json`, `_test.kark` (templates and globs) |
| B-37 | Validate version consistency | **complete** | `VERSION` = 1.1.0 and tag `v1.1.0` both exist; the scorecard and ROADMAP anchors now name `develop` = `cc44c95`; the v1.2.0-in-progress state is stated as in-progress and is not presented as released |
| B-38 | Validate roadmap dependencies | **complete** | The 150 -> 151 -> 152 -> 154 order and the two v1.3.0 carry-overs are present and consistent between `KARKAIN-VERSION-PLAN.md` and the v1.2.0 checklist. One stale status was found and is assigned to Workstream E (see section 6) |

## 5. Defects found and corrected in this workstream

| Defect | Correction |
|--------|------------|
| `ROADMAP.md` described the scorecard as "groups A-D" after the restructure to four categories | Corrected, with the four category names written out |
| `ROADMAP.md` anchored the evidence at `main` = `f66863d`, which is no longer where the newest evidence lives (D5 is committed on `develop`) | Corrected to `develop` = `cc44c95` |
| `KARKAIN-VERSION-PLAN.md` pointer did not state the category structure or the commit | Corrected |

## 6. Carried forward, not done here

| Item | Why not here | Owner |
|------|--------------|-------|
| `docs/release/v1.2.0-CHECKLIST.md` still records the pre-D4/D5 non-parity figures (`array_index` 481 vs 1096, `len_of_array` 439 vs 454, `return_value` 348 vs 353) | It is a release-status record, which is Workstream E's scope. The correction is evidence-backed and prepared there | E |
| A rollback runbook (OM4 scored 2 for want of one) | Not a documentation-validation item; it is a new operational artefact | future increment |
| A performance baseline (OM6 scored 2) | The policy now exists; a baseline requires measurement runs that the master plan does not authorise as compiler work | future increment |

## 7. Gate B verdict

**Gate B is satisfied.** The required taxonomy and coverage mapping are complete
(30 dimensions, four categories, coverage matrix, traceability), the benchmark
and conformance policy exists in full, the documentation checks ran and their
results are recorded, and no statement in the scorecard overstates maturity:
the weakest areas (performance, fuzzing, security, MacOS and accelerator
execution) are scored low or NE, and two gates are reported as not re-run
rather than as passing.
