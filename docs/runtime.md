# Karkain Runtime Architecture

## Overview

Phase 89 establishes the Karkain-owned runtime boundary. The runtime provides the language-level primitives that generated programs depend on, while platform-specific implementations remain separate.

## Architecture Layers

```
Karkain language
    |
Karkain standard library (stdlib/)
    |  — core, string, collections, math, io, system
    |  — Written in .kark + libc FFI
    |
Karkain runtime API (runtime/)
    |  — Value type system
    |  — Container operations
    |  — I/O primitives
    |  — Platform abstraction
    |
Platform abstraction (C runtime)
    |  — OS-specific implementations
    |  — POSIX / Windows / WASI
    |
Operating system / native platform
    — Kernel + C library
```

## Function Ownership Model

| Category | Prefix | Implementation | Example |
|----------|--------|----------------|---------|
| Language primitive | (none) | compiler intrinsic | `len()`, `print()` |
| Compiler builtin | (none) | codegen emits C | `str()`, `int()` |
| Standard library | `module.name` | .kark + libc FFI | `math.sqrt()` |
| Runtime API | `karkain_` | C runtime | `karkain_readFile()` |
| Platform API | (varies) | OS-specific C | `open()`, `read()` |

## Runtime Components

### Value Type System (`runtime/types.kark`)

The runtime uses a tagged union `Value` type with 10 variants:
- `TYPE_INT` (0) — 64-bit integer
- `TYPE_FLOAT64` (1) — 64-bit float
- `TYPE_STRING` (2) — heap-allocated string
- `TYPE_ARRAY` (3) — dynamic array
- `TYPE_MAP` (4) — key-value map
- `TYPE_BOOL` (5) — boolean
- `TYPE_BIGINT` (6) — arbitrary-precision integer (GMP)
- `TYPE_BIGFLOAT` (7) — arbitrary-precision float (GMP)
- `TYPE_OPTION` (8) — Option type (None/Some)
- `TYPE_RESULT` (9) — Result type (Ok/Err)

Value fits in a 64-byte cache line for efficient stack passing.

### I/O Operations (`runtime/io.kark`)

File I/O primitives:
- `openFile(path)` — open for reading
- `readLine(handle)` — read one line
- `readLineEOF(handle)` — read line with EOF detection
- `closeFile(handle)` — close file
- `createFile(path)` — create/truncate for writing
- `writeToFile(handle, content)` — write to file
- `karkain_readFile(path)` — read entire file
- `karkain_writeFile(path, content)` — write entire file
- `removeFile(path)` — delete file
- `karkain_listFiles(dir)` — list directory

### Platform Abstraction (`runtime/platform.kark`)

Platform-facing operations:
- `getArgs()` — command-line arguments
- `karkain_system(cmd)` — shell command execution
- `karkain_alloc(size)` / `karkain_free(ptr)` — raw memory
- `platform_name()` — OS detection
- `karkain_time()` — current time
- `karkain_assert()` / `karkain_assert_eq()` / `karkain_assert_ne()` — assertions
- `karkain_add_checked()` / `karkain_sub_checked()` / `karkain_mul_checked()` — checked arithmetic

### Initialization (`runtime/init.kark`)

Program startup/shutdown contract:
```
int main(int argc, char** argv) {
    _karkain_gargc = argc;
    _karkain_gargv = argv;
    karkain_init();
    Value result = karkain_user_main();
    karkain_fini();
    return 0;
}
```

### Boundary Documentation (`runtime/boundary.kark`)

Documents the complete ownership model, namespace rules, and what is NOT self-hosted.

## Namespace Rules

1. **User functions:** `karkain_user_<name>` (C namespace)
2. **Runtime functions:** `karkain_<name>` (C namespace)
3. **Built-in functions:** (no prefix, resolved by codegen)
4. **Standard library:** `<module>.<name>` (Karkain namespace)

## What Is NOT Self-Hosted

- Operating system calls (POSIX/Win32/WASI)
- C standard library (libc)
- GMP arbitrary precision (libgmp)
- GCC/Clang compilation
- Linker and loader
- Process management

These remain platform implementations. Karkain owns the language and runtime interfaces, not the OS facilities.

## Current Status

### IMPLEMENTED
- Runtime type system (Value with 10 variants)
- Container operations (array, map)
- File I/O primitives
- Platform abstraction layer
- Runtime initialization contract
- Boundary documentation

### PARTIAL
- Self-hosted compiler generates simplified runtime (6 types vs 10)
- No separate runtime compilation step (embedded in codegen)

### PLANNED
- Separate runtime library compilation
- Dynamic linking support
- Platform-specific runtime backends
- Runtime optimization passes
