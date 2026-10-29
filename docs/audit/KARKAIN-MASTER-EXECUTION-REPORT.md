# Karkain master execution report

**Date:** 2026-10-15
**Branch:** `develop` (unpushed, 9 commits ahead of `origin/develop`)
**Repository:** https://github.com/ajit-ai/Karkain
**Scope executed:** master plan Stages 1-3 plus a Stage-2 focused correction.
Stages 4-7 are queued, not executed; their next slice is named in section 13.

This report is the §21 deliverable. It supersedes nothing: the earlier
`KARKAIN-MASTER-PLAN-COMPLETION-REPORT.md` covers the previous prompt's
workstreams A-E and remains accurate for them.

---

## 1. Initial Git and release baseline (verified, not assumed)

| Property | Verified value | How |
|----------|----------------|-----|
| Branch | `develop` | `git branch --show-current` |
| `develop` HEAD at start | `4ea44c6` | `git rev-parse` |
| `main` | `f66863d`, unchanged throughout | `git rev-parse main` |
| `origin/main` | `f66863d` | unchanged; **never pushed** |
| `origin/develop` | `989a04c` | unchanged; **never pushed** |
| Working tree | CLEAN at start and end | `git status --short` |
| Stash | `stash@{0}` present, **never applied or deleted** | `git stash list` |
| Tags | 5, unchanged (`karkain-17`, `v0.14.0`, `v0.19.0`, `v1.0.0`, `v1.1.0`) | `git tag` |
| `VERSION` | `1.1.0`, **unchanged** | `cat VERSION` |

All six previously reported commits verified present with correct linear
parentage on `develop`, each parent being the previous commit:

```
4ea44c6 <- 146bb37 <- 6eff5a2 <- 024ca94 <- 64c63c6 <- cc44c95 <- 989a04c
```

No merge, tag, publish or release occurred in this session or the previous one.

## 2. Commit inventory for this execution (9 commits, all unpushed)

| Commit | Stage | Scope | Files |
|--------|-------|-------|-------|
| `cc44c95` | A | D5 close-out | 8 files: 3 `src/compiler/*.kark`, 1 new gate, 2 test files, `ci.yml` |
| `64c63c6` | B | Scorecard in four categories + governance report | 4 files |
| `024ca94` | C | Implementation control system | 4 files |
| `6eff5a2` | D | Type/variable declaration design | 2 files |
| `146bb37` | E | Roadmap + version policy reconciliation | 2 files |
| `4ea44c6` | review | Master completion report + TYPE-DESIGN status fix | 2 files |
| `e83385f` | **2** | **D5 gate strengthening (real defect found)** | 1 file |
| `ca19530` | 1 | **Issue register (23 entries)** | 1 file |
| this commit | - | Master execution report + REL-001/005 evidence | 2 files |

## 3. Stage 1 - Read-only audit findings

The reported baseline was verified item by item and **every claim held**. The
audit additionally produced:

1. **Three stale governance records** contradicted by committed work
   (`NEXT_PHASE_RECOMMENDATION.md` recommending a fixed parser OOM;
   `KARKAIN_IMPLEMENTATION_MATRIX.md` claiming no type checker and unparsed
   generics; `NAME-RESOLUTION-DESIGN.md` claiming nothing implemented while
   `pkg/sema/resolve.go` is 922 lines of exactly that). All three now carry
   evidence-backed historical banners.
2. **A false baselining claim** in the version plan ("152, 154 and 165 are not
   yet baselined"; 152 and 154 both have baseline notes).
3. **Stale closure figures** in the v1.2.0 checklist (`481 vs 1096`,
   `439 vs 454`, `348 vs 353` - D4 closed two of those shapes and D5 narrowed
   the third).
4. **A weaker-than-documented gate** (section 5).

## 4. Stage 2 - Reconciliation and a real defect found

### 4.1 D5 verification (Workstream A)

The three D5 tests were inspected for substance, as the plan requires:

| Test | Kind | Verdict |
|------|------|---------|
| `TestPhaseD5_ArrayBoundsReportThePhase100Diagnostic` | **execution differential** - builds real PE images with both engines through the real binary, runs them, asserts exact stdout bytes and exit codes | Real behaviour, not source text |
| `TestPhaseD5_DiagnosticTextIsInternedInTheImage` | **structural** - reads the emitted image for the interned diagnostic strings | Legitimate and labelled as such in the file; necessary because stderr is empty for PE images on this host |
| `TestPhaseD5_MapHelpersAppearOnTheSyntacticPredicate` | **structural** - image-size difference | **Defect found; corrected** |

None of the three tests asserts on implementation names or source text.

### 4.2 The defect

`TestPhaseD5_MapHelpersAppearOnTheSyntacticPredicate` asserted only
`delta >= 209`, and its own comment claimed the no-index case made it "a real
check" against a driver that emits the map helpers **unconditionally**. That
claim was false, and provably so: under unconditional emission both images grow
by the same amount, so the difference stays 209 and a `>=` assertion passes. The
defect the comment promised to catch was the one it could not see.

### 4.3 The correction (commit `e83385f`)

The gate now builds the same two programs on **both** engines and compares kcc's
delta against the oracle's, which is an assertion a constant cannot make.

A strict equality was tried first and **failed with real evidence**: kcc's delta
is 629 and the oracle's is 637 - a difference of exactly **8 bytes**, the
already-documented frame residual (`sub rsp,0x288` against `sub rsp,0x290`). The
indexed and unindexed programs do not share a frame, so part of each delta is
padding rather than map runtime. The final assertion is an equality within one
frame-rounding unit (16 bytes), a tolerance derived from the frame layout's own
rounding rule rather than guessed.

**Evidence discipline, stated plainly:**

- The equality arm demonstrably fires - it rejected a strict equality on a real
  8-byte measurement. It is not vacuous.
- What is **not** verified is that it fires for the unconditional-emission
  mutation. That mutation was not performed, because it requires editing
  `src/compiler/native_value.kark` and rebuilding kcc, which exceeds a gate
  correction. The claim is reasoned, not measured, and is recorded as such.
- The 209 floor is retained: the oracle comparison alone does not state a
  lower bound on the helper set's presence.

Result: `TestPhaseD5_` **3/3 PASS** (18.7 s). `go vet ./pkg/cli` clean. The
other two D5 tests are unchanged. No compiler source was touched.

## 5. Stage 3 - Reliability triage (discovery only)

### 5.1 REL-001 kcc access violation: reproduction attempted

| Item | Value |
|------|-------|
| Command | `go run ./cmd/karkain kir --verify src/compiler/kir.kark` |
| Commit | `e83385f` |
| Host | windows/amd64, ~4 GB, `GOMEMLIMIT=3GiB` |
| Result | **DID NOT REPRODUCE** |
| Exit | `0` |
| Elapsed | 996 s |
| Output | `[ok] kir text: 14050 lines` / `[ok] kir verify: 14050 lines ok` |

Prior evidence retained: the crash is intermittent, occurs on byte-identical
sources, and three hypotheses are already falsified with evidence (low-RAM class
- it succeeds at 127 MiB free and failed at 818 MiB; stack exhaustion - a 64 MiB
reserve still crashes; corrupt binary - two verifies with no rebuild both
crashed). It also reproduced on pristine HEAD, so it is not caused by any recent
change.

**Honest classification:** the incident remains **UNCLASSIFIED**, and
"did not reproduce" is *not* a fix. Two consecutive clean runs exist now; an
intermittent fault with a ~15-minute trigger needs more attempts than two before
any confidence statement is defensible.

### 5.2 The decisive experiment, and why it needs authorisation

The named cheap experiment is a build-flag change: an `-fsanitize=address` build,
or `-Wl,--stack,` to force a large stack reserve. Per the plan's own execution
authority, a change to build behaviour requires owner authorisation, so it was
**not performed**. No compiler source or build script was modified during
discovery.

### 5.3 REL-005 KIR pin: two independent measurements

The reproduction run doubled as a pin measurement, and it matters that the two
measurements were taken by different routes:

| Route | Elapsed | Result |
|-------|---------|--------|
| `TestPhase151A5_KIRPinHolds` (test harness) | 908.3 s | `text=14050 verify=14050` |
| `karkain kir --verify` (direct CLI) | 996 s | `text=14050 verify=14050` |

The pin is **measured, not predicted**, and never hand-edited. This closes the
"verified once" weakness and downgrades REL-005 from `blocked` to
reproduced-twice; a third run would close it.

### 5.4 Other reliability findings

E2 (parser robustness): the parser no-progress guard exists
(`pkg/parser/parser.go:51`) and the historical OOM report is correctly marked
stale. The gap is the absence of fuzz targets (REL-003), not a live defect.
E3 (native codegen): oracle-based differential gates exist and are green; the
known residual is NAT-001. E4 (integer/float): semantics are defined for shift
edges (151P0) and floats on the C path; the native path has no float statements,
which is NAT-003 rather than an unclassified risk. E5 (security): SEC-001 and
SEC-002 are recorded as gaps, **not** as vulnerabilities - no threat model or
impact assessment has been performed, and the plan forbids that label without
one.## 6. Architecture and capabilities as verified

**Two engines.** A Go front end (`pkg/parser`, `pkg/sema`, `pkg/codegen`) and the
self-hosted `kcc` (`src/compiler/*.kark`, kcc is the default engine since Phase
97). kcc assembles projects, resolves PM dependencies and verifies its own KIR.

**Three code paths.** SSA to C23 (both engines), a C-free native backend
(`pkg/native`: ELF, PE, Mach-O), and a handwritten WASM emitter (`pkg/wasm`).

**Three native targets, three different evidence levels** - deliberately not
described as equally verified:

| Target | Code generation | Structure | Execution |
|--------|-----------------|-----------|-----------|
| windows/amd64 | yes | yes | **executed on this host** (increment 145-150 goldens, D5 differential) |
| linux/amd64 | yes | yes | **executed on CI** only; not runnable on the dev host (no WSL/Docker) |
| darwin | yes | yes | **unverified** - no macOS runner exists; every Mach-O claim since increment 149 is structural |

**Conformance:** 59 conformance tests, a 61-file byte-identical corpus (Phase
114), probes 11/11, plus per-feature parity gates.

**Ownership:** `kccOwnsNativeTargets` is `true` and all three C-free targets are
kcc-owned (151D, A1, Phase B), with a provenance requirement before the CLI
writes any bytes.

## 7. Scorecard result and evidence gaps

30 dimensions in four categories: operational maturity (7), architecture (9),
compiler engineering (8), ecosystem (6). No aggregate grade is published.

Strong: conformance and oracle parity, CI enforcement, differential discipline.
Weak, and stated as weak: performance (OM6 = 2 - one benchmark function, no
baseline), fuzzing (OM7 = 2 - zero targets), security (OM5 = 2 - no SBOM or
signing), binary size (CE8 = 2 - no budget). NE, never "passing": macOS
execution, accelerator execution, and `MM-K-time` (unassessable - no inventory
row exists).

Fresh Sphinx `-W -E` build: succeeded, zero warnings. The first invocation was
cached ("no targets are out of date") and was **rejected as weak evidence** and
re-run with `-E`.

## 8. Crash classification and unresolved hypotheses

REL-001 remains **UNCLASSIFIED**. What is known, and what is not:

| Question | Answer |
|----------|--------|
| Does it still occur? | Not reproduced in 2 attempts at `e83385f` (908 s, 996 s) |
| Is it deterministic? | No - it occurred on byte-identical sources |
| Is it recent? | No - it reproduced on pristine HEAD |
| Is it memory pressure? | Falsified: succeeds at 127 MiB free, failed at 818 MiB |
| Is it stack exhaustion? | Falsified: a 64 MiB reserve still crashed |
| Is it a corrupt binary? | Falsified: two verifies with no rebuild both crashed |
| Where does it live? | **Unknown** - frontend, codegen, runtime, Windows ABI, harness or environment are all still open |
| Smallest decisive test | An ASan or `-Wl,--stack,` build - **needs owner authorisation** |

Missing data, named: no crash dump was ever captured (WER LocalDumps was enabled
for one attempt and removed without producing one), and no instrumented build
exists. Both are prerequisites for narrowing the hypothesis set further.

## 9. D6 recommendation (NAT-001)

**Do not implement D6 yet.** The residual is 8 bytes (`sub rsp,0x288` against the
oracle's `sub rsp,0x290`). Two reasons, both evidence-based:

1. It is cosmetic beside the open value-model kinds (NAT-003), which are
   larger, unblocked, and each of which will touch the frame layout again.
2. `natFrameLayout` was frozen in 151A Step 1 with every Step 1-9d displacement
   pinned against it, so closing D6 now reopens frozen gates for an 8-byte gain.

When it is taken, it is **narrow**: change the frame rounding rule to match the
oracle, add a focused regression test, and preserve the D2 and D5 gates. If the
oracle's difference proves non-semantic, the project's benchmark policy permits
classifying it as an accepted non-parity with a documented rationale instead of
forcing byte identity.

## 10. Standard library and platform status

- Phase 152: **partial**. 152-A, 152-B0, 152-B1, 152-B2 shipped; closure
  conditions 2 and 3 carried to v1.3.0 as UNMET by owner amendment, recorded
  verbatim.
- Increment 154: **baselined, not implemented** (GOV-001).
- Increment 165: **not started** (GOV-002).
- Standard library: 10 modules frozen; `std.net`/`std.http`/`std.db` real;
  remote registry and git fetch are explicit refusals by design (SEC-002).

## 11. Type-system and declaration decisions

Recommended: `let mut`, with `var` deprecated for one minor. Settled: one
declaration path remains (`parseVarDecl` handles `let`/`var`/`const`);
`Type string` stays for v1.x; mutability is a semantic-analysis concern, not a
codegen one; diagnostics must assert code and name on both engines.

Open, awaiting owner sign-off: D-1 (adopt `let mut`), D-2 (`var` alias
duration), D-3 (when `let` becomes immutable - breaking, and it breaks a frozen
gated corpus), D-4 (real type values), D-5 (field/parameter mutability), D-6
(exported mutable globals).

Implementation remains a separate authorised slice. No syntax was changed.

## 12. Release-readiness status (v1.2.0)

| Area | State | Evidence |
|------|-------|----------|
| Compiler correctness | partial | increment 151 remainder open (NAT-003); D5 closed the array-index mechanisms |
| Platform evidence | partial | windows executed; linux CI-only; macOS structural only |
| Runtime and stdlib | **blocked** | STD-001 deferred to v1.3.0 by owner amendment |
| CI gates | met | one dedicated step per gate family |
| Documentation | met | Sphinx `-W -E` clean, 0 warnings |
| Known defects | recorded | REL-001 unclassified; REL-002 unproven |
| Conformance | met with a caveat | CONF-001: goldens can pass a wrong value both engines agree on |
| Reproducibility | met | stage-1 byte-reproducible; KIR pin measured twice |
| Security/dependencies | **gaps recorded** | SEC-001, SEC-002 - not vulnerabilities, no threat model performed |
| Release artifacts | not produced | No merge, tag or release performed; owner-only ceremony |

**v1.2.0 is not shippable.** Two of its eight closure conditions are carried to
v1.3.0 by owner decision, and GOV-001 (154) is a closure condition with no
implementation.

## 13. The single recommended next implementation slice

**REL-001 - root-cause investigation of the kcc access violation.**
Status `proposed`; **requires owner authorisation** because its decisive
experiment changes the build.

Next action for the owner, in order:

1. Authorise an ASan (or `-Wl,--stack,`) build of kcc - discovery only, no
   committed source change.
2. Run `kir --verify` under it, three or more times, capturing any dump.
3. Classify the fault by the layer it actually lives in, then re-queue with
   evidence.

**Runner-up: NAT-003**, the next native-driver value-model kind. Unblocked and
approvable today; second only because it unblocks no measurement.

**Deferred, with reasons:** NAT-001 (premature - see section 9), STD-001 (owner
amendment), NAT-002 (needs a runner, an owner decision), SEC-001/PERF-001 (real,
not release-blocking), LANG-001 (owner language decision), GOV-003 (no incident
has required a rollback runbook yet).

## 14. Stages not executed

Stages 4-7 of the master plan are **queued, not done**. Specifically: no new
implementation slice has been started beyond the Stage-2 gate correction,
because the plan's own sequencing rule forbids starting downstream feature work
while the reliability prerequisite is unresolved, and the recommended slice
needs authorisation this prompt does not grant.
