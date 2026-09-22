# Phase 140 BASELINE — Debugger Integration

Date: 2026-09-22. Target: GA-3 third step (after 139).

## 1. Debugger availability

- Dev host (Windows x86_64): **gdb 15.2 present** (MSYS2 ucrt64), **no lldb**.
  The live-debugger leg is gdb-based here; CI ubuntu-latest ships gdb too.
  lldb stays a documented alternative (same MI/batch shape, unwired).
- `karkain debug` (Phase 112) already exists: enter/leave tracing,
  Go-engine only. Phase 140 adds `karkain dbg` beside it (live gdb walk),
  not over it.

## 2. Debug-info model (Phases 84/85/104)

- Karkain-owned DWARF-4 sections + self-hosted reader + text dump exist,
  but there is **no ELF/PE container writer**: real debuggers work on the
  **gcc C-transpile path** (`build -g` → gcc `-g` → own DWARF). Phase 140
  therefore drives gdb on `-g` binaries built from generated C; the
  Karkain-owned sections stay the future container story (documented).
- `cfg.Debug` already appends `-g` (hostNativeFlags). No codegen change
  needed to get symbols; function symbols are the deterministic
  `karkain_user_*` namespace (Phase 83).

## 3. Slice plan

- **140A (`karkain dbg`)**: temp-sandbox `-g` build (Go engine; kcc deferred
  exactly like the Phase 112 boundary), gdb batch
  (`rbreak ^karkain_user_`, run, repeated bt/continue, max-depth wins),
  frame parse (`#N ... in FUNC (...) at FILE:LINE`), demangle to Karkain
  names, `karkain_dbg trace:` output. gdb missing → ExitEnv(6) with an
  install hint, never a fake trace. Gate asserts the exact top-frame
  Karkain sequence on a 3-deep straight-line program.
- **140B (VS Code)**: `editors/vscode/launch.json` template (cppdbg/miMode
  gdb launch + attach) + `karkain.debug` command in extension.js
  (build -g, then startDebugging), package.json declarations. Tested like
  the existing 3/3 extension tests (static contract, no live VS Code).
- **140C**: phase140 gate, docs (debugging page wired + honest boundaries),
  full regression, audit report, develop→main→push.

## 4. Non-goals (documented, NOT defects)

- No `#line` directives in generated C (byte-change surface for the
  determinism gates); frames report Karkain function names + honest
  C file:line locations.
- No lldb wiring on this host; no DAP server (that's Phase 153).
- No kcc-engine dbg (same boundary as Phase 112 tracing).
