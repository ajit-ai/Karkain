#include "karkain_json.h"

#include <ctype.h>
#include <limits.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define KARKAIN_JSON_NULL "__json_null__"

static char _karkain_json_error[256] = "";

static void karkain_json_err(const char* msg) {
    snprintf(_karkain_json_error, sizeof(_karkain_json_error), "%s", msg ? msg : "json failure");
}

Value karkain_json_last_error(void) { return make_string(_karkain_json_error); }

typedef struct {
    const char* start;
    const char* p;
} JParser;

typedef struct {
    char* data;
    size_t len;
    size_t cap;
} JBuf;

static void jb_init(JBuf* b) {
    b->cap = 128;
    b->len = 0;
    b->data = (char*)malloc(b->cap);
    if (b->data) b->data[0] = '\0';
}

static void jb_reserve(JBuf* b, size_t extra) {
    if (!b->data || b->len + extra + 1 <= b->cap) return;
    size_t cap = b->cap;
    while (cap < b->len + extra + 1) cap *= 2;
    char* nd = (char*)realloc(b->data, cap);
    if (nd) { b->data = nd; b->cap = cap; }
}

static void jb_putc(JBuf* b, char c) {
    jb_reserve(b, 1);
    if (!b->data) return;
    b->data[b->len++] = c;
    b->data[b->len] = '\0';
}

static void jb_puts(JBuf* b, const char* s) {
    size_t n = strlen(s);
    jb_reserve(b, n);
    if (!b->data) return;
    memcpy(b->data + b->len, s, n);
    b->len += n;
    b->data[b->len] = '\0';
}

static void jb_free(JBuf* b) { if (b->data) free(b->data); b->data = NULL; b->len = 0; b->cap = 0; }

static void jskip(JParser* j) {
    while (*j->p == ' ' || *j->p == '\t' || *j->p == '\n' || *j->p == '\r') j->p++;
}

static void jfail_at(JParser* j, const char* msg) {
    snprintf(_karkain_json_error, sizeof(_karkain_json_error), "json: %s at byte %d",
             msg, (int)(j->p - j->start));
}

static int hex4(JParser* j, unsigned* out) {
    unsigned v = 0;
    int i;
    for (i = 0; i < 4; i++) {
        char c = *j->p;
        unsigned d;
        if (!c) return 0;
        if (c >= '0' && c <= '9') d = (unsigned)(c - '0');
        else if (c >= 'a' && c <= 'f') d = (unsigned)(c - 'a' + 10);
        else if (c >= 'A' && c <= 'F') d = (unsigned)(c - 'A' + 10);
        else return 0;
        v = v * 16 + d;
        j->p++;
    }
    *out = v;
    return 1;
}

static void jutf8(JBuf* b, unsigned cp) {
    if (cp < 0x80) jb_putc(b, (char)cp);
    else if (cp < 0x800) {
        jb_putc(b, (char)(0xC0 | (cp >> 6)));
        jb_putc(b, (char)(0x80 | (cp & 0x3F)));
    } else if (cp < 0x10000) {
        jb_putc(b, (char)(0xE0 | (cp >> 12)));
        jb_putc(b, (char)(0x80 | ((cp >> 6) & 0x3F)));
        jb_putc(b, (char)(0x80 | (cp & 0x3F)));
    } else {
        jb_putc(b, (char)(0xF0 | (cp >> 18)));
        jb_putc(b, (char)(0x80 | ((cp >> 12) & 0x3F)));
        jb_putc(b, (char)(0x80 | ((cp >> 6) & 0x3F)));
        jb_putc(b, (char)(0x80 | (cp & 0x3F)));
    }
}

static Value jparse_value(JParser* j);

static int jparse_string(JParser* j, JBuf* out) {
    if (*j->p != '"') { jfail_at(j, "expected string"); return 0; }
    j->p++;
    for (;;) {
        char c = *j->p;
        if (!c) { jfail_at(j, "unterminated string"); return 0; }
        if (c == '"') { j->p++; return 1; }
        if (c != '\\') { jb_putc(out, c); j->p++; continue; }
        j->p++;
        c = *j->p++;
        if (c == '"') jb_putc(out, '"');
        else if (c == '\\') jb_putc(out, '\\');
        else if (c == '/') jb_putc(out, '/');
        else if (c == 'b') jb_putc(out, '\b');
        else if (c == 'f') jb_putc(out, '\f');
        else if (c == 'n') jb_putc(out, '\n');
        else if (c == 'r') jb_putc(out, '\r');
        else if (c == 't') jb_putc(out, '\t');
        else if (c == 'u') {
            unsigned cp;
            if (!hex4(j, &cp)) { jfail_at(j, "invalid escape"); return 0; }
            if (cp >= 0xD800 && cp <= 0xDBFF && j->p[0] == '\\' && j->p[1] == 'u') {
                unsigned lo;
                const char* save = j->p;
                j->p += 2;
                if (hex4(j, &lo) && lo >= 0xDC00 && lo <= 0xDFFF) {
                    cp = 0x10000 + ((cp - 0xD800) << 10) + (lo - 0xDC00);
                } else {
                    j->p = save;
                }
            }
            jutf8(out, cp);
        } else { jfail_at(j, "invalid escape"); return 0; }
    }
}

static Value jparse_value(JParser* j) {
    jskip(j);
    char c = *j->p;
    if (!c) { jfail_at(j, "unexpected end"); return make_string(""); }
    if (c == '{') {
        Value obj = make_map();
        j->p++;
        jskip(j);
        if (*j->p == '}') { j->p++; return obj; }
        for (;;) {
            JBuf key;
            jb_init(&key);
            if (!jparse_string(j, &key)) { jb_free(&key); return make_string(""); }
            jskip(j);
            if (*j->p != ':') { jb_free(&key); jfail_at(j, "expected colon"); return make_string(""); }
            j->p++;
            Value v = jparse_value(j);
            if (strcmp(_karkain_json_error, "") != 0) { jb_free(&key); return make_string(""); }
            map_set(&obj, make_string(key.data ? key.data : ""), v);
            jb_free(&key);
            jskip(j);
            if (*j->p == ',') { j->p++; jskip(j); continue; }
            if (*j->p == '}') { j->p++; return obj; }
            jfail_at(j, "expected comma or brace");
            return make_string("");
        }
    }
    if (c == '[') {
        Value arr = make_array();
        j->p++;
        jskip(j);
        if (*j->p == ']') { j->p++; return arr; }
        for (;;) {
            Value v = jparse_value(j);
            if (strcmp(_karkain_json_error, "") != 0) return make_string("");
            array_push(&arr, v);
            jskip(j);
            if (*j->p == ',') { j->p++; jskip(j); continue; }
            if (*j->p == ']') { j->p++; return arr; }
            jfail_at(j, "expected comma or bracket");
            return make_string("");
        }
    }
    if (c == '"') {
        JBuf s;
        jb_init(&s);
        if (!jparse_string(j, &s)) { jb_free(&s); return make_string(""); }
        Value v = make_string(s.data ? s.data : "");
        jb_free(&s);
        return v;
    }
    if (strncmp(j->p, "true", 4) == 0) { j->p += 4; return make_bool(1); }
    if (strncmp(j->p, "false", 5) == 0) { j->p += 5; return make_bool(0); }
    if (strncmp(j->p, "null", 4) == 0) { j->p += 4; return make_string(KARKAIN_JSON_NULL); }
    if (c == '-' || (c >= '0' && c <= '9')) {
        const char* start = j->p;
        int isint = 1;
        if (*j->p == '-') j->p++;
        while (*j->p >= '0' && *j->p <= '9') j->p++;
        if (*j->p == '.' || *j->p == 'e' || *j->p == 'E') isint = 0;
        if (*j->p == '.') {
            j->p++;
            while (*j->p >= '0' && *j->p <= '9') j->p++;
        }
        if (*j->p == 'e' || *j->p == 'E') {
            j->p++;
            if (*j->p == '+' || *j->p == '-') j->p++;
            while (*j->p >= '0' && *j->p <= '9') j->p++;
        }
        char tmp[128];
        size_t n = (size_t)(j->p - start);
        if (n >= sizeof(tmp)) { jfail_at(j, "number too long"); return make_string(""); }
        memcpy(tmp, start, n);
        tmp[n] = '\0';
        double d = strtod(tmp, NULL);
        if (isint && d >= (double)LLONG_MIN && d <= (double)LLONG_MAX) return make_int((long long)d);
        return make_float(d);
    }
    jfail_at(j, "unexpected character");
    return make_string("");
}

Value karkain_json_parse(Value text) {
    _karkain_json_error[0] = '\0';
    if (text.type != TYPE_STRING) { karkain_json_err("json: expected string"); return make_string(""); }
    JParser j;
    j.start = text.strVal ? text.strVal : "";
    j.p = j.start;
    Value v = jparse_value(&j);
    if (_karkain_json_error[0] != '\0') return make_string("");
    jskip(&j);
    if (*j.p != '\0') { jfail_at(&j, "trailing content"); return make_string(""); }
    return v;
}

Value karkain_json_type(Value node) {
    _karkain_json_error[0] = '\0';
    if (node.type == TYPE_STRING && node.strVal && strcmp(node.strVal, KARKAIN_JSON_NULL) == 0)
        return make_string("null");
    if (node.type == TYPE_STRING) return make_string("string");
    if (node.type == TYPE_INT || node.type == TYPE_FLOAT64)
        return make_string("number");
    if (node.type == TYPE_BOOL) return make_string("bool");
    if (node.type == TYPE_ARRAY) return make_string("array");
    if (node.type == TYPE_MAP) return make_string("object");
    return make_string("unknown");
}

static void jserialize(Value v, JBuf* out) {
    if (v.type == TYPE_STRING && v.strVal && strcmp(v.strVal, KARKAIN_JSON_NULL) == 0) {
        jb_puts(out, "null");
        return;
    }
    if (v.type == TYPE_STRING) {
        const char* s = v.strVal ? v.strVal : "";
        jb_putc(out, '"');
        for (; *s; s++) {
            char c = *s;
            if (c == '"') jb_puts(out, "\\\"");
            else if (c == '\\') jb_puts(out, "\\\\");
            else if (c == '\n') jb_puts(out, "\\n");
            else if (c == '\r') jb_puts(out, "\\r");
            else if (c == '\t') jb_puts(out, "\\t");
            else if ((unsigned char)c < 0x20) {
                char buf[8];
                snprintf(buf, sizeof(buf), "\\u%04x", (unsigned)(unsigned char)c);
                jb_puts(out, buf);
            } else jb_putc(out, c);
        }
        jb_putc(out, '"');
        return;
    }
    if (v.type == TYPE_INT) {
        char buf[32];
        snprintf(buf, sizeof(buf), "%lld", v.intVal);
        jb_puts(out, buf);
        return;
    }
    if (v.type == TYPE_FLOAT64) {
        char buf[64];
        snprintf(buf, sizeof(buf), "%.17g", v.floatVal);
        jb_puts(out, buf);
        return;
    }
    if (v.type == TYPE_BOOL) { jb_puts(out, v.intVal ? "true" : "false"); return; }
    // Use only the shared Value API. The Go and kcc preambles have different
    // struct field names for arrays/maps, but both provide karkain_len,
    // array_get, map_get and karkain_mapKeys.
    if (v.type == TYPE_ARRAY) {
        jb_putc(out, '[');
        long long n = karkain_len(v).intVal;
        long long i;
        for (i = 0; i < n; i++) {
            if (i) jb_putc(out, ',');
            jserialize(array_get(v, make_int(i)), out);
        }
        jb_putc(out, ']');
        return;
    }
    if (v.type == TYPE_MAP) {
        jb_putc(out, '{');
        Value keys = karkain_mapKeys(v);
        long long n = karkain_len(keys).intVal;
        long long i;
        for (i = 0; i < n; i++) {
            if (i) jb_putc(out, ',');
            Value key = array_get(keys, make_int(i));
            jserialize(key, out);
            jb_putc(out, ':');
            jserialize(map_get(v, key), out);
        }
        jb_putc(out, '}');
        return;
    }
    jb_puts(out, "null");
}

Value karkain_json_serialize(Value node) {
    _karkain_json_error[0] = '\0';
    JBuf out;
    jb_init(&out);
    jserialize(node, &out);
    Value r = make_string(out.data ? out.data : "");
    jb_free(&out);
    return r;
}

Value karkain_json_get_path(Value root, Value path) {
    _karkain_json_error[0] = '\0';
    if (path.type != TYPE_STRING) { karkain_json_err("json: path must be a string"); return make_int(0); }
    const char* p = path.strVal ? path.strVal : "";
    Value cur = root;
    char key[256];
    while (*p) {
        if (*p == '.') { p++; continue; }
        if (*p == '[') {
            p++;
            long idx = strtol(p, (char**)&p, 10);
            if (*p != ']') { karkain_json_err("json: malformed path"); return make_int(0); }
            p++;
            cur = array_get(cur, make_int(idx));
            continue;
        }
        size_t n = 0;
        while (*p && *p != '.' && *p != '[') {
            if (n + 1 >= sizeof(key)) { karkain_json_err("json: path component too long"); return make_int(0); }
            key[n++] = *p++;
        }
        key[n] = '\0';
        cur = map_get(cur, make_string(key));
    }
    return cur;
}
