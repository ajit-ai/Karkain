# Phase 148 BASELINE — Native CLI Flag + Control Flow

Date: 2026-09-24. Target: INDEPENDENCE C-front second step (after 147).

## 1. Starting position

- 147 closed the P1: straight-line ints/strings/calls execute on Linux CI
  (`pkg/native` default suite green with execution running). v1 surface is
  still straight-line only: `If/While/For/ForIn` are K145, as are string
  args/returns, >6 params, `break`/`continue` (no loop context exists).
- Target plumbing precedent is `wasm32-wasi`/`native-link`: exact-string
  match in `Run/BuildCommand` before triple handling, `validTargets` map
  + `supportedTargetsLine` + `sortedTargets`/`aliasTargetNote` listing,
  `NormalizeTarget` passthrough. kcc auto-routes exotic targets to the Go
  commands (`cmd/karkain/main.go:1663-1681`) — native follows the same
  rule, so **no kcc changes** (kcc parity is 151).
- Emitter has near `jmp`/`jnz` + label/rel32 (both directions); no `cmp`,
  no `jz`/`jl`/`jle`/`jg`/`jge`. Body emission lives inline in
  `emitFunc`'s switch (needs factoring into `emitStmt` for reuse by
  loop bodies).

## 2. Baseline-holds (any red stops the phase)

- Every Stable Core row; 145/147 gates green with byte-identical images
  (new lowering must not move one byte of straight-line output —
  differential check: pre/post images of the 147 corpus identical).
- C-path output untouched (native is additive; `pkg/codegen` suite green).
- `main`/`getArgs` routing, exit-code contract, K145 behavior for every
  construct NOT listed in §4 (still loud, never silent).

## 3. Promotes (only bucket movements)

- `native-x86_64-linux` joins the closed target set (build from any host —
  emission is pure Go; run only on linux/amd64, elsewhere the Phase-111
  build-only refusal). Documented as Production Candidate (Linux-only,
  straight-line + control flow).
- `if/else`, `while`, C-style `for`, `break`/`continue` (in loops),
  string call args, string returns, >6 params move from K145-rejected to
  native-gated execution goldens. K145 table shrinks to the remaining
  rows only (`explain K145` + negatives test updated).

## 4. Slice plan (as executed)

- **148A (CLI):** `native-x86_64-linux` in `validTargets` +
  `supportedTargetsLine` + `aliasTargetNote` + `SelectedTarget` (host,
  non-triple, so no cross machinery misfires) + `karkain target` row +
  `--help` line; `nativeBuildCommand`/`nativeRunCommand`
  (`pkg/cli/native_target.go`, mirroring `wasm.go` shape):
  parse → borrow → preflight → `CompileProgram` → build writes `0755`
  image to `-o`/default path, run stages to temp + executes with stdio
  passthrough and the process exit code as the result; non-Linux run
  refused with the build-only hint (ExitEnv); kcc auto-routes to Go
  (no kcc change); `--incremental` + native refused loudly in
  `BuildCommandIncremental` (split flow is C-path; native-split is 150+).
- **148B (control flow):** emitter `CmpRegReg` (`39 /r`) + near
  `Jz/Jl/Jle/Jg/Jge` (golden-pinned like 145A) beside existing
  `Jnz`; `emitStmt` factored from `emitFunc`; conditions are int
  comparisons (`== != < <= > >=`, operands via the depth-indexed
  scratch evaluator — reuses 147's rsp-stable machinery);
  `if` (cond-false→else, jmp→end), `while` (loop/cond-false→end/body/
  jmp-loop), C-`for` (init/cond/body/post), `break`/`continue` via
  loop-label stack (outside loops = K145, mirroring kcc K108).
  Goldens (each executed on Linux, structural elsewhere):
  `if_else`, `while_sum`, `cfor_sum`, `break_continue`, nested-loop.
- **148C (extended surface):** string call args as `(ptr,len)` reg
  pairs; string returns as `(RAX=ptr,RDX=len)`; >6 params via
  caller-spilled stack args with callee-side fixed-offset reads;
  K145 rows deleted per landed construct; `TestNativeNegatives`
  updated to the remaining table only.
- **148D:** gates, docs (`karkain target` prose, `installation`-style
  native notes where the tree documents targets), regressions,
  commit→push→CI proof, final report.

## 5. Non-goals (documented, NOT defects)

- `for-in`/arrays on native (array lowering is 150 Value-model work;
  integer loops cover iteration in v1), `main` args/`getArgs`,
  floats, maps, PE/Mach-O (149), kcc parity (151), native-split
  incremental, `prof`/`debug` on native, DWARF for native images.
- `run` on non-Linux stays refused (build-only); remote/emulator runs
  are a future slice, never silent host fallback.

## 6. Gate

- New execution goldens green on Linux CI (default suite, no flag);
  straight-line image bytes byte-identical pre/post (differential);
  `pkg/cli` 148 gate (CLI E2E: build writes runnable image, run
  goldens, refusals: non-Linux-run/kcc-notes/incremental, target
  listing); full Stable battery green; three consecutive default-CI
  greens before the 149 baseline (147's rule continues).
