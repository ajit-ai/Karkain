Beta 1
======

.. note::

   This page is the **historical** Beta 1 capability assessment (Phases
   117–118). The current public label is **Karkain 1.0.0 (Stable)** — see
   :doc:`/release-notes` and :doc:`/status/compatibility` for the current
   release identity and guarantees.

Karkain Beta 1 was the first release focused on making the *implemented*
language core and developer toolchain dependable enough for broader
developer experimentation. This page defines exactly what Beta 1 promises —
and what it deliberately does not.

.. contents:: Sections
   :local:
   :depth: 1

What Beta 1 means
-----------------

Beta 1 is a maturity statement about the **implemented** core, not a claim
that every roadmap feature exists:

.. list-table:: Beta 1 expectations
   :widths: 24 76
   :header-rows: 1

   * - Property
     - Meaning
   * - Implemented features
     - Real, exercised by gate tests through the actual pipeline.
   * - Stable enough to rely on
     - The stable core behaves consistently across documented workflows and
       engines.
   * - Consistent across workflows
     - ``check``/``build``/``run``/``test``/``debug``/``prof`` share the same
       pipeline, exit codes and diagnostics.
   * - Protected by regression tests
     - Every claim below is backed by a gate (conformance corpus, Golden
       example corpus, parity tests, CLI E2E tests).
   * - Clearly documented
     - Status vocabulary, exit-code contract, error codes and known
       limitations are all documented on this site.

Features outside the stable core remain classified as
:doc:`/status/implemented`, :doc:`/status/experimental` or
:doc:`/status/planned`. Nothing listed there is silently presented as Beta
stable.

The Beta 1 stable core
----------------------

The following surface is the Beta 1 stable core. Both engines (the Go front
end and the self-hosted ``kcc``) accept and run these features with
byte-identical output on the golden corpus.

Language
~~~~~~~~

* Variables (``let``/``var``/``const``), functions, recursion, control flow
  (``if``/``else``, ``while``/C-style ``for``/``for-in``, ``break``/
  ``continue``/``return``), assertions.
* Primitive types and values: integers, floats, booleans, strings, ``None``;
  checked division/modulo/indexing with ``runtime error: <kind> at
  <file>:<line>``.
* Arrays, maps, structs (records), enums and ``match``, module imports with
  ``public`` exports and qualified calls.

Toolchain
~~~~~~~~~

* ``karkain check | build | run | test | fmt | lint | debug | prof |
  target | explain | clean | pkg | lsp``.
* Stable exit-code contract: 0 success, 2 usage error, 3 compile/parse/
  semantic failure, 4 failing tests, 5 invalid/incompatible engine, 6
  environment/toolchain failure, 7 lint findings, 1 runtime failure.
* Two engines with parity gates: Go front end (default development engine)
  and the self-hosted ``kcc`` (default engine for ``check/build/run/test``
  through the CLI).
* Runtime error diagnostics and stack traces are byte-identical on both
  engines.

Standard library
~~~~~~~~~~~~~~~~

The public stdlib modules are ``std.string``, ``std.collections``,
``std.io``, ``std.encoding`` and ``std.crypto`` (plus ``std.testing`` just
for tests). ``std.encoding``/``std.crypto`` are backed by byte-level runtime
builtins verified against NIST FIPS 180 and RFC 4648 test vectors.

Examples
~~~~~~~~

The example corpus (Phase 114/116: 49 pinned golden examples plus the
developer/real-world categories) is regression-gated: every ``Runnable``
example carries a header declaring its engine and every golden is asserted
byte-identical on every run.

Compatibility expectations
--------------------------

* **Stable core**: changes are deliberate, documented and accompanied by a
  regression test. Breaking the published stable surface requires a
  documented justification.
* **Experimental**: may change without notice. Read the specific
  experimental page entry to know exactly why it is experimental.
* **Planned / Not Yet Implemented**: no compatibility guarantee — nothing
  exists yet.

Beta 1 is **not** Karkain 1.0. No ``Stable``/``Production Ready``/``1.0``
claim is made, and the compatibility policy is intentionally lightweight
until a release candidate establishes a full 1.0 matrix.

Known limitations (honest list)
-------------------------------

* **Engine boundaries** — concurrency, profiling, debug tracing, WASM and
  SIMD vector types are Go-engine only; ``kcc`` parity for those surfaces is
  a documented post-Beta boundary, never silently claimed.
* **Closures / function values** — ``fn`` codegen is not supported on either
  engine (documented Phase 101 boundary); no gate pretends it works.
* **Syntactic gate** — unparenthesized ``while`` conditions were removed from
  the official surface in Phase 117 (both engines now accept them via the
  Phase 117 parser alignment, but the documented form remains
  ``while (cond)``).
* **Cross-compilation** — real cross-linkers (GNU cross-gcc/clang ``--target``)
  are required for foreign triples; on a host without one, ``karkain build
  --target <foreign>`` fails deterministically with a toolchain error. The
  ``wasm32-wasi`` target requires ``wasmtime`` to run.
* **Bootstrap memory** — full ``kcc`` *build* mode for the compiler's own
  sources can stall on very memory-limited hosts; the low-memory ``check``
  path passes everywhere (documented environmental limitation, not a defect).
* **Package manager** — local/workspace resolution and lockfiles are real;
  there is no public registry.

Feedback expectations
---------------------

Beta 1 exists to gather feedback on the *implemented* core. Report issues,
incorrect diagnostics, engine divergences and missing documentation at the
project repository. Before opening an issue, reproduce with the current
``karkain.exe`` and quote the exact diagnostic.

.. seealso::

   :doc:`/status/index` — the six-level status vocabulary used everywhere
   on this site.
   :doc:`/development/developer-preview` — the honest capability summary
   that preceded Beta 1.
   :doc:`/status/implemented` — every implemented feature with its gate.
   :doc:`/status/experimental` — features that exist but are not stable.
   :doc:`/status/planned` — designed but not implemented.
   :doc:`/development/roadmap` — the planned surface over time.