# Phase 145 Quarantine — Native Execution P1 (RESOLVED by Phase 147)

## Status

~~Quarantined, unresolved.~~ **CLOSED 2026-09-24 by Phase 147.**
Root causes established by CI-captured evidence and fixed; execution
green in default CI. This document is retained as the historical record;
the live state is `docs/audit/PHASE-147-FINAL-REPORT.md`.

Resolution summary: (1) `MovRegImm32` emitted a bogus `REX.W + B8+rd`
with a 32-bit immediate — per the Intel SDM that decodes as
`mov r64, imm64`, so the CPU consumed the next 4 bytes and every use
site desynchronized the stream (the immediate SIGSEGV). Fixed to the
correct prefix-less `B8+rd io` (plus `REX.B` for r8–r15); the two
goldens that pinned the buggy bytes are corrected.
(2) Binary operands pushed rax, moving rsp and shifting frame-relative
slots — `add(20,22)` read slot a twice (40). Lowering now stages
through depth-indexed scratch slots (rsp never moves; 64-deep K145
guard). (3) `print` wrote no trailing newline. Newline tails added to
both helpers. Proven: full execution suite green on Linux CI
(`TestNativeHello/Calls`, all six bisect cases, `strace exit=0`).
Quarantine flag `KARKAIN_NATIVE_EXEC` removed from
`runNativeCode`; the `native-evidence` CI job stays informational.

Original record follows unchanged.

## Evidence (bounded troubleshooting round, verbatim)

- `TestNativeHello` reports segmentation fault (0.09s, empty output).
- `TestNativeCalls` reports trace/breakpoint trap (0.07s, empty output).
- All six minimal bisect execution cases (`empty`, `retcode`, `int42`,
  `str`, `arith`, `call`) fail immediately.
- The `empty` case (`_start → call → sub/xor/add/ret → exit(0)`) contains
  no identified faultable instruction.
- Encoder goldens, structural link validation, K145 validation,
  determinism, documentation, and all Phase-114–144 gates are green.

## What this establishes (and does not)

- Established: execution of generated native binaries dies instantly and
  deterministically on the CI runners with no output.
- NOT established: the root cause. The loader, harness, and environment
  are NOT exonerated — each was argued from available evidence, but the
  decisive experiments (bisect stdout, strace/readelf on a produced
  binary) remain unobtained. Nothing here absolves any layer.

## Quarantine mechanism

- `runNativeCode` (pkg/native/program_test.go) skips unless
  `KARKAIN_NATIVE_EXEC=1` is set (after the existing Linux/amd64 skip).
- No execution test was deleted, weakened, rewritten, or converted:
  `TestNativeHello`, `TestNativeCalls`, and all six bisect cases remain
  genuine execution tests and run normally under the flag.
- Encoder, structural, determinism, K145, compiler, linker, and
  documentation gates are unaffected and stay mandatory (verified below).
- No compiler/encoder/linker/native product code was modified for this
  quarantine. The diagnostic commits `81cf175` (bisect subtests) and
  `a5e21cd` (per-case `if: always()` CI steps) are preserved as-is.

## Reopening

Set `KARKAIN_NATIVE_EXEC=1` on a Linux x86-64 host and run
`go test ./pkg/native/ -run 'TestNativeBisect' -v`. If execution still
dies, obtain the step stdout plus `readelf -h` / `strace -f` of a
produced binary — that evidence, not further static review, is what can
close this P1.

## Validation (this change)

- Default (`KARKAIN_NATIVE_EXEC` unset): full `pkg/native` suite green
  (execution paths skip with the quarantine message; everything else
  runs).
- Flag set (Windows host): platform skip still applies first — flag does
  not enable anything the platform forbids, and does not disable any
  mandatory gate.
- Linux CI behavior: execution tests will run (and, until the P1 is
  fixed, fail) only where the flag is set; default CI stays green
  because the flag is unset there.
