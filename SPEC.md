# Karkain Language Specification

**Version:** 1.0.0
**Status:** Production — extracted from the reference compiler (`pkg/lexer`, `pkg/parser`, `pkg/sema`, `pkg/codegen`, `pkg/ir/ssa`)
**Date:** 2026-09-06

> This document is the **single source of truth** for the Karkain programming
> language. It is derived from the working Go compiler, and the Phase 88
> self-hosted compiler (`src/compiler/*.kark`) must conform to it. Where this
> spec and the reference compiler disagree, the reference compiler wins until
> the spec is corrected Ã¢â‚¬â€ both must eventually converge.

## Section Status Legend

| Badge | Meaning |
|-------|---------|
| Ã¢Å“â€¦ **Implemented & tested** | Compiles via the Go reference compiler; covered by unit/E2E tests |
| Ã°Å¸Å¸Â¡ **Partial / unstable** | Syntax parsed; codegen or semantics incomplete |
| Ã°Å¸â€œÂ **Draft** | Declared in AST/parser, not implemented or unstable |
| Ã°Å¸Å¡Â« **Declared only** | In roadmap/README, not yet present in the compiler |

---

## 0. Conformance & Versioning

- Karkain follows semantic versioning for the **language spec itself**.
- A **conforming implementation** must accept every program accepted by the
  reference compiler (`cmd/karkain`) and reject programs it rejects, for all
  features marked Ã¢Å“â€¦.
- Features marked Ã°Å¸Å¸Â¡ / Ã°Å¸â€œÂ / Ã°Å¸Å¡Â« are informational and not conformance-bound until
  they reach Ã¢Å“â€¦.
- Conformance test corpus: `examples/*.kark` (E2E) + `pkg/*/*_test.go` (unit).
  One pre-existing known failure: `self_host_parser_test.kark`.

### Version history

| Spec version | Compiler phase | Notes |
|--------------|----------------|-------|
| 0.14.0 | 55d | KPM package manager integrated; SSA backend; string/slice types; closures |
| 1.0.0 | 90 | Production release: self-hosted compiler, runtime boundary, stdlib, native build, diagnostics |

---

## 1. Lexical Structure

### 1.1 Character set
Source is UTF-8 encoded text. The lexer operates on bytes; identifiers consist
of ASCII letters, digits, and underscore. Keywords are ASCII lowercase except
`Some`, `Ok`, `Err`.

### 1.2 Tokens

| Category | Tokens |
|----------|--------|
| Words | `func`, `print`, `println`, `let`, `var`, `return`, `if`, `else`, `import`, `matrix`, `alloc`, `free`, `addr`, `qreg`, `gate`, `measure`, `actor`, `spawn`, `receive`, `channel`, `send`, `macro`, `quote`, `unquote`, `comptime`, `while`, `for`, `type`, `struct`, `bool`, `bigint`, `bigfloat`, `true`, `false`, `kernel`, `device`, `global_id`, `barrier`, `mut`, `raw`, `move`, `Some`, `None`, `Ok`, `Err`, `match`, `linear`, `packed`, `enum`, `fn`, `in`, `break`, `continue` |
| Operators | `=`, `==`, `!=`, `<`, `<=`, `>`, `>=`, `+`, `-`, `*`, `/`, `%`, `&&`, `\|\|`, `!`, `&`, `@`, `.`, `?`, `=>` |
| Delimiters | `(`, `)`, `{`, `}`, `[`, `]`, `,`, `:`, `;` |
| Literal kinds | integer, float64, bigint, bigfloat, string, bool |
| Others | identifier (`IDENT`), `EOF`, `ILLEGAL` |

### 1.3 Comments
Line comments begin with `//` and run to end of line. (Block comments are not
yet defined in the lexer.)

### 1.4 Literals

| Literal | Example | Notes |
|---------|---------|-------|
| Integer | `42`, `-7` | Stored as `int` (64-bit on target) |
| Float | `3.14`, `1e9` | `float64` |
| BigInt | `12345678901234567890123` | Arbitrary precision; linked via GMP |
| BigFloat | `1.234567890123456789e100` | Arbitrary precision float |
| String | `"hello"` | Double-quoted; escaped sequences |
| Bool | `true`, `false` | |

---

## 2. Types

### 2.1 Primitive types Ã¢Å“â€¦

| Type | Description |
|------|-------------|
| `int` | 64-bit signed integer |
| `float64` | IEEE 754 double-precision float |
| `bool` | Boolean (`true` / `false`) |
| `string` | UTF-8 string (immutable) |
| `bigint` | Arbitrary-precision integer (GMP-backed) |
| `bigfloat` | Arbitrary-precision float (GMP-backed) |

### 2.2 Composite types

| Type | Status | Syntax | Description |
|------|--------|--------|-------------|
| Array | Ã¢Å“â€¦ | `[v1, v2, v3]` | Homogeneous list |
| Map | Ã¢Å“â€¦ | `{k1: v1, k2: v2}` | Key-value store |
| Slice | Ã¢Å“â€¦ | `arr[start:end]` | View over an array (open-ended with nil end) |
| Struct | Ã¢Å“â€¦ | `struct Name { f: T, ... }` | Named field record |
| Enum | Ã°Å¸Å¸Â¡ | `enum Name { A, B(int) }` | Algebraic data type with payload variants |
| Matrix | Ã¢Å“â€¦ | `matrix Name[rows, cols] of float64` | Continuous row-major 2D array, AVX2-optimized |
| Tensor | Ã°Å¸â€œÂ | `Tensor<f32, [32, 3]>` | Parametric N-D tensor with shape |
| Pointer | Ã¢Å“â€¦ | `*T` | Raw C pointer |
| Reference | Ã¢Å“â€¦ | `&T` / `&mut T` | Safe immutable/mutable borrow |

### 2.3 Option / Result Ã¢Å“â€¦

| Type | Constructors | Purpose |
|------|--------------|---------|
| `Option<T>` | `Some(v)`, `None` | Presence/absence (no null) |
| `Result<T,E>` | `Ok(v)`, `Err(e)` | Fallible operations (no exceptions) |

Exhaustive `match` is required; the compiler rejects incomplete patterns.

### 2.4 Linear & packed types Ã°Å¸Å¸Â¡

| Modifier | Example | Meaning |
|----------|---------|---------|
| `linear` | `linear type FileHandle { fd: int }` | Value must be consumed exactly once |
| `packed` | `packed struct Point { x: float32, y: float32 }` | No struct padding |

---

## 3. Grammar (concrete syntax)

### 3.1 Program structure Ã¢Å“â€¦

```
program    := statement*
statement  := funcDecl | varDecl | print | return | if | while | for
            | forIn | matrixDecl | alloc | free | addr | importBlock
            | qregDecl | gateApply | measure | actorDecl | spawn | receive
            | send | macroDecl | comptime | match | lambda (fn)
            | break | continue | exprStmt | block
```

`func` / `fn` both declare functions; `fn` also introduces lambdas and there is
a `TokenFn` for function-pointer expressions.

### 3.2 Functions Ã¢Å“â€¦

```
func name(p1: T1, p2: T2) -> R { stmt* }
fn   name(p1: T1, p2: T2) -> R { stmt* }
```

Parameter types are optional. `-> R` return type. Function bodies are statement
lists.

### 3.3 Variables Ã¢Å“â€¦

```
let  x = expr          // inferred type
let  x: T = expr       // explicit type (and *T pointer types)
var  x = expr          // alternative to `let`
matrix M[rows, cols] of T   // matrix variable
```

### 3.4 Control flow Ã¢Å“â€¦

```
if cond { ... } else { ... }
while cond { ... }
for init; cond; post { ... }        // C-style
for x in iterable { ... }           // Phase 47 for-in
for k, v in map { ... }             // Phase 48 map iteration
break
continue
return expr
```

`for-in` iterates arrays (element) and maps (key/value) by iterable type.

### 3.5 Match Ã¢Å“â€¦

```
match value {
  pattern1 => expr,
  pattern2 => expr,
  ...
}
```

Patterns: `Some(binding)`, `None`, `Ok(binding)`, `Err(binding)`, literals,
and wildcard. Arms are comma-separated.

### 3.6 Error propagation Ã°Å¸Å¸Â¡

```
func mayFail() -> Result<int, string> { ... }
let v = mayFail()?    // unwraps Ok(v), or returns Err(e) early
```

`?` desugars to `match expr { Ok(v) => v, Err(e) => return Err(e) }`.

### 3.7 Lambdas & closures Ã¢Å“â€¦

```
fn(a, b) { return a + b }      // lambda
let add = fn(x: int, y: int) { return x + y }
```

Lambdas capture free variables from the enclosing scope (Phase 54). Captures
are recorded on the AST and lowered with the SSA backend.

---

## 4. Expressions

### 4.1 Operator precedence Ã¢Å“â€¦

| Precedence | Operators | Associativity |
|-----------|-----------|---------------|
| 0 | `=` (assignment, handled at precedence 0) | right |
| 0 | `\|\|` | left |
| 1 | `&&` | left |
| 2 | `== != < <= > >=` | left |
| 3 | `+ -` | left |
| 4 | `* / %` | left |
| unary | `- ! & @` | Ã¢â‚¬â€ |

### 4.2 Expression forms Ã¢Å“â€¦

- Primary: literals, identifiers, parenthesized expressions
- Call: `name(args...)`, `C.func(args)` C interop, `ops.*` tensor ops, `qpu.*` quantum ops
- Index: `arr[i]`, `mat[r, c]`, `t[i, j, k]`
- Slice: `arr[start:end]`
- Dot: `obj.field`, nested `a.b.c`
- Binary / Unary as per precedence
- Borrow: `&x`, `&mut x`
- Raw access: `@raw(addr)`, `@raw(addr, val)`
- Move: `move(x)`
- Address-of: `@addr(x)` (via `TokenAddr`)
- Alloc/Free: `alloc<T>(n)`, `free(ptr)`
- Option/Result: `Some(v)`, `None`, `Ok(v)`, `Err(v)`
- Enum variant: `Color.Red`, `Ok(42)`
- Match (expression)

### 4.3 GPU expressions Ã°Å¸â€œÂ

- `global_id(0|1|2)` Ã¢â‚¬â€ thread index per dimension
- `barrier()` Ã¢â‚¬â€ thread-block sync
- `ops.*` tensor operations: `matmul`, `relu`, `softmax`, `conv2d`, `transpose`
- `shape_of(x)` Ã¢â‚¬â€ query tensor shape

### 4.4 Quantum expressions Ã°Å¸â€œÂ

- `measure q[i]`
- `qpu.h(q[i])`, `qpu.cx(q[i], q[j])`, `qpu.rx(theta, q[i])`

### 4.5 Coroutine/async expressions Ã°Å¸â€œÂ

- `co name(params) { ... }` Ã¢â‚¬â€ coroutine declaration
- `async { ... }`, `await(expr)`, `yield(value)`
- `ch <- value` (send), `<-ch` (receive), `chan<T>(bufsize)` (create)
- `select { case ... }`
- `gospawn(fn(...))`, `await_all(f1, f2, ...)`

---

## 5. Memory Model, Ownership & Borrowing Ã¢Å“â€¦

Karkain targets **safe by default, unsafe with `@raw`**.

### 5.1 Ownership rules
- Every value has exactly one owner at a time.
- Copying a value into a function parameter or return transfers ownership
  (move semantics) Ã¢â‚¬â€ the source binding becomes unusable.
- Escape analysis (`pkg/parser/escape.go`, Phase 49) marks variables that
  escape their scope (passed to funcs, returned, or captured by a lambda).

### 5.2 Borrowing
- `&x` Ã¢â‚¬â€ immutable borrow; multiple immutable borrows allowed, no mutation.
- `&mut x` Ã¢â‚¬â€ exclusive mutable borrow; no other borrows while active.
- The borrow checker (`pkg/sema/borrow_checker.go`, Phase 51) enforces lexical
  scoping of borrows.

### 5.3 Raw memory
- `@raw(addr)` read, `@raw(addr, val)` write Ã¢â‚¬â€ escapes safety checks, target of
  hardware/FFI access.
- `alloc<T>(n)` / `free(ptr)` Ã¢â‚¬â€ manual heap management.
- `addr`, `*T` pointer types for C interop.

### 5.4 Linear types Ã°Å¸Å¸Â¡
Values of `linear` types must be consumed exactly once Ã¢â‚¬â€ resource safety
(file handles, allocations).

---

## 6. Functions & Standard Builtins

### 6.1 Builtin callables Ã¢Å“â€¦ (`pkg/codegen/lower.go`)

| Builtin | Description |
|---------|-------------|
| `len(x)` | Length of array/map/string |
| `sqrt(x)` | Square root |
| `pow(x,y)` | Power |
| `hasKey(map, k)` | Map membership |
| `readFile(p)` / `writeFile(p, d)` | File I/O |
| `trim(s)` | Trim whitespace |
| `contains(s, sub)` | Substring check |
| `split(s, sep)` | Split string |
| `appendArray(arr, x)` / `push(arr, x)` | Append (mutating) |
| `delete(map, k)` | Delete key (mutating) |

### 6.2 Runtime functions (`.kark` runtime supported) Ã¢Å“â€¦
`getArgs`, `openFile`, `readLine`, `closeFile`, `createFile`, `writeToFile`,
`system`, `removeFile`, `substr`, `replaceExtension` (self-hosted compiler uses
these; runtime provided by `runtime.c` supporting the compiler itself).

> The general-purpose standard library beyond the above is still being
> formalized (Phases 60).

### 6.2b Standard Library v2 (Phase 109)

The importable standard library surface (`import std.<name>`, both engines,
byte-identical) comprises:

| Module | Surface |
|--------|---------|
| `std.string` | `str_len`, `str_empty`, `str_concat`, `str_repeat`, `str_starts_with`, `str_ends_with`, `str_contains`, `str_index_of`, `str_last_index_of`, `str_sub`, `str_slice`, `str_trim`, `str_trim_left`, `str_trim_right`, `str_split`, `str_to_upper`, `str_to_lower`, `str_replace`, `str_reverse`, `str_char_at`, `str_to_int`, `str_from_int`, `str_from_float`, `str_is_empty`, `str_count`, `str_join` |
| `std.collections` | `array_contains`, `array_contains_str`, `array_index_of`, `array_reverse`, `array_reverse_str`, `array_copy`, `array_fill`, `array_slice`, `array_remove`, `array_remove_at`, `array_insert`, `array_unique`, `array_flatten`, `array_sum`, `array_min`, `array_max`, `map_keys` |
| `std.io` | `io_open_write`, `io_append`, `io_create_file`, `io_file_exists`, `io_delete_file`, `io_is_file`, `io_is_dir`, `io_write`, `io_writeln`, `io_read_line`, `io_read_lines`, `io_read_all`, `io_close`, `io_list_files`, `io_path_sep`, `io_home_dir` |
| `std.encoding` | `hex_encode`, `hex_decode`, `base64_encode`, `base64_decode`, `utf8_valid`, `utf8_encode`, `utf8_decode` |
| `std.crypto` | `sha256`, `sha512` |

Strings are UTF-8 byte strings: `len()` is the byte length and indexing is
byte-wise; case conversion is ASCII-only and leaves multibyte text untouched.
Malformed hex/base64 raise the Phase 100 runtime-error model
(`runtime error: invalid hex string at <file>:<line>`, exit non-zero) rather
than returning incorrect data. Digests are hex-lowercase, deterministic and
verified against NIST vectors. `stdlib/{core,math,system,gpu,async}` remain
outside the importable surface (see Phase 109 report).

### 6.3 SIMD & vector types (Phase 106)
Lane-vector variables are declared with array-of-scalar annotations
`[N]f32` / `[N]f64` / `[N]i32` / `[N]i64`. Supported widths:

| Annotation | C lane type | x86 lowering |
|---|---|---|
| `[4]f32` / `[8]f32` | `karkain_f32x4` / `karkain_f32x8` | `__m128` / `__m256` (`-mavx` auto-appended) |
| `[2]f64` / `[4]f64` | `karkain_f64x2` / `karkain_f64x4` | `__m128d` / `__m256d` (`-mavx` for 4) |
| `[4]i32` / `[8]i32` | `karkain_i32x4` / `karkain_i32x8` | GNU `vector_size` (int) |
| `[2]i64` / `[4]i64` | `karkain_i64x2` / `karkain_i64x4` | GNU `vector_size` (int) |

Builtins (lower to portable `karkain_simd_*` C helpers using GNU vector
operators; integer mul/div and `@simd_sum` reduce lane-wise):

| Builtin | Meaning |
|---------|---------|
| `@simd_splat(x, n)` | broadcast scalar `x` into `n` lanes of an elementwise vector |
| `@simd_add(a, b)` / `@simd_sub(a, b)` / `@simd_mul(a, b)` / `@simd_div(a, b)` | elementwise lane arithmetic |
| `@simd_sum(v)` | reduce a lane vector to a scalar value (f32/f64 → float, i32/i64 → int) |
| `@simd_load` / `@simd_store` | Phase 70 SSE memory round-trip (legacy) |

Lane vectors are local-declaration and operand values; scalar-only programs
keep the Phase 70 behavior unchanged. Full parity of the self-hosted (`kcc`)
engine for these builtins is post-106 (its parser already accepts `@simd_*`).

---

## 7. Heterogeneous Backends

### 7.1 Compiler pipeline Ã¢Å“â€¦

```
.kark Ã¢â€ â€™ lexer Ã¢â€ â€™ parser (AST) Ã¢â€ â€™ SSA IR Ã¢â€ â€™ optimizer Ã¢â€ â€™ verifier Ã¢â€ â€™ C23 Ã¢â€ â€™ GCC/Clang/MSVC Ã¢â€ â€™ binary
```

- **Primary path (SSA):** `pkg/ir/ssa` Ã¢â‚¬â€ block-param CFG (no phis), variable
  memory cells, ops `{Const, BinOp, UnOp, Call, CallVoid, Br, Jmp, Ret,
  IndexGet, IndexSet, Print, RawC, OpLoad, OpStore}`.
- **Fallback path:** legacy direct AST Ã¢â€ â€™ C (`pkg/codegen/codegen.go`) used when
  SSA lowering fails (safety net).
- Verified by `pkg/ir/ssa/verify.go`.

### 7.2 GPU kernel Ã¢â€ â€™ WGSL/OpenCL/SPIR-V Ã°Å¸â€œÂ

- `kernel` functions, `global_id`, `barrier` emit through
  `pkg/codegen/{wgsl,gpu,spirv}.go`.
- Host launcher auto-generation via `gpu_host.go`.
- Tensor ops emit to WGSL (`tensor_wgsl.go`).

### 7.3 Quantum Ã¢â€ â€™ OpenQASM 3 / QIR Ã°Å¸â€œÂ

- Bare gate syntax: `H q[0]`, `CNOT q[0], q[1]`, `Rx(ÃŽÂ¸) q[2]`
- Emitters: `pkg/codegen/{qasm,qir,openpulse}.go`
- Safety analysis: `pkg/sema/quantum.go` (no-cloning theorem, measurement
  collapse, gate-after-measurement).
- Noise simulation, error correction, distribution planner present in sema.

### 7.4 Actor distributed concurrency Ã°Å¸â€œÂ

- `actor`/`spawn`/`receive`/`channel`/`send` (phases 16/37), runtime
  `pkg/runtime/actor_system.go`.

### 7.5 Coroutine / green-thread scheduler Ã°Å¸Å¸Â¡

- `co` declarations, `async`/`await`/`yield`, channels, `select`, `gospawn`
  (Phase 38), runtime `pkg/runtime/coroutine.go`.

---

## 8. Macros & Metaprogramming Ã°Å¸Å¸Â¡

| Form | Syntax | Notes |
|------|--------|-------|
| Macro declaration | `macro name(params) { ... }` | Hygienic |
| Macro expansion | `name(args)` | Via `macro.go` expander |
| Quote | `` \(`quote expr`) `` | |
| Unquote | `unquote expr` | |
| Comptime | `comptime var x = ...` / `comptime { ... }` | Compile-time evaluation |
| Reflect | `reflect` on type | |
| Derive | `derive(Trait)` | |
| Tag | `tag` | Field/type tags |

GPU kernel auto-generics (Phase 26) and tensor autodiff
(`pkg/sema/autodiff.go`) extend metaprogramming.

---

## 9. Generics & Traits

| Feature | Status | Syntax |
|---------|--------|--------|
| Monomorphized generics | Ã°Å¸Å¸Â¡ | `func max<T: Numeric>(a: T, b: T) -> T` |
| Generic parameters | Ã°Å¸Å¸Â¡ | `T: Constraint` |
| Trait declarations | Ã°Å¸â€œÂ | `trait Numeric { fn add(self, other: T) -> T; }` |
| Trait impls | Ã°Å¸â€œÂ | `impl Numeric for int { ... }` |
| Struct generics | Ã°Å¸Å¸Â¡ | `struct Vector<T> { ... }` |
| Kernel generics | Ã°Å¸â€œÂ | `kernel k<T>(...)` |

Instantiation is monomorphic (compile-time per concrete type argument).

---

## 10. Package Manager (KPM) Ã¢Å“â€¦

`karkain pkg ...` subcommands (all in the single `karkain.exe` binary):

- Project: `init`, `add`, `remove`, `update`, `upgrade`, `fetch`, `deps`
- Registry: `search`, `info`, `publish`, `login`, `logout`, `whoami`
- Security: `audit`, `audit --licenses`, `verify`
- Cache: `cache list`, `cache clean`, `cache clean --stale`, `cache path`
- Workspace: `workspace init`, `workspace add`, `workspace build`, `workspace test`

### 10.1 Manifest `karkain.toml`

```toml
[package]
name = "my-project"
version = "1.0.0"
author = "Name"
description = "..."
license = "MIT"

[dependencies]
math = { version = "^1.0.0", source = "registry" }

[features]
default = ["std"]
```

### 10.2 Version syntax (semver) Ã¢Å“â€¦

`1.2.3` exact Ã‚Â· `^1.2.3` compatible Ã‚Â· `~1.2.3` patch Ã‚Â· `>=1.0 <2.0` range Ã‚Â·
`1.2.x` wildcard Ã‚Â· `*` any.

Lock file `karkain.lock`, SHA-256 integrity checks (`pkg/pm/integrity.go`),
auth token at `~/.karkain/auth.json`. Registry endpoint:
`https://registry.karkain.dev` (override `KARKAIN_REGISTRY`).

---

## 11. C Interop Ã¢Å“â€¦

```
import { <raw C code> }
```

C import blocks are embedded verbatim and emitted into generated C. Native C
function calls use the `C.` prefix (`C.sqrt(...)`). FFI package
(`pkg/sema/ffi.go`, `pkg/codegen/native.go`) provides binding generation.

---

## 12. Compiler Diagnostics & Tooling

| Tool | Status | Notes |
|------|--------|-------|
| `run` | Ã¢Å“â€¦ | Compile & execute |
| `build` | Ã¢Å“â€¦ | Native executable |
| `transpile` | Ã¢Å“â€¦ | Emit C (keeps `.c`) |
| `check` | Ã¢Å“â€¦ | Parse + semantic validation |
| `test` | Ã¢Å“â€¦ | Discover/run `*_test.kark` |
| `lsp` | Ã°Å¸Å¸Â¡ | Go-based LSP server (`pkg/lsp`) |
| `jit` | Ã°Å¸Å¸Â¡ | `pkg/jit` JIT/FFI |
| Debug mode | Ã¢Å“â€¦ | `-g` emits `#line` + `-line N "file.kark"` directives; native pipeline emits DWARF 4 sections `.debug_info`/`.debug_abbrev`/`.debug_str`/`.debug_line` (`pkg/codegen/dwarf.go` + self-hosted `dwarf_parse.go` reader) |
| Verbose | Ã¢Å“â€¦ | `--verbose` pipeline logging |
| Targets | Ã°Å¸Å¸Â¡ | native, wasm32-wasi |

---

## 13. Known Gaps & Non-conformances

| Area | Gap | Tracking |
|------|-----|----------|
| Self-hosted compiler | `self_host_parser_test.kark` fails; `src/compiler/*.kark` incomplete | Phase 88 |
| Enum payload variants | Enums parse but payload handling unstable | BUG-7 |
| Struct codegen | Certain codegen paths incomplete | BUG-1 |
| Option/Result match arms | Codegen edge cases | BUG-2 |
| Index assignment | Specific patterns | BUG-3 |
| GPU/quantum backends | Parsers exist; full emitters not conformance-tested | Phases 58-59 |
| Concurrency stdlib | Actor/coroutine runtimes partial | Phases 57, 63 |
| wasm32-wasi | Compile-only, no runtime execution | Ã¢â‚¬â€ |

| Capability summary | See section 14 (Language Foundation) | Phase 102-F |

---

## 14. Capability Summary (Language Foundation)

Status legend: **Y** = works identically on both engines (Go front end and the
self-hosted `kcc` engine); **N** = not supported on either engine (kept out of
the foundation corpus; parity preserved). Golden-output corpus:
`examples/language_foundation/` (13 targets incl. `modules/` and `application/`),
gated by `pkg/cli/phase102_foundation_test.go`.

| Capability | Status | Notes |
|------------|--------|-------|
| Variables, `let`/`var` | Y | int/float/bool/string scalar typing |
| Literals & arithmetic | Y | `+ - * / %`, precedence, comparisons |
| Functions & returns | Y | params, named returns, multiple call sites |
| Recursion | Y | factorial / fibonacci / fib-sum probes |
| Arrays | Y | literal, `push`/`pop`/`get`/`set`, length, index assign |
| Strings | Y | concat, index, length, prefix check |
| `if` / `elif` / `else` | Y | incl. unparenthesized condition |
| Loops | Y | `while`; C-style `for` with empty init; `for (let i = ...)` |
| `match` | Y | int and string discriminants, wildcard arm |
| Maps | Y | literal, get/set/has, default-value semantics |
| Structs | Y | `type T struct { ... }`, `,` and `;` field separators |
| Records as methods | Y | record-as-first-argument idiom (no receiver syntax) |
| Multi-file modules | Y | sibling assembly in one directory; qualified access `math.twice(21)` (Phase 103) |
| Application layout | Y | parts of a project across files call each other |
| Module `import` (sibling) | Y | `import math` qualifies calls within an assembly unit (Phase 103); no external package import |
| Visibility (`public`) | Y | `public func/type/enum`; private cross-module calls rejected, exit 3 (Phase 103) |
| Runtime error diagnostics | Y | div/mod-by-zero, array/string OOB (Phase 100) |
| Stack traces on runtime error | Y | identical frames on both engines (Phase 101) |
| Multi-error reporting | Y | ALL recoverable parse + resolve diagnostics in one invocation, exit 3, on both engine paths (Phase 105) |
| Incremental build | Y | `karkain build --incremental` dependency-aware content cache; `karkain clean` purges (Phase 105) |
| `float64()` / `bool()` / `string()` casts | N | not accepted by either engine |
| Closures / `fn` codegen | N | known broken on both engines |
| `const` declarations | N | not part of the engine surface |
| User `import` | N | deferred; sibling/module assembly only |
| Visibility rules | N | deferred to module system v2 (Phase 103) |

---

## 15. Module System v2 (Core) — Phase 103

Multi-file programs are compiled as a single assembly unit (sibling files in
one directory, dependency files first, root file last). Within a unit,
files form modules by name and calls resolve through the module contract.

### 15.1 `public` export modifier

`public` may prefix `func`, `type`, or `enum` declarations. It is the gate for
cross-module access: a declaration without `public` is private to its file and
cannot be called by another module.

```
# math.kark
public func twice(x int) int { return x * 2 }
func secret(x int) int { return x * 3 }   # private

# main.kark
import math
func main() {
    println(math.twice(21))     # 42
}
```

`public` before `let`/`var` is a parse error on both engines (only
func/type/enum are exportable in the Core phase). Stdlib files are exempt from
the private rule: stdlib functions are framework API surface. Phase 109
(Standard Library v2) ships the importable `std.*` modules without physical
`public` markers — the exemption is retained and physical markers are deferred
to a future stdlib hardening phase.

### 15.2 Qualified calls

`module.function(args)` — in expression and statement position — binds to the
named module's public export. The self-hosted engine lowers qualified calls to
the same flat user-function symbol as bare calls; record-idiom receiver calls
(`acc.deposit(10)`) use the same AST shape and are interchangeable in codegen.
`C.*` remains reserved for C interop (dotted, not module-resolved).

### 15.3 Diagnostics (exit 3)

| Condition | Diagnostic |
|-----------|------------|
| caller lacks `import module` | `module 'm' is not imported; add 'import m'` |
| qualifier is not a unit file | `module 'm' is not part of the compile unit` |
| undefined function | `function 'x' is not defined` |
| wrong module | `function 'x' is not exported by module 'm' (defined in '...')` |
| private | `function 'x' in module 'm' is private and cannot be called by another module` |
| collision | existing flat-unit duplicate-definition diagnostic |

### 15.4 Deferred to Module System v2.1

Re-exports (`import math.expose ...`), aliasing (`import math as m`),
`public let/var` value exports, and cross-module qualified *construction* of
private types (parsing records `.Public` on struct/enum; check stays
conservative in Core).

---

## Appendix A: Keyword Reference Table

| Keyword | Token | Domain |
|---------|-------|--------|
| `func` / `fn` | FUNC / FN | functions, lambdas |
| `print` / `println` | PRINT | output |
| `let` / `var` | LET / VAR | variables |
| `return` | RETURN | control |
| `if` / `else` | IF / ELSE | control |
| `while` / `for` / `in` | WHILE / FOR / IN | loops |
| `break` / `continue` | BREAK / CONTINUE | loops |
| `match` | MATCH | ADT |
| `import` | IMPORT | C interop |
| `struct` / `type` / `enum` | STRUCT / TYPE / ENUM | types |
| `bool` / `bigint` / `bigfloat` | BOOL / BIGINT / BIGFLOAT | types |
| `true` / `false` | TRUE / FALSE | literals |
| `alloc` / `free` / `addr` | ALLOC / FREE / ADDR | memory |
| `matrix` | MATRIX | matrix |
| `mut` / `raw` / `move` | MUT / RAW / MOVE | memory safety |
| `linear` / `packed` | LINEAR / PACKED | type modifiers |
| `Some` / `None` / `Ok` / `Err` | SOME / NONE / OK / ERR | Option/Result |
| `macro` / `quote` / `unquote` / `comptime` | MACRO / QUOTE / UNQUOTE / COMPTIME | meta |
| `kernel` / `device` / `global_id` / `barrier` | KERNEL / DEVICE / GLOBAL_ID / BARRIER | GPU |
| `qreg` / `gate` / `measure` | QREG / GATE / MEASURE | quantum |
| `actor` / `spawn` / `receive` / `channel` / `send` | ACTOR / SPAWN / RECEIVE / CHANNEL / SEND | concurrency |

Contextual (not lexer keywords, recognized by parser in context): `simd`,
`derive`, `tag`.

---

## Appendix B: Pipeline Validation

To verify the spec matches reality, run:

```
go test ./pkg/lexer/... ./pkg/parser/... ./pkg/codegen/... ./pkg/pm/... -count=1
```

and the E2E corpus:

```
for f in examples/*.kark; do ./karkain run "$f"; done
```

---

*This is a living document. Sections transition from Ã°Å¸â€œÂÃ¢â€ â€™Ã°Å¸Å¸Â¡Ã¢â€ â€™Ã¢Å“â€¦ as the compiler
implements them, and conformance tests are added.*
