# KARKAIN FEATURE PRIORITY MATRIX

Priorities P0-P6 per the prompt. Each item scored against current status (evidence-based).

## P0 — COMPILER FOUNDATION (language correctness)
| Feature | Status | Note |
|---------|--------|------|
| Lexer | IMPLEMENTED (mature) | complete; unicode deferred |
| Parser (correctness) | **BROKEN → OOM** | must fix no-progress loops first |
| Parser generics `<T>` | STUB | parse + AST |
| AST | IMPLEMENTED (arena) | tighten Node interface |
| Semantic analysis | **MISSING type checker + symbol table** | critical |
| Type system | PARTIAL (strings) | build real Type value |
| Diagnostics | IMPLEMENTED | add col/snippets to parse errors |
| Code generation | IMPLEMENTED (SSA→C23) | fix BUG-1/2/3/7/8 |
| Tests | ~493 | add parser unit tests |

## P1 — LANGUAGE COMPLETENESS (V1 must-have)
| Feature | Status |
|---------|--------|
| Structs | IMPLEMENTED (codegen buggy) |
| Enums / ADT | PARTIAL (BUG-7) |
| Functions | IMPLEMENTED |
| Generics | STUB/PARTIAL → complete monomorphization |
| Modules + visibility (pub/private, G11) | MISSING |
| Pattern matching | PARTIAL (BUG-2) |
| Result<T,E> / Option<T> | PARTIAL (BUG-4) |

## P2 — SYSTEMS CAPABILITIES
| Feature | Status |
|---------|--------|
| Pointers / references / borrow | PARTIAL (BUG-5) |
| `unsafe` boundary | MISSING (needs explicit design) |
| Memory model | PARTIAL (ownership; see DECISION) |
| FFI / C interop | PARTIALLY_IMPLEMENTED |

## P3 — CONCURRENCY
| Feature | Status |
|---------|--------|
| Threads | not directly exposed |
| Async/Tasks/Structured | coroutine exists, not wired |
| Channels/Actors | exist, not wired |
| Atomics/memory ordering (G7) | MISSING |

## P4 — DEVELOPER ECOSYSTEM
| Feature | Status |
|---------|--------|
| Package manager (KPM) | IMPLEMENTED (mature) |
| Formatter (kfmt) | MISSING |
| Linter | MISSING |
| Documentation generator | MISSING |
| LSP | PARTIAL |
| IDE support | PARTIAL (highlighting) |

## P5 — HIGH PERFORMANCE
| Feature | Status |
|---------|--------|
| SSA optimizer depth | fold+DCE only (G2/G12) |
| SIMD vector types (G7) | MISSING |
| Memory arenas | PARTIAL (parser arena; runtime arenas MISSING) |
| Profiling | COMPLETE (Phase 110: `karkain prof`, aggregation-based, text/json/folded, Go engine) |
| Parallelism | MISSING (needs atomics) |
| Optimization flags to GCC backend | MISSING (only -c -g) |

## P6 — ADVANCED RESEARCH
| Feature | Status |
|---------|--------|
| Unified execution model | FEASIBILITY (doc) — do not build yet |
| Capability-based security | FEASIBILITY (doc) — do not build yet |
| Effect system | FEASIBILITY (doc) — do not build yet |
| GPU execution | emitters exist, not conformance-tested |
| Distributed execution (quantum/lattice) | lib exists, not wired |
| AI extension | via packages only, NOT core |

## Execution rule (from P0..P6)
Advance strictly in priority order. Do NOT build P3/P5/P6 until P0 (correctness) and P1
(language completeness) are sound. This is the discipline the "small core first" principle demands.
