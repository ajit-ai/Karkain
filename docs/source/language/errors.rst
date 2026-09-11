.. _errors:

Errors
======

Runtime error model (Phase 100)
-------------------------------

Runtime errors report a **source location** and exit with code 1:

.. code-block:: text

   runtime error: division by zero at main.kark:5
   runtime error: index out of range at main.kark:12
   runtime error: string index out of range at main.kark:18

Checked operations
------------------

The following operations are bounds-checked and produce a runtime error
on failure:

* **Division / modulo by zero**
* **Array index out of range**
* **String index out of range**

Unchecked operations silently return zero (pre-Phase-100 behaviour).

Stack traces (Phase 101)
------------------------

When a runtime error occurs the engine prints a **stack trace** showing
the call chain up to ``KARKAIN_MAX_FRAMES`` (128) deep:

.. code-block:: text

   runtime error: division by zero at main.kark:3
   stack:
     inner:2
     outer:5
     main:9

The stack is recorded by ``karkain_frame_enter`` /
``karkain_frame_leave`` calls emitted around every function body.

Cross-engine parity
-------------------

Both the Go front end and the self-hosted kcc engine produce **identical**
runtime-error messages and stack traces for the same input.
