# Karkain Language Specification

**Version:** 0.14.0
**Status:** Working draft — extracted from the reference compiler (`pkg/lexer`, `pkg/parser`, `pkg/sema`, `pkg/codegen`, `pkg/ir/ssa`)
**Date:** 2026-08-29

> This document is the **single source of truth** for the Karkain programming
> language. It is derived from the working Go compiler, and the Phase 56
> self-hosted compiler (`src/compiler/*.kar`) must conform to it. Where this
> spec and the reference compiler disagree, the reference compiler wins until
> the spec is corrected — both must eventually converge.

## Section Status Legend

| Badge | Meaning |
|-------|---------|
| ✅ **Implemented & tested** | Compiles via the Go reference compiler; covered by unit/E2E tests |
| 🟡 **Partial / unstable** | Syntax parsed; codegen or semantics incomplete |
| 📝 **Draft** | Declared in AST/parser, not implemented or unstable |
| 🚫 **Declared only** | In roadmap/README, not yet present in the compiler |

---

## 0. Conformance & Versioning

- Karkain follows semantic versioning for the **language spec itself**.
- A **conforming implementation** must accept every program accepted by the
  reference compiler (`cmd/karkain`) and reject programs it rejects, for all
  features marked ✅.
- Features marked 🟡 / 📝 / 🚫 are informational and not conformance-bound until
  they reach ✅.
- Conformance test corpus: `examples/*.kar` (E2E) + `pkg/*/*_test.go` (unit).
  One pre-existing known failure: `self_host_parser_test.kar`.

### Version history

| Spec version | Compiler phase | Notes |
|--------------|----------------|-------|
| 0.14.0 | 55d | KPM package manager integrated; SSA backend; string/slice types; closures |

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

### 2.1 Primitive types ✅

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
| Array | ✅ | `[v1, v2, v3]` | Homogeneous list |
| Map | ✅ | `{k1: v1, k2: v2}` | Key-value store |
| Slice | ✅ | `arr[start:end]` | View over an array (open-ended with nil end) |
| Struct | ✅ | `struct Name { f: T, ... }` | Named field record |
| Enum | 🟡 | `enum Name { A, B(int) }` | Algebraic data type with payload variants |
| Matrix | ✅ | `matrix Name[rows, cols] of float64` | Continuous row-major 2D array, AVX2-optimized |
| Tensor | 📝 | `Tensor<f32, [32, 3]>` | Parametric N-D tensor with shape |
| Pointer | ✅ | `*T` | Raw C pointer |
| Reference | ✅ | `&T` / `&mut T` | Safe immutable/mutable borrow |

### 2.3 Option / Result ✅

| Type | Constructors | Purpose |
|------|--------------|---------|
| `Option<T>` | `Some(v)`, `None` | Presence/absence (no null) |
| `Result<T,E>` | `Ok(v)`, `Err(e)` | Fallible operations (no exceptions) |

Exhaustive `match` is required; the compiler rejects incomplete patterns.

### 2.4 Linear & packed types 🟡

| Modifier | Example | Meaning |
|----------|---------|---------|
| `linear` | `linear type FileHandle { fd: int }` | Value must be consumed exactly once |
| `packed` | `packed struct Point { x: float32, y: float32 }` | No struct padding |

---

## 3. Grammar (concrete syntax)

### 3.1 Program structure ✅

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

### 3.2 Functions ✅

```
func name(p1: T1, p2: T2) -> R { stmt* }
fn   name(p1: T1, p2: T2) -> R { stmt* }
```

Parameter types are optional. `-> R` return type. Function bodies are statement
lists.

### 3.3 Variables ✅

```
let  x = expr          // inferred type
let  x: T = expr       // explicit type (and *T pointer types)
var  x = expr          // alternative to `let`
matrix M[rows, cols] of T   // matrix variable
```

### 3.4 Control flow ✅

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

### 3.5 Match ✅

```
match value {
  pattern1 => expr,
  pattern2 => expr,
  ...
}
```

Patterns: `Some(binding)`, `None`, `Ok(binding)`, `Err(binding)`, literals,
and wildcard. Arms are comma-separated.

### 3.6 Error propagation 🟡

```
func mayFail() -> Result<int, string> { ... }
let v = mayFail()?    // unwraps Ok(v), or returns Err(e) early
```

`?` desugars to `match expr { Ok(v) => v, Err(e) => return Err(e) }`.

### 3.7 Lambdas & closures ✅

```
fn(a, b) { return a + b }      // lambda
let add = fn(x: int, y: int) { return x + y }
```

Lambdas capture free variables from the enclosing scope (Phase 54). Captures
are recorded on the AST and lowered with the SSA backend.

---

## 4. Expressions

### 4.1 Operator precedence ✅

| Precedence | Operators | Associativity |
|-----------|-----------|---------------|
| 0 | `=` (assignment, handled at precedence 0) | right |
| 0 | `\|\|` | left |
| 1 | `&&` | left |
| 2 | `== != < <= > >=` | left |
| 3 | `+ -` | left |
| 4 | `* / %` | left |
| unary | `- ! & @` | — |

### 4.2 Expression forms ✅

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

### 4.3 GPU expressions 📝

- `global_id(0|1|2)` — thread index per dimension
- `barrier()` — thread-block sync
- `ops.*` tensor operations: `matmul`, `relu`, `softmax`, `conv2d`, `transpose`
- `shape_of(x)` — query tensor shape

### 4.4 Quantum expressions 📝

- `measure q[i]`
- `qpu.h(q[i])`, `qpu.cx(q[i], q[j])`, `qpu.rx(theta, q[i])`

### 4.5 Coroutine/async expressions 📝

- `co name(params) { ... }` — coroutine declaration
- `async { ... }`, `await(expr)`, `yield(value)`
- `ch <- value` (send), `<-ch` (receive), `chan<T>(bufsize)` (create)
- `select { case ... }`
- `gospawn(fn(...))`, `await_all(f1, f2, ...)`

---

## 5. Memory Model, Ownership & Borrowing ✅

Karkain targets **safe by default, unsafe with `@raw`**.

### 5.1 Ownership rules
- Every value has exactly one owner at a time.
- Copying a value into a function parameter or return transfers ownership
  (move semantics) — the source binding becomes unusable.
- Escape analysis (`pkg/parser/escape.go`, Phase 49) marks variables that
  escape their scope (passed to funcs, returned, or captured by a lambda).

### 5.2 Borrowing
- `&x` — immutable borrow; multiple immutable borrows allowed, no mutation.
- `&mut x` — exclusive mutable borrow; no other borrows while active.
- The borrow checker (`pkg/sema/borrow_checker.go`, Phase 51) enforces lexical
  scoping of borrows.

### 5.3 Raw memory
- `@raw(addr)` read, `@raw(addr, val)` write — escapes safety checks, target of
  hardware/FFI access.
- `alloc<T>(n)` / `free(ptr)` — manual heap management.
- `addr`, `*T` pointer types for C interop.

### 5.4 Linear types 🟡
Values of `linear` types must be consumed exactly once — resource safety
(file handles, allocations).

---

## 6. Functions & Standard Builtins

### 6.1 Builtin callables ✅ (`pkg/codegen/lower.go`)

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

### 6.2 Runtime functions (`.kar` runtime supported) ✅
`getArgs`, `openFile`, `readLine`, `closeFile`, `createFile`, `writeToFile`,
`system`, `removeFile`, `substr`, `replaceExtension` (self-hosted compiler uses
these; runtime provided by `runtime.c` supporting the compiler itself).

> The general-purpose standard library beyond the above is still being
> formalized (Phases 60).

---

## 7. Heterogeneous Backends

### 7.1 Compiler pipeline ✅

```
.kar → lexer → parser (AST) → SSA IR → optimizer → verifier → C23 → GCC/Clang/MSVC → binary
```

- **Primary path (SSA):** `pkg/ir/ssa` — block-param CFG (no phis), variable
  memory cells, ops `{Const, BinOp, UnOp, Call, CallVoid, Br, Jmp, Ret,
  IndexGet, IndexSet, Print, RawC, OpLoad, OpStore}`.
- **Fallback path:** legacy direct AST → C (`pkg/codegen/codegen.go`) used when
  SSA lowering fails (safety net).
- Verified by `pkg/ir/ssa/verify.go`.

### 7.2 GPU kernel → WGSL/OpenCL/SPIR-V 📝

- `kernel` functions, `global_id`, `barrier` emit through
  `pkg/codegen/{wgsl,gpu,spirv}.go`.
- Host launcher auto-generation via `gpu_host.go`.
- Tensor ops emit to WGSL (`tensor_wgsl.go`).

### 7.3 Quantum → OpenQASM 3 / QIR 📝

- Bare gate syntax: `H q[0]`, `CNOT q[0], q[1]`, `Rx(θ) q[2]`
- Emitters: `pkg/codegen/{qasm,qir,openpulse}.go`
- Safety analysis: `pkg/sema/quantum.go` (no-cloning theorem, measurement
  collapse, gate-after-measurement).
- Noise simulation, error correction, distribution planner present in sema.

### 7.4 Actor distributed concurrency 📝

- `actor`/`spawn`/`receive`/`channel`/`send` (phases 16/37), runtime
  `pkg/runtime/actor_system.go`.

### 7.5 Coroutine / green-thread scheduler 🟡

- `co` declarations, `async`/`await`/`yield`, channels, `select`, `gospawn`
  (Phase 38), runtime `pkg/runtime/coroutine.go`.

---

## 8. Macros & Metaprogramming 🟡

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
| Monomorphized generics | 🟡 | `func max<T: Numeric>(a: T, b: T) -> T` |
| Generic parameters | 🟡 | `T: Constraint` |
| Trait declarations | 📝 | `trait Numeric { fn add(self, other: T) -> T; }` |
| Trait impls | 📝 | `impl Numeric for int { ... }` |
| Struct generics | 🟡 | `struct Vector<T> { ... }` |
| Kernel generics | 📝 | `kernel k<T>(...)` |

Instantiation is monomorphic (compile-time per concrete type argument).

---

## 10. Package Manager (KPM) ✅

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

### 10.2 Version syntax (semver) ✅

`1.2.3` exact · `^1.2.3` compatible · `~1.2.3` patch · `>=1.0 <2.0` range ·
`1.2.x` wildcard · `*` any.

Lock file `karkain.lock`, SHA-256 integrity checks (`pkg/pm/integrity.go`),
auth token at `~/.karkain/auth.json`. Registry endpoint:
`https://registry.karkain.dev` (override `KARKAIN_REGISTRY`).

---

## 11. C Interop ✅

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
| `run` | ✅ | Compile & execute |
| `build` | ✅ | Native executable |
| `transpile` | ✅ | Emit C (keeps `.c`) |
| `check` | ✅ | Parse + semantic validation |
| `test` | ✅ | Discover/run `*_test.kar` |
| `lsp` | 🟡 | Go-based LSP server (`pkg/lsp`) |
| `jit` | 🟡 | `pkg/jit` JIT/FFI |
| Debug mode | ✅ | `-g` emits `#line` + `-line N "file.kar"` directives |
| Verbose | ✅ | `--verbose` pipeline logging |
| Targets | 🟡 | native, wasm32-wasi |

---

## 13. Known Gaps & Non-conformances

| Area | Gap | Tracking |
|------|-----|----------|
| Self-hosted compiler | `self_host_parser_test.kar` fails; `src/compiler/*.kar` incomplete | Phase 56 |
| Enum payload variants | Enums parse but payload handling unstable | BUG-7 |
| Struct codegen | Certain codegen paths incomplete | BUG-1 |
| Option/Result match arms | Codegen edge cases | BUG-2 |
| Index assignment | Specific patterns | BUG-3 |
| GPU/quantum backends | Parsers exist; full emitters not conformance-tested | Phases 58-59 |
| Concurrency stdlib | Actor/coroutine runtimes partial | Phases 57, 63 |
| wasm32-wasi | Compile-only, no runtime execution | — |

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
for f in examples/*.kar; do ./karkain run "$f"; done
```

---

*This is a living document. Sections transition from 📝→🟡→✅ as the compiler
implements them, and conformance tests are added.*
