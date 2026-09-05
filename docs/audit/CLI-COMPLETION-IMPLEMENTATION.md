# Karkain CLI Completion — Implementation Report

Phase gate: post-Phase 79 engineering. Executes the **Karkain Complete CLI
Toolchain Implementation** prompt (B-batch). All batches shipped on `develop`
and `main`, fully regression-tested. Companion matrix:
`docs/audit/CLI-COMPLETION-MATRIX.md`.

## Batch summary

| Batch | Deliverable | Commit | Files |
|-------|-------------|--------|-------|
| B1a | Exit-code scheme (§23) + `--target` validation | `5cfbf03` | `pkg/cli/exitcodes.go`, `pkg/cli/exitcodes_test.go`, `cmd/karkain/main.go` |
| B1b | `clean` + workspace family + `.kark` normalization | `685c474` | `pkg/cli/clean.go`, `pkg/cli/workspace.go`, `pkg/pm`, `pkg/bootstrap/args_test.go` |
| B1d | `bench` subcommand | `e2759aa` | `pkg/cli/bench.go`, `pkg/cli/bench_test.go` |
| B2 | `lint` + `explain` + error-code registry | `be2525a` | `pkg/diagnostics/codes.go`, `pkg/cli/lint.go`, `pkg/cli/explain.go`, `pkg/cli/lint_explain_test.go` |
| B3 | workspace `remove`/`lint`/`graph` completion | `009c0d2` | `pkg/pm/workspace.go`, `pkg/cli/workspace.go`, tests |
| B4 | `target`/`config` commands, `--json` audit/verify, mojibake fix | `017cec8` | `pkg/cli/config_target.go`, `cmd/karkain/main.go`, tests |

## Details

### B1a — Exit codes + target validation
Seven exit codes implemented as `cli.ExitSuccess..ExitEnv` (0..6). Every
dispatcher exit path classifies: usage → 2, compile → 3, test → 4, package → 5,
infrastructure (no supported C compiler detected) → 6. `--target` accepts
`native|c23|wasm32-wasi` in both `--target X` and `--target=X` forms; any other
value is a hard usage error. Empty target = default (`native`, constant
`cli.DefaultTarget`). E2E asserts codes through the real binary.

### B1b — clean + workspace + normalization
`karkain clean [path] [--all]` removes only compiler-generated artifacts
(source-anchored: `<base>.c`, `<base>.exe`, `<base>`, shaders, test binaries).
It never removes `.kark` sources, manifests, or the lockfile, and prints a
sorted deterministic report. `workspace list/build/test/check/run/clean`
wired on top of `pm.WorkspaceOrder`. Stale `.kar` references normalized to
`.kark` (only a normative doc line now remains by design).

### B1d — bench
`karkain bench [path]` discovers `bench_`-prefixed functions in
`*_bench.kark` / `*_test.kark` files, compiles each in shared module scope,
measures a single-run wall-clock time, and prints a `DURATION` table sorted by
function name. Empty dir exits 0; non-`.kark` plain files are rejected.
Parse/sema failures → exit 3. The bench harness is a genuine measurement path,
not a stub.

### B2 — lint, explain, error registry
`pkg/diagnostics` owns the stable compiler classes `E-K-SYN/RES/BRW/SEM/TYP/
CG/PKG/ENV`. `karkain lint <file.kark>` runs the complete front end — parser,
macro expansion, name resolution, kernel sema, and the borrow checker (which
`check` does not run) — short-circuiting after syntax errors and tagging every
finding with its class. `karkain explain <code>` documents compiler and
package-manager codes; `explain --list` enumerates them in sorted order. All
`E-PKG-*` explain entries reference `pkg/pm` constants so they cannot drift.

### B3 — workspace completion
`workspace remove <path>` (relative or absolute; fresh `pm.RemoveWorkspaceMember`)
rounds out member management. `workspace lint` runs the full lint pass over each
member in dependency order (deterministic stop at first failure, exit 3).
`workspace graph` renders the member dependency graph in topological order with
cycle reporting (`E-PKG-WS-CYCLE`). Both spellings (`workspace` and
`pkg workspace`) share one implementation and behave identically.

### B4 — target/config + JSON + mojibake
`karkain target` lists the closed target set with the default;
`karkain config` prints the effective codegen configuration. `pkg audit --json`
and `pkg verify --json` emit pure machine-readable JSON (no human text mixed)
while preserving the exit-code contract (verify failure → 5). A corrupted
arrow sequence in `pkg audit` human output was repaired (`->`), one-line diff.

## Honest boundary (no fake commands)

`ir`, `bootstrap`, `selfhost`, and `profile` are recorded as **NOT provided**:
no genuine dump/package/self-host-driver/profile mechanism exists in the
pipeline after this work, and fabricating "not implemented" printers would be
stub commands. Registry-gated paths (`pkg search`, `pkg info`, git/registry
fetch) surface explicit availability errors exactly as before.

## Verification

- `go test ./... -count=1` — all packages green (including the 240s+
  self-hosting bootstrap path and 180s+ CLI E2E suite).
- Every batch: `develop` commit → `main` fast-forward merge → both pushed
  (`origin`), per AGENTS.md branch workflow.
- Windows path safety held throughout (temp-dir E2E, absolute-member removal,
  `\`-safe file handling).

## Next

KTF-002 — compile-pass/compile-fail corpus + diagnostics capture (queued
behind this work).