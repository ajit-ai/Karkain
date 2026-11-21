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

## Record

This is recorded as a corrective change against the frozen baseline per the
Governance rule in `AGENTS.md`: owning layer identified (`pkg/sema`), failure
demonstrated (false positive on a parameter), downstream behavior preserved
(no codegen change), and the affected regression gate run.
