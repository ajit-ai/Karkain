# KARKAIN PACKAGE MANAGER — P0 Implementation Report

Phase: Package Manager Hardening — Deterministic Resolver, Lockfile Integration, Cache Safety, CLI Wiring
Date: 2026-09-03
Status: COMPLETE (P0). Build/run/check manifest-dep resolution deferred (recommended next step).

---

## 1. Deliverables

| Artifact | Path | Purpose |
|----------|------|---------|
| Package Manager Audit | `docs/audit/PACKAGE-MANAGER-AUDIT.md` | Capability matrix, gap analysis, plan (P0/P1/P2+). |
| Resolver | `pkg/pm/resolver.go` | Deterministic dependency resolution + `DepGraph`. |
| Workflows | `pkg/pm/flow.go` | `ResolveAndLock`, `FetchLocked`, `ResolvedDetails`, `TreeLines`, `NewProject`. |
| Cache safety | `pkg/pm/manager.go` (`FetchModule`) | Atomic temp→rename fetch; checksum record; empty-result guard. |
| Tests (unit) | `pkg/pm/resolver_test.go` | Resolver, determinism, cycles, tree, lock round-trip, `new`. |
| Tests (E2E) | `pkg/cli/package_cli_test.go` | Binary-level `new/add/update/fetch/list/tree/remove`, help text. |

## 2. Design Decisions

1. **Single `karkain.exe`** — no `kpm`/`packagemanager` binary. New commands are
   top-level (`karkain list`) with `karkain pkg list` aliases.
2. **Deterministic resolution** — `Resolve` walks manifest deps in sorted order,
   expands local transitive deps from each dependency's own `karkain.toml`,
   and is cycle-safe (already-visited nodes short-circuit). Lockfile output is
   deterministic regardless of manifest map iteration order.
3. **`update` ≠ `fetch`** — `update` re-resolves within version constraints and
   rewrites `karkain.lock`; `fetch` retrieves exactly the already-locked
   versions (`FetchLocked`). This matches Cargo/Go semantics.
4. **Cache is never written in place** — fetch goes to a temp sibling dir, is
   validated non-empty, checksummed, then atomically renamed into the cache.
   A failed/partial download never corrupts the cache.
5. **Registry/git NOT faked** — registry JSON parse and git clone remain explicit
   "not implemented"/"not available" errors, per prompt rules.
6. **`new` distinct from `init`** — `new <name>` requires a name and creates a
   project directory; `init [name]` defaults to the cwd base name.

## 3. Command Surface (before → after)

| Command | Before | After |
|---------|--------|-------|
| `fetch` | `FetchAll` (no lockfile, direct writes) | resolve → `FetchLocked` (safe, locked) |
| `update` | re-fetch only (no version re-resolve) | re-resolve + rewrite `karkain.lock` |
| `list` (new) | — | direct + transitive with resolved versions |
| `tree` (new) | — | recursive dependency graph |
| `new` (new) | — | create project directory |
| `remove` | `pkg remove` only | top-level `karkain remove` |

## 4. Tests

```
go test ./pkg/lexer/... ./pkg/parser/... ./pkg/codegen/... ./pkg/pm/... -count=1
```
All green (lexer, parser, codegen, pm). Added:
- `pkg/pm`: resolver single/transitive/cycle/determinism, tree formatting
  (libb appears once, nested), lock round-trip, `NewProject` distinct.
- `pkg/cli`: E2E through the real binary (`TestCLI_NewListTreeFetch_E2E`,
  `TestCLI_HelpListsNewCommands`).

`./pkg/bootstrap/...` continues to fail on the known stale self-hosting artifact
(`src/compiler/main.c`, borrow-checker rejects `main.kark`) — pre-existing,
unrelated to PM changes, documented in prior phases.

## 5. Files Changed
- `pkg/pm/resolver.go` (new)
- `pkg/pm/flow.go` (new)
- `pkg/pm/resolver_test.go` (new)
- `pkg/pm/manager.go` (cache safety in `FetchModule`)
- `pkg/cli/package_cli_test.go` (new)
- `cmd/karkain/main.go` (early dispatch of package commands, `list`/`tree`/
  `new`/`update`/`remove` wiring, `fetch` uses lockfile, help text)
- `docs/audit/PACKAGE-MANAGER-AUDIT.md` (audit, matrix, plan)

## 6. Deferred / Recommended Next Steps
- **P0-5 (build integration):** make `run/build/check/test` resolve manifest
  dependencies before compiling. Deliberately deferred to keep the compiler
  pipeline stable; flagged as the highest-value follow-up.
- **P1+:** registry JSON parse + publish tarball; git fetch; `upgrade`;
  `deps --outdated` using a real endpoint; workspace multi-crate build.