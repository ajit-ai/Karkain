# KARKAIN PACKAGE MANAGER — Audit, Capability Matrix, Gap Analysis & Plan

Phase: Consolidated Package Manager Architecture, Audit & Implementation
Date: 2026-09-03

---

## 1. Package Manager Audit (STEP 0-7 complete)

### Architecture
- Single executable: `karkain.exe` only. No separate package-manager binary.
- Package manager lives in `pkg/pm/*.go`; CLI dispatch in `cmd/karkain/main.go`.
- `pkg/pm` is imported ONLY by `cmd/karkain/main.go`.
- karkain.toml manifest, semver, lock, cache, integrity, registry, workspace,
  audit, auth modules all present in `pkg/pm`.

### Command surface (in `cmd/karkain/main.go`)

| Command | Handler |
|---------|---------|
| `init <name>` | routes to `handlePackageCommand("init",…)` → `pm.InitProject` |
| `add <pkg> [ver]` | routes → `pm.AddDependency` + `pm.FetchModule` |
| `fetch` | routes → `pm.FetchAll` |
| `pkg remove` | `pm.RemoveDependency` |
| `pkg update` | re-fetch (no re-resolve of versions) |
| `pkg upgrade` | re-fetch all registry deps |
| `pkg deps [--tree,--outdated]` | display only |
| `pkg search/info/publish/login/logout/whoami/audit/verify/cache/workspace` | registry/display |

### Key structural finding

**Build integration is absent.** `pkg/cli.RunCommand`/`BuildCommand` load only
sibling `.kark` files (`loadSourceWithSiblings`) and never call
`pm.ResolveModule` for manifest-declared dependencies. So `import json` for a
declared dependency does NOT get wired into `run`/`build`/`check`/`test`. The
package manager is a standalone manifest-editor, not integrated with the
compiler.

### Module resolution
`pm.ResolveModule(projectDir, importPath)` checks: `src/<p>.kark`, `src/<p>/<p>.kark`,
root `<p>.kark`, stdlib candidates, `.karkain/cache/<p>.kark`, nested stdlib.
Deterministic order. Not called by the build path.

### Version handling
Full semver: `parse`, `compare`, `Satisfies` (exact, caret `^`, tilde `~`,
range, wildcard, prerelease). Tested.

### Lockfile
`lock.go` implements `LockFile` read/write/update/remove/islocked/sorted +
`karkain.lock` format. **Tested in isolation but UNWIRED** — fetch/add/update
never read or write it.

### Cache & integrity
`cache.go` list/clean (`.karkain/cache`). `integrity.go` sha256 file/dir
checksum + `.checksum` verification. **Both UNWIRED into `fetchLocal`/**
`copyDir`** which write straight to cache (no atomic temp, no verify).

### Fetch

| Source | Status |
|--------|--------|
| `local` | WORKS (`fetchLocal`: file/dir copy) |
| `git` | **NOT IMPLEMENTED** (`fetchGit` returns error) |
| `registry` | **NOT IMPLEMENTED** (`manager.go:361` returns error) |

`FetchModule` maps `source=""`/`"registry"` to the unimplemented branch.

### Determinism
- Manifest write sorts dependency keys → deterministic. Integrity dir checksum
  sorts rel paths → deterministic. Good.
- No lockfile-driven deterministic graph (gap).
- `FetchAll` iterates map but failures order is nondeterministic (minor).

---

## 2. Package Manager Capability Matrix

| Capability | Exists | Functional | Tested | Prod Ready | Action |
| ---------- | :-----: | :--------: | :----: | :--------: | ------ |
| `init` | YES | YES | YES | YES | preserve |
| `new` | NO | - | - | - | IMPLEMENT |
| `add` | YES | YES | YES | YES (local/git stub) | harden: registry fetch + lock |
| `remove` | YES | YES | YES | YES | preserve |
| `fetch` | YES | PARTIAL | YES | NO | implement registry; lockfile |
| `update` | YES | PARTIAL | NO | NO | re-resolve semantics |
| `list` | YES (as `deps`) | YES | NO | partial | IMPLEMENT distinct |
| `tree` | PARTIAL (`deps --tree`, non-recursive) | PARTIAL | NO | NO | IMPLEMENT recursive |
| Manifest | YES | YES | YES | YES | parser robustness |
| Package identity | PARTIAL (name+ver+source) | YES | - | partial | add checksum |
| Version constraints | YES | YES | YES | YES | preserve |
| Resolver (transitive) | NO | - | - | - | IMPLEMENT P0 |
| Lockfile | YES (helpers) | **UNWIRED** | partial | NO | **WIRE IN P0** |
| Cache | YES | PARTIAL (raw write) | NO | NO | atomic+verify P0 |
| Dependency graph | NO | - | - | - | IMPLEMENT P0 |
| Path dependencies | YES (local) | YES | YES | YES | preserve |
| Workspace | YES | PARTIAL | NO | no | defer (P2) |
| Offline mode | NO | - | - | - | defer (P2) |
| Integrity verification | YES (helpers) | **UNWIRED** | partial | NO | **WIRE IN P0** |
| Registry | PARTIAL (client) | NO (fetch stub) | NO | NO | network; NOT in P0 scope unless ready |
| Publish | PARTIAL (client) | network | NO | NO | FUTURE |

---

## 3. Gap Analysis

### P0 — Foundation gaps (must fix)
1. **Lockfile unwired** — `FetchAll`/`FetchModule`/`AddDependency` don't read/write `karkain.lock`. Highest-priority reproducibility gap.
2. **No transitive resolver / graph** — only direct deps; no conflict detection; can't `list`/`tree` transgitively.
3. **Registry/git fetch unimplemented** — `add` a registry dep then `fetch` fails. Breaks the happy path.
4. **Cache unsafe** — no atomic temp→rename, no checksum verify on fetch; `copyDir` can leave partial/corrupt cache marked as valid.
5. **Build doesn't consume deps** — `run`/`build`/`check`/`test` never resolve manifest deps; imports of deps unfed to compiler.
6. **Determinism incomplete** — no lockfile-driven resolution.

### P1 — Usability
7. Distinct `list`/`tree` (recursive), recursive tree output.
8. Better diagnostics (conflict messages, cause/suggestion).
9. `new` distinct from `init`.

### P2 — Development
10. Offline mode, locked builds (`--locked`), workspace hardening.
11. Safe download atomic+verify.

### P3+/Future
12. Git deps, vendoring, advanced caching, parallel downloads, registry/search/publish/ownership/signing.

### Risks
- Wiring build→deps could change `run`/`build` behavior; must keep existing single-file bootstrap path intact.
- Registry fetch depends on a live endpoint; must fail clearly, not hang.
- Resolver recursion must guard cycles and produce deterministic order.

---

## 4. Implementation Plan

| Phase | Objective | Files | Tests | Risk |
|-------|-----------|-------|-------|------|
| P0-1 | Lockfile integration: fetch/add/remove write+read `karkain.lock` | `pkg/pm/*.go`, lock tests | lockflow tests | med |
| P0-2 | Transitive resolver + graph + conflict detection | `resolver.go`, `graph.go`, tests | resolver/tree tests | med |
| P0-3 | Cache safety: atomic write + checksum verify on fetch | `manager.go`, `integrity.go` | cache tests | low |
| P0-4 | Registry+git fetch real (safe) | `manager.go`, `registry.go`, `source.go` | fetch tests (offline stub) | med |
| P0-5 | Build integration: resolve deps for run/build/check | `pkg/cli`, `cmd/karkain` | cli dep E2E | high |
| P1-1 | Distinct `list`/`tree` (recursive) + `new` | `manager.go`, `main.go` | cli tests | low |
| P2+ | offline/--locked/workspace | later | later | - |

Acceptance (P0): single `karkain.exe`; manifest validated; resolver produces
deterministic graph + lockfile; cache atomic; `add`→`fetch`→`build` of a local
path dep actually compiles; all existing tests green.

---

## 5. Design Decisions

- **Package identity:** `name + version + source + checksum` recorded in lock.
  Not name-only (different sources may share a name).
- **Lockfile:** Karkain-native `karkain.lock` (existing format preserved), now
  actually written by resolve and respected by fetch.
- **fetch vs update:** `fetch` = retrieve locked versions; `update` = re-resolve
  within manifest constraints then rewrite lock. Explicit separation.
- **Path deps:** first-class (no registry needed to develop locally).
- **Registry:** network client exists; P0 wires it with safe download + clear
  offline failure. Do not fake.
- **stdlib:** special compiler component, NOT resolved as third-party package
  (documented; `findStdlib` handles it).