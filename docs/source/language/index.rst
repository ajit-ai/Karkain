.. _language-guide:

Karkain Language Guide
======================

Karkain is a **statically typed** language compiled to C23. Programs are
written in ``.kark`` files and executed through one of two engines:

* **Go front end** — the reference compiler (lex → parse → sema → C codegen → gcc).
* **Self-hosted kcc engine** — the primary engine for ``check`` / ``build`` /
  ``run`` / ``test`` (``src/compiler/*.kark`` → C23 → native executable).

Future road-map targets heterogeneous computing (CPU, GPU via WGSL,
NPU via vendor adapters), but the current surface is single-threaded
native code with opt-in concurrency primitives (Phase 107).

.. toctree::
   :maxdepth: 2
   :caption: Contents

   fundamentals
   types
   variables
   constants
   functions
   control-flow
   structs
   enums
   modules
   errors
   strings
   collections
   memory-model
   concurrency
