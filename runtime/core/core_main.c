#include "karkain_core_io.h"
#include "karkain_mem.h"
#include "karkain_narr.h"
#include "karkain_nstr.h"
#include "karkain_runtime.h"
#include "karkain_value.h"

/* Phase 102 gate program: exercises the Native Runtime Core (values, native
 * strings, native arrays, memory management, I/O) on top of the Phase 101
 * freestanding layer. Deterministic output is asserted by the Go gate
 * (pkg/runtime/phase102_core_test.go). */

int main(void) {
    karkain_core_print("phase102:core\n");

    /* --- fundamental value handling --- */
    Value vi = make_int(42);
    Value vf = make_float(2.5);
    Value vb = make_bool(1);
    Value vn = make_nil();
    Value vs = make_string("hello");

    karkain_core_print("int:"); karkain_core_print_value(vi); karkain_core_println();
    karkain_core_print("float:"); karkain_core_print_value(vf); karkain_core_println();
    karkain_core_print("bool:"); karkain_core_print_value(vb); karkain_core_println();
    karkain_core_print("nil:"); karkain_core_print_value(vn); karkain_core_println();
    karkain_core_print("kind:"); karkain_core_print(value_type_name(vi)); karkain_core_println();
    karkain_core_print("tag:"); karkain_core_print(value_type_tag(vf)); karkain_core_println();
    karkain_core_print("truthy int:"); karkain_core_print_int(value_truthy(vi)); karkain_core_println();
    karkain_core_print("truthy zero:"); karkain_core_print_int(value_truthy(make_float(0.0))); karkain_core_println();
    karkain_core_print("eq:"); karkain_core_print_int(values_equal(make_int(7), make_int(7))); karkain_core_println();
    karkain_core_print("neq:"); karkain_core_print_int(values_equal(make_string("a"), make_string("b"))); karkain_core_println();
    karkain_core_print("asint:"); karkain_core_print_int(value_as_int(make_float(3.9))); karkain_core_println();

    /* --- native strings --- */
    NativeString a = kns_from_cstr("hello");
    NativeString b = kns_from_cstr(" world");
    NativeString cat = kns_concat(a, b);
    NativeString sl = kns_slice(a, 1, 4);
    NativeString prefix = kns_from_cstr("hello");

    karkain_core_print("str:"); karkain_core_print_nstr(a); karkain_core_println();
    karkain_core_print("nlen:"); karkain_core_print_int(kns_len(a)); karkain_core_println();
    karkain_core_print("ncat:"); karkain_core_print_nstr(cat); karkain_core_println();
    karkain_core_print("nslice:"); karkain_core_print_nstr(sl); karkain_core_println();
    karkain_core_print("ncmp:"); karkain_core_print_int(kns_compare(kns_from_cstr("abc"), kns_from_cstr("abd"))); karkain_core_println();
    karkain_core_print("neqs:"); karkain_core_print_int(kns_equals(a, b)); karkain_core_println();
    karkain_core_print("nstart:"); karkain_core_print_int(kns_starts_with(cat, prefix)); karkain_core_println();
    Value sv = kns_to_value(cat);
    karkain_core_print("strvalue:"); karkain_core_print_value(sv); karkain_core_println();

    /* --- native arrays --- */
    NativeArray na = karr_new(2);
    karr_push(&na, make_int(10));
    karr_push(&na, make_int(20));
    karr_push(&na, make_int(30));
    karkain_core_print("arr len:"); karkain_core_print_int(karr_len(na)); karkain_core_println();
    karkain_core_print("arr[0]:"); karkain_core_print_value(karr_get(na, 0)); karkain_core_println();
    karkain_core_print("arr set:"); karkain_core_print_int(karr_set(&na, 0, make_int(99))); karkain_core_println();
    Value popped = karr_pop(&na);
    karkain_core_print("pop:"); karkain_core_print_value(popped); karkain_core_println();
    karkain_core_print("arr len:"); karkain_core_print_int(karr_len(na)); karkain_core_println();
    Value arrv = karr_to_value(&na);
    karkain_core_print("arrval:"); karkain_core_print_value(arrv); karkain_core_println();

    /* --- memory management --- */
    char* p = (char*)karkain_mem_alloc(10);
    if (!p) karkain_panic("alloc failed");
    karkain_memcpy(p, "abc", 3);
    p[3] = '\0';
    p = (char*)karkain_mem_realloc(p, 200); /* grow: copy must preserve bytes */
    karkain_core_print("realloc:");
    karkain_core_print_int(karkain_strcmp(p, "abc") == 0); karkain_core_println();

    void* q = karkain_mem_alloc(64);
    karkain_mem_free(q);
    size_t used = 0, freeb = 0;
    karkain_mem_stats(&used, &freeb);
    karkain_core_print("mem free:"); karkain_core_print_int(freeb > 0); karkain_core_println();

    char* z = (char*)karkain_mem_calloc(8, 1);
    int clean = 1;
    for (int i = 0; i < 8; i++) if (z[i] != 0) clean = 0;
    karkain_core_print("calloc:"); karkain_core_print_int(clean); karkain_core_println();
    karkain_mem_free(z);

    /* --- arena reset (freestanding integration) --- */
    karkain_arena_reset();
    karkain_mem_reset();
    void* after = karkain_mem_alloc(64);
    karkain_core_print("reset alloc:"); karkain_core_print_int(after != 0); karkain_core_println();

    /* --- I/O foundation / file reads --- */
    long fsz = 0;
    char* fdata = karkain_read_file("data.bin", &fsz);
    if (!fdata || fsz != 3) karkain_panic("read file failed");
    karkain_core_print("file:"); karkain_core_print(fdata); karkain_core_println();

    return 0;
}