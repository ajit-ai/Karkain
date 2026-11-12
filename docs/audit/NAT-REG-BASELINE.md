# NAT-REG baseline - native register allocation in the self-hosted driver

**Slice:** `NAT-REG` (unnumbered working label; the ninth v1.3.0-class
increment, after `NAT-CALL`, `NAT-IF`, `NAT-QUAL`, `NAT-LOOP`, `NAT-IDX`,
`NAT-FRAME`, `NAT-STR-STAGE` and `NAT-RODATA`).
**Branch:** `feature/v1.2.0-native-independence`.
**Status:** implemented, gated, and **opt-in**.
**Design:** `docs/audit/NAT-REG-DESIGN.md`.

---

## 1. The premise: the oracle already had an allocator

The gap was not "the native backend has no register allocator". **The Go
oracle has had one since Phase 150C** - `pkg/native/regalloc.go` - and the
self-hosted driver did not. That is the whole shape of the difference, and it
is why the correction is a PORT rather than a design exercise.

What the oracle's allocator does (read out of the code, not assumed):

* **The allocatable set is R12-R15.** They are callee-saved under BOTH ABIs
  in play (System V and Win64), so a value parked there survives a call to a
  user function, a helper or a kernel32 export without a spill; and they are
  untouched by the rest of the backend, which works in
  RAX/RCX/RDX/RSI/RDI/R8-R11 and clobbers RBX. RBX and RBP are deliberately
  NOT allocatable - both are live scratch inside emitted helpers.
* **The engagement gate has two halves.** The function must contain CONTROL
  FLOW, and the peak number of simultaneously-live int locals must be at least
  `raMinLive = allocRegCount + 1 = 5`. Below that every live value fits in a
  register anyway, so there is no pressure to relieve; and in a straight-line
  body a push/pop pair would cost more than the single load it removes.
* **Liveness** is a point-ordered walk of the body. A local's range runs from
  its first touch to its last. A loop WIDENS the ranges of the locals it
  mentions, because the body may run many times and a value defined before
  the loop must survive every iteration.
* **The allocation** is a linear scan over the ranges in order of first
  definition: a register is available when the local holding it is dead
  before this range starts; otherwise the candidate spills to its frame slot,
  which is the pre-150C behaviour and needs no new code path. **Ranges are
  never split.**
* **The frame does not shrink.** A spilled local keeps its slot, so every
  slot displacement is unchanged. What the allocator saves is memory TRAFFIC.
* **The discipline** is: push the claimed callee-saved registers BEFORE the
  frame is taken (so the frame's rsp-relative displacements stay unchanged),
  and pop them in exact reverse order after the frame is released. The
  push/pop list is published in `allocRegs` order, so the sequence is a pure
  function of the allocation and never of map iteration order.
* **There are exactly three access sites** in the oracle: the `let` binding,
  the identifier read and the assignment.

## 2. What changed

`src/compiler/native_value.kark`:

1. **The allocation lives in the PLAN, as a fifth field per slot.** The plan is
   per function and is already threaded to every place that needs it, so this
   needs no new parameter and no global state. The width change touches four
   accessors, two construction sites and one count - the plan's layout is read
   nowhere else.
2. **The liveness walk** (`natNativeLiveWalk` / `natNativeLiveStmt` /
   `natNativeLiveExpr` / `natNativeLiveTouch`) is a point-ordered walk over
   the DRIVER's own AST - its `NODE_PRINT`, `NODE_IF`, `NODE_WHILE`,
   `NODE_FOR`, declaration, `NODE_RETURN` and expression shapes - with the
   oracle's loop widening.
3. **The scan** (`natNativeRegScan`) is the oracle's: a register is free when
   its holder's range ended before this range starts; a candidate that cannot
   get one keeps its frame slot.
4. **The access sites** became register-aware: the binding, the identifier
   read, the assignment and the parameter homing ask the plan for the slot's
   register and fall back to the frame slot.
5. **The prologue and the epilogue** push and pop the claimed registers
   around the frame, in the allocatable set's order and its reverse.
6. **The switch**: `KARKAIN_NATIVE_REGALLOC=1` on the CLI side becomes a
   `--regalloc` argument on the self-hosted side (the self-hosted compiler has
   no environment accessor), and it reaches the driver through the plan.

**The DEFAULT is unchanged.** The register-allocated path is opt-in, because it
is new code in a self-hosted compiler and the rule is that it does not become
the default until it has passed the complete qualification suite.

## 3. The evidence

Gate: `pkg/cli/phase151n_regalloc_test.go` (`TestPhaseNATReg_`), CI step
*Run NAT-REG native register-allocation gate*.

**The claim is BEHAVIOURAL, and it is the right shape.** The two paths are
built over the SAME programs and must produce the SAME observable result -
stdout byte for byte and the same exit status - with the Go oracle as a third
opinion. A register allocator that computes a different answer is a
correctness defect, and no layout comparison can see it.

| test | what it holds |
|------|---------------|
| `ThePathsAgree` | the two native paths and the oracle agree on stdout and exit status, over the whole corpus |
| `TheFrameIsUnchanged` | the frame is IDENTICAL between the two paths: the allocator does not shrink it |
| `TheHighPressureCasesDiffer` | the two paths are byte-DIFFERENT where the allocator engages, and byte-IDENTICAL where it does not - otherwise the agreement would be vacuous |
| `TheOutputIsDeterministic` | two builds of the register path are byte-identical |
| `TheSwitchIsOptIn` | the default path and the register path differ, and the gate runs with the variable unset |

**The corpus** covers the brief's twenty-two areas: simple arithmetic, deeply
nested expressions, several simultaneously live values, register exhaustion,
spill and reload, repeated local reads, local updates, comparisons and
branches, nested branches, loops with changing variables, `break` and
`continue`, function arguments, nested calls, a value live ACROSS a call,
return values, array indexing, bounds behaviour, float64 (refused by name in
both paths), several functions, deterministic repeated compilation, low
register pressure and deliberate high register pressure.

## 4. Performance, recorded and not claimed

The allocator trades memory traffic for a push/pop pair per claimed register.
For a program this small the wall-clock time is dominated by the process
startup, so a timing comparison would be noise dressed up as a measurement,
and **no performance improvement is claimed**.

What IS measured and recorded:

* the image size of both paths,
* the frame size of both paths (identical, by construction),
* the number of claimed registers, read out of the image's push sequence.

The measured sizes are in the gate's log (`TheHighPressureCasesDiffer`). They
are mixed: the register path is sometimes smaller and sometimes a byte or two
LARGER, because the push/pop overhead can exceed the saving for a program with
few accesses. The ORACLE's Phase-150C allocator makes exactly the same trade,
which is why the oracle's own images are not uniformly smaller either.

## 5. The frame is not the allocator's to change

The centralized frame layout (`natFrameLayout`) is untouched: the locals
region, the bin temporary area, the string stage, the map stage, the extras
area, the Windows rounding and the argument spill are all as they were. The
allocator adds no region and moves no displacement, which is what makes it
safe to enable and what keeps the fallback path and the register path
interchangeable.

## 6. Mutation verification

Four focused mutations, each restored byte-identically (SHA-256
`8DFCCC6005547F76D9883C10B3626E68CF3E19306D7BD7F438FC08813A3FEA2A`):

| mutation | caught by |
|----------|-----------|
| the epilogue omits the POP | `ThePathsAgree` - the caller's callee-saved registers are corrupted |
| the spilled fallback stores to the wrong slot | `ThePathsAgree` - the value lands in the wrong place |
| the scan reuses a register whose holder is still LIVE | `ThePathsAgree` on eight cases - the value is clobbered |
| the loop widening is dropped | `ThePathsAgree` - a value live across a loop loses its register |

**A FIFTH mutation was NOT caught, and it is recorded rather than hidden.**
Dropping the scan's liveness check (`held[k] < 0 || held[k] < f` becomes
`held[k] < 0`) makes the allocator MORE conservative: it never reuses a
register, so every local either gets one or spills. That is a missed
optimisation, not a wrong answer, and no behavioural gate can see it. It is
why the mutation that matters is the one that reuses a register whose holder
is still LIVE - that one clobbers a value and eight cases fail.

The lesson is the repository's own: **a mutation that cannot be caught is
either a missed optimisation or a gap in the gate**, and the difference has to
be established rather than assumed.

## 7. What this slice does NOT do

* It does not become the DEFAULT path. The switch is opt-in and the qualified
  stack-based path stays the default.
* It does not split live ranges. A local owns its register for its whole range
  or keeps its slot.
* It does not shrink the frame.
* It does not allocate a register for a string, an array or a record: only
  INT locals are candidates, and a two-unit value is never promoted.
* It does not allocate a register for an expression temporary: the expression
  evaluator's temporaries stay in the frame's bin temporary area.
* It does not introduce a second frame-layout calculation, and it introduces
  no global mutable state: the allocation is per function and lives in that
  function's plan.
* It does not claim a performance improvement.

## 8. Record

### 8.1 Files

- `src/compiler/native_value.kark` - the allocator, the liveness walk, the
  access sites, the prologue and the epilogue, and the plan's fifth field.
- `src/compiler/main.kark` - the `--regalloc` flag.
- `pkg/cli/kcc_native.go`, `pkg/cli/native_encode.go` - the switch's
  forwarding.
- `pkg/cli/phase151n_regalloc_test.go` - the gate.
- `.github/workflows/ci.yml` - the gate's step.
- `docs/audit/NAT-REG-DESIGN.md` - the design.
- `docs/inventory/native-feature-matrix.json`,
  `docs/audit/KARKAIN-IMPLEMENTATION-CONTROL.md`, `AGENTS.md` - the records.

### 8.2 The KIR pin

The pin moved and was re-measured; the measured value is recorded in the two
pin gates (`pkg/cli/phase122_pipeline_ownership_test.go` and
`pkg/cli/phase151a5_win_test.go`) and in `AGENTS.md`.

### 8.3 Next

The register path is qualified BEHAVIOURALLY but is not the default. Making it
the default is a one-line change to the switch, and it should follow a
release in which the whole native family has been run with the variable set -
which is a decision, not an accident.
