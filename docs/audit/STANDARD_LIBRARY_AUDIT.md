# KARKAIN STANDARD LIBRARY AUDIT

Classify every component. Do not artificially enlarge the stdlib.

## Karkain-source stdlib
| Module | File | Status | Content |
|--------|------|--------|---------|
| std/io | std/io.kark | STUB | 1-line comment, empty |
| std/string | std/string.kark | STUB | 1-line comment, empty |
| stdlib/math | stdlib/math/math.kark | IMPLEMENTED | libc imports, PI/E, trig/exp/log/floor/ceil |
| stdlib/io | stdlib/io/io.kark | IMPLEMENTED | libc file I/O wrappers |
| stdlib/async | stdlib/async/actor.kark | IMPLEMENTED | Message struct, channel primitives, actor mgmt |
| stdlib/gpu | stdlib/gpu/gpu.kark | IMPLEMENTED | GPUDevice, buffer/dispatch abstractions |

## Go-side stdlib (pkg/stdlib)
| Component | Status | Evidence |
|-----------|--------|----------|
| reflect.go | STUB | "In a real implementation..." placeholder |
| http.go | STUB | HttpGet/HttpPost placeholders (ROADMAP D1: HTTP stub in every binary) |
| actor.go | STUB → reused | Send/Receive placeholders; real actor system is pkg/runtime |
| rpc.go | PARTIAL | RPC JS/wire integration |

## Classification by domain (target)
| Domain | Implemented | Partial | Stub | Missing |
|--------|-------------|---------|------|---------|
| Core (primitives, memory) | partial (SSA/alloc) | | | substantial |
| Memory | alloc/free/addr | | | arenas, defer/RAII |
| Collections | array/string ops in runtime | | | map/list/vec as stdlib |
| String | | | std/string empty | rich string API |
| IO | stdlib/io real | | std/io empty | full file/stream API |
| Filesystem | | | | |
| Networking | | | | |
| Concurrency | async/actor real (standalone) | | | wiring |
| Time | | | | |
| Math | math.kark real | | | |
| Encoding | | | | |
| Testing | karkain test cmd | | | framework lib |
| Reflection | pkg/stdlib reflect stub; C reflect.c real | | | |

## Decision
Build the standard library AFTER the core compiler foundation (type/sema/error-handling) is
sound (Phase 60). Focus V1 stdlib on: collections, string, io/filesystem, math, time, testing.
Defer networking/encoding/reflection-or-full to after V1. Remove the HTTP stub from every binary
(ROADMAP D1) — it inflates the runtime with dead code.
