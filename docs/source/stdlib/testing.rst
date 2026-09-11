.. _stdlib-testing:

std.testing — Assertion and result helpers
==========================================

:implemented:`Implemented` — importable on both engines, byte-identical.

``std.testing`` layers higher-level check helpers and counting utilities on
top of the language's atomic assertion builtins (``assert`` / ``assert_eq`` /
``assert_ne``), which are available on both engines. It is written in
canonical Karkain (untyped parameters, no type annotations) and parses and
compiles identically on the Go engine and the self-hosted ``kcc`` engine.

Importing
=========

.. code-block:: karkain

   import std.testing

.. note::

   The ``karkain test`` driver does not yet support ``import`` statements
   (Phase 96 boundary). ``std.testing`` is therefore validated through
   ``karkain run``, exactly like the other ``stdlib`` modules.

Module reference
================

Assertions
----------

``test_expect_true(cond)``
   Checks that ``cond`` is truthy. Fails the program otherwise.
   Returns: nothing

``test_expect_false(cond)``
   Checks that ``cond`` is falsy. Fails the program otherwise.
   Returns: nothing

``test_expect_eq(actual, expected)``
   Checks that ``actual`` equals ``expected``. Fails the program otherwise.
   Returns: nothing

``test_expect_ne(actual, expected)``
   Checks that ``actual`` does not equal ``expected``. Fails the program
   otherwise.
   Returns: nothing

``test_fail()``
   Always fails the current test. Use it to mark an unreachable or
   not-yet-implemented branch: reaching the call is a hard failure.
   Returns: nothing

Result counting
---------------

``test_pass_count(results)``
   Counts the truthy entries in an array of check results.
   Returns: ``int``

``test_fail_count(results)``
   Counts the falsy entries in an array of check results, i.e. the number of
   failed checks. Equivalent to ``len(results) - test_pass_count(results)``.
   Returns: ``int``

``test_all_pass(results)``
   Returns ``true`` when every entry in ``results`` is truthy.
   Returns: ``bool``

``test_summary(label, results)``
   Builds a printable ``"label: N passed; M failed"`` line for an array of
   check results. Returns a string, so callers decide how to print.
   Returns: ``string``

Example
=======

Soft checks accumulate a results array, then ``test_summary`` turns them into
a single report line:

.. code-block:: karkain

   import std.testing

   func main() {
       let results = []
       let nums = [1, 2, 3]

       push(results, array_contains_ok(nums))
       push(results, array_length_ok(nums))
       push(results, always_ok())

       println(test_summary("suite-a", results))   // suite-a: 3 passed; 0 failed
   }

   func array_contains_ok(nums) {
       test_expect_true(contains(nums, 2))
       return contains(nums, 2)
   }

   func array_length_ok(nums) {
       test_expect_eq(len(nums), 3)
       return len(nums) == 3
   }

   func always_ok() {
       return true
   }

Hard assertions (``test_expect_*`` / ``test_fail``) abort the program
immediately with the engine's assertion diagnostic; soft checks collect a
boolean per check and let ``test_summary`` report the batch at the end.