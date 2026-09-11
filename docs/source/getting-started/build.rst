======
Build
======

``karkain build`` compiles a ``.kark`` file into a native executable
without running it.

Usage
=====

.. code-block:: console

    $ karkain build <file.kark> [-o <output>]

Options:

- ``-o <path>`` — set the output executable path (default: ``main.exe`` on
  Windows, ``a.out`` on other platforms)
- ``--target <triple>`` — cross-compile for a different target (see
  :doc:`/targets/cross-compilation`)
- ``--verbose`` — show the full compilation pipeline
- ``-c / --compile-only`` — emit C but do not invoke gcc/clang

Examples
========

.. code-block:: console

    # Build hello.kark → hello (Linux/macOS) or hello.exe (Windows)
    $ karkain build hello.kark

    # Build with a custom output name
    $ karkain build hello.kark -o hello_world

    # Cross-compile for Linux amd64 on a Windows host
    $ karkain build hello.kark --target x86_64-linux -o hello_linux

    # Emit C only (no gcc/clang invocation)
    $ karkain build hello.kark --compile-only

    # Show pipeline steps
    $ karkain build hello.kark --verbose

Incremental builds
==================

When the ``--incremental`` flag is used, the compiler caches compiled
assembly modules. On a no-op build (no source changes), the compiler
skips recompilation and reuses the cached output:

.. code-block:: console

    $ karkain build main.kark --incremental
    $ karkain build main.kark --incremental   # much faster: cached

The incremental cache is stored in ``.karkain-cache/`` at the project root.
To clear it:

.. code-block:: console

    $ karkain clean

Engine selection
================

The default compilation engine is ``kcc`` (the self-hosted compiler).
To use the Go front end instead:

.. code-block:: console

    $ karkain build --engine go hello.kark

See :doc:`/compiler/kcc` for more on the two engines.

Exit codes
==========

- ``0`` — success
- ``3`` — compilation failure (lexer, parser, semantic, or codegen error)
- ``6`` — environment failure (no usable C compiler found)

.. seealso::

   :doc:`run` — compile and run in one step.
   :doc:`/tools/build` — the CLI reference, including
   ``-c/--compile-only`` for emitting C without compiling.
   :doc:`/targets/cross-compilation` — building for a different platform.