#ifndef KARKAIN_MEM_H
#define KARKAIN_MEM_H

/* Karkain Native Runtime Core — memory management integration.
 *
 * A Karkain-owned heap built on top of the Phase 101 freestanding page arena.
 * Every block carries a small header, so alloc/free/realloc are real
 * operations (free returns blocks to a coalescing free list and never calls
 * libc). All allocations are 16-byte aligned. OOM returns NULL.
 *
 * The only OS interaction is page acquisition through
 * karkain_arena_alloc/karkain_arena_reset — there is no libc in this layer.
 */

#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

void* karkain_mem_alloc(size_t size);
void* karkain_mem_calloc(size_t count, size_t size);
void* karkain_mem_realloc(void* p, size_t new_size);
void  karkain_mem_free(void* p);

/* Heap statistics: bytes currently allocated+free across live blocks. */
void karkain_mem_stats(size_t* used_bytes, size_t* free_bytes);

/* Reset the heap bookkeeping after karkain_arena_reset() has released the
 * backing pages. Not needed in steady state. */
void karkain_mem_reset(void);

#ifdef __cplusplus
}
#endif

#endif /* KARKAIN_MEM_H */