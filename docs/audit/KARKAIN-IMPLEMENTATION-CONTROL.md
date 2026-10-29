# Karkain Language Implementation Control System

**Status: authoritative control entry point (established 2026-10-15).**

This file is the **single authoritative implementation queue** for Karkain
language and compiler implementation work. It answers one question at a glance:
*what is the current next implementation slice, and what proves it?*

It is deliberately small: one file, tables, no tooling. It is not a service, a
database or an automation framework.

## 1. Authority hierarchy (read before editing any status record)

| Question | Authoritative source |
|----------|----------------------|
| Which version ships what, and when | `docs/audit/KARKAIN-VERSION-PLAN.md` |
| Which version a deferral is attached to | `docs/release/<version>-CHECKLIST.md` |
| What the implementation queue is and its current status | **this file** |
| What work is complete, with its record | `AGENTS.md` |
| How mature the project is, dimension by dimension | `docs/audit/KARKAIN_MATURITY_SCORECARD.md` |
| What a specific slice was authorised to do | its baseline note, `docs/audit/PHASE-*-BASELINE.md` |

**Rule 1.** This file does not restate version scope. If it disagrees with the
version plan, the version plan wins and this file is corrected.

**Rule 2.** A status is a claim about a slice; the evidence column is the
claim's proof. A status without evidence is not a status.

**Rule 3.** Never edit a status to make prose elsewhere true. Correct the prose,
or record the contradiction (section 7).

## 2. Status vocabulary

| Status | Meaning | Requires |
|--------|---------|----------|
| `proposed` | Identified, not authorised | A named problem with its evidence |
| `approved` | Authorised, not started | A baseline note naming scope and gate |
| `in-progress` | Being implemented | An open change on a working branch |
| `blocked` | Cannot proceed as designed | A named blocker with its evidence |
| `verified` | Implemented and gated, awaiting nothing | A passing gate **and** its CI step |
| `deferred` | Not implemented, deliberately | An owning version and an owning increment |
| `completed` | Delivered, gated, CI-enforced, recorded | Gate, CI step, commit, and an `AGENTS.md` record |

**Rule 4 (design vs implementation).** A design document is never a status. A
proposal reaching `approved` means the *decision* was taken; it says nothing
about code. Implementation only reaches `verified`/`completed` through a gate.

**Rule 5 (evidence, not assertion).** No historical status is invented here.
Statuses below are derived from commits, source files, gate files and CI steps
that exist in the repository today. Where a status could not be derived from
evidence it is marked `unknown` rather than guessed.

## 3. Evidence conventions

An evidence cell must name something a reader can run or read:

- **Gate**: a test file plus the `go test -run` pattern that executes it.
- **CI**: the `ci.yml` step name that runs it on every push.
- **Commit**: the short hash that introduced or verified it.
- **Measurement**: a figure with the command that produced it (per the
  benchmark policy in `KARKAIN_MATURITY_SCORECARD.md` section 4).

**Rule 6 (gate = test file + CI entry).** A test file that no `ci.yml` step runs
is not a gate. This repository shipped four such files before the rule existed
(151P0, 151B, 151C, 151C2); any new gate must add its own step in the same
change.

**Rule 7 (CI wiring verification).** Before marking a slice `verified`, confirm
its step exists:
`Select-String -Path .github/workflows/ci.yml -Pattern '<pattern>'`. An empty
result means the gate is local-only and the status is wrong.

## 4. The queue

Ordered by version, then by dependency. Statuses derived 2026-10-15 at
`develop` = `64c63c6`; re-derived 2026-10-29 at `develop` = `fb7cbf6`
(the D8 corrective change, which is the revision this table was last read
against).

### 4.1 v1.2.0 "Sovereignty I"

| ID | Item | Status | Depends on | Evidence (gate / CI / commit) |
|----|------|--------|------------|------------------------------|
| 150 | Native value model (floats, arrays, arena, string ops, `push`, records, maps), register allocation, Mach-O PIE, native OS targets | `completed` | 149 | `pkg/native` executed goldens PE+ELF; `pkg/cli/phase150_native_targets_test.go`; Phase 148 gate; commits through `989a04c`; recorded in `AGENTS.md` |
| 151P0 | Bitwise operators (P0 soundness fix) | `completed` | - | `pkg/cli/phase151p0_bitwise_test.go` 4/4, value-asserted on both engines, mutation-verified |
| 151A | Native value/frame foundation in kcc (Steps 1-9h) | `verified` | 151B, 151C | Gates `pkg/cli/phase151a*_test.go` (value, int, loop, win, print, float x4, call x3, arrays, len, string, concat, records), each with a CI step |
| 151B | Machine-code encoder in kcc | `completed` | 151P0 | `pkg/cli/phase151b_encoder_test.go` 7/7 differential vs the real `pkg/native` Emitter |
| 151C / 151C2 / 151C3 | ELF, Mach-O, PE container writers in kcc | `completed` | 151B | `phase151c_container_test.go` 5/5, `phase151c2_macho_test.go` 6/6, `pkg/native/phase151c3_pe_test.go` 9/9 |
| 151D | kcc owns `native-x86_64-windows` (ownership flip) | `completed` | 151A, 151C3 | `pkg/cli/phase151_kcc_native_test.go`; `kccOwnsNativeTargets` is `true` |
| A1 | kcc owns `native-x86_64-linux` | `completed` | 151D | `pkg/cli/phase151d_elfnative_test.go` 6 tests / 16 subtests |
| B | kcc owns `native-x86_64-macos` (Mach-O PIE) | `completed` | A1 | `pkg/cli/phase151d_macho_test.go` 4 tests / 12 subtests |
| D1 | Emit in the oracle's order (entry placement) | `completed` | B | Measured constant 39-byte delta attributed to emission order, not frame padding |
| D2 / D3 / D4 | kcc/Go byte-identity: non-allocating, allocating, return_value + len_of_array | `completed` | D1 | `pkg/cli/phase151d_parity_test.go`; D2 CI step |
| **D5** | `array_index` narrowing: runtime-error reporter + map-helper predicate | **`completed`** | D4 | `pkg/cli/phase151d5_arrbounds_test.go` 3/3 incl. Windows PE execution; CI step *Run Phase D5 kcc native array-bounds gate*; commit `cc44c95` |
| **D6** | Remaining 8-byte frame delta (`array_index` kcc 1088 vs go 1096) | **`proposed`** | D5 | Measured: Go `sub rsp,0x290` vs kcc `sub rsp,0x288`; closing it moves `natFrameLayout`, which every Step 1-9d displacement is pinned against. **Not selected** - see section 5 |
| **D8** | Array-bounds diagnostic **length immediate** (kcc-only divergence) | **`completed`** | D5 | `natArrayIndex` sent 25 as the length of a 24-byte message; the oracle derives it with `len(...)` (`pkg/native/program.go:6324`). Gate `pkg/cli/phase151d8_diaglen_test.go` (`TestPhaseD8_`), CI step *Run Phase D8 native bounds-diagnostic length gate*, commit `fb7cbf6`; mutation-verified by restoring the literal. D5 could not see it because it counts image bytes and never decodes the immediate |
| 151A-9i+ | Native driver value-model kinds still open: `for-in`, `push`, maps, float statements/params/returns | `approved` (in the 151 baseline scope) | D5 | Refusal table is the current honest state; each refused shape is named in the driver's K145 diagnostics. **macOS entry/exit removed from this row 2026-10-15** - it is implemented (`natStartEntryOS(s, os)` with OS-parameterized `natSysWrite`/`natSysExit`), and no macOS refusal exists in the driver |
| 152-A / B0 / B1 / B2 | Native safety + capability boundary, runtime-error reporter, no-C string codecs | `completed` | 150 | `pkg/native/phase152a_safety_test.go`, `pkg/cli/phase152_noc_test.go`; CI steps |
| 152 (conditions 2-3) | `stdlib_v2` and `std.net` with **no C compiler** | `deferred` to **v1.3.0** | native function-call lowering | UNMET by owner-authorized amendment 2026-10-10; the native driver lowers no function call and `pkg/native` has no socket emitter. Recorded verbatim in `v1.2.0-CHECKLIST.md` section 2 |
| 154 | I-6 self-bootstrap: retire the `KARKAIN_ENGINE=go` stage-1 pin, `kcc` builds `kcc` | `implemented, unverified` — **carried to v1.3.0** (owner-authorized 2026-10-29) | 152, plus host capacity / runner survival (REL-002) | `docs/audit/PHASE-154-BASELINE.md` §8; impl `62a6323` (pin retirement, `compilerChildEnv`/`CompilerChildEnv`, gate file, CI step, `reference/`). Fast proof: `TestPhase154_PinMechanism` (0.09 s, mutation-verified M1/M2) in the **required** `Test` job. Three-stage proof: opt-in `KARKAIN_SEED_CLOSURE=1`, non-gating `phase154-closure` job that rejects SKIP/FAIL/zero-byte witness. Carried out of v1.2.0 → `v1.2.0-CHECKLIST.md` §2.3, work item `v1.3.0-CHECKLIST.md` §1 |
| 165 | P-0 Docker/GHCR image (free-floating) | `proposed` | - | No baseline, no code; listed in the version plan as free-floating CI/packaging work |
| LH-1 / LH-2 | kcc `?` propagation and `match` binding parity | `completed` | - | Checkpoint CLOSED; `pkg/cli/lh1_propagation_test.go` 11/11, `pkg/cli/lh2_match_binding_test.go` 8/8 |

### 4.2 Blocked

| ID | Item | Status | Blocker | Evidence |
|----|------|--------|---------|----------|
| KIR-REMEASURE | Re-measure the whole-tree KIR pin | `completed` 2026-10-29 **(on CI, not on this host)** | - | Pin **14050** is now asserted on every push by the *kcc whole-tree self-check (Phase 122)* job, which passed at `fb7cbf6` (run `40815666047`); two dev-host measurements at `e83385f` agree. The dev host itself remains unreliable for the multi-minute run (REL-001, UNCLASSIFIED), so the CI job is the measurement path and `TestPhase151A5_KIRPinHolds` still asserts the pin exactly when the host allows |
| B2-SEED | Seed-closure burn on CI runners | `blocked` | Six consecutive runner deaths (latest 2026-10-29, run `40815666047`) ~35 s into the stage-2 transpile, with no witness line, timer, OOM or crash fingerprint. **The same class independently reproduced by the increment 154 step on the same run** (`exit 143`, 110 s, its first-ever execution) | `bootstrap-closure` and `phase154-closure` CI jobs are **discovery-only and not required checks**; the mechanism itself is proven on a dev host (143) |

### 4.3 Later versions (planned, not baselined)

| ID | Item | Status | Notes |
|----|------|--------|-------|
| 155, 156, 158-161 | v1.3.0: OS stdlib modules, native function-call lowering, linear types, atomics | `proposed` | Theme and exit criteria only; **no increment is authorized**. Scope, ten closure conditions and the ordered queue (including the four carried v1.2.0 items) are frozen on paper in `docs/release/v1.3.0-CHECKLIST.md`, status **PREPARED, not open**, until v1.2.0 is tagged |
| 153 | DAP debugger server | `proposed` | Explicitly **not authorised**; no DAP server exists in the repository |
| TYPE-DESIGN | Unified type and variable declaration design | `proposed` (design milestone, **awaiting owner sign-off**) | `docs/audit/UNIFIED-TYPE-DECLARATION-DESIGN.md`; decisions D-1..D-6 are open, so the document is a proposal and not an approved design. Implementation requires a separate authorised slice |

## 5. Next-slice selection policy

**Rule 8 (exactly one).** At most one slice is `in-progress` at a time, and
exactly one slice is *recommended next*. The recommendation is recorded below
with its reasoning, so a reader never has to infer it from prose.

**Rule 9 (evidence order).** A slice is recommended over another only when the
recommendation is supported by something in this repository: a measured defect,
a failing gate, an unmet closure condition, or an explicit owner amendment. A
slice is never recommended merely because it is next in a list.

**Rule 10 (authorisation).** Recommending a slice is not authorising it. A
recommendation with status `proposed` requires an owner decision before code is
written, and any build-flag, dependency or pipeline change needs its own
authorised increment.

**Rule 11 (do not auto-promote).** The next slice is **not** automatically the
lowest-numbered open item. In particular D6 is explicitly not promoted below.

### 5.1 Current recommendation

**Recommended next: close the carried v1.2.0 debt - lower user function calls in
the kcc native driver, then prove `stdlib_v2` and `std.net` with no C compiler
(`proposed` - requires owner authorisation as the v1.3.0 opener).**

Why this one, from evidence:

1. It is the largest *measured* unmet obligation in the repository. v1.2.0
   conditions 2 and 3 were carried to v1.3.0 by owner amendment on 2026-10-10,
   not withdrawn, and the reason they were carried is one named missing
   capability: the driver refuses every user call, by name, at
   `src/compiler/native_value.kark:4238` (the message begins "call to" and ends
   "is not lowered by the"). No amount of library work
   substitutes for that - a `stdlib_v2` program is a program with calls in it.
2. It converts a deferred condition into a measurable one. Today condition 2 is
   an argument about what a missing lowering would allow; after the call path
   exists it is a test: build `examples/stdlib_v2` through the C-free native
   target and compare its output, then do the same for `std::net` on the
   platforms the version claims.
3. It is incremental on a pattern that is already gated end to end. Phases
   151A-8d/8e/8f lowered the call ABI for int, float and string arguments
   (spill staging, register budget, extras area, two-unit high-half rule) and
   each was byte-verified against the real `pkg/native` emitter. What is missing
   is the driver *reaching* that machinery for a program's own functions, plus
   the kinds the driver still refuses (`for-in`, `push`, maps, float statements
   and params).
4. **REL-001 no longer has a cheap decisive experiment, so it is not the pick.**
   ASan is unavailable on this toolchain (measured), the 64 MiB stack reserve is
   falsified (measured), the low-RAM and corrupt-binary hypotheses are
   falsified, and five consecutive isolated runs completed cleanly. A sixth
   guess is not evidence, and REL-001 does not block this work: the pin is now
   verified on CI (REL-005, closed below).

**Runner-up, and why it is second: the type-system foundation
(`TYPE-DESIGN` -> `LANG-001/002/003`).** It is where the language programme
actually starts - annotations are strings, there are no type values, `let`
carries no mutability meaning - and it is the collector for most v1.3.0+
correctness work. It is second rather than first for two reasons: it is a
*design* milestone with D-1..D-6 still open and awaiting owner sign-off, and
Rule 4 means a design is not implementation; and its largest decision (the
`let` immutability flip) is explicitly a major-version boundary, so starting it
now would stall behind a decision rather than produce a gate.

**Explicitly not recommended, with reasons:**

* **D6.** Its 8-byte frame delta is cosmetic, and closing it moves
  `natFrameLayout`, which every Step 1-9d displacement is pinned against. Do it
  once, after the value model settles, not now.
* **GOV-001 / increment 154.** **Implemented, not merely baselined** (corrected
  2026-10-29: it shipped at `62a6323`). What remains is the end-to-end proof,
  which is blocked twice over: this host measures 427 MB free physical against
  the Phase 127 `error[K127]` threshold of 1536 MiB, and the CI runner kills the
  stage-2 transpile with exit 143 ~35-60 s in (REL-002 / B2). It is a capacity
  and runner-stability problem, not a work-ordering problem, and it is not
  startable work — see `PHASE-154-BASELINE.md` §8.
* **153 (DAP).** Not authorised, and no DAP server exists in the repository.
* **152-A/A1/B1/B2 and D5/D8 follow-ups.** Already delivered; finishing their
  slice list is not a next-slice recommendation.

## 6. Staleness and contradiction detection

**Rule 10 (stale record).** A status record is stale if any of the following is
true, and must be corrected rather than left to mislead:

- its named gate file no longer exists;
- its named CI step no longer matches any pattern in `ci.yml`;
- its recommendation is work that later commits completed;
- it asserts a capability status that a later phase superseded.

**Detecting a stale recommendation:**

```powershell
# does the named gate still exist?
Test-Path pkg/cli/<gate-file>.go
# is the gate actually wired in CI?
Select-String -Path .github/workflows/ci.yml -Pattern '<pattern>'
# has the recommended work since been committed?
git log --oneline --grep='<recommended phase>'
```

Two records were found stale by this rule and are marked historical:
`NEXT_PHASE_RECOMMENDATION.md` (recommends a parser-OOM fix completed long ago)
and `KARKAIN_IMPLEMENTATION_MATRIX.md` (describes no type checker and unparsed
generics, both superseded).

**Rule 11 (completed is not pending).** A completed phase is never presented as
pending work in any document. If a document lists completed work as available,
that is a defect in the document, not a reopening of the phase. Corrective work
against a frozen baseline is recorded against the baseline, per `AGENTS.md`.

**Rule 12 (no duplicate truth).** If two documents disagree about a status,
this file is corrected to match the authority in section 1, and the correction
is named in that document's next revision. Neither document is left to drift.

## 7. Deferred scope and version assignment

**Rule 13.** Deferral is recorded in three places, or it is not a deferral:

1. this queue, with status `deferred` and an owning version;
2. the owning version's checklist, in the closure conditions, **verbatim** - a
   reworded condition is a weakened condition;
3. the version plan's scope table, so the version's exit criteria reflect it.

The two v1.2.0 carry-overs (conditions 2 and 3) are the worked example: recorded
verbatim in `docs/release/v1.2.0-CHECKLIST.md` section 2, with status living only
in sections 2.1, 4 and 6, and the reason measured rather than preferred.

**Rule 14 (no silent deferral).** Deferring a condition requires an
owner-authorized amendment. An agent may propose one; it may not record one.

## 8. Change log for this control system

| Date | Change |
|------|--------|
| 2026-10-15 | Control system established. Queue, status vocabulary, evidence conventions, selection policy, staleness rules and deferral process added. Derived statuses from commits, gate files and `ci.yml` patterns at `develop` = `64c63c6` |
| 2026-10-15 | Cross-workstream consistency review: `TYPE-DESIGN` corrected from `approved` to `proposed`. The design document and the version plan both state the design is awaiting owner sign-off, so `approved` was a contradiction introduced by this control system and is corrected here under Rule 3 |
| 2026-10-29 | Queue re-derived at `develop` = `fb7cbf6` (the D8 corrective change). Added the **D8** row. Closed **REL-005** on a third, independent measurement from a different host and OS (CI job *kcc whole-tree self-check (Phase 122)*, run `40815666047`, pin exactly **14050**), which also turns **KIR-REMEASURE** from `blocked` into `completed` with CI as the measurement path. Re-recorded **GOV-001** as `blocked` by capacity (427 MB free physical against the 1536 MiB K127 threshold, measured this date) rather than by ordering. Recommendation rewritten: **STD-001** - the carried v1.2.0 conditions 2-3, entered through native user-call lowering - replaces REL-001, whose remaining hypotheses have no cheap decisive experiment left; the **type-system foundation** becomes the runner-up |
| 2026-10-29 | **Increment 154 carried out of v1.2.0 to v1.3.0** (owner-authorized). `v1.2.0-CHECKLIST.md` gains §2.3, which records conditions **5** (`stage2 == stage3`) and **6** (no Go toolchain) as UNMET and carried, reducing the effective v1.2.0 gate to conditions **1, 4, 7 and 8**; the 2026-10-10 §2.1 table and this document's §9.5 table are retained unedited and marked superseded-in-part. New `docs/release/v1.3.0-CHECKLIST.md` created as a **PREPARED, not-open** scope freeze (ten closure conditions, ordered queue, numbering rationale, explicit not-in-scope list) so the carried work has a written home. §4.1 row 154 corrected from `approved (baselined, not implemented) / No implementation exists` to `implemented, unverified` under Rule 3 — the same defect GOV-001 had, in a second location. `VERSION` stays `1.1.0`; no v1.3.0 increment is authorized before v1.2.0 is tagged |
| 2026-10-29 | **GOV-001 corrected from `not implemented` to `implemented, unverified`** under Rule 3. Increment 154 shipped at `62a6323` (pin retirement, `compilerChildEnv`/`CompilerChildEnv`, gate file, CI step, `reference/`, baseline). What is blocked is the **end-to-end proof**, and for two measured reasons, not one: the dev host is under the K127 threshold **and** CI run `40815666047` gave the gate its first-ever execution, which failed with `exit 143` (SIGTERM) 110 s in - the REL-002 / B2 runner class. The 154 gate is now split (fast `TestPhase154_PinMechanism` in the required job, mutation-verified M1/M2; three-stage half opt-in via `KARKAIN_SEED_CLOSURE=1` in a new non-gating `phase154-closure` job that rejects a SKIP, a FAIL and a zero-byte witness). Conditions 5-6 stay **open**. Record: `PHASE-154-BASELINE.md` §8 |

## 9. Issue register

Extended 2026-10-15. The queue in section 4 answers "what is next"; this
register answers "what is wrong, how bad, and what would prove it fixed".
Every entry carries a stable ID, a severity, a status from section 2, the
evidence that establishes it, and the validation command that settles it.

**Severity ladder:** `S1` blocks a release condition or a gate;
`S2` is a correctness or reliability defect with bounded impact;
`S3` is a capability or quality gap; `S4` is an improvement.

**ID prefixes:** `REL-` reliability, `NAT-` native backend, `STD-` standard
library and runtime, `SEC-` security and supply chain, `PERF-` performance,
`CONF-` conformance and testing, `DOC-` documentation, `GOV-` governance,
`LANG-` language design.

### 9.1 Status discipline

- `completed` requires a gate, a CI step, a commit and an `AGENTS.md` record.
- `verified locally` means a command ran on a named host; the host is named.
- `CI-enforced` means a `ci.yml` step runs it; the step name is given.
- An **unexecuted** check is reported as unexecuted, never as passing.
- A **structurally** validated item is never reported as execution-verified.

### 9.2 Register

| ID | Title | Sev | Status | Evidence | Validation command | Target |
|----|-------|-----|--------|----------|--------------------|--------|
| REL-001 | kcc access violation `0xC0000005` on multi-minute whole-tree runs; root cause UNCLASSIFIED | S1 | `blocked` (UNCLASSIFIED) | **Diagnostic investigation 2026-10-15 at `80a9455`: crash NOT reproduced in 4 isolated-build runs** (466/608/473 s at a 64 MiB stack reserve, 487 s control at 2 MiB; free RAM 596-814 MB), plus 2 earlier clean runs = 5 consecutive. ASan proven **unavailable** on this toolchain (`cannot find -lasan`, MSYS2 UCRT64 gcc 14.2.0); the `-Wl,--stack,67108864` diagnostic was verified in the PE header (2 MiB -> 64 MiB). Full evidence: `REL-001-CRASH-INVESTIGATION.md`. **No root cause claimed - a run that does not crash cannot discriminate between hypotheses.** Prior record: intermittent on byte-identical sources; low-RAM, 64 MiB-stack and corrupt-binary hypotheses falsified | `go run ./cmd/karkain kir --verify src/compiler/kir.kark` (3+ runs; record exit, output length, elapsed) | v1.2.0 |
| REL-002 | Seed-closure (and now increment 154) stage-2 transpile kills CI runners ~35 s in; mechanism proven on a dev host | S1 | `blocked` | Seven recorded runner deaths, all `exit 143`; five from the seed closure (143) plus 154's own step on run `40815666047` (110 s, first-ever execution) and the `bootstrap-closure` job on the same run. Both jobs are discovery-only, not required checks | `go test ./pkg/bootstrap -run TestBootstrap_SeedClosure -count=1` on a CI runner | v1.2.0 |
| REL-003 | No fuzz targets anywhere in the repository | S3 | `approved` | `func Fuzz` count = 0 | `Select-String -Path pkg/**/*_test.go -Pattern 'func Fuzz'` | v1.3.0 |
| REL-004 | No binary-size or artifact budget exists | S3 | `proposed` | Scorecard CE8 scored 2 for this reason | policy exists; numbers do not | v1.3.0 |
| REL-005 | Whole-tree KIR pin re-verification on the dev host | S2 | **`closed` 2026-10-29** | Third, independent measurement on a **different host and OS**: CI run `40815666047`, job *kcc whole-tree self-check (Phase 122)*, commit `fb7cbf6` - PASS, `TestPhase122_PipelineOwnership` asserting the pin at exactly **14050**. That joins the two 2026-10-15 dev-host runs at `e83385f` (908.3 s via `TestPhase151A5_KIRPinHolds`, 996 s via the CLI; both `text=14050 verify=14050`). The two `src/compiler` commits since that measurement were checked for KIR neutrality by probe rather than assumed: `f(25)` and `f(len("array index out of range"))` both render as one `(= s (call f ...))` line, so D5 and D8 could not have moved the count. The pin is now asserted on every push by a runner that is not this host | as REL-001, plus the CI job | `closed` |
| NAT-001 | `array_index` residual: kcc 1088 vs oracle 1096 bytes | S3 | `deferred` (not selected) | `sub rsp,0x288` vs `sub rsp,0x290` | `go test ./pkg/cli -run TestPhaseD2_KnownNonParityIsStillNonParity -count=1` | after the value model settles |
| NAT-002 | macOS Mach-O execution unverified | S2 | `blocked` | No macOS runner anywhere; every Mach-O claim since increment 149 is structural | none possible without a runner | v1.3.0+ |
| NAT-003 | kcc native driver covers a subset; `for-in`, `push`, maps and float statements are refused by name | S2 | `approved` (unblocked) | The refusal table is the honest current state. **Corrected 2026-10-15: macOS entry/exit is NOT part of this gap** - `kccNativeOwnedTargets` includes `NativeMacOSTarget`, the driver emits `natStartEntryOS(s, os)` with OS-parameterized `natSysWrite`/`natSysExit`, and no macOS refusal exists in `native_value.kark`. Phase B and D1 delivered it | `go test ./pkg/cli -run 'TestPhaseD5_|TestPhase151A9' -count=1` | v1.2.0 |
| STD-001 | `stdlib_v2` and `std.net` build with no C compiler | S1 (v1.2.0 closure) | `deferred` to v1.3.0 | Owner amendment 2026-10-10; recorded verbatim in checklist section 2 | `TestPhase152A_SocketCapabilityBoundaryRejected` | v1.3.0 |
| STD-002 | `MM-K-time` unassessable - no inventory row, no module | S3 | `proposed` | `KARKAIN-LANGUAGE-COMPLETENESS.md` | none | v1.3.0 |
| SEC-001 | No SBOM, no artifact signing, no CVE-handling SLA | S2 | `proposed` | OM5 scored 2; cosign is planned in 165, which is not started | none | v1.3.0 |
| SEC-002 | Remote registry and git fetch are explicit "not available" errors | S3 | `proposed` (by design) | `pkg/pm` refuses rather than faking | n/a | v1.3.0+ |
| PERF-001 | No performance baseline, so no regression threshold can be stated | S3 | `proposed` | One `Benchmark` function exists; the Phase 141 RSS table has no baseline commit | `go test ./pkg/native -bench BenchmarkNativeCompile -benchmem -count=3` | v1.3.0 |
| PERF-002 | `TestPhase107_CodegenSpawnJoin` OOM flake on the ~4 GB host | S3 | `needs evidence` | Passes in isolation at 3.8 s; fails in combined runs | `go test ./pkg/codegen -run TestPhase107_CodegenSpawnJoin -count=1` | investigate when convenient |
| CONF-001 | The corpus is golden-based: a wrong value both engines agree on would pass | S2 | `proposed` | 151P0 added value assertions for exactly this reason | 151P0 gate (green) | v1.3.0 |
| CONF-002 | Gates A and E not re-executed in the 2026-10-15 pass | S3 | `implemented, unverified` for this pass | Reported "not re-run" in scorecard section 5 | `go test ./pkg/cli -run 'TestPhase122' -count=1` in isolation | when the host allows |
| LANG-001 | `let` carries no mutability meaning; only `const` is immutable | S2 | `proposed` (design decision) | `parseVarDecl` handles all three keywords; Phase 112 leaves let/var reassignment untouched | `UNIFIED-TYPE-DECLARATION-DESIGN.md`, D-1..D-6 | owner decision |
| LANG-002 | The `let` immutability flip is breaking and breaks a frozen gated corpus | S2 | `blocked` on LANG-001 | The 151A Step 2b reference reassigns a `let` | as LANG-001 | major-version boundary |
| LANG-003 | Type annotations are strings; no type values, no inference | S2 | `proposed` | `VarDeclStmt.Type string`; AR5 scored 3 | `TYPE_SYSTEM_AUDIT.md`, AR5 | separate programme |
| GOV-001 | Increment 154 is **implemented** (`62a6323`); only its end-to-end proof is blocked | S1 (v1.2.0) | `implemented, unverified` — **proof blocked twice over** (dev host re-measured 2026-10-29: 427 MB free physical vs the 1536 MiB K127 threshold; CI runner killed with exit 143, `REL-002`/`B2`) | `PHASE-154-BASELINE.md` §8. In the tree since `62a6323`: the pin retirement (`SetGoEnginePin`/`RetireGoEnginePin`/`GoEnginePinRetired`), `compilerChildEnv`/`CompilerChildEnv`, the gate file, a CI step and `reference/`. The earlier "NOT implemented / the pin still exists / not started" wording described a state that never existed on `main` and is corrected under Rule 3. The fast half (`TestPhase154_PinMechanism`, 0.09 s, mutation-verified M1/M2) runs in the required `Test` job; the three-stage half is opt-in via `KARKAIN_SEED_CLOSURE=1` and carried by the non-gating `phase154-closure` job | needs >=1.5 GiB free RAM, or a CI runner that survives the minutes-long stage-2 burn (REL-002) | v1.2.0 conditions 5-6 **CARRIED to v1.3.0** (owner-authorized 2026-10-29) |
| GOV-002 | Increment 165 (Docker/GHCR) not started | S3 | `proposed` | Free-floating; no baseline | none | v1.5.0 |
| GOV-003 | No rollback runbook | S3 | `proposed` | OM4 scored 2 for want of one | none | v1.3.0 |
| DOC-001 | Several `docs/audit/*` documents contradict current code | S3 | `completed` | Three carry evidence-backed historical banners; scorecard section 7 lists them | `Select-String -Path docs/audit/NEXT_PHASE_RECOMMENDATION.md -Pattern HISTORICAL` | closed 2026-10-15 |

### 9.3 Prioritisation, and why

Ranked by correctness and safety impact first, then blocking dependencies,
then scope and regression risk, then release impact - **not** by file count,
apparent complexity, or preference for new features.

1. **STD-001** (S1, carried v1.2.0 closure). The largest measured unmet
   obligation, and it is blocked by exactly one named capability: the native
   driver refuses every user call (`native_value.kark:4238`), so neither
   `stdlib_v2` nor `std.net` can build without a C compiler. Picking it up also
   consumes NAT-003's remaining kinds, because a library program needs
   `for-in`, `push` and float statements before it can be proved end to end.
2. **TYPE-DESIGN / LANG-003** (S2, type-system foundation). Where the language
   programme actually starts, and the collector for most later correctness
   work. Second because it is a design milestone awaiting owner sign-off
   (D-1..D-6 open) and its biggest decision is a major-version boundary, so
   starting now would stall behind a decision rather than produce a gate.
3. **GOV-001** (S1, increment 154 implemented; end-to-end proof blocked). Still a
   real v1.2.0 condition, but **not schedulable on this host**: 427 MB free
   physical against the 1536 MiB guard, and the CI runner kills the burn with
   exit 143. Rank is not the binding constraint; capacity is.
4. **REL-001** (S1, unclassified crash). No remaining cheap decisive experiment:
   ASan unavailable (measured), 64 MiB stack falsified (measured), low-RAM and
   corrupt-binary falsified, five consecutive isolated runs clean. It no longer
   blocks the pin (REL-005 closed on CI), so the honest action is to keep the
   evidence and not spend a slice guessing.
5. **NAT-001 / D6** (S3). Cosmetic relative to the open kinds, and closing it
   early reopens frozen frame pins. Deliberately last among the 151 work.
6. **REL-002 / B2-SEED** (S1 for release readiness, unowned by any agent). Six
   consecutive runner deaths with no fingerprint. The discovery job exists and
   reports; only a runner-side change or an owner decision moves it.

### 9.4 Single recommendation, runner-up, deferrals

**Recommended next slice: STD-001**, the v1.3.0 opener - lower user function
calls in the kcc native driver together with the kinds a library program needs,
then prove `stdlib_v2` and `std.net` build and run **with no C compiler**.
Status `proposed`; it requires owner authorisation because it opens a numbered
increment rather than extending a completed slice, and it is the first time the
native path is asked to carry a real library program.

The measured entry point is one refusal, not a programme of work:
`src/compiler/native_value.kark:4238` refuses every user call by name. The
machinery underneath it already exists and is byte-gated (151A-8d/8e/8f:
argument staging, the six-register budget, the extras area, the two-unit
high-half rule). What is missing is the driver walking a program's own
functions, plus `for-in`, `push`, maps and float statements/params/returns,
which is NAT-003's list - so this slice closes NAT-003 as a side effect rather
than as a competing item.

* **Runner-up: TYPE-DESIGN -> LANG-003**, the type-system foundation
  (`LANG-001/002/003`). It is `proposed` and awaiting owner sign-off on D-1..D-6,
  and its `let`-immutability decision is a major-version boundary. It is the
  right *second* because it needs a decision before it needs a gate.
* **Deferred, with reasons:** NAT-001 / D6 (premature - reopens frozen frame
  pins for 8 bytes), NAT-002 (no macOS runner exists; every Mach-O claim stays
  structural), GOV-001 (implemented at `62a6323`; its end-to-end proof is blocked
  by host memory: 427 MB free vs the 1536 MiB guard - a capacity problem, not an
  ordering one), REL-002 / B2-SEED (runner
  behaviour; discovery job only, needs an owner decision), REL-001 (evidence
  kept, no cheap experiment left, and it no longer blocks the pin), SEC-001 and
  PERF-001 (real but not release-blocking), LANG-001 / LANG-002 (owner decision,
  major boundary), GOV-003 (no incident has required it yet).

**This section is the register's authority for the recommendation.** If section
5 and section 9.4 ever disagree, this one wins, because it is the newer record
and carries the evidence.

### 9.5 v1.2.0 condition status, reconciled 2026-10-15

> **Superseded in part on 2026-10-29.** Conditions **5** and **6** were carried
> over to v1.3.0 by owner authorization (`v1.2.0-CHECKLIST.md` §2.3), so the
> effective v1.2.0 gate is now conditions **1, 4, 7 and 8**. The table below is
> retained unedited as the 2026-10-15 reconciliation — its rows for 5 and 6 read
> `open`, which was correct then and is superseded, not wrong.

Against the checklist's six-condition amended gate, verified this date:

| # | Condition | Status | Evidence |
|---|-----------|--------|----------|
| 1 | hello with no C compiler | **met** | checklist §2.1; 152-A gate |
| 4 | native byte-identity where the gate requires parity | **met as written**, one documented limitation | 6 shapes asserted byte-identical by `TestPhaseD2_KccAndGoEmitIdenticalImages`; `array_index` asserted to DIFFER (1088 vs 1096) by `TestPhaseD2_KnownNonParityIsStillNonParity`. **Owner decision pending**: fund D6, or formally accept the 8-byte non-parity |
| 5 | `stage2 == stage3` | **open** | implemented (154, `62a6323`); the proof is B2 / REL-002 blocked — dev host under the K127 threshold, CI runner exit 143. Fast pin-contract half green (`TestPhase154_PinMechanism`) |
| 6 | no Go toolchain for self-bootstrap | **open** | implemented (pin retirement + `retireGoEnginePin` are in the tree); the end-to-end no-Go burn is the same blocked proof as condition 5. Tracked as GOV-001 `implemented, unverified` |
| 7 | `installation.rst` names Go reference-only | **met** | `docs/source/getting-started/installation.rst:50` |
| 8 | full QA battery | **open** | runs last, after 5 and 6 |

**REL-001 is not among these conditions.** It is a risk to condition 8's
*evidence* quality (a crash during the battery would invalidate that run), not a
blocker. See `REL-001-CRASH-INVESTIGATION.md` and checklist §2.2.
