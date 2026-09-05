# Phase 82 — IDE Integration

How Karkain plugs into editors. Three surfaces exist: a real **LSP server**,
a **VS Code extension** shelling to the CLI, and **`karkain ide info`** as the
editor/IDE probing contract. Everything described here ships and is validated
by tests; nothing is advertised as `PLANNED` while also being wired.

## Language server (`pkg/lsp`, `karkain lsp`)

A real LSP implementation over stdio with Content-Length framing
(`pkg/lsp/server.go`, `lsp.NewServer(in, out, closer)`), exercised by
`pkg/lsp/lsp_test.go`. The CLI serves it at `karkain lsp` (alias
`language-server`).

The current protocol surface is a foundation (initialize/initialized,
shutdown/exit handshake, basic parity checks). Diagnostic publication and
document sync are the next milestones.

## VS Code extension (`editors/vscode`)

A workspace-installable extension (`vsce package` → `code --install-extension
*.vsix`) that provides:

- TextMate grammar scoped `source.karkain` (`.kark`).
- Comment/bracket/folding configuration matching the Karkain lexer.
- Commands: `Karkain: Check File` (`check --format=json` + diagnostics),
  `Karkain: Compile File` (`build`), `Karkain: Run File` (`run`),
  `Karkain: Format Document` (`fmt` + `registerDocumentFormattingEditProvider`).
- Settings: `karkain.compilerPath`, `karkain.formatOnSave`.

Structure is validated by `pkg/cli/vscode_extension_test.go` (manifest + entry
point + grammar + configuration), and `node --check extension.js` passes.

### Explicitly NOT done (no faking)

- No Problems-panel wiring yet (diagnostics surface via the notification
  channel until the LSP lands diagnostics).
- No autocomplete / semantic tokens — deferred to LSP maturity.

## LiteIDE-style editors — the bridge contract

Editors without a dedicated extension use `karkain ide info`:

```json
{
  "schemaVersion": 1,
  "language": { "id": "karkain", "extensions": [".kark"], "defaultFilename": "main.kark" },
  "projectDetection": { "manifest": "karkain.toml", "marker": "main.kark" },
  "toolchain": { "check": ["karkain","check","<file>"], "checkJSON": ["karkain","check","--format=json","<file>"],
                 "build": ["karkain","build","<file>"], "run": ["karkain","run","<file>"],
                 "test": ["karkain","test","<path>"], "compileCorpus": ["karkain","test","--compile","<dir>"],
                 "format": ["karkain","fmt","<file>"], "formatCheck": ["karkain","fmt","--check","<file>"],
                 "lint": ["karkain","lint","<file>"], "lsp": ["karkain","lsp"] },
  "diagnostics": { "format": "json", "schema": "karkain-diagnostics-v1", "code": "E-K-*" },
  "exitCodes": { "success": 0, "failure": 1, "usage": 2, "compile": 3, "test": 4, "package": 5, "env": 6 }
}
```

The full render is served by the CLI and asserted by
`pkg/cli/phase82_cli_test.go`. Diagnostics respect the
`karkain-diagnostics-v1` schema in `PHASE-82-TOOLCHAIN-CONTRACT.md`.

## Verified integration flow

1. `karkain ide info` → an editor discovers the toolchain argv arrays.
2. `check --format=json` → structured, line/column-anchored diagnostics.
3. `fmt --check` / `fmt` → canonical formatting with semantic preservation.
4. `lsp` → the language server for features that need a persistent session.