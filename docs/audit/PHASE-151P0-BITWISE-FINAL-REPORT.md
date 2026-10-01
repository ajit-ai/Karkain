# Increment 151P0 — Bitwise operators: final report

*Verdict: **COMPLETE**.* New increment, baselined before the work
(`PHASE-151P0-BITWISE-BASELINE.md`), inside the frozen v1.2.0 scope,
ordered before 151B. It is a new increment rather than a 151 sub-slice
because it is a language-level soundness fix, not backend parity work, and
because it is a hole in the shipped 1.1.0 "Stable" release.

---

## 1. What the defect was

`&`, `|`, `^`, `<<` and `>>` were not implemented, and on the **reference
(Go) engine** they were accepted and silently mis-compiled. A program
containing `a & b` compiled, exited 0, and printed a wrong answer — no
diagnostic, no non-zero status. A parse error would have been strictly
better.

The increment was forced, not chosen: 151B is the machine-code encoder in
kcc, and an x86-64 encoder is bit manipulation by definition. The first
draft of `src/compiler/native_emit.kark` would not build on either engine.

## 2. Root causes — four, all measured

| # | Layer | Defect |
|---|---|---|
| 1 | `pkg/lexer/lexer.go` | no `\|`/`^` tokens at all; `<<`/`>>` never lexed, so `x >> 8` reached the parser as `x > > 8` (hence the observed `0`) |
| 2 | `pkg/parser/parser.go` | `precedence()` had no bitwise level; the infix operator switch had no cases |
| 3 | `pkg/codegen/codegen.go` | `binary_op` had no branch for the five operators and fell through to `make_int(0)` — a *latent* second defect, masked by 1 and 2 |
| 4 | `src/compiler/lexer.kark` | a single `&` was emitted as the **logical**-and token, so `a & b` silently meant `a && b` on the self-hosted engine |

`&` needed no new token: it already existed as `TokenAmp`, the Phase 41
borrow operator, and the two uses are separated by **position** — prefix
`&x` is a borrow, infix `a & b` is bitwise-and — exactly as in Rust. The
four existing prefix sites (parser.go 438, 493, 765, 1572) are untouched.

## 3. What changed

* **Go lexer** — four new tokens (`TokenPipe`, `TokenCaret`,
  `TokenShiftLeft`, `TokenShiftRight`); `TokenAmp` reused infix. Ordering
  preserved: `&&` beats `&`, `||` beats `|`, `<=` beats `<<`, and the
  concurrency send operator `<-` still beats `<<`.
* **Go parser** — the C precedence table, from loosest to tightest:
  `||` < `&&` < `|` < `^` < `&` < comparisons < `<<` `>>` < `+` `-` <
  `*` `/` `%`. The relative order of the pre-existing operators is
  untouched, so nothing that parsed before groups differently now.
* **Go codegen** — `binary_op` gained the five operators with a defined
  answer for every edge rather than inheriting C's undefined behaviour.
* **kcc** — `TK_AMP/TK_PIPE/TK_CARET/TK_SHL/TK_SHR` (ids 65–69, previously
  free) in `lexer.kark`; `kPrec`, `isOpToken`, `kindToOp` in `parser.kark`;
  the same `binary_op` block in `codegen.kark`.

## 4. Semantics pinned by the gate

| case | result |
|---|---|
| shift count < 0 | `0` |
| shift count >= 64 | `0` |
| `<<` overflow | wraps (unsigned, no UB) |
| `>>` of a negative | arithmetic, sign-preserving |
| float / string / bigint operand | `0` |
| bool operand | accepted and coerced, as in arithmetic |

## 5. Three things I got wrong and corrected

These are recorded because each was caught by measurement, not by review,
and each would have shipped as a silent wrong answer.

1. **The baseline's own expected values were wrong.** It recorded
   `597 & 21` = 5 (it is 21: `0x255 & 0x015 = 0x015`) and the shift result
   as 4799245345463037 (it is 4822678189205111, verified by hand:
   `× 256 + 136` restores the original). A baseline with wrong expectations
   is worse than none, so both are corrected in the baseline too.
2. **The precedence table was wrong twice.** The first draft put the
   bitwise levels *tighter* than the comparisons and the shifts *tighter*
   than multiply, so `1 << 3 + 1` produced 9 instead of 16. Only the probe
   caught it. The correct C grouping gives 16, and `1 | 2 == 2` gives 1.
3. **The gate's bool expectation was wrong.** I asserted `true & 1` = 0, on
   the reasoning that bitwise is stricter than arithmetic. The reference
   engine returned 1, and it is right: `is_truthy` lowers a bool to int
   before `binary_op` sees it, so refusing it would make `true & 1`
   disagree with `true + 1` for no stated reason. Both engines now accept
   and coerce bool, and the exception is stated explicitly in the gate.

## 6. Evidence

**Gate** `pkg/cli/phase151p0_bitwise_test.go` — 4/4 PASS (52.8 s):

| test | what it proves |
|---|---|
| `BitwiseCorrectOnBothEngines` | correct values on Go **and** kcc, 41 golden lines |
| `CrossEngineParity` | the two engines are byte-identical |
| `NonIntegerOperands` | float/string yield 0; bool coerces |
| `LogicalOperatorsUnaffected` | `&&`, `\|\|`, `<`, `<=`, `>`, `>=` unchanged on both engines |

**The gate asserts correct values, not just agreement.** This is the whole
reason it can catch the class of defect: a byte-identity-only gate passes
on two engines that are identically wrong, which is exactly the state this
was found in.

**Mutation-verified**: neutering the `&` case in `binary_op` fails 3 golden
lines (`255&15`, `597&21`, `-1&255`). The gate is live.

**Regressions green** (all run isolated, per the ~4 GB host rule):

| suite | result |
|---|---|
| `go build ./...`, `go vet` (lexer/parser/codegen/cli/sema) | clean |
| `pkg/lexer`, `pkg/parser`, `pkg/sema` | ok |
| `pkg/codegen` | ok (71.4 s) |
| `pkg/native`, `pkg/compiler`, `pkg/ir/...`, `pkg/target`, `pkg/wasm` | ok |
| Phase 114 corpus, **GoEngine** leg | PASS (452.6 s) |
| Phase 114 corpus, **KCCParity** leg | ok (568.5 s) |
| Phase 148 / 150D / 151 gates | ok (32.2 s) |
| Phase 122 KIRContinuity | ok (255.5 s), pin re-pinned 9574 → 9642 |
| kcc self-check of `src/compiler/main.kark` | `[ok]` |

**A harness artifact, recorded so it is not mistaken for a regression:** the
two Phase 114 legs in *one* `go test` invocation exceed Go's 10-minute
default and `panic: test timed out after 10m0s`. Each leg passes on its own.
This is a per-invocation budget, not a failure of either leg.

The KIR pin moved 9574 → 9642 (+68) because the new lexer, precedence and
`binary_op` lines are rendered by KIR; `kir verify` passes on the new
count, and the pin is re-pinned with a note naming the cause.

## 7. Boundaries (documented, NOT defects)

* Compound assignment (`&=`, `|=`, `^=`, `<<=`, `>>=`) and `~` (bitwise
  not) remain **unimplemented**; nothing in 151B needs them.
* Bitwise on a float, string or bigint yields `0` rather than trapping. That
  matches how `/` and `%` already behave on a zero divisor, and is asserted
  by the gate so it is a contract rather than an accident.
* Relational (`< <= > >=`) and equality (`== !=`) remain one precedence
  level. C splits them, but splitting them now would change the grouping of
  existing programs, which is out of scope for a bug fix.

## 8. Two findings recorded but deliberately NOT fixed here

Both are real, and both are in the baseline so they are not rediscovered
the hard way:

1. **WITHDRAWN 2026-09-30 — a zero-argument call in expression position is
   NOT dropped by the kcc C codegen.** This report originally recorded it as
   a genuine parity bug that should be filed, on the basis that `let a =
   zero()` "emits a bare `zero;` on kcc". Re-measured, it does not
   reproduce: `kcc build` emits `Value a =\nkarkain_user_zero();` — the call
   is in initializer position — and both engines print `7`. See
   PHASE-151P0-BITWISE-BASELINE.md for the emitted C and the full note.
2. **Karkain has no hex literals.** `0xFF` lexes as the identifier `xFF`.
   Every mask in a ported encoder must be decimal, which is why the encoder
   draft wraps its constants in named accessors with the hex preserved
   in the comments.

## 9. Governance

This increment reopens no frozen increment and changes no emitted byte of
an existing golden. No native image, no generated-C byte and no KIR line
count depended on a bitwise operator, because none existed to depend on.
The change is additive to the language surface and appends new branches
after the existing ones in `binary_op`.

**151B is now unblocked.** The encoder draft is preserved at
`%TEMP%\\opencode\\p151\\native_emit.kark.wip` and is intentionally **not**
committed, because it still has the one open question the baseline names:
byte-parity with the Go `Emitter` needs a decision about how far kcc's own
emission must be bug-compatible with Go's, not merely equivalent to it.
