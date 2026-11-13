# Karkain 1.2.0 (Stable) — Release Notes

**Status:** Stable release line (successor to 1.1.0, which continues as a
historical line; 1.0.x remains the LTS line). Minor bump per the SemVer policy
(`docs/source/development/semver-policy.rst`): the 1.2.0 surface is additive,
with one behavioural change — the C-free native targets are now produced by the
**self-hosted** compiler rather than by the Go backend.

**Release cut is owner-only** (annotated tag `v1.2.0` + the CI release job
publishing the 13-archive set with `checksums.txt`). This document plus the
qualification report prepare everything up to the ceremony.

## What changed since 1.1.0

### Native Independence (the theme)

The v1.2.0 release makes **Sovereignty I — Off C, Off Go** real. The three
C-free native targets are emitted by the **self-hosted** compiler:

```sh
karkain build --target native-x86_64-linux   main.kark   # a Linux ELF
karkain build --target native-x86_64-windows main.kark   # a Windows PE
karkain build --target native-x86_64-macos   main.kark   # a Mach-O PIE
```

No `gcc` and no `go` is reachable on `PATH` for that build. The qualification
gate installs PATH shims for both and asserts the native build's log is
**empty** — and, critically, that the legacy build path's log is **not** empty,
which is what makes the empty log evidence rather than a shim that never fired.

### The native value model

The self-hosted engine now lowers the language's real value model natively:
arrays, the heap arena, string operations (concatenation, records, maps),
`push`, and float64. Each construct is gated, and every construct outside the
qualified subset is **refused by name** with a diagnostic and no image — never
mis-lowered.

### The native milestones

Each milestone has its own gate in `pkg/cli/`:

| milestone | what it proves |
|-----------|----------------|
| NAT-CALL | user function calls, arguments and return values |
| NAT-IF | conditional branches |
| NAT-QUAL | native independence (the PATH-shim evidence) |
| NAT-LOOP | loops, `break`, `continue` |
| NAT-IDX | array indexing and the bounds diagnostic |
| NAT-FRAME | the frame layout matches the Go oracle's |
| NAT-STR-STAGE | the string staging area, reserved program-wide |
| NAT-RODATA | the reporter's read-only data matches the oracle's interning |
| NAT-REG | register allocation (opt-in) |

### Register allocation (NAT-REG)

Int locals can now live in callee-saved registers (R12–R15) instead of frame
slots. The allocator is a port of the oracle's Phase-150C allocator: the same
allocatable set, the same engagement gate (control flow **and** at least five
simultaneously-live int locals), the same linear scan over live ranges, and
the same no-split rule — a local that cannot get a register keeps its frame
slot, so **the frame does not shrink**.

The switch is `KARKAIN_NATIVE_REGALLOC=1` and the **default is the stack-based
path**, which is the qualified implementation this repository ships. The
register path becomes the default only after it has passed the complete
qualification suite.

### The compiler's own encoders

The self-hosted engine now contains its own machine-code encoder and all three
container writers (ELF, PE, Mach-O), so a native image is emitted by the
compiler that Karkain itself is written in.

## Compatibility

Every 1.1.0 program builds and runs identically. The surface change is
strictly additive.

## Known limitations

These are **documented, not defects**, and none is a wrong result, a
memory-safety defect, an invalid executable or an incorrect runtime
diagnostic:

1. **The reporter's rodata** — one 39-byte ELF layout difference in a single
   fixture (a program that binds a string literal *after* an index site). The
   diagnostic's **text** is identical in both engines. Affects binary layout
   only.
2. **Field assignment** — 25 bytes of code difference, because the oracle
   stages the `(pointer, length)` pair through the string staging area and the
   self-hosted driver does not. Affects binary layout only.
3. **The oracle's name-table inference** over-reserves the staging area for a
   program with no string operation. The driver follows the emission instead of
   copying the over-approximation.
4. **String equality and slicing** are refused by name (capability).
5. **The arena bound is compile-time**, so an allocation inside a loop is
   refused by name (capability).
6. **Maps, `for-in` and float statements** in the driver are refused by name.
7. **Non-integer array element types** are refused by name.
8. **Register allocation is opt-in** (binary layout only).
9. **macOS execution is unproven** — there is no macOS runner in this
   repository. Every Mach-O claim is structural; a Mach-O image is emitted and
   validated, not executed.

## Carried to v1.3.0

Closure conditions **2** (`stdlib_v2` with no C compiler), **3** (one `std.net`
program with no C compiler), **5** (`stage2 == stage3`) and **6** (no Go
toolchain for that self-bootstrap closure) were carried to v1.3.0 as **UNMET**
by owner-authorized amendments (2026-10-10 and 2026-10-29). The condition text
is untouched and is recorded verbatim in `docs/release/v1.2.0-CHECKLIST.md`.

The effective v1.2.0 gate is therefore conditions **1, 4, 7 and 8**.

## Verifying a release artifact

Every archive is published with a `checksums.txt` (SHA-256). Verify before
running:

```sh
sha256sum -c checksums.txt
```

and confirm the shipped binary identifies the release:

```sh
./karkain --version   # Karkain Compiler v1.2.0 (..., Stable Build)
```

## Where the evidence lives

The authoritative qualification report is
`docs/audit/KARKAIN-V1.2.0-GA-QUALIFICATION.md`. The scope freeze and the
closure conditions are in `docs/release/v1.2.0-CHECKLIST.md`.
