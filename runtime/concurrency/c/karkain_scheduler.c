/* karkain_scheduler.c â€” Phase 107 work-stealing scheduler + platform layer.
 *
 * Responsibilities:
 *   - Platform threading abstraction (Win32 SRWLOCK/CONDITION_VARIABLE and
 *     CreateThread; pthread equivalents elsewhere).
 *   - N worker threads, one local LIFO runnable queue each.
 *   - Work stealing: an idle worker first runs its own queue, then steals
 *     from another worker's queue (and from the global queue used by
 *     non-worker callers such as main). Idle workers sleep on a condition
 *     variable â€” no busy spin.
 *   - Task lifecycle: spawn -> runnable -> running -> completed; join()
 *     observes completion and the wrapper-set status. Pending accounting
 *     drives wait_all()/finish().
 *   - Deterministic shutdown: finish() waits for pending==0, stops workers,
 *     joins them and releases every runtime object.
 */
#include "karkain_conc.h"
#include "karkain_sched_impl.h"

#ifdef _WIN32
#else
#include <unistd.h>
#endif
#include <stdatomic.h>

/* ---------------------------------------------------------------------
 * Platform primitives
 * ------------------------------------------------------------------- */

#ifdef _WIN32

void karkain_mutex_init(karkain_mutex_t* m) { InitializeSRWLock(m); }
void karkain_mutex_lock(karkain_mutex_t* m) { AcquireSRWLockExclusive(m); }
void karkain_mutex_unlock(karkain_mutex_t* m) { ReleaseSRWLockExclusive(m); }
void karkain_mutex_destroy(karkain_mutex_t* m) { (void)m; }

void karkain_cond_init(karkain_cond_t* c) { InitializeConditionVariable(c); }
void karkain_cond_signal(karkain_cond_t* c) { WakeConditionVariable(c); }
void karkain_cond_broadcast(karkain_cond_t* c) { WakeAllConditionVariable(c); }
void karkain_cond_wait(karkain_cond_t* c, karkain_mutex_t* m) {
    SleepConditionVariableSRW(c, m, INFINITE, 0);
}
void karkain_cond_timedwait(karkain_cond_t* c, karkain_mutex_t* m, long ms) {
    SleepConditionVariableSRW(c, m, (DWORD)ms, 0);
}
void karkain_cond_destroy(karkain_cond_t* c) { (void)c; }

static DWORD WINAPI karkain_win_thread_proc(LPVOID arg) {
    void** pair = (void**)arg;
    karkain_thread_fn fn = (karkain_thread_fn)pair[0];
    void* ctx = pair[1];
    free(pair);
    fn(ctx);
    return 0;
}

karkain_thread_t karkain_thread_create(karkain_thread_fn fn, void* arg) {
    void** pair = (void**)malloc(2 * sizeof(void*));
    pair[0] = (void*)fn;
    pair[1] = arg;
    return CreateThread(NULL, 0, karkain_win_thread_proc, pair, 0, NULL);
}

void karkain_thread_join(karkain_thread_t t) {
    WaitForSingleObject(t, INFINITE);
    CloseHandle(t);
}

void karkain_thread_yield(void) { SwitchToThread(); }

#else /* POSIX */

void karkain_mutex_init(pthread_mutex_t* m) { pthread_mutex_init(m, NULL); }
void karkain_mutex_lock(pthread_mutex_t* m) { pthread_mutex_lock(m); }
void karkain_mutex_unlock(pthread_mutex_t* m) { pthread_mutex_unlock(m); }
void karkain_mutex_destroy(pthread_mutex_t* m) { pthread_mutex_destroy(m); }

void karkain_cond_init(pthread_cond_t* c) { pthread_cond_init(c, NULL); }
void karkain_cond_signal(pthread_cond_t* c) { pthread_cond_signal(c); }
void karkain_cond_broadcast(pthread_cond_t* c) { pthread_cond_broadcast(c); }
void karkain_cond_wait(pthread_cond_t* c, pthread_mutex_t* m) {
    pthread_cond_wait(c, m);
}
void karkain_cond_timedwait(pthread_cond_t* c, pthread_mutex_t* m, long ms) {
    struct timespec ts;
    clock_gettime(CLOCK_REALTIME, &ts);
    ts.tv_sec += ms / 1000;
    ts.tv_nsec += (ms % 1000) * 1000000L;
    if (ts.tv_nsec >= 1000000000L) {
        ts.tv_sec++;
        ts.tv_nsec -= 1000000000L;
    }
    pthread_cond_timedwait(c, m, &ts);
}
void karkain_cond_destroy(pthread_cond_t* c) { pthread_cond_destroy(c); }

static void* karkain_pthread_proc(void* arg) {
    void** pair = (void**)arg;
    karkain_thread_fn fn = (karkain_thread_fn)pair[0];
    void* ctx = pair[1];
    free(pair);
    fn(ctx);
    return NULL;
}

karkain_thread_t karkain_thread_create(karkain_thread_fn fn, void* arg) {
    void** pair = (void**)malloc(2 * sizeof(void*));
    pair[0] = (void*)fn;
    pair[1] = arg;
    pthread_t t;
    pthread_create(&t, NULL, karkain_pthread_proc, pair);
    return t;
}

void karkain_thread_join(pthread_t t) { pthread_join(t, NULL); }
void karkain_thread_yield(void) { sched_yield(); }

#endif

/* ---------------------------------------------------------------------
 * Scheduler state
 * ------------------------------------------------------------------- */

/* Thread-local current worker (NULL for non-worker threads such as main). */
static __thread karkain_worker_t* conc_self;

karkain_worker_t* karkain_sched_current_worker(void) {
    return conc_self;
}

static long karkain_conc_default_workers(void) {
    long n = 4;
#ifdef _WIN32
    SYSTEM_INFO si;
    GetSystemInfo(&si);
    n = (long)si.dwNumberOfProcessors;
#else
    long c = sysconf(_SC_NPROCESSORS_ONLN);
    if (c > 0) n = c;
#endif
    if (n < 1) n = 1;
    if (n > 8) n = 8;
    const char* env = getenv("KARKAIN_WORKERS");
    if (env && *env) {
        long v = atol(env);
        if (v >= 1 && v <= 64) n = v;
    }
    return n;
}

static void karkain_sched_pending_dec(karkain_sched_t* s, long n) {
    long after = karkain_conc_atomic_fetch_add(&s->pending, -n) - n;
    if (after == 0) {
        karkain_mutex_lock(&s->lock);
        karkain_cond_broadcast(&s->drained);
        karkain_mutex_unlock(&s->lock);
    }
}

/* ---------------------------------------------------------------------
 * Queue plumbing
 * ------------------------------------------------------------------- */

void karkain_sched_push_run(karkain_sched_t* s, karkain_run_t* r) {
    karkain_worker_t* self = conc_self;
    if (self) {
        karkain_mutex_lock(&self->lock);
        r->next = self->run_head;
        self->run_head = r;
        self->run_count++;
        karkain_mutex_unlock(&self->lock);
    } else {
        karkain_mutex_lock(&s->lock);
        r->next = s->global_head;
        s->global_head = r;
        s->global_count++;
        karkain_mutex_unlock(&s->lock);
    }
    /* No broadcast on the hot path: workers re-examine queues on a bounded
     * timed wait, so a sleeping idle worker notices new work within
     * KARKAIN_IDLE_WAIT_MS. This keeps per-message send throughput high. */
}

karkain_run_t* karkain_sched_pop_local(karkain_sched_t* s) {
    (void)s;
    karkain_worker_t* self = conc_self;
    if (!self) return NULL;
    karkain_mutex_lock(&self->lock);
    karkain_run_t* r = self->run_head;
    if (r) {
        self->run_head = r->next;
        self->run_count--;
    }
    karkain_mutex_unlock(&self->lock);
    return r;
}

karkain_run_t* karkain_sched_steal(karkain_sched_t* s, long id) {
    /* Never steal from ourselves; visit every other worker in round-robin. */
    for (long v = 1; v < s->nworkers; v++) {
        long victim = (id + v) % s->nworkers;
        karkain_worker_t* w = &s->workers[victim];
        karkain_mutex_lock(&w->lock);
        karkain_run_t* r = w->run_head;
        if (r) {
            w->run_head = r->next;
            w->run_count--;
        }
        karkain_mutex_unlock(&w->lock);
        if (r) return r;
    }
    karkain_mutex_lock(&s->lock);
    karkain_run_t* g = s->global_head;
    if (g) {
        s->global_head = g->next;
        s->global_count--;
    }
    karkain_mutex_unlock(&s->lock);
    return g;
}

void karkain_sched_enqueue_actor(karkain_actor_t* a) {
    if (karkain_conc_atomic_load(&a->stopped)) return;
    if (!karkain_conc_atomic_cas(&a->queued, 0, 1)) return;
    karkain_run_t* r = (karkain_run_t*)malloc(sizeof(karkain_run_t));
    r->is_actor = 1;
    r->task = NULL;
    r->actor = a;
    r->next = NULL;
    karkain_sched_push_run(a->sched, r);
}

void karkain_sched_drain_actor(karkain_sched_t* s, karkain_actor_t* a) {
    int delivered = 0;
    karkain_mutex_lock(&a->dwq_lock);
    for (;;) {
        void* m = karkain_channel_try_recv(&a->mailbox);
        if (!m) break;
        if (!karkain_conc_atomic_load(&a->stopped)) {
            a->handler(a, m, s);
        } else {
            free(m); /* stopped: discard the undelivered box */
        }
        delivered++;
    }
    karkain_conc_atomic_store(&a->queued, 0);
    karkain_mutex_unlock(&a->dwq_lock);

    if (delivered > 0) karkain_sched_pending_dec(s, delivered);

    /* A message may have arrived while we drained; re-queue under the flag. */
    karkain_mutex_lock(&a->mailbox.lock);
    int more = (a->mailbox.size > 0) && !karkain_conc_atomic_load(&a->stopped);
    karkain_mutex_unlock(&a->mailbox.lock);
    if (more) karkain_sched_enqueue_actor(a);
}

/* ---------------------------------------------------------------------
 * Task lifecycle
 * ------------------------------------------------------------------- */

karkain_task_t* karkain_sched_spawn(karkain_sched_t* s, karkain_run_fn fn, void* ctx) {
    karkain_task_t* t = (karkain_task_t*)calloc(1, sizeof(karkain_task_t));
    if (!t) return NULL;
    t->fn = fn;
    t->ctx = ctx;
    karkain_mutex_init(&t->lock);
    karkain_cond_init(&t->cv);

    karkain_mutex_lock(&s->lock);
    t->next = s->tasks_head;
    s->tasks_head = t;
    karkain_mutex_unlock(&s->lock);

    karkain_conc_atomic_fetch_add(&s->pending, 1);
    karkain_run_t* r = (karkain_run_t*)malloc(sizeof(karkain_run_t));
    r->is_actor = 0;
    r->task = t;
    r->actor = NULL;
    r->next = NULL;
    karkain_sched_push_run(s, r);
    return t;
}

int karkain_task_join(karkain_task_t* t) {
    if (!t) return -2;
    karkain_mutex_lock(&t->lock);
    while (!karkain_conc_atomic_load(&t->done)) {
        karkain_cond_wait(&t->cv, &t->lock);
    }
    long status = t->status;
    karkain_mutex_unlock(&t->lock);
    return (int)status;
}

void karkain_wait_all(karkain_sched_t* s) {
    karkain_mutex_lock(&s->lock);
    while (karkain_conc_atomic_load(&s->pending) != 0) {
        karkain_cond_wait(&s->drained, &s->lock);
    }
    karkain_mutex_unlock(&s->lock);
}

static void karkain_run_task(karkain_sched_t* s, karkain_task_t* t) {
    t->fn(t, t->ctx, s);
    free(t->ctx); /* ctx is heap-owned by the runtime (codegen wrapper allocates) */
    t->ctx = NULL;
    karkain_conc_atomic_store(&t->done, 1);
    karkain_mutex_lock(&t->lock);
    karkain_cond_broadcast(&t->cv);
    karkain_mutex_unlock(&t->lock);
    karkain_sched_pending_dec(s, 1);
}

/* ---------------------------------------------------------------------
 * Worker loop
 * ------------------------------------------------------------------- */

static void karkain_worker_loop(void* arg) {
    karkain_worker_t* w = (karkain_worker_t*)arg;
    karkain_sched_t* s = w->sched;
    conc_self = w;

    for (;;) {
        if (karkain_conc_atomic_load(&s->stopping) &&
            karkain_conc_atomic_load(&s->pending) == 0) {
            break;
        }
        karkain_run_t* r = karkain_sched_pop_local(s);
        if (!r) r = karkain_sched_steal(s, w->id);

        if (r) {
            if (r->is_actor) {
                karkain_sched_drain_actor(s, r->actor);
            } else {
                karkain_run_task(s, r->task);
            }
            free(r);
            continue;
        }

        /* Nothing runnable anywhere: sleep on a bounded timed wait, then
         * re-scan (no busy spin). finish() broadcasts to wake quickly. */
        karkain_mutex_lock(&s->lock);
        while (!karkain_conc_atomic_load(&s->stopping) && s->global_count == 0) {
            karkain_cond_timedwait(&s->work_avail, &s->lock, 2);
        }
        karkain_mutex_unlock(&s->lock);
    }
}

/* ---------------------------------------------------------------------
 * Bootstrap / shutdown
 * ------------------------------------------------------------------- */

static karkain_sched_t* g_sched;
/* One-time init guard: the very first caller creates the scheduler. */
static atomic_flag g_init_lock = ATOMIC_FLAG_INIT;

karkain_sched_t* karkain_conc_init(void) {
    while (atomic_flag_test_and_set_explicit(&g_init_lock,
                                             memory_order_acquire)) {
        karkain_thread_yield();
    }
    karkain_sched_t* s = g_sched;
    if (!s) {
        s = (karkain_sched_t*)calloc(1, sizeof(karkain_sched_t));
        s->nworkers = karkain_conc_default_workers();
        karkain_mutex_init(&s->lock);
        karkain_cond_init(&s->work_avail);
        karkain_cond_init(&s->drained);
        s->workers = (karkain_worker_t*)calloc((size_t)s->nworkers,
                                               sizeof(karkain_worker_t));
        for (long i = 0; i < s->nworkers; i++) {
            s->workers[i].sched = s;
            s->workers[i].id = i;
            karkain_mutex_init(&s->workers[i].lock);
        }
        for (long i = 0; i < s->nworkers; i++) {
            s->workers[i].thread =
                karkain_thread_create(karkain_worker_loop, &s->workers[i]);
        }
        g_sched = s;
    }
    atomic_flag_clear_explicit(&g_init_lock, memory_order_release);
    return g_sched;
}

int karkain_conc_finish(void) {
    karkain_sched_t* s = g_sched;
    if (!s) return 0;

    /* Wait for every outstanding task and actor message to complete. */
    karkain_wait_all(s);

    karkain_conc_atomic_store(&s->stopping, 1);
    karkain_mutex_lock(&s->lock);
    karkain_cond_broadcast(&s->work_avail);
    karkain_mutex_unlock(&s->lock);

    for (long i = 0; i < s->nworkers; i++) {
        karkain_thread_join(s->workers[i].thread);
    }

    /* Release bookkeeping. The process is exiting, but we still clean up so
     * the runtime itself is leak-clean for sanitizer runs. */
    karkain_task_t* t = s->tasks_head;
    while (t) {
        karkain_task_t* next = t->next;
        karkain_mutex_destroy(&t->lock);
        karkain_cond_destroy(&t->cv);
        free(t->ctx);
        free(t);
        t = next;
    }
    karkain_actor_t* a = s->actors_head;
    while (a) {
        karkain_actor_t* next = a->next;
        free(a->udata);
        karkain_channel_release(&a->mailbox);
        karkain_mutex_destroy(&a->state_lock);
        karkain_mutex_destroy(&a->dwq_lock);
        free(a);
        a = next;
    }
    karkain_channel_t* c = s->channels_head;
    while (c) {
        karkain_channel_t* next = c->next;
        karkain_channel_destroy(c);
        c = next;
    }

    for (long i = 0; i < s->nworkers; i++) {
        karkain_mutex_destroy(&s->workers[i].lock);
    }
    free(s->workers);
    karkain_mutex_destroy(&s->lock);
    karkain_cond_destroy(&s->work_avail);
    karkain_cond_destroy(&s->drained);
    free(s);
    g_sched = NULL;
    return 0;
}