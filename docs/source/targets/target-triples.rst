==============
Target Triples
==============

A target triple names the platform a program is compiled **for**,
independently of the platform the toolchain runs **on** (the host).
Karkain owns this model: ``pkg/target`` parses, validates and normalizes
triples on its own terms and never delegates triple handling to an
external compiler framework.

Canonical short triples
=======================

The canonical short form is ``<arch>-<os>``. Karkain supports eight
(Phase 139 added the Windows-ARM64, RISC-V and macOS rows):

.. list-table::
   :widths: 24 76
   :header-rows: 1

   * - Triple
     - Meaning
   * - ``x86_64-windows``
     - AMD64 / Windows (MinGW-w64 GNU toolchain flavor by default)
   * - ``aarch64-windows``
     - ARM64 / Windows (``aarch64-w64-mingw32`` cross toolchain or clang)
   * - ``x86_64-linux``
     - AMD64 / Linux (glibc, GNU)
   * - ``aarch64-linux``
     - ARM64 / Linux (glibc, GNU)
   * - ``riscv64-linux``
     - RISC-V 64-bit / Linux (parse + error paths; no backend builds it yet)
   * - ``x86_64-macos``
     - AMD64 / macOS (clang ``--target=x86_64-apple-macosx`` only)
   * - ``aarch64-macos``
     - ARM64 / macOS, Apple Silicon (clang ``--target=aarch64-apple-macosx`` only)
   * - ``wasm32-wasi``
     - WebAssembly 32-bit / WASI

Long-form normalization
=======================

Conventional vendor- and ABI-bearing triples are accepted and normalized
to the canonical short form (the vendor component is dropped):

.. code-block:: text

    x86_64-pc-windows-msvc      → x86_64-windows
    x86_64-unknown-linux-gnu    → x86_64-linux
    aarch64-unknown-linux-gnu   → aarch64-linux
    x86_64-windows-gnu          → x86_64-windows
    riscv64-unknown-linux-gnu   → riscv64-linux
    x86_64-apple-macosx         → x86_64-macos
    x86_64-darwin               → x86_64-macos
    aarch64-apple-macosx        → aarch64-macos

When the triple does not name an environment/ABI, Karkain assumes the
default for the OS: GNU for Windows and Linux (its native pipeline links
through MinGW-w64 gcc on Windows and glibc gcc on Linux); macOS and WASI
targets carry no ABI (macOS builds go through clang by design).

Model
=====

The ``pkg/target`` model components are:

- **Architecture** — ``x86_64``, ``aarch64``, ``riscv64``, ``wasm32``
- **OS** — ``windows``, ``linux``, ``macos``, ``wasi``
- **Environment/ABI** — ``gnu``, ``msvc``, ``musl`` (recognized, not
  buildable)

ABI sanity checks reject combinations no real backend supports, e.g.
``msvc`` on Linux or macOS, ``gnu`` on macOS or WASI, ``musl`` for any
current backend, ``riscv64`` outside Linux, ``macos`` outside
x86_64/aarch64, and ``wasm32`` paired with any OS other than ``wasi``.

Invalid components produce precise ``ParseError`` diagnostics listing the
supported values:

.. code-block:: console

    $ karkain build hello.kark --target sparcv9-solaris
    unsupported architecture 'sparcv9' in target 'sparcv9-solaris'
    (supported architectures: x86_64, aarch64, riscv64, wasm32)

``Features`` model
==================

Every parseable target yields a ``Features`` record describing its ABI
facts — derived from the model, never guessed from the host:

- **Pointer width** — 64 bits for x86_64/aarch64/riscv64, 32 for wasm32
- **Endianness** — little-endian for all current targets
- **Object / executable formats** — ELF, PE/COFF, Mach-O, or WASM
- **ABI name** — e.g. Microsoft x64, System V x86_64/glibc,
  System V x86_64/Apple, WebAssembly/WASI
- **Runtime variant** — ``windows``, ``linux-gnu``, ``macos``, or ``wasi``
- **Linker requirement** — what an external C toolchain must supply
  (MinGW triple, ``<arch>-linux-gnu-gcc``, clang ``--target`` for macOS,
  or "none" for WASM because the backend emits the module directly)

Host vs foreign
===============

- **``SameMachine``** compares architecture + OS — machine kind, not
  triples. ``x86_64-pc-windows-msvc`` and ``x86_64-windows`` are the
  same machine even though their ABIs differ; ``x86_64-linux`` is never
  the same machine as a Windows host.
- **``IsHost``** reports whether a target equals the running toolchain's
  platform. Host detection is used only to derive the implicit
  ``native`` target and to gate cross-run — never as a substitute for an
  explicit target.

In-language builtins
====================

``target.os`` / ``target.arch`` builtins that expose the *current*
target to Karkain source are **planned (post-111)**. They would require
kcc-parity changes and are not available today.

.. seealso::

   :doc:`host-targets` — host detection and the full ``pkg/target``
   model.
   :doc:`cross-compilation` — building for a target in the matrix.
   :doc:`/tools/target` — the ``karkain target`` command output.