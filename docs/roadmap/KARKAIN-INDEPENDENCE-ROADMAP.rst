====================================================
Karkain Independence Roadmap (consolidated RST view)
====================================================

This page is the **current, consolidated** dependency-aware roadmap. It
supersedes the older ``docs/audit/KARKAIN-INDEPENDENCE-ROADMAP.md``, which is
retained as the historical design record (its baseline is 1.0.0 and it names
1.1.0 as the future line; those statements are no longer current). It does
not invent phase numbers and does not change ``docs/audit/KARKAIN-VERSION-PLAN.md``
or ``docs/audit/KARKAIN-IMPLEMENTATION-CONTROL.md``, which remain the
authoritative version plan and implementation queue.

Baseline (audited 2026-11-22)
=============================

* ``VERSION`` = ``1.2.0``; tags ``v1.0.0``, ``v1.1.0``, ``v1.2.0`` exist and
  the v1.2.0 GitHub Release is published (locally built, **not CI-green**).
* kcc owns all three C-free native targets; a user-program native build needs
  neither gcc nor Go (``TestPhaseNATQual_``, with positive control).
* Self-bootstrap ``stage2 == stage3`` with no Go is **NOT proven** — carried to
  v1.3.0 as UNMET (conditions 5 and 6).
* No-C ``stdlib_v2`` and no-C ``std.net`` are **NOT proven** — carried to
  v1.3.0 as UNMET (conditions 2 and 3).
* The audit found and fixed a P0 defect: the compiler source used an undefined
  accessor (``indexRight``), breaking every kcc-dependent gate. See
  ``docs/audit/KARKAIN-IMPLEMENTATION-BASELINE.rst``.

Workstreams
===========

K0 — Compiler and language independence
----------------------------------------

*Existing*: Go front end + self-hosted kcc; KIR v1 emit/verify; kcc-owned
native encoder and ELF/PE/Mach-O writers; no silent Go fallback.

*Gaps*: KIR is verified but not consumed by the backend (driver reads the AST);
self-bootstrap closure unproven; compiler still built by Go + gcc.

*Gate*: ``TestPhaseNATQual_``, ``TestPhase151_``, KIR pin 15243, and the
carried conditions 5/6 in ``v1.3.0-CHECKLIST.md``.

K1 — Runtime completion
------------------------

*Existing*: native bump arena, print helpers, runtime-error reporter, map
helpers, Win64 PEB bootstrap, Unix syscall tail; C runtime with heap, strings,
arrays and concurrency.

*Gaps*: no error/panic exception system; no in-language threads or processes
on the native target; native stderr diagnostic limitation (``GetStdHandle``).

*Gate*: ``TestPhase152B0_*``, ``TestPhaseNATIdx_`` and the native feature
matrix's refusal rows.

K2 — Standard Library completion
--------------------------------

*Existing*: 25 modules in canonical Karkain; 13 with located executing gates
(string, collections, io, encoding, crypto, testing, numerics, net, http, db,
path, json, generics).

*Gaps*: 9 modules documented as importable but with no located executing gate
(env, time, random, filesystem, process, log, config, core, math); three
source-only (system, gpu, async); functional no-ops in ``std.random``;
documentation contradictions in ``stdlib/index.rst`` / ``not-implemented.rst``.

*Gate*: extend the ``TestPhase132_*`` golden pattern to un-gated modules; fix
``std.random`` no-ops; correct the stdlib documentation before claiming
importability.

K3 — Numerical computing
------------------------

*Existing*: 40-function ``std.numerics`` with both-engine goldens; SIMD lane
types and six builtins on the Go engine; matrix/tensor IR in ``pkg/tensor``.

*Gaps*: native driver refuses float statements; no linear-algebra library;
SIMD kcc lowering deferred.

*Gate*: ``TestPhase132_*``, ``TestPhase106_*``, ``TestPhase114_*`` goldens.

K4 — Scientific and AI computing
--------------------------------

*Existing*: real but **unreachable-from-source** autodiff engine
(``pkg/sema/autodiff.go``), tensor gradients, NPU quantization, WGSL emission;
six executable AI/ML examples with goldens.

*Gaps*: no tokenizer/transformer/inference/model format; the parser cannot
produce the ``TensorStmt`` AST that the autodiff engine expects.

*Gate*: ``KARKAIN-AI-READINESS.rst`` §3 lists the required ordering; no
advanced AI slice may start before it.

K5 — Accelerated and distributed computing
-------------------------------------------

*Existing*: compute-target catalog, compile-only ``@target(gpu)`` WGSL output,
NPU abstraction with CPU fallback.

*Gaps*: no hardware execution evidence; no distributed runtime; Mach-O
execution unproven (no runner).

K6 — Developer ecosystem integration
-------------------------------------

*Existing*: CLI surface, local registry, lockfile, LSP v2, ``karkain dbg``,
``prof``, conformance 64/64, Phase 114 corpus byte-identical on two engines.

*Gaps*: Go-only surfaces (LSP, prof, incremental cache, WASM/WIT, dbg) are
frozen boundaries; release provenance is local, not CI-green.

Implementation ordering
=======================

The dependency order is strict; a later workstream may not be reported green
on the strength of an earlier one.

1. **K0/K1 prerequisite correctness** — keep the compiler buildable and gates
   executable (this assignment's slice).
2. **K2 foundations** — gate the un-gated stdlib modules and repair verified
   no-ops (``std.random``), then correct contradictory docs.
3. **K3** — native float statements and SIMD parity before any tensor work.
4. **K4** — parser bridge for ``TensorStmt`` so the existing autodiff engine
   becomes reachable; only then tokenizer/model work.
5. **K5** — real accelerator execution with hardware evidence; distributed
   last.

Completion gates
================

* Compiler source builds and kcc is usable — ``TestPhaseFoundation_KCCSourceCompiles``.
* A new stdlib module is not "implemented" until a CI-wired gate executes a
  program importing it on both engines with a byte-exact golden.
* A native capability is not "supported" until it appears as ``supported`` in
  ``docs/inventory/native-feature-matrix.json`` with an existing gate file and
  a test pattern that file actually contains.
* No workstream is complete while its carried conditions remain UNMET in
  ``v1.3.0-CHECKLIST.md``.

Current foundation audit and slice queue (2026-11-22)
CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC

A full inspection of the language, compiler, runtime and standard library was
performed at commit ``b554370``. The five authoritative evidence documents are
``docs/audit/KARKAIN-FOUNDATION-STATUS.rst``,
``docs/audit/KARKAIN-STDLIB-CAPABILITY-MATRIX.rst``,
``docs/audit/KARKAIN-AI-READINESS.rst``,
``docs/audit/KARKAIN-IMPLEMENTATION-BASELINE.rst`` and this roadmap.

Authorized scope decision (owner, 2026-11-22)
----------------------------------------------

All following work stays in the **v1.2.x** line. The owner has additionally
authorized the language-semantics workstreams ``K3`` (``Result``/``Option``
with typed ``?`` propagation) and ``K4`` (making the existing autodiff engine
reachable from ``.kark``) as future slices. Those are authorizations only:
each still requires its own baselined increment, gate and KIR re-measurement.
No exceptions/panic system, borrow-checker redesign, memory-model redesign or
DAP work is authorized.

Verified slice in flight
------------------------

A P0 defect was measured and corrected: ``src/compiler/native_value.kark``
called the undefined accessor ``indexRight`` instead of ``indexExpr``. The Go
front end transpiles compiler source without semantic checking, so the defect
surfaced only at the gcc stage and broke **every** gate needing kcc. It is
guarded by ``TestPhaseFoundation_KCCSourceCompiles``, which forces a fresh kcc
build and then checks a program; the gate is mutation-verified and CI-wired.

Dependency-ordered slice queue
------------------------------

.. list-table::
   :widths: 12 30 26 32
   :header-rows: 1

   * - #
     - Slice
     - Depends on
     - Gate / exit criterion
   * - 0
     - land the verified P0 fix (commit + PR)
     - none
     - foundation gate + v1.2.1 green in CI
   * - 1
     - v1.2.4b trim corrective (four latent defects)
     - 0
     - ``TestPhaseV124_`` exercises print/decl/return/array-binding
   * - 2
     - v1.2.5a hex decode (coordinated collector fix)
     - 1
     - PE executes exact bytes; C-backend differential
   * - 3
     - v1.2.5b/c base64 encode/decode
     - 2
     - same gate pattern per codec
   * - 4
     - v1.2.6 SHA-256
     - 2
     - fixed 65-byte arena bound; golden
   * - 5
     - v1.2.7 stdlib integrity
     - 3
     - ``std.random`` no-ops fixed; nine modules gated
   * - 6
     - v1.2.8 native float statements
     - 4
     - float lowering byte-matches the oracle
   * - 7
     - v1.2.9 remaining native parity
     - 6
     - string equality/slice, ``/``/``%``, push patch guard
   * - 8
     - K4 autodiff reachability
     - 7
     - ``.kark`` program invokes the autodiff engine
   * - 9
     - K3 ``Result``/``Option`` propagation
     - 8
     - typed ``?`` gated on both engines
   * - 10
     - K5 platform/tooling breadth
     - 9
     - arm64 native, macOS runner, DAP, kcc tooling parity

The v1.2.5 hex-decode attempt of 2026-11-22 was reverted: valid inputs faulted
because the string-literal collector ignores ``CallExpr`` arguments, and a
partial fix desynchronised the rodata patch guard. The blocker is recorded in
the implementation baseline; slice 2 redoes it as one coordinated change.

Definition of done for every slice
----------------------------------

A module is VERIFIED only when a CI-wired gate executes a program importing it
on both engines with a byte-exact golden. A native capability is supported
only when the native feature matrix names an existing gate file and test
pattern. Every compiler-source change re-measures the KIR pin (never
predicted). No gate may pass without comparing anything, no predicted pin, no
skip reported as a pass.
