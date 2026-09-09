# Phase 102 — Native Runtime Core — Final Report

**Status:** COMPLETE
**Date:** 2026-09-09
**Branch:** `develop` -> `main` (committed and merged)

## Objective

Build the Karkain-owned **Native Runtime Core** — the runtime foundation for
fundamental value/object handling, native strings, native arrays, memory
management and I/O — implemented as a libc-free layer directly on the
Phase 101 freestanding runtime (`runtime/freestanding/`), compatible with the
existing architecture and ready for adoption by both the Go engine and the
self-hosted `kcc`.

## What Was Implemented (`runtime/core/`)

| File | Provides |
|------|----------|
| `karkain_mem.h/.c` | Karkain-owned heap on the freestanding arena: `karkain_mem_alloc/calloc/realloc/free/stats/reset`. Real free/realloc with an address-sorted coalescing free list; 16-byte aligned; no libc. |
| `karkain_value.h/.c` | Tagged `Value` (int/float/bool/string/array). Layout- and name-compatible with the codegen embedded runtime (`ValueType`, `TYPE_*`, `make_int/make_float/make_bool/make_string/make_array`, `values_equal`, `value_truthy`, `value_as_int/float`, `value_class`). |
| `karkain_nstr.h/.c` | Length-prefixed native strings (`NativeString`): from cstr/binary bytes, length, concat, append-char, slice, compare, equals, startswith, Value round-trip. |
| `karkain_narr.h/.c` | Native array of `Value` (`NativeArray`): new, reserve, push, pop, get, set, len, clear, Value wrap — element storage mirrors codegen (`Value** items` + `length`). |
| `karkain_core_io.h/.c` | Value-level I/O on the freestanding write primitives: print int/float/bool/string/native-string/array, `println`. |
| `core_main.c` | Phase 102 gate program (asserted by the test). |
| `README.md` | Layer description, compile recipe, boundaries. |

## Integration Fixes (directly related, in the Phase 101 freestanding layer)

Two real defects in `runtime/freestanding/karkain_memory.c` surfaced under
Phase 102's multi-pool workload:

1. **Arena grew but the growth could fail to make room** (guaranteed only
   `add >= need`, not `add >= used + need`). Fixed the capacity math.
2. **Arena relocated every outstanding pointer on grow** (allocated a new
   page block, memcpy'd all prior bytes, then freed the old region). Any
   pointer returned before a grow went stale — correct for a single-shot
   hello, fatal for a real heap. Rewrote as a **linked-segment arena**: the
   current segment doubles in size on demand and prior allocations never
   move. API unchanged; `karkain_arena_reset()` released all segments.

## Tests Performed and Results

| Test | Result |
|------|--------|
| `go test ./pkg/runtime/ -run TestPhase102_NativeRuntimeCore -count=1` | PASS (compiles core+freestanding freestanding-style, runs, exact-output assertion: values, strings, arrays, memory, I/O, arena reset) |
| `go test ./pkg/runtime/ -run TestPhase101_FreestandingHello -count=1` | PASS (regression after the arena rewrite) |
| `go test ./pkg/codegen/ -run TestPhase101 -count=1` | PASS (engine untouched) |
| `go test ./pkg/cli/ -run TestPhase101 -count=1` | PASS (both engines, stack parity) |

## Engine Compatibility

Neither the Go engine nor the self-hosted `kcc` is modified. The core layer and
its gate use the same freestanding compile pattern the compilers will adopt
when generated-C runtime migration lands. `Value` naming/layout deliberately
match the codegen embedded runtime so that migration is a drop-in.

## Remaining Dependencies / Boundaries

- **libc/libm:** none in `runtime/core/` or `runtime/freestanding/`.
- **GMP/BigInt/BigFloat + map/option/result:** deliberately outside the core
  value subset; kept behind the GMP boundary deferred to Phase 109.
- **codegen adoption:** the embedded runtime texts in the compilers are not
  rewritten here (future phase); this phase establishes the core they will
  migrate onto.

## Commit Hash

(Included in the Phase 102 commit on `develop` merged into `main`.)