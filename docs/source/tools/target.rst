======
Target
======

``karkain target`` reports the host platform and the closed set of
targets the toolchain can genuinely produce output for, using Karkain's
own target model (:doc:`/targets/target-triples`).

Usage
=====

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

The ``Host:`` line is the canonical short host triple (arch-os-env), and
the matrix lists every modeled target from the ``pkg/target`` package.

Compute targets
===============

Phase 124 adds the accelerator-target catalog below the ``Default:`` line:

.. code-block:: console

    Compute targets (Phase 124 experimental):
      cpu                  implemented   Reference scalar CPU executor (the default lowering surface)
      gpu-experimental     experimental  Data-parallel kernel executor over tensor/matrix ops (model only; no vendor dependencies)
      npu-experimental     experimental  Single-invocation neural inference over tensor/matrix ops (model only; no vendor dependencies)
      quantum-experimental research      Quantum circuit executor over gate primitives and measurement (spec model only)
      simd                 implemented   CPU executor with lane-vector arithmetic (Phase 106 @simd_* surface)
      wasm32-wasi          experimental  WebAssembly/WASI executor (Phase 108 Karkain-owned wasm backend)

``karkain target <name>`` prints the capability view of one compute target
(capabilities, accepted KIR v1 classes, native tensor operations); unknown
names are a usage error. The exact semantics and the KIR lowering boundary
are documented in :doc:`/targets/compute-targets`.

Cross-compilation
=================

``karkain build --target <triple>`` cross-compiles for a foreign target.
The same-machine targets are compiled with the historical host probe
(byte-identical flags); foreign targets are compiled with triple-prefixed
GNU cross-gcc → clang ``--target``. See
:doc:`/targets/cross-compilation` for the supported matrix and the
deterministic toolchain failure behavior.

Cross-run
=========

``karkain run --target <foreign>`` is **refused** with an explicit
hint — cross-run needs an emulator or remote target, so the CLI requires
a build-only flow:

.. code-block:: console

    $ karkain run hello.kark --target x86_64-linux
    Error: cannot run a x86_64-linux binary on the host (x86_64-windows):
    cross-run requires an emulator or a remote target; use `karkain build
    --target x86_64-linux -o <path>` to build only

Target aliases vs triples
=========================

The legacy aliases remain supported:

- ``native`` — the host platform (``target.Host()``)
- ``c23`` — C23 source output for the host
- ``native-link`` — the Phase 84 object/linker pipeline
- ``wasm32-wasi`` — WebAssembly WASI module (also a canonical triple)

Conventional triples such as ``x86_64-windows``, ``x86_64-linux``,
``aarch64-linux`` and ``wasm32-wasi`` are parsed, validated and
normalized by ``pkg/target``; normalizing long forms
(``x86_64-pc-windows-msvc``) and vendor-bearing triples is handled at
parse time.

Exit codes
==========

- ``0`` — success
- ``6`` — environment failure (a cross build for a target with no
  usable cross-linker on PATH)

.. seealso::

   :doc:`/targets/host-targets` — host detection and the target model.
   :doc:`/targets/cross-compilation` — Phase 111 cross-compilation.
   :doc:`/targets/target-triples` — triple format and normalization.
   :doc:`/targets/compute-targets` — Phase 124 compute-target model.