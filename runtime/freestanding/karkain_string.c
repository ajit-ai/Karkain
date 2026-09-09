#include "karkain_runtime.h"

size_t karkain_strlen(const char* s) {
    const char* p = s;
    while (*p) p++;
    return (size_t)(p - s);
}

void* karkain_memcpy(void* dst, const void* src, size_t n) {
    unsigned char* d = (unsigned char*)dst;
    const unsigned char* s = (const unsigned char*)src;
    for (size_t i = 0; i < n; i++) d[i] = s[i];
    return dst;
}

void* karkain_memmove(void* dst, const void* src, size_t n) {
    unsigned char* d = (unsigned char*)dst;
    const unsigned char* s = (const unsigned char*)src;
    if (d < s) {
        for (size_t i = 0; i < n; i++) d[i] = s[i];
    } else if (d > s) {
        for (size_t i = n; i > 0; i--) d[i - 1] = s[i - 1];
    }
    return dst;
}

void* karkain_memset(void* dst, int c, size_t n) {
    unsigned char* d = (unsigned char*)dst;
    while (n-- > 0) *d++ = (unsigned char)c;
    return dst;
}

int karkain_strcmp(const char* a, const char* b) {
    while (*a && *a == *b) { a++; b++; }
    return (int)((unsigned char)*a - (unsigned char)*b);
}

int karkain_strncmp(const char* a, const char* b, size_t n) {
    for (size_t i = 0; i < n; i++) {
        if (a[i] != b[i]) return (int)((unsigned char)a[i] - (unsigned char)b[i]);
        if (a[i] == '\0') return 0;
    }
    return 0;
}

void karkain_strcpy(char* dst, const char* src) {
    while ((*dst++ = *src++) != '\0') { }
}

void karkain_strcat(char* dst, const char* src) {
    karkain_strcpy(dst + karkain_strlen(dst), src);
}

long long karkain_strtoll(const char* s, char** end) {
    long long sign = 1;
    const char* p = s;
    while (*p == ' ' || *p == '\t') p++;
    if (*p == '-') { sign = -1; p++; }
    else if (*p == '+') { p++; }
    long long acc = 0;
    int any = 0;
    while (*p >= '0' && *p <= '9') {
        acc = acc * 10 + (long long)(*p - '0');
        p++;
        any = 1;
    }
    if (end) *end = any ? (char*)p : (char*)s;
    return sign * acc;
}