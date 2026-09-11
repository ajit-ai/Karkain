.. _enums:

Enums
=====

Declaration
-----------

.. code-block:: kark

   enum Color {
       Red,
       Green,
       Blue
   }

Variants may carry associated data (tagged unions):

.. code-block:: kark

   enum Shape {
       Circle(float64),
       Rectangle(float64, float64)
   }

Instantiation
-------------

.. code-block:: kark

   let c = Color.Red
   let r = Shape.Rectangle(10.0, 20.0)

Pattern matching
----------------

Use ``match`` to dispatch on variants:

.. code-block:: kark

   func describe(s): string {
       match s {
           Shape.Circle(r)        { return "circle with radius " + str(r) }
           Shape.Rectangle(w, h)  { return "rectangle " + str(w) + "x" + str(h) }
       }
   }

   let shape = Shape.Circle(5.0)
   print(describe(shape))

Non-exhaustive ``match`` on enums is allowed at runtime — unmatched
variants fall through without action.
