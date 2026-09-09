#include "karkain_runtime.h"
#include "karkain_platform.h"

/* File I/O helpers on top of raw syscalls. `read_entire` returns a NUL-
 * terminated buffer allocated from the arena, or NULL on failure. */

static long karkain_file_size(const char* path) {
    long fd = karkain_open_read(path);
    if (fd < 0) return -1;
    char chunk[512];
    long total = 0;
    for (;;) {
        long n = karkain_read(fd, chunk, sizeof(chunk));
        if (n <= 0) break;
        total += n;
    }
    karkain_close(fd);
    return total;
}

char* karkain_read_file(const char* path, long* out_size) {
    long size = karkain_file_size(path);
    if (size < 0) return NULL;

    char* buf = (char*)karkain_arena_alloc((size_t)size + 1);
    if (!buf) return NULL;

    long fd = karkain_open_read(path);
    if (fd < 0) return NULL;
    long got = 0;
    while (got < size) {
        long n = karkain_read(fd, buf + got, (unsigned long)(size - got));
        if (n <= 0) break;
        got += n;
    }
    karkain_close(fd);
    buf[size] = '\0';
    if (out_size) *out_size = size;
    return buf;
}