# PHASE 79 — SELF-HOSTING READINESS AUDIT (P79-10)

Purpose: assess whether current compiler components can converge toward a
compiler written in Karkain. This is a readiness assessment, NOT an
implementation instruction.

---

## Current Self-Hosted Compiler (`src/`)

| File | Lines | Component |
|------|-------|-----------|
| `src/lexer.kark` | 525 | lexer (Karkain) |
| `src/parser.kark` | 668 | parser |
| `src/codegen.kark` | 1477 | codegen |
| `src/sema.kark` | 407 | semantic analysis |
| `src/ast.kark` | 796 | AST definitions |
| `src/main.kark` | 318 | driver |
| `src/compiler/*.kark` | (subset) | earlier self-host iteration |
| `src/main.rs` | 3 | (placeholder, not the bootstrap host) |

Duplicated/partial: `src/analyzer.kark` is a 1-line placeholder.

**Operational status:** `pkg/bootstrap` tests FAIL
(`TestBootstrap_Stage1Compilation`, `TestBootstrap_Stage2SelfHosting`,
`TestBootstrap_BitwiseIdentity`). The 3-stage bootstrap is NOT currently
verifiably green. HIGH confidence — this is a measured test result, not an
estimate.

---

## Component Classification

### CATEGORY A — Portable concepts (present, algorithm-portable)
- Lexer tokenization (`src/lexer.kark`, `pkg/lexer`)
- Parser/AST construction (`src/parser.kark`, `src/ast.kark`)
- Borrow/scope model (`pkg/sema/borrow_checker.go`)
- SSA lowering & folding (`pkg/ir/ssa`, `pkg/codegen/lower.go`)

### CATEGORY B — Require Karkain runtime support
- Dynamic collections (map/array) — used pervasively in parser/sema/IR
- Strings + string escaping (already partially used by self-host)
- File I/O (`os`-level; `getArgs`/argc/argv wired in self-host per commit history)
- Memory management (no GC; ownership/borrow model + escape→stack)
- Error handling (Result/Option/`?` — now functional after BUG-4/2/7)

### CATEGORY C — Host-language (Go) dependent today
- `pkg/pm`, `pkg/lsp` (JSON-RPC), `pkg/bootstrap` harness — heavy stdlib/Go
  usage; not needed for the core compiler port.
- Codegen string-escape utilities and C name sanitization (portable, but Go-concise).

### CATEGORY D — Bootstrap-only mechanisms
- `pkg/bootstrap` (multi-stage + SHA-256 parity proof)
- `src/main.rs` stub
- The `restricted` vendor capabilities / adapter model is runtime, not bootstrap.

---

## Readiness Scoring Methodology

Define measurable categories with weights (sum 100):

| Category | Weight | Rationale |
|----------|--------|-----------|
| Language feature coverage | 30 | compiler needs enums/matches/borrow/strings/collections |
| Compiler component portability | 30 | how much of lex/parse/emit is written in Karkain |
| Runtime prerequisites | 20 | strings, collections, files, memory |
| Standard library readiness | 10 | core libs the compiler relies on |
| Bootstrap dependency reduction | 10 | remaining Go//main.rs reliance |

**Because evidence is partial (self-hosted components exist but the bootstrap
is not green, and language-feature/runtime readiness lacks a measured matrix in
this repo), I do NOT assign a single numeric percentage.** Per the prompt's
"no invented percentage" rule:

```text
Score: NOT CALCULABLE from current evidence as a single number.
Qualitative: component portability is well underway; runtime + feature
coverage are unverified; bootstrap is not yet green.
Confidence: HIGH (on the non-green status), MEDIUM (on component portability),
LOW (on a final readiness number).
```

---

## Major Self-Hosting Blockers (ranked)

1. **Stage1/Stage2 bootstrap is not green** (`TestBootstrap_Stage1Compilation`,
   `TestBootstrap_Stage2SelfHosting` fail) — the correctness proof fails.
2. **Bitwise parity test fails** (`TestBootstrap_BitwiseIdentity`) — the
   reproducibility/soundness gate is not met.
3. **`Src/analyzer.kark` placeholder** — semantic-analysis port incomplete.
4. **Runtime prerequisites** (full stdlib strings/collections/files/memory)
   lack a measured coverage matrix.

---

## Recommendation

Self-hosting is **underway but NOT readday for production bootstrap.** It is
not a Phase 80 (SIMD) prerequisite. If self-hosting is to become a near-term
goal, the two bootstrap tests and the bitwise-parity gate are the concrete
exit criteria to fix first.