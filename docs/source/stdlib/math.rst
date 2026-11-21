.. _stdlib-math:

std.math — Math functions
=========================

:implemented:`Implemented` — importable on both engines.

``std.math`` provides basic math utilities: absolute value, square root,
power, min/max, clamp, gcd, lcm, factorial, and parity checks. Rewritten
with canonical Karkain syntax (increment 156).

Importing
---------

.. code-block:: karkain

   import std.math

Module reference
----------------

``math_abs(x)``
   Returns the absolute value of an integer.
   Returns: ``int``

``math_sqrt(x)``
   Returns the square root of a float.
   Returns: ``float``

``math_pow(x, y)``
   Returns x raised to the power y.
   Returns: ``float``

``math_min(a, b)`` / ``math_max(a, b)``
   Returns the smaller/larger of two floats.
   Returns: ``float``

``math_min_int(a, b)`` / ``math_max_int(a, b)``
   Returns the smaller/larger of two ints.
   Returns: ``int``

``math_clamp(x, lo, hi)``
   Restricts a float to a given range.
   Returns: ``float``

``math_clamp_int(x, lo, hi)``
   Restricts an int to a given range.
   Returns: ``int``

``math_lerp(a, b, t)``
   Linear interpolation between a and b.
   Returns: ``float``

``math_gcd(a, b)``
   Returns the greatest common divisor.
   Returns: ``int``

``math_lcm(a, b)``
   Returns the least common multiple.
   Returns: ``int``

``math_is_even(n)`` / ``math_is_odd(n)``
   Returns true if n is even/odd.
   Returns: ``bool``

``math_factorial(n)``
   Returns n! (factorial).
   Returns: ``int``

Example
-------

.. code-block:: karkain

   import std.math

   func main() {
       print(math_sqrt(16.0))     // 4
       print(math_abs(-5.0))      // 5
       print(math_gcd(12, 8))     // 4
       print(math_factorial(5))   // 120
       print(math_clamp(15.0, 0.0, 10.0))  // 10
   }