.. _syntax:

================
Syntax Reference
================

Lexical structure
=================

Identifiers
-----------

::

   [a-zA-Z_][a-zA-Z0-9_]*

Identifiers must start with a letter or underscore and may contain letters,
digits and underscores. Reserved words (see :doc:`keywords`) cannot be used
as identifiers.

Literals
--------

.. list-table::
   :widths: 20 40 40
   :header-rows: 1

   * - Type
     - Syntax
     - Examples
   * - Integer
     - Decimal or ``0x``-prefixed hexadecimal
     - ``42``, ``0x1F``
   * - Float
     - Decimal with ``.``
     - ``3.14``, ``0.5``
   * - String
     - Double-quoted, supports ``\n``, ``\t``, ``\\``, ``\"``
     - ``"hello"``, ``"line\n"``
   * - Boolean
     - Keyword literals
     - ``true``, ``false``

Comments
--------

Line comments use ``//``:

.. code-block:: karkain

   // this is a line comment

Block comments use ``/* */`` and support nesting — but **only on the Go
engine**:

.. code-block:: karkain

   /* outer
      /* inner */
   outer continues */

The self-hosted ``kcc`` engine does not support nested block comments.

Keywords and operators
----------------------

See :doc:`keywords` and :doc:`operators` for the complete lists.

Statement syntax
================

Variable declarations
---------------------

.. code-block:: karkain

   let x = 5          // inferred int
   let y: float64 = 3.14   // explicit type
   var count = 0       // mutable

Type inference derives the type from the right-hand side. Explicit type
annotations follow the colon.

Function declarations
---------------------

.. code-block:: karkain

   func add(a int, b int) int {
       return a + b
   }

Struct declarations
-------------------

.. code-block:: karkain

   struct Point {
       x int
       y int
   }

Enum declarations
-----------------

.. code-block:: karkain

   enum Color { red, green, blue }

Control flow
------------

``if`` — parenthesized condition is the documented form; a bare condition
is accepted identically by both engines:

.. code-block:: karkain

    if x > 0 {
        println("positive")
    } else {
        println("non-positive")
    }

``while`` — parenthesized condition is the documented form; a bare
condition is accepted identically by both engines (aligned Phase 117):

.. code-block:: karkain

    while (i < 10) {
        i = i + 1
    }

``for`` — C-style three-part loop:

.. code-block:: karkain

   for (let i = 0; i < 3; i = i + 1) {
       println(i)
   }

``for-in`` — iterate over a collection:

.. code-block:: karkain

   for item in myArray {
       println(item)
   }

``match`` — pattern matching. Each arm is ``pattern => expr`` (the
``=>`` separator is required; arms are comma-separated, trailing comma
optional):

.. code-block:: karkain

    match x {
        1 => println("one"),
        2 => println("two"),
        _ => println("other"),
    }

Enum patterns match on the variant tag only (``Color.Red``); payload
destructuring (``Shape.Circle(r)``) is rejected by both parsers.
Unknown variants and undeclared enum types are checker errors
(``K113`` / ``K106``).

``return``, ``break``, ``continue`` — standard control-flow keywords.
``break`` and ``continue`` are only valid inside a loop body.

Expression syntax
=================

Binary expressions
------------------

::

   expr op expr

Operands are fully evaluated before the operator is applied. See
:doc:`operators` for the complete table.

Unary expressions
-----------------

::

   !expr     // logical negation

Function calls
--------------

::

   callee(arg1, arg2, ...)

Index access
------------

::

   collection[index]

Member access
-------------

::

    record.field
