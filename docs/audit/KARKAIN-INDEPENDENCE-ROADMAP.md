# Karkain Independence Roadmap — Off Go, Off C23/gcc

**Status**: PROPOSED (owner-approved direction, phase gates binding).
**Baseline**: 1.0.0 Stable (`VERSION` = `1.0.0` since Phase 119), post-132 (stdlib freeze shipped). Next phase 133.
**Future version**: `1.1.0` — all I/S/P tracks land as backwards-compatible additions per `semver-policy.rst` MINOR rules; only `1.0.x` patches ship before it (LTS branch at release, GA-142).
**Constraint**: single maintainer, Windows 11 / 4 GB host, `go build ./...` + `go vet` + gates green per phase.
**Sources of truth**: `AGENTS.md` (completion record), `docs/inventory/compiler-dependencies.json` (ownership), `docs/source/compiler/architecture.rst` (transitional surfaces), `docs/audit/GENERAL-AVAILABILITY-ROADMAP.md` (GA-2/GA-3 numbering — this doc extends it, does not replace it).

## 0. Goal definition (what "independent" means)

| Dependency | Today | Target | Kept deliberately |
|---|---|---|---|
| Go toolchain (user) | NOT needed (prebuilt `karkain.exe` contains both engines) | unchanged | — |
| Go code (build-time) | `pkg/*` reference front-end builds `kcc`; `KARKAIN_ENGINE=go` pinned for stage-1 (`pkg/cli/kcc_engine.go:184-211`, `docs/source/compiler/bootstrap.rst:84-92`) | `kcc` builds `kcc`; `pkg/*` archived as reference/oracle | Go oracle kept for differential testing, never on user path |
| C23/gcc (link-time) | BOTH engines emit C23 and shell `gcc` (`src/compiler/codegen.kark:generate`, `pkg/codegen`); direct `kcc run` uses `src/compiler/main.kark:279` (missing `-lws2_32` — the live bug) | native target links with NO `gcc` on `PATH`; `--target c23` stays as an OPTION | C23 emitter kept as a target, never required |
| Git | conditional only (`git`-source deps + clone) | unchanged (local/registry paths need no git) | git-source deps always need git by definition |
| Stdlib | 10 frozen-thin + 5 unfrozen + absent OS basics (see §3) | OS foundation + promoted modules, both-engine goldens | — |

Rule: **Go-removal first (safety net intact), native-link second.** Both-at-once is rejected (one active track + CI).

## 1. Phase matrix (at a glance)

| Phase | Name | Removes | Stdlib | Gate (must all pass) | Maps to GA |
|---|---|---|---|---|---|
| **I-0** | Direct-kcc unblock | nothing (bugfix) | none | `kcc run 01_hello_world.kark` direct prints golden; `kcc check/build/run/test` smoke; `go vet`, `go build` | pre-133 |
| **I-1** | OS stdlib foundation | enables later Go-removal | NEW `std.path/env/time/random/json` (both engines, goldens) | `phase13x_os_stdlib_test.go` Go+kcc byte-identical; `stable-api.rst` rows added | 133-slot (with fn values) |
| **I-2** | Assembly + PM in Karkain | Go BRIDGE `kccAssembleSource`/`kccStageInput` shrinks to spawn-only | triage: promote `math/system/async` OR cut to Planned + fix `scope.rst` drift | manifest/workspace/lockfile e2e via `kcc` alone; `phase122` gate re-green | 134/135-slot |
| **I-2b** | Project scaffolding in Karkain | Go `InitProject` → Karkain (`kcc init`/`new`) | none (uses I-1 OS file builtins) | scaffold output byte-identical to Go; fresh project builds via `kcc` alone, no Go/gcc on PATH (after I-5) | 134/135-slot |
| **I-3** | KIR-consumption | Go preflight text-composition | none (uses I-1) | backend reads `KIR v1`; `kir --verify` count stable; corpus 59+ goldens byte-identical | 133→137 dep |
| **I-4** | Emitter parity | Go-only `concurrency/prof/SIMD/DWARF/cross-driver` → dual | none | `phase107/110/106/111` re-gated on kcc; `scope.rst` Experimental→Stable | GA 137 |
| **I-5** | Native link (prototype→production) | `gcc` required → optional | none (uses I-1) | hello + `stdlib_v2` link with NO `gcc` on `PATH`; `c23` still available via `--target c23` | GA 141 / post-1.1.0 |
| **I-6** | Self-bootstrap | `KARKAIN_ENGINE=go` pin + `pkg/bootstrap` retired | frozen | `stage2==stage3` with NO Go front-end; full QA battery green | after I-4 + I-5 |

Dependency order: `I-0 → I-1 → I-2 → I-3 → I-4 → I-5 → I-6`. I-2/I-3 can overlap on CI; I-4 needs I-1+I-3; I-5 needs I-1; I-6 needs everything. S-track rides along: `S-0/S-1/S-3` after `I-1`; `S-4/S-5` with `I-4`; `S-2` MMIO-real with `I-5` (compile-only before); `S-6` last.

## 2. Phase details

### I-0 — Direct-kcc unblock (hours, no new surface)
**Problem**: `src/compiler/main.kark:279` `runFile` hardcodes `gcc -o karkain_temp ... -lm` (no `-lgmp`, no `-lws2_32`). Phase 125A made the net preamble unconditional (`src/compiler/codegen.kark:287` +~202 lines, `#ifdef _WIN32` `WSA*`), so EVERY Windows link needs `-lws2_32`. `compileRunDriver` (`main.kark:795-802`) got the retry; `runFile` was missed. `karkain run` (Go wrapper `winsockLibFlag()`) works, `kcc run` direct fails at `ld`.
**Do**: same retry in `runFile` (`system(compileCmd)` → on nonzero retry `+ " -lgmp -lws2_32"`); rebuild `kcc.exe` via Go bootstrap; fix `scope.rst:153-155` (`std.net does not exist` is false — `stable-api.rst:124-130` ships it); document `kcc build` (emit-only, no link) vs `kcc run` (emit+link).
**Gate**: direct `kcc run examples/01-fundamentals/01_hello_world.kark` prints golden in repo root AND in `examples/01-fundamentals/`; `kcc check/build/test` smoke; `phase122` 7/7 unaffected.
**Out**: unblocks all later I-phases (they test direct `kcc`).

### I-1 — OS stdlib foundation (+ first-class fn, 133-slot)
**Why first**: K2/K3 need `file/path/env/process/time` owned by Karkain before Go's sandbox/spawn can move.
**Do**: (a) NEW modules, canonical Karkain, both engines from day one: `std.path` (join/basename/dirname/exists), `std.env` (get/set/args), `std.time` (now/sleep/duration), `std.random` (seeded int/bytes), `std.json` (encode/decode flat subset). Small surfaces (~10 funcs each), golden programs pinned in Phase-114 gate. (b) Keep GA-133 `func(T)R` first-class fn values on the Phase-130 desugar model (env-pointer + plain functions, by-ref capture) with K-diagnostics + negatives.
**Gate**: new `pkg/cli/phase13x_os_stdlib_test.go` (Go AND kcc byte-exact goldens, live kcc leg, K127-skip note); `stable-api.rst` gains 5 rows; `phase130` gate re-green.
**Out**: Karkain can express everything Go's wrapper does.

### I-2 — Assembly + package-manager resolution in Karkain (134/135-slot)
**Do**: port `kccAssembleSource`/`resolveSourcesRun` (manifest/workspace/lockfile, local+workspace+registry-cache) into `assembleProject` (`src/compiler/main.kark:588`); Go's `kccStageInput/flatAssemblyEligible/kccMirrorFlat` shrinks to dumb file-mirror + process spawn. Stdlib triage in the same phase: promote `math` (100 funcs) / `system` (16) / `async` (37) with both-engine goldens, or cut `gpu` (31-stub)/`core` (23) to Planned and say so in `scope.rst`.
**Gate**: manifest + workspace + lockfile e2e (`add/fetch/build/run/test`) through direct `kcc` with NO Go assembly; `phase122` + `phase132` gates re-green; `scope.rst`/`stable-api.rst` agree.
**Out**: Go owns no compiler input composition.

### I-2b — Project scaffolding in Karkain (`init`/`new`)
**Today**: `karkain init/new` runs Go `InitProject` (`pkg/pm/manager.go:208` — `MkdirAll` dirs, `karkain.toml`, `src/main.kark`, `tests/main_test.kark`, `.gitignore`). Scaffolding itself needs no compiler, but everything after it does (Go assembly + `gcc` link), so a fresh project is Go/clang-bound from birth.
**Do**: implement `kcc init [name]` / `kcc new <name>` in `src/compiler/main.kark` using the existing file builtins (`createFile`/`writeToFile`/`closeFile` — same ones `buildFile`/`runFile` use) plus I-1 `std.path` helpers; emit byte-identical trees to Go `InitProject` (same manifest defaults, same `.gitignore`, same hello + test contents). `karkain init` routes to it like other commands route to kcc (default engine). Needs I-1 OS surface (dirs/files) first; full "no Go/gcc on PATH" fresh-project build additionally needs I-2 (assembly) + I-5 (native link).
**Gate**: scaffold diff empty (`diff -r` Go-tree vs kcc-tree, ignoring nothing); fresh kcc-scaffolded project passes `kcc check/test` immediately and `kcc run src/main.kark` prints the hello golden.
**Out**: projects are born Karkain-owned; Go keeps no project-creation role.

### I-3 — KIR-consumption
**Do**: backend takes `KIR v1` text as input (today `kir.kark:kirEmit/kirVerify` is on-demand only; `pipeline.kir_invariant` in the inventory). `checkFile` already verifies (`error[K121]`); now `build/run` consume the verified lines. No new syntax.
**Gate**: KIR line counts stable (whole-tree pin as in phase122 KIR-continuity); corpus goldens byte-identical via KIR path vs AST path; `phase120/121` gates re-green.
**Out**: single IR contract both engines honor; emitter ports (I-4) have one input.

### I-4 — Emitter parity (GA-137)
**Do**: port one surface at a time behind the KIR input: `spawn/channel/actor` helpers, `prof` schema (`fib(18)=8361`), `@simd_*` lane lowering, `DWARF` sections, `--target` cross-driver selection. Each is a sub-gate; `kcc` shells to the same link until I-5.
**Gate**: `phase107/110/106/111` suites pass on kcc; one corpus file per surface, Go↔kcc byte-identical; `scope.rst` moves all three to Stable Core; `TestBootstrap_BitwiseIdentity` per sub-change.
**Out**: zero Go-engine-only core surfaces (GA-2 exit).

### I-5 — Native link, gcc optional (GA-141 / post-1.1.0)
**Do**: `x86-64` machine-code emitter + `ELF` writer first (Linux), then `PE` (Windows), modeled on `pkg/wasm/module.go+emit.go` (the proven direct-emission pattern). Reuse `NativeBuilder` (`pkg/codegen/native_builder.go+native.go`) design; calling convention + relocations + runtime ABI documented in `docs/native-codegen.md`. `c23` remains (`--target c23`), gcc required ONLY for that target.
**Gate**: hello + `stdlib_v2` + one `net` program link with `gcc`/`clang`/`cl` ALL absent from `PATH`; cross-target deterministic errors preserved; `phase111` re-green.
**Out**: C23 dependency gone from the default path.

### I-6 — Self-bootstrap (retire the pin)
**Do**: `kcc` builds `kcc` end-to-end (no `KARKAIN_ENGINE=go`, no `pkg/bootstrap` stage-1); Go tree archived as `reference/` oracle for differential tests only.
**Gate**: `stage2==stage3` SHA with Go toolchain absent from the build host; full QA battery (units, 114 corpus Go+kcc, 115-118, conformance 59/59, probes 11/11, verify-examples, Sphinx `-W`, install/verify-install, rc-journey) green; `installation.rst` Go row reads "reference only".
**Out**: independence declared.

## 3. Library gap matrix (what "flourish all corners" still needs)

Per-module / per-tool detail lives in **Appendix A (stdlib)** and **Appendix B (dev tools)** — the table below is the summary.

| Bucket | Modules (funcs) | Verdict |
|---|---|---|
| Frozen, thin | `crypto` 2, `encoding` 7, `io` 11, `net` 10, `http` 20, `db` 43-text-engine | keep + extend one axis per phase (`crypto`: hmac/aes-gcm; `encoding`: json→moves to I-1 `std.json`; `io`: dirs/lock; `http`: tls/routing OUT until I-5) |
| Frozen, solid | `string` 28, `collections` 24, `testing` 9, `numerics` 40 | extend only on demand |
| Exists, unfrozen | `math` 100, `async` 37, `gpu` 31-stub, `core` 23, `system` 16 | I-2 triage: promote or cut, no middle state |
| Absent (blocks I-2+) | `std.path/env/time/random/json`, logging, `compress`, `fs-walk` | I-1 delivers the first five; rest queued post-I-4 |

## 4. Risks

| Risk | Mitigation |
|---|---|
| I-5 native ABI blowup (PE relocations, calling convention, DWARF) | ELF-first, one arch; C23 stays until native proves goldens; never both-at-once with I-4 |
| 4 GB host cannot self-build (known OOM class, K127) | `GOMEMLIMIT`/`-p 1`, isolated gates, clean `error[K127]` accepted as green on dev host; CI as oracle |
| Stdlib drift (`scope.rst` vs `stable-api.rst`) | I-0/I-2 make them agree; every new module updates both + gate in the same commit |
| Single-maintainer bandwidth | one active I-phase + CI; each phase shippable alone (K0→I-1→I-2 order is value-ordered) |

## 5. Systems-programming track (S-0..S-6 — whatever is left for a systems language)

Goal: keep the language surface unchanged; change what it lowers to. Each S-phase is gated by re-proving all I-phase goldens (layout/ABI changes break bytes by design — the gate pins the NEW bytes on both engines).

| Phase | Name | Problem today | Do | Gate |
|---|---|---|---|---|
| **S-0** | C-layout structs | `struct` is map-based (`TYPE_STRUCT`: string-keyed, `memory-model.rst:29-31`) — no cache layout, `packed` meaningless | C-layout records + `packed`/alignment; map-struct stays ONLY as `map` literal; `SPEC.md:118-123` `packed` moves draft→stable | struct-heavy goldens re-pinned byte-identical Go↔kcc; field-offset probe test; `phase102` core-layout gate extended |
| **S-1** | Linear types | `linear` declared, unenforced (`SPEC §2.4` partial) | exactly-once consumption (`linear type FileHandle`) via checker (both engines: Go `pkg/sema` + `checker.kark`), `error[K11x]` on drop/double-use + negative fixtures | linear positive + 3 negatives both engines; `stable-api.rst` row |
| **S-2** | Hardware access | no `volatile/MMIO/interrupt/inline-asm` model; `import "C"` unhardened | `volatile` load/store + `mmio_read/write(addr)` builtins over `addr`; `asm()` stub as compile-only intrinsic with K108-style reject on unsupported targets; interrupt handlers stay Planned (documented, no stub) | MMIO loopback probe via native + wasm-reject diagnostic; `K108` table extended |
| **S-3** | Freestanding target | `runtime/freestanding/` proves hello with `-ffreestanding -nostdlib` (Phase 101) but default codegen never uses it | `--target freestanding` (x86-64, no libc): arena + core + `Value` only; `net/http/db` rejected deterministically (no sockets without OS) | freestanding hello links with NO libc; `phase101` gate + new `phase13x_freestanding_test.go` |
| **S-4** | Atomics + ordering | 107 runtime has GNU atomics internally; no in-language surface; kcc parity missing (I-4 covers spawn/channel/actor) | `atomic_*` builtins + ordering args (`relaxed/acquire/release/seqcst`) on both engines; kcc emits the same C helpers Go does | litmus golden (counter == N on both engines); `phase107` gates re-green on kcc |
| **S-5** | Zero-cost wiring | SSA opt (93/94) is internal IR, never drives emit (roadmap 141); SIMD kcc-inert | SSA→codegen probe-lowering first (one pass, measured at bench); `@simd_*` kcc lowering identical (completes I-4) | `pkg/ir/ssa` bench delta + `phase106` gates on kcc; no silent perf change on existing goldens |
| **S-6** | Systems observability | DWARF emitted, never consumed; `prof` alloc-site blind (Phase 110 boundary) | `dwarf_parse.go` → `gdb/lldb` walk + `launch.json`; `prof` attributes preamble `make_*` allocs (documented today as uncounted) | backtrace match test; `phase110` + `phase104` gates extended |

Order: `S-0 → S-1 → S-3` can run after `I-1` (needs OS stdlib for tests); `S-4/S-5` ride on `I-4`; `S-2` MMIO needs `I-5` native for real addresses (QEMU/CI as oracle until then — compile-only + deterministic-reject gates like cross-targets); `S-6` last. `S-0` is the ONLY intentionally breaking change — it gets its own major-gated commit with the re-pinned goldens reviewed as the diff.

## 6. Language-improvement track (L-0..L-6 — milestones for future phases)

Each L-phase becomes one or more numbered phases (133+) when scheduled. Like S-phases, every L-phase re-proves the I-phase goldens; behavior changes are pinned as NEW goldens on both engines, never silent.

| Phase | Name | Closes (from analysis) | Do | Gate |
|---|---|---|---|---|
| **L-0** | Foundations & honesty | doc drift; un-gated wrong-code classes; lexer/doc mismatches | (a) `scope.rst` vs `stable-api.rst` agreement (6 vs 10 modules; `std.net` row); status vocabulary unification (`status/index.rst` six labels ↔ `scope.rst` buckets mapping — stated once, never redefined); `ROADMAP.md` marked superseded-pointer to `AGENTS.md`+audits; `installation.rst` splits build-time vs conditional deps. (b) repro fixtures for `alloc(T,n)` codegen + struct-in-array hang + string-array print divergence — each gets PASS or loud `K1xx`/diagnostic reject, both engines. (c) keyword/doc parity: `fundamentals.rst` lists `nil`/`assert*`/`test`/`wait_all`/`actorSend…` as keywords but no lexer emits them (`lookupIdent` is authoritative); `keywords.rst` omits `fn/matrix/alloc/free/addr/qreg/gate/measure/macro/quote/unquote/comptime/kernel/device/global_id/barrier/mut/raw/move/Some/None/Ok/Err/linear/packed/type/bigint/bigfloat/print`; `SPEC §1.3` denies block comments Phase 112 shipped. One table, generated-or-tested. (d) nested `/* */` kcc parity — Go-only today (`lexer.kark:skipWhitespace` closes at first `*/`), `fundamentals.rst:36-37` admits it | new `phase13x_lang_honesty_test.go` + keyword-table test (every `lookupIdent` keyword documented, every documented keyword lexes on both engines); docs agree in the same commit |
| **L-1** | Closures & match | first-class `fn`; closure-in-closure; arm bodies; payload vs tag-only; casts | Value-cell `func(T)R` on the Phase-130 desugar model + env-chain (or loud reject); `checkStmt`-level checking of match-arm bodies; verdict: payload binding OR documented tag-only; verdict: `float64()/bool()/string()` casts in or rejected | positives + negatives both engines; `stable-api.rst` syntax rows; `phase130` re-green (shares the 133-slot with I-1) |
| **L-2** | Types & methods | generics; `const` folding; inference; method resolution | generic fns + const params with arity gates; `const` fold + 3 goldens; extend `inferType` with negatives; true member resolution + visibility beyond `public` (builds on 103 qualified calls). (`linear` lives in S-1 — no duplication.) | generic + const goldens both engines; `SPEC.md` badges moved with the code |
| **L-3** | Runtime correctness | `alloc` wrong-code; struct-in-array hang; GMP tie | fix or deterministically reject each L-0 repro; verdict: GMP stays a declared dep (documented, `depends: libgmp`) vs vendored big-int (costed separately — default: keep GMP) | repro gates green both engines; `installation.rst` + packaging `depends` agree |
| **L-4** | Backend surfaces | WASM GC/components/`wit` + kcc-wasm verdict; GPU/NPU authoring; cross expansion | WGSL-emit compile-only first (CI oracle, no hardware); WASM GC on the 108/123 MVP; `riscv64/macos` search paths + deterministic errors (GA-138/139) | `phase108/111` re-green + new `phase13x_backend_test.go` (deterministic-error table included) |
| **L-5** | Tooling & DX | LSP v2; debugger; `prof`; registry; per-module cache; direct-`kcc` UX | semantic tokens/hover/goto/completion (GA-136); `launch.json` attach (with S-6); `prof` kcc parity + alloc accuracy (with I-4/S-6); local registry MVP (GA-135); per-module `.o` (GA-134); direct-`kcc` flag class removed by I-2 | per-surface gates; `scope.rst` moves rows to Stable as each lands |
| **L-6** | Docs & process lock-in | SPEC badges; snapshot procedure | rule: no phase closes without its `SPEC.md` badge flip + `stable-api.rst`/`scope.rst` row + gate in the same commit (checked in review, not by tooling) | L-0 gate enforces the habit; every later phase inherits it |

Order: `L-0` first (with I-0 — cheap, unblocks honest planning); `L-1` with `I-1` (133-slot); `L-2` after `L-1`; `L-3` as soon as repros exist (can parallel I-2 on CI); `L-4` with I-5/GA-138-139; `L-5` spread across GA-134/135/136/140; `L-6` is a standing rule, not a phase.

### Device coverage (general-purpose claim — microscopic to telescopic)

Purpose statement: Karkain runs on any device from a microcontroller to a compute cluster — including the instruments (sensors, probes, telescopes, lab rigs) with which we study microscopic to galactic scales. Honest bound: the language runs on computers, not on stars.

| Scale | Devices | Target / phase |
|---|---|---|
| Microscopic | MCUs (`Cortex-M`, `RISC-V` embedded), sensors, wearables | `--target freestanding` (S-3); named MCU profiles after I-5 native |
| Personal | phones (Android/iOS), desktops, laptops (Win/Linux/macOS/BSD) | Android NDK triple + iOS codesign (extends L-4); P-track installers |
| Local compute | servers, Docker/cloud images, browsers (WASM) | P-0 image; WASM GC/components/`wit` + WASI 0.3, WebGPU (L-4) |
| Telescopic | HPC clusters (SIMD/matrix/GPU/NPU), lab instruments, observatory pipelines, satellite/space payloads (deterministic, reproducible builds) | SIMD/native (I-5/S-5), WGSL + NPU surface (GA-138), `SOURCE_DATE_EPOCH` reproducibility + `freestanding` determinism |

Rule: a scale is claimed only when its gate runs on real hardware or an acknowledged emulator/CI oracle (same rule as cross-targets — never a silent host fallback). Order: freestanding + `riscv64` (CI-provable) → WASM/browser → Android → iOS/signing → HPC/space profiles.

## 7. Packaging + distribution track (P-0..P-4 — Windows/Linux/macOS/BSD/Docker)

Baseline: 13 archives on tag release (`ci.yml:141-246`: Windows `zip`, Linux/macOS/BSD `tar.gz`, `CGO_ENABLED=0`, `checksums.txt`). No MSI/DMG/PKG/DEB/RPM/repo/image. BSD archives are cross-built on `ubuntu-latest`, never executed. Generated programs still need a C compiler on the TARGET — installers must declare it, not hide it.

| Phase | Name | Do | Needs (owner vs free) | Gate |
|---|---|---|---|---|
| **P-0** | Docker image | `Dockerfile` (slim base + `gcc` + `wasmtime` for the wasm target) + CI `buildx` multi-arch (`linux/amd64+arm64`) push to `ghcr.io/ajit-ai/karkain:<tag>` on tag + `cosign` sign | free, no secrets (uses existing tag job) | `docker run ghcr.io/ajit-ai/karkain:<tag> karkain --version` + hello `run` inside image on both arches |
| **P-1** | Windows MSI | `WiX` authoring (per-machine install, `PATH`, upgrade code) on `windows-latest` runner; keep `zip` alongside | free to ship; wide SmartScreen trust needs code-sign cert (owner secret — unsigned MSI ships with a documented warning until then) | clean-VM install → `karkain --version` + hello `build` (with `gcc` present); upgrade over prior MSI keeps `PATH` |
| **P-2** | Linux DEB+RPM | `nFPM` specs (`depends: gcc`) + CI job building both; apt/yum repo decision deferred (hosting cost) — direct download first, `winget/brew` shims later | free | `docker:debian` + `docker:fedora` clean containers install the packages and compile+run hello |
| **P-3** | macOS DMG+PKG | universal `amd64+arm64` binary + `DMG` + `PKG` on `macos-latest`; signed+notarized when secrets exist | Apple Developer ID + notary secrets (owner-only); unsigned DMG builds in CI but Gatekeeper blocks — documented, never claimed as final | `hdiutil` attach + install on Intel + Apple Silicon runners; `karkain target` reports host triple correctly |
| **P-4** | BSD validation | keep `tar.gz`; validate instead of packaging: BSD VM (no GitHub-hosted BSD runner) or community-tested sign-off | VM time or community tester | hello `build+run` log on FreeBSD/NetBSD/OpenBSD checked into the audit report; until then docs read "cross-built, untested" |

Order: `P-0 → P-1 → P-2 → P-3 → P-4` (cheapest, no-secret work first; macOS signing is the only hard owner-cost item). Runs parallel to I/S tracks (packaging never touches the compiler — one CI-only lane). `installation.rst:48-113` flips each row from Planned to live as its P-phase lands; the `:planned:` release-pending note flips per artifact, not all-at-once.

## 8. Completion contract (every I-, S-, P-, L- and D-phase)
`go build ./...` + `go vet` clean; new gate `pkg/cli/phase13x_*_test.go` (+ units); regressions (114 corpus Go+kcc, 115/116/117/118/121/122, conformance+probes, `pkg/*` suites); audit report `docs/audit/PHASE-13X-*.md`; `AGENTS.md` + `scope.rst`/`stable-api.rst` updated; commit `develop` → merge `main` → push (hard rule).

## Appendix A — Stdlib per-module backlog (audited against live sources)

Convention: **EXTEND** = add methods to a frozen module (minor, gated golden); **PROMOTE** = unfreeze with both-engine gates; **NEW** = I-1 module; **MERGE** = fold overlaps, one home only. Higher-order helpers (`map/filter/reduce/sort_by`) unlock after L-1 first-class `fn`.

| Module (have) | Missing methods / extensions | Verdict + phase |
|---|---|---|
| `std.string` (28: split/join/trim/replace/reverse/case-ascii/pad-less) | `str_pad_left/right`, `str_split_lines`, `str_replace_all`, `str_to_float`, `str_is_numeric`, `str_format` (positional `{}`), unicode-aware case (today ASCII-only — document or fix) | EXTEND, I-1 batch 2 |
| `std.collections` (24: contains/index/reverse/copy/fill/slice/remove/insert/unique/flatten/sum/min/max/keys/values/merge) | `array_sort/sorted`, `array_filter/map/reduce` (needs L-1), `array_zip/enumerate`, `map_remove`, `map_get_default`, `set_*` (new set type or array-backed) | EXTEND `sort/remove/default` I-2; higher-order after L-1 |
| `std.io` (11) vs `std.system` (16) overlap | `file_exists`/`list_dir`/`remove_file` exist in BOTH — pick one home; missing `stat_size/mtime`, `mkdir_all`, `stdin/stdout/stderr` handles, buffered read/write, file locks | MERGE io⊃system-file-ops in I-2 triage; locks/buffers post-I-4 |
| `std.encoding` (7: hex/b64/utf8) | `url_encode/decode`, `base32`, moves: JSON → NEW `std.json` (I-1) | EXTEND url/base32 I-2 |
| `std.crypto` (2: sha256/512) | `hmac_sha256`, constant-time `secure_eq`, streaming `sha256_update/final`, `random_bytes` (or via NEW `std.random`); `AES-GCM/ChaCha/RSA` stay Planned (need native C deps — fights I-5) | EXTEND hmac/streaming/eq I-2; symmetric/asymmetric Planned |
| `std.testing` (9: expect/verify/counts/summary) | `assert_approx_eq` (floats), `skip(reason)`, grouped `suite` sections, `bench_*` timing helper (with L-5 bench) | EXTEND with L-5 |
| `std.numerics` (40: vec/mat/tensor/activations/loss) | broadcasting, `mat_det/inv/solve`, `fft`, tensor `save/load`, autodiff `grad` (internal `pkg` IR exists — surface after L-2 generics) | EXTEND per need, post-L-2 |
| `std.net` (10: dial/serve/recv/send/shut) | `udp_send/recv`, `set_timeout`, `dns_lookup`, non-blocking + `select` (with S-4); TLS stays Planned | EXTEND udp/timeout/dns I-2; select with S-4 |
| `std.http` (20: codec + loopback client/server) | `query_parse/build`, `header_get/set`, cookies, `follow_redirects`, timeouts, keep-alive, chunked, `router_add/route_match` helpers | EXTEND query/headers/cookies/redirects I-2; keep-alive/chunked with I-4 |
| `std.db` (43: single-table SQL-text) | `JOIN`, `ORDER BY/LIMIT/OFFSET`, aggregates (`count/sum/avg/min/max`), bound parameters (`db_bind`), `csv_import/export`, migration helper | EXTEND one clause per phase from I-2; external drivers never (text-engine is the product) |
| `std.math` (100, unfrozen) | complete — needs edge gates (`NaN/Inf`, div-zero) not methods | PROMOTE I-2 |
| `std.async`/`actor` (37, unfrozen) | handler type needs L-1 fn values; missing `timeout_recv`, `select` with timeout | PROMOTE with I-4 (+S-4) |
| `std.core` (23, unfrozen) | overlaps `math`/`collections` (`clamp/min/max/abs/sum`) — prelude or merge | MERGE verdict in I-2 |
| `std.gpu` (31 stubs) | real launch after GA-138 WGSL; until then mark stub surface explicitly | PROMOTE with L-4, not before |
| NEW `std.path/env/time/random/json` | — | NEW in I-1 (~10 funcs each, goldens) |
| NEW later `std.log/cli/fs/compress/regex/sync` | arg parsing, `fs_walk/glob`, `log_levels`, `gzip`, `regex_match`, atomics surface (with S-4) | NEW one per phase post-I-4 |

## Appendix B — Developer-tools backlog (audited against `karkain --help` + `scope.rst`)

| Tool | Have | Missing → phase |
|---|---|---|
| `check/build/run` | stable both engines | direct-`kcc` flag parity (I-0), native link (I-5) |
| `test` (+`--compile` corpus, `--filter`) | stable | `skip`, JUnit output, coverage sketch → L-5 |
| `bench` | single-run timing | stats (median/pctl), baseline compare + history → L-5 |
| `fmt` (+`--check`) | idempotent, CI-enforced | LSP range-format + style config → L-5 |
| `lint` (exit 7, borrow checker) | stable | unused/shadow lints + allow/warn config → L-5 |
| `prof` | text/json/folded, Go-only, blind preamble allocs | kcc parity (I-4) + alloc accuracy (S-6) |
| `debug` trace | Go-only (112) | breakpoints/step via DWARF + `launch.json` (S-6/L-5) |
| `lsp` | diagnostics + formatting | tokens/hover/goto/completion/rename (GA-136) → L-5 |
| `pkg` local (init/add/fetch/tree/cache/verify) | stable local/workspace | local registry serve/publish/install (GA-135) → L-5; public registry explicitly out |
| `workspace` | list/build/test/run/graph/init/add/remove | `graph --format dot/json` → L-5 |
| `kir` / `verifykir` | emit + K121 verify | `kir diff` vs saved golden (serves I-3) → L-5 |
| `target` | host + matrix listing | per-target "what to install" explainer (partially in error text today) → L-5 |
| `transpile` | c23 (+kcc wgsl/opencl/openqasm/qir/kbc flags) | Go↔kcc backend-flag parity table → L-4 |
| `explain/clean/config` | stable | nothing missing — keep |

## 10. Debugging & error-handling track (D-0..D-4 — general-language maturity)

A general-purpose language is judged by its edit→run→debug loop, not just its compile step. Current state: front-end `Kxxx` codes + `explain` + JSON diagnostics (both engines); runtime checked ops (div/mod/index + `file:line` + 128-frame `stack:`, identical both engines); `karkain debug` enter/leave trace Go-only (`tools/debug.rst` — kcc refused loudly); frame hooks shared; DWARF emitted, never consumed. Known sharp edges: **unchecked ops silently return zero** (pre-100 behavior, `language/errors.rst:27`); match-arm bodies unchecked (L-1); no breakpoints/step/inspect.

| Phase | Name | Do | Gate |
|---|---|---|---|
| **D-0** | Error-model completion | close the silent-zero class: every unchecked op is either checked at runtime OR rejected loudly at compile with a K-code; publish the complete checked/unchecked table in `language/errors.rst` + `reference/diagnostics.rst` (no undocumented third state) | table + fixtures both engines (each op: golden output or K-reject); `errors.rst:27` line deleted |
| **D-1** | `kcc` trace parity | `karkain debug --engine kcc` emits the identical `karkain:<file>:enter/leave` stderr format (frame hooks already shared — wire `KARKAIN_TRACE` through kcc codegen) | trace goldens byte-identical Go↔kcc; the "Go only" refusal + `debug.rst:31-33` removed |
| **D-2** | Breakpoints + stepping | consume own DWARF (`dwarf_parse.go` → `gdb`/`lldb` walk) + `launch.json` template; line stepping first, conditional breakpoints second (with S-6) | backtrace-match test; VS Code attach doc; `phase104` gate extended |
| **D-3** | Inspection + error context | frame-variable inspection at breakpoints; error context/chaining (`?` propagation with context lines, stack beyond 128 where native permits) | inspect golden (named values per frame); context lines in runtime-error output, identical both engines |
| **D-4** | IDE debug integration | VS Code `launch.json` + extension debug commands; DAP adapter stays Planned (documented, no stub) | extension smoke test green; `lsp` + debug docs agree |

Order: `D-0` with L-0 (honesty batch — cheapest, kills silent behavior); `D-1` with I-4 (emitter parity); `D-2/D-3` with S-6; `D-4` with L-5. With this track + I/S/P/L, the general-purpose loop (write→check→run→test→profile→debug→ship→install on any device/scale) is fully phased to **v1.1.0**.

## 9. Documentation rulebook (hard rule — every change, everywhere, in `.rst`)

No code change closes without its documentation change **in the same commit**. Reviewer check: if the row's doc column is untouched, the phase is not done.

| Change anywhere | Must update (all in same commit) |
|---|---|
| Language syntax/semantics (new keyword, statement, type, operator) | `SPEC.md` badge + `reference/keywords.rst` or `syntax.rst` + `reference/stable-api.rst` + `status/scope.rst` bucket + `language/*.rst` page + one `examples/` golden |
| New/changed diagnostic (`Kxxx`, runtime error text, exit code) | `reference/diagnostics.rst` + `cli/explain.go` table + `stable-api.rst` diagnostic section |
| CLI command/flag/behavior | `tools/cli.rst` (+ per-command page if exists) + `--help` text parity + `getting-started/*.rst` workflow if user-facing |
| Stdlib function/module | `stdlib/<module>.rst` + `reference/stable-api.rst` stdlib table + `status/scope.rst` row + both-engine golden |
| Target/backend (`--target`, triples, compute targets) | `targets/*.rst` + `tools/target.rst` + `installation.rst` matrix if shippable |
| Project/manifest/workspace/pkg behavior | `getting-started/project-layout.rst`, `dependencies.rst`, `workspace.rst`, `tools/packages.rst` as touched |
| Status change (Experimental→Stable, Planned→shipped) | `status/scope.rst` + `status/implemented.rst` or `experimental.rst` + `development/roadmap.rst` phase list |
| New phase completion | `AGENTS.md` record + `docs/audit/PHASE-XXX-*.md` + `development/roadmap.rst` + `release-notes.rst` if version-relevant |
| Install/distribution artifact | `getting-started/installation.rst` table + `:planned:` markers flipped per artifact |
| Compiler pipeline/ownership move | `compiler/*.rst` (`architecture/kcc/kir/bootstrap`) + `docs/inventory/compiler-dependencies.json` |

Enforcement (already CI, extended by L-6): Sphinx builds with `-W` (warnings are errors) + required-page existence check (`ci.yml:14-63`); L-0 adds the keyword-table test (docs↔lexer both directions). Sphinx `linkcheck -W` must stay green. Forbidden: code-only commits for user-visible changes; doc-only claims without a gate (every documented surface names its test).

### Area attention checklist (every area, every feature — audited 2026-09-21)

| Area | Pages (all exist) | Known staleness → home |
|---|---|---|
| Getting Started (`installation/first-program/first-project/project-layout/build/run/test/workflow/workspace/dependencies/modules/debug/profile`) | 13 pages | `installation.rst` runtime list mixes build/conditional deps → L-0; `first-program` vs `first-project` overlap → merge verdict L-0 |
| Language (`fundamentals/types/constants/functions/control-flow/structs/modules/memory-model` + variables/strings/collections/enums/errors/concurrency) | 14 pages | `fundamentals.rst` keyword list + nesting claim → L-0(c); `memory-model.rst` map-struct → rewritten by S-0; `constants.rst` (folding) → L-2; `modules.rst` (assembly) → I-2; `concurrency.rst` (Go-only) → I-4 |
| Stdlib (`string/collections/io/encoding/crypto/testing` + `numerics/net/http/db/core/math/not-implemented`) | 13 pages | MISSING pages `async/system/gpu` (even as not-implemented/experimental stubs) → I-2 triage; 6-module mental model → 10-module truth (L-0) |
| Tools (`cli/build/run/test/debug/profile/fmt/target` + `lsp/packages`) | 10 pages | MISSING pages `lint/bench/clean/explain/kir` (fold into `cli.rst` or stand alone) → L-5 |
| Targets (`cross-compilation/target-triples/host-targets/compute-targets`) | 5 pages | missing rows `riscv64`/Android/iOS/freestanding → L-4/S-3 as each lands |
| Internals (`architecture/kcc/hir/ssa/pipeline/runtime/kir/bootstrap`) | 9 pages | ownership paragraphs update per I-phase (rulebook table); `pipeline.rst` rewritten at I-3; native section at I-5 |
| Status (`index/scope/implemented/experimental/planned/beta/compatibility/migration-beta1/rc-checklist`) | 9 pages | `scope.rst`↔`stable-api.rst` agreement → L-0; bucket moves per phase (rulebook table) |
