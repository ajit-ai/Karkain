#define KARKAIN_PLATFORM_IMPLEMENTATION
#include "karkain_platform.h"

/* Windows: kernel32 is the OS boundary. The Windows headers (windows.h) pull
 * CRT-adjacent declarations, so the few functions we need are declared by
 * hand. All of them live in kernel32/ntdll, which -nostdlib links directly. */

#if defined(_WIN32)

#define KARKAIN_WIN_STD_OUTPUT_HANDLE ((long)-11)
#define KARKAIN_WIN_MEM_COMMIT 0x1000u
#define KARKAIN_WIN_MEM_RESERVE 0x2000u
#define KARKAIN_WIN_MEM_RELEASE 0x8000u
#define KARKAIN_WIN_PAGE_READWRITE 0x04u

void* __stdcall VirtualAlloc(void* lpAddress, unsigned long dwSize,
                             unsigned long flAllocationType, unsigned long flProtect);
int __stdcall VirtualFree(void* lpAddress, unsigned long dwSize,
                          unsigned long dwFreeType);
void* __stdcall GetStdHandle(long nStdHandle);
int __stdcall WriteFile(void* hFile, const void* lpBuffer, unsigned long nNumberOfBytesToWrite,
                        unsigned long* lpNumberOfBytesWritten, void* lpOverlapped);
int __stdcall ReadFile(void* hFile, void* lpBuffer, unsigned long nNumberOfBytesToRead,
                       unsigned long* lpNumberOfBytesRead, void* lpOverlapped);
void* __stdcall CreateFileA(const char* lpFileName, unsigned long dwDesiredAccess,
                            unsigned long dwShareMode, void* lpSecurityAttributes,
                            unsigned long dwCreationDisposition, unsigned long dwFlagsAndAttributes,
                            void* hTemplateFile);
int __stdcall CloseHandle(void* hObject);
void __stdcall ExitProcess(unsigned int uExitCode);

#define KARKAIN_WIN_GENERIC_READ 0x80000000ul
#define KARKAIN_WIN_OPEN_EXISTING 3
#define KARKAIN_WIN_FILE_ATTRIBUTE_NORMAL 0x80

void* karkain_pages_alloc(unsigned long size) {
    unsigned long pages = (size + KARKAIN_PAGESIZE - 1) / KARKAIN_PAGESIZE;
    void* p = VirtualAlloc(0, pages * KARKAIN_PAGESIZE,
                           KARKAIN_WIN_MEM_RESERVE | KARKAIN_WIN_MEM_COMMIT,
                           KARKAIN_WIN_PAGE_READWRITE);
    return p;
}

void karkain_pages_free(void* p, unsigned long size) {
    (void)size;
    VirtualFree(p, 0, KARKAIN_WIN_MEM_RELEASE);
}

long karkain_write_out(const void* buf, unsigned long n) {
    void* h = GetStdHandle(KARKAIN_WIN_STD_OUTPUT_HANDLE);
    if (!h || h == (void*)-1) return -1;
    unsigned long written = 0;
    if (!WriteFile(h, buf, n, &written, 0)) return -1;
    return (long)written;
}

long karkain_open_read(const char* path) {
    void* h = CreateFileA(path, KARKAIN_WIN_GENERIC_READ, 0, 0,
                          KARKAIN_WIN_OPEN_EXISTING, KARKAIN_WIN_FILE_ATTRIBUTE_NORMAL, 0);
    if (h == (void*)-1) return -1;
    return (long)(intptr_t)h;
}

long karkain_read(long fd, void* buf, unsigned long n) {
    unsigned long got = 0;
    if (!ReadFile((void*)(intptr_t)fd, buf, n, &got, 0)) {
        if (got != 0) return (long)got;
        return -1;
    }
    return (long)got;
}

long karkain_write(long fd, const void* buf, unsigned long n) {
    unsigned long written = 0;
    if (!WriteFile((void*)(intptr_t)fd, buf, n, &written, 0)) return -1;
    return (long)written;
}

void karkain_close(long fd) {
    CloseHandle((void*)(intptr_t)fd);
}

void karkain_exit(int code) {
    ExitProcess((unsigned int)(unsigned int)code);
}

/* ---------- POSIX: raw syscall shim, no libc ---------- */

#else

#if defined(__x86_64__)
#define KARKAIN_SYS_WRITE 1
#define KARKAIN_SYS_READ 0
#define KARKAIN_SYS_OPENAT 257
#define KARKAIN_SYS_CLOSE 3
#define KARKAIN_SYS_MMAP 9
#define KARKAIN_SYS_MUNMAP 11
#define KARKAIN_SYS_EXIT_GROUP 231
#define KARKAIN_AT_FDCWD (-100)
#define KARKAIN_O_RDONLY 0

static inline long karkain_syscall1(long n, long a0) {
    register long rax __asm__("rax") = n;
    register long rdi __asm__("rdi") = a0;
    __asm__ volatile("syscall" : "+r"(rax) : "r"(rdi) : "rcx", "r11", "memory");
    return rax;
}

static inline long karkain_syscall3(long n, long a0, long a1, long a2) {
    register long rax __asm__("rax") = n;
    register long rdi __asm__("rdi") = a0;
    register long rsi __asm__("rsi") = a1;
    register long rdx __asm__("rdx") = a2;
    __asm__ volatile("syscall" : "+r"(rax) : "r"(rdi), "r"(rsi), "r"(rdx) : "rcx", "r11", "memory");
    return rax;
}

static inline long karkain_syscall6(long n, long a0, long a1, long a2, long a3, long a4, long a5) {
    register long rax __asm__("rax") = n;
    register long rdi __asm__("rdi") = a0;
    register long rsi __asm__("rsi") = a1;
    register long rdx __asm__("rdx") = a2;
    register long r10 __asm__("r10") = a3;
    register long r8  __asm__("r8")  = a4;
    register long r9  __asm__("r9")  = a5;
    __asm__ volatile("syscall"
                     : "+r"(rax)
                     : "r"(rdi), "r"(rsi), "r"(rdx), "r"(r10), "r"(r8), "r"(r9)
                     : "rcx", "r11", "memory");
    return rax;
}

#elif defined(__aarch64__)
#define KARKAIN_SYS_WRITE 64
#define KARKAIN_SYS_READ 63
#define KARKAIN_SYS_OPENAT 56
#define KARKAIN_SYS_CLOSE 57
#define KARKAIN_SYS_MMAP 222
#define KARKAIN_SYS_MUNMAP 215
#define KARKAIN_SYS_EXIT_GROUP 94
#define KARKAIN_AT_FDCWD (-100)
#define KARKAIN_O_RDONLY 0

static inline long karkain_syscall1(long n, long a0) {
    register long x0 __asm__("x0") = a0;
    register long x8 __asm__("x8") = n;
    __asm__ volatile("svc #0" : "+r"(x0) : "r"(x8) : "memory");
    return x0;
}

static inline long karkain_syscall3(long n, long a0, long a1, long a2) {
    register long x0 __asm__("x0") = a0;
    register long x1 __asm__("x1") = a1;
    register long x2 __asm__("x2") = a2;
    register long x8 __asm__("x8") = n;
    __asm__ volatile("svc #0" : "+r"(x0) : "r"(x1), "r"(x2), "r"(x8) : "memory");
    return x0;
}

static inline long karkain_syscall6(long n, long a0, long a1, long a2, long a3, long a4, long a5) {
    register long x0 __asm__("x0") = a0;
    register long x1 __asm__("x1") = a1;
    register long x2 __asm__("x2") = a2;
    register long x3 __asm__("x3") = a3;
    register long x4 __asm__("x4") = a4;
    register long x5 __asm__("x5") = a5;
    register long x8 __asm__("x8") = n;
    __asm__ volatile("svc #0"
                     : "+r"(x0)
                     : "r"(x1), "r"(x2), "r"(x3), "r"(x4), "r"(x5), "r"(x8)
                     : "memory");
    return x0;
}

#else
#error "karkain freestanding: no syscall shim for this architecture"
#endif

#define KARKAIN_PROT_READ 1
#define KARKAIN_PROT_WRITE 2
#define KARKAIN_MAP_PRIVATE 2
#define KARKAIN_MAP_ANONYMOUS 32

void* karkain_pages_alloc(unsigned long size) {
    unsigned long pages = (size + KARKAIN_PAGESIZE - 1) / KARKAIN_PAGESIZE;
    long p = karkain_syscall6(KARKAIN_SYS_MMAP, 0,
                              (long)(pages * KARKAIN_PAGESIZE),
                              KARKAIN_PROT_READ | KARKAIN_PROT_WRITE,
                              KARKAIN_MAP_PRIVATE | KARKAIN_MAP_ANONYMOUS,
                              -1, 0);
    if (p < 0 && p > -4096) return NULL;
    return (void*)p;
}

void karkain_pages_free(void* p, unsigned long size) {
    (void)karkain_syscall3(KARKAIN_SYS_MUNMAP, (long)(uintptr_t)p, (long)size, 0);
}

long karkain_write_out(const void* buf, unsigned long n) {
    return karkain_syscall3(KARKAIN_SYS_WRITE, 1, (long)(uintptr_t)buf, (long)n);
}

long karkain_open_read(const char* path) {
    return karkain_syscall3(KARKAIN_SYS_OPENAT, KARKAIN_AT_FDCWD,
                            (long)(uintptr_t)path, KARKAIN_O_RDONLY);
}

long karkain_read(long fd, void* buf, unsigned long n) {
    return karkain_syscall3(KARKAIN_SYS_READ, fd, (long)(uintptr_t)buf, (long)n);
}

long karkain_write(long fd, const void* buf, unsigned long n) {
    return karkain_syscall3(KARKAIN_SYS_WRITE, fd, (long)(uintptr_t)buf, (long)n);
}

void karkain_close(long fd) {
    (void)karkain_syscall1(KARKAIN_SYS_CLOSE, fd);
}

void karkain_exit(int code) {
    (void)karkain_syscall1(KARKAIN_SYS_EXIT_GROUP, code);
    for (;;) { }
}

#endif