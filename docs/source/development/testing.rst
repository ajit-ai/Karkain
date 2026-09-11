Testing
=======

The Karkain test suite validates the compiler, runtime, standard library and
toolchain on **two engines** — the Go front end and the self-hosted ``kcc``
engine — with deterministic, reproducible results. This page explains the
structure and how to run each layer.

.. contents:: Test layers
   :local:
   :depth: 1

Go unit tests
=============

Every ``pkg/*/...`` package has ``*_test.go`` files that run in-process
without invoking the compiler subprocess. Run the core packages with:

.. code-block:: console

   go test ./pkg/lexer/... ./pkg/parser/... ./pkg/codegen/... ./pkg/pm/... -count=1

.. code-block:: console

   go test ./pkg/ir/ssa/... ./pkg/sema/... ./pkg/source/... -count=1

On the documented 4 GB host, run single packages sequentially:

.. code-block:: console

   GOMAXPROCS=1 GOGC=60 go test ./pkg/lexer/... -count=1
   GOMAXPROCS=1 GOGC=60 go test ./pkg/codegen/... -count=1

Conformance corpus
==================

``conformance/`` contains 11 test files with **59** ``func test_*``
functions:

.. code-block:: text

   001_arithmetic_test.kark
   002_control_flow_test.kark
   003_functions_test.kark
   004_strings_test.kark
   005_arrays_test.kark
   006_structs_test.kark
   007_maps_test.kark
   008_algorithms_test.kark
   009_phase81_regression_test.kark
   010_array_returns_test.kark
   011_namespace_test.kark

These run through the real front end + C runtime on both engines. The test
driver ensures byte-identical stdout and exit codes on both engines.

E2E CLI tests
=============

``pkg/cli/*_test.go`` exercises the real CLI subprocess — ``karkain check``,
``karkain build``, ``karkain run``, ``karkain test``, ``karkain fmt``,
``karkain lint``, ``karkain debug``, ``karkain prof``, ``karkain target``,
``karkain lsp``, ``karkain pkg``. Every command is validated end-to-end
through actual process invocations.

Run the full CLI suite (large, allocate dedicated time):

.. code-block:: console

   go test ./pkg/cli/... -count=1

Phase-specific gate tests
=========================

Each major phase has a ``phase*_test.go`` gate that pins the behavior it
introduced. Examples:

.. code-block:: text

   pkg/cli/phase95_parity_test.go      # kcc engine parity (Go + kcc goldens)
   pkg/cli/phase100_runtime_test.go     # runtime error model (parity)
   pkg/cli/phase107_concurrency_test.go  # concurrency runtime (E2E)
   pkg/cli/phase108_cli_test.go         # wasm32-wasi (E2E through wasmtime)
   pkg/cli/phase109_stdlib_test.go      # standard library v2 (multi-module)
   pkg/cli/phase110_profiling_test.go   # karkain prof (text/json/folded)
   pkg/cli/phase111_cross_compile_test.go  # cross-compilation triples

All gate tests must pass before merging.

Memory constraints (documented)
===============================

On the documented ~4 GB host, running the full ``go test ./pkg/...`` tree
in parallel causes stalls and OOMs. Mitigations:

* ``GOMAXPROCS=1 GOGC=60`` for individual packages.
* Single-package runs only for focused development.
* Bootstrap stage-2 builds may OOM/SEGFAULT with the full ``src/compiler``
  tree — this is a documented environmental limitation, not a defect.
  ``karkain check`` (low-memory path) still passes.

.. note::

   If tests stall on your machine, run them in isolation rather than as a
   single parallel invocation.

.. seealso::

   :doc:`contributing` — how to run the suite before committing.
   :doc:`/status/implemented` — conformance corpus and parity gates.