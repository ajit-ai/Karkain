# Karkain 1.0.0 — Release Checklist

**Version target:** 1.0.0 (Stable)
**Owner decision:** no tag / release cut yet — this checklist governs the
final preparation phase (Phase 119). Cutting `v1.0.0` and publishing binaries
is an owner-only decision on the existing CI release pipeline.

Where a reference doc already exists, this checklist links to it instead of
duplicating content.

## 1. Repository hygiene

- [x] Obsolete / superseded tracked material removed (see `phases` run of the
      Phase 119 cleanup):
  - stale build/release scripts (`scripts/build.sh`, `build.ps1`,
    `package.ps1`, `release.ps1`, `release.sh`)
  - legacy `std/` stub modules and `pkg/stdlib/*.go` placeholders
  - committed generated C in `compiler/` (`ast.c`, `codegen.c`, `main.c`)
  - legacy pre-90 loose `examples/*.kark` regression fixtures (24 files)
  - legacy `src/*.kark` Phase-3 prototypes + `src/main.rs`
  - `docs/ROADMAP-PRODUCTION.md` (superseded by `ROADMAP.md`)
- [x] Untracked build debris removed (root `karkain.exe`, `kcc.exe`,
      `src/kcc.exe`, `src/compiler/main.exe`, `src/compiler/main.c`,
      `bin/`, `temp/`, `docs/build/`)
- [x] No generated `.c`/`.exe` artifacts are tracked; `.gitignore` covers
      `/releases/`, `/.build/`, `/artifacts/`, `bin/`, `temp/`, `*.exe`, `*.c`.

## 2. Version identity (unified 1.0.0)

- [x] `VERSION` = `1.0.0` (authoritative single source)
- [x] Go CLI banner: `Karkain Compiler v1.0.0 (<os>/<arch>, Stable Build)`
      (`cmd/karkain/main.go`, mirrors in `pkg/cli/commands.go`)
- [x] Self-hosted kcc banner + all generated-C headers (`src/compiler/` +
      `pkg/codegen/*` emitters) say `v1.0.0 (...) Stable Build`
- [x] Version-asserting gates pass at 1.0.0 (phase 88/90/115/117/118/119,
      `pkg/bootstrap/args_test.go`)
- [x] scripts, issue templates, docs example outputs updated (no `0.117.0`,
      no `Beta 1 Build` outside historical pages)

## 3. Public label

- [x] README, ROADMAP header, SPEC version history, SECURITY support table,
      `docs/source/index.rst`, `status/index.rst`, `compatibility.rst`,
      `installation.rst`, `stable-api.rst`,
      `docs/source/release-notes.rst` all carry **Karkain 1.0.0 (Stable)**.
- [x] Historical pages (`status/beta.rst`, `status/migration-beta1.rst`,
      `development/developer-preview.rst`, `status/rc-checklist.rst`) remain
      as historical records with a pointer to the current status.

## 4. Examples & documentation

- [x] Example corpus: 49 golden `.kark` files wired into the phase-114
      both-engine gate + phase-116 test-mode files; no coverage gaps for
      implemented stable capabilities (fundamentals, algorithms, systems,
      data, AI/ML, scientific, finance, security, dev-tools).
- [x] Planned categories (04-networking, 06-database, 07-web, 11-quantum) are
      honest Planned README-only pages (no fabricated APIs).
- [x] Sphinx doc tree builds `-W` (see QA battery).

## 5. QA (master gate)

- [ ] Full QA battery run and green — `scripts/qa/run-full-qa.ps1`; evidence
      recorded in `docs/audit/PHASE-119-LANGUAGE-QA-FINAL-REPORT.md`.

## 6. Owner-only release steps (after this phase)

1. Cut tag `v1.0.0` on the existing CI release pipeline (`.github/workflows/ci.yml`).
2. Publish binaries/artifacts; update `docs/source/getting-started/installation.rst`
   binaries from "Planned" to the real ones.
3. Push `develop` and `main`.
4. Announce the release.

See `docs/audit/PHASE-119-LANGUAGE-QA-FINAL-REPORT.md` for the readiness
verdict and complete evidence.