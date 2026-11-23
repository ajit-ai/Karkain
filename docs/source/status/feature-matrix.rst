Generics v1 — Feature Matrix
============================

Authoritative per-feature ledger for the Phase 146 generics track
(146A Go engine, 146B stdlib, 146C kcc parity, 146D close-out). Statuses
mirror :doc:`scope`; every row names the gate that proves it. No row may
claim more than its gate demonstrates.

.. list-table:: Generics v1 matrix
   :widths: 28 12 12 48
   :header-rows: 1

   * - Feature
     - Status
     - Engines
     - Gate
   * - Generic functions (explicit ``f[int]``)
     - Stable
     - Go + kcc
     - ``TestPhase146_GenericFuncGoldenGo``,
       ``TestPhase146C_GenericFuncGoldenKCC``,
       ``TestPhase146D_Closeout/FuncStructGoldensBoth``
   * - Generic structs (``Point[int]{...}``)
     - Stable
     - Go + kcc
     - ``TestPhase146_GenericStructGoldenGo``,
       ``TestPhase146C_GenericStructGoldenKCC``
   * - Nested generics (fixpoint instantiation)
     - Stable
     - Go + kcc
     - ``TestPhase146C_NestedGenericKCC``
   * - K115 misuse diagnostics (bare/arity)
     - Stable
     - Go + kcc
     - ``TestPhase146_ArityMismatchRejected``,
       ``TestPhase146_BareGenericRejected``,
       ``TestPhase146C_BareGenericRejectedKCC``,
       ``TestPhase146C_ArityMismatchRejectedKCC``;
       ``karkain explain K115``
   * - Index-then-call demotion (Phase 133 preserved)
     - Stable
     - Go + kcc
     - ``TestPhase146_IndexCallDemotion``,
       ``TestPhase146C_IndexCallDemotionKCC``,
       ``TestPhase146D_Closeout/DemotionIdempotence``
   * - ``std.generics`` (``Stack[T]``/``Queue[T]``)
     - Stable
     - Go + kcc
     - ``TestPhase146B_GoldensGoEngine``,
       ``TestPhase146B_KccParity``,
       ``TestPhase146C_StdlibGenericsKCC``,
       ``TestPhase146D_Closeout/StdlibParity``
   * - Unknown-generic rejection (never silent)
     - Stable
     - kcc
     - ``TestPhase146C_UnknownGenericCallRejectedKCC``
   * - Compiler self-check through generics
     - Stable
     - kcc
     - ``TestPhase146C_CheckCleanKCC``,
       ``TestPhase146D_Closeout/SelfCheck``
   * - ``explain K115``
     - Stable
     - CLI
     - ``TestPhase146_ExplainK115``
   * - WASM generics lowering
     - Planned
     - —
     - ``TestPhase146_WasmBoundaryStays`` (K108 rejection pinned)
   * - Inference, generic methods, const generics, checked constraints,
       traits dispatch, associated types
     - Planned
     - —
     - 146 non-goals (see ``docs/audit/PHASE-146-BASELINE.md`` §6);
       no gate exists by design

Close-out consolidation: ``TestPhase146D_Closeout`` asserts every Stable
row above in a single invocation.

Concurrency task control (P0.2)
===============================

Task cancellation and timed joins, verified through the real pipeline on
both engines by ``pkg/cli/phase_p02_task_cancel_test.go``. Every row names
the subtest that proves it; nothing here claims preemption, and
``spawn``/``join``/``channel``/``actor`` signatures are unchanged.

.. list-table:: Task control matrix
   :widths: 30 12 12 46
   :header-rows: 1

   * - Feature
     - Status
     - Engines
     - Gate
   * - ``cancel(task)`` - first request returns ``true``; a queued task never
       runs and joins as ``-4``
     - Stable
     - Go + kcc
     - ``TestPhaseP02_D_QueuedCancellation``
   * - ``cancel(task)`` - a running task is cooperative, not preempted; the
       running body's result stands
     - Stable
     - Go + kcc
     - ``TestPhaseP02_E_RunningCancellationCooperative``
   * - ``cancel(task)`` - repeated/completed/non-task handles return
       ``false`` and are safe
     - Stable
     - Go + kcc
     - ``TestPhaseP02_A_NormalCompletion``,
       ``TestPhaseP02_F_RepeatedCancel``
   * - ``join_timeout(task, ms)`` - ``true`` on completion, ``false`` on
       timeout; never cancels; the task stays joinable
     - Stable
     - Go + kcc
     - ``TestPhaseP02_B_TimeoutDoesNotCancel``
   * - ``join_timeout(task, 0)`` - non-blocking completion check
     - Stable
     - Go + kcc
     - ``TestPhaseP02_C_ZeroTimeoutCompleted``
   * - Invalid handles and negative timeouts rejected safely
     - Stable
     - Go + kcc
     - ``TestPhaseP02_G_InvalidHandlesAndArgs``
   * - Existing concurrency semantics preserved (spawn/join grid ``285``;
       channel producer/consumer ``5, 0, 8``)
     - Stable
     - Go + kcc
     - ``TestPhaseP02_H_ExistingConcurrencyGoldens``
   * - Actor regression (valid handler ``6``; invalid handler rejected)
     - Stable
     - Go + kcc
     - ``TestPhaseP02_I_ActorRegression``
   * - Preemptive interruption of a running task
     - Not supported
     - -
     - Not a goal of P0.2: no scheduler redesign and no signal-based
       interruption. Cancelling a running task sets a request the program
       may observe; it never forces a stop.

Native ``sha256_hex`` (v1.2.6)
==============================

The direct runtime builtin ``sha256_hex`` is lowered by the self-hosted
native driver. The contract is the digest value: 64 lowercase hex characters,
with the C runtime's ``strlen`` truncation at the first NUL. Byte-identity
with the Go native helper is not claimed; the gate compares executed stdout.

.. list-table:: Native sha256 matrix
   :widths: 30 12 12 46
   :header-rows: 1

   * - Feature
     - Status
     - Engines
     - Gate
   * - ``sha256_hex(string)`` on ``native-x86_64-windows``
     - Stable
     - kcc native
     - ``TestPhaseV126_ExecutedPEWithExactBytes``
   * - C-backend and Go-native behavioral parity
     - Stable
     - kcc native + Go native + C
     - ``TestPhaseV126_MatchesCBackend``,
       ``TestPhaseV126_MatchesGoNativeBackend``
   * - FIPS-derived K/IV constants present in the emitted image
     - Stable
     - kcc native
     - ``TestPhaseV126_KAndIVConstantsAreInTheImage``
   * - Arity/type/return/nested-codec refusals
     - Stable
     - kcc native
     - ``TestPhaseV126_RefusalsAreByName``
   * - ``import std.crypto`` through the native target
     - Not supported
     - -
     - The qualified native subset has no stdlib runtime; the direct builtin
       is supported, the module wrapper is not yet claimed.