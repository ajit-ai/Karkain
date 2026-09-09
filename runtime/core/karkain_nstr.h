#ifndef KARKAIN_NSTR_H
#define KARKAIN_NSTR_H

/* Karkain Native Runtime Core — native strings.
 *
 * Length-prefixed UTF-8 byte strings owned by the Karkain runtime. The
 * backing bytes always carry a trailing NUL so an existing NUL-terminated
 * buffer can be printed, but the logical length is explicit (embedded NULs
 * are legal). All memory comes from karkain_mem_*.
 */

#include "karkain_value.h"

#ifdef __cplusplus
extern "C" {
#endif

typedef struct NativeString {
    long len;
    char* data; /* always NUL-terminated; len excludes the NUL */
} NativeString;

NativeString kns_from_cstr(const char* s);
NativeString kns_from_bytes(const char* s, long n);
NativeString kns_empty(void);
long kns_len(NativeString s);

NativeString kns_concat(NativeString a, NativeString b);
NativeString kns_append_char(NativeString s, char c);
/* [start, end); clamps instead of failing. */
NativeString kns_slice(NativeString s, long start, long end);

int kns_compare(NativeString a, NativeString b); /* -1/0/1 */
int kns_equals(NativeString a, NativeString b);
int kns_starts_with(NativeString s, NativeString prefix);

/* Round-trip with the Value type: string Value <-> NativeString. */
Value kns_to_value(NativeString s);
NativeString kns_from_value(Value v); /* NUL-terminated cstr value */

#ifdef __cplusplus
}
#endif

#endif /* KARKAIN_NSTR_H */