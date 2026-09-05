# KTF-002 — Compile-Pass / Compile-Fail Corpus with Diagnostics Capture

Status: COMPLETE
Baseline: KTF-001 (native testing foundation) complete; CLI toolchain B1–B4 complete.
Branches: `develop` → `main` (AGENTS.md workflow).
Extension: `.kark`.

## 1. Mission Summary

KTF-002 adds the compile-pass/compile-fail corpus to the toolchain: a bundled,
deterministic set of source fixtures that assert (a) valid programs compile
through the full front end AND the native C backend, and (b) invalid programs
produce a diagnostic of the declared stable error-code class with a matching
message substring. The corpus is exercised by `karkain test --compile [dir]`
(exit 0 all pass, 4 any fail) and is entirely driven by a manifest — the same
"declaration in the repo, execution in the toolchain" boundary established by
KTF-001.

## 2. Repository Inspection (evidence)

- Front end = lexer/parser → macro expansion → name resolution (`sema.NewResolver`)
  → kernel semantic analysis (`sema.NewKernelAnalyzer`) → borrow checker
  (`runBorrowCheck`, `pkg/cli/commands.go:578`). `lint` short-circuits after
  parse errors and tags findings with stable codes (`pkg/diagnostics/codes.go`):
  `E-K-SYN`, `E-K-RES`, `E-K-BRW`, `E-K-SEM`, `E-K-TYP`, `E-K-CG`, `E-K-PKG`,
  `E-K-ENV`.
- Backend entry point: `codegen.GenerateAndCompile(prog, file)` with
  `Config.CompileOnly` (`codegen.go:16`) for the pass cases. C compiler
  discovery is `exec.LookPath("gcc"/"clang"/"cc"/"cl.exe")` (mirrored by
  `cli.haveCCompiler`); absence means the honest outcome for pass cases is
  **SKIP**, not a manufactured failure.
- `karkain test` dispatch is in `cmd/karkain/main.go`; `--filter`
  (KTF-001) already existed there in both space and `=` forms.
- Exit-convention: `ExitSuccess`=0, `ExitTest`=4 (both dispatched through
  `os.Exit`).

## 3. Architecture Decisions

| Decision | Rationale |
|---|---|
| Manifest-driven corpus | Coverage is declared and versioned in the repo; runner stays dumb and deterministic |
| `manifest.json` = `{cases:[{file,expect,code,message}]}` | Machine-checkable; `expect ∈ {pass,fail}` |
| Diagnostics captured by running the real lint pipeline | The corpus asserts exactly what a user sees — no duplicate front-end logic |
| Pass case = front-end clean AND `CompileOnly` codegen succeeds | Strongest honest claim: the program really compiles to C |
| Fail case = one diagnostic matches declared `code` (+ substring) | Code class + message guards against both false negatives and false positives |
| No C toolchain ⇒ pass cases SKIP (summarized) | Deterministic, honest; machine without gcc/clang still runs fail cases |
| Kernel-param `in` trap documented | `in` is a reserved word and misaligns kernel param parsing — corpus avoids it |
| `;` is not a statement separator | Corpus fixtures use newline-separated statements (real grammar) |
| `x = expr` reassignment / `x y -> T` return-type / `a..b` ranges not used | Verified unsupported today; corpus only encodes real grammar |

## 4. Model (`pkg/testing/compile.go`)

```go
type CompileCase   struct { File string; Expect CompileExpect; Code diagnostics.Code; Message string }
type Diagnostic    struct { File string; Line, Col int; Code, Message string }
type CompileResult struct { ID, File string; Expect CompileExpect; Status Status; Duration;
                            Diagnostics []Diagnostic; Error string }
type CompileSummary struct { Total, Passed, Failed, Skipped int }
```

- `ExpectPass`/`ExpectFail`; `Diagnostic` is the structured front-end finding
  (location + stable code class + message).
- `SummarizeCompile` counts PASS/FAIL/SKIP; `SortCompileByID` gives the stable
  `<expect>:<file>` ordering used by both the manifest loader and the runner.

## 5. Runner (`pkg/cli/compile_corpus.go`)

- `loadCompileManifest(dir)` — parses + validates the manifest, resolves every
  referenced file (missing file ⇒ hard error, so a stale manifest can never
  silently shrink coverage), sorts deterministically.
- `captureFrontendDiagnostics(file, src)` — runs lexer/parser → macro expansion
  → resolver → kernel sema → borrow checker, mirroring `lint`; parse errors
  short-circuit. Returns `[]Diagnostic` (code-tagged, message + location).
- `runPassCase` — front end must be clean, then `codegen.GenerateAndCompile`
  with `CompileOnly=true` into `os.TempDir()` (temp `.pass.c` removed after).
  Without a C compiler the case is `StatusSkip` with an explicit reason.
- `runFailCase` — requires at least one diagnostic matching the declared code
  (and message substring when set); a clean file or a mismatched code ⇒ FAIL
  with a diagnostic-summary error.
- `RunCompileCorpus(dir, verbose, cfg)` — one line per case
  (`PASS|FAIL|SKIP`), summary with failed-count emphasis, deterministic order.

## 6. Bundled Corpus (`pkg/cli/testdata/compile/`)

15 cases, 8 pass + 7 fail:

| Case | Purpose | Verifies |
|---|---|---|
| pass/arith.kark | integer math | parse + resolve + codegen |
| pass/control_flow.kark | `if/else`, `while`, typed `let` | control-flow front end + codegen |
| pass/data_structures.kark | struct literal, field access, array literal, `len`, indexing | data-model codegen |
| pass/for_in.kark | `for i in [...]` | iteration front end + codegen |
| pass/funcs.kark | params, `return`, recursion | call/return codegen |
| pass/strings.kark | string literal, `len`, `==`, `+` concat | string codegen |
| pass/borrow_safe.kark | closure read of captured `let` | clean borrow front end + codegen |
| pass/kernel_compile.kark | `kernel` + `barrier()` + `global_id()` | kernel sema accepts, codegen emits |
| fail/undefined_func.kark | `help_me()` | `E-K-RES` "undefined function" |
| fail/duplicate_func.kark | two `foo()` | `E-K-RES` "duplicate function definition" |
| fail/syntax_junk.kark | top-level junk | `E-K-SYN` "unexpected token" |
| fail/use_after_scope.kark | inner `x` used after block | `E-K-BRW` "after its scope has ended" |
| fail/use_after_move.kark | `print(a)` after `move(a)` | `E-K-BRW` "use of moved value" |
| fail/kernel_io_forbidden.kark | `writeFile` in kernel | `E-K-SEM` "file I/O operation" |
| fail/kernel_recursion_forbidden.kark | recursive kernel call | `E-K-SEM` "recursive call forbidden" |

Every expected message was validated empirically against `karkain lint <file>`
output before being frozen into the manifest.

## 7. CLI

```bash
karkain test --compile [dir]      # default dir = ./testdata/compile
karkain test --compile <corpus>   # explicit corpus directory
```

- Exit `0` when every case passes (or is skipped for lack of a C toolchain).
- Exit `4` (`ExitTest`) when any case fails.
- Missing/invalid manifest ⇒ per-case failure summary (never a silent pass).

## 8. Self-Tests (`pkg/cli/compile_corpus_test.go`)

1. `TestLoadCompileManifest_BundledCorpus` — 15 cases, both expects, files exist.
2. `TestLoadCompileManifest_DeterministicOrder` — two loads identical.
3. `TestLoadCompileManifest_RejectsMissingFile` — missing file ⇒ error.
4. `TestLoadCompileManifest_RejectsBadExpect` — unknown expect ⇒ error.
5. `TestRunFailCases_AllClassify` — every fail case classifies (no gcc needed).
6. `TestRunFailCase_UnexpectedCleanFileFails` — clean file against a fail
   expectation must NOT pass (matcher really discriminates).
7. `TestRunPassCases_CompileOrSkip` — pass cases compile under gcc or SKIP.
8. `TestRunCompileCorpus_NoFailures` — full corpus run has 0 failures.
9. `TestCLI_TestCompile_E2E` — built binary, `test --compile` exits 0 with
   summary, "0 failed".
10. `TestCLI_TestCompile_FailingCorpusExitsTestStatus` — mismatched-code corpus
   exits 4 with "no diagnostic matched".

## 9. Verification (actually executed)

```
go build ./...                PASS
go vet ./...                  PASS (affected packages)
go test ./... -count=1        PASS — all 27 packages green
  incl. pkg/cli (KTF-002 corpus tests + prior E2E) green
  incl. pkg/bootstrap (240s self-hosting path) green
karkain test --compile ./pkg/cli/testdata/compile
                              PASS — 15 cases, 15 passed, 0 failed, 0 skipped
karkain test --compile <mismatch-corpus>
                              exit 4 — "no diagnostic matched" (verified)
```

## 10. Files Changed

| File | Change |
|---|---|
| `pkg/testing/compile.go` (new) | KTF-002 model + SummarizeCompile + SortCompileByID |
| `pkg/cli/compile_corpus.go` (new) | manifest loader, diagnostics capture, pass/fail runner, corpus runner |
| `pkg/cli/compile_corpus_test.go` (new) | 10 self-tests |
| `pkg/cli/testdata/compile/manifest.json` (new) | bundled corpus manifest |
| `pkg/cli/testdata/compile/pass/*.kark` (new) | 8 compile-pass fixtures |
| `pkg/cli/testdata/compile/fail/*.kark` (new) | 7 compile-fail fixtures |
| `cmd/karkain/main.go` | `test --compile [dir]` flag + dispatch + help |
| `docs/audit/KTF-002-REPORT.md` | this document |

## 11. Explicitly NOT Implemented (future)

- Diagnostic **snapshots** (golden files with exact text) — the corpus asserts
  code class + message substrings, not byte-exact snapshots.
- Codegen/backend-specific fail cases (SSA/MLIR/GCC-stage failures).
- Property/fuzz-style compile corpora, GPU/NPU cross-compile fail cases.
- `--update` golden regeneration tooling.
- Cross-platform corpus runs (Windows/macOS/Linux CI matrix).

## 12. Limitations

- Pass-case codegen requires a C toolchain; without one the honest result is
  SKIP (the bundle's fail cases still run and classify).
- The corpus exercises the single-file pipeline; project/module-scoped fail
  cases (dependency resolution, workspace scope) are covered by the P2/CLI E2E
  suites instead.
- Message matching is case-sensitive substring matching (deliberate — codes are
  the primary key, messages are the guard).