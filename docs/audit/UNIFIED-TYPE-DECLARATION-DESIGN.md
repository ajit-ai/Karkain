# Unified Type and Variable Declaration Design

**Status: DESIGN PROPOSAL - not approved, not implemented.**
**Owner decision required.** Written 2026-10-15 as Workstream D of the master
plan. Writing this document does not approve it (control-system Rule 4).

Design only. No parser, AST, checker or codegen change is made or authorised by
this document. Implementation requires a subsequent explicitly authorised slice
with its own baseline note.

## 1. Current behaviour, as measured

Evidence, not recollection. Every claim below was read out of the repository on
2026-10-15 at `develop` = `024ca94`.

### 1.1 Declaration forms actually parsed

There is **one** declaration path, not three. `let`, `var` and `const` are all
parsed by `parseVarDecl` (`pkg/parser/parser.go:441`), which branches only on
the keyword to set `isConst`. The three keywords therefore produce the *same*
node type:

```go
// pkg/parser/ast.go - VarDeclStmt
type VarDeclStmt struct {
    Name     string
    Value    Node
    Type     string // Optional type information (e.g. "*int", "[4]f32")
    Const    bool   // Phase 112: true for `const` (immutable binding)
    IsMatrix bool
    IsSIMD   bool
    Align    int
    Escapes  bool
    ...
}
```

Three consequences follow immediately, and they shape the whole design:

1. **The type annotation is a string.** `Type string` means type identity today
   is textual. There is no `Type` value, no unification and no lattice - which is
   exactly what `docs/audit/TYPE_SYSTEM_AUDIT.md` recorded and never stopped
   recording.
2. **`let` does not mean immutable.** Only `const` is immutable (Phase 112
   rejects `MAX = 1` with "cannot reassign constant"). Phase 112's own gate
   states the intent explicitly: it leaves "regular let/var reassignment
   untouched" on both engines. So the three keywords currently encode
   **one** semantic distinction (const-ness), not three.
3. **`let` and `var` are indistinguishable downstream.** Any design that gives
   `let` immutability changes behaviour of existing programs.

### 1.2 Scope, visibility, shadowing, name resolution

- A whole-program resolver exists: `pkg/sema/resolve.go` (922 lines), with
  `NewResolver`, `collectDefs`, `indexModules`, `validateImports`,
  `checkEscapeReturns`. **This means the "Option B, diagnostics-only name table"
  proposed in `docs/audit/NAME-RESOLUTION-DESIGN.md` was implemented**, which
  makes that document's "PROPOSAL - nothing here is implemented yet" header
  stale. Correction prepared for its next revision.
- `public` is a real export modifier on both engines (Phase 103), with five
  precise diagnostics and qualified calls (`math.twice(21)`).
- Shadowing exists only inside the borrow checker (Phase 51), which is invisible
  to codegen.
- Self-hosted parity: `src/compiler/checker.kark` is a two-pass checker emitting
  K101-K115.

### 1.3 Documented gaps that a declaration design must confront

| Gap | Evidence | Consequence |
|-----|----------|-------------|
| No type values | `TYPE_SYSTEM_AUDIT.md`; `Type string` | Annotations are compared textually; `int` and `i64` cannot be known to differ |
| No whole-program type checker | AR5, scorecard section 2.2 | Definite assignment is not checkable |
| No inference | `TYPE_SYSTEM_AUDIT.md` | Every annotation is written by hand |
| Generics constraints unchecked | Phase 146 | `f[T](x)` accepts any `T` |
| `let`/`var` carry no meaning | this document, 1.1 | The most-read part of the language is decorative |

## 2. Declaration forms: alternatives considered

The requirement is a **unified** declaration form. Three were compared.

### Option 1 - Keep three keywords, give each a distinct meaning

`let` = immutable binding, `var` = mutable, `const` = compile-time constant.

- **Pros**: minimal syntax change; matches Rust (`let`/`let mut`) and Swift
  (`let`/`var`); no new grammar.
- **Cons**: **breaking**. Every existing program that reassigns a `let` stops
  compiling. The repository contains such programs - Phase 151A Step 2b's own
  reference corpus reassigns a `let` (`let x = 1; x = 2; return x`) and that
  corpus is *frozen and gated*. Migration cost is spread across the corpus, the
  self-hosted compiler sources, and 61 golden files.
- **Verdict**: rejected for v1.x. It buys clarity with a corpus-wide migration and
  a silent semantic change to code that currently compiles.

### Option 2 - One keyword plus an explicit mutability annotation

`let` for bindings, `var` retained as a deprecated alias, and an explicit
`mut` marker where mutation is intended; `const` unchanged.

- **Pros**: additive; existing programs keep compiling and keep their meaning;
  mutation becomes *visible in the source* rather than implicit; the annotation
  gives the future type checker a place to hang mutability-dependent rules.
- **Cons**: three keywords still exist during the transition; needs a documented
  deprecation policy (one already exists: `semver-policy.rst`).
- **Verdict**: **recommended**.

### Option 3 - A single `decl` form with an attribute block

`decl x: int = 5 [mut]`. Uniform and extensible.

- **Pros**: one form to learn; the attribute block is an obvious home for future
  qualifiers (`mut`, `pub`, alignment).
- **Cons**: rejects the idiom the ecosystem already reads; a much larger
  grammar change; `pub`/`mut` as brackets conflicts with the SIMD lane syntax
  `[4]f32`, which is already parsed.
- **Verdict**: rejected. The `[N]T` collision is concrete, not stylistic.

## 3. Recommended declaration surface

```kark
let  x = 5          // binding, immutable after initialisation (NEW meaning)
let mut y = 5       // mutable binding
y = 6               // legal: mut was declared
let  z = 6          // diagnostic: assignment to immutable binding
const MAX = 100     // compile-time constant (unchanged)
```

`var` continues to parse and is treated as `let mut` for one minor version, with
a deprecation note. This is Option 2.

**The mutability annotation is orthogonal to the type annotation**, and both are
optional:

```kark
let mut total: int = 0
let     items: []string = []
```

## 4. Scope, visibility, shadowing and name resolution

- **No change to scoping.** Function-local `let` remains block-scoped; the
  existing shadowing rule (Phase 51) continues to permit an inner `let x`
  shadowing an outer `let x`, with the borrow checker reporting both.
- **Declarations are added to the whole-program table**, not to a second table.
  `pkg/sema/resolve.go` already collects definitions; the declaration record is
  enriched in place rather than introducing a parallel symbol table, which would
  create two sources of truth - the failure mode control-system Rule 3 forbids.
- **`public` continues to apply to declarations**, not only to functions. Today a
  `public let` is rejected on both engines (Phase 103); this design keeps that
  rejection for v1.x rather than opening a mutable global export surface.

## 5. Mutability and reassignment

| Declaration | Reassignment | Evidence today |
|-------------|--------------|----------------|
| `const` | rejected, "cannot reassign constant" | Phase 112, both engines |
| `let` | **currently allowed** | Phase 112 gate leaves let/var reassignment untouched; 151A Step 2b reassigns a `let` |
| `var` | allowed | same |
| `let mut` | allowed | new |
| `let` (after this design) | rejected with a new diagnostic | new |

**The migration hazard is stated plainly:** step 5 changes the meaning of an
existing keyword. The frozen 151A Step 2b corpus contains
`let x = 1; x = 2; return x`, so step 5 **cannot** be landed without either
migrating that corpus or scoping the rule to a version boundary. This is the
single largest risk in the design and it is an owner decision, not an
implementation detail (see Decision D-3).

## 6. Initialization and definite assignment

- **Today:** a declaration without a value is permitted
  (`VarDeclStmt.Value == nil`), and nothing checks whether it is read before
  being written. The SSA pass sees an undefined register and the C compiler
  decides.
- **Proposed:** a binding must be initialized at its declaration point, or by an
  explicit `uninit`. Reading a possibly-uninitialized binding is a new
  diagnostic. This cannot be implemented before a whole-program dataflow pass
  exists, so it is staged (section 12), not promised.

## 7. Duplicate declarations and diagnostics

| Situation | Today | Proposed |
|-----------|-------|----------|
| Two `let x` in the same block | shadowing, permitted | unchanged (shadowing stays legal) |
| Two `const x` in one unit | duplicate definition | K107, unchanged |
| Two `func x` in one unit | K107 on both engines | unchanged |
| `public let x` | rejected | unchanged |
| `let = 42` | parse error (Phase 117 guard) | unchanged |
| assignment to a `let` | permitted | new diagnostic, new code |

**New diagnostic codes are assigned only at implementation time**, by the
authority that owns the code range (`karkain explain`, `stable-api.rst`). This
design names no code numbers, because a code number committed in a design
document is a code number nobody re-checks.

## 8. Function, struct, array and module interactions

- **Functions:** parameters and return types keep their current annotation
  syntax; the record idiom (`func deposit(a Acct)`) is unaffected. A
  by-pointer parameter (`&mut`) is how a function mutates a caller's binding, so
  the mutability marker and the borrow checker's reference types stay separate
  concerns.
- **Structs:** a `mut` marker on a field is **not** proposed. Field mutability is
  governed by the record idiom and by whether the field is written through a
  pointer; adding field-level `mut` would require a field-path type system.
- **Arrays:** `let mut` is what allows `arr[i] = v`. Element mutability needs no
  new syntax.
- **Modules:** `public` + qualified access are unchanged; declarations become
  exportable only in the same restricted way as functions.

## 9. Backward compatibility and migration

| Change | Compatibility | Migration |
|--------|---------------|-----------|
| `let mut` accepted | additive | none |
| `var` treated as `let mut` | behaviour-preserving | deprecation note only |
| `let` becomes immutable | **breaking** | requires an owner decision and a corpus migration |
| No change to `const`, `public`, annotations, scoping | none | none |

Per `docs/source/development/semver-policy.rst`, the breaking change is a minor
or major boundary, not a patch. **Recommendation: ship `let mut` first (additive,
minor), and defer the `let` immutability flip to a major boundary** so the two
concerns are never entangled.

## 10. Parser and semantic-analysis responsibilities

- **Parser:** recognise the optional `mut` token between the keyword and the
  name, and record it as a field on the existing declaration node. No new
  statement type, no new node kind - the three keywords already share one path.
- **Semantic analysis:** the mutability check belongs in the whole-program
  resolver (`pkg/sema/resolve.go`), which already walks definitions and
  assignments, and in the self-hosted checker (`src/compiler/checker.kark`) for
  parity.
- **Codegen:** unchanged. Mutability is a check-time property, not a runtime one.
- **Explicitly not proposed:** a sweeping parser or type-system rewrite. The
  annotation stays a string in v1.x; replacing it with a real type value is a
  separate, much larger programme (see Decision D-4).

## 11. Error recovery and diagnostic stability

- Recovery stays where it is: the multi-error project-wide preflight (Phase 105)
  reports every recoverable parse error in one invocation. A new `mut` token must
  not introduce a new failure mode there - if `let mut` fails to parse, the
  recovery path must still emit a column-accurate diagnostic rather than
  desynchronising the token stream.
- **Diagnostic stability rule:** a mutability diagnostic reports the binding
  name and the assignment line, matching the style of K107 (duplicate definition)
  and K002/K102 (undefined name). It must never be reported at the declaration
  site; that is the mistake that makes a shadowed error unreadable.
- Both engines must produce the same code and the same name. The LH-2 gate is the
  worked precedent: its negative asserts on the **code and the name**, because
  asserting on an exit status proved nothing when the checker accepted the
  program.

## 12. Test matrix, negative cases and acceptance criteria

| # | Case | Kind | Expected |
|---|------|------|----------|
| T1 | `let mut x = 1; x = 2; print(x)` | positive | both engines print `2` |
| T2 | `let mut = 1` | negative | parse error naming the missing binding |
| T3 | `mut x = 1` (no keyword) | negative | parse error, not a silent `mut` binding |
| T4 | `let x = 1; x = 2` | negative **after** the flip | assignment to immutable binding, both engines, same code and name |
| T5 | `let mut x: int = 1; print(x)` | positive | annotation and mutability coexist |
| T6 | shadowed `let mut x` inside a block | positive | borrow checker reports both, as today |
| T7 | `public let mut x = 1` | negative | rejected, as `public let` is today |
| T8 | `var x = 1; x = 2` | positive | still legal during the transition window |
| T9 | frozen 151A Step 2b corpus (`let x = 1; x = 2`) | regression | **fails by design after the flip** - see Decision D-3 |
| T10 | corpus parity on both engines | regression | all three native goldens and the 61-file corpus unchanged while the flip is deferred |

Acceptance criteria for the additive slice only: T1-T3, T5-T8, T10 pass on both
engines; T4 is `not-applicable` until the flip is authorised; T9 is tracked as
the known cost of the flip.

## 13. Implementation dependencies and staged rollout

| Stage | Content | Gate | Depends on |
|-------|---------|------|------------|
| S1 | `let mut` parsed and recorded; `var` aliased to `let mut`; deprecation note | new gate + CI step | an authorised increment |
| S2 | mutability check in both checkers, negative T4 | parity gate asserting code **and** name | S1 |
| S3 | migrate the frozen corpora (151A Step 2b and any other reassigned `let`) | existing parity gates re-run green | owner decision on the flip |
| S4 | `let` immutability flip with the new diagnostic | full corpus + conformance | S3, **major-version boundary** |
| S5 | definite-assignment checking | requires a whole-program dataflow pass | a separate programme |
| S6 | string annotations replaced by real type values | a separate programme | not proposed here |

Each stage is its own increment with its own baseline note, per `AGENTS.md`.
S1 and S2 are additive and can ship in a minor release. S4 is breaking.

## 14. Open questions requiring a language-owner decision

| # | Question | Why it cannot be decided here | Recommendation |
|---|----------|------------------------------|----------------|
| D-1 | Adopt Option 2 (`let mut`)? | Changes the language surface; needs the owner | **Yes** |
| D-2 | Keep `var` as a deprecated alias, or remove it? | Semver-visible | Keep for one minor, per the semver policy |
| D-3 | When does `let` become immutable? | **Breaking**, and it breaks a frozen gated corpus (T9) | Defer to a major boundary; do not fold into S1/S2 |
| D-4 | Does the string annotation get replaced by real type values? | Large programme, interacts with generics and the WASM/native backends | Separate increment; this design only requires that the declaration record not *block* it |
| D-5 | Are field-level mutability and `mut` on parameters in scope? | Interacts with the record idiom and borrow semantics | Out of scope; revisit with the type-value programme |
| D-6 | Is `public let mut` ever legal (an exported mutable global)? | Concurrency and aliasing implications | Keep rejected for v1.x |

## 15. Decision record

| Decision | State | Basis |
|----------|-------|-------|
| One parser path for `let`/`var`/`const` is the status quo to build on | **settled** (design) | `parseVarDecl` at `pkg/parser/parser.go:441` |
| `Type string` stays for v1.x | **settled** (design) | changing it is Decision D-4's programme |
| Option 2 (`let mut`) is the recommended form | **proposed** | options compared in section 2 |
| Mutability belongs to semantic analysis, not codegen | **settled** (design) | it is a check-time property |
| Mutability diagnostics must assert code and name on both engines | **settled** (design) | LH-2 precedent |
| `let` immutability flip, and its timing | **awaiting sign-off** | Decision D-3, blocking for S4 |
| Real type values | **awaiting sign-off** | Decision D-4, not proposed here |
| Field/parameter mutability | **awaiting sign-off** | Decision D-5, out of scope |

**No decision in this document is approved by its existence.** Decisions D-1
through D-6 need an explicit owner decision before S1 is scheduled, and S1
itself needs its own baseline note. Until then this document is a proposal, and
the control-system queue carries `TYPE-DESIGN` as a design milestone only.