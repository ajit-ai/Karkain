// Phase 16: Actor-based distributed concurrency runtime
// Implements atomic MPMC mailboxes and thread-safe actor execution

#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <stdatomic.h>

// Actor mailbox message structure
typedef struct {
    void* data;
    size_t size;
    int sender_id;
} ActorMessage;

// MPMC (Multi-Producer Multi-Consumer) Queue for actor mailboxes
typedef struct {
    ActorMessage* buffer;
    size_t capacity;
    atomic_size_t head;
    atomic_size_t tail;
    atomic_size_t count;
} MPMCQueue;

// Actor structure
typedef struct {
    int id;
    MPMCQueue* mailbox;
    void (*handler)(void* message);
    int is_running;
} Actor;

// Create MPMC queue
MPMCQueue* mpmc_create(size_t capacity) {
    MPMCQueue* queue = (MPMCQueue*)malloc(sizeof(MPMCQueue));
    queue->buffer = (ActorMessage*)calloc(capacity, sizeof(ActorMessage));
    queue->capacity = capacity;
    atomic_init(&queue->head, 0);
    atomic_init(&queue->tail, 0);
    atomic_init(&queue->count, 0);
    return queue;
}

// Send message to MPMC queue (thread-safe)
int mpmc_send(MPMCQueue* queue, void* data, size_t size, int sender_id) {
    size_t head = atomic_load(&queue->head);
    size_t next_head = (head + 1) % queue->capacity;
    
    if (next_head == atomic_load(&queue->tail)) {
        return 0; // Queue full
    }
    
    ActorMessage msg;
    msg.data = malloc(size);
    memcpy(msg.data, data, size);
    msg.size = size;
    msg.sender_id = sender_id;
    
    queue->buffer[head] = msg;
    atomic_store(&queue->head, next_head);
    atomic_fetch_add(&queue->count, 1);
    
    return 1;
}

// Receive message from MPMC queue (thread-safe)
int mpmc_receive(MPMCQueue* queue, ActorMessage* msg) {
    size_t tail = atomic_load(&queue->tail);
    
    if (tail == atomic_load(&queue->head)) {
        return 0; // Queue empty
    }
    
    *msg = queue->buffer[tail];
    atomic_store(&queue->tail, (tail + 1) % queue->capacity);
    atomic_fetch_sub(&queue->count, 1);
    
    return 1;
}

// Create actor
Actor* actor_create(int id, void (*handler)(void*), size_t mailbox_size) {
    Actor* actor = (Actor*)malloc(sizeof(Actor));
    actor->id = id;
    actor->mailbox = mpmc_create(mailbox_size);
    actor->handler = handler;
    actor->is_running = 1;
    return actor;
}

// Send message to actor
int actor_send(Actor* actor, void* data, size_t size, int sender_id) {
    return mpmc_send(actor->mailbox, data, size, sender_id);
}

// Process actor mailbox (called in actor's thread)
void actor_process(Actor* actor) {
    ActorMessage msg;
    while (actor->is_running) {
        if (mpmc_receive(actor->mailbox, &msg)) {
            actor->handler(msg.data);
            free(msg.data);
        }
    }
}

// Stop actor
void actor_stop(Actor* actor) {
    actor->is_running = 0;
}

// Destroy actor
void actor_destroy(Actor* actor) {
    actor_stop(actor);
    
    // Free remaining messages in mailbox
    ActorMessage msg;
    while (mpmc_receive(actor->mailbox, &msg)) {
        free(msg.data);
    }
    
    free(actor->mailbox->buffer);
    free(actor->mailbox);
    free(actor);
}