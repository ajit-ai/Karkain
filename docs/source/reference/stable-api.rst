===========================
Stable API Snapshot (1.0.0)
===========================

This is the authoritative 1.0.0 (Stable) API snapshot — the reference point
for the release-readiness evaluation. A feature belongs to the snapshot only
when a gate test exercises it through the real pipeline on both engines.
Anything not listed here is not part of the stable surface.

.. contents:: Sections
   :local:
   :depth: 1

Scope of the snapshot
=====================

The snapshot records five stable dimensions:

* Stable **syntax** — the language surface.
* Stable **CLI** — commands, options, exit codes.
* Stable **stdlib modules** — importable ``std.*`` modules and their
  backing builtins.
* Stable **diagnostic expectations** — error-code contract and runtime
  error model.
* Stable **target model** — the closed set of ``--target`` values and the
  host matrix.

Stable syntax
=============

.. list-table:: Stable language surface
   :widths: 34 66
   :header-rows: 1

   * - Surface
     - Items
   * - Declarations
     - ``let`` / ``var`` variables, ``func`` declarations with recursion,
       ``public`` module exports, ``struct`` (record) declarations, ``enum``
       declarations, ``import module`` / ``import std.*``.
   * - Control flow
     - ``if`` / ``else``, ``while (cond)``, C-style ``for``,
       ``for-in`` over arrays, ``break`` / ``continue`` / ``return``,
       ``match`` over literals and ``Option``.
   * - Values
     - Integers, floats, booleans, strings, ``None`` / ``Option``, arrays,
       maps, struct values, lambda-free function calls, parenthesized
       expressions, arithmetic/comparison/logical operators.
   * - Runtime error model
     - Checked division/modulo by zero and array/string indexing report
       ``runtime error: <kind> at <file>:<line>`` and stack traces on both
       engines.
   * - Entry point
     - The top-level file scope is the entry point; a top-level
       ``func main()`` is an ordinary function unless file-scope code calls
       it (``examples/01-fundamentals/01_hello_world.kark`` calls it).

Full language guidance: :doc:`/language/index`.

Stable CLI
==========

.. list-table:: Stable CLI commands
   :widths: 26 74
   :header-rows: 1

   * - Command
     - Contract
   * - ``check``
     - Validate syntax + semantics; exit 3 on failure.
   * - ``build``
     - Compile to a native executable (``-o``, optional ``--target``);
       exit 3 on compile failure.
   * - ``run``
     - Compile and run in one step.
   * - ``test``
     - Discover ``*_test.kark`` files, run ``test_*`` functions,
       ``--filter`` substring, exit 4 on failure.
   * - ``fmt`` / ``lint`` / ``explain`` / ``clean`` / ``target`` / ``config``
     - Formatter (idempotent, ``--check``), full-analysis lint (exit 7),
       numeric error-code help, artifact cleanup, target listing, effective
       configuration.
   * - ``prof``
     - Profile a program (text/json/folded; Go engine).
   * - ``workspace``
     - Multi-member workspace management using dependency order
       (``list``/``build``/``test``/``run``/``graph``/``init``/``add``).
   * - ``pkg`` (local subset)
     - ``init``/``add``/``remove``/``update``/``fetch``/``list``/``tree``
       and cache subcommands; local and workspace sources only — no public
       registry.
   * - ``lsp`` / ``ide``
     - LSP over stdio; machine-readable IDE contract.

Exit-code contract: 0 success, 1 program failure, 2 CLI usage error, 3
compile/type/semantic, 4 test failure, 5 package/dependency, 6
infrastructure (e.g. no usable C compiler), 7 lint findings.

Full CLI reference: :doc:`/tools/cli`.

Stable stdlib modules
=====================

.. list-table:: Beta stdlib
   :widths: 26 74
   :header-rows: 1

   * - Module
     - Surface
   * - ``std.string``
     - String operations on UTF-8 strings.
   * - ``std.collections``
     - Collection helpers (list/map utilities).
   * - ``std.io``
     - Input/output helpers (file read/write, line reading).
   * - ``std.encoding``
     - Hex and Base64 encode/decode (RFC 4648 verified).
   * - ``std.crypto``
     - ``sha256_hex`` / ``sha512_hex`` (NIST FIPS 180 test vectors).
   * - ``std.testing``
     - ``assert`` / ``assert_eq`` / ``assert_ne`` for ``*_test.kark``.

Full stdlib reference: :doc:`/stdlib/index`.

Stable diagnostic expectations
==============================

* ``error[K...]`` numeric codes on the front end; ``karkain explain <code>``
  documents each code (K001–K008, K100, K101–K113).
* ``karkain check --format=json`` emits the ``karkain-diagnostics-v1`` JSON
  contract.
* Runtime failure model: ``runtime error: <kind> at <file>:<line>`` with a
  ``stack:`` trace, byte-identical on both engines.
* Exit codes above always map to the documented contract, never ad-hoc
  values.

Full diagnostics reference: :doc:`/reference/diagnostics`.

Stable target model
===================

.. list-table:: Target model
   :widths: 30 70
   :header-rows: 1

   * - Item
     - Contract
   * - Legacy aliases
     - ``native`` (host default), ``c23`` (C23 source output),
       ``native-link`` (Phase 84 linker pipeline), ``wasm32-wasi`` (WASM
       module; Go-engine, ``wasmtime``-gated).
   * - Conventional triples
     - ``x86_64-windows``, ``x86_64-linux``, ``aarch64-linux``,
       ``wasm32-wasi`` — canonical short triples plus long-form
       normalization.
   * - Host reporting
     - ``karkain target`` prints the host triple, the supported matrix and
       the default.
   * - Cross-compilation
     - ``karkain build --target <foreign>`` requires a real cross-linker and
       fails deterministically with a ``ToolchainError`` otherwise; never a
       silent host fallback. ``run --target <foreign>`` is refused with a
       build-only hint.

Full target reference: :doc:`/targets/index`.

Snapshot maintenance
====================

* The snapshot moves only with the 
  :doc:`/status/compatibility` policy: stable-core changes are documented,
  justified and migration-noted.
* After a release-candidate is declared, the snapshot is re-verified and
  published as the RC reference point.
* Every claim here is backed by the gate in ``pkg/cli/phase118_rc_test.go``
  and the regression baselines in :doc:`/status/rc-checklist`.

.. seealso::

   :doc:`/status/scope` — how each item in the snapshot is classified.
   :doc:`/status/compatibility` — the guarantees attached to the snapshot.
   :doc:`/status/beta` — the Beta 1 capability assessment.
   :doc:`/status/rc-checklist` — the readiness checklist that consumes this
   snapshot.