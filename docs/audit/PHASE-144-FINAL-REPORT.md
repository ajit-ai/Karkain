# Phase 144 FINAL REPORT — Toolchain Sovereignty (PM resolution)

Verdict: **COMPLETE**. The self-hosted engine assembles manifest/
workspace/registry dependencies itself; the CLI mirrors and injects
nothing on kcc check/build/run paths.

## A. kcc-native resolution (144A)

- `src/compiler/main.kark` gains a PM section (16 functions):
  manifest `[dependencies]` parse (flat name/source/url/version records,
  dev-deps excluded), lock version/rev lookup, workspace-root discovery,
  `sanitizeCacheComponent` + `depCacheName` + `normalizeSourceURL`
  mirrors, `depSourceDir` (workspace/local/registry/git switch mirroring
  Go, existence-check-free by design: empty joins are observationally
  identical to Go's silent skip), `depDirPaths` + single-pass join with
  main-skip, path-dedup and root-exclusion. Prepended upstream-first in
  `assembleProject` for BOTH shapes (imports + siblings).
- Unresolvable deps skip silently (Go parity); imported-but-missing
  modules keep `error[K122]`. No new error codes.
- Two real defects caught during development by the project's own
  machinery: (1) `fileExists` is not a Go-resolver builtin (K002) — the
  design dropped all existence checks instead of adding surface;
  (2) the sibling branch ASSIGNED instead of appending, discarding dep
  content — found by isolating `projectDepSources` in a kcc-run harness
  (all stages proven correct there) and fixed with one `+`.
- `pkg/cli/kcc_engine.go`: `kccMirrorProject` (anchor-relative mirror of
  `*.kark` + toml + lock + workspace marker; absolute deps resolve live
  via the same-machine sandbox property) + `pmWorkspaceRoot`;
  `kccStageInput` routes manifest projects to the mirror (flat/legacy
  paths untouched). Go-engine assembly and kcc test injection UNCHANGED.

## B. Tools contract (144B)

Recorded in `docs/inventory/compiler-dependencies.json`
(`tools_contract_144`): LSP/PM-commands/prof/incremental/wasm-wit/dbg
stay explicit Go-only frozen boundaries; resolution-for-assembly is the
one surface that migrated; kcc test-driver assembly is the documented
next slice.

## C. Gates + CI + regressions

- `pkg/cli/phase144_pm_test.go` (new, 6/6 PASS): direct-kcc workspace
  proof (zero Go bridge), CLI both-engine parity (`hi from library
  api/hi from app`), missing-dep negatives both paths, registry-cache
  shape (`.karkain/cache/<name@version>` computed in Karkain),
  CLI build+kir, wiring presence.
- CI: `TestPhase144` step.
- Regressions: Phase 97 + Phase 122 gates green after the reroute;
  `go vet`/`go build` clean; kcc self-check green on the extended
  compiler sources.

## D. Boundaries (documented, NOT defects)

- Manifest projects WITH dotted imports assemble directory-flat on Go
  but import-directed on kcc (pre-existing Phase 122 class, unchanged).
- `contains(src, "func main(")` main-skip matches kcc's siblingContent
  approximation, not Go's line-anchored regex (consistent in-engine).
- kcc test drivers stay Go-injected (next slice); absolute-url deps
  resolve live (same-machine sandbox property).
