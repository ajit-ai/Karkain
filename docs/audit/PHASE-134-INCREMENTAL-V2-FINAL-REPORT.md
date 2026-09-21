# Phase 134 — Incremental Compilation v2 — Final Report

**Verdict: COMPLETE.** Phase 105's whole-assembly cache is upgraded to
per-module translation units: only changed modules recompile, everything
else relinks from content-keyed cached objects.

## 1. What changed

| File | Change |
|---|---|
| `pkg/codegen/split_tu.go` (new) | Runtime header/source transform (`HeaderForRuntime`, `RuntimeSourceFor`, shared-global extern set), gcc-style toolchain probe (`HostToolchain`/`SplitToolchain`/`SplitFlags`), per-TU compile + link + best-effort PCH (`CompileTU`, `LinkObjects`, `BuildPCH`) |
| `pkg/codegen/module_split.go` (new) | `GenerateSplit`: one TU per source module by top-level function ownership (duplicates are already K107, so attribution is exact), shared decls in every TU, program extras (concurrency/profiling/C-imports/kernels/main) in the root TU |
| `pkg/codegen/codegen.go` | Monolith factored into reusable `prescanTables` / `emitSharedDecls` / `emitFuncBodies(only)` with zero monolith behavior change |
| `pkg/compiler/incremental.go` | Manifest v2 (`RuntimeKey`, `RuntimeObj`, `Objects`), atomic multi-artifact `StoreArtifacts`, `ObjectNameFor` (sanitized base + 8-hex content key), `RuntimeKey` (compiler key + flags + header text, self-invalidating) |
| `pkg/cli/incremental.go` | `buildSplitFlow`: runtime TU rebuilt only on runtime-key change, per-module TUs compiled per plan status, relink on freshness, concatenated beside-source `main.c` view preserves the Phase 105 contract; falls back to the v1 flow for non-gcc toolchains, concurrency/profiling programs, native-link |
| `pkg/cli/phase134_incremental_v2_test.go` (new) | 5-test gate (see §3) |

Deliberate boundaries (unchanged): concurrency/profiling programs use the
v1 whole-assembly flow; MSVC `cl.exe` uses the monolith fallback;
cross-module use of program-level surfaces fails loudly at C compile time
(every pinned multi-file corpus program is single-surface-per-file).

## 2. Design notes

- **Internal linkage, not cross-TU references.** The runtime header gains
  `static` on top-level function definitions (prototypes static-ified to
  match — a static definition after a non-static prototype is a hard C
  error); the 9 file-scope mutable globals become `extern` + single
  definitions in `runtime.c`. `static inline` helpers, `static const`
  tables, typedefs and macros duplicate harmlessly per TU.
- **Line-based transform over emitted preamble text**, so new runtime
  helpers are handled automatically; a missed shape fails loudly at C
  compile/link time, never silently.
- **Content-keyed object names** (`<sanitized>_<8hex>.o`): identical
  sources share objects; reused objects are never rewritten (no mtime
  churn); the manifest is always rewritten (cheap).
- **Beside-source `main.c`** is the deterministic concatenation
  header + runtime + root + modules, keeping the Phase 105 debuggability
  contract while the cache holds the real split artifacts.

## 3. Gate — `pkg/cli/phase134_incremental_v2_test.go` 5/5 PASS

| Test | Time | Pins |
|---|---|---|
| `TestPhase134_ObjectNaming` | 0.00s | determinism, content-keying, sanitization, empty-base fallback |
| `TestPhase134_RuntimeKey` | 0.00s | determinism, header-text / flag / compiler-key sensitivity |
| `TestPhase134_StoreArtifactsRoundTrip` | 0.03s | multi-file atomic persist, manifest round-trip, stale-version (v1) rejection |
| `TestPhase134_SplitEmissionStructure` | 0.01s | include guard, extern decls, single definitions, column-zero def ownership (root owns `main`, math owns `twice`, prototypes shared) |
| `TestPhase134_SplitFlowE2E` | 15.94s | clean split build → golden `41/42/Hello karkain`; manifest v2 + runtime key + 3 objects + `runtime.o`; no-op `3 reused` in **79 ms** (beats the ≤100 ms roadmap target); body-only leaf edit → golden preserved with exactly `1 compiled`; cache `Clear` → full `3 compiled` rebuild |

Two test-authoring defects root-caused along the way (both in the gate,
not the implementation): `main` keeps its canonical C name (`userFuncC`
leaves `main`/`getArgs` unmangled — the gate first asserted the mangled
name), and call sites share the `name(` substring with definitions, so
ownership is asserted on column-zero `sym(...) {` definition lines, not
substring counts.

## 4. Regressions — green

- `go build ./...` clean; `go vet ./pkg/cli/` clean.
- `pkg/compiler` full suite (10 Phase-105 plan tests) PASS 0.48s.
- Phase 105 E2E (`EquivalenceAndCache` incl. `clean=30659ms noop=136ms
  leaf=903ms, `InterfaceChangeE2E`, `FailedBuildDoesNotPoisonE2E`,
  multi-error suites) PASS on the same split-flow production code.
- `gofmt -l` flags on touched files are the repo-wide CRLF artifact
  (identical for pristine committed files, e.g. `pkg/cli/commands.go`);
  the new gate file is gofmt-clean.
- Debris discipline: `examples/phase105/{main.c,main.exe,.karkain-cache/}`
  removed; only `*.kark` sources are tracked there.

## 5. Roadmap movement

GA-2 infra track: 134 lands (132 → 134 dependency satisfied). Next:
135 local registry MVP, 136 LSP v2, 137 kcc parity
(concurrency/profiling/SIMD — the split emission's documented fallback
surfaces are exactly 137's work list).
