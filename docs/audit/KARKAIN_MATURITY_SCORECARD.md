# Karkain Language Quality Scorecard

**Status: authoritative (2026-10-15, re-derived at `develop` = `cc44c95`).**

This file supersedes the Phase-49-era maturity table that previously lived here.
That table scored Self-Hosting 1, Generics 1, Modules 1 and "Parser 2 (OOM
defect)" against a codebase in which self-hosting, generics v1, module
resolution, three kcc-owned native targets and LSP v2 have all shipped.

Scope: **documentation-only**. It changes no compiler behaviour, gate, CI step or
release state. Distinct from `docs/audit/KARKAIN-1.1.0-MATURITY-MATRIX.md`, which
is the frozen snapshot taken at v1.1.0; that release snapshot is NOT superseded
by this living scorecard.

---

## 1. Scoring model

| Score | Meaning |
|------:|---------|
| 0 | Not started |
| 1 | Experimental / prototype |
| 2 | Early implementation with known correctness gaps |
| 3 | Functional, gated, documented limitations |
| 4 | Production-oriented, regression-gated in CI |
| 5 | Mature: parity-proven, byte-gated, negative-tested |

`NE` = **Not Evaluable**: no evidence exists in either direction (for example no
runner exists for a platform). NE is reported, never averaged into a grade.

### Rules this scorecard obeys

1. Every score names the file, gate or measurement it rests on.
2. A score whose only evidence is "the source file exists" is **capped at 2**.
3. **No aggregate letter grade is published.** Averaging a byte-proven 5 with an
   NE describes nothing.
4. A dimension covering two sub-features of different maturity takes **the
   lower** score and the limitation column names the weaker half. This is not
   averaging; it is refusing to let a strong half hide a weak one.
5. **Planned is not implemented.** A baselined-but-unbuilt increment scores 0–1
   for delivery; the baseline document evidences *planning*, not capability.
6. A **defined** gate, a gate **executed locally** and a gate **enforced in CI**
   are three different states, distinguished in §5.

---

## 2. Dimension taxonomy — 30 dimensions in 4 categories

### 2.1 Operational maturity (OM1–OM7)

| # | Dimension | Score | Evidence | Limitation |
|---|-----------|------:|----------|------------|
| OM1 | CI enforcement & gate discipline | 4 | Every gate family has a dedicated `.github/workflows/ci.yml` step with a *distinct* `-run` pattern (151A, 151A2, 151A3, 151A5, 151A8a–f, 151A9a–h, 151B, 151C, 151C2, 151C3, 151D, D2, D5); companion rule in `AGENTS.md` | Not every historical gate is mutation-verified; four gates (151P0, 151B, 151C, 151C2) once shipped having never run in CI — the rule exists because of that |
| OM2 | Release reproducibility & version governance | 3 | Stage-1 byte-reproducible across CWDs, SHA `0c01de43f796b153`, intentional-tamper divergence detected (`pkg/bootstrap/phase131_repro_test.go`); version-identity pinning tests move with `VERSION` (88/90/115/118/119/120/142) | v1.2.0 is not cut and the tag ceremony is owner-only; `VERSION` = 1.1.0 while v1.2.0 is in progress is intentional and documented |
| OM3 | Documentation, onboarding & contributor experience | 3 | Sphinx `-W` HTML build + linkcheck, 15 required pages verified in CI; `docs/source/development/` carries contribution rules, semver policy and bug reporting | `ROADMAP.md` is a historical pre-50 log; several `docs/audit/*` documents are point-in-time and now stale (§7) |
| OM4 | Operational recovery & post-publish verification | 2 | `1.0.x` LTS branch published; `scripts/verify-rc-journey.ps1` (11 steps), `scripts/verify-install.ps1`, `scripts/install.ps1` / `install.sh` with deterministic exit codes 0/1/2/3 | No documented **rollback** runbook — recovery tooling exists, operator-facing recovery guidance does not |
| OM5 | Security & supply-chain practices | 2 | `SECURITY.md` including the LTS row, issue templates, SHA-256 `checksums.txt` per release, `semver-policy.rst`; 152-A added a deterministic capability boundary (`net_*` rejected on native) | No SBOM, no artifact signing (cosign is planned in 165), no CVE-handling SLA, and `docs/audit/CAPABILITY_SECURITY_MODEL.md` remains a proposal rather than a shipped model |
| OM6 | Performance & reproducible benchmarks | 2 | `BenchmarkNativeCompile` measures 11–53 µs per program (Phase 150D) — the **only** `Benchmark` function in the repository; RSS measured and recorded (Phase 141: kcc check 544.5 MB, `go build` 229.9 MB, gcc big-TU 484 MB) | No baseline exists, so no regression threshold can be honestly stated; **no performance CI gate**; one benchmark function is not a benchmark program. §4 defines the missing policy |
| OM7 | Robustness, fuzzing & incident tracking | 2 | Parser no-progress guard `advanceIfStalled` (`pkg/parser/parser.go:51`) eliminated the ~3 GB OOM class; project-wide multi-error recovery (Phase 105) reports every parse error in one run; the intermittent kcc `0xC0000005` is tracked as UNCLASSIFIED with three falsified hypotheses on record | **Zero `func Fuzz` targets** and no fuzz CI job; the `0xC0000005` incident has no root cause and no fix; parser robustness rests on a guard plus regression tests, not on fuzzing |

### 2.2 Architecture (AR1–AR9)

| # | Dimension | Score | Evidence | Limitation |
|---|-----------|------:|----------|------------|
| AR1 | Lexical analysis | 4 | `pkg/lexer` and `src/compiler/lexer.kark` token parity; 151P0 added five bitwise tokens to BOTH engines after measuring the self-hosted lexer emitting the *logical*-and token for a single `&`; probes corpus 11/11 | ASCII identifiers only; no block comments (as specified); no Unicode policy |
| AR2 | Parsing (Go engine) | 4 | Full language surface in `pkg/parser`; generics v1 optimistic parse (146); Phase 117 unparenthesized-`while` parity fix; parser suite green | One-token lookahead makes `f[T](x)` ambiguous with `ops[i](x)`, resolved by semantic demotion rather than by the grammar |
| AR3 | Parsing (self-hosted kcc) | 4 | `src/compiler/parser.kark` parses the full surface incl. 146c generics and Phase 137 concurrency keywords; Phase 99 corpus + compiler-sources type-check; LH-2 match-arm-body checking | Whole-tree kcc runs hit the intermittent `0xC0000005` (OM7) |
| AR4 | AST, arena & IR design | 4 | Arena allocation with O(1) `Get` (`pkg/parser/arena.go`); typed SSA IR plus optimizer passes (`pkg/ir/ssa`); **KIR v1 is compiler-owned** — kcc emits and structurally verifies its own IR on the default check path (Phase 121) | AST root is still `interface{}` (nominal typing only); the SSA optimizer does not wire Mem2Reg |
| AR5 | Static checking (Go engine) | 3 | `pkg/sema`: borrow checker with lexical scope and shadowing (Phase 51), module visibility diagnostics (103), NPU target preflight (98), generics monomorphization wiring (146a), K002/K102 gating of build/run (Phase 117) | **No whole-program type checker**; analysis is per-unit plus assembly scope; annotation checking is string-keyed, not a type lattice |
| AR6 | Static checking (kcc) | 3 | `src/compiler/checker.kark` two-pass checker emitting K101–K115 after the Phase 99 type checker; LH-1 `?` propagation and LH-2 match-binding parity both COMPLETE, checkpoint CLOSED | Conservative, AST-provable inference only; cannot express cross-procedure types |
| AR7 | Enum ADT & `match` parity | 4 | Phase 123 byte-identical on both engines (`13_enums` → `1/1/100/200/300/0`, `14_adt_match` → `10/20/30/1`); K106 and K113 fire in both match arms and in expressions | Payload destructuring rejected (tag-only patterns); payloads recorded but semantically dropped |
| AR8 | Closures, first-class `fn` **and** generics v1 | 3 | Closures invoke through `fn` values byte-identically on both engines (Phase 130 capture-mutation `1/2/2/3/3`; Phase 133 higher-order `11/22/6`, array-call, factory, recursion `300`); generics v1 with explicit type args, `std.generics`, and 146c kcc monomorphization | Scored on the **weaker half**: generics constraints parse but are unchecked, there is no trait/impl and no inference, and call sites require explicit type arguments. `func(T) R` type syntax and capture snapshots remain unimplemented |
| AR9 | Module system, name resolution & visibility | 3 | Phase 103 `public` export modifier on both engines with qualified calls and five precise diagnostics; Phase 122 kcc-owned project assembly; Phase 144 kcc-owned PM dependency resolution; 152 no-C capability boundary | `NAME-RESOLUTION-DESIGN.md` remains a **PROPOSAL** (diagnostics-only name table); there is still no per-file visibility default and no trait/module system; `OPTION/Result` bug class (BUG-2) documented but not repaired |

### 2.3 Compiler engineering (CE1–CE8)

| # | Dimension | Score | Evidence | Limitation |
|---|-----------|------:|----------|------------|
| CE1 | Self-hosting bootstrap & toolchain independence | 3 | `pkg/bootstrap` stage2 == stage3 bitwise identical; Phase 143 seed closure proven with `go` scrubbed from `PATH` and `gcc` intact; Phase 154 baselined | **B2 seed-closure burn is UNPROVEN**: five consecutive runner deaths ~35 s into the stage-2 transpile; the `bootstrap-closure` CI job is discovery-only and not a required check. A C compiler is still required for the link step |
| CE2 | Compiler-pipeline ownership | 4 | Phase 121 kcc verifies its own KIR on the default check path; Phase 122 kcc assembles flat projects itself; Phase 144 kcc resolves manifest/workspace/registry/git dependencies; 151D-first, 151D, A1 and Phase B give kcc all three C-free OS targets | A documented Go bridge remains for manifest edge paths; whole-tree `kcc kir --verify` is a multi-minute operation |
| CE3 | Cross-engine differential & conformance discipline | 4 | 59 conformance tests both engines; Phase 114 corpus 61 files byte-identical Go vs kcc; 151B-D3 differential gates against the real Go oracle plus Intel-SDM-stated byte layers, mutation verification and no-Go-fallback guards | Discipline is strongest in the native work; not every historical gate is mutation-verified; the Phase 114 both-legs run exceeds Go's 10-minute default and must be run in isolation |
| CE4 | Native value model, int/float semantics, ABI & platform compatibility | 3 | Increment 150 value model (floats as IEEE-754 bits, arrays, `for-in`, arena, string concat/slice/compare, `push`, records, maps) with executed goldens on PE (Windows host) and ELF (Linux CI); 151A/D, A1 and Phase B give kcc its own containers; Phase 148 documents the calls ABI (6 register units plus extras area); Mach-O is a real PIE with a decoded rebase stream | **Mach-O execution is NE** - no macOS runner exists in CI or on any project host, so every Mach-O claim since increment 149 is structural; no arm64 native; kcc's driver lowers a subset (`let`/`print`/`return`/`len`/index/concat/records) and refuses the rest by name |
| CE5 | C23 backend & concurrency codegen | 4 | SSA to C23 default path on both engines; Phase 107 work-stealing scheduler with channels and actors (>1 M msg/s); Phase 137 kcc concurrency parity byte-identical (`144/10/20/30/0/6`) | No atomics or memory-ordering model; `TestPhase107_CodegenSpawnJoin` is a known ~4 GB-host OOM flake that passes in isolation |
| CE6 | WASM target | 3 | Phase 108 dependency-free handwritten WASM emitter, wasmtime E2E (`hello wasmtime/42/done`), byte-determinism, K108 refusal table | kcc has no WASM path; the surface is deliberately K108-gated |
| CE7 | Diagnostics & runtime error model | 3 | Phases 100/101: source-located runtime errors and stack traces byte-parity on both engines (`inner:2/outer:5/main:9`); numeric codes K001-K117 documented in `karkain explain` and `stable-api.rst`; Phase 105 multi-error recovery | Native C-free targets still use `Int3` traps in the paths kcc has not yet lowered; no error *values* on the native path |
| CE8 | Build characteristics & binary size | 2 | Phase 105 content-addressed cache (no-op 101 ms vs clean 2790 ms); Phase 134 per-module translation units with a self-invalidating runtime key; Phase 150D measured pure-Go native emission at 11-53 us | **No artifact-size or binary-size budget is recorded anywhere**, so the dimension cannot score higher than "measured but unbudgeted"; the native-split cache was refused **on evidence** (Phase 150D) rather than left unconsidered |

### 2.4 Ecosystem (EX1-EX6)

| # | Dimension | Score | Evidence | Limitation |
|---|-----------|------:|----------|------------|
| EX1 | Standard library | 3 | 10 modules frozen (131/132); Phase 125A adds real `std.net`/`std.http`/`std.db` with unconditional net-runtime emission on both engines; Phase 109 NIST and RFC vectors byte-identical; 152-A/B1/B2 ship no-C codecs | `stdlib_v2` and `std.net` **without a C compiler are UNMET** and carried to v1.3.0 verbatim (checklist 2.1); `MM-K-time` is **unassessable** - no inventory row and no module exist |
| EX2 | Package manager & registry | 3 | `pkg/pm`: semver, lockfile, workspace, local registry (Phase 135) with SHA-256 verification, kcc-side resolution (Phase 144); `karkain registry`, `pkg`, `add`, `fetch`, `publish` surface | Remote registry and git fetch are explicit "not available" errors - honest, but it caps ecosystem claims; no signing, no SBOM |
| EX3 | CLI surface & toolchain contract | 4 | Single binary: `check`/`build`/`run`/`test`/`fmt`/`lint`/`dbg`/`prof`/`target`/`pkg`/`wit`/`kir`/`explain`; documented exit codes 0-7; `karkain-diagnostics-v1` JSON contract | Native `run` is restricted to host-runnable targets (refusal plus hint, by design) |
| EX4 | LSP / editor integration | 4 | `pkg/lsp` v2 (Phase 136): semantic tokens, typed and member hover, same-doc/cross-doc/disk definition with 25x no-leak, scoped completion; VS Code client; stdio E2E through the real binary | No rename or refactor; native-target constructs are not diagnosed because the kcc checker is a separate process |
| EX5 | Test runner & conformance corpus | 4 | `karkain test` owned by kcc (Phase 96): discovery, `--filter`, PASS/FAIL summary, per-test drivers; conformance 59/59; `--compile` mode; 61-file golden corpus | The corpus is golden-based, so a wrong value that both engines agree on would pass; the bitwise value gate (151P0) exists precisely because cross-engine agreement alone is weak evidence |
| EX6 | Accelerators & heterogeneous targets | 2 | Phase 106 SIMD lane types; Phase 124 target catalog; Phase 138 `@target(gpu)` emitting real WGSL **compile-only**; Phase 78 NPU INT8/INT4 with CPU reference fallback | GPU/NPU execution evidence is **NE** - no accelerator hardware exists in CI; the CPU reference backend is the only execution oracle; the quantum model is research-stage |

---

## 3. Coverage matrix

Every required evaluation area, and where it is addressed. An area with no
dimension would be stated as such rather than implied.

| Required area | Covered by | Depth of evidence |
|----------------|-----------|--------------------|
| Performance and reproducible benchmarks | OM6, CE8 | **Weak.** One benchmark function and RSS tables; no baseline, no threshold, no CI gate. Section 4 defines the missing policy |
| Binary size and build characteristics | CE8 | **Weak.** Build characteristics measured (105/134/150D); a binary-size budget does not exist |
| Undefined behavior and safety boundaries | CE7, OM5 | **Moderate.** Shift edges, divide-by-zero, index bounds and arena exhaustion all have defined answers; the safety *model* is still a proposal |
| Integer and floating-point semantics | CE4, AR8 | **Strong on the C path** (IEEE-754 bits, `%g`-compatible printing, 151P0 defined shift edges); **absent on the native path**, where kcc lowers no float statement yet |
| ABI and platform compatibility | CE4 | **Moderate.** The calls ABI is documented and implemented; Mach-O execution is NE; no arm64 |
| Fuzzing and parser/compiler robustness | OM7 | **Weak.** A no-progress guard and multi-error recovery exist; zero fuzz targets |
| Security and supply-chain practices | OM5 | **Weak-moderate.** Policy documents and release checksums; no SBOM, no signing, no enforced model |
| Conformance and oracle-based parity | CE3, EX5, AR7 | **Strong.** 59 conformance tests, a 61-file byte-identical corpus, oracle differentials with SDM-stated byte layers |
| Testing and CI enforcement | OM1, EX5 | **Strong.** A distinct CI step per gate family, with a documented companion rule |
| Documentation, onboarding, contributor experience | OM3, OM4 | **Moderate.** Sphinx `-W` clean plus contribution docs; rollback guidance is missing and several audit documents are stale |
| Release reproducibility and operational recovery | OM2, OM4 | **Moderate.** Reproducible stage-1 and an LTS branch exist; no rollback runbook and v1.2.0 is uncut |

---

## 4. Benchmark and conformance policy

New in this revision. It exists because OM6 scored 2 partly for want of a rule,
and because several existing measurements were taken without a stated
environment, which makes them un-reproducible by construction.

### 4.1 Benchmark inputs and environment

A benchmark result is admissible only when the report states:

1. **Host class**: OS, architecture and approximate RAM. This repository's
   development host is windows/amd64 at ~4 GB; the CI legs are ubuntu-latest
   and windows-latest at materially different sizes. A number measured on the
   4 GB host is not comparable with one measured on a CI runner.
2. **Toolchain versions**: Go version, gcc version, and the compiler's own
   version identity (`karkain --version`).
3. **Memory environment**: `GOMEMLIMIT` if set, and whether the run was
   isolated. The repository's documented ~4 GB-host rules (targeted tests,
   package-at-a-time, `GOMEMLIMIT=3GiB`) apply to benchmarks exactly as they
   apply to tests.
4. **Repeat policy**: minimum 3 runs after 1 warm-up, stating whether the figure
   is a median or a minimum. Single-run figures are labelled indicative.
5. **Input identity**: for corpus-shaped measurements, the exact input set and
   its pin (for example the Phase 114 corpus at 61 files), not "the examples".

### 4.2 Baselines and measurement methodology

- A **baseline** is a recorded measurement of a named revision with the
  environment of 4.1 attached. Until a baseline exists for a metric, the metric
  is reported as **unbased** and no comparison is drawn.
- **No numerical performance target may be published without a justified
  baseline.** This scorecard therefore states no throughput, compile-time or
  size target. Where a target exists it is a quality decision from a named
  phase (for example Phase 150D's "a whole-image cache cannot repay itself"),
  which was decided by measurement rather than by wish.
- Absolute figures are preferred over ratios; a ratio without both absolute
  numbers hides the case where both sides degraded.
- Memory is peak working set with the measurement tool named. Existing recorded
  examples: the Phase 141 RSS table and the Phase 129 kcc-build RSS plan.

### 4.3 Correctness before performance

**No performance comparison may be recorded unless both compared revisions pass
the correctness gate for the same input.** Concretely:

- A codegen change must pass the Phase 114 corpus on the affected engine(s)
  before any timing claim about it is admissible.
- A native backend change must pass `pkg/native`, including the 19-image ELF
  byte-identity pin, before a per-program emission time is quoted.
- A benchmark running an unfixed known defect measures the defect. This is why
  the bitwise gate (151P0) asserts **values**, not merely cross-engine agreement:
  an engine agreeing with itself on a wrong answer would make a "parity
  preserved" performance claim meaningless.

### 4.4 Go-oracle comparisons

The Go implementation (`pkg/native`, `pkg/codegen`) is the **reference oracle**.
A comparison against it must state which of three things was compared:

| Comparison | Meaning | Admissible as |
|------------|---------|---------------|
| **Byte identity** | both images are the same bytes | strongest available evidence; used by 151B-D3 |
| **Structural equivalence** | both parse under the same validator with the same layout facts | evidence for containers with no runner (Mach-O) |
| **Semantic equivalence** | identical observable behaviour (stdout bytes, exit code) | evidence where the images legitimately differ (the kcc frame-padding residual) |

A byte-identity claim is never downgraded to "close enough": a difference is
either explained or it is a defect.

### 4.5 Byte identity versus semantic equivalence

- **Byte identity** is the acceptance criterion wherever both engines can emit
  the same artifact for the same source (native images, generated C).
- **Semantic equivalence** is the criterion where the two engines legitimately
  differ (golden stdout plus exit code, conformance assertions).
- **Structural checks alone never establish behavioural correctness.** The
  Phase 9b defect emitted a structurally valid PE that printed its digits and
  silently lost its newline because a relocation entry was missing. Structural
  validation is necessary and not sufficient.

### 4.6 Platform-specific expectations

| Platform | Expectation | Status |
|----------|-------------|--------|
| windows/amd64 | PE images execute; the full native golden suite runs | **Met** on the dev host and on the `native-windows` CI job |
| linux/amd64 | ELF images execute | **Met on CI**; not runnable on the dev host (no WSL/Docker), so host-side claims are structural |
| darwin | Mach-O structural validation only | **NE** - no macOS runner exists; must never be reported as execution |
| accelerator hardware | CPU reference backend is the oracle | **NE** - compile-only guarantees only |

A capability with no runner is reported **NE**, never as passing.

### 4.7 Regression thresholds and evidence retention

- A **regression threshold** requires a baseline and a repeated measurement.
  Until both exist, a change in a measured figure is reported, not gated.
- Where a threshold does exist it is expressed **relative to a pinned baseline
  revision**, and that revision is named in the report.
- **Evidence retention**: every performance or parity claim keeps its
  measurement in the commit message or audit note that made the claim,
  including the exact figure and the command that produced it. Claims without a
  retained figure are treated as unsupported - which is why the D2 parity gate
  records `1088 vs 1096` rather than "nearly identical".
- Timing is **never** golden-tested. Determinism is tested (identical bytes on
  repeat), never duration; this is the Phase 110 precedent.

### 4.8 Capability status vocabulary

Every capability claim uses exactly one of these labels:

| Label | Criterion |
|-------|-----------|
| **supported** | Implemented, gated, CI-enforced, with a negative test for its refusal path |
| **experimental** | Implemented and reachable, with a documented limitation and **no** CI gate enforcing it |
| **deferred** | Not implemented, recorded against a named version and an owning increment |
| **blocked** | Not implementable as designed until a named blocker is resolved, with its evidence recorded |

In force today: `std.numerics` is **supported**; accelerator kernels are
**experimental** (compile-only); `stdlib_v2` without a C compiler is
**deferred** to v1.3.0; the kcc `0xC0000005` incident is **blocked** on an
unclassified root cause.

---

## 5. Gates A-F: definition versus execution

The six gates are **definitions over existing infrastructure**. This revision
creates no gate file and no CI step. Because a gate is only as strong as its
enforcement, three states are distinguished: **defined**, **executed locally**
(command run on a named host with a result), and **CI-enforced** (a `ci.yml`
step runs it on every push).

| Gate | Question | Defined by | Executed locally (this pass) | CI-enforced |
|------|----------|-------------|------------------------------|-------------|
| **A** Language conformance | Does the language behave as documented on both engines? | conformance corpus (59 tests), probes 11/11, 151P0 bitwise values, LH-1/LH-2, Phase 100 runtime-error parity | **Not re-run** - no language change since the last green run; re-running the 452 s and 568 s corpus legs was not justified by this work | Yes |
| **B** Cross-engine parity | Do Go and kcc produce byte-identical output? | Phase 114 corpus (GoEngine + KCCParity), Phase 123/137/146/133 parity gates | **Yes** - D2 differential 7/7 shapes byte-identical, `KnownNonParity` PASS, full `TestPhase151` family green (232.9 s) | Yes |
| **C** Self-hosting & pipeline ownership | Does the compiler build and check itself, alone? | `pkg/bootstrap` bitwise identity, Phase 99 compiler-sources type-check, Phase 122 KIRContinuity | **Yes** - KIR pin re-measured and verified `text=14050 verify=14050` (908.3 s) | Yes, except the seed-closure job, which is discovery-only |
| **D** Native target integrity | Does every kcc-owned native target emit valid, attributable images? | `phase151_kcc_native_test.go` (no silent Go fallback), 151D/A1/B/D2/D3/D5 gates, `pkg/native` incl. the 19-image pin | **Yes** - D5 gate 3/3 including Windows PE execution; D2 known-non-parity pinned | Yes |
| **E** Tooling functionality | Do the shipped tools run end-to-end? | Phase 136 LSP gates, Phase 140 dbg, Phase 135 registry, Phase 96 test runner, Phases 82/86 formatter | **Not re-run** - no tooling change in this work | Yes |
| **F** Release readiness | Is the version shippable under its own checklist? | `docs/release/v1.2.0-CHECKLIST.md` section 2, `scripts/qa/run-full-qa.ps1`, Sphinx `-W` + linkcheck, version-identity pinning tests | **Partial** - Sphinx `-W` run in this pass; the full QA battery was **not** run | Yes for Sphinx and the identity tests |

**No gate is claimed as passed on the strength of its definition.** Gates A and E
were not re-executed in this pass and are reported as "not re-run", not as green.

### 5.1 Governance interaction

A failing gate is **evidence, not authorization**. Per `AGENTS.md`, a fix
proceeds only through an explicitly authorized increment slice with its own
baseline note. The gates are the evidence; the roadmap is the authorization.

---

## 6. Known limitations and unverified behaviour

Recorded here so no reader mistakes a green isolated gate for a closed defect.

1. **The kcc `0xC0000005` incident is UNCLASSIFIED.** Three hypotheses are
   falsified with evidence (low-RAM class, stack exhaustion at a 64 MiB
   reserve, corrupt binary). It is nondeterministic on byte-identical sources
   with a single unchanging binary. The named decisive test is an
   `-fsanitize=address` or `-Wl,--stack,` build, which is a build change
   requiring its own increment.
2. **The KIR pin is measured, not predicted.** It stands at 14050, measured
   with `karkain kir --verify` (908.3 s, `text=14050 verify=14050`). It must
   never be hand-edited to match a prediction.
3. **No fuzzing exists.** OM7 rests on a no-progress guard and regression tests.
4. **No performance baseline exists.** OM6 is measured but unbudgeted; this
   revision adds the policy, not the numbers.
5. **Mach-O and accelerator execution are NE**, not passing.
6. **v1.2.0 is not shippable yet.** Six of its eight conditions stand; condition
   4 is met only for the covered corpus, and two conditions are carried to
   v1.3.0 as UNMET by owner-authorized amendment.
7. **`MM-K-time` is unassessable** - the gap inventory has no row for time,
   date or duration, so the scorecard cannot score it in either direction.

---

## 7. Stale records this revision supersedes

| Document | Why it is stale | Disposition |
|----------|-----------------|-------------|
| The Phase-49 table formerly in this file | Scored Self-Hosting 1, Generics 1, Parser 2 (OOM) | Superseded by this file |
| `docs/audit/KARKAIN_IMPLEMENTATION_MATRIX.md` | Describes the parser OOM as live, "no general type checker", "generics syntax not parsed" | Historical; see the control-system entry point for the authoritative queue |
| `docs/audit/NEXT_PHASE_RECOMMENDATION.md` | Recommends "Phase 56-C: parser OOM fix", completed long ago | Historical; the current next action is derived by the control system |
| `ROADMAP.md` | Encoding-corrupted pre-50 historical log | Retained as history; the forward view is `KARKAIN-VERSION-PLAN.md` |

---

## 8. Traceability to the previous 30-dimension taxonomy

The previous revision of this scorecard used four lettered groups (A1-A10
language core, B1-B8 runtime and backends, C1-C7 tooling, D1-D5 engineering
process). Every one of those 30 rows maps to a row above; none was dropped
silently.

| Previous | Now | Previous | Now | Previous | Now |
|----------|-----|----------|-----|----------|-----|
| A1 Lexer | AR1 | B1 C23 backend | CE5 | C4 Debugger | EX3 |
| A2 Parser Go | AR2 | B2 Native (oracle) | CE4 | C5 Package manager | EX2 |
| A3 Parser kcc | AR3 | B3 Native (kcc) | CE4 | C6 Test runner | EX5 |
| A4 Sema Go | AR5 | B4 WASM | CE6 | C7 Profiling | EX3 |
| A5 Sema kcc | AR6 | B5 Concurrency | CE5 | D1 Bootstrap | CE1 |
| A6 Enum/match | AR7 | B6 Runtime errors | CE7 | D2 Pipeline ownership | CE2 |
| A7 Closures/fn | AR8 | B7 Standard library | EX1 | D3 Differential discipline | CE3 |
| A8 Generics v1 | AR8 | B8 SIMD/accelerators | EX6 | D4 Release governance | OM2 |
| A9 Bitwise | AR1/CE7 | C1 CLI | EX3 | D5 Documentation | OM3 |
| A10 Error handling | AR9/CE7 | C2 Formatter | EX3 | (new) | OM1-OM7 |

Three scores changed materially and each change is evidence-backed: the previous
A1-A10/B1-B8/C1-C7/D1-D5 grouping reported Self-Hosting as 1 where it is now 3
(AR3/CE1: the self-hosted compiler parses and checks the whole language), and
reported Modules as 1 where it is now 3 (AR9: Phase 103/122/144 shipped
visibility, assembly and dependency resolution).