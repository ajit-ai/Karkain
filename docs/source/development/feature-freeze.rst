=================================================
Feature-Freeze Policy (Beta / Release-Candidate)
=================================================

During release-candidate preparation, Karkain operates under a temporary
feature freeze. The goal is to harden the Beta stable core by removing
unnecessary churn, while still fixing genuine problems.

.. contents:: Sections
   :local:
   :depth: 1

Scope of the freeze
===================

The freeze covers the **Beta stable core** and the release-candidate
readiness surface only. A freeze does not prevent work on the roadmap;
it controls what is allowed into the stable line before RC.

What is **not** allowed (during freeze)
---------------------------------------

* No new major language features.
* No unnecessary syntax changes.
* No broad standard-library redesign.
* No compiler architecture rewrite.
* No speculative refactoring that could change semantics.
* No new ``Implemented`` / ``Experimental`` items added to the stable core
  without a documented justification and a dedicated gate.

What **is** allowed
-------------------

* Bug fixes that restore documented behavior.
* Security fixes (see ``SECURITY.md``).
* Documentation corrections.
* Release tooling and packaging fixes.
* Developer-experience fixes (clarity of diagnostics, correctness of
  ``explain`` codes, clean-up of help text).
* Compatibility fixes (engine parity, target-model correctness, workspace
  dependency resolution).
* Deterministic-correctness fixes (correctness of checked operations,
  precision of ``runtime error:`` diagnostics).

Gate discipline during freeze
=============================

Any change allowed by this policy must still:

1. Keep the Phase 117 regression baselines green (conformance 59/59,
   probes 11/11, example parity 49/49, verify-examples 49 pass / 0 fail).
2. Pass the Phase 118 release-candidate gate
   (``pkg/cli/phase118_rc_test.go``).
3. Pass strict Sphinx + linkcheck.
4. Not introduce new ``*.exe``, ``*.c`` / ``*.c23`` or
   ``docs/build`` pollution.

How long does the freeze last?
==============================

The freeze lasts from the start of RC preparation through the RC decision.
If RC is deferred, the freeze continues until the next RC decision point or
until 1.0 enters active development, whichever comes first.

Unfreezing
==========

When the project moves into 1.0 active development, the freeze is lifted
and the new development conventions are documented in
:doc:`/development/roadmap` and the updated release notes.

.. seealso::

   :doc:`/status/scope` — the release-candidate scope.
   :doc:`/status/compatibility` — the compatibility policy during freeze.
   :doc:`/development/release` — the release engineering process.
   :doc:`/status/rc-checklist` — the acceptance checklist that gates RC
   declaration.