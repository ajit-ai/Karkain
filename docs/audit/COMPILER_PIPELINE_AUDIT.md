# KARKAIN COMPILER PIPELINE AUDIT

Evidence-based map of the actual pipeline, per the audit requirement to document it and identify
missing stages WITHOUT inventing HIR/MIR just because other compilers use them.

## Actual pipeline (default `karkain run`/`build`)
```
Source (.kark)
   ↓
ValidateKarFile (ext check)
   ↓
loadSourceWithSiblings (import resolution is FILE-JOIN, not module compilation)
   ↓
lexer.New(src)                  [pkg/lexer]
   ↓
parser.New(l) → ParseProgram()  [pkg/parser]  ← OOM-bearing here on large input
   ↓
Borrow checker (runBorrowCheck) [pkg/sema/borrow_checker.go]  (Run & Build only)
   ↓
ApplyMacroExpansion             [pkg/parser/macro.go]
   ↓
emitGPUShaders (if kernels)     [pkg/codegen]
   ↓
codegen.GenerateAndCompile      [pkg/codegen]
   ├─ SSA path (default): AST → SSA (lower.go) → FoldConstants → DCE → Verify → C23 (emit_ir.go)
   └─ legacy fallback on SSA failure: genFuncDecl → C23 (native.go)
   ↓
GCC/Clang/MSVC → native executable
```

## Stage classification
| Stage | Exists? | Where | Notes |
|-------|---------|-------|-------|
| Lexer | YES | pkg/lexer | complete |
| Parser | YES | pkg/parser | feature-rich; OOM bug; generics not parsed |
| AST | YES | pkg/parser/ast.go | empty Node interface |
| Semantic analysis | PARTIAL | pkg/sema | borrow + kernel analyzer only; NO full type pass |
| Typed representation | NO | — | no typed IR; types are strings |
| IR (SSA) | YES | pkg/ir/ssa | used in default path |
| Optimization | PARTIAL | ssa/opt.go | fold + DCE only |
| Code generation | YES | pkg/codegen | C23 + backends |
| Executable | YES | GCC/Clang/MSVC | delegated |

## Missing stages (purpose-justified, not cargo-culted)
| Stage | Needed? | Justification |
|-------|---------|---------------|
| **Typed representation / full type checker** | YES (critical) | Without a typed intermediate + whole-program type pass, codegen cannot be trusted for type errors. This is the biggest missing stage. |
| **Symbol table / name resolution** | YES (critical) | Needed for modules, undefined-var/func detection, visibility. |
| **Optimizer pipeline (beyond fold+DCE)** | YES (perf) | G2/G12 — see feature priority / next phase. |
| HIR / MIR | NO | Not justified for a Go-like single SSA middle-end; would add complexity without clear benefit (Rule: avoid). Keep one SSA IR. |

## Conclusion
The pipeline fetches the right shape (Lexer→Parser→AST→Sema→SSA→Opt→Codegen→native) but the
**middle is thin**: semantic analysis is a narrow borrow check, there is no typed representation,
and optimization is two passes. The architecture does NOT need more IR layers; it needs the
existing SSA middle-end to be backed by a real type checker and symbol table, then deepened.
