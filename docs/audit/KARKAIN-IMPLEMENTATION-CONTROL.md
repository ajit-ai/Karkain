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
| 152 (conditions 2-3) | `stdlib_v2` and `std.net` with **no C compiler** | `deferred` to **v1.3.0** | native stdlib runtime | UNMET by owner-authorized amendment 2026-10-10; recorded verbatim in `v1.2.0-CHECKLIST.md` section 2. **Corrected 2026-11-10 under Rule 3:** this row's blocker read "the native driver lowers no function call", which stopped being true when NAT-CALL shipped (2026-10-29). The condition is still UNMET and the deferral still stands: the native path has **no stdlib runtime at all**, which is a larger gap than the missing call lowering. `pkg/native` still has no socket emitter (`TestNativeSocketBuiltinsRefused`) |
| **NAT-IF** | Conditional branch in the kcc native driver: `if`/`else` over integer comparisons | **`completed`** | NAT-CALL | `src/compiler/native_value.kark` (`natEmitIntCond`, `natNativeCondOp`, `natNativeScanCond`, `natNativeScanIf`, `natNativeScanBranch`, branch-descending `natNativePlanDecls`, recursive `natNativeLower`); gate `pkg/cli/phase151g_if_test.go` (`TestPhaseNATIf_`), CI step *Run NAT-IF kcc native conditional-branch gate*. 19 programs execute as kcc-produced PEs with exact stdout bytes; byte-identical to the Go oracle's Linux ELF except the one case marked `expectDiff` (the Phase-150C register-allocator boundary, measured at 4 locals identical / 5 locals differing). 4 mutations, each restored byte-identically. Record: `docs/audit/NAT-IF-BASELINE.md` |
| **NAT-QUAL** | v1.2.0 native-independence QUALIFICATION: the feature matrix, the platform table and process-level independence evidence | **`completed`** | NAT-IF | `docs/inventory/native-feature-matrix.json` (54 feature rows, 3 platform rows, 9 known limitations as of 2026-11-11; the counts grow with the surface, and the NAT-QUAL gate fails if the feature rows ever fall below 20); gate `pkg/cli/phase151h_qualification_test.go` (`TestPhaseNATQual_`), CI step *Run NAT-QUAL native independence qualification gate*. Layer 3 installs PATH shims for `gcc` and `go`: the native build's shim log is EMPTY and the LEGACY build's log is NOT (the positive control that makes the empty log evidence). `docs/inventory/compiler-dependencies.json` refreshed (phase 144 → 151): it had no `native` component at all. Record: `docs/audit/NAT-IF-QUALIFICATION.md` |
| 154 | I-6 self-bootstrap: retire the `KARKAIN_ENGINE=go` stage-1 pin, `kcc` builds `kcc` | `implemented, unverified` — **carried to v1.3.0** (owner-authorized 2026-10-29) | 152, plus host capacity / runner survival (REL-002) | `docs/audit/PHASE-154-BASELINE.md` §8; impl `62a6323` (pin retirement, `compilerChildEnv`/`CompilerChildEnv`, gate file, CI step, `reference/`). Fast proof: `TestPhase154_PinMechanism` (0.09 s, mutation-verified M1/M2) in the **required** `Test` job. Three-stage proof: opt-in `KARKAIN_SEED_CLOSURE=1`, non-gating `phase154-closure` job that rejects SKIP/FAIL/zero-byte witness. Carried out of v1.2.0 → `v1.2.0-CHECKLIST.md` §2.3, work item `v1.3.0-CHECKLIST.md` §1 |
| **NAT-LOOP** | Loop control flow in the kcc native driver: `while`, C-style `for`, `break`, `continue`, nested loops | **`completed`** | NAT-IF | `src/compiler/native_value.kark` + `src/compiler/native_emit.kark` (the loop-context stack is emitter slot 17, the oracle's `b.loops`); gate `pkg/cli/phase151i_loop_test.go` (`TestPhaseNATLoop_`), CI step *Run NAT-LOOP kcc native loop-control-flow gate*. 21 programs executed as kcc-produced PEs with exact stdout bytes; a DIFFERENTIAL AGAINST THE C BACKEND (the same source compiled the old way and run) proves the SEMANTICS; Linux ELF byte-identity against the Go oracle below the allocator threshold. The three arena-using constructs are refused inside a loop with the oracle's own messages, because the arena is sized at compile time on the assumption that each site runs at most once. Record: `docs/audit/NAT-LOOP-BASELINE.md` |
| **NAT-IDX** | Array indexing inside the kcc native driver: the index is evaluated as an EXPRESSION, and the runtime-error reporter's context travels through the shared expression walker | **`completed`** | NAT-LOOP | `src/compiler/native_value.kark` (the index arm in the shared walker; the reporter context bundled as one argument; the reporter's predicate made a real walk over branches, loops, conditions and nested expressions; the two index validators consolidated onto the shared one); gate `pkg/cli/phase151j_array_index_test.go` (`TestPhaseNATIdx_`), CI step *Run NAT-IDX kcc native array-indexing gate*. 16 programs executed as kcc-produced PEs with exact stdout bytes, including ORDINARY ARRAY ITERATION in a `while` and a C-style `for`; a behavioural differential against the C backend; 5 bounds cases (index == length, index > length, out of range in a loop, a negative index, an empty array) each a runtime error with a non-zero exit and no value printed. Record: `docs/audit/NAT-IDX-BASELINE.md` |
| **NAT-FRAME** | The native frame layout against the Go oracle's: the map-stage slot reserved program-wide, and the `inc r8` REX bit corrected | **`completed`** | NAT-IDX | `src/compiler/native_value.kark` (the program-wide map-usage flag threaded into every function's layout) and `src/compiler/native_emit.kark` (`natIncReg` now uses the centralized `natRex`); gate `pkg/cli/phase151k_frame_test.go` (`TestPhaseNATFrame_`), CI step *Run NAT-FRAME native frame-layout gate*. The recorded "rounding" premise was MEASURED and found wrong: the two rules were always identical, and the 8-byte delta was an unreserved map-stage slot plus an `inc r8` that decoded as `inc rax` (a silent map-count corruption). The frames now match the oracle exactly; TWO bytes remain, isolated. The frozen 9D frame gate's own rule was corrected with it (it rounded the locals REGION rather than the TOTAL, and had no map-stage term), and its second check was found VACUOUS and replaced with a real lower bound. A THIRD divergence of the same class was measured while sweeping the whole gated corpus: the STRING STAGE is program-wide in the oracle and per-function in the driver (640 against 3200 bytes, exactly the 2560-byte string stage); it was PINNED as a known divergence by `TestPhaseNATFrame_TheStringStageIsStillDivergent` and deferred to its own increment. **That divergence is CLOSED by NAT-STR-STAGE (2026-11-11): the string stage is now reserved program-wide over the oracle's five operations, the frames match, the field-assignment case joined `natFrameCases`, and `TestPhaseNATFrame_TheStringStageIsStillDivergent` became `TestPhaseNATFrame_TheStringStageIsReservedProgramWide`.** Record: `docs/audit/NAT-FRAME-BASELINE.md` |
| **NAT-STR-STAGE** | The native string staging area: its SCOPE (program-wide, not per function) and its COVERAGE (the oracle's five operations, not two) | **`completed`** | NAT-FRAME | `src/compiler/native_value.kark` (`natNativeProgramUsesStr` -- the program-wide flag, computed once and threaded into every function's layout, exactly as the map flag is; `natNativeStrOpIn` -- the five-operation predicate, whose concat and push arms REUSE the arena bound's detectors so there is one notion of a concat site; `natNativeStrEqSites`/`natNativeStrSliceSites`/`natNativeStrFieldSites` -- the three arms the driver does not lower yet, implemented so the reservation is complete when they are); the per-function computation and a DEAD program-level copy of it were deleted. Gate `pkg/cli/phase151l_strstage_test.go` (`TestPhaseNATStrStage_`), CI step *Run NAT-STR-STAGE native string-staging-area gate*. 11 programs: the frame-size immediates of EVERY function compared element for element against the ORACLE's on the LINUX target (the Windows 16-byte rounding cannot see a 2560-byte difference), the scope asserted as a difference of exactly 2560 in every frame, the five operations measured on both engines, EXECUTED PE goldens with exact stdout bytes, determinism, and a refusal table. Two mutations, each restored byte-identically. **KIR pin 14841 -> 14938** (+97), measured with `karkain kir --verify src/compiler/kir.kark` (text=14938 verify=14938) and never predicted; re-pinned in `pkg/cli/phase122_pipeline_ownership_test.go` and `pkg/cli/phase151a5_win_test.go`. **NOT claimed:** byte-identity (the reporter's rodata order and the oracle's field-write staging still differ), and the oracle's program-wide NAME table -- which ignores shadowing and so over-reserves for a program with no string operation -- is recorded rather than reproduced, because the oracle's own EMISSION disagrees with its own scan there. Record: `docs/audit/NAT-STR-STAGE-BASELINE.md` |
| **NAT-RODATA** | The reporter's read-only data: the interning ORDER and the interned OFFSETS | **`completed`** | NAT-STR-STAGE | `src/compiler/native_value.kark` (the interning list is built in the ORACLE'S order -- print_int's two, the reporter HELPER's four, the program's literals, the call site's two; every reporter offset is read back OUT of `natRodataSection`'s offset array; and the EMITTER receives the LITERALS' offsets alone, so its cursor no longer depends on the interning order). Gate `pkg/cli/phase151m_rodata_test.go` (`TestPhaseNATRodata_`), CI step *Run NAT-RODATA native reporter rodata gate*. **THE RECORDED DIAGNOSIS WAS WRONG and measurement corrected it:** the residual was described as an interning-ORDER difference, but the two engines' read-only sections were BYTE-IDENTICAL and the driver baked the LIST POSITION of the array-bounds call site's two strings where the interned OFFSET belongs -- so the out-of-range diagnostic pointed at the middle of "runtime error: " instead of at the message and the file name. That is a correctness defect, not a layout one, and the D5 baseline had already stated both halves of the intended design. Two existing gates moved by FAILING first, which is the signal each was built to give: `array_index` left the known-non-parity list (now EMPTY, and its gate was REMOVED rather than left with nothing to compare) and NAT-FRAME's two-byte residual became an EVERY-BYTE agreement. **NOT claimed:** byte-identity for a program that binds a literal AFTER an index site (39 bytes: the oracle interns the call site's strings at the first index site -- a LAYOUT difference with no correctness consequence), and byte-identity for the PE (the Windows entry stub and the PEB bootstrap are separate implementations in the two engines). Record: `docs/audit/NAT-RODATA-BASELINE.md` |
| **NAT-REG** | Native register allocation in the self-hosted driver: int locals promoted into callee-saved registers | **`completed`** | NAT-RODATA | `src/compiler/native_value.kark` (the allocation lives in the PLAN as a fifth field per slot; the liveness is a point-ordered walk over the driver's own AST with the loop widening the oracle's rule requires; the allocation is the oracle's linear scan; the claimed registers are pushed before the frame is taken and popped in reverse after it is released). The ORACLE already had this allocator (`pkg/native/regalloc.go`, the Phase-150C one), so the correction is a PORT and not a new design. Gate `pkg/cli/phase151n_regalloc_test.go` (`TestPhaseNATReg_`), CI step *Run NAT-REG native register-allocation gate*. **The switch is `KARKAIN_NATIVE_REGALLOC=1` and the DEFAULT is the stack-based path**, which is the qualified implementation this repository ships: the register path is new code in a self-hosted compiler, and the brief's rule is that it does not become the default until it has passed the complete qualification suite. The evidence is BEHAVIOURAL: the two paths are built over the same programs and must produce the same stdout and exit status, with the Go oracle as a third opinion; the frame must be identical between them; and they must be byte-different for the high-pressure cases or the agreement would be vacuous. Mutations: reusing a live holder's register, dropping the loop widening, omitting the epilogue's POP and storing a spilled value to the wrong slot, each restored byte-identically. **A fifth mutation was NOT caught and is recorded rather than hidden**: dropping the scan's liveness check makes the allocator more conservative, which is a missed optimisation and not a wrong answer. **Not claimed:** a performance improvement -- the allocator trades memory traffic for a push/pop pair per claimed register, and the measured sizes are recorded rather than claimed. Design: `docs/audit/NAT-REG-DESIGN.md`; record: `docs/audit/NAT-REG-BASELINE.md` || 165 | P-0 Docker/GHCR image (free-floating) | `proposed` | - | No baseline, no code; listed in the version plan as free-floating CI/packaging work || LH-1 / LH-2 | kcc `?` propagation and `match` binding parity | `completed` | - | Checkpoint CLOSED; `pkg/cli/lh1_propagation_test.go` 11/11, `pkg/cli/lh2_match_binding_test.go` 8/8 |

### 4.2 Blocked

| ID | Item | Status | Blocker | Evidence |
|----|------|--------|---------|----------|
| KIR-REMEASURE | Re-measure the whole-tree KIR pin | `completed` **(re-measured 2026-11-10 on the dev host)** | - | The pin moved **14385 → 14477 → 14597**: 14385 (NAT-CALL), 14477 (the `push()` slice, which moved it and did **not** re-pin it, leaving `main` and `develop` with a red gate), 14597 (NAT-IF). Measured with `karkain kir --verify src/compiler/kir.kark` in the gate's own invocation (`text=14597 verify=14597`, 1724 s on this host) and never predicted. Both assertions re-pinned: `pkg/cli/phase122_pipeline_ownership_test.go` and `pkg/cli/phase151a5_win_test.go`. **The dev host DID complete the measurement this time** — the earlier failure was not the intermittent crash but a missing `libgmp-10.dll` on `PATH` (`C:\msys64\ucrt64\bin`), which made `kcc.exe` die with `0xC0000135` and the CLI report it as exit 3 with no output; see `docs/audit/NAT-IF-BASELINE.md` §8.3 |
| B2-SEED | Seed-closure burn on CI runners | `blocked` | Six consecutive runner deaths (latest 2026-10-29, run `40815666047`) ~35 s into the stage-2 transpile, with no witness line, timer, OOM or crash fingerprint. **The same class independently reproduced by the increment 154 step on the same run** (`exit 143`, 110 s, its first-ever execution) | `bootstrap-closure` and `phase154-closure` CI jobs are **discovery-only and not required checks**; the mechanism itself is proven on a dev host (143) |

### 4.4 v1.2.x gap-closing slices (owner-authorized 2026-11-25, amended 2026-11-26)

Owner decision: v1.3.0 is the final release; the post-2.0 deferrals below are
pulled into the v1.2.x line so v1.3.0 GA ships complete. On 2026-11-26 the
owner added three system-level closure increments after v1.2.33: portable
OS/process APIs, optimization/build profiles and the bootstrap closure proof.
They are proposed, not started; no existing number was renumbered.

| ID | Item | Status | Evidence / gate |
|----|------|--------|-----------------|
| v1.2.16 | QA battery + release ceremony closeout (verification/docs/harness only) | `completed` (PR #37 merged) | `docs/audit/V1.2.16-QA-BATTERY.md`; `scripts/qa/run-full-qa.ps1` PATH prelude + split `cli114`–`cli119`; no compiler source, no byte-identity/KIR claim |
| v1.2.31 | async/await in both engines (pulled from post-2.0) | `proposed` | Design decision (stackless vs thread-per-task) + implementation + litmus gates; SPEC.md 4.5 already reserved |
| v1.2.32 | REL-001 sanitizer investigation (libasan/clang ASan build) | `proposed` | `REL-001-CRASH-INVESTIGATION.md` names the decisive experiment; needs authorized build increment |
| v1.2.33 | Security/capability model (`unsafe {}`, SBOM, CVE SLA) | `proposed` | Closes SEC-001; increment 165 (Docker/cosign) + SBOM + CVE docs |
| v1.2.34 | Portable OS/process API completion (process control, signals, portable env/time, dynamic-library boundary, filesystem metadata) | `proposed` (owner-added 2026-11-26) | Must close the system-programming gap left by v1.2.19: real process control and portable OS APIs on both engines, native/WASM decisions named |
| v1.2.35 | Optimization/build profiles (`dev`/`release`/`size`, pass selection, benchmarks, profile-gated determinism) | `proposed` (owner-added 2026-11-26) | Closes `MM-J10` and the unowned optimization-profile gap; correctness corpus green under every profile |
| v1.2.36 | Bootstrap closure proof (increment 154 full proof inside v1.2.x) | `proposed` (owner-added 2026-11-26) | `stage2 == stage3` with non-zero byte count, Go pin retired in child env, runner-stable; until closed, condition 9 remains carried to v1.3.0 as UNMET |

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

> **Superseded-in-part banner (2026-11-27).** The dated paragraphs below are
> historical recommendations retained for audit continuity; they are not the
> current queue. The last merged numbered increment is **v1.2.31** (PR #61)
> and the numbered v1.2.x line ends at **v1.2.36** (`proposed`) in §4.4.
> The unnumbered corrective slice *kcc C-style `for` scope leak* is COMPLETE
> in this change (record `docs/audit/FOR-SCOPE-CORRECTIVE.md`). The
> owner-approved order from here is: Step M (read-only measurement baseline)
> — **COMPLETE** (`docs/audit/STEP-M-COMPILE-RUN-MEASUREMENT.md`), then the
> named slice A1 (kcc owns native build/run) — **COMPLETE**
> (`docs/audit/A1-NATIVE-BUILD-RUN-BASELINE.md`), then v1.2.34 — takeover of
> the other developer's WIP, **COMPLETE for the builtin/stdlib slice**
> (`docs/audit/V1.2.34-TAKEOVER-BASELINE.md`), v1.2.35 — slice 1
> (`--profile dev|release|size`) **COMPLETE**
> (`docs/audit/V1.2.35-BUILD-PROFILES-BASELINE.md`; native-as-default and SSA
> pass selection deferred to their own slices) — and v1.2.36 (attempted, UNMET
> on this host). Each still needs its own owner authorisation. v1.2.36 was
> attempted on this host (2026-11-27): the fast pin contract PASSed, stage 1
> built, but the K127 guard refused stage 2 at 890 MiB against 1536 MiB, so
> conditions 5/6 remain UNMET (`docs/audit/V1.2.36-BOOTSTRAP-PROOF-BASELINE.md`).
> A capable host (~1.5 GiB free) is required. Step M measured
> the warm default C23 path at ~1.98 s median versus ~0.19 s for native,
> so A1 and v1.2.35 target the actual cost; no compiler rewrite is indicated.
> **v1.2.32**
> (REL-001 sanitizer) and **v1.2.33** (security/capability) are not authorised
> by this record, and no v1.3.0 work is opened.

**Owner numbering decision (2026-11-21):** all active forward work is named
**v1.2.x** (`x = 1, 2, 3, … n`). Older non-v1.2.x labels in this register are
historical audit records, not the active plan.

**Current slice: v1.2.16 — QA battery + release ceremony closeout** —
**COMPLETE in the working tree; commit/PR pending** (2026-11-25; record
`docs/audit/V1.2.16-QA-BATTERY.md`). This is verification, documentation and
QA-harness work only: no language, compiler, backend or KIR source moved.
The original battery stopped at `units` (`build` 33.1 s and `vet` 17.6 s PASS;
`units` FAIL from `git` missing on `PATH` and the 692/699 MiB K127 guard);
the full Phase-114 family ran 1553.108 s exit 0 in isolation, and
`conformance`, `probes`, `examples`, `install`, `journey` and `fresh` were
measured PASS separately while `docs` SKIPped without Sphinx. The old
`phase115`–`phase119 exit=0` rows were `[no tests to run]` and are retired;
the corrected MinGW rerun FAILED at `TestPhase115_FreshCheckout` and the
isolated rerun ended in SIGTERM. The harness now adds Git, WindowsPowerShell
and MinGW to `PATH` when present, replaces the monolithic `cligates` stage
with `cli114`–`cli119`, and fixes the `-Stage build`/`-Stage vet` no-op.
**No full-QA-green, byte-identity or KIR-pin claim is made.**

**Current slice: v1.2.22 — real SQLite driver — COMPLETE for the C23 path** (PR #42 merged at `1f0b8b9`; record `docs/audit/V1.2.22-SQLITE-DRIVER-BASELINE.md`). `std.db` now provides vendored SQLite 3.46.1 open/exec/query/close/error on both engines; native/WASM remain excluded. **Recommended next implementation: v1.2.23 — memory safety Phase 1**, the next ordered blocker. It is **not started** and requires owner authorisation. **Historical, not guidance:** earlier recommendations here named
v1.2.3 (`s[i]`), v1.2.6 (`sha256_hex`) and P0.2 (task cancellation); all were
implemented and superseded by the measured v1.2.9–v1.2.15 merge history
(PRs #30–#36).

**Historical rationale for the superseded v1.2.6 recommendation** (retained
because the evidence was correct for that slice; it is not the reason for the
current v1.2.17 recommendation):

1. It is the next *measured* unlowered `std` primitive in a module whose
   semantics are already gated. `std.crypto` is one of the 13 VERIFIED modules
   in `KARKAIN-STDLIB-CAPABILITY-MATRIX.rst`, and Phase 109 pins NIST FIPS 180
   vectors (`sha256_hex("abc") = ba7816bf...`) on both engines — so the native
   slice has an exact-value oracle before a line of emitter code is written,
   which is the property that made v1.2.5a–c reviewable.
2. The pattern is proven and bounded. v1.2.5a/b/c established the shape for a
   codec primitive: a helper emitted only when used, five-unit argument staging,
   an arena result, runtime-error paths for invalid input, PE execution with
   exact bytes and a C-backend differential. A hash is the same shape with a
   longer body; it is not a new mechanism.
3. The instruction set already exists. A compression function needs 32-bit
   rotates, shifts, `and`/`or`/`xor` and byte loads; bitwise operators are fixed
   and gated on both engines by `TestPhase151P0_`, and the scaled/byte load
   forms are gated by the 9d/9g work. Nothing in the slice requires a new class
   of emitter primitive.
4. **REL-001 still has no cheap decisive experiment, so it is not the pick.**
   ASan is unavailable on this toolchain (measured), the 64 MiB stack reserve is
   falsified (measured), the low-RAM and corrupt-binary hypotheses are
   falsified, and five consecutive isolated runs completed cleanly. A sixth
   guess is not evidence, and REL-001 does not block this work: the pin is
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
| 2026-11-23 | Workstream evidence (repo audit at `develop`=`59d51f4`=main=`4b71060`; no roadmap/branch-name inference): **NAT-CALL is merged and gated on main.** Gate file `pkg/cli/phase151e_call_test.go` on origin/main, CI step `TestPhaseNATCall_` on origin/main (ci.yml:414-420), record `NAT-CALL-BASELINE.md` §8.2 (gate green; 20 shapes executed as PEs with exact bytes; byte-identical to the Go oracle's Linux ELF). The §4.1 row for NAT-CALL is corrected: status `completed` for the landed slice; an unmerged `feature/v1.3.0-natcall` branch [head `aa45970`] carries a separate qualification addendum (`phase151o_natcall_qual_test.go`) on top of the frozen committed gate. |
| 2026-11-23 | **v1.2.0 TAGGED** (`v1.2.0` ref on origin). The "no increments authorized until v1.2.0 is tagged" precondition for v1.3.0 is now satisfied. `docs/release/v1.3.0-CHECKLIST.md` reconciled: header note (§0) corrected for the new `VERSION` fact; §2.1 status table filled from evidence; §6 NAT-CALL sign-off row flipped to ✅ |
| 2026-11-23 | **NAT-STDLIB evidence** (repo audit): implemented, but UNMERGED. `feature/v1.3.0-natstdlib` [head `199ee87`] and `feature/v1.3.0-natstdlib-string` [head `f288d09`] (branch-gate `pkg/cli/phase151p_natstdlib_test.go` NOT on main; no main CI step; branches diverged from develop at `b5dcd7f` 2026-11-14, so they do not contain later develop work) → open; not completed while unmerged |
| 2026-11-23 | **NAT-NET evidence** (repo audit): not started - no implementation, no gate file, no branch, no CI step, no platform-support decision found (STD-002: `MM-K-time` unassessable). → open |
| 2026-11-23 | **TYPE-DESIGN re-corrected** (Rule 3): slice `bbecf1c` - 'feat(types): type-first declarations across .kark sources and both engines' - shipped on main/develop. CI step `TestTypeFirst` on origin/main (ci.yml:91), gate `pkg/sema/typefirst_test.go`. Status stays `proposed` (design decisions D-1..D-6 await owner sign-off; the `let` immutability flip, D-3, is a major-version boundary) |
| 2026-11-23 | **Recommended v1.3.0 resume (added here as a plan, not an increment):** NAT-STDLIB (merge or re-gate `feature/v1.3.0-natstdlib*`; close the `import std.*` half of Condition 2) → NAT-NET (open to Condition 3) → TYPE-FOUNDATION (owner sign-off of D-1..D-6, then implementation) → **156** verdicts (recorded evidence, no default verdicts) → **158** (`secure_eq`/HMAC/streaming digests with vectors, `std.crypto` extension surface) → **159** (`Deque[T]`/`Set[T]` goldens, const folding) → **160** (linear-type negative gates) → **161** (atomics + litmus goldens) → **154** proof (capacity/runner-blocked, REL-002). Every v1.3.0 queue item keeps its own gate-file+CI-step requirement per the AGENTS.md gate rule |
| 2026-11-25 | **v1.2.16 closeout (verification/docs/harness only).** Added the exact QA-battery record `docs/audit/V1.2.16-QA-BATTERY.md`: original battery `build`/`vet` PASS then `units` FAIL (git missing from `PATH`; 692/699 MiB K127 guard); full Phase-114 family 1553.108 s exit 0; separately measured `conformance`/`probes`/`examples`/`install`/`journey`/`fresh` PASS and `docs` SKIP; monolithic `cligates` shell-SIGTERM and retired `[no tests to run]` `phase115`–`phase119` rows; corrected MinGW rerun FAILED at `TestPhase115_FreshCheckout` and isolated rerun SIGTERM. `scripts/qa/run-full-qa.ps1` now adds Git/WindowsPowerShell/MinGW to `PATH`, replaces `cligates` with anchored `cli114`–`cli119` stages, rejects `-Stage cligates`, and fixes the `-Stage build`/`-Stage vet` no-op. No byte-identity or KIR-pin claim; no compiler source moved. §5.1 recommendation updated from the long-superseded v1.2.6/P0.2 text to **v1.2.17 — TLS/HTTPS**, gated on the owner merging/pulling the v1.2.16 PR; §9.4 demoted to historical and points at §5.1. `v1.3.0-CHECKLIST.md` §2.2 v1.2.6–v1.2.15 rows reconciled to their merged PR evidence and v1.2.16 recorded without a full-green claim. **PR #37 CI follow-up:** `native-macos` now runs only on the canonical push to `main`; PR/`develop` runs SKIP instead of sitting Queued, while the gate's file, runner and assertions are unchanged and structural Mach-O gates stay in the required `Test` job. |
| 2026-11-25 | **v1.2.17 C23 TLS/HTTPS COMPLETE** (owner chose the platform-adapter scope after the dependency tradeoff). PR #37 was MERGED at `099dc42` and `develop`/`main` synced. Added the real C23 transport surface: `https_request` (6 args) + `tls_last_error` on both engines, canonical `std.http` `https_get`/`https_post`/`https_last_error`, WinHTTP/Schannel on Windows and OpenSSL on POSIX, `verify=1` chain/hostname validation and `verify=0` loopback mode. The runtime has ONE source (`runtime/tls/c/*`), embedded by Go and generated into `src/compiler/tls_runtime.kark` by `scripts/gen-tls-runtime.ps1`; `TestTLSKarkFresh` pins the mapping. `needsTLS` gating keeps TLS-free programs (including the bootstrap compiler) on their historical link flags. Gate `pkg/cli/phase_v1217_tls_test.go` (`TestPhaseV1217_`) drove a real in-process `crypto/tls` server: loopback body and verify=1 rejection PASS on both engines, runtime wiring PASS, 17.296 s total; CI steps *Run TLS runtime freshness gate* and *Run v1.2.17 C23 TLS/HTTPS gate* wired. Native/WASM remain not supported (v1.2.10); no byte-identity or KIR claim. Audit: `docs/audit/V1.2.17-TLS-HTTPS-BASELINE.md`. |
| 2026-11-26 | **v1.2.17 first-CI corrective change against the frozen baseline** (CI run `45656354614`, no new increment). Three measured defects: (1) whole-tree KIR pin stale — CI measured `text=18187 verify=18187` vs `17924`; both assertion sites re-pinned from that measurement, never predicted. (2) Sphinx `-W` warning `feature-matrix.rst:249: Explicit markup ends without a blank line` — blank line added. (3) Phase-114 corpus failed on imported `std.http`: the Go split path did not carry the whole-program `needsTLS` flag into the shared runtime header or link libraries, so module TUs called `karkain_https_request` with no prototype. Fixed by `SplitResult.NeedsTLS`, appending `tlsRuntimeC()` to the shared runtime text and the platform TLS libraries to the split link line. Post-fix: Phase-114 GoEngine/coverage PASS 581.688 s, KCCParity PASS 694.995 s, build/vet/freshness gates PASS. Record: `V1.2.17-TLS-HTTPS-BASELINE.md` §6. |
| 2026-11-26 | **PR #38 MERGED** at `origin/main` `da2af00`; `main`/`develop` re-synced. The strict v1.2.x sequence gate for **v1.2.18 — concurrent HTTP server** is satisfied; inventory is being taken before any code is written. No tag or release action taken. |
| 2026-11-26 | **v1.2.18 concurrent HTTP server COMPLETE for C23.** No new builtin or runtime: `stdlib/http/http.kark` gained named `http_echo_worker` and `http_serve_echo`, whose accept loop spawns the worker per connection. Gate `pkg/cli/phase_v1218_http_server_test.go` (`TestPhaseV1218_`) PASS 26.487s on both engines; the first client connects but stays silent, and the second must receive `hello /second` within 3 s, which a serial server cannot do. `TestPhaseV1218_RuntimeWiring` PASS. Two harness defects found and fixed: `io.Discard` pipes made `Wait()` hang after killing the compiler (now `nil` + `WaitDelay`), and the wiring negative case had to be a no-import program because importing `std.http` legitimately pulls the spawn runtime. Dedicated CI step wired. Native/WASM excluded; no byte-identity claim; KIR pin CI-measured only. Audit: `docs/audit/V1.2.18-CONCURRENT-HTTP-SERVER.md`. |
| 2026-11-26 | **PR #39 MERGED** at `origin/main` `937feb0`; `main`/`develop` re-synced. The strict v1.2.x sequence gate for **v1.2.19 — OS metric/process enumeration APIs** is satisfied; inventory is being taken before code. No tag or release action taken. |
| 2026-11-26 | **v1.2.19 OS metrics / process enumeration COMPLETE for C23.** New single-source runtime `runtime/osmetrics/c/*` with seven `os_*` builtins (PID, CPU count, total/available memory, monotonic uptime, sorted self-containing process list, last error), embedded by Go and generated into `src/compiler/osmetrics_runtime.kark` for kcc; freshness gate wired. Canonical `stdlib/os/os.kark` wrappers fill the previously Planned `std.os` slot. `needsOS` gating covers monolith and split flows. Gate `TestPhaseV1219_` PASS 28.927s on both engines with predicate assertions, runtime wiring and native K145 refusal. Three defects found and fixed: module file had to be `os.kark`; `strings.Fields` dropped the empty `os_error()` line; the 63-entry cap also gated the self-add and evicted the current PID. Two CI steps wired. Native/WASM excluded; no byte-identity claim; KIR pin CI-measured only. Audit: `docs/audit/V1.2.19-OS-METRICS-BASELINE.md`. |
| 2026-11-26 | **v1.2.19 first-CI corrective change against the frozen baseline** (CI run `45736831726`, no new increment). Three measured defects: (1) Sphinx `-W` found the `std.os` stable-api row one indent too deep plus two short RST title underlines — corrected structurally; (2) whole-tree KIR pin stale — CI measured `text=18517 verify=18517` and both assertion sites were re-pinned from that measurement only; (3) the Linux gate mixed gcc warnings into program stdout — stdout/stderr are now separate buffers and the parser locates the first seven consecutive integer lines. Post-fix: `TestPhaseV1219_` PASS 22.153 s locally, freshness gates, build and vet PASS. Record: `V1.2.19-OS-METRICS-BASELINE.md` §6. |
| 2026-11-26 | **Owner-approved plan amendment: three additional v1.2.x system-level increments.** PR #40 (v1.2.19) merged at `origin/main` `2e839a0`; `main`/`develop` re-synced. Added **v1.2.34** portable OS/process API completion, **v1.2.35** optimization/build profiles and **v1.2.36** bootstrap closure proof to §4.4 and `v1.3.0-CHECKLIST.md` §2.2. All three are `proposed`, not started; no existing number was renumbered and v1.2.20 remains the next scheduled increment. The v1.2.x line now ends at v1.2.36 before v1.3.0 GA; full QA battery remains GA condition 10, not a version. |
| 2026-11-26 | **v1.2.20 JSON full DOM COMPLETE for C23.** New single-source runtime `runtime/json/c/*` with five builtins (`json_parse`, `json_type`, `json_serialize`, `json_get_path`, `json_last_error`), embedded by Go and generated into `src/compiler/json_runtime.kark`; freshness gate wired. Canonical `stdlib/json/json.kark` keeps the six flat helpers and adds five `json_dom_*` wrappers. `needsJSON` covers monolith and split flows. Gate `TestPhaseV1220_` PASS 33.196s on both engines with raw Go↔kcc parity, escape round-trip, deterministic errors, runtime wiring and native K145 refusal; two CI steps wired. Three defects found and fixed: Go/kcc Value-struct divergence (fixed via shared Value API), wiring negative case imported std.json, native refusal names a different `json_*` builtin. No byte-identity claim; KIR pin CI-measured only. Audit: `docs/audit/V1.2.20-JSON-FULL-DOM-BASELINE.md`. |
| 2026-11-26 | **v1.2.20 first-CI corrections against the frozen baseline** (CI run `45747053835`, no new increment). Two measured failures: Sphinx `-W` reported `json.rst:63` explicit markup ending without a blank line (fixed); whole-tree KIR pin was stale — CI reported `text=18972 verify=18972` against `18517`; both assertion sites re-pinned from that measurement only, never predicted. Delta +455 from `json_*` builtins and generated `json_runtime.kark`. Documentation-only correction plus pin re-pin; no code or gate changed. Record: `V1.2.20-JSON-FULL-DOM-BASELINE.md` §§6–7. |
| 2026-11-26 | **PR #41 MERGED** at `origin/main` `5ca3801`; `main`/`develop` re-synced. v1.2.20 JSON full DOM, its docs blank-line fix and its CI-measured KIR pin `text=18972 verify=18972` are now on main. The strict v1.2.x sequence gate for **v1.2.21 — UDP + WebSocket** is satisfied; inventory is being taken before code. No tag or release action taken. |
| 2026-11-26 | **v1.2.21 UDP/WebSocket COMPLETE for C23.** New single-source runtime `runtime/udpws/c/*` with nine builtins (UDP send/send_on/bind/recv/last_error, ws_accept/send_text/recv_text/last_error), embedded by Go and generated into `src/compiler/udpws_runtime.kark`; freshness gate wired. Generic RFC 6455 accept uses a self-contained SHA-1 + base64 implementation. Canonical wrappers added to `stdlib/net` and `stdlib/http`. `needsUDPWS` covers monolith and split flows. Gate `TestPhaseV1221_` PASS 62.486s on both engines with real UDP datagram round trip, WebSocket handshake/text frames, runtime wiring and native K145 refusal; two CI steps wired. Two defects found and fixed: Winsock not initialized caused `WSANOTINITIALISED (10093)`; gate expected `true` while the C23 path prints comparisons as `1`. No byte-identity claim; KIR pin CI-measured only. Audit: `docs/audit/V1.2.21-UDP-WEBSOCKET-BASELINE.md`. |
| 2026-11-26 | **v1.2.21 first-CI split-flow duplicate-symbol correction (verified locally, CI re-run pending)** — CI run `45762070084` failed Phase 114 GoEngine with multiple definitions of `karkain_udp_last_error` and `karkain_ws_last_error`. Root cause: `HeaderForRuntime` static-ifies only column-zero definitions whose signature line ends with `{`; the two helpers were one-line definitions ending with `}`. Both rewritten as multi-line definitions and `src/compiler/udpws_runtime.kark` regenerated. Local verification green: TestUDPWSKarkFresh PASS 2.811s and Phase 114 GoEngine/coverage PASS 743.032s; CI re-run still required. Record: `V1.2.21-UDP-WEBSOCKET-BASELINE.md` §8. |
| 2026-11-26 | **v1.2.21 KIR pin re-measured by CI** (CI run `45758961870`, no new increment): Phase 122 reported `text=19515 verify=19515` against `18972`; both assertion sites re-pinned from that measurement only, never predicted. Delta +543 from `udp_*`/`ws_*` builtins and generated `udpws_runtime.kark`. Record: `V1.2.21-UDP-WEBSOCKET-BASELINE.md` §9. |
| 2026-11-26 | **Native Mach-O CI job made manual-only** (corrective CI change against the v1.2.15 baseline, no code): the main-push-only condition still left a long Queued macOS check on PR boards. `.github/workflows/ci.yml` now uses `if: github.event_name == 'workflow_dispatch'`; PRs, develop pushes and main pushes SKIP honestly. Gate file, runner, timeout and assertions are unchanged; structural Mach-O coverage remains in required Test. Record: `V1.2.15-MACOS-EXECUTION.md` §5. |
| 2026-11-26 | **v1.2.22 real SQLite driver COMPLETE for C23.** Vendored official SQLite 3.46.1 amalgamation (hash-pinned) plus a small generated binding; five `sqlite_*` builtins, `needsSQLite` monolith gating, split-flow fallback and 20-minute compile budget for SQLite. `TestPhaseV1222_` PASS 111.075s on both engines with in-memory CRUD, temp-file reopen, deterministic `sqlite:` errors, runtime wiring and native K145 refusal. One harness defect fixed: per-engine temp DB files. Two CI steps wired. No byte-identity claim; KIR pin CI-measured only. Audit: `docs/audit/V1.2.22-SQLITE-DRIVER-BASELINE.md`. |
| 2026-11-26 | **v1.2.22 first-CI corrective change against the frozen baseline** (CI run `45769925067`, no new increment): KIR pin re-measured as `text=19769 verify=19769` and both assertion sites re-pinned from CI only; GCC `-Wdiscarded-qualifiers` from vendored sqlite3.c contaminated frozen `TestPhase132` goldens, so SQLite programs now add `-Wno-discarded-qualifiers` on POSIX paths. Local verification: Phase 132 PASS 129.666s, build/vet/freshness PASS. Record: `V1.2.22-SQLITE-DRIVER-BASELINE.md` §6. |
| 2026-11-26 | **v1.2.22 stale native-net gate correction** (no new increment): `TestPhaseV1210_NativeNetRefused` expected the literal word `socket`; the expanded `std.net` makes kcc refuse `udp_send_on` first. Gate now requires K145 naming any `socket`/`net_`/`udp_` symbol plus no image; local PASS 12.667s. |

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

> **Superseded on 2026-11-25.** The recommendation below (v1.2.3, `s[i]`) was
> implemented and followed by v1.2.4, v1.2.4b, v1.2.5a–v1.2.5c, P0.2 and the
> v1.2.6–v1.2.15 merge line (PRs #30–#36). Its rationale is retained unedited
> as the 2026-11-21 record — it was correct then. The current recommendation
> is in §5.1: **v1.2.17 — TLS/HTTPS**, after the v1.2.16 PR is merged and
> pulled. The deferral list below is unchanged except where §5.1 restates it.

**Recommended next slice: v1.2.3 — native string byte indexing (`s[i]`).** This
supersedes the older STD-001 recommendation. Measured repository state shows
user calls already work and `stdlib_v2` is blocked by string operations, not by
call dispatch. The first bounded blocker is `s[i]`, needed by `std.string`'s
`str_to_upper` and `str_trim`. Status `proposed`; it requires owner
authorisation before implementation begins.

Completed before this recommendation:

- NAT-FORIN and Phase 1.3 bounds corrections: merged in PR #5.
- v1.2.1 `std.path` / `std.json` gates and six CI-stability corrections: merged
  in PRs #6–#11.
- Post-merge `main` CI required `Test` job passed; only non-gating discovery
  jobs remain red.

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

**Historical note.** This section was the register's authority when written on
2026-11-21. It is no longer: §5.1 is the newer record and is now authoritative
for the current recommendation. If §5 and §9.4 disagree, §5 wins.

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
