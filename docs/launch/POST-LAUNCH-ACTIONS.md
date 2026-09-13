# Post-Launch Owner Actions

Owner-only checklist to complete Karkain's public launch. This is NOT a
commit/release request to me — it documents what the owner should do once
they are ready (the audit's absolute rule: I made no commits, pushes, tags,
merges, or releases).

## 1. Cut the GitHub Release `v1.0.0`

- The tag `v1.0.0` exists (points at `f23c024`, release-certified). The
  GitHub **Release page does not exist yet** — an anonymous API lookup of
  `releases/tags/v1.0.0` returns 404, and no `event=tag` CI run has ever
  fired (GitHub suppressed the trigger because the tag was pushed at the
  same SHA as the `main` push).
- Attach the 13 archives from `releases/` to the release:
  - windows-amd64, windows-arm64 (zip)
  - linux-amd64, linux-arm64, linux-armv7, linux-i386, linux-ppc64le,
    linux-s390x (tar.gz)
  - darwin-amd64, darwin-arm64 (tar.gz)
  - freebsd-amd64 (tar.gz)
- Checksums: `docs/audit/KARKAIN-1.0.0-FINAL-RELEASE-CERTIFICATION.md` §16.
- Alternative to uploading manually: re-run the release workflow after
  creating the release (`actions: workflow_dispatch` on `main`) or push a
  NEW commit (any SHA not already on `main` at tag time) followed by the
  tag so the `tags: ['v*']` trigger fires. The release-all scripts
  (`scripts/release-all.ps1`/`.sh`) rebuild the archives.

## 2. Confirm the docs Pages deploy

`.github/workflows/docs.yml` publishes on push to `main`. After the next
`main` push, confirm `docs/source` builds with `-W` and the Pages site is
live (Pages must be enabled in repo settings with action-based deployment).

## 3. Address the CI test flake

`Run unit tests` step (`go test ./pkg/lexer/... ./pkg/parser/... ./pkg/codegen/... ./pkg/pm/... -count=1 -v`)
fails intermittently: `--- FAIL: TestPhase107_CodegenSpawnJoin (pkg/codegen)`,
reproduced locally in the combined suite; isolated runs pass. Recommend:
quarantine/split in CI or a test-hardening fix on `develop` after 1.0.0.

## 4. Optional: post to public channels

Use `docs/launch/RELEASE-ANNOUNCEMENT-DRAFT.md`, then consider: Hacker
News, lobste.rs, r/programming + r/Compilers, a brief blog post. All copy
must follow the honesty vocabulary (`docs/source/status/index.rst`).

## 5. Housekeeping reminders

- The repository is PUBLIC (was private until 2026-09-13) — GitHub ALERTs /
  dependency graph / secret scanning may flag the repo for review.
- Keep `releases/` and `.build/` ignored (already in `.gitignore`).
- Delete generated `*.c`/`*.c23` files beside examples before any future
  commit (existing convention; `git status` must stay clean).