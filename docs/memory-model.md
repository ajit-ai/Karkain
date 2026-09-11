# Karkain Memory Model

## Value Representation

Every Karkain value flows through a single, unboxed `Value` union (`pkg/codegen/codegen.go` preamble, `runtime/core/karkain_values.h`):

| Type tag        | C storage          | Notes                                          |
|-----------------|--------------------|-------------------------------------------------|
| `TYPE_INT`      | `long long`        | 64-bit signed                                   |
| `TYPE_FLOAT64`  | `double`           | 64-bit IEEE-754                                 |
| `TYPE_BOOL`     | `long long`        | 0 or 1                                          |
| `TYPE_STRING`   | heap-allocated `char*` | Length-prefixed via `NativeString`            |
| `TYPE_ARRAY`    | heap-allocated `NativeArray` | `Value** items`, push/pop/get/set/clear  |
| `TYPE_STRUCT`   | map-based instance | Tagged map, `TYPE_STRUCT`                       |
| `TYPE_OPTION`   | tagged union       | Present/absent wrapper                          |
| `TYPE_RESULT`   | tagged union       | Ok/Err wrapper                                  |

`make_int`, `make_float`, `make_string`, `make_array`, `make_bool` are pure constructors with no hidden allocation except string/array payloads.

## Ownership and Borrowing

Karkain implements **lexical-scope ownership** (Phase 51):

- A value moves into a binding when assigned; the source binding becomes dead.
- A value is borrowed (shared/read-only reference) when passed to a function
  without a move.
- Dead-name analysis prevents use-after-move within a scope.
- Escape analysis (wired into the borrow checker) flags when a locally
  owned value escapes via a closure or global; warnings are emitted, not
  hard errors, to preserve practical usability.

Key guarantees:

- No data races: the borrow checker is single-threaded at compile time;
  the concurrency runtime (Phase 107) uses channels/actors with per-entity
  serialization.
- No use-after-free: the GC-free arena allocator (`runtime/freestanding/`)
  and heap-managed containers lifetime are bounded by scope; the runtime
  value copy semantics prevent dangling references.

## Allocation and Deallocation

- **Strings** and **arrays** are heap-managed by the runtime. On the Go
  engine, the C preamble `karkain_free` wrapper intercepts deallocation.
  On the kcc engine, the self-hosted runtime owns the heap directly.
- **Struct instances** are map-based; allocation is through `make_map()`.
- There is no user-visible `free`/`delete`; the runtime handles deallocation
  when bindings go out of scope.

## Concurrency (Phase 107)

- Channels are bounded/unbounded blocking primitives with deterministic
  close-drain semantics.
- Actors own a state cell and a mailbox; messages are processed serially
  (no shared mutable state).
- `spawn` creates a task; the work-stealing scheduler distributes tasks
  across workers.

## Numeric Overflow

- Integer overflow wraps (C `long long` behavior); no runtime check.
- Float overflow follows IEEE-754 (inf/nan).
- Phase 100 adds runtime error checks for **division by zero**,
  **array/string index out of bounds** with source-located diagnostics.

## Stack

- The call stack is bounded by `KARKAIN_MAX_FRAMES` (128 frames); a
  stack overflow raises a runtime error.
- Frame tracking (`karkain_frame_enter`/`leave`) records the current
  function, source file, and line for diagnostics and `karkain debug`
  tracing.

## Summary

Karkain's memory model is designed for safety without a garbage collector:
lexical ownership at compile time, heap-managed containers at runtime,
and deterministic value semantics for all primitives. The concurrency
model avoids shared-state races entirely.
