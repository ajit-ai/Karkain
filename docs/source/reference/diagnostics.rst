.. _diagnostics:

===========
Diagnostics
===========

Karkain provides structured diagnostics in check-only mode and through the
compiler pipeline. Every diagnostic includes a source location and is
reported consistently on both engines.

Check-only mode
===============

Run the front end without producing an executable:

.. code-block:: console

   $ karkain check myfile.kark

Exit code ``0`` indicates no diagnostics. Exit code ``3`` indicates one or
more diagnostics were emitted.

JSON output
===========

Request machine-readable diagnostics:

.. code-block:: console

   $ karkain check --format=json myfile.kark

The output conforms to the ``karkain-diagnostics-v1`` schema. Each entry
contains:

* ``file`` — source filename
* ``line`` — 1-based line number
* ``column`` — 0-based byte column
* ``endColumn`` — 0-based end column (optional)
* ``excerpt`` — highlighted source excerpt (optional)
* ``code`` — error code string (e.g. ``K102``)
* ``message`` — human-readable description
* ``severity`` — ``error`` or ``warning``

Error codes
===========

.. list-table::
   :widths: 10 50 30
   :header-rows: 1

   * - Code
     - Description
     - Phase
   * - ``K004``
     - Unknown ``@target(...)`` attribute value
     - 98
   * - ``K101``
     - Undefined function call
     - 99
   * - ``K102``
     - Undefined identifier
     - 99
   * - ``K103``
     - Wrong function arity
     - 99
   * - ``K104``
     - Wrong builtin arity
     - 99
   * - ``K106``
     - Undefined struct type
     - 99
   * - ``K107``
     - Duplicate definition
     - 99
   * - ``K108``
     - ``break`` / ``continue`` outside a loop
     - 99
   * - ``K109``
     - Calling a type name as a function
     - 99
   * - ``K112``
     - Primitive annotation/initializer type mismatch
     - 99
   * - ``K113``
     - Reassignment of ``const`` binding
     - 102

Exit codes
==========

.. list-table::
   :widths: 10 40
   :header-rows: 1

   * - Code
     - Meaning
   * - ``0``
     - Success (no diagnostics, or check passed)
   * - ``1``
     - Runtime failure or program error
   * - ``2``
     - Usage error (missing file, unknown flag)
   * - ``3``
     - Diagnostics emitted (compile / check error)

Span fields
===========

Every diagnostic carries a span with:

* ``file`` — source file path
* ``line`` — 1-based line number
* ``column`` — 0-based byte column (byte offset from line start)
* ``endColumn`` — 0-based byte column of the token end (when available)
* ``excerpt`` — a highlighted excerpt of the offending source line

The ``column`` field uses byte offsets (not rune offsets) for exact
alignment with the generated C source locations.

Conformance
===========

The diagnostic contract is validated by **59 test assertions** across
11 files in the ``conformance/`` directory. Both the Go and ``kcc``
engines must produce identical diagnostics for every case.
