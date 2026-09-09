#include "karkain_mem.h"

#include "karkain_runtime.h"

/* Bump-arena-backed heap with real free/realloc semantics.
 *
 * Layout of a block (payload is 16-byte aligned):
 *
 *   [ KMemHdr | payload ... ]
 *
 * The arena pools are requested in doubling chunks. A freed block links into
 * a singly-linked free list; karkain_mem_alloc reuses the first fit and
 * karkain_mem_free coalesces backward-only (forward blocks are found by
 * walking the free list). Under bump-arena semantics nothing shrinks back to
 * the OS until karkain_arena_reset() is called.
 */

typedef struct KMemHdr {
    size_t size;          /* payload bytes, excluding the header */
    int in_use;           /* 1 = owned by caller, 0 = on free list */
    struct KMemHdr* next; /* free-list link when !in_use */
} KMemHdr;

#define KMEM_BLOCK_ALIGN 16u
#define KMEM_HDR_SIZE ((sizeof(KMemHdr) + KMEM_BLOCK_ALIGN - 1u) & ~(KMEM_BLOCK_ALIGN - 1u))
#define KMEM_MIN_BLOCK KMEM_HDR_SIZE + KMEM_BLOCK_ALIGN

static KMemHdr* g_free;     /* singly-linked free list, sorted by address */
static size_t g_used_bytes; /* payload bytes of live (in_use) blocks */
static size_t g_free_bytes; /* total bytes carried by free-list blocks */

/* Round a payload size up to the block multiple, plus header. */
static size_t karkain_block_span(size_t payload) {
    size_t p = (payload + KMEM_BLOCK_ALIGN - 1u) & ~(KMEM_BLOCK_ALIGN - 1u);
    if (p < KMEM_BLOCK_ALIGN) p = KMEM_BLOCK_ALIGN;
    return KMEM_HDR_SIZE + p;
}

static KMemHdr* karkain_new_pool(size_t need) {
    size_t pool = 4096u * 4;
    while (pool < need) pool *= 2;
    void* base = karkain_arena_alloc(pool);
    if (!base) return NULL;
    KMemHdr* h = (KMemHdr*)base;
    h->size = pool - KMEM_HDR_SIZE;
    h->in_use = 0;
    h->next = NULL;
    return h;
}

void* karkain_mem_alloc(size_t size) {
    size_t span = karkain_block_span(size);
    KMemHdr** walk = &g_free;
    while (*walk) {
        KMemHdr* b = *walk;
        if (b->size >= size) {
            *walk = b->next;
            b->in_use = 1;
            b->next = NULL;
            g_free_bytes -= KMEM_HDR_SIZE + b->size;
            g_used_bytes += KMEM_HDR_SIZE + b->size;
            return (char*)b + KMEM_HDR_SIZE;
        }
        walk = &b->next;
    }
    KMemHdr* b = karkain_new_pool(span);
    if (!b) return NULL;
    b->in_use = 1;
    g_used_bytes += KMEM_HDR_SIZE + b->size;
    return (char*)b + KMEM_HDR_SIZE;
}

void* karkain_mem_calloc(size_t count, size_t size) {
    void* p = karkain_mem_alloc(count * size);
    if (p) karkain_memset(p, 0, count * size);
    return p;
}

void* karkain_mem_realloc(void* p, size_t new_size) {
    if (!p) return karkain_mem_alloc(new_size);
    if (new_size == 0) {
        karkain_mem_free(p);
        return NULL;
    }
    KMemHdr* h = (KMemHdr*)((char*)p - KMEM_HDR_SIZE);
    if (h->size >= new_size) return p; /* fit in place */
    void* np = karkain_mem_alloc(new_size);
    if (!np) return NULL;
    karkain_memcpy(np, p, h->size);
    karkain_mem_free(p);
    return np;
}

void karkain_mem_free(void* p) {
    if (!p) return;
    KMemHdr* h = (KMemHdr*)((char*)p - KMEM_HDR_SIZE);
    if (!h->in_use) return;
    h->in_use = 0;
    size_t span = KMEM_HDR_SIZE + h->size;
    g_used_bytes -= span;
    g_free_bytes += span;

    /* Insert in address order and coalesce with any free successor. */
    KMemHdr** walk = &g_free;
    while (*walk && (char*)*walk < (char*)h) walk = &(*walk)->next;
    h->next = *walk;
    *walk = h;
    if (h->next && (char*)h + (KMEM_HDR_SIZE + h->size) == (char*)h->next) {
        h->size += KMEM_HDR_SIZE + h->next->size;
        h->next = h->next->next;
        g_free_bytes += KMEM_HDR_SIZE;
    }
}

void karkain_mem_stats(size_t* used_bytes, size_t* free_bytes) {
    if (used_bytes) *used_bytes = g_used_bytes;
    if (free_bytes) *free_bytes = g_free_bytes;
}

void karkain_mem_reset(void) {
    g_free = NULL;
    g_used_bytes = 0;
    g_free_bytes = 0;
}