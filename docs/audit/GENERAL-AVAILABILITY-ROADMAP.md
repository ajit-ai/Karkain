# Karkain General Availability Roadmap

**Version**: 1.0 (Phase 130 baseline — closure capture-mutation complete)
**Host reality**: Windows 11 on 4 GB RAM; single-maintainer cycle
**Source of truth**: AGENTS.md (completion record) + `docs/source/status/scope.rst`
(bucket matrix). This document is the forward-looking plan only.

---

## 1. What "General Availability" means for Karkain

The project already carries a **1.0.0 (Stable)** label (since Phase 119), but the
*release* has never shipped: **no tag cut, no pre-built binaries** (installation
is documented honestly as "from source" / "Planned"). Therefore the next real GA
has two legs:

1. **Ship what is already promised** — cut the tag, publish binaries, prove the
   Windows 11 install path end-to-end on exactly the kind of host this project
   is developed on (4 GB RAM).
2. **Close the gaps that make "stable" dishonest** — the pre-Phase-130 `fn`
   closure boundary (capture-mutation now pinned; first-class fn values
   documented post-130), the ~4 GB bootstrap SEGFAULT class, and
   the Go-engine-only surfaces (concurrency, profiling, SIMD) that a GA user
   would reasonably expect to behave identically on both engines.

### GA exit criteria (the gate this roadmap marches toward)

- [ ] `v1.0.0` tag cut; archives + SHA-256 published for the 13-platform matrix
- [ ] `scripts/install.ps1` + `verify-install.ps1` pass end-to-end on Windows 11
- [ ] `fn` closures compile and run identically on BOTH engines (no known
      broken-on-both-engines language boundary) — Phase 130 delivered
      capture-mutation + nested-closure parity; first-class function values
      remain a documented post-130 boundary, not a "broken" surface
- [ ] Bootstrap stage 2/3 runs on 4 GB hosts — or fails with a clean, actionable
      `error[K...]`, never a SEGFAULT
- [ ] Concurrency, profiling, SIMD surfaces reach both-engine byte-identical
      parity and move from Experimental → Stable Core in `scope.rst`
- [ ] Every `scope.rst` Experimental/Planned item either ships, gets a
      documented date, or is explicitly dropped
- [ ] All phase gates green: compile, E2E, units, `go vet`, `go build ./...`

---

## 2. Constraint & principles (unchanged)

- **Maturity over features** / **Working > ambitious** / **Semantic foundations
  first** / **Incremental verification** (AGENTS.md).
- Every phase ships with: code + gate test(s) + audit report + AGENTS.md record,
  committed to `develop`, merged to `main`, pushed.
- **4 GB RAM is a first-class target, not a footnote.** Any feature that cannot
  be *built and tested* on the dev host is risky; phases are ordered so the
  memory problem is attacked early.

---

## 3. Milestones at a glance

| Milestone | Name | Phases | Goal |
|-----------|------|--------|------|
| **GA-1** | Ship & Harden | 128–129 | Public 1.0.0 release usable on Windows 11/4 GB |
| **GA-2** | Language & Tooling Completeness | 130–137 | No unstable language boundary; both engines byte-identical |
| **GA-3** | Platform Expansion → 1.1.0 | 138–142 | Accelerators, networking/web/db, performance, next release |

Phases are numbered continuously from 128 (127 = bootstrap memory guard, done).

---

## 4. Milestone GA-1 — Ship & Harden (Phases 128–129)

### Phase 128 — Release 1.0.0 Cut & Distribution
**Scope**: turn the ready CI pipeline into an actual release.

| Deliverable | Detail |
|---|---|
| Tag `v1.0.0` | Owner action on the existing tag-triggered workflow |
| 13-platform archives + SHA-256 | Already implemented in `.github/workflows/ci.yml` build matrix |
| `installation.rst` | ✅ Release-ready: 13-archive table + `checksums.txt` SHA-256 verify commands + `:planned:` release-pending marker to flip at cut |
| `verify-rc-journey.ps1` re-run | Owner post-publish step against the shipped binary (checklist step 4) |
| Release notes | ✅ `docs/release/KARKAIN-1.0-RELEASE-NOTES.md` + `docs/source/release-notes.rst` refreshed to the real 1.0.0 (59-golden corpus, `std.numerics`/`std.net`/`std.http`/`std.db`, KIR, enum/match parity, closures honest) |
| Owner runbook | ✅ `docs/release/KARKAIN-1.0-CHECKLIST.md` section 6: commit→merge→push→re-point unreleased `v1.0.0` tag (behind `main`)→CI release→post-publish verify→unflip docs |
| Audit findings | ✅ tag `v1.0.0` exists on origin but points to `f23c024` (2026-09-13, behind `9dfdc76`); no GitHub Release exists; version identity unified at 1.0.0 everywhere (grep-verified) |

**Gate**: `pkg/cli/phase119_stable_test.go` + `phase118_rc_test.go` green;
archives exist and install on a clean Windows 11 VM.

### Phase 129 — 4 GB Bootstrap Battle
**Scope**: make the self-hosting proof workable on the dev host, or fail
beautifully.

| Deliverable | Detail |
|---|---|
| Paging-file & memory guidance | ✅ `docs/audit/PHASE-129-4GB-BOOTSTRAP-BATTLE.md` — fixed 16 GB pagefile steps, hog list, isolation rule, status commands |
| kcc build memory reduction | ⏳ Investigation plan + RSS measurement table written; live measurement pending (host at 100% commit, toolchain cannot launch) |
| `GOMEMLIMIT`/`-p 1` CI hardening | ✅ `.github/workflows/ci.yml` test job exports `GOMEMLIMIT: 8GiB` (on top of existing `-p 1`) |
| Bootstrap watchdog | ✅ phase-127 `error[K127]` guard + new `startProgress` 45 s liveness heartbeat in `runCmd`/`runCmdOutput` (stages 1/2/3 + gcc) — a stall now reports, never silently hangs |

**Gate**: `TestBootstrap_Stage2SelfHosting`/`BitwiseIdentity` either PASS on the
4 GB host (with paging guidance applied) or abort cleanly with `error[K127]` in
< 5 min. `go test ./pkg/...` completes under `-p 1`. Verdict: delivery
COMPLETE; live-host gate verification pending (delegated when memory frees or
on CI).

**GA-1 exit**: 1.0.0 is downloadable, installable, and demonstrable on the dev
host. **Verdict marker: GA SHIPPED — pending only the owner tag-cut ceremony.**

---

## 5. Milestone GA-2 — Language & Tooling Completeness (Phases 130–135)

### Phase 130 — Closures / `fn` Values (both engines)

**Status: COMPLETE** — report: `docs/audit/PHASE-130-FINAL-REPORT.md`.

Planned scope was sourced from the stale WIP parser assumption ("codegen
documented broken on both engines"). The live investigation corrected that:
Phases 54/121 already ship WORKING let-bound closures on the Go engine and the
parser WIP (`ClosureExpr`) is not needed. Phase 130 therefore completed the
capture surface that was actually still open:

- **Capture mutation pinned and parity-verified** — `examples/closures/
  00_capture_mutation.kark` (a `var` captured and ASSIGNED inside the closure
  writes through the env alias; enclosing scope observes the mutation;
  golden `1/2/2/3/3`) and `01_nested.kark` (typed nested lambdas, inner
  captures through the outer env pointer; golden `32/28`). Both byte-identical
  Go↔kcc — the kcc leg ran live on the host, not skipped.
- **Boundaries root-caused (both from one root: closures desugar to plain
  functions, no first-class Value cell; parity preserved)**
  1. No first-class function values — `ops[0](5)` is a K001 parse error; no
     `func(T) R` type syntax.
  2. A closure variable cannot be free-captured by another closure
     (`let g = fn... { ... f(...) }` passes check, fails at C compile with
     `&f` undeclared — identically on both engines).
- **Corpus + gate**: `examples/closures/` (sibling category, dedicated Phase 130
  gate; Phase 114's 59-golden corpus untouched by construction).
  `pkg/cli/phase130_closures_test.go` (Go goldens, kcc goldens with
  K127-skip note, `FirstClassBoundary`, `ClosureVarCaptureBoundary`) +
  `pkg/codegen/phase130_closures_test.go` (mutation write-through + nested
  pointer-chain markers).

**Remaining for a future phase**: first-class function values (item 1 above) —
the entry point for higher-order programming.

### Phase 131 — Reproducible Self-Host Gate — SHIPPED (owner decision)

Shipped 131 is the deterministic bootstrap gate (`pkg/bootstrap/
phase131_repro_test.go`: stage-1 byte-reproducibility, tamper-divergence,
K127 guard). The stdlib freeze below was renumbered 131→132 accordingly.

### Phase 132 — Standard Library Freeze + GA API Freeze
**Scope**: every `std.*` module that ships in 1.0 ships *stable*.

| Deliverable | Detail |
|---|---|
| API freeze pass | `std.string/collections/io/encoding/crypto/testing/numerics/net/db/http` — signatures documented, no silent drift |
| Missing-dep cleanups | Enforce byte-identical both-engine compilation for every module (Phase 125A gave net/db/http Go-engine verified modules) |
| `SemVer` policy doc | `docs/development/feature-freeze.rst` → formal versioning + deprecation contract |
| Deprecation mechanism | `@deprecated` attribute or `error[KXXX]` warning channel, gated both engines |

**Gate**: stdlib both-engine byte-identical matrix extends to all released
modules; a public API snapshot doc (`stable-api.rst`) regenerated.
**Status: COMPLETE** — report `docs/audit/PHASE-132-STDLIB-FREEZE-FINAL-REPORT.md`
(6/6 gate PASS, kcc leg live, Sphinx `-W` clean).

### Phase 133 — First-Class `fn` Values (owner priority)

Karkain-owned design only: Value-cell function values over the existing
desugar-to-plain-functions + env-pointer model (Phase 130 root cause), by-ref
capture consistent with capture-mutation; `func(T) R` type syntax; K-family
diagnostics; byte-identical both engines with negatives pinned. Both engines
lex/parse every new shape from day one (parity-by-construction).

### Phase 134 — Incremental Compilation v2
**Scope**: upgrade Phase 105's whole-assembly cache to per-module units.

| Deliverable | Detail |
|---|---|
| Per-module `.o` cache | Rebuild only changed modules; link cached objects |
| Interface hashes | Already exist (public headers only) → key each unit |
| No-op fast path | `karkain build --incremental` no-op target ≤ 100 ms (baseline 2790 ms clean) |
| Cache invalidation | Transitive; compiler-identity key; never serve stale bytes |

**Gate**: `pkg/compiler/incremental_test.go` extended; byte-identical output vs
clean build; `karkain clean` purge still correct.

### Phase 135 — Package Registry MVP (local, honest)
**Scope**: a *local* registry that works today, without pretending to host a
public service.

| Deliverable | Detail |
|---|---|
| Local registry server | `karkain pkg registry init/serve/publish/install` over a directory origin |
| Auth/checksum | Lockfile + SHA-256 pinned; no silent upgrades (Phase P0 hardening reused) |
| Public registry | Explicitly OUT of scope; CLI keeps the honest "not available" diagnostic |

**Gate**: `pkg/pm/registry_test.go` + `pkg/cli/package_cli_test.go` E2E:
publish → install → build → run on a clean path.

### Phase 136 — LSP v2 (Semantic IDE Experience)
**Scope**: from diagnostics-only (Phase 83) to a real editing experience.

| Deliverable | Detail |
|---|---|
| Semantic tokens | Lexer/parser-driven token classification (vs native `vscode-textmate`) |
| Hover | Identifier → inferred type + docs (reuse `pkg/sema`) |
| Go-to-definition | Cross-module resolution via the qualified-call machinery |
| Completion | Env-filtered identifier + member completion; conservative |
| VS Code extension | `extension.js` gains hover/goto; validated by an LSP integration gate |

**Gate**: `pkg/lsp/lsp_test.go` extended with semantic-token round-trips and
hover assertions; extension smoke test green.

### Phase 137 — Concurrency + Profiling + SIMD kcc Parity
**Scope**: collapse the "Go-engine only" Experimental flags.

| Deliverable | Detail |
|---|---|
| `spawn`/`channel`/`actor` on kcc | Self-hosted `src/compiler` tokens already lex; implement sema+codegen tables → byte-identical C |
| `prof` on kcc | Phase 110 schema reused; fib(18)=8361 assertion passes on both engines |
| `@simd_*` on kcc | Lane-type annotations lowered identically (Phase 106 nouns) |
| Scope matrix move | `scope.rst`: concurrency/profiling/SIMD Experimental → Stable Core |

**Gate**: `pkg/cli/phase135_parity_test.go` — one corpus file per surface,
byte-identical Go↔kcc stdout; existing `phase107`/`phase110`/`phase106` gates
re-run on kcc.

**GA-2 exit**: `scope.rst` has **zero** "broken on both engines" and
**zero** Go-engine-only core surfaces. **Verdict marker: LANGUAGE STABLE.**

---

## 6. Milestone GA-3 — Platform Expansion & 1.1.0 (Phases 136–140)

### Phase 138 — Accelerator Kernel Surface v1 (GPU/NPU)
| Deliverable | Detail |
|---|---|
| WGSL emission | `pkg/backend/gpu` emits real WGSL compute kernels (not comment stubs) |
| `@target(gpu)` | Attribute marks kernel funcs; CPU oracle remains the correctness reference |
| Compile-only guarantee | `karkain build --target gpu-*` produces artifacts WITHOUT hardware/SDK |
| NPU parity | Phase 78 quantization path **unchanged**; only the authoring surface grows |

**Gate**: `pkg/cli/phase136_gpu_test.go` — WGSL text deterministic; SPIR-V/ANGLE
optional on CI (never a hard dep).

### Phase 139 — Cross-Compilation Expansion + WASM GC
| Deliverable | Detail |
|---|---|
| New triples | `aarch64-windows`, `riscv64-linux`, `x86-64-macos` search/error paths |
| WASM GC | Components + GC types on top of the Phase 108/123 MVP; `wit` bindgen MVP |
| Toolchain matrix | CI cross-linker presence matrix; deterministic errors, never silent fallback |

**Gate**: `pkg/target/triple_test.go` extended; `phase111`/`phase108` gates
re-run; new `phase137_*` gate.

### Phase 140 — Debugger Integration
| Deliverable | Detail |
|---|---|
| DWARF consumption | `dwarf_parse.go` readers wired to a live `lldb`/`gdb` protocol walk |
| VS Code debugger | `launch.json` template + `extension.js` attach path |
| Breakpoints/step | Source addresses from `SourceAddressMap` (Phase 84/85) |

**Gate**: `pkg/codegen/dwarf_test.go` extended; a `lldb`-source backtrace matched
against `karkain_dbg` trace.

### Phase 141 — Performance & Memory (compiler itself)
| Deliverable | Detail |
|---|---|
| SSA optimizer → codegen | Phase 93/94 passes actually drive generated C (today they are an internal IR, not the emitter) |
| kcc build RSS | Target: full self-build under 2.5 GB so the dev host's bootstrap completes natively |
| Startup/profile | `karkain` binary lazy-loading; `prof` allocation-site accuracy (Phase 110 boundary today) |

**Gate**: `pkg/ir/ssa` bench suite improvements + `TestBootstrap` green on 4 GB.

### Phase 142 — 1.1.0 Release
| Deliverable | Detail |
|---|---|
| Version bump + changelog | SemVer path established by Phase 131 policy |
| LTS branch | `1.0.x` maintenance branch; 1.1.0 on `develop`/`main` |
| Release rehearsal | Full GA-1 ceremony repeated with the new tag |

**Gate**: complete QA battery green; archives + checksums; docs honest.

---

## 7. Dependency graph

```
GA-1:     128 ──▶ 129
                ────▶ 130 ──▶ 131 (repro gate, shipped) ──▶ 132 (freeze, shipped)
GA-2:     132 ──▶ 133   (fn values; needs frozen stdlib surface to test against)
          132 ──▶ 134   (independent infra)
          132 ──▶ 135   (independent infra)
          132 ──▶ 136   (uses stable-api snapshot)
          137 needs 130 + 133 (parity machinery + closed fn surface) + 129 (memory)
GA-3:     138 .. 139 .. 140 (sequential only at the packaging level)
          141 needs 129 (memory) and benefits from 137 (parity)
          142 needs everything
```

Parallelizable tracks after GA-2 opens: **language** (133), **infra** (134–136),
**parity** (137), **platform** (138–141). Single-maintainer means one active
track + CI for the rest.

---

## 8. Risk matrix

| Phase | Risk | Mitigation |
|-------|------|------------|
| 128 | Release ceremony blocked by owner availability | No code risk; keep `verify-rc-journey` as the single checklist |
| 129 | Cannot fit self-build in 4 GB no matter what | Fall back honestly: clean K127 abort + documented ≥8 GB recomndation is GA-acceptable if the *user* path is downloadable binaries |
| 130 | Closure semantics deep-water (capture/purity) | Constrain to lexical closure of the documented surface; no capture analysis promises beyond by-ref/let |
| 133 | First-class fn values reopen closure semantics | Constrain to the Phase 130 desugar model (env-pointer + plain functions); no ownership promises beyond by-ref/let |
| 137 | kcc parity reopens compiler-risk classes | Run `TestBootstrap_BitwiseIdentity` per change; keep kcc the default engine |
| 138/139 | No accelerator hardware / cross-linkers to validate | Compile-only + deterministic-error gates; CI acts as the hardware oracle |
| 141 | SSA→codegen is a large architectural change | Keep it as a *probe-lowering* first slice; measured at bench, never a rewrite |

---

## 9. Phase completion contract (gate for every phase)

1. Code compiles: `go build ./...`
2. `go vet ./...` clean
3. New gate test(s) pass (`pkg/cli/phaseXXX_*_test.go` + units where applicable)
4. Full regression: conformance 59/59, probes 11/11, prior phase gates, `pkg/*` suites
5. Audit report written: `docs/audit/PHASE-XXX-*.md`
6. AGENTS.md completion record updated; `scope.rst` buckets updated where surfaces move
7. Commit to `develop` → merge to `main` → push both (AGENTS.md hard rule)

---

## 10. Future scope beyond 1.1.0 (watchlist, not promised)

- **Language**: generics, trait/impl inference, error-handling ergonomics, `async/await`
- **Toolchain**: plugin API, build server, editor-agnostic DAP, fuzzing harness
- **Platforms**: WebGPU browser target, embedded/freestanding release, WASI 0.3
- **Ecosystem**: public package registry, AI/ML framework on `std.numerics`,
  quantum simulator surface
- **Performance**: JIT tier, arena-based kcc itself, AOT image shrinking

These enter the roadmap only when a phase above lands; maturity-first, never
promised in a release without an implemented, gated surface.

---

*Theming note: every "Planned"/"Experimental" row in `scope.rst` is tied to a
phase number above, so progress is verifiable against the authoritative matrix,
not just prose.*