# NPU Targeting (`@target`)

Phase 98 integrates NPU execution into the Karkain compiler through a
`@target(...)` function attribute. The design principle is unchanged from the
Math/Tensor/NPU chain (Phases 71–78): **the NPU is a backend — not the
foundation, not the language.**

A `@target(npu)` function always compiles and always produces correct results,
on any machine, with or without an accelerator. There is no NPU/SDK/driver/cloud
dependency.

## Syntax

`@target(...)` prefixes a `func` declaration at the top level:

```karkain
@target(npu)
func matmul(a int, b int) {
    // ...
}
```

Supported targets: `cpu` (the default pipeline) and `npu`. A plain `func`
without the attribute runs the normal CPU path and is entirely unaffected.

An unknown target is a semantic error:

```
karkain check prog.kark
error[K004]: @target(tpu_turbo): unknown execution target; supported targets: cpu, npu
```

## Pipeline

1. **Parser** attaches the attribute to the `FuncDecl` node
   (`FuncDecl.Target`, `parseTargetAttr`). Placement is enforced here:
   `@target(...)` may only precede a `func`.
2. **Semantic analysis** (`pkg/sema/npu_check.go`) rejects unknown/unsupported
   targets with `error[K004]` — same brand, exit 3, on both engines.
3. **C codegen** emits the attribute as an inert `// @target(<name>)` comment
   marker before the function's C definition. It never emits an executable C
   statement, so the generated program needs no NPU runtime.
4. **Runtime dispatch** (`pkg/codegen/npu_compiler.go`): `@target(npu)`
   functions route their body's NPU-capable operations through the
   `NPUDispatcher`.

## The NPU Dispatcher

`NPUDispatcher` decouples language semantics from accelerator availability:

- `Available()` — whether a vendor backend (`intel`, `qualcomm`, `apple`,
  `amd`, `arm`) was detected.
- `Capabilities()` — the active backend's capability model, or the CPU
  reference capabilities when no NPU is present.
- `Dispatch(DispatchRequest)` — routes an operation to the accelerator when
  available **and** it returns computed values; otherwise it uses the CPU
  reference backend (`pkg/backend/cpu`).

Matrix multiplication (`OpMatMul`) is the first concrete NPU operation. The
dispatcher builds the Karkain-owned Tensor IR graph `C = A @ B`, compiles it for
the active device, and executes.

## Mandatory CPU Fallback

The CPU backend is the **correctness oracle**. Every accelerator path must match
its output. The dispatcher falls back to CPU whenever:

- no vendor adapter is detected (typical development machine),
- the adapter reports no hardware,
- the adapter lacks a vendor runtime and stubs out (returns no computed
  values), or
- the adapter's compile/execute errors — these are environmental (driver, SDK,
  device contention), never caller errors.

Caller-input problems (invalid dimensions, operand size mismatch, unsupported
operation) are hard errors and always rejected.

## Autodiff / Math / Tensor parity

NPU ops run on the same tensor IR as the CPU reference backend, so
`@target(npu)` functions are drop-in correct: `A @ B` via an NPU produces
bit-identical values to the CPU path for the same inputs (verified by the
`pkg/codegen/phase98_npu_test.go` mock-backend tests), matching the Autodiff
integration from Phase 74.

## Extending with a new accelerator

Adding a vendor never touches language, grammar, or pipeline code. Supply
another `npu.NPUBackend` implementation (hardware detection + capability + a
`Compile`/`Execute` that returns real computed values) and register it in
`standardAdapters()`.

## Supported API

- New execution targets: extend `TargetNames` in `pkg/sema/npu_check.go`.
- New NPU operations: add an `NPUOperation` const and a `dispatch<Op>` method
  on `NPUDispatcher`.

## Files

- `pkg/parser/parser.go` — `parseTargetAttr`
- `pkg/sema/npu_check.go` — target validation
- `pkg/codegen/codegen.go` — `// @target(...)` C marker
- `pkg/codegen/npu_compiler.go` — `NPUDispatcher`
- `src/compiler/{parser.kark,ast.kark,codegen.kark}` — self-hosted support
- `pkg/cli/{checker.go,lint.go,kcc_engine.go}` — CLI integration + kcc preflight

## Full report

See `docs/audit/PHASE-98-FINAL-REPORT.md` for the complete Phase 98 acceptance
record.