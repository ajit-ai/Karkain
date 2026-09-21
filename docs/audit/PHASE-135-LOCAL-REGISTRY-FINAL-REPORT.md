# Phase 135 — Local Registry MVP — Final Report

**Verdict: COMPLETE (local-only).** A package can now be created, published
into a local filesystem registry, indexed, resolved by name + version,
consumed by another local project, compiled and executed — with no network,
no daemon and no database.

## 1. What changed

| File | Change |
|---|---|
| `pkg/pm/local_registry.go` (new) | Directory registry: `InitLocalRegistry` (`registry.json` + `packages/` + `index/`), `ValidatePackageName` (reject empty/separators/traversal/absolute/non-`[A-Za-z0-9._-]`), `ValidateVersion` (existing semver), `PublishLocal` (staged copy + manifest + digest, immutable versions), `ReadLocalIndex`, `ResolveLocalVersion` (exact / `*`/empty / existing `LatestSatisfying`), `FetchLocal` (digest verify + safe copy), `RegistryRef`/`RegistryRefForDep` (flag > dep URL > env; http honestly refused as local-only) |
| `pkg/pm/manager.go` | `FetchModule` keeps its signature (delegates to new `FetchModuleWithRegistry`); registry case branches local (index resolve + digest fetch) vs remote (pre-135 flow byte-identical) |
| `pkg/pm/flow.go` | `FetchLocked` keeps its signature (delegates to new `FetchLockedWithRegistry`); `registeredFetch` threads the flag |
| `cmd/karkain/main.go` | `pkg registry init <dir>`; `--registry <dir>` on `add` (recorded in manifest), `fetch`, `update`, `publish` (local publish, no login); help text |
| `docs/source/tools/packages.rst` | `Registry` section rewritten to the local reality (root selection, layout, version behavior, immutability, limits, explicit local-only) |
| `pkg/pm/local_registry_test.go` (new) | 9 unit tests (init, name/version validation, publish→resolve→fetch, duplicate, unsafe names, missing/mismatch, tamper/corrupt, ref selection) |
| `pkg/cli/phase135_local_registry_test.go` (new) | E2E + negatives through the real binary (see §3) |

Deliberate non-goals (mission hard rules, none introduced): network/remote
registry, auth/accounts, serve/daemon, `search`/`list`/`info` for local,
install command (existing `add`+`fetch` cover it), LSP (136),
concurrency/profiling/SIMD (137), solver/lockfile redesign.

## 2. Design notes

- **Mission layout**: `packages/<name>/<version>/{manifest,source/,sha256}` +
  `index/<name>` JSON. Storage and lookup are separate; the index answers
  `hello → versions` without scanning sources.
- **Reuse, not duplication**: semver (`ParseVersion`/`ParseVersions`/
  `LatestSatisfying`/`Compare`), safe-extraction conventions, atomic
  temp→rename writes, checksum-file recording, manifest/lockfile/cache flow,
  `DependencySources` assembly (import-driven `import <dep>` resolves cached
  registry dirs; non-root mains filtered), `PkgError` codes.
- **Single registry selector**: `--registry <dir>` flag wins, dep URL next,
  `KARKAIN_REGISTRY` env last. Empty/unset is an actionable error naming the
  setup commands — the silent public-URL fallback is gone for local paths;
  explicit http(s) refs keep the pre-135 remote-client behavior unchanged
  (existing tests green).
- **Immutability**: duplicate publish refused with "already published";
  tampered stores fail the digest check; corrupt index JSON is a hard error.

## 3. Gate — `pkg/cli/phase135_local_registry_test.go` 2/2 PASS (16.1s)

E2E (`TestPhase135_LocalRegistryE2E`, 11.5s): `registry init` (layout
asserted) → `new` lib + `public func greeting` → `publish --registry`
(`Published superhello@0.1.0`, index lists it) → `new` app with
`import superhello` → `add` (manifest declares registry dep) → `fetch`
(lockfile pins) → `run src/main.kark` prints `Hello karkain` — the string
can only come from the published library through the real pipeline.

Negatives (`TestPhase135_LocalRegistryNegatives`, 3.3s): missing@1.0.0
fetch fails naming the package + `check` fails on the unresolved import;
duplicate publish fails saying "already published" with index + sources
byte-identical; traversal name `../evil` fails saying "invalid package
name" with zero outside-registry files and a clean `packages/` dir;
`superhello@9.9.9` fetch fails naming the version.

## 4. Regressions — green (actually ran)

- `go build ./...` clean; `go vet ./...` clean (whole tree).
- `pkg/pm` full suite PASS 14.5s (incl. 9 new local-registry tests and all
  pre-existing registry/git/resolver/lock tests — remote-client behavior
  preserved).
- Phase 134 gate 5/5 PASS 17.7s (incremental infra untouched by behavior;
  only additive `WithRegistry` variants call into it).
- `TestCLI_NewListTreeFetch_E2E` + `TestCLI_HelpListsNewCommands` PASS
  (modified `add`/`fetch` CLI paths).
- `gofmt -l` on touched `.go` files is the repo-wide CRLF artifact
  (whole-file diff, identical for pristine files); new files are
  gofmt-clean.
- Debris discipline: no build artifacts in the tree; all registry state in
  `t.TempDir()` (child env only, never process-global).

## 5. Roadmap movement

GA-2 infra track: 135 lands (132 → 135 dependency satisfied). Next: 136
LSP v2, then 137 kcc parity. No 136/137 code in this phase.
