# KARKAIN MATURITY SCORECARD

Scoring: 0 = Not started, 1 = Experimental, 2 = Early implementation, 3 = Functional,
4 = Production-oriented, 5 = Mature.

Every score is justified by repository evidence. Scores reflect behavior actually wired
into the shipped pipeline, not just source files present.

| Area | Score | Evidence | Next Requirement |
|------|------:|----------|------------------|
| Lexer | 4 | Complete tokenizer, zero-copy offsets, ~100 token kinds, all literal types, line comments. No unicode, no block comments (as spec'd). | Unicode policy; block comments; illegal-token recovery policy. Production-grade. |
| Parser | 2 | Feature-rich (expr, stmt, funcs, structs, enums, match, macros, lambdas). BUT generics syntax unparsed, no dedicated unit test, and **function-call arg loops OOM on large input**. | Fix OOM (no-progress guard); parse generics; add parser_test.go. Blocking correctness. |
| AST | 3 | Strongly-typed struct corpus (~100), arena allocation. Root `Node interface{}` is empty → nominal typing; heap/arena alloc split. | Tighten Node interface; unify allocation; add visitor for transforms. |
| Semantic Analysis | 2 | Borrow checker lexical stack + shadowing works; kernel analyzer. But NO whole-program type checker, NO symbol table/scope resolution, undefined var/func not detected. | Implement type checker + symbol table; wire validators into pipeline. |
| Type System | 1 | Primitive types string-named only; no type values; Option/Result partial (BUG-2); generics string-substitution partial. | Real type values + checker; fix BUG-2; complete generic semantics. |
| Generics | 1 | String-substitution monomorphization (reified); trait name-coverage check; func-inst instantiation incomplete; not wired to CLI (GPU kernel only). | Parse `<T>`; fix func instantiation; wire monomorphizer into pipeline. |
| Modules | 1 | C import blocks + `import`; no pub/private visibility (G11); module system merged with KPM. | Module hierarchy + visibility (Phase 65). |
| Error Handling | 1 | Option/Result/match exist; `?` is a no-op (BUG-4); error-boundary/panic (G2) missing; error-handling semantics non-functional (ROADMAP). | Fix BUG-4; define propagation; error boundary. |
| Memory Model | 2 | Ownership/borrow (Rust-style) with manual alloc/free; escape analysis foundation. Moves never expire (BUG-5); no GC; no defer/RAII (G8). | Complete ownership (Phase 63); decide model (see MEMORY_MODEL_DECISION.md). |
| Runtime | 3 | Small C runtime (actor/quantum/reflect/rpc ~24KB); Go actor+coroutine runtime. Concurrency not wired into default path. | Wire concurrency into pipeline (Phase 57); keep runtime small. |
| Code Generation | 3 | SSA→C23 default path; delegating to GCC/Clang/MSVC; GPU/quantum/SPIR-V emitters. Optimizer shallow (fold+DCE). | Deeper optimizer (Phase 64/67); conformance-test GPU/quantum backends. |
| Standard Library | 1 | 4 stdlib modules real; std/io+string are stubs; Go-side stdlib mostly stubs. | Build out stdlib (Phase 60); remove HTTP stub. |
| Testing | 3 | ~493 test functions; broad unit coverage (sema 198, codegen 116). No parser unit test, no snapshot/golden, no perf gates. | Add parser_test.go; snapshot/conformance suite; perf gates. |
| CLI | 4 | build/run/check/test/transpile/lsp/pkg all functional; exit codes; single binary. | Formatter/linter (Phase 66); REPL. |
| Tooling | 2 | LSP basics; syntax highlighting. No formatter, linter, doc generator, REPL. | Phase 66 tooling. |
| Concurrency | 2 | Actors + coroutines + channels implemented but not in default pipeline; no atomics/memory ordering (G7). | Phase 57 wiring; atomics (Phase 70). |
| FFI | 2 | C ABI extern, dynamic loading, C interop. Not fully integrated with borrow safety across boundary. | Harden unsafe boundary; callbacks; ownership across FFI. |
| Performance | 2 | SSA + fold + DCE; arena; zero-copy lexer; C23 delegation. Optimizer shallow; no SIMD/atomics; compile-time safety incomplete. | Phase 64/67 optimizer; optimization flags; SIMD/atomics (70). |
| Documentation | 4 | SPEC v0.14.0 authoritative; README architecture; ROADMAP with gap register. | Conformance tests against SPEC; keep spec/impl converged. |
| Self-Hosting | 1 | `src/compiler/*.kark` real but incomplete; Stage1 transpile OOM-blocked; seed tests only. | Phase 56: fix parser OOM, complete self-host, bootstrap. |
| Package Manager | 4 | KPM complete: semver, lock, integrity, audit, cache, workspace, registry stub, auth. | Real registry; SBOM; signed releases. |

## Aggregate maturity
- Strong: Lexer (4), CLI (4), PM (4), Docs (4).
- Functional: AST (3), Codegen (3), Runtime (3), Testing (3).
- Early: Parser (2 — correctness bug), Sema (2), Memory (2), Concurrency (2), Tooling (2), FFI (2), Performance (2).
- Weak: Type System (1), Generics (1), Modules (1), Error Handling (1), Stdlib (1), Self-Hosting (1).

## Honest overall: Language is a **functional compiler prototype / Early Alpha**, not production.
The lexer/CLI/PM/documentation are the most mature. The correctness-critical middle
(type system, generics, semantic analysis, error handling) and self-hosting are the
weakest — and the parser has a live OOM defect. Features cannot be trusted fully until the
type/sema/parser foundation is made sound.
