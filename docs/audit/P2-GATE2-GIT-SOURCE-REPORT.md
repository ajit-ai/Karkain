# KARKAIN - Phase P2 (Package Ecosystem Maturity) - GATE 2 Engineering Report

Phase: P2 - Package Ecosystem Maturity (gated methodology)
Gate: GATE 2 - Git Source Integration (P2.4)
Date: 2026-09-04
Repo: `F:\Codes\Git\Karkain`
Base: GATE 1 committed/merged/pushed (commits `9526af7` on develop + main)

---

## 1. GATE 2 SCOPE

GATE 2 covers P2.4 - reproducible Git dependency resolution and fetch - with a
hard STOP + report before GATE 3 (Registry).

Per the P2 master prompt, this gate implements the non-negotiable git rules:

> Git resolution must be reproducible: per requested reference type (default
> branch / branch / tag / exact commit), never rely only on `clone --depth 1`;
> ALWAYS resolve `git rev-parse HEAD` -> immutable commit SHA is the final
> identity.

---

## 2. WHAT WAS DONE (all in `pkg/pm/git.go` - new file)

### Reproducible revision resolution: `ResolveGitRevision(url, version, subdir)`
- Full clone (NOT `--depth 1`) so arbitrary refs can be resolved deterministically.
- Reference classification (`classifyGitRef`):
  - `""` or `"HEAD"`   -> `default` branch (via `git symbolic-ref refs/remotes/origin/HEAD`)
  - 40-hex string      -> `commit` (exact SHA)
  - anything else      -> `branch`/`tag` ref (resolved via `git rev-parse`)
- Always resolves the chosen ref to the full 40-hex commit `git rev-parse <ref>^{commit}`,
  then `git checkout <sha>` so the working tree matches the pinned commit.
- Returns `GitResolution{URL, Kind, RequestedRef, SHA, Subdir}` - the SHA is the
  immutable, reproducible package identity.

### Fetch integration: `fetchGit` / `fetchGitAt`
- `fetchGitAt(url, version, subdir, destDir)` clones, checks out the resolved SHA,
  and copies the package (optionally a `subdir` package root) into the destination.
- `FetchModule` git case (in `manager.go`) now:
  1. calls `ResolveGitRevision` first,
  2. fetches via `fetchGitAt`,
  3. writes a `.rev` file recording the resolved immutable commit into the cached
     package via `WriteRevFile`, so the pinned revision survives without re-resolving.

### Security (Rule: no unsafe shell / no path traversal)
- `runGit` uses `exec.Command("git", args...)` with a slice of args - **no shell
  interpolation**, no metacharacter execution.
- Subdir validated with `withinDir` (clean relative containment check) before any
  use; traversal subdirs are rejected with `E-PKG-GIT-SUBDIR`.
- Atomic clone into temp dir, cleaned up on all failure paths.

### Error taxonomy
- `ErrGit` implements the existing `PkgError`/`ErrCode` framework:
  - `E-PKG-GIT-NOT-FOUND` (git binary absent)
  - `E-PKG-GIT-CLONE` (clone/copy failures)
  - `E-PKG-GIT-REVISION` (ref cannot be resolved to a commit)
  - `E-PKG-GIT-SUBDIR` (subdir missing / path traversal)

### Testability
- `ResolveGitRevision` accepts any URL `git clone` accepts, including local
  `file://` and plain path URLs, enabling hermetic tests against throwaway repos.

---

## 3. GATE 2 VERIFICATION (all checks green)

| Check | Result |
|-------|--------|
| `go build ./...` | PASS (exit 0) |
| `go vet ./pkg/pm/...` | PASS |
| gofmt (new files, CR-normalized) | PASS (clean) |
| `go test ./pkg/pm/... -count=1` | PASS (incl. 10 new git tests) |
| `go test ./pkg/cli/... -count=1` | PASS (regression) |

New tests (`pkg/pm/git_test.go`, 10 tests) execute real `git` against local
repos built in `t.TempDir()`:

- `TestClassifyGitRef` - ref-kind classification (default/commit/branch/tag)
- `TestResolveGitRevision_DefaultBranch` - resolves default branch to repo HEAD
- `TestResolveGitRevision_ExactTag` - resolves tag to tagged commit SHA
- `TestResolveGitRevision_ExactCommit` - resolves exact 40-hex commit
- `TestResolveGitRevision_UnknownRefFails` - unknown ref -> `E-PKG-GIT-REVISION`
- `TestResolveGitRevision_Subdir` - nested package root resolution
- `TestResolveGitRevision_SubdirTraversalRejected` - `../escape` -> `E-PKG-GIT-SUBDIR`
- `TestFetchModule_Git_ProducesCache` - full FetchModule path: cached manifest +
  checksum + `.rev` file matching repo HEAD
- `TestCacheIdentityKey_TracksGitRev` - git cache identity varies with revision
- `TestRunGit_NoShellInterpolation` - `runGit` passes args safely

Formatting: repo-wide CRLF line endings at HEAD are a pre-existing condition
(verified in GATE 1); new files gofmt-clean under CR-normalized inspection.

---

## 4. GATE 2 VERDICT

> ### PASS

GATE 2 (Git Source Integration) meets the master-prompt acceptance criteria:
- Reproducible revision resolution for all four reference types, always pinned
  to an immutable commit SHA via `rev-parse` (never relying on depth-1 alone).
- Subdir support with traversal protection.
- Safe process invocation (no shell).
- `E-PKG-GIT-*` actionable error codes.
- `.rev` recording enables lock pinning in GATE 4/5.
- Full existing + new suite green; git deps now fetch end-to-end.

Proceeding to **GATE 3 - Registry Source Integration (P2.5, P2.6)** next, with
its own VERIFY + report, per the gated methodology.
