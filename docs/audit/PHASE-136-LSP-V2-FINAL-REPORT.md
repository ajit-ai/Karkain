# Phase 136 — LSP v2 (Semantic IDE Experience) — Final Report

**Verdict: COMPLETE.** The language server grew from diagnostics-only into
a semantic editing experience: lexer-driven semantic tokens, typed hover,
scoped go-to-definition (same-document + cross-module), scoped completion
with member completion — all served by the real `karkain lsp` stdio binary
and wired into the VS Code extension via a language client.

## 1. What changed

| File | Change |
|---|---|
| `pkg/lsp/semantic.go` (new) | `textDocument/semanticTokens/full`: lexer-driven encoder (keywords/strings/numbers/operators/variables), gap-recovered comments, byte-Col→UTF-16 conversion, delta encoding; total on all inputs |
| `pkg/lsp/scope.go` (new) | Queryable scope model: top-level declarations (globals), params + let/var with brace-matched block scopes, struct/enum member tables, annotation-then-inference types (literals, scope-aware binary/unary/identifier refinement pass) |
| `pkg/lsp/definition.go` (new) | Deterministic definition resolver: same-doc lexical → struct/enum member → module files (open docs sorted, disk siblings, declared deps); bare imports jump to the module file; unknown → null |
| `pkg/lsp/handler.go` | Real symbol ranges (replacing line-0 placeholders) for all 12 decl kinds + new enum coverage; hover/definition/completion rewired to the model; semantic capability + handler; server version 0.18.0 → 1.0.0; dead `buildSymbolIndex` removed |
| `pkg/lsp/protocol.go` | Semantic-tokens method/types/legend/options + capability; `CompletionKindEnumMember = 20` (spec-correct) |
| `pkg/lsp/server.go` | Per-document scope-model cache (built at parse, cleared at close) |
| `editors/vscode/extension.js` | LanguageClient bootstrap (`karkain lsp` stdio, guarded require); `deactivate` stops it; `node --check` clean |
| `editors/vscode/package.json` | Declares `vscode-languageclient ^9.0.1` |
| `editors/vscode/README.md` | Intelligence wired (was "planned"); client install requirement |
| `docs/source/tools/lsp.rst` | semanticTokens row; version + client-integration paragraphs corrected |
| Gates | `pkg/lsp/semantic_test.go` (4), `scope_test.go` (5), `definition_test.go` (5), `completion_test.go` (5), `pkg/cli/lsp_stdio_test.go` (stdio E2E through the real binary), `vscode_extension_test.go` +1 wiring test |

Deliberate boundaries: no `pkg/sema` dependency (the resolver is a
diagnostics engine with no per-position query API — documented deviation
from the roadmap's "reuse sema" line); no visibility enforcement on jumps
(navigation, not checking); completions never rank (filter only); dep-file
models parse per request (no cross-request cache — noted below).

## 2. Defects found and fixed during the phase

1. **Line-0 placeholder symbol ranges** — every definition jumped to line
   0. Fixed with parser spans (native Col/EndCol or keyword-derived).
2. **Unfiltered symbol append in completion** — typing `pr` offered every
   document symbol. All sources now prefix-filter.
3. **Map-order nondeterministic cross-document definition** — same name in
   two open docs resolved randomly. The global search is deleted;
   resolution is current-doc-first, then sorted open docs, then disk.
4. **STRING span excludes quotes; Col points at the quote** (lexer quirk)
   — positions derive from byte offsets uniformly; spans widen to quotes
   so gap recovery never mislabels `"` as comment.
5. **Function scope ends** — first cut used next-decl-or-EOF, visibly
   wrong past `}` (caught by the shadowing test). Scopes are now
   brace-matched over lexer tokens (string/comment-proof).
6. **Stale server version** 0.18.0 → 1.0.0.

## 3. Observations reported, not changed (out of scope)

- Top-level `let` is a **parse error** (`unexpected token 'let' at top
  level`), not a silent drop — verified live. The scope model (no binding)
  is therefore correct; no parser change.
- `karkain lsp` blocks on open stdin after the `exit` notification and
  exits 0 on EOF; the stdio gate closes stdin per the standard lifecycle.
  An explicit `os.Exit` on `exit` would be more spec-literal — deferred.
- Cross-request caching of parsed dependency models (completion/definition
  reparse dep files per request) — fine at gate scale; profile before
  optimizing.
- `print` lexes as a PRINT keyword (hover/completion categorize it as
  such) — pinned by tests, not a defect.

## 4. Gates — all green (actually ran)

- `pkg/lsp` full suite: 11 pre-existing + 19 new = 30/30 PASS (1.2s).
  New pins: exact token goldens incl. UTF-16, exact decl spans
  (`0:5-11`, `0:12-17`), shadowing + scope-end line, typed hover
  (`let doubled: int`, `n: int`), member hover, cross-file jumps, bare
  imports, 25× no-leak loop, disk-sibling fallback, prefix-filter-everything,
  member-only-after-dot, broken-file safety.
- `pkg/cli`: `TestLSP_StdioEndToEnd` PASS 9.4s (initialize capabilities,
  diagnostics push drained, tokens/hover/definition/completion asserted,
  clean exit 0) + 3/3 VS Code extension tests PASS.
- `go build ./...`, `go vet ./...` clean; Sphinx `-W` green (lsp.rst);
  `node --check` + JSON parse green; new files gofmt-clean (`gofmt -l`
  on touched files is the repo-wide CRLF artifact).
- Untouched by construction (no compiler/parser/sema/CLI-pipeline
  changes): conformance, probes, Phase 114/134/135 gates.

## 5. Roadmap movement

GA-2 tooling track: 136 lands. The `scope.rst` LSP row can move toward
Stable Core. Next: 137 (kcc parity) — its split-flow fallback surfaces
are untouched. No 137 code in this phase.
