// karkain_arena.c -- Karkain-owned arena region runtime (Phase 3, v1.2.25).
#include "karkain_arena.h"

#include <stdlib.h>

struct karkain_region {
    char *base;
    size_t capacity;
    size_t used;
};

static size_t round16(size_t n) {
    return (n + 15u) & ~15u;
}

karkain_region_t *karkain_region_new(size_t capacity) {
    if (capacity == 0) { capacity = 4096; }
    karkain_region_t *r = (karkain_region_t *)malloc(sizeof(karkain_region_t));
    if (r == 0) { return 0; }
    r->base = (char *)malloc(capacity);
    if (r->base == 0) { free(r); return 0; }
    r->capacity = capacity;
    r->used = 0;
    return r;
}

void *karkain_region_alloc(karkain_region_t *r, size_t size) {
    if (r == 0 || size == 0) { return 0; }
    size_t n = round16(size);
    if (r->used + n > r->capacity) { return 0; }
    void *p = r->base + r->used;
    r->used += n;
    return p;
}

void karkain_region_reset(karkain_region_t *r) {
    if (r == 0) { return; }
    r->used = 0;
}

size_t karkain_region_capacity(karkain_region_t *r) {
    if (r == 0) { return 0; }
    return r->capacity;
}

size_t karkain_region_used(karkain_region_t *r) {
    if (r == 0) { return 0; }
    return r->used;
}

void karkain_region_free(karkain_region_t *r) {
    if (r == 0) { return; }
    free(r->base);
    free(r);
}