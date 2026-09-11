.. _stdlib-strings:

std.string — String utilities
=============================

:implemented:`Implemented` — importable on both engines, byte-identical.

``std.string`` provides string manipulation utilities built on Karkain's
native string support. Karkain strings are UTF-8 byte strings: ``len()``
returns the byte length and ``s[i]`` returns the single byte at ``i`` as a
1-character string. The ASCII case-mapping helpers operate byte-wise
(table-based, no character arithmetic) and therefore work identically on both
engines.

All functions are implemented in pure Karkain with untyped parameters and are
identical on the Go and ``kcc`` engines.

Importing
=========

.. code-block:: karkain

   import std.string

Module reference
================

``str_len(s)``
   Returns the length (number of bytes) of ``s``. Delegates to ``len()``.
   Returns: ``int``

``str_empty(s)``
   Returns ``true`` if ``s`` has zero length.
   Returns: ``bool``

``str_concat(a, b)``
   Returns the concatenation of ``a`` and ``b``. Uses the ``+`` operator.
   Returns: ``string``

``str_repeat(s, n)``
   Returns ``s`` repeated ``n`` times (``"ab"`` × 3 → ``"ababab"``). An empty
   string or ``n <= 0`` yields ``""``.
   Returns: ``string``

``str_starts_with(s, prefix)``
   Returns ``true`` if ``s`` starts with ``prefix``.
   Returns: ``bool``

``str_ends_with(s, suffix)``
   Returns ``true`` if ``s`` ends with ``suffix``.
   Returns: ``bool``

``str_contains(s, sub)``
   Returns ``true`` if ``s`` contains ``sub``. Delegates to ``contains()``.
   Returns: ``bool``

``str_index_of(s, sub)``
   Returns the byte index of the first occurrence of ``sub`` in ``s``, or
   ``-1`` if not found. An empty ``sub`` matches at index ``0``.
   Returns: ``int``

``str_last_index_of(s, sub)``
   Returns the byte index of the last occurrence of ``sub`` in ``s``, or
   ``-1`` if not found. An empty ``sub`` returns ``len(s)``.
   Returns: ``int``

``str_sub(s, start, end)``
   Returns the substring from byte index ``start`` up to (but not including)
   ``end``, i.e. ``s[start:end]``.
   Returns: ``string``

``str_slice(s, start, length)``
   Returns the substring starting at byte index ``start`` with the given
   ``length``. Delegates to ``substr()``.
   Returns: ``string``

``str_trim(s)``
   Removes leading and trailing whitespace. Delegates to ``trim()``.
   Returns: ``string``

``str_split(s, delimiter)``
   Splits ``s`` into an array of strings on ``delimiter``. Delegates to
   ``split()``.
   Returns: ``string[]``

``str_to_upper(s)``
   Converts ASCII lowercase letters to uppercase. Non-ASCII and non-letter
   bytes are unchanged.
   Returns: ``string``

``str_to_lower(s)``
   Converts ASCII uppercase letters to lowercase. Non-ASCII and non-letter
   bytes are unchanged.
   Returns: ``string``

``str_replace(s, old, new)``
   Returns a new string with all occurrences of ``old`` replaced by ``new``.
   Returns: ``string``

``str_reverse(s)``
   Returns a new string with the bytes of ``s`` in reverse order.
   Returns: ``string``

``str_char_at(s, index)``
   Returns the single character at byte index ``index`` as a 1-character
   string, or ``""`` if ``index`` is out of range.
   Returns: ``string``

``str_to_int(s)``
   Converts a string to an integer using the builtin ``int()``. Raises a
   runtime error on invalid input.
   Returns: ``int``

``str_from_int(n)``
   Converts an integer to its decimal string representation using the
   builtin ``str()``.
   Returns: ``string``

``str_from_float(f)``
   Converts a float to its string representation using the builtin ``str()``.
   Returns: ``string``

``str_is_empty(s)``
   Returns ``true`` if ``s`` has zero length (alias of ``str_empty``).
   Returns: ``bool``

``str_count(s, sub)``
   Returns the number of *non-overlapping* occurrences of ``sub`` in ``s``.
   An empty ``sub`` returns ``0``.
   Returns: ``int``

``str_join(arr, sep)``
   Joins the elements of the string array ``arr`` with separator ``sep``
   between them.
   Returns: ``string``

``str_trim_left(s)``
   Removes leading whitespace (space, tab, newline, carriage return).
   Returns: ``string``

``str_trim_right(s)``
   Removes trailing whitespace (space, tab, newline, carriage return).
   Returns: ``string``

Example
=======

.. code-block:: karkain

   import std.string

   func main() {
       let name = "karkain"
       println(str_to_upper(name))              // KARKAIN
       println(str_concat("hello", " world"))   // hello world
       let parts = str_split("a,b,c", ",")
       println(len(parts))                      // 3
       println(str_join(parts, "-"))            // a-b-c
       println(str_repeat("ab", 3))             // ababab
   }

   $ karkain run example.kark
   KARKAIN
   hello world
   3
   a-b-c
   ababab

See also
========

:doc:`strings <../language/strings>` — the language's built-in string support.