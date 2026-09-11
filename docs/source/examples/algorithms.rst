Algorithms
==========

:implemented:`Implemented` — the algorithm examples are real, runnable
programs under ``examples/algorithms/``. The corpus contains 21 programs
(``absolute_value``, ``bellman``, ``bfs``, ``binary``, ``bubble``, ``dfs``,
``dijkstra``, ``factorial``, ``fibonacci``, ``gcd``, ``heap``, ``kadane``,
``knapsack``, ``lcm``, ``levenshtein``, ``linear``, ``merge``, ``power``,
``quick``, ``sieve``, ``ternary``).

This page documents the classical recursion/iteration set: **factorial**,
**fibonacci**, **gcd**, **lcm**, **power** and **sieve**. All outputs below
were captured by running each program through ``karkain run`` on the default
engine.

-------------

Fibonacci
---------

``examples/algorithms/fibonacci/main.kark`` — naive recursive Fibonacci:

.. code-block:: kark

   func fib(n) {
       if (n <= 1) {
           return n
       }
       return fib(n - 1) + fib(n - 2)
   }

   func main() {
       print("Fibonacci(10):")
       print(fib(10))
       print("Fibonacci(20):")
       print(fib(20))
   }

Expected output (verified):

.. code-block:: text

   Fibonacci(10):
   55
   Fibonacci(20):
   6765

Factorial
---------

``examples/algorithms/factorial/main.kark``:

.. code-block:: kark

   func factorial(n) {
       if (n <= 1) {
           return 1
       }
       return n * factorial(n - 1)
   }

   func main() {
       print("Factorial of 5:")
       print(factorial(5))
       print("Factorial of 10:")
       print(factorial(10))
   }

Expected output (verified):

.. code-block:: text

   Factorial of 5:
   120
   Factorial of 10:
   3628800

GCD
---

``examples/algorithms/gcd/main.kark`` — Euclid's algorithm:

.. code-block:: kark

   func gcd(a, b) {
       if (b == 0) {
           return a
       }
       return gcd(b, a % b)
   }

   func main() {
       print("GCD(48, 36):")
       print(gcd(48, 36))
       print("GCD(270, 192):")
       print(gcd(270, 192))
   }

Expected output (verified):

.. code-block:: text

   GCD(48, 36):
   12
   GCD(270, 192):
   6

LCM
---

``examples/algorithms/lcm/main.kark`` — least common multiple via ``gcd``:

.. code-block:: kark

   func gcd(a, b) {
       if (b == 0) {
           return a
       }
       return gcd(b, a % b)
   }

   func lcm(a, b) {
       return (a * b) / gcd(a, b)
   }

   func main() {
       print("LCM(4, 6):")
       print(lcm(4, 6))
       print("LCM(21, 6):")
       print(lcm(21, 6))
   }

Expected output (verified):

.. code-block:: text

   LCM(4, 6):
   12
   LCM(21, 6):
   42

Power
-----

``examples/algorithms/power/main.kark`` — iterative exponentiation:

.. code-block:: kark

   func power(base, exp) {
       let result = 1
       let i = 0
       while (i < exp) {
           result = result * base
           i = i + 1
       }
       return result
   }

   func main() {
       print("2^10:")
       print(power(2, 10))
       print("3^4:")
       print(power(3, 4))
   }

Expected output (verified):

.. code-block:: text

   2^10:
   1024
   3^4:
   81

Sieve
-----

``examples/algorithms/sieve/main.kark`` — trial-division prime collection:

.. code-block:: kark

   func is_prime(n) {
       if (n < 2) {
           return 0
       }
       let d = 2
       while (d * d <= n) {
           if (n % d == 0) {
               return 0
           }
           d = d + 1
       }
       return 1
   }

   func main() {
       let limit = 30
       let primes = []
       let n = 2
       while (n <= limit) {
           if (is_prime(n)) {
               push(primes, n)
           }
           n = n + 1
       }
       print("Primes up to 30:")
       let i = 0
       while (i < len(primes)) {
           print(primes[i])
           i = i + 1
       }
   }

Expected output (verified):

.. code-block:: text

   Primes up to 30:
   2
   3
   5
   7
   11
   13
   17
   19
   23
   29

Note on arrays
==============

Arrays are passed **by value** into functions. ``push(primes, n)`` above
mutates a variable in the same scope; to grow an array inside a helper and
see the result in the caller, capture the returned array (``arr =
push(arr, v)``). This is a documented behavior, not a defect — see the
showcase notes in ``examples/showcase/README.md``.

The graph-algorithm programs (``bfs``, ``dfs``, ``dijkstra``, ``bellman``)
and the sorting programs (``bubble``, ``merge``, ``quick``, ``heap``) follow
the same conventions and are validated by the same gates.

.. seealso::

   :doc:`/examples/fundamentals` — the language foundation behind these
   programs.
   :doc:`/language/index` — the language guide.