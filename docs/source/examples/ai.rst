AI
==

:implemented:`Implemented` — in the honest sense: AI *demonstration examples*
written in ordinary Karkain arithmetic exist under ``examples/09-ai/`` and run
byte-identically on both engines.

There is **no ``std.ai`` module and no AI/ML framework surface** in the
language. These examples show classic ML *geometries* (distance, weighted
votes, linear decision boundaries) implemented by hand with plain arrays,
maps and arithmetic — deliberately, so the language itself can be exercised
without pretending a framework exists. See :doc:`/status/implemented` for how
the status vocabulary is used.

.. list-table:: examples/09-ai/
   :widths: 30 70
   :header-rows: 1

   * - File
     - Demonstrates
   * - ``01_nearest_neighbor.kark``
     - Manhattan-distance nearest-neighbour classification by hand
   * - ``02_linear_classifier.kark``
     - Perceptron-style weight updates and a decision boundary

K-nearest neighbour (excerpt)
-----------------------------

.. code-block:: kark

   func distance(a, b) {
       return abs(a[0] - b[0]) + abs(a[1] - b[1])
   }
   ...

Run:

.. code-block:: console

   $ karkain run examples/09-ai/01_nearest_neighbor.kark

.. seealso::

   :doc:`/examples/machine-learning` — the adjacent hand-rolled ML category.
   :doc:`/status/planned` — the not-yet-implemented AI framework roadmap.