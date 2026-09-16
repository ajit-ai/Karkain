Developer Preview
=================

.. note::

   This page documents the Developer Preview capability summary (Phases
   113–116). It is superseded by **Karkain 1.0.0 (Stable)** — see
   :doc:`/status/index` for the current public status and the stable core.

Karkain is a **working compiler and language with tested capabilities**.
This page is an honest summary of what works today and what does not.

.. contents:: Sections
   :local:
   :depth: 1

What works today
----------------

* **Language core** — variables (``let``/``var``/``const``), functions,
  control flow (``while``/``for-in``/C-style ``for``), structs (records),
  enums, modules, constants. Tested on both engines; byte-identical
  behavior.
* **Standard library** — 6 importable ``std.*`` modules: ``string``,
  ``collections``, ``io``, ``encoding``, ``crypto`` and ``testing``.
  Tested on both engines with NIST/RFC-verified digest and codec vectors.
* **CLI** — ``build``, ``run``, ``test``, ``check``, ``fmt``, ``lint``,
  ``debug``, ``prof``, ``target``, ``pkg``, ``lsp`` — all real commands
  wired to the real pipeline.
* **Cross-compilation** — explicit ``--target <triple>`` with same-machine
  and foreign-machine handling, deterministic failure when no cross-linker
  exists.
* **Self-hosted compiler** — ``kcc`` is the default engine for
  ``check/build/run/test``; bootstrap identity (stage-2 == stage-3) is
  proven bitwise identical.
* **WASM target** — ``wasm32-wasi`` backend emits real WASM binaries
  runnable under ``wasmtime`` (Go engine only). WASI exit codes, stderr
  diagnostics and ``getArgs()`` are implemented (production candidate).
* **Concurrency runtime** — channels, actors, task spawning via the Go
  engine; kcc parity is deferred.
* **DWARF debug info** — debug sections are emitted and self-verified by a
  parser; integrated into the gcc C-transpile path.
* **Incremental compilation** — content-addressed whole-assembly cache.
* **Developer tools** — formatter, linter, VS Code extension, LSP server.

What does not work
------------------

* **Networking** — no TCP/UDP sockets, no HTTP client/server.
* **Databases** — no drivers, no query language, no structured-data format.
* **Web framework** — no HTTP server or routing; no JSON parsing.
* **AI/ML language surface** — no tensors, no autodiff, no model format
  reachable from ``.kark``.
* **GPU/NPU/quantum kernel language surface** — backend abstractions exist
  internally but are not exposed as language types, keywords or builtins.
* **Advanced package registry** — no centralized/authenticated module
  fetching; only local resolution and lockfiles.
* **Network/block-comment parity** — nested ``/* */`` is Go-engine only;
  kcc does not support nesting.

Honest assessment
-----------------

Karkain is **not a production-ready system**. It is a real, working
compiler and language with a tested self-hosted pipeline, a meaningful
standard library and a concrete cross-compilation story. The toolchain
delivers value on the features it documents as ``:implemented:``, and it
labels everything else honestly so you know exactly what you are getting.

The milestone that produced this assessment was **Phase 117 — Beta 1
Readiness & Hardening** (now superseded): the project's public status moved
to **Karkain 1.0.0 (Stable)** in Phase 119 — see :doc:`/release-notes` for
the current release identity.
Phase 117 defined a stable core,
Go/kcc parity hardening, parse-error and semantic build/run gating, numeric
error-code documentation, stdlib edge-case gates, a fresh-checkout validation
script and an honest Beta scorecard. Beyond it, the advanced package registry
and production readiness will expand the working surface and harden the
guarantees around
it.

.. seealso::

   :doc:`/development/roadmap` — the full development plan.
   :doc:`/status/index` — the status vocabulary used on this site.
   :doc:`/examples/index` — what can be learned from the examples today.