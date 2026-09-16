.. _stdlib:

Standard Library
================

The Karkain standard library is a set of ``.kark`` modules importable from
any program through the normal toolchain (``import std.string`` and friends)
on both engines — the Go front end and the self-hosted ``kcc`` engine.

Implemented and importable modules
----------------------------------

:implemented:`Implemented` — these modules are importable, tested, and
byte-identical on both engines:

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

``stdlib/`` also contains ``std.core``, ``std.math``, ``std.system``,
``std.gpu`` and ``std.async``, but these are **not importable**: they use
non-canonical syntax (type annotations, ``import "libc"`` blocks, tuples,
``Option``/``Result``) and have no runtime builtin backing. They sit behind
the Phase 109 standard-library boundary. See
:doc:`core <core>`, :doc:`math <math>` and
:doc:`not-implemented <not-implemented>`.

Modules that do not exist in the repository at all — ``std.memory``,
``std.filesystem``, ``std.process``, ``std.os``, ``std.time``,
``std.concurrency``, ``std.diagnostics`` — are tracked in
:doc:`not-implemented <not-implemented>`.

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
   core
   math
   not-implemented