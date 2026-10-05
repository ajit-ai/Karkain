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

## 11. 151A Step 3 - control-flow lowering in kcc (DONE)

Step 2/2b made statements lower; Step 3 adds the loop forms so control flow is
produced by the same differential rather than assumed.

**Scope.** `while`, C-style `for`, `break`, `continue`, and nesting. Int
operands only; integer arithmetic in a body is `+` only, reusing the Step 2
primitive. No float, array, string, map or record operand reaches a loop, and
none of those lowerings exists yet.

**Gate** `pkg/cli/phase151a3_loop_test.go`: 3 tests / 18 subtests PASS, 10
reference programs byte-identical. Four independent layers:

1. kcc's bytes vs the `.text` the oracle produced by *compiling* the same
   source (the corpus is Karkain source, not expected bytes);
2. the frame immediate, derived independently in the test file;
3. opcodes stated from the Intel SDM;
4. label names and allocation order.

**Layer 4 is load-bearing, and that was measured, not assumed.** A `rel32`
encodes only a displacement, so renumbering a label can leave every byte
identical while the label discipline is wrong. Mutating `nested_while`'s
second loop pair from `$3/$4` to `$1/$2` - exactly what a per-loop counter
would produce - left layer 1 **green on all ten cases** and failed layer 4
only. That is the case the other three layers cannot catch, and it is why
`native-value-loop-labels` is a separate surface.

A second mutation (stating `0f8c`/`jl` where the oracle emits `0f8d`/`jge`)
failed layer 3 with layers 1 and 4 green, confirming the SDM layer does
independent work rather than restating layer 1. Both mutations were reverted;
neither is in the tree.

The gate also records a correction found while writing it: the first draft of
`TestPhase151A3_BreakAndContinuePresent` asserted five labels and index 2 for
the `if` pair on all four break/continue cases. The oracle settled it - a
`while` allocates two loop labels and a `for` three, so the `if` pair starts at
index 2 for a `while` and index 3 for a `for`. An expectation written from
memory rather than from the oracle is precisely what this gate exists to catch.

**KIR pin re-measured**, never hand-edited: 11069 -> **11373**
(`karkain kir --verify`, 11373/11373 verify ok);
`TestPhase122_PipelineOwnership/KIRContinuity` PASS at 215 s.

**Regressions.** `TestPhase151A_` 3/3 PASS; `TestPhase151A2_` 4/4 PASS;
`TestPhase151A2b_` 1/1 PASS (note it is *not* matched by the `TestPhase151A2_`
pattern, so it was run explicitly); `go vet ./pkg/cli/` clean.

**Still not claimed.** No whole-image parity and no execution evidence: these
are function bodies, not runnable programs. `kccOwnsNativeTargets` stays
`false`; 151A is **not** closed. Floats, arrays, `for-in`, arena, string ops,
`push`, records and maps remain open, as does the entry stub / syscall tail /
PEB bootstrap that a runnable image needs.

## 12. 151A Step 4 - the `_start` entry stub and the Linux exit tail (AUTHORIZED)

Step 1/2/2b/3 built and consumed the value, frame and control-flow layers;
none of them emit an entry sequence, so every program built so far is a
function body rather than a runnable image. This section **authorizes the slice
that supplies the entry point and the Linux exit path.**

This section is the **owner phase decision** that gives the work its
authorization. Until it existed, the `_start` implementation was present in the
working tree with **no authorizing definition anywhere in the repository**:
`PHASE-151-BASELINE.md` §4 defines 151A/151B/151C/151D without sub-stepping
151A, and neither `KARKAIN-VERSION-PLAN.md` nor `docs/release/v1.2.0-CHECKLIST.md`
mentions an entry stub. The absence of authorization was a governance gap, not a
scope error - the substance of this slice was already named as open work by
Step 1 ("the `_start` stub, the Linux syscall tail ... remain open 151A work").
This section closes that gap by giving the work a number, a scope and exit
criteria. It does not widen scope.

**Scope.** The Linux half of the entry sequence only:

1. the `_start` entry stub - `natMark(s, "_start")`, the `rel32` call to
   `karkain_main`, and the return jump to `karkain_main$ret`;
2. the bare `syscall` primitive - `natSyscall`, selectors 34 (`0x0F`) and 35
   (`0x05`), continuing the shared `natMask` namespace rather than renumbering;
3. the Linux syscall exit tail - `mov rdi, rax`, `mov rax, 60`, `syscall`;
4. the `natRegRDI` accessor, named rather than written as the literal so the
   gate can pin it against `pkg/native.RDI` independently.

`natStartStub`, `natStartCorpus`, `natSyscall` and `natRegRDI` in
`src/compiler/native_value.kark` / `native_emit.kark`, plus the
`native-value-start` dispatch in `src/compiler/main.kark`,
`pkg/cli/native_encode.go` and `cmd/karkain/main.go`, are the deliverable. All
`src/compiler` changes are **purely additive** - zero removed or modified
existing lines - which preserves the property Step 1 established.

**Explicitly deferred - and deliberately not implemented here:**

* **Windows entry** - the 16-byte stack alignment fix (`and rsp, -16`);
* **Windows exit** - the Win64 `exit`/IAT tail;
* **PEB bootstrap** - PEB to Ldr to the export-table walk publishing
  ExitProcess/GetStdHandle/WriteFile;
* **macOS entry/exit** - including the macOS exit number `0x2000001`.

Emitting any of them would be inventing scope rather than porting it. Each is
separately open 151A work.

**Gate.** `pkg/cli/phase151a4_start_test.go` is the structural/differential
gate: 5 tests built as the same four-layer discipline Steps 1-3 used -

1. kcc's bytes vs the real `pkg/native.Emitter` building the same sequence (a
   differential, not a golden: a golden pins kcc against a transcription of the
   Go code and passes when both copies are wrong together);
2. the `rel32` displacement derived from first principles in the test file;
3. opcodes stated from the Intel SDM;
4. the exit tail occurring verbatim in the `.text` of a **real** oracle image
   that compiled a reference program.

Layer 4 compares only the bytes **after** the call. That exclusion is
deliberate, not convenient: a `rel32` encodes a displacement whose value depends
on where the compiler placed `karkain_main` in that program, so kcc's
self-contained corpus displacement (10) must not equal a real image's, while the
bytes after the call are layout-independent and must match exactly.

**Exit criteria.**

1. The structural/differential gate above is present and passing for the five
   `TestPhase151A4_*` tests.
2. A **dedicated CI step** runs `go test ./pkg/cli/ -run 'TestPhase151A4_'`,
   per the `AGENTS.md` companion rule - a gate is a test file *plus* a CI entry.
   The pattern is deliberately distinct: `TestPhase151A3_` does not match
   `TestPhase151A4_`, so a Step 4 regression cannot hide behind a green Step 3
   step.
3. The **KIR pin transition `11377 -> 11412` is coupled atomically** with this
   slice. The pin is monotonic and shared, so the code and the pin must move
   together or not at all: committing these `src/compiler` files without the pin
   at 11412 breaks `TestPhase122_PipelineOwnership/KIRContinuity`, and moving the
   pin without the files breaks it identically. The pin is re-measured with
   `karkain kir --verify`, never hand-edited. `11373 -> 11377` is **LH-2**
   (match-arm body checking) and is **not** part of this slice; 151A Step 4
   owns only `11377 -> 11412`.
4. Evidence is **structural and byte-level differential only**. **No native
   execution evidence is claimed or achievable for this slice**: the corpus has
   no `main` body, no `print` and no arena, so it is not a runnable program.
   The argument that the bytes are correct is transitive - kcc's bytes are
   byte-identical to `pkg/native`'s, whose PE images do execute on the
   increment 145-150 gates - and the transitive form is stated rather than
   upgraded to a direct claim.

**Still not claimed.** No whole-image parity. No execution evidence. No Windows
or macOS entry or exit. No PEB bootstrap. `kccOwnsNativeTargets` stays `false`
and 151A remains **not** closed; floats, arrays, `for-in`, arena, string ops,
`push`, records and maps all remain open 151A work.

## 13. 151A Step 5 / Step 6 - Windows entry, PEB bootstrap, Win64 exit (IMPLEMENTED)

This section records the IMPLEMENTED result. Sections 1-12 above were written
before implementation and are left untouched, including Step 4's closing "Still
not claimed" paragraph, which was true when written and is superseded only for
the Windows half.

**Implemented.** The Windows entry (`and rsp, -16`), the loader-independent PEB
bootstrap (PEB -> Ldr -> module walk matching `kernel32.dll` -> export-table walk),
the three export resolves (ExitProcess, GetStdHandle, WriteFile), the Win64 exit
tail, and the `native-value-win` kcc dispatch. Fourteen emitter primitives were
added in `src/compiler/native_emit.kark`, each the byte-for-byte counterpart of
the identically named Go method, with `natMask` selectors continuing at 36.

**Byte identity (MEASURED).** All four oracle shapes are byte-identical:
`align`, `exit`, `resolve` (227 bytes) and `start` (982 bytes).
`TestPhase151A5_ByteIdenticalToGoOracle` compares kcc against the REAL
`pkg/native.Emitter` on a self-contained corpus -- a differential, never a golden.
The documented `rel32` exclusion is unchanged: shapes containing a displacement
cannot be compared verbatim against a real image, and L2 establishes their
identity on the self-contained corpus where both sides share a label layout.

**Execution of a KCC-PRODUCED image (MEASURED).** `natWinExe` / `natWinExeC`
compose the Step-5 `_start` with a minimal synthetic main and link it through
kcc's OWN `natPELink`, producing three 2560-byte PE images that are executed on
the dev host:

| Case | Body | Observed |
|---|---|---|
| A | `mov rax, 0; ret` | exit **0** |
| B | `mov rax, 3; ret` | exit **3** |
| C | calls the bootstrap-resolved `GetStdHandle` | exit **0** |

Case C is the PEB evidence: on these images the host loader does NOT snap the
IAT, so `GetStdHandle` is reachable only through kcc's own export walk. A bad
published address would fault rather than exit 0.

**The synthetic main body is TEST SCAFFOLDING, not language implementation.** It
is `mov rax, <code>; ret` -- a return value and nothing else. No frame slot, no
allocator, no `print`, no stack. It is NOT `print`, NOT `print_float`, and NOT a
general calling convention. Case C deliberately avoids printing because `print`
was later 151A work (Step 7, section 14) and out of scope for this slice;
surviving the call is the proof.

**Four defects found and fixed by the gate during this work** (all in kcc code,
all byte-visible, all mutation-relevant):

1. `natModrm` / `natMovRegImm32` omitted the low-3-bit register mask that the Go
   side applies via `.low()` at every call site. R8 (register 8) wrote `0x40`
   into the **MOD** field, silently turning `movzx r8d, [rdx]` into
   `movzx r8d, [rdx+disp8]` and eating the following immediate.
2. `natLoadScaled32` reproduced Go's `if / else-if / else` chain as two
   independent `if`s, so `disp == 0` with an RBP base fell through to
   `mod=10 + disp32` and grew the image by three bytes.
3. There was no `REX.X` selector; the REX.W selector (8) was used where `2` was
   needed, emitting `REX.W` where `REX.X` was required.
4. One export-name pair was mistyped: `GetStdHandle`'s `H|a` written `0x6140`
   (`'@'`) instead of `0x6148`. It surfaced as a SINGLE differing byte inside a
   982-byte shape -- the same one-digit class 152-B2 hit on a SHA-256 round
   constant, and the reason the gate derives the pair table independently a third
   time.

**CI.** Step *Run 151A Step 5/6 Windows entry, PEB bootstrap and kcc-PE execution
gate* runs `go test ./pkg/cli/ -run 'TestPhase151A5_'`. The pattern is distinct
from `TestPhase151A4_`, so a Step 5 regression cannot hide behind a green Step 4
step. The step drives the real kcc binary and executes its images; it is not a
source-text check.

**Still NOT claimed / still open.**

* **No whole-image parity with the Go engine for real programs.** The three
  executed images use a synthetic main. A kcc-compiled *user program* still
  cannot be built natively.
* **macOS entry/exit remains unimplemented** (`0x2000001`), and the macOS leg is
  unproven on any runner.
* **`print` is now implemented (Step 7, section 14), but `print_float`, floats,
  arrays, `for-in`, arena, string ops, `push`, records and maps remain open
  151A work in kcc.**
* **`kccOwnsNativeTargets` stays `false`.** Flipping it is 151D and remains
  blocked on the value model, for the reason `PHASE-151-BASELINE.md` 10 records:
  flipping it before kcc can emit a real image would compile and pass every
  parity test while reintroducing the dishonesty 151 was opened to remove.
* **The KIR pin is re-measured, and the failure first recorded here is now
  classified.** The whole-tree pin (`karkain kir --verify src/compiler/kir.kark`,
  the file the pin is defined over) measured **11756 text / 11756 verify** on
  this slice's tree (11412 + the 344 rendered lines Step 5/6 added) and **11859**
  after Step 7 -- the values `phase122_pipeline_ownership_test.go` now pins. The
  subtest whose failure was recorded here had verified `main.kark` -- a
  driver-inclusive assembly that no pin covers -- and died inside kcc with exit
  `0xC0000005` and empty output. That is the documented ~4 GB host low-RAM class
  (`kcc check`/`kir`/`verifykir` on the full tree all crash when free RAM is
  low), NOT the new code: with free RAM above ~1.7 GB the identical tree
  verifies at 11859, and the all-HEAD scratch copy verifies at 473 MB free.
  `TestPhase151A5_KIRPinHolds` now asserts the pin on `kir.kark` exactly, so the
  gate cannot confuse "kcc ran and agreed" with "kcc died"; the finding was
  classified rather than deleting the assertion to make a red host green.

## 14. 151A Step 7 - int `print` (IMPLEMENTED)

This is the first 151A surface that makes a kcc-compiled program's OUTPUT
observable. Every earlier slice could compute a value and could enter and exit
an image, but a kcc-side program could not say anything, so whole-image parity
against the Go engine had nothing to compare.

**Implemented.** `natPrintIntHelper` in `src/compiler/native_value.kark` mirrors
`emitHelpers`' `print_int` in `pkg/native/program.go` instruction for
instruction and in the same order: frame reservation, the sign test, the sign
write through `natWriteStdout` (the Linux `write` syscall), the two pushes and
pops across it, the divide-by-10 digit loop, the length computation, the payload
write, and the newline tail. Six emitter primitives were added in
`src/compiler/native_emit.kark` -- `natCqo`, `natDivReg`, `natStoreMem8`,
`natPushReg`, `natPopReg`, `natJns` -- each the byte-for-byte counterpart of the
identically named Go method. A `native-value-print` arm prints three corpus
shapes through the same no-Go-fallback contract as the earlier slices: the whole
helper, the digit loop alone, and a bare rodata reference.

**One deliberate difference from the byte-identical containers.** `print`
contains UNRESOLVED absolute-address placeholders (`movabs rsi, <rodata>`): a
rodata address is only known at link time. `natRodataRef` therefore emits the
same 10-byte placeholder form as `imm64Patch` (REX.W + B8+rd + eight zero bytes)
and RECORDS the patch position for the linker, exactly as the oracle does. The
corpus pins that shape on its own (arm 2); the whole-helper differential masks
only those two 8-byte immediates, while every opcode, ModRM byte, both rel32
displacements and the frame arithmetic must be byte-identical to the oracle's
real emission.

**Measured (all seven A7 tests green, 6.9 s).** The digit loop is byte-identical
to the Go emitter's sequence, built in the gate through exported API only, AND
to the bytes the gate states from the Intel SDM (`B9 imm32` / `48 99` /
`48 F7 F1` / `48 83 C2 30` / `48 FF CE` / `88 16` / `48 85 C0` / `0F 85 rel32`),
with the jnz displacement recomputed from first principles (-23). The whole
helper matches the oracle's real emission from a compiled printing program with
exactly two masked rodata sites, the digit loop occurs verbatim in that image's
`.text`, the corpus is non-vacuous and deterministic across runs, and the Go
side only runs kcc (no fallback). The KIR pin moved 11756 -> **11859** (+103,
measured; recorded in `phase122_pipeline_ownership_test.go`, and asserted
exactly by `TestPhase151A5_KIRPinHolds`).

**CI.** Step *Run 151A Step 7 int print gate* runs
`go test ./pkg/cli/ -run 'TestPhase151A7_'`. The pattern is distinct from
`TestPhase151A5_`, so a Step 7 regression cannot hide behind a green Step 5/6
step.

**Still NOT claimed.** A kcc-compiled *user program* still cannot be built
natively: `kccOwnsNativeTargets` stays `false` and the CLI native targets still
refuse with `error[K116]` (151D), because the value model has no call, frame or
rodata resolution for real programs yet. `print_float`, floats, arrays,
`for-in`, arena, string ops, `push`, records, maps and the macOS entry/exit
remain open 151A work. No execution evidence is claimed for this slice: the
corpus is a helper, not a program -- it has no `main`, no entry stub and no
resolved rodata, so it cannot be run.

## 15. 151A Step 8a - the SSE2 scalar-double primitives (IMPLEMENTED)

Step 7 made a program's OUTPUT observable for ints. Step 8 opens the float
half of the value model, and it opens with the ENCODER rather than with
statements, for the reason every earlier step opened at the lowest layer
first: nothing above the encoder can be trusted until the bytes underneath it
are the oracle's bytes.

**Scope.** The eight SSE2 scalar-double instructions the value model needs
before any float statement can exist:

| Karkain | Go (`pkg/native/emit.go`) | Encoding |
|---|---|---|
| `natAddsdXmmXmm` | `AddsdXmmXmm` | `F2` + REX + `0F 58` /r |
| `natSubsdXmmXmm` | `SubsdXmmXmm` | `F2` + REX + `0F 5C` /r |
| `natMulsdXmmXmm` | `MulsdXmmXmm` | `F2` + REX + `0F 59` /r |
| `natDivsdXmmXmm` | `DivsdXmmXmm` | `F2` + REX + `0F 5E` /r |
| `natUcomisdXmmXmm` | `UcomisdXmmXmm` | `66` + REX + `0F 2E` /r |
| `natCvtsi2sdXmmGp` | `Cvtsi2sdXmmGp` | `F2` + REX.W + `0F 2A` /r |
| `natCvttsd2siGpXmm` | `Cvttsd2siGpXmm` | `F2` + REX.W + `0F 2C` /r |
| `natXorpdXmmXmm` | `XorpdXmmXmm` | `66` + REX + `0F 57` /r |

Ten `natMask` selectors were appended at 53-62, continuing the table for the
reason the Step 1 block records: a selector table is a shared namespace, and
the Step 1 draft that started a block at 17 silently turned `natCall32`'s
`0xE8` into a `0x89` mov.

**The one subtlety this slice exists to pin.** The mandatory legacy prefix
(`F2` / `66`) must be emitted BEFORE the REX byte, and the `0F` escape sits
between them. The Go oracle emits `e.byte(0xF2); e.rex(...)` in that order and
kcc reproduces it. Emit REX first and, for `xmm8`-`xmm15` -- exactly the
registers REX.B selects -- the encoding silently changes meaning rather than
failing, which is the worst possible failure mode: a wrong program that runs.

**Corpus shape, and why it is not eight near-identical sequences.** A corpus of
eight similar sequences passes while the one easy thing is wrong. The four arms
cover the distinct encoding decisions instead:

| Arm | Covers |
|---|---|
| 0 | four arithmetic ops on `xmm0 <- xmm1` -- no REX at all, so this is the ONLY arm that catches a spurious `0x40` |
| 1 | `addsd xmm8,xmm9` and `divsd xmm15,xmm8` -- REX `0x45`, catches prefix/REX ordering |
| 2 | the `F2` family adjacent to the `66` family in one sequence -- catches a swapped prefix |
| 3 | both converts on high GP and high xmm registers -- the only arm with REX.W together with REX.R and REX.B |

**Evidence.** Gate `pkg/cli/phase151a8_float_test.go`, 5/5 PASS: byte-identity
against the REAL `pkg/native` Emitter on all four sequences (a differential,
not a golden); SDM-derived bytes for one representative of each encoding shape,
with the REX and ModRM fields computed from the field layout in the test rather
than copied from output; occurrence of `addsd xmm0,xmm1` verbatim in the
`.text` of a real oracle image compiled from a float program; non-vacuity and
determinism, with a total-byte floor so a corpus that stopped carrying eight
instructions could not report success; and the no-Go-fallback guard.

**Mutation-verified, twice, and the first one is the interesting one.**

* M1 emitted REX *before* the prefix in `natAddsdXmmXmm`. Result: `45 f2 0f
  58 c1` instead of `f2 45 0f 58 c1`. Both the differential and the SDM layer
  failed. **Arm 0 still passed**, because with `xmm0`/`xmm1` no REX is emitted
  at all -- which is precisely why arm 1 exists, and why a corpus built only
  from low registers would have shipped this bug.
* M2 gave `natUcomisdXmmXmm` the `F2` family instead of `66`. Result: `f2 0f
  2e c1`. Both layers failed.

Both mutations were reverted and the file verified SHA-identical to its
pre-mutation content.

**KIR pin.** 11859 -> 11950, +91, measured with `karkain kir --verify
src/compiler/kir.kark` (11950 text / 11950 verify), not predicted. The running
arithmetic is now `11412 (HEAD) + 344 (Steps 5/6) + 103 (Step 7) + 91 (Step 8a)
= 11950`. Re-pinned in both `phase122_pipeline_ownership_test.go` and
`phase151a5_win_test.go`; `KIRContinuity` and `KIRPinHolds` both PASS.

**Still NOT claimed.** No float STATEMENT exists in kcc: no `let` of a float,
no float arithmetic lowering, no float comparison as a real branch, no float
parameter, return or call. `print_float` is NOT ported -- it is a ~120
instruction helper (`Builder.emitPrintFloatHelper`, `program.go:4185-4390`) that
also needs `movq` in both directions, `movabs`, `and`, `shr`, `lea`, `movzx`
byte-load and a store-at-offset primitive, none of which kcc has yet. The
macOS entry/exit, and every other open item listed in section 14, remain open.
`kccOwnsNativeTargets` stays `false`. No execution evidence is claimed: the
corpus is instruction encodings, not a program.

**Suggested next slice: Step 8b**, `print_float`, which is the other mandatory
helper for observable output and is best done before statement lowering so
that float output is possible at all. It should be gated the same way, and its
`fresh()` label-allocation order must match the oracle's exactly, since the
rel32 displacements depend on it.
## 16. 151A Step 8b - print_float (IMPLEMENTED)

Step 7 made int output observable; Step 8a opened the float encoder. This ports
the OTHER mandatory output helper, so a float has a rendering path at all. It is
the largest single helper in the native backend:
`Builder.emitPrintFloatHelper` (program.go:4185-4390) is ~205 lines of Go and
**744 bytes** of x86.

**Two structural facts the port had to respect, and both were stated in the
oracle rather than guessed.** The Go helper interleaves writes with live values
and parks four of them in the frame, because RCX/RAX/RDX are volatile at *both*
the Win64 kernel32 boundary and the raw syscall -- the syscall itself assigns
RCX the return RIP -- so a caller-saved park is not enough in either container.
The frame map is `rsp+0..5` fraction digits, `rsp+8..15` the R11 digit count
(across the "." write), `rsp+16..23` the fraction bit pattern, `rsp+24..31` the
integer part, `rsp+45..63` the integer digit buffer. Parking the integer part is
what makes negative values correct: the sign write would otherwise clobber RCX
before it is read. Second, the `fresh()` labels are allocated at the points the Go
helper allocates them; label *names* affect no byte, but the *order* of the marks
does, because the rel32 displacements come from positions.

**Eight new primitives**, each the counterpart of the identically named Go
method, plus `natMask` selectors 63-68: `natMovRegImm64`, `natAndRegReg`,
`natShrRegImm`, `natLeaRegStack`, `natMovzxRegMem8`, `natStoreMem8Off`,
`natMovXmmRegGp`, `natMovGpRegXmm`.

Two of them are traps worth naming. `natMovXmmRegGp` (`movq xmm, r64`) takes the
**0x66** prefix, not 0xF2 -- every other double instruction in this file is
0xF2, so reaching for F2 is the natural mistake. And `natMovGpRegXmm` passes its
operands to `natRex` in the **opposite order** from every other primitive here:
for this instruction the xmm is the ModRM.reg field and the GP the rm field,
whereas the SSE arithmetic ops have the destination xmm in the reg field.
Copying the argument order from a neighbour yields a REX byte with R and B
exchanged, which is a valid-looking encoding of a different instruction.

**Why the differential extracts from a real image.** The obvious approach --
rebuild the helper with the Go emitter inside the gate -- is unavailable:
`Emitter.imm64Patch` is **unexported**, so `pkg/cli` cannot produce the
unresolved rodata reference the helper contains. A hand-written transcription
would have been exactly the "golden against my own transcription" failure mode.
So the oracle side is read out of a real compiled image instead, which is
stronger evidence anyway. Extraction uses two anchors and a **structural count,
not a length**: the start is the 17-byte prologue containing
`movabs rcx, 0x8000000000000000` (unique -- `print_int`'s prologue's third
instruction is `movabs rcx, 0x7FFFFFFFFFFFFFFF`), and the end is the **third**
`add rsp,0x40; ret` after it, because `print_float` has two early returns (the
inf and nan paths) before its fallthrough. The count is asserted, so a changed
return structure fails loudly instead of silently re-extracting a wrong span.

**TEN rodata sites, not six.** `-`, `inf` and a newline on the infinity path;
`-`, `nan` and a newline on the nan path; `-`, `0`, `.` and a newline on the
finite path. A first draft of the gate asserted six and the site-count assertion
caught it immediately -- a baseline with a wrong expectation is worse than none.

### Two real defects, both found and both pinned

**1. The sign mask clamped to max-int64 (introduced by this slice).** The
constant is `1<<63 = 9223372036854775808`, one past the largest signed 64-bit
value. Karkain's integer surface is signed 64-bit, so writing that literal
clamps it to `...807`, and the helper emitted `48 b9 ff ff ff ff ff ff ff ff`
where the oracle emits `48 b9 00 00 00 00 00 00 00 80`. The `and r11, rcx` that
extracts the sign would then test the wrong mask and **every negative float
would print without its sign**. The fix writes it as
`0 - 9223372036854775807 - 1`, which is exact, and `natBytesLE`'s per-byte
masking turns the negative into correct two's-complement bytes.

This is the **151B constant-class bug recurring in a new place**, and it is
worth being explicit about how it was caught: by decoding the *oracle's* bytes
by hand and comparing, which is precisely the practice the 151B report says
caught the original. The differential would eventually have caught it too, but
only incidentally -- and a clamped literal now has its own dedicated subtest
(`TestPhase151A8b_SignMaskConstantIsExact`) so the differential is never the
only thing standing between it and a wrong image.

**2. A latent pre-existing bug in `natMemBaseOff` (corrective change against a
frozen baseline).** `natMemBaseOff`'s RSP branch emitted SIB `natMask(43)` =
`0x25`. That is wrong: `0x25` is index-none / base-**RBP**, and in `mod=00` an
RBP base means "**no base, disp32**", i.e. the *absolute* form. The correct SIB
for a real RSP base with a displacement is `0x24`, which is `natMask(23)` --
the value `natMemRsp` already uses.

The bug was latent from the moment `natMemBaseOff` was written because nothing
called it with `base = RSP`: `natStoreStack`/`natLoadStack` route through
`natMemRsp` (correct), and the Step 5/6 PE export walk reads gs-relative
structures through a general register. Step 8b's `natStoreMem8Off` with base
RSP is the first caller to reach the branch, and the differential reported
`88 04 25` against `88 04 24` at byte 499.

Note `natMask(43)` is **not** simply wrong: `natMovRegGsMem` legitimately uses
it, because `gs:[disp32]` *is* the absolute form. So the fix is at the call
site inside `natMemBaseOff`, not in the table. This is recorded per the
Governance rule as a corrective change against a frozen slice; it changes
emitted bytes for any `[rsp+off]` access made through `natMemBaseOff`, and the
only such sites are Step 8b's own, so no previously-shipped image changes.

### Evidence

Gate `pkg/cli/phase151a8b_printfloat_test.go`, 5/5 PASS: the 744-byte
byte-for-byte match against the oracle's **real** image emission with the ten
rodata immediates masked; all ten immediates asserted zero; the sign-mask
constant pinned; corpus non-vacuity and determinism with the 744-byte length
pinned; and the no-Go-fallback guard.

**Mutation-verified, twice.**

* M1 reintroduced the clamped sign mask. Both the differential and
  `SignMaskConstantIsExact` failed, and the latter's diagnostic named the exact
  fix.
* M2 reverted the SIB fix in `natMemBaseOff`'s RSP branch. Only the differential
  failed, at byte 523 -- which is the right division of labour, since the
  placeholder and constant subtests have nothing to say about a SIB byte.

Both reverted, both files verified SHA-identical to pre-mutation content.

### Collateral

`phase151a8_float_test.go` asserted the corpus had exactly 4 lines; Step 8b adds
a fifth. Relaxed to "at least 4", since arms 0-3 are unchanged and the byte floor
in the non-vacuity subtest still guards the encoder-level content.

**KIR pin.** 11950 -> 12163, +213, measured with `karkain kir --verify
src/compiler/kir.kark` (12163 text / 12163 verify). The running arithmetic is now
`11412 + 344 (Steps 5/6) + 103 (Step 7) + 91 (Step 8a) + 213 (Step 8b) = 12163`.
Re-pinned in both `phase122_pipeline_ownership_test.go` and
`phase151a5_win_test.go`; `KIRContinuity`, `KIRPinHolds` and the full
`TestPhase151*` sweep are green.

**Still NOT claimed.** No float STATEMENT exists in kcc: no `let` of a float, no
float arithmetic lowering, no float comparison as a real branch, no float
parameter, return or call. `print_float` is a helper, not a program -- no `main`,
no entry stub, no resolved rodata -- so no execution evidence is claimed and the
argument for the bytes being right remains transitive. The macOS entry/exit, and
every other open item listed in section 14, remain open. `kccOwnsNativeTargets`
stays `false`.

**Suggested next slice: Step 8c**, float statement lowering -- `let` of a float,
`+ - * /` through the Step 8a primitives, and the six comparisons as real
branches. That is the first slice where the two halves (encoder and helper)
become reachable from an actual statement.
## 17. 151A Step 8c - float statement lowering in kcc (IMPLEMENTED)

Step 8a ported the SSE2 primitives and Step 8b the print_float helper. Both were
unreachable from an actual statement -- you could build the instructions and the
helper, but no code path reached them. This slice adds the LOWERING, and it is
what makes whole-image parity testable for a real program.

**The load-bearing fact, and the reason this slice is smaller than it looks:** a
float local is **ONE 8-byte unit in exactly the frame slot an int occupies**.
`emitLet`'s float path is therefore the int path's `StoreStack(RAX, off)` with
the f64 bits left in RAX by `emitFloat`. Nothing about the frame changes between
kinds, and `rsp` never moves. A first draft that assumed floats needed their own
slot arithmetic would have diverged from the oracle immediately.

Mirrored from `pkg/native/program.go`: `emitFloat` -> `natFloatLit` /
`natFloatLocal`; `emitFloatBinary` -> `natFloatBinary` (staging through
`binTemp + depth*8`, then xmm0/xmm1); `emitFloatDiv` -> the zero-divisor
short-circuit; `emitFloatNeg` -> XOR with 1<<63; `emitFloatCond` -> the six
comparisons as real branches. Three new branches plus six `natMask` selectors
69-71: `natJp`, `natJbe`, `natJb`.

**`jp` carries meaning rather than convenience.** `ucomisd` sets PF on an
*unordered* result (either operand NaN), so a bare `jnz` after it would treat
`NaN == NaN` as true. The oracle's `==` emits `jp` then `jnz` to the false label,
and `!=` needs an explicit intermediate hold label because neither `jnz` nor
`jz` alone can express "or unordered".

**Two comparison facts that a transcription error would most easily get wrong,
and both are pinned:**

1. *The operand swap.* `<` and `<=` feed `ucomisd` **swapped**
   (`ucomisd xmm1, xmm0`); `>` and `>=` feed it unswapped. `ucomisd` computes the
   compare in SOURCE order, so the swapped form means "right <= left", which is
   what lets a single `jbe` express "not (left < right)". Swapping backwards
   produces a comparator that **compiles and computes the wrong answer**.
2. *`jbe` vs `jb`.* `<` uses JBE (CF=1 **or** ZF=1) and `<=` uses JB (CF=1
   alone). The gate asserts the expected branch is present AND that its sibling
   is absent.

**Corpus.** Thirteen arms: the literal store, the local reload, `+ - * /`,
unary negation, and the six comparisons. `binTemp` is taken at the oracle's real
offset for a one-local frame (locals occupy 0..7, so binTemp starts at 8), since
the staged bytes are part of the shape. The two IEEE-754 patterns are passed in
from the caller rather than written in the kcc file, so the constants live in one
place and no new magic numbers are introduced there.

**Why this slice gets a DIRECT differential where Step 8b could not.** Every
primitive involved here is exported (`MovRegImm64`, `StoreStack`, `LoadStack`,
`MovXmmRegGp`, `MovGpRegXmm`, the four arithmetic ops, `XorpdXmmXmm`,
`UcomisdXmmXmm`, `Jp`, `Jnz`, `Jmp`, `Jbe`, `Jb`), and **none of these shapes
contains a rodata reference**. Step 8b had to extract from a real image because
`Emitter.imm64Patch` is unexported; there is no such obstacle here, so the oracle
is built with the real Emitter directly and compared **RAW**, with no masking.

### Evidence

Gate `pkg/cli/phase151a8c_floatstmt_test.go`, 5/5 PASS: all thirteen arms
byte-for-byte equal to the real Go Emitter's output; the two comparison facts
checked against SDM-derived ModRM and opcode bytes; the divide short-circuit's
two rel32 displacements computed from first principles; non-vacuity and
determinism with a total-byte floor; and the no-Go-fallback guard.

**Mutation-verified, twice.**

* M1 unswapped the `<` comparison. Both the differential and the SDM layer
  failed, and the SDM layer's diagnostic named the exact operand pair
  (`reg=0 rm=1, want reg=1 rm=0`).
* M2 removed the zero-divisor short-circuit entirely. Four layers failed,
  including the dedicated displacement test -- a divide that produces an infinity
  instead of `+0.0` is exactly what the guard exists to prevent.

Both reverted, the file verified SHA-identical to pre-mutation content.

**A wrong expectation in the test, recorded because the 151A method warns about
it specifically.** The displacement test first read both branches' displacements
with one helper and reported `jmp displacement = -234881024`. The two branches
have different opcode widths: `jnz` is `0F 85 cd` (two opcode bytes, `cd` at
at+2) while `jmp` is `E9 cd` (**one** opcode byte, `cd` at at+1). The emission
was correct and the test was wrong; the fix is two readers, and the trap is now
named in the test.

**One implementation slip, caught immediately.** The first corpus run emitted
**all zeros for all six comparison arms**, because `$false` was referenced but
never `Mark`ed. In the oracle the false label belongs to the *caller* (`emitIf`
marks its `elseLabel` after the consequence), so a self-contained corpus must
mark it at the end of the condition. The corpus now marks it in all six arms.

**KIR pin.** 12163 -> 12302, +139, measured with `karkain kir --verify
src/compiler/kir.kark` (12302 text / 12302 verify). The running arithmetic is now
`11412 + 344 + 103 + 91 + 213 + 139 = 12302`. Re-pinned in both
`phase122_pipeline_ownership_test.go` and `phase151a5_win_test.go`;
`KIRContinuity`, `KIRPinHolds` and the full `TestPhase151*` sweep are green
(702.6s).

**Still NOT claimed.** No execution evidence: the corpus is thirteen reference
shapes, not a program -- no `main`, no entry stub, no resolved rodata. The
lowering is exercised directly rather than through a driver, so this slice does
not yet prove that kcc *chooses* the float path for a real program; it proves the
bytes are right when it does. Float **parameters, returns and calls** remain
open, as do arrays, arena, string ops, `push`, records, maps, the macOS
entry/exit, and every other item listed in section 14. `kccOwnsNativeTargets`
stays `false`.

**Suggested next slice: Step 8d**, float parameters, returns and calls through a
function table, which completes the float kind end to end and is the last thing
between the float value model and `emitLet`/`emitExpr` being able to *dispatch* on
a real function's float locals.
## 18. 151A Step 8d - the call ABI, int first (IMPLEMENTED)

Phase 148's convention, ported: the caller stages every argument to a per-arg
spill, loads all units back into RDI,RSI,RDX,RCX,R8,R9, materialises R10 with
the extras base if any unit exceeded the budget, and calls; the callee homes
each parameter from its argument register into its frame slot at entry, before
R10 can die.

### A misreading, corrected before it cost anything

This slice was originally scoped as "float parameters, returns and calls", and an
intervening analysis claimed it required inserting a new frame region for the
per-arg spill, which would have shifted `extrasBase`, `next` and `frame` and
re-pinned every frame-dependent displacement in Steps 1-8c.

**That analysis was wrong**, and the correction matters more than the slice. The
oracle does not insert a region. It computes:

```go
func (b *Builder) argTemp(i int) int {
    return b.frame - argSpillBytes + i*16
}
```

The spill is addressed **downward from the frame size**, 16 bytes per argument.
Phase 151A Step 1 had already reserved exactly that: `native_value.kark`
computes `frame = next + natArgSpillBytes()` with `argSpillBytes() = 96`
(6 x 16) and exposes `frame` as layout slot 6.

So no region is inserted and **nothing in the frame moves**. The slice's own
gate asserts this negatively: `TestPhase151A8d_FrameIsUnchanged` checks the
prologue still subtracts 616 and that `argTemp(0) + 96 == frame`. Had a region
really been missing, those could not hold.

### Scope: int only, and why floats still wait

Every int is ONE unit, so `stageUnit`'s string special case -- where RSI is the
high half of a two-unit argument and lands 8 bytes higher -- cannot arise here.
Float is also one unit (`kindUnits(KindFloat) == 1`), so it rides the identical
path and needs only kind plumbing, which is a separate follow-up rather than new
emission.

### Implementation

`natArgTemp(i, frame)`, `natCallArgsN`, `natCallArgsExtras`, `natPrologueArgsN`,
`natPrologueExtras`, `natArgReg(i)`, and the seven-arm `natCallCorpus`. The
literals are 1..7 so each staged value is distinguishable in a hex dump: a
corpus of identical values would hide an off-by-one in the spill indexing, which
is exactly the bug this slice could plausibly introduce. `frame` and
`extrasBase` are passed in from the caller, so the test states the oracle's real
layout numbers and the kcc file gains no magic numbers.

**RAW differential.** Every primitive involved is exported
(`MovRegImm64`, `StoreStack`, `LoadStack`, `LoadBaseOff`, `LeaRegStack`, `Call`,
`Mark`, `SubRegImm32`) and none of these shapes contains a rodata reference, so
the oracle is built with the real Emitter and compared with no masking.

### Evidence

Gate `pkg/cli/phase151a8d_call_test.go`, 6/6 PASS: all seven arms byte-for-byte
equal to the real Go Emitter; the spill indexing derived independently in the
test with the 16-byte stride asserted; the extras unit shown to travel to the
extras area rather than a spill slot, with R10 materialised; the frame asserted
unchanged; non-vacuity and determinism; and the no-Go-fallback guard.

**Mutation-verified.** M1 staged unit 6 to a spill slot instead of the extras
area. Both the differential and the dedicated extras subtest failed, the latter
printing the offending bytes.

### Three wrong expectations in the test, all mine, all caught

1. **`e.Call("fn_f")` panicked** because the label was never marked. The Go
   emitter panics on a reference to an unmarked label by design; kcc raises
   `error[K117]`. Both are the same contract in their own language, and the
   transcription needed the same `Mark` the kcc corpus has.
2. **The spill encoding was assumed to be disp8.** `argTemp(0) = 520` does not
   fit a signed byte, so the encoding is `mod=10` with a disp32. The test read a
   displacement of 8 off an unrelated store. The fix checks the RANGE first and
   builds the expected encoding from it -- the same class of mistake as assuming
   a literal fits a signed field.
3. **`callExtras = 608` was written into a disp8 byte.** Same class again: 608
   does not fit, so the store and the `lea` are asserted in their disp32 form.

### One implementation slip

The first build emitted **all zeros for all four call arms**, because `fn_f` was
referenced but never `Mark`ed -- the oracle's callee label is marked by the
function's own definition, which a self-contained corpus does not have. Same root
cause as Step 8c's `$false`, now the second time this corpus shape has needed it.

**KIR pin.** 12302 -> 12389, +87, measured with `karkain kir --verify
src/compiler/kir.kark`. Running arithmetic:
`11412 + 344 + 103 + 91 + 213 + 139 + 87 = 12389`. The delta is code growth only;
no frame displacement moved. `KIRContinuity`, `KIRPinHolds` and the full
`TestPhase151*` sweep are green.

**Still NOT claimed.** No execution evidence: seven reference shapes, not a
program. String and record arguments are out of scope -- int only -- so the
`stageUnit` high-half case and the two-unit path are NOT covered. Float
parameters, returns and calls are NOT covered, for the reason above. Arrays,
arena, string ops, `push`, records, maps and the macOS entry/exit remain open.
`kccOwnsNativeTargets` stays `false`.

**Suggested next slice: Step 8e**, floats through the call ABI -- the kind
plumbing only, which should be small now that the mechanism exists and is gated.
Alternatively, strings, which need the two-unit `stageUnit` path and would close
the register/extras boundary for the first time.
