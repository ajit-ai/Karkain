# Karkain Standard Library

Phase 87 laid the foundation for Karkain's standard library — importable
modules providing core utilities, string operations, collections, math, I/O,
and system interaction.

**Phase 109 (Standard Library v2)** reworked the practical import surface so
that real `.kark` programs can use `import std.string`, `std.collections`,
`std.io`, `std.encoding` and `std.crypto` through the normal toolchain on
**both** engines (Go front end and the self-hosted kcc engine), verified
byte-identical. These five `std.*` modules are the currently importable
component of the library; the remaining directories below remain behind the
Phase-109 boundary (see §9).

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
├── gpu/            GPU compute
├── encoding/       Hexadecimal, Base64 and UTF-8 byte encoding  [Phase 109]
└── crypto/         SHA-256 / SHA-512 digests                      [Phase 109]
```

### Importing

Standard library modules are imported using the `std.` prefix:

```kark
import std.string
import std.collections
import std.io
import std.encoding
import std.crypto
```

The module resolver maps `std.<name>` to `stdlib/<name>/` at the project root.
Phase 109 wires the import into the whole toolchain: `check`, `build`, `run`
and `test` assemble the referenced module source into the compiled unit (see
[§8](#8-architecture)); the self-hosted kcc engine performs the same assembly
and strips module import lines before parsing (its parser only understands
bare `import <name>`, and `import "C" { ... }` blocks are preserved).

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
| `str_from_float` | `(float64) -> string` | Format float |
| `str_count` | `(string, string) -> int` | Count non-overlapping occurrences |
| `str_join` | `(string[], string) -> string` | Join with separator |
| `str_is_empty` | `(string) -> bool` | Check if empty |

Strings are UTF-8 byte strings: `len()` is the byte length, `s[i]` is a single
byte as a 1-character string. Case mapping (`str_to_upper`/`str_to_lower`) is
deliberately ASCII and byte-wise (table-based, no character arithmetic), so it
produces identical results on both engines and never corrupts multibyte text —
multibyte characters pass through unchanged. Try `examples/stdlib/strings/utf8.kark`
for the byte-round-trip view.

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
| `map_keys` | `(map, ...) -> string[]` | Keys of a map as string array |

`map_keys` delegates to the `map_keys_of` builtin. Because the two engines lay
maps out differently, `map_keys_of` normalizes the key array to a string array
inside the C runtime, so both engines emit the same `make_array`/`make_string`
sequence.

---

## 5. Encoding Module (`stdlib/encoding/encoding.kark`)

Practical byte encodings. Implemented atop eight byte-level runtime builtins
(see §8) that exist in **both** engines (Go codegen and the self-hosted kcc C
preamble), so behavior — including error handling — is byte-identical.

| Function | Signature | Description |
|----------|-----------|-------------|
| `hex_encode` | `(string) -> string` | Lowercase hexadecimal of input bytes |
| `hex_decode` | `(string) -> string` | Decode hexadecimal back to bytes |
| `base64_encode` | `(string) -> string` | Standard Base64 (RFC 4648 §4, with padding) |
| `base64_decode` | `(string) -> string` | Standard Base64 decode |
| `utf8_valid` | `(string) -> bool` | True if bytes form valid UTF-8 |
| `utf8_encode` | `(string) -> string` | Identity (Karkain strings are UTF-8 bytes) |
| `utf8_decode` | `(string) -> string` | Identity |

Malformed input raises a source-located runtime error on both engines
(`runtime error: invalid hex string at <file>:<line>`,
`runtime error: invalid base64 string at <file>:<line>`) and exits non-zero —
it never silently returns incorrect data. Empty input round-trips to empty.

## 6. Crypto Module (`stdlib/crypto/crypto.kark`)

Deterministic, well-defined digests implemented in the embedded C runtime
(NIST SHA-256 with the 64-entry K256 round constants, SHA-512 with the 80-entry
K512 constants, bit-length padding, full byte-order finalization):

| Function | Signature | Description |
|----------|-----------|-------------|
| `sha256` | `(string) -> string` | Lowercase hex SHA-256 digest of the input bytes |
| `sha512` | `(string) -> string` | Lowercase hex SHA-512 digest of the input bytes |

Verified against standard test vectors (NIST FIPS 180): `sha256("abc")` =
`ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad`,
`sha256("")` = `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`,
`sha512("abc")` =
`ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f`,
plus `sha256("karkain")` =
`00e0cba20c10cac449eb885a9926a4b646f0ac163ed7fbc5704d9d8a057ef44d`.
Digests are stable across repeated runs and byte-identical on both engines.

No non-cryptographic construct is labeled as secure randomness; there is no
random/encryption surface in this module (see §9).

---

## 7. Math Module (`stdlib/math/math.kark`)

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

## 8. IO Module (`stdlib/io/io.kark`)

File I/O built on the runtime's platform abstraction (no libc FFI):

- **File lifecycle**: io_open_write, io_append, io_create_file, io_file_exists, io_delete_file, io_is_file, io_is_dir
- **Read/write**: io_write, io_writeln, io_read_line, io_read_lines, io_read_all, io_close
- **Filesystem**: io_list_files, io_path_sep, io_home_dir

File handles are shared; on Windows a file must be `io_close`d before
`io_delete_file` or it stays locked. `examples/stdlib/io/main.kark` creates a
file, writes data, reads it back, verifies the result and cleans up on both
engines.

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

## 9. Architecture

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

### Phase 109 byte-level builtins

Eight new byte-level runtime builtins back the encoding/crypto/collections
modules. They exist on **both** engines: the Go codegen emits the C helper
bodies in its preamble and dispatches them in `genExpr`; the self-hosted kcc
compiler emits the same C helpers from `src/compiler/codegen.kark` and
dispatches via the same names. Both helpers and dispatch tables agree on
semantics and error behavior, which is why hex/Base64 encoding, UTF-8
validation, map-key extraction and SHA digests are byte-identical across
engines.

| Builtin | Arity | Description |
|---------|-------|-------------|
| `hex_encode_bytes` | 1 | Lowercase hexadecimal of input bytes |
| `hex_decode_bytes` | 1 | Decode hex; malformed input → `runtime error: invalid hex string` |
| `base64_encode_bytes` | 1 | RFC 4648 §4 Base64 with padding |
| `base64_decode_bytes` | 1 | Decode; malformed input → `runtime error: invalid base64 string` |
| `utf8_valid_bytes` | 1 | True if bytes form valid UTF-8 |
| `sha256_hex` | 1 | Hex SHA-256 digest |
| `sha512_hex` | 1 | Hex SHA-512 digest |
| `map_keys_of` | 1 | Map keys as a string array |

Error reporting uses the existing Phase 100 runtime-error model — source
file and line come from the call site, identical on both engines.

### Module assembly

`import std.<name>` is resolved by the module loader during source assembly
(`resolveSourcesRun` in `pkg/cli`) for `check`, `build`, `run` and `test`.
The self-hosted kcc engine performs the same resolution in `kccAssembleSource`
and strips the (dotted) import lines before handing the assembled text to its
parser. The full `examples/stdlib_v2/main.kark` program imports
`std.string`, `std.collections`, `std.encoding` and `std.crypto` together and
runs identically on both engines.

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

## 10. What Was NOT Done

- No complete scientific mathematics library (only foundations)
- No full Unicode support (ASCII-only for case conversion; UTF-8 bytes are
  always preserved and validated, never silently treated as ASCII)
- No concurrent/parallel collections
- No secure randomness provider or encryption surface: nothing non-cryptographic
  is labeled as secure, and the crypto module deliberately exposes only the
  deterministic SHA-256/SHA-512 digests
- **`stdlib/core`, `stdlib/math`, `stdlib/system`, `stdlib/gpu`,
  `stdlib/async` are not Phase 109 importable modules** — their sources predate
  Phase 109's canonical-syntax and builtin-wiring requirements and remain
  behind the Phase-109 boundary (documented in
  `docs/audit/PHASE-109-STANDARD-LIBRARY-V2-FINAL-REPORT.md`). Future modules
  should extend the Phase 109 pattern.
- No redesign of the package manager
- No repository-wide audit
