# NAT-CALL baseline — native user-function-call lowering in kcc

**Increment:** `NAT-CALL` (unnumbered working label; v1.3.0 §1 item 1).
**Version:** 1.3.0 (not open — `VERSION` stays `1.1.0`; no release claim follows
from this slice).
**Status:** baselined before implementation.
**Definition of done:** `docs/release/v1.3.0-CHECKLIST.md` §1 item 1 — *a real
`.kark` function called from `main` builds and **executes** through the C-free
native path on both engines; refusal table shrinks by name, not by omission;
kcc↔Go byte-identity for the covered shapes.*

---

## 1. The measured blocker

Not inferred from reading. Measured on the committed compiler
(`develop` = `ad6a28c`), with the fixture

```kark
func add(a int, b int) int {
    return a + b
}

func main() {
    print(add(2, 3))
}
```

```
$ karkain build --target native-x86_64-windows prog.kark
error[K145]: native target: call to `add` is not lowered by the native driver
(main.kark line 7). ... This is refused HERE, by kcc, and is not handed to the
Go engine.
exit 3, no image written
```

The Go oracle builds the same program and prints `5`. So the gap is one of
*native lowering*, not of the language, the parser or the checker.

The driver (`src/compiler/native_value.kark`) is a **single-function** driver:

* `natNativeProgramOS` (line 5139 at baseline) finds `main` by name
  (`natNativeFindMain`), takes `natNativeBodyStmts(mainFn)` and never looks at
  any other `FuncDecl`.
* `natNativePlan(stmts, tbl)` allocates a slot per `let`/`var`; there is no
  slot for a parameter, because nothing ever lowers a parameter.
* `natNativeScanValue` routes **every** `NODE_BINARY` to `natNativeScanConcat`,
  which refuses any operator other than `+` and requires two string operands —
  so `a + b` on integers is refused as a malformed concatenation, not as an
  unsupported arithmetic form.
* `natNativeLower` hard-codes the label `karkain_main` and `karkain_main$ret`.

---

## 2. What this slice adds

### 2.1 Surface

| Construct | Baseline | After NAT-CALL |
|---|---|---|
| `func f(a int, b int) int { return a + b }` | refused | lowered, callable |
| `print(f(2, 3))` | refused | lowered, executes |
| `return f(2, 3)` (from `main` or from `f`) | refused | lowered, executes |
| `let m = f(2, 3)` inside a **called function** | refused | lowered, executes |
| `let c = f(2, 3)` inside **`main`** | refused | **still refused** — `main`'s `let` arm is frozen by 9c; see §5 |
| `let x = 1 + 2` / `print(a - b)` / `print(a * b)` / `print(-a)` | refused | lowered |
| `f(g(1), 2)` (nested call) | refused | lowered |
| 7+ int arguments | n/a | lowered (units 6+ via the caller's extras area / `R10`) |
| recursion (`f` calls `f`) | n/a | lowered (no inlining, so no cycle walk) |

### 2.2 Integer arithmetic is in scope, and why

`+ - *` and unary `-` are part of *the value model the call needs*, not a
separate feature request. The driver had no integer arithmetic at all, so a
call that returns `a + b` would be a call whose body could not be lowered —
testing function calls while refusing every interesting body would prove the
call mechanism against bodies that do nothing. The operators that are **not**
added are named in §4 so the boundary is a decision, not an omission.

### 2.3 kcc↔Go byte identity

The bar is the same one `TestPhaseD2_`/`TestPhaseD3_` established: **Linux
ELF** byte identity against `pkg/native`, measured as a differential (the
oracle's own `Link` produces the expected bytes at test time; nothing is
pinned as a golden). Windows PE images already differ from the oracle's by the
documented Win64 call-site alignment (`sub rsp, 8` around every call site,
which the oracle does not emit); parity of *bytes* is therefore not claimed on
Windows and the Windows evidence is **execution + structure**.

---

## 3. Implementation shape (and the one thing that must not move)

### 3.1 Per-function plan

Parameters come first in the frame, in declaration order — the oracle's
`layout()` (`pkg/native/program.go:4411-4428`) walks `fd.Params` before
`scanLets(fd.Body, next)`. To let a slot be resolved by NAME without a second
table, the plan widens from 3 to 4 fields:

```
[off, kind, elems, name]        (was [off, kind, elems])
```

`natNativePlanCount` becomes `len(plan) / 4`; `natNativeSlotOff/Kind/Elems`
index at `idx*4`; `natNativeSlotName(plan, idx)` and
`natNativeSlotIndex(plan, name)` are added. Every one of the 17
`natNativeLocalIndex(stmts, name)` call sites becomes
`natNativeSlotIndex(plan, name)` and `natNativeLocalIndex` is **deleted**, so
there is exactly one slot-resolution rule and no statement list can shadow it.

`natNativePlan` is retained as `natNativePlanFor([], stmts, tbl)` so the
frozen Step 9d-9h gates keep driving the same numbers.

### 3.2 Per-function frame

`natFrameLayout` is unchanged and is called **once per function**, exactly as
the oracle calls `layout(fd)` per `FuncDecl`. What differs from the current
driver is that `maxExtra` becomes a real argument
(`natNativeMaxExtras(body)`, mirroring `scanMaxExtras`) instead of the literal
`0`. `usesStr`/`usesMap`/arena remain **program-wide** (the oracle decides them
once, in `CompileProgram`, from the whole program), so they are computed from
the concatenation of every function's body in source order.

For a single-function program the concatenation *is* `main`'s body, and
`natNativeMaxExtras` returns 0 when there is no call site — so every frozen
9c-9h/D2/D3 image is byte-identical by construction, and the gates re-assert
it rather than assuming it.

### 3.3 Emission

Oracle order (`pkg/native/program.go:1585-1593`), reproduced exactly:

```
helpers                            (print_int, print_str, [alloc], [map], [runtime_error])
for each FuncDecl in source order: Mark(label); layout; [sub rsp, frame];
                                   home params; body;
                                   if !returned: xor eax, eax;
                                   Mark(label + "$ret"); add rsp, frame; ret
_start last
```

`natNativeLower` becomes per-function (`fn` instead of `stmts`) and gains the
layout array and the function table. The `$ret` label is
`natNativeFuncLabel(funcName(fn)) + "$ret"`.

### 3.4 Parameters are all `int`, and that is measured, not chosen

`src/compiler/parser.kark` `parseFunc` (`:880-940`) appends the parameter
**name** and discards its type annotation (`psIsTypeToken` is consumed, the
token is not stored). There is no annotation to read back, so a
`string`/`float`/array/record parameter cannot be distinguished from an `int`
one at this layer. Rather than guess, NAT-CALL treats **every** parameter as
one 8-byte integer unit and refuses any argument whose kind is not `int` **at
the call site**, by name. Adding a parameter-type slot to the parser moves the
whole-tree KIR pin and is a separate decision (recorded in §5).

Consequence, stated plainly: a program that passes a string to an `int`
parameter is refused by the *argument's own kind*, which is the correct
diagnostic for the wrong reason. That is recorded here so it is not mistaken
for a type system.

---

## 4. Refusals that stay, by name

Each of these is asserted in the gate with its own message, so shrinking the
table is a deliberate edit and not a silent widening:

1. **Callee body using a non-int kind** — a `string` literal, concatenation,
   array literal, index, `len()`, record literal/field, or map in any
   non-`main` function. Reason: `natNativeStrLiterals`,
   `natNativeArenaBytes`, `natNativeUsesRuntimeError` and the rodata
   interning cursor (`sidx`) are threaded per-`main` at baseline; a callee
   that interned a literal would desynchronise the offset table. Refused by
   name in the per-function scan.
2. **`/`, `%`, comparison and logical operators** on integers — a loud K145
   (the oracle has a division-by-zero short circuit at `emitBinary`, which is
   a separate slice's worth of emission).
3. **Array/record/string/map arguments** — refused at the call site by kind.
4. **`main` with parameters** — the oracle refuses it explicitly
   (`program.go:4808-4810`: "`main` takes no arguments on the native target");
   kcc now refuses it in the same words rather than silently ignoring the
   parameters.
5. **Every refusal the baseline already had** — `abs(a[0])`
   (`is not lowered by the native driver`), `chained string concatenation is
   not lowered`, `a + n` with `a` a string (`needs two strings`). The frozen
   gates that assert these keep asserting them.

`natNativeRefuse`'s enumerated subset text is updated in the same change: it is
the user-facing contract, and a refusal that describes an older subset is
worse than no description.

---

## 5. Explicitly out of scope

No phase number is invented and no other authorised work is touched.

* **`main`'s `let` arm** — it stays exactly as Step 9c left it, so
  `let c = f(2, 3)` in `main` is still refused ("initialised from a non-integer
  expression") while the SAME statement inside a called function is lowered.
  This asymmetry is deliberate rather than an oversight: widening `main`'s arm
  would have added a second emission path (and its own frozen-image risk) to
  prove something the call mechanism already proves, and the refusal message
  names the distinction. It is pinned as a refusal row (`main_let_from_call`)
  so the day someone widens it the gate says so.
* **Parameter type annotations** — the parser change that would let a
  parameter be a `string`, plus its KIR-pin movement.
* **String/float/array/record/map parameters and returns.**
* **`/`, `%`, comparisons, `&&`/`||`/`!`** on integers.
* **Loops, `if`, `break`/`continue`** — the driver still has no control flow,
  which is why `push()`/map-insert inside a loop remains refused and why the
  arena bound stays a proof.
* **Phase 150C register allocation** — it engages only under genuine pressure
  *and* control flow, and this driver has no control flow, so every local is a
  frame slot on both sides. A future control-flow slice must revisit this.
* **Windows byte parity** — see §2.3.
* **Increment 154 / REL-002 / B2** — capacity-blocked, untouched.
* `VERSION`, Phase 152, and the Language Hardening Checkpoint (closed).

---

## 6. Gate

`pkg/cli/phase151e_call_test.go`, tests named `TestPhaseNATCall_*` — the filename
was corrected from this section's first draft (`phase151d_call_test.go`) for two
reasons recorded rather than quietly fixed: `TestPhase151D_*` is a FROZEN
namespace (A1/Phase B), so a `151d` file would read as an amendment to that
slice; and a `_linux`/`_wasm` suffix becomes an implicit Go build constraint
(the trap A1 hit, where the gate was silently not compiled). `e` is simply the
next letter after A1/Phase B/D1-D5 in this increment. The `TestPhaseNATCall_`
prefix is matched by no other `ci.yml` step, so a NAT-CALL regression cannot
hide behind a green neighbour. `ci.yml` gains a dedicated step in the same
change — a gate is a test file **plus** a CI entry.

The gate carries these layers, and each exists because a different class of
wrong passes all the others:

1. **executed goldens** — real programs run through the produced PE on this
   Windows host with **exact stdout bytes** (not `TrimSpace`d: the Step 9b
   defect was a *missing byte* that trimming hid);
2. **byte identity vs the Go oracle** on Linux ELF for every covered shape,
   as a differential against `pkg/native`, plus a "still differs" assertion for
   shapes known to be outside the covered set so a gap cannot become silent;
3. **frame numbers** derived independently in the test from the oracle's own
   rule (params first, 8 bytes each, then locals, then 512 `binTemp`, then the
   extras area, then `argSpillBytes`);
4. a **refusal table** that asserts the exit code *and* that no image was
   written, for every item in §4;
5. **determinism** (same source → same image bytes, twice) and
   **non-vacuity** (distinct programs → distinct images);
6. **no-Go-fallback** — every image carries kcc's provenance line, and the
   `pkg/native` writer is never invoked for emission.

Mutation verification is part of the record: each new mechanism (param homing,
the `$ret` label, the call-site staging, the plan widening) is broken in place,
the expected gate failure observed, and the file restored byte-identically.

---

## 7. Risks, named before the code

| Risk | Why it is real | Mitigation |
|---|---|---|
| The 3→4 plan widening silently shifts a frozen slot | `natNativeSlotOff` is used by every 9d-9h path | `natNativePlanBytes` unchanged; the frozen 9c-9h + D2/D3 gates are re-run and must report **zero** byte drift |
| `maxExtra` threaded wrongly changes every frame | a wrong `extrasBase` moves `frame` | asserted to be 0 for every call-free frozen program, and derived independently in the gate for a 7-argument call |
| Function emission order drifts | D1 measured order as the root cause of the 39-byte delta | helpers → source order → `_start`; single-function programs are unchanged by construction and re-measured |
| A callee literals desynchronises rodata | `sidx` is a `main`-only cursor at baseline | callees are int-only (§4 item 1) and refused by name otherwise |
| A call reaches emission without staging | Phase 149/151B's worst class: a valid image with a wrong answer | the scan validates arity and every argument's kind before any emission; the gate compares executed bytes |

---

## 8. Record

Filled after the implementation lands: measured gate results, measured KIR pin,
the frozen-gate zero-drift result, and any defect found by a gate rather than by
inspection.
