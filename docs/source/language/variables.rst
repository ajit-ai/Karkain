.. _variables:

Variables
=========

``let`` — immutable bindings
-----------------------------

``let`` creates an immutable binding.  Once initialised the name cannot be
reassigned:

.. code-block:: kark

   let x = 10
   x = 20              // error: cannot reassign let binding

**Dead-name analysis** (Borrow Checker, Phase 51) tracks scope end: using a
variable after its owning scope has ended is a compile-time error.

``var`` — mutable bindings
---------------------------

``var`` creates a mutable binding:

.. code-block:: kark

   var counter = 0
   counter = counter + 1
   print(counter)      // 1

Type annotations
----------------

Type annotations are optional:

.. code-block:: kark

   let name: string = "karkain"
   var count: int = 0
   let pi = 3.14       // inferred as float64

Lexical scope
-------------

Variables are scoped to the block they are declared in:

.. code-block:: kark

   let a = 1
   if a == 1 {
       let b = 2
       print(b)
   }
   // b is not visible here

Shadowing
---------

A ``let`` binding may shadow an outer name in an inner scope:

.. code-block:: kark

   let x = 1
   if true {
       let x = 2
       print(x)        // 2
   }
   print(x)            // 1
