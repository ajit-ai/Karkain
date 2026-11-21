.. _stdlib-random:

std.random — Random number generation
====================================

:implemented:`Implemented` — importable on both engines.

``std.random`` provides random number generation utilities using system
calls. Platform-dependent behavior may vary.

Importing
---------

.. code-block:: karkain

   import std.random

Module reference
----------------

``random_int_range(min, max)``
   Returns a random integer in [min, max].
   Returns: ``int``

``random_int_n(n)``
   Returns a random integer in [0, n).
   Returns: ``int``

``random_float_01()``
   Returns a random float in [0.0, 1.0).
   Returns: ``float``

``random_bool()``
   Returns a random boolean.
   Returns: ``bool``

``random_choice(items)``
   Returns a random element from a string array.
   Returns: ``string``

``random_shuffle(items)``
   Returns a shuffled copy of a string array.
   Returns: ``string[]``

``random_seed(s)``
   Sets the random seed.
   Returns: ``void``

Example
-------

.. code-block:: karkain

   import std.random

   func main() {
       print(random_int_range(1, 10))   // random 1-10
       print(random_bool())             // true or false
       print(random_choice(["a","b","c"])) // random element
   }