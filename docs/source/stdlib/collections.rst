.. _stdlib-collections:

std.collections — Array and map helpers
=======================================

:implemented:`Implemented` — importable on both engines, byte-identical.

``std.collections`` provides array and map utility operations. The ``*_str``
variants are string-array companions to the integer-array helpers. Map
iterating relies on the runtime builtin ``map_keys_of`` (wired into the Go
resolver, Go codegen and the ``kcc`` checker/sema/codegen), plus the builtins
``push``, ``len`` and ``hasKey``.

All functions are implemented in pure canonical Karkain with untyped
parameters and are identical on the Go and ``kcc`` engines.

Importing
---------

.. code-block:: karkain

   import std.collections

Module reference
----------------

Lookup
~~~~~~

``array_contains(arr, val)``
   Returns ``true`` if the integer array ``arr`` contains ``val``.
   Returns: ``bool``

``array_contains_str(arr, val)``
   Returns ``true`` if the string array ``arr`` contains ``val``.
   Returns: ``bool``

``array_index_of(arr, val)``
   Returns the index of the first occurrence of ``val`` (an ``int``) in
   ``arr``, or ``-1`` if not found.
   Returns: ``int``

``array_index_of_str(arr, val)``
   Returns the index of the first occurrence of ``val`` (a string) in ``arr``,
   or ``-1`` if not found.
   Returns: ``int``

Construction
~~~~~~~~~~~~

``array_fill(val, n)``
   Returns a new array of ``n`` elements all set to ``val``. Note the
   parameter order: value first, count second.
   Returns: ``array``

``array_copy(arr)``
   Returns a shallow copy of the integer array ``arr``.
   Returns: ``array``

``array_copy_str(arr)``
   Returns a shallow copy of the string array ``arr``.
   Returns: ``array``

``array_reverse(arr)``
   Returns a new array with the elements of the integer array ``arr`` in
   reverse order (the original is unmodified).
   Returns: ``array``

``array_reverse_str(arr)``
   Returns a new array with the elements of the string array ``arr`` in
   reverse order.
   Returns: ``array``

Shaping
~~~~~~~

``array_slice(arr, start, end)``
   Returns a sub-array of the integer array ``arr`` from ``start`` up to (but
   not including) ``end``, bounded by ``len(arr)``.
   Returns: ``array``

``array_slice_str(arr, start, end)``
   Returns a sub-array of the string array ``arr`` from ``start`` up to (but
   not including) ``end``.
   Returns: ``array``

``array_remove(arr, val)``
   Returns a new array with the first occurrence of ``val`` removed.
   Returns: ``array``

``array_remove_at(arr, index)``
   Returns a new array with the element at ``index`` removed.
   Returns: ``array``

``array_insert(arr, index, val)``
   Returns a new array with ``val`` inserted at ``index``, shifting subsequent
   elements to the right.
   Returns: ``array``

``array_unique(arr)``
   Returns a new array with duplicate elements removed (first occurrence kept).
   Returns: ``array``

``array_flatten(arr)``
   Concatenates an array of arrays into a single flat array.
   Returns: ``array``

Aggregation
~~~~~~~~~~~

``array_sum(arr)``
   Returns the sum of all elements.
   Returns: ``int``

``array_min(arr)``
   Returns the minimum element, or ``0`` for an empty array.
   Returns: ``int``

``array_max(arr)``
   Returns the maximum element, or ``0`` for an empty array.
   Returns: ``int``

``array_max_2(arr)``
   Returns ``[index, value]`` identifying the maximum element; ``index`` is
   ``-1`` and ``value`` is ``0`` for an empty array.
   Returns: ``array`` — ``[int, int]``

Maps
~~~~

``map_keys(m)``
   Returns an array of all keys in the map ``m``. Delegates to the runtime
   builtin ``map_keys_of``.
   Returns: ``array``

``map_values(m)``
   Returns an array of all values in the map ``m``, in the same order as
   ``map_keys``.
   Returns: ``array``

``map_contains_key(m, key)``
   Returns ``true`` if the map ``m`` contains ``key``. Delegates to ``hasKey``.
   Returns: ``bool``

``map_merge(a, b)``
   Returns a new map combining ``a`` and ``b``; values from ``b`` override
   values from ``a`` for overlapping keys.
   Returns: ``map``

Example
-------

.. code-block:: karkain

   import std.collections

   func main() {
       let nums = [3, 1, 4, 1, 5]
       println(array_contains(nums, 4))            // true
       println(array_fill(9, 3))                   // [9, 9, 9]
       println(array_sum(nums))                    // 14
       println(array_max(nums))                    // 5

       let m = {"a": 1, "b": 2}
       println(map_keys(m))                        // keys of m
       println(map_values(m))                      // [1, 2] in key order
       println(map_contains_key(m, "a"))           // true
   }

Note
----

Map iteration order follows ``map_keys_of`` (the runtime builtin): keys are
returned in the runtime's internal order, which is deterministic for a given
program but is not insertion order. Do not rely on ``map_keys`` /
``map_values`` ordering across programs.

See also
--------

:doc:`collections <../language/collections>` — the language's built-in array
and map support.