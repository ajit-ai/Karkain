#ifndef KARKAIN_PLATFORM_H
#define KARKAIN_PLATFORM_H

/* Karkain freestanding platform layer.
 *
 * This header defines the ONLY OS boundary the libc-free runtime may touch:
 * raw syscall/OS-API writes, page allocation, and process exit. No libc
 * headers are used outside the freestanding subset (stdint/stddef).
 *
 *   Windows  : kernel32 boundary (VirtualAlloc/WriteFile/ExitProcess).
 *   Linux/POSIX: raw `syscall` instruction via inline assembly. No libc.
 */

#include <stdint.h>
#include <stddef.h>

/* OS page granularity used by the page allocator. */
#define KARKAIN_PAGESIZE 4096ul

#ifdef __cplusplus
extern "C" {
#endif

/* ---------- Memory (page allocator) ---------- */

/* Reserve+commit `size` bytes of zeroed pages. Returns NULL on failure. */
void* karkain_pages_alloc(unsigned long size);

/* Release pages previously returned by karkain_pages_alloc. */
void karkain_pages_free(void* p, unsigned long size);

/* ---------- Raw I/O ---------- */

/* Write `n` bytes of `buf` to stdout. Returns bytes written or -1. */
long karkain_write_out(const void* buf, unsigned long n);

/* Open `path` for reading. Returns a non-negative fd or -1. */
long karkain_open_read(const char* path);

/* Read up to `n` bytes into `buf` from fd. Returns bytes read, 0 on EOF, -1 on error. */
long karkain_read(long fd, void* buf, unsigned long n);

/* Write up to `n` bytes to fd. Returns bytes written or -1. */
long karkain_write(long fd, const void* buf, unsigned long n);

/* Close fd. */
void karkain_close(long fd);

/* ---------- Exit ---------- */

/* Terminate the process with `code`. Does not return. */
void karkain_exit(int code);

#ifdef __cplusplus
}
#endif

#endif /* KARKAIN_PLATFORM_H */