.. _fundamentals:

Fundamentals
============

Source files
------------

Karkain source files use the ``.kark`` extension.  Every top-level statement
is evaluated in declaration order — there is no implicit ``main()`` entry
point.

.. code-block:: kark

   print("hello")

.. _comments:

Comments
--------

**Line comments** start with ``//`` and run to end-of-line:

.. code-block:: kark

   // this is a line comment
   let x = 1  // inline comment

**Block comments** use ``/* … */``:

.. code-block:: kark

   /* this is a
      block comment */

Nesting ``/* */`` is supported in the Go front end only; the self-hosted
kcc engine does **not** support nested block comments.

Keywords
--------

The following are reserved keywords:

``let``, ``var``, ``func``, ``return``, ``if``, ``else``, ``while``,
``for``, ``in``, ``match``, ``struct``, ``enum``, ``import``, ``public``,
``true``, ``false``, ``nil``, ``spawn``, ``send``, ``receive``,
``channel``, ``actor``, ``actorSend``, ``actorState``, ``setActorState``,
``actorStop``, ``wait_all``, ``break``, ``continue``, ``assert``,
``assert_eq``, ``assert_ne``, ``test``

Statements
----------

A program is a sequence of top-level statements:

.. code-block:: kark

   let greeting = "hello"
   print(greeting)
