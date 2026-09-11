:orphan:

Quantum
=======

:not-implemented:`Not Yet Implemented`

There is **no language-level quantum syntax** in Karkain today. A quantum
backend abstraction exists internally (Phases 77–78 build the NPU/quantum
adapter abstraction, and Go codegen contains quantum simulation
internals), but none of it is exposed to ``.kark`` programs through a
stable, released surface. The ``@target(npu)`` attribute routes compute to
vendor NPU adapters or the CPU oracle fallback — it is not a quantum
instruction set.

Honest statement
----------------

The roadmap documents a quantum vision (first-class quantum primitives,
circuits, error correction), but writing programs against it is **not
possible with the current toolchain**. Nothing on this page presents a
hypothetical quantum API as available.

Planned
-------

:planned:`Planned` — design intentions named in the roadmap, with **no
implemented APIs**:

* Circuit primitives such as ``qreg`` (a quantum register type), ``H``
  (Hadamard), ``CNOT``, ``measure`` and circuit composition.
* Examples will be added only when the language surface is implemented and
  released.

.. seealso::

   :doc:`/status/planned` — the planned-feature register.
   :doc:`/status/experimental` — the NPU/quantum backend status.