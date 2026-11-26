// karkain_rc.h -- Karkain-owned reference counting runtime (Phase 2, v1.2.24).
//
// Strong (Rc) and weak references. A weak reference does not keep its target
// alive; upgrading a weak ref succeeds only if the strong count is still > 0.
//
// Thread safety: retain/release/upgrade use the same atomic helpers as the
// concurrency runtime. No GC, no cycle collection in this slice.
#ifndef KARKAIN_RC_H
#define KARKAIN_RC_H

#include <stddef.h>

typedef struct karkain_rc_box {
    volatile long strong;   // strong reference count (atomic)
    volatile long weak;     // weak reference count (atomic), includes the
                            // implicit weak held by every strong box
    void *payload;          // owned payload (freed by the release callback)
    void (*free_payload)(void *); // release callback for the payload
} karkain_rc_box_t;

typedef struct karkain_rc_weak {
    karkain_rc_box_t *target; // NULL once the target is freed
} karkain_rc_weak_t;

// Strong box API.
karkain_rc_box_t *karkain_rc_new(void *payload, void (*free_payload)(void *));
karkain_rc_box_t *karkain_rc_new_int(long value);
long karkain_rc_retain(karkain_rc_box_t *b);
long karkain_rc_release(karkain_rc_box_t *b);
long karkain_rc_strong_count(karkain_rc_box_t *b);
void *karkain_rc_payload(karkain_rc_box_t *b);
long karkain_rc_get_int(karkain_rc_box_t *b);

// Weak reference API.
karkain_rc_weak_t *karkain_rc_weak_new(karkain_rc_box_t *b);   // NULL if b is NULL
long karkain_rc_weak_count(karkain_rc_weak_t *w);               // 0 if dead/freed
karkain_rc_box_t *karkain_rc_weak_upgrade(karkain_rc_weak_t *w); // retains + returns target, or 0 if dead
void karkain_rc_weak_release(karkain_rc_weak_t *w);

#endif // KARKAIN_RC_H