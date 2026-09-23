# Karkain 1.1.0 (Stable) — Release Notes

**Status:** Stable release line (successor to 1.0.0, which continues as the
`1.0.x` LTS line for security fixes). Minor bump per the SemVer policy
(`docs/source/development/semver-policy.rst`): strictly additive surface,
no breaking changes, full backward compatibility with 1.0.0 programs.

**Release cut is owner-only** (tag `v1.1.0` + CI release job publishing the
13-archive set with `checksums.txt`). This document + the 142 gate prepare
everything up to the ceremony.

## What changed since 1.0.0 (Phases 132–141)

- **Standard library frozen** (132): 10 modules stable with SemVer +
  deprecation policy; stdlib freeze gate green on both engines.
- **First-class `fn` values** (133): closures flow through params, arrays
  and returns, invoked through values, byte-identical on both engines;
  escaping closures rejected at check (K114).
- **Incremental compilation v2** (134): per-module translation units,
  content-keyed object cache, manifest v2, `3 reused` no-op rebuilds.
- **Local package registry** (135): directory registry, immutable
  versions, digest-verified fetch, offline-first.
- **LSP v2** (136): semantic tokens, hover, go-to-definition, scoped +
  member completion; server 1.0.0 → 1.1.0; VS Code client wiring.
- **Concurrency parity** (137): kcc runs spawn/channel/actor programs
  byte-identical to Go; 08-concurrency examples graduated to Stable.
- **Accelerator kernels** (138): `@target(gpu)` WGSL emission with
  compile-only guarantee; unknown targets rejected on both engines.
- **Cross-compilation expansion** (139): `aarch64-windows`,
  `riscv64-linux` (parse + error paths), `x86_64-macos`/`aarch64-macos`
  (clang-only); WASM struct values; component envelope; `karkain wit`.
- **Debugger integration** (140): `karkain dbg` live-gdb Karkain-level
  backtraces; VS Code debug command + launch/tasks templates.
- **Optimizer + memory + profiling** (141): SimplifyCFG pass (constant-br
  folding, dead-arm sweep); Value-cell allocation counts in `prof`;
  measured RSS table (kcc check 544.5 MB, go build 229.9 MB, gcc big-TU
  484 MB); loop-soundness verdicts per pass (Mem2Reg stays unwired).
- **Standard library networking/database/web** (125A): `std.net`,
  `std.http`, `std.db` with the Windows Winsock link contract on every
  linker; **numerics** (126): `std.numerics` 40-func module.
- **Example corpus 59 → 61**: `03_mlp_forward` pinned; 08-concurrency
  Stable/both; metadata gate teaches the Stable status.

## Compatibility

- Every 1.0.0 program builds and runs identically (conformance 64/64,
  Phase 114 corpus byte-identical Go↔kcc).
- Additive surfaces only: `@target(gpu)`, new triples, `karkain wit`,
  `karkain dbg`, `cells` profile field, K114 escape rejection (previously
  accepted-and-miscompiled — the one behavior fix, and it only ever
  rejected programs that could not work).

## Known limitations (honest, unchanged-or-narrowed)

- No public package registry (local only); no JIT; no native backend
  (C/gcc still required); bootstrap needs ≥1.5 GiB free (K127 aborts
  cleanly otherwise); `func(T) R` type syntax, capture snapshots,
  generics, async/await stay Planned.
- riscv64-linux is parse + error paths only; macOS builds need clang +
  Apple SDK; lldb/DAP stay future work (153).

## Verifying the release

Per `docs/source/getting-started/installation.rst`: download the archive
for your platform, verify SHA-256 against the published `checksums.txt`,
run `karkain --version` (must read `v1.1.0 (... Stable Build)`),
`scripts/verify-rc-journey.ps1`, and `scripts/verify-install.ps1`.
