# KARKAIN CONCURRENCY AUDIT

Evidence-based audit of concurrency, plus a phased recommendation per the prompt's preferred
execution order. Do NOT implement multiple concurrency paradigms simultaneously.

## Actual concurrency facilities (evidence)
| Facility | Status | Evidence |
|----------|--------|----------|
| Atomic operations / memory ordering | NOT_IMPLEMENTED | ROADMAP G7; no atomics/seq_cst |
| Raw threads | NOT directly exposed | Go runtime underneath |
| Actor model (send/receive/spawn) | IMPLEMENTED (keyword + C/Go runtime) | lexer tokens actor/spawn/receive/channel/send; runtime/actor.c (MPMC atomic queue); pkg/runtime/actor_system.go (MPSC mailboxes, TCP mesh); stdlib/async/actor.kark |
| Coroutines / M:N scheduler | IMPLEMENTED (standalone) | pkg/runtime/coroutine.go (state-machine coroutines, green-thread scheduler, typed channels, select); coroutine_test.go |
| Async / await / yield | IMPLEMENTED (keyword/parser) | lexer/SPEC async, await, yield |
| Channels + select | IMPLEMENTED (standalone) | pkg/runtime/coroutine.go |
| Parallel loops / structured concurrency | NOT_IMPLEMENTED | — |
| Timeouts / cancellation | PARTIAL | receive/select patterns, not framework-level |

## Wiring status (important)
Concurrency runtime (actor_system.go, coroutine.go) exists and is tested in isolation, but is
**NOT wired into the default run/build pipeline** (ROADMAP G6: "Concurrency below goroutine-class";
the audit noted these are not instantiated in `commands.go`). It is a capability-ready but
non-default subsystem.

## Recommended phased architecture (per prompt, evaluate in order — one model at a time)
### Phase 1 — Threads, Synchronization, Atomics
- Add atomic ops + memory orderings (seq_cst/acq_rel/relaxed) to IR + C lowering (Phase 70).
- Direct OS thread mapping for explicit parallelism (borrow checker extended for thread-safe
  sharing: Send/Sync-like ownership rules).
### Phase 2 — Tasks, Async/await, Structured Concurrency
- Built on atoms; task scheduler; structured concurrency (parent waits for children);
  cancellation + timeouts as first-class.
### Phase 3 — Channels, Actors, Advanced Parallelism
- Wire the EXISTING actor + coroutine runtime into the default pipeline once threads+atomics are
  sound. Channels/select then become load-bearing, not standalone.

## Decision
Phase the concurrency axis as Threads/Atomics (1) → Async/Tasks/Structured (2) → Actors/Channels
(3). The actor/coroutine code already built is a strong Phase-3 foundation; do NOT try to make it
the default until ownership (memory safety) and atomics (Phase 70) are in place, else data races
are unenforceable. This respects ROADMAP ordering (atomics with/after Phase 57).
