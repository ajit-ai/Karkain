# Phase 1.3 bounds checker — corrective change

Status: **COMPLETE** (corrective change against the frozen Phase 1.3 baseline;
no new increment, no phase renumbering).

## Demonstrated failure

`pkg/sema/bounds_check.go` originally keyed its inferred array-size table by
variable name alone. That is unsound: a `Value s = [[], []]` binding in one
function made `s[14]` in a **different** function — where `s` is a parameter —
look out of range. The false positive rejected the compiler's own sources, so
this is a defect with a concrete failure, not a preference.

A related scope leak existed for blocks: an inner `let s = [1, 2, 3, 4, 5, 6]`
overwrote the outer `s`'s recorded size, and the larger size survived the block.

## Correction

`r.arraySizes` is now `map[string]map[string]int`, keyed first by the enclosing
function name. The helpers in `pkg/sema/bounds_check.go` are:

* `trackArraySize(fn, name, size)` / `forgetArraySize(fn, name)`;
* `enterArrayScope(fn)` / `leaveArrayScope(fn, saved)` for block restore.

`pkg/sema/resolve.go` calls them for `VarDeclStmt`, `BlockStmt`, `IfStmt`,
`WhileStmt`, `ForInStmt` and passes `callerName` to `checkBoundsSafety`.

## Evidence

New gate `pkg/sema/phase13_bounds_scope_test.go` (`TestPhase13_`), PASS:

* two functions reusing `s` produce exactly one bounds diagnostic, from the
  function that actually declared a two-element literal;
* a six-element declaration inside an `if` block does not change the outer
  `s`'s size: only the outer `s[3]` is diagnosed, with `length 2`, never
  `length 6`.

No emitted bytes change: this is a diagnostics-only resolver pass, so the
Phase 114 corpus and the self-hosted compiler's byte identity are untouched by
construction. `go test ./pkg/sema/ -run 'TestPhase13_' -count=1` passes.

## Second corrective change — assignment invalidation inside blocks

The first CI run on the NAT-FORIN PR exposed a second false positive:

```text
examples/closures/04_factory.kark:25
array index 0 out of range (length 0)
```

The example binds `let fns = []`, then grows it with
`fns = appendArray(fns, ...)` inside a `while` body. The size was forgotten by
the assignment, but `leaveArrayScope` restored the pre-block snapshot and
resurrected the length-0 size, so the post-loop `fns[0]` was rejected.

Fix: `pkg/sema/resolve.go` now records assignment invalidation in
`arrayForgotten`; `pkg/sema/bounds_check.go` skips invalidated names and
`leaveArrayScope` does not restore them. A fresh literal declaration clears
the invalidation. Regression: `TestPhase13_ReassignmentForgetsLiteralSize`
and `TestPhase13_LoopReassignmentSurvivesBlockRestore`, plus the real
`TestPhase133_GoldensGoEngine`/`CheckClean` gates, all PASS.

## Record

This is recorded as a corrective change against the frozen baseline per the
Governance rule in `AGENTS.md`: owning layer identified (`pkg/sema`), failure
demonstrated (false positive on a parameter and on a loop-grown array),
downstream behavior preserved (no codegen change), and the affected regression
gates run.
