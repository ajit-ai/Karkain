# Phase 139 FINAL REPORT — Cross-Compilation Expansion + WASM GC

Verdict: **COMPLETE**. GA-3 second step (after 138).

## A. New triples (slice 139A)

- `pkg/target/triple.go`: new `ArchRiscv64` + `OSMacOS` ("macos"; "darwin"/
  "macosx" aliases → canonical `*-macos`); default env none for macOS
  (clang-only by design); ABI rules: riscv64 Linux-only, macos
  x86_64/aarch64-only, gnu/msvc rejected on macos; error texts and
  `SupportedArchs/OSes` extended.
- `pkg/target/features.go`: 64-bit LE for riscv64; Mach-O + Apple ABI +
  clang linker requirement for macOS; `SupportedTargets` grows
  aarch64-windows, riscv64-linux, x86_64-macos, aarch64-macos
  (`aarch64-windows` already parsed since Phase 111 — now modeled).
- `pkg/codegen/cross_target.go`: new `detectMacOSCrossCompiler`
  (CC override → clang `--target=<arch>-apple-macosx` → deterministic
  `ToolchainError`); `MingwTriple` already covered aarch64; riscv64 Linux
  names compose generically. Same-machine rule untouched; `run --target`
  foreign stays refused (Phase 111).
- `karkain target` listing, usage-error line and `Features`/`Describe`
  follow automatically (all derive from the model).
- Host with only MinGW gcc: all four new triples resolve to listing
  ToolchainErrors (exit 6), proven by the gate; hosts WITH linkers take
  the success branch (gate asserts artifact existence either way — never
  a silent fallback).

## B. WASM GC types (slice 139B-1)

- `pkg/wasm/struct.go` (new): struct declarations lower to boxed cells
  (the array container), field→index resolution is declaration-directed;
  unknown/ambiguous fields, unknown structs and field-set mismatches are
  deterministic K108 errors, never miscompiles. Method calls untouched
  (still K108 — no receiver nodes reach DotExpr).
- `pkg/wasm/backend.go`: layouts pre-collected; scan accepts
  StructDeclStmt/StructLiteral/DotExpr/struct type annotations;
  `genStructLiteral` (literal order free, placement by declared index),
  `genFieldGet`, DotExpr assignment arm mirroring IndexExpr.
- Physical honesty: boxed-cell heap reuse (like arrays/strings), NOT
  WASM-GC-proposal struct.new/get/set ISA — documented in code as the
  future slice.
- Golden `10/20/40` via wasmtime (construct out-of-order + get + set);
  4-case K108 negative table.

## C. Component envelope + wit-bindgen MVP (slice 139B-2/3)

- `pkg/wasm/component.go` (new): `WrapComponent`/`UnwrapComponent` —
  preamble layer-1 + section-id-2 core-module nesting, deterministic,
  round-trip pinned. Canonical-ABI lift/lower documented as future work.
- `pkg/wit` (new): WIT-subset lexer/parser (package/interface/world,
  records/variants/enums/flags, funcs over prim/named/list/option/result/
  tuple), canonical `Summary` (re-parse round-trip pinned), `Stubs`
  (records→`type X struct`, variants/enums→`enum X`; composites rejected;
  functions intentionally NOT emitted — call lowering is future work).
  Stub output is proven compilable: the test parses it with the real
  Karkain parser.
- `karkain wit [--stubs] <file.wit>` wired with help text; exit contract
  0 success / 2 usage (missing file, non-.wit, bad flag) / 3 malformed WIT.

## D. Gates + CI + docs

- `pkg/target/triple_test.go` extended (valid/canonical/invalid/layout);
  stale `riscv64-linux`-rejected and `aarch64-windows`-unsupported
  expectations updated with Phase-139 notes.
- `pkg/cli/phase139_cross_test.go` (new, 7 subtests): normalize table,
  invalid-combo usage errors, missing-toolchain ToolchainErrors (dual
  success/failure branches), cross-run refusal, `target` listing,
  wasmtime struct E2E, wit command E2E. All PASS locally.
- `pkg/wasm`: struct + component tests; full suite green.
- CI: `TestPhase139` + `pkg/target+pkg/wit` steps; new informational
  `toolchain-matrix` job (presence only, `continue-on-error`, never red).
- Docs: target-triples matrix 4→8, normalization table, model/ABI/Features
  sections; tools/target + host-targets matrices; cross-compilation N/A rows
  per new triple; stable-api triple row. Sphinx `-W` clean.

## E. Regressions green

`pkg/target`, `pkg/wit`, `pkg/wasm`, full `pkg/codegen` (45s), CLI Phase
108 + Phase 111 gates (90s), Phase 111 rejection-table update
(`riscv64-linux` accepted; new invalid combos pinned), `go vet`, `go build`.

## F. Boundaries (documented, NOT defects)

- riscv64-linux: parse + error paths only; no backend builds it.
- macOS: clang-only; no GNU/MSVC by design.
- Struct field resolution is declaration-directed (ambiguous same-name/
  different-index fields → K108); type-directed lowering is future work.
- Component envelope is structural (no canonical lift/lower); wit stubs
  cover types only (no call lowering).
- `target.os`/`target.arch` in-language builtins remain post-111 Planned.
