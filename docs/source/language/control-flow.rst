.. _control-flow:

Control Flow
============

``if`` / ``else``
-----------------

.. code-block:: kark

   let x = 10
   if x > 5 {
       print("big")
   } else {
       print("small")
   }

Parentheses are **not** required around the condition.

``while``
---------

.. code-block:: kark

   var i = 0
   while (i < 5) {
       print(i)
       i = i + 1
   }

Parentheses **are required** around the condition.  Writing
``while i < 5 {`` will be mis-parsed and produce incorrect behaviour.

``for``
-------

C-style three-part loop:

.. code-block:: kark

   for (var i = 0; i < 5; i = i + 1) {
       print(i)
   }

For-in loop over an array:

.. code-block:: kark

   let items = ["a", "b", "c"]
   for item in items {
       print(item)
   }

``match``
---------

``match`` dispatches on a value. Every arm is ``pattern => body``, and the
``=>`` is **required** — omitting it is rejected by both parsers with
``expected '=>' in match arm``. Arms are separated by commas; the trailing comma
is optional.

.. code-block:: kark

   enum Color { Red, Green, Blue }

   let c = Color.Red
   match c {
       Color.Red   => { print("red") },
       Color.Green => { print("green") },
       Color.Blue  => { print("blue") },
       _           => { print("other") },
   }

Enum arms are **tag-only**: the pattern is ``EnumName.Variant`` and the arm runs
on any value of that variant. Payload destructuring — ``Shape.Circle(r)`` — is
not part of the language and is rejected by both parsers.

An arm body is either a block, a ``print``, or a bare expression; a bare
expression arm is a value-producing arm:

.. code-block:: kark

   func tag(s) {
       return match s {
           Shape.Circle => 10,
           Shape.Rect   => 20,
           _            => 0,
       }
   }

``Result`` and ``Option`` arms can **bind** their payload, which is in scope for
the body of that arm only — see :ref:`errors`:

.. code-block:: kark

   match r {
       Ok(n) => { print(n) },
       Err(e) => { print(e) },
   }

``return``
----------

Returns a value from the enclosing function:

.. code-block:: kark

   func abs(n: int): int {
       if n < 0 {
           return -n
       }
       return n
   }

``break`` / ``continue``
-------------------------

Exit or skip the current loop iteration:

.. code-block:: kark

   for (var i = 0; i < 10; i = i + 1) {
       if i == 3 {
           continue
       }
       if i == 7 {
           break
       }
       print(i)
   }
