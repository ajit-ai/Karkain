# Karkain 1.0.0 — Release Notes

The canonical public release notes page is `docs/source/release-notes.rst`
(Sphinx). This file is the concise release-facing summary for the **1.0.0
release cut (Phase 128)**.

## Identity

- **Public label:** Karkain 1.0.0 (Stable)
- **CLI banner:** `Karkain Compiler v1.0.0 (<os>/<arch>, Stable Build)`
- **`VERSION`:** `1.0.0`
- **Milestone:** Phase 128 — Release 1.0.0 Cut & Distribution
  (preceded by Phase 119 stable label, Phases 120–127 hardening)

## What ships in 1.0.0

- **Language — Stable Core (both engines, byte-identical):** variables
  (`let`/`var`), functions/recursion, control flow (`if`/`else`,
  `while (cond)`, C-style `for`, `for-in`), arrays, maps, structs (records),
  **enums + `match`** (Phase 123 parity), strings, module `import` with
  `public` exports and qualified calls, assertions, type aliases.
- **Toolchain:** `karkain check/build/run/test (--filter)/fmt/lint/explain/
  clean/bench/prof/target/config/workspace` + LSP + package manager
  (local/workspace, lockfile-backed), `--engine go|kcc`, `--target` cross
  compilation (native + `wasm32-wasi`), incremental builds, `karkain kir`
  (self-hosted KIR emitter + `--verify`), DWARF-ready native linking,
  deterministic numeric error codes (`explain K001..`).
- **Standard library:** `std.string`, `std.collections`, `std.io`,
  `std.encoding`, `std.crypto`, `std.testing`, **`std.numerics`** (Phase 126,
  40 funcs), **`std.net` / `std.http` / `std.db`** (Phase 125A, with the
  Windows Winsock link contract enforced on every generated-C linker).
- **Self-hosted compiler:** the default engine (`kcc`) owns lex/parse/sema/
  C-codegen/test; stage-2 == stage-3 bitwise bootstrap identity verified;
  **KIR v1 structural verification** runs on the default check path (Phase 121)
  and **flat project assembly** is owned inside the compiler (Phase 122).
- **Concurrency runtime** (`spawn`/`channel`/`actor`), profiling
  (`karkain prof`), SIMD lane types, NPU dispatch with CPU oracle — shipped,
  **Go-engine only** (kcc parity is the GA-2 roadmap). Documented as
  Experimental surface.
- **Compute targets (Phase 124, experimental):** `cpu`, `simd`,
  `wasm32-wasi`, `gpu-experimental`, `npu-experimental`, `quantum-experimental`
  exposed through the target catalog.
- **Examples corpus:** 59 golden `.kark` files across 15 categories, pinned
  byte-identical on both engines in the Phase 114 gate (Go + kcc).
- **Bootstrap memory guard (Phase 127):** stages 2/3 abort cleanly with
  `error[K127]` on low-RAM hosts instead of the documented SEGFAULT class.

## Explicitly NOT in 1.0 stable scope (honest)

- **Closures / `fn` values** — parser/AST work is in the tree; codegen is
  still broken on both engines. Documented Phase 101 boundary.
- **GPU / NPU / quantum kernel language surface** — backends/abstractions
  exist; the authoring surface is roadmap (GA-3).
- **Advanced (public) package registry** — local/lockfile package manager
  ships; publishing to a hosted registry does not exist as a service.
- **Concurrency / profiling / SIMD** ship but are **Go-engine only**; kcc
  parity is roadmap (GA-2).
- **Docker image** — documented Planned; not published.

## History and honesty

The phase-91 "1.0 ready" record was retracted; the label progressed Developer
Preview (`v0.115.x`) → Beta 1 (`v0.117.x`, phases 117–118) → **1.0.0
(Stable)** at Phase 119, with public release cut described by Phase 128.
Readiness verdicts: `docs/audit/PHASE-119-LANGUAGE-QA-FINAL-REPORT.md`
(stable label), `docs/audit/PHASE-127-BOOTSTRAP-MEMORY-GUARD.md` (memory
guard). End-to-end plan: `docs/audit/GENERAL-AVAILABILITY-ROADMAP.md`.