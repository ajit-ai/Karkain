# Increment 151P0 — Bitwise operators (P0 soundness fix)

*Status:* baselined 2026-09-28. Ordered **before 151B**, inside the frozen
v1.2.0 scope. A new increment (own baseline, own gate) rather than a 151
sub-slice, because it is a language-level correctness fix rather than
backend parity work, and because it is a soundness hole in the shipped
**1.1.0 "Stable"** release rather than an increment-151 deliverable.

---

## 1. The defect, demonstrated

Bitwise and shift operators are not implemented. Worse, on the reference
(Go) engine they are **accepted and silently mis-compiled**.

Reproduction — `bitprobe.kark`:

```
func show(a, b) {
    print("  " + str(a) + " & " + str(b) + " = " + str(a & b))
}

func main() {
    show(597, 21)
    let x = 1234605616436508552
    print("  x >> 8 = " + str(x >> 8))
    print("  1 << 4 = " + str(1 << 4))
}
```

`karkain run bitprobe.kark` on the **Go** engine — exit 0, no diagnostic:

| expression | expected | actual (defective) |
|---|---|---|
| `597 & 21` | `21` | `597` |
| `1234605616436508552 >> 8` | `4822678189205111` | `0` |
| `1 << 4` | `16` | `1` |

Both expected values are worth stating in full, because the *first* one is
a trap: `597 & 21` is `0x255 & 0x015` = `0x015` = **21**, not `5`. An
early draft of this baseline recorded `5`, which was an arithmetic slip in
the baseline rather than a second compiler bug — it is corrected here
because a baseline whose own expectations are wrong is worse than no
baseline. The shift is verified by hand as `4822678189205111 * 256 + 136 =
1234605616436508552`.

The same program on the **kcc** engine does not compile: the operator is
dropped from the AST and the operand is emitted as a bare identifier, so
gcc reports `'natMask' undeclared`. Both failure modes were hit for real
while porting the encoder — see section 5.

This is the most serious class of defect this repository has: a user's
program containing `a & b` compiles cleanly, exits 0, and prints a wrong
answer. No diagnostic, no non-zero status, no crash. A parse error would
be strictly better.

## 2. Root cause — three independent gaps

Measured in the current tree, not inferred.

### 2.1 The lexer never produces the tokens (Go: `pkg/lexer/lexer.go`)

| input | token | line |
|---|---|---|
| `&&` | `TokenAnd` | 274 |
| `&` | `TokenAmp` — the **Phase 41 borrow/reference** operator | 276 |
| two-char or | `TokenOr` | 283 |
| single `\|` | **`TokenIllegal`** | 285 |
| `^` | **`TokenIllegal`** | default |
| `<<` | **not lexed** — `<` scanned, then `<` again | 288 |
| `>>` | **not lexed** — two `TokenGreaterThan` | 302 |

`&` is the interesting one: the token already exists, but it is spoken for.
`TokenAmp` is the Phase 41 borrow operator, and the parser consumes it in
**prefix** position in four places (parser.go lines 438 and 493 for `&T` /
`&mut T` type annotations, 765 for an expression statement, 1572 for the
`&x` borrow expression). In `a & b` the `&` is *infix* — it follows a
complete left operand — so the two uses are separated by position, exactly
as Rust separates `&x` from `a & b`. Adding `TokenAmp` to the infix
operator switch therefore cannot disturb any existing borrow syntax, and
`&` needs no new token.

`<<` and `>>` are not *wrong* tokens, they are *no* token: `x >> 8` reaches
the parser as `x > > 8`, which is why the observed result is `0` rather
than a bit pattern — the parser evaluated a comparison whose right operand
is the leftover token.

Ordering constraints a fix must preserve: `&&` beats `&`, `||` beats the
single `|`, `<=` beats `<<`, and `<-` (`TokenSend`, the concurrency
`chanSend` keyword) must keep winning over `<<`.

### 2.2 The parser has no level and no cases (Go: `pkg/parser/parser.go`)

`precedence()` (line 1231) defines five levels — logical or, logical and,
comparisons, add/sub, mul/div/mod — with no bitwise level between them.
The operator switch in `parseBinaryExpr` (line 1357) maps tokens to
operator strings and has no case for any bitwise token, so even once 2.1
is fixed the parser would still drop them. **Both** must change or neither
helps.

### 2.3 The runtime has no implementation (Go: `pkg/codegen/codegen.go`)

`binary_op` (line 1829) implements add, mul, div, mod, the four
comparisons, inequality, equality and the string rules, then falls through
to `return make_int(0)` (line 1962). A `BinaryExpr` carrying a bitwise
operator would therefore silently yield `0`.

This is a *latent* second defect: it is masked today because the parser
drops the operator first (2.2), and fixing only the front end would
convert a wrong-but-recognisable result into a uniformly wrong `0`.

The self-hosted engine has the same gap in its own emitted C helper
(`src/compiler/codegen.kark`), which mirrors the Go preamble.

## 3. Scope

**In scope — implement on both engines, byte-identically:**

- `&` bitwise and, `|` bitwise or, `^` bitwise xor
- `<<` shift left, `>>` arithmetic shift right
- C-like precedence matching the C table exactly, because Karkain
  compiles to C. From loosest to tightest:
  `||` < `&&` < `|` < `^` < `&` < comparisons < `<<` `>>` < `+` `-` <
  `*` `/` `%`. Two placements are easy to get wrong and **both were wrong
  in the first draft** of this increment, caught by the gate probe rather
  than by inspection: the bitwise levels were placed tighter than the
  comparisons, and the shifts tighter than multiply. The correct grouping
  is `1 << 3 + 1` = `1 << (3 + 1)` = 16, and a first attempt produced 9.
  The relative order of the pre-existing operators is untouched, so no
  expression that was valid before now groups differently.
- Well-defined edge behaviour, because the C operators are undefined out
  of range. Karkain integers are signed 64-bit, so: a shift count below
  zero or at or above 64 yields 0; left shift is performed unsigned and
  wrapped, so it never invokes C undefined behaviour; right shift is
  arithmetic and therefore sign-preserving.
- Non-integer operands (float, bigint, string, bool) yield 0, matching how
  `/` and `%` already return 0 rather than trapping.

**Out of scope:** compound assignment operators (`&=`, `<<=`, ...),
`~` (bitwise not), and any use of bitwise operators in the compiler's own
sources. None are required by 151B.

## 4. Exit criteria

1. `pkg/cli/phase151p0_bitwise_test.go` green, asserting **correct** values
   for all five operators on **both** engines and byte-identical stdout
   between them — not merely that the two agree.
2. Precedence pinned: `1 << 3 + 1`, `6 & 3 + 1`, `1 | 2 ^ 3 & 1`, and a
   parenthesised control for each.
3. Edge table pinned: shift by 0, 1, 63, 64, 65, and a negative count.
4. Negative table pinned: bitwise on a float, on a string, and on a bool
   all yield 0 on both engines.
5. A **rejection** is *not* added for these operators any more — the whole
   point is that they are accepted and correct.
6. `TestPhase122_PipelineOwnership/KIRContinuity` re-pinned if the KIR
   line count moves, and the self-hosted `check` of `src/compiler/main.kark`
   still clean.
7. `go build ./...`, `go vet ./...`, and `GOARCH=386 go vet ./...` clean.

## 5. How this was found (why 151B cannot proceed without it)

Increment 151B is the machine-code encoder in kcc. The encoder is bit
manipulation by definition: truncating a byte needs `& 0xFF`, nibble
extraction needs `>> 4`, and REX prefix assembly needs `| 0x01`. The first
draft of `src/compiler/native_emit.kark` was written against those
operators and would not build on **either** engine.

That is why this is a hard blocker and not a nice-to-have. Two further
findings from the same attempt, both recorded so they are not rediscovered
the hard way:

- **Karkain has no hex literals.** `0xFF` lexes as the identifier `xFF`.
  Every mask in a ported encoder must be written in decimal, which is why
  the encoder draft wrapped its constants in named accessors with the hex
  preserved in the comments.
- **A zero-argument call in expression position is dropped by the kcc C
  codegen.** `let a = zero()` compiles on the Go engine and emits a bare
  `zero;` on kcc. One-argument calls are fine. This is a separate defect
  from the bitwise one; it does not block 151B (the encoder draft works
  around it with one-argument accessors) but it is a real parity bug and
  should be filed.

The encoder draft is preserved outside the repository at
`%TEMP%\\opencode\\p151\\native_emit.kark.wip` and is **not** committed,
because a slice that cannot work is not progress.

## 6. Governance note

This increment does not reopen 150, 151A-1, or any frozen slice, and it
changes no emitted bytes of an existing golden: no native image, no C
output byte, and no KIR line count depends on a bitwise operator, because
none existed to depend on. The change is additive to the language surface
and to `binary_op`, which appends new branches after the existing ones.
