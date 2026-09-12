===================
Migration to Beta 1
===================

Karkain moved from **Developer Preview** to **Beta 1** in Phase 117. This
page explains what actually changed for a developer who was using the
Developer Preview line (v0.115.x) and what they must do to move to Beta 1
(v1.0.0).

Developer Preview vs. Beta 1
============================

The public label changed from ``Developer Preview`` to ``🧪 Beta 1``. The
version identity changed from ``v0.115.0 (... Developer Preview Build)`` to
``v1.0.0 (... Stable Build)``.

Are there source-breaking changes?
==================================

**No source-breaking changes** were introduced by the Developer Preview to
Beta 1 transition. Programs written for the Developer Preview language core
continue to parse, type-check and run on Beta 1 through both engines.

What changed (Phase 117)
========================

.. list-table:: What moved between Developer Preview and Beta 1
   :widths: 30 70
   :header-rows: 1

   * - Area
     - Change
   * - Parsers
     - The Go parser and the self-hosted ``kcc`` parser were aligned on
       statement termination (semicolons and newlines) and on
       unparenthesized ``while`` conditions. Both engines now accept the
       same source.
   * - Semantic build/run gating
     - ``build`` and ``run`` now reject undefined identifiers with
       ``error[K002]`` (Go engine) / ``error[K102]`` (``kcc``) and
       ``ExitCompile(3)``, matching ``check`` semantics on both engines.
   * - Recoverable parse errors
     - Parse errors are no longer swallowed by ``run``/``build``;
       malformed programs such as ``func main() { let = 42 }`` exit 3.
   * - Workspace dependency resolution
     - ``pm.DependencySources`` resolves workspace roots correctly, so an
       application member can consume a sibling library member (the
       ``dep()``-style cross-member calls no longer met an undefined
       identifier).
   * - Error-code documentation
     - Numeric error codes (K001–K008, K100, K101–K113) are documented by
       ``karkain explain`` and in the diagnostics reference.
   * - Standard-library edge cases
     - Stdlib edge-case parity (e.g. UTF-8, malformed hex/Base64) is now
       gated on both engines.
   * - Example corpus
     - The corpus grew to **50** ``.kark`` programs across 15 categories
       (49 regression-pinned goldens; 5 deliberately skipped).
   * - Documentation
     - The Beta 1 capability assessment
       (:doc:`/status/beta`), the status vocabulary and the release notes
       were rewritten for the Beta label.
   * - Fresh-checkout validation
     - A reproducible fresh-checkout validator
       (``scripts/beta-fresh-checkout.ps1``) verifies the 16-step build +
       smoke + documentation path.

Language surface
================

The Beta 1 stable core (see :doc:`/status/beta` and
:doc:`/reference/stable-api`) is the same language surface external
developers were already using in the Developer Preview: variables,
functions, control flow, structs, enums, modules, and the five importable
``std.*`` modules. No language syntax was removed.

Known material limitations carried over
=======================================

* Closures / ``fn`` function values: codegen is not supported on either
  engine (documented Phase 101 boundary).
* Concurrency, profiling, debug tracing, WASM and SIMD are Go-engine only;
  ``kcc`` parity is a documented post-Beta boundary.
* There is no public package registry; the package manager resolves local
  and workspace dependencies.

Moving a Developer Preview project
==================================

1. Rebuild the toolchain from source (see
   :doc:`/getting-started/installation`).
2. `karkain --version` reports ``Karkain Compiler v1.0.0``.
3. Re-run your existing programs with ``karkain check`` / ``run`` — no
   source changes should be required.
4. If you used the workspace dependency path, verify cross-member calls now
   resolve (Phase 117 workspace dependency fix).
5. Re-run ``karkain fmt --check`` and the test suite to confirm formatting
   and behavior are unchanged.

.. seealso::

   :doc:`/status/beta` — the Beta 1 capability assessment.
   :doc:`/status/compatibility` — the compatibility policy.
   :doc:`/release-notes` — what shipped in each release.