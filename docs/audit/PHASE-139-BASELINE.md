# Phase 139 BASELINE — Cross-Compilation Expansion + WASM GC

Date: 2026-09-22. Target: GA-3 second step (after 138).

## 1. Triple model (pkg/target, Phase 111)

- Accepted today: arches `x86_64, aarch64, wasm32`; OSes `windows, linux, wasi`;
  envs `gnu, msvc, musl(recognized-unsupported)`.
- `aarch64-windows` **already parses** (ArchAArch64 + OSWindows, default GNU env).
  Work = cross-linker search/error path + gate, not parsing.
- `riscv64-linux` needs a new `ArchRiscv64` (parse accepts; no backend builds it —
  deterministic `ToolchainError`, never silent fallback).
- `x86-64-macos` needs a new `OSMacOS` ("macos"; "darwin" alias). Canonical
  short form `x86_64-macos`. No branch exists in `detectCompilerForTarget`
  (falls into the generic "no C toolchain rules" error) — needs a real macOS
  cross branch (clang `--target=x86_64-apple-macosx` or a listing error).
- `MingwTriple(tg)` (targets.go) must cover aarch64-windows
  (`aarch64-w64-mingw32`); riscv64 Linux names compose generically
  (`riscv64-linux-gnu-gcc`) in `detectLinuxCrossCompiler`.
- Same-machine rule is arch+OS (`SameMachine`); env is toolchain flavor.
  `karkain run --target <foreign>` stays refused (build-only hint, Phase 111).

## 2. C-driver selection (pkg/codegen/cross_target.go)

- Host probing unchanged (CC → gcc → clang → cl). Cross: triple-prefixed
  GNU cross-gcc → clang `--target`, else `ToolchainError` (exit 6) listing
  exactly what was searched. WASI routes to `detectWasiCompiler`.
- New work: `detectMacOSCrossCompiler`; extend windows/linux branches only
  where arch-specific names are needed (MingwTriple).

## 3. WASM backend (pkg/wasm, Phases 108/123)

- Handwritten WASM binary v1 emitter; 24-function WASI runtime; K108-gated
  feature set (floats/maps/slices/C-interop/concurrency rejected).
- wasmtime 48.0.1 present on the dev host — E2E verifiable locally.
- Slice B scope (honest): GC **types** (struct/array field ops over the
  existing boxed-cell model where the MVP maps cleanly; externref/component
  boundaries where it does not), component-model envelope for multi-module
  programs, `wit` bindgen MVP (WIT → Karkain import stubs, one direction).
  No GC-ISA (struct.new/array.new) unless it falls out of the cell model —
  MVP honesty over proposal completeness.

## 4. CI matrix (ci.yml)

- Release matrix already builds darwin amd64/arm64 + 13 archives. New: a
  presence-matrix job reporting per-triple cross-linker availability
  (informational, never red) + the deterministic-error gate (always red on
  silent fallback). `wasmtime` install step already exists for the WASM leg.

## 5. Slice plan

- **139A (triples)**: ArchRiscv64 + OSMacOS (+darwin alias), canonical
  strings, MingwTriple aarch64, macOS cross branch, `karkain target` listing,
  extended `triple_test.go` + CLI error-path tests. Verifiable on this host
  (all three new triples resolve to deterministic ToolchainErrors here).
- **139B (WASM GC)**: GC-type surface + component envelope + wit MVP,
  extended `pkg/wasm` tests + wasmtime E2E where the MVP executes.
- **139C (gates)**: `phase139` CLI gate, CI matrix job, re-green of
  phase111/phase108 gates, full regression, audit report, develop→main→push.
