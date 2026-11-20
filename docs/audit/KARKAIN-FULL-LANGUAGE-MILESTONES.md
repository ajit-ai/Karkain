# Karkain — Full-Language Completion Milestones (delivery overlay)

**Status:** PLANNING OVERLAY — not a new version plan, not authorization to implement.
**Authoritative version scope:** `docs/audit/KARKAIN-VERSION-PLAN.md`.
**Implementation queue:** `docs/audit/KARKAIN-IMPLEMENTATION-CONTROL.md`.
**Version policy (owner-authorized 2026-11-19):** the Karkain version line is
**`1.2.x`** — the owner has explicitly authorized patch releases `1.2.1`,
`1.2.2`, … `1.2.n` (`x = 1..n`). During implementation the tree stays on the
`1.2.0` identity (`VERSION=1.2.0`, compiler/LSP banners, generated headers,
pinning tests); a patch bump is applied only in an owner-authorized release step,
where `VERSION` and every identity site flip together to the chosen `1.2.x`.
The `v1.2.0` tag (`77d70693e1e3134dac312a303bffafa7d50fa988`) is historical and
never moved or replaced; a new `v1.2.x` tag is created only at that release step.
No commit/push/merge/tag/release by the agent without explicit authorization.

This document groups the remaining work into **delivery milestones M0–M9**. The
milestone labels are a planning overlay only — they reuse the increment numbers
already defined in `KARKAIN-VERSION-PLAN.md` (155, 156, 158–161, 166–172, …) and do
**not** invent new numbered phases or reassign frozen ones.

## Baseline (measured 2026-11-19)

| Item | Value |
|---|---|
| Branch / HEAD | `feature/v1.2.0-type-first-declarations` / `bbecf1c` |
| PR | #3 OPEN — type-first migration, 218 files, both engines 8/8 |
| Legacy `let` declarations | 401 |
| `.kark` files / stdlib modules | 303 / 16 (≈414 functions, 11 importable) |
| Engines | Go + self-hosted KCC, both rebuilt and qualified on focused gates |
| Native | PE executes on Windows host; ELF byte-identical, CI leg; Mach-O structural only |
| WASM | real emitter + WASI runtime + component model, subset gated (K108 rejects) |
| Consistency | tagged Value model; **no** in-language atomic surface / memory ordering / litmus gates |
| Concurrency | work-stealing scheduler, channels, actors, >1 M msg/s, KCC parity (137) |

## Definition of "full-fledged" (acceptance target)

Karkain is full-fledged when:
1. One consistent, statically typed, type-first language on both engines, with zero genuine legacy declarations.
2. A written memory-consistency model plus in-language atomics with litmus goldens.
3. First-class concurrency/parallelism with parity on both engines and honest native qualification.
4. A standard library that is both **deep** (vertical: full APIs per module) and **wide** (horizontal: json/log/cli/fs/compress/regex/sync).
5. At least one credible universal/portable artifact (WASM) alongside native binaries.
6. Tooling (LSP/debugger/editor), docs, examples, and a clean CI matrix.
7. A completed GA qualification with a version/release decision owned by the owner.

---

## M0 — Land and stabilize (current)
**Goal:** make the type-first work shippable and prove no regressions.
**Scope:** PR #3 review/merge decision; CI `Test`, kcc self-check, native PE, docs jobs green; fix any CI-only failure.
**Exit:** required CI checks green on `main`; `VERSION=1.2.0`, tag untouched.
**Owner decision:** merge PR #3 or keep open.
**Risk:** CI reproducibility — local host is ~4 GB; full corpus runs belong on CI.

## M1 — Finish the type-first language
**Goal:** zero genuine legacy declarations; generics v2; const folding.
**Scope:** the 401 residual (heterogeneous cells → nominal `Value`; call results → explicit signatures; 40 negative fixtures → embedded test strings); generics v2 (the incomplete slice was reverted pre-PR — redo properly on KCC: monomorphizer must substitute generic identifiers in codegen); `const` folding.
**Maps to:** v1.3.0 theme "generics v2 + const folding"; no new phase numbers.
**Exit:** legacy scan = 0 genuine; every textual `let` classified (prose/string/fixture); both engines pass full type-first negative matrix.
**Dependency:** M0.

## M2 — Consistency and memory model
**Goal:** a documented, testable concurrency-correctness contract.
**Scope:** memory-consistency model in SPEC + `language/{concurrency,memory-model}.rst`; in-language `atomic_*` (load/store/CAS/fence) with explicit ordering `relaxed/acquire/release/seq_cst`; same surface in Go sema/codegen and KCC checker/codegen/native; litmus goldens (store buffering, message passing, IRIW, Dekker); concurrency diagnostics with source spans.
**Maps to:** NFR-12, increment **161**.
**Exit:** litmus goldens deterministic on both engines; diagnostics carry K-codes.
**Dependency:** M1 (types must be settled before atomics are typed).

## M3 — Concurrency and parallelism depth
**Goal:** first-class, both-engine, qualified concurrency/parallelism.
**Scope:** fix stale kcc-parity doc; `std.actor` `timeout_recv` + `select` over N channels (needs M2 ordering); non-blocking `std.net` + `select`; SIMD/DWARF/`prof` parity on KCC; real WGSL-surface promotion for GPU; NPU qualification decision.
**Maps to:** increments **158/161** (select), **1.4.0** tooling parity, **156** GPU promotion verdict.
**Exit:** both-engine channel/actor/select goldens; SIMD parity gate on KCC; documented hardware platform-support decisions.
**Dependency:** M2.

## M4 — Standard library, vertical (depth)
**Goal:** complete the APIs of the 16 existing modules.
**Scope (from the version-plan backlog):**
- `std.string`: pad, split_lines, replace_all, to_float, is_numeric, format, unicode case verdict
- `std.collections`: sort/sorted, map_remove/get_default, zip/enumerate, set_*, map/filter/reduce
- `std.io` + `std.system`: MERGE — one home, stat, mkdir_all, stdio handles, buffered I/O
- `std.encoding`: url encode/decode, base32; JSON → `std.json`
- `std.crypto`: hmac_sha256, constant-time secure_eq, streaming sha256, random_bytes
- `std.testing`: approx-eq, skip, suite, bench
- `std.numerics`: broadcasting, det/inv/solve, fft, tensor save/load, autodiff
- `std.net` / `std.http`: udp, timeouts, dns, query/headers/cookies/redirects
**Maps to:** increments **156, 158, 159**.
**Exit:** every added function has both-engine goldens; no Go-only core rows.
**Dependency:** M1.

## M5 — Standard library, horizontal (width)
**Goal:** new modules the language lacks.
**Scope:** `std.json` (from encoding), `std.log`, `std.cli`, `std.fs` (walk/glob), `std.compress` (gzip), `std.regex`, `std.sync` (atomics surface, depends on M2).
**Maps to:** "one per increment after 1.4.0", version plan.
**Exit:** each module importable on both engines, documented, gated.
**Dependency:** M2, M4.

## M6 — Portability / universal artifact
**Goal:** a credible "write once, run anywhere" option alongside native.
**Scope:** close K108 WASM gates (floats/maps/slices/C-interop/concurrency); port all 16 stdlib modules to WASM; WASM GC + component model + `wit`; single `karkain run --target wasm32-wasi` path on 3 OSes; publish `.wasm` artifacts. KBC/JIT: feasibility study only — decide promote to real VM (GC + exceptions + runtime distribution) or deprecate in favour of WASM. More native triples (aarch64, riscv64, freestanding, Android) + documented C fallback.
**Maps to:** increments **166–172**, v1.5.0; KBC/JIT decision at v1.3.0/1.4.0.
**Exit:** one full-language WASM program runs on Windows/Linux/macOS with one `wasmtime`; native matrix documented honestly.
**Dependency:** M2–M5 (types and stdlib must be portable before the runtime is).

## M7 — Tooling and ecosystem
**Goal:** a professional developer experience.
**Scope:** LSP v2 completion; debugger/DAP decision (no DAP exists today — document, do not claim); VS Code grammar/snippets/syntax for type-first + generics + atomics; SPEC type-first rewrite (§3.2/3.3, Appendix A keyword table incl. new primitives and `Value`); Sphinx `-W` + linkcheck; migration guide `let → T x`; example corpus updates; README/spec status labels.
**Maps to:** v1.4.0 tooling theme, increment 153/157/163/164.
**Exit:** editor tests green, Sphinx 0 warnings, spec matches compiler keywords.
**Dependency:** M1 (syntax stable).

## M8 — Qualification and GA decision
**Goal:** evidence complete enough for a release decision.
**Scope:** full QA battery on CI — units, corpus both engines, conformance, probes, all gates, bootstrap stage-1/2/3 with ≥1.5 GiB runner, native PE/ELF/Mach-O jobs, Sphinx, install/verify, rc-journey; honest PASS/FAIL/PARTIAL/NOT RUN matrix; version-identity check.
**Maps to:** v1.2.0/v1.3.0 checklists.
**Exit:** no unexplained red; all UNMET conditions either closed or explicitly carried with owner sign-off.
**Dependency:** M0–M7.

## M9 — Owner release ceremony (not performed by agent)
**Goal:** publish.
**Scope:** owner decision on version identity (`1.2.1` patch vs `1.3.0`); re-point or create tag; CI release job; GitHub Release; Pages; post-publish verification against shipped binary.
**Constraint:** agent never creates tags/releases or publishes without explicit authorization.
**Exit:** shipped artifacts verified.

---

## Dependency chain

```
M0 ──> M1 ──> M2 ──> M3 ──┐
              └──> M4 ──> M5 ──> M6 ──> M8 ──> M9
M1 ──> M7 ──────────────────────────┘
```

## Owner decisions required before dependent work

1. **Merge PR #3?** (M0)
2. **Open v1.3.0?** — currently PREPARED, not open; no increment authorized until v1.2.0 tagged. (M1–M6)
3. **Which `1.2.x` patch number at release** — `1.2.1` by default, or a higher `x`? (M9; the 1.2.x line is authorized, the specific patch is chosen at the ceremony)
4. **KBC/JIT** — promote to real VM or deprecate in favour of WASM? (M6)
5. **Concurrency semantics** — final memory-ordering contract (M2).
6. **Native platform support matrix** — which OS/arch are claimed supported (M3/M6).

## Risk register

| Risk | Mitigation |
|---|---|
| 4 GB host OOM / gcc kills | focused local gates; full suites on CI only |
| Stale generated binaries | delete `karkain.exe`/`kcc.exe` before rebuilds |
| Wrong type inference | AST-based tools + checker validation; revert on any type error |
| Spec drift | keyword table and §3 grammar regenerated from lexer/parser each milestone |
| 1.2.x patch ceiling | owner authorized `x = 1..n`; no minor/major bump authorized |
| Unverified claims | statuses PASS/FAIL/PARTIAL/BLOCKED/NOT RUN only with evidence |

## Honest current position

Only **M0** is in progress. PR #3 contains the verified type-first work but is not merged.
M1–M9 are planned, not started. Karkain today has a real self-hosted native compiler and
a dual-engine type-first surface; it is **not yet** a full-fledged universal language by
the definition above. This document is the path to get there, preserving `VERSION=1.2.0`.
