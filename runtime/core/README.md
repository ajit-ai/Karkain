# Karkain Native Runtime Core (`runtime/core/`)

Phase 102 layer: the Karkain-owned runtime foundation for fundamental
value/object handling, native strings, native arrays, memory management and
I/O, built directly on the Phase 101 freestanding layer
(`runtime/freestanding/`).

## Layer structure

```
generated C (future: is emitted on top of this layer)
        |
   runtime/core/   <-- THIS phase (libc-free, no libm, no GMP)
        |
   runtime/freestanding/  <-- Phase 101 (arena allocator, raw syscall layer)
        |
      OS (kernel32 on Windows, raw syscalls on POSIX)
```

## Components

| File | Provides |
|------|----------|
| `karkain_mem.h/.c`    | Heap on the freestanding arena: `karkain_mem_alloc/calloc/realloc/free/stats`. Real free/realloc with an address-sorted coalescing free list; 16-byte aligned; no libc. |
| `karkain_value.h/.c`  | Tagged `Value` (int/float/bool/string/array) layout-compatible with the codegen embedded runtime (`ValueType`, `TYPE_*`, `make_int/make_float/make_bool/make_string/make_array`, `values_equal`, `value_truthy`, `value_as_int/float`). |
| `karkain_nstr.h/.c`   | Length-prefixed native strings (`NativeString`): from cstr/bytes, length, concat, append-char, slice, compare, equals, startswith, Value round-trip. |
| `karkain_narr.h/.c`   | Native array of `Value` (`NativeArray`): new, push, pop, get, set, len, clear, reserve, Value wrap — element storage mirrors codegen (`Value** items` + `length`). |
| `karkain_core_io.h/.c`| Value-level I/O on the freestanding write primitives: print int/float/bool/string/native-string/array, `println`. |
| `core_main.c`         | Phase 102 gate program (asserted by `pkg/runtime/phase102_core_test.go`). |

## Compiling (freestanding, no libc/libm)

All core sources compile with the same flags as the freestanding layer:

```
gcc -std=c11 -ffreestanding -nostdlib -fno-builtin -O2 \
    core_main.c karkain_mem.c karkain_value.c karkain_nstr.c \
    karkain_narr.c karkain_core_io.c \
    ../freestanding/karkain_runtime.c ../freestanding/karkain_memory.c \
    ../freestanding/karkain_io.c ../freestanding/karkain_string.c \
    ../freestanding/karkain_math.c ../freestanding/karkain_platform.c \
    -o prog -Wl,-e,_start -lkernel32        # Windows
    # (drop the last two flags on Linux/macOS)
```

## Boundaries

- **libc / libm:** none. The only OS contact is the freestanding platform
  layer (page allocation, raw writes, exit).
- **GMP / BigInt / BigFloat:** intentionally excluded from the core value
  subset. map/option/result + bigint/bigfloat values stay behind the
  GMP boundary deferred to Phase 109.
- **codegen adoption:** `Value` and its constructors use the same names and
  field layout as the codegen embedded runtime so the generated-C runtime can
  migrate onto `runtime/core/` without touching generated programs. Nothing in
  the compilers (Go engine or `kcc`) is changed by this phase.

## Engine compatibility

The core is engine-agnostic: neither the Go engine nor the self-hosted `kcc`
is modified. The gate in `pkg/runtime/phase102_core_test.go` links the core and
freestanding layers into one native binary and asserts exact output — the same
verification pattern the compilers will use once adoption lands.