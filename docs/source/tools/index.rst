=====
Tools
=====

Karkain ships a real CLI covering build, run, test, fmt, lint, debug,
prof, target, pkg, lsp, workspace, clean, explain and bench.

.. toctree::
   :maxdepth: 2

   cli
   build
   run
   test
   debug
   profile
   fmt
   target
   lsp
   packages

Quick start
===========

.. code-block:: console

    $ karkain run hello.kark          # compile and run
    $ karkain build hello.kark        # compile to native executable
    $ karkain test                    # discover and run *_test.kark files
    $ karkain fmt .                   # format all .kark files in place
    $ karkain prof hello.kark         # profile a program
    $ karkain target                  # list supported targets

The toolchain has two compiler engines. The default is ``kcc`` (the
self-hosted compiler); the Go front end is available via
``--engine go`` or ``KARKAIN_ENGINE=go``.

.. seealso::

   :doc:`/getting-started/installation` — install the toolchain.
   :doc:`/getting-started/build` — build workflow details.
   :doc:`/getting-started/run` — run workflow details.
