# Karkain 1.0.0 — Release Checklist

**Version target:** 1.0.0 (Stable)
**Owner decision:** cutting `v1.0.0` and publishing binaries is an owner-only
action on the existing CI release pipeline. This checklist governs the final
preparation phase (Phase 119) and the release-cut runbook (Phase 128).

Where a reference doc already exists, this checklist links to it instead of
duplicating content.

## 0. Release state (Phase 128 audit — 2026-09-19)

- [x] Tag `v1.0.0` **exists on origin** but points at `f23c024`
      (2026-09-13), which is **behind current `main`** (`9dfdc76`,
      Phase 126 merged 2026-09-18). Phases 125A/126/127 are **not** on the tag.
- [x] **No GitHub Release exists** for v1.0.0 — GitHub Releases page shows only
      the legacy "Phase 17 COMPLETED (v0.14.0)" (Latest). The tag is
      effectively unreleased, so re-pointing it is safe and correct.
- [x] Version identity is already unified at 1.0.0 (`VERSION`, Go CLI banner,
      kcc banner, all generated-C/QIR/QASM headers) — verified by grep in
      Phase 128; no stale `0.117.0`/`Beta 1 Build` strings outside historical
      pages/tests.

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

- [x] Example corpus: **59** golden `.kark` files wired into the phase-114
      both-engine gate (Go + kcc) + phase-116 test-mode files; pinned
      byte-identical (see Phase 126 record). Categories include networking
      (`stdlib/net`), database (`stdlib/db`) and web (`stdlib/http`) real
      surfaces (Phase 125A).
- [x] Planned categories (11-quantum) are honest Planned README-only pages
      (no fabricated APIs).
- [x] Sphinx doc tree builds `-W` (see QA battery).

## 5. QA (master gate)

- [ ] Full QA battery run and green — `scripts/qa/run-full-qa.ps1`; evidence
      recorded in `docs/audit/PHASE-119-LANGUAGE-QA-FINAL-REPORT.md`.

## 6. Owner-only release runbook (Phase 128)

Sequential, after Phase 127/128 changes are committed and green.

1. **Commit & merge** — commit Phase 127 (bootstrap memory guard) and Phase 128
   (release prep) to `develop`; merge `develop` into `main`; push `develop`
   and `main`. Record the new `main` head commit `NEWMAIN`.
2. **Re-point the unreleased tag** — `v1.0.0` was never released, so move it to
   the current `main` head so the release includes Phases 110–127:
   ```powershell
   git tag -f v1.0.0 NEWMAIN
   git push origin v1.0.0 --force
   ```
   This re-triggers the tag-based CI pipeline (`.github/workflows/ci.yml`).
3. **Release build** — the CI `build` job produces the 13-platform archive
   matrix; `release` job (tag-only, `softprops/action-gh-release`) creates the
   GitHub Release with archives + generated `checksums.txt` (SHA-256) + release
   notes.
4. **Post-publish verification** (on a clean Windows 11 machine):
   - Download `karkain-v1.0.0-windows-amd64.zip`; verify against
     `checksums.txt` (`Get-FileHash`), then `Expand-Archive` into
     `$env:LOCALAPPDATA\Karkain` and add to `PATH`.
   - `karkain --version` reports `Karkain Compiler v1.0.0 (windows/amd64,
     Stable Build)`.
   - Run the reproduction of the external-developer journey against the
     **downloaded binary**:
     ```powershell
     $env:RELEASE_BIN = "$env:LOCALAPPDATA\Karkain\...\karkain.exe"
     powershell -ExecutionPolicy Bypass -File scripts\verify-rc-journey.ps1
     ```
   - Run the install/verify smoke test:
     ```powershell
     powershell -ExecutionPolicy Bypass -File scripts\install.ps1
     powershell -ExecutionPolicy Bypass -File scripts\verify-install.ps1
     ```
5. **Flip the docs marker** — in
   `docs/source/getting-started/installation.rst`, remove the
   `:planned:`Release cut pending`` note now that the release is live; commit
   to `develop`, merge to `main`, push.
6. **Announce** — post the release link with install/verify instructions and
   the honest scope summary (see `KARKAIN-1.0-RELEASE-NOTES.md`).

See `docs/audit/PHASE-119-LANGUAGE-QA-FINAL-REPORT.md` for the readiness
verdict and complete evidence; `docs/audit/GENERAL-AVAILABILITY-ROADMAP.md`
for the post-1.0 GA milestones (GA-2/GA-3).