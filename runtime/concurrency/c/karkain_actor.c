/* karkain_actor.c â€” Phase 107 actors.
 *
 * An actor owns an embedded mailbox channel and opaque boxed state. Sends
 * place a message box into the mailbox and enqueue the actor onto a worker's
 * runnable queue. A per-actor drain also holds a dedicated drain lock so the
 * handler is invoked on exactly one worker at a time (single-runner
 * guarantee); messages are therefore processed serially and are never
 * silently dropped â€” a stopped actor drains its mailbox discarding remaining
 * messages only after an explicit karkain_actor_stop.
 *
 * State contract (shared with the code generation layer):
 *   udata  â€” void* pointing to a heap Karkain-value cell (box) owned by the
 *            actor. karkain_actor_get_state returns the cell pointer;
 *            karkain_actor_set_state swaps the cell out (freeing the old one).
 *   Messages â€” void* boxes owned by the mailbox; the handler wrapper
 *            (generated code) is responsible for releasing each box after
 *            delivering it to the user handler.
 */
#include "karkain_conc.h"
#include "karkain_sched_impl.h"

karkain_actor_t* karkain_actor_create(karkain_sched_t* s, karkain_actor_fn fn, void* state) {
    karkain_actor_t* a = (karkain_actor_t*)malloc(sizeof(karkain_actor_t));
    if (!a) return NULL;
    memset(a, 0, sizeof(*a));
    a->sched = s;
    a->handler = fn;
    a->udata = state;
    karkain_mutex_init(&a->state_lock);
    karkain_mutex_init(&a->dwq_lock);
    /* Embedded mailbox channel (not separately allocated/tracked). */
    a->mailbox.sched = s;
    a->mailbox.cap = -1; /* unbounded */
    karkain_mutex_init(&a->mailbox.lock);
    karkain_cond_init(&a->mailbox.not_empty);
    karkain_cond_init(&a->mailbox.not_full);
    karkain_mutex_lock(&s->lock);
    a->next = s->actors_head;
    s->actors_head = a;
    karkain_mutex_unlock(&s->lock);
    return a;
}

int karkain_actor_send(karkain_actor_t* a, void* payload) {
    if (karkain_conc_atomic_load(&a->stopped)) {
        free(payload);
        return 0;
    }
    /* Count the message: the drain decrements when the dispatch completes. */
    karkain_conc_atomic_fetch_add(&a->sched->pending, 1);
    if (!karkain_channel_send(&a->mailbox, payload)) {
        /* Closed under our feet (channel released the payload): undo count. */
        karkain_conc_atomic_fetch_add(&a->sched->pending, -1);
        return 0;
    }
    karkain_sched_enqueue_actor(a);
    return 1;
}

void* karkain_actor_get_state(karkain_actor_t* a) {
    void* out = NULL;
    karkain_mutex_lock(&a->state_lock);
    out = a->udata;
    karkain_mutex_unlock(&a->state_lock);
    return out;
}

int karkain_actor_set_state(karkain_actor_t* a, void* state) {
    karkain_mutex_lock(&a->state_lock);
    void* old = a->udata;
    a->udata = state;
    free(old);
    karkain_mutex_unlock(&a->state_lock);
    return 1;
}

int karkain_actor_stop(karkain_actor_t* a) {
    if (!karkain_conc_atomic_cas(&a->stopped, 0, 1)) return 0;
    /* Wake any drain blocked on a full wait: nothing new will be accepted. */
    karkain_channel_close(&a->mailbox);
    return 1;
}