# KARKAIN NEXT IMPLEMENTATION PHASE RECOMMENDATION

The prompt requires exactly ONE next implementation phase, based on repository evidence.

```
NEXT_PHASE_ID:  56-C (a corrective sub-phase before the planned Phase 56/57+)
NEXT_PHASE_NAME: Parser + Semantics Correction (Foundation Stabilization)

CURRENT_PREREQUISITES:
- All packages build (go build ./... = OK) and veto clean (go vet = clean).
- Canonical tests pass: ./pkg/lexer ./pkg/parser ./pkg/codegen ./pkg/pm (all ok).
- Other packages pass: sema, diagnostics, lsp, jit, ir, ir/ssa, runtime, runtime/gpu (ok).
- KNOWN FAILURES: pkg/cli TestCodeGenProfile → parser OOM; pkg/bootstrap Stage1 transpile → parser OOM.
- Root cause identified (evidence): parser.go:973 parsePrimaryExpr returns nil WITHOUT advancing
  the token; function-call arg loops parser.go:1148-1153,1190-1195,601-606,619-624 append
  p.parseExpr() with NO no-progress guard → unbounded growslice (~3GB) on large input.

OBJECTIVE:
- Make the parser terminate on ALL inputs (no infinite loops / no OOM) as the correctness
  foundation. This unblocks: TestCodeGenProfile, pkg/bootstrap Stage1, and Phase 56 self-hosting.
- Add the missing parser unit tests so this class of bug cannot ship again.

FEATURES:
- Add a no-progress guard to every argument/statement collection loop in the parser
  (break if parseExpr/parseStmt did not advance the token; emit a parse error instead of hanging).
- Validate all four arg loops (parser.go 1148-1153, 1190-1195, 601-606, 619-624) + the
  parseComptimeStmt and parseMatchExpr arm loops for the same pattern.
- Ensure `nil` expressions can never be appended in a loop without consuming input.
- Regression test: parse the concatenated src/compiler sources (the TestCodeGenProfile input)
  and assert it terminates with a bounded time/memory instead of OOM-ing.

NON_GOALS:
- NO new language features.
- NO parser redesign. NO new IR layers.
- NO refactor of existing working emitters.
- NO changes to the memory model, type system, or codegen in this phase.

DEPENDENCIES:
- None in the Go toolchain; only the existing parser package.
- Serves as the prerequisite for Phase 56 (self-hosting) and all later semantic work.

FILES_EXPECTED_TO_CHANGE:
- pkg/parser/parser.go (add progress guards + error reporting on stalled loops)
- pkg/parser/parser_test.go (NEW — the missing dedicated parser unit test)
- Possibly pkg/cli/codegen_profile_test.go (assert termination) and pkg/bootstrap (un-block Stage1)

TEST_REQUIREMENTS:
- NEW parser unit tests covering: function-call with unexpected token inside args,
  malformed call lists, dot-call chains with errors, and the large concatenated-compiler input.
- Re-run: go test ./pkg/lexer/... ./pkg/parser/... ./pkg/codegen/... ./pkg/pm/... -count=1
- Re-run: TestCodeGenProfile must pass (not OOM); pkg/bootstrap Stage1 must no longer OOM.

ACCEPTANCE_CRITERIA:
- Parser terminates on all inputs; no parse produces OOM.
- TestCodeGenProfile passes.
- pkg/bootstrap Stage1 transpile no longer hits parser OOM (even if later stages reveal other
  self-host incompleteness).
- go build ./... + go vet clean; full test suite green.

RISKS:
- Correctly identifying every stalled loop (risk of a remaining one elsewhere). Mitigate with
  the new parser_test.go fuzz/termination tests.
- Changing parse behavior: only ADDs a break + error (SAFE per Backward-Compat audit); valid
  programs unaffected.
- Scope creep into "just also fix generics parsing" — explicitly NON_GOAL; keep this phase surgical.
```

## Why this phase (not the planned Phase 56/57)
- The ROADMAP lists Phase 56 (self-hosting) next, but self-hosting is **blocked by** this exact
  parser OOM (Stage1 transpile of src/compiler/main.kark fails). Fixing the parser is the
  true prerequisite; attempting Phase 56 without it is unfixable.
- It is the single highest-leverage, lowest-risk action that unblocks the most downstream work
  (self-hosting, TestCodeGenProfile, and any large-input robustness the type-checker foundation
  will need).
- It honors "small at the core": a correct, terminating parser is core, and it carries zero
  speculative features.
