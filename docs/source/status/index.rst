Status
======

Every feature documented on this site carries exactly one of six status
labels. The vocabulary is defined here so you always know whether a section
describes something real today or something planned.

.. contents:: The six levels
   :local:
   :depth: 1

The six-level model
===================

.. list-table:: Status vocabulary
   :widths: 24 76
   :header-rows: 1

   * - Label
     - Meaning
   * - :stable:`Stable`
     - Frozen and production-safe. Behavior is guaranteed and will not
       change without a major version bump.
   * - :implemented:`Implemented`
     - Real and tested. Verifyable through gates (unit tests, parity tests,
       E2E CLI tests). May still evolve.
   * - :experimental:`Experimental`
     - Real but subject to change. The capability exists and is testable,
       but the surface is not yet guaranteed.
   * - :developer-preview:`Developer Preview`
     - Functional but incomplete. Real partial capability with documented
       gaps.
   * - :planned:`Planned`
     - Designed but not implemented. May appear as roadmap entries or
       documented intentions; **no code exists**.
   * - :not-implemented:`Not Yet Implemented`
     - An identified need with no code and no design contract. Nothing is
       presented as available.

Honesty rules
=============

* **Never** describe a hypothetical feature as available. A feature is
  ``Implemented`` only when it is exercised by a gate test through the real
  pipeline.
* ``Planned`` pages must not invent APIs, signatures or examples.
* ``Experimental`` pages must state exactly why they are experimental (e.g.
  Go-engine only, wasmtime-gated, kcc parity deferred).
* A category placeholder (no examples yet) is labeled
  ``Not Yet Implemented``, not silently omitted.

The catalog pages
=================

.. toctree::
   :maxdepth: 2

   beta
   scope
   implemented
   experimental
   planned
   compatibility
   migration-beta1
   rc-checklist

.. note::

   The overall project status is **Karkain 1.0.0 (Stable)** — see
   :doc:`/status/beta` for the Beta 1 capability assessment it builds on.
   Individual
   features are labeled per the model above; the ``Developer Preview`` label
   in the vocabulary remains a *feature maturity level* (functional but
   incomplete) rather than the project status.

.. rubric:: Practical status pages

* :doc:`scope` — the authoritative release-candidate status matrix
  (Stable / Experimental / Planned / Known Limitation / RC Blocker).
* :doc:`compatibility` — the compatibility guarantees attached to each
  bucket.
* :doc:`rc-checklist` — the release-candidate acceptance checklist.
* :doc:`migration-beta1` — what changed between Developer Preview and Beta 1.

.. seealso::

   :doc:`/examples/index` — category pages using this vocabulary.
   :doc:`/development/roadmap` — where the planned surface fits over time.