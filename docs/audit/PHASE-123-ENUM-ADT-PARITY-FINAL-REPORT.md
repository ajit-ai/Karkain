# PHASE 123 FINAL REPORT — LANGUAGE CORE COMPLETION / ENUM ADT PARITY

Status: **COMPLETE**
Date: 2026-09-15
Gate: `pkg/cli/phase123_cli_test.go`

Phase 123 finishes the language core: enums and `match` are now fully
byte-identical on the Go reference front end and the self-hosted `kcc`
engine, with checker validation behind both. The phase root-caused TWO
self-hosted checker defects that left enum diagnostics dead on arrival, then
pinned the corrected behavior with permanent corpus, negative fixtures, a
conformance file, a dedicated gate, and docs.

---

## 1. ROOT-CAUSE FIXES (`src/compiler/*.kark`)

### 1.1 Match arms were never checked (statement-position `match`)

`parseBlock`'s else branch wraps a bare `match value { ... }` statement into
`ExprStmt(Match(...))` (`src/compiler/parser.kark`). `checkStmt` has a `Match`
branch, but expression-statement matches arrive as `ExprStmt` containing a
`Match` node — and `checkExpr` had NO `Match` case. Result: match-arm patterns
were never type-checked; every invalid arm slipped through `check` with `[ok]`.

Fix: `checkExpr` (in `src/compiler/checker.kark`) now has a `if (nt == "Match")`
case mirroring the `checkStmt` branch — it validates the `matchValue`
expression, walks each arm's pattern values and bodies, and maps
`EnumName.Variant` arms through the enum registry so invalid tags are
reported. `error[K113] unknown enum variant` and `error[K106] use of undefined
enum type` now fire in BOTH match arms and expressions.

### 1.2 Member access on enum bases never resolved (NODE "Dot" vs "Member")

The checker dispatched enum-expression validation on node type `"Dot"`, but
`getNodeType(NODE_DOT)` returns `"Member"` (not `"Dot"`) — so
`MissingKind.Purple` in an expression was never validated either. Fixed the
dispatch string to `"Member"`. This closes the previously-documented
member-access gap: an expression like `MissingKind.Purple` now raises
`error[K102] undefined identifier 'MissingKind'` on kcc, matching the Go
resolver's K002.

---

## 2. VERIFICATION

### 2.1 Negative diagnostics (kcc driver)

| Fixture | Expected | Observed |
|---------|----------|----------|
| `Color.Purple` in match arm | `error[K113] unknown enum variant 'Color.Purple' in match arm` | identical |
| undefined enum in match arm | `error[K106] use of undefined enum type 'Missing' in match arm` | identical |
| undefined enum in expression | `error[K102] undefined identifier 'Missing'` | identical |
| `Shape.Circle(r) =>` (payload destructuring) | `expected '=>' in match arm, got '('` | identical on Go + kcc |

### 2.2 Programme parity (byte-identical Go ↔ kcc goldens)

- `13_enums.kark`: `1 / 1 / 100 / 200 / 300 / 0`
- `14_adt_match.kark`: `10 / 20 / 30 / 1`
- `conformance/012_enums_test.kark`: 5 `test_*` functions — `5 passed` on both
  engines.

### 2.3 Documented intentional strictness (parity-of-behaviour, not a defect)

kcc rejects unknown enum types/variants at CHECK time (K102/K106/K113); the Go
resolver deliberately defers those two to the C compiler, so `go check` passes
and the downstream build fails with the undeclared tag. Both engines reject the
programs. Match-arm BODIES remain semantically unchecked on BOTH engines (e.g.
`_ => 2 + nope` passes check and fails only at C compile) — this is parity and
is kept.

---

## 3. PERMANENT CORPUS & GATES

- `examples/01-fundamentals/13_enums.kark`, `14_adt_match.kark` — pinned in
  the Phase 114 gate (map count 49 → 51).
- `conformance/012_enums_test.kark` — both-engine conformance tests.
- `examples/type_errors/err15_unknown_enum_variant_match` (K113),
  `err16_undefined_enum_match_arm` (K106), `err17_undefined_enum_expr` (K102),
  `err18_unknown_enum_variant_expr` (K113).
- `pkg/cli/phase123_cli_test.go` — 3 subtests: `EnumMatchChecker`
  (positive + all four negative fixtures w/ exit-3 codes), `MatchArmParseErrorParity`
  (payload-destructuring `=>` error driven through the real binary with
  CombinedOutput — Go diagnostics and the Phase 105 preflight both render to
  `stderr` via `renderDiagnostics`, `pkg/cli/check.go:106`), and
  `EnumVariantExpressionKIR` (KIR renders `enum Color (Red:, Green:, Blue:)`
  and `(enum_variant Color Red)` construction nodes).

---

## 4. DOCS & METADATA

- `SPEC.md`: Enum capability row flipped to implemented ✓; §3.5 Match documents
  `EnumName.Variant` tag-only patterns, the `=>` requirement, and K106/K113;
  capability summary gains an Enums row; Known-Gaps enum-payload row now points
  at Phase 123 (payloads recorded but semantically dropped; no payload
  destructuring).
- Example counts swept 50 → 52: `README.md` (3 sites), `examples/EXAMPLES.md`
  (fundamentals 12 → 14 + Total 52 row + two new table entries),
  `docs/source/examples/index.rst` (52 files / 51 pass + 5 skipped),
  `examples/01-fundamentals/README.md` (learning-order rows 13/14 + stale
  wildcard-arm determinism note corrected).
- `AGENTS.md`: Phase 123 completion block + "Current phase: post-123".
- `ROADMAP-PLAN.md` remains UNTRACKED (never committed).

---

## 5. REGRESSIONS

- `go vet ./pkg/cli/` clean.
- All three Phase 123 subtests PASS.
- Phase 99 self-hosted gate still green after checker changes (123.3s).
- `kcc check src/compiler/main.kark` → `[ok]`.
- Tooling note: the Write tool and PowerShell `Set-Content -Encoding UTF8`
  write a UTF-8 BOM (`EF BB BF`) that the Go lexer rejects — fixtures must be
  BOM-stripped (the conformance fixture and negative goldens were written
  BOM-free).