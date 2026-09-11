Planned
=======

Everything below is :planned:`Planned` — designed or intended, but **not
implemented**. No page here invents APIs, signatures, keywords or examples.
When the capability lands, the corresponding page will move to
Implemented/Experimental and gain a real documentation section.

.. list-table:: Planned features
   :widths: 34 66
   :header-rows: 1

   * - Feature
     - Plan status
   * - **Networking (TCP/UDP, HTTP)**
     - Planned — no socket or HTTP implementation exists. No API surface is
       claimed.
   * - **Databases**
     - Planned — no drivers, no query language, no persistence model beyond
       flat file I/O.
   * - **Web framework**
     - Planned — no HTTP server, routing or templating.
   * - **Advanced package registry**
     - Planned — centralized, authenticated module fetching. Today the
       package manager is local/lockfile only.
   * - **AI/ML language surface**
     - Planned — tensors and inference are roadmap items; the Math/Tensor IR
       is internal and unreleased.
   * - **Quantum language surface**
     - Planned — ``qreg``, ``H``, ``CNOT`` and circuit composition are
       roadmap intentions with **no implemented syntax**.
   * - **GPU/NPU kernel language surface**
     - Planned — kernel/``@target(npu)`` dispatch exists only as an
       attribute + CPU oracle fallback; a full kernel authoring surface is
       not released.
   * - **Advanced diagnostics**
     - Planned — source-level debugging (breakpoints, step) and interactive
       profiling are future work.
   * - **Production readiness**
     - Planned — formal specification freeze, backwards-compatibility
       guarantees and stability commitments.

.. seealso::

   :doc:`/development/roadmap` — where each planned feature sits in time.
   :doc:`/status/experimental` — mechanisms that exist but have not
   stabilized.
   :doc:`/status/index` — the vocabulary: "Planned" means no code exists.