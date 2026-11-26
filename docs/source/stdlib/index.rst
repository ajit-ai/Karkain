.. _stdlib:

Standard Library
================

The Karkain standard library is a set of ``.kark`` modules importable from
any program through the normal toolchain (``import std.string`` and friends)
on both engines — the Go front end and the self-hosted ``kcc`` engine.

Implemented and importable modules
----------------------------------

:implemented:`Implemented` — these modules are importable, tested, and
byte-identical on both engines (Phase 132 API Freeze):

.. list-table::
   :widths: 20 50 30
   :header-rows: 1

   * - Module
     - Purpose
     - Functions
   * - :doc:`std.string <strings>`
     - String manipulation utilities
     - 27
   * - :doc:`std.collections <collections>`
     - Array and map helpers
     - 24
   * - :doc:`std.io <io>`
     - File and stream I/O
     - 11
   * - :doc:`std.encoding <encoding>`
     - Hex, Base64 and UTF-8 codecs
     - 7
   * - :doc:`std.crypto <crypto>`
     - SHA-256 / SHA-512 digests
     - 2
   * - :doc:`std.testing <testing>`
     - Assertion and check helpers
     - 9
   * - ``std.numerics``
     - Numerical/ML foundation (vector/matrix/tensor ops)
     - 40
   * - ``std.net``
     - TCP networking (Address/Endpoint/Connection/Listener)
     - 10
   * - ``std.http``
     - HTTP/1.1 client/server building blocks
     - 20
   * - ``std.db``
     - Backend-neutral database layer (SQL subset)
     - 20
   * - :doc:`std.path <path>`
     - Cross-platform path manipulation
     - 9
   * - :doc:`std.env <env>`
     - Environment variable access
     - 9
   * - :doc:`std.time <time>`
     - Time and date utilities
     - 11
   * - :doc:`std.random <random>`
     - Random number generation
     - 7
   * - :doc:`std.json <json>`
     - JSON field extraction and serialization
     - 6
   * - :doc:`std.core <core>`
     - Core helpers (identity, clamp, min/max, range, sum, product)
     - 20
   * - :doc:`std.math <math>`
     - Math utilities (abs, sqrt, pow, gcd, lcm, factorial)
     - 15
   * - :doc:`std.filesystem <filesystem>`
     - File and directory operations
     - 14
   * - :doc:`std.log <log>`
     - Structured logging
     - 6
   * - :doc:`std.config <config>`
     - Configuration management
     - 5
   * - :doc:`std.process <process>`
     - Process management and execution
     - 10

``stdlib/`` also contains ``std.system``, ``std.gpu`` and ``std.async``,
but these are **not importable**: they use non-canonical syntax (type
annotations, ``import "libc"`` blocks, tuples, ``Option``/``Result``) and
have no runtime builtin backing. They sit behind the Phase 109
standard-library boundary. See
:doc:`not-implemented <not-implemented>`.

``std.core`` and ``std.math`` were promoted to importable in the v1.2.x
line (increment 156): rewritten with canonical Karkain syntax.

Modules that do not exist in the repository at all — ``std.memory``,
``std.process``, ``std.concurrency``, ``std.diagnostics`` — are tracked in
:doc:`not-implemented <not-implemented>`. ``std.os`` now exists as the v1.2.19
C23 OS metric / process enumeration module; see :doc:`os`.

The following modules were added in the v1.2.x line (increment 155):
``std.path``, ``std.env``, ``std.time``, ``std.random``, ``std.json``.

The following modules were promoted in the v1.2.x line (increment 156):
``std.core``, ``std.math``.

``std.system`` remains **not importable**: its ``system()``-based implementation
cannot capture command output (``system()`` returns an exit code, not stdout).
Redesign needed.

Invariants shared by every implemented module
---------------------------------------------

* Every module is written in canonical Karkain (untyped parameters, no type
  annotations) and produces byte-identical behavior on the Go engine and the
  self-hosted ``kcc`` engine.
* Module functions are part of the framework API, so the ``public`` privacy
  rule does not apply to ``stdlib``; every function may be called unqualified
  or namespace-qualified (``str_to_upper(x)`` == ``std.string.str_to_upper(x)``).
* The ``karkain test`` driver does not yet support ``import`` statements
  (Phase 96 boundary); the modules are validated through ``karkain run``.

.. toctree::
   :maxdepth: 2
   :caption: Karkain Standard Library

   strings
   collections
   io
   encoding
   crypto
   testing
   numerics
   net
   http
   db
   path
   env
   time
   random
   json
   filesystem
   log
   config
   process
   os
   core
   math
   not-implemented