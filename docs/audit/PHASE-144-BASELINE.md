# Phase 144 BASELINE — Toolchain Sovereignty (PM resolution)

Date: 2026-09-23. Target: SOVEREIGNTY second step (after 143).

## 1. The bridge (Go side, to be migrated for check/build/run)

- `pm.DependencySources` (depsrc.go): manifest `[dependencies]` (dev
  excluded) → per-dep dirs. workspace (ws-root-relative else
  project-relative URL, else deps/<name>), local (project-relative URL,
  else deps/<name>), registry/git (`<project>/.karkain/cache/<name>` —
  project-relative, exists-gated). Sorted by name.
- `projectSourceFiles` (commands.go): dep dirs (each: top `*.kark` +
  `src/*.kark`) then target-dir siblings. `resolveSources VN` skips every
  file containing `func main` except the root, appended last, with
  abs-path dedup.
- Manifest format (manager.go): line-based, `#` comments, `[dependencies]`
  section; entries `name = { version, source, url }` or bare `"version"`
  (registry default `*`); stripQuotes handles both quote kinds.
- Lock (`karkain.lock`): `[[package]]` blocks with name/version/rev.
- Cache names (source.go): registry `name@version` (no url),
  `CacheIdentityKey` (registry with url: name@ver+source+normurl),
  git `name@rev-or-version-or-head`, local/workspace `name`.
  normalizeSourceURL = trim-space + trim `.git` suffix.
- `flatAssemblyEligible` returns FALSE for manifest projects → they take
  `kccAssembleSource` (Go injection). kcc `test` injection
  (`kccTestSource`) stays Go-side this phase (test-runner migration is
  the documented next slice).

## 2. kcc capabilities (verified by probe)

- Builtins: openFile/readFile/fileExists/listFiles/createFile/removeFile,
  trim/contains/split; stdlib.kark: hasPrefix/indexOf/trimPrefix...;
  `s[i]` yields comparable single-char strings; `listFiles` + manual
  sort gives sorted joins (collectKarkFiles/moduleDirSource/siblingContent
  already mirror the Go directory logic).
- `fileExists` on directories: assumed stat-based (gate proves it — dep
  dirs must resolve or the e2e fails loudly).

## 3. Migration design (144A)

- kcc (main.kark): findProjectRoot (karkain.toml walk-up), manifest dep
  parse (flat name/source/url/version records), lock version/rev lookup,
  findWorkspaceRoot, sanitize + cacheDirName mirrors, depSourceDir
  (Go-switch mirror), projectDepSources (deps first, main-skip,
  path-dedup, root-exclusion), prepended in assembleProject for BOTH
  shapes (imports + siblings). Unresolvable deps skip silently (Go
  parity); imported-but-missing modules keep error[K122].
- Go (kcc_engine.go): new kccMirrorProject (mirror project tree .kark +
  toml + lock + workspace.json + .karkain/cache; out-of-tree dep dirs
  → sandbox/.deps/<name>/ top + src/); kccStageInput routes manifest
  projects there instead of kccAssembleSource. Go-engine assembly and
  kcc test injection UNTOUCHED.
- Known accepted divergence (pre-existing 122 class, documented): a
  manifest project WITH dotted imports assembles directory-flat on Go
  but import-directed on kcc. The gate proves the no-import manifest
  shape (the ownership target).

## 4. Gate + slices

- `pkg/cli/phase144_pm_test.go`: direct-kcc proof on staged
  examples/workspace (no Go bridge: check [ok], run golden), CLI
  both-engine parity, missing-dep negatives both paths, registry-cache
  shape via .karkain/cache, wiring presence.
- 144A = above. 144B = tools contract (LSP/PM/prof/incremental/wasm/wit/
  dbg: kcc-conformance or frozen boundary, each declared). 144C = docs
  (compiler-dependencies.json), regressions, commit→merge→push.
