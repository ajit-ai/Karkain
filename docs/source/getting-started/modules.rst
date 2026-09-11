=======
Modules
=======

Karkain programs are split into modules and pulled together with ``import``.
A module is a source file; ``import <name>`` makes its declarations
available, and functions can be called by their qualified name.

Importable standard modules
===========================

Six ``std.*`` modules are implemented and importable through the normal
toolchain on both engines:

- ``std.string`` — string manipulation
- ``std.collections`` — array helpers
- ``std.io`` — file and console I/O
- ``std.encoding`` — hex and Base64 codecs
- ``std.crypto`` — SHA-256 / SHA-512
- ``std.testing`` — assertion and check helpers

Importing is syntactic: write ``import std.string`` in the source.

.. code-block:: karkain

    import std.string

    func main() {
        println(str_to_upper("hello"))       // HELLO
        println(std.string.str_to_upper("hi")) // HI — qualified call
    }

Qualified calls
===============

Imported functions can be called unqualified (``str_to_upper(...)``) or with
a module qualifier (``std.string.str_to_upper(...)``). Qualified calls
resolve against the imported module's definition and are checked for
existence.

The ``public`` modifier
=======================

By default a function is private to its own file. Mark a function
``public`` to make it part of the module's export set so other modules may
qualify-call it:

.. code-block:: karkain

    // utils.kark (in lib/)
    public func twice(n) {
        return n * 2
    }

    func helper() {        // file-scoped: not callable from other modules
        return 1
    }

.. code-block:: karkain

    // main.kark (in src/)
    import utils

    func main() {
        println(utils.twice(21))   // 42
    }

Calling a private cross-module function, or calling without importing, is a
resolution error (exit code ``3``). The ``stdlib`` modules are exempt from
the private rule — their whole surface is the framework API.

Known limitation
================

The ``karkain test`` driver does not yet support ``import`` statements
(Phase 96 boundary); the ``stdlib`` modules are validated through
``karkain run`` instead.

.. seealso::

   :doc:`project-layout` — where modules live in a project.
   :doc:`dependencies` — fetching external modules.