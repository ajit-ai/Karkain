# Phase 103 — Module System v2 (Core) — Final Report

## Objective

Turn "modules are just sibling files" into a real module contract: a module
exports exactly the declarations it marks `public`, other modules reach them
through qualified names, and cross-module misuse is rejected with precise
diagnostics — on BOTH engines (Go front end and self-hosted kcc).

Scope delivered (user-approved "Core module system"):
- export sets: `public` is a real export modifier on `func`/`type`/`enum`
- qualified-name resolution: `math.twice(21)` binds to the `math` module's
  public export, not a silent flatten
- public/private cross-module visibility enforcement with K-diagnostics
- `public` accepted by the self-hosted kcc engine (parse + codegen)
- two-module acceptance program + rejection fixtures on both engines
- stdlib private-export exemption

## Design

### Qualified calls stay qualified in the AST; codegen stays flat

`CallExpr` gained a `Module` field (`pkg/parser/ast.go`). The parser keeps the
qualifier (`Module: "math"`, `Function: "twice"`) for module-qualified calls in
both expression and statement positions, so the resolver can validate the
target against the module's export set. The flat `C`-symbol namespace is
untouched: codegen emits the bare user symbol (`karkain_user_twice`), identical
for `math.twice(21)`, a record-idiom receiver call `acc.deposit(10)`, and a
bare `twice(21)` call. `C.*` stays dotted for C-interop (Go codegen trims the
`C.` prefix per its namespace contract). Macro expansion (`pkg/parser/macro.go`,
`pkg/sema/macro.go`) carries `Module` through clones so expanded programs keep
qualifiers.

### Module resolution (Go resolver, `pkg/sema/resolve.go`)

`NewResolver` indexes the compile unit's files by basename
(`indexModules`), records declared `import` names (dotted `std.string` maps to
the file whose basename is the last segment), and `checkQualifiedCall` enforces
the contract in order:

1. module must be part of the compile unit — cleared by validateImports against
   `moduleFile` (plain and `std.*` dotted imports);
2. module must be imported (`import math` present) — receivers (locals/params)
   with a qualifier are record-idiom method calls and skip the module model;
3. the function must exist;
4. the function must be declared in the module's own file (wrong-module target);
5. the function must be `public` — except stdlib files, which are exempt
   (`isStdlibFile`): stdlib functions are framework API surface and physical
   `public` markers land with the stdlib-v2 boundary (Phase 109).

Diagnostics (all exit 3 on `karkain check`): `function 'x' in module 'm' is
private and cannot be called by another module`, `module 'm' is not imported;
add 'import m'`, `function 'x' is not defined`, `function 'x' is not exported
by module 'm' (defined in '...')`, plus the existing flat-unit duplicate
diagnostic for cross-module collisions.

### Self-hosted kcc parity (`src/compiler/`)

- `src/compiler/ast.kark`: optional public slots on FuncDecl (index 7, index 6
  reserved for the `@target(...)` attribute, padded when unset), StructDecl and
  EnumDecl (index 4), with `setFuncPublic`/`funcPublic`/`structDeclPublic`/
  `enumDeclPublic`.
- `src/compiler/parser.kark`: a `TK_PUB` branch before `func`/`type`/`enum`
  mirrors Go's parse contract — `public let x = 1` is a parse error on BOTH
  engines ("unexpected token after 'public' modifier (expected
  func/type/enum)").
- Qualified-call lowering in `parseIdentOrCall`: dotted calls emit the bare
  right-hand name (`math.twice(21)` and `acc.deposit(10)` both become the plain
  callee), so the flat `karkain_user_*` emission and user-function registration
  (forward declarations over the single assembled AST) find the real symbols;
  `C.*` keeps its dotted name.
- kcc's type checker is intentionally conservative: module export-set
  validation lives in the Go resolver (which also gates `--engine kcc` check/
  build/run via the shared assembly). kcc-side regression behavior is verified
  end-to-end by the acceptance golden + compiler self-check.

## Acceptance

`examples/module_system/` — `main.kark` imports the sibling `math` module and
calls its public exports qualified (`math.twice(21)`, `math.square(3)`);
`math.kark` also carries a private `secret` helper. Output on both engines:

```
42
9
```

`examples/module_system_errors/` — four cross-module violation fixtures
rejected by the Go engine with exit 3: `main_private.kark` (private export),
`main_unimported.kark` (missing import), `main_wrongmodule.kark` (undefined
module function), `main_duplicate.kark` (cross-module name collision).

## Verification (isolated runs on this host)

| Gate | Result |
|------|--------|
| New `pkg/cli/phase103_module_test.go`: Go engine golden | PASS 4.8s |
| New gate: kcc engine golden (isolated kcc) | PASS 10.6s |
| New gate: kcc accept-check of the module program | PASS 10.0s |
| New gate: 4 rejection fixtures (exit 3 + expected diagnostics) | PASS |
| New gate: compiler sources self-check under kcc | PASS 41.9s |
| `pkg/sema` — 7 new module tests (qualified public/private/wrong-module/undefined/not-imported/receiver-skip/dotted-std) + stdlib exemption | PASS |
| `pkg/sema/...` full, `pkg/parser/...` full, `pkg/codegen/...` full | PASS |
| `go vet ./pkg/... ./cmd/...` | clean |
| Phase 97 parity (incl. local-dep-through-kcc) | PASS 10.7s |
| Phase 99 self-hosted (CorpusAccept / CrossFileDuplicate / ErrorFixtures 14/14 / CompilerSourcesTypeCheck) | PASS 50.3s |
| Phase 100 runtime-error parity + valid-arithmetic parity | PASS 28.2s + 3.3s |
| Phase 101 stack-chain parity | PASS |
| Phase 102 Go foundation golden + KCC foundation golden | PASS 28.5s + 15.7s |
| Conformance corpus, Go engine | PASS 59/59 (116.9s) |
| `karkain fmt --check` on all new corpus files | clean |
| Legacy module E2E tests updated to `public` exports (assembly/isolation intent unchanged) | PASS |

## Deviations from the approved option text — two, both documented

1. **`public` on `let`/`var` was NOT implemented.** Go's parser rejects
   `public let x = 1` with a parse error, and this phase deliberately kept the
   two engines' parse-error contracts identical instead of adding a third
   exportable kind and its sema. If module-level variables are desired, they
   belong with Module System v2.1 (or a value-export phase), on both engines in
   one step.
2. **No physical `public` markers were added to the 291 stdlib functions.**
   Stdlib never enters a resolver compile unit today (its 8 modules are loaded
   through the module graph, not sibling assembly), so markers would have zero
   functional effect. The semantic exemption (`isStdlibFile`) is implemented
   and tested so stdlib remains callable when stdlib does get assembled into
   check units. The physical markers land with the stdlib-v2 boundary (Phase
   109), matching the existing GMP/map/option boundary.

## Deferred / out of scope (Module System v2.1 = Phase 103-B, not yet scheduled)

- re-exports (`import math.expose ...`), aliasing (`import math as m`)
- `public let/var` value exports (see deviation 1)
- cross-module qualified access to `type`/`enum` exports beyond the parser
  flag (Go parser already records `.Public` on struct/enum; visibility checks
  for *construction* of private types are conservative — no false positives)
- self-hosted module-resolution diagnostics parity (kcc check stays
  conservative; Go resolver gates both engines' CLI pipelines)

## Files

- `pkg/parser/ast.go` — `CallExpr.Module`; `pkg/parser/parser.go` —
  module-qualified parsing (expression + statement positions),
  `public` unchanged (already `.Public`); `pkg/parser/macro.go`,
  `pkg/sema/macro.go` — `Module` carried through expansion
- `pkg/sema/resolve.go` — `indexModules`, `moduleBasename`, `moduleFile`,
  `checkQualifiedCall`, `recurseArgs`, `isStdlibFile`,
  `imported` tracking
- `pkg/sema/resolve_test.go` — 7 new module-resolution tests + stdlib
  exemption
- `src/compiler/ast.kark` — public slots (FuncDecl idx 7 / Struct-Enum idx 4);
  `src/compiler/parser.kark` — `TK_PUB` branch + qualified-call lowering
- `examples/module_system/` (acceptance), `examples/module_system_errors/`
  (4 rejections)
- `pkg/cli/phase103_module_test.go` — the gate
- `pkg/cli/module_e2e_test.go` — two legacy tests updated to `public` exports