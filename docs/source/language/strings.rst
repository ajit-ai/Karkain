.. _strings:

Strings
=======

Literal syntax
--------------

Strings are UTF-8 byte sequences delimited by double quotes:

.. code-block:: kark

   let s = "hello, karkain"

Indexing
--------

Individual bytes are accessed by integer index (0-based):

.. code-block:: kark

   let s = "abc"
   print(s[0])           // 97  (ASCII 'a')

Concatenation
-------------

Strings are joined with the ``+`` operator:

.. code-block:: kark

   let first = "hello"
   let second = " world"
   print(first + second) // hello world

Built-in functions
------------------

``len(s)``
  Returns the byte length of the string.

``substr(s, start, end)``
  Returns a substring from byte index *start* to *end* (exclusive).

``trim(s)``
  Strips leading and trailing whitespace.

``split(s, sep)``
  Splits the string into an array on the separator.

``contains(s, sub)``
  Returns ``true`` if *sub* is found in *s*.

``str(value)``
  Converts an ``int`` or ``float64`` to its string representation.

``int(s)``
  Parses a string as an ``int`` (runtime error on invalid input).

Standard library: ``std.string``
--------------------------------

Additional string utilities are available through the standard library:

.. code-block:: kark

   import std.string

   let upper = std.string.to_upper("hello")
   let lower = std.string.to_lower("HELLO")
   print(upper)
   print(lower)
