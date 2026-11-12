# NAT-FRAME baseline — the native frame layout against the Go oracle's

**Slice:** `NAT-FRAME` (unnumbered working label; the sixth v1.3.0 increment,
after `NAT-CALL`, `NAT-IF`, `NAT-QUAL`, `NAT-LOOP` and `NAT-IDX`).
**Branch:** `feature/v1.2.0-native-independence`.
**Status:** implemented and gated.

---

## 1. The premise, checked rather than inherited

The brief, and the NAT-IDX record before it, described the discrepancy as a
**frame-layout rounding** difference: the driver "rounds the locals region up to
the next 16-byte boundary" and the oracle "rounds it to the next 16-byte
boundary strictly greater than the current region".

**That description is wrong, and this increment measured it rather than
trusting it.**

### 1.1 The two rounding rules are already identical

The oracle (`pkg/native/program.go:4482-4486`):

```go
	if b.goos == OSWindows && next%16 != 0 {
		next += 8
	}
```

The driver (`src/compiler/native_value.kark`, `natFrameLayout`):

```kark
    if (isWindows == 1) {
        if (next % 16 != 0) { next = next + 8 }
    }
```

Both round the frame **up to the next multiple of 16**, and only on Windows
(the rounding exists for the Win64 16-byte stack alignment; Linux syscalls do
not care, and gating it on Windows is what keeps the Linux images frozen).
Both then add the same 96-byte argument-spill area.

There is no "strictly greater" rule anywhere in the oracle. The phrase came from
the Phase-151A Step 9d record, whose measurement (640 vs 656 for two locals)
was taken against a frame layout that has since changed — the D2/D3/D4/D5
increments moved the frame since — so its numbers no longer describe the tree.

### 1.2 The frame immediates already matched, and the measurement that said
otherwise was measuring the wrong instruction

The first measurement of this increment compared the **Windows PE** and read
the first `48 81 EC` in the image. The helpers are emitted first and use the
SHORT form (`48 83 EC`), so the first long form found *is* main's frame — but
on **Windows the 8-byte rounding quantises an 8-byte difference away**: a frame
that was 8 bytes short on Linux rounds to the same multiple of 16 on Windows and
looks identical. The PE measurement could not see the difference at all.

Measured on **Linux**, where no rounding applies, the delta is exactly **8 bytes
and only for programs that index something**:

| program | native frame | oracle frame | delta |
|---|---|---|---|
| no locals | 0x260 (608) | 0x260 (608) | 0 |
| 1 local | 0x268 (616) | 0x268 (616) | 0 |
| 2 locals | 0x270 (624) | 0x270 (624) | 0 |
| 3 locals | 0x278 (632) | 0x278 (632) | 0 |
| 4 locals | 0x280 (640) | 0x280 (640) | 0 |
| 5 locals | 0x288 (648) | 0x288 (648) | 0 |
| array of 1 | 0x278 (632) | 0x280 (640) | **8** |
| array of 2 | 0x280 (640) | 0x288 (648) | **8** |
| array of 3 | 0x288 (648) | 0x290 (656) | **8** |
| a call | 0x268 (616) | 0x268 (616) | 0 |

### 1.3 The two defects that actually produced the 8 bytes

**Defect A — the map-stage slot was never reserved.**

The oracle's `b.usesMap` is a **program-wide** flag (`scanMapUsage(prog)`,
`program.go:1571`), and its `isMapIndex` predicate is *"an index expression whose
target is a VARIABLE"* (`program.go:730-737`) — which an **array access**
satisfies. So the oracle reserves the 8-byte map-stage slot in **every** frame of
a program that indexes anything (`program.go:4461`).

The driver's `natFrameLayout` reserves it correctly **when asked to** — but both
of its call sites passed a literal `0` for the flag, so it was reserved in no
frame. Hence: 8 bytes short for every indexing program, and exactly right for
every other program.

**Defect B — `inc r8` decoded as `inc rax`.**

`natIncReg` built its REX byte **by hand** and ORed in `natMask(15)` — the
REX.**R** bit (0x04) — where a register's extension lives in REX.**B**
(`natMask(16)`, 0x01). REX.R extends the ModRM **reg** field, which for `FF /0`
is the opcode extension and not a register.

So `inc r8` was encoded `4C FF C0` = `inc rax`, while the oracle emits
`49 FF C0` = `inc r8`. The map SET helper increments its entry counter and then
stores it as the map's count — so **the map's entry count was silently wrong**,
and the value stored was the un-incremented one.

That defect is invisible in a program's output (the value it corrupts is a
helper's internal counter) and it had **no gate at all**: the matrix records
map support as `implemented_not_gated`.

**Defect C (found by the mutation, not by the code):** the fix for B had to use
the **centralized** constructor. `natDecReg` already did it right
(`natRex(s, 1, 0, r)`), and `natIncReg` now does the same — so there is still
exactly one REX constructor, and the two `inc`/`dec` forms cannot drift apart
again.

---

## 2. What changed

1. **`natIncReg`** now uses `natRex(s, 1, 0, r)` — the same call `natDecReg`
   makes, and the same shape as the oracle's `rex(true, 0, r)`.
2. **`natNativeProgramUsesMap(body, fns)`** — the program-wide map-usage flag,
   computed once and threaded into every function's layout, because the
   oracle's flag is program-wide and its layout reserves the slot in every
   function.
3. **`natNativeEmitFunc`** takes the flag and passes it to `natFrameLayout`.
4. **`natNativeProgramOS`** computes the flag once and uses it for the map
   helpers and for every function.

**Nothing else moved.** No parser change, no index-semantics change, no
diagnostic change, no ABI change, no register allocation.

---

## 3. Differential evidence

For the array fixture (`let a = [3, 5, 7] print(a[0])`), Linux ELF:

| | before | after |
|---|---|---|
| image size | 1095 | 1095 |
| frame immediate (native) | 0x288 | **0x290** |
| frame immediate (oracle) | 0x290 | 0x290 |
| differing bytes | **5** | **2** |

The five differing bytes were five REX prefixes (Defect B); four were the frame
immediate and the rodata addresses derived from it (Defect A). After both fixes,
**the frames are identical** and **two bytes remain**.

---

## 4. What remains, isolated

### 4.1 Two bytes — the reporter's rodata addresses

**Two bytes**, both absolute rodata addresses inside the **runtime-error
reporter**, and the cause is the ORDER in which the reporter's strings are
interned:

* the oracle emits the reporter **helper** before the user function, so its
  strings are interned **first**;
* the driver appends them to the rodata list **after** the program's own
  literals.

The offsets therefore differ (measured: 16 and 39 bytes) and the two absolute
addresses that carry them differ with them.

The frames, the code, the image sizes and the bounds behaviour all agree. The
driver's own comment already claims the oracle's order — "helpers are emitted
BEFORE the user function, so the reporter's ... are interned first" — and the
code does not do that, which is why this is a defect and not a design choice.

**It is NOT fixed here**, and the reason is stated rather than hidden: the
driver's rodata list is a FIXED ORDER, so reordering it moves the reporter's
offsets in **every** image that can raise a diagnostic — the array-bounds
program among them — and that needs its own evidence (the D5 gate pins the
diagnostic's interning today). The brief's scope is the frame; this is a
separate, smaller, well-characterised increment.

### 4.2 The string stage — a THIRD divergence of the same class

Measuring the whole gated corpus against the oracle (every program in the
9D, 9E, 9F and 9H corpora, both engines, the frame immediate read out of each
image) found **one** program whose frame still differs, and it is the same
root cause as the map stage, one region over:

| | oracle's rule | driver's rule |
|---|---|---|
| **map stage** | program-wide (`scanMapUsage` over the program) | per function — **FIXED by this slice** |
| **string stage** | program-wide, over FIVE operations (`usesConcat \|\| usesStrEq \|\| usesStrSlice \|\| usesPush \|\| usesStrField`) | per function, over TWO (concat sites and push counts) |

So a program that assigns a string to a field gets **no string stage at all**
in the driver, and a program whose string work happens in one function gets a
different frame in another function.

**Measured** (windows PE, `s.label = "changed"`): the driver's frame is
**640** bytes and the oracle's is **3200** — a difference of exactly
**2560** bytes, which is the string stage (`8 * 5 * maxBinDepth = 8 * 5 * 64`).
The difference being *exactly* the string stage is what makes this a
string-stage difference and not something else that happens to be wrong.

**It is NOT fixed here**, for a stated reason: this is a string-stage
**policy**, not a frame **layout**. The oracle's rule is a program-wide
pre-pass over five operations; the driver's is a per-function scan over two.
Reconciling them means changing what the driver *supports* (string equality
and string slicing are not lowered at all today) and re-deciding the
per-function-versus-program-wide question for the string region — which is a
different increment with a different blast radius, and it wants its own
baseline and gate.

It is **PINNED** rather than ignored: `TestPhaseNATFrame_TheStringStageIsStillDivergent`
asserts that the frames still differ AND that they differ by exactly 2560
bytes, so the gap cannot be forgotten and cannot silently change shape. When
it closes, the gate fails and says to move the case into the parity list.

> **CLOSED by NAT-STR-STAGE (2026-11-11).** The section above describes the
> state at the end of NAT-FRAME and is kept as the record of it. The gap
> closed: the string staging area is now reserved **program-wide** over the
> oracle's **five** operations, the frames match the oracle's exactly, the
> string-assignment case joined `natFrameCases`, and
> `TestPhaseNATFrame_TheStringStageIsStillDivergent` was **inverted** into
> `TestPhaseNATFrame_TheStringStageIsReservedProgramWide`, which asserts the
> closed state on both axes. The reconciliation NAT-FRAME declined to do
> blind was done as its own increment, with its own baseline and gate:
> `docs/audit/NAT-STR-STAGE-BASELINE.md`.

### 4.3 A note on the driver's index predicate

The driver's map-usage predicate fires on **any** index expression, while the
oracle's `isMapIndex` requires the index's target to be a **variable**
(`pkg/native/program.go:730-737`). The driver's is therefore a **superset**,
which is the safe direction: it can over-reserve the map stage but never
under-reserve it.

Measured shapes that probe the difference — a plain index, a nested index, an
index whose index is a call, an index inside a binary expression, and an index
inside an `if` condition — all produce **identical** frames on both engines,
because in every one of them the inner index's target *is* a variable and the
oracle's predicate fires too.

---

## 5. The gate

`pkg/cli/phase151k_frame_test.go`, tests named `TestPhaseNATFrame_*`, with its
own `ci.yml` step (*Run NAT-FRAME native frame-layout gate*).

| Layer | What it pins |
|---|---|
| `TheFrameImmediateMatchesTheOracle` | the frame-size immediate of every boundary case, read out of each IMAGE, against the ORACLE's — for **both** targets, because the Windows rounding hides an 8-byte difference |
| `TheMapStageSlotIsReserved` | the map-stage slot is reserved for an indexing program (an 8-byte difference against the same program without the index) and not for a program that does not index |
| `TheIncUsesTheBExtBit` | the `inc r8` ENCODING, at the byte level: REX.W \| REX.B (0x49), not REX.W \| REX.R (0x4c) |
| `TheImagesAgreeOnEverythingButTheReporterStrings` | the residual: equal frame, equal sizes, and **exactly two** differing bytes — the gap is pinned, not hidden |
| `TheStringStageIsStillDivergent` | the THIRD divergence (§4.2): the frames differ by **exactly 2560** bytes, the string stage. Asserted as a difference so the gap cannot be forgotten, and so it fails loudly when it closes |
| `BoundsAndCallsStillWork` | local reads and writes, a call, a nested call, a variable index, a nested index, and the bounds-error path (a non-zero exit and no value printed) |
| `GateIsWiredInCI` | this gate has a `ci.yml` step |

The boundary cases are the ones the brief names: zero locals, a non-aligned
locals size, an exactly aligned one, aligned plus one, several frame sizes, an
array's element area, a call, nested calls, and the two index shapes.

### 5.1 The frozen 9D gate had to be corrected too

`TestPhase151A9D_FrameFollowsTheValueModelRule` asserted the frame against a
rule restated in that file — and that rule was **wrong in two ways**, in a way
that pointed the fix at the wrong place:

1. it rounded the **locals region** and then added the fixed regions, while
   the oracle rounds the **total** and adds the argument spill *after*;
2. it had no **map-stage** term at all, because the map stage did not exist
   in the driver's frames.

Both were corrected, and the corrected rule is the oracle's:

```
	locals region                       (the program's own bindings)
	+ 512  bin temporary area            (maxBinDepth 64 * 8)
	+   8  map-stage unit                (only when the program indexes)
	then the Win64 stack-discipline rounding
	+  96  argument spill area           (argSpillBytes, 6 args * 16)
```

The two formulations agree while the only fixed region is 512 (a multiple of
16), which is why the original form of the rule was not wrong until the
map-stage unit arrived; they part company by 8 bytes afterwards.

The same correction was applied to the three other gates that share that
helper — the 9E (`len`), 9F (string) and 9H (record) frame gates — and each
corpus program now states whether it indexes anything, so the map-stage term
is **visible in the case table** rather than implied.

**The 9D gate's second check was VACUOUS, and is now a real bound.** It
back-computed the locals region as `want - 512 - 96` and asserted it covered
the header plus every element. Under the pre-NAT-FRAME rule that quantity was
`round16(localBytes)`, so the assertion `round16(localBytes) >= 16 + 8*n`
held **by construction** and could not fail. It is now a **lower bound on the
frame** — the frame must be at least the sum of the regions it has to hold —
which an element-blind driver (one that reserved the header but not the
elements) fails.

---

## 6. Mutation verification

Two mutations, each restored **byte-identically** (the SHA-256 after each
restore is in §8), with the self-hosted sources' mtimes bumped so `kccStale`
cannot reuse a mutated binary.

| # | Mutation | Observed |
|---|---|---|
| M1 | the map-stage reservation reverted (the layout call passes `0` again) | the frame-immediate layer fails on **four** cases with the exact byte difference ("native 0x278 (632), oracle 0x280 (640)"), and the map-stage layer fails ("frame with an index 0x288 (648), without 0x288 (648), want a difference of 8") |
| M2 | `natIncReg` set the REX.R bit again | the inc-encoding layer fails, and the residual-bytes layer fails with **three** differing bytes instead of two |

Both are detected by **meaningful assertions**, not by a timeout: M1 by the
frame-immediate comparison and M2 by the byte-level encoding check.

---

## 7. What this slice does NOT do

* register allocation (that is `NAT-REG`);
* `for ... in`, map or string element support, non-integer element types;
* the two NAT-LOOP parser defects (a call expression in a `for` post is
  mis-parsed by both engines, and the engines disagree about a `for` init's
  scope);
* the stderr / `GetStdHandle` limitation;
* any ABI or calling-convention redesign;
* the rodata interning order (§4).

---

## 8. Record

### 8.1 Measured results

* **Gate:** `TestPhaseNATFrame_*` — 6 layers. Green.
* **The frames now match the oracle** for every boundary case, on both
  targets.
* **The residual image difference is exactly two bytes**, in the reporter's
  rodata addresses.
* **KIR pin:** 14834 → **14841** (+7). Measured with
  `karkain kir --verify src/compiler/kir.kark` in the gate's own invocation
  (`text=14841 verify=14841`) and never predicted.
* **Full regression:** see §8.3.

### 8.2 Defects found

Three, and none of them was the recorded one:

1. the REX bit in `natIncReg` (§1.3, Defect B) — a silent data-corruption bug;
2. the unreserved map-stage slot (§1.3, Defect A) — the frame delta;
3. the recorded "strictly greater rounding" premise itself (§1.1) — which was
   wrong, and which the increment's first act was to measure.

### 8.3 Full regression results

### 8.4 The slice's own boundary, stated

**No execution evidence exists for the Linux ELF or the Mach-O image of this
slice.** This host is `windows/amd64` with no WSL and no Docker, and the
repository has no macOS runner. The Windows PE images are executed; the Linux
and macOS images are validated structurally by the Go oracle's own parsers.

**The compiler's own build still uses `gcc`**, and the bootstrap proof is still
carried to v1.3.0 as UNMET.

### 8.5 Next

**The rodata interning order** (§4): two bytes, in every image that can raise
a diagnostic, with the cause identified and the driver's own comment already
describing the oracle's order. It wants its own increment because the fix moves
the reporter's offsets in every diagnostic-bearing image, and the D5 gate pins
that interning today.

**`NAT-REG`** — the register allocator — remains the largest genuine gap in
the native value model, and is unchanged by this increment.
