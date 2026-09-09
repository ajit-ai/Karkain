#ifndef KARKAIN_VALUE_H
#define KARKAIN_VALUE_H

/* Karkain Native Runtime Core — fundamental value/object handling.
 *
 * The tagged Value type is layout-compatible with (a future subset of) the
 * codegen-embedded runtime: same enum tag names, same union field names, so
 * the generated-C runtime can adopt this layer directly later. Heap storage
 * (string data, array element storage) is allocated from the Karkain heap
 * (karkain_mem_*) — no libc.
 *
 * Only the primitive subset (int/float/bool/string/array) is provided here;
 * map/option/result/bigint/bigfloat remain outside this phase (see README).
 */

#ifdef __cplusplus
extern "C" {
#endif

typedef enum {
    TYPE_INT,
    TYPE_FLOAT64,
    TYPE_STRING,
    TYPE_ARRAY,
    TYPE_BOOL
} ValueType;

typedef struct Value {
    ValueType type;
    union {
        long long intVal;
        double floatVal;
        char* strVal;
        struct {
            struct Value** items; /* array of heap Value* copies */
            int length;
        } arrVal;
    };
} Value;

typedef enum { VAL_IMMEDIATE, VAL_HEAP } ValueClass;

Value make_int(long long v);
Value make_float(double v);
Value make_bool(int v);
Value make_nil(void);                    /* int 0, the language's zero value */
Value make_string(const char* s);        /* arena-array-backed copy of NUL string */
Value make_string_n(const char* s, long n);
Value make_array(void);

ValueClass value_class(Value v);
const char* value_type_name(Value v);
const char* value_type_tag(Value v);     /* "TYPE_INT", ... */
long long value_as_int(Value v);
double value_as_float(Value v);

int values_equal(Value a, Value b);      /* deep equality (mirrors codegen) */
int value_truthy(Value v);               /* 0 for zero-value, empty string/array */

#ifdef __cplusplus
}
#endif

#endif /* KARKAIN_VALUE_H */