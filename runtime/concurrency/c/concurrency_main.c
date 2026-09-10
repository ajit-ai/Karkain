/* concurrency_main.c â€” Phase 107 runtime gate driver (hosted build).
 *
 * Exercises the concurrency runtime directly through its C API:
 *   spawn/join            â€” tasks with success and failure statuses
 *   channels              â€” producer/consumer sums over unbounded + bounded
 *   actors                â€” serialized mailbox, boxed state, stop semantics
 *   work stealing         â€” 1000-task spawn storm, exactly-once counting
 *   close semantics       â€” send/recv/close rules (no crashes on misuse)
 *   throughput            â€” 1000 actors, 1,000,000 messages, msg/sec
 *
 * Every scenario prints a deterministic line consumed by the Go gate
 * (pkg/runtime/phase107_concurrency_test.go).
 */
#include <stdio.h>
#include <stdlib.h>

#include "karkain_conc.h"

#if defined(_WIN32)
#include <windows.h>
#else
#include <time.h>
#endif

static long now_ms(void) {
#if defined(_WIN32)
    return (long)GetTickCount64();
#else
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return (long)(ts.tv_sec * 1000 + ts.tv_nsec / 1000000);
#endif
}

typedef struct { long v; } box_t;

static box_t* box_new(long v) {
    box_t* b = (box_t*)malloc(sizeof(box_t));
    b->v = v;
    return b;
}

/* ---- 1. spawn + join (success and failure) ---------------------------- */

typedef struct { long n; } ctx_spawn_t;

static void task_spawned(karkain_task_t* t, void* ctx, karkain_sched_t* s) {
    (void)s;
    ctx_spawn_t* c = (ctx_spawn_t*)ctx;
    t->status = (c->n >= 0) ? 0 : -1;
}

/* ---- 2. channels: producer/consumer over unbounded + bounded ---------- */

typedef struct { karkain_channel_t* ch; long from; long to; } ctx_pc_t;

static void task_producer(karkain_task_t* t, void* ctx, karkain_sched_t* s) {
    (void)s;
    ctx_pc_t* c = (ctx_pc_t*)ctx;
    for (long i = c->from; i <= c->to; i++) {
        if (!karkain_channel_send(c->ch, box_new(i))) break;
    }
    t->status = 0;
}

/* ---- 3. actor: serialized counter ------------------------------------- */

static void actor_counter(karkain_actor_t* a, void* m, karkain_sched_t* s) {
    (void)s;
    free(m);
    (*(long*)a->udata)++;
}

/* ---- 4. work stealing: exactly-once spawn storm ----------------------- */

static long g_storm_count; /* atomic */
static void task_storm(karkain_task_t* t, void* ctx, karkain_sched_t* s) {
    (void)s;
    (void)ctx;
    karkain_conc_atomic_fetch_add(&g_storm_count, 1);
    t->status = 0;
}

/* ---- 5. throughput: many producers -> many counter actors ------------- */

static void actor_through(karkain_actor_t* a, void* m, karkain_sched_t* s) {
    (void)s;
    free(m);
    (*(long*)a->udata)++;
}

typedef struct { karkain_actor_t** actors; long n; long total; } ctx_thr_t;

static void task_thr_producer(karkain_task_t* t, void* ctx, karkain_sched_t* s) {
    (void)s;
    ctx_thr_t* c = (ctx_thr_t*)ctx;
    long sent = 0;
    while (sent < c->total) {
        karkain_actor_t* a = c->actors[sent % c->n];
        if (!karkain_actor_send(a, box_new(1))) break;
        sent++;
    }
    t->status = (sent == c->total) ? 0 : -1;
}

static long g_result; /* last computed throughput */

static void run_throughput(karkain_sched_t* s) {
    const long NACTORS = 1000;
    const long NP = 8;          /* producer tasks */
    const long TOTAL = 1000 * 1000;

    g_storm_count = 0;
    karkain_actor_t** actors =
        (karkain_actor_t**)malloc((size_t)NACTORS * sizeof(karkain_actor_t*));
    for (long i = 0; i < NACTORS; i++) {
        actors[i] = karkain_actor_create(s, actor_through, box_new(0));
    }

    long start = 0;
    ctx_thr_t** pctx =
        (ctx_thr_t**)malloc((size_t)NP * sizeof(ctx_thr_t*));
    karkain_task_t** tasks =
        (karkain_task_t**)malloc((size_t)NP * sizeof(karkain_task_t*));
    for (long p = 0; p < NP; p++) {
        pctx[p] = (ctx_thr_t*)malloc(sizeof(ctx_thr_t));
        pctx[p]->actors = actors;
        pctx[p]->n = NACTORS;
        pctx[p]->total = TOTAL / NP;
        tasks[p] = NULL;
    }
    for (long p = 0; p < NP; p++) {
        tasks[p] = karkain_sched_spawn(s, task_thr_producer, pctx[p]);
    }
    start = now_ms();
    karkain_wait_all(s);
    long elapsed = now_ms() - start;
    if (elapsed <= 0) elapsed = 1;

    long checksum = 0;
    for (long i = 0; i < NACTORS; i++) {
        checksum += *(long*)karkain_actor_get_state(actors[i]);
    }
    g_result = (TOTAL * 1000) / elapsed;
    printf("thr:%ld\n", g_result);
    printf("thr-sum:%ld\n", checksum);
    for (long p = 0; p < NP; p++) {
        long st = karkain_task_join(tasks[p]);
        if (st != 0) printf("thr-status:%ld\n", st);
    }
}

int main(void) {
    karkain_sched_t* s = karkain_conc_init();
    /* Scenario gate for bisecting failures on CI hosts without ASan. */
    int scen = 999;
    { const char* e = getenv("KARKAIN_SCEN"); if (e && *e) scen = atoi(e); }

    /* Each scenario runs when scen >= its number (KARKAIN_SCEN=1 runs only
     * scenario 1, etc.). 999 runs everything. */

    /* 1. spawn + join. */
    if (scen >= 1) {
        ctx_spawn_t* okc = (ctx_spawn_t*)malloc(sizeof(ctx_spawn_t));
        okc->n = 5;
        ctx_spawn_t* badc = (ctx_spawn_t*)malloc(sizeof(ctx_spawn_t));
        badc->n = -3;
        karkain_task_t* ok = karkain_sched_spawn(s, task_spawned, okc);
        karkain_task_t* bad = karkain_sched_spawn(s, task_spawned, badc);
        long sok = karkain_task_join(ok);
        long sbad = karkain_task_join(bad);
        printf("spawn:%ld,%ld\n", sok, sbad);
    }

    /* 2. channels: two producers -> one consumer. */
    if (scen >= 2) {
        karkain_channel_t* ch = karkain_channel_create(s, -1);
        ctx_pc_t* a = (ctx_pc_t*)malloc(sizeof(ctx_pc_t));
        a->ch = ch; a->from = 1; a->to = 10;
        ctx_pc_t* b = (ctx_pc_t*)malloc(sizeof(ctx_pc_t));
        b->ch = ch; b->from = 11; b->to = 20;
        karkain_task_t* pa = karkain_sched_spawn(s, task_producer, a);
        karkain_task_t* pb = karkain_sched_spawn(s, task_producer, b);
        long sum = 0;
        for (int i = 0; i < 20; i++) {
            void* m = karkain_channel_recv(ch);
            sum += m ? ((box_t*)m)->v : 0;
            free(m);
        }
        karkain_task_join(pa);
        karkain_task_join(pb);
        printf("sum:%ld\n", sum);
    }

    /* 3. actors: serialized counter, 100 messages. */
    if (scen >= 3) {
        karkain_actor_t* a = karkain_actor_create(s, actor_counter, box_new(0));
        for (int i = 0; i < 100; i++) {
            if (!karkain_actor_send(a, box_new(1))) break;
        }
        karkain_wait_all(s);
        long* st = (long*)karkain_actor_get_state(a);
        printf("actor:%ld\n", *st);
        long after_stop = karkain_actor_send(a, box_new(1));
        printf("actorstop:%ld\n", after_stop);
    }

    /* 4. work stealing: 1000 tasks, each counted exactly once. */
    if (scen >= 4) {
        for (long i = 0; i < 1000; i++) {
            karkain_task_t* t = karkain_sched_spawn(s, task_storm, NULL);
            (void)t;
        }
        karkain_wait_all(s);
        printf("steal:%ld\n", g_storm_count);
    }

    /* 5. bounded channel: capacity 4, 1..20, sum preserved. */
    if (scen >= 5) {
        karkain_channel_t* ch = karkain_channel_create(s, 4);
        ctx_pc_t* b = (ctx_pc_t*)malloc(sizeof(ctx_pc_t));
        b->ch = ch; b->from = 1; b->to = 20;
        karkain_task_t* pb = karkain_sched_spawn(s, task_producer, b);
        long sum = 0;
        for (int i = 0; i < 20; i++) {
            void* m = karkain_channel_recv(ch);
            sum += m ? ((box_t*)m)->v : 0;
            free(m);
        }
        karkain_task_join(pb);
        printf("bounded:%ld\n", sum);
        (void)0;
    }

    /* 6. close semantics: send/recv/close misuse must not crash. */
    if (scen >= 6) {
        karkain_channel_t* ch = karkain_channel_create(s, -1);
        int c1 = karkain_channel_send(ch, box_new(7));
        int c2 = karkain_channel_close(ch);          /* 1 */
        int c3 = karkain_channel_close(ch);          /* 0 (already closed) */
        int c4 = karkain_channel_send(ch, box_new(9)); /* 0 (rejected, freed) */
        void* m1 = karkain_channel_recv(ch);         /* box(7) */
        void* m2 = karkain_channel_recv(ch);         /* NULL (closed+drained) */
        long v1 = m1 ? ((box_t*)m1)->v : -999;
        free(m1);
        /* c1 must be 1 (open send), c2=1 (closed now), c3=0 (double close),
         * c4=0 (send rejected on closed), then drained value 7 then NULL. */
        printf("close:%d,%d,%d,%d,%ld,%d\n", c1, c2, c3, c4, v1,
               (m2 == NULL) ? 1 : 0);
    }

    /* 7. throughput. */
    if (scen >= 7) {
        run_throughput(s);
    }

    printf("done:107\n");
    karkain_conc_finish();
    return 0;
}