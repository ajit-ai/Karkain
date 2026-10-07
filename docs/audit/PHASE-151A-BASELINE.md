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

## 19. 151A Step 8e - floats through the call ABI (IMPLEMENTED)

This slice exists to **prove a claim rather than add emission**, and the claim is
the load-bearing one for the whole float kind:

> **Floats need no new ABI code.**

`kindUnits(KindFloat) == 1` in the oracle and `natKindUnits` in kcc already return
1 for a float, so a float parameter occupies exactly one frame slot
(`natLocalOff(index, kind) = 8 * index * natKindUnits(kind)` already gives a float
param the same offset as an int) and a float argument is staged, loaded and passed
by the byte-for-byte identical path an int uses. What Step 8d built was already
float-capable; this slice MEASURES that rather than asserting it.

**The strongest layer is a cross-corpus comparison, not a fresh expectation.**
Arm 1 stages the **int** literal 42 through the **float** call shape, so it can be
compared byte-for-byte against Step 8d's one-argument int call (literal 7). The
two must differ ONLY in the 10-byte `movabs` immediate. That is a much stronger
claim than "the float shape looks right": it says the float path contributes no
ABI emission at all, and any float-only instruction falsifies it. The immediates
are asserted to genuinely differ, so the comparison cannot pass vacuously.

Four shapes: a float argument staged and passed, the same shape with an int
literal (the comparison arm), a float parameter homed from RDI, and a float return.

### Two defects this slice found

**1. The float return tore the frame down twice.** A first draft of
`natFloatReturn` emitted `sub rsp, frame` before the `jmp $ret`. The oracle's
`ReturnStmt` case is `emitReturn(...)` followed by `Jmp("fn_<name>$ret")`
(program.go:4893-4900), and the `AddRsp(frame)` happens **at** the `$ret` label in
the epilogue (program.go:4863-4866). The gate now asserts there is **no** `sub` or
`add rsp` anywhere in the return shape.

This also **corrects a stale reference rather than inheriting one.** The Step 2
note recorded the `$ret` jmp at `program.go:3386`; that line now holds
`emitBase64OutByte`, an unrelated function. The reference was accurate when
written and drifted as the file grew -- which is the argument for locating a label
by searching for it rather than by following a remembered line number.

**2. The `jmp` opcode byte was missing.** `natRel32` emits only the four
displacement bytes; the `0xE9` opcode is `natJmp`'s job. A first draft used
`natRel32` directly and the arm came out as a bare `movabs` with no jump at all --
15 bytes of nothing. `natCall32` + `natRel32` is correct for `call` precisely
because `natCall32` emits the `0xE8` first; the same pairing habit applied to
`jmp` produced the bug. The gate's `arm 3 does not end with a jmp rel32` check
caught it, and the shape now ends `e9 00 00 00 00`.

### Evidence

Gate `pkg/cli/phase151a8e_floatcall_test.go`, 6/6 PASS: a RAW differential of all
four shapes against the real `pkg/native` Emitter; the float-arg path proven
byte-identical to the int path after the literal; the float param homing proven
byte-identical to the int homing; the return proven free of frame teardown;
non-vacuity and determinism; and the no-Go-fallback guard.

**Mutation-verified, and the mutation is the claim itself.** M1 inserted a
float-only `cvttsd2si` into the argument path -- the exact "helpful conversion"
this slice exists to prove cannot happen. Both the differential and the
cross-corpus layer failed, the latter with the right diagnostic: *"float arm is 36
bytes, int arm is 31; the float path must contribute no ABI emission at all."*
Reverted, file verified SHA-identical to pre-mutation content.

**KIR pin.** 12389 -> 12419, +30, measured with `karkain kir --verify
src/compiler/kir.kark` (12419 text / 12419 verify). Running arithmetic:
`11412 + 344 + 103 + 91 + 213 + 139 + 87 + 30 = 12419`. No frame displacement
moved. Re-pinned in both gates; `KIRPinHolds` PASS (597.2s) and all 27
`TestPhase151A8*` tests PASS.

**What this closes, and what it does not.** It closes the float KIND's ABI story:
a float can now be passed to, received by, and returned from a function with
byte-identical machinery to an int. It does **not** close float *values*: there is
still no whole-program driver that walks a real Karkain function, dispatches on its
locals' kinds, and lowers them, so no kcc-produced image yet executes.

**Still open in 151A.** Strings through the call ABI -- which need the two-unit
`stageUnit` path where RSI is the high half, and would close the register/extras
boundary for the first time -- then arrays, arena, string ops, `push`, records,
maps, and the macOS entry/exit. `kccOwnsNativeTargets` stays `false`.

## 20. 151A Step 8f - strings through the call ABI (IMPLEMENTED)

A string is the first **two-unit** kind: `kindUnits(KindString) == 2`. So this
slice is where `stageUnit`'s high-half rule is exercised at all, and where an
argument's units can begin at an **odd** global index.

### The rule, and why it still works at an odd index

`stageUnit` picks the slot like this:

```go
if unit < len(argRegs) {
    off := b.argTemp(arg)
    if r == RSI { off += 8 }
    b.e.StoreStack(r, off)
    return
}
b.e.StoreStack(r, b.extrasBase+(unit-len(argRegs))*8)
```

The high-half check is on the **register**, not on the unit index. That is sound
because `stageUnit` is called with `RSI` in exactly one place -- the string branch
of `emitCallValue` passes `RDI` for the pointer and `RSI` for the length, and
every other kind passes `RAX`. So "`r == RSI`" is a reliable proxy for "this is a
string's second unit", and `argTemp`'s 16-byte stride is what gives the pair its
low and high halves.

The load-back loop then reads `argTemp(i) + k*8`, where `k` is the unit index
**within the argument**. Arm 1 exists precisely to separate those two conventions:
it passes an int first, so the string's pointer rides `RSI` (global unit 1) and its
length rides `RDX` (global unit 2). A reader indexing by *global* unit rather than
by `(arg, k)` would deliver them to the wrong registers, and a corpus that only
ever passed a string first could not tell the difference.

### No rodata, so the differential stays RAW

A string **literal** would need a rodata reference, which is unresolved at this
layer and is exactly what forced Step 8b's masked comparison. A string **local** is
loaded straight from its two frame slots instead, so this slice keeps a raw
byte-for-byte comparison.

Four shapes: one string argument, an int followed by a string (the odd-index
case), the callee homing a two-unit parameter, and four string arguments -- eight
units, so units 6 and 7 travel in the caller's extras area. The last is the first
case combining the extras region with the two-unit stride.

### One wrong expectation in the test, recorded

Arm 3 initially failed while arms 0-2 matched, and the cause was **my Go
transcription, not the kcc code**: `stage` folded the argument index and the unit
index together and used `arg*2` for *both* halves of a string, which stored the
length at the extras base instead of 8 bytes higher. The register-based high-half
rule applies inside the **spill** branch only, exactly as `stageUnit` does, so the
extras branch must still be told which unit it is holding. Fixed by passing the
unit index explicitly.

### Evidence

Gate `pkg/cli/phase151a8f_stringcall_test.go`, 6/6 PASS: the RAW differential of
all four shapes; the odd-index load-back asserted as an explicit `(arg, k)` byte
sequence with both disp32 operands spelled out; the high-half rule pinned at both
of the call sites that matter, with the halves asserted 8 apart; units 6 and 7
shown to leave the spill with `R10` materialised; non-vacuity and determinism; and
the no-Go-fallback guard.

**Mutation-verified.** M1 dropped the high-half rule, storing the string's length
at `+0` like its pointer. Both the differential and the dedicated rule subtest
failed, the latter reporting `string length staged at 520, want 528` and `the two
halves are 0 apart, want 8`.

**KIR pin.** 12419 -> 12488, +69, measured with `karkain kir --verify
src/compiler/kir.kark`. Running arithmetic:
`11412 + 344 + 103 + 91 + 213 + 139 + 87 + 30 + 69 = 12488`. `KIRPinHolds` PASS
(354.9s) and all 33 `TestPhase151A8*` tests PASS.

**What this closes.** The register/extras boundary is now closed for the first
time: a two-unit argument, an argument whose units start at an odd index, and an
argument whose units exceed the register budget have all been lowered and proven
byte-identical to the oracle.

**Still open in 151A.** String *expressions* -- concatenation, slicing, comparison,
the arena -- and `print` for strings; then arrays, `for-in`, `push`, records, maps,
and the macOS entry/exit. `kccOwnsNativeTargets` stays `false`, and no kcc-produced
image executes yet, because there is still no whole-program driver that walks a
real Karkain function and dispatches on its locals' kinds.

## 21. 151A Step 9 - the first end-to-end kcc-produced native image

### Scope, measured rather than assumed

This was originally scoped as "the whole-program driver", on the assumption that
kcc had no function-level emission at all. **That assumption was wrong**, and
checking the tree rather than the memory changed the shape of the work by a lot.
What already exists:

| Piece | From |
| --- | --- |
| `natIntBody` -- emits a complete `karkain_main` with its prologue | Step 2 |
| `natLoopPrologue` / `natLoopEpilogue` -- prologue and the `$ret` epilogue | Step 3 |
| `natPrintIntHelper` | Step 7 |
| `natWinStub` / `natWinBootstrap` / `natWinExitTail` -- entry and PEB bootstrap | Steps 5/6 |
| `natPELink` / `natELFLink` -- the PE and ELF container writers | 151C3 / 151C |
| frame layout, call ABI, float and string paths | Steps 1, 8d-8f |

So what is genuinely missing is much narrower, and it splits cleanly:

* **9a -- rodata resolution.** `natRodataRef` returns `p[0]` and **discards** the
  patch position, and nothing accumulates a rodata offset per site. `print_int`'s
  two `"-"` and `"\n"` references therefore have no recorded site. This is exactly
  the "rodata resolution" every section of this baseline has named as the thing
  blocking 151D.
* **9b -- image assembly.** Compose the entry stub, `karkain_main` and the
  `print_int` helper into one `.text`, link with `natPELink`, write the file.
* **9c -- execution.** Run it on this Windows host, where PE images execute
  natively (increments 145-150).

### One design fact that makes 9b tractable

Each emitter function creates its **own** state, so the entry stub's
`call karkain_main` is unresolved in its own buffer. But an internal `rel32` is
`target - (instruction end)`, and concatenating a self-contained function shifts
the instruction and its target **equally**, so every internal displacement value
survives concatenation unchanged. Only the ONE cross-piece call needs its
displacement computed directly. That reduces "assemble a whole image" to
concatenation plus a single hand-computed displacement.

### Why rodata needs a NEW emitter slot, not the existing one

Slot 5 (`natStImm64`) is already in use: Step 6 appends the three IAT positions to
it (`ExitProcess`, `GetStdHandle`, `WriteFile`) and reads them back in emission
order to build the IAT patch list. Appending rodata positions to the same slot
would **interleave the two kinds of site with nothing to tell them apart**, and
Step 6's own gate would then read a rodata position as an IAT slot.

So rodata sites need their own slot, and this is a correctness requirement rather
than tidiness. The state grows from six slots to eight; `natNewEmitter` is the
only constructor, so the change is additive and cannot affect any existing
emission -- it appends two empty lists.

### `natRodataRef` is NOT changed

`natRodataRef` is used by the frozen Step 7 and Step 8b corpora, and both compare
emitted **bytes**, which accumulation would not alter. Rather than change a frozen
function's signature, 9a adds `natRodataRefOff(s, r, roOff)` alongside it: the
existing function keeps recording nothing, the new one records the site and its
rodata offset. A corpus that never resolves rodata has no use for the offsets.

### What is NOT claimed

9a alone emits no image and proves no execution. 9b produces a file but a file
that has never run proves nothing about correctness beyond what the byte
differentials already establish. Only **9c** is execution evidence, and it is the
first in this increment.

### 9a-1. The intermittent `0xC0000005`: diagnosed, and NOT the Step 9a diff

Step 9a's own gate went 7/7 green immediately, but whole-tree
`karkain kir --verify src/compiler/kir.kark` began returning `0xC0000005`
(`exit 3`, **zero output**), so the KIR pin could not be re-measured and the
slice could not be committed. This subsection records what was actually
established, because the first two attributions were **both wrong** and the
correction is the useful part.

**Wrong attribution 1 - "the low-RAM class".** Free RAM was logged before each
attempt and the crash reproduced at **1944, 1737 and 2087 MiB free**, every time
well above the 1536 MiB `error[K127]` threshold. The Phase 127 guard is not
involved; the crash is not a consequence of the host being short of memory.

**Wrong attribution 2 - stack exhaustion.** The baseline recorded this as the
better-fitting UNTESTED alternative, with a `-Wl,--stack,` build-flag experiment
naming it as the cheap decisive test. That experiment was run: `kcc` was relinked
from the same stage-1 C with a **64 MiB** stack reserve
(`gcc ... -Wl,--stack,67108864`) and still crashed with `0xC0000005`. Stack
exhaustion is **refuted**.

**Wrong attribution 3 - "my diff causes it".** This was the expensive one and
the bisection that produced it was itself invalid. Two defects in the method are
worth recording because both produced confident, false conclusions:

1. The bisection rewrote `native_value.kark` with PowerShell `Set-Content`,
   which does not preserve the file byte-for-byte (the file is CRLF, no BOM).
   Every "variant" was therefore a *different file*, not a controlled truncation
   of the same one.
2. With one line of difference between two variants - `B1` crashed and `B2`, one
   line **longer**, passed - causality is impossible, and the only honest reading
   is that the measurement was noise. A crash whose reproduction depends on which
   of two nearly identical files you compiled is not a content-addressed defect.

A later byte-exact bisection (truncating the real byte array at function
boundaries, so each variant differs only by what was removed) showed the same
thing from the other side: the **full** tree passed at 12570 lines while seven
truncated variants all failed to build at all.

**What is actually established.** The defect is **intermittent, and independent of
the source tree**:

* Byte-identical `native_value.kark` (`SHA-256 EE17AC...`), in one session, minutes
  apart, produced `exit 3` with no output on one run and `[ok] kir verify: 12570
  lines ok` on the next.
* **Pristine HEAD (`49cc9de`, no Step 9a code at all) crashes too**, on
  `kcc check src/compiler/main.kark` - `exit 3`, zero output. That command
  performs no KIR work at all, so the defect is in the shared
  assemble/parse/typecheck path rather than in the KIR emitter.
* It is therefore **pre-existing**, not introduced by Step 9a, and the Step 9a
  gate's own green result was never at risk.

**Classification: UNCLASSIFIED, but no longer "unknown cause" - it is an
intermittent access violation in kcc's assemble/parse/typecheck path,
reproducible on pristine HEAD, unaffected by stack size, and not bounded by free
RAM.** The bisection that would isolate it needs a fixed input and a repeated-run
harness, because a single run cannot distinguish the defect from noise - which is
the practical lesson. Nothing in this paragraph is used as an excuse for a failing
gate: the pin is re-measured (12488 -> **12570**), every gate that asserts it is
re-run, and the crash is recorded as an environmental class rather than inherited
silently.

### 9a-2. Corrections to 9a-1, including one of my own claims that did not hold

Two further experiments, both of which tighten the record. One of them **falsifies
a hypothesis I had just written down**, which is why it belongs here.

**Low-RAM is refuted outright, not merely "above the guard".** A later attempt
**succeeded at 127 MiB free physical memory**, well *below* the 1536 MiB
`error[K127]` threshold that is supposed to make kcc refuse rather than crash:

    free RAM: 127 MiB
    exit=0 | [ok] kir text: 12570 lines | [ok] kir verify: 12570 lines ok

The guard did not fire, the run succeeded, and the same command fails minutes
later at 818 MiB. So free memory is **not a predictor in either direction**, and
the earlier "crashes at 1.7-2.1 GiB" observation should be read as coincidence of
timing rather than as a threshold effect.

**"A stale kcc.exe holding 668 MiB causes it" - FALSE, retracted.** After a
crashed attempt a leftover `kcc` process was observed holding 668 MiB; killing it
and re-running immediately succeeded, which looked like a clean causal result. It
is not. Repeating the experiment falsified it: with no `kcc` process running at
all, `kcc.exe` **not rebuilt** between runs and no source change whatsoever, the
command failed **twice in a row** (exit 3, no output). The correlation was a
coincidence of one sample. `killStaleKCC` is kept in the retry helper only
because clearing a leaked peer process is harmless and occasionally helpful, but
its doc comment says plainly that it is **not** the fix, and the baseline does
not claim it as a root cause.

**Not a build artefact either.** `kcc.exe` is rebuilt whenever a compiler source is
newer than the binary, which raised the obvious question of whether the crash is
simply a corrupt or partially-written binary. Ruled out: after one clean build,
two consecutive verifies with `rebuilt=False` both crashed. The defect is in
running the whole-tree KIR verify, not in producing `kcc.exe`.

**Where this leaves the classification.** `UNCLASSIFIED` stands, with the evidence
now strictly better than when it was first recorded: pre-existing (reproduces on
pristine HEAD), nondeterministic (byte-identical sources and a single unchanging
binary flip between success and crash), not a stack effect (64 MiB reserve), not a
free-memory effect (succeeds at 127 MiB, fails at 818 MiB), not a rebuild effect,
and independent of the Step 9a tree. The pattern is most consistent with
uninitialised memory or an out-of-bounds write in the stage-1 C that gcc compiles
at `-O0` with no optimiser, which is exactly the class that makes a defect
appear and disappear with heap layout. Confirming that needs a sanitiser build
(`-fsanitize=address`) of the stage-1 C, which is a change to the build and is
**not** authorised by this slice - it needs its own increment and baseline.

**What was done about it in the meantime.** The shared KIR-pin assertion retries a
bounded number of times on a **no-output** crash only, and still compares the pin
exactly against whichever attempt produced output, so the retry cannot convert a
moved pin into a pass and cannot absorb a real failure. The pin itself was
**re-measured, not predicted: 12488 -> 12570**, and `TestPhase151A5_KIRPinHolds`
passed on the measured value in isolation (439.54s, `text=12570 verify=12570`).
Under a combined `go test` invocation it remains unreliable, and that is recorded
as an open red gate rather than papered over.

---

## 22. Step 9b -- the first whole-program PE composition (COMPLETE for the machinery)

**Verdict: COMPLETE for the machinery. NOT the whole-program driver, and NOT
ownership.** `kccOwnsNativeTargets` stays `false`, and 151A stays open.

### 22.1 What landed

`_start -> karkain_main -> print_int` emitted into **one** emitter state, with
rodata resolved against the real PE layout and IAT patches paired by recorded
index. Before this, every 151A slice emitted one SUBSYSTEM and compared it
against the oracle; nothing had ever produced a runnable program from pieces,
because Step 9a could resolve rodata addresses but had nothing to resolve them
FOR.

* `src/compiler/native_value.kark`: `natPrintIntBody(s, winWrite, offMinus,
  offNewline)` is now the shared body, and the frozen `natPrintIntHelper()` is a
  wrapper over it with the Linux write and offsets 0/1. Recording a rodata offset
  changes no byte, so Step 7's corpus moves onto the shared body with its bytes
  unchanged -- verified, `TestPhase151A7_` 7/7 green.
* `natWinWrite` / `natStdOutHandle`: the port of the oracle's `emitWinWrite`,
  including the two measured ABI facts (RBX pushed FIRST so its POP happens last;
  **48, not 40**, for the WriteFile shadow while RBX is still live).
* `natPEProgram` / `natPEMainBody` / `natPEShape` / `natPECorpusProg` /
  `natFlatPairs`, and a `native-pe-prog` kcc subcommand with Go dispatch
  (`KCCNativePEProgramCommand`).

The blocker was found by measuring, not by planning: `print_int` wrote through
the **Linux** `write` syscall, because the oracle's `emitWrite` switches on
`b.goos` and only its non-Windows body had been ported. A PE built from the old
`print_int` would have executed a Linux `syscall` instruction on Windows and
faulted on the first digit -- which is exactly why every earlier PE gate
deliberately built images that do NOT print.

### 22.2 The IAT had to become index-tagged before this could work

`natStoreAbs64Idx` and the `natStAbsPos` / `natStAbsIdx` / `natStIatPos` /
`natStIatIdx` lists exist because kcc recovered each IAT reference's index from
EMISSION ORDER, which is correct only while the bootstrap's three stores are the
only absolute sites. `print_int` contributes four more IAT loads, so a positional
reader silently pairs the wrong position with the wrong kernel32 entry point --
not a crash, an image that calls through the wrong API. The emitter state grows
from six slots to thirteen; `natNewEmitter` is the only constructor, so no
existing emission changes and no byte moves.

### 22.3 A REAL DEFECT this slice's execution evidence exposed

**This is the substantive finding of Step 9b, and it is recorded because the
structural layers could not see it.**

The first working composition printed `12345` and **omitted its newline**, exit
code 0. Root cause: `natPEProgram` passed an **empty** rodata patch list to
`natPELink`. `natPELink` does *two* things with that list -- it writes each
immediate (`roBase + offset`) **and** it adds a DIR64 relocation entry per site.
With the list empty, the two rodata sites were resolved outside the relocation
table and got **no DIR64 entry**. This image sets `DYNAMIC_BASE` and
`HIGH_ENTROPY_VA`, and the host **does** rebase it, so those two immediates kept
the preferred load address and pointed at unmapped memory.

The symptom is the worst available shape. `print_int` writes its digits from the
**stack** -- no absolute address, so unaffected -- and its newline from an
unmapped pointer. `WriteFile` then failed, wrote **zero** bytes and returned no
error, so the process exited 0.

**Why every structural layer passed.** The image was a valid PE, the rodata
addresses were exactly the ones `native_value.kark` computed, the IAT pairing was
right, and the relocation count for the *other* sites was right. All of them were
checked against the **file**, and the file is correct. This is the same class as
the Phase 150C defect already recorded (a write that fails silently and exits 0),
and for the same reason its guard was `if out != ""`.

**And why the first gate draft also passed it.** That draft compared
`strings.TrimSpace(stdout)` against `"12345"`. The trimmed string is still
`"12345"` when the newline is missing, so the defect survived a test that looked
like it covered execution. Trimming is precisely the wrong tool for a defect that
is a **missing byte**. The gate now compares **exact bytes**, and
`TestPhase151B9_RodataSitesHaveDir64Relocations` is the structural half that
*names* the cause.

**Mutation-verified.** Reproducing the defect -- dropping the rodata patch list
again -- fails exactly two layers, `KccProducedPEExecutes` and
`RodataSitesHaveDir64Relocations`, and leaves the other seven green. That is the
honest statement of what the structural layers can and cannot see. Two further
mutations were verified in the same slice: making `natIatCall` record a constant
index instead of the one it was given fails both the index check **and** live
execution (the image calls the wrong kernel32 entry point); and hard-coding the
text length instead of measuring it fails the rodata address layer.
### 22.4 Evidence

| Layer | Result |
|---|---|
| `ImageIsValidPEAndCarriesAllThreePieces` | PASS -- `ParsePE` (the ORACLE's reader), entry at `.text` start, all three pieces by byte signature |
| `RodataPlacementAndAddresses` | PASS -- section bytes, placement, addresses derived from the MEASURED code length |
| `IATPatchCountsAndIndexCorrectness` | PASS -- `sites=2 ap=3 ip=7`, all three indices resolved and distinct |
| `IATSlotAddressIsInImage` | PASS -- slot base cross-checked against the image's own import directory |
| `MainCallsPrintIntAcrossPieces` | PASS -- the cross-piece `call rel32` lands on `print_int`'s HEAD |
| `KccProducedPEExecutes` | PASS -- stdout is exactly `31 32 33 34 35 0a`, exit 0 |
| `RodataSitesHaveDir64Relocations` | PASS -- 12+ DIR64 entries, both rodata sites named individually |
| `CorpusIsNonVacuousAndDeterministic` | PASS |
| `NoGoFallback` | PASS |

All nine tests are named `TestPhase151B9_*`. The letter is `9B` rather than `9b`
on purpose: `9b` would sit inside the increment-151B encoder family's pattern
namespace (`TestPhase151B_`) and read as a duplicate of it.

Oracle comparison, measured with no shell in the path: the Go oracle's own image
for `func main() { let x = 12345; print(x) }` and kcc's image both emit
`31 32 33 34 35 0a` and exit 0. The images are **not** byte-identical and are
not expected to be -- the oracle's `main` is a real lowered function with a frame
and kcc's is a fixed body -- so **stdout is the comparison, not the bytes**.

Frozen regressions green: `TestPhase151A5_` (except the pin, below), `151A7_`,
`151A9a_`, all `151A8*`, `151B_`, `151C*`. `go build ./...` and `go vet` clean,
gofmt clean (checked LF-normalised and BOM-free).

### 22.5 Wrong expectations in this slice's own gate, recorded

Four, all mine, all caught by measuring:

1. **The image's `.text` is NOT the code arm plus rodata.** The first draft
   asserted that and "found" a discrepancy in code that is right on both sides:
   `natPELink` resolves the bootstrap's module-walk offsets and every IAT slot,
   which the code arm correctly leaves as zeros. Both buffers are now asserted,
   each in its own right.
2. **The IAT does not start at `.idata`+16.** It starts at `.idata`+72: a 20-byte
   import directory entry and three 8-byte ILT entries precede it. Assuming 16
   made every slot address wrong by 56 bytes and reported "no IAT patch resolved"
   for a correct image -- a failure that reads as a compiler defect and is not one.
3. **`ParsePE`'s second return is a FILE offset (0x200), not the RVA (0x1000).**
   Asserting the RVA failed against a correct image.
4. **A DIR64 entry names the byte offset of the IMMEDIATE**, two bytes past the
   instruction start, not of the instruction. Checking the instruction offset
   reported both rodata sites as unrelocated against an image that had relocated
   them; the coarse count assertion is what made that discrepancy visible rather
   than silent.

Two more were found in the implementation, both gate-caught. `natResolveRodata`
returns a **count**, and assigning it to `s` replaced the emitter state with an
integer -- silently producing an image with no resolved rodata and no IAT patches
that still linked and still had the right length. That is precisely why the
corpus's arm 3 is a count report rather than hex. And `natIatCall` had to keep
appending to slot 5 after switching from `natMovImm64` to `natImm64Patch`, because
the frozen Step 6 image builders read their ipatch position out of slot 5 by
index.

### 22.6 KIR pin: STALE and RED, deliberately

**The whole-tree KIR pin is NOT re-measured, and is now known to be stale.** The
pin's only measurement path is `karkain kir --verify src/compiler/kir.kark`,
which is the whole-tree workload the 4 GB host rule forbids (measured this
increment at ~2.5 GB working set and ~11.2 GB private bytes -- see the memory
baseline recorded for this host). `TestPhase151A5_KIRPinHolds` therefore **FAILS
on this host**, and its own diagnostic distinguishes the two causes: it reports
the crash (`0xC0000005` on all four retries), explicitly *not* a pin mismatch.

The pin was last measured at **12570** (Step 9a). Step 9b adds lines to
`native_value.kark` and `native_emit.kark`, so the true value has moved.
**It is deliberately NOT predicted here.** Writing a predicted number into
`phase122_pipeline_ownership_test.go` and `phase151a5_win_test.go` would turn a
known-red gate into a green one that asserts nothing, which is the exact failure
the evidence-discipline rule in `AGENTS.md` exists to prevent. Both assertions
therefore still read **12570**, they now FAIL, and that is the honest state: they
are *stale-until-measured*, not *wrong-and-passing*.

Re-measuring requires either a host with enough commit headroom (the Linux CI
runner) or the memory-defect work, which is separate authorised work and is not
started.

### 22.7 Still open after 9b

* The whole-program driver: `natPEProgram` is a FIXED program compiled into kcc,
  not a driver that walks a real function's locals and dispatches on their kinds.
  Section 8 of the 151A scope is what makes kcc own native targets, and 9c is
  where it starts.
* Every remaining native value-model item: floats (done through Step 8c), arrays,
  `for-in`, `len()`, arena/`alloc`, string concat, string slice/compare, `push`,
  records, maps, macOS entry/exit.
* `kccOwnsNativeTargets = false`, and the seam's guard arm stays a build-time
  guard.

---

## 23. Step 9c - the whole-program driver (COMPLETE for the AST transition)

**Verdict: COMPLETE for the architectural transition. NOT ownership.**
`kccOwnsNativeTargets` stays `false`, and 151A stays open.

### 23.1 The transition, stated as what changed

Step 9b's program was a fixed shape compiled into kcc:

```
_start + karkain_main(mov edi, 12345; call print_int) + print_int
```

That proved the machinery -- one emitter state, resolved rodata, index-tagged IAT,
a linked image that executes -- while kcc still could not compile a **user's**
program natively. Step 9c replaces the fixed body with a driver that walks the
**real AST** `buildNativeFile` already parsed and type-checked.

What makes it a driver rather than a second corpus:

| Property | How it is enforced |
|---|---|
| `main` is found by NAME in the program's real top-level statements | `natNativeFindMain` scans for `FuncDecl` with name `"main"`; there is no corpus and the driver never learns which file it was given |
| the frame is DERIVED from the program's own bindings | `natFrameLayout(nLocals * 8, 0, 0, 0, 0, 1)` -- one and two locals produce different frames |
| a local's slot is its DECLARATION ORDER | `natLocalOff(idx, ...)` where `idx` counts bindings; `natNativeLocalIndex` resolves a name to it |
| `print(x)` and `print(7)` are different lowerings | one emits `mov rdi, imm`; the other emits `mov rdi, [rsp+off]` |

A new `native-ast` kcc subcommand drives it from a file. That is the first native
measurement entry point that takes an INPUT file, and it is what makes the
AST-to-emitter connection testable at all.

**Boundary, stated rather than glossed.** kcc computes, resolves and links every
byte; the CLI decodes the byte stream it is handed and writes it to disk. The
language has **no byte-level file writer** -- `writeFile` and `writeToFile` both
`fputs` a *string* and would truncate the image at its first NUL -- so the bytes
travel as hex. Go performs no lowering, no linking and no codegen here, and cannot
substitute for kcc: if kcc refused, no bytes would exist to write.

### 23.2 Evidence

`pkg/cli/phase151a9c_driver_test.go`, `TestPhase151A9C_*`, 3 tests / 10 corpus
cases, all PASS. Every case writes a different `.kark` file.

| Case | Source | Exact stdout | Exit |
|---|---|---|---|
| `literal_12345` | `print(12345)` | `31 32 33 34 35 0a` | 0 |
| `literal_42` | `print(42)` | `34 32 0a` | 0 |
| `local_12345` | `let x = 12345; print(x)` | `31 32 33 34 35 0a` | 0 |
| `local_42` | `let y = 42; print(y)` | `34 32 0a` | 0 |
| `two_locals` | `let a = 7; let b = 35; print(a); print(b)` | `37 0a 33 35 0a` | 0 |
| `return_sets_exit_code` | `let c = 3; return c` | *(empty)* | **3** |
| `refuse_while` | `while` loop | refusal naming the construct | -- |
| `refuse_binary_expression` | `print(1 + 2)` | refusal naming the construct | -- |
| `refuse_non_integer_binding` | `let s = "hi"` | refusal naming the construct | -- |
| `refuse_no_main` | no `func main` | refusal naming the construct | -- |

`return_sets_exit_code` is the case that matters for a reason worth stating: it
prints **nothing**, so a stdout-only assertion could not see it at all. The exit
code is the only observable, and it is the one that caught a real defect (23.4).

`TestPhase151A9C_SourceChangesTheGeneratedImage` compares the two single-literal
programs' **images**. They differ in **exactly 2 bytes, both inside `.text`**
(offsets 1502-1503), which is the 32-bit immediate of the `mov r32, imm32` that
carries the literal. A driver that emitted the right literal but perturbed
anything else would still fail.

### 23.3 Mutation verification

Two mutations, each caught by exactly the layers that should catch it and no
others:

* **M1 -- the AST is ignored for a literal argument** (`mov rdi, 12345` hard-coded):
  fails `literal_42` and `SourceChangesTheGeneratedImage`. It correctly LEAVES
  `literal_12345` green, because that case coincides with the hard-coded value --
  which is why a single passing case would never have been evidence.
* **M2 -- declaration order ignored** (every local resolves to slot 0): fails
  `two_locals` only. It correctly leaves `local_42` green, because that program's
  only local *is* slot 0.

That precision is the point: each mutation is caught by the case written to detect
that specific way of being wrong, and survives the cases where the mutation is
invisible.

### 23.4 Three real defects this slice's own evidence found

1. **`funcBody` is the statement LIST, not a Block node.** `parseBlock` returns
   its statements directly. The first draft assumed a Block, fell through to a
   single-element fallback, and handed the scanner a list where it expected a
   statement -- every element read back with an empty node type and the refusal
   said "statement kind [array]". The checker's own consumer is the authority:
   `checkStmts(tab, funcBody(node), 0)`.
2. **`let` and `var` are DIFFERENT node kinds.** `parseVarDecl` returns
   `makeLetDecl` for `let` (NODE_LET_DECL) and `makeVarDecl` for `var`
   (NODE_VAR_DECL); the checker tests for both. Matching only `NODE_VAR_DECL`
   meant `let x = 1` was not recognised as a binding at all.
3. **Integer literals are stored as SOURCE TEXT in both engines** (`IntLiteral.Value
   string`), so the driver must coerce with `int(intLitVal(...))`. A first patch
   used `int(pv)` -- passing the *node* -- which silently emitted `mov edi, 0` for
   every program.

A fourth defect is the instructive one:

4. **The Win64 alignment `sub rsp, 8` invalidated every frame-relative load that
   followed it.** `natLoadStack` addresses `[rsp + off]`, i.e. the *current* rsp,
   so emitting `sub rsp, 8; mov (%rsp), %rdi` read 8 bytes below the first local.
   The symptom was not a crash: with two locals the program printed `0` then `7`,
   because the second `mov 8(%rsp)` happened to land back on slot 0. **The
   literal-only case passed throughout**, because it loads an immediate rather
   than a slot -- so a single passing case would have been taken as evidence that
   the path was right. The fix is to load the argument BEFORE the alignment
   adjustment; nothing between the load and the `call` touches RDI.

And a fifth, found by the exit-code case: an unconditional `mov rax, 0` emitted
after the body **clobbered an explicit `return`**, so `let z = 7; return z` exited
0. The program printed nothing, so only the exit code exposed it. The implicit
`return 0` is now emitted only when the scan found no `return`.

### 23.5 Honest limitations

* The supported subset is `let` of an integer literal, `print` of an integer
  literal or local, and `return` of an integer literal or local. Everything else
  is refused **by name** with K145. That refusal table is the honest state of 151A
  after this slice: kcc compiles a small, growing subset natively and refuses the
  rest explicitly rather than falling back.
* The image travels as hex and is written by the CLI (23.1).
* **`kccOwnsNativeTargets` is still `false`.** This slice is not the ownership
  flip; it is the capability the flip will be justified by. Wiring the constant is
  151D, together with the remaining value-model kinds.
* Not claimed: parity with the Go engine for these constructs, or any of the
  remaining kinds.

### 23.6 KIR pin

Unchanged from 22.6: **stale and red, deliberately**. The pin was last measured at
12570 (Step 9a); Step 9b and 9c both move it, and it is **not predicted**.

---

## 24. Step 9d - integer arrays and indexing (IMPLEMENTED)

Step 9c proved the SOURCE reaches the emitter. 9d adds the first COLLECTION kind to
that driver, so it is the first step where a local is not one 8-byte unit and the
frame is no longer a flat list of scalar slots.

**This is NOT whole-image parity and NOT ownership.** `kccOwnsNativeTargets` stays
`false`; `build --target native-*` still refuses with K116 and writes no image.

### 24.1 Scope

    let a = [7, 35]        // integer array literal
    print(a[0])            // index expression, integer literal index

Nothing else. `len`, `for-in`, `push`, non-literal indices, negative elements, and
printing an array as a whole are all still refused by name.

### 24.2 The frame contract, reproduced from the oracle

An array local occupies a **two-unit header plus 8 bytes per element**, entirely in
the frame, and **nothing is allocated**:

    [off]            = address of the element area
    [off + 8]        = element count
    [off + 16 + i*8] = element i

Three consequences the implementation had to respect:

* The frame is **no longer a fixed stride**, so `natLocalOff`'s `8 * index * units`
  cannot express it. `natNativePlan` allocates **sequentially**, one
  `[offset, kind, elemCount]` triple per local in declaration order, which is what the
  oracle's own `scanLets` does.
* `rsp` never moves, and the element base is **never** placed in `rsp`: it is computed
  once with `lea` into the frame and reloaded from the frame before each element store.
  That reload is what keeps the sequence correct for the non-literal elements a later
  slice will allow -- a base held in `RBX` across an arbitrary element expression could
  have been clobbered.
* Every element access is a constant-displacement SIB through a register, which is why
  the two new encoder primitives exist.
### 24.3 New encoder primitives (`native_emit.kark`)

`natSibTail`, `natLoadScaled64`, `natStoreScaled64`, `natScaledRex`, plus
`natMask(72) = 0x0B` (the low-three-bits mask used to pack the SIB byte).

Three encoding decisions are load-bearing, and each was measured rather than assumed:

1. **The mod=00 special case.** With `disp == 0` the displacement is omitted
   entirely -- *except* when the base is RBP, which in mod=00 means RIP-relative
   rather than `[rbp]`. Omitting it for an RBP base silently rebases the access.
2. **REX is built locally, not through `natRex`.** A scaled form assembles R, X and B
   from **three different operands** (data register, SIB index, base), whereas
   `natRex` derives R from `rm` and B from `rm`. Reusing `natRex` would put the wrong
   field in the wrong bit.
3. **The selector table was appended at 72, never renumbered.** `natMask` is a shared
   namespace and two earlier blocks in this file started inside the existing range
   (Step 1 at 17) and silently changed the bytes an unrelated primitive emitted.

### 24.4 A language trap, found immediately

The first draft declared `let short = 1` as the addressing-mode selector. **`short`
is a reserved word** in Karkain (the C type), so the generated C read
`Value short = make_int(1);` and gcc rejected it with *"two or more data types in
declaration specifiers"* -- a diagnostic that points at the wrong language entirely.
Same class as Step 8b's `raw` and `addr`. Renamed to `mode`.

### 24.5 Two REAL defects, both found by the gate's refusal table

Both produced a **valid PE** -- no diagnostic, no trap, a wrong number. That is the
worst possible outcome for a native backend, and it is why the refusal table exists.

1. **Indexing a scalar was silently lowered.** `let a = 7; print(a[0])` scanned
   clean: the target was an identifier, it resolved to a local, and the index was an
   integer literal. Lowering then treated the scalar's 8-byte slot as a two-unit
   `(base, len)` header, read a "length" out of the neighbouring slot, and read an
   element from whatever address that base happened to be. Fixed by a kind check in
   `natNativeScanIndex`, which now takes the plan.
2. **Printing an array as a whole was silently lowered.** `print(a)` loaded the
   header's base pointer -- one 8-byte slot -- and printed that address as if it were
   the array's value. There is no scalar reading of a collection, so there is nothing
   to lower it to; the scan now refuses it by name.
### 24.6 Two wrong expectations in the GATE, recorded because the method warns about it

1. **The image-distinctness layer used a fingerprint.** Its first version keyed a map
   on an FNV-style hash and reported five different images as identical. The layer now
   compares images as **exact bytes**, with the same `nat9cBytesEq` helper the rest of
   the gate uses. A fingerprint that can collide is a weaker claim than the layer exists
   to make -- and a colliding "identical" verdict is exactly the failure this repository
   already paid for in increment 151B.
2. **The frame test demanded a STRICT increase per element.** Measurement showed the
   locals region is rounded **up to 16 bytes**, so a 1-element array (24 bytes) and a
   2-element array (32 bytes) legitimately share a frame. The test now asserts the
   property that actually matters, which is a **soundness** one:

       locals region >= 16 (header) + 8*n (elements)

   An element-blind frame (a flat 8-bytes-per-local stride) gives a locals region of 16
   for every `n` and fails at `n = 1`.

### 24.7 MEASURED PARITY DELTA - the oracle's frame alignment is NOT reproduced

Stated rather than hidden, because it is a real difference from the reference.

| N elements | localBytes | kcc frame | Go oracle frame |
| --- | --- | --- | --- |
| 1 | 24 | 640 | 640 |
| 2 | 32 | **640** | **656** |
| 3 | 40 | 656 | 656 |
| 4 | 48 | **656** | **672** |
| 5 | 56 | 672 | 672 |

kcc emits `round16(localBytes) + 512 + 96`, which is what `natFrameLayout` implements
and what Step 1 established. The oracle's `scanLets` computes the **same**
`localBytes` (8 + 8 + 8*N -- read directly from the source, not inferred), but its
frame is `32 + 16*floor(N/2) + 608`: it pads the locals region to the next 16
**strictly greater**, which adds 16 exactly when `localBytes` is already 16-aligned.

**Why this slice does not close it.** The difference lives in `natFrameLayout`, which
is **frozen by Step 1** and whose output every frame-relative displacement in Steps
1-9c is pinned against. Changing it would move all of them and re-open four completed
slices. Matching the oracle's alignment is whole-image-parity work, which is 151D's
bar, and it must be one deliberate change with its own differential -- not folded into
a value-model step.
### 24.8 Gate: `pkg/cli/phase151a9d_array_test.go`

`TestPhase151A9D_` -- 7 tests / 45 subtests, all PASS. Six layers:

1. **exact stdout bytes + exit code, by executing the PE.** Nothing is trimmed.
2. **element and index come from the source** -- pairwise **byte-distinct** images
   across five variants. This is the anti-hard-coding layer: a stdout comparison can be
   satisfied by choosing the right expected value once, whereas a driver emitting a
   constant produces the *same image* for all five.
3. **frame follows the value model's rule**, with the expected number derived in the
   test from the documented rule rather than recomputed the way the implementation does.
4. **elements need room** -- the soundness property above, over N = 1..5.
5. **out-of-subset shapes refused by name**, each naming *which* construct, and
   asserting a refusal did not also emit an image.
6. **no Go fallback** -- a misspelled native subcommand must fail, and
   `build --target native-x86_64-windows` must still refuse with K116, exit 6, and write
   no image.

One refusal case is caught **earlier** than the driver's scan and therefore expects a
different code: `print(q[0])` for an undeclared `q` is reported by kcc's own type
checker as `error[K102] undefined identifier 'q'` before `natNativeScan` runs. The
gate asserts K102 there, and asserts the "not handed to the Go engine" clause only for
the driver's own K145 refusals -- a program rejected by the checker never reached
native lowering at all, which is a stronger outcome than refusing it there.

### 24.9 Mutation verification

| # | Mutation | Result |
| --- | --- | --- |
| M1 | element store hard-codes `7` instead of reading the literal | `array_index_0` **PASSES** (coincidence: `[7,35][0]` really is 7); `array_index_1`, `array_index_2_of_3`, `array_and_scalar_local`, `two_indexes_one_array`, `multi_digit_elements`, `single_element_array` **FAIL**; the image-distinctness layer fails at `idx0_of_8_35`, because `[7,35]` and `[8,35]` become the same bytes. Frame and refusal layers correctly stay green. |
| M2 | index load hard-codes `0` instead of reading the index expression | `array_index_0` and `single_element_array` **PASS** (both really do index 0); `array_index_1`, `array_index_2_of_3`, `array_and_scalar_local`, `two_indexes_one_array`, `multi_digit_elements` **FAIL**; the image-distinctness layer fails at `idx1_of_7_35`. |

Both mutations were reverted and `native_value.kark` verified **SHA-256 identical** to
its pre-mutation content (`ED989A343CECE33524A01FFD57D222C7060DB371...`). The 9d gate
was then re-run green, and the 9b + 9c gates re-run green unchanged.

Each mutation keeps the *coincidentally equal* case green, which is the point: a
mutation that turned every case red would prove only that the gate runs, not that it
distinguishes reading the AST from hard-coding it.
### 24.10 KIR pin

**Stale and red, deliberately** -- unchanged in policy from 22.6 and 23.6. The pin was
last measured at **12570** (Step 9a). Step 9b, Step 9c and Step 9d all move it, and it
is **not predicted**: a predicted number would turn a known-red gate into a green one
asserting nothing, which is the exact failure the evidence-discipline rule exists to
prevent. Both assertions still read 12570 and they now fail, which is the honest
stale-until-measured state.

The pin's only measurement path is the whole-tree `karkain kir --verify`, which the
~4 GB host rule forbids. Re-measuring needs the Linux CI runner or the separate
low-memory defect work.

### 24.11 A HARNESS trap worth recording, because it looked like a compiler defect

While measuring, `karkain native-ast` once emitted **3727 diagnostics** -- mass
`K107 duplicate function definition` and `K101 undefined function` -- and no image.

The first hypothesis was the new code. It was wrong, and the measurement that killed
it was `git stash` + retry on **pristine HEAD**, which reproduced it identically.

The real cause: `assembleProject` adopts every sibling `.kark` file in the target's
directory that does **not** declare `func main(`. The scratch directory in use held
several large `main`-less `.kark` blobs left by an earlier session (117 KB, 140 KB).
They were assembled in as siblings, so the driver type-checked the *compiler's own*
sources and reported them. With a clean directory, pristine HEAD emitted a valid PE
(`4d5a` = `MZ`).

Two things follow:

* A mass duplicate/undefined cascade from `native-ast` is **not** evidence of a
  compiler defect until the scratch directory has been cleared. Check it first.
* Each gate case writes its program into its own `t.TempDir()`, which is why the 9c
  gate was never affected. That isolation is load-bearing and should be preserved.

### 24.12 What this step does NOT claim

* **No whole-image parity.** kcc's `main` is still a synthetic lowered body, while the
  oracle's is a real function; their images are not and should not be byte-identical.
  Stdout is the comparison, not the bytes. 24.7 records the one frame difference found.
* **No ownership.** `kccOwnsNativeTargets = false`.
* **No other kind.** strings + arena, `len`, `for-in`, `push`, records, maps and the
  macOS entry/exit are all still open 151A work.
* **No runtime diagnostic for an out-of-range index.** The bound IS checked and traps
  with `Int3`; the oracle raises `karkain_runtime_error` with a source location, which
  is increment 152-B0 machinery that kcc does not have yet.
* **No CI result at the time of writing.** The companion CI step
  (*Run 151A Step 9d integer array gate*, pattern `TestPhase151A9D_`, verified not to
  collide with `TestPhase151A9C_`, `TestPhase151A9a_`, `TestPhase151B9_`,
  `TestPhase151A_` or `TestPhase151C_`) is wired in this change; its result is recorded
  in the increment record once the run completes.
---

## 25. Step 9e - `len()` on arrays (IMPLEMENTED)

Step 9d added the array kind. 9e adds the first **builtin** to reach the driver, and it
is deliberately a small slice: the oracle's array branch of `emitLen` is a single
instruction (`e.LoadStack(RAX, off+8)`), so it can be proven end-to-end with almost no
new emission while establishing the shape every later builtin follows.

**Not whole-image parity, not ownership.** `kccOwnsNativeTargets` stays `false`.

### 25.1 The count offset is written literally, on purpose

The array header is two units, and they are not adjacent to what they look like:

    [off]     = element-area base
    [off + 8] = COUNT          <-- what len() returns
    [off+16+i*8] = element i   <-- the element AREA starts here

`natArrayLen` therefore writes `off + 8` literally and is **deliberately not derived
from `natLocalOff(ord, natKindArray())`**. The slot formula places an array's area at
`off + 16`, so deriving the count from it would read the element area instead. The
gate's `len_of_1` case exists for exactly this: with one element the two are 8 bytes
apart, and the mistake produces a plausible-looking pointer-sized number rather than an
obvious failure.

### 25.2 One validator for both value positions

`print` and `return` now share a single `natNativeScanValue`. This was not cosmetic:
9d's refusal table had already caught the two paths diverging (print accepted a shape
return did not). One function means the accepted and the refused sets cannot drift.
`TestPhase151A9E_PrintAndReturnAgree` walks the accepted shapes and asserts both
positions give the same verdict, which is a property of the driver rather than of any
single program -- two copies of the logic would drift only for a shape a given corpus
happens not to use.

### 25.3 One existing pin removed, and why

9d pinned `print(len(a))` as **refused**, since `len` was out of subset then. 9e makes
it supported, so that case was **deleted** from 9d's refusal table rather than left
asserting a refusal that is no longer honest. Its behaviour is now gated properly by
`TestPhase151A9E_*`, including the shape distinctions (len of a scalar, len of an
expression, wrong arity). Leaving the old pin would have made 9d fail on correct
behaviour and taught its readers to ignore it.

### 25.4 Two wrong expectations in the GATE

1. **`localBytes` left at its zero value** for `len_as_return_value`, so the frame layer
   expected 608 (no locals) while the program has a 32-byte array. The test failed and
   the implementation was right.
2. **Wrong arity diagnostic.** The gate expected the driver's own
   `len() takes exactly 1 argument`, but kcc's builtin-arity check fires first:
   `error[K104] line 3: builtin 'len' expects 1 arguments but got 2`. Same class as 9d's
   `print(q[0])` being caught earlier by K102. Asserting a diagnostic that can never
   fire teaches a reader to distrust a gate, so the expectation now names K104.

A refusal-text nit was also fixed: the driver described its subset as "Step 9d ... and
`return` of an integer literal or local", which stopped being true once `len` and
`a[i]` in return position were accepted. It now says "Step 9e" and lists both.

### 25.5 Gate: `pkg/cli/phase151a9e_len_test.go`

`TestPhase151A9E_` -- 6 tests, all PASS, with exact-bytes execution over 7 programs:

| case | stdout | note |
| --- | --- | --- |
| `len_of_2` | `320a` (`2\n`) | the basic claim |
| `len_of_3` | `330a` (`3\n`) | |
| `len_of_1` | `310a` (`1\n`) | discriminates COUNT from the element AREA |
| `len_then_element` | `330a39390a` (`3\n99\n`) | both header fields addressable |
| `len_with_scalar_after` | `320a` (`2\n`) | reads the ARRAY's header, not the scalar's slot |
| `len_as_return_value` | *(empty)*, **exit 2** | observable ONLY through the exit code |
| `len_is_a_64_bit_header_field` | `330a` (`3\n`) | the count is stored with a 64-bit `movabs` |

The exit-code case is why the gate asserts exit codes at all: `return len(a)` prints
nothing, so a stdout-only assertion could not see it.

Other layers: frame per case, three programs with the **same first element (42)** and
different counts producing distinct images, a 5-case refusal table naming each
construct, determinism/non-vacuity, and print/return agreement.

### 25.6 Mutation verification

| # | Mutation | Result |
| --- | --- | --- |
| M3 | `natArrayLen` reads `off+16` (the element AREA) instead of `off+8` (the COUNT) | **all 7 execution cases FAIL** -- `len_of_2/3/1` print the element value instead of the count, and `len_as_return_value` exits with the element value. The frame, refusal, determinism and print/return-agreement layers correctly stayed **green**: the mutation changes only which header unit is loaded, and only execution can see that. |

Reverted; `native_value.kark` verified **SHA-256 identical** to its pre-mutation content
(`F8FFF0FFBC09ED5B9A59AF0441F43FEF...`); the 9b + 9c + 9d + 9e gates then re-ran green
together (49.1s).

### 25.7 KIR pin

**Stale and red, deliberately** -- unchanged in policy from 22.6/23.6/24.10. Last
measured 12570 (Step 9a); 9b, 9c, 9d and 9e all move it, and it is **not predicted**.
Both assertions still read 12570 and now fail, which is the honest stale-until-measured
state. Re-measuring needs the Linux CI runner or the separate low-memory defect work.

### 25.8 What this step does NOT claim

* No `for-in`, no `push`, no strings/arena, no records, no maps, no macOS entry/exit.
* `len` of a **map** needs one extra indirection (a map's address lives in its slot);
  maps are not lowered by this driver, so that difference is unreachable here.
* No CI result at the time of writing: the step *Run 151A Step 9e len() gate* (pattern
  `TestPhase151A9E_`, verified not to collide with `TestPhase151A9D_`) is wired in this
  change. See the increment record for the observed CI status.
---

## 26. Step 9f - string literals and printing a string (IMPLEMENTED)

The first non-scalar kind that is **not** a frame-resident collection. 9d's array lives
entirely in the frame; a string's value is a (pointer, length) pair whose bytes live in
`.rodata`. That makes this slice different in kind from 9d/9e: the literal's **offset**,
not its content, is the load-bearing number.

**Not whole-image parity, not ownership.** `kccOwnsNativeTargets` stays `false`.

### 26.1 The value convention and the helper

`emitStr` leaves `(RDI, RSI) = (ptr, len)`, and `emitPrint` dispatches on
`isStringExpr` before any kind check, so a string never reaches `print_int`. The
oracle's `print_str` is **seven instructions**, because a string needs no formatting:

    Mark("print_str"); MovRegReg(RDX, RSI); MovRegReg(RSI, RDI);
    MovRegImm32(RDI, 1); emitWrite(); printNewline(); Ret()

`natPrintStrBody` reproduces exactly that, reusing kcc's existing `natPayloadWrite` and
`natPrintNewlineOff` rather than duplicating a write path. The three moves are the
(RDI=ptr, RSI=len) convention reshuffled into (RDI=fd, RSI=ptr, RDX=len), which is what
both the oracle's `emitWrite` and kcc's `natPayloadWrite` consume.

**The arena is deliberately absent.** Concatenation must *build* a string and so needs
the bump allocator; a literal is already built. This slice therefore needs no arena and
performs no allocation at all -- which is also why it adds no heap to the image.

### 26.2 Two real defects, one of them a crash

1. **`print("hi")` with no binding CRASHED kcc.** The literal collector walked only `let`
   initialisers, so a directly-printed literal recorded a reference to an offset that had
   never been interned, and `natNativeLower` died with *"array index out of range"* inside
   `natRodataLookup`. Fixed by collecting from both places a literal can appear, in the
   order the lowering records them -- one walk, so the two orders agree by construction.
2. **`return s` for a string silently emitted an image** and exited with the string's
   POINTER as the exit status. `natNativeScanValue` accepts a string (it is printable as
   itself), and `return` reused that shared validator, so nothing stopped the return path
   treating a two-unit string as an int. The oracle refuses this outright. Fixed by
   `natNativeScanReturn`, which is the shared validator plus one rule: a return value
   becomes the process exit status and must be an int.

### 26.3 `++` does not exist in Karkain

The first draft built the rodata list as `["-", "\n"] ++ natNativeStrLiterals(body)`.
There is no `++` operator, and the expression compiled into an image that printed **a run
of NUL bytes** -- no diagnostic, a valid PE, wrong output. The list is now built with an
explicit `appendArray` loop.

That symptom is worth naming precisely because it is so quiet: every string reference was
recorded against a list that was not the list the linker interned, so each one resolved to
bytes that were never there. A layer that compared images pairwise would have called the
images "identical" and the run of NULs would have had to be caught by reading stdout.

### 26.4 Offsets come from `natRodataSection` itself

`natRodataSection` returns `[bytes, offs]` -- one offset per **input** entry, with a
repeated literal resolving to the slot it was first interned into. The driver therefore
computes the section **before** emission (the body needs those offsets to record
references) and walks a single `sidx` in lockstep with the literals, starting at 2
because entries 0 and 1 are print_int's own `"-"` and `"\n"`.

Recomputing an offset from a literal's *value* instead was the rejected alternative: it
breaks the moment two bindings share one string, because the second reference would look
for a slot the section never interned.

### 26.5 `len(s)` came for free

A string's second unit IS its length, at exactly the offset an array's COUNT occupies, so
the same single `LoadStack(RAX, off+8)` returns both. It is accepted deliberately rather
than refused, because refusing would have been an arbitrary restriction instead of a real
boundary. `len_of_string` pins `"karkain"` -> `7`.

### 26.6 Gate: `pkg/cli/phase151a9f_string_test.go`

`TestPhase151A9F_` -- 8 tests, all PASS, exact-bytes execution over 8 programs:

| case | stdout |
| --- | --- |
| `print_literal` (`print("hi")`, no binding) | `68690a` |
| `print_local` | `68690a` |
| `long_literal` | `68656c6c6f206b61726b61696e0a` |
| `two_distinct_literals` | `6f6e650a74776f0a` |
| `repeated_literal_interned_once` | `73616d650a73616d650a` |
| `string_and_int` | `6e3d34320a34320a` |
| `string_and_array` | `6974656d733a0a33350a` |
| `len_of_string` | `6b61726b61696e0a370a` |

`string_and_array` is the sharpest layout case in the slice: a string's length unit and
an array's COUNT both sit at `slot+8`, so getting the string's size wrong makes one of
the two prints wrong.

Other layers: six string variants producing **pairwise byte-distinct images** (a driver
resolving every string reference to offset 0, where print_int's `"-"` lives, would print
a plausible byte for every program); the frame per case with the two-unit rule stated;
**literal bytes searched for in the image**; interned-once; a 3-case refusal table;
determinism/non-vacuity.

### 26.7 Two wrong expectations in the GATE

1. **The interning layer measured the wrong thing.** Its first version asserted that
   lengthening a literal lengthened the image, and it failed with a delta of **0**. The
   cause was the measurement, not the compiler: a PE pads `.text` up to the 512-byte
   `FileAlignment`, so a 4-byte growth in rodata need not change the file size at all. The
   layer now searches the image for the literal's ASCII bytes, which cannot be satisfied
   vacuously because every image begins with print_int's own `"-"`.
2. A first draft of the interning test was left half-written with three undefined helpers
   and dead statements before it ever ran. Rewritten rather than completed, because a test
   whose helpers do not exist cannot be reasoned about.

### 26.8 Mutation verification

| # | Mutation | Result |
| --- | --- | --- |
| M4 | a bound string's length is hard-coded to `1` | **all 7 bound-string cases FAIL** (`print_literal` correctly stays **green** -- it never takes the binding path, so its length comes from the literal in the print arm). The frame, image-distinctness, interning and refusal layers correctly stayed green. |

Reverted; `native_value.kark` verified SHA-256 identical to pre-mutation content
(`C490F79C0BB702B76999CAA1542CB0B2...`).

### 26.9 Two earlier pins moved, and were corrected rather than left failing

Both are the same class as 24.3's `len_of_array`: a construct that moved from **refused**
to **supported**, or a diagnostic that was reworded.

* **9c's `refuse_non_integer_binding`** pinned `let s = "hi"; print(s)` as REFUSED, which
  was true when 9c landed and int-only. That program now compiles and prints `hi`. The
  case was **replaced** with `let b = true` rather than deleted, because the table's
  intent is "a binding initialised from something this driver cannot lower is refused by
  name", and a boolean preserves that intent while staying refused. The string program is
  now gated by `TestPhase151A9F_*`.
* **9e's `len_of_expression`** asserted the exact phrase `len() requires an array
  variable`, which became `array or string` when this slice made `len(s)` legal. The
  expectation now names the new phrase. Asserting the exact wording is deliberate: it
  makes a future reword a visible test change instead of a silent drift.

### 26.10 What this step does NOT claim

* **No concatenation**, so **no arena** in kcc at the time of this step. This was true
  when 9f landed and is **superseded by section 27**: 9g adds concatenation, the bump
  arena and the `alloc` helper. Slicing, comparison, string parameters and string returns
  are still refused by name.
* No `for-in`, `push`, records, maps, macOS entry/exit.
* `print_str` is emitted **only when the program can produce a string** (`natNativeUsesString`).
  `print_int` is still emitted unconditionally, so the honest byte-identity comparison for
  a string-free program is against today's behaviour rather than an ideal.
* **KIR pin**: stale and red, deliberately, per 22.6/23.6/24.10/25.7. Last measured 12570
  (Step 9a); 9b-9f all move it, and it is **not predicted**.
* **No CI result at the time of writing.** See the increment record.

## 27. Step 9g - string concatenation and the arena allocator (GATED)

### 27.1 What 9g adds

9g is the first slice whose result is **built** rather than referenced. Everything before
it is frame-resident: an int is one unit, an array's header and elements live in the
frame, and a string literal's *value* is a `(pointer, length)` pair pointing into
`.rodata`. Concatenation has to write bytes somewhere the compiler cannot address at
build time, so 9g introduces:

* the **bump arena**, a compile-time-sized region living inside `.idata` (already R/W and
  loader-proven writable, so no fourth PE section is needed);
* the **16-byte arena header** `[cursor][limit]`, which lives *inside* the arena, so no
  globals and no writable image text are required. The cursor self-initialises on the
  first `alloc`; a stored cursor of 0 is the "not started" sentinel;
* the **`alloc` helper**, which nothing else in the compiler exercises;
* `strTemp`, a dedicated five-units-per-depth staging area. The int path's `binTemp`
  carries 8 bytes per depth and cannot hold the five units a concatenation needs.

A **heapless program is unchanged**: the arena, `alloc` and `strTemp` are emitted only
when the program actually concatenates. This is what keeps increment 150's byte-identity
pins honest, and it is why the 9b-1 corpus still reports `sites=2 ap=3 ip=7`.

### 27.2 Supported forms

| form | example | result |
| --- | --- | --- |
| literal + literal | `print("ab" + "cd")` | `abcd` |
| local + literal | `let a = "ab"; print(a + "cd")` | `abcd` |
| literal + local | `let a = "ab"; print("cd" + a)` | `cdab` |
| local + local | `let a = "ab"; let b = "cd"; print(a + b)` | `abcd` |
| empty operands | `print("" + "cd")`, `print("ab" + "")`, `print("" + "")` | `cd`, `ab`, `` |
| multiple sites | two or three concatenations in one `main` | concatenated per site |

Operand order is **observable in the output**, so `concat_lit_local` and
`concat_local_lit` have deliberately *different* expected bytes. The first draft of the
gate expected `abcd` for both, which would have made the reversed case indistinguishable
from the forward one and would have proved nothing about which side is which.

### 27.3 The arena bound is a proof

    size = sites * totalLiteralBytes + 16

1. Every concat **site** runs at most once. The driver does not lower loops at all, so a
   loop-contained concatenation is refused before emission rather than being sized.
2. Every string value in a supported program is a literal or a concatenation of literals,
   so no runtime string exceeds the sum of the program's literals.

Together those bound the sum of all allocations. Exhaustion is therefore unreachable for
a supported program and is kept anyway: `alloc` traps with `Int3` rather than writing
outside the arena.

The size is **read back out of the image**, not taken on trust. `alloc` holds the arena
base in `r10` and the arena end (`arena + header + size`) in `r11`, so the span between
the two `movabs` immediates *is* the computed size. The arena bytes in the file are all
zeros - the cursor self-initialises at run time - so a gate that read the header from the
image would compute `0 - 0` and prove nothing.

The gate's cases vary **both** terms independently: `sites` takes 1, 2 and 3 and
`totalLiteralBytes` takes 0, 4, 6, 8 and 15. An arena sized from either term alone, or
from a constant, produces the wrong span on at least one case, and
`TestPhase151A9G_CorpusIsNonVacuousAndDeterministic` fails outright if the case set ever
stops varying both.

### 27.4 Four real defects, all producing a structurally valid PE

The failure class that makes this slice worth a gate: each of these produced an image
that passed every structural layer and then did the wrong thing, because **the linker
does not check opcode semantics**.

1. **`natMask(72)` returned 11, not 7.** It is documented as "the low three bits of a
   register number" and 11 is `0b1011`, which is four bits. Step 9d survived it by luck -
   the scaled accesses there use `R9` and `RDX`, whose low bits happen to survive the
   wrong mask. 9g uses `R10`/`R11`, where `184 + (11 & 11) = 195 = 0xC3`, which is the
   **`ret` opcode**: the arena limit was loaded by returning from the middle of `alloc`.
2. **`natHeapRef` reused `natImm64Patch`**, whose `natMovImm64Op(rd) = 184 + rd` is valid
   only for `rax`..`rdi`. It now emits `REX.W|REX.B` (`0x49`) and `B8 + (rd & 7)`.
3. **Two literal collectors disagreed.** The one used for rodata interning and for the
   `print_str` gate did not descend through `BinaryExpr "+"`, while the arena-sizing path
   did, so for `print("ab" + "cd")` the first returned `[]`, `print_str` was never
   emitted while the concat arm still called it, and `natFinish` returned the *string*
   `"undefined label 'print_str'"` where code was expected. They are now one function.
4. **Two shapes reached emission unchecked** and crashed in `natNativeSlotOff` with
   `array index out of range`: a **chained** concat (`a + b + "ef"`), because
   `natNativeStrKind` *recurses* so the validator was satisfied and then
   `natEmitStrValue` called `identName` on a `BinaryExpr`; and `return a + "b"`, because
   the concat is a valid string expression while a string is not an exit status.

The rule 9g exists to enforce: **a shape that is not lowered must be refused by name,
never guessed at.** A crash that produces no diagnostic is worse than a refusal, and a
refusal that names the construct teaches the reader what is unsupported.

### 27.5 Gate: `pkg/cli/phase151a9g_concat_test.go`

`TestPhase151A9G_` - six tests, ten executed programs, five refusals:

* `ConcatsExecuteWithExactBytes` - compile, structurally validate, **execute**, compare
  exact stdout bytes. Nothing is trimmed: trimming is the wrong tool for a defect that is
  a *missing* byte, which is how 9b's first draft passed a program that printed `12345`
  with no newline.
* `ArenaSizeIsTheProofNotAGuess` - the span read out of the image against the rule.
* `AllocEncodesR10AndR11Correctly` - `49 BA` / `49 BB` present, `49 C3` (`ret`) absent.
  Scanned, not pinned to an offset: the emitter's layout is not a contract, and the scan
  fails outright if neither correct encoding is present, so it cannot pass vacuously.
* `UnsupportedShapesAreRefusedByName` - each refusal asserted three ways: kcc refuses,
  kcc does **not** crash (`array index out of range` is the bug, not a refusal), and the
  text is kcc's own `native target:` wording, which also proves it was not handed to the
  Go engine.
* `DifferentConcatProgramsProduceDifferentImages` - exact-byte pairwise distinctness.
* `CorpusIsNonVacuousAndDeterministic` - corpus floors, the both-terms-vary invariant,
  and byte-identical recompilation of every case.

Mutation-verified, all reverted with production SHAs confirmed:

| mutation | caught by |
| --- | --- |
| copy 0 bytes of the left operand | all ten execution cases fail on stdout |
| arena header term `+16` → `+8` | all five arena cases fail |
| `natMask(72)` 7 → 11 | the encoding guard *and* all five arena cases |
| chained-concat refusal removed | the refusal table |

### 27.6 What this step does NOT claim

* **Chained concatenation is refused**, not lowered: `print(a + b + "ef")` is
  `error[K145]`. Supporting it needs depth threaded through `natEmitStrValue`, `strTemp`
  sized for the deepest chain, and the arena bound re-proved per nesting level. It is its
  own slice.
* **Binding a concatenation is refused**: `let c = a + b` is `error[K145]`. The lowering
  exists in print position only; a binding needs the result registered in the slot plan.
* **`return <concat>` is refused** - a string is not a process exit status.
* **`len()` of a concatenation is refused** - the result is not a variable.
* No `for-in`, `push`, records, maps, string slicing or string comparison, and the macOS
  entry/exit remain open.
* **KIR pin**: stale and red, deliberately, per 22.6/23.6/24.10/25.7 and section 26.10.
  Last measured 12570 (Step 9a); 9b-9g all move it, and it is **not predicted**.
* `kccOwnsNativeTargets` remains `false` and **151A is not closed**.