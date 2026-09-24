# Phase 148 FINAL REPORT — Native CLI Flag + Control Flow + Calls ABI

Verdict: **COMPLETE**. `karkain build/run --target native-x86_64-linux`
is a real user path: ELF images from any host with no C compiler,
execution on linux/amd64, control flow and a full int/string calling
convention, all proven green on Linux CI.

## A. 148A — CLI (Go engine; kcc auto-routes, no kcc changes)

- `native-x86_64-linux` in `validTargets` + supported-targets line +
  `karkain target` row ("run needs linux/amd64") + `--help`
  (`pkg/cli/exitcodes.go`, `config_target.go`, `cmd/karkain/main.go`).
  Exact-match alias (host, non-triple — never parsed as a cross triple,
  pinned by `TestPhase148_NativeRejectedAsTriple`).
- `cfreeBuildCommand` writes `0755` ELF to `-o` (default
  `build/<base>.elf`); `cfreeRunCommand` stages to temp and executes
  with stdio passthrough, program failure → ExitFailure (Phase-100
  contract), non-Linux run refused with the build-only hint, exit 6
  (`pkg/cli/cfree_target.go`, wired in `commands.go` beside the wasm
  branches). kcc routes exotic targets to the Go commands (existing
  main.go rule) — 151 territory untouched.
- `--incremental` + native refused loudly in `BuildCommandIncremental`
  (the cache serves the C pipeline; native-split caching is post-148).
- Gate `pkg/cli/phase148_native_test.go` 5/5 (+ABI build probe):
  build-writes-ELF with magic check (straight-line, control-flow and
  ABI programs), run-refusal off Linux (real golden on Linux),
  incremental refusal, listing, routing shape. `--engine go` pinned
  (kcc parity is 151).

## B. 148B — Control flow

- Emitter: `CmpRegReg` + near `Jz/Jl/Jle/Jg/Jge`, golden-pinned
  (`TestEmitCondJumps`, hand-computed displacements).
- `emitFunc` body factored into `emitStmt` (straight-line paths
  byte-identical by construction); `emitCond` accepts int comparisons
  only (loud K145 otherwise — never miscompiled truthiness);
  `if/while`/C-`for` (bare-assignment `post` wrapped), nested `let`s
  via function-wide pre-scan (first declaration wins, shadowing writes
  through — documented v1 semantics), int reassignment statements,
  `break`/`continue` via loop-label stack (outside loops = K145),
  `for-in` stays K145 (arrays are 150 Value-model work).
- Goldens `TestNativeControl` (executed Linux, structural elsewhere):
  `if_else 10/40`, `while_sum 55`, `cfor_sum 45`,
  `break_continue 18`, `nested 9`.

## C. 148C — Calls ABI (the 7-8-9 design)

- Native calling convention (documented in `program.go` header):
  argument 8-byte units pack in order (int 1, string 2); units 0-5 in
  RDI,RSI,RDX,RCX,R8,R9; further units in the caller-frame extras area
  (sized by the hungriest call site) addressed by R10 (`lea` before the
  call); string returns leave `(RAX=ptr,RDX=len)`; rsp never moves
  during evaluation (147's rule extended).
- Function table (arity exact — previously unchecked) with kind-checked
  arguments (loud int/string mismatches naming the parameter),
  string-typed params via Phase-46 annotations (two slots), string
  returns via whole-unit inference (all returns must agree; cycles and
  mixes are loud K145), `main`-takes-no-arguments guard.
- New emitter primitives `LeaRegStack`/`LoadBaseOff` (golden-pinned).
- Goldens `TestNativeCallsABI`: string arg (literal + variable),
  string return, mixed args, seven params (`28`), reg/stack straddle,
  nested string calls. Negatives extended: arity, main-args,
  kind mismatches, main-string-return. `explain K145` rewritten to the
  remaining surface (the 145 "straight-line" text was stale since 148B).

## D. Gates + regressions + evidence

- `pkg/native` full suite (encoder/structural/determinism/K145/
  execution), `pkg/cli` Phase-148 gate, `pkg/target`, Phase-111/117
  target-list suites, `go vet`, `go build ./...` — all green.
- CI: new "Run Phase 148 native-target gate" step; execution goldens
  (control + ABI) run in the default `pkg/native` gate on Linux.
- Differential: straight-line emission paths untouched (refactor-only
  + additive cases); 147 corpus goldens byte-identical by construction
  and re-proven green.
- Second-defect note (147 record): the push-shifted slot read and the
  missing print newlines were found by this phase's bisection
  discipline and are closed here, not carried.

## E. Boundaries (documented, NOT defects)

- `for-in`/arrays, `main` args/`getArgs`, floats, maps, PE/Mach-O
  (149), kcc parity (151), native-split incremental, `prof`/`debug`
  on native, native DWARF. `run` off Linux stays refused.
