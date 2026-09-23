# Phase 145 BASELINE — Native Backend v1 (C-free)

Date: 2026-09-23. Target: SOVEREIGNTY third step (after 144).

## 1. What exists (and what it is not)

- `pkg/codegen/native*.go`, `object.go`, `linker.go`, `symbols.go`,
  `relocation.go`: Karkain-owned object MODEL (sections/symbols/
  relocations/KOBJ) + linker + DWARF — but NO container writer
  (ELF/PE) and NO machine-code emitter. The "NativeGenerator" emits
  C11/LLVM-IR TEXT, still compiled by gcc.
- `runtime/freestanding` + `runtime/core`: libc-free C runtime
  (arena, platform, Value, strings, arrays) — C sources, still
  gcc-compiled. Proves the runtime CAN live without libc; not yet
  without a C compiler.
- `pkg/wasm`: the template — a handwritten binary emitter with golden
  byte tests + wasmtime execution. Phase 145 follows the same shape
  for x86-64.
- No x86-64 encoder, no ELF/Mach-O/PE writer, no register allocator
  exist anywhere. The Value model (boxed i64) does not map to machine
  registers without an allocator — so v1 works on UNBOXED values
  (ints/strings/calls) with stack slots, full Value support later.

## 2. Slice plan (as executed)

- **145A** (`pkg/native/emit.go`): x86-64 encoder — mov/add/sub/imul/
  push/pop/cmp/near-call/near-jmp/ret/syscall + ModRM/SIB/REX/rel32 —
  golden byte pins, no execution needed. Straight-line programs only
  (no branches yet); stack slots, no register allocator.
- **145B** (`pkg/native/elf.go` + `program.go`): static ELF64-LE writer
  (one PT_LOAD, _start entry, .text+.rodata, no INTERP/libc) + lowering
  (int arithmetic, string literals, user calls, print via raw
  write/exit). Execution E2E where GOOS=linux (skip elsewhere).
- **145C**: gates, docs, regressions, commit→merge→push. No CLI flag in
  145 (package API + tests, like pkg/wasm's own suite; CLI follows).

## 3. Non-goals (documented, NOT defects)

- Branches/loops, Windows PE/Mach-O, Value-model ops, register
  allocation, `-lm/-lgmp` replacement beyond unused, debugging info —
  all future slices. libcsyscall surface is write/exit only.
