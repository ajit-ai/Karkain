.. _types:

Types
=====

Karkain is statically typed.  The compiler performs type inference, so
**type annotations are optional** on variables, parameters, and return
values.

Primitive types
---------------

``int``
  64-bit signed integer.

``float64``
  64-bit IEEE 754 double.

``bool``
  ``true`` or ``false``.

``string``
  UTF-8 byte string (length-prefixed).

Container types
---------------

``array``
  Homogeneous, dynamically-sized list: ``[1, 2, 3]``.

``map``
  Homogeneous key→value table: ``{"a": 1, "b": 2}``.

Composite types
---------------

``struct``
  Named record with typed fields — see :ref:`structs`.

``enum``
  Tagged union with variants — see :ref:`enums`.

Standard-library wrappers
-------------------------

``Option``
  ``some(value)`` or ``none`` — ``std.option`` module.

``Result``
  ``ok(value)`` or ``err(message)`` — ``std.result`` module.

Type annotations
----------------

Annotations are accepted but not required:

.. code-block:: kark

   let x: int = 42
   let y = 42          // inferred as int

   func double(n: int): int {
       return n * 2
   }
