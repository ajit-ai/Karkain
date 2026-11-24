# v1.2.6 — native `sha256_hex` — Baseline and Record

**Verdict: COMPLETE (gate green).**

## 1. The red case and root cause

The v1.2.6 gate `TestPhaseV126_ExecutedPEWithExactBytes` was RED on the case
`mixed_concat_sha`: `let a = sha256_hex("abc") let b = a + "x" print(sha256_hex(b))`.
Error: `error[K145] native target: sha256_hex() requires a string argument`.

Root cause: in `natNativeStrKind` (`src/compiler/native_value.kark:7898-7918`),
the `NODE_IDENT` arm returns 1 only if the slot kind is `natKindString()`.
The slot kind is set by `natNativePlanDecls` (`src/compiler/native_value.kark:3961+`),
which for `let b = a + "x"` found no matching arm (only `NODE_ARRAY_LIT`,
`NODE_INDEX`, `NODE_STRUCT_LIT`, `NODE_CALL` set kinds), so `b`'s slot stayed
at its default `natKindInt()` (line 3967). `natNativeStrKind(b)` → `-1` →
refusal at `native_value.kark:4490` (twin at `:6104`).

## 2. The fix (2 lines added, no Go changes)

`src/compiler/native_value.kark:3979-3988` — new `NODE_BINARY()` arm in
`natNativePlanDecls`:

```karkain
if (nodeType(v) == NODE_BINARY()) {
    if (binaryOp(v) == "+" && natNativeStrKind(binaryLeft(v), stmts, plan) == 1) {
        k = natKindString()
    }
}
```

Mirrors `natNativeStrKind`'s own `+` case (`native_value.kark:7904-7908`).
When `a` is planned first with kind string (`sha256_hex` sets `k` at
`native_value.kark:3999`), the `NODE_BINARY` arm sets `b`'s kind to string.

## 3. Gate evidence

- `go test ./pkg/cli/ -run 'TestPhaseV126_ExecutedPEWithExactBytes/mixed_concat_sha' -count=1 -timeout 30m -v` → `--- PASS: TestPhaseV126_ExecutedPEWithExactBytes/mixed_concat_sha (2.87s)`
- Full gate: `go test ./pkg/cli/ -run 'TestPhaseV126_' -count=1 -timeout 30m` → `ok karkain/pkg/cli 284.361s`
- `phase95KCC(t)` (used by the gate) builds a fresh kcc from the current
  `src/compiler/*.kark` (reads the dir, copies `.kark` files to a sandbox,
  `karkain build main.kark`) — the fix is exercised, not inherited from a
  stale binary.

## 4. Commit and PR

- Commit on `develop`: the fix + this audit note (all files listed below).
- PR: `develop` → `main` (owner reviews and merges per AGENTS.md cadence).

## 5. Non-claims

- KIR pin NOT repinned: the native driver consumes the parsed AST directly
  (the KIR does not render the driver — per Phase 151A-1).
- No byte-identity with the Go native backend: the v1.2.6 gate compares
  executed stdout, not image bytes (documented design: kcc materialises the
  K table in the helper frame vs Go's rodata).
