=============
Release Notes
=============

This page tracks the honest development state of Karkain. The toolchain is a
**Karkain 1.1.0 (Stable)** build: functional, tested, and self-hosted. The
v1.1.0 identity is the public label from Phase 142 (1.0.x continues as the
LTS line); the 1.0.0 story is preserved below as history.

Versioning: the CLI reports ``Karkain Compiler v1.1.0`` via
``karkain --version``; the public label progressed Developer Preview (≤115,
v0.115.x) → Beta 1 (117–118, v0.117.x) → **1.0.0 (Stable)** (Phase 119) →
**1.1.0 (Stable)** (Phase 142, minor bump: strictly additive surface).

v1.1.0 — Stable Build (current)
================================

.. list-table::
   :widths: 30 70
   :header-rows: 0

   * - Status
     - Karkain 1.1.0 (Stable) — the project's public status; 1.0.x is the
       LTS line (security fixes). Full story:
       ``docs/release/KARKAIN-1.1-RELEASE-NOTES.md``
   * - Since 1.0.0
     - Stdlib freeze + SemVer policy (132); first-class ``fn`` values (133);
       incremental v2 (134); local registry (135); LSP v2 (136);
       concurrency parity, 08-concurrency Stable (137); ``@target(gpu)``
       WGSL kernels (138); new triples + WASM structs + ``karkain wit``
       (139); ``karkain dbg`` + VS Code debug surface (140); SimplifyCFG
       optimizer pass + profile cell counts + measured RSS table (141);
       ``std.net``/``std.http``/``std.db`` (125A); ``std.numerics`` (126)
   * - Compatibility
     - Every 1.0.0 program builds and runs identically (conformance 64/64,
       Phase 114 corpus byte-identical Go↔kcc); the one behavior fix is
       K114 escape rejection (previously accepted-and-miscompiled)
   * - Example corpus
     - **61 files** across 15 categories (59 Runnable incl. test-mode,
       2 Stable, 1 Planned dir), pinned goldens byte-identical both engines

v1.0.0 — Stable Build (historical)
=====================================

.. list-table::
   :widths: 30 70
   :header-rows: 0

   * - Status
     - Karkain 1.1.0 (Stable) — the project's public status (see
       :doc:`/status/compatibility` for the guarantees attached to it)
   * - Language core
     - Stable core defined and regression-gated on both engines: variables,
       functions, recursion, control flow, arrays, maps, structs, enums,
       match, strings, modules with ``public`` exports; ``while (cond)``
       syntax aligned between the Go and kcc parsers
   * - Diagnostics
     - Recoverable parse errors are no longer swallowed by ``build``/``run``:
       ``let = 42`` and friends exit 3 with a rendered ``error[K001]``;
       undefined identifiers exit 3 on both engines during build/run
       (``error[K002]`` Go / ``error[K102]`` kcc) — same contract as ``check``
   * - Error codes
     - Numeric codes (K001-K008, K100, K101–K115) documented by
       ``karkain explain`` and ``explain --list``
   * - Standard library
     - Public modules hardened: ``std.string``, ``std.collections``,
       ``std.io``, ``std.encoding``, ``std.crypto``, ``std.testing`` with new
       edge-case parity tests (empty/invalid/duplicate inputs), plus
       ``std.numerics`` (40 funcs, Phase 126) and ``std.net`` / ``std.http`` /
       ``std.db`` (Phase 125A) with the Winsock link contract enforced on
       every generated-C linker
   * - Example corpus
     - Phase 114/116 corpus (**60 pinned goldens**) verified under the new
        semantic gating and byte-identical on both engines (Go + kcc)
   * - Self-hosted compiler
     - kcc owns the default check/build/run/test path; KIR v1 structural
        verification runs on the default check (Phase 121); flat project
        assembly is owned inside the compiler (Phase 122); stage-2 == stage-3
        bitwise bootstrap identity
   * - Bootstrap memory guard
     - ``error[K127]`` clean abort (stages 2/3) on low-RAM hosts replaces the
        documented SEGFAULT class (Phase 127)
   * - Compute targets
     - Phase 124 experimental catalog: ``cpu``, ``simd``, ``wasm32-wasi``,
        ``gpu-experimental``, ``npu-experimental``, ``quantum-experimental``
   * - Toolchain
     - Exit-code contract verified end-to-end; ``debug`` and ``prof`` opt-in
        diagnostics verified in the Beta gate; fresh-checkout script
        ``scripts/beta-fresh-checkout.ps1``
   * - Developer readiness
     - Beta documentation (``status/beta.rst``), Beta readiness scorecard,
        Phase 117 CI gate, version identity ``v1.0.0 (Stable Build)``
   * - Known gaps (honest)
     - Closures/``fn`` codegen remains broken on both engines (documented);
        GPU/NPU/quantum kernel language surface and a public package registry
        are Planned; concurrency, profiling, debug tracing, WASM and SIMD
        remain Go-engine only; Docker image is Planned

Entry requirements for every release note entry

* The CLI version reported by ``karkain --version``.
* What is newly Implemented (gated) vs experimental vs planned.
* Any behavior changes that might break existing programs.

Previous highlighted milestones

.. list-table::
   :widths: 12 28 60
   :header-rows: 1

   * - Phase
     - Version era
     - Summary
   * - 117
     - v0.117.0
     - Beta 1 readiness (historical): Go/kcc ``while`` and semantic-gate
       parity, parse-error hardening, numeric ``explain`` codes, stdlib edge
       tests, Beta docs + gate + fresh-checkout script
   * - 116
     - v0.115.0
     - Corpus normalization + real-world programming examples (50 files,
       dash-named categories, stack/queue/knapsack/token-stats) + metadata gate
   * - 115
     - v0.115.0
     - Developer Preview readiness: license/contributing/CoC, honest docs,
       version identity, fresh-checkout gate
   * - 114
     - pre-0.115
     - Example corpus (46 files, 15 categories) + parity gate
   * - 113
     - pre-0.115
     - Developer Preview documentation + status model established
   * - 111
     - pre-0.115
     - Cross-compilation via explicit target triples
   * - 107–110
     - pre-0.115
     - Concurrency runtime, WASM target, standard library v2, profiling
   * - 95–99
     - pre-0.115
     - Self-hosted compiler becomes the default engine; bootstrap identity
   * - 91
     - pre-0.115
     - (Historical note: this phase claimed a "1.0 release-ready" decision;
       that claim was retracted — the label moved Developer Preview → Beta 1
       → 1.0.0 at Phase 119.)

.. seealso::

   :doc:`/development/roadmap` — forward-looking plan.
   :doc:`/status/beta` — what Beta 1 promises and the honest limitation list.