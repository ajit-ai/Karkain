/* karkain_channel.c — Phase 107 channels.
 *
 * FIFO message channel. Unbounded (cap < 0) or bounded (cap >= 0). All
 * operations are thread-safe. Payloads are opaque void*; ownership passes to
 * the receiver on karkain_channel_recv (the caller is responsible for
 * releasing the payload box). Waiter counters keep the notify fast path free
 * of condition-variable calls when nobody is actually blocked.
 */
#include "karkain_conc.h"

karkain_channel_t* karkain_channel_create(karkain_sched_t* s, long cap) {
    karkain_channel_t* c = (karkain_channel_t*)malloc(sizeof(karkain_channel_t));
    if (!c) return NULL;
    memset(c, 0, sizeof(*c));
    c->sched = s;
    c->cap = cap;
    karkain_mutex_init(&c->lock);
    karkain_cond_init(&c->not_empty);
    karkain_cond_init(&c->not_full);
    karkain_mutex_lock(&s->lock);
    c->next = s->channels_head;
    s->channels_head = c;
    karkain_mutex_unlock(&s->lock);
    return c;
}

static void channel_append(karkain_channel_t* c, karkain_conc_msg_t* m) {
    m->next = NULL;
    if (c->tail) {
        c->tail->next = m;
    } else {
        c->head = m;
    }
    c->tail = m;
    c->size++;
}

static karkain_conc_msg_t* channel_pop(karkain_channel_t* c) {
    karkain_conc_msg_t* m = c->head;
    if (!m) return NULL;
    c->head = m->next;
    if (!c->head) c->tail = NULL;
    c->size--;
    return m;
}

int karkain_channel_send(karkain_channel_t* c, void* payload) {
    karkain_conc_msg_t* m = (karkain_conc_msg_t*)malloc(sizeof(karkain_conc_msg_t));
    if (!m) return 0;
    m->payload = payload;
    m->next = NULL;

    karkain_mutex_lock(&c->lock);
    int sent = 1;
    if (c->closed) {
        sent = 0;
    } else if (c->cap >= 0) {
        /* Bounded: wait for space while nobody has closed us. */
        c->waiters_send++;
        while (c->size >= c->cap && !c->closed) {
            karkain_cond_wait(&c->not_full, &c->lock);
        }
        c->waiters_send--;
        if (c->closed) sent = 0;
    }
    if (sent) {
        channel_append(c, m);
        if (c->waiters_recv > 0) karkain_cond_signal(&c->not_empty);
    }
    karkain_mutex_unlock(&c->lock);
    if (!sent) {
        /* Rejected: nothing owns the payload anymore — release it here. */
        free(m);
        free(payload);
    }
    return sent;
}

void* karkain_channel_recv(karkain_channel_t* c) {
    karkain_mutex_lock(&c->lock);
    c->waiters_recv++;
    while (c->size == 0 && !c->closed) {
        karkain_cond_wait(&c->not_empty, &c->lock);
    }
    c->waiters_recv--;
    karkain_conc_msg_t* m = (c->size > 0) ? channel_pop(c) : NULL;
    if (m && c->waiters_send > 0) karkain_cond_signal(&c->not_full);
    karkain_mutex_unlock(&c->lock);
    if (!m) return NULL; /* closed and drained */
    void* payload = m->payload;
    free(m);
    return payload;
}

void* karkain_channel_try_recv(karkain_channel_t* c) {
    karkain_mutex_lock(&c->lock);
    karkain_conc_msg_t* m = (c->size > 0) ? channel_pop(c) : NULL;
    if (m && c->waiters_send > 0) karkain_cond_signal(&c->not_full);
    karkain_mutex_unlock(&c->lock);
    if (!m) return NULL;
    void* payload = m->payload;
    free(m);
    return payload;
}

int karkain_channel_close(karkain_channel_t* c) {
    karkain_mutex_lock(&c->lock);
    if (c->closed) {
        karkain_mutex_unlock(&c->lock);
        return 0;
    }
    c->closed = 1;
    karkain_cond_broadcast(&c->not_empty);
    karkain_cond_broadcast(&c->not_full);
    karkain_mutex_unlock(&c->lock);
    return 1;
}

/* Free residual messages + locks but keep the struct (used for embedded
 * actor mailboxes and by destroy). */
void karkain_channel_release(karkain_channel_t* c) {
    karkain_mutex_lock(&c->lock);
    karkain_conc_msg_t* m = c->head;
    while (m) {
        karkain_conc_msg_t* next = m->next;
        free(m->payload);
        free(m);
        m = next;
    }
    c->head = c->tail = NULL;
    c->size = 0;
    karkain_mutex_unlock(&c->lock);
    karkain_mutex_destroy(&c->lock);
    karkain_cond_destroy(&c->not_empty);
    karkain_cond_destroy(&c->not_full);
}

void karkain_channel_destroy(karkain_channel_t* c) {
    karkain_channel_release(c);
    free(c);
}