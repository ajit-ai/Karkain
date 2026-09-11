:orphan:

Machine Learning
================

:not-implemented:`Not Yet Implemented`

There is **no machine-learning language surface** in Karkain today. Tensors,
autodiff and neural networks are planned, and a Math/Tensor IR chain exists
internally (Phases 71–78 build ``pkg/math``, ``pkg/tensor`` and the
CPU/GPU/NPU backend abstraction), but none of it is exposed to ``.kark``
programs. Nothing on this page describes an ML API as available.

What exists today
-----------------

* Closed-form / direct numeric fitting works in plain Karkain (e.g. a
  least-squares fit), because that is arithmetic and statistics, not an ML
  framework.
* The Math/Tensor IR and backends are Go-package internals — there is no
  language-level type, keyword, or builtin for them.

Planned
-------

:planned:`Planned` — design intentions, not APIs:

* Tensors, autodiff and neural-network runtime reachable from ``.kark``.
* Examples will be added when the capability is implemented; Phase 114 will
  populate this category with validated programs once the surface exists.

.. seealso::

   :doc:`/examples/ai` — the adjacent planned category.
   :doc:`/status/planned` — the planned-feature register.
   :doc:`/development/architecture` — where the internal IR/backends live.