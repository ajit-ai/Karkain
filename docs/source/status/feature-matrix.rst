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
