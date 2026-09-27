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

- [x] Boxed Value model covers arrays/for-in/floats/maps/structs/string
  ops with executed goldens (ELF live, PE live where applicable).
- [x] Register allocation lands with zero golden drift (differential).
- [x] Mach-O PIE/rebase structural + parsed; win/macOS targets listed,
  built, refused correctly per host matrix.
- [x] ELF byte-identity differential vs the 147/148/149 corpus green.
- [x] `go build ./...`, `go vet`, full `pkg/native` suite, neighboring
  CLI gates (148, 122-KIR untouched) green.
- [x] AGENTS.md increment-150 record; commit `develop` → merge `main` →
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

## 10. Current state — slices 150A–150C complete, 150D not started

Increment 150 is **one increment**, planned as eight slices:

| Slice | Deliverable | State |
|---|---|---|
| 150A | Native Value model (floats, arrays, `for-in`, `len`) | ✅ complete |
| 150B1 | Heap arena + string concatenation | ✅ complete |
| 150B2 | String slice + string comparison | ✅ complete |
| 150B3a | `push()` | ✅ complete |
| 150B3b | Records / structs | ✅ complete |
| 150B3c | Maps | ✅ complete |
| 150C | Register allocation | ✅ complete |
| 150D | **Mach-O PIE + native OS targets + incremental decision** | ✅ **complete** |

**Increment 150 is complete** (all eight slices, 150A–150C plus 150D). 150A–150C
are frozen unless a directly demonstrated regression requires an explicit
corrective change (see §12); 150D's record is §13. The next increment is **151**
(kcc native parity).

## 11. 150D — scope (the only remaining slice)

### Mach-O
* PIE flag
* rebase opcodes
* `ParseMachO` support
* writable `__DATA` segment for allocating programs (today an allocating
  program gets a loud, named refusal on macOS)

### CLI native targets
* `native-x86_64-windows`
* `native-x86_64-macos`
* target listing / `--help`
* per-OS build/run matrix
* run refusal when the target OS does not match the host
* **exit code 6** for unsupported-host execution

### Incremental compilation
* native-split incremental cache, **or** a documented, explicitly gated refusal
  if cache soundness cannot be established — the refusal text is itself gated

### Gate
`pkg/cli/phase150_native_targets_test.go` (magic per OS, run refusals, listing,
incremental refusal), plus the existing `pkg/native` unit goldens.

### Explicitly OUT OF SCOPE for 150D
* ARM64 native
* PE delay-load
* TLS / SEH / resources / code signing
* Mach-O **execution** on Intel macOS (no runner exists; structural only)
* kcc native parity (that is increment 151)
* unrelated backend optimization

## 12. Corrective changes against the frozen 150 baseline

**A completed increment is frozen.** A later defect does not automatically
reopen it. A corrective change must (1) identify the owning layer, (2) document
why it is required, (3) preserve downstream behavior, (4) run the affected
regression gates, and (5) be recorded as a corrective change against the frozen
baseline. None of the changes below reopened a slice.

**Scope freezes when a version opens. A version is not closed as "green except
X".** An unmet condition moves to the next version or to an explicit carry-over
list.

### 12.1 Test-validation correction (read before trusting the slice records)

The per-slice sections above say "executed live" and "all N programs run for
real". **Those claims were affected by a test guard and were not verified as
written.** The PE execution guard read:

```go
if out != "" { /* compare */ }
```

A program that printed **nothing** was therefore never compared — the guard
skipped it silently. The guard was later replaced with `requireNativeOut`
(`program_test.go`), which treats empty output as a hard failure, and that change
exposed **38 red subtests** across the increment.

This is a **test-validation correction, not a claim that the implementation did
not exist.** The value model, arena, string operations, `push()`, records, maps
and register allocation are all present in the tree. What was wrong was the
*evidence*: the "executed" suites had been passing vacuously, and three real
defects had shipped underneath them.

Subsequent targeted validation exercised the **actual output checks** — real PE
execution on this Windows host, structural validation for Mach-O, and the ELF
differential — and the corrected suites now pass on that evidence.

### 12.2 The three defects that had shipped

| # | Owning layer | Defect | Why it survived | Fix |
|---|---|---|---|---|
| 1 | `pkg/native` helper I/O (`print_int`) | the sign write used a raw `syscall` — Linux/macOS `write(1,…)`, with no valid meaning in the PE container — so **every negative integer printed unsigned** (`print(-1)` → `1`, `3-10` → `7`) | the broken path still printed digits, so output was non-empty and the guard was satisfied; and **no PE case ever printed a negative int**, so the branch never ran on that container | sign write goes through `emitWrite` |
| 2 | `pkg/native` value lowering (`emitPush`) | `len` was stored as `oldlen + 8` instead of `+1`, at two sites: a pushed array reported `len` 10 for three elements, and `for-in` over it emitted eight phantom elements | same vacuous guard — the affected suites never ran to comparison | both sites add the one appended element |
| 3 | `pkg/native` map lowering (`emitMapInsertEntry`) | the key was evaluated into `RAX`, then the value — a second expression that also lands in `RAX` — overwrote it, and the key was never moved to `RSI`. Every pair was filed under a junk key: reads always missed, and a two-entry literal stored both under one key so `len()` reported 1 | same vacuous guard | the key now rides a dedicated `mapStage` frame unit |

**Downstream behavior preserved:** non-map programs keep their exact frame and
therefore their exact emitted bytes; the new staging slot is conditional, the
same discipline `strTemp` already used.

**Gates run:** `pkg/native` (fully green, from 38 red subtests), the Phase 148
native CLI gate, `TestPhase150C`, `pkg/parser`, `pkg/lexer`, `pkg/sema`,
`go build ./...`, `go vet`.

**New coverage for the class that had none:** `nativeNegIntCases` pins eight
sign shapes (`-1`, `-x`, `-(-x)`, `3-10`, `0-1`, array elements, a loop, a
branch) on **both** containers, every golden cross-checked against the C
backend, plus a structural pin on the sign-write ordering.

**One earlier fix was reverted as incorrect.** A `MovzxRegMem8` change claimed
`REX.W + 0F B6` is an eight-byte load. It is not — that encoding is
`movzx r64, r/m8`, a one-byte load zero-extended to 64 bits. `emit.go` is
therefore untouched by the corrective change, and the two emit byte pins it had
broken are green again.

### 12.3 Test-data corrections (no behavior change)

* `forin_nested`'s golden said `10/20/11/21`, which no evaluation of that source
  can produce. The C backend — the language's semantic oracle — prints
  `20/30/30/40`; the golden now matches arithmetic.
* `nativeLegacyELF` is re-pinned. Two entries had been left stale by the
  preceding encoder commit, and the `print_int` reordering moves every image.
  The drift was **verified benign, not assumed**: old and new `p145_hello`
  differ in exactly 13 bytes — the three instructions of the sign path in a
  different order — identical size and identical ELF semantics, since all three
  argument registers are still set before the syscall.

### 12.4 Known limitation carried forward

Building the **compiler** itself without a C compiler is still unscheduled:
after increment 154 the toolchain is Go-free, but bootstrap links through `gcc`
because `kcc` emits C23. See `KARKAIN-ARCHITECTURE-ROADMAP.md` §6. Tracked as a

## 13. 150D — Mach-O PIE, native OS targets, and the incremental decision (DONE)

150D had two open items: the Mach-O container (PIE, rebase opcodes, a writable
`__DATA` segment) and the native-split incremental cache. The CLI half of the
targets landed in the previous commit (`2604bfb`); this is the rest.

### 13.1 Mach-O is now a real PIE

Increment 149 shipped a Mach-O that was *position-dependent*: flags
`NOUNDEFS|DYLDLINK|TWOLEVEL`, one R+X `__TEXT` covering the whole file, and a
zeroed `LC_DYLD_INFO_ONLY`. That is not loadable as a PIE, and it is not a valid
modern executable: dyld reserves the 4 GiB hole below `0x100000000` for
`__PAGEZERO`, and every address the backend bakes into the instruction stream is
an imm64 that a slide would invalidate.

So `pkg/native/macho.go` now emits a conventional PIE:

* `__PAGEZERO` (unmapped hole, 4 GiB at 0) then `__TEXT` R+X at `MachoBase`,
  then — only when the program allocates — a **writable `__DATA`** for the
  arena, then `__LINKEDIT` for the rebase opcodes. Sections stay empty
  (`nsects=0`), so there is no section table to get wrong.
* `MH_PIE` (`0x200000`) joins the flag set.
* **Real rebase opcodes** in `LC_DYLD_INFO_ONLY` (`rebase_off`/`rebase_size`),
  pointing at the `__LINKEDIT` contents. `bind`/`weak_bind`/`lazy_bind`/`export`
  stay zero: the image imports nothing (raw syscalls) and exports nothing.
* `__DATA` and `__LINKEDIT` each start on a page boundary, so `fileoff` and
  `vmaddr` agree modulo the page size and no segment overlaps the one before it.

**The load-bearing decision** is where the rebase sites come from. They are
derived from `machoRebaseSiteOffsets(b)` — the union of the `.rodata` patch list
and the arena patch list, i.e. exactly the lists the addresses were *resolved
from*. A hand-maintained inventory of "the places we bake an address" would be a
second source of truth that could silently miss a site, and a missed site in a
PIE is not a crash: it is an image that loads and then dereferences a pointer
nobody slid. Deriving it makes the omission structurally impossible.

**The allocation refusal is gone.** Increment 149 refused any allocating program
on macOS by name ("a writable `__DATA` segment lands with 150D"), because its
single R+X `__TEXT` would have faulted on the arena's first bump-cursor store.
Both gates that pinned that refusal (`TestNativeMapStructural`,
`TestPhase150D_BuildIsCrossHost`) were flipped to assert a real, validated image
in the same commit that added `__DATA`, exactly as the test file's own comment
asked for.

**`parseMachO` is now a real validator**, not a shape smoke-test. It checks
MH_PIE, the exact segment set, each segment's protection, the page-aligned
placement, the entry, `sizeofcmds` against the commands actually present, and it
**decodes the rebase stream**, rejecting it if it does not terminate, has bytes
after `DONE`, names a non-POINTER type, names the wrong segment, or carries an
over-long uleb128. `MachORebaseSites` and `MachOHasDataSegment` are exported so
gates can assert the published set is exactly the patched set.

### 13.2 Two real encoder defects, caught by the goldens

The byte-level golden table for the rebase encoder failed on first run, and both
failures were genuine logic errors in `machoRebaseOpcodes`:

1. **The cursor was conflated with the last emitted site.** The code tracked one
   variable for both, and treated "site equals cursor" as a duplicate. But dyld
   advances the cursor by 8 after every rebase, so a site landing exactly on the
   cursor is the *adjacent 8-byte slot*, not a duplicate — so it was silently
   **dropped** from the stream. A second, subtler consequence: for a genuine
   duplicate the delta is negative, `s-prev >= 0x80` is false for a negative
   number, and the code fell into the short-hop branch and encoded
   `byte(s-prev)` — a truncated negative (`0xf8` = 248) — a bogus forward hop of
   248 bytes instead of "skip".

   Fixed by tracking `last` (last emitted slot) and `cur` (the cursor)
   separately, and by requiring `0 < d < 0x80` for the hop.

   **How live was this?** Less than it looked, and the measurement is worth
   recording: every baked address is an imm64 inside a 10-byte `movabs`, so
   distinct sites are ≥10 bytes apart and the cursor always lands *past* the
   previous site. The `d == 0` branch is therefore defensive rather than
   currently reachable; the negative-delta path was reachable only for a
   duplicate site. Both guards stay, because "sites are ≥10 apart" is a property
   of today's emission, not of the encoding.

2. Three test *fixtures* were wrong rather than the code, and all three are
   recorded because a green check is only as strong as what it checked:
   * a "truncated uleb" case `{0x80, 0x51}` is a perfectly valid two-byte
     uleb128 (10368) — the decoder was right to accept it; replaced with a
     genuinely over-long uleb.
   * an assertion that the arena base and its limit are *adjacent* rebase sites
     failed; they are 10 bytes apart, for the reason above.
   * the limit is not the stored header field at all: reading `emitAllocHelper`
     shows R11 is re-materialised as `arena + heapHeaderLen + heapSize` (the
     arena **end**), because the helper never loads the limit field. The test now
     asserts the base and the end are both published, which is the property that
     actually matters.

### 13.3 The incremental cache: refusal kept, on evidence

The baseline allowed either a native-split cache or "a documented, explicitly
gated refusal if cache soundness cannot be established". 150D investigated and
**kept the refusal**, with the reason measured rather than asserted. A new
`BenchmarkNativeCompile` in `pkg/native` measures pure-Go emission per program:

| case | ns/op | B/op | allocs/op |
|---|---|---|---|
| hello_linux | 11,016 | 3,664 | 43 |
| concat_linux | 33,982 | 14,726 | 65 |
| map_linux | 20,188 | 14,078 | 67 |
| concat_windows | 53,106 | 30,006 | 104 |
| concat_macos | 25,875 | 19,388 | 76 |

* **A whole-image content-addressed cache cannot repay itself.** The entire cost
  it could ever save is 11–53 µs, smaller than the stat + read + hash work of
  looking the cache up.
* **A per-function split cache is not sound with today's emitter.** The native
  backend emits one monolithic `.text` and resolves every intra-text reference

### 13.4 Gates and regressions

* `pkg/cli/phase150_native_targets_test.go` — 6/6 PASS (19.4 s), including the
  flipped macOS allocating build, with MH_PIE + writable `__DATA` + decodable
  rebase opcodes asserted on the real image the CLI wrote.
* `pkg/native` — full suite green (8.5 s), including the **19-image ELF
  byte-identity differential at zero drift** (Mach-O work touches no ELF or PE
  byte) and the live PE execution suites on this windows/amd64 host.
* New `pkg/native/phase150d_macho_test.go`: encoder goldens, decoder negatives,
  PIE load-command pins, the `__DATA`/arena-site assertions, and an **8-case
  tamper table** (cleared MH_PIE, non-R+X `__TEXT`, shrunk `__PAGEZERO`,
  non-writable `__DATA`, unaligned `__DATA`, moved rebase stream, truncated
  rebase stream, bind opcodes present).
* `TestPhase148*` green (6.8 s); `pkg/parser`, `pkg/lexer`, `pkg/sema`,
  `pkg/target`, `pkg/compiler` green; `go build ./...` and `go vet` clean.

### 13.5 Boundaries carried out of 150D (documented, NOT defects)

* **Mach-O execution is still unproven** — no Intel-mac runner exists (GitHub's
  macOS legs are arm64), so every Mach-O claim here is structural. The honest
  limit is unchanged from 149 and now stated in the writer's own doc comment.
* `LC_DYLD_INFO_ONLY` rebase opcodes, not `LC_DYLD_CHAINED_FIXUPS`: the baseline
  named "rebase opcodes", and the legacy form is what the existing load command
  already carried. Chained fixups would be a separate change.
* arm64 native, PE delay-load/TLS/SEH/resources/signing, and kcc native parity
  (increment 151) remain out of scope.

  *eagerly*: `rel32` branches in `Emitter.Bytes`, and the `.rodata`/arena imm64
  sites in the linker. Caching a function's bytes independently therefore has no
  key that keeps those references valid; it needs a real relocation model
  (offset+type relocations against symbol boundaries), which is a larger piece of
  work than this flag and honestly out of scope here.

The refusal text was rewritten to carry that reason, and
`TestPhase150D_IncrementalRefused` now asserts the **reason** is present
("monolithic", "relocation model"), not just the exit code and the target name —
a refusal that decayed into "not supported yet" would pass an exit-code-only gate
while losing the explanation, and the explanation is the deliverable. The
benchmark is what will say "revisit this" if emission ever gets slow.


roadmap gap, not as a 150 defect.
