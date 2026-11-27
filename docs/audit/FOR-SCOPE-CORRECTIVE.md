# Unnumbered corrective slice — kcc C-style `for` scope leak

Status: **COMPLETE** (gate + checker fix + CI step + docs).
Line: v1.2.x. No version number assigned: `v1.2.32` is reserved for the
REL-001 sanitizer investigation and `v1.2.33` for the security model in
`KARKAIN-IMPLEMENTATION-CONTROL.md` §4.4. This is a corrective change against
the frozen checker baseline, not a new increment.

## 1. Measured defect

At the committed baseline (`37c845e`), Go `check` and kcc `check` disagreed
about the lifetime of a binding introduced by a C-style `for`:

| Program | Go `check` | kcc `check` |
|---|---|---|
| `for (let i = 0; ...)` then `print(i)` | `error[K002]` undefined identifier `i`, exit 3 | `[ok]`, exit 0 |
| `for (var i = 0; ...)` then `print(i)` | `error[K002]` undefined identifier `i`, exit 3 | `[ok]`, exit 0 |
| `let y = 5` in the body, `print(y)` after | `error[K002]` undefined identifier `y`, exit 3 | `[ok]`, exit 0 |
| bare `z = 7` in the body, `print(z)` after | `error[K002]` undefined identifier `z`, exit 3 | `[ok]`, exit 0 |

The earlier working premise — "only the `for`-init binding leaks" — was
**falsified by measurement**: the body `let` and the implicitly declared bare
assignment leak too. The single mechanism is that kcc's pass-1 `collectLocals`
had a `For` branch that added the init name and descended into the body, so
those names became permanent locals of the enclosing function; pass-2's `For`
arm added the init again and never removed anything. Go's
`pkg/sema/resolve.go:walkLocalDefs` has **no `ForStmt` case at all**, which is
why the Go reference rejects all four.

## 2. Fix

Two edits in `src/compiler/checker.kark`:

1. Deleted the pass-1 `collectLocals` `For` branch. Loop names are no longer
   registered as permanent locals of the function.
2. In the pass-2 `For` arm: snapshot the enclosing locals by resetting
   `tab[4]` to a fresh array and copying the saved prefix **before** any
   append; pre-seed the body names with `collectLocals(tab, forBody(s))`;
   check init / condition / post / body as before; restore `tab[4] = saved`.

The body seed is load-bearing, not cosmetic. `print(y); let y = 5` inside a
loop body is accepted by **both** engines (forward reference), so simply
deleting the pass-1 branch would have broken that program. Seeding in pass 2
preserves both the acceptance and the existing linear-registration order.

The save/restore follows the discipline already used by `ckCheckArmBody`
(`checker.kark:785-824`) and `checkFuncDecl` (`:695-744`): reset the table slot
to a fresh array **first**, so every append lands in the fresh array and the
saved array stays pristine. Truncating an array that is still being iterated
is the Phase 133 append-into-iterated-array use-after-free class.

### A real defect found while landing the fix

An intermediate implementation built the restore prefix in a local
(`int prefix = []` plus `appendArray`) and assigned it to `tab[4]` at the end.
That produced a deterministic `0xC0000005` on `kcc check` for **every valid
program with a `for`-init binding**, with empty output — which the CLI surfaced
as exit 3 through `kccCheckPreflight`, looking exactly like a checker rejection.
Bisection isolated it:

- pristine HEAD checker: all programs pass;
- pass-1 branch deleted only: all pass (but the leak remains);
- plus body seed: all pass (leak still remains);
- plus local-array restore: valid programs crash;
- with `tab[4] = []` first and `tab[4] = saved` at the end: all correct.

The final code uses the proven form. This is recorded rather than hidden
because the failure mode — a crash that looks like a semantic rejection — is
the exact class a checker gate must not be allowed to absorb silently.

## 3. Evidence

Gate: `pkg/cli/phase_for_scope_test.go`, `TestPhaseForScope_`.

- **Unfixed tree:** `TestPhaseForScope_LeakedBindingsRejectedByBothEngines`
  failed all four cases with `[ok]`, exit 0 and no `K102`; the accepted corpus
  passed. Log: `C:\Windows\Temp\opencode\forscope_gate_before.log`.
- **Fixed tree:** all three tests pass — four leak cases rejected by Go with
  `error[K002]` and by kcc with `error[K102]` (code **and** identifier name
  asserted, never just non-zero status); five exact-byte run goldens agree on
  both engines; forward reference, post-loop `for-in` read and post-loop
  `while`-body `let` remain accepted. 59.248 s.
- **Mutation:** removing the `tab[4] = saved` restore reproduced all four
  pre-fix diagnostics; the file was then restored byte-identically (SHA-256
  `6B67E2513AE413224CC77B3BE5B2A0D64F40EE5CC4591C1D1A6951BFCDB78D86`) and
  the full gate re-run green. Two explanatory comments were later reworded
  for accuracy (no behavior change); the final file SHA-256 is
  `4FDD505C0B503BCCCB46FEF8187D462B23E246E7D6B96F1B0C07ED71E39EC9AD` and
  the full gate re-ran green in 67.789 s.
- **Fresh compiler:** the gate unsets inherited `KARKAIN_KCC` and builds kcc
  from the working tree via `phase95KCC`, copying the binary to a
  process-lifetime directory, so a stale `kcc.exe` cannot mask the change.

CI: dedicated step *Run kcc for-scope corrective gate*, pattern
`TestPhaseForScope_`, wired in `.github/workflows/ci.yml` in the same commit.

## 4. Boundaries, stated explicitly

- **This is not full `for` parity.** Go's `checkStmt` has no `ForStmt` case and
  does not check inside a `for` body; kcc does and is stricter there. That
  behaviour is deliberately kept.
- `for-in`, `if`, `while` and `Block` scoping are untouched. Go `check`
  accepts post-loop reads for those, so changing them would create a new
  divergence. Three check-only pins in the gate guard this.
- Only `tab[4]` (locals) is scoped. Const and linear-type tables are not
  truncated; no reported case depends on them, and widening the restore would
  change unrelated checker behaviour.
- **KIR pin not re-pinned here.** `checker.kark` changed, so the whole-tree
  count will move. Local whole-tree verification crashes/hangs on this host;
  per the standing rule the value is taken only from a CI-measured
  `text=N verify=N`. Both assertion sites retain the current value until then.
- No language surface, stdlib, native backend or version identity changed.
