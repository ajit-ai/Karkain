=============
First Program
=============

A Karkain program is a ``.kark`` file compiled and run through the
``karkain`` CLI.

Hello, World!
=============

Create ``hello.kark``:

.. code-block:: karkain

    println("Hello, Karkain!")

Build and run it:

.. code-block:: console

    $ karkain run hello.kark
    Hello, Karkain!

That is the complete loop. Every Karkain program starts from here.

What just happened
==================

1. ``karkain run hello.kark`` invokes the Go front end (or ``kcc``,
   depending on configuration) to parse the source.
2. The parser produces an AST.
3. The semantic resolver resolves builtins and scope.
4. The C emitter produces a ``.c`` file.
5. GCC or Clang compiles the C to a native executable.
6. The executable runs and prints ``Hello, Karkain!`` to stdout.

You do not need to understand any of these steps to use Karkain. The
CLI handles everything; the pipeline is described in detail in
:doc:`/compiler/pipeline`.

A second program
=================

Create ``count.kark``:

.. code-block:: karkain

    func count_to(n) {
        let i = 1
        while(i <= n) {
            println(i)
            i = i + 1
        }
    }

    count_to(5)

Run it:

.. code-block:: console

    $ karkain run count.kark
    1
    2
    3
    4
    5

Key points:

- Functions are declared with ``func name(params) { body }``.
- ``let`` declares a local variable.
- ``while(cond) { ... }`` is the loop form (parentheses required).
- ``println(value)`` prints to stdout.
- There is no ``main`` function; the top-level scope is the entry point.

What's next
===========

- :doc:`build` — compile to a native executable.
- :doc:`run` — compile and execute in one step.
- :doc:`test` — write and run tests.
- :doc:`project-layout` — organize a real project.