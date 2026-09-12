# Phase 116 — Developer Examples & Real-World Programming Corpus — Final Report

## Scope

Phase 116 turns the Phase 114 example corpus into a **normalized,
documented, validated real-world programming corpus** under `examples/`:

1. 15 canonical categories with **dash-named directories**
   (`01-fundamentals` … `15-developer-tools`) instead of underscore names;
2. honest tri-state status model on every `.kark` file
   (`// Status: Runnable | Experimental | Planned` + `// Engine:` + `// Category:`);
3. 4 new **both-engine runnable** examples (array stack, FIFO queue,
   0/1-knapsack dynamic programming, token-statistics pipeline) pinned
   byte-identical in the Phase 114 gate (45 → 49 goldens);
4. a top-level `examples/README.md` index with a status model table and a
   per-category count matrix;
5. a new corpus-metadata validation gate `TestPhase116` (status/engine/
   category consistency, planned README-only dirs, `.kark`-not-`.kar` rule,
   pin-map ↔ disk coverage, doc-page presence);
6. Sphinx "Karkain by Example" updates (`examples/index.rst`,
   `algorithms.rst`, `data.rst`, `reference/example-matrix.rst`,
   `release-notes.rst`) with `literalinclude` for the new examples;
7. CI `TestPhase116` step; `verify-examples.ps1` dash-directory fix.

Scaled back (deliberately, per the Phase 116 prompt and the earlier plan):
no compiler changes were made — the corpus demonstrates what the language
already does, honestly.

## Corpus status by category

| # | Category           | Files | Runnable (both) | Experimental (go) | Planned |
|---|--------------------|------:|----------------:|------------------:|--------:|
| 01| Fundamentals       | 12    | 12              | 0                 | 0       |
| 02| Algorithms         | 13    | 13              | 0                 | 0       |
| 03| Systems            | 1     | 1               | 0                 | 0       |
| 04| Networking         | 0     | –               | –                 | README  |
| 05| Data               | 4     | 4               | 0                 | 0       |
| 06| Database           | 0     | –               | –                 | README  |
| 07| Web                | 0     | –               | –                 | README  |
| 08| Concurrency        | 2     | 0               | 2                 | 0       |
| 09| AI                 | 2     | 2               | 0                 | 0       |
| 10| Machine Learning   | 2     | 2               | 0                 | 0       |
| 11| Quantum            | 0     | –               | –                 | README  |
| 12| Scientific Computing | 4   | 4               | 0                 | 0       |
| 13| Finance            | 4     | 4               | 0                 | 0       |
| 14| Security           | 4     | 4               | 0                 | 0       |
| 15| Developer Tools    | 2     | 1 (+1 test-mode)| 0                 | 0       |
| **Total**            | **50**| **47 (+1 test)**| **2**             | **4 dirs** |
| Golden-pinned        |       | 47              | +2 (go)           | deliberately excluded |

- 50 `.kark` files (49 corpus programs + 1 test-mode file), **49** of them
  golden-pinned in `pkg/cli/phase114_examples_test.go` (47 both-engine + 2
  Go-engine Experimental concurrency examples).
- Planned categories stay **README-only with zero runnable code** — they are
  never pinned, so they can never "accidentally execute".

## New examples (verified byte-identical on BOTH engines)

| File | Golden output |
|------|---------------|
| `examples/02-algorithms/11_stack.kark` (pointer-based array stack) | `3 30 20 10 0` |
| `examples/02-algorithms/12_queue.kark` (FIFO queue, moving head)    | `10 20 50 20 30 40 50` |
| `examples/02-algorithms/13_knapsack.kark` (2D DP table)             | `7 9` |
| `examples/05-data/04_token_stats.kark` (tokenize/filter/aggregate)  | `9 3 35` |

All four were run through `karkain run --engine go` and `karkain run
--engine kcc` with identical stdout, then pinned in `phase114Examples`
(gate count assertion 45 → 49).

## Files changed

- `examples/*` — renamed 15 category dirs underscore → dash; per-category
  READMEs, `EXAMPLES.md` category map + individual tables updated.
- `examples/README.md` — NEW top-level index (status model, category count
  matrix, run instructions, verification, conventions).
- `examples/02-algorithms/{11_stack,12_queue,13_knapsack}.kark` — NEW.
- `examples/05-data/04_token_stats.kark` — NEW.
- `examples/showcase/` — dirs/content kept on their own underscore
  convention (reverted the collateral dash rename; content self-paths
  consistent; references to the main corpus stay dash).
- `pkg/cli/phase114_examples_test.go` — 4 new pinned goldens; 45 → 49.
- `pkg/cli/phase116_examples_test.go` — NEW: 3 gates
  (`TestPhase116_CorpusMetadata`, `TestPhase116_NoKarFiles`,
  `TestPhase116_ExampleDocs`).
- `pkg/cli/phase115_developer_preview_test.go` — inventory paths updated to
  dash form by the global rewrite (values, not logic).
- `scripts/verify-examples.ps1` — dir filter regex `'^\d{2}_'` → `'^\d{2}-'`.
- `.github/workflows/ci.yml` — new `TestPhase116` gate step.
- `docs/source/examples/{index,algorithms,data}.rst` — corpus counts,
  `literalinclude` for the 4 new examples, "49 pass / 5 skipped" sentence.
- `docs/source/reference/example-matrix.rst` — corpus row + "Stacks, queues
  & DP" capability row.
- `docs/source/release-notes.rst` — v0.115.0 status row + 115/116 milestone
  rows.
- `.gitignore`, `README.md`, several `docs/source/examples/*.rst`,
  `src/*` headers — token-level path updates from the global rename rewrite.
- `AGENTS.md` — roadmap record for Phase 116 (local-only amendment).

## Verification (all green)

- `go build ./...` — PASS.
- `go vet ./...` — PASS.
- `go test ./pkg/cli/ -run 'TestPhase114_CorpusExamples_GoEngine|TestPhase114_CorpusCoverage'` — PASS (98.99 s).
- `go test ./pkg/cli/ -run 'TestPhase115|TestPhase116'` — PASS (116.21 s).
- `go test ./pkg/cli/ -run 'TestPhase114_CorpusExamples_KCCParity|TestPhase114_CorpusTestFiles'` — PASS (114.02 s, self-hosted kcc parity).
- Unit regressions (`pkg/lexer`, `pkg/parser`, `pkg/codegen`, `pkg/pm`) — PASS.
- `scripts/verify-examples.ps1` — **49 passed, 0 failed, 5 skipped**
  (1 test-mode + 4 planned categories).
- Sphinx strict HTML `-W --keep-going` — build succeeded.
- Sphinx `-W` linkcheck — succeeded (only the known-ignored GitHub URL).

## Problems encountered

1. **Slice reassignment on a `var` stack crashed the Go engine's C compile** —
   the stack example was rewritten to a pointer-based stack (no reassign),
   byte-identical on both engines.
2. **Global rename collateral on `examples/showcase/`** — the token rewrite
   touched showcase content (which references the main corpus, correctly)
   and the showcase dirs (which use their own underscore scheme, e.g.
   `10_ml`). Showcase dirs were restored to underscore and content
   self-paths reverted; final state is consistent (corpus dash, showcase
   underscore).
3. **Minor** — new-test compile fix (an `Errorf` format/arg mismatch) and a
   private planned-dir README allowance corrected before the gate was run.
4. `corpus_demo.txt` (generated by `03-systems/01_file_io.kark` runs) is a
   benign artifact of corpus execution — removed; future runs regenerate it.
5. Git shows the 15 dir renames as delete+untracked until `git add -A`
   recodes them as renames (expected pre-commit state; no staging performed).

## Git status

Per the local Phase 116 amendment, the working tree was **not** committed
and **not** pushed. The changes above are staged solely in the working tree
for review.

`No commit created. No push performed.`