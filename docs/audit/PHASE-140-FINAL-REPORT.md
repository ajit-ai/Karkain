# Phase 140 FINAL REPORT — Debugger Integration

Verdict: **COMPLETE**. GA-3 third step (after 139).

## A. `karkain dbg` — live gdb walk (slice 140A)

- `pkg/cli/dbg.go` (new): `DbgCommand` stages the source into a temp
  sandbox (beside-source `.c` debris never touches user code), runs the
  RunCommand gate prologue (syntax → borrow → semantic; same codes),
  links a `-g` binary through the **Go engine** (kcc deferred — the exact
  Phase 112 boundary), then walks it under gdb batch:
  `rbreak ^karkain_user_` + run + 10× bt/continue, deepest stack wins.
- Frames parse `#N [addr in] FUNC (...) [at FILE:LINE]`, demangle the
  `karkain_user_` namespace, render `karkain_dbg trace:` + numbered
  Karkain frames with honest locations. gdb missing → ExitEnv(6) with an
  install hint; 120s walk timeout; one retry on transient gcc failure
  (Windows AV-lock class, Phase 114 precedent); missing file → ExitUsage;
  semantic errors → ExitCompile before any build.
- Debug mode already emitted `#line` directives (Phase 55b), so gdb
  locations name Karkain units — but lines are **assembly-relative**
  under sibling-join (proven: identical program reports 6/17/28 in both
  isolated and repo-root builds). The gate therefore pins the exact
  function SEQUENCE (`inner <- outer <- main`) and location presence/
  shape, never exact lines. No `#line` changes were made (determinism
  byte-surface untouched).
- Wired in `cmd/karkain/main.go` (`case "dbg"`, early dispatch like
  `debug`) + help line.

## B. VS Code surface (slice 140B)

- `editors/vscode/launch.json` + `tasks.json` (new templates): cppdbg/
  miMode-gdb launch with pre-launch `-g` build task + attach config.
  JSONC (comment headers); the gate strips full-line comments for
  strict validation.
- `extension.js`: `runDebug()` (build `-g` via execFile — failure never
  launches — then `startDebugging` with a cppdbg/gdb config) +
  `karkain.debug` registration; `package.json`: command, activation,
  `karkain.debuggerPath` setting.
- Existing command-list assertions extended; new
  `TestVSCodeExtension_DebugWiring` (manifest + code + templates).

## C. Gates + CI + docs

- `pkg/cli/phase140_debug_test.go` (new, 4 subtests): exact top-3 frame
  sequence, run-to-run determinism, missing-file usage, semantic-error
  compile rejection. All PASS (gdb-gated skip when absent).
- CI: `TestPhase140|TestVSCodeExtension` step.
- Docs: `tools/debug.rst` gains the `karkain dbg` section (format,
  assembly-relative caveat, VS Code wiring, lldb/DAP boundaries).

## D. Regressions green

Phase 140 gate, all 4 VS Code extension tests, `go vet`, `go build`,
Sphinx `-W`. `karkain debug` (112) untouched (no shared code changed).

## E. Boundaries (documented, NOT defects)

- Go-engine only; kcc-engine dbg deferred (mirrors Phase 112).
- No source-line stepping/variable inspection (DAP is Phase 153).
- lldb unwired (same batch shape; no lldb on the dev host).
- Frame lines assembly-relative under sibling-join (documented in gate
  + docs); function sequence always exact.
- Walk depth cap 10 breakpoint rounds; 120s timeout.
