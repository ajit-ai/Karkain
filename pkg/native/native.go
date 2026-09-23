// Package native implements Karkain's owned x86-64 machine-code backend,
// Phase 145 (Native Backend v1).
//
// It emits position-dependent, statically-linked, libc-free executables:
// hand-encoded x86-64 (emit.go), a minimal ELF64 writer (elf.go), and a
// straight-line program lowering (program.go) over unboxed values with
// stack slots. No C compiler, linker, or C library participates anywhere.
//
// v1 boundaries (documented, NOT defects): straight-line code plus calls
// only (no source-level branches/loops yet — the encoder already carries
// test/jnz/jmp for the next slice); ints, strings and calls; Linux x86-64
// ELF only (PE/Mach-O unwired); no register allocator (stack slots); write
// and exit are the only syscalls; no DWARF (Phase 140 stays on the C path).
package native
