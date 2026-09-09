#ifndef KARKAIN_NARR_H
#define KARKAIN_NARR_H

/* Karkain Native Runtime Core — native arrays of Values.
 *
 * Growable array whose element storage mirrors the codegen runtime
 * (Value** items + length, each element a heap Value* copy). Pushes copy
 * the Value; the caller keeps ownership of the original. All memory is
 * karkain_mem_*.
 */

#include "karkain_value.h"

#ifdef __cplusplus
extern "C" {
#endif

typedef struct NativeArray {
    struct Value** items;
    long len;
    long cap;
} NativeArray;

NativeArray karr_new(long capacity);
void karr_reserve(NativeArray* a, long need);
void karr_push(NativeArray* a, Value v);
Value karr_pop(NativeArray* a);          /* make_nil() when empty */
Value karr_get(NativeArray a, long i);   /* make_nil() when out of range */
int karr_set(NativeArray* a, long i, Value v); /* 0 when out of range */
long karr_len(NativeArray a);
void karr_clear(NativeArray* a);         /* len = 0, storage retained */

/* Wrap a NativeArray into a string-less array Value (shares storage). */
Value karr_to_value(NativeArray* a);

#ifdef __cplusplus
}
#endif

#endif /* KARKAIN_NARR_H */