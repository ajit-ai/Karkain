=====
Debug
=====

``karkain debug`` compiles and runs a ``.kark`` file with function-level
execution tracing enabled.

Usage
=====

.. code-block:: console

    $ karkain debug <file.kark> [--engine go]

Options:

- ``--engine go`` — the Go front end (the only supported engine for
  tracing)
- ``--verbose`` — show the full compilation pipeline
- ``-h, --help`` — show debug help

Engine
======

Debugging is a **Go-engine-only** capability. The self-hosted ``kcc``
engine is not yet trace-aware and is refused with an explicit error —
there is no silent fallback:

.. code-block:: console

    $ karkain debug --engine kcc app.kark
    Debug Error: karkain debug supports the Go engine only; the self-hosted
    kcc engine is not yet trace-aware (deferred, Phase 112 boundary).

Trace output
============

Every function call and return records a deterministic trace line on
stderr, while the program's normal stdout passes through unmodified:

.. code-block:: console

    $ karkain debug fib.kark
    karkain:fib.kark:enter main
    karkain:fib.kark:enter fibonacci
    karkain:fib.kark:enter fibonacci
    karkain:fib.kark:leave fibonacci
    ...
    karkain:fib.kark:leave main
    55

The trace format is ``karkain:<file>:enter <func>`` on entry and
``karkain:<file>:leave <func>`` on return.

Implementation
==============

``karkain debug`` runs the ordinary ``run`` pipeline with ``Config.Trace``
set, which emits ``#define KARKAIN_TRACE 1`` in the generated C. The
existing ``karkain_frame_enter`` / ``karkain_frame_leave`` runtime hooks
(shared by both engines) print the enter/leave lines to stderr when the
define is present.

Tracing is strictly opt-in: ``karkain run`` and ``karkain build`` never
enable it.

Scope
=====

This is function-level enter/leave tracing only. There is no source-line
stepping, breakpoint support, or variable inspection — a full source-level
debugger is not part of this release.

Exit codes
==========

Identical to :doc:`run`:

- ``0`` — success
- ``1`` — program failure
- ``3`` — compilation failure
- ``6`` — environment failure (no usable C compiler)

.. seealso::

   :doc:`run` — the underlying compilation and execution pipeline.
   :doc:`profile` — call counts and timing instead of trace lines.