===
Run
===

``karkain run`` compiles a ``.kark`` file and executes it in one step.

For the full run documentation — options, engine selection, runtime
errors and exit codes — see :doc:`/getting-started/run`.

Summary
=======

- ``karkain run <file.kark> [--engine go|kcc] [--target <target>]`` —
  compile and run in one step.
- The program's stdout passes through to the terminal unchanged;
  diagnostics and tracing go to stderr.
- Runtime errors abort with a source-located diagnostic and a function
  stack dump.

.. code-block:: console

    $ karkain run hello.kark
    Hello, Karkain!

Exit codes: ``0`` success, ``1`` program failure, ``3`` compilation
failure, ``6`` environment failure (no C compiler, or a cross-run
attempt).

.. seealso::

   :doc:`/getting-started/run` — complete run documentation.
   :doc:`/getting-started/build` — compile to a native executable
   without running.