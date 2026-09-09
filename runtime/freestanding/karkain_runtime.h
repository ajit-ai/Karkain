#ifndef KARKAIN_RUNTIME_H
#define KARKAIN_RUNTIME_H

/* Karkain libc-free runtime public API.
 *
 * A freestanding Karkain program links this runtime and defines `main`.
 * The runtime provides its own `_start` entry point, output, panic, an arena
 * allocator, string operations, and no-libm math. Compile with:
 *
 *   gcc -std=c11 -ffreestanding -nostdlib -fno-builtin \
 *       prog.c karkain_runtime.c karkain_memory.c karkain_io.c \
 *            karkain_string.c karkain_math.c \
 *       -o prog -lkernel32        # Windows
 *
 *   gcc -std=c11 -ffreestanding -nostdlib -fno-builtin \
 *       prog.c karkain_runtime.c karkain_memory.c karkain_io.c \
 *            karkain_string.c karkain_math.c \
 *       -o prog                   # Linux/macOS (raw syscalls)
 *
 * On Windows the entry point is selected with -Wl,-e,_start.
 */

#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

/* The user program. The runtime calls it and exits with its return value. */
int main(void);

/* ---------- Output ---------- */

void karkain_putchar(char c);
void karkain_print(const char* s);
void karkain_print_int(long long v);
void karkain_print_double(double v);
void karkain_println(const char* s);

/* Panic: prints "panic: <msg>" to stderr-adjacent output and exits 1. */
void karkain_panic(const char* msg);

/* ---------- Arena allocator ---------- */

/* Allocation from a page-backed bump arena. Never freed. Returns null on OOM. */
void* karkain_arena_alloc(size_t size);

/* Release all arena pages and reset to empty. All prior arena pointers are
 * invalidated. */
void karkain_arena_reset(void);

/* ---------- Files ---------- */

/* Read an entire file into an arena buffer (NUL-terminated). NULL on error. */
char* karkain_read_file(const char* path, long* out_size);

/* ---------- String ---------- */

size_t karkain_strlen(const char* s);
void* karkain_memcpy(void* dst, const void* src, size_t n);
void* karkain_memmove(void* dst, const void* src, size_t n);
void* karkain_memset(void* dst, int c, size_t n);
int karkain_strcmp(const char* a, const char* b);
int karkain_strncmp(const char* a, const char* b, size_t n);
void karkain_strcpy(char* dst, const char* src);
void karkain_strcat(char* dst, const char* src);

/* Signed decimal parse (strtol subset). */
long long karkain_strtoll(const char* s, char** end);

/* ---------- Math (no libm) ---------- */

double karkain_fabs(double x);
double karkain_copysign(double x, double y);
double karkain_floor(double x);
double karkain_ceil(double x);
double karkain_trunc(double x);
double karkain_round(double x);
double karkain_fmod(double x, double y);
double karkain_sqrt(double x);
double karkain_exp(double x);
double karkain_log(double x);
double karkain_pow(double x, double y);
double karkain_fmin(double a, double b);
double karkain_fmax(double a, double b);

#ifdef __cplusplus
}
#endif

#endif /* KARKAIN_RUNTIME_H */