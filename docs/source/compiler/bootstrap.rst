Self-Hosting Bootstrap
======================

**Package:** ``pkg/bootstrap``

The Karkain compiler is self-hosted: the ``src/compiler/*.kark`` sources
are compiled by the compiler itself. This is proven correct through a
three-stage bootstrap pipeline.

Pipeline
--------

::

   src/compiler/*.kark
         │
         ▼
   Stage 1: Go front end compiles → C23 → gcc → karkain-compiler1
         │
         ▼
   Stage 2: karkain-compiler1 compiles → C23 → gcc → karkain-compiler2
         │
         ▼
   Stage 3: karkain-compiler2 compiles → C23 → gcc → karkain-compiler3

The invariant:

.. code-block:: text

   SHA256(karkain-compiler2) == SHA256(karkain-compiler3)

If Stage 2 and Stage 3 are bitwise identical, the compiler is a
deterministic fixed point of itself — compiling the compiler source
produces the same binary regardless of which prior binary was used to
compile it.

Implementation
--------------

``RunBootstrap(projectRoot)`` orchestrates the three stages:

Stage 1 (``RunStage1``)
~~~~~~~~~~~~~~~~~~~~~~~

1. ``go build -o bin/karkain-stage1 ./cmd/karkain`` — the Go-based
   compiler binary.
2. ``karkain-stage1 build src/compiler/main.kark --target c23`` — the
   Go binary transpiles the compiler sources into C23 (``main.c`` or
   ``main.c23``).
3. ``gcc -std=c2x -o bin/karkain-compiler1 <generated.c> -lm -lgmp`` —
   the first native compiler binary.

The generated C file is removed after compilation so stale artifacts
cannot shadow a freshly generated file in later stages.

Stage 2 (``RunStage2``)
~~~~~~~~~~~~~~~~~~~~~~~

1. ``karkain-compiler1 build src/compiler/main.kark --target c23`` —
   the stage-1 binary re-compiles the same sources.
2. ``gcc -std=c2x -o bin/karkain-compiler2 <generated.c> -lm -lgmp``

Stage 3 (``RunStage3``)
~~~~~~~~~~~~~~~~~~~~~~~

Identical to Stage 2, but using ``karkain-compiler2`` as the compiler:

1. ``karkain-compiler2 build src/compiler/main.kark --target c23``
2. ``gcc -std=c2x -o bin/karkain-compiler3 <generated.c> -lm -lgmp``

Reproducibility guarantees
--------------------------

``SOURCE_DATE_EPOCH`` injection
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

GNU ld (binutils ≥ 2.40, as shipped with MSYS2/MinGW gcc 14.x) embeds
the current wall-clock time into the PE ``TimeDateStamp`` of produced
binaries unless ``SOURCE_DATE_EPOCH`` is set. The bootstrap pipeline
injects a fixed epoch (``1072915200`` — 2004-01-01T00:00:00Z) into
every child process via ``setSourceDateEpoch`` so the link step is fully
reproducible.

``forceGoEngine`` contract
~~~~~~~~~~~~~~~~~~~~~~~~~~

The bootstrap pipeline pins ``KARKAIN_ENGINE=go`` on every stage-1
invocation via ``forceGoEngine``. The default engine is kcc since
Phase 97; without this pin the bootstrap would recurse into kcc and
never produce the expected C artifact. Stage 2 and Stage 3 are
implicitly Go-engine too, since the compiled stage-1 binary is itself
the Go-compiled ``karkain-stage1``.

Test gate
---------

``TestBootstrap_BitwiseIdentity`` (``pkg/bootstrap/bootstrap_test.go``)

1. Runs ``RunStage1`` → ``RunStage2`` → ``RunStage3``.
2. Asserts ``VerifyIdentity(s2, s3)`` — bitwise SHA256 comparison.
3. On failure: ``Bitwise identity FAILED: stage2 SHA256=... != stage3
   SHA256=...``.

This test validates the fundamental self-hosting property: the compiler
can compile its own source and produce the same binary regardless of
which version was used. Verified at Phase 99 (SHA ``aff1d624d9e52c2d``)
and Phase 107 (concurrency changes).

Other tests
~~~~~~~~~~~

- ``TestBootstrap_Stage1Compilation`` — stage-1 binary is non-empty with
  a valid SHA.
- ``TestBootstrap_Stage2SelfHosting`` — stage-1 compiles the sources to
  produce a non-empty stage-2 binary.

All tests clean up their binary artifacts (``karkain-stage1``,
``karkain-compiler1``, ``karkain-compiler2``, ``karkain-compiler3``).

Known limitation: ~4 GB host OOM
--------------------------------

On hosts with approximately 4 GB of RAM, Stage 2 (and sometimes Stage 1)
may stall or segfault during compilation of the full compiler source
tree (~214 KB of ``.kark`` source). This is a known environmental class
documented in the Phase 99 and Phase 101 bootstrap gates:

- It affects only the full-tree ``go test ./pkg/...`` concurrent path,
  not isolated test runs.
- The low-memory ``check`` path still passes.
- The ``TestBootstrap_BitwiseIdentity`` test skips in short mode
  (``-short``) to avoid hitting the memory limit on constrained hosts.

This is NOT a compiler defect; it is a resource constraint that does not
affect the correctness proof (when the host has sufficient memory, the
identity test passes).

Phase history
-------------

.. list-table::
   :header-rows: 1
   :widths: 14 76

   * - Phase
     - Milestone
   * - 88
     - Self-hosted compiler foundation: 7 representative test programs,
       build script, ``bootstrap.go`` (stage-1 compilation)
   * - 99
     - Bootstrap identity verified: stage-2 SHA ``aff1d624d9e52c2d``
       equals stage-3 SHA; full pipeline proven
   * - 107
     - Concurrency changes to ``src/compiler/*.kark`` — bootstrap
       identity re-verified

Assembly step
-------------

The bootstrap pipeline transpiles ``src/compiler/main.kark`` with the
``--target c23`` flag. The Go front end first assembles all sibling
``.kark`` files in ``src/compiler/`` (``lexer.kark``, ``parser.kark``,
``ast.kark``, ``sema.kark``, ``checker.kark``, ``atypes.kark``,
``codegen.kark``, ``stdlib.kark``) alongside ``main.kark`` into a
single C23 translation unit. The generated C file is found next to the
source (``.c23`` or ``.c`` extension). The legacy pointer-based
``src/compiler/runtime.c`` is deliberately NOT linked — the generated
C already defines every runtime helper by-value, and linking
``runtime.c`` causes duplicate symbol errors (see
``docs/audit/C-ABI.md``).
