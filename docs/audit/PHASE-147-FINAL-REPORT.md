# Phase 147 FINAL REPORT — Native Execution P1 Resolution

Verdict: **COMPLETE**. Generated native binaries run on Linux CI:
`TestNativeHello`, `TestNativeCalls`, all six `TestNativeBisect` cases
and `strace exit=0` green; quarantine lifted; gate met.

## A. Evidence (147A — the closer the quarantine doc demanded)

- New `TestNativeEvidenceDump` (`pkg/native/evidence_test.go`, env-gated
  `KARKAIN_NATIVE_DUMP`, bytes-only so it runs everywhere) + a Linux-only
  informational CI job `native-evidence` (`continue-on-error`, never red)
  capturing `readelf -h -l` and `strace -f` into the step log, plus a
  147C prover step running the genuine suite under the quarantine flag.
- Decisive capture (run 35951487781): `execve(...) = 0` (loader
  exonerated — ELF structure clean, entry `0x400112` inside the single
  `LOAD`), then immediate `SIGSEGV si_code=SI_KERNEL si_addr=NULL` with
  zero userspace progress. Call-chain forensics on the deterministic
  291-byte image (identical bytes reproduced locally): entry `call -17`
  lands exactly on `main`'s prologue — rel32/backpatch exonerated.

## B. Root cause 1 (the P1): REX.W misencoding in MovRegImm32

- `MovRegImm32` emitted `REX.W + B8+rd` with a 32-bit immediate. Per the
  Intel SDM vol. 2 (MOV), `REX.W + B8+rd` **is** `mov r64, imm64`
  (10 bytes) — no such 6-byte form exists. The CPU consumed the next
  4 bytes as immediate at every use site (syscall numbers, fds, loop
  constants, the `_start` exit sequence), desynchronizing the stream
  from the first use. The tail decoded as `movabs` with a truncated
  immediate swallowing the final `syscall`.
- The Phase-145 "hand-verified" comment ("sign-extended") was wrong, and
  the two goldens pinned the buggy bytes (`0x48,0xB8` / `0x49,0xB8`).
- Fix (`pkg/native/emit.go`): prefix-less `B8+rd io` (zero-extends —
  exactly the small-nonneg-constant semantics every caller needs),
  `REX.B` (`0x41`) only for r8–r15. Goldens corrected
  (`0xB8,…` / `0x41,0xB8,…`). `imm64Patch` (genuine rodata addresses)
  verified correct, untouched.
- Effect on CI: `empty`/`retcode` PASS + `strace exit=0` (run
  35952787008) — the crash class fixed; print/call cases then failed as
  *output mismatches*, which bisected the second defect.

## C. Root cause 2: push-shifted slots + missing newlines

- `emitBinary` pushed rax for the left operand, moving rsp: the right
  operand's frame-relative slot load then re-read the left's slot, so
  `add(20,22)` computed `20+20=40` (and `mul3(40)=120` downstream).
  Immediates-only expressions were unaffected — exactly matching the
  observed `14`/`-7`/`10^12` correct alongside `40`/`120` wrong.
- Fix (`pkg/native/program.go`): depth-indexed scratch stack
  (`binTemp`, 64 levels, rsp never moves during eval; deeper nesting is
  a loud K145 error), verified structurally in the linked image
  (`48 89 44 24 10 … 48 8B 44 24 08 … 48 89 C1 … 48 8B 44 24 10 …
  48 01 C8`).
- `print` wrote no trailing newline (C-backend contract is `"42\n"`):
  newline tails added to `print_int`/`print_str` via interned `"\n"`.
- Effect on CI (run 35953665647): **entire execution suite PASS** —
  Hello, Calls, all six bisect cases; `strace exit=0`.

## D. Un-quarantine + gates

- `KARKAIN_NATIVE_EXEC` gate removed from `runNativeCode`
  (`pkg/native/program_test.go`); platform skip retained.
  Quarantine doc marked closed (kept as history).
- Baseline §7 gate: default-CI `pkg/native` green with execution
  running (this report's runs: prover-all-green 35953665647, default
  suite green in the same run's `test` job), plus two confirmation
  re-runs to reach three consecutive greens before the 148 baseline.
- `native-evidence` CI job stays informational (dump + readelf +
  strace + prover) — the permanent forensics path for the native track.
- Regressions at close: `go vet ./pkg/native/`, full `pkg/native`
  suite (Windows: execution skips by platform), `go build ./...`,
  full CI `test` job green on the un-quarantine run.

## E. Boundaries (unchanged — 148+ territory)

- No new lowering, no CLI flag, no PE/Mach-O, no K145 changes beyond
  the depth guard. Straight-line surface only. kcc native parity is 151.
