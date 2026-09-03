# PHASE 51 — Borrow Checker Lexical Scoping

**Status:** COMPLETE
**Result:** All regression tests green; scope model hardened for shadowing correctness.
**Branch:** `develop` → merged to `main` → pushed to `origin` (per AGENTS.md).

---

## 1. Scope & Goals

Make the borrow checker's scope model fully lexical and correct under shadowing:

- Scope stack entry/exit is honored for every lexical construct.
- Inner bindings **expire** when their scope exits; the outer binding with the same
  name is restored at each exit.
- A use of a name whose binding lifetime already ended (with **no live shadow**)
  is **diagnosed**, not silently accepted.
- Borrows expire when their scope exits.
- References must not be usable beyond their referent's scope (Phase 63 lifetime
  tracking deferred — see §6).
- Move semantics persist across scope exit (moves are never undone by a scope pop).
- BUG-5 (permanent move / double-move-across-scopes) is preserved.
- Escape-analysis markings (Phase 49, `VarDeclStmt.Escapes`) are consumed by the
  borrow checker.

Constraints honored: no GC, no new syntax, no closures, no IR redesign, no Phase
54/63 semantics, reuse of existing per-scope identity model, smallest coherent
change.

---

## 2. Investigation Findings

The existing `pkg/sema/borrow_checker.go` already had a working scope stack
(`scope.parent`, `pushScope`/`popScope`), per-scope `varEntry` maps, and a
`borrows` reversion list. That model passed every pre-existing borrow-checker
test, including a pre-existing "Phase 51: Lexical scoping tests" section.

**Remaining correctness gaps found during investigation:**

1. **Imprecise borrow reversion under shadowing.** `popScope` reverted borrows by
   `name` via the parent-traversing `lookup` (line 79-85 in the original). When a
   name is shadowed, this could resolve to the *wrong* (outer or restored) entry
   instead of the exact binding that was borrowed.
2. **Use-after-scope-end was silently accepted.** `checkIdentifierUse` returned
   early when `lookup` returned `nil`, so an identifier referencing a dead inner
   binding (e.g. `{ let x = 42 } print(x)`) produced **zero errors**.
3. **Escape analysis was disconnected.** `VarDeclStmt.Escapes` (Phase 49) was set
   by the parser but never recorded by the borrow checker.

These gaps correspond directly to Phase 51 Test Groups C (inner binding expires)
and the requirement to integrate escape analysis.

---

## 3. Implementation

All changes are confined to `pkg/sema/borrow_checker.go` (+ regression tests in
`borrow_checker_test.go`).

### 3.1 Precise binding identity for borrow reversion

- `varEntry` gains `declScope *scope` — the exact lexical scope that declared the
  binding.
- `borrowRecord` gains `declScope *scope` — the declaring scope of the borrowed
  binding, captured at borrow time.
- `popScope` now reverts each borrow by resolving the binding through its recorded
  `declScope.lookupOwn(name)` rather than a parent-traversing `lookup`. A shadowed
  name therefore reverts the *correct* binding and can never corrupt an outer or
  restored entry.
- `declare` sets `varEntry.declScope = bc.scope` and records the name in
  `scope.decls`.

### 3.2 Use-after-scope-end diagnostic (Test Group C)

- `scope` gains `decls []string` (names introduced by this scope) and
  `dead map[string]bool` (names whose binding lifetime ended at this scope).
- On `popScope`, every name in the popped scope's `decls` is marked dead in the
  parent scope, and usage counts still propagate as before.
- `checkIdentifierUse`: when `lookup` returns `nil` (no live binding) **and**
  `isDeadName(name)` reports the name in the current scope chain, a new error is
  emitted: `use of value '<name>' after its scope has ended`.
- `isDeadName` walks the current scope chain checking each scope's `dead` map.
  Because it only fires when *no live binding* resolves, existing valid programs
  (which resolve to a live outer/shadow binding) are unaffected.

### 3.3 Escape-analysis integration

- `varEntry` gains `escapes bool`.
- `checkVarDecl` records the parser's `VarDeclStmt.Escapes` (Phase 49) onto the
  declared entry, closing the loop so the borrow checker records whether a value
  outlives its declaring scope. This is a one-way, non-circular dependency
  (parser → AST → borrow checker).

### 3.4 Preserved semantics (unchanged)

- `checkMove` still sets `ownership = Moved` **without** a borrow record, so moves
  persist across `popScope` and are never undone at scope exit. This preserves
  BUG-5 and Rust-like move semantics.
- Borrow expiry, shadow restore, ownership reset on re-shadow, and per-construct
  scoping (if/while/for/for-in/match-arm/lambda) are all unchanged.

---

## 4. Regression Tests (Groups A–I)

Appended to `pkg/sema/borrow_checker_test.go` as `TestPhase51_*`:

| Group | Test | Verifies |
|-------|------|----------|
| A | `ScopeEntryExit` | inner bindings usable inside their scope |
| B | `OuterBindingSurvives` | outer binding restored after shared-name inner scope |
| C | `InnerBindingExpires` | using a dead inner binding → `scope has ended` diagnostic |
| D | `ShadowDistinctIdentity` | shadow is a distinct binding; outer unaffected |
| E | `NestedShadowThreeLevels` | 3-level shadowing restores correctly at each exit |
| F | `BorrowExpiryAllScopes` | borrows expire at scope exit across constructs |
| G | `ReferenceOutlivesReferent` | pins current behavior (see §6) |
| H | `MovePersistsAcrossScope` | a moved value stays moved after scope exit |
| I | `Bug5DoubleMoveAcrossScopes` | BUG-5 regression: exactly one error, move persists |

---

## 5. Validation

All green except the **documented pre-existing bootstrap failures** (independent of
`pkg/sema` — they self-host/recompile and perform bitwise SHA checks):

- `pkg/sema` — PASS
- `pkg/codegen` — PASS
- `pkg/cli` — PASS (E2E)
- AGENTS.md suite (`lexer`, `parser`, `codegen`, `pm`) — PASS
- `pkg/...` full run — FAIL only in `pkg/bootstrap` (pre-existing: bitwise SHA
  mismatch, stage1/stage2 self-hosting, `TestArgs_*` environment issues).

Bootstrap/test-only failures are environmental and unrelated to this phase; the
borrow-checker Go code path is fully green.

---

## 6. Known Limitations / Deferred

**Reference outlives referent (Test Group G).** The current model does not track
full binding-to-binding reference lifetime (a reference `r` stored in an outer
scope pointing at a referent that dies in an inner scope). Full lifetime/borrow
region tracking is inherent to the Phase 63 ownership model and is deliberately
scoped out here. Group G pins the current documented behavior so future deltas
surface in CI. Uninitialized-`let` + cross-scope reference assignment with the
required lifetime check is not supported by current Karkain syntax and is likewise
deferred to Phase 63.

---

## 7. Diff Summary

```
pkg/sema/borrow_checker.go       | 65 +++++++++++++--  (scope/identity/dead/escape)
pkg/sema/borrow_checker_test.go  |174 ++++++++++++++++  (Groups A–I)
```