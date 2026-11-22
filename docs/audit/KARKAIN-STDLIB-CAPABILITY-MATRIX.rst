======================================================
Karkain Standard Library — Capability Matrix (audited)
======================================================

This matrix is the module-by-module inventory of ``stdlib/`` at the audited
baseline (``VERSION`` = ``1.2.0``, branch ``feature/v1.2.5-native-codecs``,
commit ``b554370``). It deliberately distinguishes **executable evidence**
from source presence: a module is ``VERIFIED`` only when a located gate runs a
program that imports it and checks output.

All 25 module directories contain exactly one ``.kark`` file each, written in
canonical Karkain. There are no Go, C23, README or test files under
``stdlib/``. Backing builtins (hex/Base64/SHA/net) are emitted as C23 by
``pkg/codegen/codegen.go`` and mirrored in ``src/compiler/codegen.kark``, and
are registered in both checkers.

Status definitions
==================

``VERIFIED``
   A located, CI-wired gate executes a program importing the module and checks
   output on Go and/or kcc.
``PARTIAL``
   Executable evidence exists but APIs, determinism or parity are incomplete.
``UNVERIFIED``
   Source exists and docs claim it is importable, but no executing gate was
   located (or a known gate is red).
``SOURCE ONLY``
   Source exists but is documented as not importable / blocked.

Modules with executable evidence
================================

.. list-table::
   :widths: 18 10 14 12 46
   :header-rows: 1

   * - Module
     - Funcs
     - Language
     - Status
     - Evidence
   * - ``std.string``
     - 28
     - Karkain
     - VERIFIED
     - ``TestPhase109_*``, ``TestPhase132_StdlibGoldens*``, ``TestPhase117_*``;
       NIST/RFC hex/Base64 vectors, UTF-8 round-trips
   * - ``std.collections``
     - 24
     - Karkain
     - VERIFIED
     - ``TestPhase109_*``, ``TestPhase132_StdlibGoldens*``
   * - ``std.io``
     - 11
     - Karkain
     - VERIFIED
     - ``TestPhase109_*``, ``TestPhase132_StdlibGoldens*`` (wasm target K108-gated)
   * - ``std.encoding``
     - 7
     - Karkain
     - VERIFIED
     - ``TestPhase109_*``, ``TestPhase117_StdlibMalformedInput`` (malformed
       hex/base64 → same source-located error on both engines)
   * - ``std.crypto``
     - 2
     - Karkain
     - VERIFIED
     - ``TestPhase109_*`` (sha256/sha512 NIST vectors byte-identical)
   * - ``std.testing``
     - 9
     - Karkain
     - VERIFIED
     - ``TestPhase132_StdlibGoldens*`` via ``examples/testing/main.kark``
   * - ``std.numerics``
     - 40
     - Karkain
     - VERIFIED
     - ``TestPhase132_StdlibGoldens*`` and Phase 114 goldens for
       ``09-ai/03_numerics_forward.kark`` (15-line golden, both engines)
   * - ``std.net``
     - 10
     - Karkain
     - VERIFIED (C backend)
     - ``TestPhase132_StdlibGoldens*`` via ``04-networking/02_tcp_roundtrip.kark``;
       native target rejects ``net_*`` by name (``TestPhase152A_*``)
   * - ``std.http``
     - 21
     - Karkain
     - VERIFIED (C backend)
     - ``TestPhase132_StdlibGoldens*`` via ``07-web/02_http_codec.kark``
   * - ``std.db``
     - 43
     - Karkain
     - VERIFIED (C backend)
     - ``TestPhase132_StdlibGoldens*`` via ``06-database/01_db_crud.kark``;
       deterministic pipe-delimited text engine
   * - ``std.path``
     - 9
     - Karkain
     - VERIFIED
     - ``TestPhaseV121_StdlibPathAndJsonBothEngines`` calls every documented
       function; raw Go↔kcc byte equality
   * - ``std.json``
     - 6
     - Karkain
     - VERIFIED
     - Same gate (v1.2.1); note the documented function count was corrected
       from 8 to 6 in that slice
   * - ``std.generics``
     - 12
     - Karkain
     - VERIFIED
     - ``TestPhase146B_*`` and ``TestPhase146C_StdlibGenericsKCC`` via the
       stack/queue examples, both engines

Modules with source but no located executing gate
===================================================

.. list-table::
   :widths: 18 10 12 60
   :header-rows: 1

   * - Module
     - Funcs
     - Status
     - Finding
   * - ``std.env``
     - 9
     - UNVERIFIED
     - Shell/PowerShell backed; v1.2.9 work item in
       ``V1.2.2-CONTROL-REFRESH.md`` ("gates and portability")
   * - ``std.time``
     - 11
     - UNVERIFIED
     - PowerShell backed, nondeterministic (``V1.2.1-STDLIB-PATH-JSON-GATES.md``)
   * - ``std.random``
     - 7
     - UNVERIFIED / PARTIAL
     - ``random_shuffle`` returns its input unchanged and ``random_seed``
       ignores its argument (functional no-ops in source)
   * - ``std.filesystem``
     - 14
     - UNVERIFIED
     - Windows-oriented; docs ``index.rst`` paradoxically says it "does not
       exist" while listing it as implemented
   * - ``std.process``
     - 10
     - UNVERIFIED
     - Shell backed; no gated example located
   * - ``std.log``
     - 6
     - UNVERIFIED
     - Imports ``std.time`` (shell backed)
   * - ``std.config``
     - 5
     - UNVERIFIED
     - Imports ``std.env``
   * - ``std.core``
     - 18
     - UNVERIFIED
     - Only file-existence/fixture-resolution tests; docs disagree about
       importability
   * - ``std.math``
     - 15
     - UNVERIFIED
     - ``math_sqrt`` shells out to PowerShell; docs/source signature mismatch
       (``math_abs`` int vs float)

Modules documented as not importable (source only)
====================================================

.. list-table::
   :widths: 18 10 72
   :header-rows: 1

   * - Module
     - Funcs
     - Finding
   * - ``std.system``
     - 16
     - ``import "libc"``; output cannot be captured; redesign needed
       (``stdlib/index.rst:108-110``). ``get_args`` is a no-op.
   * - ``std.gpu``
     - 31
     - ``import "libc"``, typed annotations; no runtime builtin backing
       (``not-implemented.rst``)
   * - ``std.async``
     - 37
     - ``import "libc"``, tuples/Option/Result required; no runtime backing

API completeness and anomalies
===============================

* No module uses the ``public`` keyword; stdlib functions are framework API,
  callable unqualified or qualified (``docs/source/stdlib/index.rst:118-120``).
* Declared-vs-documented function counts differ for ``string`` (28 vs 27),
  ``core`` (18 vs 20), ``http`` (21 vs 20) and ``db`` (43 including private
  helpers vs 20 documented public).
* ``docs/source/stdlib/not-implemented.rst`` contradicts ``index.rst`` on
  ``std.core``/``std.math``/``std.time``; these are documentation defects, not
  compiler defects, and are recorded here rather than silently "fixed".
* ``TestPhase109_StdlibAssemblyPullsModules`` is recorded as failing on
  pristine HEAD (``V1.2.1-STDLIB-PATH-JSON-GATES.md:88-92``) — one of the five
  Phase 109 gates is known-red, not green.
* ``TestPhase132_StdlibAPIFreeze`` checks 10 frozen modules; ``std.generics``
  is absent from ``stdlib/index.rst``'s module table despite being gated.

Native-target stdlib status
============================

No standard-library module can currently build as a kcc-produced native
image: the native driver has no stdlib import support, and the Go backend has
no socket emitter. These are v1.2.0 closure conditions 2 and 3, carried to
v1.3.0 as UNMET by owner-authorized amendment. See
``KARKAIN-FOUNDATION-STATUS.rst`` §4.
