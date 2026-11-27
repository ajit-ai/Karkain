#include "karkain_osctl.h"

#include <ctype.h>
#include <string.h>

static char _karkain_osctl_error[512] = "";

static void karkain_osctl_err(const char *msg) {
    snprintf(_karkain_osctl_error, sizeof(_karkain_osctl_error), "%s", msg ? msg : "osctl failure");
}

/* ---- Environment -------------------------------------------------------- */

Value karkain_env_get(Value name) {
    _karkain_osctl_error[0] = '\0';
    const char *key = name.strVal ? name.strVal : "";
#ifdef _WIN32
    // Convert to wide; use GetEnvironmentVariableW
    int wlen = MultiByteToWideChar(CP_UTF8, 0, key, -1, NULL, 0);
    if (wlen <= 0) { karkain_osctl_err("env_get: bad key encoding"); return make_string(""); }
    wchar_t *wkey = (wchar_t *)malloc((size_t)wlen * sizeof(wchar_t));
    if (!wkey) { karkain_osctl_err("env_get: out of memory"); return make_string(""); }
    MultiByteToWideChar(CP_UTF8, 0, key, -1, wkey, wlen);
    DWORD sz = GetEnvironmentVariableW(wkey, NULL, 0);
    if (sz == 0) { free(wkey); return make_string(""); }
    wchar_t *wval = (wchar_t *)malloc((size_t)sz * sizeof(wchar_t));
    if (!wval) { free(wkey); karkain_osctl_err("env_get: out of memory"); return make_string(""); }
    GetEnvironmentVariableW(wkey, wval, sz);
    free(wkey);
    char buf[4096];
    int n = WideCharToMultiByte(CP_UTF8, 0, wval, -1, buf, (int)sizeof(buf), NULL, NULL);
    free(wval);
    if (n <= 0) { karkain_osctl_err("env_get: encoding error"); return make_string(""); }
    return make_string(buf);
#else
    const char *v = getenv(key);
    return make_string(v ? v : "");
#endif
}

Value karkain_env_set(Value name, Value value) {
    _karkain_osctl_error[0] = '\0';
    const char *key = name.strVal ? name.strVal : "";
    const char *val = value.strVal ? value.strVal : "";
#ifdef _WIN32
    int wklen = MultiByteToWideChar(CP_UTF8, 0, key, -1, NULL, 0);
    int wvlen = MultiByteToWideChar(CP_UTF8, 0, val, -1, NULL, 0);
    if (wklen <= 0 || wvlen <= 0) { karkain_osctl_err("env_set: encoding error"); return make_int(-1); }
    wchar_t *wk = (wchar_t *)malloc((size_t)wklen * sizeof(wchar_t));
    wchar_t *wv = (wchar_t *)malloc((size_t)wvlen * sizeof(wchar_t));
    if (!wk || !wv) { free(wk); free(wv); karkain_osctl_err("env_set: out of memory"); return make_int(-1); }
    MultiByteToWideChar(CP_UTF8, 0, key, -1, wk, wklen);
    MultiByteToWideChar(CP_UTF8, 0, val, -1, wv, wvlen);
    BOOL ok = SetEnvironmentVariableW(wk, wv);
    free(wk); free(wv);
    if (!ok) { karkain_osctl_err("env_set failed"); return make_int(-1); }
    return make_int(0);
#else
    if (setenv(key, val, 1) != 0) { karkain_osctl_err("env_set failed"); return make_int(-1); }
    return make_int(0);
#endif
}

Value karkain_env_unset(Value name) {
    _karkain_osctl_error[0] = '\0';
    const char *key = name.strVal ? name.strVal : "";
#ifdef _WIN32
    int wklen = MultiByteToWideChar(CP_UTF8, 0, key, -1, NULL, 0);
    if (wklen <= 0) { karkain_osctl_err("env_unset: encoding error"); return make_int(-1); }
    wchar_t *wk = (wchar_t *)malloc((size_t)wklen * sizeof(wchar_t));
    if (!wk) { karkain_osctl_err("env_unset: out of memory"); return make_int(-1); }
    MultiByteToWideChar(CP_UTF8, 0, key, -1, wk, wklen);
    BOOL ok = SetEnvironmentVariableW(wk, NULL);
    free(wk);
    if (!ok && GetLastError() != ERROR_ENVVAR_NOT_FOUND) {
        karkain_osctl_err("env_unset failed"); return make_int(-1);
    }
    return make_int(0);
#else
    unsetenv(key);
    return make_int(0);
#endif
}

Value karkain_env_args(void) {
    // Returns the program's arguments (excluding argv[0]) as an array of strings.
    _karkain_osctl_error[0] = '\0';
    Value out = make_array();
#ifdef _WIN32
    int argc = 0;
    LPWSTR cmdline = GetCommandLineW();
    LPWSTR *argv = CommandLineToArgvW(cmdline, &argc);
    if (!argv) { karkain_osctl_err("env_args: CommandLineToArgvW failed"); return out; }
    for (int i = 1; i < argc; i++) {
        char buf[4096];
        int n = WideCharToMultiByte(CP_UTF8, 0, argv[i], -1, buf, (int)sizeof(buf), NULL, NULL);
        if (n > 0) array_push(&out, make_string(buf));
    }
    LocalFree(argv);
#else
    // On POSIX, argc/argv are not directly accessible from a library function.
    // We read /proc/self/cmdline when available; otherwise return empty.
    FILE *f = fopen("/proc/self/cmdline", "r");
    if (!f) { return out; }
    char buf[65536];
    size_t total = fread(buf, 1, sizeof(buf) - 1, f);
    fclose(f);
    buf[total] = '\0';
    // Arguments are NUL-separated; skip argv[0]
    size_t i = 0;
    while (i < total && buf[i]) i++; // skip argv[0]
    i++; // skip NUL
    while (i < total) {
        if (buf[i] == '\0') { i++; continue; }
        char *start = buf + i;
        while (i < total && buf[i]) i++;
        array_push(&out, make_string(start));
        i++;
    }
#endif
    return out;
}

/* ---- Filesystem metadata ------------------------------------------------ */

Value karkain_fs_exists(Value path) {
    _karkain_osctl_error[0] = '\0';
    const char *p = path.strVal ? path.strVal : "";
#ifdef _WIN32
    int wlen = MultiByteToWideChar(CP_UTF8, 0, p, -1, NULL, 0);
    if (wlen <= 0) return make_int(-1);
    wchar_t *wp = (wchar_t *)malloc((size_t)wlen * sizeof(wchar_t));
    if (!wp) { karkain_osctl_err("fs_exists: out of memory"); return make_int(-1); }
    MultiByteToWideChar(CP_UTF8, 0, p, -1, wp, wlen);
    DWORD attr = GetFileAttributesW(wp);
    free(wp);
    return make_int(attr == INVALID_FILE_ATTRIBUTES ? 0 : 1);
#else
    struct stat st;
    return make_int(stat(p, &st) == 0 ? 1 : 0);
#endif
}

Value karkain_fs_is_dir(Value path) {
    _karkain_osctl_error[0] = '\0';
    const char *p = path.strVal ? path.strVal : "";
#ifdef _WIN32
    int wlen = MultiByteToWideChar(CP_UTF8, 0, p, -1, NULL, 0);
    if (wlen <= 0) return make_int(0);
    wchar_t *wp = (wchar_t *)malloc((size_t)wlen * sizeof(wchar_t));
    if (!wp) { karkain_osctl_err("fs_is_dir: out of memory"); return make_int(0); }
    MultiByteToWideChar(CP_UTF8, 0, p, -1, wp, wlen);
    DWORD attr = GetFileAttributesW(wp);
    free(wp);
    if (attr == INVALID_FILE_ATTRIBUTES) return make_int(0);
    return make_int((attr & FILE_ATTRIBUTE_DIRECTORY) ? 1 : 0);
#else
    struct stat st;
    if (stat(p, &st) != 0) return make_int(0);
    return make_int(S_ISDIR(st.st_mode) ? 1 : 0);
#endif
}

Value karkain_fs_stat_size(Value path) {
    _karkain_osctl_error[0] = '\0';
    const char *p = path.strVal ? path.strVal : "";
#ifdef _WIN32
    int wlen = MultiByteToWideChar(CP_UTF8, 0, p, -1, NULL, 0);
    if (wlen <= 0) return make_int(-1);
    wchar_t *wp = (wchar_t *)malloc((size_t)wlen * sizeof(wchar_t));
    if (!wp) { karkain_osctl_err("fs_stat_size: out of memory"); return make_int(-1); }
    MultiByteToWideChar(CP_UTF8, 0, p, -1, wp, wlen);
    WIN32_FILE_ATTRIBUTE_DATA fad;
    BOOL ok = GetFileAttributesExW(wp, GetFileExInfoStandard, &fad);
    free(wp);
    if (!ok) { karkain_osctl_err("fs_stat_size failed"); return make_int(-1); }
    ULONGLONG sz = ((ULONGLONG)fad.nFileSizeHigh << 32) | fad.nFileSizeLow;
    return make_int((long long)sz);
#else
    struct stat st;
    if (stat(p, &st) != 0) { karkain_osctl_err("fs_stat_size failed"); return make_int(-1); }
    return make_int((long long)st.st_size);
#endif
}

Value karkain_fs_stat_mtime(Value path) {
    _karkain_osctl_error[0] = '\0';
    const char *p = path.strVal ? path.strVal : "";
#ifdef _WIN32
    int wlen = MultiByteToWideChar(CP_UTF8, 0, p, -1, NULL, 0);
    if (wlen <= 0) return make_int(-1);
    wchar_t *wp = (wchar_t *)malloc((size_t)wlen * sizeof(wchar_t));
    if (!wp) { karkain_osctl_err("fs_stat_mtime: out of memory"); return make_int(-1); }
    MultiByteToWideChar(CP_UTF8, 0, p, -1, wp, wlen);
    WIN32_FILE_ATTRIBUTE_DATA fad;
    BOOL ok = GetFileAttributesExW(wp, GetFileExInfoStandard, &fad);
    free(wp);
    if (!ok) { karkain_osctl_err("fs_stat_mtime failed"); return make_int(-1); }
    // FILETIME: 100ns intervals since 1601-01-01; convert to Unix epoch
    ULONGLONG ft = ((ULONGLONG)fad.ftLastWriteTime.dwHighDateTime << 32) | fad.ftLastWriteTime.dwLowDateTime;
    ULONGLONG unix_epoch_offset = 116444736000000000ULL; // 100ns intervals from 1601 to 1970
    if (ft < unix_epoch_offset) return make_int(0);
    return make_int((long long)((ft - unix_epoch_offset) / 10000000ULL));
#else
    struct stat st;
    if (stat(p, &st) != 0) { karkain_osctl_err("fs_stat_mtime failed"); return make_int(-1); }
    return make_int((long long)st.st_mtime);
#endif
}

Value karkain_fs_mkdir(Value path) {
    _karkain_osctl_error[0] = '\0';
    const char *p = path.strVal ? path.strVal : "";
#ifdef _WIN32
    int wlen = MultiByteToWideChar(CP_UTF8, 0, p, -1, NULL, 0);
    if (wlen <= 0) return make_int(-1);
    wchar_t *wp = (wchar_t *)malloc((size_t)wlen * sizeof(wchar_t));
    if (!wp) { karkain_osctl_err("fs_mkdir: out of memory"); return make_int(-1); }
    MultiByteToWideChar(CP_UTF8, 0, p, -1, wp, wlen);
    BOOL ok = CreateDirectoryW(wp, NULL);
    free(wp);
    if (!ok && GetLastError() != ERROR_ALREADY_EXISTS) {
        karkain_osctl_err("fs_mkdir failed"); return make_int(-1);
    }
    return make_int(0);
#else
    if (mkdir(p, 0755) != 0 && errno != EEXIST) {
        karkain_osctl_err("fs_mkdir failed"); return make_int(-1);
    }
    return make_int(0);
#endif
}

Value karkain_fs_remove(Value path) {
    _karkain_osctl_error[0] = '\0';
    const char *p = path.strVal ? path.strVal : "";
#ifdef _WIN32
    int wlen = MultiByteToWideChar(CP_UTF8, 0, p, -1, NULL, 0);
    if (wlen <= 0) return make_int(-1);
    wchar_t *wp = (wchar_t *)malloc((size_t)wlen * sizeof(wchar_t));
    if (!wp) { karkain_osctl_err("fs_remove: out of memory"); return make_int(-1); }
    MultiByteToWideChar(CP_UTF8, 0, p, -1, wp, wlen);
    // Try file first, then directory
    BOOL ok = DeleteFileW(wp);
    if (!ok) ok = RemoveDirectoryW(wp);
    free(wp);
    if (!ok) { karkain_osctl_err("fs_remove failed"); return make_int(-1); }
    return make_int(0);
#else
    if (unlink(p) != 0) {
        if (rmdir(p) != 0) { karkain_osctl_err("fs_remove failed"); return make_int(-1); }
    }
    return make_int(0);
#endif
}

/* ---- Process control ---------------------------------------------------- */

Value karkain_proc_spawn(Value cmd) {
    // Spawns cmd as a new process (shell-free, argv[0] is cmd).
    // Returns the child PID on success, -1 on failure.
    _karkain_osctl_error[0] = '\0';
    const char *c = cmd.strVal ? cmd.strVal : "";
#ifdef _WIN32
    int wlen = MultiByteToWideChar(CP_UTF8, 0, c, -1, NULL, 0);
    if (wlen <= 0) { karkain_osctl_err("proc_spawn: encoding error"); return make_int(-1); }
    wchar_t *wcmd = (wchar_t *)malloc((size_t)wlen * sizeof(wchar_t));
    if (!wcmd) { karkain_osctl_err("proc_spawn: out of memory"); return make_int(-1); }
    MultiByteToWideChar(CP_UTF8, 0, c, -1, wcmd, wlen);
    STARTUPINFOW si;
    PROCESS_INFORMATION pi;
    memset(&si, 0, sizeof(si));
    si.cb = sizeof(si);
    memset(&pi, 0, sizeof(pi));
    BOOL ok = CreateProcessW(NULL, wcmd, NULL, NULL, FALSE, 0, NULL, NULL, &si, &pi);
    free(wcmd);
    if (!ok) { karkain_osctl_err("proc_spawn: CreateProcess failed"); return make_int(-1); }
    DWORD pid = pi.dwProcessId;
    CloseHandle(pi.hThread);
    CloseHandle(pi.hProcess);
    return make_int((long long)pid);
#else
    pid_t pid = fork();
    if (pid < 0) { karkain_osctl_err("proc_spawn: fork failed"); return make_int(-1); }
    if (pid == 0) {
        // child: exec via sh to handle paths with spaces / shell features
        execlp("sh", "sh", "-c", c, (char *)NULL);
        _exit(127);
    }
    return make_int((long long)pid);
#endif
}

Value karkain_proc_kill(Value pid_val) {
    _karkain_osctl_error[0] = '\0';
    long long pid = pid_val.intVal;
    if (pid <= 0) { karkain_osctl_err("proc_kill: invalid pid"); return make_int(-1); }
#ifdef _WIN32
    HANDLE h = OpenProcess(PROCESS_TERMINATE, FALSE, (DWORD)pid);
    if (!h) { karkain_osctl_err("proc_kill: OpenProcess failed"); return make_int(-1); }
    BOOL ok = TerminateProcess(h, 1);
    CloseHandle(h);
    if (!ok) { karkain_osctl_err("proc_kill: TerminateProcess failed"); return make_int(-1); }
    return make_int(0);
#else
    if (kill((pid_t)pid, SIGTERM) != 0) {
        karkain_osctl_err("proc_kill: kill failed"); return make_int(-1);
    }
    return make_int(0);
#endif
}

Value karkain_proc_wait(Value pid_val) {
    _karkain_osctl_error[0] = '\0';
    long long pid = pid_val.intVal;
    if (pid <= 0) { karkain_osctl_err("proc_wait: invalid pid"); return make_int(-1); }
#ifdef _WIN32
    HANDLE h = OpenProcess(SYNCHRONIZE | PROCESS_QUERY_INFORMATION, FALSE, (DWORD)pid);
    if (!h) { karkain_osctl_err("proc_wait: OpenProcess failed"); return make_int(-1); }
    WaitForSingleObject(h, INFINITE);
    DWORD code = 0;
    GetExitCodeProcess(h, &code);
    CloseHandle(h);
    return make_int((long long)code);
#else
    int status = 0;
    pid_t r = waitpid((pid_t)pid, &status, 0);
    if (r < 0) { karkain_osctl_err("proc_wait: waitpid failed"); return make_int(-1); }
    if (WIFEXITED(status)) return make_int((long long)WEXITSTATUS(status));
    if (WIFSIGNALED(status)) return make_int((long long)(128 + WTERMSIG(status)));
    return make_int(-1);
#endif
}
