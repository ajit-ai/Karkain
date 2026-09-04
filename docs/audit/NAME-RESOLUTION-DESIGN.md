# Karkain Whole-Program Name Resolution + Module/Visibility — DESIGN PROPOSAL

Status: **PROPOSAL — requires sign-off.** This is a language-design decision, not
a wiring change. Nothing here is implemented yet.

## 1. FINDING

Karkain has **no name resolution, no module scoping, and no visibility model.**
A compile unit is formed by concatenating `.kark` files into one flat global
namespace; function/variable references are emitted as bare C identifiers and
resolved only by the C linker.

## 2. EVIDENCE (current state)

- Calls are emitted verbatim with no lookup: `pkg/codegen/codegen.go:2314-2328`
  (`return fmt.Sprintf("%s(%s)", n.Function, ...)`).
- Variables emitted as-is: `pkg/codegen/codegen.go:2090-2091`
  (`return n.Name`).
- The only "resolution" is a forward-declaration pass over **every** `FuncDecl`
  in the concatenated AST: `pkg/codegen/codegen.go:99-114`.
- No duplicate-definition detection at the Karkain level. Two files defining
  `func foo` yield two C bodies → GCC "multiple definition" linker error
  (the compiler never reports it). No `funcDecls` map in `Generator`
  (`pkg/codegen/codegen.go:31-46`).
- `Program.Statements` is a flat list; no module/file/package wrapper
  (`pkg/parser/ast.go:7-11`). Borrow checker's scope stack is for
  borrow/ownership only, invisible to codegen (`pkg/sema/borrow_checker.go:40-51`).
- Self-hosting compiler concatenates siblings the same way
  (`src/compiler/main.kark`, `func loadSourceWithSiblings`); no imports/modules.
- Lexer/parser: `import` exists but ONLY for C interop
  (`pkg/lexer/lexer.go:486-487`, `pkg/parser/parser.go:1548`). `pub`,
  `private`, `module`, `mod`, `export` are **not** keywords.

The package-manager→compiler gap is now closed (build/run/check/test all
project-aware), which makes a proper name table over the resolved unit the
natural and highest-leverage next step.

## 3. Constraint: must not break self-hosting

`src/compiler/*.kark` use a flat global namespace with no visibility
annotations. Any feature must be an **opt-in additive** one (old files compile
identically); the self-hosting cycle (stages 1–3, bitwise-identical) must stay
green.

## 4. OPTIONS

### Option A — Hard `pub`/`private` visibility (full module/import language)
Add `pub` keyword; default-private per file; `import <pkg>` inside files; a
whole-program pass builds a name table, resolves references to imported
definitions, enforces visibility, detects duplicates/undefined references.
- Pros: full module system; enables registry/git deps cleanly.
- Cons: large; new syntax; risk to self-hosting (would require annotating every
  cross-file call in `src/compiler`); the biggest change.

### Option B — Name-table + duplicate/undefined diagnostics ONLY (no new syntax)
Add a whole-program pass that builds a symbol table over the resolved unit and
**reports duplicate-function and undefined-reference errors at the Karkain level**
(instead of leaking GCC linker errors), but keeps the flat global namespace and
adds **no** `pub`/`import` syntax. This is the V1 MUST-HAVE "whole-program
name-resolution pass", done minimally.
- Pros: small, safe, additive; no new syntax; self-hosting unaffected; gives
  real diagnostics today (the immediate correctness/UX win); foundation for B.
- Cons: does not by itself give modules/visibility.

### Option C — Minimal `pub` for the SELF-HOSTED COMPILER only
Introduce `pub` semantics but apply them only inside `src/compiler` (which
becomes the first "package"); the rest of the language stays flat.
- Pros: validates the visibility mechanism on the real self-hosting oracle.
- Cons: bifurcates the language; confusing; not a general solution.

## 5. RECOMMENDATION

**Phase 1 (this phase): Option B** — implement the whole-program name-resolution
pass as a **diagnostics-only** addition: build a symbol table over the resolved
compile unit, detect duplicate top-level definitions and undefined function
references, and emit clear Karkain-level errors. No new syntax. Self-hosting
unaffected (it is currently valid; the pass only *adds* errors that already
manifest as GCC failures today).

This directly satisfies the V1 MUST-HAVE "whole-program name-resolution" that has
been the documented gate, with minimal risk and immediate diagnostic value, and
it builds the exact machinery (a single symbol table over the resolved unit)
that a later `pub`/`private`/`import` module system needs.

**Phase 2 (future, sign-off separately): Option A** — layer `pub` visibility +
`import` on top of the Phase 1 table, converting `src/compiler` to annotated
packages only after the table proves correct.

## 6. RISKS

- False positives: the pass must account for the current loose model (arbitrary
  global names) so it does not reject today-valid programs. Mitigation: gate
  strictly on true duplicates and references to *defined* functions; run the full
  suite + self-hosting.
- Order sensitivity: undefined-before-defined is legal today (forward
  declarations). The table must be two-pass (collect, then validate) to avoid
  ordering false errors.
- Concatenation collisions from dependency names: with deps now concatenated,
  two deps defining the same helper would become a real conflict. The pass must
  report it clearly (today it is a GCC error).
- Self-hosting byte-identity: the pass must be **purely diagnostic** (never
  alters emitted output), so stage2 == stage3 is preserved.

## 7. Decision needed

I recommend **Option B for this phase** (diagnostics-only name-resolution pass,
no new syntax), deferring Option A (pub/private/import) as a separate sign-off.
Options A, B, C are mutually exclusive for the module feature; choosing neither
means stopping at the current (GCC-deferred) resolution model.

## 8. NEXT ACTION (if approved)

Inspect the CLI command entry points + runSingleTestFile/findTestFiles and the
full-suite harness, then implement `pkg/sema/resolve.go` (two-pass name table),
wire it into CheckCommand/build/run, add unit tests, keep it diagnostic-only,
and run the full suite + bootstrap to confirm no regression.