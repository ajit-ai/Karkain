# Karkain Standard Library

Phase 87: the foundation for Karkain's standard library — importable modules
providing core utilities, string operations, collections, math, I/O, and
system interaction.

---

## 1. Overview

The standard library is organized under `stdlib/` at the repository root:

```
stdlib/
├── core/           Fundamental operations (clamp, min, max, abs, range, etc.)
├── string/         String manipulation (starts_with, ends_with, replace, etc.)
├── collections/    Array/map utilities (contains, index_of, reverse, etc.)
├── math/           Math operations (trig, exp, vectors, matrices, stats)
├── io/             I/O operations (file read/write, formatted printing)
├── system/         Host environment (args, exit, file ops, platform info)
├── async/          Async/actor concurrency
└── gpu/            GPU compute
```

### Importing

Standard library modules are imported using the `std.` prefix:

```kark
import std.core
import std.string
import std.collections
import std.math
import std.io
import std.system
import std.gpu
import std.async
```

The module resolver maps `std.<name>` to `stdlib/<name>/` at the project root.

---

## 2. Core Module (`stdlib/core/core.kark`)

Fundamental operations that complement language builtins:

| Function | Signature | Description |
|----------|-----------|-------------|
| `clamp` | `(int, int, int) -> int` | Restrict value to range [lo, hi] |
| `clamp_float` | `(float64, float64, float64) -> float64` | Float clamp |
| `min` | `(int, int) -> int` | Smaller of two ints |
| `max` | `(int, int) -> int` | Larger of two ints |
| `min_float` | `(float64, float64) -> float64` | Smaller of two floats |
| `max_float` | `(float64, float64) -> float64` | Larger of two floats |
| `abs` | `(int) -> int` | Absolute value |
| `sign` | `(int) -> int` | Sign: -1, 0, or 1 |
| `sign_float` | `(float64) -> float64` | Float sign |
| `range` | `(int, int) -> int[]` | Generate integer sequence |
| `range_step` | `(int, int, int) -> int[]` | Generate sequence with step |
| `repeat` | `(int, int) -> int[]` | Array of n copies |
| `sum` | `(int[]) -> int` | Sum of array |
| `product` | `(int[]) -> int` | Product of array |
| `max_of` | `(int[]) -> int` | Maximum element |
| `min_of` | `(int[]) -> int` | Minimum element |
| `is_none` | `(Option) -> bool` | Check if Option is None |
| `is_some` | `(Option) -> bool` | Check if Option is Some |
| `is_ok` | `(Result) -> bool` | Check if Result is Ok |
| `is_err` | `(Result) -> bool` | Check if Result is Err |

---

## 3. String Module (`stdlib/string/string.kark`)

String manipulation built on Karkain's native string support:

| Function | Signature | Description |
|----------|-----------|-------------|
| `str_len` | `(string) -> int` | String length |
| `str_empty` | `(string) -> bool` | Check if empty |
| `str_concat` | `(string, string) -> string` | Concatenate two strings |
| `str_repeat` | `(string, int) -> string` | Repeat string n times |
| `str_starts_with` | `(string, string) -> bool` | Check prefix |
| `str_ends_with` | `(string, string) -> bool` | Check suffix |
| `str_contains` | `(string, string) -> bool` | Check substring |
| `str_index_of` | `(string, string) -> int` | First occurrence index (-1 if not found) |
| `str_last_index_of` | `(string, string) -> int` | Last occurrence index |
| `str_sub` | `(string, int, int) -> string` | Substring [start, end) |
| `str_slice` | `(string, int, int) -> string` | Substring by length |
| `str_trim` | `(string) -> string` | Trim whitespace |
| `str_trim_left` | `(string) -> string` | Trim leading whitespace |
| `str_trim_right` | `(string) -> string` | Trim trailing whitespace |
| `str_split` | `(string, string) -> string[]` | Split by delimiter |
| `str_to_upper` | `(string) -> string` | Uppercase (ASCII) |
| `str_to_lower` | `(string) -> string` | Lowercase (ASCII) |
| `str_replace` | `(string, string, string) -> string` | Replace all occurrences |
| `str_reverse` | `(string) -> string` | Reverse characters |
| `str_char_at` | `(string, int) -> string` | Character at index |
| `str_to_int` | `(string) -> int` | Parse integer |
| `str_from_int` | `(int) -> string` | Format integer |
| `str_count` | `(string, string) -> int` | Count non-overlapping occurrences |
| `str_join` | `(string[], string) -> string` | Join with separator |

---

## 4. Collections Module (`stdlib/collections/collections.kark`)

Array and map utility operations:

| Function | Signature | Description |
|----------|-----------|-------------|
| `array_contains` | `(int[], int) -> bool` | Check if value exists |
| `array_contains_str` | `(string[], string) -> bool` | String array contains |
| `array_index_of` | `(int[], int) -> int` | Index of first occurrence (-1) |
| `array_reverse` | `(int[]) -> int[]` | Reverse array |
| `array_reverse_str` | `(string[]) -> string[]` | Reverse string array |
| `array_copy` | `(int[]) -> int[]` | Shallow copy |
| `array_fill` | `(int, int) -> int[]` | Create filled array |
| `array_slice` | `(int[], int, int) -> int[]` | Sub-array [start, end) |
| `array_remove` | `(int[], int) -> int[]` | Remove first occurrence |
| `array_remove_at` | `(int[], int) -> int[]` | Remove at index |
| `array_insert` | `(int[], int, int) -> int[]` | Insert at index |
| `array_unique` | `(int[]) -> int[]` | Remove duplicates |
| `array_flatten` | `(int[][]) -> int[]` | Flatten nested arrays |
| `array_sum` | `(int[]) -> int` | Sum elements |
| `array_min` | `(int[]) -> int` | Minimum element |
| `array_max` | `(int[]) -> int` | Maximum element |

---

## 5. Math Module (`stdlib/math/math.kark`)

Comprehensive math operations using libc FFI for float64 support:

- **Constants**: PI, E, LN2, SQRT2, GOLDEN_RATIO, DEG_TO_RAD, RAD_TO_DEG
- **Basic**: abs, sqrt, cbrt, pow, square, cube
- **Rounding**: ceil, floor, round, clamp, lerp
- **Min/Max**: min, max, min_int, max_int
- **Exp/Log**: exp, exp2, log, log2, log10
- **Trig**: sin, cos, tan, asin, acos, atan, atan2 (radians and degrees)
- **Hyperbolic**: sinh, cosh, tanh
- **Modular**: fmod, mod, div, gcd, lcm
- **Distance**: hypot, distance_2d, distance_3d
- **Vectors**: vec2, vec3, vec4 with add/sub/scale/dot/normalize
- **Matrix**: identity, zero, mul, transpose, add, sub
- **Statistics**: mean, variance, stddev, sum, dot

---

## 6. IO Module (`stdlib/io/io.kark`)

I/O operations using libc FFI:

- **Formatted printing**: io_printf, io_eprintf, io_println, io_print
- **File operations**: io_open_read/write/append, io_close, io_read_all, io_read_line
- **File write**: io_write, io_writeln, io_writef, io_flush
- **File utils**: io_file_exists, io_delete, io_rename, io_file_size
- **Stdin**: io_getchar, io_read_stdin
- **Byte-level**: io_read_bytes, io_tell, io_seek

---

## 7. System Module (`stdlib/system/system.kark`)

Host environment interaction:

| Function | Signature | Description |
|----------|-----------|-------------|
| `exit` | `(int) -> void` | Terminate with exit code |
| `get_args` | `() -> string[]` | Command-line arguments |
| `get_env` | `(string) -> string` | Environment variable |
| `platform_name` | `() -> string` | OS identifier |
| `current_dir` | `() -> string` | Working directory |
| `file_exists` | `(string) -> bool` | Path existence check |
| `is_file` | `(string) -> bool` | Regular file check |
| `is_dir` | `(string) -> bool` | Directory check |
| `make_dir` | `(string) -> int` | Create directory |
| `list_dir` | `(string) -> string[]` | List directory contents |
| `temp_dir` | `() -> string` | Temporary directory path |
| `sleep` | `(int) -> int` | Pause execution |

---

## 8. Architecture

### Builtin vs Standard Library

Karkain maintains a clear distinction:

| Level | Example | Implementation |
|-------|---------|---------------|
| **Primitive** | `+`, `-`, `*`, `/` | Compiler codegen |
| **Builtin** | `len()`, `print()`, `str()` | C runtime (embedded) |
| **Standard Library** | `std.string.str_len()` | Karkain `.kark` files |
| **Runtime** | `karkain_alloc()` | C runtime (internal) |

Standard library functions may delegate to builtins (e.g., `str_len` calls `len`),
but provide discoverable, importable APIs.

### Import Resolution

```
import std.string
       │
       ▼
resolveImport("std.string")
       │
       ▼
findStdlibDir() → <project>/stdlib/
       │
       ▼
stdlib/string/ → string.kark files
       │
       ▼
Added to module graph in topological order
```

### C FFI

Standard library modules that need C functions use `import "libc" { ... }`:

```kark
import "libc" {
    func sqrt(x: float64) -> float64
    func sin(x: float64) -> float64
}
```

Multiple modules may declare the same libc functions — C allows duplicate
declarations with matching signatures.

---

## 9. What Was NOT Done

- No complete scientific mathematics library (only foundations)
- No full Unicode support (ASCII-only for case conversion)
- No concurrent/parallel collections
- No redesign of the package manager
- No repository-wide audit
