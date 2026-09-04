# NEXT-STAGE ENGINEERING REPORT

Engineering continuity log for Karkain development stages. Append the most
recent completed stage at the top of the "Stage Log" section.

---

## Stage Log

### Stage: P2 Follow-up - Manifest Dependency Resolution in build/run/check (COMPLETE)
Date: 2026-09-04

Closed the P2-deferred item: build/run/check/test now resolve manifest
dependencies of ALL kinds (local/workspace/registry/git) into source assembly,
dev-deps excluded, offline-safe (never auto-fetches), backward compatible.

- `pkg/pm/depsrc.go` (new): `DependencySources()` - deterministic source dirs
  per dep; local->canonical path, registry->`<name>@<version>` cache, git->
  `<name>@<rev>` cache (lock rev). New codes E-PKG-LOCAL, E-PKG-SOURCE.
- `pkg/cli/commands.go`: `projectSourceFiles` assembles all dep kinds upstream.
- 8 new tests; full suite green.
Report: `docs/audit/P2-DEPSRC-RESOLUTION-REPORT.md`.

Remaining deferred (priority order):
1. Public registry deployment (protocol v1 defined; no production server yet).
2. Transitive registry/git dependency expansion behind real host access.

---

### Stage: Phase P2 - Package Ecosystem Maturity (COMPLETE)
Date: 2026-09-04

Phase P2 delivered a source-aware, reproducible, offline-capable package
ecosystem and workspace orchestration, split across six gated sub-deliveries
(each committed to `develop`, ff-merged to `main`, and pushed):

| Gate | Delivery | Commit |
|------|----------|--------|
| GATE 1 | Source infrastructure: hardened manifest, unified SourceKind, PkgError codes | `9526af7` |
| GATE 2 | Git source: rev-parse pinning to immutable SHA, subdir safety | `08891c4` |
| GATE 3 | Registry source: protocol v1, safe extraction, local test server | `1f8a933` |
| GATE 4 | Cache/resolution/lock: source-aware cache, offline-first, git rev pin | `b3b9e85` |
| GATE 5 | Workspace build/test: dep graph, topological order, cycle detection, dev-dep isolation | `5b32edd` |
| GATE 6 | Final verification + consolidated report | (docs only) |

Evidence: `docs/audit/P2-GATE1..5-*.md`, `docs/audit/P2-FINAL-REPORT.md`.
Full `go test ./... -count=1` suite green across all 26 packages.

Recommended next stages (priority order):
1. Wire manifest dependency resolution into build/run/check (kept out of P2 to
   keep the compiler pipeline stable).
2. Public registry deployment (protocol v1 defined; no production server yet).
3. Transitive registry/git dependency expansion behind real host access.
