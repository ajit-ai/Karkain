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
     - **Self-hosted (kcc)** and Go. Phase A1 wired the same whole-program
       driver to the Linux container, so the ELF is emitted by kcc through
       the same scan/plan/lower sequence as the other two targets.
   * - ``native-x86_64-macos``
     - Mach-O PIE (``.macho``)
     - **Self-hosted (kcc)** and Go. Phase B wired the same driver to the
       Mach-O container. The image is a real position-independent
       executable (``MH_PIE``) with a writable ``__DATA`` arena and a
       decodable rebase stream.

All three targets are kcc-owned: ``kccNativeOwnedTargets`` has no member
left to refuse, and nothing refuses with ``error[K116]`` any more.

The self-hosted driver lowers ``for x in arr`` for an **array variable**,
including nested loops, ``break``/``continue`` and an index inside the loop
body. The iterator must be a variable with a frame-resident header: a literal
``for x in [1, 2]`` is refused by both engines. Executed PE goldens and a
byte-for-byte Linux ELF differential against the Go oracle live in
``pkg/cli/phase151a9i_forin_test.go``; the machine-readable ledger is
``docs/inventory/native-feature-matrix.json``. A ``push()`` program built
through the native CLI target is still refused by the PE patch guard -- a
pre-existing defect pinned by the same gate, not a for-in limitation.

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
- The Go engine remains a supported, documented path for all three
  targets. It is the **reference** implementation the native differential
  compares against, not a required fallback.

Verified status of this claim
-----------------------------

``native-x86_64-windows`` is marked self-hosted because kcc-built
images have been **executed** and produced correct output
(``pkg/cli/phase151_kcc_native_test.go``). The Linux ELF is emitted by
kcc through the same driver and is compared byte for byte against the Go
oracle; its **execution** evidence comes from the Linux CI leg, because
this development host is windows/amd64 with no WSL and no Docker.

The macOS row is marked self-hosted on **structural** evidence only:
``parseMachO`` validates ``MH_PIE``, the segment set and protections,
page-aligned placement, the entry, and decodes the rebase stream, but no
Mach-O image has been **executed**, because this repository has no macOS
runner and no Intel-Mac host in its history. Every Mach-O claim since
increment 149 has been structural, and the v1.2.0 release does not change
that.

.. seealso::

   :doc:`/tools/target` — the ``karkain target`` command.
   :doc:`target-triples` — accepted triple forms and normalization.
   :doc:`cross-compilation` — what builds where, and the honest matrix.