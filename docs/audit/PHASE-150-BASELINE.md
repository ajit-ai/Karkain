# Phase 150 BASELINE — Native Value Model + Windows/macOS Targets

Date: 2026-09-25. Version-plan slot: 1.2.0 "Sovereignty I", increment 150
(first of 150 → 151 → 152 → 154; 165 free-floating). Parent reports:
`PHASE-147/148/149-FINAL-REPORT.md`. KIR pin 9574 unaffected (native path
does not consume KIR).

## 1. Starting position (verified in-tree)

- **Value model today: ints + strings only.** `pkg/native/program.go`
  rejects floats (`error[K145]`, line 356) and `for-in` (line 922:
  "arrays lower in a later slice"); maps/structs have no native
  representation. Locals live in frame slots (`Builder.slots`); **no
  register allocator exists** in `pkg/native/emit.go`.
- **Formats:** ELF executes green (Linux, byte-identical pre/post-149);
  PE executes 7/7 live on windows/amd64 (`TestNativePE`); Mach-O is
  structural only — flags `NOUNDEFS|DYLDLINK|TWOLEVEL`, **no MH_PIE**,
  no rebase opcodes (149 report §D: "PIE/rebase opcodes are 150+ work").
- **CLI:** one native target, `native-x86_64-linux` (`pkg/cli/
  cfree_target.go` build/run commands + `exitcodes.go` registry/listing);
  run refused off Linux (exit 6); `--incremental` refused for native
  ("native-split caching is future work", `incremental.go:127`).
- **Parity/compat:** kcc auto-routes exotic targets to Go (no kcc native
  changes through 149 — kcc parity is increment 151, not here).

## 2. Contract (from `KARKAIN-VERSION-PLAN.md` §4, quoted)

Native **Value model** (boxed `Value`: arrays, `for-in`, floats, maps,
structs, string ops) + register allocation + Mach-O PIE/rebase +
native-split incremental cache + **`--target native-x86_64-windows` /
`native-x86_64-macos`** CLI targets (listing, `--help`, per-OS build/run
matrix, run only on matching hosts else exit 6).

Gate: `pkg/native` executed goldens on windows/amd64 (PE) and
linux/amd64 (ELF), structural Mach-O everywhere; new
`pkg/cli/phase150_native_targets_test.go` (magic per OS, run refusals,
listing, incremental refusal); ELF byte-identity differential vs the
147/148/149 corpus.

Boundaries (NOT this increment): arm64 native; PE delay-load/TLS/SEH/
resources/signing; Mach-O **execution** (no Intel-mac runner); kcc
native parity (151).

## 3. Slice plan (proposed)

- **150A — Value model core:** arrays (literal/index/len/push) + `for-in`
  over arrays + float64 arithmetic, boxed `Value` layout shared across
  OS lowerings; K145 table updated (float/for-in rows removed, maps/
  structs rows kept). Executed goldens on ELF (+PE where OS-neutral).
- **150B — Value model complete:** maps + structs + string ops
  (concat/slice/compare) in the boxed model; whole K145 reject table
  re-pinned to the smaller remainder.
- **150C — Register allocation:** frame-slot locals → register assignment
  with spill discipline; all 147–150 goldens byte-identical pre/post
  (behavioral differential, not just new goldens).
- **150D — Formats + CLI surface:** Mach-O PIE flag + rebase opcodes
  (structural, ParseMachO-extended); `native-x86_64-windows` /
  `native-x86_64-macos` targets registered (listing/`--help`/matrix,
  exit-6 refusals off-matching-hosts); native-split incremental cache
  (or a documented keep-refusal with reason, if the cache proves
  unsound — the refusal text is itself gated).
- **Gate file:** `pkg/cli/phase150_native_targets_test.go` (+ unit
  goldens in `pkg/native` per slice).

## 4. Non-goals (documented, NOT defects)

arm64; PE delay-load/TLS/SEH/resources/signing/codesigning; Mach-O
execution; kcc native parity (151); removing the C path (`--target c23`
stays); WASM/native interplay.

## 5. Exit criteria

- [ ] Boxed Value model covers arrays/for-in/floats/maps/structs/string
  ops with executed goldens (ELF live, PE live where applicable).
- [ ] Register allocation lands with zero golden drift (differential).
- [ ] Mach-O PIE/rebase structural + parsed; win/macOS targets listed,
  built, refused correctly per host matrix.
- [x] ELF byte-identity differential vs the 147/148/149 corpus green.
- [x] `go build ./...`, `go vet`, full `pkg/native` suite, neighboring
  CLI gates (148, 122-KIR untouched) green.
- [ ] AGENTS.md increment-150 record; commit `develop` → merge `main` →
  push (hard rule).

## 6. Progress

### 150A — DONE (committed to `develop`)

Floats and arrays landed as one step; the K145 table's float and for-in
rows are gone, replaced by array rows.

* **Float64**: `float64 -> IEEE-754 bits -> one 8-byte unit`, identical to
  an int in frames, calls and returns. `+ - * /`, unary minus, the six
  comparisons, float params/returns/calls. Division by zero yields 0, and
  printing is `%g`-compatible (fixed for `-4 <= X < 6`, else scientific,
  round-half-even at the 7th digit, trailing zeros trimmed) — matching the
  C backend rather than inventing a third formatting. SSE2 only, so it is
  baseline-x86-64 with no FMA/AVX dependency.
* **Arrays**: an array is a `(base, len)` header plus a compile-time-sized
  element area in the frame. Literal binding, `arr[i]` with a checked
  bound (negative or `>= len` traps), `len(arr)`, and `for x in arr`.
  Scaled `[base + i*8]` addressing keeps the array base out of `rsp`, so
  Phase 147's "rsp never moves" rule holds and every frame slot address
  stays stable.
* **Executed**: 9 float + 11 array programs, each run on **both**
  containers. On this windows/amd64 host the PE legs execute for real
  (PE + Win64 boundary + PEB bootstrap), which is how the array lowering
  is proven rather than merely asserted.

Two real defects were found and fixed while landing it:

1. **The for-in guard was inverted.** `cmp [len], i` leaves `len - i`, so
   the exit branch must be `jle`, not `jge`. With `jge` the body never ran
   for any non-negative index. Pinned by the nested-loop golden.
2. **Every image grew ~700 bytes.** `print_float` was emitted
   unconditionally, so an int-only program carried a formatter it could
   never call (hello: 503 → 1226 bytes) — which would have failed the
   byte-identity criterion. It is now gated on `scanFloatUsage`, a
   whole-unit pre-pass (it has to be: `emitHelpers` runs before any body).
   The differential then measured **zero drift** across all 19 legacy
   programs.

**Deviation from the slice plan, carried to 150B:** `push()` is listed in
150A but is *not* implemented — it is a loud K145 naming itself. A push has
to grow the element area, and 150A arrays are fixed-footprint frame values
sized by the layout pass; making them growable is heap/boxed-`Value` work,
which is exactly 150B. Reporting it here rather than shipping a
half-semantics version.

## 7. Not yet started

- **150B** — maps, structs, string ops (concat/slice/compare) in the boxed
  model; `push()` (above); whole K145 reject table re-pinned.
- **150C** — register allocation, validated against the `nativeLegacyELF`
  table landed in 150A.
- **150D** — Mach-O PIE/rebase, the two new CLI targets, the native-split
  cache, and `pkg/cli/phase150_native_targets_test.go`.
