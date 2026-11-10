# NAT-LOOP baseline — loops in the kcc native driver

**Slice:** `NAT-LOOP` (unnumbered working label; the fourth v1.3.0 increment,
after `NAT-CALL`, `NAT-IF` and `NAT-QUAL`).
**Branch:** `feature/v1.2.0-native-independence`.
**Status:** implemented; gate written; the measured results are in §7.
**Parent:** `NAT-IF` (`docs/audit/NAT-IF-BASELINE.md`).

---

## 1. The gap this slice closes

Measured on the tree this slice started from (`cf58cbc`): the native driver
lowered **no loop at all**. A `while` was refused:

```kark
func main() {
    let i = 0
    while i < 3 {
        print(i)
        i = i + 1
    }
}
```

```
error[K145]: native target: `while` statement is not lowered by the native driver in `func main`
exit 3, no image written
```

The Go oracle builds the same program and prints `0`, `1`, `2`. So the gap was
one of *native lowering*, not of the language, the parser or the checker.

A loop is the first construct in this backend whose **correctness** depends on
a control-flow structure rather than on a frame layout, and the first whose
**memory behaviour** depends on an argument about how many times a site runs.
Both are why this slice has more to prove than NAT-IF did.

---

## 2. The semantics, taken from the reference implementation

Nothing here is invented. The semantics are the Go oracle's
(`pkg/native/program.go`), which is the reference the C backend also agrees
with, and they are reproduced exactly.

### 2.1 `while` (`program.go:5234-5251`)

```
loopLabel = fresh("while"); endLabel = fresh("whileend")
Mark(loopLabel)
push loops{brk: endLabel, cont: loopLabel}
emitCond(cond, endLabel)      // jump to endLabel when the comparison FAILS
body
pop
Jmp(loopLabel)
Mark(endLabel)
```

* The condition is evaluated **before every iteration**, including the first.
* `break` targets the loop's **end** label.
* `continue` targets the loop's **head**, so the condition is evaluated again.
  The body is not re-run, but the condition is.

### 2.2 C-style `for` (`program.go:5256-5295`)

```
init                                  // emitted BEFORE any label
loopLabel = fresh("for"); postLabel = fresh("forpost"); endLabel = fresh("forend")
Mark(loopLabel)
push loops{brk: endLabel, cont: postLabel}
cond (if present)
body
Mark(postLabel)
post
pop
Jmp(loopLabel)
Mark(endLabel)
```

* The **init runs once**, before the loop. A `let` in the init is a real
  binding with a real frame slot.
* The **post runs after every iteration**, including after a `continue`.
* `continue` targets the **post label**, not the head — so a `continue` in a
  `for` runs the update clause and then re-evaluates the condition. That is
  the asymmetry with `while`, and it is the oracle's (`program.go:5266`).
* The post is parsed as a bare expression; the oracle wraps a bare `=`
  BinaryExpr in an `ExprStmt` before lowering it (`program.go:5282-5285`), and
  so does this driver.

### 2.3 `break` and `continue` (`program.go:4913-4923`)

Both act on the **nearest** enclosing loop, through a loop-context stack:

```go
b.e.Jmp(b.loops[len(b.loops)-1].brk)   // break
b.e.Jmp(b.loops[len(b.loops)-1].cont)  // continue
```

Outside a loop both are refused, with the words
`break outside of a loop` / `continue outside of a loop`.

---

## 3. The loop-context mechanism

The oracle keeps the loop contexts on the **builder** (`b.loops`), not on the
emitter. kcc has ONE object where the oracle has two — the builder and the
emitter are the same state array — so the stack lives in that state array as
**slot 17**, as a flat pair of label names per loop (break target, then
continue target). This language has no structs, and a flat pair is the same
convention the four-slot plan uses.

The stack is what makes "the nearest enclosing loop" true rather than
approximate: an inner loop pushes its own targets, and the outer loop's
targets survive it intact. `natLoopPop` rebuilds the stack without its last
two entries, because this language's arrays are values and there is no slice
operator.

The mechanism is the oracle's, not a set of ad-hoc jump patches: `break` and
`continue` read their target from the stack, they do not carry one.

---

## 4. Memory and arena safety — the part that decides whether a loop can be
lowered at all

### 4.1 What allocates, and where

| Allocation kind | Where it lives | Inside a loop? |
|---|---|---|
| compiler-time (the compiler's own AST/arena) | the compiler process | irrelevant — it is not the generated program's memory |
| frame slots (every int local) | the generated function's stack frame | **fine** — a slot is written, not allocated |
| array literal elements | the frame (Phase 150A) | **fine** — static storage |
| record field areas | the frame (Phase 150B3b) | **fine** — static storage |
| string literal bytes | the static `.rodata` section | **fine** — static storage |
| the heap **arena** | the arena region inside the image | **NOT fine** — the arena is sized at compile time |
| map entries | the arena | **NOT fine** — the map's capacity is sized at compile time |

### 4.2 The rule the oracle already has

The arena's size is computed at compile time from the program's allocation
sites, on the assumption that **each site runs at most once**. A loop breaks
that assumption: the same site runs an unbounded number of times, so the
bound stops being provable.

The oracle therefore refuses exactly three constructs inside a loop, with the
reason in the message:

* `error[K145]: string concatenation inside a loop is not supported on the native target (the heap arena is sized at compile time, so a repeated concatenation cannot be bounded)` (`program.go:6629`)
* `error[K145]: push() inside a loop is not supported on the native target (the heap arena is sized at compile time, so a repeated push cannot be bounded)` (`program.go:5487`)
* `error[K145]: map insertion inside a loop is not supported on the native target (map capacity is sized at compile time, so a repeated insert cannot be bounded)` (`program.go:4136`)

This driver refuses the same three with the same words, and the refusal is
**by name** — "not supported" alone would leave the reader guessing whether
the program is wrong or the toolchain is.

The detection walks the **whole** loop body, including nested branches and
nested loops: a concatenation inside an `if` inside a loop is still a
concatenation inside a loop.

### 4.3 What was NOT done, and why

* **No arena checkpoint/restore.** An iteration-scoped checkpoint would be the
  alternative, but it requires proving that no live reference can escape an
  iteration — and in this value model an array's element area is reachable
  through the array's header, so a value bound before the loop could still
  point into an iteration's allocation. Proving that is a separate analysis,
  and the honest answer here is to refuse the construct rather than to
  implement an unsound checkpoint.
* **No fixed iteration limit.** A limit would turn an unbounded allocation
  into a wrong answer instead of a diagnostic.
* **No larger arena.** The arena bound is not touched by this slice, because
  no construct inside a loop allocates.

### 4.4 The consequence

Every value a loop touches is a **static** frame slot or a static section.
There is no per-iteration allocation, so nothing can escape an iteration, and
the compile-time arena bound keeps its meaning: every allocation site is
outside a loop, so each still runs at most once.

---

## 5. Frame layout and the register-allocation boundary

The frame is the existing one: the plan allocates a slot per binding, and the
loop's bindings live in it exactly as any other local does. A back-edge does
not move `rsp`, so a value written in an iteration is still at the same
address in the next one.

The oracle's Phase-150C register allocator engages when a function has control
flow **and** more than `raMinLive` (= `allocRegCount + 1` = 5) simultaneously
live int locals. A loop is control flow, so the same boundary that NAT-IF
pinned applies here and is **not** concealed:

* four locals and a loop → **byte-identical**;
* five locals and a loop → **different bytes**.

The self-hosted driver has no allocator: every local is a frame slot, the
pre-150C path, and always correct. The difference is an encoding strategy,
not a wrong answer, and the five-local case is in the execution corpus so
"different bytes" is backed by "same behaviour".

This increment deliberately does **not** implement a register allocator. That
is the separate `NAT-REG` increment.

---

## 6. What this slice does NOT lower

| Construct | Why | Message |
|---|---|---|
| `while` / `for` with a non-comparison condition | the oracle's `emitCond` accepts only the six comparisons over ints or floats | `condition must be a comparison (...) over ints or floats` |
| a string or float comparison in a condition | the self-hosted driver's string layer is main-only and it has no float statement lowering | the operand's kind is named |
| a string **binding** inside a loop body | the rodata cursor is threaded per statement list in this driver | `is not an integer expression` |
| an **array index** inside a loop body or condition | the index path carries the runtime-error reporter's rodata offsets, and the shared integer expression walker does not carry them | `is not an integer expression` |
| a `for` whose post is not an assignment | the post is the update clause | `a `for` post must be an assignment, not ...` |
| `break` / `continue` outside a loop | there is no loop to leave | `break outside of a loop` / `continue outside of a loop` |
| a string concatenation, a `push()` or a map insertion inside a loop | the arena bound (§4) | the oracle's three messages, verbatim |
| `for ... in` iteration | a separate construct with its own iterator lowering | `for-in statement is not lowered ...` |

The array-index limitation is the one that will be felt first: iterating over
an array by index is the most natural loop. It is a REFUSAL, not a
miscompilation, and it is a separate increment's work (threading the reporter's
state into the expression walker).

---

## 7. The gate

`pkg/cli/phase151i_loop_test.go`, tests named `TestPhaseNATLoop_*`, with its own
`ci.yml` step (*Run NAT-LOOP kcc native loop-control-flow gate*).

Seven layers, each answering a question the others cannot:

1. **Executed PE goldens.** 21 programs, each compiled through the kcc native
   driver, written to disk and EXECUTED on this host, stdout compared as
   **exact bytes** and the exit code separately.
2. **A DIFFERENTIAL AGAINST THE C BACKEND.** The same source is compiled the
   old way — through the C code generator and gcc — and run, and its stdout
   is compared byte for byte with the native image's. This is the layer that
   proves the *semantics*: a `continue` that skipped the update clause, a
   `break` that left the wrong loop, or a condition evaluated one time too
   many would all show up here and nowhere else. The C backend is the
   reference and is **not** modified.
3. **A Linux ELF byte-identity DIFFERENTIAL** against the Go oracle, for every
   corpus case except the one marked `expectDiff` (§5) — whose difference is
   **asserted**, not skipped.
4. **A refusal table** — 8 rows, each asserting the exit code, that the
   message NAMES the construct, and that **no image was written**.
5. **The register-allocator boundary**, asserted from both sides.
6. **Determinism and non-vacuity**, plus the CI-wiring check.
7. **Process-level INDEPENDENCE evidence** for a loop program: the same
   PATH-shim method as NAT-QUAL, with the legacy build path as the positive
   control.

### 7.1 Measured results

Filled in §9.

---

## 8. Risks, named before the code

| Risk | Why it is real | Mitigation |
|---|---|---|
| The `for`'s `continue` skips the update clause | the oracle's continue target is the POST label, not the head | the C-backend differential compares the exact byte sequence |
| A `break` leaves the wrong loop | nested loops need independent targets | the loop-context STACK, plus `break_from_inner_loop` in the corpus |
| A binding inside a loop gets no slot | the layout pass must descend into the body | the plan descends exactly as the oracle's `scanLets` does, and the byte-identity differential checks the frame |
| The arena bound silently becomes wrong | a loop re-runs an allocation site | the three arena constructs are refused BY NAME, and the refusal walks nested bodies |
| A loop changes every frame displacement | the plan's slot ORDER decides the offsets | the plan descends in the oracle's order (init before body), and the byte-identity differential checks every frame |
| The register allocator makes the two engines differ | control flow engages the oracle's Phase-150C allocator | the boundary is measured and pinned from both sides, not concealed |

---

## 9. Record

### 9.1 Measured results

* **Gate:** `TestPhaseNATLoop_*` — 8 layers, 21 corpus programs, 8 refusal
  rows. Green: `ok karkain/pkg/cli 137 s` (the gate alone), and 396 s for the
  gate together with the whole frozen family.
* **Every corpus program executes** on this Windows host as a kcc-produced PE
  with **exact** stdout bytes.
* **Every corpus program is byte-identical** to the Go oracle's Linux ELF —
  including the nested loops, the `break`/`continue` cases and the `for`-init
  binding. The single `expectDiff` case (the register-allocator boundary) is
  **asserted to differ**, not skipped.
* **Every corpus program produces the SAME OUTPUT as the C backend** — the
  layer that proves the semantics rather than the bytes.
* **Refusal table:** 8 rows, each asserting the exit code, that the message
  names the construct, and that **no image was written**.
* **Register-allocator boundary:** 3 locals and a loop are byte-identical;
  8 locals and a loop are not. Asserted from both sides.
* **Determinism and non-vacuity** ✓.
* **Independence:** the native build's shim log is **empty**; the legacy
  build's log is **not** (the positive control) ✓.
* **Frozen gates re-run, all green in one invocation (396 s):**
  `TestPhase151A9C_`/`9D_`/`9E_`/`9F_`/`9G_`/`9H_`, `TestPhase151B9_`,
  `TestPhaseD2_`/`D5_`/`D8_`, `TestPhase151D_`, `TestPhaseNATCall_`,
  `TestPushMain_`, `TestPhaseNATIf_`, `TestPhaseNATQual_`,
  `TestPhaseNATLoop_`.
* **KIR pin:** 14597 → **14792** (+195). Measured with
  `karkain kir --verify src/compiler/kir.kark` in the gate's own invocation
  (`text=14792 verify=14792`) and never predicted.

### 9.2 Defects the gates found

Every one of these was found by a gate, and none by inspection.

1. **A `while` corpus program that never terminated.** The first draft's
   `while_zero_iterations` was `while i < 3 { print(1) }` — a body that does
   not advance the counter, so the program is an infinite loop. It HUNG the
   gate, because the shared `nat9dRun` helper has no timeout. Two fixes, both
   kept: the corpus case's bound became `i > 3` (false on the first
   evaluation, so zero iterations and a terminating program), and the loop
   gate runs its programs under a **20-second timeout** of its own. A loop
   corpus has to be checked for TERMINATION, not only for its expected
   output — and a hang is a worse failure than a red test, because it takes
   the whole suite with it.
2. **Two hand-computed goldens were wrong, and the compiler was right.**
   `break_from_inner_loop` and `continue_in_inner_loop` expected
   `0,1,0,1` and `0,0,1,0,0,1`; the correct outputs are `0,1,0,0,1,1` and
   `0,2,0,0,2,1` — the outer loop's counter is `1` on its second iteration,
   and the inner loop skips `1` because of the `continue`. The native path
   produced the right answers in both cases; the expectations were mine and
   were wrong. Recorded because a gate whose goldens are hand-computed needs
   that said out loud.
3. **A `for` post that is a CALL expression is mis-parsed by BOTH engines.**
   `for (let i = 0; i < 3; print(i))` — the Go parser reports "unexpected
   token ')'" and the self-hosted parser silently produces a **Block node**,
   which the driver then refuses with "statement kind 10 is not lowered".
   This is a **parser** defect in both engines, not a driver one, and it
   predates this slice. It is recorded here because this slice found it; the
   fix belongs to a parser increment, and the refusal is loud rather than a
   silent miscompilation ✓.
4. **The two engines disagree about a C-style `for`'s init SCOPE.** The Go
   engine's borrow checker scopes the init binding to the loop ("use of value
   'i' after its scope has ended"), while the self-hosted checker scopes it
   to the function. So a program that reads the counter after the loop is
   REFUSED by the C path and ACCEPTED by the native path — a pre-existing
   divergence between the engines, not something this slice introduced. It
   is why the corpus's `break_from_for` does not read the counter after the
   loop: such a program cannot be compared against the oracle at all.
5. **The C backend's stdout is CRLF and the native program's is LF.** The C
   runtime's stdout is in Windows text mode, so its `\n` reaches the terminal
   as `\r\n`; the native program writes raw bytes. The differential therefore
   compares the PROGRAMS' output with the C path's CRLF normalised, and says
   so in the failure message. The native path's own exact bytes are still
   pinned by layer 1 against a golden with **no** normalisation at all, so
   the byte-level claim is untouched.
6. **A plain-local assignment arrived as an expression statement, not as a
   bare `=`.** The parser wraps `i = i + 1` in an `ExprStmt`, so the
   assignment reached the driver's EXPR_STMT arm, which only knew the
   record-field form. Both the scan and the lowering now dispatch on the
   target's shape.

### 9.3 Mutation verification

Three mutations, each restored **byte-identically** (the file's SHA-256 after
every restore: `7EA6FC42B34F19C0F80E1FAB3F313B0BD49AEF51E9E302E6439CEC217B2958B6`),
with the self-hosted source's mtime bumped so `kccStale` cannot reuse a
mutated binary.

| # | Mutation | Observed |
|---|---|---|
| M1 | the C-style `for`'s CONTINUE target changed from the post label to the loop head | the execution layer and the C-backend differential both fail — a `continue` that skips the update clause, which is exactly the asymmetry between `while` and `for` |
| M2 | the plan's descent into a loop body removed | the execution and byte-identity layers fail with "undeclared local `j`" — a LOUD refusal, not a silent slot collision |
| M3 | `break` made to jump to the continue target | the execution layer prints `10` where the reference prints `3`, and the differential fails |

M1 is the instructive one: it is the difference between a `continue` that
re-evaluates the condition and one that runs the update clause, and only the
**C-backend differential** can see it — the program's own output differs, but
nothing about the image's structure says which is right.

### 9.4 The slice's own boundary, stated

**No execution evidence exists for the Linux ELF or the Mach-O image of this
slice.** This host is `windows/amd64` with no WSL and no Docker, and the
repository has no macOS runner. The Windows PE images are executed; the Linux
and macOS images are validated structurally by the Go oracle's own parsers and
compared byte for byte against the oracle.

**The compiler's own build still uses `gcc`**, and the bootstrap proof
(increment 154) is still carried to v1.3.0 as UNMET. What this slice proves
is narrower and is stated precisely: *compiling a user program with a loop to
a native image* invokes neither a C compiler nor the Go toolchain.

### 9.5 Next

**`NAT-REG` — the register allocator.** The oracle's Phase-150C allocator
engages under control flow and pressure, so from five live locals upward the
two engines encode the same program differently. Growing the allocator for the
self-hosted driver would close that, and it is the one piece of the native
value model that is genuinely missing.

Two smaller candidates, each smaller than the allocator:

* **An array index inside a loop body or condition.** It is refused today,
  and it is the most-felt limitation of the qualified subset: iterating over
  an array by index is the most natural loop. The work is to thread the
  runtime-error reporter's rodata offsets into the shared integer expression
  walker, so the index path can be lowered there as well as in `print` and
  `return`.
* **`for ... in`.** Its iterator lowering has never been ported.

And one **parser** defect, found by this slice and belonging to no slice yet:
**a call expression in a C-style `for`'s post is mis-parsed by both engines**
(the Go parser errors; the self-hosted parser produces a Block node). It is
loud rather than silent, so it is not urgent, but it is a real hole in the
