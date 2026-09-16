=================================
Karkain — Official Documentation
=================================

Karkain is a **statically typed systems programming language** for
heterogeneous computing — CPU, GPU, NPU and quantum — with a self-hosted
compiler written in the language itself. It compiles to C23 via GCC/Clang
and ships a real CLI: build, run, test, format, debug, profile, package and
cross-compile.

.. note::

   Status: **Karkain 1.0.0 (Stable)**. The implemented language core, standard
   library and toolchain are functional and regression-tested on two engines
   (the Go front end and the self-hosted ``kcc`` engine), with a documented
   stable core and broad parity for the claimed surface. Large planned
   surfaces (networking, databases, web, GPU/NPU/quantum kernels, an
   advanced package registry) are **not yet implemented**. Nothing on this
   site describes hypothetical features as available. Every advanced
   capability states its real status.

   Public status: **Karkain 1.0.0 (Stable)** (``karkain --version`` →
   ``Karkain Compiler v1.0.0 (... Stable Build)``). See
   :doc:`release-notes` and :doc:`status/beta`.

   Feedback: open an issue at
   https://github.com/ajit-ai/Karkain/issues.

-----------

Quick tour
==========

* **Getting Started** — :doc:`installation </getting-started/installation>`,
  :doc:`a first program </getting-started/first-program>`, and the
  :doc:`project layout </getting-started/project-layout>`.
* **Language** — :doc:`fundamentals </language/fundamentals>`,
  :doc:`types </language/types>`, :doc:`constants </language/constants>`,
  :doc:`functions </language/functions>`,
  :doc:`control flow </language/control-flow>`,
  :doc:`structs </language/structs>`, :doc:`modules </language/modules>`,
  and the :doc:`memory model </language/memory-model>`.
* **Standard library** — every implemented ``std.*`` module, currently
  :doc:`string </stdlib/strings>`, :doc:`collections </stdlib/collections>`,
  :doc:`io </stdlib/io>`, :doc:`encoding </stdlib/encoding>`,
  :doc:`crypto </stdlib/crypto>` and :doc:`testing </stdlib/testing>`.
* **Tools** — the :doc:`CLI </tools/cli>`, :doc:`build </tools/build>`,
  :doc:`run </tools/run>`, :doc:`test </tools/test>`,
  :doc:`debug </tools/debug>`, :doc:`profile </tools/profile>`,
  :doc:`fmt </tools/fmt>` and :doc:`target </tools/target>`.
* **Targets** — :doc:`cross-compilation </targets/cross-compilation>` with
  explicit :doc:`target triples </targets/target-triples>`.
* **Internals** — the compiler :doc:`architecture </compiler/architecture>`,
  :doc:`KCC </compiler/kcc>`, :doc:`HIR </compiler/hir>`,
  :doc:`SSA </compiler/ssa>`, :doc:`pipeline </compiler/pipeline>`,
  :doc:`runtime </compiler/runtime>`, and :doc:`self-hosting </compiler/bootstrap>`.
* **Status** — the honest :doc:`status model </status/index>` and what
  is :doc:`implemented </status/implemented>`.

Getting Karkain
===============

.. reveal a first program inline

Hey there. Create ``hello.kark``:

.. code-block:: karkain

    println("Hello, Karkain!")

Build and run it:

.. code-block:: console

    $ karkain run hello.kark
    Hello, Karkain!

That is the whole loop. From here read :doc:`getting-started/index` for the
full path — installation, first program, build, run, test, debug, profile,
project structure, modules and dependencies.

----------------

Contents
========

The documentation is organized in three layers.

.. toctree::
   :maxdepth: 2
   :caption: Learn Karkain

   getting-started/index
   language/index

.. toctree::
   :maxdepth: 2
   :caption: Karkain Developer

   stdlib/index
   tools/index
   targets/index
   examples/index

.. toctree::
   :maxdepth: 2
   :caption: Karkain Internals

   compiler/index
   development/index

.. toctree::
   :maxdepth: 1
   :caption: Status & Reference

   status/index
   reference/index
   release-notes

----------------

How to read this documentation
==============================

:doc:`getting-started/index` is the entry point for new developers:
installation, a first runnable program, then build → run → test → debug →
profile in the order you will actually reach for them.

:doc:`language/index` is the authoritative guide to the implemented Karkain
language: syntax, variables, constants, types, functions, control flow,
structs, modules, collections, errors and the memory model.

:doc:`stdlib/index` documents the implemented standard-library modules. A
module is only listed when it is importable and tested — ``std.string``,
``std.collections``, ``std.io``, ``std.encoding``, ``std.crypto`` and
``std.testing`` today.

:doc:`tools/index` covers the real CLI — every command, option, exit code
and example.

:doc:`targets/index` explains host and cross-compilation targets, including
the :doc:`Phase-111 cross-compilation </targets/cross-compilation>` behavior.

:doc:`compiler/index` is for contributors: the pipeline from source to
binary, the two engines (Go and self-hosted ``kcc``), the IR layers, the
runtime boundary and the bootstrap/self-hosting story.

:doc:`status/index` defines the vocabulary used everywhere on the site —
Stable, Implemented, Experimental, Developer Preview, Planned and Not Yet
Implemented — so you always know what is real today.

-----------

Project status
==============

.. list-table:: Current overall status
   :widths: 40 60
   :header-rows: 1

   * - Area
     - Status
   * - Language core (variables, functions, control flow, structs, modules)
     - :implemented:`Implemented` — tested on both engines
   * - Standard library
     - :implemented:`Implemented` — 6 importable ``std.*`` modules
   * - CLI (build, run, test, fmt, debug, prof, target, pkg)
     - :implemented:`Implemented`
   * - Cross-compilation
     - :implemented:`Implemented` — explicit triples, deterministic failure
   * - Debugger / profiler
     - :implemented:`Implemented` — Go engine; kcc/wasm deferred
   * - Self-hosted compiler (``kcc``)
     - :implemented:`Implemented` — byte-identical bootstrap
   * - Concurrency runtime
     - :implemented:`Implemented` — channels & actors
   * - SIMD / vector types
     - :implemented:`Implemented` — x86 + ARM lane types
   * - WASM (``wasm32-wasi``)
     - :production-candidate:`Production Candidate` — Go-engine backend,
        wasmtime-gated; WASI exit codes, stderr and ``getArgs()`` implemented
   * - GPU / NPU / quantum compute targets
     - :experimental:`Experimental` — Phase 124: compute-target model and KIR
         lowering boundary; the language surface for kernels stays
         :planned:`Planned`
   * - Networking, databases, web, AI/ML
     - :planned:`Planned` — not yet supported
   * - Package registry
     - :experimental:`Experimental` — local resolver + lockfile; no registry

The authoritative versioned language specification lives in ``SPEC.md``
at the repository root; the :doc:`reference/index` section distills the
parser-implemented reference behavior.

-----------

Legal
=====

Karkain is released under the MIT License. See the repository
``LICENSE`` file.