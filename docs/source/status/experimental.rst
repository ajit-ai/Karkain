Experimental
============

Everything below is :experimental:`Experimental` — real and testable, but
subject to change. The honest reason each is experimental is stated next to
it.

.. list-table:: Experimental features
   :widths: 34 66
   :header-rows: 1

   * - Feature
     - Why it is experimental
   * - **Package manager**
     - Local resolver + lockfile + atomic fetch only; there is **no
       registry**. ``pkg add/remove/update`` from a remote registry is a
       documented "not available" error, not a fake success.
   * - **Concurrency on the kcc engine**
     - The Phase 107 runtime works end-to-end on the Go engine; the
       self-hosted ``kcc`` engine lexes and parses the keywords but codegen
       parity is deferred. Run with ``--engine go``.
   * - **Nested block comments**
     - ``/* /* */ */`` nesting works in the Go front end; the self-hosted
       ``kcc`` lexer does not support nesting. Use nesting only when pinned
       to the Go engine.
   * - **Math/Tensor/NPU backend abstraction**
     - ``pkg/backend/*`` (CPU/GPU) and ``pkg/npu/*`` are real Go packages
       but the **language surface is not released**: you cannot write tensor
       or NPU kernels in ``.kark`` today.
   * - **GPU backend** (``pkg/backend/gpu``)
     - Emits WGSL compute shaders internally (Phase 76) but exposes **no
       language surface**. Nothing in the language selects or drives a GPU
       today.
   * - **Quantum backend** (``pkg/npu/*``)
     - A quantum backend abstraction exists internally (Phases 77–78) but
       there is **no language-level quantum syntax** — no ``qreg``, gates or
       circuits are reachable from ``.kark``.

.. note::

   "Experimental" always means *the mechanism is real*. When the mechanism
   cannot be reached from Karkain source, that limitation is called out —
   see the Math/Tensor/NPU, GPU and quantum rows above.

.. seealso::

   :doc:`/status/implemented` — full capabilities that are stable enough to
   rely on.
   :doc:`/status/planned` — what has no code yet.