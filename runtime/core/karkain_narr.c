#include "karkain_narr.h"

#include "karkain_mem.h"
#include "karkain_runtime.h"

NativeArray karr_new(long capacity) {
    NativeArray a;
    a.len = 0;
    a.cap = 0;
    a.items = NULL;
    if (capacity > 0) {
        a.items = (struct Value**)karkain_mem_alloc((size_t)capacity * sizeof(struct Value*));
        a.cap = a.items ? capacity : 0;
    }
    return a;
}

void karr_reserve(NativeArray* a, long need) {
    if (!a || need <= a->cap) return;
    long new_cap = (a->cap == 0) ? 4 : a->cap;
    while (new_cap < need) new_cap *= 2;
    struct Value** items = (struct Value**)karkain_mem_alloc((size_t)new_cap * sizeof(struct Value*));
    if (!items) return;
    for (long i = 0; i < a->len; i++) items[i] = a->items[i];
    if (a->items) karkain_mem_free(a->items);
    a->items = items;
    a->cap = new_cap;
}

void karr_push(NativeArray* a, Value v) {
    if (!a) return;
    if (a->len >= a->cap) karr_reserve(a, a->len + 1);
    if (a->len >= a->cap) return; /* OOM */
    struct Value* copy = (struct Value*)karkain_mem_alloc(sizeof(Value));
    if (!copy) return;
    *copy = v;
    a->items[a->len++] = copy;
}

Value karr_pop(NativeArray* a) {
    if (!a || a->len == 0) return make_nil();
    Value v = *a->items[a->len - 1];
    karkain_mem_free(a->items[a->len - 1]);
    a->len--;
    return v;
}

Value karr_get(NativeArray a, long i) {
    if (i < 0 || i >= a.len) return make_nil();
    return *a.items[i];
}

int karr_set(NativeArray* a, long i, Value v) {
    if (!a || i < 0 || i >= a->len) return 0;
    *a->items[i] = v;
    return 1;
}

long karr_len(NativeArray a) { return a.len; }

void karr_clear(NativeArray* a) {
    if (!a) return;
    for (long i = 0; i < a->len; i++) karkain_mem_free(a->items[i]);
    a->len = 0;
}

Value karr_to_value(NativeArray* a) {
    Value v;
    v.type = TYPE_ARRAY;
    v.arrVal.items = a ? a->items : NULL;
    v.arrVal.length = a ? (int)a->len : 0;
    return v;
}