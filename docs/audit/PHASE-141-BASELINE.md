# Phase 141 BASELINE — Performance & Memory (compiler itself)

Date: 2026-09-23. Target: GA-3 fourth step (after 140).

## 1. SSA optimizer state (pkg/ir/ssa, Phases 93/94)

- Full pass inventory with green unit suites: Mem2Reg, FoldConst, CSE,
  DCE-simple, LICM, SROA + Pipeline orchestrator (fixpoint ≤ 10).
- Emission wiring (Phase 53, `emitFunctionViaIR`): lower → FoldConstants +
  EliminateDeadCode (module methods delegating to FoldConstPass/
  DCESimplePass) → Verify → emit, with recover+fallback to legacy per
  function, and a blanket bypass for closure-bearing bodies (Phase 121).
- Nothing else is wired: the Pipeline, Mem2Reg, CSE, LICM and SROA have
  no emission effect. The IR path is therefore fold+DCE-only in practice.

## 2. kcc build memory (the 4GB-host class)

- Documented: stage-2/3 self-build SEGFAULTs/OOMs on ~4GB hosts (kcc-build
  OOM class); K127 aborts cleanly below 1.5 GiB free; Phase 129 opened a
  measurement table (scripts were planned, never shipped).
- No RSS instrumentation exists in-repo. A `measure-rss.ps1` sampler
  (500 ms WorkingSet64 polls over the child tree) is the missing tool;
  WMI PID-reuse cycles must be guarded (visited-set + depth cap) or the
  sampler itself hangs.
- Biggest lever on paper: split-TU kcc self-build (per-file gcc, Phase 134
  pattern) instead of one giant TU. Not E2E-provable on this host
  (stage-2 aborts) — mechanism + unit proof only, if attempted at all.

## 3. Profiler allocation accuracy (Phase 110)

- `malloc`/`free` text-wrapped in user code only (preamble helpers
  excluded by placement). make_string/make_array/make_map run inside
  preamble helpers → container constructions are invisible (structs stay
  invisible: map-backed). Schema `karkain-profile-v1`, Go-side
  `ProfileAllocation{count,bytes,peak}` + text/JSON renderers; existing
  gates assert via Contains/unmarshal (additive fields are safe).

## 4. Slice plan (as executed)

- **141A**: per-pass empirical trials on a runtime-parity corpus;
  SimplifyCFGPass (constant-br folding + dead-block sweep) as the one
  production addition; loop-soundness verdicts per pass.
- **141B**: `scripts/measure-rss.ps1` + measured table on this host.
- **141C**: Value-cell counter (macro redirect at user-code call sites,
  ISO blue-paint) + additive `cells` schema field + gates.
