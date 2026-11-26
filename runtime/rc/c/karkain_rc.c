// karkain_rc.c -- Karkain-owned reference counting runtime (Phase 2, v1.2.24).
#include "karkain_rc.h"

#include <stdlib.h>

// Atomic helpers mirror the concurrency runtime's GNU-atomic wrappers so the
// same memory model applies. On MSVC these would need _Interlocked*; the
// generated-C path compiles with GCC/Clang, which is the supported toolchain.
static long karkain_rc_atomic_inc(volatile long *p) {
    return __atomic_add_fetch(p, 1, __ATOMIC_ACQ_REL);
}
static long karkain_rc_atomic_dec(volatile long *p) {
    return __atomic_sub_fetch(p, 1, __ATOMIC_ACQ_REL);
}
static long karkain_rc_atomic_load(volatile long *p) {
    return __atomic_load_n(p, __ATOMIC_ACQUIRE);
}

karkain_rc_box_t *karkain_rc_new(void *payload, void (*free_payload)(void *)) {
    karkain_rc_box_t *b = (karkain_rc_box_t *)malloc(sizeof(karkain_rc_box_t));
    if (b == 0) { return 0; }
    b->strong = 1;
    b->payload = payload;
    b->free_payload = free_payload;
    return b;
}

long karkain_rc_retain(karkain_rc_box_t *b) {
    if (b == 0) { return 0; }
    return karkain_rc_atomic_inc(&b->strong);
}

long karkain_rc_release(karkain_rc_box_t *b) {
    if (b == 0) { return 0; }
    long n = karkain_rc_atomic_dec(&b->strong);
    if (n <= 0) {
        if (b->free_payload != 0 && b->payload != 0) {
            b->free_payload(b->payload);
        }
        b->payload = 0;
        b->free_payload = 0;
        free(b);
        return 0;
    }
    return n;
}

long karkain_rc_strong_count(karkain_rc_box_t *b) {
    if (b == 0) { return 0; }
    return karkain_rc_atomic_load(&b->strong);
}

void *karkain_rc_payload(karkain_rc_box_t *b) {
    if (b == 0) { return 0; }
    return b->payload;
}