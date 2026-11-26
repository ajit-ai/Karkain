# Karkain Hybrid Memory Model — Phase 3 Design (Arena Regions)

**Status: SLICE 1 COMPLETE (runtime foundation).** Phase 3 adds bulk arena regions:
a region is a bump allocator whose entire contents are freed in one call. No GC,
no refcounting of individual objects, no cycle collection.

## Scope (slice 1)

- `runtime/arena/c/karkain_arena.{h,c}`: `karkain_region_new`, `alloc`, `reset`,
  `capacity`, `used`, `free`.
- Direct C-toolchain gate `pkg/runtime/phase_v1225_arena_test.go`.

## Out of scope (later slices)

- Language surface (`region`/`with region`), codegen integration, compiler
  insertion of region scopes, escape-aware promotion into regions.