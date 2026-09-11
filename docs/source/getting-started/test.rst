====
Test
====

``karkain test`` discovers and runs native test files. A test file is any
``*_test.kark`` file whose ``func test_*()`` functions become individual test
cases.

Usage
=====

.. code-block:: console

    $ karkain test [dir] [--filter <substring>]

- ``dir`` — a directory (recursive ``*_test.kark`` discovery) or a single
  ``*_test.kark`` file. Defaults to ``.``.
- ``--filter <substring>`` — run only tests whose identity contains the
  substring. Results stay deterministic.

Example ``math_test.kark``:

.. code-block:: karkain

    func test_addition() {
        assert(1 + 1 == 2)
    }

    func test_values() {
        assert_eq(6, 2 * 3)
        assert_ne(4, 5)
    }

Assertions
==========

The language provides three assertion builtins. Any failure aborts the test
with a source location and fails the run:

- ``assert(cond)`` — fail when ``cond`` is false
- ``assert_eq(actual, expected)`` — fail when the values differ, printing
  ``expected`` and ``actual``
- ``assert_ne(actual, expected)`` — fail when the values are equal

.. code-block:: console

    assertion failed: assert_eq: assert_eq(actual, expected)  [file.kark:5]
      expected: 5
      actual:   4

``std.testing`` module
======================

The importable :doc:`/stdlib/testing` module layers higher-level helpers on
the atomic assertions:

- ``test_expect_true(cond)`` / ``test_expect_false(cond)``
- ``test_expect_eq(actual, expected)`` / ``test_expect_ne(actual, expected)``
- ``test_fail()`` — always fails (unreachable / not-yet-implemented marker)
- ``test_pass_count(results)`` / ``test_fail_count(results)`` —
  count truthy / falsy entries in an array of check results
- ``test_all_pass(results)`` — true when every check passed
- ``test_summary(label, results)`` — builds a ``"label: N passed; M failed"``
  string

Test runner output
==================

Each test reports PASS or FAIL; the run ends with the summary line
``N passed; M failed; S skipped; T total`` on both engines.

.. code-block:: console

    $ karkain test conformance/
      PASS test_integer_arithmetic      1.2ms
      PASS test_recursion_fib           1.5ms
      PASS test_while_loop              0.8ms

    === Test Summary: 3 tests, 3 passed, 0 failed, 0 skipped ===

Exit codes
==========

- ``0`` — all tests passed
- ``4`` — one or more tests failed (``ExitTest``)
- ``3`` — a test failed to compile

Known limitation
================

The synthesized test driver does not yet support ``import`` statements
(Phase 96 boundary); ``stdlib`` modules are validated through ``karkain run``
instead of ``karkain test``.

.. seealso::

   :doc:`profile` — measure tests and programs.
   :doc:`/stdlib/testing` — the assertion/check helper module.