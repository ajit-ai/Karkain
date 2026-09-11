Roadmap
=======

The canonical development roadmap lives in ``ROADMAP.md`` at the repository
root. This page summarizes the current state for documentation purposes.

.. contents:: Sections
   :local:
   :depth: 1

Completed phases (50–112)
=========================

Phases 50–112 are complete. A concise summary:

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

Key cross-cutting milestones
============================

* **Phases 71–78**: Math/Tensor IR chain and the CPU/GPU/NPU backend
  abstraction — internal packages with no language surface yet released.
* **Phase 79**: compiler integrity audit and self-hosting readiness — audit
  of the full pipeline, IR layers and backend parity.
* **Phase 95–99**: the self-hosted ``kcc`` engine becomes the default
  compiler for ``check/build/run/test``, with bootstrap identity.

Current phase (Phase 113)
=========================

Phase 113 — **official documentation** (this documentation set). The 15-
category examples framework, status model and contributor documentation are
the Phase 113 deliverables.

Phase 114
=========

Phase 114 — **example corpus validation**. The remaining example categories
(networking, databases, web, AI/ML, quantum, scientific-computing, finance,
security) will be populated with validated ``.kark`` programs as their
underlying capabilities are implemented. Every program added must be tested
through the real CLI with byte-identical output pinned by a gate test.

Future
======

Beyond Phase 114:

* **Advanced package registry** — centralized, authenticated module
  fetching (current resolver is local + lockfile only).
* **Production readiness** — formal language spec freeze, backwards
  compatibility guarantees, stability commitments.
* **kcc parity boundaries** — concurrency, profiling, tracing, SIMD, WASM
  and native-linking on the self-hosted engine.

The full road-map with versioned phase descriptions lives in
``ROADMAP.md`` at the repository root — that file is authoritative.

.. seealso::

   :doc:`/development/developer-preview` — what works today, honestly.
   :doc:`/status/index` — the status vocabulary applied throughout this site.