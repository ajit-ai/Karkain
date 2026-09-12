=======================
Command-Line Interface
=======================

The ``karkain`` binary is the single entry point for the entire Karkain
toolchain. Every operation — compile, run, test, format, debug, profile,
package management, LSP — is exposed through subcommands of this one
executable.

.. code-block:: console

    $ karkain <command> [options] <file.kark>

Command set
===========

Compiler commands
-----------------

.. list-table::
   :widths: 28 72
   :header-rows: 1

   * - Command
     - Description
   * - ``run <file>``
     - Compile and execute in one step (default when no command is given)
   * - ``build <file>``
     - Compile to a native executable without running
   * - ``transpile <file>``
     - Generate C23 source without compiling or linking
   * - ``check <file>``
     - Validate syntax and semantics (no binary produced)
   * - ``test [path]``
     - Discover and run ``*_test.kark`` files; ``--filter <sub>``
       selects a subset
   * - ``bench <path>``
     - Time ``bench_``-prefixed functions (single run each)
   * - ``prof <file>``
     - Compile, run, and profile with per-function instrumentation
   * - ``lint <file>``
     - Full front-end analysis including borrow checker
   * - ``explain <code>``
     - Explain a toolchain error code (``--list`` shows all codes)

Tool commands
-------------

.. list-table::
   :widths: 28 72
   :header-rows: 1

   * - Command
     - Description
   * - ``fmt <file|dir>``
     - Canonicalize formatting (``--check`` verifies without rewriting)
   * - ``clean [path] [--all]``
     - Remove generated artifacts (sources never touched)
   * - ``target``
     - List the host triple and all supported ``--target`` values
   * - ``config``
     - Print the effective toolchain configuration

IDE commands
------------

.. list-table::
   :widths: 28 72
   :header-rows: 1

   * - Command
     - Description
   * - ``lsp``
     - Start the Language Server Protocol server (JSON-RPC 2.0 over stdio)
   * - ``language-server``
     - Alias for ``lsp``
   * - ``ide info``
     - Print machine-readable IDE/toolchain contract (JSON)

Workspace commands
------------------

.. list-table::
   :widths: 28 72
   :header-rows: 1

   * - Command
     - Description
   * - ``workspace list``
     - List members in dependency order
   * - ``workspace build``
     - Build all members (dependency order)
   * - ``workspace test``
     - Test all members
   * - ``workspace check``
     - Validate all members (no binaries produced)
   * - ``workspace run``
     - Build and run all member entrypoints
   * - ``workspace clean [--all]``
     - Clean generated artifacts from all members
   * - ``workspace lint``
     - Full front-end analysis of all members
   * - ``workspace graph``
     - Show member dependency graph
   * - ``workspace init``
     - Initialize workspace root
   * - ``workspace add <path>``
     - Add member package
   * - ``workspace remove <path>``
     - Remove member package

Package commands
----------------

Top-level shortcuts and the full ``karkain pkg`` namespace are
implemented. See :doc:`packages` for the complete reference.

.. code-block:: console

    $ karkain init [name]            # create new project
    $ karkain new <name>             # create a new project directory
    $ karkain add <pkg> [version]    # add dependency
    $ karkain remove <pkg>           # remove dependency
    $ karkain fetch                  # fetch resolved dependencies (lockfile)
    $ karkain update [pkg]           # re-resolve versions, write karkain.lock
    $ karkain list                   # list direct + transitive dependencies
    $ karkain tree                   # show recursive dependency graph

These are equivalent to the ``karkain pkg <verb>`` spellings.

Options
=======

.. list-table::
   :widths: 22 8 70
   :header-rows: 1

   * - Option
     - Arg
     - Description
   * - ``-o <path>``
     - yes
     - Output binary path (``build``)
   * - ``-c``, ``--compile-only``
     - no
     - Emit C source but do not invoke gcc/clang
   * - ``-g``, ``--debug``
     - no
     - Generate debug symbols and ``#line`` directives
   * - ``--target <triple>``
     - yes
     - Target architecture for ``build``/``run`` (see :doc:`target`)
   * - ``--filter <pattern>``
     - yes
     - Run only matching tests (substring of test name)
   * - ``--verbose``
     - no
     - Emit detailed pipeline logs
   * - ``--engine <go|kcc>``
     - yes
     - Select the compiler engine (default ``kcc``)
   * - ``-v``, ``--version``
     - no
     - Show version
   * - ``-h``, ``--help``
     - no
     - Show help

Engine selection
================

Karkain has two compiler engines:

- **kcc** — the self-hosted compiler, bootstrapped from
  ``src/compiler/*.kark``. This is the **default** engine.
- **go** — the in-tree Go front end (``pkg/lexer`` + ``pkg/parser`` +
  ``pkg/codegen``).

The engine is selected by (in order of priority):

1. The ``--engine`` flag on the command line.
2. The ``KARKAIN_ENGINE`` environment variable (``go``/``Go``/``GO``
   selects the Go front end; any other value or an empty value selects
   ``kcc``).
3. Default: ``kcc``.

.. code-block:: console

    $ karkain run --engine go hello.kark   # force Go engine
    $ KARKAIN_ENGINE=go karkain run hello.kark  # same via env var

.. note::

   Some commands (``prof``, ``debug``) are Go-engine-only in the current
   release and explicitly refuse the ``kcc`` engine with an error — there
   is no silent fallback.

Exit codes
==========

.. list-table::
   :widths: 8 28 64
   :header-rows: 1

   * - Code
     - Name
     - Meaning
   * - ``0``
     - ``ExitSuccess``
     - Command completed successfully
   * - ``1``
     - ``ExitFailure``
     - General or program failure (runtime error, assertion, non-zero
       program exit, missing/invalid profile dump)
   * - ``2``
     - ``ExitUsage``
     - CLI usage error (unknown flag or command, missing argument,
       invalid target or extension)
   * - ``3``
     - ``ExitCompile``
     - Lexer, parser, semantic, type, borrow, or codegen failure
   * - ``4``
     - ``ExitTest``
     - One or more tests failed
   * - ``5``
     - ``ExitPackage``
     - Package or dependency failure
   * - ``6``
     - ``ExitEnv``
     - Infrastructure or toolchain failure (no usable C compiler,
       missing cross-linker)
   * - ``7``
     - ``ExitLint``
     - Lint-specific failure (warnings treated as errors)

Version
=======

.. code-block:: console

    $ karkain -v
    Karkain Compiler v1.0.0 (linux/amd64, Stable Build)

The version string reports the toolchain version, host OS and host
architecture.

.. seealso::

   :doc:`build` — full build documentation.
   :doc:`run` — full run documentation.
   :doc:`test` — test discovery and execution.
   :doc:`packages` — package manager reference.
