=============
Release Notes
=============

This page tracks the honest development state of Karkain. The toolchain is a
**Developer Preview Build**: functional, tested, and self-hosted — but not a
production-ready 1.0 release. There is intentionally no ``v1.0.0`` claim.

Versioning: the CLI reports ``Karkain Compiler v0.<phase>.<minor>`` via
``karkain --version``; the ``0`` major documents the Developer Preview
status until a production release is declared.

v0.115.0 — Developer Preview Build (current)
============================================

.. list-table::
   :widths: 30 70
   :header-rows: 0

   * - Status
     - Developer Preview — the project's public status is
       **🚀 Karkain Developer Preview**
   * - Language core
     - Implemented and tested on both engines (Go front end and self-hosted
       ``kcc``): variables, functions, control flow, structs, enums, match,
       arrays, strings, maps, modules
   * - Standard library
     - Implemented: ``std.string``, ``std.collections``, ``std.io``,
       ``std.encoding``, ``std.crypto``, ``std.testing``
   * - Example corpus
     - Phase 114/116: 50 files across 15 categories, 47 single-file programs
       pinned to golden output on both engines + 2 Go-engine Experimental
   * - Toolchain
     - Implemented: ``check/build/run/test/transpile/fmt/lint/debug/prof/
       target/pkg/workspace/clean/explain/bench/lsp/new``
   * - Developer readiness
     - ``CONTRIBUTING.md``, ``CODE_OF_CONDUCT.md`` and ``LICENSE`` added;
       fresh-checkout build+run validated; feedback via GitHub Issues
   * - Known gaps (honest)
     - Networking, databases, web, quantum, GPU/NPU kernel language surface,
       advanced package registry — Planned / Not Yet Implemented

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
   * - 113
     - pre-0.115
     - Developer Preview documentation + status model established
   * - 114
     - pre-0.115
     - Complete example corpus (46 files, 15 categories) + parity gate
   * - 115
     - v0.115.0
     - Developer Preview readiness: license/contributing/CoC, honest docs,
       version identity, fresh-checkout gate
   * - 116
     - v0.115.0
     - Corpus normalization + real-world programming examples (50 files,
       dash-named categories, stack/queue/knapsack/token-stats) + metadata gate
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
       that claim was superseded — Karkain is Developer Preview, not 1.0.)

.. seealso::

   :doc:`/development/roadmap` — forward-looking plan.
   :doc:`/development/developer-preview` — the honest capability summary.