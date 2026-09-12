Scientific Computing
====================

:implemented:`Implemented` — numerical-method examples in ordinary Karkain
arithmetic live under ``examples/12_scientific_computing/`` and run
byte-identically on both engines.

Again with the honest caveat: there is **no scientific-computing framework,
no tensor language surface and no arbitrary-precision type** in Karkain. What
exists is IEEE ``double`` arithmetic through the generated C runtime plus the
implemented array/loop surface. These examples show iterative methods that
avoid the unreliable builtin float helpers where possible — which is both
correct and more instructive.

.. list-table:: examples/12_scientific_computing/
   :widths: 32 68
   :header-rows: 1

   * - File
     - Demonstrates
   * - ``01_sqrt_newton.kark``
     - Newton–Raphson square root iteration (√2 → 1.41421 …)
   * - ``02_numerical_integration.kark``
     - Trapezoid rule converging to 1/3 on ∫₀¹ x² dx
   * - ``03_statistics.kark``
     - Mean, variance and standard deviation over a fixed sample
   * - ``04_matrix_multiply.kark``
     - 3×3 integer matrix product

Newton square root (excerpt)
----------------------------

.. code-block:: kark

   func sqrt_newton(x) {
       let guess = x
       let i = 0
       while (i < 10) {
           guess = (guess + x / guess) / 2.0
           i = i + 1
       }
       return guess
   }

Run:

.. code-block:: console

   $ karkain run examples/12_scientific_computing/01_sqrt_newton.kark

Output format: floats print at 6 significant figures, identically on both
engines (``1.41421``, ``0.333333``, …).

.. seealso::

   :doc:`/examples/machine-learning` — gradient-descent shares these numeric
   idioms.
   :doc:`/reference/types` — the float type used by the runtime.