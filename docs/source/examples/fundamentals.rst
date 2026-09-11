Fundamentals
============

:implemented:`Implemented` — the fundamentals examples are real, checked-in
programs under ``examples/language_foundation/`` that round-trip the
language surface on **both** engines (the Go front end and the self-hosted
``kcc`` engine), producing byte-identical stdout and exit codes.

The corpus has 13 targets: ``01_variables``, ``02_functions``,
``03_recursion``, ``04_arrays``, ``05_strings``, ``06_control_flow``,
``07_structs``, ``08_maps``, ``09_types``, ``10_match``, ``methods``,
``modules/`` and ``application/``. The excerpts below are taken verbatim
from those files.

Run any of them with ``karkain run``; for example
``karkain run examples/language_foundation/01_variables.kark``.

-------------

Variables
---------

``01_variables.kark`` demonstrates ``let`` / ``var`` bindings, literal
types, arithmetic and explicit type annotations:

.. code-block:: kark

   func main() {
       let name = "Karkain"
       var age = 29
       let pi = 3.14159
       let ok = true
       let count = 100

       age = age + 1
       print(name)
       print(age)
       print(pi)
       print(ok)
       print(count * 2)

       let a: int = 5
       let b: float64 = 2.5
       print(a)
       print(b)
   }

.. note::

   ``let`` bindings are immutable; ``var`` bindings may be reassigned.
   Since Phase 112, ``const`` declares an immutable constant on both
   engines. See :doc:`/language/variables` and
   :doc:`/language/constants`.

Functions
---------

``02_functions.kark`` shows parameter passing, returns, string
concatenation and composition:

.. code-block:: kark

   func add(a, b) { return a + b }

   func sub(a, b) { return a - b }

   func describe(x) {
       print("value is " + str(x))
       return x * 10
   }

   func main() {
       print(add(10, 20))
       print(sub(10, 20))
       let r = describe(7)
       print(r)
       print(add(add(1, 2), add(3, 4)))
   }

Recursion
---------

``03_recursion.kark`` covers recursive factorial and Fibonacci:

.. code-block:: kark

   func factorial(n) {
       if (n <= 1) { return 1 }
       return n * factorial(n - 1)
   }

   func fib(n) {
       if (n <= 1) { return n }
       return fib(n - 1) + fib(n - 2)
   }

   func main() {
       print(factorial(10))
       print(factorial(0))
       print(fib(10))
   }

Arrays
------

``04_arrays.kark`` demonstrates index access, mutation, append, slicing and
iteration:

.. code-block:: kark

   func main() {
       let nums = [1, 2, 3, 4, 5]
       print(len(nums))
       print(nums[0])
       nums[4] = 99
       print(nums[4])
       appendArray(nums, 6)
       print(len(nums))

       let grid = [[1, 2], [3, 4]]
       print(grid[1][0])

       let sliced = nums[1:3]
       print(sliced)

       let total = 0
       for v in nums {
           total = total + v
       }
       print(total)
   }

Strings
-------

``05_strings.kark`` shows concatenation, length, indexing, slicing,
comparison and casts:

.. code-block:: kark

   func main() {
       let hello = "Hello"
       let world = "World"
       let greeting = hello + ", " + world + "!"
       print(greeting)
       print(len(greeting))
       print(greeting[0])
       print(greeting[7])
       print(greeting[1:5])
       print(hello == "Hello")
       print(hello != world)
       print(str(42))
       print(int("99"))
       print(str(3.5))
   }

Control flow
------------

``06_control_flow.kark`` covers ``while``, ``for-in``, the C-style ``for``,
``break``/``continue`` and chained conditionals:

.. code-block:: kark

   func main() {
       let i = 0
       while (i < 5) {
           print(i)
           i = i + 1
       }

       for j in [0, 1, 2] {
           print(j)
       }

       let k = 0
       for (; k < 3; k = k + 1) {
           print(k)
       }

       if (3 > 2) { print("big") } else { print("small") }
       if (1 > 2) { print("no") } else if (2 > 1) { print("elif") } else { print("no") }
   }

Structs
-------

``07_structs.kark`` declares record types, constructs literals, and reads
and mutates fields:

.. code-block:: kark

   type Person struct { name string; age int }

   type Point struct { x int; y int }

   func birthday(p) {
       p.age = p.age + 1
       return p
   }

   func main() {
       let alice = Person{name: "Alice", age: 30}
       print(alice.name)
       print(alice.age)
       alice.age = 31
       print(alice.age)

       let aged = birthday(alice)
       print(aged.age)

       let origin = Point{x: 0, y: 0}
       print(origin.x)
       print(origin.y)
   }

Methods (record idiom)
----------------------

Karkain has no receiver-method syntax. The supported idiom, shown in
``methods.kark``, is free functions that take the record as their first
argument:

.. code-block:: kark

   type Counter struct { value int }

   func increment(c, by) {
       c.value = c.value + by
       return c
   }

   func read(c) {
       return c.value
   }

   func main() {
       let c = Counter{value: 0}
       let c2 = increment(c, 5)
       print(read(c2))

       let c3 = increment(c, 10)
       print(read(c3))
       print(c.value)
   }

Modules
-------

User modules are plain sibling-source files in the same directory; the
compiler assembles them in deterministic order. ``modules/main.kark`` calls
``double`` / ``square`` / ``describeMath``, which live in the sibling
``modules/math.kark``:

.. code-block:: kark

   // modules/main.kark
   func main() {
       print(double(21))
       print(square(9))
       print(describeMath())
   }

.. code-block:: kark

   // modules/math.kark
   func double(x) { return 2 * x }

   func square(x) { return x * x }

   func describeMath() { return "module math" }

See :doc:`/language/modules` for the full module model (the ``public``
export modifier, qualified calls and ``import``).

Application layout
------------------

``application/`` is a multi-file application: the ``Account`` type lives in
``model.kark``, numeric helpers in ``math.kark``, and the entry point in
``main.kark``:

.. code-block:: kark

   // application/model.kark
   type Account struct { owner string; balance int }

   func deposit(a, amount) {
       a.balance = a.balance + amount
       return a
   }

   func formatAccount(a) {
       return a.owner + ": " + str(a.balance)
   }

.. code-block:: kark

   // application/main.kark
   func main() {
       let acc = Account{owner: "Ada", balance: 1000}
       print(formatAccount(acc))

       let grown = deposit(acc, 250)
       print(formatAccount(grown))

       print(interest(grown.balance, 5))
       print(interest(1000, 10))
   }

Also in the corpus
==================

* ``08_maps.kark`` — map construction and access.
* ``09_types.kark`` — type annotations and primitives.
* ``10_match.kark`` — ``match`` expressions, including the checked
  multiplication builtin ``mul_checked``.

.. seealso::

   :doc:`/examples/algorithms` — more recursive programs with expected output.
   :doc:`/language/index` — the language guide these examples exercise.