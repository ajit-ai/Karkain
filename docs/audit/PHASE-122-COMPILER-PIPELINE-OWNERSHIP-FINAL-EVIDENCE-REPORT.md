# PHASE 122 FINAL EVIDENCE REPORT — COMPILER PIPELINE OWNERSHIP

Status: **COMPLETE**
Date: 2026-09-14
Baseline: `docs/audit/PHASE-122-BASELINE-COMPILER-PIPELINE-OWNERSHIP.md`
Gate: `pkg/cli/phase122_pipeline_ownership_test.go`

Phase 122 moves FLAT project source assembly into the self-hosted compiler.
`src/compiler/main.kark` now owns input composition for the common flat case
(no `karkain.toml`, every import is `std.*` or resolves to a sibling
file/directory), and the production CLI path (default kcc engine
`check/build/run/kir/verifykir`) routes those projects through it. Go keeps
manifest / workspace / package-manager assemblies, the module-graph
diagnostics layer, NPU staging, sandbox file mirroring and orchestration
(documented BRIDGE dependency; no Go text-composition step was deleted on that
path).

---

## 1. WHAT CHANGED

### 1.1 `src/compiler/main.kark` — the Karkain-owned assembler

`assembleProject(path)` (line 588) replaced the previously dormant
`loadSourceWithSiblings` as the compiler's own input composer. Every driver
entry now compiles through it: `checkFile`/`kirFile`/`verifyFile`/
`buildFile`/`runFile` (lines 116/170/196/224/253). Behavior:

- **No module imports** → joins the root's same-directory sibling `.kark`
  files in sorted order (excluding the root and any sibling declaring its own
  `func main(`), root last. This is the `src/compiler` whole-tree assembly
  (10 files) and the `examples/module_system` sibling case.
- **Module imports** → resolves each distinct `import <name>` in declaration
  order: `std.*` names against the effective standard-library root
  (`resolveStdlibModule`), bare names against a sibling `<name>.kark` file or
  `<name>/` directory (`resolveUserModule`). Unresolvable modules print
  `error[K122]: module '<name>' not found (assembling <path>)` and abort.
- Module import declaration lines are stripped by `stripModuleImports`
  (preserving blank lines → exact source line numbering; C import blocks
  `import "C"` untouched) — mirroring the Go assembler's dotted-import strip.
- Helpers are written in Karkain using only builtins: `sourceLines`,
  `stripLineEnd`, `importNameOf` (CRLF/LF-robust), `isImportLine`,
  `moduleImportNames`, `stripModuleImports`, `collectKarkFiles` (`sortStrings`),
  `moduleDirSource` (dir-of-files then single-file fallback), `siblingContent`,
  `resolveStdlibModule`, `resolveUserModule`.

### 1.2 The marker-gated standard-library root (defect fix)

`findStdlibRoot(rootDir)` first checks the sandbox staging area
(`<rootDir>/lib`), then walks up ≤14 ancestors probing `<candidate>/stdlib`.
**A candidate is adopted only if it carries the canonical `std.string` module**
(`<cand>/string/string.kark` present via `moduleDirSource`). Without the gate,
the walk-up from `examples/stdlib_v2` picked the leftover demo tree
`examples/stdlib/collections` (Phase 109 demo with its own `func main`) and the
project silently compiled against the fake module set. This was the root-cause
failure the gate subtest `StdlibShadowMarker` locks in.

### 1.3 `pkg/cli/kcc_engine.go` — flat-path staging (Go BRIDGE)

- `flatAssemblyEligible(file)` — flat when the root dir has no `karkain.toml`
  AND every import is `std.*` / a present sibling file or directory, AND
  `findKarkainStdlib(dir)` locates a marker-gated stdlib.
- `kccStageInput(file, sandbox)` — stages flat projects by mirroring root +
  siblings into the sandbox plus a `lib/` copy of the stdlib tree (real
  `stdlib/string/...` mirrored), then returns the mirrored root path for kcc
  to assemble alone. Non-flat projects keep the legacy `kccAssembleSource`
  pre-assembled single file.
- `KCCCheckCommand`, `KCCBuildCommand`, `KCCRunCommand`, and the KIR routes in
  `pkg/cli/kir.go` all stage through `kccStageInput`; the kcc test runner
  (`KCCTestCommand`) and NPU target preflight (`kccTargetPreflight`) remain
  unchanged.

## 2. EVIDENCE (gate, `TestPhase122_PipelineOwnership`, 7 subtests — PASS,
191.02s)

| Subtest | What it proves |
|---|---|
| `CompilerSelfCheck` (94.68s) | `karkain check src/compiler/main.kark` on the DEFAULT kcc path: the compiler assembles its own 10-file tree via `assembleProject` and passes through the Phase 121 checkFile KIR hook → `[ok]`. |
| `KIRContinuity` (74.44s) | `karkain kir --verify src/compiler/kir.kark` → exactly 6399 text lines and 6399 verified — the whole-tree KIR invariant survives the pipeline change byte-for-byte. |
| `ModuleSystem` (4.18s) | `examples/module_system` (bare `import math` sibling): check `[ok]`, run prints `42`/`9`, KIR shows `twice`+`square` before a single `main`, byte-deterministic across runs. |
| `StdlibV2` (5.63s) | `examples/stdlib_v2` (four `std.*` imports): check `[ok]`, run output byte-identical to the Phase 109 Go-engine golden (11 lines incl. `sha256("karkain")` digest) on the DEFAULT kcc engine, KIR contains the real `collections` module + one `main`. |
| `StdlibShadowMarker` (4.45s) | Direct kcc and CLI on a fixture whose local `stdlib/` holds only a fake `collections` (own `main`, `array_max→999`): the marker gate rejects it, resolves the real stdlib, program prints `7`. Regression for the original defect. |
| `MissingModuleRejected` (0.02s) | `import std.nope` → CLI exits non-zero naming `nope`/`not found`, never `[ok]`. Direct kcc prints `error[K122]`. |
| `WiringPresence` (0.17s) | Guards the wiring itself: the 6 Karkain assembler helpers/`error[K122]` exist in `main.kark`; `kccStageInput`/`flatAssemblyEligible`/`findKarkainStdlib`/`kccMirrorFlat` exist in `kcc_engine.go`; `kir.go` stages through `kccStageInput`. |

Supplementary E2E proof (manual, both engines):

- `karkain run examples/stdlib_v2/main.kark --engine go` → identical golden
  stdout to the default kcc run (cross-engine parity).
- Direct `kcc` (no Go CLI) `check`/`run`/`kir` of `stdlib_v2` and
  `module_system` succeed — the compiler composes its own input with zero Go
  front-end assistance for flat projects.
- `karkain kir examples/stdlib_v2/main.kark` → 440 lines, all four modules
  assembled, exactly one `main`.

## 3. REGRESSION BATTERY

All gates PASS (individually): Phase 122 (191.0s), 121 (175.0s, incl.
CompilerSelfVerify 166.96s), 120 (82.0s), 119/118/117/116/115,
`TestPhase120_GaEnvelope`, 114 corpus incl. GoEngine+KCCParity goldens
(386.8s), conformance 59/59 + probes 11/11 (243.0s), 111, 110, 105/106/107,
103, 102 (foundation goldens both engines), 100/101, 99 (self-hosted
parser/type-checker), 98, 97, 96, 95, and the non-phase CLI unit tests
(389.6s). All non-CLI package suites green (codegen 27.1s, sema/parser/lexer/
pm/target, ir/hir/ssa, runtime incl. concurrency/freestanding, backend incl.
parity, npu vendors, wasm, compiler, source/module/diagnostics, lsp). `go vet
./...` and `go build ./...` clean.

### Pre-existing failures (NOT regressions — reproduced identically on a
pristine worktree at `477f448`)

1. `TestPhase88_SelfHostedBuildTestPrograms`, `TestPhase89_/
2. TestPhase90_NativeExecutableGeneration` — legacy Phase 88–90 expectation that
   `karkain build <file> --target c23` writes `.c` beside the source. The
   kcc-default era (Phase 97) moved non-compile-only builds to sandbox staging
   (`.c23` surfaced beside source only under `--compile-only`); these tests pass
   only under `KARKAIN_ENGINE=go`. Proven pre-existing on pristine HEAD.
3. `TestBootstrap_Stage2SelfHosting`, `TestBootstrap_BitwiseIdentity` — Stage 2
   SEGFAULT (exit `0xc0000005`) transpiling the compiler with itself: the
   documented ~4GB-host kcc-build OOM class, re-proven on pristine HEAD
   (Stage 1 GO-engine transpile succeeds — compiler built fine, checksums
   differ only because Phase 122 adds the assembler).

None of these touch Phase 122 code paths; none were weakened or modified.

## 4. BOUNDARIES (documented)

- Flat eligibility is the single-level resolution boundary: manifest /
  workspace / PM-dependency projects and transitive (non-local) module
  resolution keep the Go module-aware assembly (`kccAssembleSource`/
  `resolveSourcesRun`) — the documented next bottleneck, unchanged.
- `kccTargetPreflight` (NPU) remains Go-side on both paths.
- `kcc` itself always exits 0; missing-module rejection on the CLI path is
  pre-empted at the Go legacy layer (exit 1, names the module), K122 fires on
  direct kcc. `KCCCheckCommand` maps any `[ok]`-less output to `ExitCompile(3)`.
- The stdlib demo tree `examples/stdlib/` is left intact (documented sample
  corpus); the marker gate is why it can never shadow the real library.

## 5. VERDICT

Phase 122 **COMPLETE**: the self-hosted compiler now owns input composition for
flat projects on the real CLI path — assigning its own `src/compiler` tree and
import-ful stdlib/module projects — with E2E executable proof, cross-engine
parity, the 6399-line KIR invariant preserved, zero new regressions, and three
pre-existing legacy/environmental failures re-proven at pristine HEAD.