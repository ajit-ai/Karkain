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
| TYPE-DESIGN | Unified type and variable declaration design | `approved` as a design milestone | See `docs/audit/UNIFIED-TYPE-DECLARATION-DESIGN.md`; implementation requires a separate authorised slice |

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
