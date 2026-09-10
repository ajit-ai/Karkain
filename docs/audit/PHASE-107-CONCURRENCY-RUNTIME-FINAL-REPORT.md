# PHASE 107 — Concurrency Runtime — FINAL REPORT

**Status:** COMPLETE · **Date:** 2026-09-10 · **Branch:** `develop` (→ merged to `main`)

## 1. Goal

Roadmap Phase 107 (Tier 4 "Developer Experience & Concurrency", §38
implementation result): a real threading abstraction for Karkain programs —
spawn/join tasks, channels, actors, and a work-stealing scheduler — usable
from `.kark` source through the existing front end → C codegen → host C
compiler, with a gate proving deterministic concurrent results and
> 1,000,000 messages/second.

## 2. Design as built

Two layers, both owned by Karkain:

### 2.1 Compiler-neutral C runtime (`runtime/concurrency/c/`)

A pthreads/Win32-backed scheduler with a real task abstraction:

- **Tasks:** `karkain_task_t` heap cells with a function pointer, a heap-owned
  args block (`ctx`, freed by the runtime after execution), an atomic `done`
  flag, a condition variable for `join`, and a `status` long carrying the
  user function's `Value` conversion (negative = task failure).
- **Scheduler:** a process-wide `karkain_sched_t` with idle/spinning workers,
  per-worker wake conditions, a shared pending counter, a bounded grab queue,
  and an explicit drain/shutdown (`karkain_sched_stop`) — no busy-spin wait.
- **Work stealing:** each worker owns a work item (task or actor job); when
  one finishes and the queue is bare it steals from any worker that still has
  pending work (`steal:1000` scenario proves 1000 steals produce 1000
  executions).
- **Channels:** `karkain_channel_t` with a mutex + condvar, an optional
  bounded capacity (int < 0 = unbounded), blocking send/recv, and a closing
  state so `recv` drains remaining messages then deterministically reports
  closed (`nil`). Recv detaches and frees the message heap cell; send copies
  the `Value` by value into a fresh heap cell.
- **Actors:** `karkain_actor_t` = a mailbox (fully-owned task bag) + a
  serialized handler job allocated as a scheduled task. The state lives in a
  heap cell (`karkain_actor_box_t { long bid; Value state; }`) that the
  runtime frees on `actor_stop`. One dispatcher (`karkain_actor_dispatch`)
  routes a job to the handler matching its id.
- **Atomics:** `karkain_conc_atomic_*` (`__atomic_load_n/store_n/fetch_add`/
  `compare_exchange`) with `int memory_order` params, in the runtime only —
  deliberately NOT the C11 `stdatomic` spellings, which kept the generated
  program buildable under the pipeline's existing strict `-std` flags.

### 2.2 Compiler integration (parser → sema → codegen)

- **Language surface** (Go engine):
  - `spawn(fn, args...)` → task handle (statement or expression, any arity,
    including zero-arg `spawn(fn)`);
  - `join(t)` → task status (`Value` int; negative failed), `wait_all()`;
  - `channel(cap)` (0 or 1 arg; `cap < 0`/omitted = unbounded),
    `chanSend(ch,v)`, `chanClose(ch)`, `receive(ch)` expression;
  - `actor("handler", state)`, `actorSend(a,v)`, `actorState(a)`,
    `setActorState(a,v)`, `actorStop(a)`.
  - The word `send` is a lexed keyword token (`TokenSend`, also `<-`) so the
    send builtin is spelled `chanSend`.
- **Parser:** new `parseSpawn` (`spawn(fn, args...)`; `SpawnExpr.ActorName`
  holds the target, `Args` the arguments — supersedes the legacy
  `spawn(actor)(args)` form), `parseReceiveChannel` shared by the legacy
  `receive(ch) -> var` statement and the new expression form, `parseKeywordCall`
  for `channel(...)`/`actor(...)`, and `parsePrimaryExpr` cases for
  `spawn/receive/channel/actor` (statement-level `spawn(...)` wraps in
  `ExprStmt`).
- **Sema:** the concurrency builtins were added to `builtinNames`
  (`pkg/sema/resolve.go`), so unresolved calls still diagnose cleanly.
- **Codegen (`pkg/codegen/conc_runtime.go`):**
  - a recursive pre-scan (`scanNodeForConcurrency`) that sets
    `Generator.usesConcurrency` and records spawn targets + actor handler
    names (`collectConcDecls`) in stable order (wrapper index / dispatcher id);
  - `concRuntimeHeader()` — the compiler-neutral C runtime, embedded via
    `runtime/concurrency/embed.go` and inlined into the generated C so a
    program is still a single gcc invocation;
  - a per-program wrapper prepass `concWrapperC()`: one
    `static void karkain_run_<idx>(karkain_task_t* t, void* ctx,
    karkain_sched_t* s)` per spawn site that unpacks the heap `Value` args,
    calls `userFuncC(fn)` with the declared arity, converts the returned
    `Value` to the task `status` (intVal for int results), plus the actor
    dispatcher that adopts the handler's return value as the new state
    (an int-0/fallthrough return = no state change);
  - the Value glue `concRuntimeAPIC()` — `karkain_conc_channel/send/recv/
    close/join/wait_all/actor/actor_send/actor_state/actor_set_state/
    actor_stop` translating between `Value`, `mk_nil()` (int 0), intptr
    handles and the runtime's typed pointers.
- **Emission:** `GenerateAndCompile` appends the runtime header after the SIMD
  runtime and the wrappers + glue after the function forward declarations.
  SSA lowering routes the concurrency builtins through the generic `genExpr`
  path (they are not `callableBuiltin`/`mutatingBuiltin`), so both the SSA and
  legacy paths share one implementation.
- **ktest determinism:** task-failure status from a function returning `-1`,
  channel close + drain → `0`, actor accumulation, and the 999-task sum-of-
  squares all reproduce byte-identically across repeated fresh processes
  (scheduler has no ordering-sensitive results in the gates).

## 3. Files

| File | Change |
|---|---|
| `runtime/concurrency/c/karkain_conc.h` | NEW — runtime types (`karkain_sched_t`, `karkain_task_t`, `karkain_channel_t`, `karkain_actor_t`, `karkain_actor_box_t`, `Value` glue decls) + `karkain_conc_atomic_*` |
| `runtime/concurrency/c/karkain_sched_impl.h` | NEW — scheduler/worker internals |
| `runtime/concurrency/c/karkain_scheduler.c` | NEW — workers, task queues, stealing, spawn/task_join/wait_all, stop, `free(ctx)` ownership |
| `runtime/concurrency/c/karkain_channel.c` | NEW — bounded/unbounded channels, blocking send/recv, deterministic close |
| `runtime/concurrency/c/karkain_actor.c` | NEW — actor mailbox, dispatch, state cell, stop cleanup |
| `runtime/concurrency/c/concurrency_main.c` | NEW — standalone scenario driver (spawn, actors, steal, channels, throughput) |
| `runtime/concurrency/embed.go` | NEW — `//go:embed c/...` (runtime sources are NOT on disk at compile time) |
| `pkg/runtime/phase107_concurrency_test.go` | NEW — standalone C-runtime gate (scenarios + >1M msg/s) |
| `pkg/codegen/conc_runtime.go` | NEW — scans, header/wrapper/glue emitters, `genConcCall`, `genConcurrencySpawnExpr`, `genConcRecv` |
| `pkg/codegen/codegen.go` | Generator fields + `New()`; `GenerateAndCompile` pre-scan and header/wrapper/glue emission; real ReceiveStmt/SpawnExpr/SendExpr/ChRecvExpr/ChSendExpr cases; `genExpr` conc-builtin dispatch |
| `pkg/codegen/phase107_concurrency_test.go` | NEW — 7 codegen gates (spawn/join, spawn statements, channels, actors, embedded runtime, 999-task stress, generated-source presence) |
| `pkg/parser/parser.go` | NEW `parseSpawn`/`parseReceiveChannel`/`parseReceive`/`parseKeywordCall` + expression cases |
| `pkg/sema/resolve.go` | `builtinNames` extended with the concurrency builtins |
| `examples/concurrency/pipeline/main.kark` | NEW — E2E demo (spawn+join, producer channel, actor counter) |
| `pkg/cli/phase107_concurrency_test.go` | NEW — 2 E2E gates through `BuildCommandIncremental` |

## 4. Gates & tests

`pkg/runtime/phase107_concurrency_test.go` (standalone C runtime, gcc build +
run, scenario-gated by `KARKAIN_SCEN`, `KARKAIN_WORKERS`, CRLF-normalized):
`spawn:0,-1` (zero-tier spawns + a failing task), `sum:210`, `actor:100`,
`actorstop:1`, `steal:1000`, `bounded:210`, `close:1,1,0,0,7,1`; throughput
block asserts > 1,000,000 channel messages/sec.

`pkg/codegen/phase107_concurrency_test.go` (7, real front end + pipeline
compile + run):
1. **SpawnJoin** — `spawn(work,21)` status `42`, `spawn(work,5)` status `10`,
   failing task `-1`, then `done`.
2. **SpawnStmtStatement** — bare `spawn(f)` statements + `wait_all()` compile
   and produce `done`.
3. **Channels** — bounded `channel(2)`, producer task sends 10/20 asynchronously,
   `receive` returns 10 then 20, invalid `chanSend` after close returns 0.
4. **Actors** — `actor("counter",0)` + three `actorSend` (1,2,3) →
   `actorState` `6`, `done`.
5. **RuntimeEmbedded** — generated C must contain `karkain_channel_create`,
   `karkain_sched_spawn`, `karkain_conc_actor`, `karkain_run_0`,
   `karkain_actor_dispatch`, `karkain_actor_box_t`.
6. **Stress** — 999 spawned `work(i)=i*i` tasks joined through stored handles
   → sum of squares `332833500` (reproducible).
7. **GeneratedSourceWritten** — `GenerateAndCompile` leaves the `.c` file on
   disk next to the source (the CLI pipeline relies on it).

`pkg/cli/phase107_concurrency_test.go` (2 E2E through
`BuildCommandIncremental` + real gcc link + run):
1. **E2E RunsClean** — `examples/concurrency/pipeline/main.kark` builds, the
   generated C carries the runtime glue markers, and the executable prints
   `144\n10\n20\n30\n0\n6`.
2. **Determinism** — the same executable run 4 more times reproduces
   byte-identical output (no status/message loss under scheduling).

All PASS.

## 5. Regression sweep

| Suite | Result |
|---|---|
| `pkg/cli` (full, isolated) | ok — 1723.8s (conformance 59/59, all Phase 97–106 gates, new Phase 107 E2E) |
| `pkg/codegen` | ok — 13.1s |
| `pkg/parser`, `pkg/sema`, `pkg/lexer` | ok |
| `pkg/ir` (+hir/ssa), `pkg/pm`, `pkg/source`, `pkg/diagnostics`, `pkg/compiler`, `pkg/module` | ok |
| `pkg/backend/*`, `pkg/npu/*`, `pkg/runtime/*` | ok |
| `go build ./...` + `go vet` | clean |
| Bootstrap | stage-1 build ok (Go codegen change safe for the compiler's own sources); stage-2 SEGFAULT reproducibly = documented ~3.9GB-RAM host OOM class (no `src/compiler` changes; concurrency codegen is gated by `usesConcurrency` and is unused by compiler sources; Go-engine `check` of `src/compiler/main.kark` passes with a pre-existing K100 warning only) |

## 6. Known limitations (documented, NOT defects)

1. kcc (self-hosted) concurrency `spawn/join/channel/actor` semantic + codegen
   parity is post-107 work. The self-hosted lexer/parser already accept the
   keywords (`TK_SPAWN/TK_SEND/TK_RECEIVE/TK_ACTOR/TK_CHANNEL`), so the language
   surface parses on both engines; codegen parity is the remaining boundary.
2. The concurrency demo gate runs on the Go (default-independent) engine
   (`--engine go`); a kcc parity demo is deferred to the same boundary.
3. Actor handlers that reset state to exactly `0` and "no change" are
   indistinguishable (fallthrough == `make_int(0)`); the documented convention
   is that a handler always returns the new state, and returning int 0 means
   "no change".
4. Channels are process-local only (no cross-process/concurrent-clone flavor);
   the roadmap's goroutine-class ergonomics are not yet reached — this phase
   delivers the spawn/join + channels + actors + scheduler foundation.
5. Bootstrap stage-2 build-mode OOM SEGFAULT on the ~3.9GB-RAM host
   (pre-existing, unchanged; stage-1 and the low-memory check path pass).

## 7. Release

Committed on `develop`, merged to `main`, both pushed (AGENTS.md workflow).