# Phase 146 BASELINE — Generics v1

Date: 2026-09-23. Target: LANGUAGE opener (after 145 quarantine).

## 1. Starting position

- AST carries GenericParams (Phase 26) on FuncDecl/StructDecl/KernelDecl,
  but NO parser fills them (parseFunc/parseStructDecl pass nil).
- Monomorphizer (`pkg/sema/generics.go`): traits/impls/constraints,
  InstantiateGenericStruct/Func/Kernel (`Name_arg` mangling), partial
  substitution — unit-tested, wired ONLY to kernels. No func/struct
  call-site wiring exists.
- kcc: zero generic handling anywhere (parser drops nothing today
  because nothing generic parses).
- WASM K108 rejects "generics" (stays).

## 2. Syntax contract (v1, pinned by the gate)

- Decl: `func name[T, U](params)`, `type Name[T] struct {...}`.
  Constraints (`T: Numeric`) parse and store but are UNCHECKED in v1
  (no trait/impl syntax exists to declare against).
- Call: `f[int](args)` explicit type args. Square brackets (angle
  brackets collide with `<`/`>` comparisons in expression grammar).
- No inference in v1 (explicit args always; 147 candidate).

## 3. The ambiguity and its resolution

- `f[T](x)` is syntactically identical to index-then-call
  (`ops[idx](5)`, Phase 133). The parser has 1-token lookahead — no
  syntactic disambiguation is possible.
- Design: OPTIMISTIC PARSE + SEMANTIC DEMOTION. `IDENT[ident-list](args)`
  parses as CallExpr+TypeArgs (all-Identifier bracket contents + `(`
  after `]`; everything else keeps today's Index/Matrix/Slice shapes
  byte-identically). The sema monomorphize pass then: generic callee →
  instantiate + rewrite to the specialized name; non-generic callee →
  demote back to IndirectCall(IndexExpr) (Phase 133 behavior preserved
  exactly, including variable indices); unresolved callee → K002 as
  today. `ops[0](5)` never even enters the generic shape (int index).
- Struct literals: `Point[int]{...}` (no valid legacy reading of
  `Name[...]{`, so no demotion needed — misuse is K115).

## 4. Error codes

- kcc: **K115** (verified free) — generic arity mismatch + misuse.
- Go: resolve-class diagnostics (E-K-RES family, exit 3), mirroring
  existing arity handling; `explain` gains the K115 row.

## 5. Slice plan (as executed)

- **146A**: Go parser (decl params, call TypeArgs, struct TypeArgs) +
  Monomorphizer hardening (body/param substitution, field preservation)
  + monomorphize pass (collect → validate arity → instantiate →
  append specializations → rewrite/demote) + emission (plain units) +
  K115/explain.
- **146B**: stdlib generic Stack[T]/Queue[T] + examples + both-engine
  goldens.
- **146C**: kcc parser + checker + codegen parity (flat-namespace
  lowering, Phase-103 pattern) + byte-identical goldens.
- **146D**: phase146 gate + CI + docs (SPEC/stable-api) + reports.

## 6. Non-goals (documented, NOT defects)

Variance (invariant only), HKT, trait objects/dispatch, generic
methods, const generics, checked constraints, WASM lowering (K108),
inference (147 candidate), cross-build specialization caching.
