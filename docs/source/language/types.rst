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

Language-provided carriers
--------------------------

These are part of the language itself, not standard-library modules. They are
written **bare**, without type arguments:

``Option``
  ``Some(v)`` or ``None`` — presence/absence (no null). See :ref:`errors`.

``Result``
  ``Ok(v)`` or ``Err(e)`` — fallible operations (no exceptions). See
  :ref:`errors`.

``int[]`` / ``string[]``
  Element-typed arrays, used for parameter and return annotations (for example
  ``func range(start: int, end: int) -> int[]``). An unannotated ``let`` infers
  the element type from the literal.

Type annotations
----------------

Annotations are accepted but not required:

.. code-block:: kark

   let x: int = 42
   let y = 42          // inferred as int

   func double(n: int): int {
       return n * 2
   }
