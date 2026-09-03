# PHASE 79 — DETAILED AUDIT (Architecture, Pipeline, IR, Backends, Determinism, Optimization, Tests, Bugs, Code Health, Docs)

Companion to `PHASE-79-EXECUTIVE-SUMMARY.md`. All findings cite concrete
files/symbols. Confidence labels: HIGH / MEDIUM / LOW / INSUFFICIENT EVIDENCE.

---

## 1. Repository Architecture (P79-01/02/03)

Top-level layout: `cmd/karkain/main.go` (CLI driver), `pkg/` (17 packages),
`src/` (+`src/compiler/`) = Karkain-written self-hosted compiler (lexer/parser/
codegen/sema/ast/main `.kark`, plus `runtime.c`), `runtime/` C runtime
(`actor.c`, `quantum.c`, `reflect.c`, `rpc.c`), `docs/audit/` (17 audit docs),
`docs/phases/`, `stdlib/`, `editors/`, `std/`, `examples/`.

### Dependency graph (internal imports only, verified via `go list -f {{.Imports}}`)

```text
lexer, math, tensor, ir, ir/ssa, runtime, runtime/gpu, diagnostics,
std*, pm, bootstrap   (leaves — no internal deps)
    ↑
parser -> lexer
jit -> ir
sema -> jit, parser
codegen -> ir/ssa, parser, sema
cli -> codegen, diagnostics, lexer, parser, sema
lsp -> lexer, parser, sema
backend -> tensor
backend/cpu -> backend, tensor
backend/gpu -> backend, tensor
npu -> backend, tensor
npu/{intel,qualcomm,apple,arm,amd} -> backend, npu, tensor
```

**Layering observations (evidence):**

- Backends consume **Tensor IR** (`pkg/tensor`), never the AST. No backend
  imports `pkg/parser`. **Conforms** to "backend implements semantics, not
  defines them." Confidence HIGH.
- `pkg/backend` is the single abstraction (backend.go `Backend` interface:
  `Name/Capabilities/Supports/Execute`), with `dispatch.go` CPU-first fallback.
  No backend-to-backend coupling. Conforms. HIGH.
- **Math IR (`pkg/math`) is a leaf with NO importers.** Confirmed via
  `go list -deps ./...` (no non-self importer) and grep for `"karkain/pkg/math"`
  (zero matches outside the package). **Architectural finding** — either
  intentionally a reference eval layer or dead. HIGH.

---

## 2. Compiler Pipeline (P79-02)

Actual end-to-end path (from `pkg/cli/commands.go` + `pkg/codegen/codegen.go`):

```text
main.go → cli.RunCommand
  → ValidateKarFile
  → loadSourceWithSiblings (multi-file concat)
  → parseSource (pkg/parser ParseProgram → AST via arena)
  → runBorrowCheck (pkg/sema BorrowChecker.Check)
  → codegen.New(cfg).GenerateAndCompile(prog, file)
      → write C (emitCHeader, enum/struct fwd decls, per-function)
      → for each FuncDecl: emitFunctionViaIR (SSA path)
          → ssa.Module → newFnLowerer.lowerStmts (AST→SSA)
          → FoldConstants → EliminateDeadCode → Verify
          → emitSSAFunction (SSA→C23)
          → on failure: fallback genFuncDecl (legacy C)
      → write .c → gcc compile → .exe
```

Alternate targets: GPU/WGSL (`pkg/codegen/wgsl.go`), OpenCL (`gpu.go`),
SPIR-V (`spirv.go`), native C11 (`native.go`), quantum (qasm/qir/qec/...),
Tensor WGSL (`tensor_wgsl.go`). Bytecode VM via `pkg/jit` executes `.kbc`.

Confidence: HIGH. The pipeline is real, traceable, and the cli E2E suite passes.

---

## 3. IR Architecture (P79-04)

Identified actual IRs:

| IR | Location | Producer → Consumer | Purpose | Mut model |
|----|----------|--------------------|---------|-----------|
| SSA IR | `pkg/ir/ssa` | `lower.go` (AST→SSA) → `emit_ir.go` (SSA→C23) | compiler middle-end | 3-addr, single-assignment |
| Bytecode IR | `pkg/ir/bytecode.go` | parser → `pkg/jit` engine | `.kbc` VM | stack-based |
| Math IR | `pkg/math` | (standalone, self-tested) → none | algebraic IR + reference eval | functional Node tree |
| Tensor IR | `pkg/tensor` | `pkg/sema/autodiff.go` → CPU/GPU/NPU backends | shared tensor IR | graph of TensorNode |

### 4.1 Core IR
No explicitly named "Core IR". The SSA IR serves as the middle-end core.
Equivalent responsibilities handled by SSA. No IR contains hardware knowledge
(no CUDA/OpenVINO specifics in `pkg/ir/*`). CONFORMS. HIGH.

### 4.2 Math IR
Opset/type system (`math/ir.go`, `types.go`), builder (`builder.go`), reference
eval (`eval.go`), optimizer (`optimize.go`: const-fold, x+0, x*1, x*0, --x).
No imported CPU/GPU/NPU/parser knowledge. However it is **disconnected**.

### 4.3 Tensor IR
`TensorGraph`/`TensorNode` (`ir.go`), full opset (`ops.go`, 33 ops incl.
create, arithmetic, matmul, reshape/transpose/broadcast/slice/concat,
reduce_sum/mean, relu/sigmoid/tanh/softmax, comparison, logical), shape rules
+ broadcasting (`ops.go`), autodiff `BuildGradient` (`gradient.go`), C lowering
(`lower.go`, `EmitC`/`EmitRuntimeHeader`).

- Relationship to Math IR: **independent/hybrid. Tensor IR does not build on
  Math IR** — the two are separate and Tensor IR is the one backends consume.
- **A (correctly builds upon Math IR): NO** — Tensor IR is self-contained.
- **B (duplicates Math IR): PARTIAL** — scalar arithmetic intent overlaps, but
  Tensor IR is tensor-focused. Not harmful, but ownership is split.
- **C (bypasses Math IR): YES** — functionally, tensor path bypasses Math IR.
- Report as a documented architectural decision, not a defect.

### 4.4 IR boundary validation
- Upper layer (Tensor IR) does NOT know GPU/NPU vendor specifics. HIGH.
- Lower layer (backends) does NOT know AST internals. HIGH.
- Duplication: math semantics exist in both Math IR and Tensor IR ops, but
  each at its intended level. No 3x backend-dup of tensor optimization found.
  (GPU/NPU are thin; optimization is centralized in `pkg/npu/*` for NPU and in
  SSA `opt.go` for the frontend.) HIGH.

---

## 4. CPU/GPU/NPU Backend Audit + Parity (P79-05)

### Capability matrix (from real `Capabilities()` in code)

| Capability | CPU | GPU (WGSL) | NPU (default) | NPU Arm |
|------------|-----|-----------|---------------|---------|
| dtypes | i32,i64,f32,f64,bool | f32,i32 | f32,f64,i32 | f32,i32 |
| MaxRank | 8 | 8 | 8 | 4 |
| create/copy | ✓/✓ | -/- | ✓/- | ✓/- |
| add/sub/mul/div | ✓ | ✓ | ✓ | ✓ |
| matmul | ✓ | ✓ | ✓ | ✓ |
| relu/sigmoid/tanh/softmax | ✓ | ✓ | ✓ | ✓ |
| transpose | ✓ | ✓ | ✓ | ✓ |
| reshape | ✓ | - | - | - |
| reduce_sum | ✓ | - | - | - |
| broadcast | - | - | - | - |

Sources: `pkg/backend/cpu/runtime.go:30`, `pkg/backend/gpu/wgsl_backend.go:30`,
`pkg/npu/npu_capabilities.go:11` (`NewCPUCapabilities`), `pkg/npu/arm/ethos.go`.

**Legend notes:** broadcast is defined by shape rules (`ops.go broadcastShapes`)
but is in **no** backend's `SupportedOps`. GPU/NPU execute at codegen/metadata
level, not hardware.

### Semantic parity
- **CPU:** true execution (gcc compile+run). `TestExecuteAdd/MatMul/Relu` pass.
- **GPU:** WGSL generated, returns metadata; no hardware run. Parity UNVERIFIED.
- **NPU:** vendor `Execute` returns stub results (metadata only). Parity UNVERIFIED.
- Integer/float equality between backends: **INSUFFICIENT EVIDENCE** — no
  differential test exists. Tolerance policy not defined anywhere.

Confidence: HIGH that stubs are stubs; LOW on any GPU/NPU semantic parity claim.

---

## 5. Determinism (P79-06)

| Concern | Status | Evidence |
|---------|--------|----------|
| Unordered map→output (SSA emit) | DETERMINISTIC | decls map iterated then `sort.Strings` (emit_ir.go:156) |
| Top-level emit order | DETERMINISTIC | iterates ordered `prog.Statements` slice (codegen.go:54,85,101) |
| rand usage | NONE | no `math/rand`/`crypto/rand` import anywhere in pkg |
| Timestamps in output | NONE | `time` in codegen only for 10s timeout (codegen.go:184) |
| Sema diag ordering | POTENTIALLY NON-DETERMINISTIC | `bc.scope.usage`/`dead` map iteration only affects error *count/aggregation*, not C output (borrow_checker.go:112,116,189) |
| CPU temp files | NON-DETERMINISTIC (runtime artifacts) | `os.CreateTemp` (runtime.go:73) — not compiler output |
| Version string in generated C | DETERMINISTIC | `quantum_opt.go:27` hardcoded `v0.14.0`, not a timestamp |

Overall: **the emitted compiler output ordering is deterministic on the CPU
path.** HIGH confidence for code emission; MEDIUM overall due to unverified
sema diagnostic ordering and temp-file artifacts.

---

## 6. Optimization Boundaries (P79-07)

| Optimization | Layer | Correct? | Shared? | Tested? |
|--------------|-------|----------|---------|---------|
| `Module.FoldConstants` | SSA IR (`opt.go:9`) | yes (semantic/core) | shared | yes |
| `Module.EliminateDeadCode` | SSA IR (`opt.go:148`) | yes | shared | yes |
| `math/optimize.go` `Optimize` | Math IR | unreachable (no callers) | no | self-test only |
| `tensor/gradient.go` `BuildGradient` | Tensor IR | yes (tensor autodiff) | shared | yes |
| `npu` fusion/memory/quantize | NPU-specific | yes (target) | NPU only | yes (opt_test) |
| `genAVX2MatrixMul`/`genScalarMatrixMul` | codegen CPU | yes (target) | CPU only | indirect |
| `sema.OptimizeCircuit` | quantum | yes | quantum | yes |

Boundary violations: **none confirmed.** Optimization is placed at appropriate
layers. The one anomaly is `math/optimize.go` — placed but unreachable.
Confidence HIGH.

---

## 7. Test Architecture (P79-08)

Categories vs existence:

| Category | Status |
|----------|--------|
| Unit (lexer/parser/sema/codegen) | EXISTS (67 files; codegen 15, sema 14) |
| Package | EXISTS |
| Integration / CLI E2E | EXISTS (cli 5 incl. phase70_e2e, bugfix_e2e) |
| Golden/snapshot | MISSING |
| Backend E2E (CPU) | EXISTS (cpu_test: Add/MatMul/Relu, gradient) |
| Cross-backend parity/differential | MISSING |
| Regression (BUG) | EXISTS (bugfix_e2e, Phase 51 Groups A-I) |

Baseline: `go test ./pkg/... -count=1` → 22 ok; `pkg/bootstrap` **FAIL**
(`TestArgs_*`, `TestBootstrap_Stage1Compilation`,
`TestBootstrap_Stage2SelfHosting`, `TestBootstrap_BitwiseIdentity`).
AGENTS.md suite (lexer/parser/codegen/pm) → **PASS**.

Highest-value test improvements: (1) golden/snapshot for SSA→C output,
(2) CPU-vs-GPU-vs-NPU differential harness for shared ops, (3) fix or quarantine
`pkg/bootstrap`.

---

## 8. Bug Regression (P79-09)

| Bug | Area | Status | Regression coverage |
|-----|------|--------|---------------------|
| BUG-1 | Struct codegen | FIXED | `phase19_test.go` TestGenStructDecl/Literal; top-level emit (codegen.go:93) |
| BUG-2 | Option/Result match arms | FIXED | `phase19_test.go` TestGenMatchExpr/OptionSomeNone |
| BUG-3 | Index assignment rvalue | FIXED | `phase19_test.go` TestGenHasKeyAndDelete; identifier-base `m[k]=v` works |
| BUG-4 | `?` propagation no-op | FIXED | `bugfix_e2e_test.go` TestBugFix_OptionPropagation/Unwrap (commit b94c32f) |
| BUG-5 | Borrow lexical scope | FIXED | `borrow_checker_test.go` Phase 51 Groups A-I + TestBorrowCheck_* (commit 1c42a5d) |
| BUG-6 | make_int pool mutable | FIXED | codegen.go:572 value-by-value pool (Phase 52) |
| BUG-7 | Enum payload `_make` | FIXED | `bugfix_e2e_test.go` TestBugFix_EnumPayloadConstructor (b94c32f) |
| BUG-8 | Typed decl rep mix | FIXED | `bugfix_e2e_test.go` TestBugFix_TypedColonDecl (b94c32f) |

BUG-6 explicitly investigated: canonical description ROADMAP:98; fix at
`pkg/codegen/codegen.go:572`; regression via value-by-value pool semantics
(covered indirectly by codegen E2E). NOT "insufficient evidence".

Documentation finding: ROADMAP bug table previously marked BUG-1..5,7,8 OPEN —
corrected in this phase (Category 2).

---

## 9. Code Health (P79-11)

### High
- **Disconnected code:** `pkg/math` (Math IR, optimizer, eval) and the
  `src/analyzer.kark` (1-line placeholder). Dead until wired.
- **GPU/NPU stubs:** `Execute` returns metadata, not results — maintainability
  risk (looks complete, is not).

### Medium
- **Duplicated scalar arithmetic concepts** across Math IR and Tensor IR
  (intentional, but ownership should be documented).
- **`pkg/bootstrap` failing tests** — blocks honest green-suite claim.

### Low
- `src/main.rs` 3-line placeholder vs `.kark` self-host — misleading.
- Temp artifacts (`src/compiler/main.c`) regenerated in working tree; untracked.

### Informational
- No circular imports identified among compiler packages.
- No TODO/FIXME explosion in the main codegen path (spot-checked).

Confidence: MEDIUM (based on package/import + symbol-level inspection).

---

## 10. Documentation Status (P79-12)

| Doc | Status |
|-----|--------|
| AGENTS.md | ACCURATE re 70-78; needs 52-79 phase tracking update (this project) |
| README roadmap table | ACCURATE (52-79 Done / 56 Next) — see README update below |
| ROADMAP bug table (416-423) | was OUTDATED → corrected |
| SPEC.md bug table (462-465) | OUTDATED (BUG-1/2/3 still listed as bugs) → noted |
| KARKAIN_* audit docs (BUG-1..8 open) | OUTDATED (bugs now fixed) → noted |
| NPU_ARCHITECTURE.md | UNVERIFIED (arch spec, not the primary evidence) |

Prioritized: README/AGENTS phase status (52-79 done) — completed in this phase.

---

## 11. Self-Hosting Readiness (P79-10) — see separate doc

`PHASE-79-SELF-HOSTING-READINESS.md`.