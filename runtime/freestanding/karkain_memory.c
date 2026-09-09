#include "karkain_runtime.h"
#include "karkain_platform.h"

/* Page-backed bump arena. Allocation never frees (arena semantics) and the
 * backing pages are delivered zeroed by the OS, so fresh memory needs no
 * clearing. */

#define KARKAIN_ARENA_INIT_PAGES 4

static char* g_arena_base;
static unsigned long g_arena_cap;
static unsigned long g_arena_used;

static int karkain_arena_grow(unsigned long need) {
    unsigned long add = (g_arena_cap == 0) ? KARKAIN_ARENA_INIT_PAGES * KARKAIN_PAGESIZE : g_arena_cap;
    while (add < need) add *= 2;
    void* p = karkain_pages_alloc(add);
    if (!p) return 0;
    if (g_arena_base) {
        karkain_memcpy(p, g_arena_base, g_arena_used);
        karkain_pages_free(g_arena_base, g_arena_cap);
    }
    g_arena_base = (char*)p;
    g_arena_cap = add;
    return 1;
}

void* karkain_arena_alloc(size_t size) {
    unsigned long n = (unsigned long)size;
    n = (n + 15u) & ~15ul;
    if (n == 0) n = 16;
    if (g_arena_used + n > g_arena_cap) {
        if (!karkain_arena_grow(n)) return NULL;
    }
    char* p = g_arena_base + g_arena_used;
    g_arena_used += n;
    return p;
}