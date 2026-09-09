#include "karkain_nstr.h"

#include "karkain_mem.h"
#include "karkain_runtime.h"

NativeString kns_empty(void) {
    NativeString s;
    s.len = 0;
    s.data = (char*)karkain_mem_alloc(1);
    if (s.data) s.data[0] = '\0';
    return s;
}

NativeString kns_from_bytes(const char* s, long n) {
    NativeString out;
    if (n < 0) n = 0;
    out.len = n;
    out.data = (char*)karkain_mem_alloc((size_t)n + 1);
    if (out.data) {
        if (n > 0) karkain_memcpy(out.data, s, (size_t)n);
        out.data[n] = '\0';
    }
    return out;
}

NativeString kns_from_cstr(const char* s) {
    if (!s) return kns_empty();
    return kns_from_bytes(s, (long)karkain_strlen(s));
}

long kns_len(NativeString s) { return s.len; }

NativeString kns_concat(NativeString a, NativeString b) {
    NativeString out;
    out.len = a.len + b.len;
    out.data = (char*)karkain_mem_alloc((size_t)out.len + 1);
    if (out.data) {
        karkain_memcpy(out.data, a.data, (size_t)a.len);
        karkain_memcpy(out.data + a.len, b.data, (size_t)b.len);
        out.data[out.len] = '\0';
    }
    return out;
}

NativeString kns_append_char(NativeString s, char c) {
    NativeString out;
    out.len = s.len + 1;
    out.data = (char*)karkain_mem_alloc((size_t)out.len + 1);
    if (out.data) {
        karkain_memcpy(out.data, s.data, (size_t)s.len);
        out.data[s.len] = c;
        out.data[out.len] = '\0';
    }
    return out;
}

NativeString kns_slice(NativeString s, long start, long end) {
    if (start < 0) start = 0;
    if (end > s.len) end = s.len;
    if (start >= end) return kns_empty();
    return kns_from_bytes(s.data + start, end - start);
}

int kns_compare(NativeString a, NativeString b) {
    long n = a.len < b.len ? a.len : b.len;
    int c = karkain_strncmp(a.data, b.data, (size_t)n);
    if (c != 0) return c < 0 ? -1 : 1;
    if (a.len == b.len) return 0;
    return a.len < b.len ? -1 : 1;
}

int kns_equals(NativeString a, NativeString b) {
    if (a.len != b.len) return 0;
    return karkain_strncmp(a.data, b.data, (size_t)a.len) == 0;
}

int kns_starts_with(NativeString s, NativeString prefix) {
    if (prefix.len > s.len) return 0;
    return karkain_strncmp(s.data, prefix.data, (size_t)prefix.len) == 0;
}

Value kns_to_value(NativeString s) {
    return make_string_n(s.data, s.len);
}

NativeString kns_from_value(Value v) {
    if (v.type != TYPE_STRING) return kns_empty();
    return kns_from_cstr(v.strVal ? v.strVal : "");
}