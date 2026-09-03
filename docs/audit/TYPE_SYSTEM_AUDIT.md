# KARKAIN TYPE SYSTEM AUDIT

Evidence-based. This audit determines what the type system ACTUALLY does today and what a
realistic, architecture-fitting type system looks like.

## Actual type handling today
- **No type values**: types are handled as strings in domain validators (`ffi.go:213`,
  `kernel_analyzer.go:49`, `actor.go:317`). There is no `Type` value type.
- **No general type checker**: nothing walks the AST to verify/conclude types for the program.

## Type capability matrix (actual)
| Type | Status | Evidence |
|------|--------|----------|
| Boolean | string-checked only | ffi.go:214, kernel_analyzer.go:49 |
| Signed/unsigned integers | string-checked only | ffi.go:214-216 |
| Float | string-checked only | ffi.go, kernel_analyzer |
| Character | NOT handled | — |
| String | string-checked only | ffi.go:220 |
| Void/unit | partial | SSA Void type (ssa.go:10) |
| Arrays | prefix/string check | kernel_analyzer.go:54, actor.go:327 |
| Tuples | NOT handled | — |
| Structs | NOT type-checked | — |
| Enums | partial (parser only) | BUG-7 |
| Functions | partial (SSA) | — |
| References / pointers | SSA Ptr type, borrow-checked | ssa.go |
| Generic types | string substitution | generics.go |
| Optional / Result | literal-classified only | borrow_checker.go:490-511; BUG-2 |

## Type inference (actual)
- Local var inference: STUB (VarDeclStmt Type=="" never resolved; only inferMatchType classifies Option/Result).
- Function return inference: NOT implemented.
- Generic inference: NOT implemented.
- Literal/numeric inference: NOT implemented.
- Bidirectional inference: NOT implemented.

## Generics (actual)
- Trait/impl registry: method-NAME coverage only (generics.go:77-120) — no signature check.
- Monomorphization: string substitution (generics.go:246-316); `InstantiateGenericFunc` MISSES
  body substitution (generics.go:216-220); only wired for GPU kernels (generic_kernel.go).
- Generic `<T>` syntax: NOT parsed (parser.go:95 passes nil).

## Realistic recommendation (fits existing Go-like, string-based architecture)
Do NOT import Rust/ML-level type inference or trait machinery wholesale. Instead:
1. Introduce a minimal `Type` value (Kind + element/arg types) and a whole-program type checker
   that lowers the existing string annotations to real types. This is the critical missing stage
   (see PIPELINE_AUDIT).
2. Complete generics as **reified monomorphization** (matches the Go-like SSA strategy): parse
   `<T>`, fix func-instantiation body substitution, wire the monomorphizer into the CLI pipeline.
   Do NOT add trait-object / dynamic-dispatch complexity unless V1 demands it.
3. Fix BUG-2 (match arms) and BUG-7 (payload variants) so Option/Result/ADTs actually compile.
4. Add only the type features P1 (V1) requires; defer advanced inference (bidirectional, HKTs)
   to RESEARCH ONLY.

## V1 type scope
MUST: bool, i64/i32/etc., u64/u32, f64/f32, char?, string, arrays, structs, enums (ADT), function
types, references/pointers (borrow), generics (monomorphized), Option<T>, Result<T,E>, unit/void.
DEFER: tuples (if not needed), traits as dynamic dispatch, sophisticated inference, HKT.
