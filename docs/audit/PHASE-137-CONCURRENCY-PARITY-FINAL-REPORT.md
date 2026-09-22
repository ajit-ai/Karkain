# Phase 137 — Concurrency Parity Final Report

**Status**: ✅ COMPLETE
**Date**: 2026-09-22
**Milestone**: GA-2 Infrastructure Track

## Executive Summary

Phase 137 achieves full concurrency parity between the Go reference engine and the self-hosted kcc compiler. The kcc compiler now correctly parses, type-checks, and codegens `spawn`, `receive`, and all concurrency builtins (`channel`, `chanSend`, `chanClose`, `join`, `wait_all`, `actor`, `actorSend`, `actorState`, `setActorState`, `actorStop`) with byte-identical executable output to the Go engine.

**Key Achievement**: The Phase 107 concurrency runtime (work-stealing scheduler, channels, actors) is now fully operational on both engines, removing a major post-107 boundary that previously limited kcc's capabilities.

## Implementation

### 1. Parser Changes (`src/compiler/parser.kark`)

- Added `parseSpawnExpr()` to parse `spawn(fn, args...)` expressions
- Added `parseChRecv()` to parse `receive(ch)` expressions  
- Added `parseKeywordCall()` to parse `channel(...)` and `actor(...)` as plain calls
- Added `parseWithState()` to preserve parser diagnostics for driver entries
- Enforced that `send` is operator-only (`ch <- msg`), rejecting call form with K001

### 2. AST Changes (`src/compiler/ast.kark`)

- Added `NODE_SPAWN` node type with structure `[NODE_SPAWN, funcName, args, line]`
- Added `NODE_RECEIVE` node type with structure `[NODE_RECEIVE, channel, line]`
- Added accessor functions: `makeSpawn`, `spawnFunc`, `spawnArgs`, `spawnLine`, `makeChRecv`, `chRecvChannel`, `chRecvLine`

### 3. Checker Changes (`src/compiler/checker.kark`)

- Added all 10 concurrency builtins to `isCheckerBuiltin()`
- Added `Spawn` case in `checkExpr()` to validate argument expressions
- Added `Receive` case in `checkExpr()` to validate channel operand
- Mirrors Go resolver behavior (existence-only, arity validated at C compile time)

### 4. Codegen Changes (`src/compiler/codegen.kark`)

- Added concurrency prescan: `collectConcPrescan()`, `collectConcPrescanStmt()`, `collectConcPrescanExpr()`
- Added state slots: `concUses`, `concSpawnOrder`, `concHandlerOrder`
- Added helper functions: `concIsBuiltin()`, `concSpawnSiteIndex()`, `concActorHandlerIndex()`
- Added runtime emission: `emitConcRuntime()` (calls generated `conc_runtime.kark`)
- Added wrapper emission: `emitConcWrappers()` for spawn sites and actor handlers
- Added Value glue: `emitConcGlue()` for builtin-to-runtime adapters
- Added builtin lowering: `emitConcCall()` for channel/actor/wait_all builtins
- Added expression lowering: `emitConcSpawn()`, `emitConcRecv()`
- Integrated prescan and runtime emission into `generateC11()`

### 5. KIR Changes (`src/compiler/kir.kark`)

- Added explicit KIR rendering for `spawn` and `receive` expressions
- Previously these were dropped to `_`, now render as `(spawn func arg1 arg2...)` and `(receive ch)`

### 6. Semantic Analysis (`src/compiler/sema.kark`)

- Added all 10 concurrency builtins to `isBuiltinFunc()` for codegen-time resolution

### 7. Generated Runtime (`src/compiler/conc_runtime.kark`)

- Auto-generated from `runtime/concurrency/c/*` by `scripts/gen-conc-runtime.ps1`
- Contains `emitConcRuntime()` function that embeds the C runtime as emitLine calls
- SHA-256 digests in header validate freshness via `pkg/codegen/conc_kark_fresh_test.go`

### 8. CLI Bridge Changes (`pkg/cli/kcc_engine.go`)

- Changed gcc flag from `-std=c99` to `-std=c2x` for C11 atomics support
- Applied to both `KCCBuildCommand` and `KCCRunCommand` link paths
- Mirrors Go backend's use of C11 atomics for the concurrency runtime

### 9. Driver Changes (`src/compiler/main.kark`)

- Updated all driver entries (`checkFile`, `kirFile`, `verifyFile`, `buildFile`, `runFile`, `runTests`, `compileRunDriver`) to use `parseWithState()` instead of `parse()`
- Updated compile commands to use `-std=c2x` for C11 atomics
- Ensures parser diagnostics survive for proper error reporting

## Testing

### 1. Freshness Gate (`pkg/codegen/conc_kark_fresh_test.go`)

- Validates that `src/compiler/conc_runtime.kark` is up-to-date with `runtime/concurrency/c/*`
- Compares SHA-256 digests recorded in generated header
- Fails with regen command when runtime sources drift

### 2. Concurrency Parity Gate (`pkg/cli/phase137_concurrency_test.go`)

**TestPhase137_ConcurrencyGoGolden**: Verifies Go engine produces expected golden output
- Input: `examples/concurrency/pipeline/main.kark`
- Expected: `144\n10\n20\n30\n0\n6\n`
- Status: ✅ PASS

**TestPhase137_ConcurrencyKccGolden**: Verifies kcc engine produces byte-identical output
- Same input and expected output as Go test
- Status: ✅ PASS

**TestPhase137_ConcurrencyByteParity**: Direct byte-identical comparison
- Builds with both engines in same directory
- Compares executable output byte-for-byte
- Status: ✅ PASS

**TestPhase137_SpawnReceiveKIR**: Verifies KIR rendering
- Calls `KCCKirCommand` on pipeline example
- Validates command succeeds (KIR contains spawn/receive)
- Status: ✅ PASS

**TestPhase137_ConcurrencyCheckerParity**: Verifies both engines check successfully
- Runs `CheckCommand` with both engines
- Status: ✅ PASS

**TestPhase137_ConcurrencyNegativeCases**: Verifies error handling parity
- Tests invalid `send(ch, 42)` usage (send is operator-only)
- Both engines reject with parse errors
- Status: ✅ PASS

### 3. Existing Phase 107 Tests

All existing Phase 107 tests continue to pass:
- `TestPhase107_CodegenSpawnJoin` ✅
- `TestPhase107_CodegenSpawnStmtStatement` ✅
- `TestPhase107_CodegenChannels` ✅
- `TestPhase107_CodegenActors` ✅
- `TestPhase107_CodegenRuntimeEmbedded` ✅
- `TestPhase107_CodegenStress` ✅
- `TestPhase107_ConcurrencyE2E_RunsClean` ✅
- `TestPhase107_ConcurrencyDeterminism` ✅

## Documentation Updates

### Example Files

Updated concurrency example headers to reflect Phase 137 completion:
- `examples/08-concurrency/01_parallel_sum.kark`: Status Experimental → Stable, Engine go → both
- `examples/08-concurrency/02_channel_ping.kark`: Status Experimental → Stable, Engine go → both
- `examples/concurrency/pipeline/main.kark`: Already marked for both engines

## Known Limitations

None new to Phase 137. All previously documented concurrency runtime limitations remain:
- Single-threaded scheduler (no parallel task execution on kcc side - same as Go)
- C11 atomics requirement (now satisfied by `-std=c2x` on both engines)
- Platform-specific threading (Win32/pthread) - unchanged from Phase 107

## Regression Testing

All existing regression suites pass:
- Phase 107 concurrency tests ✅
- Phase 130 closure tests ✅
- Phase 133 first-class fn tests ✅
- Phase 134 incremental compilation tests ✅
- Phase 135 local registry tests ✅
- Phase 136 LSP v2 tests ✅
- Full `pkg/codegen` suite ✅
- Full `pkg/cli` suite ✅
- `go vet` ✅
- `go build` ✅

## Performance Impact

No measurable performance impact on non-concurrent programs:
- Concurrency prescan only runs when `spawn`/`channel`/`actor` keywords are present
- Runtime emission only occurs when `concUses` flag is set
- Zero overhead for programs without concurrency features

## Boundary Closure

**Before Phase 137**: kcc engine did not support concurrency features; this was a documented post-107 boundary in AGENTS.md and example headers.

**After Phase 137**: kcc engine has full concurrency parity with Go engine, including:
- Parse parity for spawn/receive expressions
- Checker parity for concurrency builtins
- Codegen parity for runtime emission
- Byte-identical executable output
- KIR rendering for concurrency constructs

## Future Work

No immediate follow-up work required. Concurrency is now fully operational on both engines. Potential future enhancements (not Phase 137 scope):
- Parallel task execution in kcc scheduler (currently single-threaded like Go)
- Additional concurrency primitives (e.g., select, mutex, rwlock)
- Performance optimization of runtime emission

## Conclusion

Phase 137 successfully closes the concurrency parity boundary between the Go and kcc engines. The self-hosted compiler now supports the full concurrency feature set with byte-identical behavior to the reference implementation. This removes a significant limitation of the kcc engine and brings it to feature parity with the Go front end for all currently implemented language features.

**Verdict**: ✅ READY FOR COMMIT
**Next Step**: Commit to develop branch following the mandatory branch workflow rule
