# KARKAIN - Phase P2 (Package Ecosystem Maturity) - GATE 4 Engineering Report

Phase: P2 - Package Ecosystem Maturity (gated methodology)
Gate: GATE 4 - Cache, Resolution & Lock (P2.7, P2.8, P2.9)
Date: 2026-09-04
Repo: `F:\Codes\Git\Karkain`
Base: GATE 3 committed/merged/pushed (`1f8a933`)

---

## 1. GATE 4 SCOPE

GATE 4 covers:
- P2.7 - source-aware caching,
- P2.8 - offline / cache-first resolution,
- P2.9 - lock-file pinning (git revs).

Extends the existing cache/lock architecture (no subsystem rewritten).

---

## 2. WHAT WAS DONE

### P2.7 - Source-aware cache identity
- `pkg/pm/source.go`: added `cacheDirName(dep, rev)` - the source-aware on-disk
  cache identity, with backward-compatible rules:
  - **local/workspace**: `name` (preserves existing `TestPackageManager_FetchLocal`
    and canonical-source semantics),
  - **registry (no url)**: `name@version` (preserves `VerifyIntegrity` layout),
  - **git**: `name@<rev>` (source-aware - immutable commit is the identity),
  - **url-bearing/other**: full `CacheIdentityKey` (name@identity+source+url).
- `pkg/pm/integrity.go`: fixed `ComputeDirChecksum` to exclude the `.checksum`
  file from the hashed content. Previously the recorded checksum excluded
  `.checksum` (it did not exist at write time) but verification re-included it,
  so every fresh fetch would "fail" verification. Both write and verify are now
  self-consistent. `VerifyIntegrity` (audit.go) updated to compute cache dirs via
  `cacheDirName` so git packages keyed by rev are found.

### P2.8 - Cache-first / offline resolution
- `pkg/pm/flow.go`: new `CachedValid(projectDir, dep, rev) (bool, error)` - the
  offline gate. Returns true when the package is present at the identity's
  source-aware cache dir AND its stored checksum verifies. Local/workspace deps
  are never cache-valid (they use canonical source paths).
- `registeredFetch` (used by `FetchLocked`) now calls `CachedValid` first and
  **skips the network entirely** when the locked package is already valid in the
  cache. This makes `karkain pkg fetch` offline-capable for already-materialized
  deps.

### P2.9 - Lock-file git rev pinning
- `pkg/pm/flow.go`:
  - `LockFromGraph` now records `Rev` for git nodes.
  - `attachGitRevs(graph)` resolves each git dep to its immutable commit
    (`ResolveGitRevision`) and sets `node.Rev` before locking - the `update`
    path pins git deps by commit SHA.
  - `ResolveAndLock` calls `attachGitRevs` so `karkain update` produces a lock
    with git deps pinned by rev.
- `pkg/pm/git.go`: `cloneAtRev(url, rev, subdir, destDir)` checks out a
  given (already-resolved) commit WITHOUT re-resolving - the pin-fetch path.
  Falls back to fetching the pinned commit if not reachable from the clone head.
- `registeredFetch` uses `cloneAtRev` + `fetchGitPinned` for locked git deps,
  writing to the source-aware `name@rev` cache dir atomically (temp dir then
  rename + checksum + `.rev`). Cache-first: `CachedValid(dep, rev)` short-circuits.
- `FetchModule` now computes its cache dir via `cacheDirName` (source-aware) and
  resolves the git rev once up front (reused, avoiding a double clone).

---

## 3. GATE 4 VERIFICATION (all checks green)

| Check | Result |
|-------|--------|
| `go build ./...` | PASS (exit 0) |
| `go vet ./pkg/pm/...` | PASS |
| gofmt (modified/new, CR-normalized) | PASS |
| `go test ./pkg/pm/... -count=1` | PASS (incl. 11 new GATE 4 tests) |
| `go test ./pkg/cli/... -count=1` | PASS (regression) |
| module-system regression (lexer/parser/codegen/sema) | PASS |

New tests (`pkg/pm/cache_lock_test.go`, 11):

- `TestCacheDirName_SourceAware` / `_GitRevDiffersByIdentity` - source-aware keys
- `TestCachedValid_FalseWhenAbsent` / `_LocalNeverCachedByChecksum` / `_GitRev`
- `TestFetchLocked_CacheFirstSkipsNetwork` - offline, no registry contact
  (points at unreachable `127.0.0.1:1`)
- `TestLockFromGraph_RecordsGitRev` / `TestLock_GitRevRoundTrip` - rev lock round-trip
- `TestResolveAndLock_PinsGitRev` - `update` pins git dep to repo HEAD (`setupGitRepo`)
- `TestVerifyIntegrity_SourceAwareCache` - integrity finds git package by rev

Notable correctness fix discovered during this gate: the pre-existing
`.checksum` self-inclusion bug (GATE 4 verification surfaced it via the new
cache-first path). Fixed by excluding `.checksum` from the dir checksum so write
and verify agree - without changing the stored format.

---

## 4. GATE 4 VERDICT

> ### PASS

GATE 4 (Cache, Resolution & Lock) meets the master-prompt acceptance criteria:
- Source-aware cache identity (Rule 3) with backward-compatible legacy layouts,
- cache-first/offline resolution gate without network,
- git deps pinned in the lock by immutable commit and fetched by pin,
- integrity-consistent checksums (latent bug fixed),
- extended, not rewritten, architecture; full suite green.

Proceeding to **GATE 5 - Workspace Build & Test (P2.10, P2.11)** next, per the
gated methodology.
