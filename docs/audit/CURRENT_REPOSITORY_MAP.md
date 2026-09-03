# KARKAIN CURRENT REPOSITORY MAP

Evidence-based audit of the repository structure as of this audit. All statuses derive
from actual source/tests inspected, not folder names.

Status legend: IMPLEMENTED = real, working code with tests. PARTIAL = real but incomplete.
EXPERIMENTAL = built but not wired into the main path. STUB = placeholder only. NOT_IMPLEMENTED = absent.

| Directory | Purpose | Implementation status | Key source files | Maturity | Notes |
|-----------|---------|----------------------|-------------------|----------|-------|
| `cmd/karkain/` | CLI entry point / toolchain dispatcher | IMPLEMENTED | `main.go` (835 lines) | Functional | Single binary; dispatch to build/run/check/test/lsp/pkg. `pkg` subcommands integrated (init/add/fetch/audit/cache/workspace...). |
| `pkg/cli/` | Build/run/check/test command orchestration | IMPLEMENTED | `commands.go` | Functional | Lexer->Parser->BorrowCheck->Macro->Codegen pipeline. `ValidateKarFile`, TestCommand. |
| `pkg/lexer/` | Tokenizer | IMPLEMENTED | `lexer.go` (566) | Functional | ~100 token kinds; zero-copy offset tokens; ASCII-only (no unicode); line comments only (matches SPEC). |
| `pkg/parser/` | PRECEDENCE-CLIMBING parser + AST + arena | PARTIAL (see OOM) | `parser.go` (1748), `ast.go` (901), `arena.go`, `macro.go`, `captures.go`, `escape.go` | Feature-rich, DISABLED-generics stub, OOM risk | 52 parse methods; error recovery collects multiple errors; **generics `<T>` syntax NOT parsed (stub)**; **function-call arg loops have no no-progress guard → OOM on large input**. |
| `pkg/sema/` | Domain-specific semantic validators | PARTIAL | `borrow_checker.go`, `generics.go`, `ffi.go`, `actor.go`, `coroutine.go`, `kernel_analyzer.go`, `autodiff.go`, `quantum*.go` | No whole-program type checker | No general type checker / symbol table / scope resolution. Borrow checker lexical stack works but moves never expire; `BorrowError.Line` always 0. Most checkers (FFI/Actor/Coroutine/Quantum) NOT wired into run/build pipeline. |
| `pkg/codegen/` | C23 native + GPU + quantum emitters | PARTIAL | `codegen.go`, `lower.go`, `emit_ir.go`, `native.go`, `wgsl.go`, `gpu.go`, `spirv.go`, `qasm.go`, `qir.go`, `openpulse.go`, `qec.go`, `quantum_opt.go`, `generic_kernel.go` | SSA default; many backends | Emits C23 and delegates to GCC/Clang/MSVC. SSA path is default (`!DisableSSA`), FoldConstants+DCE, Verify, fallback to legacy. GPU/WGSL/SPIR-V/QASM/QIR emitters exist with tests but are **not conformance-tested end-to-end against hardware**. |
| `pkg/ir/` | Bytecode IR + SSA IR | PARTIAL | `bytecode.go`, `ssa/ssa.go`, `ssa/opt.go`, `ssa/verify.go` | Functional SSA core | SSA: block-param CFG, 24 opcodes, typed regs. Optimizer = constant folding + DCE only (shallow, G2). Bytecode: KRK\x01 format, unified CPU/GPU/Quantum/Tensor opcodes. |
| `pkg/jit/` | Stack-based bytecode VM execution | EXPERIMENTAL | `engine.go`, `ffi.go` | Functional as VM, not wired to CLI | Executes .kbc modules. FFI via dlopen/LoadLibrary. Tests pass. Not the default run path. |
| `pkg/runtime/` | Go actor + coroutine runtime | PARTIAL | `actor_system.go`, `coroutine.go`, `gpu/` | Real but not wired into main pipeline | MPSC mailboxes, ActorRef, TCP mesh, M:N coroutines, typed channels, select. Concurrency not yet integrated into run/build (G6). |
| `pkg/pm/` | KPM package manager | IMPLEMENTED | `manager.go`, `workspace.go`, `semver.go`, `lock.go`, `integrity.go`, `auth.go`, `audit.go`, `cache.go`, `registry.go` | Mature (Phase 55d) | semver, lockfile, SHA-256 integrity, registry stub, auth, cache, workspace, audit --licenses. |
| `pkg/lsp/` | Language server | PARTIAL | `server.go`, `protocol.go`, `handler.go` | Early | hover, definition, textDocument sync. Formatter/REPL not present. |
| `pkg/diagnostics/` | Error reporting | IMPLEMENTED | `reporter.go`, `sourcemap.go` | Functional | Source snippets, line/col, severity. `#line` for GDB lives in codegen, not here. |
| `pkg/bootstrap/` | Self-hosting bootstrap pipeline | PARTIAL | `bootstrap.go`, `bootstrap_test.go` | Stage0 works; Stage1 OOM-blocked | Stage0 validates/writes compiler sources + stage0.c. Stage1 transpile of `src/compiler/main.kark` fails (parser OOM). |
| `pkg/stdlib/` | Standard library (Go-side) | STUB | `reflect.go`, `http.go`, `actor.go`, `rpc.go` | STUB | Several are placeholders ("In a real implementation..."). |
| `runtime/` | C runtime runtime layer | PARTIAL | `actor.c`, `quantum.c`, `reflect.c`, `rpc.c` | Real C | Actor mailbox (MPMC atomic queue), quantum statevector sim, RPC TCP, reflect. Small (~24KB total). |
| `src/compiler/` | Self-hosted compiler sources (Karkain) | PARTIAL / INCOMPLETE | `main.kark`, `lexer.kark`, `parser.kark`, `sema.kark`, `codegen.kark`, `ast.kark`, `runtime.c` | In-progress (Phase 56) | Real but incomplete self-hosted implementation. Transpile of `main.kark` parser-internally OOMs. |
| `compiler/` | Older C self-host bootstrap | PARTIAL | `main.c`, `ast.c`, `codegen.c`, `lexer.kark`, `parser.kark` | Legacy | C bootstrap + partial .kark ports. |
| `src/` | Additional self-host lexer/parser/analyzer/codegen | PARTIAL | `main.kark`, `lexer.kark`, `parser.kark`, `analyzer.kark`, `codegen.kark` | In-progress | |
| `std/` | Standard library (Karkain) | STUB | `io.kark`, `string.kark` | STUB | Each is a 1-line comment placeholder. |
| `stdlib/` | Standard library modules (Karkain) | PARTIAL | `math/math.kark`, `io/io.kark`, `async/actor.kark`, `gpu/gpu.kark` | Real subset | math/io/actor/gpu have real implementations. |
| `examples/` | E2E demo/test corpus | IMPLEMENTED | 25 `.kark` files | Functional demos | phase1-17, struct, map, quantum, closure, strings_slices, self_host_*, stdlib_test etc. NOT directly invoked by Go tests. |
| `scripts/` | Build/release automation | IMPLEMENTED | `bootstrap.ps1/.sh`, `build.ps1/.sh`, `package.ps1`, `release.ps1` | Functional | Cross-platform. |
| `.github/workflows/` | CI | IMPLEMENTED | `ci.yml` | Functional | Build + test matrix. |
| `editors/vscode/` | Editor integration | PARTIAL | `package.json`, `syntaxes/karkain.tmLanguage.json` | Early | Syntax highlighting, `.kark` ext, debug command. |
| `ROADMAP.md` | Development plan | IMPLEMENTED | — | Living | Phases 50-70; gap register G1-G15; bugs BUG-1..8; execution order. |
| `SPEC.md` | Language specification | IMPLEMENTED | v0.14.0 | Authoritative | Single source of truth for syntax/semantics. |
| `README.md` | Docs | IMPLEMENTED | — | Good | Architecture diagrams, CLI reference, feature matrix. |
| `AGENTS.md` | Dev conventions | IMPLEMENTED | — | Operating rule | Branch workflow (develop→main→push), test command. |

## Top-level counts
- Go files: 119; test files: 48; total test functions ~493.
- Karkain `.kark` source: 47 files (compiler/, examples/ 25, src/, std/, stdlib/).
- C runtime: `runtime/*.c` (actor, quantum, reflect, rpc) + `src/compiler/runtime.c`.
- Single binary `karkain.exe` builds from `cmd/karkain` + `pkg/*`.
