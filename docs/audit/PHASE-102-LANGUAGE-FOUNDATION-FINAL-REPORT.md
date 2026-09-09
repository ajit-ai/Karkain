# Phase 102-F — Language Foundation Completion — Final Report

## Objective

Round out the baseline Karkain language surface to a usable end-to-end
foundation on BOTH engines (Go front end and self-hosted `kcc`) and pin it
with a golden-output regression corpus:

variables -> types -> functions -> recursion -> arrays -> strings ->
collections -> control flow -> structs -> methods (record idiom) -> modules
(sibling assembly) -> application-level multi-module projects -> error
handling.

No new syntax was invented: every example uses only the syntax the compiler
already accepted; the work removed engine asymmetries that made existing
syntax unusable.

## What Fixed / Now Works

Cross-engine parity bugs found and fixed while validating the corpus:

1. **kcc struct `;` field separators** — `src/compiler/parser.kark`
   `parseStructDecl` only skipped `,` separators between fields, so
   `{ owner string; balance int }` swallowed the source until the next `}`
   and silently dropped every subsequent top-level function. This was the
   root cause of the `application` example losing `interest`/`deposit`/
   `formatAccount` and then mis-handling `deposit()` ("deposit drop").
   Fixed by skipping both `,` and `;`.

2. **Go BorrowChecker poisoned the global scope** — `pkg/sema/borrow_checker.go`
   `popScope` marked every dead name in a function's root scope, re-marking
   function parameters as "dead" in the outer/global scope even though the
   function has exited. A later struct literal keyed with the same name
   (e.g. `Counter{ value: 0 }` after a `value` parameter) was false-flagged
   "use of value 'value' after its scope has ended". Fixed with a `fnRoot`
   scope flag: function/lambda root scopes stop dead-name propagation to the
   outer scope while keeping block-level propagation and usage counting.

3. **kcc C-style `for`** — empty-init `for (; k < 3; ...)` emitted `for (`
   with no `;`, and `for (let i = 0; ...)` could not be parsed. Fixed in
   `src/compiler/parser.kark` (`parseFor` accepts `let`/`var` init via
   `parseVarDecl`) and `src/compiler/codegen.kark` (`"; "` when init is empty;
   condition wrapped in `is_truthy(...)`).

4. **Go `for` with `let` init** — `pkg/codegen/codegen.go` `genForStmt` emitted
   a double `;;`. Fixed by trimming the init statement's terminating `;`.

## Verification (all run on this host, isolated runs)

| Gate | Result |
|------|--------|
| New `pkg/cli/phase102_foundation_test.go`: Go engine, 13 golden targets | PASS 44.6s |
| New gate: kcc engine, same 13 golden targets (isolated kcc) | PASS 30.5s |
| New gate: compiler sources self-check under kcc (Phase 99 gate survived) | PASS 55.6s |
| Phase 99 self-hosted parser + type checker (CorpusAccept / CrossFileDuplicate / ErrorFixtures 14/14 / CompilerSourcesTypeCheck) | PASS 58.8s |
| Phase 100 runtime-error parity (7 fixtures + positive control) | PASS |
| Phase 101 stack-chain parity + normal-run-has-no-stack | PASS |
| `pkg/codegen` Phase 100 emission tests | PASS |
| `pkg/sema` Phase 51 borrow-check tests (after `fnRoot` change) | PASS |
| `go vet ./pkg/cli ./pkg/sema ./pkg/codegen` | clean |
| `karkain fmt` on examples is semantic-preserving (single- and multi-file) | PASS |

All 13 examples under `examples/language_foundation/` produce byte-identical
output on both engines including the multi-file `modules/` and `application/`
projects.

## Capability Summary (what Phase 102-F locks in)

See `SPEC.md` section 14 (Capability Summary) for the full table. Highlights:

- Variables, int/float/bool/string literals, arithmetic, comparisons — both engines
- Functions, parameters, returns, recursion — both engines
- Arrays (push/pop/get/set), strings (concat/index/length/prefix) — both engines
- `if`/`elif`/`else`, `while`, C-style `for` incl. empty init, `for (let i = ...)` — both engines
- `match` over int and string — both engines
- Maps (get/set/has, default handling) — both engines
- Structs (`type T struct { ... }` with `,` and `;` separators) — both engines
- Methods via the record-as-first-argument idiom — both engines
- Multi-file modules (kcc sibling assembly) and application-style layout — both engines
- Runtime error diagnostics (div/mod by zero, OOB) and stack traces — both engines
- NOT supported on either engine (kept out of the corpus, engine-parity):
  `float64()`/`bool()`/`string()` casts; closures/`fn` codegen; `const`;
  user `import`; visibility enforcement.

## Environmental Note

The 4 GB-RAM / limited-paging host still OOM-stalls or accesses-violation
crashes when `kcc` compiles the full self-hosted compiler sources in build
mode (a known, documented class of limitation — already noted for the
bootstrap identity gates). The in-scope verification for this phase is the
`check` path, which is fast and low-memory and passes clean; a full bitwise
bootstrap re-verification on this host is deferred (to be re-run on a
higher-RAM machine).

## Commit Hash

(To be filled on final commit of Phase 102-F.)