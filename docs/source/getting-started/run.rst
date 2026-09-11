====
Run
====

``karkain run`` compiles a ``.kark`` file and executes it in one step.

Usage
=====

.. code-block:: console

    $ karkain run <file.kark> [--engine go|kcc] [--target <target>] [--verbose]

Options:

- ``--engine go|kcc`` — select the compiler engine (default: ``kcc``, the
  self-hosted compiler; the Go front end is used when ``KARKAIN_ENGINE=go``)
- ``--target <target>`` — target to build and run for (``native``/``c23`` by
  default; ``wasm32-wasi`` runs through ``wasmtime``)
- ``--verbose`` — show the full compilation pipeline

How it works
============

On the Go engine the compiler sets ``RunAfter = true``: the program is
parsed, checked, transpiled to C, compiled, and the resulting native
executable runs immediately. On the ``kcc`` engine, ``KCCRunCommand``
assembles the source (local modules and manifest dependencies upstream, the
root file last), builds to C23 in a temporary sandbox, links with ``gcc`` and
executes the binary there — build artifacts never sit next to the source.

The program's stdout passes through to the terminal unchanged; diagnostics and
tracing go to the process stderr.

Examples
========

.. code-block:: console

    $ karkain run hello.kark
    Hello, Karkain!

    $ karkain run --engine go hello.kark     # force the Go front end

    $ karkain run --verbose hello.kark       # show the pipeline

Program arguments
=================

Argument forwarding to the executed program is not yet implemented: extra
CLI arguments after ``<file.kark>`` are accepted but not passed to the
program's ``argv`` (documented boundary).

Runtime errors
==============

A runtime failure aborts with a source-located diagnostic on stderr and a
function stack dump:

.. code-block:: console

    runtime error: integer division by zero at div_by_zero.kark:3
      stack:
        inner (div_by_zero.kark:3)
        outer (div_by_zero.kark:5)
        main (div_by_zero.kark:9)

Exit codes
==========

- ``0`` — success
- ``1`` — program failure (runtime error, assertion, or non-zero program exit)
- ``3`` — compilation failure (lexer, parser, semantic, or codegen error)
- ``6`` — environment failure (no usable C compiler, or a cross-run attempt)

.. seealso::

   :doc:`build` — compile to a native executable without running.
   :doc:`debug` — the same pipeline with function enter/leave tracing.
   :doc:`profile` — the same pipeline with profiling instrumentation.