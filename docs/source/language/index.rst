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
   control-flow
   functions
   structs
   enums
   strings
   collections
   modules
   errors
   memory-model
   concurrency

How to read this guide
----------------------

The chapters are ordered so that each one depends only on the chapters above
it, so a new programmer can read them straight through:

.. code-block:: text

   fundamentals        lexical structure, literals, first program shape
   types               int / float64 / bool / string, array, map
   variables           let, var, inference, shadowing
   constants           compile-time constants
   control-flow         if / while / for / for-in / match / break / continue
   functions           parameters, return types, the record idiom
   structs             records
   enums               tagged unions
   strings             UTF-8 strings and slicing
   collections         arrays and maps
   modules             imports and project layout
   errors              Result / Option, `?` propagation, runtime errors
   memory-model        ownership, borrowing, references
   concurrency         channels, actors, tasks

``control-flow`` deliberately precedes ``functions``: every construct in it can
be read without a function around it, and ``functions`` then has control flow
already in hand. ``strings`` and ``collections`` sit with ``structs`` and
``enums`` because they are the remaining data structures; ``errors`` follows
``modules`` because it is where ``Result``/``Option`` and ``?`` become usable
end to end.

This guide *teaches* the language. For the precise normative definition see
:doc:`/reference/stable-api` and ``SPEC.md`` at the repository root; for how the
compiler and runtime are built see :doc:`/compiler/index`.

Karkain reference
~~~~~~~~~~~~~~~~~

* :doc:`/getting-started/index` — install, first project, workspace
* :doc:`/stdlib/index` — the shipped standard library
* :doc:`/examples/index` — the runnable example corpus
* :doc:`/tools/index` — ``karkain`` CLI commands
* :doc:`/targets/index` — build targets and accelerators
* :doc:`/status/index` — compatibility, scope and release status
* :doc:`/development/index` — contributing and release process
