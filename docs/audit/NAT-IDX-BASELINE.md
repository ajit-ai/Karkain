# NAT-IDX baseline — array indexing inside the kcc native driver

**Slice:** `NAT-IDX` (unnumbered working label; the fifth v1.3.0 increment,
after `NAT-CALL`, `NAT-IF`, `NAT-QUAL` and `NAT-LOOP`).
**Branch:** `feature/v1.2.0-native-independence`.
**Status:** implemented and gated.

---

## 1. The limitation, diagnosed rather than assumed

NAT-LOOP recorded the limitation in one sentence: *"the index path carries the
runtime-error reporter's rodata offsets, and the shared integer expression walker
does not carry them"*. Tracing the implementation showed that sentence covered
**two** separate defects, and that the second one was hiding a third.

### 1.1 The index was a literal-only special case

`natArrayIndex` took the index NODE and did:

```kark
s = natMovRegImm32(s, natRegRCX(), int(intLitVal(idx)))
```

It materialised a literal straight into RCX. An index that was a variable, or
an arithmetic expression, never reached that code — the SCANNER refused it
first (`natNativeScanIndex`: *"array index is not an integer literal"*).

The **oracle** has never required a literal. It evaluates the index as an
EXPRESSION through the shared walker and moves the result into RCX
(`pkg/native/program.go:6320-6324`):

```go
b.emitExpr(x.Index, depth+1)
b.e.MovRegReg(RCX, RAX)
b.e.LoadStack(RBX, off)
b.e.LoadStack(RAX, off+8)
```

So the kcc index path was a **literal-only reimplementation** of a sequence the
oracle computes generically. That is the whole of the limitation, and it is
why ordinary array iteration was impossible.

### 1.2 The reporter's context reached two positions and no others

The runtime-error reporter needs three values at an index site: the rodata
offset of its "kind" argument, and the source basename's offset and length.
Those reached the `print` and `return` paths — the only two places an index
could be lowered — and **no further**. An index inside an arithmetic
expression, a comparison, a call's arguments or a loop body had nowhere to get
them from.

### 1.3 The reporter's PREDICATE could not see an index either — and that was
the crash

`natNativeUsesRuntimeError` decided whether to emit the reporter at all. It
looked at a `print`'s argument and a `return`'s value, **at the top level of the
function**. An index inside a branch, a loop, a condition or a nested
expression was invisible to it.

The consequence was not a missing diagnostic. It was a **link-time crash**: the
bounds check's failure block CALLS `karkain_runtime_error`, so with no
reporter emitted the image contained a call into a label that was never marked.
Measured, not inferred:

```
runtime error: array index out of range at main.kark:11397
  stack:
    natPEStore64 (main.kark:11397)
    natPELink (main.kark:11427)
```

`natPEStore64` is the PE linker applying a patch whose position was recorded
against a text that had been laid out without the reporter. The crash was in
the LINKER, with nothing in the message to connect it to an index — which is
why the predicate is the third defect and not a footnote.

---

## 2. What this slice changes

### 2.1 The index is an expression

`natArrayIndex` no longer takes the index; the caller evaluates it through the
shared walker and moves it into RCX, exactly as the oracle does. The index
sequence is therefore the oracle's, byte for byte.

### 2.2 The reporter's context travels through the walker as ONE argument

The three reporter values are bundled into `rctx = [rtKindSlot, srcOff, srcLen]`
and passed to `natEmitIntExpr`, `natEmitIntCond`, `natEmitIntCall` and
`natEmitAssignLocal`.

* It is an **explicit parameter**, not global state: two functions in one image
  have different basenames, and a global would have to be saved and restored
  around every call.
* It is **one argument, not three** — the values are a property of the
  FUNCTION, not of a statement, so bundling them where the function's own
  parameters arrive keeps them out of every call site.
* The walker is **not duplicated**: the index arm lives in the ONE
  `natEmitIntExpr` that every integer expression already goes through.

### 2.3 The reporter's predicate became a real walk

`natStmtHasIndex` now descends into branches, loops, conditions, initialisers,
post clauses and declarations, and `natExprHasIndex` walks binary expressions,
unary expressions and call arguments. The predicate and the lowering can no
longer disagree about where an index can appear.

### 2.4 One validator for the index

The separate `natNativeScanIndex` was removed and its job moved into the
shared `natNativeScanIntExpr`. Two validators for one construct is how they
drift apart — and they HAD drifted: the old one said *"is indexed but is not an
array"* while the oracle says *"index target must be an array (got %s '%s')"*.
The consolidated validator uses the oracle's sentences, so the two engines now
agree about the same program.

---

## 3. Bounds semantics

The bounds check is the oracle's and was not changed: `0 <= i < len`, with the
negative arm and the too-large arm both reaching the Phase-100 runtime error.

| Case | Behaviour | Measured |
|---|---|---|
| index 0 of a 3-element array | the element | `3` ✓ |
| index 2 (the last) | the element | `7` ✓ |
| index == length | runtime error | exit 1, no stdout ✓ |
| index > length | runtime error | exit 1, no stdout ✓ |
| a NEGATIVE index | runtime error | exit 1, no stdout ✓ |
| an EMPTY array, index 0 | runtime error | exit 1, no stdout ✓ |
| an out-of-range access inside a loop | the elements before it are printed, then the runtime error | `3 5 7`, then exit 1 ✓ |

**NOTE ON THE DIAGNOSTIC TEXT:** the reporter writes to stderr, and the
Phase-152-B0 `GetStdHandle` limitation leaves that stream EMPTY on this host.
The **exit status** is therefore the observable, and the message is not asserted
by the gate. That limitation is recorded rather than papered over.

A negative index is reachable, but only through an assignment (`i = i - 1`):
main's `let` accepts literals only, so `let i = 0 - 1` is refused before the
index ever exists.

---

## 4. The byte-identity boundary, stated

The index ENCODING now matches the oracle's — the images are the same SIZE.
They are not the same BYTES, because of the **frame delta** the D2 gate
documents: the kcc driver rounds the locals region up to the next 16
(`round16`) while the oracle rounds it up to the next 16 **strictly greater**,
so the oracle's frame is 8 bytes larger whenever the locals region is already
aligned. A program with an array has an element area in its frame, so it
reaches that case.

That is the Phase-151A Step 9d MEASURED PARITY DELTA, it is the last entry in
the D2 known-non-parity list, and it is **not an indexing question**: closing it
moves `natFrameLayout`, which every frame displacement in the backend is pinned
against.

So array programs are compared **BEHAVIOURALLY** (against the C backend) and
the byte difference is **asserted** (the sizes must agree) rather than hidden.
The gate is explicit about this: it asserts the sizes are EQUAL, because a size
difference would mean something else moved.

---

## 5. The gate

`pkg/cli/phase151j_array_index_test.go`, tests named `TestPhaseNATIdx_*`, with
its own `ci.yml` step (*Run NAT-IDX kcc native array-indexing gate*).

| Layer | What it proves |
|---|---|
| `ProgramsExecuteWithExactBytes` | 16 programs, compiled by the self-hosted engine, written to disk and EXECUTED, stdout compared as exact bytes |
| `OutOfRangeIsARuntimeError` | 5 bounds cases: a non-zero exit and NO value printed, with the stdout produced *before* the failure asserted |
| `MatchesTheCBackend` | the same source compiled the old way and run, compared byte for byte — the behavioural differential |
| `RefusalsAreByName` | 3 shapes still refused, each by name, with no image written |
| `TheFrameDeltaIsTheOnlyByteDifference` | the images agree in SIZE with the oracle's |
| `DeterminismAndNonVacuity` | the same source twice → identical bytes; a different index → different bytes |
| `TheNativePathInvokesNeitherCNorGo` | the PATH-shim evidence, with the legacy path as the positive control |
| `GateIsWiredInCI` | this gate has a `ci.yml` step |

Every golden's expected stdout was **MEASURED**, not computed by hand — two of
NAT-LOOP's hand-computed goldens were wrong and the compiler was right.

---

## 6. An environmental flake, found and named

The C-backend differential failed intermittently on this host: the C path
reported *"C compilation failed: exit status 1"* with **empty stderr**, which is
the signature of a gcc that was **KILLED** — the host's memory class, not a
difference in the program. It reproduced twice in three runs and never on the
same source run standalone.

The gate now **retries a C COMPILATION failure** (up to three attempts) and
does **not** retry a program that fails at run time, because a real failure is
deterministic and would fail every attempt. The retry is not a way of hiding a
defect: it re-runs the SAME compilation, and a genuine compile error fails
every time and is reported with the CLI's message.

The child's stderr is now captured rather than discarded, so a failing
comparison reports the C path's own diagnostic instead of an unexplained
difference.

---

## 7. What this slice does NOT do

* **The register allocator** — unchanged, and the boundary is unchanged: the
  oracle's Phase-150C allocator engages under control flow and pressure, so
  programs with many live locals still encode differently.
* **The frame delta** — unchanged (§4).
* **A map or a string element** — the element types are int, as before.
* **`for ... in`** — still refused.
* **The two NAT-LOOP parser defects** — a call expression in a `for` post is
  still mis-parsed by both engines, and the two engines still disagree about a
  `for` init's scope. Neither blocks array indexing: the index path needs
  neither, and the corpus works around them. They remain isolated and reported.

---

## 8. Record

### 8.1 Measured results

* **Gate:** `TestPhaseNATIdx_*` — 8 layers, 16 corpus programs, 5 bounds cases,
  3 refusal rows. Green: `ok karkain/pkg/cli 162.9 s`, and green again in
  three subsequent runs (155 s, 157 s, 162 s) — the stability is recorded
  because the C-backend layer was flaky until §6 fixed it.
* **Every corpus program executes** on this Windows host as a kcc-produced PE
  with **exact** stdout bytes: the first element, the last, a variable index, an
  arithmetic index, iteration in a `while`, iteration in a C-style `for`, an
  element in an arithmetic expression, an element in a comparison, an index in a
  branch inside a loop, two arrays in one loop, an index with a `continue`, an
  index with a `break`, an element passed to a function, a nested index, and
  `len()`.
* **Every corpus program produces the SAME OUTPUT as the C backend** — the
  behavioural differential.
* **Bounds:** 5 cases, each a runtime error with a non-zero exit and no value
  printed: index == length, index > length, out of range inside a loop (the
  three valid elements are printed first, then the error), a negative index,
  and an empty array.
* **Refusal table:** 3 rows, each asserting the exit code, that the message
  names the construct, and that **no image was written**.
* **Image sizes agree with the oracle's** for every corpus program — the index
  encoding is the oracle's (§4).
* **Determinism and non-vacuity** ✓.
* **Independence:** the native build's shim log is **empty**; the legacy
  build's log is **not** (the positive control) ✓.
* **Frozen gates re-run, all green in one invocation (443.9 s):**
  `TestPhase151A9C_`/`9D_`/`9E_`/`9F_`/`9G_`/`9H_`, `TestPhase151B9_`,
  `TestPhaseD2_`/`D5_`/`D8_`, `TestPhase151D_`, `TestPhaseNATCall_`,
  `TestPhaseNATIf_`, `TestPhaseNATLoop_`, `TestPhaseNATQual_`,
  `TestPushMain_`, `TestPhaseNATIdx_`.
* **KIR pin:** 14792 → **14834** (+42). Measured with
  `karkain kir --verify src/compiler/kir.kark` in the gate's own invocation
  (`text=14834 verify=14834`) and never predicted.

### 8.2 Defects the gates found

1. **The reporter's PREDICATE could not see an index in a CONDITION**, and the
   consequence was a crash in the PE LINKER, not a diagnostic:

   ```
   runtime error: array index out of range at main.kark:11397
     stack:
       natPEStore64 (main.kark:11397)
       natPELink (main.kark:11427)
   ```

   The bounds check's failure block CALLS `karkain_runtime_error`, so with no
   reporter emitted the image contained a call into a label that was never
   marked, and the linker applied a patch against a text that had been laid out
   without it. The stack names the LINKER, with nothing in the message to
   connect it to an index — which is why the predicate, not the index path, was
   the third defect and not a footnote.
2. **The reporter's predicate could not see an index in a branch, a loop, a
   declaration or a nested expression either.** The same consequence: a missing
   reporter wherever an index can now appear.
3. **The C backend's gcc is intermittently KILLED on this host** (§6) — an
   environmental failure that presented as a semantic difference in the
   differential layer, and that the layer's own diagnostic could not explain.

### 8.3 Mutation verification

Three mutations, each restored **byte-identically** (the file's SHA-256 after
every restore: `C66E8FA5C0DE7F337A2D41B455DE110ABA6B302B1DB51062E3069FEB3626C2BA`),
with the self-hosted source's mtime bumped so `kccStale` cannot reuse a mutated
binary.

| # | Mutation | Observed |
|---|---|---|
| M1 | the index's move into RCX removed from the index arm | the execution layer fails — the bounds check then compares against an unrelated register |
| M2 | the reporter's predicate no longer looks at an `if` CONDITION | the LINKER CRASH returns, with the identical signature ("array index out of range at main.kark:11397" in `natPEStore64`), and the gate fails |
| M3 | the bounds check's `Jge` flipped to `Jle` | the bounds layer fails — an index equal to the length passes the check and the program reads past the array |

M2 is the instructive one: it reproduces the crash that this increment found,
which is the strongest possible statement that the gate covers the defect it
was written for.

A FOURTH mutation was attempted and abandoned: making the index literal-only
again made the corpus HANG rather than fail an assertion (the wild index read
produced a loop that never terminated), so it was detected by the gate's
per-program timeout instead of by a comparison. It is recorded because "the
mutation was caught, but by a timeout" is a weaker result than "caught by an
assertion", and pretending otherwise would be dishonest.

### 8.4 The slice's own boundary, stated

**No execution evidence exists for the Linux ELF or the Mach-O image of this
slice.** This host is `windows/amd64` with no WSL and no Docker, and the
repository has no macOS runner. The Windows PE images are executed; the Linux
and macOS images are validated structurally by the Go oracle's own parsers.

**The compiler's own build still uses `gcc`**, and the bootstrap proof is still
carried to v1.3.0 as UNMET. What this slice proves is narrower: *compiling a
user program that indexes an array, in a loop, to a native image* invokes
neither a C compiler nor the Go toolchain.

**Array programs are not byte-identical to the oracle** (§4) and the difference
is asserted, not concealed.

### 8.5 Next

**The frame delta.** It is now the ONLY thing standing between the array path
and byte identity with the oracle: the images are the same size and differ
only in the frame's size immediate. Closing it means reconciling the locals
region's rounding — the kcc driver rounds up to the next 16, the oracle up to
the next 16 *strictly greater* — and that moves `natFrameLayout`, which every
frame displacement in the backend is pinned against. It is a small change in
one function and a large one in its consequences, so it wants its own
increment.

**`NAT-REG` — the register allocator** — remains the largest genuine gap in
the native value model, and it is unchanged by this increment.

And the two **parser** defects NAT-LOOP found are still isolated and still
belong to a parser increment: a call expression in a C-style `for`'s post is
mis-parsed by both engines, and the two engines disagree about a `for` init's
scope. Neither blocks array indexing — the index path needs neither — and the
NAT-IDX corpus works around both.
