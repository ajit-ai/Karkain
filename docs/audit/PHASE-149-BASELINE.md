# Phase 149 BASELINE — PE + Mach-O Writers

Date: 2026-09-24. Target: INDEPENDENCE C-front third step (after 148).

## 1. Starting position

- 148 proved the shape: one encoder + one lowering, ELF container,
  Linux run matrix, CLI flag, execution goldens. K145 surface covers
  straight-line + calls ABI + control flow; `pkg/native` suite green
  with execution on Linux CI.
- Missing: any non-ELF container. PE and Mach-O differ asymmetrically:
  - **Mach-O (x86-64 macOS)** reuses encoder + lowering almost
    entirely — raw `syscall` works under dyld, only the *numbers*
    differ (write `0x2000004`, exit `0x2000001`) plus the container.
    Needs a dyld-hosted image (LC_LOAD_DYLINKER + LC_MAIN).
  - **PE (Windows x86-64)** cannot use raw syscalls (no stable user
    ABI) — exit/output go through kernel32 imports (`ExitProcess`,
    `GetStdHandle`, `WriteFile`) with the Win64 convention
    (RCX,RDX,R8,R9 + 32-byte caller shadow). v1 keeps System V
    *inside* user functions and speaks Win64 only at three
    kernel32 boundary sequences (no lowering fork).
- No Intel-mac runner exists (GitHub macOS legs are arm64): Mach-O
  execution is structural-only + honest "unproven" note. PE executes
  on windows/amd64 — including the dev host, which proves it locally.

## 2. Baseline-holds (any red stops the phase)

- ELF output byte-identical pre/post (OS parameterization must not
  move one Linux byte — differential check on the 147/148 corpus).
- Every Stable row; 145/147/148 gates green; C path untouched.
- No K145 surface change (same language surface on all three OSs —
  any OS-specific restriction is a loud diagnostic naming the OS).

## 3. Promotes (only bucket movements)

- `native-x86_64-windows` (PE) and `native-x86_64-macos` (Mach-O) join
  the closed target set with `karkain target` rows, `--help` line,
  per-OS build/run matrix: build from anywhere (pure-Go emission),
  run only on matching hosts (linux/amd64, windows/amd64,
  darwin/amd64), elsewhere the build-only refusal (exit 6).
- PE execution goldens join the mandatory suite (Windows legs);
  Mach-O structural goldens join everywhere.

## 4. Slice plan (as executed)

- **149A (Mach-O + OS param):** `pkg/native/macho.go` (header +
  LC_SEGMENT_64 nsects=0 + zeroed LC_DYLD_INFO_ONLY + LC_LOAD_DYLINKER
  + LC_MAIN; base `0x100000000`) + `ParseMachO` validator;
  `CompileProgramForOS(prog, os)` (`linux` default preserved by
  `CompileProgram`); per-OS syscall numbers; per-OS `_start` tail;
  Mach-O structural tests (run everywhere; execution skips without
  darwin/amd64, future-proofed like the old Linux skip).
- **149B (PE + Win64 boundary):** `pkg/native/pe.go` (DOS + PE sig +
  COFF + PE32+ + 2 sections: `.text` R-X with code+rodata, `.idata`
  R-- with IDT/ILT/IAT/Hint-Name/kernel32.dll; base `0x140000000`,
  file-align `0x200`, TimeDateStamp 0 for determinism) + `ParsePE`
  validator; IAT slots resolved through the existing imm64-patch
  machinery (`movabs` + `call rax` — no RIP-relative encoding needed);
  print helpers + `_start` tail branch per OS (Win64 sequences with
  caller shadow + stack scratch; user functions stay System V).
- **149C (CLI):** `validTargets` + listing + help + `SelectedTarget`
  (host, non-triple); `cfreeBuildCommand` default extensions per OS
  (`.elf`/`.exe`/no-ext… decided at implementation: self-describing
  wins); `cfreeRunCommand` per-OS host check; kcc auto-route unchanged
  (exotic → Go); incremental refusal extended to all native targets.
- **149D:** gates (new CI `windows-latest` job runs the full
  `pkg/native` suite: Linux images skip, PE executes, Mach-O
  structural), docs rows, regressions, final report, merge/push.

## 5. Non-goals (documented, NOT defects)

- Mach-O execution proof (no runner — structural + lore-based dyld
  commands, flagged for a real Mac); arm64 anything; PE delay-load,
  TLS, SEH, resources/version info; codesigning; debuggers on native;
  `run` on mismatched hosts (still refused); kcc parity (151);
  console codepage/UTF-8 policy beyond byte passthrough (WriteFile
  passes bytes through; UTF-8 validated at the Karkain level already).

## 6. Gate

- PE goldens execute green on windows/amd64 (dev host + CI Windows
  job); Mach-O structural green everywhere; ELF byte-identical +
  execution still green on Linux; `pkg/cli` 149 gate (three-target
  matrix: build magic per OS, run matrix refusals/passthrough,
  listing); full Stable battery green; three consecutive default-CI
  greens before the 150 baseline (house rule continues).
