==========================
Your First Karkain Project
==========================

The :doc:`first-program` page showed a single file. Real projects split
code into a few ``.kark`` modules. This page builds the canonical minimal
multi-file project using only stable, supported syntax.

.. contents:: Sections
   :local:
   :depth: 1

The project shape
=================

Create a directory ``greeter/`` with two sibling files:

.. code-block:: text

    greeter/
      main.kark    # application entry point
      math.kark    # a sibling module with public exports

``main.kark``:

.. code-block:: karkain

    import math

    func main() {
        print(math.twice(21))
    }

``math.kark``:

.. code-block:: karkain

    public func twice(n) {
        return n * 2
    }

What you are using
==================

* ``import math`` imports the sibling ``math.kark`` module that lives in
  the same directory.
* ``public`` marks ``twice`` as an export. A private sibling function is
  not callable from another module.
* ``math.twice(21)`` is a qualified call against the imported module's
  export set.
* ``func main()`` plus the top-level ``import`` describe the program; the
  pipeline is the same as the single-file case.

Build and run
=============

.. code-block:: console

    $ karkain check main.kark
    [ok]

    $ karkain build main.kark
    $ karkain run main.kark
    42

Every step uses only stable Beta 1 syntax; the same commands work through
both engines (the Go front end and the default self-hosted ``kcc``).

Now add a test
==============

Add ``math_test.kark`` next to the others:

.. code-block:: karkain

    import math

    func test_twice() {
        assert_eq(math.twice(21), 42)
    }

Run it:

.. code-block:: console

    $ karkain test math_test.kark
    1 passed; 0 failed; 0 skipped; 1 total

Layout conventions
==================

For a full project layout (``src/``, ``tests/``, ``lib/``, ``karkain init``
scaffolding and the manifest) see :doc:`project-layout`. For importing the
standard library see :doc:`modules`.

.. seealso::

   :doc:`modules` — module imports and the ``public`` export model.
   :doc:`test` — writing and running ``*_test.kark`` files.
   :doc:`project-layout` — organizing a larger project.
   :doc:`workspace` — when one repository needs several member packages.