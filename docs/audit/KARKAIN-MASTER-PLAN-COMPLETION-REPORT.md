# Master completion report - Karkain controlled execution plan

**Date:** 2026-10-15
**Branch:** `develop` (unpushed)
**Baseline:** `develop` = `989a04c` at start, master report commit at end
**Nature of this work:** one compiler slice (A) plus four documentation and
governance workstreams (B-E). No merge, push, tag, publish or release was
performed.

---

## 1. Initial and final repository snapshots

| Property | Initial | Final |
|----------|---------|-------|
| Branch | `develop` (switched from `main` after proving the trees identical) | `develop` |
| HEAD | `989a04c` | this report's commit |
| `main` | `f66863d` | `f66863d` - **unchanged, not merged into** |
| `origin/develop` | `989a04c` | `989a04c` - **unchanged, nothing pushed** |
| `origin/main` | `f66863d` | `f66863d` |
| Uncommitted changes | 10 modified + 1 untracked | **none** - all preserved, committed in scope-separated commits |
| Stashes | 1 (`stash@{0}`) | 1, **untouched** |
| Tags | 5 | 5, **unchanged** |
| `VERSION` | 1.1.0 | 1.1.0 - **unchanged**, as required |

Branch-switch safety: `git diff main develop --stat` was **empty** before the
switch, so the trees were identical and the uncommitted work could not conflict.
SHA-256 hashes of all eleven changed files were recorded before the switch,
re-verified after it, and re-verified again after the D5 commit.

## 2. Workstream status matrix

| Workstream | Gate | Status | Commit |
|------------|------|--------|--------|
| **A** D5 array-index close-out | A | **complete** | `cc44c95` |
| **B** Maturity scorecard and governance | B | **complete** | `64c63c6` |
| **C** Implementation control system | C | **complete** | `024ca94` |
| **D** Unified type/variable declaration design | D | **complete (design only, as scoped)** | `6eff5a2` |
| **E** Roadmap, architecture and version policy | E | **complete** | `146bb37` |
| Cross-workstream consistency review | - | **complete** | this commit |

No gate is unmet. Two have explicitly partial content, stated rather than
hidden: Gate A's parity work is closed except for an 8-byte residual assigned to
D6, and Gate F's local evidence is partial because the full QA battery was not
run.

## 3. Workstream A - D5 close-out

The plan required verifying the reported claims rather than assuming them:

| Reported claim | Verdict | Evidence |
|----------------|---------|----------|
| D5 work uncommitted, test file untracked | true at start | 10 modified + 1 untracked |
| narrows `array_index` from 496 to 8 bytes | **true, with a correction** | Measured **1088** vs go 1096. The "496" was a pre-D4 figure; the true gap closed was 481 -> 8 bytes |
| bounds-error path exits 1 | **true** | Executed: out-of-range exits 1; valid indexing prints and exits 0 |
| frame delta deferred to D6 | **true** | Residual is exactly `sub rsp,0x290` vs `sub rsp,0x288` |

| Command | Result |
|---------|--------|
| `go test ./pkg/cli -run TestPhase151A5_KIRPinHolds -count=1` | **PASS** 908.3 s, `text=14050 verify=14050`; pin re-pinned from 13755 in two files, never hand-edited |
| `go test ./pkg/cli -run 'TestPhaseD5_\|TestPhaseD2_' -count=1` | **PASS** 36.5 s: D5 3/3, D2 differential 7/7 shapes byte-identical, `KnownNonParity` kcc 1088 vs go 1096 |
| `go test ./pkg/cli -run 'TestPhase151' -skip 'TestPhase151A5_KIRPinHolds' -count=1` | **PASS** 232.9 s (KIR skipped because it had just passed in isolation) |
| `python -m sphinx -b html docs/source docs/build/html -W --keep-going -E` | build succeeded, 0 warnings |

**Gate discipline:** the three D5 tests were preserved; one wrong expectation was
corrected (`"3"` -> `"3\n"`), which the test had been hiding with `TrimSpace`.
The gate asserts **exact stdout bytes**, which is what catches the Phase 9b
class of defect where a missing newline passed a trimmed comparison.

**CI wiring:** `.github/workflows/ci.yml` gained *Run Phase D5 kcc native
array-bounds gate* (`-run 'TestPhaseD5_'`, step at L324), verified present after
the commit, with the pattern checked not to collide with `TestPhaseD2_` or
`TestPhase151A*`. The YAML was re-parsed after editing, because this file has
been broken before by an editor stripping indentation.

**Residual:** 8 bytes of frame padding (D6, not selected). ELF and Mach-O gain
the same driver code but carry no new execution evidence on this host. No D6
work was started.

## 4. Workstream B - scorecard and governance

Full requirement-by-requirement table (B-1 to B-38):
`docs/audit/KARKAIN-GOVERNANCE-COMPLETION-REPORT.md`.

Delivered: the scorecard restructured into the four required categories with
exactly **30** dimensions (verified mechanically: OM 7 + AR 9 + CE 8 + EX 6), a
coverage matrix for all eleven required evaluation areas, a benchmark and
conformance policy, three-state gate reporting, known limitations, a
supersession table, and full traceability to the previous 30-row taxonomy.

Honesty properties that matter: no aggregate grade; a two-feature dimension takes
the **lower** score; Gates A and E are reported as "Not re-run" rather than
green; performance, fuzzing and security are scored 2, and MacOS and accelerator
execution are NE.

Docs checks: fresh-environment Sphinx `-W -E` build (the first, cached, run was
rejected as weak evidence); all backticked references in the three modified
documents resolved repo-wide, leaving four legitimate non-file references.

## 5. Workstream C - implementation control system

Delivered: `docs/audit/KARKAIN-IMPLEMENTATION-CONTROL.md` - one queue, seven
status values each with entry requirements, an authority hierarchy, evidence
conventions, a next-slice selection policy with exactly one recommendation, a
runner-up, an explicit non-recommendation, staleness detection rules, and a
three-place deferral process. `AGENTS.md` now names it as the authoritative
entry point.

Recommendation recorded: a root-cause slice for the intermittent kcc
`0xC0000005` (`proposed`, needs owner authorisation) - the only blocker on a
currently unmeasurable gate, with a named decisive experiment never yet run.
**D6 is explicitly not recommended**, because its 8-byte delta is cosmetic beside
the open value-model kinds and closing it moves `natFrameLayout`, which every
Step 1-9d displacement is pinned against.

Three stale records neutralised with evidence rather than silent rewrites:
`NEXT_PHASE_RECOMMENDATION.md` (recommends a parser fix completed in Phase
55c), `KARKAIN_IMPLEMENTATION_MATRIX.md` (six rows contradicted by later work,
each enumerated), `NAME-RESOLUTION-DESIGN.md` (claimed nothing implemented;
`pkg/sema/resolve.go` is 922 lines implementing its recommendation).

## 6. Workstream D - type and variable declaration design

Delivered: `docs/audit/UNIFIED-TYPE-DECLARATION-DESIGN.md`, design only. All
fourteen required topics present, verified mechanically. Three options compared;
the recommended one (`let mut`, with `var` deprecated) is additive.

The finding that shaped it: `let`, `var` and `const` are all parsed by one
function (`parseVarDecl`, `pkg/parser/parser.go:441`) producing one node type,
and **only `const` is immutable**. The most-read part of the language currently
carries no meaning.

Open decisions are visible: D-1 to D-6, with D-3 (`let` immutability)
identified as breaking because it breaks a frozen, gated corpus - the 151A Step
2b reference reassigns a `let`. Test case T9 records that cost rather than hiding
it. No implementation was performed; no syntax was changed.

## 7. Workstream E - roadmap and version policy

Verified against evidence: v1.1.0 released (`VERSION` plus tag); v1.2.0 in
progress with 150/151P0/151/152/154 in scope; 152 partial with conditions 2 and 3
carried to v1.3.0 verbatim; 154 baselined but **not implemented**; 165 not
started; D6 proposed and not promoted.

Corrections: the version plan's claim "152, 154 and 165 are not yet baselined"
was **false** - both 152 and 154 have baseline notes - and was replaced with a
per-increment table carrying baseline *and* delivery state. A version-policy
section now separates RELEASED / ACTIVE / FUTURE and states that `VERSION` moves
only at the tag cut. The checklist's residual non-parity figures (`481 vs 1096`,
`439 vs 454`, `348 vs 353`) were stale and are corrected to the single remaining
shape. The type design is placed as a design milestone.

## 8. Cross-workstream consistency review

| Check | Result |
|-------|--------|
| No user changes lost | **Pass** - all eleven files accounted for; hashes re-verified after the switch and after the D5 commit |
| D5 and governance in separate, scope-clean commits | **Pass** - five commits, one per workstream, none mixing workstreams |
| Every new gate has CI wiring | **Pass** - the `TestPhaseD5_` step is present at `ci.yml` L324 |
| Proposed designs not represented as implemented | **Pass after one correction** - the queue had marked `TYPE-DESIGN` as `approved` while the design document says awaiting sign-off; corrected to `proposed` and logged in the control document's change log |
| Scorecard agrees with test and CI evidence | **Pass** - every score cites a gate file, CI step or measurement; none rests on prose |
| No stale next-phase recommendation contradicts commits | **Pass** - three stale records carry evidence-backed historical banners |
| Roadmap version assignments consistent | **Pass** - `VERSION` 1.1.0, tag v1.1.0, v1.2.0 declared "In progress", future versions declared "not baselined" |
| Reports identify exact evidence and limitations | **Pass** - this report and the Workstream B report both state limitations, not only successes |
| No merge, push, tag or release occurred | **Pass** - `main` and both remotes unchanged; `develop` is 5 commits ahead of `origin/develop` and unpushed |

One defect was found by this review **in work produced during this execution**
(the `TYPE-DESIGN` status contradiction). It is recorded and corrected rather
than quietly fixed, per the rule that a later workstream documenting an earlier
one's defect prepares a focused corrective change.

## 9. Scorecard coverage summary and remaining evidence gaps

Coverage matrix: `docs/audit/KARKAIN_MATURITY_SCORECARD.md` section 3. Strong
areas: conformance and oracle parity, testing and CI enforcement, cross-engine
differential discipline. Weak areas, stated rather than smoothed: performance
(OM6 = 2, no baseline), fuzzing (OM7 = 2, zero targets), security (OM5 = 2, no
SBOM or signing), binary size (CE8 = 2, no budget), MacOS and accelerator
execution (NE).

## 10. Control-system entry point and current next task

Entry point: `AGENTS.md` -> `docs/audit/KARKAIN-IMPLEMENTATION-CONTROL.md`.

Recommended next: **a root-cause investigation slice for the intermittent kcc
`0xC0000005`**, status `proposed`, requiring owner authorisation because it
changes the build. Runner-up: the next native-driver value-model kind
(`151A-9i+`). Explicitly not promoted: D6.

## 11. Declaration-design decisions and unresolved questions

Recommended: `let mut` with `var` deprecated. Settled: one declaration path
remains, `Type string` stays for v1.x, mutability is a semantic-analysis concern
not a codegen one, diagnostics must assert code and name on both engines.
Awaiting sign-off: D-1 (adopt Option 2), D-2 (`var` alias duration), D-3 (when
`let` becomes immutable), D-4 (real type values), D-5 (field/parameter
mutability), D-6 (exported mutable globals).

## 12. Roadmap and version alignment

| State | Version | Evidence |
|-------|---------|----------|
| Released | 1.1.0 | `VERSION`, tag `v1.1.0` |
| Active | 1.2.0 "Sovereignty I" | increments 150, 151P0, 151, 152, 154; 165 free-floating |
| Future | 1.3.0+ | themes and exit criteria only, **not baselined** |

`VERSION` is unchanged at 1.1.0 and moves only in the tag-cut commit.

## 13. Commit inventory

| Commit | Workstream | Files |
|--------|------------|-------|
| `cc44c95` | A | 8 files: `src/compiler/{main,native_emit,native_value}.kark`, `pkg/cli/phase151d5_arrbounds_test.go` (new), `phase151d_parity_test.go`, `phase151a5_win_test.go`, `phase122_pipeline_ownership_test.go`, `.github/workflows/ci.yml` |
| `64c63c6` | B | 4 files: `KARKAIN_MATURITY_SCORECARD.md`, `KARKAIN-GOVERNANCE-COMPLETION-REPORT.md` (new), `ROADMAP.md`, `KARKAIN-VERSION-PLAN.md` |
| `024ca94` | C | 4 files: `KARKAIN-IMPLEMENTATION-CONTROL.md` (new), `AGENTS.md`, `NEXT_PHASE_RECOMMENDATION.md`, `KARKAIN_IMPLEMENTATION_MATRIX.md` |
| `6eff5a2` | D | 2 files: `UNIFIED-TYPE-DECLARATION-DESIGN.md` (new), `NAME-RESOLUTION-DESIGN.md` |
| `146bb37` | E | 2 files: `KARKAIN-VERSION-PLAN.md`, `v1.2.0-CHECKLIST.md` |
| this commit | review | `KARKAIN-MASTER-PLAN-COMPLETION-REPORT.md` (new), `KARKAIN-IMPLEMENTATION-CONTROL.md` (status correction) |

## 14. Remaining blockers, risks and deferred work

| Item | State | Unblocking evidence needed |
|------|-------|----------------------------|
| kcc `0xC0000005` | **blocked** | An `-fsanitize=address` or `-Wl,--stack,` build; three hypotheses already falsified |
| B2 seed-closure runner burn | **blocked** | A runner-stable run of the ~3-minute stage-2 transpile; discovery-only job today |
| Whole-tree KIR re-measurement on the dev host | **blocked** | The crash above; the pin stands at 14050, measured once |
| `stdlib_v2` and `std.net` without C | **deferred to v1.3.0** | Native function-call lowering and a socket emitter |
| Increment 154 | **baselined, not implemented** | Owner scheduling; its gate is `stage2 == stage3` with Go absent |
| Increment 165 | **not started** | A baseline note |
| D6 (8-byte frame delta) | **proposed, not selected** | Nothing - it is cheap but premature; the value model should settle first |
| Native driver value-model kinds | **approved, unblocked** | An authorised increment (`151A-9i+`) |
| Type declaration design | **awaiting owner sign-off** | Decisions D-1 to D-6 |

**Not done, by instruction:** no merge, no push, no tag, no release, and D6 was
not started. The five commits sit on `develop` unpushed and require an owner
decision to publish.