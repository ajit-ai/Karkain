# Karkain Architecture Roadmap

**Status**: authoritative architecture view (how the compiler is layered today,
and which layer changes hands in which release).
**Baseline**: increment **150** in progress — slices 150A–150C complete, 150D
not started. `VERSION` = `1.1.0`, `main` @ `da5082c`.
**Role in the document set** (this file does not replace any of them):

| Document | Role |
|---|---|
| `KARKAIN-VERSION-PLAN.md` | release strategy, version ↔ increment matrix, per-increment gates |
| `KARKAIN-INDEPENDENCE-ROADMAP.md` | the I/S/L/P/D work tracks and the stdlib backlog |
| `PHASE-150-BASELINE.md` | the current increment's slice plan and scope |
| `AGENTS.md` | completion record + branch/cadence rules |
| `docs/inventory/compiler-dependencies.json` | machine-readable layer ownership |

Every capability claim below is **measured against the tree**, not aspirational.
§6 lists, explicitly, what the architecture does *not* yet do.

---

## 1. Where the architecture is today

Two engines exist and both are real. The **self-hosted engine (`kcc`, written in
Karkain, `src/compiler/*.kark`) has been the default since increment 97**; the Go
front end remains available and is selected with `KARKAIN_ENGINE=go`.

```text
                    ┌──────────────────────────────┐
   .kark source ───▶ │  kcc  (self-hosted, Karkain) │  ← DEFAULT since 97
                    │  lexer → parser → checker     │
                    │  → sema → codegen(C23)        │
                    │  → KIR v1 emit / verify       │
                    │  → project assembly (122)     │
                    └──────────────┬───────────────┘
                                   │ emits C23
                                   ▼
                            gcc / clang / MSVC
                                   │
                                   ▼
                          native executable

   ┌──────────────────────────────────────────────────────────────┐
   │ Go implementation (pkg/*) — reference engine + CLI + backends │
   │   language/frontend · sema · KIR composition · SSA            │
   │   NATIVE BACKEND (pkg/native: ELF/PE/Mach-O, no linker)      │
   │   WASM backend · C backend · PM · LSP bridges · CLI           │
   └──────────────────────────────────────────────────────────────┘
```

**Measured ownership today** — who owns which layer:

| Layer | Owner today | Note |
|---|---|---|
| Lexer / parser / AST | **kcc** (mirrored by Go) | Go engine still selectable |
| Checker + sema | **kcc** (mirrored by Go) | kcc is the stricter of the two (K101–K115) |
| C23 code generation | **kcc** (mirrored by Go) | this is why a C compiler is still needed |
| KIR v1 emit + verify | **kcc** | emitted and structurally verified; **not yet consumed as backend input** (§6) |
| Project assembly / module resolution | **kcc** | since 122; Go reduced to a bridge |
| Package-manager resolution | **kcc** | since 144; Go bridge remains for composition |
| SSA / optimisation | Go (`pkg/ir/ssa`) | drives a gated corpus (141), not the full path |
| **Native backend** | **Go** (`pkg/native`) | emits ELF/PE/Mach-O directly; **executed on PE and ELF** |

## 2. Where it is going

### 2.1 Sovereignty I (v1.2.0)

```text
                              kcc  (Karkain)
                                 │
      ┌──────────────────────────┼──────────────────────────┐
      │                          │                          │
      ▼                          ▼                          ▼
  lexer/parser/sema        compiler pipeline        native code generation
      │                          │                          │
      │                          │                          ▼
      │                          │                   native targets
      │                          │              (ELF / PE / Mach-O)
      │                          ▼
      │                    native stdlib
      │                  (no libc, no gcc)
      ▼
   Go implementation: reference oracle only, never on the user path
```

### 2.2 The sovereignty objective

```text
.kark  ──▶  kcc  ──▶  Karkain compiler pipeline  ──▶  native backend  ──▶  native executable
```

with **no Go toolchain and no C compiler** required for the defined closure.

Measured status of each step: `kcc` exists and is the default engine; the
native backend exists and executes; the native stdlib is **partial** (§6); the
self-hosted build and the C-free toolchain build are **not yet achieved**
(increments 152 and 154).

## 3. Three distinct roles — not one

## 4. Compiler dependency layers

```text
Source Language
      ↓
Lexer
      ↓
Parser / AST
      ↓
Semantic Analysis
      ↓
KIR / SSA
      ↓
Optimization
      ↓
Native Code Generation
      ↓
Object / Image Writer
      ↓
OS / Target Runtime
```

| Layer | Current | Future | Owning increment |
|---|---|---|---|
| Frontend (lexer → sema) | increasingly **kcc**-owned | kcc-only | complete (97, 122) |
| Compiler pipeline (assembly → codegen) | increasingly **kcc**-owned | kcc-only | 151 |
| KIR / SSA | shared / reference during the transition | backend reads KIR v1 | 1.4.0 |
| Native backend | Go-owned, executes | kcc-owned, byte-identical | 151 |
| Stdlib | native / no-C transition | OS modules owned by Karkain | 152, then 1.3.0 |
| Runtime | native / self-hosted transition | Karkain-owned arena + syscalls | 152, 1.5.0 (freestanding) |
| Go compiler | **reference oracle** | reference only, off the user path | 154 |
| C compiler | **compatibility path, not the sovereignty path** | optional, `--target c23` only | 152 |

## 5. Release architecture milestones

| Increment | Architectural meaning |
|---|---|
| **150** — Native foundation | Native execution capability and the three image writers (ELF/PE/Mach-O): value model, arena, register allocation. |
| **151** — Native parity | `kcc` produces the **same** native output as the Go reference, byte for byte. The native backend stops being Go-only. |
| **152** — No-C native closure | Native stdlib and compiler execution **without an external C compiler** for the defined closure. |
| **154** — Self-bootstrap | `kcc` builds `kcc`; the Go toolchain leaves the stage-1 production path. |
| **165** — Distribution / CI | Docker + GHCR. **Independent of compiler sovereignty** — packaging only, and it carries no compiler risk. |

Later progression:

```text

## 6. What the architecture does **not** yet do

Stated plainly, because these are the claims most likely to be assumed:

1. **`kcc` has no native emitter.** It emits C23 only — `src/compiler/` has no
   machine-code backend. That is why 151–152 are required before the no-C
   closure is real, and why a C compiler is still needed to *build the compiler*.
2. **KIR is emitted and verified, not consumed.** `kir --verify` checks the
   structural contract, but no backend reads KIR v1 as its input today.
   KIR-consumption is 1.4.0 work (track I-3).
3. **SSA is not on the production path.** It drives a gated corpus (141); it is
   not the optimiser for the whole pipeline.
4. **The OS stdlib modules do not exist.** There is no `std.path`, `std.env`,
   `std.time`, `std.random` or `std.json` source. They open 1.3.0 (increment
   155), and they are what lets a Karkain program do OS work without libc.
5. **Native coverage is a subset.** `std.net` / `std.http` / `std.db` and the
   unsupported value kinds are refused deterministically on the native target;
   the supported surface is ints, floats, arrays, `for-in`, maps, records and
   string operations.
6. **Building the compiler without a C compiler is not scheduled.** After 154
   the toolchain is Go-free but still C-linked (bootstrap transpiles `kcc` to C).
   Closing that needs `kcc` to compile and link *itself* natively — no increment,
   no baseline, no gate. It is the last borrowed dependency and the only one
   without a scheduled removal.

## 7. Cross-references

* Per-increment deliverables and gates: `KARKAIN-VERSION-PLAN.md` §4
* Work tracks (I/S/L/P/D) and the stdlib backlog: `KARKAIN-INDEPENDENCE-ROADMAP.md`
* The current increment's slices and scope: `PHASE-150-BASELINE.md`
* Machine-readable ownership: `docs/inventory/compiler-dependencies.json`
* Completion record and cadence rules: `AGENTS.md`

v1.2.0  Sovereignty I
   ↓   no Go to build · no C to run
v1.3.0  Language + library completeness      (own the OS stdlib)
   ↓
v1.4.0  Tooling + observability              (KIR consumed by the backend)
   ↓
v1.5.0  Platform + package breadth           (freestanding, installers, Android)
   ↓
v1.6.0  Performance / resource maturity
   ↓
v2.0.0  Breaking layout + hardware model     (C-layout structs, MMIO, asm)
```


| Role | What it is | Fate |
|---|---|---|
| **Reference implementation** | The Go front end in `pkg/*`. It is the **differential oracle**: self-hosted output is accepted only when it is byte-identical to Go's. | Kept **forever**, archived as `reference/` at increment 154. Never on the user path. This is the safety net, not a crutch. |
| **Target architecture** | `kcc` as the production compiler: it owns the frontend, the pipeline, native emission and the native stdlib. | Reached incrementally across 151 → 152 → 154. |
| **Sovereignty objective** | The toolchain builds and runs with neither Go nor C. | Two of the three properties land in v1.2.0; the third is not yet scheduled (§6). |

| WASM backend | Go (`pkg/wasm`) | compile-only for the accelerator rows |
| CLI, PM, LSP scaffolding | Go | thin routing over the two engines |
| Bootstrap driver | Go (`pkg/bootstrap`) | stage 1 pins `KARKAIN_ENGINE=go`; stages 2–3 link with gcc |
