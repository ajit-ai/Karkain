# 15 — Developer Tools

Demonstrates the Karkain developer ecosystem itself: the CLI, the engines,
and the write -> build -> run -> test -> profile workflow.

| File                         | Status   | What it shows                |
|------------------------------|----------|------------------------------|
| `01_hello_toolchain.kark`    | Runnable | check / build / run loop     |
| `02_assertions_test.kark`    | Runnable | `karkain test` discovery     |

## The workflow

```
karkain check   app.kark              # validate syntax + semantics      (exit 3 on error)
karkain build   app.kark -o bin/app   # native executable (PE32+/ELF)
karkain run     app.kark              # build + run in temp sandbox
karkain test    .                     # discover *_test.kark, run test_*
karkain prof    app.kark              # call counts, timing, flame-graph data
karkain fmt     app.kark --check      # canonical formatting
karkain lint    app.kark              # full front-end analysis (+borrow checker)
karkain target                        # host triple + supported target matrix
karkain build   app.kark --target x86_64-linux   # real cross-compilation switch
```

## Engines

- **kcc** (self-hosted compiler, default): assembles the compiler sources
  into a native executable that lexes/parses/type-checks/codegens `.kark`.
- **Go front end**: selected via `KARKAIN_ENGINE=go` or `--engine go`, used
  to bootstrap and for parity verification.

## More tooling

`karkain lsp`, `karkain explain <K-code>`, `karkain config`, `karkain pkg`
(registry + workspaces), and `karkain ide info` (JSON contract).

## Run

```
karkain run examples/15_developer_tools/01_hello_toolchain.kark
karkain test examples/15_developer_tools/02_assertions_test.kark
```