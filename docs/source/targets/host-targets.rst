============
Host Targets
============

Karkain detects the platform it runs on and reports it as a canonical
short target triple.

``karkain target``
==================

The ``Host:`` line of ``karkain target`` is the exact host triple from
``pkg/target``:

.. code-block:: console

    $ karkain target
    Host: x86_64-windows
    Supported targets:
      native         (host default; x86_64-windows)
      c23            (C23 source output; x86_64-windows)
      native-link    (Phase 84 object/linker pipeline)
      wasm32-wasi    (WebAssembly WASI module)
      x86_64-windows 64-bit little-endian; objects PE/COFF / executables PE/COFF; ...
      aarch64-windows 64-bit little-endian; objects PE/COFF / executables PE/COFF; ...
      x86_64-linux    64-bit little-endian; objects ELF / executables ELF; ...
      aarch64-linux   64-bit little-endian; objects ELF / executables ELF; ...
      riscv64-linux   64-bit little-endian; objects ELF / executables ELF; ...
      x86_64-macos    64-bit little-endian; objects Mach-O / executables Mach-O; ...
      aarch64-macos   64-bit little-endian; objects Mach-O / executables Mach-O; ...
      wasm32-wasi     32-bit little-endian; objects WASM / executables WASM; ...
    Default: native

The host triple names architecture, OS and environment/ABI
(``arch-os-env``). On the Windows x86_64 host that is ``x86_64-windows``
(the GNU/MinGW-w64 toolchain flavor is implied).

The ``pkg/target`` model
========================

Host detection lives in the Karkain-owned ``pkg/target`` package,
alongside the whole target model:

- **Architecture** — ``x86_64``, ``aarch64``, ``riscv64``, ``wasm32`` (plus
  ``unknown``)
- **OS** — ``windows``, ``linux``, ``macos``, ``wasi``
- **Environment/ABI** — ``gnu``, ``msvc``, ``musl`` (recognized but not
  buildable)
- **Canonical short triples** — ``x86_64-windows``, ``aarch64-windows``,
  ``x86_64-linux``, ``aarch64-linux``, ``riscv64-linux``,
  ``x86_64-macos``, ``aarch64-macos``, ``wasm32-wasi``
  (see :doc:`target-triples`)
- **Long-form normalization** — ``x86_64-pc-windows-msvc``,
  ``x86_64-unknown-linux-gnu`` and friends parse and normalize to the
  canonical short form; the vendor component is dropped
- **``Features``** — the target data-layout model: pointer width (32/64
  bits), endianness, object/executable formats (ELF, PE/COFF, WASM), ABI
  name, runtime variant and the external-C-toolchain requirement
- **``SameMachine``** — compares architecture + OS (machine kind), not
  triples: ``x86_64-pc-windows-msvc`` and ``x86_64-windows`` are the
  same machine even with different ABIs, while ``x86_64-linux`` can
  never run on a Windows host
- **``IsHost``** — whether a target equals the running toolchain's
  platform

Why host detection matters
==========================

Host detection is used for exactly two things:

1. Deriving the implicit ``native`` target (the default when
   ``--target`` is omitted).
2. Deciding whether a cross-run is possible — cross-run restrictions
   compare machines via ``SameMachine``, never triples.

  Host detection is **never** a substitute for an explicit target: the
  same-machine C-driver selection compiles ``native`` with the historical
  host probe (byte-identical flags), and a foreign target is always
  compiled by cross-compiler discovery (:doc:`cross-compilation`). There is
  no silent host fallback for a foreign triple.

C-free native targets (``native-x86_64-*``)
===========================================

In addition to the triples above, three **C-free machine-code targets**
exist. They emit a linked executable directly, with no C compiler on
``PATH`` — no ``gcc``, ``clang`` or ``cl``. They are built from any host
for any of the three containers; ``run`` requires the host OS to match
the target OS and is refused elsewhere with the Phase-111 build-only
hint (exit 6).

.. list-table::
   :header-rows: 1
   :widths: 26 20 54

   * - Target
     - Container
     - Engine ownership
   * - ``native-x86_64-windows``
     - PE32+ (``.exe``)
     - **Self-hosted (kcc)** and Go. Since Phase 151D kcc emits this
       target itself, through the same whole-program driver that
       ``karkain native-ast`` uses. Every kcc build prints its provenance
       line, and the CLI refuses to write bytes that lack it.
   * - ``native-x86_64-linux``
     - ELF64 (``.elf``)
     - **Go backend only.** kcc refuses with ``error[K116]``. It has no
       ELF emitter for a user program: ``native_elf.kark`` lowers the
       fixed reference corpora that Phase 151C compares against the Go
       oracle, and does not take a program.
   * - ``native-x86_64-macos``
     - Mach-O PIE (``.macho``)
     - **Go backend only.** kcc refuses with ``error[K116]``, for the
       same reason as ELF.

Engine selection vs. target ownership
-------------------------------------

Ownership of a native target and the default engine are two separate
facts, and both are worth stating precisely:

- The **default engine** is the self-hosted engine (kcc) unless
  ``--engine go`` or ``KARKAIN_ENGINE=go`` says otherwise.
- A native target requested under kcc is answered by kcc. Where kcc owns
  the target it emits it; where it does not, it refuses with ``K116``
  and names the target. It never hands the request silently to the Go
  backend — the earlier behaviour, measured in
  ``docs/audit/PHASE-151-BASELINE.md`` §1.1, produced a Go image with no
  signal that the engine had changed.
- To build all three native targets, use the Go engine
  (``--engine go``). That is a supported, documented path, not a
  workaround.

Verified status of this claim
-----------------------------

``native-x86_64-windows`` is marked self-hosted because kcc-built
images have been **executed** and produced correct output
(``pkg/cli/phase151_kcc_native_test.go``). The Linux and macOS rows are
marked Go-backend-only because kcc refuses them by name in its own
source (``src/compiler/main.kark``, ``buildNativeFile``); no
Mach-O-execution claim is made anywhere, since no Intel Mac runner
exists in CI.

.. seealso::

   :doc:`/tools/target` — the ``karkain target`` command.
   :doc:`target-triples` — accepted triple forms and normalization.
   :doc:`cross-compilation` — what builds where, and the honest matrix.