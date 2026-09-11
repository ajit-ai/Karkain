# Phase 110: Profiling & Diagnostics — Final Report

```
Phase 110: COMPLETE

Deliverable:
`karkain prof <file.kark>` compiles and runs a Karkain program exactly once with
compiler-inserted instrumentation and reports real, deterministic execution
data. The profiler is opt-in (aggregation-based, not sampling): `karkain run`
and `karkain build` never instrument a program without explicit opt-in.

Implemented:
- pkg/codegen/codegen.go
  * Config.Profiling (Phase 110 opt-in flag)
  * Generator profiling state: profiling, profFID map[string]int, profNames
    []string, curProfID int (New() seeds curProfID = -1)
  * initProfiling(prog): source-order function-id table (deterministic fids
    matching the emitted name table), called from GenerateAndCompile right
    after the concurrency pre-scan
  * profID(fnName): -1 when the function is not in the table (hooks skipped)
  * GenerateAndCompile: when profiling, emits profNameTableC(g.profNames) then
    profRuntimeC() immediately after concRuntimeAPIC() — before function
    bodies, so hooks are in scope and the malloc/free interception macros are
    defined AFTER the whole preamble (headers + preamble helpers are never
    wrapped)
  * Hook sites in genFuncDecl: getArgs enter/leave; main emits
    karkain_prof_init() before quantum_init; per-function enter after
    frame_enter + set_line; leave before frame_leave at both fallthrough
    exits; genReturnStmt leaves after the frame value is computed and before
    frame_leave
- pkg/codegen/emit_ir.go
  * emitSSAFunction: pid/profEnter/profLeave closures; getArgs enter/leave;
    enter after karkain_set_line; leave before karkain_frame_leave on OpRet
    and the final fallthrough; main karkain_prof_init() after argv assign
- pkg/codegen/prof_runtime.go (new)
  * profNameTableC: static const char* k_pf_names[k] (fid = index), sentinel 0
  * profRuntimeC: bounded static aggregation runtime
      K_PF_MAX_FUNCS 512, K_PF_MAX_EDGES 4096, K_PF_MAX_PATHS 4096,
      K_PF_MAX_DEPTH 256, K_PF_MAX_PATH 32, K_PF_MAX_LIVE 8192
    * per-function counters: count, total, min, max, child-inclusive
    * call stack (fid + start ns), caller->callee edge table (count + total)
    * folded stack paths recorded at each completed call
    * allocation metrics: count, total bytes, live bytes, peak live bytes,
      live-pointer set (K_PF_MAX_LIVE), overflow-safe
    * volatile k_pf_overflow flag (hard tables; flush stops recording)
    * timing: Windows QueryPerformanceCounter/Frequency via the header
      prototypes already provided by windows.h (the C preamble includes
      winsock2.h/windows.h unconditionally — no manual decls, no conflicts);
      elsewhere clock_gettime(CLOCK_MONOTONIC)
    * karkain_prof_init(): records start + atexit(karkain_prof_flush)
    * karkain_prof_malloc/free: call real malloc/free (via #undef first),
      #undef/#define malloc/free AFTER the preamble so only generated user
      code is intercepted
    * karkain_prof_flush(): JSON dump to $KARKAIN_PROF_OUT ("w"), guarded by
      overflow flag (missing KARKAIN_PROF_OUT -> no-op; no terminal IO)
- pkg/cli/prof.go (new)
  * ProfCommand(targetFile, format, output, engine, target, verbose): validates
    format/engine/target/file, unique temp dump path (makeProfDumpPath),
    sets KARKAIN_PROF_OUT around RunCommand (restored + dump removed in
    defer), reads + parses the dump, renders text|json|folded to stdout or
    --output
  * Profile/ProfileAllocation/ProfileFunction/ProfileCall/ProfileFolded —
    canonical report model, JSON schema "karkain-profile-v1", version 1
  * RenderJSON (MarshalIndent), RenderFolded (sorted by path),
    RenderText (function table sorted by inclusive time; call graph; allocation
    line), nsString, parseProfDump, writeProfileReport
- cmd/karkain/main.go
  * help text, early `prof` dispatch, runProfCommand (flags incl. = forms,
    --json, -o, --verbose, -h/--help; unknown flag = usage error),
    printProfHelp (Phase 110 CLI contract text); handleLSP untouched nearby
  * Exit codes re-used: ExitSuccess=0, ExitFailure=1 (kcc engine, wasm32-wasi
    target, missing dump, program failure), ExitUsage=2 (unknown --format,
    missing/invalid file via ValidateKarFile), ExitCompile=3 (compile failure)

Real examples (examples/profiling/):
- basic.kark      — two helper functions; shows function table + main->add /
                    main->square call graph
- recursion.kark  — fib(18): 8361 recursive fib invocations aggregated
                    deterministically; fib->fib edge; folded recursion stacks
- hotspot.kark    — compute-bound countOdds/isOdd loop: 100000 isOdd calls,
                    correct call-chain profile, meaningful total/max split
- README.md       — usage, formats, opt-in + boundary documentation

Tests:
- pkg/codegen/phase110_profiling_test.go (3)
  * TestPhase110_ProfilingInstrumentation: profiled C contains the name table,
    the timing+flush runtime, malloc/free wrappers, main init and per-function
    enter/leave hooks; fids follow source declaration order
  * TestPhase110_ProfilingIsOptIn: default (non-profiled) builds carry NONE of
    the instrumentation (opt-in contract)
  * TestPhase110_ProfilingRuntimeAndDump: real gcc-compiled run of fib(18) with
    instrumentation; dump is valid JSON with duration_ns >= 0, overflow false,
    fib calls == 8361, main == 1
- pkg/cli/phase110_profiling_test.go (5)
  * TestPhase110_ProfCommand_Text: real-pipeline text report includes header,
    "p110 = 45" passthrough, call graph main->add x2 / main->double x1,
    Allocation section
  * TestPhase110_ProfCommand_JSON: captured report parses as the
    karkain-profile-v1 document; program/engine fields, non-negative allocation
    metrics, add==2/double==1/main==1 counts, main->add x2 + main->double x1
    edges, folded stacks non-empty
  * TestPhase110_ProfCommand_Folded: "path ns" lines sorted; main, main;double,
    main;add stacks present (program passthrough lines skipped)
  * TestPhase110_ProfCommand_OutputFile: --output writes a report file
    containing the karkain-profile-v1 document
  * TestPhase110_ProfCommand_Boundaries: kcc engine -> ExitFailure "Go engine
    only"; wasm32-wasi -> ExitFailure "no silent fallback"; --format bogus ->
    ExitUsage; missing file -> ExitFailure

Determinism:
PASS — fib(18) produces exactly 8361 fib calls across every run
(text report, JSON report and raw dump agree); names/counts/orderings stable;
timings are real wall-clock ns (never golden-tested). Flame-graph stacks and
call edges are sorted deterministically by the renderers.

Native:
PASS  (examples/profiling* compile + run through the real pipeline,
       Go engine, gcc, -O0)

Sentinel / boundary contract (explicitly rejected, no silent fallback):
PASS  (kcc: "supports the Go engine only ... deferred, Phase 110 boundary";
       wasm32-wasi: "profiling is unsupported/deferred for this target"; both
       exit 1)

WASM/WASI:
N/A — deferred boundary. The WASM backend is unchanged; its suite stays green.

Phase 106 regression:   PASS  (pkg/cli TestPhase106 green)
Phase 107 regression:   PASS  (pkg/cli phase107 concurrency + pkg/codegen +
                               pkg/runtime green)
Phase 108 regression:   PASS  (full pkg/wasm + phase108 CLI E2E green)
Phase 109 regression:   PASS  (phase109 CLI + sema builtins green)
Conformance + probes:   PASS  (59/59 conformance assertions + probes corpus)
Full pkg/codegen:       PASS  (23.267s)
Other packages:         PASS  (lexer/parser/sema/source/module/diagnostics/
                               ir/ssa/hir/runtime/compiler/backend/npu/pm)

go build:
PASS  (go build ./...)

go vet:
PASS  (go vet ./...)

Documentation:
- docs/audit/PHASE-110-PROFILING-DIAGNOSTICS-FINAL-REPORT.md (this report)
- ROADMAP.md (Phase 110 row + "Current phase" block)
- AGENTS.md ("Current phase: post-110" + Phase 110 summary block)
- docs/ROADMAP-PRODUCTION.md (Phase 110 marked COMPLETE)
- docs/audit/KARKAIN_FEATURE_PRIORITY_MATRIX.md (Profiling: MISSING -> COMPLETE)
- examples/profiling/README.md (usage + boundaries)

Known limitations / notes (documented, not Phase 110 defects):
1. Allocation metrics count ONLY malloc/free emitted as text in generated user
   code. String/array/map/struct helpers allocate inside the uninstrumented C
   preamble, so typical programs report allocation count 0; structs compile to
   map-based representations (no raw struct malloc). The metrics machinery is
   live (peak-bytes live-set, overflow-safe) and reports 0 honestly.
2. `alloc(T,n)` raw-pointer expressions emit a broken
   `make_int(n) * sizeof(T)` count expression on default builds. This is a
   pre-existing raw-pointer emission defect, unrelated to profiling.
3. `while` requires parentheses: `while (x < n) { ... }`. Without parens the
   parser folds later `if`/bodies into the condition — this is what caused the
   infinite-loop probe hangs during development; unrelated to profiling.
4. Struct-in-array programs infinite-loop on both engines (pre-existing).
5. Profiling is single-threaded by design: spawn/channel/actor programs are
   out of scope (shared static tables would be corrupted by concurrent tasks).
   kcc engine and WASM target profiling are documented post-110 boundaries.
6. Programs that exit via explicit exit() may report partial leaves for active
   frames; atexit flush still writes the dump.

Commit:
(see git log — "Phase 110: Profiling and Diagnostics")

develop:
PUSHED (develop == working tree)

main:
MERGED + PUSHED (develop == main == origin)
```

## Summary

`karkain prof` is now a real, opt-in, deterministic profiling CLI on the Go
engine: bounded static aggregation instrumentation injected by the codegen,
one instrumented run, and text/JSON/folded reports with per-function counts and
timing, the call graph, flame-graph stacks and allocation metrics. The
boundaries are explicit and rejected — kcc, WASM, and concurrency programs — so
nothing silently degrades. Regression suite (Phases 100–109 gates + all
foundation packages) is fully green, `go build`/`go vet` clean.