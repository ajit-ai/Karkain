# NAT-FORIN — `for ... in` in the self-hosted native driver

Status: **COMPLETE for the covered subset** (increment 151, slice "NAT-FORIN").
This is a close-out record, not a new numbered phase.

## Scope

The kcc native driver previously refused `for x in arr` by name. This slice
ports the oracle's `emitForIn` shape (`pkg/native/program.go`) into
`src/compiler/native_value.kark`:

* a hidden index slot keyed by `forInVar(n) + "$idx$" + str(forInLine(n))`,
  so nested loops cannot collide;
* plan order mirroring the oracle's `scanLets`: loop variable first
  (first-declaration-wins), hidden index immediately after, nested body
  bindings next;
* `natCmpMemReg` / `natIncMem` in `src/compiler/native_emit.kark` for the
  `len(a) > i` guard and the increment, with golden-pinned encodings;
* `break` / `continue` using the existing nearest-loop stack.

The iterator must be an array **variable**. A literal `for x in [1, 2]` is
refused by both engines, because there is no frame-resident `(base, len)`
header to compare against.

## Evidence

Gate: `pkg/cli/phase151a9i_forin_test.go`, pattern `TestPhase151A9I_`, with a
dedicated CI step *Run 151A Step 9I for-in gate* in `.github/workflows/ci.yml`.
Measured locally: **PASS** (36.7 s).

* **Executed PE** (Windows host, exact bytes, no trimming):
  `plain` `10/20/30`, `single` `42`, `sum` `10`, `nested`
  `110/120/210/220`, `break_continue` `1/3`, `index_inside` `5/6/7`,
  `cond_body` `0/8/12/0`. Every kcc image was validated by the oracle's
  `native.ParsePE` and compared to a Go-oracle image built from the same source.
* **Linux ELF byte identity**: all seven corpus programs were compared
  byte-for-byte against the Go oracle's images (differential, not a golden),
  with the kcc provenance marker asserted so a Go fallback cannot pass.
  Measured sizes in the validation harness: 501, 445, 575, 652, 651, 1188
  and 1155 bytes respectively. Linux **execution** remains a CI-leg claim on
  this Windows host (no WSL, no Docker).
* **Non-vacuity**: distinct sources produce pairwise-distinct images; a rebuild
  reproduces the same SHA; the corpus is non-empty.
* **Refusals**: literal iterator, `len()` inside a for-in body, and the
  pre-existing PE `push()` patch-guard failure all exit non-zero, name the
  construct, and write no image.
* **Register path**: the opt-in `KARKAIN_NATIVE_REGALLOC=1` path prints the same
  exact bytes as the qualified stack-based path on the covered programs.

## KIR

Whole-tree pin re-measured with
`KARKAIN_ENGINE=go karkain.exe kir --verify src/compiler/kir.kark`:
`[ok] kir text: 16017 lines`, `[ok] kir verify: 16017 lines ok`.
The old pin was 15780; the delta (+237) comes from the driver arms and the two
new emitter primitives. Both pinning tests were updated to 16017. The number
was measured, never predicted.

## Corrective changes to frozen gates

Three frozen refusal tables pinned "for-in is not lowered". Where the program
became supported, the row was **replaced**, not deleted:

* `pkg/cli/phase151a9d_array_test.go`: variable-array for-in → literal-iterator
  refusal (shared boundary with the oracle).
* `pkg/cli/phase151e_call_test.go`: `callee_for_in` / `main_for_in` now assert
  the literal-iterator message.
* `pkg/cli/phase151d_elfnative_test.go` and `phase151a9c_driver_test.go`
  already used a literal iterator and remained valid refusals.

Four additional frozen rows failed for reasons that predate this slice and were
**replaced or corrected with measured messages**, never silently deleted:

* `phase151a9f_string_test.go`: `let c = a + b` is now lowered by the already
  committed Phase 2.1 arena work, so the row became a string-condition refusal;
* `phase151a9g_concat_test.go`: the same Phase 2.1 change made bound
  concatenation supported, so the row became a string-slice refusal, and the
  file header was corrected;
* `phase151a9h_records_test.go`: the record-alias diagnostic was sharpened to
  name the struct value in an int position, so the expected text was updated;
* `phase_push_main_test.go`: the PE patch guard added by this slice turned a
  silent out-of-bounds store into `error[K117]` for `push()` programs. The
  former execution gate now pins the refusal (and the one loud slot-scan
  failure) instead of expecting an image that must not be trusted.

All four were demonstrated against the current compiler before editing; none
is a relaxation of a gate that still describes supported behavior.

## Not claimed

macOS execution (no runner, no Intel-Mac host); PE byte identity (the two entry
bootstraps are separate implementations, so PE compares stdout); a fix for the
`push()` PE patch guard; a register-allocation performance claim.
