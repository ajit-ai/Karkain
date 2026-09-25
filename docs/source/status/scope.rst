=======================
Release-Candidate Scope
=======================

This page is the authoritative status matrix for the Karkain release
readiness. It classifies every surface of Karkain into exactly one of the
buckets below so an external contributor can tell at a glance what may be
relied on, what may change, what does not exist yet, and what blocks a
release. Current status reflects the **1.1.0** line (Phases 128–149 complete
in-tree).

.. contents:: Sections
   :local:
   :depth: 1

The status buckets
==================

.. list-table:: Status matrix vocabulary
   :widths: 22 78
   :header-rows: 1

   * - Bucket
     - Meaning
   * - :stable:`Stable Beta Core`
     - Regression-gated on both engines. External developers can rely on
       this surface; changes are deliberate, documented and justified.
   * - :production-candidate:`Production Candidate`
     - Implemented, testable, functionally complete for its documented
       surface, and deterministic — but tied to one engine and not yet in
       the Stable Core gates.
   * - :experimental:`Experimental`
     - Implemented and testable, but the surface may change between
       releases without notice. Every experimental item states why.
   * - :planned:`Planned`
     - Designed but not implemented. **No code exists.** Nothing here is
       promised for the release candidate.
   * - Known Limitation
     - Implemented but intentionally constrained. The constraint is
       documented and the behavior is deterministic (fails loudly, never
       silently wrong).
   * - Release Candidate Blocker
     - A defect that must be fixed before a release candidate can be
       declared. A missing roadmap feature is **not** a blocker unless it
       was promised as part of the Beta stable scope.

Stable Core
===============

The language, toolchain and library surface that an external developer can
build on today. This matches the :doc:`Beta 1 assessment </status/beta>` (the
most recent detailed capability review) and is enforced by the regression
gates.

.. list-table:: Stable Beta Core
   :widths: 26 74
   :header-rows: 1

   * - Surface
     - Scope
   * - Language
     - Variables (``let``/``var``), functions and recursion, control flow
       (``if``/``else``, ``while (cond)``, C-style ``for``, ``for-in``,
       ``break``/``continue``/``return``), arrays, maps, structs (records),
       enums with ``match``, assertions, module ``import`` with ``public``
       exports and qualified calls.
   * - Toolchain
     - ``check``, ``build``, ``run``, ``test`` (``--filter``), ``fmt``,
       ``lint``, ``prof``, ``explain``, ``clean``, ``target``, ``config``,
       ``workspace``, ``pkg`` (local/workspace only), ``lsp``.
   * - Exit-code contract
     - 0 success, 1 runtime failure, 2 usage error, 3 compile/type/sema,
       4 test failure, 6 environment/toolchain, 7 lint findings.
   * - Diagnostics
     - ``error[K...]`` codes, ``runtime error: <kind> at <file>:<line>``
       with stack traces, ``karkain-diagnostics-v1`` JSON contract.
   * - Standard library
     - ``std.string``, ``std.collections``, ``std.io``,
       ``std.encoding``, ``std.crypto``, ``std.testing``,
       ``std.numerics``, ``std.net``, ``std.http``, ``std.db``,
       ``std.generics``. Frozen under the SemVer policy.
   * - Concurrency runtime
     - ``spawn``, ``channel``, ``actor`` — work-stealing scheduler with
       full ``kcc`` parity (Phase 137).
   * - First-class functions
     - First-class ``fn`` values, closures with capture-mutation,
       higher-order functions, factory patterns, and ``error[K114]``
       escape rejection (Phases 130, 133).
   * - Generics
     - Generics v1 with explicit type arguments on functions and structs,
       monomorphization on both engines (Phase 146).
   * - Native backend
     - C-free native backend (Phases 145–149): ELF64 static binaries on
       Linux (green execution), PE32+ (Windows x86-64, execution verified),
       Mach-O structural writer.
   * - Engines
     - Go front end and self-hosted ``kcc`` produce byte-identical output
       on the gated corpus; ``kcc`` is the default engine for
       ``check/build/run/test``.

See :doc:`/reference/stable-api` for the full Beta API snapshot.

Production Candidate
====================

Implemented, testable, and functionally complete for the documented
surface, but still tied to one engine and/or not yet part of the
:stable:`Stable Beta Core` regression gates.

.. list-table:: Production Candidate
   :widths: 26 74
   :header-rows: 1

   * - Surface
     - Scope
   * - WASM target
     - ``wasm32-wasi`` Go-engine backend, ``wasmtime``-gated (Phase 123).
       WASI process entry with real exit codes, ``stderr`` runtime-error
       diagnostics and ``getArgs()`` are implemented and gated. The K108
       diagnostic contract rejects every unsupported construct
       deterministically: structs, maps, C-interop, concurrency, modules/
       imports and cross-engine ``kcc`` use.

Experimental
============

Implemented, real, and testable — but the surface is not yet guaranteed and
may change between releases.

.. list-table:: Experimental
   :widths: 26 74
   :header-rows: 1

   * - Surface
     - Scope
   * - Accelerator kernels
     - ``@target(gpu)`` WGSL compute kernels (Phase 138), compile-only
       guarantee. NPU quantization pipeline (Phase 78).
   * - Profiling diagnostics
     - ``karkain prof`` — text/json/folded reports (Phase 110).
   * - Debug tracing
     - Phase 112 debug-trace module and ``karkain dbg`` gdb backtraces (Phase 140).
   * - SIMD / vector types
     - ``@simd_*`` lane types (Phase 106) — x86 + ARM via portable helpers.
   * - Package manager
     - Local directory registry (Phase 135) with immutable versions and
       digest-verified fetch. Public registry is Planned.
   * - Incremental compilation v2
     - Per-module translation unit cache with content-keyed objects (Phase 134).
   * - Intermediate representation
     - KIR v1 text emitter and structural verifier (Phases 120–121).

Planned
=======

Designed but not implemented. **No code exists.** Nothing in this bucket is
promised for the release candidate.

.. list-table:: Planned
   :widths: 26 74
   :header-rows: 1

   * - Surface
     - Scope
   * - Pre-built binary distribution
     - The CI build matrix and tag-triggered release workflow are ready;
       archives/sha-256 will be produced on GitHub Releases for tagged
       releases. Today installation is **from source**.
   * - Public package registry
     - ``karkain pkg publish/search/login`` against an operated central
       registry. Local registry is implemented (Phase 135).
   * - Full-platform native backends
     - AArch64 native emitter, native macOS execution (Mach-O runner),
       freestanding/MCU target (Phases 150–172).
   * - Language features
     - Trait/interface system, explicit struct layout, linear/move types,
       advanced pattern matching with payload destructuring (Phases 159–180).
   * - Async / await
     - Syntax and event-loop runtime (post-2.0).
   * - Standard library expansion
     - OS modules (``std.path``, ``std.env``, ``std.time``, ``std.random``,
       ``std.json``) planned for 1.3.0.

Known Limitations
=================

Implemented but intentionally constrained. Each one fails loudly or is
documented as environment-scoped — never silently wrong.

.. list-table:: Known Limitations
   :widths: 26 74
   :header-rows: 1

   * - ``while`` condition parentheses
     - The documented form is ``while (cond)``. Unparenthesized conditions
       are accepted by both engines via the Phase 117 parser alignment but
       are not the official surface.
   * - Cross-compilation
     - Foreign triples require real cross-linkers; ``karkain build
       --target <foreign>`` fails deterministically with a toolchain error
       when none is installed. Never a silent host fallback.
   * - WASM execution
     - ``wasm32-wasi`` output requires ``wasmtime`` on PATH to run.
   * - Bootstrap memory
     - Full ``kcc`` *build* mode for the compiler's own sources can stall
       on hosts with very limited RAM; the low-memory ``check`` path passes
       everywhere. Environmental limitation, not a defect.
   * - Concurrency/profiling scope
     - Single-threaded programs only for ``prof``; concurrency programs are
       out of scope for profiling and WASM. Documented boundaries.

Release Candidate Blockers
==========================

Updated at the end of each release-candidate phase. A missing feature is
never listed here unless it was in the promised Beta stable scope.

- During Phase 118, no release-candidate blockers were found. See
  :doc:`/status/rc-checklist` for the authoritative check state.

.. seealso::

   :doc:`/status/compatibility` — what the buckets mean for compatibility
   guarantees.
   :doc:`/status/rc-checklist` — the release-candidate acceptance checklist.
   :doc:`/reference/stable-api` — the Beta API snapshot referenced above.
   :doc:`/status/beta` — the Beta 1 capability assessment.