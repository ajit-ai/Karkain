#include "karkain_core_io.h"

#include "karkain_platform.h"
#include "karkain_runtime.h"

void karkain_core_print(const char* s) { karkain_print(s ? s : ""); }

void karkain_core_print_int(long long v) { karkain_print_int(v); }

void karkain_core_print_float(double v) { karkain_print_double(v); }

void karkain_core_print_bool(int v) { karkain_print(v ? "true" : "false"); }

void karkain_core_print_nstr(NativeString s) {
    if (s.data && s.len > 0) karkain_write_out(s.data, (unsigned long)s.len);
}

void karkain_core_print_array(Value v) {
    karkain_core_print("[");
    for (int i = 0; i < v.arrVal.length; i++) {
        if (i > 0) karkain_core_print(", ");
        karkain_core_print_value(*v.arrVal.items[i]);
    }
    karkain_core_print("]");
}

void karkain_core_print_value(Value v) {
    switch (v.type) {
        case TYPE_INT:
            karkain_core_print_int(v.intVal);
            break;
        case TYPE_FLOAT64:
            karkain_core_print_float(v.floatVal);
            break;
        case TYPE_BOOL:
            karkain_core_print_bool((int)v.intVal);
            break;
        case TYPE_STRING:
            karkain_core_print(v.strVal ? v.strVal : "");
            break;
        case TYPE_ARRAY:
            karkain_core_print_array(v);
            break;
        default:
            karkain_core_print("<?>");
            break;
    }
}

void karkain_core_println(void) { karkain_putchar('\n'); }