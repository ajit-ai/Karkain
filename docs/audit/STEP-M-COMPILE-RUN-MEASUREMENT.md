# Step M — compile/run measurement baseline (read-only)

Status: **COMPLETE**. Unnumbered slice on the v1.2.x line. **No code, no gate and
no CI step**: a measurement is not a capability and must not be presented as one.
This note replaces prose about "Karkain is slow" with wall-clock numbers and
states exactly what was and was not measured.

## 1. Question

Why does `karkain run` feel slower than Go or C, and which planned slice
(A1 or v1.2.35) actually targets the cost?

## 2. Environment and provenance

- Host: windows/amd64, ~4 GB RAM. Free memory during the run varied from
  ~142 MB to ~900 MB (543 MB at the end) — the documented low-memory class.
- gcc: MSYS2 MinGW **16.2.0**. Go: **go1.27.0 windows/amd64**.
- Repo at `74d3a32` (`develop` tip merged into main as `c186d75`).
- CLI: `karkain-measure.exe`, SHA-256
  `D230F225BF6F1C350876D49455E8EAC0005483C4A31B2D1BB94C0B08C128C654`.
  It was built from HEAD; PR #63 changed no CLI source, only `checker.kark`.
- kcc: a fresh binary built from the merged `src/compiler/*.kark`, SHA-256
  `BF9F22B762FA409E1C82813C1E6BFF7F8047F1F4E9C97F98EAA99774FFBCFD09`.
- The working tree contains an **untracked** `src/compiler/sqlite3.c` that the
  CLI links into every stage-1 build. It was **not touched**. Measurement used
  a clean tracked-files-only tree produced by `git archive --format=zip HEAD`.
- Raw data: 135 rows, SHA-256
  `5847E8DB47885DA8C7FF435E218C73790BC69807FC3827AD7E2D18A2EEDB59C8`,
  at `C:\Windows\Temp\opencode\stepm\results4.txt`.

## 3. Method

Five small programs (hello/print, integer `for` loop, string concat, array
literal + `len`/index, user function call), each run five times through five
paths:

| Path | Command shape |
|---|---|
| default | `karkain run p.kark` (kcc default → C23 → gcc link → run) |
| goengine | `karkain run p.kark --engine go` |
| native | `karkain run p.kark --target native-x86_64-windows` |
| gorun | `go run p.go` |
| gcc | `gcc -O2 p.c -o p.exe` (**compile only** — the exe was not run) |

Timing used PowerShell `&` direct invocation with a `Stopwatch`. PATH shims for
`gcc` and `go` logged every invocation and forwarded to the real tool; the
logs are the positive control: `default` and `goengine` each made exactly
**one** gcc call, `gorun` made exactly **one** go call, and `native` made
**zero** of either. An empty log for native is therefore evidence, not a
broken shim.

## 4. Measurement defects found and corrected

These are recorded because each initially produced plausible but wrong numbers:

1. **Start-Process harness floor ~1021 ms.** A no-op `cmd /c echo` measured
   ~1.02 s through `Start-Process -Wait`; every path was inflated. Replaced
   with direct `&` invocation: measured floor **25.5 ms**.
2. **PowerShell `$args` parameter shadowing.** Naming the runner's parameter
   `$args` made commands execute with no arguments (the CLI printed help in
   ~28 ms). Renamed to `$cmdArgs`.
3. **UCRT64 before MinGW breaks gcc silently.** With `ucrt64\bin` ahead of
   `mingw64\bin`, gcc failed with no diagnostic even on a three-line program.
   Removed from PATH; the kcc binary was linked from MinGW, so MinGW's DLLs
   satisfy it.
4. **Generated `.c` overwrote the C baseline.** The Go engine writes `<name>.c`
   beside the source. C/Go reference programs were moved to a separate `refs/`
   directory.
5. **Raw newline inside a C string literal** in three reference programs made
   the gcc baseline fail. Replaced with `\n` escapes.
6. **Cold kcc rebuild was not measured.** With only 142–900 MB free, a stage-1
   rebuild is the `error[K127]`/OOM class. A prebuilt kcc was used throughout,
   so **all numbers below are WARM paths**. No cold-build claim is made.

## 5. Measured results (milliseconds, 5 runs each)

| Path | N | min | median | max | gcc calls/run | go calls/run |
|---|---:|---:|---:|---:|---:|---:|
| default (kcc → C23 → gcc) | 25 | 1303.0 | **1975.2** | 2715.5 | 1 | 0 |
| goengine (`--engine go`) | 25 | 1277.6 | **2000.8** | 2698.9 | 1 | 0 |
| native (kcc → PE) | 25 | 131.4 | **191.0** | 365.7 | 0 | 0 |
| gorun (`go run`) | 25 | 204.1 | **296.3** | 1072.2 | 0 | 1 |
| gcc (compile only) | 25 | 210.0 | **308.6** | 500.7 | 1 | 0 |

All 125 program runs exited 0; stdout byte counts were identical across
`default`, `goengine`, `native` and `gorun` for every program (7, 4, 6, 6, 4
bytes respectively). The gcc column shows 0 stdout bytes because the exe was
not executed.

Median by program (default / goengine / native / gorun / gcc):

```
hello   1855 / 2155 / 264 / 388 / 226
loop    2186 / 1712 / 142 / 281 / 442
concat  1520 / 1581 / 335 / 422 / 354
array   1617 / 2356 / 352 / 419 / 309
call    2581 / 1938 / 147 / 224 / 249
```

## 6. Verdict

- The **default path is ~2.0 s**, the **Go engine ~2.0 s**, and the **native
  path ~0.19 s** — roughly a **10×** difference. `native` invokes neither gcc
  nor Go and still executes a real PE with the correct output.
- The ~1.8 s excess on the C23 paths is the **gcc link plus staging/preflight**,
  not code generation. This agrees with the repository's existing measured
  facts: native emission is 11–53 µs/program (Phase 150D) and the no-op
  incremental cache hit is 79 ms (Phase 134).
- Therefore the right next moves are **A1** (kcc writes and runs its own native
  image, removing the Go launcher) and **v1.2.35** (make the native path the
  default for supported programs). **Rewriting the compiler is not supported
  by this evidence.**
- `gorun` (~0.30 s) and bare `gcc` compile (~0.31 s) are external baselines on
  a different toolchain; they are not parity claims.

## 7. Limits, stated explicitly

- Native results apply only to the **supported subset** used here. stdlib
  imports, sockets, TLS, SQLite and atomics are refused by name on native and
  would still take the C23 path.
- WARM only. A cold kcc rebuild remains unmeasured on this host.
- The C23 paths have large variance (1.28–2.72 s) consistent with the
  documented gcc-kill/pagefile class; medians are reported, not single samples.
- No claim about CI hosts, Linux, macOS, other CPU counts, or larger programs.
- No code changed; nothing was staged or committed from this measurement.
