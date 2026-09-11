:orphan:

Concurrency
===========

:experimental:`Experimental` — **partially implemented.**

The Phase 107 concurrency runtime exists and is real:

* **Work-stealing task scheduler** — ``spawn(fn, args...)``,
  ``join(task)``, ``wait_all()``.
* **Channels** — ``channel()``, ``chanSend(ch, v)``, ``chanClose(ch)``,
  ``receive(ch)`` with deterministic close-drain.
* **Actors** — ``actor(handler, state)``, ``actorSend``, ``actorState``,
  ``setActorState``, ``actorStop``.

A validated example lives at ``examples/concurrency/pipeline/main.kark``
(see :doc:`/examples/systems`), with gate tests in
``pkg/runtime/phase107_concurrency_test.go``,
``pkg/codegen/phase107_concurrency_test.go`` and
``pkg/cli/phase107_concurrency_test.go``.

Honest boundaries
-----------------

* The concurrency language surface is **Go-engine only**. The self-hosted
  ``kcc`` engine lexes and parses the keywords (``spawn``, ``send``,
  ``receive``, ``channel``, ``actor``) but kcc codegen parity is deferred.
  Run concurrency programs with ``karkain run --engine go`` or
  ``KARKAIN_ENGINE=go``.
* The runtime is single-process; actors and tasks share one scheduler and
  communicate through channels/mailboxes only.
* No WASM concurrency: the ``wasm32-wasi`` target is single-threaded.

Planned
-------

:planned:`Planned` — the following are intentions, not commitments:

* kcc-engine parity for the concurrency language surface.
* More example categories will be populated in Phase 114 once the parity
  boundary is resolved.

.. seealso::

   :doc:`/language/concurrency` — the language surface reference.
   :doc:`/status/experimental` — why this is labeled experimental.