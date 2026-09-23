# Phase 142 BASELINE — 1.1.0 Release

Date: 2026-09-23. Target: GA-3 final step (after 141).

## 1. Version machinery

- Authoritative `VERSION` = 1.0.0; Go banners (`cmd/karkain/main.go`,
  `pkg/cli/commands.go`); kcc banners (`src/compiler/main.kark` ×3);
  generated headers (kcc `codegen.kark` ×6; Go quantum/qir/qec/qasm/
  openpulse/dwarf); LSP server version (`pkg/lsp/handler.go`).
- Version-pinning tests: phase88/90 (--version output), 115 (skeleton +
  DocTree), 118 (RC identity), 119 (identity + labels), 120 (envelope),
  bootstrap args_test. All assert live identity → all move together.
- Historical (keep): audit reports, AGENTS history, beta/rc pages, SPEC
  history rows, 1.0 archive names, manifest/registry fixture versions.

## 2. Release ceremony state (from the 1.0 runbook, Phase 128)

- Tag `v1.0.0` on origin (rewritten-history position, ancestor of main).
- No tag cut in-phase: `v1.1.0` cut + CI release job stay owner-only,
  exactly like 1.0.0. This phase prepares everything up to the ceremony
  and rehearses it on the host triple.
- `1.0.x` LTS branch: does not exist yet → create from `v1.0.0`, push.

## 3. Docs honesty debt found in the sweep

- README capability table predates 125A/137/138 (networking/db/web/GPU
  listed Not-Yet-Implemented; concurrency listed Go-only).
- Corpus counts stale (59 vs 61 files; Experimental vocabulary retired
  by 137; 03_mlp_forward missing from totals).
- installation Binaries row vs release-status note tension (align to the
  honest pending-cut note).

## 4. Slice plan (as executed)

- **142A**: 1.0.x LTS branch; VERSION + banners + emitters + LSP;
  pinning-test updates; docs sweep (README, SPEC, status/*,
  release-notes + KARKAIN-1.1 notes, EXAMPLES + corpus recount, cli/lsp
  pages, showcase, issue template); script defaults.
- **142B**: 142 gate (identity both engines, emitter-header allowlist,
  changelog, LTS ref, host archive rehearsal); QA spot battery.
- **142C**: commit → merge → push. Tag cut explicitly out of scope.
