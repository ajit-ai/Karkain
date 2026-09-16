.. _stdlib-math:

std.math — Math functions (Experimental)
========================================

:experimental:`Experimental` — **not importable**.

``stdlib/math/math.kark`` exists in the repository, but it is **not
importable** via ``import std.math``. It sits behind the Phase 109
standard-library boundary for two reasons:

* **Non-canonical syntax.** The file opens with an ``import "libc" { ... }``
  block declaring libm entry points, uses ``const`` declarations and typed
  annotations, and relies on C interop for every operation — surface that is
  not uniform across both engines.
* **No runtime builtin backing.** Unlike the importable modules, none of its
  math helpers are wired into the Go resolver, Go codegen or the ``kcc``
  checker/sema/codegen tables.

:planned:`Planned` — ``std.math`` will become importable when the Phase 109
standard-library expansion lands canonical-syntax, builtin-backed math
operations (the Phase 101 freestanding runtime already ships a libm-free
math-primitive layer under ``runtime/freestanding`` and ``runtime/core`` as a
future foundation).

What is in the source file
--------------------------

The file defines grouped math helpers matching this surface (documented
**only** as the future direction of the module — treat none of these as
usable API today):

* **Constants:** ``PI``, ``E``, ``LN2``, ``LN10``, ``SQRT2``, ``SQRT3``,
  ``GOLDEN_RATIO``, ``DEG_TO_RAD``, ``RAD_TO_DEG``, ``EPSILON``.
* **Basic ops:** ``math_abs``, ``math_sqrt``, ``math_cbrt``, ``math_pow``,
  ``math_square``, ``math_cube``, ``math_abs_int``.
* **Rounding and clamping:** ``math_ceil``, ``math_floor``, ``math_round``,
  ``math_round_int``, ``math_clamp``, ``math_clamp_int``, ``math_lerp``.
* Plus trigonometry / exponential / linear-algebra helpers declared against
  the ``libc`` block.

Signatures will change when the module is made importable, so do not write
code against this surface.

Error status
------------

.. code-block:: text

   import std.math      // rejected — module resolution error (exit code 3)

See :doc:`core` for the shared Phase 109 boundary, and :doc:`not-implemented`
for modules that do not exist in the repository at all.