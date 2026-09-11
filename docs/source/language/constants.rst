.. _constants:

Constants
=========

.. versionadded:: Phase 112

The ``const`` keyword declares a **compile-time constant**.

Behaviour
---------

* **Function-scoped** — valid only inside a function body (top-level ``const``
  is not yet supported).
* **No reassignment** — attempting to assign to a ``const`` name is a
  compile-time error (diagnostic ``K113``).
* **Type annotation is optional** — the compiler infers the type from the
  initializer expression.

.. code-block:: kark

   func area(radius: float64): float64 {
       const pi = 3.141592653589793
       return pi * radius * radius
   }

.. code-block:: kark

   func greet(): string {
       const msg = "hello"
       msg = "bye"        // error [K113]: cannot reassign constant
   }
