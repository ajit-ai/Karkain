=================
Compute targets
=================

Phase 124 adds the compute-target model: a vendor-neutral catalog of
accelerator executors (CPU, SIMD, WASM, GPU, NPU, quantum) that the compiler
target selection/dispatch layer and the KIR lowering boundary can reason
about. The model lives in the existing ``pkg/target`` module (alongside the
Phase 111 triple/ABI model) and deliberately reuses three already-existing
abstractions instead of inventing new ones:

* the ``Target`` identity model (``pkg/target``) for host triples,
* the tensor operation vocabulary (``pkg/tensor``) for accelerator ops,
* the KIR v1 text classes (``src/compiler/kir.kark``) for the lowering
  boundary.

There is **no second IR**, no LLVM IR, no vendor SDK and no hardware
dependency anywhere in the model. Compute targets express what a program may
*express* when lowered to a target, never how a vendor implements it.

Catalog
=======

``karkain target`` lists the registered compute targets with an honest
readiness label:

.. list-table:: Compute targets
   :widths: 22 12 66
   :header-rows: 1

   * - Name
     - Maturity
     - Description
   * - ``cpu``
     - implemented
     - Reference scalar CPU executor (the default lowering surface)
   * - ``simd``
     - implemented
     - CPU executor with lane-vector arithmetic (Phase 106 ``@simd_*`` surface)
   * - ``wasm32-wasi``
     - experimental
     - WebAssembly/WASI executor (Phase 108 Karkain-owned wasm backend)
   * - ``gpu-experimental``
     - experimental
     - Data-parallel kernel executor over tensor/matrix ops (model only)
   * - ``npu-experimental``
     - experimental
     - Single-invocation neural inference over tensor/matrix ops (model only)
   * - ``quantum-experimental``
     - research
     - Quantum circuit executor over gate primitives and measurement
       (spec model only)

Capabilities and KIR classes
============================

Every compute target declares two vendor-neutral sets:

* **Capabilities** — coarse execution features (``float_arithmetic``,
  ``tensor_ops``, ``matrix_ops``, ``kernel_launch``, ``async_execution``,
  ``host_device_memory``, ``device_memory``, ``function_calls``, ``io``,
  ``quantum_gates``, ``quantum_measurement``, ``allocation``,
  ``simd_vector_ops``, and friends).
* **Accepted KIR v1 classes** — the subset of ``src/compiler/kir.kark``
  statement/expression classes the target can lower (``func``, ``kernel``,
  ``block``, ``let``, ``alloc``, ``measure``, ...).

The full view of either set for any registered target is available through:

.. code-block:: console

    $ karkain target npu-experimental
    Compute target: npu-experimental
    Family:         npu
    Maturity:       experimental
    Memory model:   device-only (single-invocation inference graph)
    Execution model:  synchronous single-invocation inference
    Capabilities:   allocation, device_memory, float_arithmetic, ...
    KIR classes:    alloc, block, break, case, const, ...
    Tensor ops:     add, broadcast, concat, ...

KIR lowering boundary
=====================

`ComputeTarget.Lower` validates KIR v1 text (``karkain kir`` output) against a
target's accepted classes and capability requirements. It returns a
deterministic ``LowerPlan`` on success or an ``UnsupportedError`` naming the
first operation the target cannot lower, always of the form::

    error[K124] target '<name>' cannot lower '<op>' (<class>); <hint>

The boundary is deliberately strict and honest — accelerator targets reject
host surfaces:

* ``gpu-experimental`` rejects ``print`` (host I/O happens on the host).
* ``npu-experimental`` rejects ``func`` (single-invocation inference has no
  function calls).
* ``quantum-experimental`` rejects ``func`` and ``print``; it accepts the
  ``measure`` class and claims the quantum gate/measurement capabilities.

Nothing in the Phase 124 model drives real accelerator hardware. The
``experimental``/``research`` labels are statements about the Karkain-side
surface; execution backends remain the Phase 76–78 Go packages documented
in :doc:`/status/experimental`.

.. seealso::

   :doc:`index` — the target command and cross-compilation model.
   :doc:`/status/experimental` — why the compute targets are experimental.