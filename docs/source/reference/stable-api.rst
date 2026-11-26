===========================
Stable API Snapshot (1.2.0)
===========================

This is the authoritative 1.2.0 (Stable) API snapshot — the reference point
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
   * - Concurrency
     - ``spawn`` / ``join`` / ``channel`` / ``actor`` with full ``kcc``
       parity (Phase 137), plus task control on both engines: ``cancel(task)``
       returns ``true`` on the first accepted request (``false`` for a
       completed, already-cancelled or non-task handle) and is cooperative -
       a queued task never runs, a running task is not preempted;
       ``join_timeout(task, ms)`` returns ``true`` if the task completed
       within the budget and ``false`` on timeout, never cancels, leaves the
       task joinable, treats ``0`` as a non-blocking check, rejects negative
       timeouts/foreign handles, and uses a monotonic deadline (no
       busy-wait).
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

.. list-table:: Stable stdlib (Phase 132 API Freeze)
   :widths: 26 74
   :header-rows: 1

   * - Module
     - Surface
   * - ``std.string``
     - String operations on UTF-8 strings (27 functions).
   * - ``std.collections``
     - Collection helpers (list/map utilities, 24 functions).
   * - ``std.io``
     - Input/output helpers (file read/write, line reading, 11 functions).
   * - ``std.encoding``
     - Hex and Base64 encode/decode (RFC 4648 verified, 7 functions).
   * - ``std.crypto``
     - ``sha256_hex`` / ``sha512_hex`` (NIST FIPS 180 test vectors, 2 functions).
   * - ``std.testing``
     - ``assert`` / ``assert_eq`` / ``assert_ne`` for ``*_test.kark`` (9 functions).
   * - ``std.numerics``
     - Numerical/ML foundation (vector/matrix/tensor ops, 40 functions).
   * - ``std.net``
     - TCP networking plus UDP datagrams (Address/Endpoint/Connection/Listener,
       udp_send_to/udp_send_from/udp_bind_on/udp_recv_from/udp_error; 15 functions).
   * - ``std.http``
     - HTTP/1.1 client/server building blocks (Request/Response, 20 functions)
       plus ``https_get`` / ``https_post`` / ``https_last_error`` (C23 TLS,
       v1.2.17; 23 functions total) plus ``http_echo_worker`` /
       ``http_serve_echo`` (v1.2.18; 25 functions total) plus ``ws_connect`` /
       ``ws_send`` / ``ws_recv`` / ``ws_accept_key`` / ``ws_error`` (v1.2.21; 30 total).
   * - ``std.os``
     - OS metrics and process enumeration (7 functions, v1.2.19; C23 path only).
   * - ``std.db``
     - Backend-neutral SQL-text engine (20 functions) plus v1.2.22 SQLite driver:
       ``db_sqlite_open`` / ``db_sqlite_exec`` / ``db_sqlite_query`` /
       ``db_sqlite_close`` / ``db_sqlite_error`` (25 total, C23 path only).
   * - ``std.path``
     - Cross-platform path manipulation (9 functions, increment 155).
   * - ``std.env``
     - Environment variable access (9 functions, increment 155).
   * - ``std.time``
     - Time and date utilities (11 functions, increment 155).
   * - ``std.random``
     - Random number generation (7 functions, increment 155).
   * - ``std.json``
     - JSON field extraction plus v1.2.20 DOM (11 functions): parse, type, path query,
       escaped serialization and deterministic errors; both engines, C23 path only.
   * - ``std.filesystem``
     - File and directory operations (14 functions, increment 155).
   * - ``std.log``
     - Structured logging (6 functions, increment 155).
   * - ``std.config``
     - Configuration management (5 functions, increment 155).
   * - ``std.process``
     - Process management and execution (10 functions, increment 155).
   * - ``std.core``
     - Core helpers: identity, clamp, min/max, sign, range, sum, product (20 functions, increment 156).
   * - ``std.math``
     - Math utilities: abs, sqrt, pow, min/max, clamp, gcd, lcm, factorial (15 functions, increment 156).
   * - ``std.generics``
     - Persistent ``Stack[T]`` / ``Queue[T]`` over plain arrays (12
       functions; new in Phase 146, both engines since Phase 146C).

Full stdlib reference: :doc:`/stdlib/index`.

Stable diagnostic expectations
==============================

* ``error[K...]`` numeric codes on the front end; ``karkain explain <code>``
  documents each code (K001–K008, K100, K101–K116, K145, K160, including K114 escape
  rejection, K115 generic-arity/misuse, and K116 engine-capability refusal).
* ``K116`` is a capability refusal, not a program defect: the self-hosted engine
  (kcc) has no machine-code backend, so a C-free native target
  (``native-x86_64-linux`` / ``-windows`` / ``-macos``) requested under kcc is
  refused loudly instead of being silently built by the Go engine. The three
  native targets are Go-owned; kcc owns C23. See
  :doc:`/development/roadmap` (increment 151A) and
  ``docs/audit/PHASE-151-BASELINE.md``.
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
        module; Go-engine, ``wasmtime``-gated, production candidate with
        WASI exit codes/stderr/``getArgs()``).
   * - Conventional triples
     - ``x86_64-windows``, ``aarch64-windows``, ``x86_64-linux``,
       ``aarch64-linux``, ``riscv64-linux`` (parse + error paths),
       ``x86_64-macos``, ``aarch64-macos``, ``wasm32-wasi`` —
       canonical short triples plus long-form normalization.
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
