# KARKAIN TOOLCHAIN & CLI EVOLUTION — Repository Audit & Implementation Plan

Phase: Consolidated Architecture, Audit & Implementation Master Prompt
Date: 2026-09-03

This document is DELIVERABLES A–D: the repository audit report, CLI capability
matrix, gap analysis, and implementation plan. It precedes any code change.

---

## DELIVERABLE A — Repository Audit Report

### Repository structure (relevant to CLI)

```text
cmd/karkain/main.go    CLI entry point & monolithic command dispatch (835 lines)
pkg/cli/commands.go    Compiler command implementations (run/build/check/test) (507 lines)
pkg/cli/*_test.go      CLI + E2E tests (commands, codegen_profile, bugfix, bootstrap, phase70)
pkg/pm/                Package manager (manager, registry, semver, lock, integrity, cache, audit, auth, workspace)
pkg/lsp/               LSP server (server, handler, protocol) — real, 1600+ lines
pkg/codegen/           Code generation (Config, GenerateAndCompile, C/WGSL/SPIR-V/native/quantum)
pkg/lexer/             Lexer (Token, NextToken, Literal(sourceText))
pkg/parser/            Parser (ParseProgram, Program, nodes)
pkg/sema/              Semantic analysis + borrow checker + kernel analyzer
pkg/ir, ir/ssa         Bytecode + SSA IR
pkg/jit/               Bytecode VM
pkg/backend/           CPU/GPU/NPU backend abstraction + tensor
pkg/math, tensor, npu  Math/Tensor/NPU IR + backends
pkg/diagnostics/       Diagnostic reporter (severity + source span)
pkg/bootstrap/         Self-hosting stage1/stage2 harness
```

### CLI entry point and dispatch

- `cmd/karkain/main.go:main()` is a **single hand-rolled loop** over `os.Args`
  doing: flag parsing (`--verbose`, `-c/--compile-only`, `-g/--debug`,
  `--target`, `-o`, `-v/--version`, `-h/--help`), command detection
  (`build/run/check/transpile/test/lsp/init/add/fetch/pkg`), and dispatch into
  `cli.*Command` or inline handlers.
- `handlePackageCommand(args)` at `main.go:147` is a **huge switch** over all
  `pkg` subcommands (init/add/remove/fetch/update/upgrade/deps/search/info/
  publish/login/logout/whoami/audit/verify/cache/workspace) — 480 lines,
  calling into `pkg/pm`.
- `handleLSP()` at `main.go:628` is a **minimal inline LSP** that does NOT use
  `pkg/lsp` — it returns canned responses for initialize/hover/definition.
- `printHelp()` at `main.go:22` is a static string.

### Compiler pipeline (clipped from Phase 79 audit)

```text
lexer → parser (ParseProgram + ApplyMacroExpansion)
      → sema (BorrowChecker.Check, KernelAnalyzer)
      → codegen (AST→SSA→C23, or WASM via clang --target=wasm32-wasi)
      → gcc/clang → native exe or .kbc (jit)
```

### Package management state

**Implemented, not stubbed.** `pkg/pm` contains real logic with unit tests:
- `manager.go`: init project, manifest parse/write, add/remove dep, resolve
  module, fetch (local/git/registry), list deps, find project root.
- `semver.go`: full semver parse/compare/Satisfies (caret/tilde/range/wildcard).
- `integrity.go`, `lock.go`, `cache.go`, `registry.go`, `audit.go`, `auth.go`,
  `workspace.go`: real code.
Caveat: registry-dependent operations (search/info/latest) require a live
registry endpoint; local+git source fetch works offline.

### LSP state

Two implementations exist:
- `pkg/lsp` — real, full LSP server (protocol.go, handler.go, server.go). Tested.
- `cmd/karkain/main.go:handleLSP` — minimal inline duplicate with canned replies.

**Finding:** the inline duplicate should be removed and `karkain lsp` routed to
`pkg/lsp`.

### Target support

`codegen.Config.Target` (`codegen.go:21`) supports two working targets:
- `native` (default) — C path via `detectCompiler` (gcc, or CC env).
- `wasm32-wasi` — clang `--target=wasm32-wasi` (`codegen.go:2449`).
No other targets reach codegen. `transpile` is an alias of build (C output).

### Test infrastructure

- Go unit tests across `pkg/*`, E2E in `pkg/cli` and `pkg/codegen`.
- `pkg/bootstrap` self-host tests fail (pre-existing, documented separately).
- No formatter, linter, or CLI-golden test infrastructure exists yet.

### Self-hosting state

- `src/*.kark` partial Karkain-written compiler (lexer/parser/codegen/sema/ast).
- Stage1/Stage2/bitwise parity not green (`pkg/bootstrap`).
- CLI changes must not assume bootstrap-only features.

---

## DELIVERABLE B — CLI Capability Matrix

| Command | Exists | Works | Tested | Production Ready | Action |
| ------- | :----: | :----: | :----: | :--------------: | ------ |
| `run <file>` | YES | YES | YES (cli E2E) | YES | Refactor dispatch only |
| `build <file>` | YES | YES | YES | YES | Refactor; keep `-o/-c/-g` |
| `check <file>` | YES | YES | YES | YES | Refactor; keep diagnostics |
| `test <path>` | YES | YES | YES | YES | Refactor dispatch only |
| `transpile <file>` | YES | YES (alias build) | partial | partial | Keep as alias |
| `lsp` | YES | MINIMAL (inline) | no | NO | **Route to pkg/lsp** |
| `pkg init` | YES | YES | YES | YES | Preserve |
| `pkg add` | YES | YES | YES | YES | Preserve |
| `pkg fetch` | YES | YES | YES | YES | Preserve |
| `pkg remove/update/upgrade/deps/search/info/publish/login/logout/whoami/audit/verify/cache/workspace` | YES | partial(user) | tests | partial | Preserve; registry ops need network |
| `init <name>` (top-level) | YES | YES (routes to pkg init) | YES | YES | Preserve |
| `add <pkg>` (top-level) | YES | YES (routes to pkg add) | YES | YES | Preserve |
| `fetch` (top-level) | YES | YES (routes to pkg fetch) | YES | YES | Preserve |
| `fmt` | NO | - | - | - | **IMPLEMENT (P1)** |
| `lint` | NO | - | - | - | **IMPLEMENT (P1)** |
| `clean` | NO | - | - | - | **IMPLEMENT (P1)** |
| `doctor` | NO | - | - | - | **IMPLEMENT (P1)** |
| `new` | NO | - | - | - | **IMPLEMENT (P1)** |

Options matrix (`-o`, `-c/--compile-only`, `-g/--debug`, `--target`,
`--verbose`, `-v/--version`, `-h/--help`) all exist and work today.

---

## DELIVERABLE C — Gap Analysis

### Missing functionality
- Formatter (`fmt`), linter (`lint`), cleanup (`clean`), env doctor (`doctor`),
  distinct project scaffold (`new`).
- Consistent per-command help (`karkain build --help`).
- Consistent exit-code policy (usage vs compile vs env vs test).
- Typo suggestions for unknown commands.

### Duplicate functionality
- **Two LSP implementations** (`main.go` inline vs `pkg/lsp`) — the inline one
  must go.
- `transpile` === `build` (intentional alias; keep).

### Architectural problems
- **Monolithic dispatch** in `main.go` — all parsing, help, pkg dispatch, and
  LSP in one file/loop. Hard to test, grow, or reason about.
- CLI commands not behind a testable interface (only `cli.*Command` funcs are).

### Temporary/incomplete implementations
- `main.go:handleLSP` canned responses (should be removed for `pkg/lsp`).
- `pkg pm` registry operations require network; not offline-fakeable.

### Testing gaps
- No CLI-golden tests for help/usage text.
- No tests for the `pkg`/`init`/`add`/`fetch` top-level routing.
- `existing` commands have some E2E but not per-option coverage.

---

## DELIVERABLE D — Implementation Plan

| Step | Objective | Files | Arch change | Tests | Risk | Acceptance |
|------|-----------|-------|-------------|-------|------|-----------|
| 1 | Validate existing commands | - | - | run cli/pm tests | low | all existing green |
| 2 | Refactor dispatch into command registry | `cmd/karkain/main.go` (+ `cmd/karkain/commands_*.go`) | split parsing & dispatch; command table | help/usage/exit-code tests | med | existing commands identical |
| 3 | Consistent help + exit codes | `cmd/karkain` | per-command help, ExitCode constants | usage tests | med | clear errors, stable codes |
| 4 | Route lsp to pkg/lsp | `cmd/karkain`, remove inline handler | use pkg/lsp.Server.Run | lsp-level test | low | functional parity |
| 5 | Implement fmt | `pkg/formatter`, `cmd/karkain` | lexer/parser-based canonical emit | idempotency + --check tests | med | format(format)==format |
| 6 | Implement lint | `pkg/linter`, `cmd/karkain` | rule engine over AST/sema | rule tests | med | check-only semantics |
| 7 | Implement clean | `cmd/karkain` | remove Karkain-owned artifacts only | path-safety tests | low | no arbitrary deletion |
| 8 | Implement doctor | `cmd/karkain` | inspect real capabilities | env tests | low | only real checks |
| 9 | Implement new | `cmd/karkain` + `pkg/pm` | scaffold distinct from init | scaffold tests | low | dir + files created |

Exit codes (documented, stable): 0 success, 1 compile/exec failure, 2 usage
error, 3 environment/config error, 4 test failure.

---

## Risks

- Refactoring dispatch risks changing existing behavior. Mitigation: preserve
  the exact command surface + options; golden-style usage tests; run full suite.
- `pkg/pm` registry commands depend on network — do not fake; mark as
  network-dependent in doctor/help.
- Formatter must not corrupt semantics — use parser AST (not regex).
- Linter must reuse compiler diagnostics, not duplicate check errors.