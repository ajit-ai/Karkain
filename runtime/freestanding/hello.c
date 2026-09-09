#include "karkain_runtime.h"

/* Phase 101 gate: a fully freestanding (no libc) Karkain runtime smoke test.
 * Compiled with -ffreestanding -nostdlib; exercises output, integers,
 * no-libm math, the arena, string ops and raw file I/O, then exits 0. */

int main(void) {
    karkain_print("hello\n");

    karkain_print("i=");
    karkain_print_int(-42);
    karkain_putchar('\n');

    karkain_print("sqrt2=");
    karkain_print_double(karkain_sqrt(2.0));
    karkain_putchar('\n');

    karkain_print("floor2.7=");
    karkain_print_int((long long)karkain_floor(2.7));
    karkain_putchar('\n');

    karkain_print("fmod=");
    karkain_print_double(karkain_fmod(7.5, 2.0));
    karkain_putchar('\n');

    karkain_print("pow=");
    karkain_print_double(karkain_pow(2.0, 0.5));
    karkain_putchar('\n');

    karkain_print("parse=");
    karkain_print_int(karkain_strtoll("  -42", 0));
    karkain_putchar('\n');

    karkain_print("parse2=");
    karkain_print_int(karkain_strtoll("42abc", 0));
    karkain_putchar('\n');

    karkain_print("cmpabc=");
    karkain_print_int(karkain_strcmp("abc", "abc"));
    karkain_putchar('\n');

    karkain_print("cmpl=");
    karkain_print_int(karkain_strcmp("abc", "abd"));
    karkain_putchar('\n');

    void* a = karkain_arena_alloc(100);
    if (!a) karkain_panic("arena alloc failed");
    karkain_memset(a, 0xAB, 100);
    {
        unsigned char* bytes = (unsigned char*)a;
        if (bytes[0] != 0xAB || bytes[99] != 0xAB) karkain_panic("arena contents corrupt");
    }
    karkain_print("arena: ok\n");

    long sz = 0;
    char* data = karkain_read_file("data.bin", &sz);
    if (!data || sz != 3) karkain_panic("read file failed");
    karkain_print("file=");
    karkain_print(data);
    karkain_putchar('\n');

    return 0;
}