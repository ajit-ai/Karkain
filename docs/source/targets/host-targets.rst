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
      x86_64-linux    64-bit little-endian; objects ELF / executables ELF; ...
      aarch64-linux   64-bit little-endian; objects ELF / executables ELF; ...
      wasm32-wasi     32-bit little-endian; objects WASM / executables WASM; ...
    Default: native

The host triple names architecture, OS and environment/ABI
(``arch-os-env``). On the Windows x86_64 host that is ``x86_64-windows``
(the GNU/MinGW-w64 toolchain flavor is implied).

The ``pkg/target`` model
========================

Host detection lives in the Karkain-owned ``pkg/target`` package,
alongside the whole target model:

- **Architecture** — ``x86_64``, ``aarch64``, ``wasm32`` (plus
  ``unknown``)
- **OS** — ``windows``, ``linux``, ``wasi``
- **Environment/ABI** — ``gnu``, ``msvc``, ``musl`` (recognized but not
  buildable)
- **Canonical short triples** — ``x86_64-windows``, ``x86_64-linux``,
  ``aarch64-linux``, ``wasm32-wasi`` (see :doc:`target-triples`)
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

.. seealso::

   :doc:`/tools/target` — the ``karkain target`` command.
   :doc:`target-triples` — accepted triple forms and normalization.
   :doc:`cross-compilation` — what builds where, and the honest matrix.