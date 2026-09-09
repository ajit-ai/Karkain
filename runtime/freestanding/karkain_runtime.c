#include "karkain_runtime.h"
#include "karkain_platform.h"

/* ---------- Entry point ---------- */

/* Mingw injects a call to __main atop any definition of main; with -nostdlib
 * the CRT stub is absent, so provide the no-op ourselves. */
void __main(void) { }

void _start(void) {
    int code = main();
    karkain_exit(code);
}

/* ---------- Output ---------- */

void karkain_putchar(char c) {
    (void)karkain_write_out(&c, 1);
}

void karkain_print(const char* s) {
    if (s) {
        size_t n = karkain_strlen(s);
        (void)karkain_write_out(s, (unsigned long)n);
    }
}

void karkain_println(const char* s) {
    karkain_print(s);
    karkain_putchar('\n');
}

static void karkain_print_u64(unsigned long long v) {
    char buf[24];
    int n = 0;
    if (v == 0) {
        karkain_putchar('0');
        return;
    }
    while (v > 0) {
        buf[n++] = (char)('0' + (v % 10));
        v /= 10;
    }
    while (n > 0) {
        karkain_putchar(buf[--n]);
    }
}

void karkain_print_int(long long v) {
    if (v < 0) {
        karkain_putchar('-');
        karkain_print_u64((unsigned long long)(-(v + 1)) + 1u);
    } else {
        karkain_print_u64((unsigned long long)v);
    }
}

static void karkain_print_digits(unsigned long long v, int digits, double scale) {
    /* Emits exactly `digits` decimal places, zero-padded. */
    unsigned long long div = (unsigned long long)(scale / 10.0);
    while (div > 0) {
        karkain_putchar((char)('0' + (int)((v / div) % 10)));
        div /= 10;
    }
}

void karkain_print_double(double v) {
    const int decimals = 6;

    if (v != v) { karkain_print("nan"); return; }
    if (v > 1e308) { karkain_print("inf"); return; }
    if (v < -1e308) { karkain_print("-inf"); return; }

    if (v < 0.0) {
        karkain_putchar('-');
        v = -v;
    }
    if (v > 9.007199254740992e15) {
        karkain_print("<overflow>");
        return;
    }

    double scale = 1.0;
    for (int i = 0; i < decimals; i++) scale *= 10.0;

    unsigned long long ipart = (unsigned long long)karkain_floor(v);
    double frac = v - (double)ipart;
    unsigned long long fpart = (unsigned long long)karkain_floor(frac * scale + 0.5);
    if (fpart >= (unsigned long long)scale) {
        fpart = 0;
        ipart++;
    }

    karkain_print_u64(ipart);
    karkain_putchar('.');
    karkain_print_digits(fpart, decimals, scale);
}

void karkain_panic(const char* msg) {
    karkain_print("panic: ");
    karkain_print(msg);
    karkain_putchar('\n');
    karkain_exit(1);
}