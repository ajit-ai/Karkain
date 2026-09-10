/* karkain_sched_impl.h — internal scheduler declarations shared between the
 * concurrency runtime translation units. Included only by karkain_actor.c and
 * karkain_scheduler.c; in a single-TU build (embedded into generated C) the
 * include guard makes a second inclusion a no-op.
 */
#ifndef KARKAIN_SCHED_IMPL_H
#define KARKAIN_SCHED_IMPL_H

#include "karkain_conc.h"

/* The worker currently running on this thread, or NULL for non-worker
 * threads (e.g. main). Maintained by the worker loop. */
karkain_worker_t* karkain_sched_current_worker(void);

/* Push a runnable onto the caller's worker queue (local first); a non-worker
 * caller pushes to the scheduler's global queue. */
void karkain_sched_push_run(karkain_sched_t* s, karkain_run_t* r);

/* Enqueue an actor for processing (respects the per-actor queued flag). */
void karkain_sched_enqueue_actor(karkain_actor_t* a);

/* Pop one runnable from the caller's local queue (LIFO). */
karkain_run_t* karkain_sched_pop_local(karkain_sched_t* s);

/* Steal one runnable from another worker's queue (FIFO-ish fairness). */
karkain_run_t* karkain_sched_steal(karkain_sched_t* s, long id);

/* Run the actor dispatch pipeline (drain mailbox, handler, bookkeeping). */
void karkain_sched_drain_actor(karkain_sched_t* s, karkain_actor_t* a);

#endif /* KARKAIN_SCHED_IMPL_H */