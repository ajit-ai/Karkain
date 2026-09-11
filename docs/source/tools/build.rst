=====
Build
=====

``karkain build`` compiles a ``.kark`` file into a native executable
without running it.

For the full build documentation — usage, options, incremental builds,
engine selection and exit codes — see :doc:`/getting-started/build`.

Summary
=======

- ``karkain build <file.kark> [-o <output>]`` — compile to a native
  executable.
- ``--target <triple>`` — cross-compile for a different target (see
  :doc:`/targets/cross-compilation`).
- ``--incremental`` — reuse the content-addressed cache in
  ``.karkain-cache/`` for fast no-op rebuilds.
- ``-c / --compile-only`` — emit C without invoking gcc/clang.
- ``--engine go|kcc`` — select the compiler engine (default ``kcc``).

.. code-block:: console

    $ karkain build hello.kark
    $ karkain build hello.kark --target x86_64-linux -o hello_linux
    $ karkain build main.kark --incremental

Exit codes: ``0`` success, ``3`` compilation failure, ``6`` environment
failure (no usable C compiler).

.. seealso::

   :doc:`/getting-started/build` — complete build documentation.
   :doc:`/getting-started/run` — compile and run in one step.
   :doc:`/targets/cross-compilation` — building for a different
   platform.