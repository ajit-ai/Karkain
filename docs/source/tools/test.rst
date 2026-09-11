====
Test
====

``karkain test`` discovers and runs native test files.

For the full test documentation — assertions, the ``std.testing`` module,
runner output and exit codes — see :doc:`/getting-started/test`.

Summary
=======

- ``karkain test [dir] [--filter <substring>]`` — discover and run
  ``*_test.kark`` files; any ``func test_*()`` function becomes a test
  case.
- Three assertion builtins: ``assert(cond)``, ``assert_eq(a, b)`` and
  ``assert_ne(a, b)``; see also the :doc:`/stdlib/testing` module.
- ``karkain test --compile [dir]`` runs the compile pass/fail corpus
  (diagnostics lint pipeline, exit 4 on failure).

.. code-block:: console

    $ karkain test conformance/
      PASS test_integer_arithmetic      1.2ms
      PASS test_recursion_fib           1.5ms

    === Test Summary: 3 tests, 3 passed, 0 failed, 0 skipped ===

Exit codes: ``0`` all tests passed, ``4`` one or more tests failed,
``3`` a test failed to compile.

.. seealso::

   :doc:`/getting-started/test` — complete test documentation.
   :doc:`/stdlib/testing` — the test helper module.