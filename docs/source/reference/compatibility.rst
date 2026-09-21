.. _compatibility:

=======================
Engine Compatibility
=======================

Karkain programs run on two engines. The **Go front end** is the reference
compiler; the self-hosted **kcc** engine is the primary engine for
``check`` / ``build`` / ``run`` / ``test``.

Feature parity matrix
=====================

.. list-table::
   :widths: 40 15 15
   :header-rows: 1

   * - Feature
     - Go
     - kcc
   * - ``let`` / ``var`` declarations
     - Yes
     - Yes
   * - ``func`` declarations
     - Yes
     - Yes
   * - ``struct`` / ``enum``
     - Yes
     - Yes
   * - ``const`` declarations
     - Yes
     - Yes
   * - ``if`` / ``else``
     - Yes
     - Yes
   * - ``while`` / ``for`` / ``for-in``
     - Yes
     - Yes
   * - ``match``
     - Yes
     - Yes
   * - ``return`` / ``break`` / ``continue``
     - Yes
     - Yes
   * - ``import`` / ``public``
     - Yes
     - Yes
   * - ``float()`` conversion
     - Yes
     - Yes
   * - ``K113`` const reassignment diagnostic
     - Yes
     - Yes
   * - Runtime error diagnostics (div-by-zero, OOB)
     - Yes
     - Yes
   * - Stack traces
     - Yes
     - Yes
   * - ``std.string`` / ``std.collections`` / ``std.io``
     - Yes
     - Yes
   * - ``std.encoding`` / ``std.crypto`` / ``std.testing``
     - Yes
     - Yes
   * - ``std.numerics`` / ``std.net`` / ``std.http`` / ``std.db``
     - Yes
     - Yes
   * - ``spawn`` / ``receive`` / ``channel`` / ``actor`` / ``send``
     - Yes
     - Partial (surface syntax lexes and parses; the C helper emission is
       Go-engine only — tracked as kcc parity work)
   * - Cross-compilation (``--target <triple>``)
     - Yes
     - No (kcc does not implement target switching; use ``--engine go``)
   * - Nested block comments (``/* /* */ */``)
     - Yes
     - No
   * - ``karkain debug trace``
     - Yes
     - No
   * - ``karkain prof``
     - Yes
     - No
   * - WASM target (``wasm32-wasi``)
     - Yes
     - No
   * - Closures / ``fn`` codegen
     - Partial (``let``-bound closures with by-ref capture mutation and
       nesting; no first-class values)
     - Partial (same — byte-identical goldens on both engines)
   * - ``float64()`` / ``bool()`` / ``string()`` casts
     - No
     - No

.. note::

   Let-bound closures work on both engines. Only **first-class**
   function values (``ops[0](5)``, ``func(T) R`` types) and
   closure-variable capture are open boundaries — identical on both
   engines, tracked as future work.

Known divergences
=================

``const`` location reporting
   The ``kcc`` engine reports the diagnostic at the **declaration** line of
   the ``const``. The Go engine reports it at the **reassignment**
   assignment.

Multi-argument ``println``
   Both engines print **only the first** argument. Multiple arguments are
   not concatenated or space-separated.

``str()`` on arrays
   The Go engine prints ``["x"]`` while ``kcc`` prints ``[x]`` (without
   quotes around string elements).

WASM target
   WASM compilation (``--target wasm32-wasi``) is only available on the
   Go engine and requires ``wasmtime`` on the host. The ``kcc`` engine
   does not support WASM. Since Phase 123 the Go-engine WASM backend is a
   :production-candidate:`Production Candidate` — WASI exit codes, stderr
   diagnostics and ``getArgs()`` are implemented and gated.

Runtime errors and stack traces
   Both engines produce identical ``runtime error: <kind> at <file>:<line>``
   diagnostics and ``stack:`` traces. The stack-trace format is tested
   across both engines with the same expected frames.
