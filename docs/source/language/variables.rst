.. _variables:

Variables
=========

Explicit type declarations
--------------------------

Karkain variables are declared with an explicit type. The declaration form
is ``TYPE name = value``:

.. code-block:: kark

   int x = 10
   string name = "karkain"
   double pi = 3.14
   bool active = true
   char grade = 'A'
   int[] nums = [1, 2, 3]
   string[] names = ["a", "b"]
   int[][] matrix = [[1, 2], [3, 4]]

The public primitive types are ``byte`` (8-bit unsigned), ``short`` (16-bit
signed), ``int`` (32-bit signed), ``long`` (64-bit signed), ``float``
(IEEE-754 binary32), ``double`` (IEEE-754 binary64), ``bool``, ``char``
(16-bit unsigned code point) and ``string`` (UTF-8). Array types append
``[]`` and can be nested. ``float64`` remains accepted as a legacy alias;
new code uses ``float`` and ``double``.

Reassignment
------------

Assignments use ``=``. A binding declared with ``const`` is immutable:

.. code-block:: kark

   int counter = 0
   counter = counter + 1
   print(counter)      // 1

   const int MAX = 5
   // MAX = 6          // error: cannot reassign a const binding

**Dead-name analysis** (Borrow Checker, Phase 51) tracks scope end: using a
variable after its owning scope has ended is a compile-time error.

Lexical scope
-------------

Variables are scoped to the block they are declared in:

.. code-block:: kark

   int a = 1
   if a == 1 {
       int b = 2
       print(b)
   }
   // b is not visible here

Shadowing
---------

A binding may shadow an outer name in an inner scope:

.. code-block:: kark

   int x = 1
   if true {
       int x = 2
       print(x)        // 2
   }
   print(x)            // 1
