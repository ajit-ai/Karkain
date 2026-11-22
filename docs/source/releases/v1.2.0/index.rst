================================
Karkain v1.2.0 Release Documents
================================

Karkain **v1.2.0 — Native Independence** is the current stable release. Its
tag is ``v1.2.0`` (peeled commit ``77d7069``) and the GitHub Release was
published on 2026-11-14. The previous release is **v1.1.0** (``v1.1.0`` tag,
merged commit ``951ee10``).

This section collects the evidence-based technical record of what changed
between v1.1.0 and v1.2.0, how to upgrade, how the architecture evolved, and
which capabilities are actually verified. It is documentation only: it does
not grant release status to work that landed after the tag.

.. toctree::
   :maxdepth: 1
   :titlesonly:
   :caption: v1.1.0 to v1.2.0

   V1.1-TO-V1.2-CHANGELOG
   UPGRADE-FROM-V1.1.0
   V1.2.0-ARCHITECTURE-CHANGES
   V1.2.0-VERIFICATION-MATRIX

.. seealso::

   :doc:`/release-notes` — the release-note summary (v1.2.0 current,
   v1.1.0 and v1.0.0 historical).
   :doc:`/status/scope` — the implemented surface.
   :doc:`/status/compatibility` — the compatibility guarantees.
   :doc:`/targets/host-targets` — native target ownership and verification.

Authoritative evidence
======================

The primary sources behind these documents are the repository itself and the
release's own qualification record:

* ``docs/audit/KARKAIN-V1.2.0-GA-QUALIFICATION.md`` — the final, authoritative
  qualification report (measured results, known differences, closure conditions).
* ``docs/release/v1.2.0-CHECKLIST.md`` — the frozen scope and close gates.
* ``docs/inventory/native-feature-matrix.json`` — the machine-verifiable native
  feature ledger (each supported row names a gate file and test pattern).
* ``VERSION`` → ``1.2.0`` — the authoritative version identity.

Statement of provenance
=======================

v1.2.0 is a **locally built and published release, NOT a CI-green release**.
The account was subject to a billing/payment restriction, so the tag-triggered
workflow did not execute its Build matrix; the 13 published archives were built
on a Windows host from the tagged commit with Go 1.21.13, and ``checksums.txt``
was generated and verified locally. This is stated everywhere it matters and
is never softened into a claim of CI success.
