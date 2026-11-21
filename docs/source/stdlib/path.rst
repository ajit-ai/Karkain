.. _stdlib-path:

std.path — Path manipulation
===========================

:implemented:`Implemented` — importable on both engines.

``std.path`` provides cross-platform path manipulation utilities built on
Karkain's native string support. No libc interop: everything works through
the documented builtin boundary on both engines.

The v1.2.1 gate ``TestPhaseV121_`` executes every documented function on both
engines. A corrective change in that slice removed loops that indexed a path
after the range guard; range checks are now outer ``if`` statements so the
module is portable to the self-hosted engine without relying on short-circuit
evaluation.

Importing
---------

.. code-block:: karkain

   import std.path

Module reference
----------------

``path_join(a, b)``
   Combines two path components with a separator, avoiding double separators.
   Returns: ``string``

``path_basename(path)``
   Returns the last component of a path.
   Returns: ``string``

``path_dirname(path)``
   Returns the directory portion of a path.
   Returns: ``string``

``path_extension(path)``
   Returns the file extension including the dot, or ``""`` if none.
   Returns: ``string``

``path_stem(path)``
   Returns the filename without extension.
   Returns: ``string``

``path_is_absolute(path)``
   Returns ``true`` if the path is absolute.
   Returns: ``bool``

``path_normalize(path)``
   Replaces backslashes with forward slashes and collapses double separators.
   Returns: ``string``

``path_has_extension(path, ext)``
   Returns ``true`` if the path has the given extension.
   Returns: ``bool``

``path_replace_extension(path, new_ext)``
   Changes the file extension of a path.
   Returns: ``string``

Example
-------

.. code-block:: karkain

   import std.path

   func main() {
       print(path_join("a", "b"))              // a/b
       print(path_basename("x/y.txt"))          // y.txt
       print(path_extension("x/y.txt"))        // .txt
       print(path_normalize("a//b\\c"))        // a/b/c
   }