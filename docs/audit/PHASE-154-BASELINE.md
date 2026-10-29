# Phase 154 BASELINE — I-6 Self-bootstrap (no-Go toolchain, C-free default path)

**Increment:** 154 (version **1.2.0**, "Sovereignty I — Off C, Off Go")
**Independence item:** **I-6** — toolchain independence (no Go anywhere on the build path; default user path C-free)
**Status at authoring:** baseline only. **No implementation.** Every claim below is in-tree evidence with a file/line, not a roadmap restatement.

Order is dependency-forced: **150 → 151 → 152 → 154** (`KARKAIN-VERSION-PLAN.md:157`). Increment 154 is the next queued increment after 152. `150` = native value model + native OS targets; `151` = kcc native parity (including 151D — kcc owns `native-x86_64-windows` and executes its own images); `152` = native stdlib + no-C closure (I-5); **154 = I-6**. `165` is free-floating CI/packaging work (Docker image), not a compiler dependency.

---

## 1. Objective

Shrink the compile-time surface of the Karkain toolchain: the default user path must not need the Go toolchain or a C compiler, and the self-hosted kcc must be able to build itself end-to-end. Concretely:

1. **Retire the `KARKAIN_ENGINE=go` stage-1 pin** in `pkg/bootstrap` — the `forceGoEngine` wrapper that forces the Go engine onto every compiler child the pipeline launches.
2. **Default engine of compiler children = the self-hosted kcc** (the default since Phase 97). The self-hosted compiler must compile and link itself: `kcc builds kcc`.
3. **Archive the Go tree** (`cmd/karkain`, `pkg/cli` Go drivers) as `reference/` at the repository root — off the user path, kept **only** as a differential oracle. It is never deleted.
4. **Gate:** with the Go toolchain absent from the build host, the self-bootstrap completes all stages yet and `stage2 == stage3` (SHA) bit-identical, with every compiler child running the default (kcc) engine.
5. **Full QA battery green in the no-Go state**: units, Phase-114 corpus on both engines, conformance, probes, every gate file, Sphinx `-W` + linkcheck, `verify-examples`, `install`/`verify-install`, `rc-journey`.

---

## 2. Current measured state (evidence from in-tree files, not assumption)

| # | Capability | State | Evidence |
|---|---|---|---|
| 1 | Stage-1 seed (`go build -o bin/karkain-stage1 ./cmd/karkain`) | present, required once per fresh tree | `pkg/bootstrap/bootstrap.go:202-206` |
| 2 | `KARKAIN_ENGINE=go` forced onto **every** compiler child across all stages | `forceGoEngine` in `runCmdOutputLabel` | `pkg/bootstrap/bootstrap.go:125, 149-151` |
| 3 | Stages 2/3 Go-free closure | proven with `go` shadowed (**Phase 143**, quarantined) — but the go pin still forces the Go engine on the compiler children | `pkg/bootstrap/phase143_seed_test.go:128-207` |
| 4 | kcc default engine | self-hosted kcc (Phase 97 onward) | CLI / `cmd/karkain/main.go` dispatch |
| 5 | Go tree location | `cmd/karkain`, `pkg/cli` source trees, tracked in the repo | repository root |
| 6 | Go-tree archival | none yet | `reference/` added by 154 |
| 7 | kcc-native targets | Go engine: `native-x86_64-windows` (151D, executed); none for a real user program on ELF/Mach-O (151A) | `pkg/cli/kcc_native.go` |

**Consequence worth stating plainly:** with the pin retired, the default engine of compiler children is kcc. The self-hosted kcc is therefore the only engine that must be able to emit and link its own compiler, and the Go tree is only a differential oracle. No language or target surface is removed by this increment.
## 3. Exit criteria (verbatim from `KARKAIN-VERSION-PLAN.md:212`, with the gate that proves each)

| # | Exit criterion | Proving gate |
|---|---|---|
| 1 | `kcc builds kcc`; **`stage2 == stage3` (SHA) with the Go toolchain absent from the build host** | `TestPhase154_SeedClosureNoGo` (pkg/bootstrap) |
| 2 | `KARKAIN_ENGINE=go` **stage-1 pin retired** (no compiler child forced the Go engine) | `TestPhase154_NoGoEnginePin` (pkg/bootstrap) |
## 5. Non-functional requirements

| ID | Requirement | Measured now | Target |
|---|---|---|---|
| NFR-15 | Toolchain independence (no Go; no C on the default path) | stages 2–3 go-free proven (143 gate); C still required for native links | no Go anywhere; no C compiler on the **default** path | 152, 154 |
| NFR-2 | Reproducible builds | stage-1 byte-reproducible (SHA `0c01de43…`) across CWDs; tamper divergence detected | same for the no-Go bootstrap + every packaged artifact | 152, 165, 166–168 |
| NFR-1 | Engine determinism (byte-identical Go↔kcc) | 61 corpus goldens + 64 conformance tests byte-identical | zero divergence on every Stable row, incl. native | 151 |
| NFR-3 | Memory footprint (4 GB host) | kcc check 544.5 MB · `go build` 229.9 MB · gcc big-TU 484 MB · guard threshold 1536 MiB | full no-Go self-build ≤ 2.5 GB | 162 |

## 6. Scope and boundaries

## 7. Implementation notes (filled during the increment, before any gate is written)

1. `pkg/bootstrap/bootstrap.go`:
   - Introduce a package-level pin knob (`goEnginePin`, default `true`) and a `RetireGoEnginePin()`/`GoEnginePinRetired()` pair; **`runCmdOutputLabel` applies `forceGoEngine` only while the pin is active**. Verify by mutation: re-enabling the pin unconditionally fails `TestPhase154_NoGoEnginePin`.
   - `RunStage1` keeps the `go build` seed step (the seed build is the 143 ceremony, not 154's work), but stages 2 and 3 must launch the compiler children with the **default** engine — i.e., with the pin retired and `KARKAIN_ENGINE` unset, so the self-hosted kcc drives the transpile.
2. `pkg/cli/kcc_native.go` / dispatch: assert, at CLI/CI level, that the default engine for a user program is the self-hosted kcc (the 150–151 paths are the proof of concept).
3. Archive the Go tree into `reference/` (root of the repository), `.gitignore`-covered, with a `reference/README.md` naming it the differential oracle (Go CLI + Go-engine C) — a differential, never a replacement.
4. New gate `pkg/bootstrap/phase154_selfbootstrap_test.go` (see §4) + CI step under the `Test` job.
5. Regression audit: full `go test ./...`, `go vet`, `pkg/native`, `pkg/parser`, `pkg/lexer`, `pkg/sema`, `pkg/codegen`, Phase 114 corpus both legs, Phase 122 KIRContinuity, Phase 123/124/135 gates, Sphinx `-W`, `verify-examples`; version identity (VERSION, both Go banner sites, three kcc banners, generated headers, LSP, every pinning test).
6. Audit note written; commit `develop` → merge `main` → push; per the branch rule.

**Amendment discipline:** a version is not closed as "green except X". Any new idea in this increment goes to the next version or an explicit carry-over list.

**In scope for 154:** pin retirement; default-engine wiring; `reference/` archival; the `TestPhase154_*` gate; the CI step; the phase-154 audit note; a no-Go bootstrap regression suite in `pkg/bootstrap`; the phase-154 version-identity and regression audit note.

**Out of scope for 154 (carried or owned elsewhere):** deleting the Go tree (keeps `reference/`); the C path (`--target c23`) — always stays; the standalone whole-toolchain bootstrap without Go (154 proves **stages 2–3** go-free, not a zero-Go build of the whole repository); 152's no-C stdlib closure (done by 152-A/B0/B1/B2); 165 (Docker image); the kcc ELF/Mach-O user-program emitters (151A).

**Stated, not papered over:** without a C compiler on the default path the native target is limited to the kcc Windows emitter (151D). The Linux/macOS native paths still need the value-model completion (151A) before their default-path proof can be claimed. The Go tree is the differential oracle for both engines for the lifetime of the repo.

| 3 | Go tree **archived as `reference/`** differential oracle, off-path, never deleted | `TestPhase154_ReferenceArchive` (pkg/bootstrap) |
| 4 | **Full QA battery green in the no-Go state** (units, Phase-114 both engines, conformance, probes, every gate, Sphinx `-W` + linkcheck, verify-examples, install/verify-install, rc-journey) | version-close ceremony (owner-verified; the gate step below runs on capable hosts) |
| 5 | `--target c23` and all other targets unchanged; the C path stays a first-class option | existing `--target c23` gates; 154 gate values asserted per §4 |

## 4. Gates (each is a test file **plus** its CI entry — AGENTS.md "gate is a test file plus a CI entry")

- `pkg/bootstrap/phase154_selfbootstrap_test.go`
  - `TestPhase154_PinMechanism` — **the fast half, and the only part that runs in the required `Test` job.** Proves the contract 154 actually changes (whether `KARKAIN_ENGINE=go` reaches a compiler child's environment) in both pin states, using a probe child and `CompilerChildEnv()`. No compiler is built, so the required job reports a real conclusion rather than a skip. It is deliberately not a substitute for the end-to-end proof.
  - `TestPhase154_ControlPinActive` — control: with the pin still active, all three stages build and `stage2 == stage3`, and the child env carried the Go pin. Opt-in behind `KARKAIN_SEED_CLOSURE=1`.
  - `TestPhase154_PinRetired` — the 154 main assertion: with the pin retired, stages 2 and 3 run the default self-hosted kcc engine and `stage2 == stage3` (SHA) bit-identical. Opt-in behind `KARKAIN_SEED_CLOSURE=1`; the `phase154-closure` discovery job runs exactly this test.
  - `TestPhase154_NoGoEnginePin` — the direct contract: after retirement no compiler child's environment carries `KARKAIN_ENGINE=go`. Opt-in behind `KARKAIN_SEED_CLOSURE=1`.
  - `TestPhase154_ReferenceArchive` — asserts `reference/` exists, is gitignored, and documents its role as the differential oracle.
- CI steps (named explicitly, not incidentally picked up): `.github/workflows/ci.yml` — "Run Increment 154 self-bootstrap gate" (`go test ./pkg/bootstrap/ -run 'TestPhase154_' -count=1 -timeout 60m`, in the required `Test` job) plus the non-gating `phase154-closure` discovery job that sets `KARKAIN_SEED_CLOSURE=1` and treats a skip as a failure.
- **No-Go full QA battery** (criterion 4) is verified by the owner at the version-close ceremony — the same arrangement as the 152-A no-C closure gate on the Linux host, and the B2 seed-closure job on capable runners.

**Corrected 2026-10-29.** The list above previously named a `TestPhase154_NoGoQACheck` that does not exist, described `TestPhase154_NoGoEnginePin` as running with `go` shadowed (it does not; the shadowing proof is `TestBootstrap_SeedClosure`), and named one CI step where there are now two. Recorded rather than quietly rewritten, because a gate list that names a non-existent test is how a gate comes to be believed without ever running. §8 has the measurement that exposed this.

---

## 8. Corrective record — the gate had never executed in CI, and cannot complete on a hosted runner (2026-10-29)

**This is a corrective change against the committed 154 baseline (`62a6323`), not a new increment and not a reopen of its scope.** Increment 154's implementation — the pin retirement, `SetGoEnginePin`/`RetireGoEnginePin`/`GoEnginePinRetired`, `compilerChildEnv`/`CompilerChildEnv`, the gate file, the CI step and `reference/` — shipped and is in the tree. What follows is a measurement about **where the proof can run**, plus the harness repair that follows from it.

### 8.1 The measurement

1. **The gate had never executed.** CI run `40815666047` (`develop` = `fb7cbf6`) gave the step "Run Increment 154 self-bootstrap gate" its **first execution since it was added on 2026-10-12**. Every run in between died earlier — at what is now step 16 — and skipped the rest: run `38606695556` records step 16 `failure` and steps 17–44 `skipped`, and the 154 step sits at index 54. So the step's green/red history before 2026-10-29 is empty, not green.
2. **That first execution failed, and not for a compiler reason.** The check-run annotation reads `Process completed with exit code 143.` The step started `04:15:03Z` and completed `04:16:53Z` — **110 s** — i.e. ~35–60 s into the stage-2 transpile. Exit 143 is SIGTERM.
3. **That signature is already recorded in this repository.** `PHASE-143-FINAL-REPORT.md` §F records five consecutive GitHub-runner deaths ~35 s into the same stage-2 transpile, exit 143, with no witness line even unbuffered and no timer/OOM/crash fingerprint anywhere in-repo, while the short smoke proof passes on the same runner. `REL-002` / `B2` is the same limitation and the `bootstrap-closure` job exists because of it. **154's failure is that class, not a defect in 154.**
4. **The cost of leaving it there is larger than one red step.** Steps 55–78 — 24 further gates, including the whole 151A/151B/151C/151D native family, 152-A/B1/B2, D5 and D8 — were `skipped` behind it and have still never executed in CI. A gate that reports a runner limit as a compiler failure therefore also hides everything behind it.

### 8.2 What changed

- **The three-stage tests are opt-in**, via a new `requireSeedClosure(t)` helper that keys on the **same** variable and states the **same** reason as Phase 143's seed closure (`KARKAIN_SEED_CLOSURE=1`). It is the same measured limitation, so it is not a second, weaker rule.
- **A fast half runs in the required job.** `TestPhase154_PinMechanism` proves the contract 154 actually changes — whether `KARKAIN_ENGINE=go` reaches a compiler child's environment — by launching `go env GOOS` in both pin states and inspecting `CompilerChildEnv()`. Neither half builds a compiler, so the required `Test` job now reports a real pass instead of a skip.
- **Witness lines were added** to the three heavy tests (`phase154 witness: pin RETIRED, stage2 == stage3 bitwise identical (N bytes)`, plus the control and no-go-pin variants) so a completion is greppable rather than inferred from an exit code.
- **A non-gating `phase154-closure` discovery job** was added to `.github/workflows/ci.yml`, mirroring `bootstrap-closure`: it sets `KARKAIN_SEED_CLOSURE: '1'`, runs **only** `TestPhase154_PinRetired` (the main assertion) with `tee` + `pipefail` so an external kill still leaves a partial log, never sets `GOMEMLIMIT` (one test, and the exit-143 diagnosis should carry no extra unknown), and has a verification step that treats a **missing log, a SKIP, a FAIL, or a witness line with a zero byte count** as failures.

### 8.3 Evidence for the harness itself

- **Mutation M1** — replace the pin's effect with a no-op (`_ = cmd`): `TestPhase154_PinMechanism` fails with `pin active: compiler child env does not carry KARKAIN_ENGINE=go`. Reverted.
- **Mutation M2** — apply `forceGoEngine(cmd)` unconditionally, ignoring the pin state: the same test fails with `pin retired: compiler child env still carries KARKAIN_ENGINE=go`. Reverted. `bootstrap.go` was then restored and its SHA-256 verified as `D6790736A94A99DAE5D569800D430064738DDE27CF336D17072212193D717191`, the pre-mutation content.
- **The CI verification block was executed, not just written**: five synthetic logs (good / skip / fail / 0-byte witness / no witness) were fed to it under bash; only the good one exits 0, and the 0-byte case is rejected by the witness regex.
- Locally `go test ./pkg/bootstrap/ -run 'TestPhase154_' -count=1 -v` exits 0 with `TestPhase154_PinMechanism` PASS (0.09 s), `TestPhase154_ReferenceArchive` PASS, and the three heavy tests SKIP with the honest message. `go vet ./pkg/bootstrap/` clean.

### 8.4 What is NOT claimed

- **v1.2.0 closure conditions 5–6 stay OPEN.** Nothing here proves that kcc can rebuild itself end-to-end; it proves that the pin contract is real and that the end-to-end proof is waiting for a host that can run it.
- **`TestPhase154_ControlPinActive` and `TestPhase154_NoGoEnginePin` remain unproven.** They each drive a full three-stage build, so running them alongside the main assertion would triple a burn the runner already cannot finish once. They are recorded as unproven rather than implied green by their sibling's pass.
- **The discovery job is not a release gate.** It is not a required check, so it may report red without blocking; a green there is the only thing that would close the claim.

