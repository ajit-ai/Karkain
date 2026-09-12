# Karkain Development Conventions

## Branch Workflow (MANDATORY RULE)

After EVERY Phase completion and successful test run:
1. Commit all changes to `develop` branch
2. Merge `develop` into `main` branch
3. Push both branches to origin

This ensures `main` always reflects the latest working state.
NEVER skip this step. This is a hard rule, not optional.

## Roadmap

See `ROADMAP.md` for the complete development plan (Phases 50–79+).
Current phase: **116** — Phases 50–106 complete, 107 (Concurrency
Runtime) complete, 108 (WASM target) complete, 109 (Standard Library v2)
complete, 110 (Profiling & Diagnostics) complete, 111 (Cross-Compilation)
complete, 112 (Language probe hardening/debug trace/testing module) complete,
113 (Developer Preview docs+status model) complete, 114 (Complete Example
Corpus: 50 files / 15 categories + parity gate) complete, 115 (Developer
Preview Readiness: LICENSE/CONTRIBUTING/COC, honest README + docs +
release notes, unified v0.115.0 version identity across Go CLI + kcc +
generated headers, "Did you mean" typo hints + friendly missing-file errors,
fresh-checkout gate `pkg/cli/phase115_developer_preview_test.go`,
CI vet/phase-114/115 steps, Phase 115 audit reports) complete, 116 (Developer
Examples & Real-World Programming Corpus: 15 dash-named categories under
`examples/`, 50 `.kark` files with honest `Status:`/`Engine:` headers,
top-level `examples/README.md` + per-category READMEs, 4 new both-engine
examples — array stack `11_stack`, FIFO queue `12_queue`, 0/1-knapsack DP
`13_knapsack`, token statistics pipeline `05-data/04_token_stats` — pinned
byte-identical in the Phase 114 gate (45→49 goldens), `TestPhase116`
corpus-metadata gate, verify-examples.ps1 dash-directory fix,
`examples/EXAMPLES.md` + Sphinx pages + example-matrix updated, CI
`TestPhase116` step; full Go+kcc gates, unit regressions, vet, strict Sphinx
`-W` HTML+linkcheck green. NOTE: Phase 116 was developed per a local-only
amendment — no commit/push performed; directions to commit when reviewed.)
complete.
Project status:
**🚀 Karkain Developer Preview** (v0.115.0). NOTE: the Phase 91 historical
"KARKAIN 1.0 — RELEASE READY" record was superseded; the honest public label
is Developer Preview, never 1.0.
Also completed: **111** — Cross-Compilation
(`--target <triple>` is a real, explicit cross-compilation switch backed by
the Karkain-owned target model `pkg/target` (arch/os/env, canonical short
triples `x86_64-windows`/`x86_64-linux`/`aarch64-linux`/`wasm32-wasi` +
conventional long-form normalization, `Features`, host-vs-foreign
`SameMachine`); `karkain target` reports the host triple + full supported
matrix; the C-driver selection in `pkg/codegen/cross_target.go` compiles
same-machine targets with the historical host probe (byte-identical flags)
and cross targets with triple-prefixed GNU cross-gcc → clang `--target`,
failing deterministically with a cross-linker `ToolchainError` (exit 6, lists
exactly what was searched) when none exists — never a silent host fallback;
`karkain run --target <foreign>` is refused with a build-only hint (cross-run
needs an emulator/remote target); triple `karkain build` now emits real
native artifacts (PE32+ x86-64 machine field validated on this host —
previously Go-engine builds only transpiled to C); generated C self-describes
each target via a `karkain-target:` header comment + `KARKAIN_TARGET_ARCH_*`/
`KARKAIN_TARGET_OS_*` preprocessor defines. Honest matrix on the Windows x86_64
host: `x86_64-windows` PASS, `x86_64-linux` + `aarch64-linux` N/A (no
cross-linker on PATH — mechanism implemented, artifacts unverifiable here),
`wasm32-wasi` unchanged (Phase 108). In-language `target.os`/`target.arch`
builtins are intentionally post-111 (would require kcc-parity changes).
Gates: `pkg/cli/phase111_cross_compile_test.go` + `pkg/target/triple_test.go`;
regressions green: full `pkg/codegen`, Phase 105–110 CLI gates, `go vet`,
`go build ./...`. Report:
`docs/audit/PHASE-111-CROSS-COMPILATION-FINAL-REPORT.md`.)
Also completed: **110** — Profiling & Diagnostics
(`karkain prof <file.kark>` is a real CLI: it compiles and runs the program
once with opt-in, aggregation-based instrumentation and reports deterministic
data — per-function call counts + inclusive/exclusive/min/max/avg wall ns,
caller->callee call graph, folded (flame-graph) stacks, allocation metrics —
in `text` (default), `json` (`karkain-profile-v1` schema) and `folded`
formats, plus `--output <path>`. Implementation: `Config.Profiling` +
`Generator` profiling state (`profiling`, `profFID`, `profNames`, `curProfID`)
in `pkg/codegen/codegen.go` (`New` inits `curProfID:-1`), `initProfiling`
(source-order fid table) + `profID` helpers, and `GenerateAndCompile` injects
`profNameTableC(g.profNames)` + `profRuntimeC()` right after
`concRuntimeAPIC()` — a bounded static C runtime (`pkg/codegen/prof_runtime.go`:
K_PF_MAX_FUNCS 512/EDGES 4096/PATHS 4096/DEPTH 256/PATH 32/LIVE 8192,
`k_pf_overflow` flag, QPC on Windows reusing the preamble's already-included
`windows.h` prototypes — no manual decls, `clock_gettime(CLOCK_MONOTONIC)`
elsewhere, `#undef/#define malloc/free` wrappers emitted AFTER the preamble so
only generated user-code allocations are counted, JSON dump via
`atexit(karkain_prof_flush)` to `$KARKAIN_PROF_OUT`). Hook sites (enter after
frame_enter/set_line, leave on OpRet/fallthrough before frame_leave — also in
`genReturnStmt` after the frame value is computed; getArgs enter/leave; main
`karkain_prof_init()` before quantum_init) in `pkg/codegen/codegen.go` and
`pkg/codegen/emit_ir.go`. CLI: `pkg/cli/prof.go` (`ProfCommand` + `Profile`
schema + `RenderJSON`/`RenderFolded`/`RenderText` + `nsString` +
`parseProfDump` + `makeProfDumpPath` unique temp dump) and
`cmd/karkain/main.go` `prof` dispatch + `runProfCommand`/`printProfHelp`.
Exit codes re-used: 0 success, 1 failure (kcc target, wasm32-wasi target —
explicit "no silent native fallback" diagnostics, missing dump, program
failure), 2 usage (unknown `--format`, missing file via `ValidateKarFile`),
3 compile. Determinism proven: fib(18) = exactly 8361 fib calls, stable
schema/names/counts across runs; raw timings vary (never golden-tested).
Program stdout passes through. Boundaries (documented, rejected explicitly):
kcc engine deferred, WASM deferred, single-threaded (concurrency programs
out of scope), allocation metrics = only malloc/free text in generated code
(preamble helpers like make_string/make_map uninstrumented; structs are
map-based so our struct probes report count 0). Notes: `alloc(T,n)` raw
pointers emit broken `make_int(n) * sizeof(...)` on default builds (pre-existing,
unrelated); `while` requires parens `while(...)` — `while x < n {` WRONGLY
parses `if`/bodies into the condition (caused infinite-loop hangs in probes);
struct-in-array programs infinite-loop on both engines (pre-existing,
unrelated to profiling). Gates:
`pkg/codegen/phase110_profiling_test.go` (3: instrumentation markers + fid
order, opt-in — default builds carry NO hooks, compile+run+raw-dump validation
nat schema/overflow/fib 8361/main 1) + `pkg/cli/phase110_profiling_test.go`
(5: text report markers + passthrough, json schema/counts/edges via
`strings.Index(out,"{")` slice, folded stacks (skip passthrough lines), output
file, boundaries kcc/wasm/bad-format/missing-file). Regressions green: full
`pkg/codegen` 23.3s, CLI Phase 100–109 gates + conformance 59/59 + probes
corpus 524.7s, pkg/sema/runtime/wasm/compiler, lexer/parser/ir/ssa/hir/source/
module/diagnostics/backend/npu/pm, `go vet`, `go build ./...`. Examples:
`examples/profiling/{basic,recursion,hotspot}.kark` + README.
`prof` builds leave `*.c` files beside source (existing CLI behavior) — delete
generated `.c` before committing. Report:
`docs/audit/PHASE-110-PROFILING-DIAGNOSTICS-FINAL-REPORT.md`.)
Also completed: **109** — Standard Library v2
(Real `.kark` programs can `import std.string / std.collections / std.io /
std.encoding / std.crypto` through the normal toolchain on BOTH engines — Go
front end and the self-hosted kcc engine — byte-identical. Five canonical
`.kark` modules; eight byte-level runtime builtins backing the encoding/crypto/
collections surface (`hex_encode_bytes`, `hex_decode_bytes`,
`base64_encode_bytes`, `base64_decode_bytes`, `utf8_valid_bytes`,
`sha256_hex`, `sha512_hex`, `map_keys_of`) wired into the Go resolver
(`builtinNames` in `pkg/sema/resolve.go`), Go codegen preamble/helpers + genExpr
dispatch (`pkg/codegen/codegen.go`), kcc checker/sema tables
(`src/compiler/checker.kark`, `src/compiler/sema.kark` — incl. newly added
`readLineEOF`/`listFiles` arity-1 entries) and kcc C-helper emission
(`src/compiler/codegen.kark` `emitLine` preamble; helpers emitted as single
one-liners after `karkain_appendArray`). NIST FIPS 180 vectors (sha256 "abc"
`ba7816bf...`, sha256 "" `e3b0c442...`, sha512 "abc" `ddaf35a1...`) and RFC
4648 hex/Base64 verified byte-identical on both engines; malformed hex/base64
raise the SAME `runtime error: invalid (hex|base64) string at <file>:<line>`
(Phase 100 model) and `import std.does_not_exist` is rejected on both.
`kccAssembleSource` (`pkg/cli/kcc_engine.go`) is now module-aware (uses
`resolveSourcesRun`) and strips dotted `import std.x` lines (regex
`^[ \t]*import[ \t]+[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z0-9_]+)*`), preserving
`import "C" { ... }` blocks. Go parser requires `while(...)` parens (`if` does
not). PowerShell 5.1 `Set-Content -Encoding UTF8` writes a BOM that breaks
mid-assembly module loads (`unexpected token '�'`) — write stdlib/.kark files
UTF-8-no-BOM. IO handles are shared: `io_close` before `io_delete_file` on
Windows or the file stays locked. String-array print differs (Go `["x"]` vs
kcc `[x]`) — examples iterate instead. Gates: `pkg/cli/phase109_stdlib_test.go`
(5 golden examples incl. UTF-8 byte round-trips `68c3a9...`, multi-module E2E
`examples/stdlib_v2` with `sha256("karkain") =
00e0cba20c10cac449eb885a9926a4b646f0ac163ed7fbc5704d9d8a057ef44d` +
determinism, module-assembly guard, negative suite, missing-module rejection)
+ `pkg/sema/phase109_builtins_test.go`. Regressions green: Phase 106/107/108
CLI gates, `pkg/wasm`, `pkg/compiler` (incremental), `pkg/codegen`,
lexer/parser/sema/ir/pm/source/diagnostics/module/backend/npu/runtime,
`go vet`, `go build`. WASM: new builtins are K108-gated (native-only) — no
WASM change, documented boundary. `stdlib/core`, `stdlib/math`, `stdlib/system`,
`stdlib/gpu`, `stdlib/async` remain behind this phase (non-canonical syntax /
no builtin backing). Report:
`docs/audit/PHASE-109-STANDARD-LIBRARY-V2-FINAL-REPORT.md`.)
Also completed: **108** — WASM Target
(Karkain-owned wasm32-wasi backend in new package `pkg/wasm/` — a
dependency-free handwritten WASM binary v1 emitter:
`module.go` (value model + encoder), `emit.go` (section serialization),
`runtime.go` (embedded WASI runtime: `fd_write` via
`wasi_snapshot_preview1.fd_write`, unboxed i64 ints `v=i64<<1`, boxed
container cells `(ptr<<1)|1` tag/len/data, heap base 0x10000 global 0,
scratch nwritten@0/iovs@8/digits@16–144, `rt_Write`/`rt_PrintValue`/
`rt_Box`/`rt_SetTag`/`rt_MkArray`/`rt_MkString`/`rt_Eq`/`rt_Ne`/`rt_Error`,
24 runtime bodies = wasm indices 1–24, user-func index =
`len(mb.Codes)+i+1`, rt.Eq=11, rt.Ne=12) and `backend.go`
(`CompileProgram` imports+pre-indexes user funcs first, walks FuncDecl
bodies, `scanUnsupported` K108-gates floats/maps/slices/C-interop/
concurrency with `error[K108]` diagnostics; `spew`-level no, no `I32Mul`/
`I32Shl` — `I64ExtendI32U;I64Const(8);I64Mul;I32WrapI64`, `BeginIf
(noResult)`=`04 40`). Root-cause fix: `eqLocals` had 7 entries
`{I32×6,I64}` so p7 was i32 while the shifted i64 accumulator wrote through
p7 → wasmtime `expected i32, found i64`; corrected to 6
`{I32×5,I64}` (comment `// p2=cellA p3=cellB p4=tagA p5=n p6=i p7=acc`).
CLI: `pkg/cli/wasm.go` `wasmBuildCommand`/`wasmRunCommand`/`findWasmtime`,
dispatched from BuildCommand/RunCommand on `--target wasm32-wasi`
(exit codes: ExitCompile=3 for K108, ExitEnv=6 = no wasmtime, ExitFailure=1,
ExitSuccess=0; `wasmtime run --dir . <tmp.wasm>` stdio passthrough;
findWasmtime: LookPath then `$HOME/bin/wasmtime{.exe}` — Windows mode has
no execute bits so dropped `Mode()&0o111`). Gates: 9 backend tests
(pkg/wasm/backend_test.go — TestHello/ArithmeticAndCalls/
ControlFlowAndRecursion/ArraysAndStrings incl. string equality/
RuntimeErrorDivByZero `main.kark:3`/DeterministicBuild/UnsupportedFeatures
6-case K108 table/BooleanLogic/ForIn) + 2 E2E CLI tests
(pkg/cli/phase108_cli_test.go, filename avoids go-build GOARCH trap:
`*_wasm_test.go` is build-constrained to GOARCH=wasm and lands in
IgnoredGoFiles on windows/amd64 — use non-GOARCH suffixes like
`phase108_cli_test.go`): TestPhase108WasmE2E builds
`examples/wasm/hello.kark` via real `karkain build --target wasm32-wasi` and
wasmtime-runs to byte-exact `hello wasmtime/42/done`, determinism test
proves byte-identical 2488-byte repeats. Regressions green: pkg/wasm full,
Phase 106/107 codegen+runtime gates, Phase 95/96/97/105/106/107 CLI gates,
`go vet`, `go build ./...`; compiled standalone `karkain.exe`
build+run byte-exact. kcc parity for the wasm target = documented post-108
boundary (Go engine is the reference). Report:
`docs/audit/PHASE-108-WASM-TARGET-FINAL-REPORT.md`.)
Also completed: **107** — Concurrency Runtime
(A work-stealing task scheduler with channels and actors usable end-to-end
from `.kark` through a compiler-neutral C runtime embedded in the generated
assembly. Runtime under `runtime/concurrency/c/`
(`karkain_conc.h`/`karkain_sched_impl.h`/`karkain_scheduler.c`/
`karkain_channel.c`/`karkain_actor.c`/`concurrency_main.c`): `karkain_sched_t`
workers with a bounded grab queue, per-worker wake condvar, shared pending
drain and no busy-spin stop; `karkain_task_t` heap cells (fn ptr + heap-owned
`ctx` freed by the runtime, atomic `done`, cv, `status` long — negative =
failure); bounded/unbounded blocking channels with deterministic
close-drain; actors as serialized dispatcher jobs over a mailbox with a
state-cell box; `karkain_conc_atomic_*` GNU atomic helpers in the runtime
only. Embedded via `runtime/concurrency/embed.go` (`//go:embed c/...`).
Compiler integration: parser `spawn(fn, args...)`/`receive(ch)` expression/
`channel(...)`+`actor(...)` keyword calls (legacy `receive(ch) -> var`
kept), sema `builtinNames`, codegen `pkg/codegen/conc_runtime.go` (recursive
pre-scan → `usesConcurrency` + stable spawn-site wrapper indices +
actor-handler ids; per-site `karkain_run_<idx>` wrappers; actor dispatcher
adopting returned state with int-0 = no change; `concRuntimeAPIC` Value
glue), `GenerateAndCompile` header/wrapper/glue emission gated on
`usesConcurrency`. Language surface: `spawn/join/wait_all/channel/chanSend/
chanClose/receive` and `actor/actorSend/actorState/setActorState/actorStop`
(`send` is a lexed keyword → `chanSend`). Gates: `pkg/runtime/
phase107_concurrency_test.go` (7 scenarios + >1M msg/s), `pkg/codegen/
phase107_concurrency_test.go` (7: spawn 42/10/-1/done, spawn statements,
channels 10/20/0, actors `6`, embedded markers, 999-task stress
`332833500`, generated-source presence), `pkg/cli/phase107_concurrency_test.go`
(2 E2E: `examples/concurrency/pipeline/main.kark` → `144/10/20/30/0/6` +
byte-identical repeats). Regressions green: full `pkg/cli` 1723.8s
(conformance 59/59 + all Phase 97–107 gates), whole `pkg/codegen`,
sema/parser/lexer/ir/pm/source/diagnostics/compiler/module,
backend/npu/runtime, `go vet`, `go build`; bootstrap stage-1 builds (Go
codegen change safe for the compiler's own sources); stage-2 build SEGFAULT
on the ~3.9GB-RAM host is the documented kcc-build-mode OOM class (no
`src/compiler` changes; concurrency codegen gated by `usesConcurrency`,
unused by compiler sources; Go-engine `check` of `src/compiler/main.kark`
clean). kcc already lexes/parses the keywords (`TK_SPAWN/TK_SEND/...`);
codegen parity is a documented post-107 boundary. Report:
`docs/audit/PHASE-107-CONCURRENCY-RUNTIME-FINAL-REPORT.md`.)
Also completed: **106** — SIMD & Vector Types
(Lane-vector types: `[N]f32`/`[N]f64`/`[N]i32`/`[N]i64` variable annotations
select Karkain-owned C lane types `karkain_<elem><width>x<lanes>` (float →
`__m128`/`__m256` with auto `-mavx` for 256-bit widths on x86; int → GNU
`vector_size` types), and `@simd_splat/add/sub/mul/div/sum` lower to portable
`karkain_simd_*` runtime helpers using GNU vector operators — one body serves
x86 intrinsics and ARM `vector_size` types; integer mul/div and every sum
reduce lane-wise via scalar loops. Type flow: SIMD declaration records
name→type in `Generator.simdVars`, operand dispatch takes lane width from the
vector operand, splat seed infers f32 vs i32 (raw literal or `.floatVal`/
`.intVal`, never an invalid Value→scalar C cast), `@simd_sum` reduces a lane
vector to a printable scalar Value, scalar-only operands keep the Phase 70 SSE
fallback. Integration: `pkg/codegen/simd_emit.go` (new) + `genSIMDDecl`/typed
`genSIMDExpr`/header-runtime assembly/`appendAVXFlags` in `pkg/codegen/codegen.go`,
SSA `lowerStmt` → raw-c SIMD decls in `pkg/codegen/lower.go`. Bonus hardening:
the Phase 14 AVX2 matrix kernel now uses portable `mul+add` (no
`_mm256_fmadd_pd`), so `-mavx2` builds no longer require `-mfma`. Gates:
`pkg/codegen/simd_emit_test.go` (7) + updated `phase70_test.go` +
`pkg/cli/phase106_simd_test.go` (3 E2E: golden executable output
40/-8/48/12/5/56/40, SIMD-vs-scalar differential, AVX assembly probe
`vaddps`-family under the pipeline's own `-O0 -mavx`). Regressions green: full
`pkg/cli` 940.5s (conformance 59/59, all Phase 97–105 gates), whole
`pkg/codegen`, sema/parser/ir, backend/npu/runtime/diagnostics, `go vet`,
`go build`; bootstrap stage-1 builds (Go codegen change safe for the
compiler's own sources); stage-2 build SEGFAULT on the ~4GB-RAM host is the
documented kcc-build-mode OOM class (no `src/compiler` changes; `@simd`
unused by compiler sources). kcc parity boundary documented (parser already
accepts `@simd_*` via `NODE_SIMD_BUILTIN`; semantic/codegen parity post-106).
Report: `docs/audit/PHASE-106-SIMD-VECTOR-TYPES-FINAL-REPORT.md`.)
Also completed: **105** — Error Recovery & Incremental Compilation
(Multi-error reporting: engine-agnostic project-wide syntax preflight
`pkg/cli/multierror.go` parses every unit file independently and renders ALL
recoverable parse errors in one invocation with exit 3 through BOTH check
engine paths — Go `CheckCommandFormatted` and default-kcc
`KCCCheckCommand`, which previously `[ok]`-ed multi-error inputs; the
resolve stage aggregates its diagnostics the same way, so
`examples/phase105_errors/` proves 6 recoverable parse errors (multierr.kark:
`@bad_attr_*` + `public <non-decl>` + expression-shape) and 3 resolve errors
(semantic.kark) surface together. Incremental compilation: content-addressed
whole-assembly cache in new package `pkg/compiler` (per-module content
sha256 + interface hashes — public func param types / struct-enum headers /
top-level var headers / imports only, never bodies; compiled/reused/
invalidated; dependency-aware invalidation; compiler-identity key; atomic
writes; failed builds never Store); `karkain build --incremental` /
`--incremental-cache` via `pkg/cli/incremental.go` and `--incremental` flags
in `cmd/karkain/main.go`; `karkain clean` purges `.karkain-cache`. Real
3-module example `examples/phase105/` (main+math+strings). Gates:
`pkg/compiler/incremental_test.go` (10) + `pkg/cli/phase105_incremental_test.go`
(3 E2E through gcc) + `pkg/cli/phase105_multierror_test.go` (5) — all PASS;
measured no-op 101 ms vs clean 2790 ms; post-clean rebuild byte-identical C
and stdout; full regression suite green incl. entire `pkg/cli` (conformance
59/59, all Phase 97–104 gates) 1066.8s, other pkg suites, `go vet`, `go build`.
Design decision (documented): whole-assembly cache; per-module `.o` TU
splitting deferred as post-105 work on the same interface machinery.)
Also completed: **104** — Debug Information (DWARF)
(Karkain-owned DWARF 4 debug sections in native executables:
`.debug_info`/`.debug_abbrev`/`.debug_str`/`.debug_line` emitted by
`DwarfEmitter` in `pkg/codegen/dwarf.go` from the Phase 84 `DebugInfo`
model — compile unit, subprogram per function with nested local variables,
base-type DIEs, `DW_AT_high_pc` size form over relocated addresses, and a
correct v4 line-number state machine with per-function end-sequence resets.
Self-hosted DWARF-4 reader `pkg/codegen/dwarf_parse.go` round-trips the
sections and powers the readelf-style `DwarfTextDump`; linker lays out
`SectionTypeDebug`; `NativeBuilder.Build`/`BuildMultiObject` attach DWARF
after `relocateDebugAddresses`; new `Executable.GetSection`,
`GetDebugSections`, `HasDebugSections`. Gates `pkg/codegen/dwarf_test.go`
(8 tests) + `pkg/cli/phase104_dwarf_test.go` (3 E2E through the real
pipeline) — all PASS; regressions green: conformance 59/59, Phase 102
Go+kcc goldens, compiler-sources self-check, probes, all pkg suites,
`go vet`. Known limitation (documented, NOT a defect): no ELF/PE container
writer yet, so debuggers/readelf remain on the gcc C-transpile path; the
DWARF sections are ready for embedding when a container writer lands.
Tier 3 (Phases 104–109) is now in progress.)
Also completed: **103** — Module System v2 (Core)
(`public` is a real export modifier accepted by BOTH engines: Go parser sets
`.Public` on func/type/enum; self-hosted kcc gained a `TK_PUB` branch mirroring
Go's parse-error contract (`public let` rejected on both engines). Qualified
calls `math.twice(21)` now resolve against the imported module's export set in
the Go resolver (`checkQualifiedCall`, `pkg/sema/resolve.go`) with precise
diagnostics — private cross-module call, missing `import`, undefined module
function, wrong-module target, cross-module duplicate — all exit 3. The
self-hosted parser lowers dotted calls onto the flat `karkain_user_*`
namespace identically for module qualifiers and record-idiom receivers
(`acc.deposit(10)` == `math.twice(21)` == bare callee); `C.*` stays dotted for
C-interop. Stdlib files are exempt from the private-export rule (framework API
surface; physical `public` markers deferred to the Phase 109 stdlib-v2
boundary). Acceptance: `examples/module_system/` byte-identical output on both
engines; 4 rejection fixtures `examples/module_system_errors/`; gate
`pkg/cli/phase103_module_test.go` (Go golden 4.8s, kcc golden 10.6s, kcc
accept-check, rejections, compiler-sources self-check 41.9s); regressions:
sema/parser/codegen/vet + Phase 97/99/100/101/102 + conformance 59/59 —
all green. Report: `docs/audit/PHASE-103-MODULE-SYSTEM-FINAL-REPORT.md`.)
Also completed: **99** — Self-Hosted Parser & Type Checker
(Certified: READY TO COMMIT WITH DOCUMENTED ENVIRONMENTAL LIMITATION — committed
and merged). **100** — Runtime Error Model (runtime error diagnostics with source
location, checked division/modulo/indexing, cross-engine parity). The self-hosted
compiler now parses the full language surface and type-checks programs itself:
`src/compiler/atypes.kark` (conservative static type inference: `inferType`,
`primitiveTag`, `isPrimitiveTag`, `typeDescription`, `annotationMismatch`; the
"a" prefix sorts it after `ast.kark` for the letter-order assembler) and
`src/compiler/checker.kark` (two-pass checker mirroring `pkg/sema/resolve.go`:
pass 1 `collectDecls`/`collectLocals`, pass 2 `checkStmt`/`checkExpr`/
`checkCall`/`checkIdent`; K101 undefined call, K102 undefined identifier,
K103 function/kernel arity, K104 builtin arity, K106 undefined struct type,
K107 duplicate definition, K108 break/continue outside loop, K109 calling a
type name, K112 primitive annotation/initializer mismatch — conservative,
provable-from-AST only, so the compiled corpus and the compiler's own sources
keep passing). `src/compiler/main.kark` `checkFile` runs `typeCheckProgram(ast)`
check-only: `error[K1XX]` lines map to exit 3, clean files still print `[ok]`;
build/run/test codegen untouched (bit-identical to Phase 98). Gate
`pkg/cli/phase99_selfhosted_test.go` (CorpusAccept, CrossFileDuplicateDetected
at assembly scope, ErrorFixturesRejected 14/14, CompilerSourcesTypeCheck —
assembled sources 214,127 bytes type-check clean) — PASS 54.58s; fixtures under
`examples/type_errors/`. Bootstrap identity PASS 347.60s, stage2==stage3,
1,432,956 bytes, SHA `aff1d624d9e52c2d`; isolated `pkg/cli` 634.128s and
`pkg/bootstrap` 571.688s; `go vet` clean. Harness stabilizers: bootstrap
subprocess timeout 2min→5min (measured clean transpile ~145–174s exceeded the
120s budget), and `phase88`/`phase95` tests pin `KARKAIN_ENGINE=go` (legacy
contract + stage-1 determinism) — no production semantics changed. Known
environmental limitation (documented, NOT a defect): on the ~4GB-RAM
limited-paging host, concurrent full-tree `go test ./pkg/...` runs stall
`TestBootstrap_BitwiseIdentity` and `TestConformanceCorpus_RunsClean`; both pass
in isolation. Full report: `docs/audit/PHASE-99-FINAL-REPORT.md`.
Also completed: **100** — Runtime Error Model
(Runtime error diagnostics with source filename and line number: division/modulo
by zero, array/string index out of range now report `runtime error: <kind> at
<file>:<line>` and exit(1) instead of silently returning zero. Both Go and
self-hosted kcc engines produce identical diagnostics. Implementation:
`karkain_runtime_error()`, `karkain_checked_div/mod/get/set()` in
`pkg/codegen/codegen.go` and `src/compiler/codegen.kark`, source file tracking
via `sourceBaseC()` and `fileBaseName()`, enhanced string concatenation to
support int/float/bool operands for diagnostic embedding. Test fixtures under
`examples/runtime_errors/` (7 error cases + 1 positive control), parity gates
`pkg/cli/phase100_runtime_test.go` and `pkg/codegen/phase100_runtime_test.go` —
all PASS.)
Also completed: **101** — Libc-Free Runtime Foundation
(libc-free/libm-free Karkain-owned runtime layer established: `runtime/freestanding/`
with arena allocator (`karkain_memory.c`), raw OS abstraction (`karkain_platform.c/h`),
I/O, string and math primitives that compile and link WITHOUT libc — gate
`pkg/runtime/phase101_freestanding_test.go` compiles and runs a hello program;
runtime error stack traces added end-to-end: `KARKAIN_MAX_FRAMES` 128,
`karkain_frame_enter/leave`, `karkain_set_line`, runtime-error `stack:` dump on both
engines (kcc and Go produce identical `inner:2/outer:5/main:9` frames for
`examples/runtime_errors/stack_chain.kark`); Go side: `pkg/codegen/codegen.go`,
`pkg/ir/ssa/ssa.go` (`Function.Line`), `emit_ir.go`; self-hosted side:
`src/compiler/codegen.kark` (frame helpers, `FuncDecl` prologue, return wrapper
`{ Value _karkain_fret = ...; karkain_frame_leave(); return _karkain_fret; }`);
latent self-hosted checker crash fixed (`kcc check` OOB on bare `return` empty
value nodes — empty-node guards in `src/compiler/checker.kark`) plus `parseFunc`
func-token line fix in `src/compiler/parser.kark`; regression gates
`pkg/cli/phase99_selfhosted_test.go` / `phase100_runtime_test.go` /
`phase101_stacktrace_test.go` all PASS (CompilerSourcesTypeCheck clean on all 9
self-hosted sources); GMP remains an intentional isolated boundary deferred to
Phase 109; pre-existing limitation (not a blocker): closure/`fn` codegen is broken
on both engines, covered by no gate, out of Phase 101 scope — docs under
`docs/audit/PHASE-101-FINAL-REPORT.md`).
Also completed: **102** — Native Runtime Core
(Karkain-owned libc-free runtime foundation `runtime/core/` stacked directly on
the Phase 101 freestanding layer: `karkain_mem` heap with real
`alloc/calloc/realloc/free` on an address-sorted coalescing free list; tagged
`Value` int/float/bool/string/array with codegen-parity naming/layout
(`ValueType`, `TYPE_*`, `make_int/…/make_array`, `values_equal`,
`value_truthy`); length-prefixed native strings `NativeString` (concat/slice/
compare/startswith/Value round-trip); native arrays `NativeArray`
(push/pop/get/set/clear, codegen-parity `Value** items` storage); value-level
I/O on freestanding write primitives; gate `pkg/runtime/phase102_core_test.go`
compiles core+freestanding with `-ffreestanding -nostdlib` and asserts exact
output. Directly-related freestanding fixes: `karkain_memory.c` rewritten as a
linked-segment arena (old arena reloc'd every outstanding pointer on grow and
could fail to make room — only guaranteed `add>=need`, not `add>=used+need`;
the old hello gate only ever allocated once, so it never tripped) +
`karkain_arena_reset`; `karkain_mem_reset` clears heap bookkeeping after arena
teardown. Compilers untouched (both engines keep passing); GMP/bigfloat and
map/option/result stay behind the Phase 109 boundary; codegen embedded-runtime
adoption is a future phase. Docs under `docs/audit/PHASE-102-FINAL-REPORT.md`).
Also completed: **102‑F** — Language Foundation Completion
(Language surface round-trip proven on BOTH engines with a golden-output
corpus: 13 targets under `examples/language_foundation/` (variables→functions→
recursion→arrays→strings→collections→control flow→structs→methods(record
idiom)→modules(sibling assembly)→application layout→error handling) producing
byte-identical stdout + exit codes on Go and kcc engines. Cross-engine parity
bugs fixed: kcc `parseStructDecl` dropped every top-level function after a `;`
field separator (`{ owner string; balance int }` ate the source to the next
`}`) — the application `deposit()` regression; Go BorrowChecker `fnRoot`
flag stops dead-name propagation out of function/lambda root scopes (params no
longer poison the global scope and false-flag struct-literal keys like
`Counter{ value: 0 }` as "used after scope end"); kcc C-style `for` empty-init
`for (; k < 3; ...)` and `for (let i = 0; ...)` (parseFor let/var init +
`"; "`/`is_truthy()` emission in `src/compiler/parser.kark`/`codegen.kark`);
Go `genForStmt` double-`;;` on let-init trim in `pkg/codegen/codegen.go`.
Gates `pkg/cli/phase102_foundation_test.go`: Go golden 44.6s PASS, kcc golden
(13 targets, isolated kcc) 30.5s PASS, compiler-sources self-check
(`check --engine kcc` of `src/compiler/main.kark`, Phase 99 gate survival)
55.6s PASS; targeted regressions green: Phase 99 self-hosted
(CorpusAccept/CrossFileDuplicate/ErrorFixtures 14/14/CompilerSourcesTypeCheck
58.8s), Phase 100 runtime parity, Phase 101 stack parity, pkg/codegen Phase 100,
pkg/sema Phase 51 borrow; `go vet` clean; `karkain fmt` semantic-preserving.
Not supported on either engine (kept out of corpus, parity preserved):
`float64()/bool()/string()` casts, closures/`fn` codegen, `const`, user
`import`, visibility. Known environmental limitation (not a defect): on the
~4GB-RAM host kcc BUILD mode for the full compiler sources OOM-stalls/SEGFAULTs
(known class, see Phase 99/101 notes); the low-memory `check` path passes.
Docs under `docs/audit/PHASE-102-LANGUAGE-FOUNDATION-FINAL-REPORT.md` +
SPEC §14 Capability Summary).
Also completed: **98** — NPU Integration into Compiler
(`@target(...)` function attribute implemented end-to-end: parser attaches the
attribute to `FuncDecl` (`pkg/parser/ast.go` `Target`, `parseTargetAttr` in
`pkg/parser/parser.go`, preserved by `pkg/parser/macro.go` expansion), semantic
validation in `pkg/sema/npu_check.go` (`targets: cpu, npu` — unknown targets
rejected with `error[K004]`, exit 3), C codegen emits a `// @target(...)`
comment marker, and `pkg/codegen/npu_compiler.go` adds the `NPUDispatcher`
with `DispatchRequest`/`DispatchResult` and matrix multiplication as the first
concrete NPU operation: `OpMatMul` dispatches to an available vendor NPU
adapter (intel/qualcomm/apple/amd/arm — `backend->Execute` must return computed
values) and ALWAYS falls back to the CPU reference backend (`pkg/backend/cpu`,
the correctness oracle) when no adapter is available, the adapter errors, or it
stubs out — so `@target(npu)` functions compile and run everywhere with no
NPU/SDK/driver/cloud dependency; the self-hosted compiler was taught the
attribute too (`src/compiler/parser.kark` `parseTargetAttr` claims top-level
`@target(name)` before `func`, `ast.kark` `setFuncTarget`/`funcTarget`, `codegen.kark`
emits the C comment instead of an invalid `target(npu);` statement), and the
default kcc engine's check/build/run/test paths run a Go-side NPU target
preflight (`kccTargetPreflight`/`kccStagedPreflight`) so `karkain check`
rejects unknown targets on BOTH engines; CLI wiring in `pkg/cli/checker.go`
(`npuTargetDiagnostics` in `AnalyzeSource`) + `pkg/cli/lint.go`; parity tests
`pkg/sema/phase98_npu_test.go` (8), `pkg/codegen/phase98_npu_test.go` (10,
mock NPU backend with software-emulated execute proving NPU==CPU results) and
`pkg/cli/phase98_npu_test.go` (8) all green; docs under
`docs/npu-targeting.md` + `docs/audit/PHASE-98-FINAL-REPORT.md`).
Also completed: **97** — Default kcc Engine + Manifest Dependency Resolution
(kcc is now the DEFAULT engine: `EngineFromEnv` returns `EngineKCC` for empty
env/unrelated `KARKAIN_ENGINE` values, Go selected only via explicit
`KARKAIN_ENGINE=go|Go|GO`; `--engine go|kcc` unchanged; manifest dependency
resolution wired into the source-assembly pipeline: new `kccAssembleSource`
(local deps → siblings → root last) feeds `KCCCheckCommand`/`KCCBuildCommand`/
`KCCRunCommand` temp-sandbox builds, `projectModuleSources` prepended to each
test driver in `KCCTestCommand` (dependency-aware test compilation), and
`resolveSources` (check/build/run) + `projectModuleSources` (test) pull
`pm.DependencySources` entries (local & workspace) for all engines; bootstrap
pipeline pins `KARKAIN_ENGINE=go` (`forceGoEngine` in `pkg/bootstrap`) so stage-1
still emits `src/compiler/main.c` through the Go front end; E2E exit-code tests
pin Go deterministically; parity gate `pkg/cli/phase97_parity_test.go` (6 tests:
default-engine flip, explicit Go fallback, local & workspace deps in assembly,
end-to-end kcc run using a dependency function, shared check/build/run/test
dependency-aware pipeline), plus existing parity gates (95/96) green and
`TestBootstrap_BitwiseIdentity` stage2==stage3 bitwise identical; docs under
`docs/audit/PHASE-97-FINAL-REPORT.md`).
Also completed: **96** — Self-Hosted kcc Owns the Test Runner
(kcc is the primary engine for `karkain test`: self-hosted discovery in
`src/compiler/main.kark` (`collectTestFiles` mirrors Go `findTestFiles` —
recursive `*_test.kark` discovery, sorted, single-file fallback, empty
directory reports "No test files found."), per-test driver synthesis
(parse → collect `test_*` → generated `main` calling selected tests),
gcc compile+run with per-test PASS/FAIL, full-file fast path, `--filter`
substring support, and Go-parity summary output (`N passed; M failed;
S skipped; T total`) parsed by `KCCTestCommand` (`karkain test
--engine=kcc` / `KARKAIN_ENGINE=kcc`) which maps `failed>0` → `ExitTest(4)`
and runs kcc in a temp sandbox; root-cause fixes: `INT==BOOL` `values_equal`
mismatch from `endsWith(...) == true` → bare truthiness for INT/BOOL, Windows
`system("./x")` → bare `.exe` name, empty-directory vs single-file
classification via `readFile`/suffix probe; parity gate
`pkg/cli/phase96_parity_test.go` (conformance parity 59 assertions, failing
test, single-file+filter, empty directory); bootstrap identity
`TestBootstrap_BitwiseIdentity` stage2==stage3 bitwise identical
(stage2/stage3 SHA `f39111a2…`) and conformance corpus 59/59 all pass;
docs under `docs/audit/PHASE-96-FINAL-REPORT.md`).
Also completed: **95** — Self-Hosted kcc Owns the Core Pipeline
(self-hosted compiler from `src/compiler` is the primary engine for
lex+parse+sema+C codegen; `karkain check/build/run --engine=kcc` or
`KARKAIN_ENGINE=kcc` decompiled through kcc; `pkg/cli/kcc_engine.go` with
`KCCCheckCommand`/`KCCBuildCommand`/`KCCRunCommand` (temp-sandboxed run), a
staleness check rebuilding `kcc.exe` when any `src/compiler/*.kark` is newer,
and Go fallback for LSP/test-runner/exotic backends; CLI `--engine go|kcc`
flag + env wiring in `cmd/karkain/main.go`; kcc parity fixes for type-keyword
annotations, unparenthesized if, map/struct literals, slices, index/member
assignment, assertions, appendArray Value* and struct-decl comments;
parity gate `pkg/cli/phase95_parity_test.go` proving kcc reproduces all 12
probe goldens + all 11 conformance files (59 assertions); verified
`TestBootstrap_BitwiseIdentity` stage2==stage3 bitwise identical;
docs under `docs/audit/PHASE-95-FINAL-REPORT.md`).
Also completed: **94** — SSA Optimization Pipeline
(Multi-pass optimizer: Mem2Reg, FoldConst with algebraic simplification,
CSE, DCE, LICM for natural loops; Pipeline orchestrator with fixpoint
iteration and stats; 49 passing tests including benchmarks; `pkg/ir/ssa/`).
Also completed: **93** — Typed SSA IR
(TypeRegistry, TypeBits, TypePromote, TypeIsCompatible, dominance tree
(Cooper et al.), liveness analysis, SSA verifier; 30 tests; `pkg/ir/ssa/`).
Also completed: **92** — HIR Infrastructure
(High-Level IR with typed nodes: 20 expression kinds, 16 statement kinds,
19 type kinds, AST→HIR lowering via BuildHIR(), Format() for debug output,
7 passing tests including 5-program round-trip; `pkg/ir/hir/`).
Also completed: **91** — Comprehensive Validation & Release Decision
(audit of all documentation for consistency: SPEC.md version 0.14.0→1.0.0,
self-hosted compiler reference Phase 56→88; stdlib.md import examples updated
for async/gpu modules; full regression suite GREEN: lexer/parser/sema/ssa/pm/
codegen/backend/npu/source/module/diagnostics all pass, CLI passes with 300s
timeout for conformance corpus; release decision: KARKAIN 1.0 — RELEASE READY;
`docs/audit/PHASE-91-FINAL-REPORT.md`).
Also completed: **51** — Borrow Checker Lexical Scoping
(scope-stack identity hardened for shadowing, borrow reversion via declaring scope,
use-after-scope-end diagnostics, escape-analysis flag wiring, Groups A–I tests).
Also completed: **81** — Compiler Correctness (immutable/mutability semantics,
escape-analysis wiring, executable probes under `examples/phase81-probes/`,
diagnostics hardening; unparenthesized `if` + single-path `%`).
Also completed: **82** — Language Conformance, Examples & Developer Tooling
Foundation (native conformance corpus `conformance/` — 9 files, 48 `func test_*`
tests through the real front end + C runtime, self-contained file scope via
`testFileOwnScope`; deterministic probes corpus `examples/probes/` — 11 golden
output programs enforced by `pkg/cli/probes_corpus_test.go`; algorithm corpus
grown to 21 dirs (factorial/fibonacci/gcd/lcm/sieve/power/absolute_value);
toolchain contract `check --format=json` (`karkain-diagnostics-v1`, real parse
columns via `Parser.ErrorCols`) + `fmt`/`fmt --check` (idempotent token-level
canonicalizer); real LSP served by `karkain lsp`/`language-server`; `ide info`
JSON contract; VS Code extension rebuilt (`extension.js` commands check/compile/
run/format, fixed manifest, corrected grammar) validated by
`pkg/cli/vscode_extension_test.go`; CI runs conformance + format checks; docs
under `docs/audit/PHASE-82-*`).
Last completed: **83** — Compiler Symbol Namespacing, True Diagnostic Spans &
LSP↔CLI Pipeline Sharing (deterministic `karkain_user_*` C namespace for user
functions — Go `userFuncC` + self-hosted `codegen.kark` mirror — fixes
C-library collisions like `abs` and makes user functions that shadow builtins
win; `E-K-RES` true columns: lexer 0-based byte columns, parser `Col`/`EndCol`
spans surviving macro expansion, optional `endColumn` + `excerpt` fields in the
`karkain-diagnostics-v1` contract; new `pkg/source` line-index/excerpt package;
single `cli.AnalyzeSource` driver shared by `karkain check` and the LSP with
real-time `didChange` diagnostics sync; conservative undefined-identifier
resolution; array-return semantics; conformance corpus 48→59 tests in 11 files
+ `namespace` probe golden (12 total); docs under
`docs/audit/PHASE-83-FINAL-REPORT.md`).
Last completed: **94** — SSA Optimization Pipeline
(Multi-pass optimizer: Mem2Reg, FoldConst with algebraic simplification,
CSE, DCE, LICM for natural loops; Pipeline orchestrator with fixpoint
iteration and stats; 49 passing tests including benchmarks; `pkg/ir/ssa/`).
Last completed: **93** — Typed SSA IR
(TypeRegistry with 14 primitive types, TypeBits/TypePromote/TypeIsCompatible
type-lattice, Cooper et al. iterative dominance tree (DomTree with IDom,
DomChildren, Dominates/StrictlyDominates/DominanceFrontier/DomTreeDepth),
iterative liveness analysis (LiveRange, Interferences, NumLiveAt),
SSA verifier (undefined register, unterminated block, undefined block target),
30 passing tests across `typed.go`, `dom.go`, `liveness.go`, `typed_test.go`;
`pkg/ir/ssa/`).
Last completed: **90** — Production Release Gate / Karkain 1.0
(production build workflow verified, CLI commands validated: check/build/fmt/lint,
stdlib imports work, runtime type discrepancy resolved as intentional bootstrap
limitation, release acceptance test, 15 focused Go tests, version 1.0.0 consistent,
40+ total tests pass; `docs/audit/PHASE-90-FINAL-REPORT.md`).
Prior completed: **89** — Self-Hosted Runtime & Toolchain
(Karkain-owned runtime boundary with Value type system, container ops,
I/O primitives, platform abstraction, initialization contract;
runtime/ directory with 5 boundary docs; acceptance test proving
self-hosted compiler → C23 → gcc → native executable pipeline;
12 focused Go tests; `docs/runtime.md`; `docs/self-hosted-compiler.md`).
Prior completed: **88** — Self-Hosted Compiler Foundation
(self-hosted Karkain compiler compiles via bootstrap: src/compiler/*.kark
→ C23 → native executable; lexer/parser/AST/sema/codegen in Karkain;
7 representative test programs; 8 focused Go tests; build script;
docs under `docs/self-hosted-compiler.md`).
Prior completed: **87** — Standard Library Foundation
(stdlib structure with core/string/collections/math/io/system modules,
`std.*` import resolution wired into module graph via `findStdlibDir`,
6 focused module-resolution tests; `docs/stdlib.md`).
Prior completed: **86** — Developer Toolchain
(test timeout safety via goroutine+select with 30s default, multi-file
formatter (`karkain fmt .` recursive), exported `Canonicalize` for LSP
reuse, `textDocument/formatting` LSP support, `ExitLint(7)` exit code,
8 focused tests; docs under `docs/audit/PHASE-86-FINAL-REPORT.md`).
Prior completed: **85** — Native Build & Linking Integration
(NativeBuilder orchestrating Object→Linker→Executable pipeline via
`NativeBuilder.Build` and `BuildMultiObject`; CLI integration:
`--target=native-link` flag dispatches `BuildCommand`/`RunCommand` to
`nativeBuildCommand`/`nativeRunCommand`; KOBJ binary artifact format
(sections/symbols); debug address relocation from 0-based counters to
actual linked .text base (0x1000+) via `relocateDebugAddresses`;
SourceAddressMap built from relocated DebugInfo; 8 focused NativeBuilder
tests + 6 CLI integration tests; docs under `docs/native-codegen.md`).
Prior completed: **84** — Native Codegen, Linker & Debug Information
(Karkain-owned object model: sections/symbols/relocations/debug-info;
scope-aware SymbolTable with `karkain_user_*` namespacing via
`CollectSymbolsFromAST`; RelocationManager with Addr32/64, PCRel32/64,
PLT32, GOT32 and byte-level application; Linker (symbol resolution,
section layout, entry-point handling, Executable production);
DebugInfoBuilder + SourceAddressMap bidirectional source↔address mapping;
CodegenDiagnostics wrapper reusing Phase-83 `E-K-CG` diagnostics;
NativeGenerator.GenerateObject AST→Object path; 48 focused tests across
6 new test files; docs under `docs/native-codegen.md`).
Prior completed: **79** — Compiler Integrity, IR Architecture & Self-Hosting
Readiness Audit (evidence-based audit: pipeline, dependency map, Math/Tensor/SSA
IR, CPU/GPU/NPU parity, determinism, optimization boundaries, tests,
BUG-1..8 regression, self-hosting readiness; Phase 80 gate = READY WITH
PREREQUISITES; docs under `docs/audit/PHASE-79-*`).
Prior completed: **78** — NPU Optimization (fusion, memory planning, INT8/INT4 quantization, MLIR codegen);
**70-78** Math/Tensor/NPU chain (see below); **52-69** value/SSA/closures/slices/self-hosting/actors/GPU/quantum/stdlib/borrow/optimizer.

### Completed: Package Manager Hardening (P0)
Deterministic resolver (`pkg/pm/resolver.go`), lockfile-integrated workflows
(`pkg/pm/flow.go`: `ResolveAndLock`, `FetchLocked`, `ResolvedDetails`,
`TreeLines`, `NewProject`), fetch cache safety (atomic temp→rename + checksum in
`FetchModule`).

### Completed: CLI Toolchain (Batches B1–B4) + KTF-001 + KTF-002
Full CLI surface (`bench/lint/explain/clean/workspace/*/target/config/pkg audit
--json/pkg verify --json` + exit-code scheme + `--filter`), the native test
foundation (`karkain test`: language-level `assert/assert_eq/assert_ne`, KTF-001
model, deterministic discovery/execution, `--filter`), and the compile
pass/fail corpus (`karkain test --compile [dir]`: manifest-driven, real lint
pipeline diagnostics, gcc-gated pass cases, exit 4 on failure). Reports:
`docs/audit/KTF-001-REPORT.md`, `docs/audit/KTF-002-REPORT.md`,
`docs/audit/CLI-COMPLETION-IMPLEMENTATION.md`, `docs/audit/CLI-COMPLETION-MATRIX.md`. CLI wiring under single `karkain.exe`: top-level
`new/remove/update/list/tree/fetch` (incl. `karkain pkg ...` aliases);
`update` now re-resolves and writes `karkain.lock`; `fetch` uses the lockfile.
Registry/git fetching remain explicit "not available" errors (not faked).
Tests: `pkg/pm/resolver_test.go`, `pkg/cli/package_cli_test.go` (E2E). Audit:
`docs/audit/PACKAGE-MANAGER-AUDIT.md`,
`docs/audit/PACKAGE-MANAGER-IMPLEMENTATION.md`. Deferred recommended next step:
wire manifest dep resolution into build/run/check (avoided to keep compiler
pipeline stable).

### Completed: Math/Tensor/NPU Chain (Phases 71–78)

```
71 (Math IR) → 72 (Tensor IR) → 73 (CPU Backend)
                                      ↓
                                 74 (Autodiff Integration)
                                      ↓
                                 75 (Backend Abstraction)
                                    ↓        ↓
                               76 (GPU)   77 (NPU) → 78 (NPU Opt)
```
All 8 phases built, tested (full suite green), committed and pushed to `main`.
Backends: CPU reference (`pkg/backend/cpu`, embedded C23 runtime, correct GCC E2E),
GPU/WGSL (`pkg/backend/gpu`), NPU abstraction + 5 vendor adapters (`pkg/npu/*`).
NPU optimization passes: fusion, memory planning, quantization, Karkain-owned MLIR dialect.

### Phase Dependency Chain (Math/Tensor/NPU)

```
71 (Math IR) → 72 (Tensor IR) → 73 (CPU Backend)
                                      ↓
                                 74 (Autodiff Integration)
                                      ↓
                                 75 (Backend Abstraction)
                                    ↓        ↓
                               76 (GPU)   77 (NPU) → 78 (NPU Opt)
```

### Key Design Decisions

1. **No Tensor keyword** — NPU/math ops work on existing `array` types
2. **No Google TPU** — NPU targets Intel/Qualcomm/Apple/AMD/Arm only
3. **Math IR is Karkain-owned** — not ONNX, not MLIR, not vendor-specific
4. **Tensor IR is Karkain-owned** — not NumPy, not PyTorch
5. **CPU is reference backend** — correctness oracle before any accelerator
6. **NPU is a backend** — not the foundation, not the language

## Guiding Principles

1. **Maturity over features** — depth, correctness, performance, verification before expansion
2. **Semantic foundations first** — value representation, ownership, lifetimes, IR before features
3. **Working > ambitious** — fix broken fundamentals before adding new capabilities
4. **Incremental verification** — every phase must compile, pass E2E, pass all tests

## Testing

Run the full test suite before committing:
```
go test ./pkg/lexer/... ./pkg/parser/... ./pkg/codegen/... ./pkg/pm/... -count=1
```

All tests must pass before committing.


