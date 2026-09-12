=============
Release Notes
=============

This page tracks the honest development state of Karkain. The toolchain is a
**Karkain 1.0.0 (Stable)** build: functional, tested, and self-hosted. The
v1.0.0 identity is the public label from Phase 119; the release-readiness
verdict and full QA evidence live in ``docs/audit/PHASE-119-LANGUAGE-QA-FINAL-REPORT.md``.

Versioning: the CLI reports ``Karkain Compiler v1.0.0`` via
``karkain --version``; the public label progressed Developer Preview (≤115,
v0.115.x) → Beta 1 (117–118, v0.117.x) → **1.0.0 (Stable)** from Phase 119.

v1.0.0 — Stable Build (current)
=================================

.. list-table::
   :widths: 30 70
   :header-rows: 0

   * - Status
     - Karkain 1.0.0 (Stable) — the project's public status (see
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
     - Numeric codes (K001-K008, K100, K101-K113) documented by
       ``karkain explain`` and ``explain --list``
   * - Standard library
     - Public modules hardened: ``std.string``, ``std.collections``,
       ``std.io``, ``std.encoding``, ``std.crypto``, ``std.testing`` with new
       edge-case parity tests (empty/invalid/duplicate inputs)
   * - Example corpus
     - Phase 114/116 corpus (49 pinned goldens) verified under the new
       semantic gating — no golden changed
   * - Toolchain
     - Exit-code contract verified end-to-end; ``debug`` and ``prof`` opt-in
       diagnostics verified in the Beta gate; fresh-checkout script
       ``scripts/beta-fresh-checkout.ps1``
   * - Developer readiness
     - Beta documentation (``status/beta.rst``), Beta readiness scorecard,
       Phase 117 CI gate, version identity ``v1.0.0 (Stable Build)``
   * - Known gaps (honest)
     - Networking, databases, web, quantum, GPU/NPU kernel language surface,
       advanced package registry — Planned / Not Yet Implemented; concurrency,
       profiling, debug tracing, WASM and SIMD remain Go-engine only

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