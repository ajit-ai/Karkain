Implemented
===========

Everything below is :implemented:`Implemented` — real, tested through the
pipeline, and (where appliable) byte-identical on both engines. Each entry
carries a one-line description.

.. list-table:: Implemented features
   :widths: 32 68
   :header-rows: 1

   * - Feature
     - Description
   * - **Language core**
     - Variables (``let``/``var``/``const``), functions, control flow,
       structs, enums, modules — tested on both engines.
   * - **Standard library**
     - 6 importable modules: ``std.string``, ``std.collections``,
       ``std.io``, ``std.encoding``, ``std.crypto``, ``std.testing``.
   * - **CLI**
     - ``build``, ``run``, ``test``, ``check``, ``fmt``, ``lint``,
       ``debug``, ``prof``, ``target``, ``pkg``, ``lsp``,
       ``workspace``, ``clean``, ``explain``, ``bench``.
   * - **Cross-compilation** (Phase 111)
     - Explicit ``--target <triple>`` with a Karkain-owned target model,
       same-machine/foreign-machine detection, deterministic failure on a
       missing cross-linker.
   * - **Debug / trace** (Phase 112)
     - ``karkain debug`` emits an opt-in function enter/leave trace.
       Go engine only.
   * - **Profiling** (Phase 110)
     - ``karkain prof`` emits deterministic per-function counts, timings,
       call graph and folded stacks in text/json/folded formats. Go engine
       only.
   * - **Concurrency runtime** (Phase 107)
     - Work-stealing scheduler with ``spawn/join/chanSend/receive`` and
       actor primitives. Go engine only; kcc parity deferred.
   * - **SIMD / vector types** (Phase 106)
     - ``[N]f32``/``[N]f64``/``[N]i32``/``[N]i64`` lane types with
       ``@simd_*`` builtins on x86 and ARM.
   * - **WASM target** (Phase 123)
     - Karkain-owned ``wasm32-wasi`` backend emits deterministic WASM
        binaries; WASI exit codes, ``stderr`` diagnostics and ``getArgs()``
        implemented. :production-candidate:`Production Candidate` — see
        :doc:`/reference/stable-api` target column.
   * - **Runtime error model** (Phase 100)
     - Checked division/modulo/indexing report
       ``runtime error: <kind> at <file>:<line>``, identically on both
       engines.
   * - **Stack traces** (Phase 101)
     - Runtime-error stack dumps with function + line frames on both
       engines.
   * - **Self-hosted kcc compiler** (Phases 95–99)
     - The default engine for ``check/build/run/test``; bootstrap
       stage-2 == stage-3 bitwise identical.
   * - **Package manager local resolution**
     - Deterministic resolver with lockfiles and atomic fetch; no remote
       registry yet.
   * - **DWARF debug info** (Phase 104)
     - DWARF 4 sections emitted and self-verified by a Karkain-owned parser;
       integrated via the gcc C-transpile path.
   * - **Incremental compilation** (Phase 105)
     - Content-addressed whole-assembly cache with interface hashing and
       dependency-aware invalidation.
   * - **Format / lint toolchain** (Phases 82–86)
     - ``karkain fmt`` (idempotent, recursive) and ``karkain lint`` with a
       real diagnostics pipeline.
   * - **VS Code extension**
     - ``extension.js`` with check/compile/run/format commands and a
       grammar, validated by a gate test.
   * - **LSP server**
     - Real language-server features (diagnostics, completion, hover,
       go-to-definition, formatting) on a shared analyzer driver with
       ``karkain check``.

.. seealso::

   :doc:`/development/developer-preview` — the honest capability summary.
   :doc:`/status/experimental` — features that work but are not yet stable.