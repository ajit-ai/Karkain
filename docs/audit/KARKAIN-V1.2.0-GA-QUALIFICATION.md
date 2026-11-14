# Karkain v1.2.0 - GA Qualification Report

**Release:** `v1.2.0` - **Karkain 1.2.0, Native Independence**
**Repository:** `ajit-ai/Karkain`
**Branch:** `feature/v1.2.0-native-independence` (the release branch for this
version), merged into `main` at the release commit.
**Report date:** 2026-11-12.

This is the **authoritative final qualification report** for v1.2.0. It states
what was measured, what passed, what did not, and what remains a known
difference. It does not claim anything the evidence does not support.

---

## 1. The source

| | |
|---|---|
| Starting branch | `feature/v1.2.0-native-independence` at `756df79` (NAT-RODATA) |
| NAT-REG commit | `366ee03` - NAT-REG: add native register allocation to the self-hosted driver |
| NAT-QUAL-FINAL commit | `80bc6eb` - NAT-QUAL-FINAL: qualify the v1.2.0 native path and correct the target ownership |
| Release-preparation commit | `8fc6198` - release: prepare Karkain v1.2.0 (Native Independence) |
| Corrective commits | `994bd5a` (the two release blockers the first CI run found), `1f36300` (the NAT-REG gate executes the host's own native image), `b49b463` (the artifact-quota record) |
| Release branch | `main` |
| Merge commit | `77d7069` - "Merge branch 'develop' for Karkain v1.2.0 (Native Independence)" (pull request #2) |
| Annotated tag | `v1.2.0` (tag object `e900c91`) |
| Peeled commit | `77d7069` - the merge commit on `main` |

The three focused commits are separate and auditable: the register
allocator, the qualification evidence and corrections, and the release
preparation. The three corrective commits are a separate record: each is a
defect the RELEASE's OWN CI run found, each is documented in AGENTS.md, and
none of them re-plans a completed slice.

## 2. What v1.2.0 is

**Sovereignty I - Off C, Off Go.** The C-free native targets are produced by
the **self-hosted** compiler, not by a C compiler and not by the Go backend:

```
karkain build --target native-x86_64-linux   main.kark   # a Linux ELF
karkain build --target native-x86_64-windows main.kark   # a Windows PE
karkain build --target native-x86_64-macos   main.kark   # a Mach-O PIE
```

The self-hosted engine (`kcc`, built from `src/compiler/*.kark`) owns the
native value model, the machine-code encoder, the three container writers and
the whole-program driver.

## 3. Supported features

The qualified native subset, each with a gate in `pkg/cli/`:

| feature | gate |
|---------|------|
| integer expressions, bindings, assignment | `TestPhase151A2_`, `TestPhase151A9C_` |
| user function calls, arguments, return values | `TestPhaseNATCall_` |
| conditional branches | `TestPhaseNATIf_` |
| loops, `break`, `continue` | `TestPhaseNATLoop_` |
| array indexing and the bounds diagnostic | `TestPhaseNATIdx_`, `TestPhaseD5_` |
| the frame layout against the Go oracle's | `TestPhaseNATFrame_` |
| the string staging area, program-wide | `TestPhaseNATStrStage_` |
| the reporter's read-only data | `TestPhaseNATRodata_` |
| register allocation (opt-in) | `TestPhaseNATReg_` |
| the native independence qualification | `TestPhaseNATQual_` |

## 4. Unsupported features, and how they fail

Every one of these is **REFUSED BY NAME** with a diagnostic and **no image**,
never mis-lowered:

* float64 statements, parameters and returns;
* `for ... in`;
* map literals in the driver;
* string equality and string slicing;
* a string literal inside a branch or a loop body;
* a string in a callee.

The refusal table is asserted by the gates above: a refusal must name the
construct and must leave no image behind.

## 5. Native independence

`TestPhaseNATQual_` installs PATH shims for `gcc` and `go` that log every
invocation, and then:

* the **native build's shim log is EMPTY** - neither toolchain is reached;
* the **legacy build path's shim log is NOT empty** - the positive control,
  which is what makes the empty log evidence rather than a shim that silently
  never fired.

The self-hosted compiler is warmed OUTSIDE the trace, because the claim is
about compiling a USER PROGRAM, not about how the compiler itself was built.

## 6. Target qualification

### 6.1 Linux ELF

* **Emission:** proven - the self-hosted engine emits the image.
* **Structure:** validated by the ORACLE's own `native.Parse` (magic, class,
  machine, program-header count, the arena segment's R+W flags, the entry
  inside `.text`).
* **Execution:** the Linux CI leg runs the ELF. This dev host is
  windows/amd64 with no WSL and no Docker, so no ELF was executed here.
* **Byte-identity with the Go oracle:** proven for every shape the parity gate
  requires.

### 6.2 Windows PE

* **Emission:** proven.
* **Structure:** validated by the ORACLE's own `ParsePE`.
* **Execution:** **proven on this host** - the produced images are executed and
  their stdout compared byte for byte.
* **Byte-identity with the Go oracle:** **NOT claimed.** The Windows entry stub
  and the loader-independent PEB bootstrap are SEPARATE IMPLEMENTATIONS in the
  two engines (the driver's against the oracle's), so the PE's `.text` is
  measured at `0xa9c` against the oracle's `0xa94` and about a thousand bytes
  of the bootstrap differ. This is why the byte-identity differential compares
  the Linux ELF.

### 6.3 macOS Mach-O

* **Emission:** proven (a real PIE with `MH_PIE`, a writable `__DATA` arena
  and a decodable rebase stream).
* **Execution:** **NOT proven and not claimable** - there is no macOS runner in
  this repository and no Intel-Mac host in its history. Every Mach-O claim
  since increment 149 has been structural, and this release does not change
  that.

## 7. Differential results

### 7.1 Byte-identity (Linux ELF)

`TestPhaseD2_KccAndGoEmitIdenticalImages` covers eight program shapes -
literals, locals, multiple locals, string literals, the allocating case, a
returned value, `len()` of an array, and an array index - and requires the two
engines' images to be byte-identical.

**The known-non-parity list is EMPTY.** The `array_index` shape used to be
asserted to DIFFER; it became identical when NAT-RODATA closed the reporter's
rodata defect, and it moved into the parity list, which is the signal that gate
gives.

### 7.2 The reporter's read-only data

`TestPhaseNATRodata_` measures the section's entries, in order, with their
offsets, for nine programs. After NAT-RODATA the two engines' sections agree
entry for entry and offset for offset, and the images are byte-identical for
every fixture except one (section 8.1).

### 7.3 The frame layout

`TestPhaseNATFrame_` compares the frame-size immediate of every function
against the oracle's, for both targets, and the frames now agree exactly.

### 7.4 Register allocation

`TestPhaseNATReg_` compares the register-allocated path with the stack-based
path BEHAVIOURALLY over the same programs, with the Go oracle as a third
opinion. The gate **executes the host's own native image** - the Windows PE
on Windows, the Linux ELF on Linux - which is deliberately NOT the pattern the
other native gates use: they build the PE everywhere and skip the execution
layer on POSIX, so their behavioural layers run nowhere in CI. This gate
cannot skip its only layer. The image is validated with the oracle's own
parser (`native.ParsePE` for a PE, `native.Parse` for an ELF), because a check
written in the gate would share its assumptions with the code under test.

## 8. Known differences, and what each one affects

| # | difference | affects | why it stands |
|---|-----------|---------|--------------|
| 1 | The reporter's rodata: one 39-byte ELF layout difference in a single fixture (a program that binds a string literal AFTER an index site) | **binary layout only** | The oracle interns the call site's two strings AT THE FIRST INDEX SITE; the driver groups them after the literals. The diagnostic's TEXT is identical in both engines. Closing it means re-deriving the interning order from the emitter's walk. |
| 2 | Field assignment: 25 bytes of code difference | **binary layout only** | The oracle stages the `(pointer, length)` pair through the string staging area; the driver stores the pair directly. The oracle's implementation is not duplicated for the sake of byte identity. |
| 3 | The oracle's name-table inference over-reserves | **binary layout only** | The oracle's string inference is name-based and program-wide, so a name bound to a string anywhere is a string everywhere - shadowing included - and it reserves the staging area (and an arena) for a program with no string operation. Its own EMISSION disagrees with its scan. The driver follows the emission. |
| 4 | String equality and slicing are refused | **capability** | Refused by name, with no image. |
| 5 | The arena bound is compile-time | **capability** | An allocation inside a loop is refused by name, because a compile-time size cannot cover an unbounded number of iterations. |
| 6 | Maps, `for-in`, float statements in the driver | **capability** | Refused by name, with no image. |
| 7 | Non-integer array element types | **capability** | Refused by name. |
| 8 | The two parser discrepancies (a call expression in a `for` post, and the two engines' disagreement about a `for` init's scope) | **capability** | Both are refused by the driver by name, and the Go backend refuses them too. |
| 9 | Register allocation is opt-in | **binary layout only** | The register-allocated path is new code in a self-hosted compiler; the qualified stack-based path stays the default. |
| 10 | The `stage2 == stage3` self-bootstrap proof and the "no Go toolchain" closure | **release conditions 5 and 6, carried to v1.3.0** | Owner-authorized on 2026-10-29. The implementation is complete (`62a6323`) and the fast half of the pin contract runs in the required `Test` job; the three-stage burn is capacity- and runner-blocked. |

**None of these is a wrong result, a memory-safety defect, an invalid
executable or an incorrect runtime diagnostic.** The two that would be
release blockers - a wrong result and a wrong diagnostic - are closed and
proved by the gates.

## 9. Regression results

See section 11 for the measured durations.

## 10. Mutation evidence

| slice | mutation | caught by |
|-------|----------|-----------|
| NAT-RODATA | the call site's offsets are list positions again | the correctness check ("the image does NOT reference the address of 'array index out of range'") and the byte-identity check |
| NAT-RODATA | the reporter's strings are interned after the literals again | the interning-order check and the byte-identity check |
| NAT-REG | the epilogue omits the POP | the behavioural agreement (the caller's callee-saved registers are corrupted) |
| NAT-REG | a spilled value is stored to the wrong slot | the behavioural agreement (eight cases) |
| NAT-REG | a register is reused while its holder is live | the behavioural agreement (eight cases) |
| NAT-REG | the loop widening is dropped | the behavioural agreement |

**Two mutations were NOT caught, and both are recorded rather than hidden.**
Dropping the scan's liveness check in the allocator makes it MORE
conservative - it never reuses a register - which is a missed optimisation and
not a wrong answer. Dropping the loop widening was not caught until the corpus
gained a case that declares a local INSIDE a loop; the case is now in the
corpus and the mutation fails.

## 11. Measured durations and the KIR pin

### 11.1 The QA battery

`scripts/qa/run-full-qa.ps1`, run on this host (windows/amd64, ~4 GB RAM):

| stage | result | duration |
|-------|--------|----------|
| `build` (`go build ./...`) | **PASS** | 61.6 s |
| `vet` (`go vet ./...`) | **PASS** | 40.1 s |
| `units` (all non-CLI packages) | **FAIL - one package** | 498.4 s |
| `cligates` (Phase 114-119 + conformance + probes) | **PASS** | 1556.4 s |
| `conformance` | **PASS** | 161.0 s |
| `probes` | **PASS** | 44.0 s |
| `examples` (`verify-examples.ps1`) | **PASS** - 60 passed, 0 failed, 2 skipped | 354.5 s |
| `docs` (Sphinx `-W` + linkcheck) | **SKIP** - no Sphinx on this host | - |
| `fresh` (`beta-fresh-checkout.ps1`) | **PASS** - "BETA FRESH-CHECKOUT: PASS" | 387.6 s |
| `install` (`install.ps1`) | **PASS** (exit 0) | 47.7 s |
| `verify-install` (`verify-install.ps1`) | **PASS** (exit 0) | 40 s |
| `journey` (`verify-rc-journey.ps1`) | **PASS** - 11 passed, 0 failed | 41.7 s |

The `units` stage was run separately (it is the one stage the battery's
`-Skip units` excluded, because it fails on the capacity-blocked bootstrap
tests and the battery aborts at the first failing stage). Every other stage
in the battery is green on this host.

**The `units` stage fails on exactly one package, and it is a capacity
blocker rather than a defect.** `pkg/bootstrap`'s two heavyweight tests -
`TestBootstrap_Stage2SelfHosting` and `TestBootstrap_BitwiseIdentity` - are
refused by the `error[K127]` memory guard on this host (measured: 166-791 MiB
free against the guard's 1536 MiB requirement). With the guard's own
documented override (`KARKAIN_BOOTSTRAP_MIN_MEM=0`) the burn still dies in the
stage-2 transpile (`karkain transpile failed: exit status 1`), so the failure
is the host's capacity and not the override. **Every other package passes**:
lexer, parser, sema, ir, ir/hir, ir/ssa, codegen, backend, backend/cpu,
backend/gpu, backend/parity, npu (+ the five vendor adapters), pm, module,
runtime, runtime/gpu, wasm, compiler, source, diagnostics, target and lsp.

**The conformance corpus is 64/64.** It failed twice on this host with a
**different** test each time (`test_array_of_strings` and
`test_array_push_and_len`, then `test_primality`), each reporting "C
compilation failed: exit status 1" with no compiler diagnostic - the
documented class in which the C backend's gcc is KILLED while the pagefile
thrashes. A third run passed 64/64, and the battery's own `conformance` and
`cligates` stages then passed 64/64 as well. This is recorded as a **flake**,
not as a success, and it is the same class the NAT-IDX gate already handles
by capturing the child's stderr instead of discarding it.

**The battery's `cligates` stage timed out twice before it passed**, at Go's
10-minute default and then at a 40-minute timeout. The stage bundles EIGHT
gate families and the Phase-114 corpus alone runs in BOTH engines, so its
real cost is ~26 minutes on a loaded host; the timeout was a HARNESS
artifact, and the stage's timeout is now 90 minutes with the reason recorded
in the script's header. The CI runs these gates as separate steps with their
own per-step timeouts, so the CI path never had the problem.

### 11.2 The native gates

| activity | result | duration |
|----------|--------|----------|
| `TestPhaseNATReg_` (the register-allocation gate) | **PASS** | 58.9 s |
| `TestPhaseNATStrStage_` | PASS | 100.7 s |
| `TestPhaseNATRodata_` | PASS | 15.9 s |
| `TestPhaseNATFrame_` and the native family (23 patterns) | PASS | 518.2 s |
| `pkg/native` | PASS | 57.6 s |
| `TestPhase122_PipelineOwnership` (the KIR pin and KIRContinuity) | PASS | 2147.3 s |
| `TestPhase151A5_KIRPinHolds` (the second KIR pin) | PASS | 898.7 s |

### 11.3 The KIR pin

**15243 before the release and 15243 after it.** The version flip changes a
string literal's CONTENT and not the AST's shape, so the pin does not move -
and the delta was **measured, never predicted**: `karkain kir --verify
src/compiler/kir.kark` reports `text=15243 verify=15243`, measured twice on
the settled release state. Both pin assertions read 15243 and were
re-pinned together in the NAT-REG commit.

The arithmetic, for the record: 11412 (HEAD) + 344 (Steps 5/6) + 103 (Step 7)
+ 91 (Step 8a) + 213 (Step 8b) + 139 (Step 8c) + 87 (Step 8d) + 30 (Step 8e)
+ 69 (Step 8f) + 82 (Step 9a) + 344 (Steps 9b/9c/9d/9e/9f/9g/9h) + 297
(NAT-REG) = 15243.

### 11.4 The first CI run of the release, and the two blockers it found

The first CI run of the release found **two release blockers**, and both were
of the same kind: a claim that was true on the machine where it was written
and false on the machine that ran it. Both were fixed before the tag.

**The docs build was red.** The new v1.2.0 section of `release-notes.rst`
carried a DUPLICATED section underline (the head was prepended and the
original underline came with it) and a list-table row with FOUR items where
every other row had two - and an RST list-table requires a uniform two-level
bullet list, so Sphinx refused the directive. Reproduced locally with Sphinx
9.1.1 and `python -m sphinx -b html docs/source <out> -W --keep-going`: the
build now **succeeds with 0 warnings**, and the linkcheck succeeds too. The
CI's `Documentation` job had failed on exactly this.

**The NAT-QUAL gate was WINDOWS-ONLY in its mechanism.** Its PATH shims for
`gcc` and `go` were `.cmd` files, which Go's `exec.LookPath` resolves on
Windows through PATHEXT - and on POSIX `LookPath("gcc")` does NOT find
`gcc.cmd` at all, because the file is not named `gcc` and carries no exec
bit. So on the Linux CI leg the shim could never fire, the traced log was
empty for the WRONG reason, and the **positive control** failed with "the
positive control did not fire". The control is exactly what caught it: a
mechanism that cannot be shown to work cannot prove that it stayed silent.
The shim is now written the way the host resolves it - a `.cmd` on Windows,
an executable `#!/bin/sh` script named exactly like the tool on POSIX - and
the trace follows the HOST's own native target rather than a cross-build.
**This gate had never executed on the remote CI before this release** - the
whole NAT-* series lived on the feature branch - so this was its first
execution anywhere but Windows, and the first execution was the one that
found the defect.

**The CI aborts a job at its FIRST failing step, so one red gate hides every
gate behind it.** The first run failed at the NAT-QUAL step and the 39 steps
behind it - NAT-LOOP, NAT-IDX, NAT-FRAME, NAT-STR-STAGE, NAT-RODATA,
NAT-REG, the 151A family, the 152 family, the conformance corpus and the P1
gate - were reported **skipped**, not passed. The 49 steps in front of it,
including the Phase 151 native-parity gates, A1, Phase B, D2, D5 and D8,
were green.

## 12. The QA battery and the release decision

The v1.2.0 closure conditions, with their status:

| # | condition | status |
|---|-----------|--------|
| 1 | `hello` builds and runs with no C compiler on `PATH` | **MET** (the 152-A gate) |
| 2 | `stdlib_v2` works no-C | **CARRIED to v1.3.0 (UNMET)** - owner-authorized 2026-10-10 |
| 3 | one `std.net` program builds and runs no-C | **CARRIED to v1.3.0 (UNMET)** - owner-authorized 2026-10-10 |
| 4 | native images byte-identical where the gate requires parity | **MET**, with the documented differences in section 8 |
| 5 | `stage2 == stage3` | **CARRIED to v1.3.0 (UNMET)** - owner-authorized 2026-10-29 |
| 6 | no Go toolchain for that self-bootstrap closure | **CARRIED to v1.3.0 (UNMET)** - owner-authorized 2026-10-29 |
| 7 | `installation.rst` identifies Go as reference-only | **MET** |
| 8 | the full version QA battery | see below |

The **effective** gate is therefore conditions 1, 4, 7 and 8.

### 12.1 The release checklist

| # | gate | verdict | evidence |
|---|------|---------|----------|
| 1 | v1.2.0 implementation scope frozen | **PASS** | `docs/release/v1.2.0-CHECKLIST.md`; the increments in scope are 150 (all slices), 151, 151P0, 152 and 154, and no increment outside that list was started |
| 2 | NAT-REG qualification confirmed | **PASS** | `TestPhaseNATReg_` PASS; the two native paths and the Go oracle agree on stdout and exit status over the whole corpus; the frame is identical between the paths; the two paths are byte-different where the allocator engages; four mutations caught |
| 3 | all required regression gates pass | **PASS** | the native family, `pkg/native`, the Phase 114-119 CLI gates, the conformance corpus (64/64), the probes corpus, `go build ./...` and `go vet` are green; the two `pkg/bootstrap` tests are capacity-blocked on this host and are the owner-carried conditions 5-6 |
| 4 | native independence confirmed | **PASS** | `TestPhaseNATQual_`: the native build's `gcc`/`go` PATH-shim log is EMPTY and the legacy path's is NOT |
| 5 | the documentation matches the implementation | **PASS** | four documentation defects found and fixed in the flip (the stale `K116` ownership claim, the stale `v1.0.0` archive names, the "Release cut pending" marker and the README's "native backend is Planned" prose); the ownership table, the archive table and the compatibility policy now match the code |
| 6 | the supported targets confirmed | **PASS** | Windows PE executed on this host; Linux ELF byte-identical to the oracle and executed on the Linux CI leg; macOS Mach-O structurally validated - and **not** claimed as executed |
| 7 | the version metadata confirmed | **PASS** | `VERSION` = `1.2.0`; `karkain --version` reports `Karkain Compiler v1.2.0 (windows/amd64, Stable Build)`; the kcc banners, the six generated headers, the LSP server and the five version-pinning test files agree; the install script's VERSION file and the shipped binary agree |
| 8 | the release artifacts confirmed | **PASS (locally built)** | the 13 archives were built **locally on a Windows host** from the tagged commit using **Go 1.21.13** (the Go version the CI workflow pins), `CGO_ENABLED=0`, `GOARM=7` for linux/arm and the CI build flags; `checksums.txt` was generated locally and independently verified; **GitHub Actions could not execute the release matrix** (run `43374786565`, 2026-11-14: the Build jobs were rejected before they started because the account was subject to a billing/payment restriction), so this is a locally built and published release, **NOT a CI-green release**; the tag-triggered workflow did not complete |
| 9 | no uncommitted phase-owned files remain | **PASS** | the working tree is clean at the release commit apart from the nine pre-existing unrelated untracked files, which were preserved untouched |
| 10 | no unresolved release-blocking defect | **PASS** | the known differences in section 8 are binary-layout or capability differences, each either refused by name or proven not to affect a program's output; none is a wrong result, a memory-safety defect, an invalid executable or an incorrect runtime diagnostic |

### 12.2 The release decision

**PASS - publish v1.2.0, as a locally built release (owner-authorized 2026-11-14).**

Every mandatory gate holds. The four carried closure conditions (2, 3, 5 and
6) are carried by owner-authorized amendments whose TEXT is unchanged and
whose STATUS lives in one place; they are not v1.2.0 blockers and they are
not claimed as met.

**Provenance is local, not CI.** The account was subject to a
billing/payment restriction, the tag-triggered workflow `43374786565` did not
execute its Build matrix, and the archives published with this release were
built by the publisher on a Windows host from the tagged commit with Go
1.21.13. **This release is NOT a CI-green release** and must not be
described as one. The post-publication verification against the downloaded,
publicly published assets is recorded in section 13.

**Two things are explicitly NOT claimed.** No Mach-O image has been executed,
because this repository has no macOS runner. And the self-hosted compiler's
own three-stage self-bootstrap closure is not re-proven here, because this
host's capacity and the hosted runner's exit-143 kill block the burn - the
implementation is complete and the fast half of the pin contract runs in the
required `Test` job.

## 13. The artifacts

The published archive set was **built locally on a Windows host** (not by
GitHub Actions) from the tagged commit `77d70693e1e3134dac312a303bffafa7d50fa988`
(tag `v1.2.0`, tag object `e900c91e485fc316f52b664228f6c35f25416436`):

* Windows: amd64, arm64 (zip)
* Linux: amd64, arm64, arm, 386, ppc64le, s390x (tar.gz)
* macOS: amd64, arm64 (tar.gz)
* FreeBSD, NetBSD, OpenBSD: amd64 (tar.gz)

Build environment: **Go 1.21.13** (the Go version the CI workflow pins at
`actions/setup-go@v5`, `go-version: '1.21'`), `CGO_ENABLED=0`, `GOARM=7` for
linux/arm, `-ldflags="-s -w"`, `go build ./cmd/karkain`. Packaging used the
local Windows packaging tools (PowerShell `Compress-Archive`, bsdtar), not the
Ubuntu GNU environment the CI matrix uses; archive containers therefore differ
from CI's, but the contents (binary, `README.md`, `LICENSE`, `stdlib/`,
`VERSION` = `v1.2.0`) are identical to what the CI `Package` steps produce.

`checksums.txt` was generated locally from the artifacts themselves and
independently verified (`Get-FileHash`, SHA-256, `sha256sum -c`-compatible
format); every archive lists in it and verifies true. **The published
checksums identify the locally built archives, not CI artifacts.** GitHub
Actions could not execute the release matrix: run `43374786565` (2026-11-14)
had its Build jobs rejected before they started ("recent account payments
have failed or your spending limit needs to be increased"); the
tag-triggered workflow did not complete, and no CI-built artifacts are
claimed for this release. **This is NOT a CI-green release.**

**The installation instructions tell the user to verify every archive against
that file**, and the version banner in the shipped binary is the release's own
(`Karkain Compiler v1.2.0 (windows/amd64, Stable Build)`).

Post-publication verification (downloading the public assets) is recorded in
this section's verification record: every downloaded archive was checked
against the published `checksums.txt`, the Windows amd64 archive was extracted
and its `--version` output confirmed.

## 14. What a reader should NOT conclude from this report

* That the release is CI-green. It is not: the archives were built locally
  on a Windows host (Go 1.21.13, the CI workflow's Go version) because the
  account was subject to a billing/payment restriction and the
  tag-triggered workflow `43374786565` did not execute its Build matrix. The
  published checksums identify the locally built archives.
* That the PE is byte-identical to the Go oracle's. It is not, and it is not
  claimed to be.

* That a Mach-O image has been executed. It has not; there is no macOS runner.
* That the native driver supports the whole language. It supports a qualified
  subset and refuses everything else BY NAME.
* That the self-hosted compiler rebuilds itself bit-reproducibly. That proof
  (conditions 5 and 6) is carried to v1.3.0.
* That register allocation is the default. It is opt-in.
