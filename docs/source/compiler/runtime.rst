The Karkain Runtime
===================

Karkain programs ship with an embedded C runtime that provides the
value model, memory management, I/O, concurrency, profiling, and error
reporting. All runtime C is embedded into the generated translation unit
via the codegen preamble — no external ``.c`` files are needed for
standard builds.

Runtime layers
--------------

Phase 101: freestanding layer (``runtime/freestanding/``)
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

A libc-free/libm-free foundation that establishes the Karkain-owned
platform boundary:

- **Arena allocator** — ``karkain_memory.c`` implements a linked-segment
  arena; ``karkain_arena_reset`` tears it down, ``karkain_mem_reset``
  clears heap bookkeeping after teardown.
- **Raw OS abstraction** — ``karkain_platform.c`` / ``karkain_platform.h``
  define the only OS boundary the runtime may touch:

  - **Windows**: ``kernel32`` boundary (``VirtualAlloc``, ``WriteFile``,
    ``ExitProcess``).
  - **Linux/POSIX**: raw ``syscall`` instruction via inline assembly.
    No libc.

  Surface: ``karkain_pages_alloc``/``free``,
  ``karkain_write_out``, ``karkain_open_read``/``read``/``write``/
  ``close``, ``karkain_exit``.
- **I/O, string, and math primitives** that compile and link without
  libc.

The gate ``pkg/runtime/phase101_freestanding_test.go`` compiles and
runs a hello program with ``-ffreestanding -nostdlib``.

Phase 102: core layer (``runtime/core/``)
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

The libc-free runtime core stacked directly on the freestanding layer:

- **Heap** — ``karkain_mem.c`` implements a real heap with
  ``alloc``/``calloc``/``realloc``/``free`` on an address-sorted
  coalescing free list.
- **Tagged Value** — ``karkain_value.c`` / ``karkain_value.h`` define
  the ``Value`` type (``TYPE_INT``, ``TYPE_FLOAT64``, ``TYPE_STRING``,
  ``TYPE_ARRAY``, ``TYPE_BOOL``) with codegen-parity naming and layout:
  ``make_int``, ``make_float``, ``make_bool``, ``make_nil``,
  ``make_string``, ``make_string_n``, ``make_array``,
  ``values_equal`` (deep equality), ``value_truthy``,
  ``value_class``, ``value_type_name``, ``value_type_tag``,
  ``value_as_int``, ``value_as_float``.
- **NativeString** — length-prefixed strings
  (concat/slice/compare/startswith/Value round-trip).
- **NativeArray** — ``Value** items`` element storage with
  push/pop/get/set/clear.
- **Value-level I/O** on the freestanding write primitives.

The gate ``pkg/runtime/phase102_core_test.go`` compiles
core+freestanding with ``-ffreestanding -nostdlib`` and asserts exact
output.

Embedded runtime in generated C
-------------------------------

The Go codegen emits a runtime preamble (a large C string literal) at the
top of every generated C23 translation unit. It is emitted from
``pkg/codegen/codegen.go`` and includes:

- The ``Value`` type and `make_*` constructors
  (``make_int``, ``make_float``, ``make_string``, ``make_bool``,
  ``make_array``, ``make_nil``).
- ``values_equal`` and ``value_truthy`` for semantics parity.
- ``karkain_runtime_error(kind, file, line)`` — the Phase 100 runtime
  failure model: reports ``runtime error: <kind> at <file>:<line>`` on
  stderr and terminates with exit code 1. Checked helpers
  (``karkain_checked_div``/``karkain_checked_mod``/``get``/``set``)
  enforce division/modulo by zero and array/string index bounds.
- ``karkain_frame_enter``/``karkain_frame_leave``/``karkain_set_line`` —
  the Phase 101 stack-trace machinery. Every generated function pushes a
  named frame (function name + file); a runtime error then dumps the
  Karkain call chain (``KARKAIN_MAX_FRAMES`` 128) with source file and
  line per frame.
- Phase 109 encoding/crypto builtins: ``hex_encode_bytes``,
  ``hex_decode_bytes``, ``base64_encode_bytes``,
  ``base64_decode_bytes``, ``utf8_valid_bytes``, ``sha256_hex``,
  ``sha512_hex``, ``map_keys_of``.
- Phase 106 SIMD helpers (``karkain_simd_*``) using GNU vector
  operators; Phase 70 SSE fallback for scalar operands.
- Phase 107 concurrency runtime (see below).
- Phase 110 profiling instrumentation (see below).

Deterministic namespace
~~~~~~~~~~~~~~~~~~~~~~~

Every helper uses the ``karkain_*`` prefix and user functions live in
the ``karkain_user_*`` namespace, so generated code never collides with
C standard library symbols.

Concurrency runtime
-------------------

Phase 107 implements a work-stealing task scheduler with channels and
actors, embedded as C in ``runtime/concurrency/c/``:

- ``karkain_conc.h`` — runtime contract and types.
- ``karkain_sched_impl.h`` / ``karkain_scheduler.c`` — worker threads
  with a bounded grab queue, per-worker wake condvar, shared pending
  drain, and no busy-spin stop; ``karkain_task_t`` heap cells with a
  function pointer and heap-owned context.
- ``karkain_channel.c`` — bounded/unbounded blocking channels with
  deterministic close-drain.
- ``karkain_actor.c`` — actors as serialized dispatcher jobs over a
  mailbox with a state-cell box.
- ``concurrency_main.c`` — standalone scenario driver used by the
  ``pkg/runtime`` gate.

These sources are embedded byte-for-byte via
``runtime/concurrency/embed.go`` (``//go:embed c/...``) and appended to
the generated C when ``usesConcurrency`` is detected by the codegen
pre-scan. Atomic helpers (``karkain_conc_atomic_*``) use GNU atomics in
the runtime only.

Language surface: ``spawn``/``join``/``wait_all``, ``channel``/
``chanSend``/``chanClose``/``receive``, ``actor``/``actorSend``/
``actorState``/``setActorState``/``actorStop``.

Profiling hooks
---------------

Phase 110 (``karkain prof <file.kark>``) adds opt-in, aggregation-based
instrumentation. When ``Config.Profiling`` is set the codegen:

- Emits a source-order function-id table (``profNameTableC``) and the
  bounded profiling C runtime (``profRuntimeC``) right after the
  concurrency runtime glue.
- Injects enter/leave probes into every function body (after
  ``frame_enter``/``set_line``; on ``OpRet``/fallthrough before
  ``frame_leave``).
- Emits an ``atexit`` flush that writes a deterministic dump
  (call counts, inclusive/exclusive/min/max/avg ns, call graph, folded
  stacks, allocation metrics) to ``$KARKAIN_PROF_OUT``.

Default builds carry no hook sites (opt-in only).

Trace hooks
-----------

Phase 112 trace instrumentation is gated by the ``KARKAIN_TRACE``
preprocessor define. When ``karkain debug`` (``Config.Trace``) is used,
the generator emits ``#define KARKAIN_TRACE 1`` at the top of the C
output, activating the ``#ifdef KARKAIN_TRACE`` blocks inside
``karkain_frame_enter``/``karkain_frame_leave``. Those blocks print
``karkain:<file>:enter <func>`` / ``karkain:<file>:leave <func>`` lines
to stderr, producing a deterministic execution trace. Default builds
never define ``KARKAIN_TRACE``.

WASM runtime
------------

``pkg/wasm/runtime.go`` implements the Karkain value model inside WASM
linear memory:

- **Values** are ``i64``: bit 0 == 0 means an unboxed integer
  (``v >> 1``, matching ``make_int`` semantics); bit 0 == 1 means a
  boxed heap cell pointer.
- **Heap cells** (8-byte aligned): i32 type tag (same order as the C
  runtime: ``TYPE_INT=0`` … ``TYPE_BOOL=5``), i32 length, then data
  (string bytes or array element slots).
- **WASI boundary** (Phase 123): four imports are fixed at the head of
  the function index space — ``fd_write`` (0), ``args_sizes_get`` (1),
  ``args_get`` (2) and ``proc_exit`` (3). Output uses a fixed scratch
  region (``iovs`` at offset 8, decimal digits at 16); runtime-error
  paths write to fd 2 and terminate with ``proc_exit(1)``.
- **Runtime functions**: the module-defined helpers start at index 4
  (``rt_Alloc``) up to ``UserBase=30``; ``rt_GetArgs`` (28) and the
  ``_start`` bootstrap (29) are part of this range. 25 runtime bodies
  implement ``rt_Write``, ``rt_PrintValue``, ``rt_Box``, ``rt_SetTag``,
  ``rt_MkArray``, ``rt_MkString``, ``rt_Eq``, ``rt_Ne``, ``rt_Error``,
  ``rt_GetArgs``, arithmetic helpers and more.
- **``_start``**: zero the argc/argv-length scratch cells, call
  ``args_sizes_get``, allocate the argument buffer and pointer array via
  ``rt_Alloc``, call ``args_get``, store the argument count and pointer
  array into globals 1 and 2, call ``main``, derive the process exit code
  from the returned ``Value`` (``int(v>>1)`` when unboxed, else 0) and
  terminate with ``proc_exit``.
- **Heap** begins at ``0x10000`` (heap base tracked in global 0).

The emitters ``module.go``/``emit.go`` produce a dependency-free WASM
binary v1 module that ``wasmtime run --dir . <tmp.wasm>`` executes with
stdio passthrough and argument forwarding.

Host platform boundary
----------------------

The freestanding platform layer is the authority on host-specific
behavior. QPC (QueryPerformanceCounter) timing on Windows reuses the
preamble's already-included ``windows.h`` prototypes; other platforms use
``clock_gettime(CLOCK_MONOTONIC)``. GCC can also be configured to inject
reproducible link values (``SOURCE_DATE_EPOCH``) so bootstrap binaries
are bit-identical.

Consumption paths
~~~~~~~~~~~~~~~~~

- **Generated C** — the preamble inlines the runtime that the program
  actually needs.
- **Standalone gates** — ``pkg/runtime`` tests compile the same
  ``runtime/`` sources from disk with ``-ffreestanding -nostdlib``,
  proving the two consumption paths share one source of truth.
- **kcc** — the self-hosted compiler emits its own equivalent helpers
  into its C output (mirroring the Go preamble), keeping both engines
  behaviorally identical (e.g. identical ``runtime error`` messages and
  stack frames).