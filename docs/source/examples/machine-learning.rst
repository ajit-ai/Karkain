Machine Learning
================

:implemented:`Implemented` — as hand-rolled numeric examples in ordinary
Karkain arithmetic under ``examples/10-machine-learning/``, byte-identical on
both engines.

The honest label matters here: Karkain has **no tensor types, no autodiff, no
ML framework callable from ``.kark``**. The matrix/vector machinery built in
Phases 71–78 exists as compiler/backend infrastructure, not as a language
surface. These examples instead implement two classic learning algorithms
with plain floats, arrays and loops — which is exactly the kind of program a
Beta 1 user should be able to write today.

.. list-table:: examples/10-machine-learning/
   :widths: 30 70
   :header-rows: 1

   * - File
     - Demonstrates
   * - ``01_linear_regression.kark``
     - Closed-form ordinary least squares over a synthetic point set
   * - ``02_gradient_descent.kark``
     - Iterative loss minimization over a convex 1-D surface

Linear regression (excerpt)
---------------------------

.. code-block:: kark

   func main() {
       let xs = [1.0, 2.0, 3.0, 4.0]
       let ys = [2.0, 4.0, 6.0, 8.0]
       ...
   }

Legend: ``0.9 / 1.3 / 6.7`` pins the learned slope, intercept and a
prediction for the synthetic fit.

Run:

.. code-block:: console

   $ karkain run examples/10-machine-learning/01_linear_regression.kark

.. seealso::

   :doc:`/examples/ai` — nearest-neighbour and linear-classifier geometry.
   :doc:`/development/roadmap` — where a real ML surface may land.