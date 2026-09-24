# Phase 149 FINAL REPORT — PE + Mach-O Writers, Win64 Boundary, PEB Bootstrap

Verdict: **COMPLETE (execution proven)**. PE images execute green on
windows/amd64 (7/7 goldens live on the dev host); Mach-O is structural
(PARSE + determinism, no runner exists); ELF byte-identical and still
green on Linux.

## A. What was built

- `pkg/native/pe.go`: DOS + PE sig + COFF + PE32+ (240B, correct
  64-bit offsets) + 3 sections (`.text` R-X with code+rodata, `.idata`
  R/W with IDT/ILT/IAT/Hint-Name/kernel32.dll, `.reloc` with DIR64
  blocks per 4KB page) + `ParsePE` validator (magic, machine,
  sections, entry, import + relocation directories, per-entry types).
- `pkg/native/macho.go`: header + LC_SEGMENT_64 (nsects=0) +
  LC_DYLD_INFO_ONLY (zeroed, best-effort) + LC_LOAD_DYLINKER +
  LC_MAIN + `ParseMachO` validator.
- OS-parameterized lowering (`CompileProgramForOS`): per-OS syscall
  numbers (Linux 1/60, macOS 0x2000004/0x2000001), per-OS `_start`
  tails, shared encoder/lowering for everything else.
- Win64 boundary (PE only): System V inside user functions, Win64
  (RCX,RDX,R8,R9 + 32B shadow) at three kernel32 call shapes;
  16-alignment discipline (entry AND, 16-rounded frames, 48B
  WriteFile shadow); IAT slots via movabs + call.
- Loader-independent PEB bootstrap (PE only): PEB→Ldr→module walk
  (exact "kernel32.dll" match, 64-entry bound, Int3 on exhaustion) +
  per-export table walks publishing ExitProcess/GetStdHandle/WriteFile
  into the IAT. Needed because the host loader maps minimal images
  and runs entry but never snaps the IAT (proven repeatedly:
  slots keep file content at runtime).

## B. Root causes fixed (all load-bearing, all gated)

1. **CallReg ModRM (the execution blocker):** emitted `FF D0`
   (modrm 11) = `call r64` (call the address IN the register) instead
   of `FF 10` (modrm 00) = `call m64` (call the address STORED
   there). Every import call jumped into the IAT slot bytes (fault
   RIP == slot address in every run) instead of the resolved
   address. One-byte fix + goldens (`FF 10`, `41 FF 12`, `FF 14 24`).
2. **PE loader rejections (bisected empirically):** `.idata` must be
   writable (loader writes IAT), MajorSubsystemVersion ≥ 6
   (0 fails with ERROR_BAD_EXE_FORMAT on Win11).
3. **Rebase without relocs:** the host ASLR-relocates despite no
   .reloc need being obvious (live-process proof: base 0x7FF6…);
   `.reloc` with DIR64 entries for every movabs site keeps
   DYNAMIC_BASE honest (verified entry-by-entry against opcodes).
4. **Bootstrap correctness:** 32-bit LDR offsets (→ +0x30/+0x58/+0x60),
   uppercase BaseDllName folding, PE32+ export dir at NT+136 (not
   +96), RAX-not-RDX store bug — each found by evidence (live PEB
   reads, fault offsets, register snapshots via a DebugActiveProcess
   shim) and each golden-pinned.
5. **MovRegImm32 REX.W (carried over):** same 147-class fix pattern
   re-verified for the new primitives; goldens pin canonical forms.

## C. Gates + proof

- `pkg/native` full suite green: encoder goldens (incl. all 149
  primitives), structural (ELF/PE/Mach-O), determinism, K145,
  `TestNativePE` 7/7 EXECUTED live on windows/amd64, `TestNativeMachO`
  structural everywhere, Linux execution still green.
- New CI `native-windows` job (windows-latest) runs the suite where
  PE can execute; the ubuntu legs cover ELF + structural.
- `go vet`, `go build ./...` clean. ELF images byte-identical
  pre/post (new lowering is additive + OS-gated).

## D. Boundaries (documented, NOT defects)

- Mach-O execution: BLOCKED, no runner (structural only; dyld-info
  shape awaits a real Mac; PIE/rebase opcodes are 150+ work).
- No PE delay-load/TLS/SEH/resources/codesigning; no arm64;
  console UTF-8 is byte passthrough; debuggers on native images stay
  on the C path; kcc native parity is 151; native CLI targets
  (`--target native-x86_64-windows/macos`) are the immediate
  follow-up, not this phase.
- The import table is retained (documentation + compatible hosts);
  the bootstrap is the execution path everywhere.
