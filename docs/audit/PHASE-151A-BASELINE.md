# Phase 151A BASELINE — the native value model in kcc

Date: 2026-09-27. Version-plan slot: 1.2.0 "Sovereignty I", increment 151,
slice A. Parent: `PHASE-151-BASELINE.md` (its §4 named this slice; this file
re-measures it and splits the plan it left open).

**KIR pin: 9574, unchanged by 151A-1, and the first version of this note
claimed it would drift.** Corrected by measurement. The pin gate
(`TestPhase122_PipelineOwnership/KIRContinuity`) runs
`karkain kir --verify src/compiler/kir.kark` and asserts exactly 9574. Adding
151A-1's ~83 lines to `src/compiler/main.kark` left it at **9574/9574 verified
on both runs**, because KIR does not render the driver at all: the emitted text
contains `runFile`? **No.** `buildNativeFile`? **No.** `isNativeTarget`? **No.**
The pin therefore covers the compiler's library modules (ast/lexer/parser/sema/
codegen/checker/kir), not `main.kark`.

The real 151B/151C emitter and container work lands in the modules KIR *does*
cover, so the pin will move then and must be re-measured there — not
hand-edited.

## 1. Starting position (measured, not assumed)

* **Increment 150 is COMPLETE** and frozen. `pkg/native` (14 Go files, 396,417
  bytes) is the **oracle**: boxed Value model, register allocation, three
  containers (ELF / PE / Mach-O PIE), three CLI targets.
* **The silent Go fallback is closed** (151D-first, `f228d31`): a kcc native
  build now refuses with `error[K116]`, exit 6, and writes no image.
  `pkg/cli/kcc_native.go` holds the seam; `kccOwnsNativeTargets = false`.
* **kcc has ZERO machine-code codegen.** Verified again this session: no `elf`,
  `pe`, `macho` or `x86` token in `src/compiler/*.kark`. Its whole command
  surface is `checkFile` / `buildFile` / `runFile` / `runTests` / `kirFile` /
  `verifyFile`, and `buildFile` ends in `generate(ast, target, ...)` → C23 text →
  `createFile` + `writeToFile`. `runFile` shells out to `gcc`.
* **kcc module assembly is `siblingContent` + letter order**
  (`main.kark:collectKarkFiles:502`, `siblingContent:571`, `assembleProject:964`).
  A new `native*.kark` is therefore picked up **only** if its name sorts after
  every file that calls it — this is a real ordering constraint, not a detail.
* **kcc has no byte-level buffer primitive.** The available I/O is
  `createFile` / `writeToFile` / `openFile` / `removeFile`, all **string**-valued.
  There is no `readFile` on a handle, no byte append, and no numeric byte store.
  `asciiVal` (`lexer.kark:198`) converts a one-character string to a code point.

## 2. The load-bearing problem, stated before any code

Writing the image is the easy half. **151A's hard requirement is that kcc's
image be byte-identical to Go's**, and the Go oracle is a 396 KB Go program that
lowers a `*parser.Program` — a data structure kcc does not have and cannot
reconstruct, because its AST is a different shape built by a different parser.

There are exactly three honest ways to close this, and only one of them is
"parity":

| Option | What it means | Verdict |
|---|---|---|
| **(A) Re-implement the lowering in Karkain** | port the value model, emitter and three containers into `src/compiler`, reproducing the Go byte-for-byte | **this is the real 151A–151C.** Weeks. |
| **(B) kcc shells out to a helper that owns the image** | kcc emits a *request*, something else writes the bytes | **not parity.** Same dishonesty as the Go fallback, one process hop further away. Rejected by §3 of the parent baseline. |
| **(C) kcc reuses the Go oracle through a stable, auditable interface** | the self-hosted engine drives the *pipeline and decisions*; image bytes come from the one implementation that is proven | **honest if — and only if — the seam is disclosed, versioned and gated** |

## 3. Why (A) is not startable in one increment, stated plainly

The blocker is structural, not effort. The Go image bytes are a function of
`pkg/native`'s **decisions**, and the decisions are not recorded anywhere
machine-readable:

* label names (`karkain_main`, `fn_<name>`, `print_int`, `print_str`, `map_find`…)
  and their **emission order**;
* the `fresh()` label counter, which suffixes control-flow labels
  (`while$1`, `forpost$2`, …) and therefore depends on the exact traversal order
  of the whole program;
* the frame layout (`argSpillBytes`, `binTemp`, `extrasBase`, per-slot offsets);
* the register allocator's 150C plan;
* the container field ordering and the link-time patch lists.

Porting that into Karkain is not "write the same logic twice" — it is writing a
**second compiler front-to-back that is bit-exactly bug-compatible with the
first**, including every incidental constant. Any divergence in one label
counter or one frame offset changes the SHA and fails the gate. That is the
correct standard (Phase 109/123 precedent: parity is asserted on golden bytes),
and it is why 151 is split A/B/C in the first place.

## 4. Decision for this increment: make (C) honest, and stage (A)

I am **not** claiming parity. I am doing the part that is genuinely deliverable
and honest, and recording what (A) still needs.

**151A-1 — the decision seam moves into kcc.** Instead of Go deciding
"is this a native target, and hand it to the oracle", kcc becomes the engine
that *identifies* the request and *refuses or delegates* it explicitly, with
its own banner and a recorded reason. This is the one thing the Go fallback
could never do, because the Go fallback never consulted kcc at all.

**151A-2 — the oracle interface is made stable and gated.** `pkg/native`
already exposes exactly one entry point, `CompileProgramForOS(*parser.Program,
osName)`. The seam will call **only** that, and the gate will pin that it
remains the only symbol 151 depends on, so the coupling is explicit and cannot
widen silently.

**151A-3 — a byte-differential harness over the real feature corpus.** The
existing `phase151_parity_test.go` is extended from one `print(42)` probe to
one corpus file per native feature (float64, arrays, `for-in`, `len`, arena +
string ops, `push`, records, maps, register allocation, PE, Mach-O), each
compared by SHA-256, with the provenance assertion retained.

**Why this is not (B).** (B) would re-add a silent hand-off. The difference is
that (C) **discloses** it: the build output says which engine made the
decision and which wrote the bytes, the gate fails if the seam ever stops
saying so, and the docs carry it as an open, owned gap against (A). A disclosed
delegation is a tracked debt; a silent one is a lie. The parent's §1.1 exists
precisely because the silent version was shipped and looked like parity.

## 5. Exit criteria for 151A-1 (this increment)

Met, and marked against what was actually measured rather than what was planned.
Two criteria from the original §4/§5 sketch did **not** survive contact with the
code, and are recorded as such instead of being quietly ticked:

* [x] A kcc native build **reaches kcc first**, and the provenance line is
      emitted by kcc itself (`buildNativeFile`, `src/compiler/main.kark`).
* [x] `kccProducedImage` in the parity harness is **true** for a kcc native
      build, and it is true because kcc was consulted — proven by mutation.
* [x] An unsupported capability is still a **loud K1xx**, never a fallback; and
      the three failure modes (type error / kcc refusal / kcc unreachable) are
      reported distinctly, because a toolchain failure is not a capability
      refusal.
* [x] `go build ./...`, `go vet ./...`, `GOARCH=386` vet, `pkg/native`,
      `pkg/parser`, `pkg/lexer`, `pkg/sema`, and the `pkg/cli` 148/150/151
      gates are green. `TestPhase122_KIRContinuity` green.
* [x] KIR pin **re-measured**: 9574/9574, unchanged, and the reason recorded
      (KIR does not render the driver).
* [x] `v1.2.0-CHECKLIST.md` updated; `AGENTS.md` record added.
* [x] `.gitignore` covers `.karkain-dev/`, the scratch tree the native/kcc
      gates build in the repo root (a 15 MB `karkain.exe` was sitting untracked).

**Dropped, with the reason.** §4 originally listed two criteria that turned out
to be unachievable as written:

* *"The seam calls exactly one oracle symbol; a gate pins that."* — **dropped.**
  151A-1 makes no oracle call at all; kcc refuses and writes no image, so there
  is no symbol to pin. A gate for it would have been theatre. The single-symbol
  constraint only becomes real when 151B/151C actually delegate the bytes, and
  it should be pinned then, with the real symbol.
* *"The corpus differential runs over every native feature, byte-exact."* —
  **deferred, not dropped.** It is meaningless while kcc emits no image: there
  are no kcc bytes to compare, and comparing Go-to-Go would be the vacuous check
  §2 exists to prevent. It becomes the 151C exit criterion.

## 6. 151A-1 — the decision moves into kcc (DONE)

**What landed.** `src/compiler/main.kark` gained `isNativeTarget`,
`nativeTargetOS` and `buildNativeFile`, and its `build` arm routes the three
C-free targets there. `pkg/cli/kcc_native.go` now **stages the user's real
project** and **runs kcc on it** (`kccStageInput` + `exec`), returning kcc's own
output instead of synthesising a refusal in Go.

Verified with the real binary:

```
$ KARKAIN_ENGINE=kcc karkain build hello.kark --target native-x86_64-linux
[kcc] native request native-x86_64-linux (os=linux): hello.kark   <- from kcc
error[K116]: the self-hosted engine (kcc) has no machine-code backend...
exit 6, no image written
```

`buildNativeFile` runs the **full self-hosted pipeline** — assemble, tokenize,
parse, and the two-pass type check — before refusing, so the refusal is a
statement about a program kcc actually read, and a program that does not
type-check reports that instead of a backend error.

**`kccProducedImage` is now TRUE, and it is real.** 151D-first could not make
it true, because the marker was a string Go chose to print. It is now emitted
by `buildNativeFile` inside `src/compiler/main.kark`, and the Go backend has no
code path that prints it. The harness invariant is unchanged and now satisfied
by kcc rather than by Go.

**A gate that could not fail, found by mutation.** The first version of the
provenance check searched the output for the bare marker. Removing the marker
from kcc's source **did not fail the gate**, because the "kcc did not answer"
diagnostic quoted the very marker it was reporting missing — a self-defeating
assertion. Two fixes, both kept: the diagnostic no longer contains the marker
text in any form, and the gate checks the marker **followed by the target**.
Re-mutated and confirmed failing.

A second, unrelated trap worth recording: `Copy-Item` preserves the source
file's `LastWriteTime`, so restoring a mutated `main.kark` left it looking older
than the `kcc.exe` built from the mutation, and `kccStale` happily reused the
mutated binary. The "restored" tree was not what the tests were exercising. Any
future mutation of a self-hosted source must bump the mtime explicitly.

**Evidence.**

| Gate | Result |
|---|---|
| `TestPhase151_NoSilentGoFallback` (3 targets) | PASS — refuses, K116, **kcc provenance**, no image |
| `TestPhase151_NativeGoBackendUnaffected` (3 targets) | PASS — Go still builds all three |
| `TestPhase151_KccNativeRefusalIsNotAFallback` | PASS — premature-flip guard |
| `TestPhase151_HarnessDetectsNonParity` | PASS — "kcc is the engine of record and refused with K116" |
| `TestPhase151_HarnessComparesBytes` | PASS |
| mutation: kcc provenance line removed | **FAILS** (after the fix above) |
| mutation: `kccOwnsNativeTargets` flipped early | FAILS with the guard diagnosis |
| `karkain kir --verify` (KIR pin) | 9574/9574 — unchanged; driver is not rendered |
| `TestPhase122_KIRContinuity` | PASS (275 s) |
| `go build ./...`, `go vet ./...`, `GOARCH=386` vet | clean |
| `pkg/native`, `pkg/parser`, `pkg/lexer`, `pkg/sema`, cli 148/150/151 | green |

**What is still not claimed.** kcc still cannot emit a machine-code image, so
this is **not parity**. `kccOwnsNativeTargets` stays `false`. Slices 151B
(encoder) and 151C (three containers) are untouched, and they are the work that
actually earns the byte-identity the parent baseline requires.

## 7. What this increment explicitly does NOT claim

* **Not parity.** No claim is made that kcc's own codegen produces these bytes,
  because it does not — there is none. The parent's §1.1 measured problem is
  *eliminated as a lie*, not *solved as a compiler*.
* **Not a 151 substitute.** 151A/B/C (real Karkain codegen) remain open, and
  their order is unchanged: value model → encoder → three containers.
* **Not 152.** The no-C closure (hello / `stdlib_v2` / `std.net` with
  `gcc`+`clang`+`cl` absent from `PATH`) depends on 151A–151C, not on this.
* **No new native features.** Per parent §3.
* **No Value-model redesign**, no C-path change, no 1.3.0+ work.

## 8. Risk register

1. **Provenance is not parity, and can be mistaken for it.** Mitigated by
   naming the mechanism in output, docs and `AGENTS.md`, and by keeping the
   parent's byte-comparison gate armed for the day (A) lands.
2. **KIR pin drift is expected for 151B/151C, and NOT for 151A-1.** Measured:
   the pin held at 9574/9574 because KIR does not render the driver. The
   emitter and container slices touch the modules it does cover, so re-measure
   there; never hand-edit the number.
3. **The seam could widen.** Pinned by a gate on the symbol set.
4. **A "temporary" delegation becomes permanent** because it passes gates. The
   gates cannot detect that, so the audit record carries it as an explicit
   carry-over against (A) rather than as a success.

## 9. 151A Step 2 - integer statement/expression lowering (DONE)

Step 1 built the value/frame half and left it **unconsumed**: nothing emitted
through it. Step 2 makes int statements and expressions actually lower, so the
frame layout is read on the default lowering path.

**Scope.** `let` of an int, int arithmetic (`+ - *`), unary minus, the six
comparisons as real branches, `return`, the `jmp` to the `$ret` label that the
oracle emits after **every** return, and the `if !returned` zero fallback.
Float, array, `for-in`, arena, string, `push`, record and map lowering are
**still not claimed** -- they remain 151A work.

**Gate.** `pkg/cli/phase151a2_int_test.go`, 14/14 PASS, built as the same
**four-layer differential** Step 1 used:

1. byte containment in the oracle's `.text`, which the oracle *compiled* from
   the same source (rules out a shared transcription error);
2. the frame numbers derived independently in the test file (rules out a
   layout that is wrong the same way on both sides);
3. bytes stated from the Intel SDM (catches shared drift);
4. a refusal table matching the oracle's own refusals.

**Four defects found. Two were real, and one of those had been latent since
Step 1.**

1. **`natFrBinTemp()` returned the scratch region's END, not its start.** It
   stored `localBytes + recArgBytes + binTempBytes`, so every staged operand
   landed 512 bytes too high: kcc emitted `[rsp+0x210]` where the oracle emits
   `[rsp+0x10]`. **This was latent through all of Step 1**: nothing emitted
   through `binTemp`, and Step 1's layout corpus printed only
   `frame`/`strTemp`/`mapStage`/slot offsets, so no gate could see it. Fixing
   it turned all six comparison cases green at once, which is the signature
   that identifies it as the single cause.
2. **Unary minus incremented the scratch depth.** The oracle's `emitReturn`
   calls `emitExpr(value, 0)` and `UnaryExpr` passes the *same* depth, so the
   nested binary in `-(3 + 4)` also lands at depth 0.
3. **A wrong expectation in the test, not the code.** Two corpus cases were
   declared with `nLocals: 1` when their sources contain no `let`, so the
   oracle correctly allocates no local and its frame is 608, not 616 -- and the
   frame layer reported the oracle's own correct frame as wrong. This is the
   151B lesson recurring: a baseline with wrong expectations is worse than no
   baseline. Fixed by measuring, and the expectation now derives from the
   oracle.
4. **The `mov r64, r64` stated bytes were backwards** (`ModRM` 0xC8 vs 0xC1),
   caught by the third layer reporting that the *oracle* disagreed with the
   stated bytes. kcc was right in both attempts; the test's argument order was
   wrong. `89 /r` puts the source in `reg` and the destination in `rm`.

**Non-vacuity, measured.** Reverting defect 1 reproduces the pre-fix failure
signature exactly -- 3 int cases and all 6 comparisons fail -- and the fix
restores 14/14. The gate is armed, not decorative.

**KIR pin re-measured, never hand-edited:** 10838 -> **11040**
(`karkain kir --verify`, 11040/11040 verify ok), and
`TestPhase122_PipelineOwnership/KIRContinuity` PASS at 334 s on the new count.

**Evidence.**

| Gate | Result |
|---|---|
| `TestPhase151A2_IntegerLoweringByteIdentical` (5) | PASS |
| `TestPhase151A2_ComparisonsAreBranches` (6) | PASS |
| `TestPhase151A2_RefusalsMatchOracle` | PASS |
| `TestPhase151A_*` (Step 1, re-run) | PASS -- unchanged, incl. the layout and primitive gates |
| all `TestPhase151*` in `pkg/cli` + `pkg/native` (43) | PASS |
| `pkg/native` (full) | PASS |
| `TestPhase122_PipelineOwnership/KIRContinuity` | PASS (334 s), pin 11040 |
| `go build ./...`, `go vet ./pkg/cli ./pkg/native ./cmd/karkain` | clean |
| mutation: defect 1 reverted | **FAILS** (3 int + 6 cmp, the pre-fix signature) |

**Still not claimed.** No whole-image parity, and **no execution evidence**:
these are function bodies, not runnable programs -- there is no entry stub, no
syscall tail, no PEB bootstrap, and no `print`, so there is nothing a loader
could usefully execute. The argument remains transitive. `kccOwnsNativeTargets`
stays `false`; 151A is **not** closed.

**The next Step 2b boundary, stated so it is not discovered late:** a bare
identifier operand (`return x` where `x` is a `let`) and the `?` propagation
operator (`?`) have no `emitStr`/int path yet, so they are not lowered here.

---

## 10. 151A Step 2b — bare assignment statements and `?` refusal parity

Step 2's closing note named two items for Step 2b. **Both were re-measured
before any code was written, and the measurement corrects the note.**

### 10.1 What the oracle actually does (measured, not read off the switch)

A temporary probe compiled each construct with `native.CompileProgramForOS`:

| Construct | Oracle result |
|---|---|
| `let x = 42; return x` | **ACCEPTED**, 233-byte `.text` |
| `let x = 1; x = 2; return x` | **ACCEPTED**, 247-byte `.text` |
| `let x = 3; x = 4; return x + 1` | **ACCEPTED**, 273-byte `.text` |
| `let x = 1?` | **REFUSED** `error[K145]: unsupported **return** expression *parser.PropagateExpr` |
| `return 1?` | **REFUSED** `error[K145]: unsupported **return** expression *parser.PropagateExpr` |
| `print(1?)` | **REFUSED** `error[K145]: unsupported expression *parser.PropagateExpr` |
| `let x = 1` then `x = 1?` then `return x` | **REFUSED** `error[K145]: unsupported expression *parser.PropagateExpr` |
| `let s = "a" + 1?` | **REFUSED** `error[K145]: unsupported **return** expression *parser.PropagateExpr` |
| `while 1? { }` | **REFUSED** `error[K145]: condition must be a comparison ...` (never reaches `?`) |

**CORRECTION — this section's own first draft had the discriminator wrong, and
the error is instructive enough to record.** The draft claimed the message
depends on the `?` being **textually in return position**. Measurement says
otherwise, and two of the rows above falsify it directly: `let x = 1?` is
textually a `let` yet reports the RETURN message, while `x = 1?` on an
already-bound local is followed by `return x` yet reports the plain EXPRESSION
message.

**The real rule is reachability from a return, not textual position.** The
oracle runs a return-kind inference pass *before* emission: `walkReturns`
(`program.go:1788`) calls `retKindOfExpr` (`program.go:1881`), and that helper
**follows an `Identifier` back to its `let` initializer**
(`program.go:1920`). So:

* a `?` **reachable from a return statement** → `unsupported RETURN expression`
  (refused during kind inference, at `retKindOfExpr`'s `default:`, line 1971,
  and never reaching emission);
* any other `?` → `unsupported expression` (`exprKind` line 4325, or `emitExpr`
  line 4451).

A **reassignment** does not update the table that walk reads — `vars` is built
from `let` bindings — which is exactly why `x = 1?; return x` reports the
non-return wording. The general lesson is the 151B one again: the discriminator
had to be *measured*, because reading the three `default:` sites suggests a
positional rule that the actual control flow does not implement.

**The consequence for the gate is a hardening, not just a doc fix.** The first
draft of `natIntRefusal` and its test both **hardcoded** the two message
strings — two transcriptions of the same belief, which is precisely the 151C2
defect class (`LC_DYLD_INFO_ONLY` written wrong in two places). The strings are
now **measured from the oracle at test time** by
`TestPhase151A2_PropagateRefusalsMatchOracle`, which compiles four real
programs and requires kcc to emit exactly what the oracle produced. That test
is what caught this section's own wrong table.

**Two corrections to Step 2's closing note.**

1. **A bare identifier operand is ALREADY lowered.** Shape 0 emits
   `mov rax,42` / `mov [rsp],rax` / `mov rax,[rsp]` / `jmp $ret` for exactly
   `let x = 42; return x`, and shape 1 already uses `a` and `b` as **binary
   operands** (load both slots, stage through `binTemp`, multiply). The
   identifier-as-value and identifier-as-operand paths both exist. The note was
   written from the shape list rather than from a measurement, and the 151B
   lesson applies: a baseline with wrong expectations is worse than none.

2. **`?` is NOT lowered by the oracle, and is not Step 2b lowering work at
   all.** `?` desugars to `PropagateExpr` (`pkg/parser/ast.go:209`), and
   `emitExpr` has no case for it, so it lands in the `default:` branch
   (`program.go:4450`). There is **no `PropagateExpr` case anywhere in
   `pkg/native`** — a grep for `Propagate` across the backend returns
   nothing. The increment-150 native value model has no Result/Option
   representation, so there is nothing to propagate *to*.

   So the honest 151A Step 2b deliverable for `?` is **refusal parity**, not
   lowering: kcc must refuse it with the **same message the oracle does**, and
   there are **two distinct messages at three distinct sites**, which is the
   part that is easy to get wrong:

   | Site | Function | Message |
   |---|---|---|
   | 1 | `exprKind` (`program.go:4325`) | `unsupported expression %T` |
   | 2 | `retKindOfExpr` (`program.go:1971`) | `unsupported return expression %T` |
   | 3 | `emitExpr` (`program.go:4451`) | `unsupported expression %T` |

   `%T` renders as `*parser.PropagateExpr`. **Correction to the paragraph the
   first draft wrote here**, which repeated the same positional mistake: a `?`
   is not refused at site 2 because it is *in return position*, but because it
   is **reachable from a return** — the inference walk resolves an identifier
   back to its `let` initializer, so `let x = 1?` + `return x` takes site 2
   while `x = 1?` + `return x` takes site 1/3. See the correction block above.

### 10.2 The real Step 2b gap: bare assignment statements

With the identifier path already covered, the genuine gap Step 2b closes is
**a bare `x = 2` assignment to an existing int local** — an `ExprStmt` whose
expression is a `BinaryExpr` with operator `=`, which `emitExprStmt` handles
(`program.go:3422`) and which no Step 2 shape exercises. Its bytes are
measured, not assumed:

```
reassign_then_read   4881ec68020000 48b80100000000000000 48890424
                     48b80200000000000000 48890424 488b0424 e900000000
                     4881c468020000 c3
```

Two `mov rax, imm64` + `mov [rsp], rax` pairs, then the read. The frame is
**unchanged at 0x268** — a reassignment writes an existing slot and allocates
nothing, which is the property worth pinning.

And with a binary operand (`x + 1`), the identifier goes through the ordinary
staging path:

```
reassign_in_binary   ... 488b0424 4889442408 48b80100000000000000
                     4889c1 488b442408 4801c8 e900000000
```

`mov rax,[rsp]` (read the local) / `mov [rsp+8],rax` (stage at
`binTemp + 0*8`) / literal / `mov rcx,rax` / reload / `add` — identical in
shape to shape 1, which is the point: **a reassigned local takes exactly the
same lowering path as a freshly bound one.**

### 10.3 Exit criteria for 151A Step 2b

- [x] kcc emits the reassignment bodies **byte-identical** to the oracle's
      `.text`, by the same three layers Step 2 used.
      *Measured:* `reassign_then_read` 52 bytes
      `sha256=6e38af60…`, `reassign_in_binary` 78 bytes `sha256=07c039d4…`,
      each occurring **exactly once** in the oracle's compiled `.text`.
- [x] kcc refuses `?` with the **oracle's exact message, per site** — both
      the expression-position and the return-position wording, **measured from
      the oracle** rather than hardcoded (see the hardening note in §10.1).
      *Measured:* four constructs compiled against `native.CompileProgramForOS`;
      kcc's two slots match; the two messages are provably distinguishable.
- [x] The gate is **non-vacuous**: reverting the new kcc code fails it.
      *Mutation-verified, three mutations, each reverted and hash-confirmed:*
      shape 5's reassigned literal `2 → 9` → differential reports **0
      occurrences** *and* the stated bytes go missing; shape 6's staging offset
      `bt+0*8 → bt+1*8` → same two layers fire; the expression-message slot
      rewritten to the return wording → the refusal test fires. A **fourth**
      mutation on the *test's own* expectation for `let_then_return` — i.e.
      re-encoding this section's original wrong table — is caught by the new
      measured differential, which is the direct evidence that the hardening
      works and the old hardcoded pair could not have caught it.
- [x] KIR pin re-measured, never hand-edited. **11040 → 11069**
      (`TestPhase122_PipelineOwnership/KIRContinuity` PASS, 457.5s).
- [x] Step 1 and Step 2 gates unchanged and green.
      *Measured:* `TestPhase151A2_*` and the full `TestPhase151A*` set pass,
      `pkg/native` green (18.1s), `go vet ./pkg/cli/` clean, `gofmt` clean
      once LF-normalised (the only remaining flag is the repo-wide CRLF
      artifact; re-checking found and fixed one real missing blank line).

### 10.4 Explicitly NOT claimed

No `?` lowering, because the oracle has none. No whole-image parity, and **no
execution evidence** — these remain function bodies, with no entry stub, no
syscall tail and no PEB bootstrap, so nothing a loader could run. The argument
stays transitive. `kccOwnsNativeTargets` stays `false`; 151A is **not** closed.

The reassignment shapes add **no new capability to the compiler's surface**:
there is still no AST-driven native driver, so `natIntBody(5|6, …)` remains a
compiled-in shape rather than a general statement lowering. What Step 2b
establishes is that the *shape* matches the oracle for this construct, and that
the oracle's behaviour was measured rather than assumed.
