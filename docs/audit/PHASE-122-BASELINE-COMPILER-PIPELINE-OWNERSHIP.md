# PHASE 122 BASELINE — COMPILER PIPELINE OWNERSHIP

Status: BASELINE RECORDED (pre-implementation)
Date: 2026-09-14

This document records the Phase 122 baseline component inventory and the
target-selection decision, as required before implementation. Every claim was
verified against the repository state at commit `477f448` (Phase 120 + 121
merged) / `c9099b3` (develop).

---

## 1. COMPONENT INVENTORY — `src/compiler/`

| File            | Purpose                            | Implementation status | Actually compiled | Actually executed | Called by                        | Calls                                            | Feeds                        | Tests                                       | Production path                                   | Go equivalent                                            | Duplication |
|-----------------|------------------------------------|-----------------------|-------------------|-------------------|----------------------------------|--------------------------------------------------|------------------------------|---------------------------------------------|---------------------------------------------------|--------------------------------------------------------------|-------------|
| `main.kark`     | Compiler driver (check/build/run/test/kir/lsp) | ACTIVE               | yes               | yes (every kcc invocation) | bootstrap/gcc                        | lexer, parser, checker, codegen, kir, stdlib helpers | CLI verdicts, C23 files      | phase95/96/99/103/117/121 gates             | kcc binary entry point                          | `pkg/cli/*_command.go`                                  | CLI surface (intentional mirror) |
| `lexer.kark`    | Tokenizer                          | ACTIVE               | yes               | yes               | main.kark (all parse paths)        | —                                                | tokens → parser              | compiler-sources self-check gates            | in-process kcc tokenize                         | `pkg/lexer`                                           | full (parity-pinned) |
| `parser.kark`   | Parser / AST build                 | ACTIVE               | yes               | yes               | main.kark                          | lexer, ast                                       | AST                         | compiler-sources self-check gates            | in-process kcc parse                            | `pkg/parser`                                         | full (parity-pinned) |
| `ast.kark`      | Node constructors/accessors        | ACTIVE               | yes               | yes               | parser/checker/codegen/kir         | —                                                | —                            | gates as above                               | in-process                                      | `pkg/parser/ast.go`                                  | full (parity-pinned) |
| `checker.kark`  | Two-pass conservative type checker | ACTIVE               | yes               | yes               | main.kark (check path + CLI preflight via check) | ast, atypes                          | type diagnostics (error[K1xx]) | phase99/103/117/118                          | in-process kcc check                            | `pkg/sema/resolve.go`                                 | full (parity-pinned, conservative subset) |
| `codegen.kark`  | C23 text emitter                   | ACTIVE               | yes               | yes               | main.kark (build/run)              | ast                                            | C23 files                    | runtime/parity gates                          | in-process kcc build/run                        | `pkg/codegen/codegen.go`                              | full (parity-pinned) |
| `sema.kark`     | Builtin tables + inference helpers | PARALLEL             | yes               | NO (no production caller) | — (not invoked by main.kark)       | —                                              | —                            | compiler-sources self-check only             | dormant                                        | `pkg/sema/builtins.go`                                | duplicate of checker.kark tables |
| `kir.kark`      | KIR v1 text emitter + verifier     | ACTIVE               | yes               | yes               | main.kark (check/verifykir/kir)    | ast                                             | KIR text                     | phase120/121                                  | in-process kcc kir/verifykir + check hook       | none (Karkain-owned)                                  | none |
| `stdlib.kark`   | String utilities                   | ACTIVE               | yes               | yes               | main.kark                          | builtins                                       | assembled program             | compiler-sources self-check                  | in-process                                      | `pkg/cli/commands.go` helpers                          | low |
| `atypes.kark`   | Type inference helper              | ACTIVE               | yes               | yes               | checker.kark                       | —                                              | type info                    | compiler-sources self-check                  | in-process                                      | `pkg/sema/type.go`                                    | medium |

Verification notes:

- `sema.kark` is compiled (part of every whole-tree assembly) but has NO caller:
  `grep` of `main.kark` shows the check path uses `typeCheckProgram` from
  `checker.kark`. It is classified PARALLEL (builtin tables duplicated with
  `checker.kark`), NOT a production-path component.
- `main.kark` `loadSourceWithSiblings` (lines 378–398) is ACTIVE only when kcc
  is invoked directly on a bare file; the CLI always pre-assembles with Go
  (`kccAssembleSource`), so the Karkain assembler is DORMANT on the production
  CLI path. This is the evidence for the Phase 122 target (below).

## 2. PRODUCTION PATH (kcc engine, before Phase 122)

```
karkain check/build/run/kir/verifykir  (default engine kcc)
  │
  ├─ Go  pkg/cli: projectSyntaxDiagnostics   (multi-error syntax preflight)
  ├─ Go  pkg/cli: kccAssembleSource          (resolveSourcesRun + regex strip)
  │      ├─ no imports  → Go legacy sibling join  (resolveSources / loadSourceWithSiblings[Go])
  │      └─ imports     → Go pkg/module graph (SortedFiles topo order) + dotted-import regex strip
  ├─ Go  pkg/cli: kccTargetPreflight         (NPU @target diagnostics)
  ├─ Go  pkg/cli: write assembled text to temp sandbox single file
  └─ kcc (self-hosted): tokenize/parse/typecheck/kir/codegen on the sandbox file
```

The last Go-controlled step that composes the compiler's *input text* is
`kccAssembleSource`. Karkain's only assembler (`loadSourceWithSiblings`) does
not run on this path. This is the highest-remaining, text-composing Go
ownership inside the compiler core and the Phase 121-documented BRIDGE
bottleneck.

## 3. TARGET SELECTION

```
Candidate:                 Project source assembly (import-aware)
Current Karkain implementation: src/compiler/main.kark loadSourceWithSiblings
Current Go implementation: pkg/cli/kcc_engine.go kccAssembleSource +
                           pkg/cli/module_build.go resolveSourcesRun/assembleModuleUnit +
                           pkg/module (graph) + pkg/pm (manifest deps)
Current production path: (kcc engine) Go composes the full assembled text,
                           kcc compiles it in a sandbox.
Why not yet fully active: the Go CLI pre-composes the input text before kcc
                           runs; the self-hosted assembler only spins up on
                           direct bare `kcc` invocations.
Phase 122 target:         Move FLAT project assembly into src/compiler
                           (replace the dormant loadSourceWithSiblings with an
                           import-aware assembleProject) and route the real CLI
                           kcc check/build/run/kir/verifykir path through it for
                           every flat project (no karkain.toml manifest and all
                           imports std.* / sibling-resolvable). Go keeps:
                           manifest/workspace/PM assemblies, module-graph
                           diagnostics, NPU staging, sandbox file mirroring and
                           task orchestration (BRIDGE).
Expected independence gain: the kcc engine composes its own input for the
                           common flat case; one Go text-composition step
                           (dotted-import strip + sibling/stdio module join) is
                           retired from that path; kcc becomes directly runnable
                           on flat import-ful projects without any Go front-end
                           assistance.
```

Explicitly NOT chosen (evidence-based): lexer/parser/ast/checker/codegen/kir are
all ACTIVE already; a second KIR verifier is banned; module graph / manifest /
workspace resolution is intentionally left Go-owned this phase (the prompt's
"smaller verified step > large fake migration").

## 4. REQUIREMENTS REAFFIRMED

- Preserve Phase 120/121 results (KIR v1 emitter + kirVerify on the default
  check path). `assembleProject` output must keep flowing through
  `kirEmit`→`kirVerify` unchanged.
- Byte/determinism contract: assembled file SET for `src/compiler` stays the
  same 10 files (9 siblings + root), so the 6399-line KIR self-verification and
  the `[ok] kir text: N lines` markers are preserved.
- 1.0.0 behavior intact: conformance 59/59, probes 11/11, verify-examples,
  all phase gates.