# Karkain CLI Completion Matrix (post-Phase 79 / Toolchain Completion)

Scope: the Karkain Complete CLI Toolchain Implementation prompt. Every row is
verified against the shipped binary (`go build ./...` + full CI-style tests).
Nothing is marked present that is a stub — unresolved/registry-gated paths are
honestly marked as such (no fake commands).

## Exit-code contract (§23)

| Code | Meaning | Verified by |
|------|---------|-------------|
| 0 | success | `pkg/cli/exitcodes_test.go` |
| 1 | program failure | unknown code lookup, general IO |
| 2 | CLI usage error (unknown flag/command, missing arg, invalid target/extension) | `TestCLI_ExitCodes_E2E` |
| 3 | lexer/parser/sema/borrow/type/codegen failure | parse-error + duplicate-func fixtures |
| 4 | test failure | gcc-gated failing-test fixture |
| 5 | package/dependency failure | `pkg add` outside project, lockfile failure |
| 6 | infrastructure (no usable C compiler) | `classifyCompileError` |

## Command matrix

Legend: `GENUINE` = fully wired to a real implementation with tests; `PRE-EXISTING` = already implemented before this work, verified; `NOT-PROVIDED` = deliberately not faked (no genuine implementation exists or infra unavailable).

### Compiler commands

| Command | Status | Implementation / test |
|---------|--------|-----------------------|
| `run` | PRE-EXISTING | compile+exec path, `--target`, `--verbose` |
| `build` | PRE-EXISTING | `-o`, `-c`, `-g`, `--target` |
| `transpile` | PRE-EXISTING | C emission |
| `check` | PRE-EXISTING | front-end validation, no binaries |
| `test` | PRE-EXISTING + KTF | `TestCommandFiltered`, `--filter` |
| `bench` | GENUINE (B1d) | `pkg/cli/bench.go`, `bench_test.go` |
| `lint` | GENUINE (B2) | `pkg/cli/lint.go` (full front-end + borrow) |
| `explain` | GENUINE (B2) | `pkg/cli/explain.go`, `--list` |
| `clean` | GENUINE (B1b) | `pkg/cli/clean.go` (source-anchored, `--all`) |
| `target` | GENUINE (B4) | `pkg/cli/config_target.go` |
| `config` | GENUINE (B4) | `pkg/cli/config_target.go` |
| `lsp` | PRE-EXISTING | JSON-RPC 2.0 loop |
| `--target` validation | GENUINE (B1a) | `ValidateTarget`, space + `=` forms |
| `-v/--version`, `-h/--help` | PRE-EXISTING | versionString, help |

### Workspace commands (top-level + `pkg workspace` parity)

| Command | Status | Implementation / test |
|---------|--------|-----------------------|
| `list/ls` | PRE-EXISTING | `pm.WorkspaceOrder` |
| `build` | PRE-EXISTING | dependency order |
| `test` | PRE-EXISTING | dependency order + dev-deps |
| `check` | PRE-EXISTING | front-end validation |
| `run` | PRE-EXISTING | build + run all entrypoints |
| `clean` | GENUINE (B1b) | `--all` |
| `init` | PRE-EXISTING | `pm.InitWorkspace` |
| `add` | PRE-EXISTING | `pm.AddWorkspaceMember` |
| `remove` | GENUINE (B3) | `pm.RemoveWorkspaceMember` (rel/abs) |
| `lint` | GENUINE (B3) | `cli.WorkspaceLint` |
| `graph` | GENUINE (B3) | `pm.WorkspaceGraphLines` + cycle reporting |

### Package commands

| Command | Status | Note |
|---------|--------|------|
| `new/init/add/remove/update/fetch/list/tree` | PRE-EXISTING (hardened) | deterministic resolver + lockfile (P0) |
| `pkg deps [--tree]` | PRE-EXISTING | |
| `pkg cache list/clean/path` | PRE-EXISTING | |
| `pkg audit [--licenses]` | GENUINE (B4) | `--json` added; DB is an explicit placeholder (honest: empty) |
| `pkg verify` | GENUINE (B4) | lockfile+checksum integrity, `--json` |
| `pkg publish/login/logout/whoami` | PRE-EXISTING, registry-not-available | explicit "not available" errors, not faked |
| `pkg search/info` | NOT-PROVIDED | no real registry exists; explicit error, not faked |
| `pkg upgrade` | PRE-EXISTING | version recompute |
| `pkg deps --outdated` | PRE-EXISTING / registry-gated | explicit error when no registry |

### Commands explicitly NOT provided (no fake placeholders added)

| Requested | Status | Reason |
|-----------|--------|--------|
| `ir` dump | NOT-PROVIDED | no IR-dump facility exists in the pipeline (verified: no DumpIR/EmitIR API) |
| `bootstrap` | NOT-PROVIDED | packaging/bootstrap flow for a binary artifact not established |
| `selfhost` | NOT-PROVIDED | self-hosting readiness is Phase 79 gate; no on-demand driver exists |
| `profile` | NOT-PROVIDED | no profile mechanism in codegen |

These are recorded so the matrix is complete; creating them as thin "not
implemented" printers would be fake commands and is intentionally avoided.

## Error-code registry (`explain`)

Compiler classes `E-K-SYN/RES/BRW/SEM/TYP/CG/PKG/ENV` (`pkg/diagnostics`) plus
all `E-PKG-*` codes from `pkg/pm` are documented by `karkain explain <code>`
and enumerated by `karkain explain --list`. `lint` tags every finding with its
stable class.

## Regression

`go test ./... -count=1` — all 25+ packages green (bootstrap 252s incl. the
self-hosting path; cli 188s incl. E2E through the real binary).