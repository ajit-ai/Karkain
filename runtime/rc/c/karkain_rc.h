// karkain_rc.h -- Karkain-owned reference counting runtime (Phase 2, v1.2.24).
//
// A reference-counted box. The payload is an opaque pointer owned by the box;
// the box owns one strong reference count. retain() increments, release()
// decrements and frees the payload + box when the count reaches zero.
//
// Thread safety: retain/release use the same atomic helpers as the concurrency
// runtime (karkain_conc_atomic_*), so Rc is safe to share across spawned tasks.
// No GC, no cycle collection, no weak references in this slice (those are
// later slices of Phase 2).
#ifndef KARKAIN_RC_H
#define KARKAIN_RC_H

#include <stddef.h>

typedef struct karkain_rc_box {
    volatile long strong;   // strong reference count (atomic)
    void *payload;          // owned payload (freed by the release callback)
    void (*free_payload)(void *); // release callback for the payload
} karkain_rc_box_t;

// Allocate a new box wrapping `payload` with strong count 1. `free_payload`
// is invoked exactly once when the last reference is released. Returns 0 on
// allocation failure.
karkain_rc_box_t *karkain_rc_new(void *payload, void (*free_payload)(void *));

// Increment the strong count. Returns the new count. No-op on null.
long karkain_rc_retain(karkain_rc_box_t *b);

// Decrement the strong count; if it reaches zero, invoke free_payload(payload)
// and free the box. Returns the new count (0 if freed). No-op on null.
long karkain_rc_release(karkain_rc_box_t *b);

// Current strong count (0 for null/freed).
long karkain_rc_strong_count(karkain_rc_box_t *b);

// Raw payload pointer (NULL for null/freed). Does not change the count.
void *karkain_rc_payload(karkain_rc_box_t *b);

#endif // KARKAIN_RC_H