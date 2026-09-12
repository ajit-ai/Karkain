Developer Tools
===============

:implemented:`Implemented` — this category documents tools *for* Karkain
(the CLI, engines, formatter, LSP, profiler) and carries example programs in
``examples/15_developer_tools/`` that exercise them.

Corpus

.. list-table:: examples/15_developer_tools/
   :widths: 32 68
   :header-rows: 1

   * - File
     - Demonstrates
   * - ``01_hello_toolchain.kark``
     - The write → build → run loop for any environment
   * - ``02_assertions_test.kark``
     - ``karkain test`` discovery of ``*_test.kark`` files +
       language-level ``assert / assert_eq / assert_ne``

The toolchain loop
------------------

.. code-block:: console

   $ karkain check examples/15_developer_tools/01_hello_toolchain.kark
   $ karkain build examples/15_developer_tools/01_hello_toolchain.kark -o bin/hello.exe
   $ karkain run   examples/15_developer_tools/01_hello_toolchain.kark

Run the test file through the real test runner:

.. code-block:: console

   $ karkain test examples/15_developer_tools/02_assertions_test.kark

Expected summary:

.. code-block:: text

   3 passed; 0 failed; 0 skipped; 3 total

What the CLI covers today

* ``karkain check`` — syntax + semantics, exit 3 on error, ``--format=json``
* ``karkain build`` — native executable (PE32+ on Windows, ELF elsewhere)
* ``karkain run`` — build + run in an isolated working directory
* ``karkain test`` — ``*_test.kark`` discovery, ``--filter``, exit 4 on failure
* ``karkain debug`` — trace (frame enter / line) on stderr
* ``karkain prof`` — call counts, inclusive/exclusive timing, flame-graph data
* ``karkain target`` — host triple + supported target matrix
* ``karkain fmt`` / ``karkain lint`` / ``karkain explain`` / ``karkain lsp``

Each command is documented in :doc:`/tools/index`.

.. seealso::

   :doc:`/tools/index` — the full command reference.
   :doc:`/getting-started/test` — the testing workflow.