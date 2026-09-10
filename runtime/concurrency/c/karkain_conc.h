/* karkain_conc.h â€” Phase 107 Concurrency Runtime (compiler-neutral core).
 *
 * This header is the single source of truth for the Karkain concurrency
 * runtime. It is embedded verbatim (with `#include "karkain_conc.h"` lines
 * stripped) into the code-generated C translation unit, and it is also
 * compiled standalone by the pkg/runtime gate test with a driver.
 *
 * Design notes:
 *   - Threads: Win32 (CreateThread/SRWLOCK/CONDITION_VARIABLE) on Windows,
 *     pthread on POSIX. Hosted builds; no freestanding constraints.
 *   - Channels: FIFO message queues with a producer/consumer guard. A channel
 *     is either unbounded (cap < 0) or bounded (cap >= 0). Every API operation
 *     is thread-safe. Payloads are opaque void* â€” the Karkain code generation
 *     layer boxes/unboxes karkain_value cells around this boundary.
 *   - Actors: an actor owns a mailbox channel plus opaque state (udata).
 *     handler(actor, msg) is invoked on exactly one worker at a time
 *     (single-runner drain). Messages are never silently dropped: every
 *     enqueued message is either delivered to the handler or dropped only
 *     by an explicit actor_stop.
 *   - Scheduler: N worker threads each with a local LIFO runnable queue.
 *     Workers execute their local queue first; idle workers steal from other
 *     workers' queues and from the global queue (used by non-worker callers
 *     such as main). Idle workers sleep on a condition variable (no busy
 *     spin).
 *   - Pending accounting: a global atomic counter tracks outstanding tasks and
 *     outstanding actor messages. wait_all()/finish() wait until it reaches
 *     zero, giving deterministic shutdown semantics.
 *
 * The runtime depends only on the C library (malloc/free) and the host
 * threading primitives â€” no libm, no third-party dependencies.
 */
#ifndef KARKAIN_CONC_H
#define KARKAIN_CONC_H

#include <stddef.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

#ifdef _WIN32
#define _CRT_SECURE_NO_WARNINGS
#include <windows.h>
#else
#include <pthread.h>
#include <sched.h>
#include <time.h>
#endif

#ifdef __cplusplus
extern "C" {
#endif

/* ---------------------------------------------------------------------
 * Platform abstraction
 * ------------------------------------------------------------------- */

#ifdef _WIN32
typedef SRWLOCK karkain_mutex_t;
typedef CONDITION_VARIABLE karkain_cond_t;
typedef HANDLE karkain_thread_t;
#define KARKAIN_MUTEX_INIT KARKAIN_MUTEX_INITIALIZER()
#else
typedef pthread_mutex_t karkain_mutex_t;
typedef pthread_cond_t karkain_cond_t;
typedef pthread_t karkain_thread_t;
#endif

void karkain_mutex_init(karkain_mutex_t* m);
void karkain_mutex_lock(karkain_mutex_t* m);
void karkain_mutex_unlock(karkain_mutex_t* m);
void karkain_mutex_destroy(karkain_mutex_t* m);

void karkain_cond_init(karkain_cond_t* c);
void karkain_cond_signal(karkain_cond_t* c);
void karkain_cond_broadcast(karkain_cond_t* c);
void karkain_cond_wait(karkain_cond_t* c, karkain_mutex_t* m);
void karkain_cond_timedwait(karkain_cond_t* c, karkain_mutex_t* m, long ms);
void karkain_cond_destroy(karkain_cond_t* c);

typedef void (*karkain_thread_fn)(void* arg);
karkain_thread_t karkain_thread_create(karkain_thread_fn fn, void* arg);
void karkain_thread_join(karkain_thread_t t);
void karkain_thread_yield(void);

/* GCC __atomic builtins (work with -std=c11 and -std=c2x on gcc/clang). */

static inline long karkain_conc_atomic_fetch_add(long* p, long v) {
    return __atomic_fetch_add(p, v, __ATOMIC_SEQ_CST);
}
static inline long karkain_conc_atomic_load(const long* p) {
    return __atomic_load_n(p, __ATOMIC_SEQ_CST);
}
static inline void karkain_conc_atomic_store(long* p, long v) {
    __atomic_store_n(p, v, __ATOMIC_SEQ_CST);
}
static inline int karkain_conc_atomic_cas(long* p, long expected, long desired) {
    return __atomic_compare_exchange_n(p, &expected, desired, 0,
                                       __ATOMIC_SEQ_CST, __ATOMIC_SEQ_CST);
}

/* ---------------------------------------------------------------------
 * Channels
 * ------------------------------------------------------------------- */

typedef struct karkain_conc_msg {
    void* payload;
    struct karkain_conc_msg* next;
} karkain_conc_msg_t;

typedef struct karkain_channel {
    struct karkain_sched* sched;   /* owner scheduler (unused by calls) */
    long cap;                       /* < 0 unbounded, >= 0 bounded */
    karkain_mutex_t lock;
    karkain_cond_t not_empty;       /* receivers wait for data */
    karkain_cond_t not_full;        /* bounded senders wait for space */
    karkain_conc_msg_t* head;
    karkain_conc_msg_t* tail;
    long size;                      /* current queue length */
    long waiters_send;              /* bounded: producers blocked on not_full */
    long waiters_recv;              /* consumers blocked on not_empty */
    long closed;                    /* 0 open, 1 closed (set once under lock) */
    struct karkain_channel* next;   /* scheduler bookkeeping link */
} karkain_channel_t;

karkain_channel_t* karkain_channel_create(struct karkain_sched* s, long cap);
int  karkain_channel_send(karkain_channel_t* c, void* payload); /* 1 sent, 0 closed */
void* karkain_channel_recv(karkain_channel_t* c);               /* NULL when closed+drained */
void* karkain_channel_try_recv(karkain_channel_t* c);           /* NULL when empty (non-blocking) */
int  karkain_channel_close(karkain_channel_t* c);               /* 1 now closed, 0 already */
void karkain_channel_release(karkain_channel_t* c);             /* free residuals + locks, keep struct */
void karkain_channel_destroy(karkain_channel_t* c);             /* release + free struct */

/* ---------------------------------------------------------------------
 * Tasks (spawned functions)
 * ------------------------------------------------------------------- */

typedef struct karkain_task karkain_task_t;
typedef void (*karkain_run_fn)(karkain_task_t* t, void* ctx, struct karkain_sched* s);

struct karkain_task {
    karkain_run_fn fn;
    void* ctx;
    long done;          /* 0 running, 1 complete (atomic) */
    long status;        /* 0 success, <0 failure code (atomic, set before done) */
    karkain_mutex_t lock;
    karkain_cond_t cv;
    struct karkain_task* next; /* free-list link (owned by scheduler) */
};

/* ---------------------------------------------------------------------
 * Actors
 * ------------------------------------------------------------------- */

typedef struct karkain_actor karkain_actor_t;
typedef void (*karkain_actor_fn)(karkain_actor_t* a, void* msg, struct karkain_sched* s);

struct karkain_actor {
    karkain_channel_t mailbox;   /* embedded channel (messages are void*) */
    struct karkain_sched* sched;
    karkain_actor_fn handler;
    void* udata;                 /* boxed state (Karkain heap Value cell) */
    long queued;                 /* atomic: 1 while a runnable entry exists */
    karkain_mutex_t state_lock;  /* guards udata for get/set_state */
    long stopped;                /* atomic: actor_stop requested */
    karkain_mutex_t dwq_lock;    /* drains one message at a time */
    struct karkain_actor* next;  /* scheduler bookkeeping link */
};

karkain_actor_t* karkain_actor_create(struct karkain_sched* s, karkain_actor_fn fn, void* state);
int  karkain_actor_send(karkain_actor_t* a, void* payload);  /* 1 accepted, 0 stopped */
void* karkain_actor_get_state(karkain_actor_t* a);
int  karkain_actor_set_state(karkain_actor_t* a, void* state);
int  karkain_actor_stop(karkain_actor_t* a);                 /* 1 now stopping, 0 already */

/* ---------------------------------------------------------------------
 * Scheduler
 * ------------------------------------------------------------------- */

typedef struct karkain_run {
    int is_actor;              /* 0 task, 1 actor */
    karkain_task_t* task;
    karkain_actor_t* actor;
    struct karkain_run* next;
} karkain_run_t;

typedef struct karkain_worker {
    struct karkain_sched* sched;
    long id;
    karkain_thread_t thread;
    karkain_mutex_t lock;      /* guards local stack + pending stack */
    karkain_run_t* run_head;   /* LIFO local queue */
    long run_count;
    long notified;             /* rollover: an item arrived since last sleep */
} karkain_worker_t;

typedef struct karkain_sched {
    karkain_worker_t* workers;
    long nworkers;
    long pending;              /* atomic: tasks + actor messages in flight */
    long stopping;             /* atomic: 1 once finish() allowed */
    karkain_mutex_t lock;      /* guards global queue + cv */
    karkain_cond_t work_avail; /* workers sleep here when idle */
    karkain_cond_t drained;    /* wait_all / finish waits for pending==0 */
    karkain_run_t* global_head;/* LIFO for non-worker spawns */
    long global_count;
    karkain_task_t* tasks_head;/* bookkeeping list of every spawned task */
    karkain_actor_t* actors_head;
    karkain_channel_t* channels_head;
    long initialized;          /* atomic: sched starts its workers once */
} karkain_sched_t;

karkain_sched_t* karkain_conc_init(void);
int  karkain_conc_finish(void);    /* wait for pending==0, stop workers, 0 on success */

karkain_task_t* karkain_sched_spawn(karkain_sched_t* s, karkain_run_fn fn, void* ctx);
int  karkain_task_join(karkain_task_t* t);  /* returns status; blocks until done */
void karkain_wait_all(karkain_sched_t* s);  /* blocks until pending==0 */

#ifdef __cplusplus
}
#endif

#endif /* KARKAIN_CONC_H */