.. _functions:

Functions
=========

Syntax
------

Functions are declared with ``func``:

.. code-block:: kark

   func double(n: int): int {
       return n * 2
   }

   func greet(name: string): string {
       return "hello " + name
   }

Parameters
----------

* Type annotations on parameters are optional.
* Return-type annotation is optional (inferred from the ``return`` value).

.. code-block:: kark

   func add(a, b) {
       return a + b
   }

Recursion
---------

Functions may call themselves directly:

.. code-block:: kark

   func fib(n: int): int {
       if n <= 1 {
           return n
       }
       return fib(n - 1) + fib(n - 2)
   }

   print(fib(10))        // 55

Forward declarations
--------------------

Functions may be called before their declaration — the resolver resolves
names across the entire file:

.. code-block:: kark

   greet()              // works even though greet is declared below

   func greet() {
       print("hello")
   }

Closures
--------

Closures and ``fn`` literal expressions are **parsed and accepted** by both
engines but are **not lowered** to working code (Phase 101 — documented
boundary).  Using closures in generated output will produce incorrect C
or no output.
