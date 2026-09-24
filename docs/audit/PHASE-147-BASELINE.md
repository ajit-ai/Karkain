# Phase 147 BASELINE — Native Execution P1 Resolution

Date: 2026-09-24. Target: INDEPENDENCE C-front opener (after 145 quarantine).

## 1. Starting position

- `pkg/native/` ships a trusted text pipeline: x86-64 encoder golden pins
  (`emit.go`/`emit_test.go`), static ELF64 writer + structural `Parse`
  validator (`elf.go`), lowering for straight-line ints/strings/calls
  (`program.go`), determinism + 7-case K145 table (`program_test.go`).
- Execution dies instantly and deterministically on CI runners:
  `TestNativeHello` segfault, `TestNativeCalls` trap, all six
  `TestNativeBisect` cases fail with zero output
  (`docs/audit/PHASE-145-QUARANTINE.md`). Root cause NOT established.
- Quarantine: `runNativeCode` skips unless `KARKAIN_NATIVE_EXEC=1`
  (`pkg/native/program_test.go:51`); default CI stays green because the
  flag is unset. Bisect steps run per-case with `if: always()` so outcomes
  stay visible (`ci.yml:184-201`).
- The quarantine doc explicitly bounds the closer: loader, harness and
  environment are NOT exonerated, and further static review cannot close
  the P1 — only `readelf`/`strace` evidence from a produced binary can.
- Dev host is Windows: execution evidence can only come from Linux
  (CI legs). This phase is therefore evidence-driven by construction.

## 2. Baseline-holds (must stay green; any red stops the phase)

- Every Stable Core row (Part 1 of the independence plan): full
  `pkg/cli` gates, conformance, probes, `pkg/codegen`, `pkg/sema`,
  `pkg/parser`, `go vet`, `go build ./...`.
- Phase 145's non-execution gates stay mandatory everywhere: encoder
  goldens, structural link validation, determinism, K145 table,
  documentation gates (`pkg/native` default suite green).
- No product-code change lands without naming the indicted layer from
  captured evidence (loader vs entry vs first-syscall).

## 3. Promotes (the only bucket movements this phase)

- `pkg/native` execution tests (`TestNativeHello`, `TestNativeCalls`,
  all six `TestNativeBisect` cases) move from quarantined-skip to
  mandatory green in default CI.
- The `KARKAIN_NATIVE_EXEC` gate and the quarantine doc move from
  active to closed (doc marked, flag removed, bisect `if: always()`
  diagnostic steps kept as permanent regression coverage).

## 4. Baseline-after

- "Native binary runs on Linux CI" is a fact, not a quarantine note.
- Phases 148–152 unblock in order (CLI flag → surface → PE/Mach-O →
  Value/regalloc → kcc parity → stdlib + no-C closure).

## 5. Slice plan (as executed)

- **147A (evidence):** `TestNativeEvidenceDump` (env-gated,
  `KARKAIN_NATIVE_DUMP=<dir>`; compiles the `empty` bisect program via
  the existing `CompileProgram` entry and writes an executable image —
  bytes only, runs on every platform) + a Linux-only CI job
  `native-evidence` (`continue-on-error: true`, never red) that dumps
  the image and captures `readelf -h -l` plus
  `strace -f -e trace=process,write,exit_group,memory` into the step
  log. Informational only: it cannot fail the build.
- **147B (diagnosis):** read the captured evidence; the trap address
  decides the layer — loader (bad ELF/PHDR/entry), entry (stack
  alignment at `_start` before `call`), or first syscall (bad
  encoding). Suspects in that order; no fix without an indicted layer.
- **147C (fix + un-quarantine):** minimal single-layer diff, remove the
  env gate, full `pkg/native` green in default CI, close the quarantine
  doc, final report with the evidence attached.

## 6. Non-goals (documented, NOT defects)

- No new lowering, no encoder extensions, no CLI flag, no PE/Mach-O
  (148–150). No K145 table changes. No performance work.
- The evidence job stays informational even after the fix lands for one
  full green cycle (removed only when the mandatory suite proves stable).

## 7. Gate

`go test ./pkg/native/ -count=1` green in DEFAULT CI (flag unset,
no skips on Linux) three consecutive runs; `PHASE-145-QUARANTINE.md`
marked closed; `PHASE-147-FINAL-REPORT.md` with root cause + evidence.
