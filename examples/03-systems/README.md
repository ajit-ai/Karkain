# 03 — Systems

Demonstrates the systems capabilities Karkain genuinely implements today:
native build tooling, target handling, and file I/O through `std.io`.

| Example               | Status      | Engine | What it shows                    |
|-----------------------|-------------|--------|----------------------------------|
| `01_file_io.kark`     | Runnable    | both   | write / append / read / delete   |
| README command maps   | Runnable    | both   | `build`, `run`, `target`, `test` |

## Toolchain commands

```powershell
karkain target                      # host triple + supported target matrix
karkain build -o app.exe app.kark   # native native artifact (PE32+ on Windows)
karkain run app.kark                # build + run in a temp sandbox
karkain check app.kark              # syntax + semantic validation, exit 3 on error
```

`karkain build` with `--target <triple>` is a real cross-compilation switch
(Phase 111). Same-machine targets compile with the historical host probe;
foreign targets need the matching cross-gcc on PATH and fail deterministically
(exit 6) when it is missing — never a silent host fallback.

## Not yet implemented / honest scope

No process creation, raw memory APIs, command-line argument parsing, sockets,
or environment access from karkain source today. `std.io` covers files and
directories; platform/target queries live at the toolchain level
(`karkain target`, `karkain config`), not in the running program. See
`docs/source/examples/systems.rst`.

## Run

```
karkain run examples/03-systems/01_file_io.kark
```