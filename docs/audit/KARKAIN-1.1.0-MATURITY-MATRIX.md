# Karkain 1.1.0 — Maturity & Capability Gap Audit

**Audit type:** evidence-based repository audit. **Audit-only** — no compiler
change, no refactoring, no new phase, no commit.

| Field | Value |
|---|---|
| Repository | `F:\Codes\Git\Karkain` |
| Audited commit | `6162db448a66c11577ee08b4deba01b76916cb2d` (`main`, = `origin/main` = `origin/develop`) |
| Released baseline | tag **`v1.1.0`** -> `951ee10893140bd16d1f39215e31731319c9b222`, dated **2026-09-25** |
| `describe` | `v1.1.0-68-g6162db4-dirty` |
| Baseline distance | **68 commits** ahead of `v1.1.0` (49 non-merge) |
| `VERSION` | `1.1.0` |
| Working tree | **dirty** — see §9. Uncommitted Step-3 work is **excluded** from every finding below. |

Every substantive claim is keyed to a file, line, git object or counted
artifact. Claims that could not be evidenced are marked **unevidenced** rather
than inferred.

---

## 1. Verdict

**Karkain 1.1.0 is a mature *project* and an *immature* released artifact.**

Three findings drive the rating, in descending severity:

1. **The released `v1.1.0` tag ships a silent wrong-code defect (P0).**
   Bitwise operators `&`, `|`, `^`, `<<`, `>>` did not exist and were accepted
   and **silently mis-compiled**: a program using them compiled, exited 0 and
   printed a wrong answer — no diagnostic, no non-zero status. This is verified
   directly against the tag, not merely read from a report (§3.1). It was
   inherited by a release publicly labelled **Stable**.
2. **1.1.0 is not distributable as a release.** The tag exists, but the
   documentation still carries `:planned:`Release cut pending`` and the Docker
   image is `Planned`. Source-build is the only working install path (§5).
3. **The post-1.1.0 line is the strongest engineering evidence in the
   repository.** 68 commits of baselined, gate-verified work closed a *shipped*
   soundness hole before building anything on top of it (151P0), and closed the
   honesty gap in a feature that had been *shipping silently* (151D-first). The
   project's own process found and fixed defects that its release label would
   have concealed.

Maturity by dimension (§6):

| Dimension | Rating | One-line basis |
|---|---|---|
| Language core | **Mature** | 64 conformance tests, 61-golden corpus, both engines |
| Toolchain / CLI | **Mature** | 45 CI gate steps, 231 test files |
| Self-hosting (kcc) | **Advanced / partial** | default engine; 151A value model still open |
| Native backend | **Mature (Go-owned)** | ELF/PE/Mach-O, executed on PE + ELF |
| C-free / Go-free build | **Not achieved** | `kccOwnsNativeTargets = false`; stage-1 pinned to Go |
| Distribution | **Pre-release** | tag cut, archives not published |
| Docs accuracy | **Needs correction** | 4 verified drifts, incl. a false "no implementation exists" |
| Security posture | **Adequate process, no model** | `SECURITY.md` present; no capability model |
| Observability | **Mature (Go engine)** | `prof`, DWARF, `dbg` |

---

## 2. Method and evidence rules

* Baseline vs develop is distinguished by **git object**, not by prose:
  `git rev-list -n 1 v1.1.0` -> `951ee10`, dated 2026-09-25; `HEAD` ->
  `6162db4`, dated 2026-10-01; `git log v1.1.0..HEAD` -> 68 commits.
* Counts are **measured**, not quoted from prior reports: 231 `*_test.go`, 96
  `phase*_test.go` (69 in `pkg/cli`), 220 `examples/**/*.kark`, 16 `stdlib/`
  modules, 12 conformance files / **64** `func test_*`, 45 CI steps.
* Where a prior report and the tree disagree, **the tree wins** and the
  disagreement is reported (§7).
* The full/long test suite was **not** run, per the audit constraint. No claim
  below depends on a test run performed by this audit.

---

## 3. Released baseline: `v1.1.0`

### 3.1 P0 — bitwise operators silently mis-compiled (verified at the tag)

`docs/audit/PHASE-151P0-BITWISE-FINAL-REPORT.md` records the defect. This
audit independently confirms it **against the tagged tree**:

```
git grep -c 'TokenShiftLeft' v1.1.0 -- pkg/lexer   ->  no match
```

The symbol does not exist at the tag, so the operators could not have been
lexed. The report states the observed behaviour on the **Go reference engine**
(`597 & 21` -> `597`, `x >> 8` -> `0`, `1 << 4` -> `1`), and that the
self-hosted engine was worse — a single `&` was lexed as logical-and, so
`a & b` silently became `a && b`.

Four independent root causes are documented (lexer, parser, C codegen
fallthrough to `make_int(0)`, kcc lexer). **Severity is not theoretical**:
exit code 0 with a wrong answer defeats every downstream gate that checks
output, and it was present in a release labelled Stable.

**Fixed on `develop`** by increment 151P0 (`fc82806`), a *new baselined*
increment ordered ahead of 151B, with a gate asserting correct *values* rather
than cross-engine agreement — a byte-identity-only gate would have passed on
the defective code, which is the state it was found in.

> **Audit note.** This is the single most important maturity fact in the
> report: **the released baseline contains a wrong-code defect that the current
> tree does not.** Anyone evaluating 1.1.0 from the tag is evaluating a compiler
> that can return a wrong answer silently. The `1.0.x` LTS branch should be
> checked for the same class before being advertised for security fixes, since
> it predates the fix.

### 3.2 Released scope

`docs/release/KARKAIN-1.1-RELEASE-NOTES.md` documents increments 128-149:
stdlib freeze (132), first-class `fn` (133), incremental v2 (134), local
registry (135), LSP v2 (136), concurrency parity (137), accelerator kernels
(138), cross-compilation (139), debugger (140), optimizer/memory/profiling
(141), plus 125A (`std.net`/`std.http`/`std.db`) and 126 (`std.numerics`).

Stated compatibility is "every 1.0.0 program builds and runs identically" with
one behavior fix (K114 escape rejection). That claim is **plausible and
consistent** with the additive-scope evidence, but this audit did not execute
the battery that would prove it; it is recorded as documented-not-reverified.

---

## 4. Post-1.1.0 `develop` (68 commits)

The post-baseline work is **not** 68 independent features. It is one coherent
program, "Sovereignty I" (v1.2.0), plus two corrective slices:

| Group | Commits | Substance |
|---|---|---|
| **Increment 150** — native value model | ~22 | 150A (float64, arrays/`for-in`/`len`), 150B1 arena + concat, 150B2 string slice/compare, 150B3a `push`, 150B3b records, 150B3c maps, 150C register allocation, 150D Mach-O PIE + native OS targets + measured incremental refusal |
| **Increment 151P0** — bitwise soundness | 1 | `fc82806` (the P0 fix above) |
| **Increment 151** — kcc native parity | ~10 | 151D-first (silent-fallback closure), 151A-1, 151B encoder, 151C ELF, 151C2 Mach-O, 151C3 PE |
| **LH-1 / LH-2** — language hardening | 4 | kcc `?` propagation parity; kcc `match` binding parity |
| **Corrective** | ~6 | `karkain run` stdout parity; 32-bit build break; 3 native PE defects; Mach-O base 64-bit; CI job fixes |
| **Docs / process** | ~15 | baselines, audits, AGENTS records, a **withdrawn** claim |


### 4.1 Findings that show the process is real, not ceremonial

**(a) The silent Go fallback — a shipped dishonesty, found and removed.**
Increment 150 implemented the C-free native targets in Go while making kcc the
default engine, and `cmd/karkain/main.go` silently handed native targets off to
Go: a kcc-requested native build printed the **Go** lexer's banner and produced
a Go-identical image. Consequence: "parity" was untestable, because a byte
comparison passed with **zero lines of kcc native code**. Fixed by a seam that
**refuses** (`error[K116]`, exit 6, no image written) —
`pkg/cli/kcc_native.go`. This is the correct response: the defect was
*measurement*, not preference.

**(b) A cross-arch build break that made `main` red for three pushes.**
`MachoBase` was an untyped constant overflowing 32-bit `int`; `develop` and
`main` were **red on three consecutive CI runs** before the fix. Notably the
project's own first account of this was wrong — it claimed the break was
"invisible" to CI, and the run history disproved that. The record was
corrected rather than quietly inherited.

**(c) A claim withdrawn because it did not reproduce.** The 151P0 record
originally carried a claimed parity bug (zero-argument call dropped by kcc
codegen). It **does not reproduce**; the claim was **withdrawn, not filed**, and
the cosmetic artifact that misled it is explained. Recording a retracted claim
is the correct behaviour and is unusual enough to be called out.

### 4.2 What `develop` still does not do

* `const kccOwnsNativeTargets = false` (`pkg/cli/kcc_native.go:67`) — the three
  native targets are **Go-owned**; kcc refuses them.
* 151A is **open**: floats, arrays/`for-in`, arena, string ops, `push`,
  records, maps, and the `_start` stub / syscall tail / PEB bootstrap are
  unclaimed (`docs/audit/PHASE-151A-BASELINE.md` sections 9, 10.4).
* Stage-1 bootstrap is **pinned to the Go front end**
  (`pkg/bootstrap/bootstrap.go`, `forceGoEngine`). Increment 154 exists to
  retire this.
* No kcc-produced image **executes** anywhere; the parity argument is
  transitive.

---

## 5. Distribution: pre-release

| Artifact | State | Evidence |
|---|---|---|
| Tag `v1.1.0` | **cut** (2026-09-25) | `git rev-list -n 1 v1.1.0` |
| `VERSION` | `1.1.0` | `VERSION` |
| Install docs | **"Release cut pending"** | `docs/source/getting-started/installation.rst:32` |
| Docker `ghcr.io` | **Planned** | `installation.rst:113,176` |
| Published archives | **not evidenced** | no release artifact in tree |

The tag exists **while the install guide says the release is pending**. One of
those two is wrong; the documentation is the one a user reads first, so the
docs are the defect. This is an owner-ceremony item (tag + CI release job),
not an implementation gap — but it means **1.1.0 is not consumable by
download**, and build-from-source is the only working path.

Also note `installation.rst:41` states binaries ship the **self-contained Go
engine**, which is accurate and honest.


---

## 6. Capability matrix

Status vocabulary follows `docs/source/status/`: **Stable** (gate-proven),
**Experimental** (real mechanism, not stabilized), **Planned** (no code).

### 6.1 Language & core

| Capability | Status | Engines | Evidence |
|---|---|---|---|
| Variables, functions, control flow | Stable | Go + kcc | 64 conformance tests; 61-golden corpus |
| Structs, enums, `match` | Stable | Go + kcc | `docs/audit/PHASE-123-*.md` |
| Modules (`import`, `public`, qualified calls) | Stable | Go + kcc | `PHASE-103`, `PHASE-122` |
| Generics v1 (explicit `[T]`) | Stable | Go + kcc | `feature-matrix.rst`, 146A-D gates |
| Closures / `fn` values | Stable | Go + kcc | `PHASE-133` |
| Bitwise ops `& \| ^ << >>` | **Stable on develop, absent at tag** | Go + kcc | section 3.1 |
| `?` propagation (kcc parity) | Stable on develop | kcc | LH-1 gate `lh1_propagation_test.go` |
| `match` bindings (kcc parity) | Stable on develop | kcc | LH-2 gate `lh2_match_binding_test.go` |
| `const`, `func(T) R` type syntax | Planned | — | no gate |
| Enum payload destructuring | Documented gap | both | recorded in `PHASE-123` |
| Runtime exception system | Not authorized | — | checkpoint explicitly excludes |

### 6.2 Toolchain

| Capability | Status | Evidence |
|---|---|---|
| CLI surface (`build/run/test/check/fmt/lint/dbg/prof/target/pkg/lsp/...`) | Stable | 45 CI steps |
| `karkain fmt`, `lint` | Stable | `PHASE-82/86` |
| Diagnostics contract (`karkain-diagnostics-v1`) | Stable | `PHASE-83` |
| LSP (tokens, hover, definition, completion) | Stable | `PHASE-136` |
| Incremental compilation v2 | Stable | `PHASE-134` |
| Local package registry | Experimental | no remote registry, by design |
| Multi-error reporting | Stable | `PHASE-105` |
| DAP / source-level debugging | Planned | section 7.4 |

### 6.3 Targets

| Target | Status | Executed? |
|---|---|---|
| Native ELF (linux/amd64) | Stable (Go-owned) | **yes** (Linux CI) |
| Native PE (windows/amd64) | Stable (Go-owned) | **yes** (this host) |
| Native Mach-O (macOS PIE) | Stable **structurally only** | **no** — no Intel-mac runner |
| `wasm32-wasi` | Production candidate | wasmtime-gated |
| `riscv64-linux` | Parse + error paths only | no |
| `aarch64-windows`, `*-macos` | Cross-compile model | no runner |
| kcc native targets | **Refused (K116)** | no |

The PE/Mach-O distinction matters: PE claims are backed by execution on real
hardware; **every Mach-O claim in this repository has been structural since
increment 149**, and the 151C2 record restates that explicitly.

### 6.4 Library

16 `stdlib/` module directories exist. Importable and frozen on both engines:
`std.string`, `collections`, `io`, `encoding`, `crypto`, `testing`,
`numerics`, `generics`, `net`, `http`, `db`. Non-importable / unreleased:
`core`, `math`, `system`, `gpu`, `async` (mechanism present, no released
surface).

### 6.5 Observability & performance

`prof` (text/json/folded, deterministic counts), DWARF 4 emission + a
Karkain-owned reader, `karkain dbg` (live-gdb backtraces, Go engine), measured
RSS table. Strong for a v1.1 line; the incremental-native-cache question was
answered by **measurement** (11-53 microseconds/program) and a documented
**refusal** rather than an assumption — the correct call for that magnitude.


---

## 7. Documentation accuracy — 4 verified drifts

These are **defects**, not stylistic differences: in each case the docs
contradict the tree. Left uncorrected they actively mislead users.

### 7.1 `planned.rst` claims shipped features do not exist — most serious

`docs/source/status/planned.rst:15-21` states, for networking, databases and
the web framework:

> "Planned — **no socket or HTTP implementation exists**. No API surface is
> claimed." ... "Planned — no drivers, no query language..."

**The tree contradicts this directly**: `stdlib/` contains `net/`, `http/` and
`db/` modules; `roadmap.rst:58` records **125A** as shipping them;
`PHASE-125A` (via AGENTS) records ~27 net functions, 22 http and 24 db
functions, both-engine byte-identical, plus the Windows `-lws2_32` link
contract enforced across every generated-C linker.

A user reading the status page concludes a capability is absent. It is present
and gated. **This inverts the meaning of "Planned" as defined on the same
site** ("'Planned' means no code exists") — the one invariant the ledger most
needs to hold.

### 7.2 `experimental.rst` — two stale rows

* Lines 18-21: "Concurrency on the kcc engine ... codegen parity **is
  deferred**." **Phase 137 closed this** — kcc runs spawn/channel/actor
  programs byte-identically, and the concurrency examples were promoted to
  Stable/both.
* Lines 36-39: "GPU backend ... emits WGSL compute shaders internally but
  exposes **no** language surface... nothing selects or drives a GPU."
  **Phase 138 landed `@target(gpu)` emitting real WGSL** with a compile-only
  guarantee.

Both rows understate shipped capability in the conservative direction, which is
safer than the reverse — but they still send users down the wrong path.

### 7.3 `LANGUAGE-HARDENING-CHECKPOINT-FINAL-AUDIT.md` — stale status block

Lines 514-516 record `LH-1 ... AUTHORIZED / NOT STARTED` and
`LH-2 ... AUTHORIZED / NOT STARTED`. A dated update at line 526 corrects LH-1 to
`COMPLETE`, but **LH-2 is still recorded as NOT STARTED**, while commit
`38209cc` ("fix kcc match binding parity") and gate
`pkg/cli/lh2_match_binding_test.go` exist, and CI runs
`Run LH-2 kcc match-binding parity gate`. The authoritative authorization
record is therefore **wrong about work that is complete and gated**.

### 7.4 Release-state marker

`installation.rst:32` says the v1.1.0 release cut is pending; the tag exists
(section 5).

### 7.5 Not a defect — recorded for accuracy

* Root-level `kcc.exe`, `karkain.exe`, `karkain148.exe`, `check_isolated.out`,
  `poll.out` are present on disk but **untracked** (`git ls-files
  --error-unmatch` -> no match) and correctly ignored. Local workspace noise,
  not repository hygiene debt.
* `.gitignore` covers `.karkain-dev/` (the scratch tree the native/kcc gates
  build in).


---

## 8. Capability gaps, ranked

Ordered by user impact x confidence. Each names the evidence and the
increments that already own the work, so none of these is a new phase.

| # | Gap | Severity | Owner |
|---|---|---|---|
| **G1** | **Released `v1.1.0` mis-compiles bitwise ops silently** (exit 0, wrong answer) | **P0** | Fix is on `develop`; needs a **1.1.1 / re-tag decision** + `1.0.x` LTS check |
| **G2** | 1.1.0 not downloadable; install docs say "pending" while the tag exists | High | Owner ceremony + docs flip |
| **G3** | `planned.rst` says shipped net/http/db do not exist | High | Docs correction |
| **G4** | Not C-free: kcc emits C23; native targets Go-owned (`K116`) | High | 151A -> 152 |
| **G5** | Not Go-free: stage-1 pinned to `KARKAIN_ENGINE=go` | High | 154 |
| **G6** | Mach-O unproven (no runner); riscv64/aarch64 parse-only | Medium | 1.5.0 (166-172) |
| **G7** | No public registry (local-only by design) | Medium | 179, **only if operated** |
| **G8** | Stale status ledgers (`experimental.rst`, checkpoint audit) | Medium | Docs correction |
| **G9** | No security/capability model; no container image | Medium | roadmap / owner |
| **G10** | No DAP/source-level debugging; `prof`/`dbg` Go-engine-only | Low | 153 |
| **G11** | Unreleased stdlib modules (`core`, `math`, `system`, `gpu`, `async`) | Low | 1.3.0 |

### 8.1 What is deliberately *not* a gap

The project refuses work it cannot evidence, and that discipline is itself a
maturity signal:

* **The native-split incremental cache is a measured refusal**, not a TODO —
  pure-Go emission measured at 11-53 microseconds/program cannot repay a lookup.
* **The version plan says "Planned != baselined"**: v1.3.0+ carry themes only,
  and numbers are assignable until a baseline exists. This prevents the
  phantom roadmap inflation that makes maturity claims unverifiable.
* **`179` stays Planned unless a registry service is operated** — explicitly
  never asserted as shipped.

---

## 9. Working-tree state (audit hygiene)

The tree is **dirty at audit time** with uncommitted, un-gated Step-3 work
carried over from a superseded task:

```
 M cmd/karkain/main.go            (new native-value-loop dispatch)
 M pkg/cli/native_encode.go       (two new KCC subcommand wrappers)
 M src/compiler/main.kark         (subcommand dispatch)
 M src/compiler/native_value.kark (Step-3 loop shapes)
?? pkg/cli/zz_measure_loops_test.go
?? pkg/native/zz_measure_test.go
```

Per the audit-only constraint these were **not** modified, reverted or
committed. Two notes for the owner:

1. They are **not** part of any finding in this report, and were **not** gated.
2. `src/compiler/native_value.kark` currently produces loop bytes that
   **differ from the Go oracle** (`while_sum`, `for_sum`, `while_break`,
   `while_continue`, `nested_while` all differ). This is consistent with the
   baseline's own standard — a slice is not complete until its differential is
   byte-identical — so it must **not** be committed as-is.

Recommend discarding or stashing before further work.

---

## 10. Audit status

| Item | Status |
|---|---|
| Repository verified at `F:\Codes\Git\Karkain` | done |
| HEAD / tag / describe captured | done — `6162db4` / `v1.1.0`->`951ee10` |
| Released baseline vs post-1.1.0 develop distinguished | done — 68 commits (49 non-merge) |
| Capability matrix built from repository evidence | done |
| Gaps ranked with evidence | done — G1-G11 |
| Documentation drifts identified | done — 4 verified |
| P0 verified **against the tagged tree** (not a report) | done — section 3.1 |
| Implementation / refactoring / new phases | none |
| Full test suite | **not run** (per constraint); no finding depends on one |
| Files modified | only `docs/audit/KARKAIN-1.1.0-MATURITY-MATRIX.md` |
| Commits created | none |

**Recommended next actions (owner decisions, not agent work):**

1. Decide the response to **G1** — re-tag/re-release 1.1.x carrying 151P0, and
   check `1.0.x` LTS for the same class.
2. Correct the four documentation drifts (section 7). These are cheap,
   evidence-backed, and each currently misinforms a user.
3. Flip the install docs once the archives are actually published (section 5).
4. Complete or discard the uncommitted Step-3 work (section 9).

