Roadmap
=======

The canonical development roadmap lives in ``ROADMAP.md`` at the repository
root. This page summarizes the current state for documentation purposes.

.. contents:: Sections
   :local:
   :depth: 1

Completed phases (50–119)
=========================

Phases 50–119 are complete and shipped as **Karkain 1.0.0 (Stable)**. A
concise summary:

* **50–106** — language core, semantic model (borrow checker, escape
  analysis), SSA optimization pipeline, HIR, stdlib foundation, self-hosted
  compiler foundation, conformance/examples, diagnostic spans, native
  codegen/linker/DWARF, incremental compilation, SIMD, package manager,
  debugger, profile, error recovery.
* **107** — concurrency runtime (work-stealing scheduler, channels, actors).
* **108** — WASM target (``wasm32-wasi`` backend, wasmtime-gated).
* **109** — standard library v2 (importable ``std.string / collections /
  io / encoding / crypto / testing`` on both engines).
* **110** — profiling and diagnostics (``karkain prof``, text/json/folded
  reports).
* **111** — cross-compilation (``--target <triple>``, Karkain-owned target
  model, same-machine/foreign machine detection).
* **112** — programming language foundation additions (``const`` keyword,
  ``float()`` builtin, ``karkain debug`` trace, ``std.testing`` module,
  nested block comments).
* **113** — official documentation set (this site): 15-category examples
  framework, status model and contributor documentation.
* **114** — example corpus (50 ``.kark`` programs across 15 categories,
  pinned byte-identical on both engines by ``pkg/cli/phase114_examples_test.go``).
* **115** — developer preview readiness: LICENSE/CONTRIBUTING/CODE_OF_CONDUCT,
  honest README + docs, unified v0.115.0 identity. Superseded by the 1.0.0
  label.
* **116** — developer examples corpus and real-world programming corpus.
* **117** — Beta 1 readiness and hardening (``while`` parity, semantic
  gating, exit-code contract, kcc test runner, v0.117.0 identity).
* **118** — Beta 1 external validation and release-candidate readiness
  (RC READY verdict; release hygiene, issue templates, install scripts,
  rc-journey gate).
* **119** — language QA and public repository finalization: **Karkain
  1.0.0 (Stable)** label, v1.0.0 identity, superseded-material cleanup,
  QA battery, release documentation.

Key cross-cutting milestones
============================

* **Phases 71–78**: Math/Tensor IR chain and the CPU/GPU/NPU backend
  abstraction — internal packages with no language surface yet released.
* **Phase 79**: compiler integrity audit and self-hosting readiness — audit
  of the full pipeline, IR layers and backend parity.
* **Phase 95–99**: the self-hosted ``kcc`` engine becomes the default
  compiler for ``check/build/run/test``, with bootstrap identity.

Current status
==============

The project is **Karkain 1.0.0 (Stable)** (label adopted at Phase 119). The
versioned language specification lives in ``SPEC.md``; the authoritative
status matrix for the released scope is :doc:`/status/scope`, and the
compatibility guarantees are documented at :doc:`/status/compatibility`.
Release notes are tracked at :doc:`/release-notes`.

What ships in 1.0.0 today
=========================

* Importable standard library modules (``std.string``, ``std.collections``,
  ``std.io``, ``std.encoding``, ``std.crypto``, ``std.testing``) on both
  engines.
* CLI surface: ``check``, ``build``, ``run``, ``test`` (``--filter``),
  ``transpile``, ``fmt``, ``lint``, ``debug``, ``prof``, ``target``,
  ``pkg``, ``workspace``, ``clean``, ``explain``, ``bench``, ``lsp``.
* Cross-compilation via ``--target <triple>`` with deterministic failure
  when a cross-linker is missing (never a silent host fallback).
* Runtime error model with source locations and stack traces.
* Incremental compilation cache, DWARF debug sections, profiling.
* Experimental surfaces (may change): concurrency runtime, WASM target,
  SIMD/vector types, profiling/trace — Go engine only, kcc parity deferred.

Future
======

Beyond 1.0.0:

* **Advanced package registry** — centralized, authenticated module
  fetching (current resolver is local + lockfile only).
* **kcc parity boundaries** — concurrency, profiling, tracing, SIMD, WASM
  and native-linking on the self-hosted engine.
* **Planned language surfaces** — networking, databases, web, GPU/NPU and
  quantum kernels ``from .kark``, and the advanced package registry.

The full road-map with versioned phase descriptions lives in
``ROADMAP.md`` at the repository root — that file is authoritative.

.. seealso::

   :doc:`/development/developer-preview` — the historical development-preview
   assessment and how the label changed.
   :doc:`/status/index` — the status vocabulary applied throughout this site.