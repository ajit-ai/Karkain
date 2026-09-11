# Karkain Language Foundation

This document summarizes the core language capabilities validated by
Phase 112 ("Programming Language Foundation & Developer Readiness"),
with a focus on features that make Karkain a practical, general-purpose
language.

## Primitive Types

| Kind          | Examples                     | Engine parity |
|---------------|------------------------------|---------------|
| `int`         | `42`, `-7`, `0`              | Yes (Go, kcc) |
| `float64`     | `3.14`, `-0.5`               | Yes           |
| `bool`        | `true`, `false`              | Yes           |
| `string`      | `"hello"`, `"with \"escapes\""` | Yes        |

Phase 112 additions:
- **`const`** keyword: compile-time constant declarations (function-scoped,
  no reassignment) — Go engine + kcc parity (`K113` on reassignment).
- **`float()`** conversion: widens int, parses string literal, passes
  float through — Go engine + kcc parity (`karkain_float` C helper).
- **Nested block comments** (`/* outer /* inner */ still outer */`):
  Go-engine only; kcc does not nest (documented divergence).

## Control Flow

| Construct                | Status |
|--------------------------|--------|
| `if` / `else`            | Yes    |
| `while(cond)`            | Yes    |
| `for (init; cond; step)` | Yes    |
| `for item in array`      | Yes    |
| `match` expression       | Yes    |
| `return`                 | Yes    |
| `break` / `continue`     | Yes    |

## Functions

- Untyped parameters: `func add(a, b) { return a + b }`
- Recursive calls: yes (including mutual recursion via forward declarations)
- Closures: syntactically accepted; codegen is deferred (documented
  Phase 101 boundary)

## Collections

| Type     | Syntax                          | Notes                      |
|----------|----------------------------------|----------------------------|
| Array    | `[1, 2, 3]`, `make_array()`     | Length-prefixed `NativeArray` |
| Map      | `{}`                             | Key-value via runtime map  |
| Struct   | `Counter{ value: 0 }`           | Map-based instances        |
| Option   | implicit (absent = nil)          | Phase 72+                  |
| Result   | implicit (error = runtime abort)| Phase 72+                  |

## Modules

- `import std.string`, `import std.collections`, etc.
- `public` export modifier (Phase 103): private cross-module calls
  rejected with precise diagnostics.
- Qualified calls: `math.twice(21)` resolves against the imported
  module's export set.

## Runtime Error Model (Phase 100)

- Division by zero, array/string index out of bounds:
  `runtime error: <kind> at <file>:<line>`
- Stack traces on both engines (Phase 101):
  `inner:func/file:line` frames.

## Debugging & Profiling

| Command             | Engine | Scope |
|----------------------|--------|-------|
| `karkain prof`       | Go only | Aggregated per-function timing, call graph, folded stacks |
| `karkain debug`      | Go only | Function enter/leave trace on stderr (opt-in) |
| `-g` / `--debug` flag | Both  | DWARF symbols + `#line` directives |

Phase 112 addition: **`karkain debug <file>`** compiles and runs with
`#define KARKAIN_TRACE 1` emitting `TRACE enter/leave` lines via the
existing `karkain_frame_enter`/`leave` runtime, without modifying any
emission sites. kcc tracing is a documented boundary (Phase 112).

## Testing

- `karkain test` runs all `*_test.kark` files containing `test_*`
  functions (KTF model, both engines).
- `assert()`, `assert_eq()`, `assert_ne()` builtins abort on failure.
- `std.testing` module (Phase 112): `test_expect_eq`, `test_expect_true`,
  `test_pass_count`, `test_all_pass`, `test_summary` — pure Karkain,
  no new builtins, both engines.

## Cross-Compilation (Phase 111)

- `--target <triple>`: `x86_64-windows`, `x86_64-linux`,
  `aarch64-linux`, `wasm32-wasi`
- Same-machine targets build via host probe; cross targets via
  triple-prefixed cross-gcc/clang, refusing cross-run explicitly.

## Language Limitations (documented, not defects)

- Closures/`fn` codegen is syntactically accepted but not lowered
  (documented Phase 101 boundary).
- No user-defined types beyond struct/record idiom and enum.
- No `float()` casts from user-defined types; only `int`/`string`/`float`.
- `println` with multiple args prints only the first.
- Nested block comments are Go-engine-only.
