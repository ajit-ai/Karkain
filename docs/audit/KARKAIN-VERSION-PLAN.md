# Karkain Version Plan — continuous v1.x.x line (independence completion)

**Status:** authoritative forward view (version strategy + release ledger + historical reconciliation).
**Baseline:** `VERSION` = **1.2.0** (published 2026-11-14); `main` @ `b5dcd7f`; `develop` @ `8d16990`.
Last updated: 2026-11-15 (documentation reconciliation; no compiler work).

**Companion documents** (this file does not replace them):

| Document | Role |
|---|---|
| `AGENTS.md` | completion record + branch/cadence rules |
| `docs/audit/KARKAIN-ARCHITECTURE-ROADMAP.md` | how the compiler is layered today, and which layer changes hands in which release |
| `docs/audit/KARKAIN-INDEPENDENCE-ROADMAP.md` | I / S / L / P / D track definitions + per-module stdlib backlog |
| `docs/audit/GENERAL-AVAILABILITY-ROADMAP.md` | GA-1/2/3 milestones (phases 128–142) |
| `docs/release/v1.2.0-CHECKLIST.md` | v1.2.0 scope freeze and carried conditions (verbatim) |
| `docs/release/v1.3.0-CHECKLIST.md` | the PREPARED-not-open home for v1.3.0 (this plan governs when and whether it opens) |
| `docs/source/development/semver-policy.rst` | what a patch / minor / major may contain |
| `docs/source/status/scope.rst`, `docs/source/reference/stable-api.rst` | public status + API ledger |
| `docs/audit/KARKAIN_MATURITY_SCORECARD.md` | Language Quality Scorecard (30 dimensions) + gates A–F |
| `docs/audit/KARKAIN-IMPLEMENTATION-CONTROL.md` | the single authoritative implementation queue |
| `docs/audit/UNIFIED-TYPE-DECLARATION-DESIGN.md` | design milestones, not delivery claims |
| `ROADMAP.md` | historical phase log (Phases 1–49) |

---

## 0. Master objective

**Karkain Independence Completion.**

> Karkain develops Karkain. Karkain compiles Karkain. Karkain runs Karkain.

Every increment below serves one of the seven independence objectives (§7). No
increment is accepted because it is intellectually interesting, and no
completion claim is made beyond what its evidence supports.

---

## 1. Versioning policy — one continuous `v1.x.x` line

1. **One line.** All future releases sit on the `v1.x.x` continuous line. No
   renumbering of published tags, no rewriting of release history, no
   "next big version" jumping for its own sake.
2. **Next available number respects history.** `v1.2.0` is published; the next
   release is `v1.2.1`. `v1.1.1`, `v1.1.2` and `v1.2.3` **do not exist**
   historically (§3), and no future release may claim them as history.
3. **Small increments by default.** Each release ships one or a few bounded
   capabilities with its own gate, evidence and record. A release is done when
   its gate is green, not when its theme sounds complete.
4. **A new minor version is justified only when it earns one.** `v1.3.0` opens
   only when the *Language + library completeness* theme is genuinely ready
   (§7.3), not in order to group ordinary work. Small compatible improvements
   ship as `v1.2.x` patches.
5. **No silent additions.** Scope freezes at version open. New ideas go to the
   next version or an explicit carry-over list.
6. **SemVer class is decided at open**, against `semver-policy.rst`.
7. **Cadence is version-planned, increment-executed.** Per increment: code +
   gate (a test file **and** a `ci.yml` step) + audit note + `AGENTS.md`
   record, then `develop` → `main` per repo policy. Per version close: QA
   battery + version identity + docs flip + owner-only tag + GitHub Release.
8. **Owner-only actions are never milestones.** Cutting tags, container
   registry visibility and code-signing identities are owner actions.
9. **No unrelated expansion.** Product, platform, enterprise or "platform"
   roadmap items do not enter this line. They wait for the independence
   objective that needs them, or never enter.

---

## 2. Historical version register (verified 2026-11-15)

Every statement below was checked against local Git objects, `git ls-remote`,
and `gh release list` against `ajit-ai/Karkain`. Anything not listed here is
**unverified**, not assumed.

### 2.1 Tags and peeled commits

| Tag / ref | Tag object | Peeled commit | Type | Release exists? |
|---|---|---|---|---|
| `karkain-17` | `e9cc396d35de6f6a24e87cd35b9385f19b534818` | `e9cc396d35de6f6a24e87cd35b9385f19b534818` | commit (lightweight) | **yes** — "Phase 17 COMPLETED (v0.14.0)", 2026-08-14, 0 assets |
| `v0.14.0` | `44256f4c903d143b4c3f60b1c4fdbaa53a21aac0` | `627e68a62f713b4b907b9b966ce8f79f84574918` | tag | **no** |
| `v0.19.0` | `658fdb7ac1dd86fa75e137565455bfbadcff4c8c` | `658fdb7ac1dd86fa75e137565455bfbadcff4c8c` | commit (lightweight) | **no** |
| `v1.0.0` | `236ed11ab1310fe42684386f963ed595f32ee71c` | `e8f81634714b2279bf1884dbc8039adfbbcac68a` | tag | **no** (no GitHub Release) |
| `v1.1.0` | `618fff9ae0d91436d160225dd78765ae0767c56d` | `951ee10893140bd16d1f39215e31731319c9b222` | tag | **yes** — 2026-09-25, 14 assets |
| `v1.2.0` | `e900c91e485fc316f52b664228f6c35f25416436` | `77d70693e1e3134dac312a303bffafa7d50fa988` | tag | **yes** — 2026-11-14, 14 assets |
| `1.0.x` branch | — | `e8f81634714b2279bf1884dbc8039adfbbcac68a` | branch | LTS line; never released as a GitHub Release |

### 2.2 Published GitHub Releases

| Release | Published | Assets | Notes |
|---|---|---|---|
| `karkain-17` | 2026-08-14 | 0 | legacy pre-release bookkeeping; not a v1.x.x version |
| `v1.1.0` | 2026-09-25 | 14 | published by the tag-triggered CI release job |
| `v1.2.0` | 2026-11-14 | 14 | **locally built and published; NOT a CI-green release** — see §3.3 |

### 2.3 The queried versions — factual finding

| Version | Status | Evidence |
|---|---|---|
| `v1.1.1` | **never existed** | not a tag, not a branch, not a file, not a test fixture, not a release |
| `v1.1.2` | **never existed** | same |
| `v1.2.3` | **never existed as a version** | occurs only as SemVer parser test fixtures (`pkg/pm/semver_test.go`, `manager_test.go`, `lock_test.go`) — e.g. `ParseVersion("1.2.3")`, `Satisfies(v, "^1.2.3")`. These are parser tests, not a release, a tag or a plan. |

### 2.4 Classification of everything found

| Class | Members |
|---|---|
| **Published release** | `v1.1.0`, `v1.2.0`, `karkain-17` |
| **Existing tag, no published release** | `v0.14.0`, `v0.19.0`, `v1.0.0` |
| **Completed development, not released** | Increments 1–149; increments 150, 151P0, 151 (all slices), 152 (152-A/B0/B1/B2), 154 (implementation); NAT-CALL (qualified); NAT-STDLIB int-array slice (qualified locally, unreleased) |
| **Work in progress** | NAT-STDLIB string tranche (blocked, documented in `NAT-STDLIB-STRING-BASELINE.md`); NAT-NET and later v1.3.0 items |
| **Proposed only** | every `v1.2.1`..`v1.x.x` step and beyond in §7 of this document |
| **Unverified historical reference** | none remaining — every entry above is now either verified or explicitly marked absent |

### 2.5 Branches (verified)

`main` (`b5dcd7f`), `develop` (`8d16990`), `1.0.x` (`e8f8163`, LTS),
`feature/v1.2.0-native-independence` (`b49b463`), `feature/v1.3.0-natcall`
(`aa45970`), `feature/v1.3.0-natstdlib` (`199ee87`),
`feature/v1.3.0-natstdlib-string` (`a8191af`),
`feature/automated-review-system` (`951ee10`), `fix/struct-field-checking`.

---

## 3. Completed work and its honest status

### 3.1 What actually shipped in v1.2.0 (2026-11-14)

**Theme: Sovereignty I — Off C, Off Go.** Verified against the tagged commit
(`77d70693e1e3134dac312a303bffafa7d50fa988`):

* **NAT-CALL** — kcc native driver lowers user function calls (int arguments,
  int returns, multi-function programs, forward references, recursion).
  `pkg/cli/phase151e_call_test.go`; executed PE goldens on Windows/amd64;
  Linux ELF byte-identity vs the Go oracle by differential; CI step
  `Run NAT-CALL kcc native user-call gate`.
* **NAT-IF / NAT-LOOP / NAT-IDX / NAT-FRAME / NAT-STR-STAGE / NAT-RODATA /
  NAT-REG** — branches, loops, array indexing, frame layout, string staging,
  reporter rodata, register allocation (opt-in `KARKAIN_NATIVE_REGALLOC=1`;
  default remains the stack-based path; no performance improvement claimed).
* **Three native OS targets are kcc-owned** — `native-x86_64-windows`
  (executed on the dev host), `native-x86_64-linux` (ELF byte-identical to the
  Go oracle; executed on the Linux CI leg), `native-x86_64-macos` (Mach-O PIE,
  **structurally validated only; never executed** — no macOS runner exists).
* **Runtime error model on native** — array bounds report
  `runtime error: array index out of range at <file>:<line>` instead of an
  `Int3` trap.
* **Bitwise operators** (151P0) on both engines, with defined edge behaviour.
* **Native container** — PE32+ (DOS/COFF/PE32+ headers, IDT/ILT/IAT,
  `.reloc` DIR64 blocks), ELF64, Mach-O PIE.
* **Library / stdlib** — 10 modules frozen; `std.numerics` (40 functions);
  59-golden corpus.

### 3.2 What is NOT in v1.2.0 and is honestly UNMET

Carried to v1.3.0 by owner-authorized amendments, text unchanged:

| Carried condition | Status | Reason (measured) |
|---|---|---|
| 2 — `stdlib_v2` with no C compiler | **UNMET** | native driver had no function call when the carry was authorized (NAT-CALL since closed the call lowering); `stdlib_v2` itself refused by name for string/codec reasons (see §3.4) |
| 3 — `std.net` with no C compiler | **UNMET** | no socket emitter in `pkg/native` |
| 5 — `stage2 == stage3` | **UNMET** | increment 154 implemented (`62a6323`); proof needs ≥1.5 GiB free or a runner that survives the stage-2 burn (exit 143) |
| 6 — no Go toolchain for that closure | **UNMET** | same proven run |

Effective v1.2.0 gate = conditions **1, 4, 7, 8**.

### 3.3 Release provenance defect (recorded, does not recur)

The v1.2.0 archives were **built locally on a Windows host and published by
the owner**; the tag-triggered CI `release` job could not execute (account
billing/payment restriction). The release is explicitly **NOT a CI-green
release**, recorded in `KARKAIN-V1.2.0-GA-QUALIFICATION.md` §4–§5,
`KARKAIN-1.2-RELEASE-NOTES.md`, `installation.rst`, `release-notes.rst`,
`development/release.rst`, `status/scope.rst`, and the v1.2.0 Release body
itself. A stale-CI-manifest incident on 2026-11-14 (triggered by the release
provenance commit) re-confirmed the restriction; runs `43556707796`,
`43556713438`, `43556713444` all failed at artifact upload.

### 3.4 NAT-STDLIB — what exists and what does not

**Implemented (int-array slice, unreleased):**

* `import std.collections` — unmodified stdlib source assembled and lowered
  through the self-hosted engine for the supported subset; `array_sum` (26),
  `array_max` (9), `array_contains` (1, 0) execute as kcc-produced PEs with
  exact stdout bytes on Windows/amd64.
* Two pre-existing driver defects found and fixed: `while (i < len(arr))`
  routed `len` into user-call lowering and crashed the PE linker; the
  runtime-error reporter gate scanned only `main`'s body, so a callee's index
  crashed the linker.
* Emit-time skipping of unsupported-unreachable callees (keeps
  byte-identity for supported dead code; `NAT-STR-STAGE` frame pin stayed
  valid: 2 functions emitted for a supported dead helper).
* KIR pin re-measured 15243 → **15671** (never predicted).
* Gates green locally; CI step wired, **not executed in CI** (billing).

**Not present (do not claim):**

* `std.string`, `std.encoding`, `std.crypto` — no native runtime, no codecs
  (trim/substr/split, hex/base64/sha256).
* `examples/stdlib_v2` does **not** compile natively (refused by name at
  `let name = …`).
* Array-of-string elements; `str_split`; string arrays.
* Any allocation in a loop — the bump arena is compile-time bounded.
* String ABI/parameter/return-kind inference — WIP, blocked by host memory
  (see `NAT-STDLIB-STRING-BASELINE.md`).

### 3.5 NAT-STDLIB-STRING (attempted, BLOCKED)

Outcome recorded in `docs/audit/NAT-STDLIB-STRING-BASELINE.md` (verdict
**BLOCKED — NOT QUALIFIED**): the host exhausted memory repeatedly in this
session (free RAM fell to 138 MiB), so no gates were run, no kcc rebuilt, and
nothing is claimed for the string tranche. WIP preserved off-branch.

### 3.6 What "increment 154" actually is

`KARKAIN_ENGINE=go` stage-1 pin *retired* in `pkg/bootstrap` — the
self-hosted compiler is now the default engine for the compiler itself.
Gate `TestPhase154_PinMechanism` runs in the required `Test` job. The
end-to-end `stage2 == stage3` proof is carried to v1.3.0 and remains unmet.

---

## 4. Outstanding independence gaps (distinguishing incomplete from done)

| # | Gap | Objective | Status |
|---|---|---|---|
| G1 | Native `stdlib_v2` acceptance (build + execute no-C) | §7.4 | **NOT STARTED** — string runtime + codecs missing |
| G2 | Native `std.net` (sockets) | §7.4 | **NOT STARTED** — no socket emitter; needs platform decision |
| G3 | Stage-2 == stage-3 bit-reproducible closure, no Go toolchain | §7.5 | **BLOCKED** — implemented (`62a6323`); needs ≥1.5 GiB or a surviving runner |
| G4 | Native string runtime: params, returns, concat, equality, slicing, indexing, len-in-callee | §7.4 | **WIP, blocked on host** |
| G5 | `trim`/`substr`/`split` and hex/base64/sha256 native builtins | §7.4 | **NOT STARTED** |
| G6 | `std.path`/`std.env`/`std.time`/`std.random`/`std.json` modules | §7.4 | **NOT STARTED** |
| G7 | `std.net` platform support matrix (Linux/Windows/macOS) with execution evidence | §7.4 | **NOT STARTED** |
| G8 | Undecided: whether the arena becomes a dynamic allocator, or v1.3.0 scope waits | §7.3/§7.4 | **OWNER DECISION** |
| G9 | Mach-O execution evidence (no macOS runner) | §7.2 | **BLOCKED** — structural only |
| G10 | Register allocation as default (opt-in today) | §7.2 | **NOT STARTED** — needs full qualification |
| G11 | Full native/Go byte parity across every program shape (`array_index` still differs by 8 bytes) | §7.2 | **PARTIAL** — corpus parity on covered shapes; D4/D5 closed; D6 not scheduled |

Nothing on this list is "almost done" unless its evidence says so; every row
states the evidence it needs to move.

---

## 5. Historical and current release ledger (one place)

| Version | State | Evidence |
|---|---|---|
| `karkain-17` | legacy tag + GitHub release | 2026-08-14; 0 assets |
| `v0.14.0` | tag only, no release | tag object `44256f4c`, commit `627e68a` |
| `v0.19.0` | tag only, no release | lightweight tag `658fdb7` |
| `v1.0.0` | tag, no release; `1.0.x` LTS branch kept | tag `236ed11a`, commit `e8f8163` |
| `v1.1.0` | **released** | tag `v1.1.0` (`618fff9a`, peeled `951ee10`), GitHub release 2026-09-25, 14 assets |
| `v1.2.0` | **released** | tag `v1.2.0` (`e900c91e`, peeled `77d7069`), GitHub release 2026-11-14, 14 assets; **local build, not CI-green** |
| `v1.1.1` | **does not exist** | — |
| `v1.1.2` | **does not exist** | — |
| `v1.2.3` | **does not exist** | SemVer test fixtures only |
| `v1.3.0` | **not open** — `PREPARED` in `v1.3.0-CHECKLIST.md`, not authorized | work present on `feature/v1.3.0-*` branches; see §6 |

---

## 6. Preserved current work (do not discard, duplicate or re-plan)

| Slice | State | Record |
|---|---|---|
| NAT-CALL | **complete + qualified** | `phase151e_call_test.go`, `phase151o_natcall_qual_test.go` |
| NAT-IF / NAT-LOOP / NAT-IDX / NAT-FRAME / NAT-STR-STAGE / NAT-RODATA / NAT-REG | **complete** | respective baselines; all gates green on this host |
| NAT-STDLIB (int-array slice) | **implemented, execution evidence on Windows/amd64** | `phase151p_natstdlib_test.go`; CI wired, not executed (billing); not released |
| NAT-STDLIB-STRING | **BLOCKED, not qualified** | `NAT-STDLIB-STRING-BASELINE.md`; WIP preserved off-branch |
| Increment 150 (all slices) | complete | `PHASE-150-BASELINE.md` — frozen |
| Increment 151 (all slices) | complete | `PHASE-151-BASELINE.md`, container + P1 resolution |
| Increment 152 | **partial** — 152-A/B0/B1/B2 shipped | `v1.2.0-CHECKLIST.md` §2.1/§4 |
| Increment 154 | **implemented, unverified end-to-end** | `62a6323`; pin contract gated; three-stage proof carried |
| 165 (Docker image) | **not started** | no baseline |
| Language Hardening Checkpoint | **CLOSED** | LH-1, LH-2 complete and gated |

Frozen increments (increment 150, 151, LH-1/LH-2, and completed slices of
151A–151C3) are not re-planned.

---

## 7. Incremental future roadmap — continuous `v1.x.x`

### 7.1 Strategy in one line

Ship small, evidence-backed increments on `v1.2.x`. Open `v1.3.0` only when the
*Language + library completeness* theme genuinely closes its acceptance
conditions (§7.3). `v1.4.0` and beyond open only for their own themes — never
to sweep up unrelated work.

### 7.2 Independence objectives → candidate increments

| Objective | Definition | Candidate increments (§7.3) |
|---|---|---|
| **O1 Compiler implementation independence** | Replace Go-dependent compiler functionality; kcc natively emits KIR and selects targets without Go-oracle comparison | v1.2.1, v1.2.2 |
| **O2 Native backend independence** | Karkain-owned machine code + object writers; PE/ELF/Mach-O | v1.2.4, v1.2.6 |
| **O3 Runtime independence** | Reduce external runtime dependencies; native runtime helpers (no libc) | v1.2.5, v1.2.8 |
| **O4 Standard-library independence** | Native stdlib execution: strings first, then codecs, then OS modules | v1.2.3, v1.2.7, v1.3.0 items |
| **O5 Bootstrap independence** | Remove Go/C requirements from the compiler bootstrap | v1.2.9, v1.3.0 acceptance |
| **O6 Toolchain independence** | Reduce external dependencies in dev tools; kcc owns PM | v1.3.0 items |
| **O7 Self-hosting qualification** | Reproducible, independently verified builds | v1.2.9, v1.3.0 acceptance |

### 7.3 Proposed incremental releases (all provisional IDs)

Numbering rule: `v1.2.N` increments ship while v1.3.0 is unavailable; if a
theme genuinely changes surface compatibility, the version jumps to `v1.3.0`.
All acceptance criteria are gated; all regressions are focused.

| Version | Objective (one line) | Why | Dependencies | Acceptance criterion | Focused regression expectations |
|---|---|---|---|---|---|
| **v1.2.1** | Qualify the string ABI: parameter/return-kind inference + two-unit staging | Prerequisite for every stdlib string call | NAT-STDLIB int-array (landed) | string-ABI gate green on Windows PE; Linux ELF structural; NAT-CALL/NAT-STDLIB gates green | NAT-CALL, NAT-STDLIB (int-array), NAT-IF/LOOP |
| **v1.2.2** | String concat + equality + `let x = <string call>` qualified | Enables `str_concat`, `str_to_upper` | v1.2.1 | string-golden gate green, byte-identical Go↔kcc on Linux ELF; no regression in existing `.kark` corpus | NAT-STDLIB, NAT-CALL, NAT-LOOP, NAT-IDX |
| **v1.2.3** | Native `trim`/`substr`/`split` as driver builtins | `stdlib_v2` unblocks | v1.2.2, concat | the three builtins' goldens pass on both engines; `import std.string` + `str_len("  x  ")` works natively | NAT-STDLIB int-array, NAT-IF, NAT-CALL |
| **v1.2.4** | Record & map runtime polish: field offsets for ≤8 fields, literal addresses | records byte-identity for stdlib | v1.2.1 | record golden corpus green on PE; no drift in existing goldens | NAT-IDX, NAT-FRAME, NAT-STR-STAGE |
| **v1.2.5** | Arena redesign: decide growable heap vs keep bump-only arena; document decision | Loops that allocate strings are mandatory for `std.string` | owner decision on allocator; §4 G8 | decision recorded in `PHASE-…-BASELINE.md`; allocation lowering gated on the chosen model | NAT-IDX, NAT-RODATA, NAT-FRAME (frame-byte pins cannot silently move — gate asserts) |
| **v1.2.6** | Codecs: hex, base64, sha256 as driver builtins | `std.encoding`/`std.crypto` | v1.2.5 | FIPS 180 / RFC 4648 vectors byte-identical on Go↔kcc; both engines import `std.encoding` | NAT-STDLIB int-array, NAT-CALL |
| **v1.2.7** | `std.path`, `std.env`, `std.time`, `std.random`, `std.json` — the I-1 OS modules | stdlib completeness | v1.2.6 | each module builds + runs natively (no C compiler), goldens pinned; docs pages | NAT-STDLIB, NAT-CALL, NAT-LOOP |
| **v1.2.8** | `std.net` decision + sockets (if Linux syscall surface chosen) | closes v1.2.0 carried condition 3 | v1.2.6, allocator decision, platform matrix | the chosen platform(s) execute `examples/04-networking/02_tcp_roundtrip.kark` no-C; others refused by name | NAT-STDLIB, NAT-CALL, Phase 152 no-C gate |
| **v1.2.9** | Stage-2 == stage-3 proof on a runner (or ≥1.5 GiB host); witness recorded | closes carried conditions 5–6 | v1.2.8 | `stage2 == stage3` bit-identical, nonzero byte count, no Go toolchain in child env; CI job records verdict | 154 pin gate, bootstrap gate |
| **v1.3.0** | **Language + library completeness** — `stdlib_v2` builds and runs no-C; `std.net` on claimed platforms; PM owned by kcc; zero Go-engine-only core rows | the closure the carried conditions demand | v1.2.1…v1.2.9 | all ten v1.3.0 closure conditions green (see `docs/release/v1.3.0-CHECKLIST.md` §2) | full QA battery + all gates |

`v1.4.0+` (each needs its own justification, not a number assignment):

| Possible (not scheduled) | Objective | Note |
|---|---|---|
| v1.4.0 | Tooling/debugging: DWARF consumed (breakpoints, step), `prof` allocation sites | not promised; needs evidence |
| v1.5.0 | Platforms + freestanding + riscv64 + packaging | needs real runners |
| v1.6.0 | Performance + footprint (full self-build ≤2.5 GB) | needs measurement |
| v2.0.0 | Breaking layout change (C-layout structs) | the only planned breaking change |

### 7.4 What v1.3.0 requires to open (and when)

`docs/release/v1.3.0-CHECKLIST.md` is **PREPARED, not open**. v1.3.0 opens
only when:

1. `examples/stdlib_v2` builds and executes with gcc/clang/cl absent from
   `PATH`, output byte-identical to the Go/C backend;
2. a real `std.net` program builds and runs no-C **on each platform claimed**;
3. increment 154's `stage2 == stage3` / no-Go-closure proof is demonstrated on
   suitable host/runner;
4. all remaining ten closure conditions are green with their own gates.

Until then, work continues on `v1.2.x` increments.

### 7.5 Version numbering rules (future)

* **Patch (`v1.2.x`)**: one focused capability, no new or changed public
  syntax/API, no release ceremony beyond QA + tag.
* **Minor (`v1.x.0`)**: opens only for a named, justified theme with its own
  frozen checklist (as v1.3.0's exists). Opening one is an owner decision, not
  an automatic promotion after "enough" increments.
* **Major (`v2.0.0`)**: only for the intentional breaking change (C-layout
  structs); never used as "completion theatre".
* **At version open**: `docs/release/vX.Y.Z-CHECKLIST.md` is written (freezes
  scope) and `VERSION` moves **in the tag commit**. Version identity tests pin
  it; no document may promote an unreleased version to shipped.
* **Per increment**: baseline note → work → gate (test file + ci.yml step) →
  audit note → `AGENTS.md` record → merge → push → CI → single merge push to
  `main` (owner ceremony only for tags/releases).
* **At close**: full QA battery → docs flip → owner-only tag → GitHub Release
  → post-publish verification against shipped artifacts → previous line keeps
  its LTS branch (`1.0.x` keeps representing 1.0.x; `main` always latest).

---

## 8. Verified state snapshot (2026-11-15)

| Fact | Value |
|---|---|
| `VERSION` / banner | `1.2.0` / `Karkain Compiler v1.2.0 (windows/amd64, Stable Build)` |
| `main` / `develop` | `b5dcd7f` / `8d16990` |
| Published releases | `karkain-17` (0 assets), `v1.1.0` (14), `v1.2.0` (14) |
| Tags (with peeled commit) | `karkain-17`→`e9cc396`, `v0.14.0`→`627e68a`, `v0.19.0`→`658fdb7`, `v1.0.0`→`e8f8163`, `v1.1.0`→`951ee10`, `v1.2.0`→`77d7069` |
| Branches | `main`, `develop`, `1.0.x`, `feature/{v1.2.0-native-independence, v1.3.0-natcall, v1.3.0-natstdlib, v1.3.0-natstdlib-string, automated-review-system}`, `fix/struct-field-checking` |
| Corpus | 61 pinned goldens / 15 categories; 64 conformance tests |
| KIR pin | whole-tree KIR **15671** lines (re-measured, never predicted) |
| Gates | ~24 CI gate steps; Windows PE executed on this host; Linux ELF CI-executed |
| Audit reports | ~110 `PHASE-*.md` records + the milestone baselines |

---

## 9. Status ledger (kept current, not updated silently)

| Version | State | Blocker / next step |
|---|---|---|
| v1.1.0 | released; maintenance LTS via `1.0.x` | none |
| v1.2.0 | released; published locally, not CI-green | carried conditions 2/3/5/6 remain UNMET; see §3.2 |
| v1.2.1… | proposed increments | v1.2.1 string ABI qualification is the next step on a host with ≥1.5 GiB free or a runner |
| v1.3.0 | PREPARED, not open | requires §7.3 exit criteria |
| v1.4.0+ | not scheduled | not planned; not baselined |
| v2.0.0 | not scheduled | future breaking change only |

## 10. Anti-scope

Not in this line unless an independence objective forces them: public package
registry, TLS, AES-GCM/ChaCha/RSA as new dependencies, async/await, full JIT,
quantum authoring surface, GPU/NPU hardware validation, any platform without a
runner, Apple/Google distribution pipelines. Anything of that shape waits for
its own justified theme or never ships.

## 11. What does not change because of this document

* No compiler source, runtime, stdlib, CI workflow, or test was modified.
* No tag moved; no release created, edited or deleted; no push attempted.
* `VERSION` remains `1.2.0`.
* Existing branches and the nine untracked working-tree files are untouched.
