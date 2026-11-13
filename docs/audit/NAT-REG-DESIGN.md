# NAT-REG design - native register allocation in the self-hosted driver

**Slice:** `NAT-REG` (unnumbered working label; the ninth v1.3.0-class
increment, after `NAT-CALL`, `NAT-IF`, `NAT-QUAL`, `NAT-LOOP`, `NAT-IDX`,
`NAT-FRAME`, `NAT-STR-STAGE` and `NAT-RODATA`).
**Branch:** `feature/v1.2.0-native-independence`.
**Status:** designed; the implementation follows in the same commit series.

---

## 1. The investigation

Everything below was read out of the code, not assumed.

### 1.1 The ORACLE already has an allocator

`pkg/native/regalloc.go` is the Phase-150C allocator, and it is the reference
for this slice because the driver's job is to be the same compiler.

* **The allocatable set is `R12`-`R15`** (`allocRegs`), for two independent
  reasons the file states: they are callee-saved under BOTH ABIs in play
  (System V and Win64), so a value parked there survives a call to a user
  function, a helper or a kernel32 export without a spill; and they are
  untouched by the rest of the backend, which works in
  RAX/RCX/RDX/RSI/RDI/R8-R11 and clobbers RBX. `RBX` and `RBP` are
  deliberately NOT allocatable because both are live scratch inside emitted
  helpers.

* **The engagement gate** (`planRegs`) has two halves, and BOTH must hold:
  the function contains control flow (`hasControlFlow`: an `if`, a `while` or a
  `for` anywhere in the body), AND the peak number of simultaneously-live int
  locals is at least `raMinLive = allocRegCount + 1 = 5`. The rationale is in
  the file: at or below four live values everything fits in a register anyway,
  so there is no pressure to relieve, and in a straight-line body a push/pop
  pair around the function would cost more than the single load it removes.

* **The liveness analysis** (`liveRanges`) walks the body in emission order
  and produces one range per int local, in "points" of that walk. A local's
  range starts at its definition and ends at its last use. Loops need explicit
  handling: a value used inside a loop body is live for the WHOLE loop, so the
  ranges of the names the loop mentions are widened to span it (`hold`). That
  is conservative in the safe direction - it can only cause an extra spill,
  never a wrong answer.

* **The allocation** is a linear scan over the ranges in order of first
  definition: a register is available when the local holding it is dead
  before this range starts; otherwise the candidate spills to its frame slot,
  which is the pre-150C behaviour and needs no new code path. **Ranges are
  never split** - a local either owns its register for the whole range or
  keeps its frame slot.

* **The frame does not shrink.** A spilled local keeps its slot, so the
  frame's locals region is unchanged and every slot displacement stays the
  same. What the allocator saves is memory TRAFFIC, not frame size.

* **The discipline** is: push the claimed callee-saved registers BEFORE the
  frame is taken (so the frame's rsp-relative displacements are unchanged),
  and pop them in exact reverse order AFTER the frame is released. `regsUsed`
  is published in `allocRegs` order, so the push/pop sequence is a pure
  function of the allocation and never of map iteration order.

* **There are exactly three access sites** in the oracle: the `let` binding
  (`emitLet`), the identifier read (`emitExpr`) and the assignment
  (`emitExprStmt`). Each asks `regOfFor(name)` and falls back to the frame
  slot when the local was not allocated.

### 1.2 The DRIVER has no allocator

`src/compiler/native_value.kark`:

* Every local lives in a frame slot. The driver's `plan` gives each slot an
  offset, a kind, an element count and a name - four fields, and the four
  accessors (`natNativeSlotOff`, `natNativeSlotKind`, `natNativeSlotElems`,
  `natNativeSlotName`) are the ONLY places that assume the width.

* The driver's expression evaluator uses RAX as the accumulator and stages a
  binary operand's left into the frame's `binTemp` area. So the expression
  TEMPORARIES are frame-resident and the allocator only ever has to deal with
  the LOCALS.

* The driver's local-access sites in the real lowering path are few:
  `natEmitIntExpr`'s identifier case (a read), the binding arm of
  `natNativeLower` (a write), `natEmitAssignLocal` (a write) and
  `natNativeHomeParams` (a parameter write). The array and record paths
  address their own areas and are not locals.

* The frame layout is CENTRALISED: `natFrameLayout` computes the locals
  region, the bin temporary area, the string stage, the map stage, the extras
  area, the Windows rounding and the argument spill, in that order. The
  allocator must not introduce a second frame calculation.

### 1.3 The recorded boundary

The NAT-IF gate pins the boundary between the two engines today: a function
with **four** simultaneously-live int locals and control flow is byte-identical
(454 bytes both sides), and with **five** it is not (469 against 471). The
difference is that the oracle allocates and the driver does not.

---

## 2. The design

### 2.1 Where the allocation lives

**In the PLAN**, as a fifth field per slot.

The plan is per FUNCTION and is already threaded to every place that needs it,
so putting the allocation there needs no new parameter and no new global.
The width change touches exactly four accessors and two construction sites -
`natNativePlanFor` and `natNativePlanDecls` - and nothing else in the tree
reads the plan's layout directly.

The fifth field is the allocated REGISTER NUMBER, or -1 for "keep the frame
slot". The existing four keep their meaning, so every pre-existing behaviour
is preserved by construction.

### 2.2 The allocator

The driver's allocator mirrors the oracle's:

* the same allocatable set (`R12`-`R15`), the same reserved registers
  (RAX is the expression accumulator; RBX is the array and record base;
  RCX/RDX/RSI/RDI/R8-R11 are the helpers' scratch);
* the same engagement gate: control flow AND a peak liveness of at least five;
* the same linear scan over live ranges, in the plan's slot order (which is
  the declaration order, i.e. first definition order);
* the same no-split rule: a candidate that cannot get a register keeps its
  frame slot, which is the pre-existing path and needs no new code.

### 2.3 The liveness analysis

A point-ordered walk of the function's statements, exactly as the oracle's:
every statement and every expression advances the point counter, an
identifier touches its local, and a loop widens the ranges of the names it
mentions to span the loop. The peak is the maximum number of overlapping
ranges.

The driver's statement shapes are its own (`NODE_PRINT`, `NODE_IF`,
`NODE_WHILE`, `NODE_FOR`, `NODE_EXPR_STMT`, a declaration, `NODE_RETURN`),
so the walk is written against those - it is NOT a copy of the oracle's Go
code, it is the same ALGORITHM over the driver's own AST.

### 2.4 The access sites

Each of the four sites asks the plan for the slot's register and falls back to
the frame slot:

| site | allocated | not allocated |
|---|---|---|
| binding (`let`) | `mov reg, rax` | `mov [rsp+off], rax` |
| identifier read | `mov rax, reg` | `mov rax, [rsp+off]` |
| assignment | `mov reg, rax` | `mov [rsp+off], rax` |
| parameter homing | `mov reg, argreg` | `mov [rsp+off], argreg` |

### 2.5 The prologue and the epilogue

The claimed registers are pushed BEFORE `sub rsp, frame` and popped in exact
reverse order AFTER `add rsp, frame`, so the frame's displacements never
move. A function that did not qualify claims nothing and its prologue is
byte-for-byte what it was.

### 2.6 The switch, and the fallback

The assignment requires the stack-based path to stay available as a
correctness fallback and a comparison mechanism.

**The switch is the environment variable `KARKAIN_NATIVE_REGALLOC=1`**, read
by the Go CLI, which passes `--regalloc` to the self-hosted engine, which
passes it to the driver. The DEFAULT is the stack-based path.

That default is the safe one for a release: the register-allocated path is
new code in a self-hosted compiler, and it becomes the default only when it
has passed the complete qualification suite - which is the assignment's own
rule (section 7).

---

## 3. What the slice does NOT do

* It does not split live ranges. A local owns its register for its whole
  range or keeps its slot.
* It does not shrink the frame. A spilled local keeps its slot.
* It does not allocate a register for a string, an array or a record: only
  INT locals are candidates, and a two-unit value is never promoted.
* It does not allocate a register for an expression temporary: the
  expression evaluator's temporaries stay in the frame's `binTemp` area.
* It does not introduce a second frame-layout calculation.
* It does not introduce global mutable state: the allocation is per function
  and lives in that function's plan.

---

## 4. Files

- `src/compiler/native_value.kark` - the allocation, the liveness walk, the
  four access sites, the prologue and the epilogue.
- `pkg/cli/phase151n_regalloc_test.go` - the gate (prefix `TestPhaseNATReg_`).
- `pkg/cli/kcc_native.go`, `cmd/karkain/main.go`, `src/compiler/main.kark` -
  the `--regalloc` switch.
- `.github/workflows/ci.yml` - the gate's step.
- `docs/audit/NAT-REG-BASELINE.md` - the slice's record.
