# KTF-001 — Karkain Native Testing Foundation

Status: COMPLETE
Baseline: P2 Package Ecosystem Maturity (Gates 1–6 + follow-up) complete.
Branches: `develop` → `main` (AGENTS.md workflow).
Extension: `.kark`.

## 1. Mission Summary

KTF-001 establishes the first Karkain-native testing foundation: language-level
assertions, a structured toolchain test model, deterministic discovery, per-test
execution with structured results, deterministic filtering, correct exit
status, and package/workspace compatibility — without creating a second
architecture and without rewriting any working subsystem.

## 2. Repository Inspection (evidence)

### Compiler pipeline (unchanged by KTF-001)

```
.kark → Lexer (pkg/lexer) → Parser AST (pkg/parser) → MacroExpansion
      → Borrow Checker (pkg/sema) → Kernel Analyzer (check only)
      → codegen (pkg/codegen, AST→C23) → gcc/clang/MSVC → native binary
```

- The default compile path is AST→C23 codegen. SSA (`pkg/ir/ssa`) is embedded
  per-function in codegen with graceful fallback to legacy emission.
- `codegen.GenerateAndCompile` runs the produced binary when `RunAfter` is set,
  and returns the subprocess exit error — this is the hook the test runner uses
  to detect genuine assertion failures.

### CLI

- `cmd/karkain/main.go` dispatches `build/run/check/transpile/test/lsp` and
  top-level `pkg ...` commands. `test` already existed and invoked
  `cli.TestCommand`.
- Exit convention: `CommandResult{ExitCode, Message}`; 0 = success, non-0 =
  failure. `main()` calls `os.Exit(result.ExitCode)`.

### Pre-existing test runner (reconciled, NOT rewritten)

- `cli.TestCommand` discovered `*_test.kark` files (`findTestFiles`) and ran
  `cli.runSingleTestFile`, which discovers `test_`-prefixed functions
  (`discoverTestFunctions`) and compiles a synthetic mini-program per test.
- **Gap:** the runner counted per-FILE results (not per-test), printed no
  per-test pass/fail in default mode, had no filtering, and COULD NOT detect a
  logic failure — a test that compiled and ran "successfully" always passed,
  because the language had **no assertion primitive**.

### Language / stdlib

- No `assert`/`assert_eq`/`assert_ne` anywhere. Std `std/` files are stubs;
  `stdlib/` modules are handwritten `.kark` C interop bindings.
- Builtins (`print`, `len`, `str`, ...) are dispatched in codegen's `genExpr`
  CallExpr block and whitelisted in `sema/builtinNames` for `check`.

## 3. Architecture Reconciliation

| KTF requirement | Decision | Rationale |
|---|---|---|
| Native test declaration | Reuse `test_`-prefixed function convention | Existing, tested, deterministic; §"do not force syntax if a valid equivalent representation exists" |
| Test identity | `<file-basename>:<function>` | Stable, deterministic, file-qualified |
| Assertions | Language-level builtins lowering to C runtime | Fully native; no third-party framework; works in `run`/`test`/`check` |
| Test model | New `pkg/testing` (TestCase/TestResult/Status/Failure/Summary/Filter) | Toolchain-owned, reusable by future KTF phases |
| Discovery | Deterministic, name-sorted | Files sorted + per-file name-sorted; no map/FS-order dependency |
| Execution | Per-test synthetic mini-program compiled via existing codegen | Preserves existing architecture; failure = non-zero exit |
| Filtering | Substring over ID/Name (`--filter`) | Minimal, deterministic |
| Exit status | 0 all pass; non-0 any fail OR infrastructure failure | Stable contract |
| Package/workspace | `test` reuses `projectModuleSources` (P2 dep resolution); `WorkspaceTest` unchanged | No second ecosystem |

Language/toolchain boundary is preserved: the language owns test declaration +
assertions; the toolchain owns discovery/execution/filtering/aggregation/reporting.

## 4. Native Assertion Foundation

Added three language-level builtins backed by the embedded C23 runtime:

```karkain
assert(cond)                // fail iff cond is falsy
assert(cond, "message")
assert_eq(actual, expected) // fail iff actual != expected
assert_eq(a, e, "message")
assert_ne(actual, expected) // fail iff actual == expected
assert_ne(a, e, "message")
```

On failure the runtime prints structured diagnostics to **stderr** and calls
`exit(1)`, so a compiled test program exits non-zero and the runner records a
genuine failure:

```
assertion failed: assert_eq: assert_eq(actual, expected)  [loc]
  expected: 5
  actual:   4
```

Implementation notes:

- `pkg/codegen/codegen.go`: C runtime functions `karkain_assert`,
  `karkain_assert_eq`, `karkain_assert_ne`, `karkain_assert_report` (placed
  after `is_truthy`/`values_equal`/`karkain_str` so all dependencies precede
  them). Values render via `karkain_str` to stderr, so structured output is
  captured cleanly.
- `pkg/codegen/codegen.go`: `genAssertCall` dispatch in `genExpr`
  CallExpr block. Works in both the SSA path (via `rawcExpr` fallback) and the
  legacy path. Optional trailing string literal = user message; otherwise the
  canonical expression text is the diagnostic label.
- `pkg/sema/resolve.go`: `assert/assert_eq/assert_ne` added to `builtinNames`
  so `karkain check` does not flag them as undefined.
- The SSA lowerer intentionally does NOT register these as `callableBuiltin`:
  `assert(...)` calls fall through to `rawcExpr` → `genExpr`, so both codegen
  paths share one dispatch.

## 5. Test Model (`pkg/testing`, new package)

```go
type TestCase  struct { ID, Name, File string; Line int; Kind Kind }
type TestResult struct { ID, Name string; Status Status; Duration time.Duration;
                         Failure *Failure; Stdout, Stderr string }
type Failure   struct { Message, Expected, Actual, Location string }
type Summary   struct { Total, Passed, Failed, Skipped int }
```

- Status: `PASS`, `FAIL`, `SKIP` (SKIP reserved; not forced in KTF-001).
- Kind: `UNIT` implemented; the Kind seam permits future kinds.
- `Filter` (substring, order-preserving, deterministic), `SortByID`,
  `Summarize`, `ParseAssertionFailure` (extracts expected/actual/location from
  the assertion stderr format).
- Output capture: `codegen.Config` gained `Stdout`/`Stderr io.Writer`
  (nil ⇒ process streams) so the runner captures per-test output without
  touching global state.

## 6. Runner (`pkg/cli`)

- New `pkg/cli/testing.go`: `discoverTestCases`, `testIdentity`,
  `runSingleTest`, `runTestsInFile`, `testFileModuleScope` reuse the existing
  mini-program + module-scope mechanism.
- `TestCommand` now delegates to `TestCommandFiltered(path, cfg, verbose,
  filter)`; deterministic file ordering (`sort.Strings`) + per-file name-sorted
  results.
- Human-readable default output:

```
  PASS test_add    2.201s
  FAIL test_fail   1.465s: assertion failed: assert_eq: ...

=== Test Summary: 2 tests, 1 passed, 1 failed, 0 skipped ===
```

- Exit status: `0` all selected tests passed; `1` any failure; unmatched filter
  exits `0` with `No tests matched filter '<pattern>'.`; no tests ⇒ exit `0`.

## 7. CLI

```bash
karkain test                 # run all tests in the current package/project
karkain test <path>          # run tests in a file or directory
karkain test --filter <substr>  # deterministic substring filter over ID/Name
karkain test --filter=foo    # same, equals form
```

`--filter` is wired in `cmd/karkain/main.go` (both space and `=` forms).

## 8. Package / Workspace Integration

- The runner reuses P2 `projectModuleSources` (local/workspace deps + sibling
  modules, `func main` excluded) as the shared compile scope, so tests can call
  the code they target — no language import needed, no new resolver.
- `WorkspaceTest` (`karkain pkg workspace test`) is unchanged and keeps
  working because `TestCommand`'s contract (ExitCode) is preserved.
- Registry/git deps resolve through the same P2 `DependencySources` path used
  by `build`/`run`/`check`; `karkain test` does not bypass dependency
  validation or security checks. No arbitrary shell; process execution uses the
  existing codegen subprocess path.

## 9. Determinism Requirements

- Discovery: files sorted lexically; cases sorted by ID within a file.
- Identity: `<file-basename>:<function>`.
- Aggregation/reporting: sequential, in deterministic order.
- No map-iteration or filesystem-order dependence. Parallel execution is a
  future feature (deferred, documented).

## 10. Self-Tests (KTF tested)

- `pkg/testing/model_test.go` (8 tests): Summarize, status helpers, Filter
  order-preservation, SortByID determinism, ParseAssertionFailure
  (expected/actual/location/no-match).
- `pkg/cli/testing_test.go` (14 tests): discovery (finds/sorts/ignores/
  stable), filter select/unmatched/empty, aggregation, assert/assert_eq/
  assert_ne pass+fail (E2E via GCC), structured expected/actual capture,
  captured stdout, TestCommand exit status (pass=0/fail≠0), filter E2E,
  unmatched-filter exit.

## 11. Verification (actually executed)

```
go build ./...                PASS
go vet ./...                  PASS (affected packages)
go test ./... -count=1        PASS — all 27 packages green
  incl. pkg/bootstrap 240s (self-hosting/bootstrap path intact)
  incl. pkg/pm (P2 regression green)
  incl. pkg/cli (P2 CLI E2E green)
karkain check/run with assert builtins: PASS (manual, plus covered by tests)
karkain test --filter: PASS (manual + tests)
```

## 12. Regression

`build`, `run`, `check`, `test`, package operations, workspace operations,
registry/git/cache/lock tests — all green in the full `go test ./...`. The
bootstrap stage (`pkg/bootstrap`, 240s) passed, confirming the self-hosting
path is unbroken.

## 13. Bootstrap / Self-Hosting

`go test ./pkg/bootstrap/... -count=1` ran a full stage-0/stage-1 pipeline
build (240.682s) and passed. KTF changes are entirely additive (assertion
builtins + runner) and do not touch `src/compiler/*.kark` or the bootstrap
pipeline.

## 14. Explicitly NOT Implemented (future KTF phases)

coverage, fuzzing, property-based testing, benchmarks, sanitizers, debugger,
distributed testing, sandboxing, compile-pass/compile-fail frameworks,
diagnostic snapshots, conformance, bootstrap/self-hosting test frameworks, GPU/
NPU/AI/Quantum testing, public testing service, external CI, IDE protocol.
The model (Kind, Status, TestCase/TestResult) is the extension seam for these.

## 15. Files Changed

| File | Change |
|---|---|
| `pkg/codegen/codegen.go` | assertion C runtime + `genAssertCall` dispatch + `Config.Stdout/Stderr` + `io` import |
| `pkg/sema/resolve.go` | `assert/assert_eq/assert_ne` in `builtinNames` |
| `pkg/cli/testing.go` (new) | discovery, per-test runner, structured result printing |
| `pkg/cli/commands.go` | `TestCommandFiltered` + `TestCommand` wrapper + structured flow |
| `cmd/karkain/main.go` | `--filter` flag (both forms) + test dispatch |
| `pkg/testing/model.go` (new) | TestCase/TestResult/Status/Failure/Summary/Filter/SortByID |
| `pkg/testing/failure.go` (new) | ParseAssertionFailure |
| `pkg/testing/model_test.go` (new) | model unit tests |
| `pkg/cli/testing_test.go` (new) | KTF self-tests (14) |
| `docs/audit/KTF-001-REPORT.md` | this document |

## 16. Current Limitations

- Source location on assertion failures is limited to the expression/message
  label (the mini-program path has no retained `.kark` line map); a location
  string is accepted by the runtime `[loc]` slot for future wiring.
- Per-test compile via GCC is sequential and takes ~1–2 s each (existing
  architecture); parallel execution is future work.
- Test declaration is the `test_` prefix convention, not a new `test "name"`
  grammar keyword.
- `std/stdlib` testing modules were not added (assertions are language
  builtins); a future `stdlib/testing` façade over the builtins remains an
  option.