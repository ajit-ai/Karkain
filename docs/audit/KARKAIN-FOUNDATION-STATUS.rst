=====================================================
Karkain Foundation Status — Compiler, Runtime, Stdlib
=====================================================

This is a fact-based inventory of the Karkain language foundation at the
audited baseline: branch ``feature/v1.2.5-native-codecs``, commit ``b554370``
(documentation alignment), with version ``1.2.0`` and tags ``v1.0.0``,
``v1.1.0``, ``v1.2.0`` present. It distinguishes capabilities **proven by
executable evidence** from those that exist only in source or documentation.

The authoritative existing records remain:

* ``docs/audit/KARKAIN-V1.2.0-GA-QUALIFICATION.md`` — the v1.2.0 release
  qualification (measured gates, known differences, closure conditions).
* ``docs/inventory/compiler-dependencies.json`` — component ownership
  (verified 2026-11-10).
* ``docs/inventory/native-feature-matrix.json`` — native feature/gate ledger
  (verified 2026-11-21).
* ``docs/audit/KARKAIN-IMPLEMENTATION-CONTROL.md`` — the authoritative
  implementation queue.
* ``docs/audit/KARKAIN-INDEPENDENCE-ROADMAP.md`` — the pre-existing
  independence roadmap (historical; the RST roadmap in ``docs/roadmap/`` is
  the current, consolidated view).

1. Language and compiler capabilities (verified)
==================================================

The parser, type checker and semantic analyser exist on **both** engines:

.. list-table::
   :widths: 24 32 44
   :header-rows: 1

   * - Component
     - Go side
     - Self-hosted side
   * - Lexer
     - ``pkg/lexer``
     - ``src/compiler/lexer.kark``
   * - Parser / AST
     - ``pkg/parser``
     - ``src/compiler/parser.kark``, ``ast.kark``
   * - Types / checker
     - ``pkg/sema``
     - ``checker.kark``, ``sema.kark``, ``atypes.kark``
   * - C23 codegen
     - ``pkg/codegen``
     - ``src/compiler/codegen.kark``
   * - KIR v1
     - routing only (``pkg/cli/kir.go``)
     - ``src/compiler/kir.kark`` (emit + structural verify)
   * - Native backend
     - REFERENCE only (``pkg/native``)
     - OWNER (``native_value.kark`` + encoder + containers)
   * - Flat project assembly
     - Go sandbox bridge
     - ``assembleProject`` in ``main.kark``

Verified by executable evidence:

* The conformance corpus passes **64/64** (recorded in the GA qualification,
  §11.1); the Phase 114 example corpus is byte-identical Go↔kcc.
* ``kcc`` is the **default** engine; only ``KARKAIN_ENGINE=go`` selects Go.
* All three C-free native targets (Linux ELF, Windows PE, macOS Mach-O) are
  kcc-owned; nothing refuses with ``error[K116]``.
* The v1.2.0 KIR pin is **15243** (``TestPhase122_PipelineOwnership`` and
  ``TestPhase151A5_KIRPinHolds``).

Important boundary: **KIR is verified but is not the backend's input**. The
native driver consumes the parsed AST directly
(``docs/inventory/compiler-dependencies.json`` lines 41-47).

2. Runtime capabilities
=======================

Two runtime families exist:

* **C23 runtime** (``runtime/freestanding``, ``runtime/core``,
  ``runtime/concurrency/c``): arena allocator, tagged ``Value``, native
  strings/arrays, I/O, math, scheduler/channel/actor. Used by the C23 path,
  which still needs ``gcc`` and GMP.
* **Native runtime** (emitted by the kcc driver into the image): bump arena
  with header inside the arena, ``print_int``/``print_str``/``print_float``,
  the runtime-error reporter, map helpers, Win64 PEB bootstrap, and the Unix
  syscall entry tail. It has no libc dependency.

Verified native execution:

* Windows PE: executed on the dev host, stdout compared byte-for-byte.
* Linux ELF: byte-identical to the Go oracle for gated shapes; executed on the
  Linux CI leg (no ELF runner on the Windows dev host).
* macOS Mach-O: **structural only** — no macOS runner exists in this
  repository.

Native capabilities that are **refused by name with no image** (never
silently mis-lowered): float statements/parameters/returns, ``for ... in``,
map literals in the driver, string equality and slicing, a string inside a
branch or loop, strings in a callee, integer division and modulo, comparison
as a value, recursion, stdlib imports and networking. The authoritative list
is ``docs/inventory/native-feature-matrix.json``.

3. Go dependency analysis
=========================

.. list-table::
   :widths: 36 64
   :header-rows: 1

   * - Operation
     - Go requirement
   * - Building the compiler from a checkout
     - **Yes** — ``buildKCC`` runs ``go build`` and a Go-front-end transpile
       (``pkg/cli/kcc_engine.go:182-221``, ``956-972``).
   * - Using an already-built kcc for ``check``
     - No Go; kcc binary plus the C23 runtime for build/run.
   * - Compiling a USER program to a C-free native target
     - **No Go, no gcc** — proven by ``TestPhaseNATQual_`` with PATH shims and
       a non-empty legacy positive control.
   * - Building that kcc binary in the first place
     - Go + gcc (if absent or stale).
   * - Self-bootstrap ``stage2 == stage3`` with no Go on the host
     - **NOT PROVEN** — conditions 5/6 carried to v1.3.0 as UNMET; the fast
       pin-contract half runs in CI, the three-stage burn is capacity- and
       runner-blocked.

Go-only surfaces (frozen boundaries): LSP, ``prof``, incremental cache
(``pkg/compiler``), WASM/WIT, ``dbg``, and package-manager CLI commands.

4. C23 dependency analysis
==========================

.. list-table::
   :widths: 36 64
   :header-rows: 1

   * - Path
     - C23 requirement
   * - ``--target c23`` (default C path)
     - **Yes** — both engines emit C23 and link with ``gcc`` + GMP (+ Winsock
       on Windows).
   * - C-free native targets
     - **No C compiler** at user-program compile time; the kcc driver emits
       machine code and the container itself.
   * - Compiler self-build
     - **Yes** — bootstrap links each stage with ``gcc -std=c2x ... -lm
       -lgmp`` (``pkg/bootstrap/bootstrap.go:373-407``).
   * - External linker / OS interface
     - The native images are static and need no external linker at compile
       time; the OS loader remains an external runtime facility, which is not
       a C23 *language* dependency.

5. Verified defect corrected during this assignment
====================================================

The audit executed ``go test ./pkg/cli -run TestPhaseV121_`` and it FAILED:
the self-hosted compiler source did not compile, with
``implicit declaration of function 'indexRight'``. Root cause: a single call
site in ``src/compiler/native_value.kark`` (``natNativeCodecIn``) used the
non-existent accessor ``indexRight``; the correct accessor is ``indexExpr``
(``src/compiler/ast.kark:585``). Because the Go front end transpiles the
compiler source without semantic checking, the defect surfaced only at the
gcc stage — breaking **every** gate that needs kcc.

This was corrected and is guarded by ``TestPhaseFoundation_KCCSourceCompiles``.
See ``KARKAIN-IMPLEMENTATION-BASELINE.rst`` for the full evidence.
