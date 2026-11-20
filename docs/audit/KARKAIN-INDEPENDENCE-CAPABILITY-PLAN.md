# Karkain — Full Independence Capability Plan

**Status:** PLANNING — owner decision required before implementation.
**Authoritative docs this plan serves:** `docs/audit/KARKAIN-INDEPENDENCE-ROADMAP.md`
(I-0…I-6), `docs/audit/KARKAIN-VERSION-PLAN.md`, `docs/audit/KARKAIN-FULL-LANGUAGE-MILESTONES.md`
(M0–M9), `docs/release/v1.2.0-CHECKLIST.md`.
**Version line:** `1.2.x` (owner-authorized, `x = 1..n`); implementation identity stays
`1.2.0`; tag `v1.2.0` (`77d70693…`) immutable.

## 1. Objective

Make Karkain a **general-purpose, self-sufficient language**: every Karkain facility
runs from Karkain source with **no C compiler and no Go toolchain on PATH**, while
`--target c23` remains an optional compatibility target. This closes the four UNMET
conditions carried out of v1.2.0 (2: `stdlib_v2` no-C; 3: `std.net` no-C; 5:
`stage2 == stage3`; 6: no-Go self-bootstrap) and completes the independence tracks.

## 2. Current position (measured)

| Capability | Status today |
|---|---|
| C-free native emission | COMPLETE — all three targets (`native-x86_64-{windows,linux,macos}`) owned by kcc through one driver (`kccOwnsNativeTargets = true`) |
| PE execution | PROVEN on Windows host; ELF byte-identical, Linux CI leg; Mach-O structural only (no macOS runner) |
| PM/assembly in kcc | PARTIAL — flat projects self-assembled; manifest/workspace/registry transitive resolution keeps Go BRIDGE |
| KIR | Engine invariant + on-demand text; **does not feed any backend** (declared LIMITATION) |
| Go-only emitters | concurrency/profiling/SIMD/DWARF/cross-driver remain Go-path (KCC parity partial for some) |
| Native function calls | Native driver lowers **no** user function calls — blocks stdlib (`std.*` is all calls) |
| Native sockets | No socket emitter in `pkg/native` — blocks `std.net` |
| Self-bootstrap | 154 implemented but capacity/runner-blocked; stage-1 Go pin retired, proof carried |
| stdlib | 16 modules / ~414 functions, 11 importable; OS basics incomplete |

## 3. Independence tracks I-0…I-6 (mapped from the authority)

| Track | Removes dependency on | Current | Work required |
|---|---|---|---|
| **I-0** | nothing (unblock) | mostly done | direct-kcc unblock, -lws2_32 fix |
| **I-1** | enables Go-removal | partial | OS stdlib foundation: `std.path/env/time/random/json`, both-engine goldens, `stable-api.rst` rows |
| **I-2** | Go assembly/PM BRIDGE | partial (flat only) | port manifest/workspace/registry resolution into `assembleProject`; Go side shrinks to file-mirror + spawn |
| **I-2b** | Go scaffolding | not started | `kcc init/new` byte-identical to Go `InitProject` |
| **I-3** | Go preflight text composition | NOT STARTED | make **KIR v1 the backend input** — the native driver consumes the parsed AST today; KIR must drive codegen on both engines |
| **I-4** | Go-only emitters | partial | KCC parity for concurrency (137 done), prof, SIMD, DWARF, cross-driver; `scope.rst` Experimental→Stable where proven |
| **I-5** | gcc at native link | `hello` proven; stdlib/net UNMET | native function-call lowering + native socket emitter + `stdlib_v2`/`std.net` no-C programs |
| **I-6** | Go toolchain for self-bootstrap | implementation present, proof blocked | `stage2 == stage3` with Go absent; deterministic on a ≥1.5 GiB runner (K127 guard) |

**Rule (from the authority):** Go-removal first with the C path as safety net; native-link second. Never both at once.

## 4. Capability slices (dependency ordered, version 1.2.x)

| Slice | Capability | Deliverable | Gate | Version |
|---|---|---|---|---|
| **C0** | Baseline + guards | record version/tag; K127 guard; CI memory cap | existing 151/127 gates | 1.2.0 |
| **C1** | OS stdlib foundation (I-1) | `std.path/env/time/random/json` on both engines | both-engine byte goldens | 1.2.1 |
| **C2** | Native call lowering (I-5a) | kcc + Go native backend lowers real user function calls | E2E program calling a user function in a native image | 1.2.2 |
| **C3** | stdlib native (I-5b) | `stdlib_v2` program builds/runs with gcc/clang/cl absent | no-C gate (PHPATH scrub) | 1.2.3 |
| **C4** | Native sockets (I-5c) | socket emitter in native backend per-OS; explicit platform-support decision | std.net program no-C on each claimed OS | 1.2.4 |
| **C5** | Assembly/PM full (I-2/2b) | manifest+workspace+registry in `assembleProject`; Go BRIDGE deleted; `kcc init/new` | phase122/135 gates re-green with Go absent | 1.2.5 |
| **C6** | KIR backend (I-3) | KIR v1 text becomes codegen input on both engines | corpus goldens byte-identical; `kir --verify` stable | 1.2.6 |
| **C7** | Emitter parity (I-4) | KCC prof/SIMD/DWARF/cross-driver parity | phase106/110/104/111 gates on kcc | 1.2.7 |
| **C8** | Self-bootstrap (I-6) | `stage2 == stage3` with Go absent, on CI runner | phase154 full three-stage gate green | 1.2.8 |
| **C9** | General-purpose completion | stdlib vertical+horizontal (M4/M5), umbrella docs, migration guide, full QA | full QA battery green | 1.2.9+ |

Each slice is a version-tagged increment under the authorized `1.2.x` line (owner chooses
the actual patch number at each release step). Slices are **not** new numbered phases.

## 5. General-purpose language framing

Karkain becomes general-purpose when, in addition to independence:

1. **Type-first language complete** — M1 (401 legacy declarations → 0, generics v2).
2. **Consistency + concurrency** — M2/M3 (memory model, atomics/litmus, `select`).
3. **Stdlib depth and breadth** — M4/M5 (16 modules deep + json/log/cli/fs/compress/regex/sync).
4. **Portability** — M6 (WASM universal artifact + more native triples).
5. **Tooling** — M7 (LSP, debugger decision, editor, docs).
6. **Independence** — this plan, C0–C9.

## 6. Risks and controls

| Risk | Control |
|---|---|
| 4 GB host OOM (K127, gcc kills) | focused local gates; full bootstrap/GPU/native on CI with ≥1.5 GiB |
| Removing Go before native calls work breaks stdlib | strict order C2 → C3; safety net until C3 green |
| Native socket semantics vary by OS | explicit platform-support decision in C4; deterministic rejects where unsupported |
| KIR-as-backend changes emitted bytes | differential goldens per slice; re-measure pins, never predict |
| Unverifiable claims | PASS/FAIL/PARTIAL/BLOCKED/NOT RUN labels with evidence only |
| Version drift | implementation identity `1.2.0`; bumps only at owner release step |

## 7. Owner decisions required

1. **Open the `1.2.x` increments** C0–C9 under the patch line, or keep them planned until a 1.3.0 decision.
2. **Which patch number** per slice (default: next available `1.2.N`).
3. **Native socket platform matrix** — which OSes are genuinely supported in C4.
4. **KBC/JIT fate** — promote to a real VM (GC + exceptions) or deprecate in favour of WASM (M6).
5. **Merge PR #3** before starting C-slices (they build on the type-first tree).

## 8. Honest current position

PR #3 contains the verified type-first work and is OPEN (not merged). Independence today:
native images are real and self-hosted, but stdlib/net no-C, KIR-as-backend, full emitter
parity, and the no-Go bootstrap proof are **not** done. This plan is the dependency-ordered
path to close them. No implementation of C0–C9 has started; no version/tag/release changed.
