#include "karkain_runtime.h"
#include "karkain_platform.h"

/* Page-backed linked-segment arena. Allocations hand out memory inside the
 * current segment and NEVER move: when the current segment runs out, a new
 * (doubled) segment is appended and prior pointers stay valid. Backing pages
 * are delivered zeroed by the OS. karkain_arena_reset tears every segment
 * down. */

#define KARKAIN_ARENA_INIT_PAGES 4
#define KARKAIN_ARENA_HDR_ALIGN 32u /* 16-byte alignment floor, room for the
                                       inline segment header */

typedef struct ArenaSeg {
    struct ArenaSeg* next;
    unsigned long block; /* total bytes in this segment (header + payload) */
    unsigned long used;  /* payload bytes handed out */
} ArenaSeg;

static ArenaSeg* g_seg;

static unsigned long karkain_arena_round16(unsigned long n) {
    n = (n + 15u) & ~15ul;
    if (n == 0) n = 16;
    return n;
}

static ArenaSeg* karkain_seg_new(unsigned long block) {
    block = (block + KARKAIN_PAGESIZE - 1u) & ~(KARKAIN_PAGESIZE - 1u);
    if (block < KARKAIN_ARENA_INIT_PAGES * KARKAIN_PAGESIZE)
        block = KARKAIN_ARENA_INIT_PAGES * KARKAIN_PAGESIZE;
    void* p = karkain_pages_alloc(block);
    if (!p) return NULL;
    ArenaSeg* s = (ArenaSeg*)p;
    s->next = NULL;
    s->block = block;
    s->used = 0;
    return s;
}

void* karkain_arena_alloc(size_t size) {
    unsigned long n = karkain_arena_round16((unsigned long)size);
    if (!g_seg || g_seg->used + n > g_seg->block - KARKAIN_ARENA_HDR_ALIGN) {
        unsigned long next_block = (g_seg && g_seg->block > n)
            ? g_seg->block * 2 : KARKAIN_ARENA_INIT_PAGES * KARKAIN_PAGESIZE;
        if (next_block < KARKAIN_ARENA_HDR_ALIGN + n) next_block = KARKAIN_ARENA_HDR_ALIGN + n;
        ArenaSeg* s = karkain_seg_new(next_block);
        if (!s) return NULL;
        s->next = g_seg;
        g_seg = s;
    }
    char* p = (char*)g_seg + KARKAIN_ARENA_HDR_ALIGN + g_seg->used;
    g_seg->used += n;
    return p;
}

/* Phase 102: release every arena segment and reset to empty. All pointers
 * previously handed out by karkain_arena_alloc become invalid. */
void karkain_arena_reset(void) {
    ArenaSeg* s = g_seg;
    while (s) {
        ArenaSeg* next = s->next;
        karkain_pages_free(s, s->block);
        s = next;
    }
    g_seg = NULL;
}