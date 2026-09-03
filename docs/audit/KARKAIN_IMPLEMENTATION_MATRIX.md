# KARKAIN IMPLEMENTATION MATRIX

Status values: IMPLEMENTED / PARTIALLY_IMPLEMENTED / EXPERIMENTAL / PLANNED / NOT_IMPLEMENTED / UNKNOWN.
Nothing is marked IMPLEMENTED without source/tests evidence.

| Capability | Status | Evidence | Tests | Notes |
|-----------|--------|----------|-------|-------|
| **LEXER** | | | | |
| Tokenizer (streaming) | IMPLEMENTED | `pkg/lexer/lexer.go:207-318` | lexer_test.go (15 fns) | One token at a time, zero-copy offsets |
| ~100 token kinds | IMPLEMENTED | lexer.go:9-123 | lexer_test.go | incl. quantum/actor/meta/GPU tokens |
| Identifiers (ASCII) | IMPLEMENTED | lexer.go:340-350 | — | `[a-zA-Z_]` + digits |
| Keywords (57 in lookupIdent) | IMPLEMENTED | lexer.go:453-565 | lexer_test.go | print & println share TokenPrint |
| Integer/Float/BigInt/BigFloat literals | IMPLEMENTED | lexer.go:352-388 | lexer_test.go:149,196,210,244 | `42n`, `3.14b` |
| Bool literals | IMPLEMENTED | lexer.go:519-524 | — | true/false |
| String literals + escapes | IMPLEMENTED | lexer.go:390-419 | lexer_test.go:121,135 | escapes skipped, raw kept |
| Operators/delimiters | IMPLEMENTED | lexer.go:67-123 | lexer_test.go:94 | ` <- ` → TokenSend |
| Line comments (//) | IMPLEMENTED | lexer.go:331-336 | (SPEC.md:61) | No block comments (as spec'd) |
| Whitespace tracking | IMPLEMENTED | lexer.go:320-328 | lexer_test.go:72 | Line/Col |
| Illegal token | PARTIALLY_IMPLEMENTED | lexer.go:268,312 | — | Emits TokenIllegal, no recovery policy |
| Unicode | NOT_IMPLEMENTED | lexer.go:445-447 (byte-based) | — | ASCII only |
| **PARSER** | | | | |
| Expressions / precedence / associativity | IMPLEMENTED | parser.go:670,692,733 | — | left-assoc precedence climbing |
| Statements / control flow | IMPLEMENTED | parser.go:227,242,370,482 | — | if/while/for/for-in/break/continue |
| Functions | IMPLEMENTED | parser.go:93-134 | — | type annotations |
| Structs + struct literals | IMPLEMENTED | parser.go:1210,1242 | struct tests | |
| Enums | IMPLEMENTED | parser.go:1266 | — | enumNames map |
| Pattern matching (match) | IMPLEMENTED | parser.go:977-1095 | quantum_test | Some/None/Ok/Err/literal/wildcard/binding |
| Macros / quote / unquote / comptime | IMPLEMENTED | macro.go | macro tests | ApplyMacroExpansion |
| Lambdas / closures / captures | IMPLEMENTED | parser.go:337, captures.go, escape.go | captures_test, escape_test | Phase 54 |
| C import blocks | IMPLEMENTED | parser.go:1345 | — | extern "C" |
| Generics (`<T>` syntax) | PARTIALLY_IMPLEMENTED | AST GenericTypeParam ast.go:544; `parseFunc` passes nil genericParams parser.go:95 | generics_test (sema-level) | **Syntax not parsed; AST/allocator only** |
| Error recovery (multi-error) | PARTIALLY_IMPLEMENTED | parser.go:22,33,82,1075 | — | strings, no col/snippet; **OOM risk** |
| **Parser OOM / runaway-memory safety** | **EXPERIMENTAL (BROKEN)** | parser.go:1148-1153,1190-1195,601-606,619-624 (no no-progress guard); parsePrimaryExpr returns nil without advancing (parser.go:973) | TestCodeGenProfile OOMs | Unbounded `args` growslice on concatenated src/compiler — ~3GB OOM |
| **AST** | | | | |
| Strongly-typed node structs | PARTIALLY_IMPLEMENTED | ast.go:5 (`Node interface{}`) + ~100 structs | — | Empty interface root; nominal typing only |
| Arena allocation | IMPLEMENTED | arena.go:13-315 | arena_test.go | NodeID uint32, O(1) Get |
| **SEMANTIC ANALYSIS** | | | | |
| General type checker | NOT_IMPLEMENTED | no TypeChecker anywhere | — | Primitive types only as strings in validators |
| Symbol table / scope resolution / name lookup | NOT_IMPLEMENTED | only borrow scope + bytecode SymbolTable | — | no semantic name-resolution pass |
| Shadowing | IMPLEMENTED (borrow-scope) | borrow_checker.go:37-45,96 | TestBorrowCheck_ShadowVariable | ownership map only |
| Duplicate detection | PARTIALLY_IMPLEMENTED | actor.go:96, coroutine.go:128 | — | actors/coroutines only, not funcs/structs |
| Undefined var/function detection | NOT_IMPLEMENTED | borrow_checker.go:365-369 returns silently; only JIT jit/engine.go:433 | — | not compile-time |
| Borrow checker (lexical scope stack) | IMPLEMENTED | borrow_checker.go:30-45,66-94 | borrow_checker_test | push/pop scope, shadowing |
| Borrow expiry on scope exit | PARTIALLY_IMPLEMENTED | borrow_checker.go:74-85 | BorrowExpiresInScope | name-based restore; **moves never expire** |
| **BorrowError.Line** | **BUG** | borrow_checker.go always 0 | — | Line never set |
| FFI type validation | PARTIALLY_IMPLEMENTED | ffi.go:212-242,290-364 | ffi_test (not in pipeline) | extern "C" whitelist; NOT wired into run/build |
| Actor type validation | PARTIALLY_IMPLEMENTED | actor.go:316-331 | actor_test | LSP only |
| Coroutine validation | PARTIALLY_IMPLEMENTED | coroutine.go | coroutine_test | LSP only |
| Kernel analyzer | IMPLEMENTED | kernel_analyzer.go | kernel_analyzer_test | runs only in `check` cmd |
| Whole-program pre-codegen type pass | NOT_IMPLEMENTED | commands.go gates = parse+borrow only | — | |
| **TYPE SYSTEM / GENERICS** | | | | |
| Primitive types (string-named) | PARTIALLY_IMPLEMENTED | ffi.go:213, kernel_analyzer.go:49 | — | no type values |
| Option<T> / Result<T,E> | PARTIALLY_IMPLEMENTED | match/inferMatchType borrow_checker.go:490-511; BUG-2 | — | match arms compile to `1` (BUG-2) |
| Generics trait/impl registration | PARTIALLY_IMPLEMENTED | generics.go:44-73 | generics_test | method-name coverage only |
| Generics monomorphization | PARTIALLY_IMPLEMENTED | generics.go:125-316; only wired for GPU generic_kernel.go | generics_test | `InstantiateGenericFunc` misses body substitution (generics.go:216-220) |
| Generic type inference | NOT_IMPLEMENTED | — | — | typeArgs passed as map |
| Function return inference | NOT_IMPLEMENTED | — | — | |
| ADTs (enum/sum/tagged union) | PARTIALLY_IMPLEMENTED | parser enums; BUG-7 | — | payload variants emit undefined `_make` (BUG-7) |
| **CODECEN** | | | | |
| AST→SSA→C23 (default path) | IMPLEMENTED | codegen.go:121 (!DisableSSA), emit_ir.go:202-225, lower.go | ssa_test, codegen tests | default; fallback to legacy on SSA failure |
| SSA constant folding | IMPLEMENTED | ssa/opt.go | ssa_test | |
| SSA DCE | IMPLEMENTED | ssa/opt.go | ssa_test | |
| SSA optimize passes (deeper) | NOT_IMPLEMENTED | opt.go only fold+DCE | — | G2/G12 |
| C23 native backend | IMPLEMENTED | codegen.go, native.go | codegen tests | delegates to GCC/Clang/MSVC |
| GPU kernels (WGSL/OpenCL) | IMPLEMENTED (emitter) | wgsl.go, gpu.go | wgsl_test, gpu_test | emitter-level tests; no end-to-end hardware conformance (SPEC L466) |
| SPIR-V | IMPLEMENTED (emitter) | spirv.go | spirv_test | |
| Quantum (QASM/QIR/OpenPulse) | IMPLEMENTED (emitter) | qasm.go, qir.go, openpulse.go | quantum_test, openpulse_test | |
| Quantum opt/distributed/sim/QEC/QML/autodiff | IMPLEMENTED (lib) | quantum_opt.go, quantum_dist.go, quantum_sim.go, qec.go, qml_lowering.go, quantum_autodiff.go | respective tests | library-level; not in default path |
| Generic kernel monomorphization | IMPLEMENTED | generic_kernel.go | generic_kernel_test | not called by CLI run/build |
| Tensor→WGSL | IMPLEMENTED | tensor_wgsl.go | tensor_test | |
| **IR / JIT** | | | | |
| SSA IR (block-param CFG) | IMPLEMENTED | ir/ssa/ssa.go | ssa_test | 24 opcodes |
| Bytecode IR | IMPLEMENTED | ir/bytecode.go | bytecode_test | KRK\x01 |
| JIT VM execution | EXPERIMENTAL | jit/engine.go | engine_test (24 fns) | not default run path |
| FFI dynamic loading | EXPERIMENTAL | jit/ffi.go | JIT_FFIIntegration | |
| **RUNTIME** | | | | |
| C actor mailbox (MPMC atomic) | IMPLEMENTED | runtime/actor.c | — | |
| C quantum statevector | IMPLEMENTED | runtime/quantum.c | — | gates + measure |
| C RPC node | IMPLEMENTED | runtime/rpc.c | — | spawn/send/stop/ping |
| C reflect | IMPLEMENTED | runtime/reflect.c | — | TypeInfo/FieldInfo |
| Go actor system (MPSC, TCP mesh) | IMPLEMENTED | pkg/runtime/actor_system.go | actor_test | not wired into run/build (G6) |
| Go M:N coroutines + channels + select | IMPLEMENTED | pkg/runtime/coroutine.go | coroutine_test | not wired into run/build |
| **STANDARD LIBRARY** | | | | |
| std/io.kark | STUB | 1-line comment | — | empty |
| std/string.kark | STUB | 1-line comment | — | empty |
| stdlib/math/math.kark | IMPLEMENTED | libc + PI/E + trig/exp/log | — | real |
| stdlib/io/io.kark | IMPLEMENTED | libc file I/O | — | real |
| stdlib/async/actor.kark | IMPLEMENTED | Message, channels, actor mgmt | — | real |
| stdlib/gpu/gpu.kark | IMPLEMENTED | GPUDevice, buffer/dispatch | — | real |
| **CLI / TOOLING** | | | | |
| build/run/check/test | IMPLEMENTED | commands.go, main.go | cli tests | |
| transpile | IMPLEMENTED | BuildCommand | — | build path |
| KPM pkg manager | IMPLEMENTED | pkg/pm/* | pm tests (28) | Phase 55d |
| LSP server | PARTIALLY_IMPLEMENTED | pkg/lsp/* | lsp_test | hover/definition |
| Formatter (kfmt) | NOT_IMPLEMENTED | — | — | Phase 66 |
| Linter | NOT_IMPLEMENTED | — | — | |
| Docs generator | NOT_IMPLEMENTED | — | — | |
| Exit codes / help / version | IMPLEMENTED | main.go | — | |
| **TESTING** | | | | |
| Unit tests (lexer/parser/sema/codegen/...) | IMPLEMENTED | ~493 functions | — | sema 198, codegen 116 |
| E2E via bootstrap/codegen_profile | IMPLEMENTED | bootstrap_test, codegen_profile_test | — | OOM in codegen_profile |
| examples/ corpus | IMPLEMENTED | 25 .kark files | — | not directly Go-test-invoked |
| Parser dedicated unit test | NOT_IMPLEMENTED | no parser_test.go | — | gap → OOM shipped |
| Snapshot / golden tests | NOT_IMPLEMENTED | — | — | Phase 69 |
| Perf/regression gates | NOT_IMPLEMENTED | — | — | |
| **SPEC / DOCS** | | | | |
| SPEC.md v0.14.0 | IMPLEMENTED | SPEC.md | — | authoritative |
| ROADMAP / gap register | IMPLEMENTED | ROADMAP.md | — | G1-G15, BUG-1..8 |
| README architecture docs | IMPLEMENTED | README.md | — | |
| Codegen/impl-spec conformance | NOT_IMPLEMENTED | — | — | Phase 69 |
