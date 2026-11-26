#ifndef KARKAIN_OSMETRICS_H
#define KARKAIN_OSMETRICS_H

// v1.2.19: OS metric and process enumeration builtins. The generated preamble
// defines Value/make_int/make_array/array_push before this block is appended.
// No new link library: Win32 calls live in kernel32 (already linked), and the
// POSIX calls use libc plus /proc (already available on the Linux CI leg).
#ifdef _WIN32
#include <windows.h>
#include <tlhelp32.h>
#include <sysinfoapi.h>
#include <processthreadsapi.h>
#else
#include <dirent.h>
#include <errno.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <unistd.h>
#ifdef __APPLE__
#include <sys/sysctl.h>
#endif
#endif

Value karkain_os_pid(void);
Value karkain_os_cpu_count(void);
Value karkain_os_total_mem_mb(void);
Value karkain_os_available_mem_mb(void);
Value karkain_os_uptime_ms(void);
Value karkain_os_process_list(void);
Value karkain_os_last_error(void);

#endif
