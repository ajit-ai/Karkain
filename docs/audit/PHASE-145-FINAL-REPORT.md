# Phase 145 FINAL REPORT — Native Backend v1 (C-free)

Verdict: **COMPLETE**. The first machine code Karkain has ever emitted
without a C compiler: hand-encoded x86-64, statically linked ELF64,
zero libc, executed on Linux (CI leg).

## A. Encoder (145A, `pkg/native/emit.go`)

- mov (reg/reg/imm32/imm64/stack/disp8/disp32/SIB/mem8), add/sub
  (reg/reg/imm), imul, push/pop, xor-zero, neg, dec, cqo, div, test,
  near call/jmp/jnz/jns (label + rel32 backpatch), ret, syscall.
- Golden byte pins for every primitive (hand-verified against the Intel
  SDM). One self-caught correction during development: rel32 resolves
  against the displacement END (RIP of the next instruction), and the
  test initially encoded it wrong — implementation was right.
- `imm64Patch`/`PatchImm64` support was designed then removed from the
  final shape: address resolution lives in the Builder (single owner),
  keeping the Emitter minimal.

## B. Linker + lowering (145B, `elf.go` + `program.go`)

- `Link`: ELF64-LE, ET_EXEC, one PT_LOAD (R+X) mapping the whole file at
  0x400000, entry `_start`, .text+.rodata, no section headers, no
  INTERP — plus a structural `Parse` validator for non-Linux hosts.
- Lowering: ≤6 params (System V order), let/call/print/return,
  int arithmetic + unary minus, string literals + variables, user calls
  (spilled-arg convention proven sound: eval scratch is rax/rcx/rdx
  only), int/string slot-kind tracking (cross-kind flows are K145, never
  miscompiles), mid-body returns jump to the epilogue, missing return
  yields 0. `main`'s value becomes the exit code.
- `print_int` (signed, division loop, zero-safe with no special case)
  and `print_str` over raw write/exit syscalls only.
- Every out-of-surface construct is a K145 diagnostic in Karkain words
  (never Go type names); K145 added to `explain`.

## C. Gates + CI + regressions

- `pkg/native` (new, 7 tests): 3 encoder golden suites, structural
  link validation, determinism, 7-case K145 table, and execution goldens
  (`hello native/14/12`, `42/126/-7/10^12`) that run on Linux x86-64 and
  skip elsewhere. Local: green minus execution (Windows host); Linux
  execution is the CI leg.
- CI: `pkg/native` step. `go build ./...`, `go vet` clean. Zero
  regression surface by construction (new package + one explain row;
  no existing code touched).

## D. Boundaries (documented, NOT defects)

- No branches/loops/calls-with-strings in v1 programs; no PE/Mach-O;
  no Value-model ops or register allocation (stack slots); write/exit
  syscalls only; no DWARF (debugging stays on the C path); no CLI flag
  yet (package API + tests, CLI follows).
- `main` returning a string, >6 params, and duplicate functions are
  K145 by design on this surface.
