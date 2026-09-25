====================
Compatibility Policy
====================

Karkain 1.1.0 defines what external developers may rely on between
releases. This policy is governed by the formal SemVer policy
(:doc:`/development/semver-policy`) and the version-wise development plan
(``docs/audit/KARKAIN-VERSION-PLAN.md``).

.. contents:: Sections
   :local:
   :depth: 1

Guarantee levels
================

.. list-table:: Guarantee levels
   :widths: 22 78
   :header-rows: 1

   * - Level
     - Policy
   * - Stable Core
     - Reliable. Changes are deliberate, documented in
       :doc:`/release-notes`, and accompanied by a regression test. A
       change to the published stable surface requires a documented
       justification and a migration note.
   * - Experimental
     - May change between releases without notice. Read the specific
       :doc:`/status/experimental` or :doc:`/status/scope` entry to know
       exactly why a surface is experimental.
   * - Planned / Not Yet Implemented
     - No compatibility guarantee. Nothing in a plan is part of the API
       until a gate test exercises it through the real pipeline.
   * - Release Candidate
     - Between the Beta 1 line and a release candidate, only
       release-blocking corrections (bugs, security, compatibility breaks,
       deterministic-correctness fixes) are made.

Karkain 1.1.0 versioning
=========================

The ``v1.1.0`` ``Stable Build`` label applies from Phase 142 (1.0.x
continues as the LTS line). Before that, 1.0.0 (``v1.0.0``, Phase 119),
Beta 1 (``v0.117.x``) and Developer Preview (``v0.115.x``) carried the
earlier lines. The versioning convention is:

* ``Karkain Compiler v1.1.0 (... Stable Build)`` — the current Stable line.
* ``VERSION`` at the repository root: ``1.1.0``.
* Patch releases within the 1.0 line stay ``1.0.x`` (LTS).

Versioning
==========

* The minor version (``0.x``) signals the public-surface stability level.
* Within a Beta line, patch levels are used for corrections.
* Engine behavior: the Go front end and the self-hosted ``kcc`` engine share
  the same version identity and produce byte-identical output on the gated
  corpus. A divergence in a ``Implemented`` feature is a defect, reported
  through the issue tracker.

What is covered
===============

* The Beta API snapshot: :doc:`/reference/stable-api`.
* The stable diagnostic/exit-code contract
  (:doc:`/reference/diagnostics`).
* The target model (:doc:`/targets/target-triples`).
* Both engines (Go front end and ``kcc``).

What is not covered
===================

* Anything labeled :experimental:`Experimental` or :planned:`Planned`.
* The exact text of diagnostics, which may be reworded as long as the
  error-code contract is preserved.
* Toolchain internals (IR formats, C emission details, cache layout).

Feedback on a breaking change
=============================

If you believe a documented stable behavior changed, open a bug report with
the exact before/after behavior. Compatibility regressions are treated as
release blockers during the Beta/RC preparation period (see
:doc:`/development/feature-freeze`).

.. seealso::

   :doc:`/status/scope` — the authoritative status matrix.
   :doc:`/development/feature-freeze` — the temporary freeze policy.
   :doc:`/reference/stable-api` — the Beta API snapshot.
   :doc:`/reference/compatibility` — Go front end vs. ``kcc`` engine parity.