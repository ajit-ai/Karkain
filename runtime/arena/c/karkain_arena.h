// karkain_arena.h -- Karkain-owned arena region runtime (Phase 3, v1.2.25).
//
// A region is a bump allocator: allocations are O(1) and the whole region is
// freed in one call (bulk reset). Individual frees are not supported -- that is
// the point: a request's lifetime maps to one region, which is torn down at
// scope/exit. No GC, no refcounting, no fragmentation.
#ifndef KARKAIN_ARENA_H
#define KARKAIN_ARENA_H

#include <stddef.h>

typedef struct karkain_region karkain_region_t;

// Create a region with at least `capacity` bytes of usable space. Returns 0 on
// failure. The region owns its backing memory.
karkain_region_t *karkain_region_new(size_t capacity);

// Bump-allocate `size` bytes (16-byte aligned) from the region. Returns 0 if
// the region is exhausted. Never frees individual allocations.
void *karkain_region_alloc(karkain_region_t *r, size_t size);

// Reset the region: every allocation made so far becomes invalid and the
// cursor returns to the start. O(1). Use when a request/scope ends.
void karkain_region_reset(karkain_region_t *r);

// Total capacity (bytes) of the region.
size_t karkain_region_capacity(karkain_region_t *r);

// Bytes currently used.
size_t karkain_region_used(karkain_region_t *r);

// Free the region and its backing memory.
void karkain_region_free(karkain_region_t *r);

#endif // KARKAIN_ARENA_H