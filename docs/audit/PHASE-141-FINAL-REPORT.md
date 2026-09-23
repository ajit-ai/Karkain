# Phase 141 FINAL REPORT — Performance & Memory (compiler itself)

Verdict: **COMPLETE** (A: optimizer headline + C: alloc accuracy + B:
measurements; split-TU self-build stays future work with the lever
documented).

## A. Optimizer → codegen (141A)

- **New gate** `pkg/codegen/phase141_opt_test.go`: 6-case corpus
  (fold/cse/loop/dce/licm/sroa), optimized-vs-legacy runtime parity
  goldens, fold pin (`make_int(14)`, no surviving 2/3/4), DCE pin (no
  `99`), determinism (byte-identical C twice), anti-bloat bound (+2%).
- **New pass** `pkg/ir/ssa/passes_simplify_cfg.go`: constant-br → jmp +
  unreachable-block sweep (loop-safe: BFS from entry), with 3 unit tests.
  Wired Fold → SimplifyCFG → DCE. This is the one production behavior
  addition, and it visibly deletes folded-out arms.
- **Measured per-pass verdicts** (each trialed against the corpus):
  fold/DCE sound, wired. **Mem2Reg: UNSOUND on loop-carried cells**
  (same-block-only use rewrite deletes cross-block-referenced loads —
  root-caused to `replaceLoadAt`; a function-wide rewrite was trialed,
  keeps the ssa suite green, but loop programs still miscompile through
  an unisolated second interaction) — stays UNWIRED pending the
  loop-aware slice. **CSE: sound but inert** (fresh raw temps per
  occurrence defeat syntactic matching) — unwired, no benefit. **LICM/
  SROA: sound on the corpus but unreachable** (the lowerer falls back to
  legacy for C-style for loops, so no loop IR reaches them) — unwired
  until lowerer coverage grows.
- **Honesty fix**: the `emitFunctionViaIR` fallback comment overstated
  ("can never miscompile") — Verify checks structure only (terminators,
  single-assignment, arities), not use-def soundness. Comment corrected
  in `codegen.go`.
- Transient-gcc retry in the gate builder (Windows AV-lock class, Phase
  114 precedent); CRLF stdout normalization (Phase 111 pattern).

## B. Memory measurements (141B)

- `scripts/measure-rss.ps1` (new, fixed mid-phase: WMI PID-reuse cycles
  hung the first version; visited-set + depth cap). Table, dev host,
  2026-09-23 (500 ms WorkingSet64 polls, child-tree sums):

  | Workload | Peak RSS |
  |---|---|
  | kcc `check src/compiler/main.kark` (self-hosted full assembly) | 544.5 MB |
  | `go build ./cmd/karkain` | 229.9 MB |
  | `gcc -O0 -c src/compiler/main.c` (1.3 MB single TU) | 484 MB |

- Reading: no single spike explains the 4GB-host OOM class — it is the
  STACK (Go build + kcc run + gcc + parallel suites) against ~600 MB
  free, plus stage-2/3 doing strictly more than `check` (full codegen +
  link). The 2.5 GB roadmap target concerns the full stage-2/3 pipeline,
  unmeasurable here (K127 aborts first — itself the correct behavior).
- Prescribed next lever (not implemented): split-TU kcc self-build
  (per-file gcc, Phase 134 pattern) to cut the ~484 MB gcc component;
  mechanism provable by unit, E2E only on a ≥8 GB host.

## C. Allocation-site accuracy (141C)

- `prof_runtime.go`: `k_pf_cells` + macro redirect of
  make_string/make_array/make_map at user-code call sites (ISO C blue
  paint — self-reference calls the real helper; preamble definitions sit
  above, unaffected). Structs stay invisible (map-backed) — documented.
- Schema: additive `"cells"` in the C dump + Go `ProfileAllocation` +
  text renderer (`N value cell(s)`); `karkain-profile-v1` unchanged.
- Gate `pkg/cli/phase141_prof_test.go`: container-heavy program pins
  exactly 6 cells (2 strings + 1 key-string + 2 arrays + 1 map) in JSON
  and text. Phase 110 suites green on both sides (Contains/unmarshal
  assertions unaffected by additive fields).

## D. Gates + CI + regressions

- New: `TestPhase141_PipelineParity`, `TestPhase141_DeterministicBuild`,
  `TestPhase141_ProfCellsJSON/Text`, 3 SimplifyCFG unit tests.
- CI: `TestPhase141` (codegen) + `pkg/ir/ssa` steps.
- Green: full `pkg/codegen` (65s), `pkg/ir/ssa`, Phase 110 (both sides),
  conformance 64/64 (with SimplifyCFG live), `go vet`, `go build`.

## E. Boundaries (documented, NOT defects)

- Mem2Reg/CSE/LICM/SROA unwired (verdicts above); loop-aware promotion
  + lowerer loop coverage are the next optimizer slices.
- Struct cells invisible to the profiler (map-backed).
- Split-TU self-build unimplemented (documented lever).
- RSS sampler is Windows-only PowerShell (CI hosts differ; numbers are
  host-relative, never gates).
