=======
Profile
=======

``karkain prof`` compiles and runs a ``.kark`` program once with opt-in
profiling instrumentation and reports deterministic per-function data.

Usage
=====

.. code-block:: console

    $ karkain prof <file.kark> [--format text|json|folded] [--output <path>]

Options:

- ``--format text|json|folded`` — report format (default ``text``)
- ``--output <path>`` — write the report to a file instead of stdout
- ``--json`` — shorthand for ``--format json``
- ``-o <path>`` — shorthand for ``--output <path>``

Engine
======

Profiling is a **Go-engine-only** capability. ``kcc`` and the WASM
(``wasm32-wasi``) target are explicitly unusupported with no silent fallback:

.. code-block:: console

    $ karkain prof --engine kcc app.kark
    Profiling Error: karkain prof supports the Go engine only; the
    self-hosted kcc engine is not yet profiler-aware (deferred, ...).

What is measured
================

The report contains:

- per-function call counts and inclusive/exclusive/min/max/average wall
  nanoseconds
- the caller → callee call graph with edge counts and times
- folded (flame-graph) stacks (``"A;B;C ns"`` paths)
- allocation metrics (count, bytes, peak live bytes)

Formats
=======

- ``text`` — the human-readable default: a function table sorted by total
  time, the call graph, and allocation metrics
- ``json`` — the machine-readable ``karkain-profile-v1`` schema
- ``folded`` — one ``path ns`` line per folded stack, sorted by path, ready
  for flamegraph tooling

Bounds
======

Instrumentation capacity is bounded: at most 512 functions and 4096 distinct
call-graph edges. When the program exceeds the bounds, counts are truncated
and the report sets an overflow flag (a ``note:`` line in text output).

Profiling is strictly opt-in — ``karkain run`` and ``karkain build`` never
instrument the program.

Exit codes
==========

- ``0`` — success
- ``1`` — program failure, missing/invalid profile dump, or a refused
  engine/target
- ``2`` — CLI usage error (unknown ``--format``, missing file)
- ``3`` — compilation failure

.. seealso::

   :doc:`debug` — trace every function entry/exit.
   :doc:`run` — the underlying execution pipeline.