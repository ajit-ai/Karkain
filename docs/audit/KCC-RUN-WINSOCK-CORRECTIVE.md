# Corrective change — direct `kcc run` Winsock link on Windows

**Classification:** corrective change against the frozen v1.2.x baseline. Not a
new increment, no phase number, no scope increase.

## 1. Owning layer and demonstrated failure

Owning layer: `src/compiler/main.kark`, `runFile` — the self-hosted compiler's
own `run` driver. The Go CLI path is **not** at fault: `pkg/cli/kcc_engine.go`
already applies `winsockLibFlag()` to every generated-C link, and
`compileRunDriver` in the same `main.kark` already retries with `-lws2_32`
(Phase 125A).

Measured failure on Windows 11, from `examples/01-fundamentals`:

```
PS F:\codes\git\karkain\examples\01-fundamentals> kcc run 01_hello_world.kark
Compiling: gcc -std=c2x -o karkain_temp karkain_temp.c -D_POSIX_C_SOURCE=200809L -Wno-psabi -lm
... undefined reference to `__imp_WSAGetLastError`
... undefined reference to `__imp_WSAStartup`
... undefined reference to `__imp_getaddrinfo`
... (Winsock symbols through `__imp_send`)
collect2.exe: error: ld returned 1 exit status
```

Line 872 of `src/compiler/main.kark` built exactly that command: no
`-lws2_32`. Every generated C preamble now contains the net runtime, so on
Windows the plain link always fails. MinGW ignores `#pragma comment(lib, ...)`,
so the library must be supplied on the command line.

## 2. Required change

Mirror the existing `compileRunDriver` retry: attempt the plain C2X link first;
only if it fails, retry the **same** command with ` -lws2_32`. POSIX hosts never
take the retry branch because the plain link succeeds (there are no WSA
symbols), and the added flag is never emitted on the success path.

## 3. Downstream behaviour preserved

- No emitted C changes, no image bytes change, no public surface change.
- The KIR pin is **not predicted locally**: whole-tree `kir --verify` crashes on
  this host. The fix adds statements inside an existing function (no new
  function), but whether the rendered KIR count moves is a measurement question.
  CI must re-measure and, if the count moved, re-pin from that measurement only.
- The compile command printed on success is byte-identical to the historical
  one; the retry line appears only on the Windows failure path.
- Cleanup on final failure remains identical (remove the generated `.c`).

## 4. Gate and evidence plan

New gate `pkg/cli/phase_kcc_run_winsock_test.go` (`TestPhaseKCCRunWinsock_`):

1. Builds kcc fresh from the working tree via `phase95KCC(t)`.
2. Runs `kcc run` on a tiny `.kark` program in an isolated temp CWD.
3. Asserts exact program stdout and exit 0 on the host's native container.
4. On Windows, asserts the retry line with `-lws2_32` is actually printed —
   otherwise an empty/skipped retry could satisfy an exit-code-only assertion.
5. Mutation check: reverting the retry reproduces the undefined-reference
   failure; the source file is restored byte-identically afterwards.

CI step: `Run direct kcc run Winsock link gate`, pattern
`TestPhaseKCCRunWinsock_` (checked not to collide with existing patterns).

## 5. Regression set

`TestPhase95_`, `TestPhase117_` (or the narrow subset that fits the host),
`go vet ./pkg/cli`, `go build ./...`, plus the new gate. Heavy gates are run in
isolation per the ~4 GB host rule; failures caused by the K127 guard or gcc OOM
are recorded as environmental, not as regressions of this change.

## 6. Measured results (2026-11-28, Windows 11, ~4 GB host)

| Check | Result |
|---|---|
| `npx js-yaml .github/workflows/ci.yml` | exit 0 |
| `gofmt -l pkg/cli/phase_kcc_run_winsock_test.go` | empty output, exit 0 |
| `go vet ./pkg/cli/` | exit 0 |
| `go build ./...` | exit 0 |
| `go test ./pkg/cli/ -run 'TestPhaseKCCRunWinsock_' -count=1 -v` | **PASS**, 31.59 s (gate itself) / 33.081 s (package) — first link emits the expected WSA warnings, retry with `-lws2_32` links, program prints `42` |
| Mutation: restore the pre-fix link block | gate **FAIL** with the original `__imp_WSAGetLastError`/`__imp_WSAStartup`/`__imp_send` undefined references; mutated SHA `95730FA4…E0F` |
| Restore | `main.kark` SHA back to `143865A2…E0F`, byte-identical to the fixed baseline; gate re-run PASS |
| `go test ./pkg/cli/ -run 'TestPhase95_' -count=1` | **PASS**, 125.342 s |
| `go test ./pkg/cli/ -run 'TestPhaseFoundation_KCCSourceCompiles' -count=1` | **PASS**, 25.868 s |
| CI pattern collision | exactly one Go test with `TestPhaseKCCRunWinsock_`; one CI step at line 194 |

## 7. Follow-up: silent first-attempt link (same corrective slice)

The first fix worked but produced a wall of expected `undefined reference`
diagnostics before the retry. User-visible result after that fix:

```
Compiling: gcc ... -lm
... twenty __imp_WSA* undefined-reference lines ...
Compiling: gcc ... -lm -lws2_32
--- Output ---
Hello, Karkain!
```

Second correction: detect the Windows host with the same `openFile(...)`
existence idiom the driver already uses (`C:/Windows/System32/kernel32.dll`)
and put `-lws2_32` on the **first** link command. The retry remains only as a
fallback for an unrecognised host. POSIX keeps the historical command
byte-identical. The gate was strengthened accordingly: on Windows it asserts
the FIRST `Compiling:` line contains `-lws2_32` and that `undefined reference`
never appears. Mutation (remove the first-attempt flag) reproduced the noisy
failure and FAILED the gate; `main.kark` restored byte-identically (SHA-256
`7D1191A55510E9197FCB53D2439B2810AAF7250DBEF54BD492D844CDB0459C23`), gate
PASS 43.29 s.

**Remaining operational note:** the repository-root `kcc.exe` is a stale build
from before this source edit, so the user's direct `kcc run` still uses the old
binary until kcc is rebuilt from `src/compiler`. The `karkain` CLI path was
never affected because it already applies `winsockLibFlag()`.
