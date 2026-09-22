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
stepping, breakpoint support, or variable inspection from ``karkain debug``
itself — for a live debugger walk, see ``karkain dbg`` below.

Live walk: ``karkain dbg``
==========================

Phase 140 adds ``karkain dbg``, which builds with debug symbols and walks
the program under GDB, rendering a Karkain-level backtrace:

.. code-block:: console

    $ karkain dbg prog.kark
    karkain_dbg trace: prog.kark
    #0 inner at prog.kark:6
    #1 outer at prog.kark:17
    #2 main at prog.kark:28

Frames name Karkain functions (the ``karkain_user_`` namespace demangled)
with their locations in the debug unit. Frame line numbers are
assembly-relative when sibling files join the unit (the known assembly
phenomenon) — the function sequence is the exact call chain.

- Go-engine only (same boundary as tracing above); GDB missing is exit 6
  with an install hint, never a fake trace.
- VS Code: the ``Karkain: Debug File (gdb)`` command (``karkain.debug``)
  builds with ``-g`` and launches a cppdbg/gdb session; ``launch.json``
  and ``tasks.json`` templates ship under ``editors/vscode`` — copy them
  to ``.vscode/`` in your workspace.
- lldb works the same batch shape but is unwired; DAP is Phase 153.

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