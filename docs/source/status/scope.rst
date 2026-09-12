=======================
Release-Candidate Scope
=======================

This page is the authoritative status matrix for the Karkain 1.0.0
release-readiness work (Phase 119). It classifies every surface of Karkain
into exactly one of five buckets so an external contributor can tell at a
glance what may be relied on, what may change, what does not exist yet, and
what blocks a release candidate.

.. contents:: Sections
   :local:
   :depth: 1

The five buckets
================

.. list-table:: Status matrix vocabulary
   :widths: 22 78
   :header-rows: 1

   * - Bucket
     - Meaning
   * - :stable:`Stable Beta Core`
     - Regression-gated on both engines. External developers can rely on
       this surface; changes are deliberate, documented and justified.
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
       ``std.encoding``, ``std.crypto``, ``std.testing``.
   * - Engines
     - Go front end and self-hosted ``kcc`` produce byte-identical output
       on the gated corpus; ``kcc`` is the default engine for
       ``check/build/run/test``.

See :doc:`/reference/stable-api` for the full Beta API snapshot.

Experimental
============

Implemented, real, and testable — but the surface is not yet guaranteed and
may change between releases.

.. list-table:: Experimental
   :widths: 26 74
   :header-rows: 1

   * - Concurrency runtime
     - ``spawn``/``channel``/``actor`` — Go-engine only, ``kcc`` parity is a
       documented post-Beta boundary.
   * - Profiling diagnostics
     - ``karkain prof`` — Go-engine only.
   * - Debug tracing
     - Phase 112 debug-trace module — Go-engine only.
   * - WASM target
     - ``wasm32-wasi`` Go-engine backend, ``wasmtime``-gated.
   * - SIMD / vector types
     - ``@simd_*`` lane types — Go-engine only, x86 + ARM via portable
       helpers.
   * - Package manager
     - Local/workspace resolution and lockfiles are real; there is no
       public registry. Registry subcommands fail loudly with a clear
       "not available" diagnostic.
   * - Incremental compilation
     - Content-addressed whole-assembly cache (``--incremental``).
   * - Native object/linker pipeline
     - ``--target native-link`` Phase 84 object/linker pipeline.

Planned
=======

Designed but not implemented. **No code exists.** Nothing in this bucket is
promised for the release candidate.

.. list-table:: Planned
   :widths: 26 74
   :header-rows: 1

   * - Pre-built binary distribution
     - The CI build matrix and tag-triggered release workflow are ready;
       archives/sha-256 will be produced on GitHub Releases for the first
       tagged release. Today installation is **from source**.
   * - GPU / NPU / quantum kernels
     - Backend abstractions exist (`pkg/backend`, `pkg/npu`); the language
       surface is not released. Honest plan only.
   * - Networking, databases, web, AI/ML frameworks
     - ``std.net``/``std.ai``/etc. do **not** exist. Category examples are
       computational demonstrations, never a framework claim.
   * - Advanced package registry
     - ``karkain pkg publish/search/login`` do not exist as services.
   * - Closures / ``fn`` values
     - Design exists; codegen is broken on both engines (documented Phase
       101 boundary). No gate pretends otherwise.

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