# Phase 86 — Developer Toolchain

## Status: COMPLETE

**Date:** 2026-09-06  
**Scope:** Strengthening the Karkain developer CLI workflow

---

## Objective

Strengthen the Karkain developer experience around the existing compiler and CLI
by filling genuine gaps in the toolchain: test timeout safety, multi-file
formatting, LSP formatting support, and lint exit codes.

---

## Changes

### 1. Test Timeout (`pkg/cli/testing.go`)

Added a 30-second timeout to `runSingleTest` to prevent hanging tests from
blocking the test runner indefinitely. Tests that exceed the timeout are
terminated with a clear timeout failure message.

```go
const defaultTestTimeout = 30 * time.Second
```

The timeout uses a goroutine + select pattern: the test execution runs in a
goroutine, and the main goroutine selects between the completion channel and
the timeout timer.

### 2. Multi-File Formatter (`pkg/cli/formatter.go`)

Extended `FormatCommand` to accept directories in addition to single files.
When given a directory, it recursively finds all `*.kark` files and formats
each one.

- `karkain fmt .` — formats all `.kark` files in the current directory
- `karkain fmt src/` — formats all `.kark` files recursively
- `karkain fmt --check .` — checks if all files are formatted (CI mode)

The directory mode reports:
- Number of files formatted (or needing formatting)
- Per-file errors if any occur
- Exit success only if all files are already formatted (in check mode)

### 3. Exported Canonicalize (`pkg/cli/formatter.go`)

Renamed `canonicalize` to `Canonicalize` (exported) so the LSP package can
use the same formatting logic without duplicating code.

### 4. LSP Formatting (`pkg/lsp/handler.go`)

Added `textDocument/formatting` support to the LSP server:

- Declares `formattingProvider: true` in server capabilities
- Routes `textDocument/formatting` requests to `handleFormatting`
- Uses `cli.Canonicalize` (the shared formatting logic)
- Returns a single `TextEdit` replacing the entire document, or an empty
  array if the document is already formatted

This enables format-on-save in editors that support LSP formatting.

### 5. ExitLint Code (`pkg/cli/exitcodes.go`)

Added `ExitLint = 7` for lint-specific failures. This distinguishes lint
failures (warnings treated as errors) from general compilation failures.

| Code | Constant | Meaning |
|------|----------|---------|
| 0 | ExitSuccess | Command completed successfully |
| 1 | ExitFailure | General/program failure |
| 2 | ExitUsage | CLI usage error |
| 3 | ExitCompile | Compilation failure |
| 4 | ExitTest | Test failure |
| 5 | ExitPackage | Package/dependency failure |
| 6 | ExitEnv | Infrastructure failure |
| 7 | ExitLint | Lint-specific failure |

---

## Files Changed

| File | Change |
|------|--------|
| `pkg/cli/testing.go` | Added test timeout (30s) with goroutine+select pattern |
| `pkg/cli/formatter.go` | Multi-file support, exported `Canonicalize` |
| `pkg/cli/exitcodes.go` | Added `ExitLint = 7` |
| `pkg/lsp/handler.go` | Added `textDocument/formatting` handler |
| `pkg/lsp/protocol.go` | Added `FormattingProvider` to capabilities, `MethodTextDocumentFormatting` |
| `pkg/cli/phase86_test.go` | **NEW** — 6 focused tests |
| `pkg/lsp/lsp_test.go` | Added 2 LSP formatting tests |

---

## Test Results

### Phase 86 CLI Tests (6/6 PASS)
- `TestFormatCommand_Directory` — recursive directory formatting
- `TestFormatCommand_DirectoryCheck` — check-only mode preserves files
- `TestFormatCommand_DirectoryAllFormatted` — already-formatted directory
- `TestFormatCommand_DirectoryNoKarkFiles` — directory with no .kark files
- `TestCanonicalize_Exported` — exported function works correctly
- `TestExitLintCode` — ExitLint constant is 7

### Phase 86 LSP Tests (2/2 PASS)
- `TestLSP_Formatting` — unformatted document returns edit
- `TestLSP_Formatting_AlreadyFormatted` — formatted document returns empty

### Existing Tests (all PASS)
- `pkg/lexer` — PASS
- `pkg/parser` — PASS
- `pkg/codegen` — PASS
- `pkg/pm` — PASS
- `pkg/lsp` — PASS
- `pkg/cli` (formatting, check, build, lint, validate) — PASS

---

## What Was NOT Done

- No repository-wide audit
- No redesign of existing commands
- No Phase-87 work
- No new compiler pipeline
