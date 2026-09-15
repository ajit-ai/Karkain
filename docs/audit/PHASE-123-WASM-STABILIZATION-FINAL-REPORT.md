# PHASE 123 FINAL REPORT — WASM STABILIZATION & PRODUCTION FOUNDATION

Status: **COMPLETE**
Date: 2026-09-15
Gate: `pkg/cli/phase123_cli_test.go` (`TestPhase123_Wasm*` subtests)

Phase 123 completes the WASM stabilization work: the `wasm32-wasi` backend
is promoted from "Experimental" to **Production Candidate** with full WASI
boundary implementation (exit codes, `stderr` diagnostics, `getArgs()`),
a five-program example corpus, deterministic output verification, and
comprehensive negative-feature documentation.

---

## 1. DELIVERABLES

### 1.1 WASI Boundary Implementation

**Imports** (`pkg/wasm/runtime.go`):

| WASI Host Function | WASM Import Index |
|--------------------|-------------------|
| `fd_write` (stderr = fd 2) | 0 |
| `args_sizes_get` | 1 |
| `args_get` | 2 |
| `proc_exit` | 3 |

**Runtime helpers** — all Karkain-owned, emitted after imports:

| Index | Function | Purpose |
|-------|----------|---------|
| 4–27 | `rt_Alloc` … `rt_Eq/Ne/Error` | Standard runtime helpers |
| 28 | `rt_GetArgs` | Reads WASI argc/argv into heap globals |
| 29 | `_start` | WASM entry point: reads args → calls `main` → `proc_exit` |

Global slots 0–2: `wasiArgcGlobal`, `wasiArgvGlobal`, `_pad`.

**`_start` bootstrap sequence:**
1. Zero `offWasiArgc` (160) / `offWasiArgvBufLen` (164)
2. `args_sizes_get` → get buffer sizes
3. Allocate `argvBuf` + `argvPtrs` on heap
4. `args_get` → populate buffer and pointer array
5. Store globals: `wasiArgcGlobal=1`, `wasiArgvGlobal=2`
6. Call `main`
7. Derive exit code: `(v & 1 == 0) ? int(v >> 1) : 0`
8. `proc_exit(code)`
9. `rt_error` → `proc_exit(1)` with stderr diagnostic via `fd_write`

### 1.2 Postfix `n--` Parser Fix (`pkg/wasm/backend.go`)

The `--` operator was not handled by the existing postfix chain detection.
Root cause: `i++` lexes as two `+` tokens and builds `BinaryExpr` with
nil-Right, but `n--` lexes as two `-` tokens, building `BinaryExpr{op:"-", left:n, right: UnaryExpr{"-", nil}}`.

Fix in `genArith`: the postfix check now also catches `Right.(*parser.UnaryExpr)` with `Operator == "-"` and `Operand == nil` and routes through `genPostfix`.

### 1.3 Five-Program Example Corpus (`examples/wasm/`)

| Directory | Source | Expected stdout |
|-----------|--------|-----------------|
| `hello.kark` | `print("hello wasmtime")` + getArgs | `hello wasmtime\|42\|done` |
| `functions/` | `add(2,3)`, `double(7)` | `5\|14` |
| `control_flow/` | `for`/`while`/`if` + `n--` | `30\|120` |
| `data/` | Array create + index | `5\|30` |
| `strings_builtin/` | `len("Hello, WASM!")` | `12` |

### 1.4 KIR Pin Refresh (`pkg/cli/phase122_pipeline_ownership_test.go`)

The Phase 122 `KIRContinuity` test pinned `kir text == 6399 lines`. This
was a pre-existing stale pin: Phase 123's enum-ADT commit (`89b7961`)
added ~370 lines to `src/compiler/` (ast +23, checker +134, codegen +132,
kir +6, parser +69). Updated pin to 6645 with explanatory comments.
Verified deterministic: `kir text == kir verify == 6645` (twice, ~277s runs).

### 1.5 Docs & Status Promotion

New Sphinx role `:production-candidate:` added to `conf.py` + CSS.
WASM promoted to Production Candidate across the doc tree:

- `index.rst` — target table row updated
- `status/scope.rst` — new "Production Candidate" bucket (six total)
- `status/experimental.rst` — WASM entry removed
- `status/implemented.rst` — WASM row updated
- `reference/example-matrix.rst` — WASM row updated (5 examples)
- `reference/compatibility.rst` — divergence note updated
- `reference/stable-api.rst` — target description updated
- `examples/systems.rst` — expanded WASI boundary section + corpus
- `examples/index.rst` — WASM entry updated
- `compiler/runtime.rst` — WASM runtime section rewritten (4 imports, `_start`, exit-code derivation)
- `compiler/pipeline.rst` — WASM path updated
- `development/developer-preview.rst` — WASM bullet updated
- `development/roadmap.rst` — experimental surfaces list updated
- `development/testing.rst` — test map updated
- `targets/cross-compilation.rst` — WASM row updated
- `README.md` — WASM promoted from Experimental to Production Candidate
- `examples/EXAMPLES.md` — WASM corpus reference updated
- `.gitignore` — WASM build output dirs ignored

### 1.6 `.gitignore` Update

Added:
```
# WASM build outputs (karkain build --target wasm32-wasi)
/examples/wasm/**/build/
examples/self-hosting/**/*.c
```

---

## 2. TEST RESULTS

### 2.1 WASM Gate Tests (`TestPhase123_Wasm*`)

| Subtest | Status | Duration |
|---------|--------|----------|
| `TestPhase123_WasmExampleCorpus` | PASS | 5 programs × compile+run |
| `TestPhase123_WasmExitCodeContract` | PASS | exit 0/1/3/42 verified |
| `TestPhase123_WasmRuntimeError` | PASS | stderr `runtime error:` + exit 1 |
| `TestPhase123_WasmGetArgs` | PASS | argc/argv round-trip |
| `TestPhase123_WasmNegativeFeatures` | PASS | struct/map/import → K108 |
| `TestPhase123_WasmDeterminism` | PASS | byte-identical .wasm output |
| **Total** | **PASS** | **~48.5s** |

### 2.2 Phase 108 Regression

| Gate | Status |
|------|--------|
| `TestPhase108WasmE2E` | PASS |
| `TestPhase108WasmDeterministicBuild` | PASS |
| `pkg/wasm` full suite | PASS |

### 2.3 Phase 122 Gate (Post-Pin Fix)

| Subtest | Status | Duration |
|---------|--------|----------|
| `CompilerSelfCheck` | PASS | ~103s |
| `KIRContinuity` (6645 lines) | PASS | ~277s |
| `ModuleSystem` | PASS | — |
| `ExitCodeContract` | PASS | — |
| **Total `TestPhase122_PipelineOwnership`** | **PASS** | **~532s** |

### 2.4 Sphinx Documentation

| Check | Status |
|-------|--------|
| `sphinx -b html -W` | 0 warnings |
| `sphinx -b linkcheck` | 0 broken links |

### 2.5 Go Vets

| Command | Status |
|---------|--------|
| `go vet ./pkg/cli/` | PASS |

---

## 3. KNOWN LIMITATIONS (NOT DEFECTS)

| Limitation | Evidence | Acceptance |
|------------|----------|------------|
| WASM target is Go-engine only; kcc parity deferred | `TestPhase123_WasmNegativeFeatures` confirms K108 `modules/imports` | Documented |
| `import std.*` on wasm → K108 (not link error) | In-tree fixture shows `feature 'modules/imports' is not supported for target wasm32-wasi` | Documented |
| Structs/maps/slices C-interop → K108 | `TestPhase123_WasmNegativeFeatures` confirms | Documented |
| `1..N` range syntax not supported in for-loops | C-style `for (let i = 0; ...)` required | Documented |
| WASM profiling deferred | `pkg/wasm` does not instrument | Documented |
| kcc parity boundaries (concurrency, SIMD, WASM) | Experimental surfaces; kcc parity deferred | Documented |
| Default wasm build writes to `build/` beside source | `pkg/cli/wasm.go` line 27 | Documented |

---

## 4. EVIDENCE TRAIL

- WASM gate: `pkg/cli/phase123_cli_test.go`
- KIR pin: `pkg/cli/phase122_pipeline_ownership_test.go`
- WASM runtime: `pkg/wasm/runtime.go`, `pkg/wasm/backend.go`
- CLI: `pkg/cli/wasm.go`
- Examples: `examples/wasm/{hello,functions,control_flow,data,strings_builtin}/`
- Docs: `docs/source/{conf.py, _static/karkain.css, index.rst, status/scope.rst, status/experimental.rst, status/implemented.rst, reference/example-matrix.rst, reference/compatibility.rst, reference/stable-api.rst, examples/systems.rst, examples/index.rst, compiler/runtime.rst, compiler/pipeline.rst, development/developer-preview.rst, development/roadmap.rst, development/testing.rst, targets/cross-compilation.rst}`, `README.md`, `examples/EXAMPLES.md`, `.gitignore`

---

## 5. COMMIT

Focused commit on `develop` → merge to `main` → push both.
