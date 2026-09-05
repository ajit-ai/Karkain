# Karkain Language Support (VS Code)

Language support for the Karkain programming language: syntax highlighting,
format-on-save, and command wiring for the `karkain` toolchain.

This extension is a **tooling foundation**. Everything it advertises is real
and wired to the actual compiler CLI — nothing is faked:

- Syntax highlighting via a scoped TextMate grammar (`source.karkain`).
- Comment/bracket/folding configuration matching the Karkain lexer.
- `karkain check` (structured diagnostics via `--format=json`).
- `karkain fmt` (document formatting, optional format-on-save).
- `karkain compile` and `karkain run` in an integrated terminal.

Language intelligence (completion, hover, goto-definition, semantic tokens) is
provided by the Karkain LSP and will be wired to this extension as it matures.

## Requirements

- The `karkain` executable (or configure `karkain.compilerPath`).
- The LSP is not yet served by this extension (see `karkain lsp`).

## Install from source

```
cd editors/vscode
npm install -g @vscode/vsce
vsce package
code --install-extension karkain-0.1.0.vsix
```

## Commands

| Command | Action |
|---------|--------|
| `Karkain: Check File` | Runs `karkain check --format=json` and reports diagnostics |
| `Karkain: Compile File` | Runs `karkain build` in a terminal |
| `Karkain: Run File`   | Runs `karkain run` in a terminal |
| `Karkain: Format Document` | Runs `karkain fmt` on the active file |

## Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `karkain.compilerPath` | `karkain` | Path to the compiler executable |
| `karkain.formatOnSave` | `false` | Format `.kark` files on save |

## Limitations

- No semantic intelligence yet (LSP wiring is planned).
- Diagnostics are reported via the notification channel, not the Problems
  panel, until the LSP integration lands.
- Compiler must be installed on the same machine as VS Code.