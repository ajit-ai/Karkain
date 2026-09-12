Fundamentals
============

:implemented:`Implemented` — the fundamentals examples are real, checked-in
programs under ``examples/01_fundamentals/`` (12 targets) that round-trip the
language surface on **both** engines (the Go front end and the self-hosted
``kcc`` engine), producing byte-identical stdout and exit codes. Each file is
gated by ``pkg/cli/phase114_examples_test.go``.

A deeper multi-file round-trip corpus (`variables → functions → recursion →
arrays → strings → control flow → structs → maps → types → match → methods →
modules → application layout`) also lives under ``examples/language_foundation/``
(13 targets, Phase 102).

.. list-table:: examples/01_fundamentals/
   :widths: 30 70
   :header-rows: 1

   * - File
     - Demonstrates
   * - ``01_hello_world.kark``
     - The obligatory first program
   * - ``02_variables.kark``
     - ``let`` / ``var`` bindings and reassignment
   * - ``03_constants.kark``
     - ``const`` immutable local bindings (Phase 112)
   * - ``04_functions.kark``
     - Functions, returns and composition
   * - ``05_conditionals.kark``
     - ``if`` / ``else if`` / ``else`` chains (grade classification)
   * - ``06_loops.kark``
     - ``while`` loops, ``push``, reversal, Fibonacci accumulation
   * - ``07_strings.kark``
     - Concatenation, ``len``, indexing, slices, casts
   * - ``08_arrays.kark``
     - Index access, mutation, slices, 2-D arrays, iteration
   * - ``09_maps.kark``
     - Map construction/access plus ``std.collections`` helpers
   * - ``10_structs.kark``
     - Record types, literals, field reads and mutation
   * - ``11_match.kark``
     - ``match`` on ``Option`` and literal-bound values
   * - ``12_casts.kark``
     - ``int(f)`` / ``str(n)`` / float-mixing casts

Hello world
-----------

``01_hello_world.kark``:

.. code-block:: kark

   func main() {
       println("Hello, Karkain!")
   }

Run it:

.. code-block:: console

   $ karkain run examples/01_fundamentals/01_hello_world.kark

Expected output:

.. code-block:: text

   Hello, Karkain!

A walk through the beginner path:

1. ``01_hello_world.kark`` — print to stdout
2. ``02_variables.kark`` + ``03_constants.kark`` — bindings and constants
3. ``04_functions.kark`` — program structure
4. ``05_conditionals.kark`` + ``06_loops.kark`` — control flow
5. ``07_strings.kark`` + ``08_arrays.kark`` — the core collections
6. ``09_maps.kark`` + ``10_structs.kark`` — associative and record data
7. ``11_match.kark`` + ``12_casts.kark`` — option matching and numeric casts

Match (excerpt)
---------------

From ``11_match.kark`` — literal-bound arms and ``Some``/``None``:

.. code-block:: kark

   func triple(x) {
       return match x {
           Some(v) => v * 3
           None => 0
       }
   }

.. note::

   For computed values, prefer exhaustive literal arms: wildcard-match
   behavior is not yet reliable on both engines.

.. seealso::

   :doc:`/examples/algorithms` — more recursive programs with expected output.
   :doc:`/language/index` — the language guide these examples exercise.
   :doc:`/getting-started/first-program` — the guided first program.