========
Workflow
========

A typical Karkain development loop — scaffold, write, build, run, test,
format, lint, then cross-compile when needed.

1. Scaffold the project
=======================

.. code-block:: console

    $ karkain new my_project
    $ cd my_project

Creates ``karkain.toml``, ``src/main.kark`` and ``tests/main_test.kark``.

2. Write code
=============

.. code-block:: karkain

    // src/main.kark
    func square(n) {
        return n * n
    }

    println(square(7))

3. Type-check
=============

``karkain check`` validates syntax and semantics without producing a binary:

.. code-block:: console

    $ karkain check src/main.kark
    [ok]

4. Build
========

.. code-block:: console

    $ karkain build src/main.kark -o build/app.exe

5. Run
======

.. code-block:: console

    $ karkain run src/main.kark
    49

6. Write tests
==============

.. code-block:: karkain

    // tests/math_test.kark
    func test_square() {
        assert_eq(49, square(7))
    }

7. Run tests
============

.. code-block:: console

    $ karkain test tests/
    ...
    === Test Summary: 1 test, 1 passed, 0 failed, 0 skipped ===

8. Format
=========

``karkain fmt`` canonicalizes formatting; ``--check`` only verifies:

.. code-block:: console

    $ karkain fmt src/main.kark
    $ karkain fmt --check src/main.kark   # non-zero if not canonical

9. Lint
=======

``karkain lint`` runs the full front-end analysis (including the borrow
checker) and surfaces warnings:

.. code-block:: console

    $ karkain lint src/main.kark

10. Cross-compile (when needed)
===============================

Build for another architecture/OS with an explicit ``--target`` triple:

.. code-block:: console

    $ karkain build src/main.kark --target x86_64-linux -o build/app_linux
    $ karkain target                      # list the supported matrix

Cross-built binaries cannot be run on the host (``karkain run --target
<foreign>`` is refused with an emulator/remote-target hint, exit code ``6``).

Summary
=======

- ``check`` — type checking only
- ``build`` — compile to a native executable
- ``run`` — compile and execute in one step
- ``test`` — discover and run ``*_test.kark``
- ``fmt`` — canonical formatting (``--check`` to verify)
- ``lint`` — full front-end analysis and warnings
- ``target`` — the supported compilation targets

.. seealso::

   The steps are documented in turn: :doc:`build`, :doc:`run`, :doc:`test`,
   :doc:`debug`, :doc:`profile` and, for cross-compilation,
   :doc:`/targets/cross-compilation`. For the full command surface see
   :doc:`/tools/cli`.