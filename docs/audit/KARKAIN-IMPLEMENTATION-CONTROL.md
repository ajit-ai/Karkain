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
`develop` = `64c63c6`.

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
| 151A-9i+ | Native driver value-model kinds still open: `for-in`, `push`, maps, float statements/params/returns, macOS entry/exit | `approved` (in the 151 baseline scope) | D5 | Refusal table is the current honest state; each refused shape is named in the driver's K145 diagnostics |
| 152-A / B0 / B1 / B2 | Native safety + capability boundary, runtime-error reporter, no-C string codecs | `completed` | 150 | `pkg/native/phase152a_safety_test.go`, `pkg/cli/phase152_noc_test.go`; CI steps |
| 152 (conditions 2-3) | `stdlib_v2` and `std.net` with **no C compiler** | `deferred` to **v1.3.0** | native function-call lowering | UNMET by owner-authorized amendment 2026-10-10; the native driver lowers no function call and `pkg/native` has no socket emitter. Recorded verbatim in `v1.2.0-CHECKLIST.md` section 2 |
| 154 | I-6 self-bootstrap: retire the `KARKAIN_ENGINE=go` stage-1 pin, `kcc` builds `kcc` | `approved` (baselined, not implemented) | 152 | `docs/audit/PHASE-154-BASELINE.md`; gate is `stage2 == stage3` with the Go toolchain absent. **No implementation exists** |
| 165 | P-0 Docker/GHCR image (free-floating) | `proposed` | - | No baseline, no code; listed in the version plan as free-floating CI/packaging work |
| LH-1 / LH-2 | kcc `?` propagation and `match` binding parity | `completed` | - | Checkpoint CLOSED; `pkg/cli/lh1_propagation_test.go` 11/11, `pkg/cli/lh2_match_binding_test.go` 8/8 |

### 4.2 Blocked

| ID | Item | Status | Blocker | Evidence |
|----|------|--------|---------|----------|
| KIR-REMEASURE | Re-measure the whole-tree KIR pin on the development host | `blocked` | The intermittent kcc `0xC0000005` makes a multi-minute whole-tree run unreliable | Pin stands at **14050**, verified once cleanly (`text=14050 verify=14050`, 908.3 s). Low-RAM, 64 MiB-stack and corrupt-binary hypotheses are falsified with evidence; the incident is UNCLASSIFIED |
| B2-SEED | Seed-closure burn on CI runners | `blocked` | Five consecutive runner deaths ~35 s into the stage-2 transpile | `bootstrap-closure` CI job is **discovery-only and not a required check**; the mechanism itself is proven on a dev host |

### 4.3 Later versions (planned, not baselined)

| ID | Item | Status | Notes |
|----|------|--------|-------|
| 155, 156, 158-161 | v1.3.0: OS stdlib modules, native function-call lowering, linear types, atomics | `proposed` | Theme and exit criteria only; no baseline notes exist yet |
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

**Recommended next: a root-cause investigation slice for the intermittent kcc
`0xC0000005` (`proposed` - requires owner authorisation).**

Why this one, from evidence:

1. It is the only blocker on a gate that is currently unmeasurable. The whole-tree
   KIR pin cannot be re-measured reliably on the development host, so the pin is
   carried as "measured once, at 14050" rather than re-verified.
2. It has a named, cheap, decisive experiment that has never been run: an
   `-fsanitize=address` build, or a `-Wl,--stack,` build. Three hypotheses are
   already falsified with evidence, which is why a fourth guess is not being
   made.
3. It may be the same class as the largest unproven release-readiness item
   (B2-SEED runner deaths). Two unexplained crashes of different shapes should
   not be investigated as if they were unrelated.
4. It is a prerequisite-shaped piece of work: it changes nothing user-visible,
   so it carries no compatibility risk, and its result either closes a red gate or
   redirects effort away from a red herring.

**Runner-up, and why it is second: the next native-driver value-model kind
(`151A-9i+`, `for-in` or `push`).** It is `approved` and unblocked, and it is
the largest remaining gap inside increment 151. It is second only because it
does not depend on the crash investigation, whereas the crash investigation
blocks measurement work that every future slice will rely on.

**Explicitly not recommended: D6.** The evidence says its 8-byte frame delta is
cosmetic relative to the open value-model kinds, and closing it requires moving
`natFrameLayout`, which every Step 1-9d displacement is pinned against. Doing it
now would reopen frozen gates for a cosmetic gain; doing it once after the value
model settles is cheaper.

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
| REL-002 | Seed-closure stage-2 transpile kills CI runners ~35 s in; mechanism proven on a dev host | S1 | `blocked` | Five consecutive runner deaths; the `bootstrap-closure` job is discovery-only, not a required check | `go test ./pkg/bootstrap -run TestBootstrap_SeedClosure -count=1` on a CI runner | v1.2.0 |
| REL-003 | No fuzz targets anywhere in the repository | S3 | `approved` | `func Fuzz` count = 0 | `Select-String -Path pkg/**/*_test.go -Pattern 'func Fuzz'` | v1.3.0 |
| REL-004 | No binary-size or artifact budget exists | S3 | `proposed` | Scorecard CE8 scored 2 for this reason | policy exists; numbers do not | v1.3.0 |
| REL-005 | Whole-tree KIR pin re-verification on the dev host | S2 | **reproduced twice, 2026-10-15** | Pin re-measured twice at commit `e83385f`: 908.3 s via `TestPhase151A5_KIRPinHolds` and 996 s via the CLI directly, both reporting `text=14050 verify=14050`. Two independent measurements agreeing is materially stronger than the single one previously on record | as REL-001 | closed pending a 3rd run |
| NAT-001 | `array_index` residual: kcc 1088 vs oracle 1096 bytes | S3 | `deferred` (not selected) | `sub rsp,0x288` vs `sub rsp,0x290` | `go test ./pkg/cli -run TestPhaseD2_KnownNonParityIsStillNonParity -count=1` | after the value model settles |
| NAT-002 | macOS Mach-O execution unverified | S2 | `blocked` | No macOS runner anywhere; every Mach-O claim since increment 149 is structural | none possible without a runner | v1.3.0+ |
| NAT-003 | kcc native driver covers a subset; `for-in`, `push`, maps, floats and macOS entry/exit are refused by name | S2 | `approved` (unblocked) | The refusal table is the honest current state | `go test ./pkg/cli -run 'TestPhaseD5_|TestPhase151A9' -count=1` | v1.2.0 |
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
| GOV-001 | Increment 154 is baselined and gated but NOT implemented | S1 (v1.2.0) | `approved`, **blocked by host environment 2026-10-15** | `PHASE-154-BASELINE.md`; the `forceGoEngine` pin still exists at `pkg/bootstrap/bootstrap.go:152`. The gate needs `stage2 == stage3` with Go absent, which **cannot run on the dev host today**: 874 MB free against the Phase-127 K127 guard's 1536 MB threshold, so stage-2 refuses with `error[K127]` before doing any work. Not started rather than started-unverifiable | needs >=1.5 GiB free RAM, or the CI runner (which REL-002 shows is currently unsafe for the minutes-long stage-2 burn) | v1.2.0 |
| GOV-002 | Increment 165 (Docker/GHCR) not started | S3 | `proposed` | Free-floating; no baseline | none | v1.5.0 |
| GOV-003 | No rollback runbook | S3 | `proposed` | OM4 scored 2 for want of one | none | v1.3.0 |
| DOC-001 | Several `docs/audit/*` documents contradict current code | S3 | `completed` | Three carry evidence-backed historical banners; scorecard section 7 lists them | `Select-String -Path docs/audit/NEXT_PHASE_RECOMMENDATION.md -Pattern HISTORICAL` | closed 2026-10-15 |

### 9.3 Prioritisation, and why

Ranked by correctness and safety impact first, then blocking dependencies,
then scope and regression risk, then release impact - **not** by file count,
apparent complexity, or preference for new features.

1. **REL-001** (S1, unclassified crash). It is the only blocker on REL-005,
   the highest-severity open item with no root cause, and its decisive
   experiment is cheap. An unknown-cause crash can hide anything behind it.
2. **GOV-001** (S1, 154 not implemented). A v1.2.0 closure condition that is
   already baselined, so no planning stands between it and a slice. Second only
   because it is large and interacts with REL-002.
3. **NAT-003** (S2, driver subset). The largest remaining gap inside increment
   151, unblocked, and mechanically incremental - the value-model kinds follow
   an established pattern.
4. **NAT-001 / D6** (S3). Cosmetic relative to the open kinds, and closing it
   early reopens frozen frame pins. Deliberately last among the 151 work.
5. **STD-001** (deferred to v1.3.0). Carried by an explicit owner amendment;
   picking it up inside v1.2.0 would contradict that amendment.

### 9.4 Single recommendation, runner-up, deferrals

**Recommended next slice: REL-001**, a root-cause investigation slice for the
kcc access violation. Status `proposed`; it requires owner authorisation because
its decisive experiment changes the build (`-fsanitize=address`, or
`-Wl,--stack,`).

* **Runner-up: NAT-003**, the next native-driver value-model kind. Unblocked and
  approvable today; second because it does not unblock measurement.
* **Deferred, with reasons:** NAT-001 (premature), STD-001 (owner amendment),
  NAT-002 (needs a runner, which is an owner decision), SEC-001 and PERF-001
  (real but not release-blocking), LANG-001 (owner decision on language
  semantics), GOV-003 (no incident has required it yet).

**This section is the register's authority for the recommendation.** If section
5 and section 9.4 ever disagree, this one wins, because it is the newer record
and carries the evidence.

### 9.5 v1.2.0 condition status, reconciled 2026-10-15

Against the checklist's six-condition amended gate, verified this date:

| # | Condition | Status | Evidence |
|---|-----------|--------|----------|
| 1 | hello with no C compiler | **met** | checklist §2.1; 152-A gate |
| 4 | native byte-identity where the gate requires parity | **met as written**, one documented limitation | 6 shapes asserted byte-identical by `TestPhaseD2_KccAndGoEmitIdenticalImages`; `array_index` asserted to DIFFER (1088 vs 1096) by `TestPhaseD2_KnownNonParityIsStillNonParity`. **Owner decision pending**: fund D6, or formally accept the 8-byte non-parity |
| 5 | `stage2 == stage3` | **open** | B2 / REL-002; also blocked by GOV-001 |
| 6 | no Go toolchain for self-bootstrap | **open** | GOV-001 unimplemented (`forceGoEngine` still pins `KARKAIN_ENGINE=go`) |
| 7 | `installation.rst` names Go reference-only | **met** | `docs/source/getting-started/installation.rst:50` |
| 8 | full QA battery | **open** | runs last, after 5 and 6 |

**REL-001 is not among these conditions.** It is a risk to condition 8's
*evidence* quality (a crash during the battery would invalidate that run), not a
blocker. See `REL-001-CRASH-INVESTIGATION.md` and checklist §2.2.
