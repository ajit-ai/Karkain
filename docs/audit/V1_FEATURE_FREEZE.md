# KARKAIN V1 FEATURE FREEZE

Objective: a SMALL but COMPLETE language, not a LARGE but INCOMPLETE one. This freeze is
realistic — it ships only what the current architecture can genuinely support soundly.

## MUST HAVE (V1 core — language correctness + completeness)
- Lexer (complete; unicode as SPEC policy, keep ASCII if undecided).
- Parser fixed (OOM guard) + generics `<T>` syntax.
- AST with arena.
- **Whole-program type checker** + **symbol table / name resolution** (undefined var/func,
  shadowing, duplicate detection).
- Primitive types: bool, i8..u64, f32/f64, string, char (policy), unit/void.
- Composite: arrays, structs, enums (ADTs), function types, references/pointers (borrow).
- Generics via **reified monomorphization** (parse + fix func-instantiation + wire pipeline).
- Option<T>/Result<T,E> with WORKING match (fix BUG-2) and `?` propagation (fix BUG-4).
- Error boundary/panic (G2) + defer/RAII (G8).
- **Complete ownership/borrow** (fix BUG-5, real BorrowError.Line), escape analysis → stack alloc.
- Codegen (SSA→C23) with BUG-1/3/7/8 fixed.
- Diagnostics (line/col/snippet/suggestion).
- CLI: build/run/check/test/transpile/lsp + exit codes. ONE official toolchain.
- Standard library: collections, string, io/filesystem, math, time (small but real).
- Testing framework + conformance suite skeleton.
- SPEC v1.0 converged with implementation.

## SHOULD HAVE
- Modules + visibility (pub/private) with package layout (project/src/*.kark).
- KPM integration (already built) polished; real registry.
- Formatter (kfmt), linter; LSP completion.
- Basic error classes (E#### codes, suggestions).
- Fuzzing harness (parser/codegen) with 0-crash gate.

## DEFER (after V1)
- GPU/quantum production backends (keep emitters experimental, not default).
- Actor/coroutine runtime wired as DEFAULT (keep as opt-in).
- Atomics/memory-ordering first-class (Phase 70) / explicit SIMD.
- Distributed execution (quantum/lattice).
- Rich stdlib (networking, encoding, reflection, full FS).

## RESEARCH ONLY (do NOT include in V1)
- Unified execution model.
- Capability-based security.
- Effect system.
- AI-native syntax (keep AI in packages).
- Bidirectional/HKT type inference.

## Freeze discipline
- Every V1 MUST HAVE must be IMPLEMENTED with tests BEFORE v1.0 tag.
- Anything not in the list is deferred by default (Rule 3: no fashionable features).
- Breaking fixes (BUG-1..8) grouped under this freeze as ONE controlled migration
  (see BACKWARD_COMPATIBILITY_AUDIT).
