#ifndef KARKAIN_CORE_IO_H
#define KARKAIN_CORE_IO_H

/* Karkain Native Runtime Core — runtime I/O foundation.
 *
 * Value-level output on top of the Phase 101 freestanding write primitives
 * (karkain_print / karkain_print_int / karkain_print_double /
 * karkain_putchar). Formats are chosen to match the language runtime
 * (print = decimal int, 6-decimals float, true/false bool, raw string),
 * so tooling can depend on identical output from both engines later.
 */

#include "karkain_narr.h"
#include "karkain_nstr.h"
#include "karkain_value.h"

#ifdef __cplusplus
extern "C" {
#endif

void karkain_core_print(const char* s);
void karkain_core_print_int(long long v);
void karkain_core_print_float(double v);
void karkain_core_print_bool(int v);
void karkain_core_print_nstr(NativeString s);
void karkain_core_print_value(Value v); /* int/float/bool/string/array */
void karkain_core_print_array(Value v); /* [e0, e1, ...] using print_value */
void karkain_core_println(void);

#ifdef __cplusplus
}
#endif

#endif /* KARKAIN_CORE_IO_H */