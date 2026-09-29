# `karkain run` stdout parity between the kcc and Go engines

*Verdict: **COMPLETE** — corrective change, no new increment. Not a phase, and
no phase number is assigned to it. Recorded here per the Governance rule in
`AGENTS.md`: the owning layer, the demonstrated failure, the evidence, and what
was re-pinned.*

---

## 1. The defect

On the **default** engine (kcc), `karkain run prog.kark` printed the program's
output differently from `--engine go`, in two observable ways:

1. **The compiler's build banner was written to the program's stdout.**
   `KCCRunCommand` prepended `out` — the `[ok] <file> -> <file>.c23 (c23)` line
   — to the program's output. A build diagnostic was therefore interleaved into
   program stdout, so `karkain run p.kark | head -1` printed a compiler message
   rather than the program's first line.
2. **The trailing newline was stripped and re-added as a bare LF.**
   `strings.TrimSpace(...)` removed the program's own terminator, and
   `fmt.Println` in `cmd/karkain/main.go` then appended `0x0a`. The Go engine
   streams the child with inherited stdio, so the program's own CRLF
   (`0x0d 0x0a`) reached the terminal. The last line of every default-engine
   run therefore ended in a bare LF where the Go engine's ended in CRLF.

**The compiled programs were never wrong.** A kcc-linked executable and a
Go-linked executable emit identical bytes; the divergence was introduced
entirely in the CLI's handling of the child's output.

## 2. Why no existing gate saw it

The Phase 114 corpus gate builds to `main.exe` and runs that executable
directly, capturing build output into a separate buffer. It therefore never
exercised the CLI's own stdout rendering, and the parity it enforces held. This
is the same class as the 151D-first silent Go fallback: the comparison was
made downstream of the place the defect lived, so it could not observe it.

## 3. Root cause and fix

Owning layer: **`pkg/cli/kcc_engine.go` (`KCCRunCommand`)**, with a
contributing change in **`cmd/karkain/main.go`**.

`runCmd.CombinedOutput()` merged the two streams and destroyed the program's
own terminator. The fix keeps the streams **distinct**:

* `progOut` / `progErr` are captured separately, and `msg` is the program's
  stdout **verbatim** — no `TrimSpace`.
* On failure, the program's stderr is appended so the Phase 100 runtime-error
  contract (`runtime error: division by zero at <file>:<line>`) still reaches
  the user; a run with neither stream still reports the exec error rather than

## 4. Gate

`pkg/cli/kcc_run_parity_test.go` (new), 3 tests / 5 subtests, **PASS in
73.2 s**. Wired in `ci.yml` as its own step
(`Run kcc run stdout parity gate`, `-run 'TestKCCRun_'`), because relying on a
broader `pkg/cli` run is exactly how 151P0/151B/151C/151C2 shipped without
executing in CI.

| Test | What it establishes |
|---|---|
| `TestKCCRun_StdoutIsByteIdenticalToTheGoEngine` | 3 programs; stdout, exit code and the trailing terminator compared **across engines** |
| `TestKCCRun_FailureStillReportsAndExitsNonZero` | a divide-by-zero program still reports the error and the `boom.kark:3` source location, and exits non-zero, with no banner |
| `TestKCCRun_SuccessMessageIsNotEmptyForPrintingPrograms` | a program that prints nothing yields empty stdout (the old code produced a banner-only Message) |

**Every expectation is a differential against the Go engine, not a golden.** A
golden pins the current bytes, and the defect *was* that the two engines'
bytes differed — a golden on either side alone would have kept passing through
the whole defect. Comparing the engines to each other is the only formulation
that fails when they disagree. The three specific corruptions are additionally
asserted independently so the failure message names the cause.

The differential also carries an explicit **anti-vacuity guard**: if the Go
engine produced no stdout, the test fails rather than comparing two empty
strings.

## 5. Non-vacuity: mutation-verified

The pre-fix body was temporarily restored in place (the `CombinedOutput` +
`TrimSpace` + banner-prepend path) and the gate re-run. It **failed all three
subtests, and all three diagnostics fired** — `stdout differs between engines`,
`ends in a bare LF where go ends in CRLF`, and `carries the build banner`. The
fix was then restored and the tree confirmed byte-identical to its
pre-mutation state (no `MUTATION-PROBE` residue).

## 6. Regressions

Green, each measured:

| Gate / check | Result |
|---|---|
| `go build ./...` | clean |
| `go vet ./pkg/cli/ ./cmd/karkain/` | clean |
| `TestKCCRun_` (the new gate) | **ok**, 73.2 s, 3 tests / 5 subtests, no skips |
| Phase 151 + 151A + 151A2 + 151B + 151C + 151C2 | **ok**, no FAIL, no SKIP |
| `TestPhase97_` / `TestPhase96_` (kcc default-engine parity) | **ok**, 272.4 s |
| `TestPhase95_` (kcc owns the core pipeline) | **ok**, 133.8 s |
| `sphinx -b html -W --keep-going` | `build succeeded`, 0 warnings |

**`gofmt`:** `gofmt -l` flags `pkg/cli/kcc_engine.go` and
`cmd/karkain/main.go`, but this is the repository-wide CRLF artifact documented
in `AGENTS.md` — 951 of 951 lines in `kcc_engine.go` are CRLF, so `gofmt -d`
renders the whole file. Normalising to LF and writing **BOM-free** (PowerShell
`Set-Content -Encoding UTF8` adds a `EF BB BF` BOM, a trap already recorded in
this repository) both files are **gofmt-clean**, so the added code is correctly
formatted and only the line endings are at issue.

## 7. Defects found while writing the accompanying docs

The uncommitted `docs/source/development/index.rst` page (Contribution Rules)
carried two defects that `sphinx -W` — the exact gate CI runs — rejects. Both
were found by running the real build rather than by reading:

1. **Short section underline.** `1. Attribution` is 14 characters; its `===`
   underline was 13. Fixed.
2. **Inline strong start-string without end-string** (line 103). A `**gate**`
   span was opened on one line and closed on the next; docutils does not allow
   inline markup to cross a line break. Fixed by closing it on the same line.

## 8. Scope

`kccOwnsNativeTargets` is unchanged (`false`); no native codegen, encoder,
value-model, container or `src/compiler/*.kark` file was touched. No KIR pin
moves, because KIR does not render this path. `VERSION` unchanged, Phase 152
untouched, and increment 151's open item remains **151A, the value model**.

  exiting silently.
* The build banner is no longer on stdout at all. It is emitted to **stderr**
  under `--verbose`, which is where a build diagnostic belongs.
* `cmd/karkain/main.go` prints a `Message` that already ends in a newline
  **verbatim** and otherwise still appends one, so every diagnostic keeps its
  existing rendering and no message gains a second terminator.
