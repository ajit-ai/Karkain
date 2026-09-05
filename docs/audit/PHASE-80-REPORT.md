# PHASE 80 REPORT — SIMD / Vector Execution Architecture

**Status:** COMPLETE
**Gate:** Phase 79 verdict = READY WITH PREREQUISITES; all 3 prerequisites executed + SIMD delivered.
**Branches:** develop → main → push both (AGENTS.md mandatory workflow).

---

## 1. Scope (from Phase 79 Executive Summary §12)

1. Reconcile Math IR (connect to pipeline OR mark as reference caller-free evaluator).
2. Document `pkg/bootstrap` failures as a known blocker with an exit criterion.
3. Establish the parity baseline: CPU = cross-backend oracle + differential
   (tolerance) harness for CPU vs GPU vs NPU — before hardware/vectorization
   validation.
4. Deliver the SIMD / vector execution path on the CPU backend.

## 2. Prerequisite Outcomes

### 2.1 Math IR reconciled (prereq 1)
`pkg/math/ir.go` package doc explicitly restates ownership: **reference
caller-free evaluator** (invoked by nothing today; contract = caller executes
evaluator). Decision record: `docs/audit/PHASE-80-MATH-IR-RECONCILIATION.md`.

### 2.2 Self-hosting blocker documented (prereq 2)
`docs/audit/SELF-HOSTING-BLOCKER.md`: exit criterion (Stage1/Stage2/
BitwiseIdentity/TestArgs green + full suite no skip) + per-phase tracking.
**Re-measurement during Phase 80:** `go test ./pkg/bootstrap/... -count=1` →
`ok karkain/pkg/bootstrap 283.772s`, and inside `go test ./... -count=1` →
`ok ... 300.748s`. The Phase 79 FAIL measurement predates this environment;
current ground truth is GREEN. The exit criterion is retained as the standing
gate against future regressions (a re-failure flips status back to TRACKED).

### 2.3 Parity baseline established (prereq 3)
`pkg/backend/parity/parity.go`:

| Symbol | Purpose |
|---|---|
| `Candidate` | Backend name + `*backend.Result` |
| `Diff` | One tolerance-exceeded value event |
| `Comparison` | Verdict record (numeric parity, max abs diff, diff count, no-numeric flag) |
| `Report` | Oracle case name, pass verdict, reusable candidate map, `NoNumeric` list |
| `Compare(caseName, graph, oracle, candidates, tol)` | Per-output differential comparison |
| `ReportString` | Deterministic text report |

Semantics:
- CPU scalar backend = the only numeric oracle (returns `Result.Values` for all
  output nodes).
- GPU / NPU execute stubs return metadata only ⇒ recorded as `NoNumericOutput`
  (structural parity gap surfaced in the report, never faked as equality).
- Candidates with numeric output are compared per output node within a
  tolerance; a single exceeding diff fails that candidate and the report.

## 3. SIMD Implementation

### 3.1 Approach
GNU vector extensions: `typedef double double4 __attribute__((vector_size(32)))`
 = 4 × double lane. Supports gcc and clang, **no `-march` flags, no ISA gating**
 (SSE introduces specialization later; SSE2 is a floor since ~2003 x86-64, but
 *vector_size* keeps vectorization portable to ARM NEON-capable compilers too).

### 3.2 Files
- `pkg/backend/cpu/simd_c.go` — `SimdExtensions` (C runtime: `tensor_simd_contig2`
  guard, `tensor_simd_add/sub/mul/div`, `tensor_simd_matmul` K-loop in 4-lane
  blocks, `tensor_simd_relu`), `TensorSimdCRuntime = TensorCRuntime +
  SimdExtensions` (concat; no symbol collisions).
- `pkg/backend/cpu/runtime.go` — `CPUBackend{simd bool}`; `New()` (scalar
  oracle); `NewSimd()` (SIMD variant); `Execute` → `execute(graph, inputs,
  simd)`; `generateCProgramVariant(graph, inputs, simd)` embeds
  `TensorSimdCRuntime` when `simd`; `emitOpNamed` gained the `simd` flag +
  `simdFn` helper mapping add/sub/mul/div/matmul/relu to the SIMD runtime.
  SIMD result sets `Metadata["simd"]="true"`.

### 3.3 Coverage
- Vectorized elementwise: contiguous **same-shape** add/sub/mul/div.
- Vectorized relu (elementwise, same rule).
- Vectorized matmul: inner-K loop consumed in 4-double lanes (chained
  multiply-add within the lane) — FP rounding order differs from the scalar
  backend (documented, handled by tolerance in parity).
- **Broadcast falls back to scalar** `tensor_*` (contiguity guard), preserving
  correctness while keeping the vector path simple.
- Non-vectorized ops (create, transpose, etc.) unaffected.

## 4. Oracle Completeness Fix (found by the parity tests)

The pre-existing RESULT emission only covered `OpCreate` outputs. Computed
outputs (Add/MatMul/Relu) never printed values, so `Result.Values` was empty
for them on **both** scalar and SIMD paths. The C emitter now prints a
multi-line RESULT block (`RESULT <id>` / shape line / `|numel|` line / values
line) for **every** declared output — the oracle now exposes numeric values for
the full graph. This is a correctness-strengthening fix to a latent gap; the
test suite that previously asserted only `Outputs != nil` now additionally
verifies numeric Values via parity.

## 5. Verification

`pkg/backend/parity/parity_test.go` (all pass, gcc-gated):

| Test | Proves |
|---|---|
| `TestCpuScalarIsTheNumericOracle` | GPU + Intel-NPU recorded as NoNumericOutput (stub contract) |
| `TestCpuSimdMatchesOracleNumeric/add` | SIMD add == scalar oracle, exact (tol 0.0) |
| `TestCpuSimdMatchesOracleNumeric/matmul` | SIMD matmul == scalar within 1e-12 (FP-order tolerance) |
| `TestCpuSimdMatchesOracleNumeric/relu` | SIMD relu == scalar, exact |
| `TestSimdBroadcastFallsBack` | broadcast add falls back to scalar path, still correct |
| `TestSimdReportsFailureOnMismatch` | harness detects injected divergence (fails verdict) |
| `TestDispatcherSimdBackendIntegration` | SIMD backend is a first-class dispatcher backend; Values merge through |

Static runs: `go vet ./pkg/...` clean; `go test ./pkg/backend/... ./pkg/tensor/
... ./pkg/math/... ./pkg/npu/... -count=1` green; full suite
`go test ./... -count=1` green including `pkg/bootstrap`.

## 6. Risks / Follow-ups

1. Matmul FP-order divergence is intentionally out of lock-step (SIMD lane
   chaining) → always compare matmul with tolerance ≥1e-12; document EXPECTED.
2. Wide-vector specialization (AVX-512 / SVE) and multi-threading deferred;
   `vector_size(32)` keeps the code portable now (gcc/clang on x86-64 + ARM).
3. GPU/NPU numeric execution remains stubs (WGSL/NPU backends unchanged this
   phase); parity harness will consume them as numeric candidates when they
   materialize — currently `NoNumericOutput` is the honest structural record.
4. Distance from scalar in exactness for elementwise ops is 0 in tests; if a
   later optimizer reorders, keep `tolerance` explicit, never 0 for matmul.

## 7. Commands
```
go build ./pkg/backend/...
go vet ./pkg/...
go test ./pkg/backend/parity/... ./pkg/backend/cpu/... -count=1
go test ./pkg/backend/... ./pkg/tensor/... ./pkg/math/... ./pkg/npu/... -count=1
go test ./... -count=1            # full suite, incl. pkg/bootstrap
```