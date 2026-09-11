:orphan:

Scientific Computing
====================

:not-implemented:`Not Yet Implemented`

There is **no scientific-computing framework** in Karkain today: no tensor
types, no linear-algebra APIs, no statistics module, no plotting/(graphing)
surface. None of it is exposed to ``.kark`` programs. Classical numerics
(Newton iteration, integration, mean/variance) can be written in plain
Karkain — that is ordinary arithmetic, not a scientific-computing API.

Note that the Math/Tensor IR chain (Phases 71–78, ``pkg/math``,
``pkg/tensor``) is Go-package internals: there is no stable language-level
surface for them yet.

Planned
-------

:planned:`Planned` — design intention only. No APIs exist.

* A scientific-computing surface and the example programs that go with it.
* Examples will be added when the capability is implemented; Phase 114 will
  populate this category with validated programs once the surface exists.

.. seealso::

   :doc:`/examples/machine-learning` — the adjacent planned category.
   :doc:`/status/planned` — the planned-feature register.