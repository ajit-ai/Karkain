# Karkain Development Conventions

## Branch Workflow (MANDATORY RULE)

After EVERY Phase completion and successful test run:
1. Commit all changes to `develop` branch
2. Merge `develop` into `main` branch
3. Push both branches to origin

This ensures `main` always reflects the latest working state.
NEVER skip this step. This is a hard rule, not optional.

### Release cadence (version-wise, since the 1.2.0 line)

Work is planned by **version** (`docs/audit/KARKAIN-VERSION-PLAN.md`) but
executed by **increment** — an increment *is* a phase. The two levels are:

1. **Per increment (every time):** code + gate + audit note + this file's
   record → commit `develop` → merge `main` → push. The push triggers the
   full CI matrix (`.github/workflows/ci.yml`, `on: push`) and the GitHub
   Pages deploy (`.github/workflows/docs.yml`, push to `main`). No
   long-lived version branch, ever — `main` stays shippable at all times.
2. **Per version close (one go):** the whole ceremony —
   `docs/release/vX.Y.Z-CHECKLIST.md` → full QA battery (units, corpus
   both engines, conformance, probes, every gate file, Sphinx `-W` +
   linkcheck, `verify-examples`, install/verify-install, rc-journey) →
   version identity (`VERSION`, Go + kcc banners, generated headers, LSP,
   every version-pinning test) → docs flip (`roadmap.rst`,
   `compatibility.rst`, `scope.rst`, `stable-api.rst`, `installation.rst`,
   release notes) → final `develop` → `main` merge + push → **tag
   `vX.Y.Z`** (owner-only; the tag is what triggers the CI `release` job
   that publishes the 13 archives + `checksums.txt`) → GitHub Release →
   Pages → post-publish verification against the *shipped* binary.

Version scope is frozen when its checklist is written: new ideas go to the
next version or an explicit carry-over list, never silently into an open
one.

## Roadmap

**Version-wise plan (authoritative forward view):
`docs/audit/KARKAIN-VERSION-PLAN.md`** — 1.1.0 (in-tree, ceremony pending),
1.2.0 "Sovereignty I" (increments 150, 151, 152, 154, 165), 1.3.0, 1.4.0,
1.5.0, 1.6.0, 2.0.0. Per-version exit criteria, increment queue, NFR targets
and the library backlog live there. This file remains the **completion
record**; `ROADMAP.md` is the historical phase log (Phases 1–49).

Current increment: **post-149** — increment 149 (PE + Mach-O writers, Win64
boundary, PEB bootstrap) complete; next queued increment is 150 (native Value
model + Windows/macOS native CLI targets) under version **1.2.0**.
Increments 138–149 are recorded below (version **1.1.0**, all in-tree; only
the owner tag/archive ceremony is outstanding). Historical chain:
Phases 50–106 complete, 107 (Concurrency
Runtime) complete, 108 (WASM target) complete, 109 (Standard Library v2)
complete, 110 (Profiling & Diagnostics) complete, 111 (Cross-Compilation)
complete, 112 (Language probe hardening/debug trace/testing module) complete,
113 (Developer Preview docs+status model) complete, 114 (Complete Example
Corpus: 50 files / 15 categories + parity gate) complete, 115 (Developer
Preview Readiness: LICENSE/CONTRIBUTING/COC, honest README + docs +
release notes, unified v0.115.0 version identity across Go CLI + kcc +
generated headers, "Did you mean" typo hints + friendly missing-file errors,
fresh-checkout gate `pkg/cli/phase115_developer_preview_test.go`,
CI vet/phase-114/115 steps, Phase 115 audit reports) complete, 116 (Developer
Examples & Real-World Programming Corpus: 15 dash-named categories under
`examples/`, 50 `.kark` files with honest `Status:`/`Engine:` headers,
top-level `examples/README.md` + per-category READMEs, 4 new both-engine
examples — array stack `11_stack`, FIFO queue `12_queue`, 0/1-knapsack DP
`13_knapsack`, token statistics pipeline `05-data/04_token_stats` — pinned
byte-identical in the Phase 114 gate (45→49 goldens), `TestPhase116`
corpus-metadata gate, verify-examples.ps1 dash-directory fix,
`examples/EXAMPLES.md` + Sphinx pages + example-matrix updated, CI
`TestPhase116` step; full Go+kcc gates, unit regressions, vet, strict Sphinx
`-W` HTML+linkcheck green. NOTE: Phase 116 was developed per a local-only
amendment — no commit/push performed; directions to commit when reviewed.)
and 117 (Developer-examples were finalized under Phase 116; **117 — Beta 1
Readiness & Hardening** is complete: the public label became
**🧪 Karkain Beta 1** (v0.117.0); Go↔kcc parity hardened — unparenthesized
`while` accepted identically by both parsers (`pkg/parser/parser.go` +
`src/compiler/parser.kark` mirroring `parseIf`), semantic build/run gating on
BOTH engines (Go `runSemanticPreflight` in `pkg/cli/checker.go` +
`kccCheckPreflight` in `pkg/cli/kcc_engine.go`: undefined identifiers now
reject `build`/`run` with `error[K002]`/`error[K102]` + `ExitCompile(3)`),
recoverable parse errors no longer swallowed by run/build (`parseVarDecl`
name-token guard in `pkg/parser/parser.go` + `parseSourceWithErrors` in
`pkg/cli/commands.go` — `func main() { let = 42 }` exits 3), numeric error
codes documented by `explain` (K001-K008, K100, K101-K113, hyphenless),
stdlib edge-case parity tests (`pkg/cli/phase117_stdlib_edge_test.go`),
Beta gate `pkg/cli/phase117_beta_test.go` (while parity, semantic gating,
exit-code contract, both-engine stdlib parity, kcc test runner, target
matrix, debug+prof diagnostics, beta artifacts), `scripts/
beta-fresh-checkout.ps1`, `docs/source/status/beta.rst`, readiness scorecard
`docs/audit/PHASE-117-BETA-1-READINESS.md`, CI `TestPhase117` step, and the
v0.117.0 unified version identity across the Go CLI (`versionString` both
engines), generated headers, `VERSION` (0.117.0-beta1), docs and release
scripts.)
complete. **118 — Beta 1 External Validation & Release Candidate Readiness**
complete: verdict **RC READY** (evidence in `docs/audit/
PHASE-118-RELEASE-CANDIDATE-READINESS-FINAL-REPORT.md`; the 21-item checklist
`docs/source/status/rc-checklist.rst` — satisfies today, owner-only decision
to actually cut the `v0.117.0-rc1` tag on the existing CI release pipeline);
deliverables: release hygiene (`releases/` stale v0.14.0 artifacts deleted,
`.gitignore` covers `/releases/` `/.build/` `/artifacts/`, SECURITY.md,
`.github/ISSUE_TEMPLATE/*` bug/feature/docs/config), docs pages wired and
Sphinx-clean (`status/{scope,compatibility,migration-beta1,rc-checklist}.rst`,
`reference/stable-api.rst`,
`development/{release,reporting-bugs,feature-freeze}.rst`,
`getting-started/{first-project,workspace}.rst`, toctrees updated),
two-member workspace example `examples/workspace/`, install scripts
(`scripts/install.ps1`, `scripts/install.sh`, `scripts/verify-install.ps1`,
deterministic exit codes 0/1/2/3), external-developer journey simulation
`scripts/verify-rc-journey.ps1` (11 steps), automated gate
`pkg/cli/phase118_rc_test.go` (9 subtests), `engine:` line in `karkain config`
(`EngineFromEnv()`), CI Phase 118 step + required doc-tree pages +
Phase 115 step rename, README example count 46→50 + milestone 118 +
reporting/security links, showcase README refreshed; **unrelated stale-test
fix**: `pkg/lsp/lsp_test.go` `TestLSP_RealTimeSync` fixture `print(1);` → `let = 42`
(`print(1);` parses valid since the parser evolved — failure verified
pre-existing on pristine HEAD). Regressions: all unit suites, CLI Phase
114/115/116/117/118 gates, conformance 59/59, probes 11/11, verify-examples
49/0/5, Sphinx html `-W` + linkcheck `-W` 0 warnings, beta-fresh-checkout,
install/verify-install, rc-journey 11/11, `go build`/`go vet` full tree green.
Documented environmental (NOT defects): bootstrap stage-2 SEGFAULT + one
combined-CLI-gate OOM crash on the ~4GB host — the known kcc-build OOM class
(individual gates pass in isolation); no release tag cut yet (binaries listed
as Planned in `installation.rst` honestly).
Project status:
**Karkain 1.0.0 (Stable)**. NOTE: the Phase 91 historical
"KARKAIN 1.0 — RELEASE READY" record was superseded; the public label
progressed Developer Preview (v0.115.x) → Beta 1 (v0.117.x, phases 117–118)
→ **1.0.0 (Stable)** at Phase 119. The honest label is now 1.0.0, and the
Phase 119 language-QA final report (`docs/audit/
PHASE-119-LANGUAGE-QA-FINAL-REPORT.md`) documents the classification
(Stable Core / Experimental / Planned / Known Limitation), the final root
inventory, and the READY FOR 119A verdict. `VERSION` = `1.0.0`.
Also completed: **119 — Language QA / Public Repository Finalization &
Karkain 1.0.0 Stable Release Preparation** (verdict **READY FOR 119A**;
cleanup of superseded material — committed generated `compiler/*.c`,
stale `scripts/{build,package,release}.*`, legacy `std/` stubs,
`pkg/stdlib/*.go` placeholders, prior-phase `src/*.kark`+`src/main.rs`
prototypes, 24 loose legacy `examples/*.kark` [kept `app.kark`, referenced
by CLI/pm tests], `docs/ROADMAP-PRODUCTION.md`; untracked debris removed
[bin/, temp/, docs/build/, root exes]; collateral updates [root-example
paths in `cmd/karkain/main.go` help, `ci.yml` release-notes.html path,
`pkg/pm/manager.go` `findStdlib` stdlib-only, SPEC lines 33/510, 5 docs
citing `pkg/stdlib` reworded]; unified **v1.0.0 / Stable Build** identity
across `VERSION` (authoritative), Go CLI + kcc banners, generated headers,
`src/compiler`, scripts, issue templates, docs (README/Beta-1 label flips,
release notes rewrite, compatible status/scope/index/installation/
first-project/stable-api pages; historical beta/rc/developer-preview/
migration-beta1 pages kept with current-status pointers); master QA gate
`scripts/qa/run-full-qa.ps1` + `docs/release/KARKAIN-1.0-{CHECKLIST,
QA-PLAN,RELEASE-NOTES}.md`; QA battery measured and green — units (all
non-cli packages), phase-88, phase-114 GoEngine 49/49 + KCCParity 49/49
byte-identical, phases 115/116/117/118 gates, conformance 59/59, probes
11/11, verify-examples 49/0/5, Sphinx html `-W`+linkcheck `-W` 0 warnings,
beta-fresh-checkout, install/verify-install, rc-journey 11/11,
`go build`/`go vet` green; the two documented ~4GB-host environmental
classes re-proven non-regressions — bootstrap stage-2 SEGFAULT reproduced
identically on pristine HEAD worktree 1017068, combined-CLI-gate OOM crash
with every constituent gate passing individually; **NO tag/release cut**
(owner-only, listed as Planned in `installation.rst` honestly).)
Also completed: **120 — GA Envelope + Compiler Independence / KIR v1 text
emitter** (verdict **COMPLETE**; GA/public-material envelope — LICENSE,
AUTHORS, CODE_OF_CONDUCT, `.github/workflows/ci.yml` Phase-120 steps, README
export-permissions/contributing sections, Sphinx conf.py copyright/GA
metadata, developer-preview/installation docs [binaries still listed Planned
honestly], owner attribution, validated by `pkg/cli/phase120_ga_envelope_test.go`;
compiler-owned intermediate representation: **KIR v1 text** — a deterministic,
line-oriented, depth-indented rendering of the parsed AST emitted by
`kirEmit` + 5 accessor helpers in `src/compiler/kir.kark` (the first compiler
component written ENTIRELY in Karkain and owned by the self-hosted kcc;
parser-agnostic S-expression forms, raw string-token echoes; `fileBaseName`
header so output is path-location-independent — byte-identical across temp
sandbox runs), exposed by `kcc kir` (dispatch in `src/compiler/main.kark`) and
the CLI `karkain kir` (`pkg/cli/kir.go` KCCKirCommand — Go side only routes:
syntax preflight, assembly, dispatch; NO Go KIR emitter, no fallback);
space-form `print x` (kcc-parity) now accepted by the Go parser
(`parsePrint` optional parens in `pkg/parser/parser.go`) so the Go preflight no
longer rejects KIR-renderable programs; gates `pkg/cli/phase120_kir_test.go`
(EmitterEndToEnd markers, ByteDeterminism, SpaceFormPrintParity, SelfHosting —
kcc renders kir.kark itself, ExampleCoverage — `examples/self-hosting/kir/
main.kark` prints `15/20/1/15` identically on both engines, ExitCodeContract),
plus `src/compiler/main.kark` `dirName`/`baseName`/`endsWith` user helpers
(later WIP-removed); regressions green: all unit suites, CLI phase 114/115/116/
117/118/119 gates, conformance 59/59, probes 11/11, verify-examples 49/0/5,
Sphinx html `-W` + linkcheck `-W`, `go vet`, `go build`. Known limitation
(document, NOT a defect): kcc-per-file KIR emission assembles the whole
`src/compiler` tree (~6399 KIR lines, ~66–95s on the 4GB host) — pre-existing
Phase-120 emitter cost, gates sized accordingly. Reports:
`docs/audit/PHASE-120-KIR-FINAL-REPORT.md`.)
Also completed: **121 — Compiler Independence Foundation** (verdict
**COMPLETE**; plan-driven — mandatory BASELINE report
`docs/audit/PHASE-121-BASELINE-COMPILER-INDEPENDENCE.md`, ONE target chosen:
**Target E** — KIR becomes an engine-internal invariant rather than a CLI-only
artifact; only existing `.kark` extended, no second compiler architecture.
Implementation: `kirVerify` + 4 helpers (`kirIndentCount`, `kirIsDigits`,
`kirIsStructureLine`, `kirHasLineNo` [last-` line: `-occurrence scan so
expression-embedded match-arm markers can't confuse the suffix check]) in
`src/compiler/kir.kark` verify the KIR v1 structural contract — exact `KIR v1`
header, basename `source:` line, two-space indentation per level, monotone
+1 nesting, and ` line: <decimal>` statement suffixes exempting bare `block`
introducers and `stmt <type>` fallbacks — written in Karkain with only
builtins/user helpers (checked by the Phase 99 self-hosted checker, zero Go);
`checkFile` runs it on the DEFAULT kcc check path after the Phase 99 type
checker — silent on success, `error[K121]` + no `[ok]` + exit 3 on drift;
new `verifykir` kcc command + `verifyFile` (`[ok] kir text: N lines` +
`[ok] kir verify: N lines ok`); CLI `karkain kir --verify <file>` via new
`KCCKirVerifyCommand` in `pkg/cli/kir.go` + `--verify` flag routing/rejection
in `cmd/karkain/main.go`; gate `pkg/cli/phase121_compiler_independence_test.go`
(VerifyCommand + count agreement + determinism, CompilerSelfVerify [kir.kark
self-verify = 6399 lines + default check of full compiler assembly through the
checkFile hook], DefaultCheckInvariant, VerifierPresence, ExitCodeContract);
deleted the orphaned `pkg/cli/phase121_module_resolution_test.go` (guarded the
rejected hard-coded WIP `resolveModulePath`/`replaceDots`/
`loadSourceWithModules`/`extractImports`/`startsWith`). Go dependency
classification final: KEEP (CLI routing, main.go contract), BRIDGE
(`kccAssembleSource`/module resolution stays Go — the documented next
bottleneck), REMOVE-only-for-IR (no Go KIR). Self-hosting Q4 flipped
NO→YES (compiler emits AND verifies its own IR on the default path). Docs:
`kir.rst` (interface + "Structural verification" section + boundaries),
`docs/inventory/compiler-dependencies.json` (`kirVerify`, `structural_rules`,
engine-invariant status). Regressions green: Phase 121 gate 164.3s, Phase 120
85.8s, Phase 99/117/118 204.7s (incl. CompilerSourcesTypeCheck 71.5s), Phase
114 497.2s, conformance+probes 132.5s, phases 115/116, verify-examples
49/0/5, all pkg suites, `pkg/codegen` (incl. carried-over correctness-sweep
gate `phase121_correctness_test.go`), `go vet`, `go build`. NUMBERING
RECONCILIATION: the untracked draft `ROADMAP-PLAN.md` numbered 120=GA envelope,
121=correctness sweep; the driving plan doc names 120=GA+KIR emitter and
121=Compiler Independence Foundation — the correction-sweep (closures/`fn`,
`alloc(T,n)`/`free` hardening) is retained as carried-over hardening under its
own green gate. `ROADMAP.md` remains a stale pre-50 planning doc (encoding-
corrupted); AGENTS.md + audit reports are the authoritative completion record.)
Also completed: **122 — Compiler Pipeline Ownership** (verdict **COMPLETE**;
plan-driven — mandatory BASELINE `docs/audit/
PHASE-122-BASELINE-COMPILER-PIPELINE-OWNERSHIP.md`; the kcc production CLI
path had NO self-hosted input composition — Go `kccAssembleSource`/
`resolveSourcesRun` composed the compiler's input text, so Phase 122 moves
**flat** project assembly (no `karkain.toml`, every import `std.*` or sibling
file/dir) INTO the compiler: `src/compiler/main.kark` gains `assembleProject`
(module `import <name>` declared on a source line with `import std.x` /
`import math` — resolved via `moduleImportNames`, `resolveStdlibModule`,
`resolveUserModule`, `siblingContent` [sorted, excludes the root and any
sibling declaring its own `func main(`], `moduleDirSource` [dir-of-files then
single-file fallback], `stripModuleImports` [preserves blank lines → exact
line numbers; C import blocks untouched], `collectKarkFiles`/`sourceLines`/
`stripLineEnd`/`importNameOf` CRLF-robust, no new builtins) and every driver
entry — check/build/run/kir/verifykir — now compiles through it (lines
116/170/196/224/253). **Defect root-caused and fixed**: stdlib discovery was a
blind walk-up that adopted the leftover demo tree `examples/stdlib/collections`
(Phase-109 demo with its own `func main`) shadowing the real repo `stdlib`
(breaking `examples/stdlib_v2` on flat paths); `findStdlibRoot` is now
MARKER-GATED — a candidate stdlib root must carry the canonical `std.string`
module, and the sandbox `<rootDir>/lib` is checked first. Go BRIDGE (kept
deliberately): `kccStageInput`/`flatAssemblyEligible`/`findKarkainStdlib`/
`kccMirrorFlat` in `pkg/cli/kcc_engine.go` mirror flat projects into a temp
sandbox (root + siblings + `lib/` copy of the real stdlib tree) so kcc
assembles alone; manifest/workspace/PM and transitive non-local assemblies
keep Go `kccAssembleSource` (documented next bottleneck); `kccTargetPreflight`
(NPU) unchanged. Gate `pkg/cli/phase122_pipeline_ownership_test.go` (7
subtests — PASS 191.0s): CompilerSelfCheck (default kcc `check` of the
compiler's own tree via assembleProject, 94.7s), KIRContinuity (`kir --verify`
of `kir.kark` still exactly 6399 lines + 6399 verified), ModuleSystem
(`examples/module_system` 42/9 + sibling-join KIR ordering + byte-determinism),
StdlibV2 (`examples/stdlib_v2` four `std.*` imports byte-identical golden to
the Phase 109 Go-engine run + KIR has the real `collections` module + one main),
StdlibShadowMarker (regression: fake local `stdlib/` with its own `func main`
is never adopted — direct kcc AND CLI print `7`, never `SHADOW-BAD`/`999`),
MissingModuleRejected (`import std.nope` → CLI exit non-zero naming `nope`/
`not found`, never `[ok]`; direct kcc prints `error[K122]`), WiringPresence
(guards the 6 Karkain helpers + `kccStageInput`/`flatAssemblyEligible`/
`findKarkainStdlib`/`kccMirrorFlat` + kir.go staging). Negatives: kcc itself
always exits 0 — CLI maps `[ok]`-less output to `ExitCompile(3)` and pre-empts
unresolvable modules at the Go legacy layer (exit 1). Regressions green:
Phase 122/121/120 gates, Phase 119/118/117/116/115 + `TestPhase120_GaEnvelope`,
Phase 114 corpus (GoEngine+KCCParity goldens), conformance 59/59 + probes
11/11, phases 111/110/105/106/107/103/102(foundation both engines)/100/101/99/
98/97/96/95 + non-phase CLI unit tests (389.6s), all non-CLI pkg suites,
`go vet`, `go build`. Pre-existing failures re-proven at pristine HEAD
(477f448) — documented, NOT regressions, NOT modified: legacy Phase 88–90
`build --target c23`-writes-`.c`-beside-source tests (kcc-default era Phase 97
moved non-compile-only builds to sandbox `*.c23`; pass only under
`KARKAIN_ENGINE=go`) and the bootstrap Stage-2 SEGFAULT (`TestBootstrap_
Stage2SelfHosting`/`BitwiseIdentity`, exit 0xc0000005 — documented ~4GB-host
kcc-OOM class; Stage-1 GO-engine transpile succeeds). Docs: final evidence
report `docs/audit/PHASE-122-COMPILER-PIPELINE-OWNERSHIP-FINAL-EVIDENCE-REPORT.md`,
`docs/inventory/compiler-dependencies.json` (new `assembly` component,
flat-path ownership, Karkain-owned status).)
Also completed: **123 — Language Core Completion / Enum ADT Parity** (verdict
**COMPLETE**; enums and `match` now byte-identical on both engines with checker
validation. Root-cause fixes in `src/compiler/*.kark`: (1) statement-position
`match` is wrapped by `parseBlock`'s else branch into `ExprStmt(Match(...))`,
so `checkStmt`'s Match branch never fired and match arms were never type-checked
— `checkExpr` now has a `Match` case (mirrors the checkStmt branch: validates
`matchValue`, enum-variant arms → `error[K106]`/`error[K113]`, walks arm pattern
values + bodies); (2) the checker dispatched on node type `"Dot"` but
`getNodeType(NODE_DOT)` returns `"Member"` — fixed, so `MissingKind.Purple` in an
expression now raises `error[K102] undefined identifier 'MissingKind'` on kcc
(matching Go's K002; closes the previously-documented member-access gap);
`error[K113] unknown enum variant 'Color.Purple' in match arm` and
`error[K106] use of undefined enum type` now fire in BOTH match arms and
expressions. Parity: payload destructuring `Shape.Circle(r) =>` rejected
identically on both parsers (`expected '=>' in match arm, got '('`); program
goldens byte-identical Go↔kcc (13_enums `1/1/100/200/300/0`, 14_adt_match
`10/20/30/1`); documented intentional strictness (kcc rejects unknown enum
variant/enum at check time K113/K106, Go check defers to C compile — both
reject). Match-arm **bodies** remain unchecked on both engines (parity; e.g.
`_ => 2 + nope` passes check, fails at C compile on both). New permanent
corpus: `examples/01-fundamentals/13_enums.kark` + `14_adt_match.kark` (pinned
in Phase 114 gate, 49→51), `conformance/012_enums_test.kark` (5 tests, passes
both engines), negative fixtures `examples/type_errors/err15_unknown_enum_
variant_match` (K113) / `err16_undefined_enum_match_arm` (K106) / `err17_
undefined_enum_expr` (K102 — both engines resolve dotted base) / `err18_unknown_
enum_variant_expr` (K113); gate `pkg/cli/phase123_cli_test.go` (3 subtests:
EnumMatchChecker positive+negative, MatchArmParseErrorParity — drives the real
binary with CombinedOutput because Go-engine and Phase 105 preflight diagnostics
render to stderr via `renderDiagnostics` at `pkg/cli/check.go:106`, both engines
route parse errors through the Go-side kcc preflight, EnumVariantExpressionKIR).
SPEC.md: Enum row status now ✓, §3.5 Match documents `EnumName.Variant` tag-only
patterns + `=>` requirement + K106/K113, capability summary gains an Enums row,
Known-Gaps enum-payload row now Phase 123 (payload recorded but semantically
dropped; no payload destructuring). Example counts updated 50→52 in README.md,
`examples/EXAMPLES.md` (fundamentals 12→14, total 49+1 test-mode+2 experimental
+4 planned-dirs), `docs/source/examples/index.rst` (52 files), category README
determinism note (both-engine match parity now). `go vet ./pkg/cli/` clean; all
three Phase 123 subtests PASS; Phase 99 gate still green after checker changes
(123.3s); `kcc check src/compiler/main.kark` → `[ok]`. Write tool / PowerShell
`Set-Content -Encoding UTF8` write a UTF-8 BOM (`EF BB BF`) that the Go lexer
rejects — test fixtures must be BOM-stripped. Reports:
`docs/audit/PHASE-123-ENUM-ADT-PARITY-FINAL-REPORT.md`.)

Also completed: **126 — Standard-Library Numerics Module (std.numerics) + both-engine corpus 59/59 byte-identical**.
Module: `stdlib/numerics/numerics.kark` (~520 lines, 40 `numerics_*` funcs, canonical per-dimension print). Example: `examples/09-ai/03_numerics_forward.kark` pinned with 15-line golden; Phase 114 gate 59/59 both engines PASS (Go 255.1s, kcc 433.2s). README/EXAMPLES counts updated 52→59. Generated debris cleaned.

Also completed: **124 — Compute Targets: GPU / NPU / Quantum Model** (verdict **COMPLETE** — previously unrecorded in AGENTS.md; `karkain target` exposes a Phase 124 experimental compute-target catalog alongside the host/target listing: `cpu`, `simd`, `wasm32-wasi`, `gpu-experimental`, `npu-experimental`, `quantum-experimental`, with per-target capability views (Family / Maturity / Memory model / Capabilities / KIR classes / Tensor ops); unknown targets get a deterministic usage error (`generative-ai-9000` example); maturity levels: gpu `experimental` (host-device), npu `experimental` (device-only, synchronous single-invocation inference), quantum `research`; gate `pkg/cli/phase124_compute_test.go` (catalog, detail, CLI E2E) + CI "Run Phase 124 compute-target gate" step; commits `a5f762e` / `0234bb2`, docs page wiring included.)

Also completed: **127 — Bootstrap Memory Guard (low-RAM OOM/SEGFAULT class)** (implemented — verification pending on the ~4GB host; the Go toolchain itself OOMs below ~500 MB free/100% commit charge, see Verification in the report). `pkg/bootstrap` gains a dependency-free memory probe + guard: `CheckBootstrapMemory(stage)` runs at the top of `runCompileStage` for stages 2/3 ONLY (stage 1 = Go front end, always-working, unguarded), probing available RAM via `GlobalMemoryStatusEx` on Windows (stdlib syscall + unsafe, no cgo, no new dependency) and `MemAvailable` from `/proc/meminfo` on Linux; unsupported platforms pass through (guard disabled, no guessing). When available RAM < threshold it returns a clean `error[K127]` naming the stage and the MiB shortfall instead of letting the native self-hosted compiler exhaust the address space (the documented `0xc0000005` SEGFAULT class on ~4GB hosts). Default minimum 1.5 GiB; env override `KARKAIN_BOOTSTRAP_MIN_MEM` (plain bytes or K/M/G/KiB/MiB/GiB power-of-two suffix; `0` disables; invalid values reported, never swallowed). Deterministic unit tests `pkg/bootstrap/memcheck_test.go` (injected 512 MiB probe — host-RAM-independent: parseMemBytes cases + guard on/off + errInsufficientMemory errors.Is + no-probe pass-through). Analysis/housekeeping: 124-A (5-min subprocess timeout) and 124-C (CI `-p 1`) were ALREADY in place; `pkg/parser/parser.go` had been emptied (0 bytes) in the working tree alongside WIP `ClosureExpr` additions to `arena.go`/`ast.go`/`captures.go` — restored from HEAD (the additive AST changes compile against it); the untracked `fix_parser.py` chase after a `make(map[string)bool}` typo did nothing (typo does not exist). Reports: `docs/audit/PHASE-127-BOOTSTRAP-MEMORY-GUARD.md`, `docs/audit/PHASE-127-POST-GA-ROADMAP.md`.)

Also completed: **128 — Release 1.0.0 Cut & Distribution preparation** (verdict
**COMPLETE — PREPARATION; the tag-cut ceremony itself is owner-only**).
Audit findings: tag `v1.0.0` EXISTS on origin but points at `f23c024`
(2026-09-13) which is **behind current main** (`9dfdc76`, Phase 126, 2026-09-18)
— Phases 125A/126/127 are not on the tag; **no GitHub Release exists** (page
shows only legacy "Phase 17 COMPLETED (v0.14.0)"); version identity is already
unified at 1.0.0 across `VERSION`, Go CLI banner, kcc banner and every
generated-C/QIR/QASM header (grep-verified; no stale `0.117.0`/`Beta 1 Build`
outside historical pages/tests). Deliverables: `docs/release/KARKAIN-1.0-RELEASE-NOTES.md`
rewritten to the real 1.0.0 story (59-golden corpus, stdlib now
`std.string/collections/io/encoding/crypto/testing/numerics/net/http/db`, KIR +
compiler-pipeline ownership, enum/match parity, closures honestly
codegen-deferred, GPU/NPU/quantum + registry + container Planned);
`docs/source/release-notes.rst` refreshed (59 pin, shipped stdlib/numerics/net/
db/http, self-hosted+KIR rows, corrected known-gaps — net/db/http no longer
"Planned"); `docs/source/getting-started/installation.rst` release-ready
(13-archive table + `checksums.txt` SHA-256 verify commands for bash and
PowerShell + a `:planned:` release-pending note to flip at cut);
`docs/release/KARKAIN-1.0-CHECKLIST.md` gained section 0 (release state audit)
and a 6-step owner runbook (commit/merge/push → **re-point the unreleased
`v1.0.0` tag** to current main via `git tag -f` + force-push → CI release job →
post-publish verify with `scripts/verify-rc-journey.ps1`/`verify-install.ps1`
against the shipped binary → flip the docs marker → announce);
`docs/source/development/roadmap.rst` extended 119→127 + GA-1/2/3 future with
pointer to `GENERAL-AVAILABILITY-ROADMAP.md` (dropped the now-wrong
"networking/databases/web Planned" list). Gate: existing Phase 118/119 gates +
owner ceremony. Verification note: doc-only changes this phase; full build
deferred to the post-cut QA pass.)

Also completed: **129 — 4GB Bootstrap Battle** (verdict **COMPLETE** —
documentation + hardening shipped; live-host stage-2/3 verification pending,
see report). Deliverables: (1) **progress heartbeat** — `startProgress`
(45 s liveness ticker, self-terminating on context cancellation) wraps every
`runCmd`/`runCmdOutput` in `pkg/bootstrap/bootstrap.go` (stage-1 `go build`,
all three transpile steps, gcc links) with child-derived labels, so the 2–3
minute transpiles (Phase 99 raised the timeout 2→5 min for exactly this) read
as liveness reports instead of a silent "hang"; (2) **CI memory cap** —
`.github/workflows/ci.yml` test job exports `GOMEMLIMIT: 8GiB` job-wide on top
of the existing Phase-107 `-p 1` sequential-package fix; (3) **Windows 11 4GB
memory guidance** in the report — fixed 16 GB pagefile steps (AutomatedManagedPagefile=false
+ `PagingFiles` 'C:\pagefile.sys 16384 16384'), hog list (MsMpEng ~420 MB,
browser/editor sessions), isolation rule (full-tree `go test ./pkg/...`
OOM-mixes on 4GB; every gate passes isolated), status commands
(`TotalVisibleMemorySize`/`FreePhysicalMemory`/`Win32_PageFileUsage`);
(4) **kcc build RSS measurement plan** — PowerShell 1 s poll of
`karkain-compiler1/kcc/cc1/gcc` WorkingSet64, target stage-2/3 total peak RSS
≤ 2.5 GB, levers documented (GOGC, C TU splitting, guard threshold); table
open for the owner to fill. Static checks: `gofmt -e` clean on all
`pkg/bootstrap` files (the `gofmt -l` flags are the repo-wide CRLF artifact,
verified identical for committed files like `args_test.go`). **Live
verification (2026-09-19, memory transiently freed to 0.6 GB):** go vet clean;
memcheck 3/3 PASS; TestArgs_ + TestBootstrap_Stage1Compilation 7/7 PASS
(stage-1 transpile+gcc 18.7 s; heartbeat wiring harmless);
TestBootstrap_Stage2SelfHosting aborts with the **clean `error[K127]`** (642
MiB available < 1536 MiB required) — the historical `0xc0000005` SEGFAULT class
is now an actionable diagnostic on this host; stage-2 success on a ≥1.5 GiB-free
host remains the open item (page-file guidance in the report). Report:
`docs/audit/PHASE-129-4GB-BOOTSTRAP-BATTLE.md`.)
Also completed: **130 — Closures / fn Values: Capture-Mutation Completion**
(verdict **COMPLETE**; GA-2 milestone. Go↔kcc capture MUTATION validated and
byte-identical, corpus seeded, two happy boundaries root-caused and documented.
Corpus: `examples/closures/00_capture_mutation.kark` (a closure whose body
ASSIGNS to a captured `var`: C emits the pointer alias
`#define counter (*_env->karkain_cap_counter)`, the assignment writes THROUGH
the alias `counter = binary_op(counter, "+", make_int(1))`, binding site wires
`_e.karkain_cap_counter = &counter`; enclosing scope sees every increment —
golden `1/2/2/3/3`, **byte-identical on both engines verified live**) and
`examples/closures/01_nested.kark` (typed lambdas `fn(a int) int`, nested inner
captures base through the outer env pointer + outer local plain address,
invoked twice in one expression — golden `32/28`, **kcc leg genuinely ran this
host and passed**, not skipped). Root-caused boundaries (documented, NOT
defects, both from the same root cause — closures desugar to plain functions
with no first-class Value cell): (1) **no first-class function values** —
calls through an expression (`ops[0](5)`) are rejected at parse with K001
`unexpected token ')'` (pinned by `TestPhase130_FirstClassBoundary`), no
`func(T) R` type syntax; (2) **a closure variable cannot be free-captured by
another closure** — `let twice = fn... { ... apply(...) ... }` passes `check`
on both engines but fails at C compile because the env init emits `&apply` for
a name with no C declaration, identically on both engines (parity preserved;
pinned by `TestPhase130_ClosureVarCaptureBoundary`). Gates: new
`pkg/codegen/phase130_closures_test.go` (capture-mutation write-through +
nested pointer-chain markers) and `pkg/cli/phase130_closures_test.go` (Go
goldens, kcc goldens — skip-with-note when the Phase 127 `error[K127]`
low-RAM guard fires, FirstClassBoundary, ClosureVarCaptureBoundary; the CLI is
built ONCE via `sync.Once` into a persistent temp dir so three subtests do not
each spawn a go-build/gcc pipeline on the ~4GB host — the documented Phase 127
OOM class). Regressions green: `go vet` + `gofmt` on both gates, `pkg/codegen`
`TestPhase121_Capture*` + `TestPhase130*`, `pkg/parser` capture/lambda tests;
NO compiler/source changes this phase (fixtures + gates + docs only), so the
Phase 114 59-golden corpus is untouched by construction (`examples/closures/`
is a sibling category, not in the 15-category map; parity asserted by the
dedicated Phase 130 gate). Host notes: session hit the documented ~4GB classes
twice (Go toolchain OOM mid-test: `go: error obtaining buildID for go tool
compile: exit status 2`; gate passes consistently under `GOMEMLIMIT=2GiB`, and
in this session the host had memory so the kcc leg ran for real). Report:
`docs/audit/PHASE-130-FINAL-REPORT.md`.)
Also completed: **131 — Standard Library v3 + GA API Freeze** (verdict
**COMPLETE**; plan-driven per GA-2 milestone in GENERAL-AVAILABILITY-ROADMAP.md.
API freeze pass on all stdlib modules (10 modules frozen: string, collections, io,
encoding, crypto, testing, numerics, net, http, db); missing-dep cleanups verified
(both-engine byte-identical compilation for Phase 109/125A/126 modules); SemVer
policy documentation created (`docs/source/development/semver-policy.rst` —
comprehensive versioning and deprecation policy); deprecation mechanism
documented (policy infrastructure ready for future `@deprecated` attribute);
stable-api.rst updated (extended with Phase 125A/126 modules); Phase 131 gate
`pkg/cli/phase131_stdlib_api_freeze_test.go` (5 subtests: API freeze verification,
both-engine parity, SemVer policy documentation, deprecation mechanism, stable
API consistency — PASS 34.9s). Documentation additions: 4 new stdlib module docs
(`docs/source/stdlib/{numerics,net,http,db}.rst`), 1 new SemVer policy doc,
3 modified docs (stable-api.rst, stdlib/index.rst, feature-freeze.rst). Regressions
green: Phase 109 stdlib gate (68.8s), go vet clean, go build clean. GA-2 milestone
progress: API freeze foundation established, enabling future GA-2 phases (132–135).
Report: `docs/audit/PHASE-131-STDLIB-API-FREEZE-FINAL-EVIDENCE-REPORT.md`.)

Also completed: **131 — Reproducible Self-Host Gate** (verdict **COMPLETE**;
3/3 PASS 38.5s: stage-1 byte-reproducible across CWDs SHA `0c01de43f796b153`,
intentional-tamper divergence detected `0c01de43…` vs `0f677bd7…`, K127 guard
fires honestly; two vacuity defects root-caused and fixed — `binPath`+
`exeName` double-suffix stat and untranspilable `data[1]` tamper + missing
sibling staging. Gate `pkg/bootstrap/phase131_repro_test.go`.)
Also completed: **132 — Standard Library Freeze + GA API Freeze** (verdict
**COMPLETE**; GA-2 first step. Renumbered from the draft "131" strand by owner
decision — shipped 131 is the repro gate above. All 10 stdlib modules frozen
with docs pages + `stable-api.rst` entries; parity rewritten from a wordy
default-engine smoke check to 9 golden cases asserted byte-exact on Go AND kcc
(kcc leg ran live, 46.7s, no skip); SemVer policy + deprecation contract gated
precisely (`@deprecated` attribute honestly stays Planned, not claimed);
`semver-policy.rst` toctree-wired; Sphinx `-W` clean-room green; Phase 114
corpus 59→60 pinned (`03_mlp_forward` dead undefined call removed, golden
`0/0/0/0/1/3` both engines). Gate `pkg/cli/phase132_stdlib_freeze_test.go`
6/6 PASS 92.7s. Report: `docs/audit/PHASE-132-STDLIB-FREEZE-FINAL-REPORT.md`.)
Also completed: **133 — First-Class `fn` Values** (verdict **COMPLETE**;
GA-2 owner-priority track. Closures flow (params, arrays, returns) and invoke
through values, byte-identical on both engines. New `IndirectCallExpr` (Go) /
node 96 `IndirectCall` (kcc); `TYPE_FUNC` cells `{canonical wrapper, heap
env}` + `karkain_call_fn` dispatch (non-function/arity → file:line runtime
errors); per-binding heap envs + cell dispatch for direct calls (except
self-reference) so rebinding/recursion stay correct (countdown golden `300`);
captures stay by-ref, and returning a capturing closure is rejected at check
(Go `error[K002]` / kcc `error[K114]`, exit 3). Corpus `examples/closures/
02_higher_order` (`11/22/6`), `03_array_call` (`15/10/20`), `04_factory`
(`101/102/21/21/21`), `05_recursion` (`300`). Root-caused along the way:
kcc node-id 39 collision, checker UAF segfault (alias-then-realloc-then-
restore), dead `"Binary"`/`"Unary"` tag strings (incl. a struct-field K102
cascade), wrapper use-before-def, prescan misses. Gates: new `pkg/cli/
phase133_closures_test.go` 6/6 (Go+kcc goldens live, check-clean, escape
rejection, pure factory, runtime negatives); Phase 130 boundaries retired to
promotion guards; K114 in `explain`; KIR pin 6910→7416. Full regression
green: every `pkg/...` suite, all CLI phase gates, conformance 59/59,
probes, `go vet`, `go build`. `func(T) R`, capture snapshots, WASM
indirect calls stay Planned. Report: `docs/audit/
PHASE-133-FIRSTCLASS-FN-FINAL-REPORT.md`.)

Also completed: **134 — Incremental Compilation v2** (verdict **COMPLETE**;
GA-2 infra track. Phase 105's whole-assembly cache upgraded to per-module
translation units: shared runtime TU (`karkain_runtime.h/.c`, internal
linkage + 9 extern shared globals, best-effort PCH) + one TU per source
module by top-level function ownership (duplicates already K107, exact
attribution) + content-keyed cached objects, linked together; manifest v2
(`RuntimeKey` = compiler key + flags + header text, self-invalidating;
`RuntimeObj`, per-module `Objects`); atomic multi-artifact `StoreArtifacts`;
monolith factored into reusable `prescanTables`/`emitSharedDecls`/
`emitFuncBodies(only)` with zero monolith behavior change; `buildSplitFlow`
falls back to v1 for non-gcc toolchains, concurrency/profiling programs and
native-link (exactly 137's work list). Gate `pkg/cli/
phase134_incremental_v2_test.go` 5/5 (naming/keying/store/round-trip +
emission structure via column-zero def lines + E2E: phase105 golden
`41/42/Hello karkain`, manifest v2 + runtime.o + 3 objects, no-op `3 reused`
in 79ms beating the ≤100ms target, body-only leaf edit → golden with exactly
`1 compiled`, `Clear` → full rebuild). Regressions green: `go build`,
`go vet`, `pkg/compiler` suite, Phase 105 E2E on the same split-flow code.
Report: `docs/audit/PHASE-134-INCREMENTAL-V2-FINAL-REPORT.md`.)

Also completed: **135 — Local Registry MVP** (verdict **COMPLETE
(local-only)**; GA-2 infra track, no network/daemon/database. Directory
registry `registry.json` + `packages/<name>/<version>/{manifest,source/,
sha256}` + `index/<name>` JSON; `ValidatePackageName` rejects
empty/separators/traversal/absolute/non-`[A-Za-z0-9._-]` with the path kept
under root; semver via existing `ParseVersion`/`LatestSatisfying`; immutable
versions (duplicate refused, contents intact); `FetchLocal` digest-verified;
single selector `--registry <dir>` > dep URL > `KARKAIN_REGISTRY` (empty is an
actionable error, http honestly refused local-only; explicit http refs keep
the pre-135 remote flow byte-identical). CLI: `pkg registry init <dir>`,
`--registry` on `add` (recorded in manifest)/`fetch`/`update`/`publish`
(local, no login); `import <dep>` resolves cached registry dirs through
existing `DependencySources` assembly. Gates: `pkg/pm/local_registry_test.go`
9/9 + `pkg/cli/phase135_local_registry_test.go` 2/2 (E2E publishes
`superhello@0.1.0`, consumer `run` prints `Hello karkain` through the real
binary+pipeline; negatives missing/duplicate/traversal/mismatch; temp-dir
isolation, child-only env). Regressions green: `go build ./...`,
`go vet ./...`, `pkg/pm` full, Phase 134 gate 5/5, package CLI E2E, Sphinx
`-W`. Report: `docs/audit/PHASE-135-LOCAL-REGISTRY-FINAL-REPORT.md`.)

Also completed: **136 — LSP v2 (Semantic IDE Experience)** (verdict
**COMPLETE**; GA-2 tooling track. Server grew from diagnostics-only to
semantic editing: lexer-driven `textDocument/semanticTokens/full`
(new `pkg/lsp/semantic.go` — gap-recovered comments, UTF-16 cols, delta
encoding), queryable scope model (new `pkg/lsp/scope.go` — globals, params,
brace-matched block lets, struct/enum members, annotation-then-inference
types), deterministic definition resolver (new `pkg/lsp/definition.go` —
same-doc lexical, struct/enum member, module files via sorted open docs +
disk siblings + declared deps, bare imports, null on unknown), real symbol
ranges for all 12 decl kinds + new enum coverage (replacing line-0
placeholders), scoped completion with member completion (prefix filters
everything, keywords never after dot), server version 0.18.0→1.0.0,
VS Code LanguageClient bootstrap (`karkain lsp` stdio, guarded require) +
declared `vscode-languageclient` dep. Fixed en route: unfiltered symbol
append, map-order cross-doc nondeterminism (global search deleted), STRING
span/Col quirks, function scope ends (next-decl→brace-matched). Deliberate:
no `pkg/sema` dep (no query API there), no visibility enforcement on jumps,
no dep-model cross-request cache. Gates: `pkg/lsp` 30/30 (11 old + 19 new:
goldens, exact spans, shadowing, typed/member hover, cross-file + disk +
25× no-leak definition, completion filtering/members/broken-file),
`pkg/cli` stdio E2E through the real binary (capabilities, diagnostics
push, tokens/hover/definition/completion, exit 0) + 3/3 extension tests
(incl. client wiring); `go build`/`go vet` full tree, Sphinx `-W`, untouched
compiler suites by construction. Report:
`docs/audit/PHASE-136-LSP-V2-FINAL-REPORT.md`.)
Also completed: **137 — Concurrency Parity** (verdict **COMPLETE**; GA-2
infra track. kcc engine gains full concurrency support with byte-identical
output to Go engine — closes the post-107 boundary. Parser changes
(`src/compiler/parser.kark`): `parseSpawnExpr`, `parseChRecv`, `parseKeywordCall`
for channel/actor, `parseWithState` for diagnostic preservation, send-as-operator
enforcement. AST changes (`src/compiler/ast.kark`): `NODE_SPAWN`/`NODE_RECEIVE`
with accessors. Checker changes (`src/compiler/checker.kark`): 10 concurrency
builtins in `isCheckerBuiltin`, Spawn/Receive cases in `checkExpr`. Codegen
changes (`src/compiler/codegen.kark`): concurrency prescan
(`collectConcPrescan`), state slots (`concUses`, `concSpawnOrder`,
`concHandlerOrder`), runtime emission (`emitConcRuntime` via generated
`conc_runtime.kark`), wrappers (`emitConcWrappers`), Value glue
(`emitConcGlue`), builtin lowering (`emitConcCall`), expression lowering
(`emitConcSpawn`, `emitConcRecv`). KIR changes (`src/compiler/kir.kark`):
explicit render for spawn/receive (previously dropped to `_`). Sema changes
(`src/compiler/sema.kark`): 10 concurrency builtins in `isBuiltinFunc`.
Generated runtime (`src/compiler/conc_runtime.kark`): auto-generated from
`runtime/concurrency/c/*` by `scripts/gen-conc-runtime.ps1`, SHA-256 freshness
gate in `pkg/codegen/conc_kark_fresh_test.go`. CLI bridge
(`pkg/cli/kcc_engine.go`): gcc flag `-std=c99` → `-std=c2x` for C11 atomics.
Driver changes (`src/compiler/main.kark`): all entries use `parseWithState`,
compile commands use `-std=c2x`. Gates: `pkg/cli/phase137_concurrency_test.go`
(6 subtests: Go golden, kcc golden, byte parity, KIR rendering, checker parity,
negative cases — byte-identical `144\n10\n20\n30\n0\n6\n` output on both
engines for `examples/concurrency/pipeline/main.kark`), Phase 107 gates remain
green, freshness gate validates runtime generation. Example headers updated:
`examples/08-concurrency/01_parallel_sum.kark` and `02_channel_ping.kark`
changed from Status Experimental/Engine go to Status Stable/Engine both.
Regressions green: Phase 107, 130, 133, 134, 135, 136 gates, full
`pkg/codegen`/`pkg/cli` suites, `go vet`, `go build`. Report:
`docs/audit/PHASE-137-CONCURRENCY-PARITY-FINAL-REPORT.md`.)

Also completed: **138 — Accelerator Kernel Surface v1 (GPU/NPU)** (verdict
**COMPLETE**; GA-3 first step. `@target(gpu)` kernel functions emit real WGSL
compute kernels from `pkg/backend/gpu` (no comment stubs) with a
**compile-only guarantee**: `karkain build --target gpu-*` produces artifacts
without hardware, SDK or driver, and the CPU reference backend stays the
correctness oracle. Unknown accelerator targets are rejected on BOTH engines
(the Phase 98 `@target` preflight extended); the Phase 78 NPU quantization
path is unchanged — only the authoring surface grew. Gate
`pkg/cli/phase138_gpu_test.go` (`TestPhase138`, isolated-CWD harness so
generated `.c` never lands in the package dir) + CI step "Run Phase 138
accelerator-kernel-surface gate". NOTE: no in-phase audit report file exists
for 138 — this record is retro-filled from the gate and the GA-3 plan
(`docs/audit/GENERAL-AVAILABILITY-ROADMAP.md` §6).)

Also completed: **139 — Cross-Compilation Expansion + WASM GC** (verdict
**COMPLETE**; GA-3 second step. `pkg/target/triple.go` gains `ArchRiscv64`
and `OSMacOS` (`darwin`/`macosx` normalize to `*-macos`); ABI rules enforce
riscv64 = Linux-only, macOS = x86_64/aarch64-only, gnu/msvc rejected on
macOS; `pkg/target/features.go` marks Mach-O + Apple ABI + clang-only
linking, 64-bit LE for riscv64, and extends `SupportedTargets` with
`aarch64-windows`, `riscv64-linux`, `x86_64-macos`, `aarch64-macos` (error
texts and `SupportedArchs/OSes` extended — foreign targets still fail
deterministically with the toolchain error, never a silent host fallback).
WASM component envelope + `karkain wit` MVP land here, plus a CI
cross-linker presence matrix that is informational only (never red). Gates:
`pkg/cli/phase139_cross_test.go` + `pkg/target`/`pkg/wit` unit suites. Report:
`docs/audit/PHASE-139-FINAL-REPORT.md`.)

Also completed: **140 — Debugger Integration** (verdict **COMPLETE**; GA-3
third step. New `pkg/cli/dbg.go` `DbgCommand`: `karkain dbg` stages the
source into a temp sandbox (so beside-source `.c` debris never touches user
code), runs the RunCommand gate prologue (syntax → borrow → semantic, same
codes), links a `-g` binary through the **Go engine** (kcc deferred — the
Phase 112 boundary), then walks it under gdb batch (`rbreak
^karkain_user_` + run + 10× `bt`/`continue`, deepest stack wins); frames
parse `#N [addr in] FUNC (...) [at FILE:LINE]`, demangle the
`karkain_user_` namespace and render as `karkain_dbg trace:` with numbered
frames. VS Code extension gains the debug command + `launch.json`/tasks
templates; validated by `pkg/cli/phase140_debug_test.go` +
`TestVSCodeExtension`. Boundaries: no lldb wiring on this host; no DAP
server (that is Phase 153). Report: `docs/audit/PHASE-140-FINAL-REPORT.md`.)

Also completed: **141 — Performance & Memory (compiler itself)** (verdict
**COMPLETE** — optimizer headline + allocation accuracy + measurements;
split-TU self-build stays future work with its lever documented. New
`pkg/ir/ssa/passes_simplify_cfg.go`: constant-branch → `jmp` plus
unreachable-block sweep (loop-safe BFS from entry) with 3 unit tests; the
optimizer now drives emitted C for the gated corpus, pinned by
`pkg/codegen/phase141_opt_test.go` (6-case corpus fold/cse/loop/dce/licm/
sroa, optimized-vs-legacy runtime parity goldens, fold pin `make_int(14)`,
DCE pin, byte-determinism twice, anti-bloat bound +2%). `prof` reports
Value-cell allocation counts. Measured RSS table recorded: kcc check
544.5 MB, `go build` 229.9 MB, gcc big-TU 484 MB — target ≤2.5 GB for the
full self-build; per-pass loop-soundness verdicts documented (Mem2Reg stays
unwired). Report: `docs/audit/PHASE-141-FINAL-REPORT.md`.)

Also completed: **142 — 1.1.0 Release** (verdict **COMPLETE (preparation)**;
GA-3 final step — everything up to the owner-only ceremony is done, gated and
rehearsed; no tag cut in-phase, matching the 1.0.0 precedent. Version
identity unified at **1.1.0** across `VERSION`, both Go banner sites, three
kcc banners, six kcc codegen headers, the Go quantum/QIR/QEC/QASM/OpenPulse/
DWARF headers and the LSP server; the pinning tests moved together
(88/90, 115 skeleton + DocTree, 118 RC identity, 119 identity + labels,
120 envelope, bootstrap args) while historical assertions stayed untouched;
kcc rebuilt so its banner matches, and the concurrency pipeline golden stayed
intact. `1.0.x` LTS branch created from `v1.0.0` and pushed; SECURITY gains
the LTS row. New `docs/release/KARKAIN-1.1-RELEASE-NOTES.md` (phases 132–141
deltas, compatibility statement — one behavior fix: K114 escape rejection —
honest limitations, verify ceremony); `release-notes.rst` restructured (1.1.0
on top, 1.0.0 demoted to historical); README/SPEC/status/cli/lsp/showcase/
issue-template/scripts swept; corpus recounted to **61 files in the 15
categories** (114 `.kark` total; 59 runnable incl. test-mode, 2 Stable, 1
Planned dir). Gate `pkg/cli/phase142_release_test.go` (identity on both
engines, emitter-header allowlist, changelog presence, LTS ref, host archive
rehearsal with SHA-256 → verify → `--version` from the extracted tree).
Report: `docs/audit/PHASE-142-FINAL-REPORT.md`.)

Also completed: **143 — Seed-Binary Bootstrap Closure (Go-free)** (verdict
**COMPLETE**; SOVEREIGNTY opener, gate + docs only — zero new production
paths. `pkg/bootstrap/phase143_seed_test.go`: `TestBootstrap_SeedClosure`
builds the seed via stage 1, scrubs the Go toolchain's directory from `PATH`
while asserting `go` is unresolvable and `gcc` intact, runs stages 2–3
go-less and asserts bitwise identity; `TestBootstrap_ScrubGoFromPath` pins
the separator-agnostic scrub logic without needing RAM or gcc (it caught a
real Windows bug during development: `filepath.Dir` mis-splits forward-slash
paths — fixed with a dual-style split). CI gains a dedicated (fast
self-skipping) closure step; `docs/self-hosted-compiler.md` gains the Seed
Closure section with the honest remaining boundaries (optional Go reference
toolchain; gcc linker until Phase 145/152). Dev host: the closure **skips
honestly** under the Phase-127 guard (`error[K127]`, ~100–600 MiB free <
1536 MiB required) — the guard working as designed. Report:
`docs/audit/PHASE-143-FINAL-REPORT.md`.)

Also completed: **144 — Toolchain Sovereignty (PM resolution in kcc)**
(verdict **COMPLETE**; SOVEREIGNTY second step. The self-hosted engine now
assembles manifest/workspace/registry/git dependencies itself:
`src/compiler/main.kark` gains a 16-function PM section (flat
`[dependencies]` parse with dev-deps excluded, lock version/rev lookup,
workspace-root discovery, `sanitizeCacheComponent`/`depCacheName`/
`normalizeSourceURL` mirrors, `depSourceDir` source switch, `depDirPaths`
single-pass join with main-skip, path-dedup and root-exclusion) prepended
upstream-first in `assembleProject` for both import and sibling shapes.
Unresolvable deps skip silently (Go parity); imported-but-missing modules
still raise `error[K122]`; no new error codes. The CLI mirrors the project
rather than composing its input (`kccMirrorProject` + `pmWorkspaceRoot`;
`kccStageInput` routes manifest projects to the mirror, flat/legacy paths
untouched; Go-engine assembly and kcc test injection unchanged). Two real
defects were caught by the project's own machinery during development:
`fileExists` is not a Go-resolver builtin (K002 → all existence checks
dropped by design instead of adding surface), and the sibling branch
**assigned instead of appended**, discarding dependency content (one `+`
fix, found by isolating the join in a kcc-run harness). Gate
`pkg/cli/phase144_pm_test.go`. Report:
`docs/audit/PHASE-144-FINAL-REPORT.md`.)

Also completed: **145 — Native Backend v1 (C-free)** (verdict **COMPLETE**;
SOVEREIGNTY / INDEPENDENCE C-front first step — the first machine code
Karkain has ever emitted without a C compiler. `pkg/native/emit.go`:
hand-encoded x86-64 (mov reg/reg/imm32/imm64/stack/disp8/disp32/SIB/mem8,
add/sub, imul, push/pop, xor-zero, neg, dec, cqo, div, test, near
call/jmp/jnz/jns with label + rel32 backpatch, ret, syscall, ModRM/SIB/REX)
with golden byte pins for every primitive; `pkg/native/elf.go`: static
ELF64-LE writer (single PT_LOAD, `_start`, `.text`+`.rodata`, no INTERP, no
libc) + structural `Parse` validator; lowering for straight-line ints,
string literals and user calls, `print` via raw `write`/`exit`; determinism
plus a 7-case K145 reject table. Slice 145C was gates/docs/regressions only —
**no CLI flag in 145** (package API + tests, mirroring `pkg/wasm`'s shape).
Execution on the Linux CI leg died instantly and deterministically
(`TestNativeHello` segfault, `TestNativeCalls` trap, all six bisect cases
zero output), so the phase closed under **quarantine**
(`docs/audit/PHASE-145-QUARANTINE.md`, flag `KARKAIN_NATIVE_EXEC`, per-case
`if: always()` bisect CI steps) — every non-execution gate stayed mandatory.
Report: `docs/audit/PHASE-145-FINAL-REPORT.md`.)

Also completed: **146 — Generics v1 (Go-engine, then kcc parity)** (verdict
**COMPLETE**; LANGUAGE opener after the 145 quarantine, delivered in three
commits. Contract: `func name[T,U](params)` and `type Name[T] struct {...}`
declarations; call sites use **explicit type args** with square brackets
(`f[int](args)`) because angle brackets collide with `<`/`>` in the
expression grammar; no inference in v1; constraints parse and store but are
unchecked (no trait/impl syntax exists to declare against). Because
`f[T](x)` is syntactically identical to index-then-call (`ops[idx](5)`,
Phase 133) and the parser has one token of lookahead, the design is
**optimistic parse + semantic demotion**: `IDENT[ident-list](args)` parses as
CallExpr+TypeArgs, and the sema monomorphize pass either instantiates and
rewrites to the specialized name or demotes back to
IndirectCall(IndexExpr), preserving Phase-133 behavior exactly (including
variable indices); unresolved callees stay K002. New **K115** for misuse.
Slice 146a wired `pkg/sema/generics.go` to function/struct call sites
(previously kernels only) and filled `GenericParams` in the parser; 146b
shipped the importable `std.generics` module (persistent `Stack[T]`/
`Queue[T]`, 12 functions) with goldens and a kcc boundary pin; 146c delivered
**kcc generics parity** (monomorphize pass, keyword type-arg gate, goldens;
plus a kcc index-then-brace fix in bare conditions and a `stable-api.rst`
list-table indent fix), refreshing the whole-tree KIR pin 8986 → 9574. Gates
`pkg/cli/phase146_generics_test.go`, `phase146b_generics_test.go`,
`phase146c_generics_test.go`. NOTE: no in-phase final report file exists for
146 — this record is composed from `docs/audit/PHASE-146-BASELINE.md` and the
146a/b/c commits.) Pass B supplement (2026-09-25, traceability + evidence):
slice commits `fdde0b9` (146a) / `8e6b62d` (146b) / `4d3d7a6` (146c) with
follow-up fixes `4be2d7c` (index-then-brace + stable-api indent) /
`736cea4` (KIR pin); gates measured green locally — 146 (7 tests) 13.6s,
146b (4 tests) 21.5s, 146c (10/10 incl. 265s compiler self-check) 313s,
full 146A–C legs 21 tests; 146c closure verdict **CLOSED — PASS WITH
ENVIRONMENT LIMITATION** (combined 120–145 sweep interrupted on the 4GB
host, not failed); index-then-brace root cause is Go parity with
`pkg/parser/parser.go:1902` (`!unparenthesizedIfCondition` guards both the
bracket and brace attempts — kcc guarded only the brace); KIR pin verified
deterministic 9574/9574 text/verify on repeat runs. 146D is baselined
separately (`docs/audit/PHASE-146D-BASELINE.md` — close-out only).
Also completed: **146D — Generics v1 Close-Out** (verdict **COMPLETE**;
consolidation gate `pkg/cli/phase146d_closeout_test.go`
(`TestPhase146D_Closeout`: GoTrack, FuncStructGoldensBoth, RejectionsBoth,
DemotionIdempotence, StdlibParity, SelfCheck — green in one invocation);
CI `Run Phase 146 generics gate` step (146/146B/146C/146D patterns);
`SPEC.md` §9 rewritten to the honest v1 surface (mojibake badges replaced,
`type Point[T] struct` real syntax, constraints parse-but-unchecked,
traits Planned) after live-syntax probes proved the old examples overclaimed
(constrained/annotated decls exit 3, `struct Vector[T]` exits 3);
`stable-api.rst` diagnostics range extended to K101–K115 (K114 escape +
K115 generics); new `docs/source/status/feature-matrix.rst` ledger wired
into the status toctree. No language change — docs, gates, and CI only.)

Also completed: **147 — Native Execution P1 Resolution** (verdict
**COMPLETE**; INDEPENDENCE C-front opener after the 145 quarantine — the
generated native binaries now execute on Linux CI and the quarantine is
lifted. Evidence first: new `TestNativeEvidenceDump`
(`pkg/native/evidence_test.go`, env-gated `KARKAIN_NATIVE_DUMP`, bytes-only so
it runs everywhere) plus an informational Linux-only `native-evidence` CI job
capturing `readelf -h -l` and `strace -f`; the decisive capture showed
`execve(...) = 0` (loader exonerated — clean ELF, entry `0x400112` inside the
single `LOAD`) followed by immediate `SIGSEGV si_code=SI_KERNEL si_addr=NULL`
with zero userspace progress. **Root cause 1 (the P1):** `MovRegImm32`
emitted `REX.W + B8+rd` *with a 32-bit immediate* — per the Intel SDM that
opcode **is** `mov r64, imm64` (10 bytes), so the CPU consumed the next four
bytes at every use site (syscall numbers, fds, loop constants, the `_start`
exit sequence) and the stream desynchronized from the first use; the
Phase-145 comment calling it sign-extended was wrong and two goldens pinned
the buggy bytes. Fixed to the correct prefix-less `B8+rd io` (zero-extends)
with `REX.B` only for r8–r15, goldens corrected. **Root cause 2:** binary
operands pushed `rax`, moving `rsp` and shifting frame-relative slots
(`add(20,22)` read slot a twice → 40); lowering now stages through
depth-indexed scratch slots (rsp never moves; 64-deep K145 guard). **Root
cause 3:** `print` wrote no trailing newline; newline tails added to both
helpers. Result: `TestNativeHello`, `TestNativeCalls`, all six
`TestNativeBisect` cases and `strace exit=0` green in default CI; the
`KARKAIN_NATIVE_EXEC` flag removed from `runNativeCode`; the quarantine doc
marked closed; bisect `if: always()` steps kept as permanent regression
coverage. Report: `docs/audit/PHASE-147-FINAL-REPORT.md`.)

Also completed: **148 — Native CLI Flag + Control Flow + Calls ABI** (verdict
**COMPLETE**; INDEPENDENCE C-front second step. `karkain build/run --target
native-x86_64-linux` is now a real user path: a pure-Go emission that writes
a `0755` ELF from **any** host with no C compiler, executes on linux/amd64
with stdio passthrough (program failure → `ExitFailure`), refuses `run` off
Linux with the build-only hint and exit 6, and refuses `--incremental`
loudly (the cache serves the C pipeline; native-split is 150+). kcc
auto-routes exotic targets to the Go commands, so **no kcc changes** (kcc
native parity is 151). Control flow: `CmpRegReg` + near `Jz/Jl/Jle/Jg/Jge`
golden-pinned; `emitFunc` body factored into `emitStmt` (straight-line paths
byte-identical by construction); `emitCond` accepts int comparisons only
(loud K145 — never miscompiled truthiness); `if`/`while`/C-style `for`
(bare-assignment post wrapped), nested `let`s via a function-wide pre-scan
(first declaration wins, shadowing writes through — documented v1 semantics),
int reassignment, `break`/`continue` via a loop-label stack (outside loops =
K145); `for-in` stays K145 (arrays are 150 Value-model work). Calls ABI
(documented in `program.go`): 8-byte units packed in order (int 1, string 2),
units 0–5 in RDI/RSI/RDX/RCX/R8/R9, further units in the caller-frame extras
area addressed by R10; string returns `(RAX=ptr, RDX=len)`; `rsp` never moves
during evaluation; a function table enforces exact arity with kind-checked
arguments (loud int/string mismatches naming the parameter), string params
via Phase-46 annotations (two slots), string returns via whole-unit
inference (cycles/mixes loud), and `main` takes no arguments. Goldens
`TestNativeControl` (`if_else 10/40`, `while_sum 55`, `cfor_sum 45`,
`break_continue 18`, `nested 9`) and `TestNativeCallsABI` (executed on Linux,
structural elsewhere); gate `pkg/cli/phase148_native_test.go` 5/5 (including
an ABI build probe, run-refusal, incremental refusal and listing) —
`--engine go` pinned. Report: `docs/audit/PHASE-148-FINAL-REPORT.md`.)

Also completed: **149 — PE + Mach-O Writers, Win64 Boundary, PEB Bootstrap**
(verdict **COMPLETE (execution proven)**; INDEPENDENCE C-front third step —
**PE images execute green on windows/amd64 (7/7 goldens live on the dev
host)**, Mach-O is structural (parse + determinism; no Intel-mac runner
exists) and ELF stays byte-identical and green on Linux. `pkg/native/pe.go`:
DOS header + PE signature + COFF + PE32+ (240 B, correct 64-bit offsets) +
three sections (`.text` R-X with code+rodata, `.idata` R/W with
IDT/ILT/IAT/Hint-Name/kernel32.dll, `.reloc` with DIR64 blocks per 4 KB page)
+ a `ParsePE` validator. `pkg/native/macho.go`: header + `LC_SEGMENT_64`
(nsects=0) + zeroed `LC_DYLD_INFO_ONLY` + `LC_LOAD_DYLINKER` + `LC_MAIN` + a
`ParseMachO` validator. OS-parameterized lowering (`CompileProgramForOS`,
Linux default preserved by `CompileProgram`) with per-OS syscall numbers
(Linux 1/60, macOS 0x2000004/0x2000001) and per-OS `_start` tails over one
shared encoder/lowering. Win64 boundary (PE only): System V **inside** user
functions, Win64 (RCX,RDX,R8,R9 + 32-byte caller shadow) at three kernel32
call shapes, 16-byte alignment discipline (entry `and`, 16-rounded frames,
48-byte WriteFile shadow), IAT slots reached by `movabs` + `call`.
Loader-independent PEB bootstrap (PE only): PEB → Ldr → module walk (exact
"kernel32.dll" match, 64-entry bound, Int3 on exhaustion) plus per-export
table walks publishing ExitProcess/GetStdHandle/WriteFile into the IAT —
required because the host loader maps minimal images and runs entry but never
snaps the IAT. Five load-bearing root causes isolated and golden-pinned:
CallReg ModRM (`FF D0` call r64 → `FF 10` call m64 — the one-byte execution
blocker), `.idata` must be writable and MajorSubsystemVersion ≥ 6 (loader
rejections, bisected empirically), `.reloc` DIR64 entries for every `movabs`
site (the host ASLR-rebases regardless), 32-bit LDR offsets
(+0x30/+0x58/+0x60) with uppercase BaseDllName folding and the PE32+ export
directory at NT+136 (not +96), and an RAX-not-RDX store bug. New CI
`native-windows` job (windows-latest) runs the full `pkg/native` suite where
PE can execute. Boundaries (documented, NOT defects): Mach-O execution
blocked (no runner; PIE/rebase opcodes are 150+ work), no PE
delay-load/TLS/SEH/resources/codesigning, no arm64, console UTF-8 is byte
passthrough, debuggers on native images stay on the C path, kcc native parity
is 151, and `--target native-x86_64-windows/macos` CLI targets are the
immediate follow-up. Report: `docs/audit/PHASE-149-FINAL-REPORT.md`.)

**In progress: 150 — Native Value Model + Windows/macOS native targets**
(version **1.2.0** "Sovereignty I" opener; scope, slices and exit criteria
in `docs/audit/PHASE-150-BASELINE.md`).

Also completed: **150A — Native Value Model core** (increment 150, slice A
of 4; verdict **COMPLETE**). The native backend stopped being an
int+string toy. **Float64**: `float64 -> IEEE-754 bits -> one 8-byte
unit`, identical to an int in frames, calls and returns — `+ - * /`, unary
minus, all six comparisons, float params/returns/calls, SSE2 only (no
FMA/AVX dependency). Division by zero yields 0 and printing is
`%g`-compatible (fixed for `-4 <= X < 6` else scientific, round-half-even
at the 7th digit, trailing zeros trimmed), matching the C backend instead
of inventing a third formatting. **Arrays**: a `(base, len)` header plus a
compile-time-sized element area in the frame; literal binding, `arr[i]`
with a checked bound (negative or `>= len` traps), `len(arr)`, and
`for x in arr` with `break`/`continue`/nesting sharing the loop-label
stack. Scaled `[base + i*8]` addressing keeps the array base out of `rsp`,
preserving Phase 147's "rsp never moves" rule. New emitter primitives are
all golden-pinned byte-for-byte (`TestEmitSSE`, `TestEmitPrintFloatPrims`:
movq/addsd/subsd/mulsd/divsd/ucomisd/cvtsi2sd/cvttsd2si/xorpd, the
print_float carry+bitscan+limb primitives, ja/jae/jb/jbe/jp, and the
scaled SIB load/store forms). 9 float + 11 array programs run on **both**
containers; on this windows/amd64 host the PE legs execute for real
(PE + Win64 boundary + PEB bootstrap), so the array lowering is proven by
execution, not assertion. **Two real defects found and fixed**: (1) the
for-in guard was **inverted** — `cmp [len], i` leaves `len - i`, so the exit
branch must be `jle`, not `jge`; with `jge` the body never ran for any
non-negative index (pinned by the nested-loop golden); (2) `print_float`
was emitted **unconditionally**, growing every image ~700 bytes for a
formatter an int-only program can never call (hello 503 → 1226), which
would have failed the byte-identity criterion — it is now gated on
`scanFloatUsage`, a whole-unit pre-pass (necessarily a pre-pass:
`emitHelpers` runs before any function body). Also fixed while landing it:
`emitLet` never dispatched arrays so `emitArrayValue` was dead code, that
routine stored elements through an uninitialised RCX and then clobbered
RAX with the length reload, and every `for-in` shared one hidden `"for$idx"`
slot so a nested loop resumed its parent with the inner counter (now
per-`ForInStmt`-node). **New: `TestNativeELFByteIdentity`** — 19 pre-150A
programs (145 straight-line/strings, 147 bisect, 148 control + calls ABI)
pinned by SHA-256 to the increment-149 (`951ee10`) images, measured rather
than regenerated, which is what makes "zero golden drift" a measurement
and gives 150C (register allocation) its differential. Result: **zero byte
drift**. Regressions green: `go build ./...`, `go vet`, full `pkg/native`
(no FAIL; honest platform skips for ELF/Mach-O execution and the evidence
dump), Phase 148 CLI gate, `pkg/parser`, `pkg/lexer`. KIR pin 9574
unaffected (the native path does not consume KIR). **Carried to 150B, not a
defect:** `push()` is a loud K145 naming itself — a push must grow the
element area, and 150A arrays are fixed-footprint frame values sized by the
layout pass, so growable storage is heap/boxed-`Value` work. 150B
(maps/structs/string ops), 150C (register allocation) and 150D (Mach-O
PIE/rebase + the two new CLI targets + native-split cache +
`pkg/cli/phase150_native_targets_test.go`) are not started.)

Also completed: **150B1 — Heap arena + string concat** (increment 150,
slice B1; verdict **COMPLETE**). The memory substrate the rest of 150B
sits on, plus one real consumer so it is not dead code. **Arena**: a bump
allocator over a static region with a 16-byte header
(`[cursor][limit]`, both *absolute* addresses) followed by the data area —
holding the bookkeeping inside the arena itself means no globals and no
writable image text, and the cursor self-initialises on the first `alloc`
(zero cursor = "not started"). No `free()`: bump-only suffices because
every 150B allocation is compile-time bounded, which keeps the invariant
checkable and the exhaustion `Int3` unreachable. **Writable storage, two
ways, both gated**: ELF gets a second R+W `PT_LOAD` (page-aligned,
`p_offset ≡ p_vaddr`); PE puts the arena inside `.idata`, which is already
R/W and loader-proven writable — no fourth section, no extra import. macOS
has a single R+X `__TEXT`, so an allocating program gets a loud K145
naming 150D instead of an image whose first cursor store would fault.
**Gating** is what keeps the increment-149 byte-identity pins green: the
arena, the `alloc` helper and the concat frame area are emitted only when
the program actually concatenates, and `TestNativeELFByteIdentity` still
measures **zero drift** across all 19 legacy programs after the ELF header
count, `.idata` sizing and frame layout all changed. **String concat**:
`a + b` allocates `len(a)+len(b)` and copies byte-wise (the lengths are
runtime values, so a constant-offset load cannot address them); operands
stage through a dedicated 5-unit-per-depth frame area rather than the int
path's 8-byte `binTemp` scratch; the second copy reaches its offset by
advancing the destination *pointer*, because the SIB `disp` field is a
compile-time immediate in every x86-64 memory form. New emitter
primitives: `sibTail` (factored out of `scaledSIB` so the 8-bit and
two-byte-opcode forms reuse the register/disp encoding), `LoadScaled8`
(`REX.W 0F B6`), `StoreScaled8` (`REX 88`), `IncReg`. The arena bound is a
**proof, not a guess**: every string value in a concatenating program is a
literal or a concatenation of literals, so `sites × totalLiteralBytes`
bounds the sum of all runtime allocations. **Executed**: 14 concat programs
(chained, grouped, variable operands, call args, returns, branches, loops,
multi-site, long operands); the 7 PE cases execute for real on this Windows
host. **Two real defects found and fixed**: (1) the concat pre-pass missed
sites with **no literal operand** (`a + a`, two variables), so no arena was
emitted while emission still called `alloc` — an undefined-label panic; the
pre-pass now classifies string-ness syntactically with a fixpoint over
`let` bindings and `string` parameters, and `emitStrConcat` refuses loudly
if ever reached with no arena so a future gap is a diagnostic, not a panic;
(2) the concat scratch area was first written against `binTemp` (8
bytes/depth) with 40 bytes of staging needed — it now has its own
`strTemp`, reserved only when the program concatenates (reserving it
unconditionally would have grown every frame and broken byte-identity).
Regressions green: `go build ./...`, `go vet`, full `pkg/native` (no FAIL;
honest platform skips), Phase 148 CLI gate, `pkg/parser`, `pkg/lexer`.
**Still open in 150B:** `push()` (now unblocked by the arena), maps, structs.

Also completed: **150B2 — String slice + string comparison** (increment
150, slice B2; verdict **COMPLETE**). The two string operations that only
*read* bytes, so they need the staging area but no heap. **Slice**
`s[a:b]` is a **view**, not a copy — the result is `(s.ptr + a, b - a)`,
so no allocation and no copy happen; bounds are `0 <= a <= b <= len` with
a loud `Int3` on violation, matching the array-index contract rather than
silently clamping, and a missing `b` means "to the end". **Equality and
inequality** compare by content: a length check first (a different length
is decisively unequal, with no byte reads at all), then a byte loop. Only
`==`/`!=` exist; ordering is a loud K145 rather than a pointer comparison
or an invented lexicographic helper. The string staging pre-pass was
generalised so the area is shared by concat, comparison and slice — a
program that only compares or only slices still gets correctly-sized
scratch, and one that does none of them keeps its exact increment-149
frame — and the string classifier was factored into a single
`collectStringNames` + `strKindOf` pair instead of being duplicated per
scanner. `emitStrEqCond` validates **both** operand kinds so `1 == s`
names the offending int side whichever way round the operands appear.
**One imprecise diagnostic found and fixed:** classifying only `+` as a
string operation routed `s - "c"` into the int path, which reported the far
less helpful "string 's' in int position"; any operator over two string
operands is now a string operation, so `emitStr` owns the decision and
names the operator precisely. **Executed**: 15 string-view programs
(slicing with literal and variable bounds, empty slices, slices of a
concat result, equality over equal/unequal content, the length-mismatch
path, both-empty, a slice compared against a literal to gate a branch, and
both operations inside a loop) — all 15 run for real on this Windows host
through the PE container — plus 10 new negative cases. Regressions green:
`go build ./...`, `go vet`, full `pkg/native` (27 tests, no FAIL; honest
platform skips), byte-identity still zero drift, Phase 148 CLI gate,
`pkg/parser`, `pkg/lexer`. **Still open in 150B:** maps, structs.

Also completed: **150B3a — `push()`** (increment 150, slice B3a; verdict
**COMPLETE**). `push()` was the one 150A item explicitly carried over, and
the arena unblocks it. **push is functional**: `let b = push(a, v)`
allocates a *new* `(base, len+1)` array in the arena, copies the old
elements, and appends `v`. The source array is untouched (bump-only arena,
nothing moved or freed), which is what makes the two-slot header keep
working — indexing, `len` and `for-in` all operate on a pushed array
through exactly the same code as a literal one. The copy uses the scaled
64-bit load/store forms (elements are 8-byte ints) and the appended value is
a single `StoreScaled64` with the old length as the index, so no separate
address arithmetic is needed. **Everything is staged through the frame
before the `alloc` call** — deliberately, because `alloc` uses
RAX/RCX/R10/R11 and takes RDI, so keeping the old base and length in R8/R9
across the call would have been exactly the kind of implicit register
contract that produced the Phase-147 `add(20,22) = 40` bug. **A push inside
a loop is a loud K145**: that is the honest bound, since a push in a
`while`/`for`/`for-in` body can run an unbounded number of times and no
compile-time arena size can cover it — the alternative was a program that
exhausts the arena and traps at an arbitrary iteration, so the pre-pass
(`scanPushSites`, which tracks loop nesting depth) refuses it by name. **The
arena bound is still a proof**: outside loops, site *i* of a push chain sees
at most `maxArrayLiteralLength + i` elements, so the sum over all sites is
bounded by `pushSites × (maxLit + pushSites) × 8` bytes — and with the loop
case refused, no site executes more than once. **Executed**: 12 push
programs (single push, push onto empty, source provably unchanged, two- and
three-deep chains, iteration and summing over a pushed array, two pushes off
the same source, negative and large values, an eight-element chain, a
computed value, and a program mixing a push with a string concat) — all 12
run for real on this Windows host through the PE container — plus 9 negative
cases (non-int element, wrong arity, non-array receiver, literal receiver,
push inside `while` and inside `for-in`). Regressions green: `go build
./...`, `go vet`, full `pkg/native` (31 tests, no FAIL; honest platform
skips), byte-identity still zero drift, Phase 148 CLI gate, `pkg/parser`,
`pkg/lexer`. **Still open in 150B:** maps, structs.

Also completed: **150B3b — records (structs) on the native target** (increment
150, slice B3b; verdict **COMPLETE**). The native value model's last big gap:
a record. `KindStruct` is **one** 8-byte unit whose value **IS** the address of
a compile-time-sized field area reserved in the frame, so field assignment and
by-pointer record passing work without copying a byte and **nothing is
allocated** — records stay out of the heap arena entirely, which is exactly
what keeps every pre-150B3b image byte-identical. Surface: `type X struct
{a int; label string; r float}` with a fixed layout (int/float one unit,
string two, `newStructInfo` + a `TestNativeStructLayout` pin); struct literals
bound at `let`, **returned**, or **passed straight to a call** (each with its
own reserved area — `retRec` keyed by the `ReturnStmt` node, `recArgArea` keyed
by the literal node, following the existing for-in node-keying rule); field
reads and `rec.field = v` writes; the **record idiom** (a free function taking
the record, annotated `func deposit(a Acct)`, whose field writes are visible to
the caller); and record returns (the address in RAX, the int convention).
Record params are named by annotation — the untyped record idiom is a loud
K145 rather than an inferred guess. **Six real defects found and fixed, all
gate-pinned**: (1) `newStructInfo` rejected the explicit `int` field
annotation (the parser records `int`, not only the bare form) so *every* int
field was refused; (2) `emitStr` had no `DotExpr` case, so a string field could
not be read/concatenated/compared; (3) `emitFloat` had no `DotExpr` case either
and a float field **panicked the emitter** on an undefined `print_float`
label — root cause: the float pre-pass did not treat a declared float FIELD as
a float root (`hasFloatField`); (4) returned and call-argument record literals
had no field area at all; (5) the three syntactic string classifiers could not
see a string field, so `s.label + "rk"` reached emission with no arena
(`stringFieldNames`/`isStrFieldDot`, a deliberate safe-direction superset since
over-detection only reserves bytes while a miss reaches emission uncaught);
(6) assigning a string to an int field failed deep in the int path with a
message about expression shape, so `emitFieldWrite` now checks the assigned
kind first and names the field and both types. **Executed**: 31 record
programs — construction, field-in-expression, cumulative/expression/loop
field writes, mixed int+string+float layouts, string-field
concat/compare/write-through-a-parameter, the record idiom (mutation visible
to the caller, two records, record+scalar), record returns (literal,
computed, forwarded), computed initialisers, record independence, and the
aliasing rule (`let b = a` shares ONE field area — pinned as an executed
golden because it is the one place a reader expects a copy) — **all 31 run
for real on this Windows host through the PE container, zero skipped**, plus
the same set on Linux ELF and a structural pass over all three containers
(ELF/PE/Mach-O), a 17-case K145 reject table, and the layout pin.
Regressions green: `go build ./...`, `go vet ./pkg/native`, full `pkg/native`
(no FAIL; honest platform skips), **byte-identity still zero drift across all
19 legacy ELF images**, Phase 148 native CLI gate, `pkg/parser`, `pkg/lexer`,
`pkg/sema`, `pkg/ir/...`, `pkg/target`, `pkg/wasm`, `pkg/compiler`. The
`pkg/codegen` `TestPhase107_CodegenSpawnJoin` failure in a combined run is the
documented ~4GB-host OOM flake class (passes in isolation; this slice touches
only `pkg/native`). **Still open in 150B:** maps.

Also completed: **125A — Standard-Library Networking / Database / Web slice +
Windows Winsock linking** (verdict **COMPLETE**; three new stdlib modules,
six new examples, unconditional net-runtime emission on BOTH engines, and the
Windows `-lws2_32` link contract enforced across every generated-C linker.
Modules: `stdlib/net/net.kark` (`net_address`/`net_endpoint`/`net_dial`/
`net_serve`/`net_accept_next`/`net_recv`/`net_send`/`net_shut`/`net_error`/
`net_fd_open` — thin wrappers over `net_connect`/`net_listen`/`net_accept`/
`net_read`/`net_write`/`net_close`/`net_last_error`), `stdlib/http/http.kark`
(`http_make_request`/`http_make_response`/`http_new_headers`/
`http_parse_request`/`http_parse_response`/`http_parse_headers`/
`http_split_message`/`http_build_request`/`http_build_response`/
`http_request_send`/`http_get`/`http_post`/`http_listen`/`http_accept`/
`http_read_request`/`http_write_response`/`http_read_all`/`http_read_until`/
`http_close`), `stdlib/db/db.kark` (SQL-text engine: `db_engine`/`db_backend`/
`db_open`/`db_execute`/`db_query`/`db_result`/`db_last_error`/`db_last_count`/
`db_create`/`db_insert`/`db_select`/`db_update`/`db_delete`/`db_drop`/
`db_begin`/`db_commit`/`db_rollback`/`db_prepare`/`db_execute_stmt`/`db_format`/
`db_save`/`db_close`/`db_load`; documented `#karkain-db-v1` deterministic
pipe-delimited persistence; reported via `db_engine()`; written in canonical
Karkain for identical both-engine compile). Runtime emission: Go `pkg/codegen/
codegen.go` emits the net runtime **unconditionally** in the generated-C
preamble (+205 verified by the same gates that pin every byte), and the
self-hosted `src/compiler/codegen.kark` mirrors it (+202) so kcc output is
byte-identical; builtin registration in `pkg/sema/resolve.go` and checker
tables in `src/compiler/{checker,sema}.kark`. **Windows Winsock link contract
(real defect, root-caused and fixed):** because the preamble now references
Winsock unconditionally and MinGW gcc ignores `#pragma comment(lib, ...)`,
EVERY generated-C link on Windows needs `-lws2_32`. The Go host-native and
cross paths already gained it (`pkg/codegen/cross_target.go` `hostNativeFlags`
+ 3 cross-compiler branches); kcc paths use `winsockLibFlag()` in
`pkg/cli/kcc_engine.go` (stage-1 link, artifact link, sandbox run link + the
`phase95_parity_test.go` raw link). **Bootstrap defect fixed**: `pkg/bootstrap`
stage-1/stage-2/stage-3 `compileWithGCC` lacked the flag, so the bootstrap
compiler link failed with undefined `__imp_WSAGetLastError`/`__imp_WSAStartup`/
`__imp_getaddrinfo`/`__imp_socket`/`__imp_connect`/etc.; now appends `-lws2_32`
on Windows (runtime.GOOS guard). Legacy `phase88/89/90` test raw-gcc links also
gained `winsockLibFlag()`. Corpus: 6 new examples pinned in the Phase 114 gate
(→61 total, all both-engine): `examples/04-networking/01_tcp_echo.kark`
(print `4/ping/4/pong/4`-style loopback echo), `02_tcp_roundtrip.kark`,
`examples/06-database/01_db_crud.kark` (`db_create`/`db_insert`/`db_select`/
`db_update`/`db_delete`), `02_db_persist.kark` (`db_save`/`db_load` to a temp
file), `examples/07-web/01_http_loopback.kark`, `02_http_codec.kark`;
goldens byte-identical Go↔kcc (verified by targeted + full KCCParity runs).
Docs honesty fix: `docs/source/examples/{networking,database,web}.rst` + the
three category `README.md`s previously documented INVENTED surfaces
(`tcp_connect`/`socket_send`/`db_where`/`http_serve`, etc.) — rewritten to the
real module surfaces above, plus malformed `.. implemented:` → `:implemented:`.
KIR continuity: whole-tree KIR count grew **6645 → 6910** (net runtime +
builtins in `src/compiler/*.kark`), pin + note updated in
`pkg/cli/phase122_pipeline_ownership_test.go` (re-verified stable, all 7
subtests PASS 258s). Regressions green after the winsock fixes: `go build
./...`, `go vet` (cli/bootstrap/codegen/sema), TestPhase88/89/90 (full suite
86s), TestPhase95 (probes+conformance+engine-selection 167s), TestPhase101/
109/117 (kcc+stdlib edge), TestPhase122, TestPhase123, `pkg/bootstrap`
Stage-1 + all args tests (16s), `pkg/codegen`, net-example end-to-end
`run --engine go` AND `run --engine kcc` both print the identical golden
(`3/one/3/two/3` for 02_tcp_roundtrip). Documented environmental (NOT
defects): bootstrap Stage-2/Stage-3 (`TestBootstrap_Stage2SelfHosting`/
`BitwiseIdentity`) still hang/SEGFAULT on this ~4GB host — the pre-existing
kcc-build-mode OOM class, unchanged by this slice (Stage-1 GO-engine path is
the green gate on small hosts); the `TestPhase107_CodegenSpawnJoin` OOM flake
in full-tree `./pkg/...` runs also remains the documented class (passes
isolated). Reports: this record; `ROADMAP-PLAN.md` is a stale pre-50 planning
doc whose "Phase 124/125" (x64 backend / incremental-build v2) are unrelated —
AGENTS.md records the authoritative completion state.)
Also completed: **111** — Cross-Compilation
(`--target <triple>` is a real, explicit cross-compilation switch backed by
the Karkain-owned target model `pkg/target` (arch/os/env, canonical short
triples `x86_64-windows`/`x86_64-linux`/`aarch64-linux`/`wasm32-wasi` +
conventional long-form normalization, `Features`, host-vs-foreign
`SameMachine`); `karkain target` reports the host triple + full supported
matrix; the C-driver selection in `pkg/codegen/cross_target.go` compiles
same-machine targets with the historical host probe (byte-identical flags)
and cross targets with triple-prefixed GNU cross-gcc → clang `--target`,
failing deterministically with a cross-linker `ToolchainError` (exit 6, lists
exactly what was searched) when none exists — never a silent host fallback;
`karkain run --target <foreign>` is refused with a build-only hint (cross-run
needs an emulator/remote target); triple `karkain build` now emits real
native artifacts (PE32+ x86-64 machine field validated on this host —
previously Go-engine builds only transpiled to C); generated C self-describes
each target via a `karkain-target:` header comment + `KARKAIN_TARGET_ARCH_*`/
`KARKAIN_TARGET_OS_*` preprocessor defines. Honest matrix on the Windows x86_64
host: `x86_64-windows` PASS, `x86_64-linux` + `aarch64-linux` N/A (no
cross-linker on PATH — mechanism implemented, artifacts unverifiable here),
`wasm32-wasi` unchanged (Phase 108). In-language `target.os`/`target.arch`
builtins are intentionally post-111 (would require kcc-parity changes).
Gates: `pkg/cli/phase111_cross_compile_test.go` + `pkg/target/triple_test.go`;
regressions green: full `pkg/codegen`, Phase 105–110 CLI gates, `go vet`,
`go build ./...`. Report:
`docs/audit/PHASE-111-CROSS-COMPILATION-FINAL-REPORT.md`.)
Also completed: **110** — Profiling & Diagnostics
(`karkain prof <file.kark>` is a real CLI: it compiles and runs the program
once with opt-in, aggregation-based instrumentation and reports deterministic
data — per-function call counts + inclusive/exclusive/min/max/avg wall ns,
caller->callee call graph, folded (flame-graph) stacks, allocation metrics —
in `text` (default), `json` (`karkain-profile-v1` schema) and `folded`
formats, plus `--output <path>`. Implementation: `Config.Profiling` +
`Generator` profiling state (`profiling`, `profFID`, `profNames`, `curProfID`)
in `pkg/codegen/codegen.go` (`New` inits `curProfID:-1`), `initProfiling`
(source-order fid table) + `profID` helpers, and `GenerateAndCompile` injects
`profNameTableC(g.profNames)` + `profRuntimeC()` right after
`concRuntimeAPIC()` — a bounded static C runtime (`pkg/codegen/prof_runtime.go`:
K_PF_MAX_FUNCS 512/EDGES 4096/PATHS 4096/DEPTH 256/PATH 32/LIVE 8192,
`k_pf_overflow` flag, QPC on Windows reusing the preamble's already-included
`windows.h` prototypes — no manual decls, `clock_gettime(CLOCK_MONOTONIC)`
elsewhere, `#undef/#define malloc/free` wrappers emitted AFTER the preamble so
only generated user-code allocations are counted, JSON dump via
`atexit(karkain_prof_flush)` to `$KARKAIN_PROF_OUT`). Hook sites (enter after
frame_enter/set_line, leave on OpRet/fallthrough before frame_leave — also in
`genReturnStmt` after the frame value is computed; getArgs enter/leave; main
`karkain_prof_init()` before quantum_init) in `pkg/codegen/codegen.go` and
`pkg/codegen/emit_ir.go`. CLI: `pkg/cli/prof.go` (`ProfCommand` + `Profile`
schema + `RenderJSON`/`RenderFolded`/`RenderText` + `nsString` +
`parseProfDump` + `makeProfDumpPath` unique temp dump) and
`cmd/karkain/main.go` `prof` dispatch + `runProfCommand`/`printProfHelp`.
Exit codes re-used: 0 success, 1 failure (kcc target, wasm32-wasi target —
explicit "no silent native fallback" diagnostics, missing dump, program
failure), 2 usage (unknown `--format`, missing file via `ValidateKarFile`),
3 compile. Determinism proven: fib(18) = exactly 8361 fib calls, stable
schema/names/counts across runs; raw timings vary (never golden-tested).
Program stdout passes through. Boundaries (documented, rejected explicitly):
kcc engine deferred, WASM deferred, single-threaded (concurrency programs
out of scope), allocation metrics = only malloc/free text in generated code
(preamble helpers like make_string/make_map uninstrumented; structs are
map-based so our struct probes report count 0). Notes: `alloc(T,n)` raw
pointers emit broken `make_int(n) * sizeof(...)` on default builds (pre-existing,
unrelated); `while` requires parens `while(...)` — `while x < n {` WRONGLY
parses `if`/bodies into the condition (caused infinite-loop hangs in probes);
struct-in-array programs infinite-loop on both engines (pre-existing,
unrelated to profiling). Gates:
`pkg/codegen/phase110_profiling_test.go` (3: instrumentation markers + fid
order, opt-in — default builds carry NO hooks, compile+run+raw-dump validation
nat schema/overflow/fib 8361/main 1) + `pkg/cli/phase110_profiling_test.go`
(5: text report markers + passthrough, json schema/counts/edges via
`strings.Index(out,"{")` slice, folded stacks (skip passthrough lines), output
file, boundaries kcc/wasm/bad-format/missing-file). Regressions green: full
`pkg/codegen` 23.3s, CLI Phase 100–109 gates + conformance 59/59 + probes
corpus 524.7s, pkg/sema/runtime/wasm/compiler, lexer/parser/ir/ssa/hir/source/
module/diagnostics/backend/npu/pm, `go vet`, `go build ./...`. Examples:
`examples/profiling/{basic,recursion,hotspot}.kark` + README.
`prof` builds leave `*.c` files beside source (existing CLI behavior) — delete
generated `.c` before committing. Report:
`docs/audit/PHASE-110-PROFILING-DIAGNOSTICS-FINAL-REPORT.md`.)
Also completed: **109** — Standard Library v2
(Real `.kark` programs can `import std.string / std.collections / std.io /
std.encoding / std.crypto` through the normal toolchain on BOTH engines — Go
front end and the self-hosted kcc engine — byte-identical. Five canonical
`.kark` modules; eight byte-level runtime builtins backing the encoding/crypto/
collections surface (`hex_encode_bytes`, `hex_decode_bytes`,
`base64_encode_bytes`, `base64_decode_bytes`, `utf8_valid_bytes`,
`sha256_hex`, `sha512_hex`, `map_keys_of`) wired into the Go resolver
(`builtinNames` in `pkg/sema/resolve.go`), Go codegen preamble/helpers + genExpr
dispatch (`pkg/codegen/codegen.go`), kcc checker/sema tables
(`src/compiler/checker.kark`, `src/compiler/sema.kark` — incl. newly added
`readLineEOF`/`listFiles` arity-1 entries) and kcc C-helper emission
(`src/compiler/codegen.kark` `emitLine` preamble; helpers emitted as single
one-liners after `karkain_appendArray`). NIST FIPS 180 vectors (sha256 "abc"
`ba7816bf...`, sha256 "" `e3b0c442...`, sha512 "abc" `ddaf35a1...`) and RFC
4648 hex/Base64 verified byte-identical on both engines; malformed hex/base64
raise the SAME `runtime error: invalid (hex|base64) string at <file>:<line>`
(Phase 100 model) and `import std.does_not_exist` is rejected on both.
`kccAssembleSource` (`pkg/cli/kcc_engine.go`) is now module-aware (uses
`resolveSourcesRun`) and strips dotted `import std.x` lines (regex
`^[ \t]*import[ \t]+[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z0-9_]+)*`), preserving
`import "C" { ... }` blocks. Go parser requires `while(...)` parens (`if` does
not). PowerShell 5.1 `Set-Content -Encoding UTF8` writes a BOM that breaks
mid-assembly module loads (`unexpected token '�'`) — write stdlib/.kark files
UTF-8-no-BOM. IO handles are shared: `io_close` before `io_delete_file` on
Windows or the file stays locked. String-array print differs (Go `["x"]` vs
kcc `[x]`) — examples iterate instead. Gates: `pkg/cli/phase109_stdlib_test.go`
(5 golden examples incl. UTF-8 byte round-trips `68c3a9...`, multi-module E2E
`examples/stdlib_v2` with `sha256("karkain") =
00e0cba20c10cac449eb885a9926a4b646f0ac163ed7fbc5704d9d8a057ef44d` +
determinism, module-assembly guard, negative suite, missing-module rejection)
+ `pkg/sema/phase109_builtins_test.go`. Regressions green: Phase 106/107/108
CLI gates, `pkg/wasm`, `pkg/compiler` (incremental), `pkg/codegen`,
lexer/parser/sema/ir/pm/source/diagnostics/module/backend/npu/runtime,
`go vet`, `go build`. WASM: new builtins are K108-gated (native-only) — no
WASM change, documented boundary. `stdlib/core`, `stdlib/math`, `stdlib/system`,
`stdlib/gpu`, `stdlib/async` remain behind this phase (non-canonical syntax /
no builtin backing). Report:
`docs/audit/PHASE-109-STANDARD-LIBRARY-V2-FINAL-REPORT.md`.)
Also completed: **108** — WASM Target
(Karkain-owned wasm32-wasi backend in new package `pkg/wasm/` — a
dependency-free handwritten WASM binary v1 emitter:
`module.go` (value model + encoder), `emit.go` (section serialization),
`runtime.go` (embedded WASI runtime: `fd_write` via
`wasi_snapshot_preview1.fd_write`, unboxed i64 ints `v=i64<<1`, boxed
container cells `(ptr<<1)|1` tag/len/data, heap base 0x10000 global 0,
scratch nwritten@0/iovs@8/digits@16–144, `rt_Write`/`rt_PrintValue`/
`rt_Box`/`rt_SetTag`/`rt_MkArray`/`rt_MkString`/`rt_Eq`/`rt_Ne`/`rt_Error`,
24 runtime bodies = wasm indices 1–24, user-func index =
`len(mb.Codes)+i+1`, rt.Eq=11, rt.Ne=12) and `backend.go`
(`CompileProgram` imports+pre-indexes user funcs first, walks FuncDecl
bodies, `scanUnsupported` K108-gates floats/maps/slices/C-interop/
concurrency with `error[K108]` diagnostics; `spew`-level no, no `I32Mul`/
`I32Shl` — `I64ExtendI32U;I64Const(8);I64Mul;I32WrapI64`, `BeginIf
(noResult)`=`04 40`). Root-cause fix: `eqLocals` had 7 entries
`{I32×6,I64}` so p7 was i32 while the shifted i64 accumulator wrote through
p7 → wasmtime `expected i32, found i64`; corrected to 6
`{I32×5,I64}` (comment `// p2=cellA p3=cellB p4=tagA p5=n p6=i p7=acc`).
CLI: `pkg/cli/wasm.go` `wasmBuildCommand`/`wasmRunCommand`/`findWasmtime`,
dispatched from BuildCommand/RunCommand on `--target wasm32-wasi`
(exit codes: ExitCompile=3 for K108, ExitEnv=6 = no wasmtime, ExitFailure=1,
ExitSuccess=0; `wasmtime run --dir . <tmp.wasm>` stdio passthrough;
findWasmtime: LookPath then `$HOME/bin/wasmtime{.exe}` — Windows mode has
no execute bits so dropped `Mode()&0o111`). Gates: 9 backend tests
(pkg/wasm/backend_test.go — TestHello/ArithmeticAndCalls/
ControlFlowAndRecursion/ArraysAndStrings incl. string equality/
RuntimeErrorDivByZero `main.kark:3`/DeterministicBuild/UnsupportedFeatures
6-case K108 table/BooleanLogic/ForIn) + 2 E2E CLI tests
(pkg/cli/phase108_cli_test.go, filename avoids go-build GOARCH trap:
`*_wasm_test.go` is build-constrained to GOARCH=wasm and lands in
IgnoredGoFiles on windows/amd64 — use non-GOARCH suffixes like
`phase108_cli_test.go`): TestPhase108WasmE2E builds
`examples/wasm/hello.kark` via real `karkain build --target wasm32-wasi` and
wasmtime-runs to byte-exact `hello wasmtime/42/done`, determinism test
proves byte-identical 2488-byte repeats. Regressions green: pkg/wasm full,
Phase 106/107 codegen+runtime gates, Phase 95/96/97/105/106/107 CLI gates,
`go vet`, `go build ./...`; compiled standalone `karkain.exe`
build+run byte-exact. kcc parity for the wasm target = documented post-108
boundary (Go engine is the reference). Report:
`docs/audit/PHASE-108-WASM-TARGET-FINAL-REPORT.md`.)
Also completed: **107** — Concurrency Runtime
(A work-stealing task scheduler with channels and actors usable end-to-end
from `.kark` through a compiler-neutral C runtime embedded in the generated
assembly. Runtime under `runtime/concurrency/c/`
(`karkain_conc.h`/`karkain_sched_impl.h`/`karkain_scheduler.c`/
`karkain_channel.c`/`karkain_actor.c`/`concurrency_main.c`): `karkain_sched_t`
workers with a bounded grab queue, per-worker wake condvar, shared pending
drain and no busy-spin stop; `karkain_task_t` heap cells (fn ptr + heap-owned
`ctx` freed by the runtime, atomic `done`, cv, `status` long — negative =
failure); bounded/unbounded blocking channels with deterministic
close-drain; actors as serialized dispatcher jobs over a mailbox with a
state-cell box; `karkain_conc_atomic_*` GNU atomic helpers in the runtime
only. Embedded via `runtime/concurrency/embed.go` (`//go:embed c/...`).
Compiler integration: parser `spawn(fn, args...)`/`receive(ch)` expression/
`channel(...)`+`actor(...)` keyword calls (legacy `receive(ch) -> var`
kept), sema `builtinNames`, codegen `pkg/codegen/conc_runtime.go` (recursive
pre-scan → `usesConcurrency` + stable spawn-site wrapper indices +
actor-handler ids; per-site `karkain_run_<idx>` wrappers; actor dispatcher
adopting returned state with int-0 = no change; `concRuntimeAPIC` Value
glue), `GenerateAndCompile` header/wrapper/glue emission gated on
`usesConcurrency`. Language surface: `spawn/join/wait_all/channel/chanSend/
chanClose/receive` and `actor/actorSend/actorState/setActorState/actorStop`
(`send` is a lexed keyword → `chanSend`). Gates: `pkg/runtime/
phase107_concurrency_test.go` (7 scenarios + >1M msg/s), `pkg/codegen/
phase107_concurrency_test.go` (7: spawn 42/10/-1/done, spawn statements,
channels 10/20/0, actors `6`, embedded markers, 999-task stress
`332833500`, generated-source presence), `pkg/cli/phase107_concurrency_test.go`
(2 E2E: `examples/concurrency/pipeline/main.kark` → `144/10/20/30/0/6` +
byte-identical repeats). Regressions green: full `pkg/cli` 1723.8s
(conformance 59/59 + all Phase 97–107 gates), whole `pkg/codegen`,
sema/parser/lexer/ir/pm/source/diagnostics/compiler/module,
backend/npu/runtime, `go vet`, `go build`; bootstrap stage-1 builds (Go
codegen change safe for the compiler's own sources); stage-2 build SEGFAULT
on the ~3.9GB-RAM host is the documented kcc-build-mode OOM class (no
`src/compiler` changes; concurrency codegen gated by `usesConcurrency`,
unused by compiler sources; Go-engine `check` of `src/compiler/main.kark`
clean). kcc already lexes/parses the keywords (`TK_SPAWN/TK_SEND/...`);
codegen parity is a documented post-107 boundary. Report:
`docs/audit/PHASE-107-CONCURRENCY-RUNTIME-FINAL-REPORT.md`.)
Also completed: **106** — SIMD & Vector Types
(Lane-vector types: `[N]f32`/`[N]f64`/`[N]i32`/`[N]i64` variable annotations
select Karkain-owned C lane types `karkain_<elem><width>x<lanes>` (float →
`__m128`/`__m256` with auto `-mavx` for 256-bit widths on x86; int → GNU
`vector_size` types), and `@simd_splat/add/sub/mul/div/sum` lower to portable
`karkain_simd_*` runtime helpers using GNU vector operators — one body serves
x86 intrinsics and ARM `vector_size` types; integer mul/div and every sum
reduce lane-wise via scalar loops. Type flow: SIMD declaration records
name→type in `Generator.simdVars`, operand dispatch takes lane width from the
vector operand, splat seed infers f32 vs i32 (raw literal or `.floatVal`/
`.intVal`, never an invalid Value→scalar C cast), `@simd_sum` reduces a lane
vector to a printable scalar Value, scalar-only operands keep the Phase 70 SSE
fallback. Integration: `pkg/codegen/simd_emit.go` (new) + `genSIMDDecl`/typed
`genSIMDExpr`/header-runtime assembly/`appendAVXFlags` in `pkg/codegen/codegen.go`,
SSA `lowerStmt` → raw-c SIMD decls in `pkg/codegen/lower.go`. Bonus hardening:
the Phase 14 AVX2 matrix kernel now uses portable `mul+add` (no
`_mm256_fmadd_pd`), so `-mavx2` builds no longer require `-mfma`. Gates:
`pkg/codegen/simd_emit_test.go` (7) + updated `phase70_test.go` +
`pkg/cli/phase106_simd_test.go` (3 E2E: golden executable output
40/-8/48/12/5/56/40, SIMD-vs-scalar differential, AVX assembly probe
`vaddps`-family under the pipeline's own `-O0 -mavx`). Regressions green: full
`pkg/cli` 940.5s (conformance 59/59, all Phase 97–105 gates), whole
`pkg/codegen`, sema/parser/ir, backend/npu/runtime/diagnostics, `go vet`,
`go build`; bootstrap stage-1 builds (Go codegen change safe for the
compiler's own sources); stage-2 build SEGFAULT on the ~4GB-RAM host is the
documented kcc-build-mode OOM class (no `src/compiler` changes; `@simd`
unused by compiler sources). kcc parity boundary documented (parser already
accepts `@simd_*` via `NODE_SIMD_BUILTIN`; semantic/codegen parity post-106).
Report: `docs/audit/PHASE-106-SIMD-VECTOR-TYPES-FINAL-REPORT.md`.)
Also completed: **105** — Error Recovery & Incremental Compilation
(Multi-error reporting: engine-agnostic project-wide syntax preflight
`pkg/cli/multierror.go` parses every unit file independently and renders ALL
recoverable parse errors in one invocation with exit 3 through BOTH check
engine paths — Go `CheckCommandFormatted` and default-kcc
`KCCCheckCommand`, which previously `[ok]`-ed multi-error inputs; the
resolve stage aggregates its diagnostics the same way, so
`examples/phase105_errors/` proves 6 recoverable parse errors (multierr.kark:
`@bad_attr_*` + `public <non-decl>` + expression-shape) and 3 resolve errors
(semantic.kark) surface together. Incremental compilation: content-addressed
whole-assembly cache in new package `pkg/compiler` (per-module content
sha256 + interface hashes — public func param types / struct-enum headers /
top-level var headers / imports only, never bodies; compiled/reused/
invalidated; dependency-aware invalidation; compiler-identity key; atomic
writes; failed builds never Store); `karkain build --incremental` /
`--incremental-cache` via `pkg/cli/incremental.go` and `--incremental` flags
in `cmd/karkain/main.go`; `karkain clean` purges `.karkain-cache`. Real
3-module example `examples/phase105/` (main+math+strings). Gates:
`pkg/compiler/incremental_test.go` (10) + `pkg/cli/phase105_incremental_test.go`
(3 E2E through gcc) + `pkg/cli/phase105_multierror_test.go` (5) — all PASS;
measured no-op 101 ms vs clean 2790 ms; post-clean rebuild byte-identical C
and stdout; full regression suite green incl. entire `pkg/cli` (conformance
59/59, all Phase 97–104 gates) 1066.8s, other pkg suites, `go vet`, `go build`.
Design decision (documented): whole-assembly cache; per-module `.o` TU
splitting deferred as post-105 work on the same interface machinery.)
Also completed: **104** — Debug Information (DWARF)
(Karkain-owned DWARF 4 debug sections in native executables:
`.debug_info`/`.debug_abbrev`/`.debug_str`/`.debug_line` emitted by
`DwarfEmitter` in `pkg/codegen/dwarf.go` from the Phase 84 `DebugInfo`
model — compile unit, subprogram per function with nested local variables,
base-type DIEs, `DW_AT_high_pc` size form over relocated addresses, and a
correct v4 line-number state machine with per-function end-sequence resets.
Self-hosted DWARF-4 reader `pkg/codegen/dwarf_parse.go` round-trips the
sections and powers the readelf-style `DwarfTextDump`; linker lays out
`SectionTypeDebug`; `NativeBuilder.Build`/`BuildMultiObject` attach DWARF
after `relocateDebugAddresses`; new `Executable.GetSection`,
`GetDebugSections`, `HasDebugSections`. Gates `pkg/codegen/dwarf_test.go`
(8 tests) + `pkg/cli/phase104_dwarf_test.go` (3 E2E through the real
pipeline) — all PASS; regressions green: conformance 59/59, Phase 102
Go+kcc goldens, compiler-sources self-check, probes, all pkg suites,
`go vet`. Known limitation (documented, NOT a defect): no ELF/PE container
writer yet, so debuggers/readelf remain on the gcc C-transpile path; the
DWARF sections are ready for embedding when a container writer lands.
Tier 3 (Phases 104–109) is now in progress.)
Also completed: **103** — Module System v2 (Core)
(`public` is a real export modifier accepted by BOTH engines: Go parser sets
`.Public` on func/type/enum; self-hosted kcc gained a `TK_PUB` branch mirroring
Go's parse-error contract (`public let` rejected on both engines). Qualified
calls `math.twice(21)` now resolve against the imported module's export set in
the Go resolver (`checkQualifiedCall`, `pkg/sema/resolve.go`) with precise
diagnostics — private cross-module call, missing `import`, undefined module
function, wrong-module target, cross-module duplicate — all exit 3. The
self-hosted parser lowers dotted calls onto the flat `karkain_user_*`
namespace identically for module qualifiers and record-idiom receivers
(`acc.deposit(10)` == `math.twice(21)` == bare callee); `C.*` stays dotted for
C-interop. Stdlib files are exempt from the private-export rule (framework API
surface; physical `public` markers deferred to the Phase 109 stdlib-v2
boundary). Acceptance: `examples/module_system/` byte-identical output on both
engines; 4 rejection fixtures `examples/module_system_errors/`; gate
`pkg/cli/phase103_module_test.go` (Go golden 4.8s, kcc golden 10.6s, kcc
accept-check, rejections, compiler-sources self-check 41.9s); regressions:
sema/parser/codegen/vet + Phase 97/99/100/101/102 + conformance 59/59 —
all green. Report: `docs/audit/PHASE-103-MODULE-SYSTEM-FINAL-REPORT.md`.)
Also completed: **99** — Self-Hosted Parser & Type Checker
(Certified: READY TO COMMIT WITH DOCUMENTED ENVIRONMENTAL LIMITATION — committed
and merged). **100** — Runtime Error Model (runtime error diagnostics with source
location, checked division/modulo/indexing, cross-engine parity). The self-hosted
compiler now parses the full language surface and type-checks programs itself:
`src/compiler/atypes.kark` (conservative static type inference: `inferType`,
`primitiveTag`, `isPrimitiveTag`, `typeDescription`, `annotationMismatch`; the
"a" prefix sorts it after `ast.kark` for the letter-order assembler) and
`src/compiler/checker.kark` (two-pass checker mirroring `pkg/sema/resolve.go`:
pass 1 `collectDecls`/`collectLocals`, pass 2 `checkStmt`/`checkExpr`/
`checkCall`/`checkIdent`; K101 undefined call, K102 undefined identifier,
K103 function/kernel arity, K104 builtin arity, K106 undefined struct type,
K107 duplicate definition, K108 break/continue outside loop, K109 calling a
type name, K112 primitive annotation/initializer mismatch — conservative,
provable-from-AST only, so the compiled corpus and the compiler's own sources
keep passing). `src/compiler/main.kark` `checkFile` runs `typeCheckProgram(ast)`
check-only: `error[K1XX]` lines map to exit 3, clean files still print `[ok]`;
build/run/test codegen untouched (bit-identical to Phase 98). Gate
`pkg/cli/phase99_selfhosted_test.go` (CorpusAccept, CrossFileDuplicateDetected
at assembly scope, ErrorFixturesRejected 14/14, CompilerSourcesTypeCheck —
assembled sources 214,127 bytes type-check clean) — PASS 54.58s; fixtures under
`examples/type_errors/`. Bootstrap identity PASS 347.60s, stage2==stage3,
1,432,956 bytes, SHA `aff1d624d9e52c2d`; isolated `pkg/cli` 634.128s and
`pkg/bootstrap` 571.688s; `go vet` clean. Harness stabilizers: bootstrap
subprocess timeout 2min→5min (measured clean transpile ~145–174s exceeded the
120s budget), and `phase88`/`phase95` tests pin `KARKAIN_ENGINE=go` (legacy
contract + stage-1 determinism) — no production semantics changed. Known
environmental limitation (documented, NOT a defect): on the ~4GB-RAM
limited-paging host, concurrent full-tree `go test ./pkg/...` runs stall
`TestBootstrap_BitwiseIdentity` and `TestConformanceCorpus_RunsClean`; both pass
in isolation. Full report: `docs/audit/PHASE-99-FINAL-REPORT.md`.
Also completed: **100** — Runtime Error Model
(Runtime error diagnostics with source filename and line number: division/modulo
by zero, array/string index out of range now report `runtime error: <kind> at
<file>:<line>` and exit(1) instead of silently returning zero. Both Go and
self-hosted kcc engines produce identical diagnostics. Implementation:
`karkain_runtime_error()`, `karkain_checked_div/mod/get/set()` in
`pkg/codegen/codegen.go` and `src/compiler/codegen.kark`, source file tracking
via `sourceBaseC()` and `fileBaseName()`, enhanced string concatenation to
support int/float/bool operands for diagnostic embedding. Test fixtures under
`examples/runtime_errors/` (7 error cases + 1 positive control), parity gates
`pkg/cli/phase100_runtime_test.go` and `pkg/codegen/phase100_runtime_test.go` —
all PASS.)
Also completed: **101** — Libc-Free Runtime Foundation
(libc-free/libm-free Karkain-owned runtime layer established: `runtime/freestanding/`
with arena allocator (`karkain_memory.c`), raw OS abstraction (`karkain_platform.c/h`),
I/O, string and math primitives that compile and link WITHOUT libc — gate
`pkg/runtime/phase101_freestanding_test.go` compiles and runs a hello program;
runtime error stack traces added end-to-end: `KARKAIN_MAX_FRAMES` 128,
`karkain_frame_enter/leave`, `karkain_set_line`, runtime-error `stack:` dump on both
engines (kcc and Go produce identical `inner:2/outer:5/main:9` frames for
`examples/runtime_errors/stack_chain.kark`); Go side: `pkg/codegen/codegen.go`,
`pkg/ir/ssa/ssa.go` (`Function.Line`), `emit_ir.go`; self-hosted side:
`src/compiler/codegen.kark` (frame helpers, `FuncDecl` prologue, return wrapper
`{ Value _karkain_fret = ...; karkain_frame_leave(); return _karkain_fret; }`);
latent self-hosted checker crash fixed (`kcc check` OOB on bare `return` empty
value nodes — empty-node guards in `src/compiler/checker.kark`) plus `parseFunc`
func-token line fix in `src/compiler/parser.kark`; regression gates
`pkg/cli/phase99_selfhosted_test.go` / `phase100_runtime_test.go` /
`phase101_stacktrace_test.go` all PASS (CompilerSourcesTypeCheck clean on all 9
self-hosted sources); GMP remains an intentional isolated boundary deferred to
Phase 109; pre-existing limitation (not a blocker): closure/`fn` codegen is broken
on both engines, covered by no gate, out of Phase 101 scope — docs under
`docs/audit/PHASE-101-FINAL-REPORT.md`).
Also completed: **102** — Native Runtime Core
(Karkain-owned libc-free runtime foundation `runtime/core/` stacked directly on
the Phase 101 freestanding layer: `karkain_mem` heap with real
`alloc/calloc/realloc/free` on an address-sorted coalescing free list; tagged
`Value` int/float/bool/string/array with codegen-parity naming/layout
(`ValueType`, `TYPE_*`, `make_int/…/make_array`, `values_equal`,
`value_truthy`); length-prefixed native strings `NativeString` (concat/slice/
compare/startswith/Value round-trip); native arrays `NativeArray`
(push/pop/get/set/clear, codegen-parity `Value** items` storage); value-level
I/O on freestanding write primitives; gate `pkg/runtime/phase102_core_test.go`
compiles core+freestanding with `-ffreestanding -nostdlib` and asserts exact
output. Directly-related freestanding fixes: `karkain_memory.c` rewritten as a
linked-segment arena (old arena reloc'd every outstanding pointer on grow and
could fail to make room — only guaranteed `add>=need`, not `add>=used+need`;
the old hello gate only ever allocated once, so it never tripped) +
`karkain_arena_reset`; `karkain_mem_reset` clears heap bookkeeping after arena
teardown. Compilers untouched (both engines keep passing); GMP/bigfloat and
map/option/result stay behind the Phase 109 boundary; codegen embedded-runtime
adoption is a future phase. Docs under `docs/audit/PHASE-102-FINAL-REPORT.md`).
Also completed: **102‑F** — Language Foundation Completion
(Language surface round-trip proven on BOTH engines with a golden-output
corpus: 13 targets under `examples/language_foundation/` (variables→functions→
recursion→arrays→strings→collections→control flow→structs→methods(record
idiom)→modules(sibling assembly)→application layout→error handling) producing
byte-identical stdout + exit codes on Go and kcc engines. Cross-engine parity
bugs fixed: kcc `parseStructDecl` dropped every top-level function after a `;`
field separator (`{ owner string; balance int }` ate the source to the next
`}`) — the application `deposit()` regression; Go BorrowChecker `fnRoot`
flag stops dead-name propagation out of function/lambda root scopes (params no
longer poison the global scope and false-flag struct-literal keys like
`Counter{ value: 0 }` as "used after scope end"); kcc C-style `for` empty-init
`for (; k < 3; ...)` and `for (let i = 0; ...)` (parseFor let/var init +
`"; "`/`is_truthy()` emission in `src/compiler/parser.kark`/`codegen.kark`);
Go `genForStmt` double-`;;` on let-init trim in `pkg/codegen/codegen.go`.
Gates `pkg/cli/phase102_foundation_test.go`: Go golden 44.6s PASS, kcc golden
(13 targets, isolated kcc) 30.5s PASS, compiler-sources self-check
(`check --engine kcc` of `src/compiler/main.kark`, Phase 99 gate survival)
55.6s PASS; targeted regressions green: Phase 99 self-hosted
(CorpusAccept/CrossFileDuplicate/ErrorFixtures 14/14/CompilerSourcesTypeCheck
58.8s), Phase 100 runtime parity, Phase 101 stack parity, pkg/codegen Phase 100,
pkg/sema Phase 51 borrow; `go vet` clean; `karkain fmt` semantic-preserving.
Not supported on either engine (kept out of corpus, parity preserved):
`float64()/bool()/string()` casts, closures/`fn` codegen, `const`, user
`import`, visibility. Known environmental limitation (not a defect): on the
~4GB-RAM host kcc BUILD mode for the full compiler sources OOM-stalls/SEGFAULTs
(known class, see Phase 99/101 notes); the low-memory `check` path passes.
Docs under `docs/audit/PHASE-102-LANGUAGE-FOUNDATION-FINAL-REPORT.md` +
SPEC §14 Capability Summary).
Also completed: **98** — NPU Integration into Compiler
(`@target(...)` function attribute implemented end-to-end: parser attaches the
attribute to `FuncDecl` (`pkg/parser/ast.go` `Target`, `parseTargetAttr` in
`pkg/parser/parser.go`, preserved by `pkg/parser/macro.go` expansion), semantic
validation in `pkg/sema/npu_check.go` (`targets: cpu, npu` — unknown targets
rejected with `error[K004]`, exit 3), C codegen emits a `// @target(...)`
comment marker, and `pkg/codegen/npu_compiler.go` adds the `NPUDispatcher`
with `DispatchRequest`/`DispatchResult` and matrix multiplication as the first
concrete NPU operation: `OpMatMul` dispatches to an available vendor NPU
adapter (intel/qualcomm/apple/amd/arm — `backend->Execute` must return computed
values) and ALWAYS falls back to the CPU reference backend (`pkg/backend/cpu`,
the correctness oracle) when no adapter is available, the adapter errors, or it
stubs out — so `@target(npu)` functions compile and run everywhere with no
NPU/SDK/driver/cloud dependency; the self-hosted compiler was taught the
attribute too (`src/compiler/parser.kark` `parseTargetAttr` claims top-level
`@target(name)` before `func`, `ast.kark` `setFuncTarget`/`funcTarget`, `codegen.kark`
emits the C comment instead of an invalid `target(npu);` statement), and the
default kcc engine's check/build/run/test paths run a Go-side NPU target
preflight (`kccTargetPreflight`/`kccStagedPreflight`) so `karkain check`
rejects unknown targets on BOTH engines; CLI wiring in `pkg/cli/checker.go`
(`npuTargetDiagnostics` in `AnalyzeSource`) + `pkg/cli/lint.go`; parity tests
`pkg/sema/phase98_npu_test.go` (8), `pkg/codegen/phase98_npu_test.go` (10,
mock NPU backend with software-emulated execute proving NPU==CPU results) and
`pkg/cli/phase98_npu_test.go` (8) all green; docs under
`docs/npu-targeting.md` + `docs/audit/PHASE-98-FINAL-REPORT.md`).
Also completed: **97** — Default kcc Engine + Manifest Dependency Resolution
(kcc is now the DEFAULT engine: `EngineFromEnv` returns `EngineKCC` for empty
env/unrelated `KARKAIN_ENGINE` values, Go selected only via explicit
`KARKAIN_ENGINE=go|Go|GO`; `--engine go|kcc` unchanged; manifest dependency
resolution wired into the source-assembly pipeline: new `kccAssembleSource`
(local deps → siblings → root last) feeds `KCCCheckCommand`/`KCCBuildCommand`/
`KCCRunCommand` temp-sandbox builds, `projectModuleSources` prepended to each
test driver in `KCCTestCommand` (dependency-aware test compilation), and
`resolveSources` (check/build/run) + `projectModuleSources` (test) pull
`pm.DependencySources` entries (local & workspace) for all engines; bootstrap
pipeline pins `KARKAIN_ENGINE=go` (`forceGoEngine` in `pkg/bootstrap`) so stage-1
still emits `src/compiler/main.c` through the Go front end; E2E exit-code tests
pin Go deterministically; parity gate `pkg/cli/phase97_parity_test.go` (6 tests:
default-engine flip, explicit Go fallback, local & workspace deps in assembly,
end-to-end kcc run using a dependency function, shared check/build/run/test
dependency-aware pipeline), plus existing parity gates (95/96) green and
`TestBootstrap_BitwiseIdentity` stage2==stage3 bitwise identical; docs under
`docs/audit/PHASE-97-FINAL-REPORT.md`).
Also completed: **96** — Self-Hosted kcc Owns the Test Runner
(kcc is the primary engine for `karkain test`: self-hosted discovery in
`src/compiler/main.kark` (`collectTestFiles` mirrors Go `findTestFiles` —
recursive `*_test.kark` discovery, sorted, single-file fallback, empty
directory reports "No test files found."), per-test driver synthesis
(parse → collect `test_*` → generated `main` calling selected tests),
gcc compile+run with per-test PASS/FAIL, full-file fast path, `--filter`
substring support, and Go-parity summary output (`N passed; M failed;
S skipped; T total`) parsed by `KCCTestCommand` (`karkain test
--engine=kcc` / `KARKAIN_ENGINE=kcc`) which maps `failed>0` → `ExitTest(4)`
and runs kcc in a temp sandbox; root-cause fixes: `INT==BOOL` `values_equal`
mismatch from `endsWith(...) == true` → bare truthiness for INT/BOOL, Windows
`system("./x")` → bare `.exe` name, empty-directory vs single-file
classification via `readFile`/suffix probe; parity gate
`pkg/cli/phase96_parity_test.go` (conformance parity 59 assertions, failing
test, single-file+filter, empty directory); bootstrap identity
`TestBootstrap_BitwiseIdentity` stage2==stage3 bitwise identical
(stage2/stage3 SHA `f39111a2…`) and conformance corpus 59/59 all pass;
docs under `docs/audit/PHASE-96-FINAL-REPORT.md`).
Also completed: **95** — Self-Hosted kcc Owns the Core Pipeline
(self-hosted compiler from `src/compiler` is the primary engine for
lex+parse+sema+C codegen; `karkain check/build/run --engine=kcc` or
`KARKAIN_ENGINE=kcc` decompiled through kcc; `pkg/cli/kcc_engine.go` with
`KCCCheckCommand`/`KCCBuildCommand`/`KCCRunCommand` (temp-sandboxed run), a
staleness check rebuilding `kcc.exe` when any `src/compiler/*.kark` is newer,
and Go fallback for LSP/test-runner/exotic backends; CLI `--engine go|kcc`
flag + env wiring in `cmd/karkain/main.go`; kcc parity fixes for type-keyword
annotations, unparenthesized if, map/struct literals, slices, index/member
assignment, assertions, appendArray Value* and struct-decl comments;
parity gate `pkg/cli/phase95_parity_test.go` proving kcc reproduces all 12
probe goldens + all 11 conformance files (59 assertions); verified
`TestBootstrap_BitwiseIdentity` stage2==stage3 bitwise identical;
docs under `docs/audit/PHASE-95-FINAL-REPORT.md`).
Also completed: **94** — SSA Optimization Pipeline
(Multi-pass optimizer: Mem2Reg, FoldConst with algebraic simplification,
CSE, DCE, LICM for natural loops; Pipeline orchestrator with fixpoint
iteration and stats; 49 passing tests including benchmarks; `pkg/ir/ssa/`).
Also completed: **93** — Typed SSA IR
(TypeRegistry, TypeBits, TypePromote, TypeIsCompatible, dominance tree
(Cooper et al.), liveness analysis, SSA verifier; 30 tests; `pkg/ir/ssa/`).
Also completed: **92** — HIR Infrastructure
(High-Level IR with typed nodes: 20 expression kinds, 16 statement kinds,
19 type kinds, AST→HIR lowering via BuildHIR(), Format() for debug output,
7 passing tests including 5-program round-trip; `pkg/ir/hir/`).
Also completed: **91** — Comprehensive Validation & Release Decision
(audit of all documentation for consistency: SPEC.md version 0.14.0→1.0.0,
self-hosted compiler reference Phase 56→88; stdlib.md import examples updated
for async/gpu modules; full regression suite GREEN: lexer/parser/sema/ssa/pm/
codegen/backend/npu/source/module/diagnostics all pass, CLI passes with 300s
timeout for conformance corpus; release decision: KARKAIN 1.0 — RELEASE READY;
`docs/audit/PHASE-91-FINAL-REPORT.md`).
Also completed: **51** — Borrow Checker Lexical Scoping
(scope-stack identity hardened for shadowing, borrow reversion via declaring scope,
use-after-scope-end diagnostics, escape-analysis flag wiring, Groups A–I tests).
Also completed: **81** — Compiler Correctness (immutable/mutability semantics,
escape-analysis wiring, executable probes under `examples/phase81-probes/`,
diagnostics hardening; unparenthesized `if` + single-path `%`).
Also completed: **82** — Language Conformance, Examples & Developer Tooling
Foundation (native conformance corpus `conformance/` — 9 files, 48 `func test_*`
tests through the real front end + C runtime, self-contained file scope via
`testFileOwnScope`; deterministic probes corpus `examples/probes/` — 11 golden
output programs enforced by `pkg/cli/probes_corpus_test.go`; algorithm corpus
grown to 21 dirs (factorial/fibonacci/gcd/lcm/sieve/power/absolute_value);
toolchain contract `check --format=json` (`karkain-diagnostics-v1`, real parse
columns via `Parser.ErrorCols`) + `fmt`/`fmt --check` (idempotent token-level
canonicalizer); real LSP served by `karkain lsp`/`language-server`; `ide info`
JSON contract; VS Code extension rebuilt (`extension.js` commands check/compile/
run/format, fixed manifest, corrected grammar) validated by
`pkg/cli/vscode_extension_test.go`; CI runs conformance + format checks; docs
under `docs/audit/PHASE-82-*`).
Last completed: **83** — Compiler Symbol Namespacing, True Diagnostic Spans &
LSP↔CLI Pipeline Sharing (deterministic `karkain_user_*` C namespace for user
functions — Go `userFuncC` + self-hosted `codegen.kark` mirror — fixes
C-library collisions like `abs` and makes user functions that shadow builtins
win; `E-K-RES` true columns: lexer 0-based byte columns, parser `Col`/`EndCol`
spans surviving macro expansion, optional `endColumn` + `excerpt` fields in the
`karkain-diagnostics-v1` contract; new `pkg/source` line-index/excerpt package;
single `cli.AnalyzeSource` driver shared by `karkain check` and the LSP with
real-time `didChange` diagnostics sync; conservative undefined-identifier
resolution; array-return semantics; conformance corpus 48→59 tests in 11 files
+ `namespace` probe golden (12 total); docs under
`docs/audit/PHASE-83-FINAL-REPORT.md`).
Last completed: **94** — SSA Optimization Pipeline
(Multi-pass optimizer: Mem2Reg, FoldConst with algebraic simplification,
CSE, DCE, LICM for natural loops; Pipeline orchestrator with fixpoint
iteration and stats; 49 passing tests including benchmarks; `pkg/ir/ssa/`).
Last completed: **93** — Typed SSA IR
(TypeRegistry with 14 primitive types, TypeBits/TypePromote/TypeIsCompatible
type-lattice, Cooper et al. iterative dominance tree (DomTree with IDom,
DomChildren, Dominates/StrictlyDominates/DominanceFrontier/DomTreeDepth),
iterative liveness analysis (LiveRange, Interferences, NumLiveAt),
SSA verifier (undefined register, unterminated block, undefined block target),
30 passing tests across `typed.go`, `dom.go`, `liveness.go`, `typed_test.go`;
`pkg/ir/ssa/`).
Last completed: **90** — Production Release Gate / Karkain 1.0
(production build workflow verified, CLI commands validated: check/build/fmt/lint,
stdlib imports work, runtime type discrepancy resolved as intentional bootstrap
limitation, release acceptance test, 15 focused Go tests, version 1.0.0 consistent,
40+ total tests pass; `docs/audit/PHASE-90-FINAL-REPORT.md`).
Prior completed: **89** — Self-Hosted Runtime & Toolchain
(Karkain-owned runtime boundary with Value type system, container ops,
I/O primitives, platform abstraction, initialization contract;
runtime/ directory with 5 boundary docs; acceptance test proving
self-hosted compiler → C23 → gcc → native executable pipeline;
12 focused Go tests; `docs/runtime.md`; `docs/self-hosted-compiler.md`).
Prior completed: **88** — Self-Hosted Compiler Foundation
(self-hosted Karkain compiler compiles via bootstrap: src/compiler/*.kark
→ C23 → native executable; lexer/parser/AST/sema/codegen in Karkain;
7 representative test programs; 8 focused Go tests; build script;
docs under `docs/self-hosted-compiler.md`).
Prior completed: **87** — Standard Library Foundation
(stdlib structure with core/string/collections/math/io/system modules,
`std.*` import resolution wired into module graph via `findStdlibDir`,
6 focused module-resolution tests; `docs/stdlib.md`).
Prior completed: **86** — Developer Toolchain
(test timeout safety via goroutine+select with 30s default, multi-file
formatter (`karkain fmt .` recursive), exported `Canonicalize` for LSP
reuse, `textDocument/formatting` LSP support, `ExitLint(7)` exit code,
8 focused tests; docs under `docs/audit/PHASE-86-FINAL-REPORT.md`).
Prior completed: **85** — Native Build & Linking Integration
(NativeBuilder orchestrating Object→Linker→Executable pipeline via
`NativeBuilder.Build` and `BuildMultiObject`; CLI integration:
`--target=native-link` flag dispatches `BuildCommand`/`RunCommand` to
`nativeBuildCommand`/`nativeRunCommand`; KOBJ binary artifact format
(sections/symbols); debug address relocation from 0-based counters to
actual linked .text base (0x1000+) via `relocateDebugAddresses`;
SourceAddressMap built from relocated DebugInfo; 8 focused NativeBuilder
tests + 6 CLI integration tests; docs under `docs/native-codegen.md`).
Prior completed: **84** — Native Codegen, Linker & Debug Information
(Karkain-owned object model: sections/symbols/relocations/debug-info;
scope-aware SymbolTable with `karkain_user_*` namespacing via
`CollectSymbolsFromAST`; RelocationManager with Addr32/64, PCRel32/64,
PLT32, GOT32 and byte-level application; Linker (symbol resolution,
section layout, entry-point handling, Executable production);
DebugInfoBuilder + SourceAddressMap bidirectional source↔address mapping;
CodegenDiagnostics wrapper reusing Phase-83 `E-K-CG` diagnostics;
NativeGenerator.GenerateObject AST→Object path; 48 focused tests across
6 new test files; docs under `docs/native-codegen.md`).
Prior completed: **79** — Compiler Integrity, IR Architecture & Self-Hosting
Readiness Audit (evidence-based audit: pipeline, dependency map, Math/Tensor/SSA
IR, CPU/GPU/NPU parity, determinism, optimization boundaries, tests,
BUG-1..8 regression, self-hosting readiness; Phase 80 gate = READY WITH
PREREQUISITES; docs under `docs/audit/PHASE-79-*`).
Prior completed: **78** — NPU Optimization (fusion, memory planning, INT8/INT4 quantization, MLIR codegen);
**70-78** Math/Tensor/NPU chain (see below); **52-69** value/SSA/closures/slices/self-hosting/actors/GPU/quantum/stdlib/borrow/optimizer.

### Completed: Package Manager Hardening (P0)
Deterministic resolver (`pkg/pm/resolver.go`), lockfile-integrated workflows
(`pkg/pm/flow.go`: `ResolveAndLock`, `FetchLocked`, `ResolvedDetails`,
`TreeLines`, `NewProject`), fetch cache safety (atomic temp→rename + checksum in
`FetchModule`).

### Completed: CLI Toolchain (Batches B1–B4) + KTF-001 + KTF-002
Full CLI surface (`bench/lint/explain/clean/workspace/*/target/config/pkg audit
--json/pkg verify --json` + exit-code scheme + `--filter`), the native test
foundation (`karkain test`: language-level `assert/assert_eq/assert_ne`, KTF-001
model, deterministic discovery/execution, `--filter`), and the compile
pass/fail corpus (`karkain test --compile [dir]`: manifest-driven, real lint
pipeline diagnostics, gcc-gated pass cases, exit 4 on failure). Reports:
`docs/audit/KTF-001-REPORT.md`, `docs/audit/KTF-002-REPORT.md`,
`docs/audit/CLI-COMPLETION-IMPLEMENTATION.md`, `docs/audit/CLI-COMPLETION-MATRIX.md`. CLI wiring under single `karkain.exe`: top-level
`new/remove/update/list/tree/fetch` (incl. `karkain pkg ...` aliases);
`update` now re-resolves and writes `karkain.lock`; `fetch` uses the lockfile.
Registry/git fetching remain explicit "not available" errors (not faked).
Tests: `pkg/pm/resolver_test.go`, `pkg/cli/package_cli_test.go` (E2E). Audit:
`docs/audit/PACKAGE-MANAGER-AUDIT.md`,
`docs/audit/PACKAGE-MANAGER-IMPLEMENTATION.md`. Deferred recommended next step:
wire manifest dep resolution into build/run/check (avoided to keep compiler
pipeline stable).

### Completed: Math/Tensor/NPU Chain (Phases 71–78)

```
71 (Math IR) → 72 (Tensor IR) → 73 (CPU Backend)
                                      ↓
                                 74 (Autodiff Integration)
                                      ↓
                                 75 (Backend Abstraction)
                                    ↓        ↓
                               76 (GPU)   77 (NPU) → 78 (NPU Opt)
```
All 8 phases built, tested (full suite green), committed and pushed to `main`.
Backends: CPU reference (`pkg/backend/cpu`, embedded C23 runtime, correct GCC E2E),
GPU/WGSL (`pkg/backend/gpu`), NPU abstraction + 5 vendor adapters (`pkg/npu/*`).
NPU optimization passes: fusion, memory planning, quantization, Karkain-owned MLIR dialect.

### Phase Dependency Chain (Math/Tensor/NPU)

```
71 (Math IR) → 72 (Tensor IR) → 73 (CPU Backend)
                                      ↓
                                 74 (Autodiff Integration)
                                      ↓
                                 75 (Backend Abstraction)
                                    ↓        ↓
                               76 (GPU)   77 (NPU) → 78 (NPU Opt)
```

### Key Design Decisions

1. **No Tensor keyword** — NPU/math ops work on existing `array` types
2. **No Google TPU** — NPU targets Intel/Qualcomm/Apple/AMD/Arm only
3. **Math IR is Karkain-owned** — not ONNX, not MLIR, not vendor-specific
4. **Tensor IR is Karkain-owned** — not NumPy, not PyTorch
5. **CPU is reference backend** — correctness oracle before any accelerator
6. **NPU is a backend** — not the foundation, not the language

## Guiding Principles

1. **Maturity over features** — depth, correctness, performance, verification before expansion
2. **Semantic foundations first** — value representation, ownership, lifetimes, IR before features
3. **Working > ambitious** — fix broken fundamentals before adding new capabilities
4. **Incremental verification** — every phase must compile, pass E2E, pass all tests

## Testing

Run the full test suite before committing:
```
go test ./pkg/lexer/... ./pkg/parser/... ./pkg/codegen/... ./pkg/pm/... -count=1
```

All tests must pass before committing.


