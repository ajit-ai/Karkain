# KARKAIN — Phase P2 (Package Ecosystem Maturity) — GATE 1 Engineering Report

Phase: P2 — Package Ecosystem Maturity (gated methodology)
Gate: GATE 1 — Source Infrastructure (P2.1, P2.2, P2.3)
Date: 2026-09-04
Repo: `F:\Codes\Git\Karkain`
Branches: `develop` → `main` (both at `c27d6ea` before this gate)

---

## 1. GATE 1 SCOPE

Per the P2 master prompt, GATE 1 covers three steps and a hard STOP with report
before any GATE 2 (Git) work:

| Step | Deliverable |
|------|-------------|
| P2.1 | Manifest + Package Identity Hardening |
| P2.2 | Dependency Graph Verification (comprehensive tests) |
| P2.3 | Unified Source Abstraction |

Approach: extend the existing `pkg/pm` architecture. **No subsystem rewritten.**
All additions are additive and backward compatible.

---

## 2. WHAT WAS DONE

### P2.1 — Manifest + Identity Hardening (`pkg/pm/manager.go`)
- Added `Manifest.Repository string` metadata field (Rule 5: **no `edition`**
  added; repository is publish metadata only).
- Added `Manifest.DevDependencies map[string]Dependency` with full
  parse/write support for a `[dev-dependencies]` TOML section (Rule 8: test/dev
  deps must not leak into normal builds).
- Introduced `validPackageName(name)` — the package identity rule:
  lowercase-alphanumeric plus `-`/`_`, non-empty, ≤64 chars, must not start/end
  with a separator. This is a **package** identity rule, deliberately separate
  from file/module naming.
- Hardened `ValidateManifest()` — now also flags:
  - invalid package names (project + dependencies),
  - duplicate dependency entries within a group,
  - git/local deps missing a required `url`,
  - registry deps missing a version.
  All previously-valid manifests remain valid (additive only).
- Rewrote `WriteManifest()` and `ParseManifest()` to round-trip `dev-dependencies`
  and `repository` deterministically (sorted keys, TOML table syntax).

### P2.2 — Dependency Graph Verification (`pkg/pm/graph_verify_test.go`)
Added a comprehensive graph test suite covering:
- multiple direct dependencies,
- deep transitive chains (a→b→c→d) with direct vs transitive flags,
- `Children()` parent relationships + sorted ordering,
- missing local dep (declared but absent → remains declared, no hard failure),
- source-kind presence on resolved nodes (local/git/registry),
- hard version-conflict detection (`Resolver.add`),
- `ResolveDirect` excludes dev-dependencies.

Existing resolver tests (single, transitive, cycle, deterministic order, tree,
lock round-trip) all remain green.

### P2.3 — Unified Source Abstraction (`pkg/pm/source.go` — new file)
- `SourceKind` type: `local`, `workspace`, `git`, `registry`.
  `workspace` is kept distinct from `local` because workspace members share the
  workspace source tree (does not become a copy).
- `ParseSourceKind` / `ValidateSourceKind` — the single validation entry point
  reused by manifest validation (workspace now a recognized source).
- `CacheIdentityKey(name, version, rev, source, url)` — **source-aware cache
  identity** (Rule 3): package name + resolved identity (git rev overrides
  version for git deps) + source kind + normalized source URL. Two packages of
  the same `name@version` from different sources are distinct identities.
- `SanitizeCacheComponent` — replaces path-unsafe characters, preventing path
  traversal / ambiguity in cache keys.
- `normalizeSourceURL` — lowercases scheme/host, trims trailing `.git` so URL
  aliases map to one normalized source identity.
- `ErrCode` + `PkgError` — Karkain `E-PKG-*` error codes (e.g.
  `E-PKG-GIT-REVISION`, `E-PKG-REGISTRY`, `E-PKG-CACHE`) with actionable
  diagnostics, ready for GATE 2+.
- `GraphNode.Rev string` added to `pkg/pm/resolver.go` to hold the resolved
  immutable git commit for lock pinning (GATE 4/5), additive and unused-for-now.

---

## 3. GATE 1 VERIFICATION (all checks green)

| Check | Result |
|-------|--------|
| `go build ./...` | PASS (exit 0) |
| `go vet ./pkg/pm/...` | PASS |
| gofmt (3 new/edited files, CR-normalized) | PASS (clean) |
| `go test ./pkg/pm/... -count=1` | PASS |
| `go test ./pkg/cli/... -count=1` | PASS |
| module-system regression: lexer/parser/codegen/sema | PASS |
| New P2.1/P2.2/P2.3 tests | PASS (all introduced) |

New tests added:
- `pkg/pm/manifest_hardening_test.go` — 11 tests (repository round-trip,
  dev-deps round-trip + no-leak, dev-deps excluded from prod resolution,
  invalid/valid names, new validation rules, source abstraction, cache identity).
- `pkg/pm/graph_verify_test.go` — 8 tests (graph structure as above).

Note on repo formatting: the entire `pkg/pm` (and repo) uses CRLF line endings
committed at HEAD (verified: HEAD `manager.go` = 683 CRLF pairs). `gofmt -l`
therefore lists every file (CRLF vs LF). This is a **pre-existing repo-wide
condition**, untouched by this gate; only genuine alignment issues in the new
files were corrected. Reformatting the whole tree is out of scope and would
violate the "do not rewrite working subsystems" rule.

---

## 4. GATE 1 VERDICT

> ### ✅ PASS

GATE 1 (Source Infrastructure) meets the master-prompt acceptance criteria:
- additive, backward compatible, no subsystem rewrite;
- source-aware identity foundation in place (`CacheIdentityKey`);
- `E-PKG-*` error code framework established for GATE 2+;
- dev/test deps proven isolated from production resolution;
- full existing + new suite green.

Proceeding to **GATE 2 — Git Source Integration (P2.4)** next, followed by its
own VERIFY + report, per the gated methodology.
