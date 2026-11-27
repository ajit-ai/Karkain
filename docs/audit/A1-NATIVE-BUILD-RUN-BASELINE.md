# A1 baseline — kcc owns native build/run

Status: **COMPLETE** (baseline below retained verbatim as the pre-change
record; evidence in §5). Unnumbered named slice on the v1.2.x line. Follows Step M
(`docs/audit/STEP-M-COMPILE-RUN-MEASUREMENT.md`), which measured the warm
default C23 path at 1975 ms median versus 191 ms for native.

## 1. Measured current state

`kcc` is a real compiler for user programs, but it is not yet a **toolchain**:

- `kcc build <file.kark> --target native-x86_64-windows` computes and links
  every byte of a PE itself, then **prints the image as hex**
  (`src/compiler/main.kark:680-687`, `natToHex` at `native_emit.kark:928`).
  It does not write an executable.
- `kcc run <file.kark>` ignores `--target` and always calls `runFile`
  (`main.kark:60-66`), which emits C23 to `karkain_temp.c`, invokes `gcc` and
  runs the result (`main.kark:724-777`).
- The Go CLI therefore performs the remaining work in `pkg/cli/kcc_native.go`:
  require the provenance line, hex-decode kcc's output, validate the container
  with the Go oracle's parser, `os.WriteFile` the bytes, then launch the image.
- Reason kcc cannot write the bytes itself: `writeToFile` uses `fputs`
  (`src/compiler/codegen.kark:926`), and a Karkain string is NUL-terminated.
  A PE/ELF/Mach-O contains NUL bytes, so text writing truncates the image.

## 2. Why this is the next slice

Step M's shims showed the native path invokes **zero** gcc and **zero** go
processes, but that measurement still went through the Go CLI launcher
(staging, hex decode, validation, file write, process launch). A1 removes that
launcher from the user's compile/run path, so "kcc compiles and runs a native
program" becomes a statement about the compiler rather than about Go glue.

## 3. Scope

1. A byte-level writer `writeBytes(path, array)` in **both** engines:
   - Go: `pkg/sema/resolve.go` builtin table + arity, `pkg/codegen/codegen.go`
     preamble helper and `genExpr` dispatch.
   - kcc: `src/compiler/sema.kark` builtin/arity, `src/compiler/checker.kark`
     checker table, `src/compiler/codegen.kark` preamble helper and dispatch.
   Both are required: stage-1 bootstrap emits `src/compiler/*.kark` through
   the Go front end, so a kcc-only builtin breaks the compiler's own build.
2. kcc `build` writes the image to an explicit output path when asked, while
   **keeping the provenance and hex output** that frozen 151D/A1/Phase-B gates
   parse. No frozen gate is rewritten.
3. kcc `run` accepts `--target native-*` and executes the image it emitted.

## 4. Boundaries

- Only the three C-free x86-64 targets; no arm64, no WASM changes.
- No change to the restricted native language subset — unsupported programs
  are still refused by name with `error[K145]` and no image.
- No full-language native path, no bootstrap proof, no C bootstrap removal.
- The CLI's legacy hex path stays intact for backward compatibility; A1 adds
  a direct path rather than deleting the old one.
- KIR pin must be re-pinned only from a CI-measured `text=N verify=N`.

## 5. Evidence (post-change)

- Gate `pkg/cli/phase_a1_native_own_test.go`, `TestPhaseA1_`: unfixed tree
  failed both tests — `build` wrote no file (only the frozen hex output) and
  `run` printed `Compiling: gcc ...` with a non-empty shim log. Fixed tree:
  both PASS (42.264 s).
- `build` writes a real PE via `writeBytes`, validated with the Go oracle's
  `native.ParsePE` and **executed** with exact stdout `hello\n`.
- `run --target native-x86_64-windows` prints `hello\n` exactly and exits 0.
- Both native paths produce an **empty** gcc/go shim log; the legacy
  `--engine go` path, with the same shims, produces a **non-empty** log — the
  positive control that makes the empty log evidence.
- Mutation: disabling the `outPath` write reproduced the pre-fix failure;
  `src/compiler/main.kark` was restored byte-identically (SHA-256
  `2ED8A6015B6354BD4F04A3BF5FAADDCBB5C6CA2345ABFBE0E808975C2CC39152`) and
  the gate re-ran green.
- Regressions: `TestPhase151D_`, `TestPhase151A9C_`, `TestPhaseV126_`,
  `TestPhase151A9G_` PASS (296.921 s); `go vet` on cli/sema/codegen and
  `go build ./...` clean.
- A local stale gitignored `karkain.exe` initially made other gates report
  `writeBytes` undefined; rebuilding that ignored binary from the working
  tree reproduced a fresh-checkout state. No tracked file was affected.
- CI step *Run A1 native build/run ownership gate* wired in the same commit.
- KIR pin not predicted locally: CI must re-measure `text=N verify=N`.
