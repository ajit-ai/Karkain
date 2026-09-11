.. _stdlib-core:

std.core — Core helpers (Experimental)
======================================

:experimental:`Experimental` — **not importable**.

``stdlib/core/core.kark`` exists in the repository, but it is **not
importable** via ``import std.core``. It sits behind the Phase 109
standard-library boundary for two reasons:

* **Non-canonical syntax.** The file uses typed parameters, ``->`` return
  annotations, tuple returns, and ``Option``/``Result``/``match`` — surface
  that is not parsed identically (or at all) by both engines today.
* **No runtime builtin backing.** Unlike the importable modules, none of its
  operations are wired into the Go resolver, Go codegen or the ``kcc``
  checker/sema/codegen tables.

:planned:`Planned` — ``std.core`` will become importable when the Phase 109
standard-library expansion lands proper builtin backing and canonical-syntax
versions of these helpers.

What is in the source file
==========================

The file defines helper functions matching this surface (documented
**only** as the future direction of the module — treat none of these as
usable API today):

* ``identity`` / ``identity_float``
* ``clamp`` / ``clamp_float``
* ``min`` / ``max`` / ``min_float`` / ``max_float``
* ``abs`` / ``sign`` / ``sign_float`` / ``swap``
* ``is_none`` / ``is_some`` / ``is_ok`` / ``is_err``
* ``range`` / ``range_step`` / ``repeat`` / ``sum`` / ``product``
* ``max_of`` / ``min_of``

Typed signatures and ``Option``/``Result`` interaction will change when the
module is made importable, so do not write code against this surface.

Error status
============

.. code-block:: text

   import std.core      // rejected — module resolution error (exit code 3)

``std.system``, ``std.gpu`` and ``std.async`` are in the same position:
source files exist under ``stdlib/`` but are not importable (see
:doc:`not-implemented`).