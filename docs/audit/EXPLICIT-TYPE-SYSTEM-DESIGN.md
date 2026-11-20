# Karkain — Explicit Type System Redesign: Authoritative Design (Stage A)

- **Date:** 2026-11-18
- **Status:** **STAGE A DESIGN (authoritative for the redesign being executed)**
- **Owner authorization:** "KARKAIN — COMPLETE EXPLICIT TYPE SYSTEM REDESIGN AND ECOSYSTEM MIGRATION" (2026-11-18), including the **ABSOLUTE VERSION FREEZE** at `1.2.0`.
- **Version freeze:** `VERSION` = `1.2.0`; compiler banners `Karkain Compiler v1.2.0 (Stable Build)`, `Karkain LSP Server v1.2.0 (Stable Build)`, codegen `karkain-compiler v1.2.0 ...`; annotated tag `v1.2.0` → peeled commit `77d70693e1e3134dac312a303bffafa7d50fa988`; **no version change made** (verified 2026-11-18).
- **Baseline this design builds on:** branch `main` @ `b5dcd7ff3748c3bb7778ab6d5885ca4c4771f74c`; clean tracked tree; no tracked modifications; 14 untracked files (owner/user files + task reports), all preserved.

## 1. Mission and scope

The redesign moves Karkain from the generalized, weakly-typed `let`-based declaration
surface to an **explicitly typed, statically checked** language:

1. Explicit primitive types are the normal declaration mechanism (`byte`, `short`,
   `int`, `long`, `float`, `double`, `bool`, `char`, `string`).
2. Strongly typed arrays are first-class (`int[]`, `double[]`, `string[]`, `bool[]`,
   custom-type arrays).
3. User-defined types (`struct`, `enum`) with typed fields, params and returns.
4. Explicitly typed constants (`const int MAX = 100`, `const string VERSION = "1.2.0"`).
5. Typed functions (`func add(int a, int b) -> int`) and typed globals/locals/params/returns.
6. Consistent semantics across Go backend, native backend and self-hosted kcc.
7. Migration policy for the existing `let`/`var`/`const` surface, without introducing a
   generalized `var`.

**Non-goals:** this stage does not implement any of the code changes; it is the
authoritative design. Implementation stages (B–G) follow with their own baselines.

## 2. Baseline measurements (evidence, not recollection)

All findings below were read from the repository on 2026-11-18.

### 2.1 There is ONE declaration path, not three

- `parseVarDecl` (`pkg/parser/parser.go:441`) branches only on the keyword to set
  `isConst`. `let`, `var` and `const` all produce the **same** node type
  (`VarDeclStmt`: `Name`, `Value`, `Type string`, `Const`, `IsMatrix`, `IsSIMD`, ...).
- Consequence: the type annotation is a **string** (`Type string`) — no type value, no
  unification, no lattice.
- Consequence: `let` does not mean immutable; only `const` is immutable (Phase 112).
  `let`/`var` are indistinguishable downstream.
- Consequence: the most-read part of the language (`let`) is currently decorative w.r.t.
  mutability.

### 2.2 Gaps that the design must confront (cross-referenced)

| Gap | Source | Consequence |
|-----|--------|-------------|
| No type values | `TYPE_SYSTEM_AUDIT.md` | Annotations compared textually; `int` vs `i64` unknowable |
| No whole-program type checker | AR5 / scorecard §2.2 | Definite assignment not checkable |
| No inference | `TYPE_SYSTEM_AUDIT.md` | Every annotation hand-written |
| Generics constraints unchecked | Phase 146 | `f[T](x)` accepts any `T` |
| `let`/`var` carry no meaning | `UNIFIED-TYPE_DECLARATION-DESIGN.md` | Reassignment of a `let` compiles today |

### 2.3 Prior design position

- `docs/audit/UNIFIED-TYPE_DECLARATION-DESIGN.md` (written 2026-10-15, Workstream D of the
  master plan; **DESIGN PROPOSAL - not approved, not implemented; owner decision required**):
  - Option 1 (three keywords each with distinct meaning) — **rejected for v1.x** (breaking;
    frozen corpus reassigns `let`).
  - Option 2 (one keyword + explicit mutability annotation; `var` deprecated alias) —
    **recommended**; adds `let mut` / optional mutability + optional type annotation.
  - Option 3 (single `decl x: T = v [mut]`) — **rejected** (`[N]T` SIMD collision).
  - Design position: mutability orthogonal to type annotation; both optional; `let mut
    total: int = 0`; `let items: []string = []`.

## 3. Type catalogue and representation decisions

### 3.1 Primitive types

| Type | Width | Signed | Min/Max | Literal | Notes |
|------|-------|--------|---------|---------|-------|
| `byte` | 8 | unsigned | 0–255 | `54u`? → Karkain integer literal forms | choose representation |
| `short` | 16 | signed | −32 768–32 767 | | |
| `int` | 32 | signed | ±2 147 483 647 | | |
| `long` | 64 | signed | ±9.22e18 | | |
| `float` | 32 | | | | IEEE-754 binary32 |
| `double` | 64 | | | | IEEE-754 binary64 |
| `bool` | 1 | | | `true`/`false` | |
| `char` | 32 | | | `'A'` | character literal |
| `string` | UTF-8 | | | `"..."` | first-class |

**Decision:** Karkain-precise semantics are chosen, not blindly Java semantics. Widths and
representation must be identical on Go and native backends; native uses the same units as the
Go backend (int = 1 unit / 8 bytes, string = 2 units, etc.).

### 3.2 Type semantics rules (cross-backend)

- Arithmetic, comparison, overflow, conversion, default initialization must agree between the
  Go and native backends; native must not diverge.
- Overflow behavior: **decision required** (see §7, decision D-4).

### 3.3 Strings

- Representation: `(pointer, length)` pair — two units, matching Phase 150 Value model.
- First-class: not an integer/object workaround.
- Equality: content-based; length check first; slicing is a view; concat allocates in arena;
  indexing uses `asciiVal(substr(s, i, 1))`; UTF-8 handling per existing implementation.
- Mutability: immutable binding semantics; concat allocates new.

### 3.4 Typed arrays

- `int[]`, `double[]`, `string[]`, `bool[]`, custom-type arrays; element kind derived from the
  element type; array value = `(base, len)` header + compile-time-sized element area; bounds
  checked; `len`; for-in; push; maps/structs per Phase 150 work.

### 3.5 User-defined types

- `struct` with fixed layout (int/float 1 unit, string 2, alignment); `enum` with tag-only
  patterns + `=>`; fields, init, arrays of custom types, params/returns.

### 3.6 Constants

- `const <type> <name> = <literal>`; compile-time evaluation; immutability; validation.

## 4. Declaration grammar and semantics decisions (Stage A)

### 4.1 Target declaration surface (new official programs)

```kark
byte age = 54;
short count = 1000;
int population = 500000;
long distance = 9876543210;
float temperature = 36.5;
double balance = 12500.75;
bool active = true;
char grade = 'A';
string name = "Karkain";

int[] numbers = [10, 20, 30];
double[] prices = [10.5, 20.75];
string[] names = ["Alpha", "Beta"];

const int MAX_RETRIES = 5;
const string VERSION = "1.2.0";

struct Employee { int id; string name; double salary }
Employee employee;

int add(int a, int b) { return a + b; }
func greet(string name) -> string { return "Hello, " + name; }
```

### 4.2 Mutability and reassignment semantics

- `const <T> <name> = <lit>`: immutable; reassignment → diagnostic.
- `let <T> <name> = <lit>`: **immutable** after initialization (design decision adopted from
  Option 2; changes existing `let` meaning — the migration hazard, owner decision D-3).
- `let mut <T> <name> = <lit>`: mutable.
- `var` parses, treated as `let mut` for one minor version, deprecated.
- Type annotation optional on `let`/`let mut` (defaults by literal inference); required for
  explicitness in new official programs.

### 4.3 Initialization and definite assignment

- New programs: declarations must be initialized (or use explicit `uninit` form) — decision
  pending (Stage C). Reading before init → new diagnostic.

## 5. String and array semantics (summary of adopted design)

- String: `(ptr,len)` two-unit value; content equality; concat via arena; slice = view;
  bounds; UTF-8.
- Arrays: `(base,len)` header + compile-time-sized element area; bounds check; `len`; for-in;
  push; custom element kinds.

## 6. Migration policy and `let` compatibility

1. New official Karkain programs use explicit types.
2. Existing programs receive a documented migration path.
3. Compatibility behavior defined explicitly.
4. If `let` is deprecated → diagnostics + migration strategy.
5. Any eventual removal respects release policy.
6. No `var` replacement.

## 7. Stage A decisions and open owner decisions

- **D-1:** Adopt Option 2 (mut annotation) as the transitional mutability mechanism; adopt
  explicit types as the new official surface.
- **D-2:** `let` becomes immutable by default (owner decision D-3 — largest risk; frozen corpus
  contains `let x = 1; x = 2; return x`).
- **D-3:** Migration boundary for `let` reassignment programs (version-boundary or corpus
  migration).
- **D-4:** Overflow and definite-assignment semantics (implementation-time decisions).

## 8. Stage A exit condition

Stage A is complete when this authoritative design is written, reviewed against the baseline,
and closed with positive and negative examples.

## 8.1. Stage A positive and negative examples

**Positive (target surface — must type-check and lower):**

```kark
const int MAX_RETRIES = 5;
const string LANGUAGE = "Karkain";

func add(int a, int b) -> int { return a + b; }

struct Employee { int id; string name; double salary }
Employee employee;

int[] numbers = [10, 20, 30];

func main() -> int {
    let mut total: int = 0;
    for x in numbers { total = add(total, x); }
    return total;
}
```

**Negative (target surface — must be rejected with precise diagnostics):**

```kark
func add(int a, string b) -> int { return a + b; }      // int + string -> type error (binary op type mismatch)
func add(string a, string b) -> int { return a + b; }  // string + string is string, not int -> return-type mismatch
let mut count: byte = 70000;                            // byte out of range -> literal/width error
int[] bad = [1, "two"];                                 // array element type mismatch -> type error
let count2: int = 1;
count2 = 2;                                              // `let` reassignment -> immutable-assignment error
```

## 9. Stage B — compiler pipeline dependency matrix (authoritative assessment)

The redesign must change, in dependency order, the pipeline below. Columns: layer, current
implementation location, change required for explicit types, risk.

| # | Layer | Current location | Change required for explicit types | Risk |
|---|-------|------------------|----------------------------------|------|
| 1 | **Lexer** | `pkg/lexer`, `src/compiler/lexer.kark` | Token kinds for `byte/short/int/long/float/double/bool/char/string`, `[]`, `mut`, `const <T>` | low–medium |
| 2 | **Parser / AST** | `pkg/parser/parser.go` (`parseVarDecl`), `pkg/parser/ast.go` (`VarDeclStmt.Type string`, `Const`); mirrors in `src/compiler/parser.kark`, `src/compiler/ast.kark` | Replace `Type string` with a typed type representation; add typed array/struct/enum nodes; keep the `const` flag; mirror in kcc | **high** (single declaration path must not fork; both engines must stay byte-parity) |
| 3 | **Semantic analysis / checker** | `pkg/sema/resolve.go`, `pkg/sema/*`, `src/compiler/checker.kark` (K101–K115) | Type resolution from typed records; definite assignment; mutability; assignment compatibility; numeric conversions; new diagnostics | **high** (whole-program checker + new diagnostics) |
| 4 | **IR / HIR / SSA** | `pkg/ir/hir`, `pkg/ir/ssa`, `pkg/ir/ssa/passes_*` | Type-carrying IR; typed array/struct/enum types; string type | medium |
| 5 | **Go backend codegen** | `pkg/codegen/codegen.go`, `pkg/codegen/*` | Width-specific lowering (byte/short/int/long, float/double, bool, char, string), typed arrays, structs, enums, constants | **high** (byte-identity pins) |
| 6 | **Native backend** | `pkg/native/program.go`, `src/compiler/native_*.kark`, `src/compiler/native_value.kark` | Same units; string = 2 units; array `(base,len)`; record = address of field area; arena | **high** (native qualification + gates) |
| 7 | **Self-hosted kcc** | `src/compiler/*` (lexer/parser/sema/codegen/native_*) | Mirror the above in Karkain; native value model `native_value.kark`; parity gates | **critical** (self-hosting must not fork) |
| 8 | **Runtime** | `runtime/`, `pkg/codegen/*runtime*` | Numeric widths, string, array representations consistent | medium |
| 9 | **Standard library** | `stdlib/`, `pkg/sema/resolve.go` builtin tables, `src/compiler/sema.kark` | Typed declarations; builtin tables updated | low–medium |
| 10 | **Tests / fixtures / examples / docs / CI** | `pkg/...` tests, `examples/`, `conformance/`, `docs/source/**`, `.github/workflows/ci.yml` | Migrate/mirror; gates | medium |

**Order of dependency (must-change chain):** Lexer → Parser/AST → Checker/Sema → IR → Go
backend → Native backend → Self-hosted kcc mirror → Runtime → Stdlib → Ecosystem
(tests/examples/docs/CI). Self-hosting parity is the last and load-bearing gate.

## 10. Stage C - implementation stages (A-G) and gate chain

The redesign is delivered as seven implementation slices, in the order above, each with its own
baseline note, gate file and regression run; no slice starts before its predecessors' gates are
green.

- **Stage C1 — Lexer token kinds** (Go + kcc mirror): `byte`, `short`, `int`, `long`, `float`,
  `double`, `bool`, `char`, `string`, `[]`, `mut`, `const <T>`.
- **Stage C2 — Typed declaration parsing** (Go + kcc): replace the `VarDeclStmt.Type string`
  textual annotation with a typed type value; typed arrays (`int[]`), typed struct/enum decls;
  keep the single declaration path; both engines byte-parity on parse errors.
- **Stage C3 — Checker / sema typed declarations**: type resolution from typed records,
  definite assignment, mutability enforcement, assignment compatibility, width-aware arithmetic
  and literal ranges; new diagnostics (existing K1xx namespace plus any new code, stated).
- **Stage C4 — Go backend lowering** (+ typed IR): width-specific lowering, typed arrays,
  structs, enums, constants; byte-identity pins re-measured, not regenerated.
- **Stage C5 — Native backend + kcc native driver**: same units; string = 2 units; array
  `(base,len)`; record = address of field area; arena; native goldens re-pinned.
- **Stage C6 — Self-hosted kcc mirror**: lexer/parser/sema/codegen/native in Karkain; KIR pin
  re-measured; self-host `check` clean; no fork.
- **Stage C7 — Stdlib / runtime / ecosystem / docs**: typed stdlib declarations, runtime
  representations, examples/conformance migration, docs.

## 11. Stage D - regression protection

- Every existing gate that pins `let`-era behaviour is the regression baseline for the slice
  that touches its surface; behaviour-preserving slices re-run them unchanged.
- The redesign's own gates (per slice) are differential, not golden-only, where the divergence
  risk is frame/byte related (following the Phase-151 four-layer differential discipline).
- Mutation-verification is mandatory for each new check that could pass vacuously.
- Full-suite runs in isolation per the ~4 GB host rule; no gate may silently skip on empty
  output.

## 12. Stage E - documentation deliverables

- `SPEC.md` sections 2 (Types), 3.3 (Variables), 3.2 (Functions), appendix keyword table.
- `docs/source/language/*.rst` (types, variables, functions, structs, enums, strings,
  collections, constants) + reference + capability summary + known-gaps.
- Sphinx `-W` clean-room build with 0 warnings and green linkcheck before any docs merge.

## 13. Stage F - final report sections A-I

The final report carries, in order: (A) executive summary, (B) baseline and measurements,
(C) design decisions and rationale, (D) implementation stages delivered, (E) regression
evidence, (F) documentation deliverables, (G) ecosystem migration status, (H) open owner
decisions, (I) version-freeze and release-readiness statement.

## 14. Post-design implementation status (2026-11-19)

The type-first surface described here has been implemented in both compiler engines:
`int x = 42`, `int[]`, `int[][]`, `map[K]V`, `const int MAX = 100`, and typed function
signatures. Public primitives are `byte`, `short`, `int`, `long`, `float`, `double`,
`bool`, `char`, `string`; `float64` is a documented legacy alias. Acceptance programs
A–E execute green on both engines; 156 `.kark` files are migrated. Remaining generalized
declarations and partial kcc argument checking are recorded in the final report.
`VERSION` remains `1.2.0`. Repository-wide pass converted 3,189 declarations; 987 genuine legacy declarations remain for documented design-level reasons (heterogeneous compiler-internal values and undeclared stdlib return types), so final verdict is PARTIALLY COMPLETE.

## 15. Strict restrictions during the redesign

1. **Version freeze is absolute**: `VERSION`, all banners, generated headers, LSP version and
   every version-pinning test stay at `1.2.0`; no tag, release, package, Pages deploy or docs
   version bump is made from this work.
2. No commit and no push without explicit owner approval; no branch/merge operations.
3. Stop and report to the owner if any step appears to require a version change.
