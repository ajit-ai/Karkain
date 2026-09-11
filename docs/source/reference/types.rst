.. _types-reference:

===============
Type Reference
===============

Built-in types
==============

.. list-table::
   :widths: 15 20 40
   :header-rows: 1

   * - Type
     - C representation
     - Description
   * - ``int``
     - ``long long`` (64-bit signed)
     - Integer arithmetic; default numeric type
   * - ``float64``
     - ``double`` (64-bit IEEE-754)
     - Floating-point arithmetic
   * - ``bool``
     - ``long long`` (internally ``0`` or ``1``)
     - Logical values; truthiness
   * - ``string``
     - Heap-allocated ``NativeString`` (UTF-8 bytes)
     - Length-prefixed; supports concatenation and indexing
   * - ``array``
     - ``NativeArray`` (``Value** items``)
     - Dynamic array; ``[val, ...]`` literal
   * - ``map``
     - ``{}`` string-keyed
     - Associative map (string keys only)
   * - ``struct``
     - Tagged map instances
     - User-defined record types

Type inference
==============

``let`` and ``var`` declarations infer the type from the right-hand side:

.. code-block:: karkain

   let x = 42          // int
   let pi = 3.14       // float64
   let msg = "hello"   // string
   let flag = true     // bool
   let arr = [1, 2, 3] // array

Typed declarations
==================

Explicit type annotations follow a colon:

.. code-block:: karkain

   let y: float64 = 3.14
   var count: int = 0

The annotation must be compatible with the initializer. A mismatch produces
a diagnostic (``K112`` — primitive annotation/initializer mismatch).

Conversion functions
====================

``float()`` converts a value to ``float64``. Supported inputs:

.. list-table::
   :widths: 20 40
   :header-rows: 1

   * - Input type
     - Behavior
   * - ``int``
     - Integer-to-float promotion
   * - ``float64``
     - Identity (no-op)
   * - ``string``
     - Parse string as float
   * - ``bool``
     - ``true`` → ``1.0``, ``false`` → ``0.0``

.. code-block:: karkain

   let x: float64 = float(42)     // 42.0
   let y: float64 = float("3.14") // 3.14
   let z: float64 = float(true)   // 1.0
