.. _stdlib-core:

std.core — Core helpers
=======================

:implemented:`Implemented` — importable on both engines.

``std.core`` provides fundamental operations: identity, type helpers,
conversion, comparison, range generation, and array aggregation. Rewritten
with canonical Karkain syntax (increment 156).

Importing
---------

.. code-block:: karkain

   import std.core

Module reference
----------------

``identity(x)``
   Returns its argument unchanged.
   Returns: ``int``

``identity_float(x)``
   Returns its argument unchanged.
   Returns: ``float``

``clamp(val, lo, hi)``
   Restricts a value to a given range [lo, hi].
   Returns: ``int``

``clamp_float(val, lo, hi)``
   Restricts a float value to a given range.
   Returns: ``float``

``min(a, b)`` / ``max(a, b)``
   Returns the smaller/larger of two integers.
   Returns: ``int``

``min_float(a, b)`` / ``max_float(a, b)``
   Returns the smaller/larger of two floats.
   Returns: ``float``

``core_abs(x)``
   Returns the absolute value of an integer.
   Returns: ``int``

``sign(x)`` / ``sign_float(x)``
   Returns -1, 0, or 1 depending on the sign.
   Returns: ``int`` / ``float``

``range(start, end)``
   Generates an array of integers from start (inclusive) to end (exclusive).
   Returns: ``int[]``

``range_step(start, end, step)``
   Generates an array with a custom step.
   Returns: ``int[]``

``repeat(val, n)``
   Creates an array with n copies of the given value.
   Returns: ``int[]``

``sum(arr)`` / ``product(arr)``
   Returns the sum/product of all elements.
   Returns: ``int``

``max_of(arr)`` / ``min_of(arr)``
   Returns the maximum/minimum element.
   Returns: ``int``

Example
-------

.. code-block:: karkain

   import std.core

   func main() {
       print(clamp(5, 0, 10))    // 5
       print(min(3, 7))          // 3
       print(max(3, 7))          // 7
       print(sum([1,2,3,4]))     // 10
       print(range(0, 5))        // [0,1,2,3,4]
   }