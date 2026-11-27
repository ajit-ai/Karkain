#ifndef KARKAIN_OSCTL_H
#define KARKAIN_OSCTL_H

// v1.2.34: Portable OS/process control builtins. Covers environment variables,
// filesystem metadata, and process spawn/kill/wait on the C23 path.
// No new link library: Win32 calls live in kernel32 (already linked); POSIX
// uses libc. macOS: all ops work via POSIX; no /proc, so process list is
// enumeration-only via os_process_list (v1.2.19).
#ifdef _WIN32
#include <windows.h>
#include <processthreadsapi.h>
#include <fileapi.h>
#include <shellapi.h>
#else
#include <dirent.h>
#include <errno.h>
#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <time.h>
#include <unistd.h>
#endif

// Environment
Value karkain_env_get(Value name);
Value karkain_env_set(Value name, Value value);
Value karkain_env_unset(Value name);
Value karkain_env_args(void);

// Filesystem metadata
Value karkain_fs_exists(Value path);
Value karkain_fs_is_dir(Value path);
Value karkain_fs_stat_size(Value path);
Value karkain_fs_stat_mtime(Value path);
Value karkain_fs_mkdir(Value path);
Value karkain_fs_remove(Value path);

// Process control
Value karkain_proc_spawn(Value cmd);
Value karkain_proc_kill(Value pid);
Value karkain_proc_wait(Value pid);

#endif /* KARKAIN_OSCTL_H */
