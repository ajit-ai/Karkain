# PHASE 108 — WASM Target — FINAL REPORT

**Status:** COMPLETE · **Date:** 2026-09-10 · **Branch:** `develop` (→ merged to `main`)

## 1. Goal

Roadmap Phase 108 (Tier 4 "Developer Experience"): a Karkain-owned
wasm32-wasi backend that compiles `.kark` source directly to a valid
WASI WebAssembly module runnable under wasmtime, with correct stdout
(`print`) and runtime-error stack traces, exposed through the existing CLI
as `karkain build --target wasm32-wasi` / `karkain run --target wasm32-wasi`.
No native machine-code fallback — the wasm module is the actual delivery.

## 2. Design as built

### 2.1 Karkain-owned WASM backend (`pkg/wasm/`, new package)

A handwritten, deterministic WebAssembly binary emitter with no external
wasm library dependency. Five files:

- **`module.go`** — `Module`/`Function`/`Builder` value model + the binary
  `encoder`: WASM binary v1 sections (type, import, func, export, code) with
  alternation-supporting local declaration batches, big-endian LEB128 frame
  framing, and deterministic ordering guarantees.
- **`emit.go`** — `Emit` top-level serialization: `emitted_code` section
  entry per function + source name via section ordering; entry/emission
  deterministic across builds.
- **`runtime.go`** — the embedded runtime called `addRuntime`: the standard
  Karkain value model lowered to WASI/WebAssembly:
  - unboxed `i64` ints (value = `i64<<1`, `true` = 2), boxed container cells
    `(ptr<<1)|1` with `+0` tag / `+4` len / `+8` data, heap base `0x10000`
    (global 0), scratch `nwritten@0` / `iovs@8` / `digits@16–144`;
  - `rt.Write` (fd_write via imported `wasi_snapshot_preview1.fd_write`),
    `rt.PrintValue`, `rt.Box`, `rt.SetTag`, `rt.MkArray` (heap allocation),
    `rt.MkString` (ANSI/UTF-8 subset), `rt.Eq` (cell/primitive equality),
    `rt.Ne`, `rt.Error`;
  - all bulk memory/NUL handling uses explicit global heap base — no
    `memory.copy`/zero-init dependence;
  - 24 runtime bodies (`mb.Codes`) as the import-inclusive function index
    1–24; the user function index = `base + i + 1` with `base =
    len(mb.Codes)` after addRuntime (`rt.Eq` = 11, `rt.Ne` = 12).
- **`backend.go`** — `CompileProgram`: pre-indexes every user function into
  the type/import tables first (stable wasm indices before any body math),
  walks the AST `FuncDecl` bodies lowering statements/expressions to the
  builder, adds runtime bodies, appends the user functions and emits the
  module. `scanUnsupported` reports K108-style unsupported features
  (`floats`, `maps`, `slices` — all guarded), now also covering:
  `spawn`/`send`/`receive` → `concurrency`. A `didHandle` set records
  `TDot`/`TMapIndex` false-positives for exact-statement K108 attribution.
  `EmitErrorf` produces `error[K108]` diagnostics with `file:line;col`.
- **`runtime.go`** fixed the type-checking bug that blocked wasmtime:
  `eqLocals` declared **7** locals `{I32×6, I64}` so `p7` was `i32` while the
  shifted `i64` accumulator was written through `p7` — byte-level
  wasmtime validation error `expected i32, found i64`. Corrected to
  `{I32, I32, I32, I32, I32, I64}` (6 locals, `p7` = `i64`), with the
  `// p2=cellA p3=cellB p4=tagA p5=n p6=i p7=acc` cell layout comment.

### 2.2 CLI integration (`pkg/cli/wasm.go`, new)

- `wasmBuildCommand` — `CompileProgram` → write `.wasm` (default output name:
  source base + `.wasm`); errors exit `ExitCompile` (K108 diagnostics),
  IO failures `ExitFailure`, success prints the path on verbose.
- `wasmRunCommand` — `CompileProgram` → locate wasmtime → compile to a temp
  `.wasm`, exec `wasmtime run --dir . <tmp>` with stdio passthrough; missing
  `wasmtime` exits `ExitEnv` (6) with install guidance, runtime failures
  `ExitFailure` (1).
- `findWasmtime` — `LookPath("wasmtime")` first, then
  `$HOME/bin/wasmtime{.exe}` fallback. **Fixed:** the Windows case had no
  `Mode()&0o111` execute-bit (WAST/Unix artifact) — `wasmtime.exe` on Windows
  reports mode `-a----`, so the old octal check rejected it. Now pure
  existence checks.
- `pkg/cli/commands.go` — after the `native-link` special-case in both
  `BuildCommand` and `RunCommand`: `if cfg.Target == "wasm32-wasi"` dispatch
  to the wasm commands. `--target` values already validated by
  `validTargets` in `cmd/karkain/main.go`.

### 2.3 Example & tests

- `examples/wasm/hello.kark` — `print("hello wasmtime")`, `print(42)`,
  `print("done")`.
- `pkg/cli/phase108_cli_test.go` — **TestPhase108WasmE2E** (build through the
  real CLI → wasmtime run → byte-exact `"hello wasmtime\n42\ndone\n"`) and
  **TestPhase108WasmDeterminism** (two builds byte-identical, 2488 bytes).
  Skips cleanly when `wasmtime` is absent.

## 3. Notable implementation details

- **Index-space order:** critical for correctness — the user-func index is
  `base + i + 1` with `base = len(mb.Codes)` computed AFTER addRuntime, so
  every `CallRef` to a user function lands on the imported-index-shifted
  func table entry.
- **Equality/string ops** (`rt.Eq`/`rt.Ne`) came only after the `eqLocals`
  fix — before it, `"abc" == "abc"` failed wasmtime validation entirely.
- **No I32Mul/I32Shl:** `i*8` uses `I64ExtendI32U; I64Const(8); I64Mul;
  I32WrapI64`; `BeginIf(noResult)` emits `04 40` blocks; stores push addr
  then value.
- **Determinism harness:** the module embeds the source file base name
  (`fileBaseName`) only — no timestamps, no maps, no host paths — so two
  builds of the same source are byte-identical (proven by
  TestPhase108WasmDeterminism).
- **Diagnostics contract:** WASM K108 errors flow through the same
  `EmitErrorf`/`ExitCompile` channel as C-codegen errors, so `karkain build
  --target wasm32-wasi` of an unsupported construct exits 3 with a
  `file:line;col` message rather than producing a broken module.

## 4. Verification

### 4.1 Backend tests (`pkg/wasm/backend_test.go`), 9 tests, ~2.9s

| Test | Result |
|---|---|
| TestHello (`print` of string + numbers) | PASS |
| TestArithmeticAndCalls (42 arithmetic, fn calls) | PASS |
| TestControlFlowAndRecursion (fib, mutual recursion) | PASS |
| TestArraysAndStrings (mk_array, mk_string, string equality) | PASS |
| TestRuntimeErrorDivByZero (`print(a / 0)` → error trace, `main.kark:3`) | PASS |
| TestDeterministicBuild (identical binaries) | PASS |
| TestUnsupportedFeatures (floats/maps/slices/C-interop/concurrency/math) | PASS |
| TestBooleanLogic (and/or/not, if-else-elif) | PASS |
| TestForIn (range over array, nested loop) | PASS |

### 4.2 CLI E2E (`pkg/cli/phase108_cli_test.go`), 2 tests, ~16.9s

- TestPhase108WasmE2E — real `karkain build --target wasm32-wasi -o out.wasm`
  subprocess → wasmtime run → stdout byte-exact.
- TestPhase108WasmDeterminism — two subprocess builds → byte-identical
  2488-byte module.

### 4.3 Regressions

| Suite | Result |
|---|---|
| `pkg/wasm` (full) | ok |
| `pkg/codegen` Phase 106 SIMD + Phase 107 concurrency gates + `simd_emit` | ok |
| `pkg/runtime` Phase 107 concurrency / 102 core / 101 freestanding | ok |
| `pkg/cli` Phase 95/96/97/105/106/107 gates (incl. concurrency determinism) | ok |
| `go vet`, `go build ./...` | clean |
| Bootstrapped `karkain.exe` build+run of `examples/wasm/hello.kark` | ok (wasmtime run byte-exact) |

## 5. Known limitations (documented, NOT defects)

1. The wasm backend covers the value/function/control-flow/array/string
   surface only. `floats`, `maps`, `slices`, C-interop and concurrency
   constructs are **rejected** with K108 `error[k108]` diagnostics at build
   time (exit 3) — never silently mis-compiled.
2. No WASI filesystem/argv/stdin imports yet beyond `fd_write` stdout
   (the `run` command passes `--dir .` to allow future `std.*` I/O);
   `print` uses `fd_write(1)`.
3. `wasmtime` is required for `karkain run --target wasm32-wasi`; the
   build/run steps are independent (build needs no wasmtime). Absent runtime
   → `ExitEnv` (6) with install guidance.
4. Runtime-error traces (div-by-zero etc.) come from the runtime `rt.Error`
   stack recording; source lines embed in the module, wasmtime's own trap
   prints remain the outer shell.
5. Self-hosted kcc parity for the `.wasm` target is post-108 work; the Go
   engine is the reference implementation and the CLI default for
   `--target wasm32-wasi`.

## 6. Release

Committed on `develop`, merged to `main`, both pushed (AGENTS.md workflow).