# PHASE D5 BASELINE — `array_index` byte-identity: the array helper set and the runtime-error reporter in kcc

Date: 2026-10-14
Version: 1.2.0 (Sovereignty I)
Baseline commit: `604b96b` (main) / `657ac99` (develop)
Status: **NOT STARTED — this document is the mandatory baseline, not a completion claim.**

## 1. Why this exists

Phase D4 closed `return_value` and `len_of_array`, leaving **seven of eight**
parity shapes byte-identical. The eighth, `array_index`, is the last known gap in
the increment's closure condition 4 ("native images are byte-identical between
the Go compiler and kcc wherever the gate requires parity").

D4 deliberately did **not** patch it, and recorded why in
`pkg/cli/phase151d_parity_test.go`. This document is the baselined work that
deferred decision points to.

## 2. Measured state at the baseline

| shape | kcc | go | identical |
|---|---|---|---|
| print_literal | 351 | 351 | yes |
| local | 359 | 359 | yes |
| two_locals | 387 | 387 | yes |
| string_literal | 355 | 355 | yes |
| concat (allocating) | 5136 | 5136 | yes |
| return_value | 353 | 353 | yes (D4) |
| len_of_array | 454 | 454 | yes (D4) |
| **array_index** | **496** | **1096** | **NO** |

KIR pin at baseline: **13755** (measured twice, never predicted).

## 3. Root cause — measured, not inferred

Both `.text` sections were dumped and mapped region by region (D4 record).
`karkain_main` is nearly identical — Go `sub rsp,0x290`, kcc `sub rsp,0x288` —
but **Go emits 12 `ret`s where kcc emits 4**.

### 3.1 The SECOND root cause, found during D5 and not foreseen by §3's first draft

The 209-byte gap was traced to an exact byte offset by locating each image's
runtime-error digit loop (`cqo; div rcx`): **Go at .text 579, kcc at 370** — a
constant delta of 209. The region Go has and kcc lacks was then disassembled and
matched against the Go source instruction by instruction:

```
49 89 f8   mov r8, rdi
48 31 c9   xor rcx, rcx
4d 8b 08   mov r8, [r8]
4c 39 c9   cmp r9, rcx
0f 8d ..   jge
48 c1 e0 05  shl rax, 5        ; ×32 = mapEntryBytes
48 83 c0 10  add rax, 16       ; mapHeaderBytes
48 8b 10   mov rdx, [rax]      ; candidate key
48 39 f2   cmp rdx, rsi
```

That is **`Builder.emitMapFind` (program.go) verbatim** — the map helper set
(`map_find` / `map_set` / `map_get`), emitted into an image whose program
contains **no map at all**.

The cause is `scanMapUsage`'s `isMapIndex` (program.go:730):

```go
isMapIndex := func(n parser.Node) bool {
    ix, ok := n.(*parser.IndexExpr)
    if !ok { return false }
    _, isVar := ix.Left.(*parser.Identifier)
    return isVar          // true for an ARRAY index as well as a map index
}
```

It decides "is this a map index" purely by the syntactic shape `name[...]`,
never consulting the receiver's kind. So `print(a[1])` on an **array** sets
`usesMap = true` and the Go backend emits 209 bytes of map runtime that the
program can never reach.

**Consequence for byte-identity:** kcc must reproduce the over-approximation.
This is not a kcc bug to be "fixed" — parity is defined as agreement with the
oracle, and the oracle's behaviour here is deliberate-looking but at minimum
load-bearing. Two candidate readings, and the choice is an explicit language
decision rather than an implementation detail:

1. **Reproduce it** (emit the map helper set whenever an `IndexExpr` on a
   variable appears, regardless of the receiver's kind). Zero divergence, but
   kcc inherits an over-approximation whose only justification is that the
   other engine has it.
2. **Tighten both engines** to consult the receiver's kind, which drops 209
   bytes from every array-using image on **both** sides and is strictly closer
   to what the program needs.

Reading 2 changes the Go backend's output, so it is **not** inside D5's scope
(D5's scope is kcc-native parity, and tightening the oracle would move every
frozen Go-side gold). It is recorded here as a finding for the owner.

### 3.2 The FIRST root cause

The Go backend also emits the Phase-152-B0 **runtime-error reporter**, which
152-B0 added as 379 lines to `pkg/native/program.go` and **no** `.kark` file. So
on the kcc target an out-of-range index still `Int3`-traps instead of reporting
`runtime error: array index out of range at <file>:<line>` and exiting 1.

So `array_index` needs **two** mechanisms ported, not one: the reporter (§4.2)
and the map helper set (§3.1). The first draft of this baseline named only the
reporter; the map helper set was found by disassembly during D5 and is recorded
here so the next attempt starts from the measurement rather than the guess.

## 3a. What the D5 attempt established (and was reverted)

A reporter port was built, verified to emit, and **reverted** — it was
incomplete because §3.1's helper set was still missing, so it could not reach
parity, and an unverified emission path is worse than none. What it established,
all of it reusable:

- **The gate must look INSIDE a statement.** `print(a[1])` is a `NODE_PRINT`
  whose *argument* is the index. A predicate comparing only the statement's own
  node type returns 0, the reporter is never emitted, and the lowering still
  records a reference to `karkain_runtime_error` — an undefined label. From the
  outside that reads as "the port had no effect". This is the evidence-discipline
  failure the project warns about, and it is now the reason §9 requires
  `scanRuntimeErrorUsage`'s full recursive walk (§4.4) rather than a two-case
  test.
- **`natStdErrHandle` cannot be written as a literal.** `0xFFFFFFF6` is
  4294967286, past this language's literal clamp — the same trap as Step 8b's
  `1<<63` sign mask, which silently produced a wrong immediate that compiled
  cleanly. It must be written as an arithmetic form (`0 - 10`) so a reader can
  see the relationship; `MovRegImm32` masks to 32 bits and `-10`'s two's
  complement *is* `0xFFFFFFF6`.
- **The rodata list is interned by FIRST USE, not fixed.** A hardcoded offset
  table cannot match an interning list whose order depends on emission order.
  The reporter's strings must be appended in the order the oracle emits them
  (helpers first, then the call site), and their offsets read back OUT of
  `natRodataSection`'s offset array.
- **`sourceBase()` is reachable** and the plumbing is small: `buildNativeFile`
  already holds `path`, `fileBaseName` already exists, and the driver call needs
  one extra argument.
- **The 8-byte frame difference is a SEPARATE issue.** Measured: Go main is
  `sub rsp,0x290` (656), kcc `sub rsp,0x288` (648), with identical `localBytes`
  read from the oracle source. This is the `natFrameLayout` rounding rule that
  Step 9d recorded and D1 superseded as "not the cause of the 39-byte delta" —
  it is real, and it is NOT yet closed. Parity cannot be reached while it
  stands.

## 3b. The D5 scope as revised

In scope: port the reporter **and** the map helper set, in the oracle's order,
with the map helper set gated on the same syntactic predicate `isMapIndex`
implements.

Out of scope, newly: closing the 8-byte frame-layout difference (§3a), which
touches `natFrameLayout` — frozen in Step 1 with every Step 1-9d displacement
pinned against it — and tightening `isMapIndex` (§3.1 reading 2), which changes
the Go backend.

**So `array_index` parity needs TWO further increments, not one.** D5 delivers
the reporter plus the map helper set; D6 delivers the frame-layout rule.

## 4. What the oracle actually emits

Read from `pkg/native/program.go` at the baseline, not inferred:

### 4.1 The call site — `emitArrayIndex` (program.go:6287-6335)

```
MovRegReg(RCX, RAX)          ; index
LoadStack(RBX, off)          ; array base
LoadStack(RAX, off+8)        ; array count
TestRegReg(RCX, RCX)
Jns(badLbl + "$neg")
Jmp(badLbl + "$fail")
Mark(badLbl + "$neg")
CmpRegReg(RCX, RAX)
Jge(badLbl + "$fail")
LoadScaled64(RAX, RBX, RCX, 8, 0)
Jmp(doneLbl)
Mark(badLbl + "$fail")
rodataRef(RDI, "array index out of range")
MovRegImm32(RSI, 25)
rodataRef(RDX, sourceBase())
MovRegImm32(RCX, len(sourceBase))
MovRegImm32(R8, line)
Call("karkain_runtime_error")
Mark(doneLbl)
```

**The two jump labels** are allocated by `b.fresh("idxbad")` / `b.fresh("idxok")`
so their NAMES depend on the emission counter. Label names and allocation order
are therefore load-bearing for every `rel32` in the image — the Step 3 record
established this, and a mutation there stayed byte-identical while failing only a
label-discipline gate.

The `$fail` block "never returns (it ends in exit(1))" but the emitter does not
assume that, so `Mark(doneLbl)` exists. Its absence is recorded as a real past
regression (every array-using PE golden ran the diagnostic on a **valid** index).

### 4.2 The reporter — `emitRuntimeErrorHelper` (program.go:3712-3793)

64-byte frame; five argument units homed at +0/+8/+16/+24/+32; 24 bytes of digit
scratch written backwards from the frame top (+40..+64). Six stderr writes
(`"runtime error: "`, kind, `" at "`, file, `":"`, digits, `"\n"`), the digit
loop being the same divide-by-10 loop `print_int` uses, then `exit(1)`.

### 4.3 The stderr route — `emitWriteStderr` (program.go:2227)

```
if goos == OSWindows { emitWinWriteHandle(stdErrHandle); return }
MovRegImm32(RDI, 2); MovRegImm32(RAX, sysWrite()); Syscall()
```

## 5. The stderr limitation — inherited, and load-bearing

`PHASE-152-BASELINE.md:511-513` records that **stderr output is empty on this
host** for PE images, "by the 152-B0 `GetStdHandle(STD_ERROR_HANDLE)`
limitation", and that 152-B1's reject tests therefore "assert exit code, stdout
route and interned text rather than captured stderr".

This was verified as an existing property, not a new claim: the **Go** backend
built for the same out-of-range program exits **1** with empty stdout on this
host. So:

- The requirement is **byte-identity with the oracle**, which writes through the
  same `GetStdHandle(2)` route and is equally silent here.
- A kcc port must reproduce that route, **not** improve on it. Routing the
  diagnostic to stdout instead would be a *divergence*, not a fix.
- The gate must assert **exit code + stdout bytes + the interned diagnostic
  text**, exactly as 152-B1 does. Asserting captured stderr on this host would
  be asserting something that is true for neither engine.

## 6. The first attempt, and what it teaches

A reporter port was attempted during D4 and **reverted**; a fuller attempt was
made during D5 and **also reverted** once §3.1 showed it could not reach parity
alone. The facts:

- It emitted successfully; a **valid** index printed `5`, exit 0.
- The **out-of-range** case crashed `0xC0000005` where the Go backend exits 1.
- Defects found in the attempt's own code, all instructive:
  1. `natStmtHasIndex` compared only the statement's own node type, so
     `print(a[1])` (a `NODE_PRINT` whose *argument* is the index) was never
     detected — the gate passed without checking anything. See §3a.
  2. The port assumed a **fixed** rodata layout; the oracle interns by first
     use. See §3a.
  3. `natStdErrHandle` cannot be a literal. See §3a.

The revert was correct both times: an unverified or partial emission path is
worse than the honest `Int3`, and a diagnostic that **crashes** instead of
trapping is strictly worse still.

## 7. Scope

In scope:

1. `natRuntimeErrorHelper` — the reporter, byte-for-byte.
2. `natWinWriteStderr` / `natSysWriteStderr` — both OS routes.
3. `natMapFind` / `natMapSet` / `natMapGet` — the 209 bytes §3.1 identified,
   gated on the SAME syntactic predicate `isMapIndex` implements so the two
   engines cannot disagree about when they appear.
4. `sourceBase()` plumbing — the real file basename, so `file:line` is genuine.
   `buildNativeFile` already holds `path` in scope and `fileBaseName` exists.
5. The `emitArrayIndex` bounds check with `natFresh`-allocated labels, matching
   the oracle's `b.fresh` allocation order.
6. The gate: byte-identity against the oracle, **plus** execution (exit code and
   stdout bytes) for the Windows PE, **plus** the valid-index case that catches
   a missing `Mark(doneLbl)`.

Explicitly out of scope:

- **The 8-byte frame-layout difference (§3a).** `natFrameLayout` was frozen in
  Step 1 with every Step 1-9d displacement pinned against it; closing it is D6.
  D5 therefore CANNOT reach byte-identity on its own, and this baseline says so
  rather than letting a partial result be reported as parity.
- **Tightening `isMapIndex` (§3.1 reading 2)**, which changes the Go backend and
  every frozen Go-side gold. An owner decision, recorded not taken.
- The `hex_decode_bytes` / `base64_decode_bytes` reporter call sites (152-B1
  surface). The reporter becomes available to them later; they are not wired here.
- Maps, records, `for-in`, `push`, floats, string slice/compare.
- macOS entry/exit parity.

## 8. Risks, stated before coding

1. **Label naming.** `b.fresh` counter order must match exactly; a `rel32`
   encodes only a displacement, so a wrong label can leave every byte identical.
   Mitigation: a label-discipline layer, as Step 3 established.
2. **REX-prefix traps.** `B8+rd`/`B9+rd`/`BE`/`BF` are valid only for
   `rax..rdi`; RCX, RDX, RSI and RDI need `REX.B`/`REX.R`. D3 and D4 each hit
   this. Mitigation: every new immediate load states the register class.
3. **RoDATA interning order.** §6.2 — must intern by first use, not by a fixed
   layout. Mitigation: assert the resulting offsets, not an assumed ordering.
4. **The 4 GB host.** The whole-tree KIR measurement needs ~2.5 GB working set and
   is killed at the 11-minute default. Mitigation: `GOMEMLIMIT=3GiB`, isolated
   runs, generous `-timeout`. This is the documented class, not a defect.
5. **Falsifiable-by-construction gate.** Parity is asserted as a **differential**
   against the oracle, never a golden — for most of increment 151 the current
   bytes were wrong, and a golden would have frozen the divergence.

## 9. Exit criteria

1. The reporter and the map helper set are emitted byte-identically to the
   oracle, and `array_index` **narrows** to the remaining 8-byte frame delta.
2. The seven previously-parity shapes are **unchanged** (no silent regression).
3. The frozen Step 1/2/7/8/9a/9b/9c/9e/151B/151C corpora are byte-unchanged.
4. A valid index prints correctly and exits 0 (the missing-`Mark` regression).
5. An out-of-range index exits **1** — matching the Go backend on this host.
6. KIR pin re-MEASURED with `karkain kir --verify`, never predicted.
7. `go build ./...` and `go vet ./pkg/... ./cmd/...` clean.

**Explicitly NOT an exit criterion:** byte-identity for `array_index`. §3a
measured an independent 8-byte frame difference (Go `sub rsp,0x290`, kcc
`sub rsp,0x288`) that D5 cannot close without reopening `natFrameLayout`. The
gate therefore keeps `array_index` on the known-non-parity list, with the
narrowed numbers recorded, and D6 owns the move. Claiming parity here would be
asserting a bug.

## 10. Not claimed

- **Execution of a Linux ELF on this host.** There is no WSL and no Docker; every
  ELF claim here is structural. The Linux CI leg is where these bytes execute.
- **Execution of a macOS image.** There is no macOS runner; every Mach-O claim
  since increment 149 has been structural.
- **That the stderr TEXT is observable on this host.** Per §5 it is not, for
  either engine. The interned strings are asserted in the image; the visible
  behaviour asserted is the exit code.
- **That the reporter is generally available on the kcc target.** Only the
  `array_index` call site is wired (§7.4). The 152-B1 codec sites are not.

---

## 11. Corrective change D8 - the length immediate (COMPLETE, 2026-10-29)

**Not a new increment.** Recorded against the completed D5 baseline per the
Governance rule.

- **Owning layer:** `natArrayIndex` in `src/compiler/native_value.kark`, the
  failure arm that calls `karkain_runtime_error`.
- **Demonstrated failure:** the emitter wrote the literal `25` into RSI as the
  length of `"array index out of range"`, which is **24** bytes. The reporter
  therefore received a length one byte greater than the string. The Go oracle
  derives the value with `len("array index out of range")`
  (`pkg/native/program.go:6324`), so this was a kcc-only divergence.
- **Why D5 could not see it:** the diagnostic is written to a stream that the
  152-B0 `GetStdHandle` limitation leaves empty for PE images on this host, and
  D5's differential counts image bytes rather than decoding the immediate. A
  one-byte change inside an immediate does not move the total.
- **Correction:** `natMovRegImm32(s, natRegRSI(), len("array index out of range"))`.
  Deriving from the literal, rather than from the interned offset table (which
  carries no lengths), means a future edit to the message cannot desynchronise
  the immediate.
- **Downstream behaviour:** no byte moves for any program that does not index
  out of bounds. The failure arm changes one immediate (`BE 19 00 00 00` ->
  `BE 18 00 00 00`); no frame displacement, label or container field moves.
- **Gate:** `pkg/cli/phase151d8_diaglen_test.go` (`TestPhaseD8_`), CI step
  *Run Phase D8 native bounds-diagnostic length gate*. It locates the argument
  sequence by decoding it and asserts the immediate, reporting the observed
  value in the failure message.
- **Evidence:** PASS locally. **Mutation-verified** by restoring the literal
  `25`, which fails with "the bounds-diagnostic length immediate is 25, want
  24"; reverted and re-ran with `TestPhaseD5_` green (27.5s).