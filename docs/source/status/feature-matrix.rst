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

Native ``==``/``!=`` over strings (v1.2.7, NAT-STREQ)
======================================================

String equality in a condition is lowered by the self-hosted native driver.
It compares content (length first, then bytes), not pointers. The string staging
area is reserved program-wide, including inside ``if``/``while``/``for`` bodies,
so a loop's index slot is not clobbered by an equality in its body.

.. list-table:: Native string equality matrix
   :widths: 30 12 12 46
   :header-rows: 1

   * - Feature
     - Status
     - Engines
     - Gate
   * - ``==`` / ``!=`` over strings in a condition
     - Stable
     - kcc native
     - ``TestPhaseV127Streq_ExecutedPEWithExactBytes``
   * - String equality inside a loop body
     - Stable
     - kcc native
     - ``TestPhaseV127Streq_ExecutedPEWithExactBytes``
   * - ``<``, ``<=``, ``>``, ``>=`` over strings
     - Not supported
     - -
     - Refused by name (``string ordering ('<') is not supported``)

Native ``s[a:b]`` string slicing (v1.2.7, NAT-STRSLICE)
========================================================

The view form: the result is ``(s.ptr + a, b - a)`` with no allocation, and
bounds are ``0 <= a <= b <= len`` with a loud ``Int3`` on violation. Present
bounds are integer expressions, not only literals. An absent bound is an EMPTY
node and the emitter substitutes 0 / len(s).

.. list-table:: Native string slice matrix
   :widths: 30 12 12 46
   :header-rows: 1

   * - Feature
     - Status
     - Engines
     - Gate
   * - ``s[a:b]`` (both bounds present)
     - Stable
     - kcc native
     - ``TestPhaseV127Strslice_ExecutedPEWithExactBytes``
   * - ``s[a:]``, ``s[:b]``, ``s[:]``
     - Stable
     - kcc native
     - ``TestPhaseV127Strslice_ExecutedPEWithExactBytes``
   * - Bounds as integer expressions
     - Stable
     - kcc native
     - ``TestPhaseV127Strslice_ExecutedPEWithExactBytes``
   * - Out-of-bounds or negative index
     - Stable
     - kcc native
     - ``TestPhaseV127Strslice_RefusalsAreByName``
   * - Slice of an integer
     - Not supported
     - -
     - Refused by name (``a string slice target must be a string``)
   * - Chained slicing
     - Not supported
     - -
     - Refused by name (``chained string slicing is not lowered``)

C23 TLS/HTTPS (v1.2.17)
=======================

Transport TLS is available on the C23 path on both front ends. Native and
WASM targets have no socket syscalls (v1.2.10 decision) and refuse these
names; no native or byte-identity claim is made.

.. list-table:: TLS matrix
   :widths: 34 14 20 32
   :header-rows: 1

   * - Feature
     - Status
     - Engines
     - Gate
   * - ``https_request`` builtin
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1217_LoopbackTLSBothEngines``
   * - ``https_get`` / ``https_post``
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1217_LoopbackTLSBothEngines``
   * - ``verify=1`` chain + hostname check
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1217_VerifyRejectsSelfSigned``
   * - ``verify=0`` loopback self-signed
     - Stable (test only)
     - Go + kcc C23
     - ``TestPhaseV1217_LoopbackTLSBothEngines``
   * - ``tls_last_error`` / ``https_last_error``
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1217_VerifyRejectsSelfSigned``
   * - Native / WASM TLS
     - Not supported
     - -
     - v1.2.10 socket boundary (no socket syscalls)
   * - Runtime source mapping
     - Stable
     - Go + kcc C23
     - ``TestTLSKarkFresh``

Concurrent HTTP server (v1.2.18)
=================================

The C23 path serves each connection in a spawned named worker. Native and
WASM have no sockets/concurrency surface here and remain excluded.

.. list-table:: Concurrent HTTP server matrix
   :widths: 38 14 20 28
   :header-rows: 1

   * - Feature
     - Status
     - Engines
     - Gate
   * - ``http_serve_echo(host, port)``
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1218_ConcurrentLoopback``
   * - Per-connection ``spawn(http_echo_worker, conn)``
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1218_ConcurrentLoopback``
   * - Silent first client cannot block a second
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1218_ConcurrentLoopback``
   * - Concurrency runtime gated by import
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1218_RuntimeWiring``
   * - Native / WASM server
     - Not supported
     - -
     - v1.2.10 socket boundary, K108 concurrency boundary

OS metrics / process enumeration (v1.2.19)
=============================================

Real OS values are available on the C23 path on both engines. Native and WASM
have no process/metric syscalls and remain excluded.

.. list-table:: OS metrics matrix
   :widths: 40 14 18 28
   :header-rows: 1

   * - Feature
     - Status
     - Engines
     - Gate
   * - ``os_current_pid`` / ``os_cpu_total``
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1219_MetricsBothEngines``
   * - ``os_memory_total_mb`` / ``os_memory_available_mb``
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1219_MetricsBothEngines``
   * - ``os_uptime`` (monotonic)
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1219_MetricsBothEngines``
   * - ``os_processes`` (sorted, capped, contains self)
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1219_MetricsBothEngines``
   * - Metric-free generated C
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1219_RuntimeWiring``
   * - Native / WASM
     - Not supported
     - -
     - ``TestPhaseV1219_NativeRefused``, K108 boundary

JSON full DOM (v1.2.20)
===============================

The C23 path parses JSON into a real Value tree on both engines. Native and
WASM have no JSON builtins and remain excluded.

.. list-table:: JSON DOM matrix
   :widths: 40 14 18 28
   :header-rows: 1

   * - Feature
     - Status
     - Engines
     - Gate
   * - ``json_dom_parse`` (nested objects/arrays)
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1220_DOMBothEngines``
   * - ``json_dom_type``
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1220_DOMBothEngines``
   * - ``json_dom_get`` dotted/indexed path
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1220_DOMBothEngines``
   * - ``json_dom_serialize`` escaping
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1220_EscapesRoundTrip``
   * - ``json_dom_error`` deterministic diagnostics
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1220_DeterministicErrors``
   * - JSON-free generated C
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1220_RuntimeWiring``
   * - Native / WASM
     - Not supported
     - -
     - ``TestPhaseV1220_NativeRefused``, K108 boundary

UDP and WebSocket (v1.2.21)
====================================

Real UDP datagrams and RFC 6455 text frames are available on the C23 path on
both engines. Native and WASM have no sockets and remain excluded.

.. list-table:: UDP/WebSocket matrix
   :widths: 40 14 18 28
   :header-rows: 1

   * - Feature
     - Status
     - Engines
     - Gate
   * - ``udp_send`` / ``udp_bind`` / ``udp_recv``
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1221_UDPLoopback``
   * - ``udp_send_on`` bound-socket send
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1221_UDPLoopback``
   * - ``ws_connect`` handshake
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1221_WebSocketLoopback``
   * - ``ws_send`` / ``ws_recv`` text frames
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1221_WebSocketLoopback``
   * - Runtime gating
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1221_RuntimeWiring``
   * - Native / WASM
     - Not supported
     - -
     - ``TestPhaseV1221_NativeRefused``, K108 boundary

Real SQLite driver (v1.2.22)
======================================

A real SQLite 3.46.1 driver is available on the C23 path on both engines.
Native and WASM have no FFI/SQLite linkage and remain excluded.

.. list-table:: SQLite driver matrix
   :widths: 40 14 18 28
   :header-rows: 1

   * - Feature
     - Status
     - Engines
     - Gate
   * - ``db_sqlite_open`` / ``db_sqlite_exec``
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1222_InMemoryBothEngines``
   * - ``db_sqlite_query`` array rows
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1222_InMemoryBothEngines``
   * - Temp-file reopen
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1222_FileReopen``
   * - Deterministic ``sqlite:`` errors
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1222_DeterministicErrors``
   * - Runtime gating
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1222_RuntimeWiring``
   * - Native / WASM
     - Not supported
     - -
     - ``TestPhaseV1222_NativeRefused``, K108 boundary

C-style for scope (unnamed corrective, v1.2.x)
================================================

A C-style ``for`` ends a checker scope on both engines. kcc remains stricter
than Go inside a loop body (Go's ``checkStmt`` has no ForStmt case); that
difference is deliberate and is not claimed as parity.

.. list-table:: C-style for scope matrix
   :widths: 42 14 16 28
   :header-rows: 1

   * - Behaviour
     - Status
     - Engines
     - Gate
   * - Init binding invisible after the loop
     - Stable
     - Go + kcc
     - ``TestPhaseForScope_LeakedBindingsRejectedByBothEngines/a_letinitleak``,
       ``TestPhaseForScope_LeakedBindingsRejectedByBothEngines/f_forvar``
   * - Body ``let`` / bare assignment invisible after the loop
     - Stable
     - Go + kcc
     - ``TestPhaseForScope_LeakedBindingsRejectedByBothEngines/i_bodyleak``,
       ``TestPhaseForScope_LeakedBindingsRejectedByBothEngines/k_assignleak``
   * - Forward reference inside the body still accepted
     - Stable
     - Go + kcc
     - ``TestPhaseForScope_AcceptedProgramsKeepTheirMeaning/j_forward_check``
   * - ``for-in`` / ``while`` / ``Block`` scope unchanged
     - Stable
     - Go + kcc
     - ``TestPhaseForScope_AcceptedProgramsKeepTheirMeaning/p_forin_after_check``,
       ``TestPhaseForScope_AcceptedProgramsKeepTheirMeaning/q_while_after_check``
   * - Valid loop value semantics unchanged
     - Stable
     - Go + kcc
     - ``TestPhaseForScope_AcceptedProgramsKeepTheirMeaning/n_valid_all`` and
       four further exact-byte goldens

A1 — kcc owns native build/run (unnamed v1.2.x slice)
======================================================

The self-hosted compiler writes and executes a C-free native image directly.
The Go CLI hex path is retained for backward compatibility; A1 adds a direct
path rather than removing the old one.

.. list-table:: A1 native ownership matrix
   :widths: 42 14 16 28
   :header-rows: 1

   * - Capability
     - Status
     - Engines
     - Gate
   * - ``writeBytes(path, array)`` (raw bytes, NUL-safe)
     - Stable
     - Go + kcc
     - ``TestPhaseA1_KccBuildWritesNativeImageDirectly``
   * - ``kcc build --target native-* --output <path>`` writes the image
     - Stable
     - kcc
     - ``TestPhaseA1_KccBuildWritesNativeImageDirectly``
   * - ``kcc run --target native-*`` executes the emitted image
     - Stable
     - kcc (host OS must match)
     - ``TestPhaseA1_KccRunNativeImageDirectly``
   * - No gcc/go invoked in either native path
     - Stable
     - kcc
     - PATH-shim log assertion plus a non-empty legacy positive control in
       both ``TestPhaseA1_`` tests

v1.2.35 slice 1 — build profiles (unnamed follow-up)
=====================================================

``--profile`` selects the C23 optimization flag on both engines. The default
``dev`` profile preserves the historical ``-O0`` behavior. Native-as-default
and SSA pass selection are explicitly deferred to their own slices.

.. list-table:: Build profile matrix
   :widths: 42 14 16 28
   :header-rows: 1

   * - Capability
     - Status
     - Engines
     - Gate
   * - ``--profile dev`` -> ``-O0`` (default)
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1235_ProfileFlagsReachCompiler/dev``
   * - ``--profile release`` -> ``-O2``
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1235_ProfileFlagsReachCompiler/release``
   * - ``--profile size`` -> ``-Os``
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1235_ProfileFlagsReachCompiler/size``
   * - Unknown profile rejected, no executable written
     - Stable
     - CLI
     - ``TestPhaseV1235_UnknownProfileRejected``
   * - Native path as default for supported programs
     - Stable
     - kcc (host OS must match)
     - ``TestPhaseV1235B_NativeDefaultForSupportedProgram``,
       ``TestPhaseV1235B_C23DefaultForUnsupportedProgram``,
       ``TestPhaseV1235B_ExplicitTargetsAreUnchanged``
   * - SSA pass selection
     - Deferred
     - -
     - ``docs/audit/V1.2.35-NATIVE-DEFAULT-BASELINE.md`` §6

v1.2.34 — portable OS/process control (takeover)
=================================================

``std.env`` / ``std.filesystem`` / ``std.process`` wrappers are backed by
internal ``osctl_*`` builtins on the C23 path on both engines. Native and WASM
refuse them by name.

.. list-table:: OS/process control matrix
   :widths: 42 14 16 28
   :header-rows: 1

   * - Capability
     - Status
     - Engines
     - Gate
   * - Environment get/set/unset/args
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1234_OsctlBothEngines``
   * - Filesystem metadata (exists/is_dir/stat/mkdir/remove)
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1234_OsctlBothEngines``
   * - Process spawn/kill/wait
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1234_OsctlBothEngines``
   * - Runtime source freshness (generated kcc mirror)
     - Stable
     - kcc
     - ``TestOSCtlKarkFresh``
   * - Native / WASM
     - Not supported
     - -
     - ``TestPhaseV1234_NativeRefusedByBothEngines``, K145/K108 boundary
   * - Signal ignore/default/raise (SIGINT 2, SIGTERM 15)
     - Stable
     - Go + kcc C23
     - ``TestPhaseV1234S_SignalControlBothEngines``,
       ``TestPhaseV1234S_UnsupportedSignalIsNotSilentlyAccepted``
   * - Native / WASM signal boundary
     - Not supported
     - -
     - ``TestPhaseV1234S_NativeRefusedByBothEngines``, K145/K108