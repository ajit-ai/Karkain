==================
Cross-Compilation
==================

Phase 111 made cross-compilation explicit: ``karkain build --target
<triple>`` compiles for the requested target through Karkain-owned
compiler-driver selection, and a missing cross-linker fails
deterministically instead of silently producing a host binary.

Usage
=====

.. code-block:: console

    $ karkain build hello.kark --target <triple> [-o <output>]

The ``--target`` switch accepts any :doc:`target-triples` form; long
forms such as ``x86_64-unknown-linux-gnu`` are normalized at parse time.

Supported matrix
================

On this documentation's reference host (Windows x86_64), the honest
matrix is:

.. list-table::
   :widths: 24 14 62
   :header-rows: 1

   * - Target
     - Status
     - Notes
   * - ``x86_64-windows``
     - **PASS**
     - Native build; verifiable here. Emits a real PE32+ executable
       whose x86-64 machine field is validated on the host.
   * - ``x86_64-linux``
     - N/A
     - No cross-linker on PATH in the reference environment. The
       mechanism is fully implemented — see "Toolchain discovery"
       below — but artifacts are unverifiable on this host.
   * - ``aarch64-linux``
     - N/A
     - Same story as ``x86_64-linux``: mechanism implemented, needs a
       triple-prefixed GNU cross-gcc on PATH to actually build.
   * - ``wasm32-wasi``
     - :production-candidate:`Production Candidate`
     - The dedicated Karkain-owned WASM backend emits the module directly
       (no cross-linker needed). Phase 123 added the WASI boundary: exit
       codes, stderr diagnostics and ``getArgs()``.

A compiler that does not have a cross-linker installed gets a
deterministic failure (below), not a wrong-architecture binary.

Toolchain discovery
===================

The C-driver selection in ``pkg/codegen/cross_target.go`` works in two
modes:

- **Same-machine targets** (``SameMachine(host)``) compile with the
  historical host probe — ``$CC`` override, then ``gcc``, ``clang``,
  and MSVC ``cl.exe`` on Windows — with byte-identical flags
  (``-std=c2x -O0 -lgmp``, ``-mconsole`` on Windows).
- **Cross targets** probe conventional **triple-prefixed GNU
  cross-gcc** first, then **clang with an explicit ``--target``**:

  - ``x86_64-linux`` → ``x86_64-linux-gnu-gcc``,
    ``x86_64-pc-linux-gnu-gcc``, ``x86_64-unknown-linux-gnu-gcc``,
    then ``clang --target=x86_64-unknown-linux-gnu``
  - Windows cross targets → ``x86_64-w64-mingw32-gcc`` then
    ``clang --target=x86_64-w64-mingw32``

Missing cross-linker
====================

When no cross toolchain is on PATH, the build fails deterministically
with a ``ToolchainError`` (exit 6) that lists exactly what was searched
and what to install:

.. code-block:: console

    $ karkain build hello.kark --target x86_64-linux -o hello_linux
    no cross-linker available for target 'x86_64-linux' on host
    'x86_64-windows' (searched: 'x86_64-linux-gnu-gcc', ...); install a C
    cross-toolchain for x86_64-linux (see `karkain target`) and ensure it
    is on PATH — the host toolchain can neither link nor execute
    x86_64-linux output
    $ echo $?    # 6

There is **never** a silent fallback to the host toolchain and never a
wrong-architecture binary.

Cross-run
=========

Running a foreign binary on the host is impossible without an emulator
or remote target, so ``karkain run --target <foreign>`` is refused with
a build-only hint:

.. code-block:: console

    $ karkain run hello.kark --target x86_64-linux
    cannot run a x86_64-linux binary on the host (x86_64-windows):
    cross-run requires an emulator or a remote target; use `karkain build
    --target x86_64-linux -o <path>` to build only

Generated C self-description
============================

Every generated C file self-describes its target via a header comment
and preprocessor defines:

.. code-block:: c

    /* karkain-target: x86_64-linux */
    #define KARKAIN_TARGET_ARCH_X86_64 1
    #define KARKAIN_TARGET_OS_LINUX 1

Boundaries
==========

- In-language ``target.os`` / ``target.arch`` builtins are planned
  post-111 (they would require kcc-parity changes) and are not
  available in Karkain source yet.
- ``pkg/backends`` for the native engine produce host binaries; the
  ``native-link`` pipeline is a host-object/linker path unaffected by
  cross-builds.

Gates
=====

Phase-111 behavior is locked by ``pkg/cli/phase111_cross_compile_test.go``
and ``pkg/target/triple_test.go`` (parse/normalize/host-vs-foreign
matrix).

.. seealso::

   :doc:`/tools/target` — the ``karkain target`` command.
   :doc:`target-triples` — the accepted triple forms.
   :doc:`host-targets` — host detection and the ``pkg/target`` model.
   :doc:`/tools/build` — ``karkain build`` options.