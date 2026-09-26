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

### 150B1 — Heap arena + string concat (see §7 for the full note)

The memory substrate 150B needs, with string concat as its first real
consumer. Details and the two defects fixed are in §7.

## 7. Progress (continued)

### 150B1 — Heap arena + string concat (DONE)

The first 150B slice: the memory substrate the rest of 150B (push, maps,
structs) sits on, plus one real consumer so the arena is not dead code.

* **Arena.** A bump allocator over a static region: a 16-byte header
  (`[cursor][limit]`, both *absolute* addresses) followed by the data area.
  Holding the bookkeeping inside the arena itself means no globals and no
  writable image text, and the cursor self-initialises on the first `alloc`
  (a zero cursor means "not started"). No `free()`: bump-only is sufficient
  because every allocation 150B makes is compile-time bounded, which is what
  keeps the invariant checkable and the exhaustion `Int3` unreachable.
* **Writable storage, two ways, both gated.** ELF gets a second R+W `PT_LOAD`
  (page-aligned, `p_offset ≡ p_vaddr`); PE puts the arena inside `.idata`,
  which is already R/W and loader-proven writable — so no fourth section and
  no extra import. macOS has a single R+X `__TEXT`, so a program that
  allocates gets a loud K145 naming 150D rather than an image whose first
  cursor store would fault.
* **Gating.** The arena, the `alloc` helper and the concat frame area are all
  emitted only when the program actually concatenates. This is what keeps the
  increment-149 byte-identity pins green: `TestNativeELFByteIdentity` still
  measures zero drift across all 19 legacy programs after the ELF header
  count, the `.idata` sizing and the frame layout all changed.
* **String concat.** `a + b` on two strings allocates `len(a)+len(b)` and
  copies byte-wise (the lengths are runtime values, so a constant-offset load
  cannot address them). Operands stage through a dedicated 5-unit-per-depth
  frame area, not the int path's 8-byte `binTemp` scratch. The second copy
  reaches its offset by advancing the destination *pointer*, because the SIB
  `disp` field is a compile-time immediate in every x86-64 memory form.
* **The arena bound is a proof, not a guess.** Every string value in a
  concatenating program is a literal or a concatenation of literals, so
  `sites × totalLiteralBytes` bounds the sum of all runtime allocations. That
  is why the exhaustion trap cannot fire for a program that compiles.
* **Executed**: 14 concat programs (chained, grouped, variable operands, call
  arguments, returns, branches, loops, multi-site, long operands). The 7 PE
  cases execute for real on this Windows host; the ELF cases execute on the
  Linux CI leg and structurally validate everywhere.

**Two real defects found and fixed while landing it:**

1. **The concat pre-pass missed sites with no literal operand.** `a + a`
   (two variables) was not recognised, so no arena was emitted while emission
   still called `alloc` — an undefined-label panic. The pre-pass now
   classifies string-ness syntactically (with a fixpoint over `let` bindings
   and `string` parameters) instead of pattern-matching for a literal, and
   `emitStrConcat` refuses loudly if it is ever reached with no arena, so a
   future gap is a diagnostic rather than a panic.
2. **The concat scratch area would have overrun the int scratch.** It was
   first written against `binTemp` (8 bytes per depth) with 40 bytes of
   staging needed; it now has its own `strTemp` region, reserved only when
   the program concatenates (reserving it unconditionally would have grown
   every frame and broken byte-identity).

## 8. 150B2 — String slice + string comparison (DONE)

The two string operations that only *read* bytes, so they need the staging
area but no heap.

* **Slice** `s[a:b]` is a **view**, not a copy: the result is
  `(s.ptr + a, b - a)`, so no allocation and no copy happen. Bounds are
  `0 <= a <= b <= len` with a loud `Int3` on violation, matching the array
  index contract rather than silently clamping. A missing `b` means "to the
  end".
* **Equality/inequality** compares by content: a length check first (a
  different length is decisively unequal, with no byte reads at all), then a
  byte loop. Only `==` and `!=` exist; ordering is a loud K145 rather than a
  pointer comparison or an invented lexicographic helper.
* The pre-pass that reserves the string staging area was generalised: the
  area is now shared by concat, comparison and slice, so a program that only
  compares or only slices still gets correctly-sized scratch, and a program
  that does none of them keeps its exact increment-149 frame. The string
  classifier was also factored into one `collectStringNames` +
  `strKindOf` pair instead of being duplicated per scanner.
* `emitStrEqCond` validates **both** operand kinds, so `1 == s` names the
  offending int side whichever way round the operands appear.

**One imprecise diagnostic found and fixed while landing it:** classifying
only `+` as a string operation routed `s - "c"` into the int path, which
reported the far less helpful "string 's' in int position". Any operator
over two string operands is now classified as a string operation, so
`emitStr` owns the decision and names the operator precisely.

**Executed**: 15 string-view programs (slicing with literal and variable
bounds, empty slices, slices of a concat result, equality over equal/unequal
content, the length-mismatch path, both-empty, a slice compared against a
literal to gate a branch, and both operations inside a loop). All 15 execute
for real on this Windows host through the PE container. Ten new negative
cases pin the boundaries (string ordering, int operand in a string
comparison, slicing a non-string, `-`/`*` on strings, mixed string+int
concat).

## 9. 150B3a — `push()` (DONE)

`push()` was the one 150A item explicitly carried over, and the arena
unblocks it.

* **push is functional.** `let b = push(a, v)` allocates a *new*
  `(base, len+1)` array in the arena, copies the old elements, and appends
  `v`. The source array is untouched (the arena is bump-only, so nothing is
  moved or freed), which is what makes the two-slot header keep working:
  indexing, `len` and `for-in` all operate on a pushed array through exactly
  the same code as a literal one.
* **The copy uses the scaled 64-bit load/store forms** (elements are 8-byte
  ints) and the appended value is a single `StoreScaled64` with the old
  length as the index — no separate address arithmetic needed.
* **Everything is staged through the frame before the `alloc` call.** This
  is deliberate: `alloc` uses RAX/RCX/R10/R11 and takes RDI, so keeping the
  old base and length in R8/R9 across the call would have been exactly the
  kind of implicit register contract that produced the Phase-147
  `add(20,22) = 40` bug.
* **A push inside a loop is a loud K145.** That is the honest bound: a push
  in a `while`/`for`/`for-in` body can run an unbounded number of times, so
  no compile-time arena size can cover it. The alternative was a program
  that exhausts the arena and traps at an arbitrary iteration, so the
  pre-pass (`scanPushSites`, which tracks loop nesting depth) refuses it by
  name instead.
* **The arena bound is still a proof.** Outside loops, site *i* of a push
  chain sees at most `maxArrayLiteralLength + i` elements, so the sum over
  all sites is bounded by `pushSites × (maxLit + pushSites) × 8` bytes — and
  with the loop case refused, no site can execute more than once.

**Executed**: 12 push programs — single push, push onto an empty array, the
source array provably unchanged, two- and three-deep chains, iteration and
summing over a pushed array, two pushes off the same source, negative and
large values, an eight-element chain, a computed value, and a program mixing
a push with a string concat. All 12 execute for real on this Windows host
through the PE container. Nine negative cases pin the boundaries (non-int
element, wrong arity, non-array receiver, a literal receiver, and push inside
`while` and inside `for-in`).

## 10. Not yet started

- **150B3b** — maps, structs.
- **150C** — register allocation, validated against the `nativeLegacyELF`
  table landed in 150A.
- **150D** — Mach-O PIE/rebase (and the writable `__DATA` the heap refusal
  above names), the two new CLI targets, the native-split cache, and
  `pkg/cli/phase150_native_targets_test.go`.
