// karkain_rc.c -- Karkain-owned reference counting runtime (Phase 2, v1.2.24).
#include "karkain_rc.h"

#include <stdlib.h>

static long rc_atomic_inc(volatile long *p) { return __atomic_add_fetch(p, 1, __ATOMIC_ACQ_REL); }
static long rc_atomic_dec(volatile long *p) { return __atomic_sub_fetch(p, 1, __ATOMIC_ACQ_REL); }
static long rc_atomic_load(volatile long *p) { return __atomic_load_n(p, __ATOMIC_ACQUIRE); }

karkain_rc_box_t *karkain_rc_new(void *payload, void (*free_payload)(void *)) {
    karkain_rc_box_t *b = (karkain_rc_box_t *)malloc(sizeof(karkain_rc_box_t));
    if (b == 0) { return 0; }
    b->strong = 1;
    b->weak = 1; // implicit weak held by the strong box itself
    b->payload = payload;
    b->free_payload = free_payload;
    return b;
}

karkain_rc_box_t *karkain_rc_new_int(long value) {
    long *p = (long *)malloc(sizeof(long));
    if (p == 0) { return 0; }
    *p = value;
    // free_payload frees p; box frees itself.
    karkain_rc_box_t *b = karkain_rc_new(p, (void (*)(void *))free);
    return b;
}

long karkain_rc_retain(karkain_rc_box_t *b) {
    if (b == 0) { return 0; }
    if (rc_atomic_load(&b->strong) <= 0) { return 0; } // dead
    return rc_atomic_inc(&b->strong);
}

long karkain_rc_release(karkain_rc_box_t *b) {
    if (b == 0) { return 0; }
    long n = rc_atomic_dec(&b->strong);
    if (n <= 0) {
        // Free payload once.
        if (b->free_payload != 0 && b->payload != 0) {
            b->free_payload(b->payload);
        }
        b->payload = 0;
        b->free_payload = 0;
        // Drop the implicit weak; if no weak refs remain, free the shell.
        long w = rc_atomic_dec(&b->weak);
        if (w <= 0) {
            free(b);
        }
        return 0;
    }
    return n;
}

long karkain_rc_strong_count(karkain_rc_box_t *b) {
    if (b == 0) { return 0; }
    long n = rc_atomic_load(&b->strong);
    return n > 0 ? n : 0;
}

void *karkain_rc_payload(karkain_rc_box_t *b) {
    if (b == 0) { return 0; }
    if (rc_atomic_load(&b->strong) <= 0) { return 0; }
    return b->payload;
}

long karkain_rc_get_int(karkain_rc_box_t *b) {
    if (b == 0) { return 0; }
    if (rc_atomic_load(&b->strong) <= 0) { return 0; }
    if (b->payload == 0) { return 0; }
    return *(long *)b->payload;
}

// Weak refs.
karkain_rc_weak_t *karkain_rc_weak_new(karkain_rc_box_t *b) {
    if (b == 0) { return 0; }
    karkain_rc_weak_t *w = (karkain_rc_weak_t *)malloc(sizeof(karkain_rc_weak_t));
    if (w == 0) { return 0; }
    rc_atomic_inc(&b->weak);
    w->target = b;
    return w;
}

long karkain_rc_weak_count(karkain_rc_weak_t *w) {
    if (w == 0 || w->target == 0) { return 0; }
    return rc_atomic_load(&w->target->weak);
}

karkain_rc_box_t *karkain_rc_weak_upgrade(karkain_rc_weak_t *w) {
    if (w == 0 || w->target == 0) { return 0; }
    karkain_rc_box_t *b = w->target;
    // Try to acquire a strong ref only if alive.
    while (1) {
        long s = rc_atomic_load(&b->strong);
        if (s <= 0) { return 0; } // dead
        // CAS s -> s+1
        if (__atomic_compare_exchange_n(&b->strong, &s, s + 1, 0, __ATOMIC_ACQ_REL, __ATOMIC_ACQUIRE)) {
            return b;
        }
    }
}

void karkain_rc_weak_release(karkain_rc_weak_t *w) {
    if (w == 0) { return; }
    karkain_rc_box_t *b = w->target;
    if (b != 0) {
        long wn = rc_atomic_dec(&b->weak);
        if (wn <= 0 && rc_atomic_load(&b->strong) <= 0) {
            // shell still present and fully released: free it.
            free(b);
        }
        w->target = 0;
    }
    free(w);
}