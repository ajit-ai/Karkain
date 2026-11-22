=======================================================
Implementation Baseline — Foundation Defect Correction
=======================================================

This record documents the single bounded implementation slice attempted in
the foundation audit. Per the assignment, no commit, push, merge, tag or
version change was made; the changes are in the working tree for review.

Initial repository state
========================

* Branch: ``feature/v1.2.5-native-codecs``
* HEAD: ``b554370e83b242b26c0ced527a4b6634380a65b1`` —
  ``docs: align documentation with Karkain v1.2.0``
* Tags: ``v1.0.0``, ``v1.1.0``, ``v1.2.0``; ``VERSION`` = ``1.2.0``
* Working tree: one pre-existing modified compiler file
  (``src/compiler/native_emit.kark``, 8 content lines of prior v1.2.5 work,
  plus line-ending churn) and numerous pre-existing untracked files. All were
  preserved untouched.
* Environment: Go 1.27.0 on windows/amd64; gcc 16.2.0 at
  ``C:\msys64\mingw64\bin\gcc.exe`` (present on disk but **not** on ``PATH``,
  so it was prepended for the test commands below); no Python/Sphinx.

Defect selected (priority 1: incorrect existing foundational functionality)
=============================================================================

During verification, ``go test ./pkg/cli -run TestPhaseV121_`` FAILED. The
compiler source could not be compiled into the self-hosted engine:

.. code-block:: text

   main.c:77863:19: error: implicit declaration of function 'indexRight'
   main.c:77863:18: error: incompatible types when assigning to type 'Value'
                           from type 'int'

Root cause (measured, not inferred)
------------------------------------

``src/compiler/native_value.kark`` line 8856, in ``natNativeCodecIn``, called
``indexRight``. The AST module defines ``indexLeft``, ``indexExpr`` and
``indexLine`` but **no** ``indexRight`` (``src/compiler/ast.kark:584-586``).
The defect is present at HEAD (``git show HEAD:...`` confirms it), so every
gate needing kcc was broken, not just the one that exposed it.

This defect class is subtle: the Go front end transpiles the compiler source
to C23 without semantic checking, so the source parse/check passes and the
failure appears only at the gcc stage. A source-level assertion would not
reliably catch it; an executable build gate is required.

Implementation (one line)
=========================

.. code-block:: text

   - natNativeCodecIn(indexLeft(e)) + natNativeCodecIn(indexRight(e))
   + natNativeCodecIn(indexLeft(e)) + natNativeCodecIn(indexExpr(e))

Changed source file: ``src/compiler/native_value.kark`` (1 line).

Test added
==========

``pkg/cli/phase_foundation_compiler_test.go`` —
``TestPhaseFoundation_KCCSourceCompiles``:

1. Forces ``KARKAIN_KCC`` empty so a prebuilt binary cannot mask the defect.
2. Builds kcc from the current working tree via the existing isolated
   ``phase95KCC`` helper (Go front-end transpile + gcc link).
3. Proves the freshly built binary is usable by running ``kcc check`` on a
   trivial program and requiring the ``[ok]`` marker.

Tests executed and exact results
================================

All commands ran with ``PATH`` prepended by ``C:\msys64\mingw64\bin``.

.. list-table::
   :widths: 58 12 30
   :header-rows: 1

   * - Command
     - Result
     - Detail
   * - ``go test ./pkg/lexer/... -count=1``
     - PASS
     - ``ok karkain/pkg/lexer 1.245s``
   * - ``go test ./pkg/cli/ -run TestPhaseV121_ -count=1`` (BEFORE fix)
     - **FAIL**
     - ``implicit declaration of function 'indexRight'``; the previously
       green-looking gate could not build kcc
   * - ``go test ./pkg/cli/ -run TestPhaseFoundation_KCCSourceCompiles -count=1 -v``
     - PASS
     - ``--- PASS (36.84s)``; fresh kcc built and checked a program
   * - ``go test ./pkg/cli/ -run TestPhaseV121_ -count=1`` (AFTER fix)
     - PASS
     - ``ok karkain/pkg/cli 82.586s``
   * - Mutation: restore ``indexRight`` in the source, re-run the new gate
     - FAIL as required
     - identical ``implicit declaration of function 'indexRight'`` signature;
       gate failed in 9.064s
   * - Restore from SHA-256-verified backup
     - PASS
     - ``C9C6C27C18E7300DD05BDEB352D3656140B69E5FD7E733F307F0D373CEC45B3A``
       before and after; the gate passed again

Files changed in this slice
===========================

* ``src/compiler/native_value.kark`` — one-line fix (tracked, modified).
* ``pkg/cli/phase_foundation_compiler_test.go`` — new regression gate
  (untracked, not committed).
* ``.github/workflows/ci.yml`` — dedicated CI step for the new gate.
* Five audit/roadmap RST artifacts (untracked, not committed) — listed in the
  final report.

Outstanding limitations
=======================

* No commit was made (assignment §13); changes are for owner review.
* The CI step for ``TestPhaseFoundation_KCCSourceCompiles`` was added to
  ``.github/workflows/ci.yml`` (*Run foundation compiler-source build gate*)
  in the same change, following the repository convention that a gate is a
  test file **plus** a CI entry. The workflow was not validated with a YAML
  parser locally (no Python/js-yaml invocation was required for this slice);
  it should be checked by CI on the first push.
* Sphinx HTML could not be regenerated locally (no Python/Sphinx); the CI
  ``Documentation`` job remains the build authority.
* Full-suite runs were intentionally avoided on the ~4 GB host; only targeted
  gates were executed.


Appendix A - v1.2.5 hex-decode attempt (not shipped)
=====================================================

An initial v1.2.5 slice (native ``hex_decode_bytes``) was implemented and
gated, then deliberately REVERTED because it was not safe to ship. The
working tree was restored to HEAD plus the one-line ``indexRight`` fix, and
the full foundation + v1.2.1 gates were re-run green (91.6 s).

What the attempt proved
-----------------------

* The five-unit ``codecTemp`` staging and the trim call path are reusable for
  a second codec; the new helper compiled and linked.
* Valid inputs still faulted (0xC0000005). Disassembly of the produced PE
  showed the argument's string-literal pointer resolved to print_int's ``"-"``
  (offset 0) rather than the program's literal: ``natNativeStrLitAppend``
  descends ``+`` but NOT ``CallExpr`` arguments, so the literal was never
  interned. Adding that descent fixed the pointer but exposed a rodata patch
  guard failure on trim (``rodata patch 1 of 5 ... does not fit the section
  body``), i.e. the collector, emitter cursor and PE patch guard disagree.
* A correctly scoped fix must update all three together (collector, emitter
  offset cursor, and patch-list sizing) and re-pin the KIR. That is larger
  than one safe slice on this host, so it is deferred with this evidence.

Four latent v1.2.4 defects found during the attempt
----------------------------------------------------

These are in the committed v1.2.4 code (not regressions from the attempt):

1. ``natNativeScanValue`` placed the ``trim`` case AFTER an unconditional
   ``return natNativeScanCall(...)``, so it was unreachable.
2. ``natNativeScanDecl`` nested the ``trim`` case INSIDE the
   ``callFunc(v) == "push"`` branch, so a trim call could never match.
3. ``natNativeScanReturn`` had a malformed string-literal/call block (an
   ``if`` with no body followed by a second ``if``).
4. Decl lowering dropped the ``else if (nodeType(v) == NODE_ARRAY_LIT())``
   branch header, routing array literals into the array-value body of the
   call branch.

They are documented here rather than fixed hurriedly: the v1.2.4 trim slice
has no gate, so a corrective change should add one that exercises all four
paths before altering them.

Net result of this assignment
-----------------------------

The one safe, fully verified slice (the ``indexRight`` P0 fix plus its
mutation-verified gate and CI step) remains in the working tree. The v1.2.5
hex codec is NOT implemented; its blocker and the four latent defects are
recorded for the next authorized corrective slice.