.. _collections:

Collections
===========

Arrays
------

Literal syntax — a comma-separated list inside square brackets:

.. code-block:: kark

   let nums = [1, 2, 3, 4, 5]
   let empty: array = []

Common operations
-----------------

``push(arr, item)``
  Appends *item* to the end of the array.

``len(arr)``
  Returns the number of elements.

``arr[i]``
  Returns the element at index *i* (0-based).

.. code-block:: kark

   let items = [10, 20, 30]
   push(items, 40)
   print(len(items))       // 4
   print(items[2])         // 30

Runtime bounds checking
-----------------------

Indexing an array outside its valid range produces a runtime error:

.. code-block:: text

   runtime error: index out of range at main.kark:3

Maps
----

Literal syntax — key/value pairs inside ``{}``:

.. code-block:: kark

   let scores = {"alice": 100, "bob": 85}
   print(scores["alice"])  // 100

Common operations
-----------------

``scores[key]``
  Returns the value for *key*.

``hasKey(map, key)``
  Returns ``true`` if the key exists.

.. code-block:: kark

   let m = {"x": 1}
   print(hasKey(m, "x"))   // true
   print(hasKey(m, "y"))   // false

Standard library: ``std.collections``
--------------------------------------

Additional collection utilities are available through the standard library:

.. code-block:: kark

   import std.collections

   let m = {"a": 1, "b": 2}
   let keys = std.collections.keys_of(m)
   print(len(keys))         // 2
