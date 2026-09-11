.. _stdlib-not-implemented:

Not Yet Implemented modules
===========================

:not-implemented:`Not Yet Implemented` — these modules do **not** exist.

The following ``std.*`` module names have **no directory and no source file**
in the repository. Nothing can be imported from them today, and no API is
documented here because none has been designed or implemented:

* ``std.memory``
* ``std.filesystem``
* ``std.process``
* ``std.os``
* ``std.time``
* ``std.concurrency``
* ``std.diagnostics``

Each of these is :planned:`Planned`: the module will be added when the
corresponding capability is implemented and tested. Until then, ``import`` of
any of them is a module resolution error (exit code ``3``):

.. code-block:: text

   import std.time        // rejected — no such module

Source-present but not importable
=================================

Separately, these module names **do** have source files under ``stdlib/``,
but the files are **not importable** through the toolchain:

* ``std.core`` (``stdlib/core/core.kark``) — see :doc:`core`
* ``std.math`` (``stdlib/math/math.kark``) — see :doc:`math`
* ``std.system`` (``stdlib/system/system.kark``)
* ``std.gpu`` (``stdlib/gpu/gpu.kark``)
* ``std.async`` (``stdlib/async/actor.kark``)

These sit behind the Phase 109 standard-library boundary: they use
non-canonical syntax (typed annotations, ``import "libc"`` blocks, ``const``,
``Option``/``Result``) and have no runtime builtin backing, so they compile
and parse behavior is not uniform across both engines. They are
:experimental:`Experimental` — source present, future direction sketched —
and will become importable when that boundary is lifted. Do not write
programs that depend on them.

Implemented today
=================

The importable, tested modules are :doc:`strings`, :doc:`collections`,
:doc:`io`, :doc:`encoding`, :doc:`crypto` and :doc:`testing`.