# PHASE 133 — First-Class `fn` Values — FINAL REPORT

- **Date**: 2026-09-21
- **Milestone**: GA-2 Language & Tooling Completeness (owner priority track)
- **Verdict**: **COMPLETE** — closures flow (params, arrays, returns) and
  invoke through values, byte-identical on both engines; escaping captures
  rejected at check; full regression green.
- **Baseline**: Phase 130 (capture-mutation; two documented boundaries).

---

## 1. Baseline matrix (both engines agreed before the phase)

| Shape | Before | After (both engines) |
|---|---|---|
| `f(5)` call via let-bound var | worked (`6`) | works, unchanged golden |
| `ops[0](5)` array-index call | K001 parse error | works (`6`) |
| `apply_twice(inc, 10)` higher-order param | check ok, C-compile fail | works (`22`) |
| `func mkadder(n int) fn` + call | K001 parse error | **rejected at check** (see §3) |
| non-capturing factory | K001 (return-type `fn`) | works (`11/21`) |

## 2. Root causes (one per layer, verified by probe)

1. **Parser**: call callees were identifier-only (`CallExpr.Function string`);
   `(` after any other expression was a K001. `fn` lexes as keyword token,
   so `fn` annotations/return types were silently dropped or misaligned.
2. **AST**: no node for computed callees on either engine.
3. **Codegen**: bare lambdas emitted unusable nested-function names;
   let-bindings declared no variable; params/aliases had no callable form.
4. **Runtime**: no function Value kind; no indirect-call helper.
5. **Semantics**: escaping captures (by-ref into a dying frame) produced
   silent wrong results (factory printed `0` instead of `8`) — the worst class.

## 3. Design (Karkain-owned; nothing imported)

- New AST: `IndirectCallExpr{Target, Args}` (Go) / node 96 `IndirectCall`
  (kcc). Identifier callees keep every existing static path untouched.
- Runtime: `TYPE_FUNC` cell `{canonical wrapper, heap env}` +
  `karkain_call_fn` dispatch (non-function/arity → file:line runtime
  error, exit 1, Phase 100 model). Print shows `<fn>`; equality is
  code+env identity; truthy.
- Wrappers: one static `karkain_fncall_X` per let-lambda in the uniform
  `(env, argc, argv, file, line)` ABI; arity enforced with diagnostics.
- Binding: heap env per execution + global-holder compat + `Value` cell
  declaration. Direct calls dispatch through the cell except self-reference
  (no cell variable exists there) — this makes rebinding/recursion correct
  (countdown golden `300`; static-holder dispatch printed `0`).
- Calls through locals dispatch (a local can hold nothing callable today,
  so no working program changes meaning — proven by full regression).
- Captures stay by-ref. **Returning a closure that captures function-local
  state is rejected at check** (Go `error[K002]` / kcc `error[K114]`,
  exit 3): direct, alias, array-contained, and block/match-arm shapes all
  covered; call results (unknown provenance) allowed — documented gap.
- `func(T) R` full signatures deferred honestly (bare `fn` suffices; arity
  is runtime-enforced). WASM rejects indirect calls with `error[K108]`.

## 4. Vacuity-class defects found and fixed during the phase

1. **kcc node-id collision**: `NODE_INDIRECTCALL` first took 39, already
   owned by `NODE_LET_DECL` — every indirect call read as LetDecl
   (caught by KIR dump showing `print (expr <LetDecl>)`). Moved to 96
   after a full id census.
2. **kcc checker segfault** (`0xC0000005`): skipping the locals-reset kept
   appending into the caller-aliased array; a later realloc left the saved
   restore pointer dangling. Fixed by always resetting to a fresh array
   and COPYING outer names in (reads only). Root-caused via trace
   instrumentation, not guessing.
3. **Dead `"Binary"`/`"Unary"` tag strings** on kcc (getNodeType spells
   `"BinaryExpr"`/`"UnaryExpr"`): checkExpr never checked binary operands;
   the struct-literal arm had the same defect, which — once checkExpr was
   fixed — misread field keys as value reads (K102 on every struct
   field). All three sites fixed; pre-existing gap documented.
4. **Wrapper use-before-def** for nested lambdas (registration order):
   fixed with uniform-signature prototypes ahead of defs.
5. **Prescan misses**: assignment-nested lambdas (same dead-string cause).
6. **kcc capture rewrite**: dispatch through a captured name must read
   `(*_env->name)` (no C variable exists in nested defs); factored into
   one helper used by both call branches.
7. **Phase 121/130 marker updates**: static-init and static-call spellings
   replaced by heap/cell/dispatch spellings (intent preserved).

## 5. Gates (all green, measured live on this host)

- `pkg/cli/phase133_closures_test.go` 6/6: Go goldens, kcc goldens
  (live, no skip), check-clean both engines, escape rejection both
  engines (K002/K114 + exit 3, direct + alias), pure-factory goldens,
  runtime-error negatives (message + exit 1, both engines).
- Corpus `examples/closures/02_higher_order` (`11/22/6`),
  `03_array_call` (`15/10/20`), `04_factory` (`101/102/21/21/21`),
  `05_recursion` (`300`) — byte-identical Go↔kcc, check-clean both.
- Phase 130 boundary tests retired into promotion guards (goldens 15, 21).
- Phase 117 explain lists extended with K114.
- Full regression: every `pkg/...` suite green (units, all CLI phase
  gates 88–133, conformance 59/59, probes, KIRContinuity at re-pinned
  7416 = 7416 verified); `go build ./...`, `go vet` clean; Sphinx
  unaffected (no `docs/source` changes this phase).

## 6. Honest boundaries (NOT defects)

- Capturing factories rejected (soundness over ambition; §3).
- Loop-body lets share one C slot per iteration: post-loop reads see the
  last value (`21/21/21` pinned identically on both engines).
- Self-recursive capturing closures inside rebinding functions keep
  holder semantics (four-condition corner, documented).
- `func(T) R`, capture snapshots, WASM indirect calls: Planned.
- kcc diagnostics report assembly-relative lines (pre-existing quirk,
  verified identical class on untouched K102).
- Probes must run from litter-free directories (sibling-join assembles
  same-dir `.kark` files into one unit; pre-existing CLI behavior).

## 7. Files

Go: `pkg/parser/{ast,arena,parser,macro,captures,escape}.go`,
`pkg/sema/{resolve,macro,borrow_checker,unused,kernel_analyzer}.go`,
`pkg/codegen/{codegen,conc_runtime,native}.go`,
`pkg/codegen/{phase121_correctness,phase130_closures}_test.go`,
`pkg/ir/hir` (untouched — legacy path forced),
`pkg/wasm` (untouched — auto-K108),
`pkg/cli/{phase133_closures_test.go,phase130_closures_test.go,
phase117_beta_test.go,phase122_pipeline_ownership_test.go (KIR pin),
explain.go}`.
kcc: `src/compiler/{ast,parser,checker,sema,codegen,kir}.kark`.
Corpus: `examples/closures/{02,03,04,05}.kark`. Docs: `SPEC.md` §3.7,
this report. No `func(T) R`, no ownership-model changes.

**Next: Phase 134 (Incremental Compilation v2) per owner plan.**
