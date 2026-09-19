# Phase 127+ — Post-GA Roadmap (Karkain 1.0.0 Stable)

> **SUPERSEDED for planning** — the authoritative, milestone-based plan is now
> [`GENERAL-AVAILABILITY-ROADMAP.md`](GENERAL-AVAILABILITY-ROADMAP.md)
> (milestones GA-1 Ship & Harden → GA-2 Language & Tooling Completeness →
> GA-3 Platform Expansion & 1.1.0; phases 128–140 with per-phase gates).
> This file is retained as the phase-by-phase reference table that the GA
> roadmap restructures.

**Baseline**: Karkain 1.0.0 (Stable), last verified green at Phase 126 (`std.numerics`, corpus 59/59 byte-identical).
**Constraint**: 4GB RAM Windows host — environmental limitations documented in Phase 127.
**Immediate prerequisite**: Phase 128 (cut `v1.0.0` tag) → Phase 129 (4 GB bootstrap battle) → then proceed along the GA roadmap.

---

## Proposed Phase Order (Flexible, Not Crated in Stone)

| Phase | Theme | Target Completion | Success Metric |
|-------|-------|-------------------|----------------|
| **127** | **Windows Bootstrap Hardening** | 2a | `TestBootstrap_BitwiseIdentity` passes with 5min timeout; no SEGFAULT; clean abort with `error[K124]` if pressure too high |
| **128** | **Incremental Compilation v2** | 2b | Per-module `.o` cache: no-op `karkain build` ≤ 100ms vs clean 2790ms (Phase 105 baseline); byte-identical C/stdout on re-build |
| **129** | **Package Registry (Beta)** | 3a | `karkain pkg publish` + `karkain pkg install` end-to-end on local repo; lockfile integrity verified |
| **130** | **Language Server v2** | 3b | `karkain lsp` serves semantic tokens, goto-definition, hover; VS Code extension `extension.js` commands functional |
| **131** | **Closure / `fn` Codegen** | 3c | Both Go and kcc engines compile `fn` closures without ICE; at least one golden corpus program passes check |
| **132** | **Const Evaluation** | 4a | `const` folding works; generic const params compile; at least 3 golden test programs |
| **133** | **Async / Await Runtime** | 4b | `std.async` module + `spawn`/`receive`/`actor` integrated end-to-end; at least one concurrency golden program |
| **134** | **GPU Compute (WGSL) v2** | 4c | `pkg/backend/gpu` emits WGSL; at least one compute kernel runs on ANGLE/Vulkan backend |
| **135** | **WASM GC / Component Model** | 5a | Phase 108 `wasm32-wasi` → add GC types, `wit` bindings; at least one WASM component builds & links |
| **136** | **Cross-Compilation Matrix Expansion** | 5b | `aarch64-linux`, `riscv64-linux`, `x86_64-macos` triples: cross-linker searched, deterministic error if missing |
| **137** | **Profiling v2 (kcc Parity)** | 5c | `karkain prof` on kcc engine: text report + JSON schema + folded stacks; fib(18)=8361 fid order stable |
| **138** | **Debugger Integration** | 5d | DWARF sections + `readelf` round-trip; VS Code `launch.json` attaches to `karkain.exe` generated binary |
| **139** | **Standard Library v3** | 5e | `std.crypto` hardware accel (`sha256-hw`), `std.compress` (zlib), `std.image` (basic BMP/PNG load); byte-identical both engines |
| **140** | **1.1.0 Release** | 6a | Semantic versioning bump; deprecation policy documented; LTS branch created; all above phases green |

---

## Phase 127 Detail: Windows Bootstrap Hardening

**Problem** (from Phase 124): 
- `TestBootstrap_Stage2SelfHosting` → SEGFAULT (exit 0xc0000005) on 4GB host during kcc stage-2 build
- `TestBootstrap_BitwiseIdentity` → exceeds 120s subprocess timeout (clean transpile ~145–174s)

**Scope** (do not exceed — keep < 4h effort):

| Sub-task | Description | Estimated |
|----------|-------------|-----------|
| 127-A | Add `runtime/debug` guard in `pkg/bootstrap/stage2.go`: check `HeapInuse()` > 3 GB → abort with `error[K124]` + message | 2h |
| 127-B | Increase test harness timeout in `pkg/cli/phase99_selfhosted_test.go` from 2min → 5min (already noted in AGENTS.md) | 30m |
| 127-C | Add `GOMEMLIMIT` hint to CI `.github/workflows/ci.yml` for `go test ./pkg/...` | 30m |
| 127-D | Update AGENTS.md bootstrap timeout note: `2min→5min` (measured ~145–174s) | 30m |

**Success Criteria**:
- [ ] `go test ./pkg/bootstrap/... -count=1 -timeout 5m` does not SEGFAULT on 4GB host
- [ ] If heap > 3 GB, test exits 3 with `error[K124]` message (no crash)
- [ ] CI pipeline `go test ./pkg/...` runs with `-p 2` or `GOMEMLIMIT=1.5G` without OOM

---

## Guiding Principles (unchanged from AGENTS.md)

1. **Maturity over features** — depth, correctness, performance, verification before expansion
2. **Semantic foundations first** — value representation, ownership, lifetimes, IR before features
3. **Working > ambitious** — fix broken fundamentals before adding new capabilities
4. **Incremental verification** — every phase must compile, pass E2E, pass all tests

---

## Risk Matrix

| Phase | High-Risk If | Mitigation |
|-------|--------------|------------|
| 127 | OOM guard incomplete → still SEGFAULT | Add `runtime.GC()` before check; test on both 4GB and 8GB hosts |
| 131 | Closure codegen broken on both engines → waste of time | First verify: `karkain check` of `examples/closures/` — if ICE, skip to 132 |
| 134 | GPU backend requires vendor SDK → blocker | Keep backend abstract; only emit WGSL comment stub until hardware available |
| 136 | Cross-triple links unverifiable on Windows host | Use CI matrix; only require that search logic is correct, not that artifacts build |

---

## Immediate Next Steps (Owner Decision)

1. **Phase 119A**: Cut `v1.0.0` tag on CI release pipeline (zero code changes). Update `installation.rst` from "Planned" → actual download links.
2. **Phase 124**: Apply bootstrap OOM fixes (Phase 124 spec already written). Owner chooses 124-A only vs 124-A+B.
3. **Phase 127**: Begin with 127-A (bootstrap guard). Verify on this 4GB Windows host.
4. **After 127 green**: Pull next phase from roadmap based on team priority (language features vs tooling).

---

*Roadmap generated from AGENTS.md audit trail (Phases 119–126 completed). Phase numbering follows project convention. All phases require: compile ✓, E2E ✓, all unit tests ✓, `go vet` ✓, `go build ./...` ✓ before marking complete.*