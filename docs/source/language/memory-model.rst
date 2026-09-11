.. _memory-model:

Memory Model
============

Value representation
--------------------

Every Karkain value is a tagged union (``Value``) with one of the
following type tags:

``TYPE_INT``
  64-bit signed integer, stored unboxed.

``TYPE_FLOAT64``
  64-bit IEEE 754 double, stored unboxed.

``TYPE_BOOL``
  ``true`` / ``false``.

``TYPE_STRING``
  Length-prefixed UTF-8 byte array (heap-allocated, managed by the
  runtime).

``TYPE_ARRAY``
  Dynamic array of ``Value`` pointers (heap-allocated, managed by the
  runtime).

``TYPE_STRUCT``
  Map-based container — field names are string keys.

``TYPE_OPTION``
  Tagged wrapper: ``some(value)`` or ``none``.

``TYPE_RESULT``
  Tagged wrapper: ``ok(value)`` or ``err(message)``.

Equality is value-level — ``values_equal`` compares by type tag and
contents, not by pointer identity.

Ownership
---------

Karkain uses **lexical-scope ownership** (Borrow Checker, Phase 51):

* Variables own the values they bind.
* Ownership transfers on re-binding or scope exit.
* Dead-name analysis prevents use after scope end.

There is no runtime reference counting or garbage collector — heap
containers (strings, arrays, maps) are managed by the C runtime's
arena allocator (Phase 101/102).

Heap-managed containers
-----------------------

Strings, arrays, and maps are **heap-allocated**:

* ``make_string`` / ``make_array`` / ``make_map`` create heap cells.
* ``karkain_memory.c`` provides the arena allocator
  (``karkain_arena_alloc`` / ``karkain_arena_reset``).
* ``karkain_mem.c`` provides a coalescing free-list allocator on top of
  the arena (``karkain_mem_alloc`` / ``karkain_mem_free``).

Concurrency (Phase 107)
-----------------------

The concurrency runtime adds:

* **Work-stealing task scheduler** — fixed pool of workers, bounded grab
  queue, per-worker wake condvar.
* **Bounded/unblocking channels** — ``channel()``, ``chanSend()``,
  ``chanClose()``, ``receive()``.
* **Actors** — serialised mailbox dispatch with state cell.

All concurrency primitives are implemented in C
(``runtime/concurrency/c/``) and embedded into generated assemblies.

Stack frames
------------

Each function call pushes a frame onto the C call stack.  The maximum
depth is ``KARKAIN_MAX_FRAMES`` (128).  ``karkain_frame_enter`` /
``karkain_frame_leave`` bookend every function body and power the stack
trace on runtime errors.
