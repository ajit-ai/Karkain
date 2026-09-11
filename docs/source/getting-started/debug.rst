=====
Debug
=====

``karkain debug`` compiles and runs a ``.kark`` file with function-level
execution tracing enabled.

Usage
=====

.. code-block:: console

    $ karkain debug <file.kark> [--engine go]

Engine
======

Debugging is a **Go-engine-only** capability. The self-hosted ``kcc`` engine
is not yet trace-aware and is refused with an explicit error — there is no
silent fallback:

.. code-block:: console

    $ karkain debug --engine kcc app.kark
    Debug Error: karkain debug supports the Go engine only; the self-hosted
    kcc engine is not yet trace-aware (deferred, Phase 112 boundary).

Trace output
============

Every function call and return records a deterministic trace line on stderr,
while the program's normal stdout passes through unmodified:

.. code-block:: console

    $ karkain debug fib.kark
    karkain:fib.kark:enter main
    karkain:fib.kark:enter fibonacci
    karkain:fib.kark:enter fibonacci
    karkain:fib.kark:leave fibonacci
    ...
    karkain:fib.kark:leave main
    55

Implementation
==============

``karkain debug`` runs the ordinary ``run`` pipeline with ``Config.Trace``
set, which emits ``#define KARKAIN_TRACE 1`` in the generated C. The existing
``karkain_frame_enter`` / ``karkain_frame_leave`` runtime hooks (shared by
both engines) print ``karkain:<file>:enter <func>`` / ``leave <func>`` to
stderr when the define is present. ``karkain run`` and ``karkain build`` never
enable tracing — it is strictly opt-in.

Scope
=====

This is function-level enter/leave tracing only; there is no source-line
stepping or variable inspection.

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