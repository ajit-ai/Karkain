# Karkain 1.0.0 — Release Notes (draft)

The canonical public release notes page is `docs/source/release-notes.rst`
(Sphinx). This file is the concise release-facing summary for the 1.0.0
preparation (Phase 119).

## Identity

- **Public label:** Karkain 1.0.0 (Stable)
- **CLI banner:** `Karkain Compiler v1.0.0 (<os>/<arch>, Stable Build)`
- **`VERSION`:** `1.0.0`
- **Milestone:** Phase 119 — Public Repository Finalization & Stable Release
  Preparation

## What is in stable scope (1.0.0)

- Language: a complete, both-engine-validated core — variables/constants,
  functions/recursion, control flow (incl. unparenthesized-`if`/`while`
  parity), arrays, strings, structs, enums/match, type aliases, casts,
  slices, closures (codegen-deferred), modules with `public` exports, C
  interop, error handling via assertions and runtime-error diagnostics.
- Toolchain: `karkain check/build/run/test/fmt/lint/explain/clean/bench/prof/
  target/config/workspace` + LSP (`karkain lsp`) + package manager
  (`karkain new/remove/update/list/tree/fetch/pkg`), `--engine go|kcc`,
  `--target` cross-compilation (native + `wasm32-wasi`), DWARF-ready native
  linking, incremental builds, deterministic diagnostics with numeric error
  codes (`explain K001..`).
- Standard library v2: `import std.string / std.collections / std.io /
  std.encoding / std.crypto` on both engines.
- Self-hosted compiler: kcc owns lex/parse/sema/C-codegen/test on the default
  pipeline; `go build` parity is regression-gated.
- Concurrency runtime (spawn/join/channels/actors), profiling (`karkain prof`).
- Examples corpus: 49 golden files across 15 categories, both engines.

## Explicitly NOT in 1.0 stable scope

Planned (documented, honest, no fabricated API): networking, web, database,
GPU acceleration, quantum, package registry/git fetches. Experimental:
WASM target, native (non-C) linking/DWARF container embedding, SIMD vector
types, NPU dispatch, incremental caching, profiling instrumentation details.

## History and honesty

The phase-91 "1.0 ready" record was retracted; the label moved Developer
Preview (`v0.115.x`) → Beta 1 (`v0.117.x`, phases 117–118) → **1.0.0
(Stable)** at Phase 119. `docs/audit/PHASE-119-LANGUAGE-QA-FINAL-REPORT.md`
records the readiness verdict and full QA evidence.