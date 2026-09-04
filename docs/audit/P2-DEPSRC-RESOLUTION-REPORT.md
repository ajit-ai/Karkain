# KARKAIN - Phase P2 Follow-up: Manifest Dependency Resolution in build/run/check

Phase: P2 continuation (deferred P2 item, per AGENTS.md and P2-FINAL-REPORT)
Sub-stage: Complete manifest-dep wiring into build/run/check/test
Date: 2026-09-04
Repo: `F:\Codes\Git\Karkain`
Base: P2 GATE 6 committed/merged/pushed (`c2b6f3c`)

---

## 1. SCOPE

The P2 final report deferred: "wire manifest dependency resolution into
build/run/check (avoided to keep compiler pipeline stable)". Investigation
showed local deps were already resolved by `projectSourceFiles`
(`pkg/cli/commands.go`), but **registry and git** dependencies - which are
fetch()ed into the project cache - were NOT included in build/run/check/test
source assembly. This sub-stage closes that gap additively.

---

## 2. WHAT WAS DONE

### `pkg/pm/depsrc.go` (new)
- `DependencySources(projectDir) []DepSource` - deterministic, sorted-by-name
  resolution of every **non-dev** dependency's on-disk source directory:
  - **local/workspace**: canonical source path (the dependency directory itself;
    no manifest required - backward compatible with leaf dirs of `.kark` files).
  - **registry**: fetched cache dir (`.karkain/cache/<name>@<version>`), using
    the manifest version (lock version as fallback).
  - **git**: fetched cache dir (`.karkain/cache/<name>@<rev>`) keyed by the
    **lock-file revision** (view-consistent with `fetch`/`update`).
  - Registry/git deps report an error (not fetched) unless already cached.
  - **Never auto-fetches / no network** - preserves the explicit
    `fetch`/`update` rule and compiler-pipeline stability.
  - Dev-dependencies excluded (never leak into build/run/check).
- `DepSource{Name, Source, Version, Rev, Dir, Cached, Err}` + `Resolved()`.
- New error codes: `ErrLocal` (`E-PKG-LOCAL`), `ErrSource` (`E-PKG-SOURCE`).

### `pkg/cli/commands.go`
- `projectSourceFiles` now assembles dependency sources via
  `pm.DependencySources` (all kinds) instead of the former local-only block.
  Registry/git deps contribute `.kark` sources only when cached on disk
  (upstream, before the project's own modules and the root file), preserving
  deterministic order and backward compat.

### Tests
- `pkg/pm/depsrc_test.go` (new, 7): local resolves to canonical path (with and
  without manifest); missing local errors; registry fetched vs not-fetched;
  git uses lock rev; dev-deps excluded; sorted deterministic; empty.
- `pkg/cli/module_resolve_test.go`: new `TestResolveSources_RegistryDepAssemblesFromCache`
  - unfetched registry dep contributes nothing; fetch()ed dep (seeded cache)
    contributes its source upstream of main.

---

## 3. REGRESSIONS FOUND & FIXED (during implementation)

Two existing tests regressed because the FIRST version of `DependencySources`
required a `karkain.toml` inside a local dep; the pre-existing behavior (and
both tests) treat a local dep as just its directory of `.kark` files, with no
manifest required. Fixed by resolving local deps on **directory existence**,
not manifest presence - restoring backward compatibility exactly while still
allowing registry/git cache resolution. Re-ran the two regressed tests and the
full suite to confirm green.

---

## 4. VERIFICATION

| Check | Result |
|-------|--------|
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| gofmt (new/modified, CR-normalized) | PASS |
| `go test ./... -count=1` | PASS - full suite (all 26 packages) |
| New tests | 7 (pm) + 1 (cli) = 8, all PASS |

---

## 5. VERDICT

> This sub-stage is **COMPLETE**.

Build/run/check/test now resolve manifest dependencies of ALL kinds
(local/workspace/registry/git) into their source assembly, dev-deps excluded,
deterministic, offline-safe, and backward-compatible with the existing local
source layout. The deferred P2 item is closed.
