#include "karkain_osmetrics.h"

#include <ctype.h>

#define KARKAIN_OS_MAX_PROCESSES 64
#define KARKAIN_OS_NAME_LEN 300

static char _karkain_os_error[256] = "";

static void karkain_os_err(const char* msg) {
    snprintf(_karkain_os_error, sizeof(_karkain_os_error), "%s", msg ? msg : "os failure");
}

Value karkain_os_last_error(void) { return make_string(_karkain_os_error); }

Value karkain_os_pid(void) {
    _karkain_os_error[0] = '\0';
#ifdef _WIN32
    return make_int((long long)GetCurrentProcessId());
#else
    return make_int((long long)getpid());
#endif
}

Value karkain_os_cpu_count(void) {
    _karkain_os_error[0] = '\0';
#ifdef _WIN32
    SYSTEM_INFO si;
    GetSystemInfo(&si);
    if (si.dwNumberOfProcessors < 1) { karkain_os_err("cpu count unavailable"); return make_int(-1); }
    return make_int((long long)si.dwNumberOfProcessors);
#else
    long n = sysconf(_SC_NPROCESSORS_ONLN);
    if (n < 1) { karkain_os_err("cpu count unavailable"); return make_int(-1); }
    return make_int((long long)n);
#endif
}

#ifndef _WIN32
static long long karkain_os_meminfo_kb(const char* key) {
    FILE* f = fopen("/proc/meminfo", "r");
    char line[256];
    size_t keylen = strlen(key);
    long long out = -1;
    if (!f) { karkain_os_err("meminfo unavailable"); return -1; }
    while (fgets(line, sizeof(line), f)) {
        if (strncmp(line, key, keylen) == 0) {
            out = strtoll(line + keylen, NULL, 10) / 1024;
            break;
        }
    }
    fclose(f);
    if (out < 0) karkain_os_err("meminfo field unavailable");
    return out;
}
#endif

Value karkain_os_total_mem_mb(void) {
    _karkain_os_error[0] = '\0';
#ifdef _WIN32
    MEMORYSTATUSEX st;
    st.dwLength = sizeof(st);
    if (!GlobalMemoryStatusEx(&st)) { karkain_os_err("memory status failed"); return make_int(-1); }
    return make_int((long long)(st.ullTotalPhys / 1048576ULL));
#elif defined(__APPLE__)
    uint64_t bytes = 0;
    size_t len = sizeof(bytes);
    if (sysctlbyname("hw.memsize", &bytes, &len, NULL, 0) != 0) {
        karkain_os_err("memory size unavailable");
        return make_int(-1);
    }
    return make_int((long long)(bytes / 1048576ULL));
#else
    return make_int(karkain_os_meminfo_kb("MemTotal:"));
#endif
}

Value karkain_os_available_mem_mb(void) {
    _karkain_os_error[0] = '\0';
#ifdef _WIN32
    MEMORYSTATUSEX st;
    st.dwLength = sizeof(st);
    if (!GlobalMemoryStatusEx(&st)) { karkain_os_err("memory status failed"); return make_int(-1); }
    return make_int((long long)(st.ullAvailPhys / 1048576ULL));
#elif defined(__APPLE__)
    karkain_os_err("available memory unavailable on macOS");
    return make_int(-1);
#else
    return make_int(karkain_os_meminfo_kb("MemAvailable:"));
#endif
}

Value karkain_os_uptime_ms(void) {
    _karkain_os_error[0] = '\0';
#ifdef _WIN32
    return make_int((long long)(GetTickCount64() / 1000ULL));
#else
    struct timespec ts;
    if (clock_gettime(CLOCK_MONOTONIC, &ts) != 0) {
        karkain_os_err("clock failed");
        return make_int(-1);
    }
    return make_int((long long)ts.tv_sec * 1000LL + (long long)ts.tv_nsec / 1000000LL);
#endif
}

static int karkain_os_cmp(const void* a, const void* b) {
    return strcmp(*(char* const*)a, *(char* const*)b);
}

static void karkain_os_add(char** ptrs, int* count, char* entry) {
    if (*count >= KARKAIN_OS_MAX_PROCESSES) return;
    size_t n = strlen(entry);
    char* copy = (char*)malloc(n + 1);
    if (!copy) { karkain_os_err("process list out of memory"); return; }
    memcpy(copy, entry, n + 1);
    ptrs[*count] = copy;
    (*count)++;
}

Value karkain_os_process_list(void) {
    _karkain_os_error[0] = '\0';
    char entries[KARKAIN_OS_MAX_PROCESSES][KARKAIN_OS_NAME_LEN];
    char* ptrs[KARKAIN_OS_MAX_PROCESSES];
    char line[KARKAIN_OS_NAME_LEN];
    int count = 0;
    int i = 0;

#ifdef _WIN32
    HANDLE snap = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
    if (snap == INVALID_HANDLE_VALUE) {
        karkain_os_err("process snapshot failed");
        return make_array();
    }
    PROCESSENTRY32W pe;
    pe.dwSize = sizeof(pe);
    BOOL ok = Process32FirstW(snap, &pe);
    while (ok && count < KARKAIN_OS_MAX_PROCESSES - 1) {
        int n = WideCharToMultiByte(CP_UTF8, 0, pe.szExeFile, -1, line, sizeof(line), NULL, NULL);
        if (n <= 0) { snprintf(line, sizeof(line), "unknown"); }
        snprintf(entries[count], KARKAIN_OS_NAME_LEN, "%lu:%s",
                 (unsigned long)pe.th32ProcessID, line);
        ptrs[count] = entries[count];
        count++;
        ok = Process32NextW(snap, &pe);
    }
    CloseHandle(snap);
#elif defined(__APPLE__)
    // No /proc on macOS. Report only the current process; the v1.2.19 gate
    // runs on Windows and Linux, and this boundary is documented.
    snprintf(entries[0], KARKAIN_OS_NAME_LEN, "%ld:karkain", (long)getpid());
    ptrs[0] = entries[0];
    count = 1;
#else
    DIR* d = opendir("/proc");
    if (!d) {
        karkain_os_err("proc unavailable");
        return make_array();
    }
    struct dirent* e;
    while ((e = readdir(d)) && count < KARKAIN_OS_MAX_PROCESSES - 1) {
        const char* p = e->d_name;
        if (!*p) continue;
        int numeric = 1;
        for (const char* q = p; *q; q++) { if (!isdigit((unsigned char)*q)) { numeric = 0; break; } }
        if (!numeric) continue;
        char path[128];
        snprintf(path, sizeof(path), "/proc/%s/comm", p);
        FILE* f = fopen(path, "r");
        line[0] = '\0';
        if (f) {
            if (fgets(line, sizeof(line), f)) {
                size_t n = strlen(line);
                while (n && (line[n - 1] == '\n' || line[n - 1] == '\r')) line[--n] = '\0';
            }
            fclose(f);
        }
        if (!line[0]) snprintf(line, sizeof(line), "unknown");
        snprintf(entries[count], KARKAIN_OS_NAME_LEN, "%s:%s", p, line);
        ptrs[count] = entries[count];
        count++;
    }
    closedir(d);
#endif

    // Always include the current PID; on /proc hosts with an exotic comm this
    // also makes the "list contains self" assertion deterministic.
    long self = (long)karkain_os_pid().intVal;
    int have_self = 0;
    for (i = 0; i < count; i++) {
        char prefix[32];
        snprintf(prefix, sizeof(prefix), "%ld:", self);
        if (strncmp(ptrs[i], prefix, strlen(prefix)) == 0) { have_self = 1; break; }
    }
    if (!have_self && count < KARKAIN_OS_MAX_PROCESSES) {
        snprintf(entries[count], KARKAIN_OS_NAME_LEN, "%ld:karkain", self);
        ptrs[count] = entries[count];
        count++;
    }

    qsort(ptrs, (size_t)count, sizeof(char*), karkain_os_cmp);
    Value out = make_array();
    for (i = 0; i < count; i++) array_push(&out, make_string(ptrs[i]));
    return out;
}
