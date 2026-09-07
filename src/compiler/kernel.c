/* ============================================================
 * Karkain C Kernel (Layer 0) — COMPLETE
 * Only raw primitives that CANNOT be written in Karkain:
 *   - Memory (malloc/free/realloc)
 *   - String operations requiring pointer arithmetic
 *   - File I/O
 *   - Console output
 *   - Math (from libm)
 *   - System calls
 *   - Program arguments
 *
 * Every function here exists because Karkain cannot do pointer
 * arithmetic. Everything above this is pure Karkain.
 *
 * Compile: gcc -std=c11 -o karkain kernel.c -lm
 * ============================================================ */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <math.h>
#include <ctype.h>
#ifdef _WIN32
#include <direct.h>
#include <sys/stat.h>
#else
#include <sys/stat.h>
#include <sys/types.h>
#include <unistd.h>
#endif

/* ============================================================
 * Memory
 * ============================================================ */

void* k_alloc(int size) { return malloc(size); }
void  k_free(void* ptr) { free(ptr); }
void* k_realloc(void* ptr, int newSize) { return realloc(ptr, newSize); }

/* ============================================================
 * String primitives (pointer arithmetic — can't do in Karkain)
 * ============================================================ */

int k_strlen(const char* s) {
    return s ? (int)strlen(s) : 0;
}

int k_strcmp_raw(const char* a, const char* b) {
    return strcmp(a ? a : "", b ? b : "");
}

/* Substring: allocate + copy. Returns empty string on bad input. */
char* k_substr(const char* s, int start, int length) {
    if (!s) { char* r = malloc(1); r[0] = '\0'; return r; }
    int slen = (int)strlen(s);
    if (start < 0) start = 0;
    if (start >= slen) { char* r = malloc(1); r[0] = '\0'; return r; }
    if (start + length > slen) length = slen - start;
    if (length < 0) length = 0;
    char* buf = malloc(length + 1);
    memcpy(buf, s + start, length);
    buf[length] = '\0';
    return buf;
}

/* Character at index (as single-char string). Returns "" on bad input. */
char* k_char_at(const char* s, int idx) {
    if (!s) { char* r = malloc(1); r[0] = '\0'; return r; }
    int slen = (int)strlen(s);
    if (idx < 0 || idx >= slen) { char* r = malloc(1); r[0] = '\0'; return r; }
    char* buf = malloc(2);
    buf[0] = s[idx];
    buf[1] = '\0';
    return buf;
}

/* String concatenation. Returns newly allocated string. */
char* k_strcat(const char* a, const char* b) {
    if (!a) a = "";
    if (!b) b = "";
    int la = (int)strlen(a);
    int lb = (int)strlen(b);
    char* buf = malloc(la + lb + 1);
    memcpy(buf, a, la);
    memcpy(buf + la, b, lb + 1);
    return buf;
}

/* 3-way string concatenation. */
char* k_strcat3(const char* a, const char* b, const char* c) {
    if (!a) a = "";
    if (!b) b = "";
    if (!c) c = "";
    int la = (int)strlen(a);
    int lb = (int)strlen(b);
    int lc = (int)strlen(c);
    char* buf = malloc(la + lb + lc + 1);
    memcpy(buf, a, la);
    memcpy(buf + la, b, lb);
    memcpy(buf + la + lb, c, lc + 1);
    return buf;
}

/* String equality: 1 if equal, 0 otherwise. */
int k_streq(const char* a, const char* b) {
    if (!a && !b) return 1;
    if (!a || !b) return 0;
    return strcmp(a, b) == 0;
}

/* Find first occurrence of sub in s. Returns index or -1. */
int k_str_index(const char* s, const char* sub) {
    if (!s || !sub || !*sub) return -1;
    const char* p = strstr(s, sub);
    return p ? (int)(p - s) : -1;
}

/* Find last occurrence of sub in s. Returns index or -1. */
int k_str_rindex(const char* s, const char* sub) {
    if (!s || !sub || !*sub) return -1;
    int slen = (int)strlen(s);
    int sublen = (int)strlen(sub);
    for (int i = slen - sublen; i >= 0; i--) {
        if (memcmp(s + i, sub, sublen) == 0) return i;
    }
    return -1;
}

/* Check if s starts with prefix. */
int k_str_hasprefix(const char* s, const char* prefix) {
    if (!s || !prefix) return 0;
    return strncmp(s, prefix, strlen(prefix)) == 0;
}

/* Check if s ends with suffix. */
int k_str_hassuffix(const char* s, const char* suffix) {
    if (!s || !suffix) return 0;
    int slen = (int)strlen(s);
    int suflen = (int)strlen(suffix);
    if (suflen > slen) return 0;
    return strcmp(s + slen - suflen, suffix) == 0;
}

/* Count non-overlapping occurrences of sub in s. */
int k_str_count(const char* s, const char* sub) {
    if (!s || !sub || !*sub) return 0;
    int count = 0;
    int sublen = (int)strlen(sub);
    const char* p = s;
    while ((p = strstr(p, sub)) != NULL) {
        count++;
        p += sublen;
    }
    return count;
}

/* Trim leading and trailing whitespace. Returns newly allocated. */
char* k_str_trim(const char* s) {
    if (!s) { char* r = malloc(1); r[0] = '\0'; return r; }
    while (*s && isspace((unsigned char)*s)) s++;
    if (!*s) { char* r = malloc(1); r[0] = '\0'; return r; }
    const char* end = s + strlen(s) - 1;
    while (end > s && isspace((unsigned char)*end)) end--;
    int len = (int)(end - s + 1);
    char* buf = malloc(len + 1);
    memcpy(buf, s, len);
    buf[len] = '\0';
    return buf;
}

/* Trim prefix if present. Returns newly allocated. */
char* k_str_trimprefix(const char* s, const char* prefix) {
    if (!s || !prefix) return strdup(s ? s : "");
    if (k_str_hasprefix(s, prefix)) {
        return strdup(s + strlen(prefix));
    }
    return strdup(s);
}

/* Trim suffix if present. Returns newly allocated. */
char* k_str_trimsuffix(const char* s, const char* suffix) {
    if (!s || !suffix) return strdup(s ? s : "");
    if (k_str_hassuffix(s, suffix)) {
        int len = (int)strlen(s) - (int)strlen(suffix);
        char* buf = malloc(len + 1);
        memcpy(buf, s, len);
        buf[len] = '\0';
        return buf;
    }
    return strdup(s);
}

/* To lowercase. Returns newly allocated. */
char* k_str_tolower(const char* s) {
    if (!s) { char* r = malloc(1); r[0] = '\0'; return r; }
    int len = (int)strlen(s);
    char* buf = malloc(len + 1);
    for (int i = 0; i < len; i++) buf[i] = tolower((unsigned char)s[i]);
    buf[len] = '\0';
    return buf;
}

/* To uppercase. Returns newly allocated. */
char* k_str_toupper(const char* s) {
    if (!s) { char* r = malloc(1); r[0] = '\0'; return r; }
    int len = (int)strlen(s);
    char* buf = malloc(len + 1);
    for (int i = 0; i < len; i++) buf[i] = toupper((unsigned char)s[i]);
    buf[len] = '\0';
    return buf;
}

/* Replace all occurrences. n=-1 means replace all. Returns newly allocated. */
char* k_str_replace(const char* s, const char* old, const char* repl, int n) {
    if (!s || !old || !repl) return strdup(s ? s : "");
    if (!*old) return strdup(s);
    int oldlen = (int)strlen(old);
    int replen = (int)strlen(repl);
    int count = k_str_count(s, old);
    if (n >= 0 && n < count) count = n;
    if (count == 0) return strdup(s);
    int slen = (int)strlen(s);
    int resultlen = slen + count * (replen - oldlen);
    char* buf = malloc(resultlen + 1);
    char* p = buf;
    const char* cur = s;
    int remaining = count;
    while (remaining > 0) {
        const char* found = strstr(cur, old);
        if (!found) break;
        int segment = (int)(found - cur);
        memcpy(p, cur, segment);
        p += segment;
        memcpy(p, repl, replen);
        p += replen;
        cur = found + oldlen;
        remaining--;
    }
    strcpy(p, cur);
    return buf;
}

/* Repeat string n times. Returns newly allocated. */
char* k_str_repeat(const char* s, int n) {
    if (!s || n <= 0) { char* r = malloc(1); r[0] = '\0'; return r; }
    int slen = (int)strlen(s);
    char* buf = malloc(slen * n + 1);
    for (int i = 0; i < n; i++) memcpy(buf + i * slen, s, slen);
    buf[slen * n] = '\0';
    return buf;
}

/* ============================================================
 * Split string into array of strings.
 * Returns array count via out_count. Array is malloc'd.
 * ============================================================ */

char** k_str_split(const char* s, const char* sep, int* out_count) {
    if (!s || !sep || !*sep) {
        *out_count = 0;
        return NULL;
    }
    int slen = (int)strlen(s);
    int seplen = (int)strlen(sep);
    
    /* Count occurrences to pre-allocate */
    int count = k_str_count(s, sep) + 1;
    char** result = malloc(sizeof(char*) * count);
    int idx = 0;
    
    const char* cur = s;
    const char* found;
    while ((found = strstr(cur, sep)) != NULL) {
        int segment = (int)(found - cur);
        result[idx] = malloc(segment + 1);
        memcpy(result[idx], cur, segment);
        result[idx][segment] = '\0';
        idx++;
        cur = found + seplen;
    }
    /* Last segment */
    result[idx] = strdup(cur);
    idx++;
    
    *out_count = idx;
    return result;
}

/* ============================================================
 * Join array of strings with separator.
 * Returns newly allocated string.
 * ============================================================ */

char* k_str_join(char** parts, int count, const char* sep) {
    if (count <= 0) { char* r = malloc(1); r[0] = '\0'; return r; }
    if (!sep) sep = "";
    int seplen = (int)strlen(sep);
    
    /* Calculate total length */
    int total = 0;
    for (int i = 0; i < count; i++) {
        total += (int)strlen(parts[i]);
        if (i < count - 1) total += seplen;
    }
    
    char* buf = malloc(total + 1);
    char* p = buf;
    for (int i = 0; i < count; i++) {
        int len = (int)strlen(parts[i]);
        memcpy(p, parts[i], len);
        p += len;
        if (i < count - 1) {
            memcpy(p, sep, seplen);
            p += seplen;
        }
    }
    *p = '\0';
    return buf;
}

/* ============================================================
 * Integer to string (base 10, decimal)
 * ============================================================ */

char* k_int_to_str(long long val) {
    char buf[64];
    int neg = 0;
    unsigned long long uval;
    if (val < 0) { neg = 1; uval = (unsigned long long)(-(val + 1)) + 1; }
    else { uval = (unsigned long long)val; }
    int i = 0;
    if (uval == 0) { buf[i++] = '0'; }
    else {
        while (uval > 0) { buf[i++] = '0' + (int)(uval % 10); uval /= 10; }
    }
    char* result = malloc(neg + i + 1);
    int pos = 0;
    if (neg) result[pos++] = '-';
    for (int j = i - 1; j >= 0; j--) result[pos++] = buf[j];
    result[pos] = '\0';
    return result;
}

/* Integer to hex string. */
char* k_int_to_hex(long long val) {
    char buf[64];
    unsigned long long uval = (unsigned long long)val;
    int i = 0;
    const char* digits = "0123456789abcdef";
    if (uval == 0) { buf[i++] = '0'; }
    else {
        while (uval > 0) { buf[i++] = digits[uval % 16]; uval /= 16; }
    }
    char* result = malloc(i + 1);
    for (int j = 0; j < i; j++) result[j] = buf[i - 1 - j];
    result[i] = '\0';
    return result;
}

/* Integer to octal string. */
char* k_int_to_octal(long long val) {
    char buf[64];
    unsigned long long uval = (unsigned long long)val;
    int i = 0;
    if (uval == 0) { buf[i++] = '0'; }
    else {
        while (uval > 0) { buf[i++] = '0' + (int)(uval % 8); uval /= 8; }
    }
    char* result = malloc(i + 1);
    for (int j = 0; j < i; j++) result[j] = buf[i - 1 - j];
    result[i] = '\0';
    return result;
}

/* Integer to binary string. */
char* k_int_to_binary(long long val) {
    char buf[128];
    unsigned long long uval = (unsigned long long)val;
    int i = 0;
    if (uval == 0) { buf[i++] = '0'; }
    else {
        while (uval > 0) { buf[i++] = '0' + (int)(uval % 2); uval /= 2; }
    }
    char* result = malloc(i + 1);
    for (int j = 0; j < i; j++) result[j] = buf[i - 1 - j];
    result[i] = '\0';
    return result;
}

/* String to integer. */
long long k_str_to_int(const char* s) {
    if (!s || !*s) return 0;
    long long result = 0;
    int neg = 0;
    const char* p = s;
    while (*p && isspace((unsigned char)*p)) p++;
    if (*p == '-') { neg = 1; p++; }
    else if (*p == '+') { p++; }
    while (*p >= '0' && *p <= '9') { result = result * 10 + (*p - '0'); p++; }
    return neg ? -result : result;
}

/* ============================================================
 * Console output
 * ============================================================ */

void k_print_raw(const char* s) {
    fputs(s ? s : "(null)", stdout);
    putchar('\n');
    fflush(stdout);
}

void k_print_noraw(const char* s) {
    fputs(s ? s : "(null)", stdout);
    fflush(stdout);
}

void k_putchar(int c) {
    putchar(c);
    fflush(stdout);
}

/* ============================================================
 * File I/O
 * ============================================================ */

void* k_file_open(const char* path, const char* mode) {
    return fopen(path, mode);
}

void k_file_close(void* handle) {
    if (handle) fclose((FILE*)handle);
}

char* k_file_read_all(const char* path, int* out_len) {
    FILE* f = fopen(path, "rb");
    if (!f) { *out_len = 0; return NULL; }
    fseek(f, 0, SEEK_END);
    long sz = ftell(f);
    fseek(f, 0, SEEK_SET);
    if (sz <= 0) { fclose(f); *out_len = 0; return NULL; }
    char* buf = (char*)malloc(sz + 1);
    size_t n = fread(buf, 1, sz, f);
    fclose(f);
    buf[n] = '\0';
    *out_len = (int)n;
    return buf;
}

char* k_file_readline(void* handle) {
    if (!handle) return NULL;
    FILE* f = (FILE*)handle;
    char buf[8192];
    if (fgets(buf, sizeof(buf), f) == NULL) return NULL;
    int len = (int)strlen(buf);
    while (len > 0 && (buf[len-1] == '\n' || buf[len-1] == '\r')) buf[--len] = '\0';
    return strdup(buf);
}

int k_file_write(void* handle, const char* data, int len) {
    if (!handle) return 0;
    return (int)fwrite(data, 1, len, (FILE*)handle);
}

int k_file_write_path(const char* path, const char* data) {
    FILE* f = fopen(path, "wb");
    if (!f) return 0;
    fputs(data, f);
    fclose(f);
    return 1;
}

int k_file_exists(const char* path) {
    FILE* f = fopen(path, "rb");
    if (f) { fclose(f); return 1; }
    return 0;
}

int k_file_remove(const char* path) {
    return remove(path) == 0 ? 1 : 0;
}

int k_mkdir(const char* path) {
#ifdef _WIN32
    return _mkdir(path) == 0 ? 1 : 0;
#else
    return mkdir(path, 0755) == 0 ? 1 : 0;
#endif
}

int k_rename_file(const char* old, const char* new_name) {
    return rename(old, new_name) == 0 ? 1 : 0;
}

/* ============================================================
 * Environment & Process
 * ============================================================ */

char* k_getenv_raw(const char* key) {
    char* val = getenv(key);
    return val ? strdup(val) : strdup("");
}

int k_setenv_raw(const char* key, const char* val) {
#ifdef _WIN32
    return _putenv_s(key, val) == 0 ? 1 : 0;
#else
    return setenv(key, val, 1) == 0 ? 1 : 0;
#endif
}

void k_exit(int code) {
    exit(code);
}

/* ============================================================
 * Math (wrappers around libm)
 * ============================================================ */

double k_math_abs(double x)    { return fabs(x); }
double k_math_floor(double x)  { return floor(x); }
double k_math_ceil(double x)   { return ceil(x); }
double k_math_round(double x)  { return round(x); }
double k_math_sqrt(double x)   { return sqrt(x); }
double k_math_pow(double x, double y) { return pow(x, y); }
double k_math_log(double x)    { return log(x); }
double k_math_log10(double x)  { return log10(x); }
double k_math_sin(double x)    { return sin(x); }
double k_math_cos(double x)    { return cos(x); }
double k_math_tan(double x)    { return tan(x); }
double k_math_asin(double x)   { return asin(x); }
double k_math_acos(double x)   { return acos(x); }
double k_math_atan(double x)   { return atan(x); }
double k_math_atan2(double y, double x) { return atan2(y, x); }
double k_math_mod(double x, double y)   { return fmod(x, y); }
double k_math_pi(void)         { return 3.14159265358979323846; }
double k_math_e(void)          { return 2.71828182845904523536; }

/* ============================================================
 * Program arguments
 * ============================================================ */

static int _k_argc = 0;
static char** _k_argv = NULL;

void k_set_args(int argc, char** argv) { _k_argc = argc; _k_argv = argv; }
int  k_get_argc(void) { return _k_argc; }
const char* k_get_argv(int idx) {
    if (idx < 0 || idx >= _k_argc) return "";
    return _k_argv[idx];
}

/* ============================================================
 * Type name helper (for typeof)
 * ============================================================ */

/* typeof is handled at codegen level — maps type tags to strings */
/* This is a placeholder; the compiler emits typeof checks */

/* ============================================================
 * Entry point
 * ============================================================ */

extern int kar_main(void);

int main(int argc, char* argv[]) {
    k_set_args(argc, argv);
    return kar_main();
}
