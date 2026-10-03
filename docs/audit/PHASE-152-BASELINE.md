# Phase 152 BASELINE — Native stdlib + no-C closure

**Increment:** 152 (version **1.2.0** "Sovereignty I — Off C, Off Go")
**Independence item:** **I-5** (native link, `gcc` optional) — the last I-track item before I-6 (154)
**Status at authoring:** baseline only. **No implementation.** Every claim below is
in-tree evidence with a file/line, not a roadmap restatement.

Order is dependency-forced: `150 → 151 → 152 → 154` (`KARKAIN-VERSION-PLAN.md:157`).
Increment 152 is the next compiler increment after 151.

---

## 1. Objective

Make the **native path** — `karkain build/run --target native-<os>` — carry the
language's standard library and its socket boundary **without a C compiler
anywhere on `PATH`**, and prove it with a gate that *cannot pass by skipping*.

Contract (verbatim, `KARKAIN-VERSION-PLAN.md:211`):

> **Native stdlib + no-C closure** (completes I-5): `hello`, `stdlib_v2` and one
> `std.net` program link and run with no C compiler on `PATH`.

Exit criteria (from `docs/release/v1.2.0-CHECKLIST.md`):

1. `hello` builds and runs with `gcc`, `clang` and `cl` absent from `PATH`.
2. `stdlib_v2` works in the same no-C environment.
3. One `std.net` program builds and runs under the defined no-C constraint.
4. `--target c23` remains available and unchanged.
5. Network/database/http operations must **deterministically reject** on a
   runtime with no OS sockets.

---

## 2. Current measured state

Measured by reading the implementation, not by assuming "a native backend exists".

### 2.1 What already works

| Capability | State | Evidence |
|---|---|---|
| Direct machine-code emission, no C compiler | **ALREADY C-FREE** | `pkg/cli/cfree_target.go` — the only `exec.Command` is `:136`, *running the produced image*. `pkg/native` never shells out (its only `os/exec` references are in `_test.go`). |
| Native containers | ELF, PE32+, Mach-O PIE — all writable and validated | `pkg/native/{elf,pe,macho}.go`; increments 145/149/150D |
| Runtime startup | Linux syscall tail; Win64 + PEB bootstrap; Mach-O PIE rebase | increment 149, 150D |
| OS syscalls available natively | **only `write` and `exit`** (+ Win32 `GetStdHandle`/`WriteFile`/`ExitProcess`) | `pkg/native/native.go:13`; runtime helpers `src/compiler/codegen.kark:817-825` |
| `hello` (`examples/app.kark`: `print` string/int, `+ - * /`) | **builds and runs natively, no-C** | increments 148/149/150 gates; PE legs execute on the dev host |
| Value model | int, float64, string (concat/slice/compare), array (+`for-in`, `len`, `push`), map, struct/record, control flow, calls ABI, register allocation (150C) | increments 148/150A–150D |
| Assembly of `import std.*` into the native unit | **works** — `BuildCommand` resolves sources first | `pkg/cli/commands.go` → `resolveSourcesCheck` → `assembleModuleUnit` |

**Consequence worth stating plainly: exit criterion 1 is already satisfied** for
the native path. 152 is not about the toolchain chain; it is about **native
stdlib coverage**.

### 2.2 What does not work

| Capability | State | Evidence |
|---|---|---|
| `stdlib_v2` on native | **BROKEN** — 6 C-runtime builtins have no native implementation (§3) | `examples/stdlib_v2/main.kark` |
| `std.net` on native | **BROKEN** — 7 C-runtime builtins, and **zero socket syscalls** | `pkg/sema/resolve.go:81-82` |
| Filesystem on native | **ABSENT** — `createFile`/`writeToFile`/`readLineEOF` are C builtins; no file syscalls | `resolve.go:68-69` |
| String concat inside a loop | **UNSOUND** — unguarded, and the arena bound assumes each site runs once (§4.3) | `program.go:280-303`: `pushInLoop` is refused; **no `concatInLoop` counterpart exists** |
| Native under the **default** engine | **REFUSES** — `kccOwnsNativeTargets = false`, so kcc answers `error[K116]`, exit 6 | `pkg/cli/kcc_native.go` |
| Bitwise / shift on native | **ABSENT** — int ops are `case "+", "-", "*", "/", "%"` only | `pkg/native/program.go:1952`, `:4218` |

### 2.3 The two dependency chains have different answers

**Chain A — Go front end → native image → link/run: NO BLOCKER.**
`cfreeBuildForOS` → `native.CompileProgramForOS` is in-process byte emission and
already satisfies "no C compiler on `PATH`". **This is where 152 does its work.**

**Chain B — kcc → native image: BLOCKED, and that is increment 151's scope.**
151A is incomplete: the value model exists (Steps 1, 2, 2b, 3) but the **entry
stub, syscall tails and PEB bootstrap are still absent from kcc**, so
`kccOwnsNativeTargets` is still `false` and 151D has not been flipped.

> **Scoping consequence.** 152 must **not** depend on Chain B. Requiring
> `--target native-*` to work on the *default* engine would drag 151A/151D into
> 152 and break the documented order. Removing the **Go front end** is I-6 =
> **increment 154**, a separate increment. 152 removes the **C compiler**
> dependency only; the Go front end remains the reference.

---

## 3. Existing stdlib boundary (do not expand the API)

Traced from `examples/stdlib_v2/main.kark` through the stdlib to the runtime.

**Portable on native today** (Karkain source over already-supported features):

| Symbol | Why portable |
|---|---|
| `str_concat` | `a + b` → 150B1 arena concat |
| `ascii_lower` / `ascii_upper` | Karkain funcs returning literal tables (`stdlib/string/string.kark:10,14`) |
| `array_max` / `array_sum` / `array_contains` | Karkain funcs (`stdlib/collections/collections.kark:236,209,9`) |
| `len`, `while`, array literal, indexing | 150A / 148 |

**Blocked — Phase-109 runtime builtins realised only in the generated C preamble:**

| Symbol | Reached via | Native? |
|---|---|---|
| `trim` | `str_trim` | ❌ |
| `split` | `str_split` | ❌ |
| `hex_decode_bytes` | `hex_decode` | ❌ |
| `base64_encode_bytes` | `base64_encode` | ❌ |
| `base64_decode_bytes` | `base64_decode` | ❌ |
| `sha256_hex` | `sha256` | ❌ |

All six are registered in `pkg/sema/resolve.go:70-74` and emitted as C helpers by
`pkg/codegen/codegen.go`. `pkg/native` has **no** equivalent.

`str_to_upper` is a third category: its **body** is portable Karkain, but its
`result = result + mapped` sits **inside a `while`** (`stdlib/string/string.kark`),
landing on the concat-in-loop gap in §4.3. Blocked by soundness, not by a
missing builtin.

In the same class but outside the `stdlib_v2` gate: `createFile`, `writeToFile`,
`readLineEOF` (filesystem) and the 7 `net_*` builtins.

---

## 4. Exact blockers

### 4.1 Missing native runtime surface for six builtins (primary)

Neither the six builtins nor a general "builtin unavailable on this target"
mechanism exists in `pkg/native`. A program calling `sha256(...)` on native today
reaches native lowering with no implementation and no designed diagnostic.

### 4.2 No socket syscall layer, and no "no sockets" rejection path

The 7 `net_*` builtins (`resolve.go:81-82`) are C-runtime. The native backend has
**no socket syscalls** — only `write`/`exit`. The only existing error path is a C
buffer `_karkain_net_error` fed by `WSAGetLastError()`/`strerror(errno)`
(`pkg/codegen/codegen.go:1606-1616`), which is meaningless without sockets.

`std.db` is a pure text engine needing **no sockets**, but calls `trim`
(`db.kark:134,148`), so it is blocked by §4.1. `std.http` is 20 functions layered
on `net`, so it is blocked by §4.2.

### 4.3 Concat-in-loop is unsound, not merely unsupported (latent defect)

The arena bound is documented as a proof: *"every string value in a
concatenating program is a literal or a concatenation of literals, so
`sites × totalLiteralBytes` bounds the sum of all runtime allocations."* That
proof silently assumes **each concat site executes at most once**. A concat in a
loop body violates it, and — unlike `push()` — nothing guards it:
`program.go:300-303` defines and refuses `pushInLoop`, and **no `concatInLoop`
field or refusal exists**.

Consequence: `str_to_upper` on native would **not** be rejected. It would exhaust
the bump arena mid-loop and trap at an arbitrary iteration. This is exactly the
class increment 150B3a refused rather than shipped; 152 must not be the increment
that reintroduces it.

### 4.4 The "no sockets" rejection must be *designed*, not inherited

"Deterministic reject" is not satisfied by the status quo. With the builtins
simply absent, the honest current outcome would be an unresolved-identifier
diagnostic at check time — deterministic, but it conflates *"this target has no
sockets"* with *"this program is invalid"*, and it fires before the program ever
runs. Exit criterion 5 asks the program to **build and run** and reject at the
operation, so 152 needs an explicit, documented native net rejection.

---

## 5. Scope

1. **Native runtime surface for the six blocked builtins.** The smallest credible
   shapes are: `trim`/`split` in portable Karkain where that is genuinely
   smaller, and native helpers for the byte-level codecs (`hex_*`, `base64_*`,
   `sha256_hex`). `sha256_hex` needs 64-bit rotates — the native target has **no
   bitwise ops** (`program.go:1952`) — so either bitwise lowering lands first or
   the digest is computed with add/carry. **This is the single largest unknown in
   152 and must be measured before it is scoped further.**
2. **Close the concat-in-loop hole (§4.3).** Either refuse it exactly as
   `pushInLoop` is refused, or make the arena bound sound for it. Refusing is the
   smaller and safer first step and is honest; shipping an unsound bound is not
   an option either way.
3. **A deterministic native socket rejection path (§4.4)** — an explicit,
   documented refusal for `net_*` on a target with no sockets, distinguishable
   from an invalid program, so criterion 5 is satisfiable and testable.
4. **The no-C gate (§7).**
5. **`--target c23` unchanged**, asserted by the gate rather than assumed.

## 6. Non-scope

* **Increment 151** — no change to 151A/151B/151C/151C2/151C3, no flip of
  `kccOwnsNativeTargets`, no kcc native emission. Chain B stays 151's problem.
* **Increment 154 (I-6)** — no self-bootstrap, no retirement of the Go front end
  or `pkg/bootstrap`.
* **Increment 165** — no container work.
* **No removal of the C path.** `--target c23` stays a first-class target
  (`pkg/cli/exitcodes.go:36`, `:140`).
* **No new stdlib modules**, and no expansion of the stdlib API beyond making the
  *existing* surface usable natively. 1.3.0 roadmap modules are out.
* **No new networking behaviour.** `std.net` gains a rejection path, not
  capability.
* **No filesystem syscalls on native** unless a named exit criterion demands it
  (none does).
* **Not a Language Hardening Checkpoint item.**

---

## 7. Implementation boundary

The smallest boundary that satisfies all five exit criteria:

| Component | Change | Rationale |
|---|---|---|
| `pkg/native` | Native implementations or Karkain-level ports of the six builtins | the actual blocker (§4.1) |
| `pkg/native/program.go` | `concatInLoop` refusal (or a sound bound) | latent soundness defect (§4.3) |
| `pkg/native` | explicit net rejection surfaced as a normal runtime error | criterion 5 (§4.4) |
| `pkg/cli` | **no behaviour change expected** | the native dispatch path is already C-free and already assembles `std.*` |
| `stdlib/**` | **no API change**; possibly internal rewrites if a Karkain port is smaller than a native helper | boundary discipline |
| `docs/release/v1.2.0-CHECKLIST.md`, `AGENTS.md` | status only | on completion |

Explicitly **outside** the boundary: anything in `pkg/codegen` (the C23 path must
stay byte-identical), `src/compiler/native_*.kark` (151's encoder/containers), and
the Language Hardening Checkpoint.

---

## 8. Gate design

New gate `pkg/cli/phase152_noc_test.go`, plus a CI step (a gate without a CI entry
is not a gate).

**Toolchain unavailability must be *proven*, not assumed.** Reuse the Phase-143
mechanism (`pkg/bootstrap/phase143_seed_test.go:51` `shadowGoWithStub`): prepend
a directory of failing stubs for `gcc`, `clang` and `cl` to `PATH`. Shadowing, not
directory removal — the 143 record documents that `go` and `gcc` can share a
directory, so removing it would amputate unrelated tools.

**Windows note that must not be rediscovered:** Windows resolves these through
`PATHEXT`, so the stub directory needs both a shell-script form and a
`.bat`/`.cmd` form (143 does this for `go`), otherwise the real compiler is found
and the proof silently evaporates.

Required assertions:

1. **Anti-vacuity, the load-bearing one:** for each of `gcc`, `clang`, `cl`,
   `exec.LookPath` must resolve **to the stub**, and running it must **fail**.
   The gate records the resolved path and the failure. If any of the three cannot
   be shadowed and proven unavailable, the test **fails** — it never skips.
2. `hello` builds and runs under the scrub, on the host-native target, with the
   expected output.
3. `stdlib_v2` builds and runs under the scrub, byte-identical to its existing
   both-engine golden.
4. One `std.net` program builds and runs under the scrub, and its socket
   operation produces the documented deterministic rejection — asserted on the
   **diagnostic text**, not merely on a non-zero exit.
5. `--target c23` is still registered and still builds and links (criterion 4).
6. Cross-target determinism preserved: an unknown `--target` still fails with a
   toolchain error, never a silent host fallback.

Explicitly **not** in the gate: full corpus, conformance, probes, bootstrap, or
the 150 legacy-ELF byte-identity differential. On the ~4 GB dev host the gate must
run isolated, one program per subtest, reusing a single built CLI via `sync.Once`
(the pattern `phase130_closures_test.go` already uses for this reason).

---

## 9. Expected files/components

* `pkg/native/program.go` — builtin surface, `concatInLoop`, net rejection
* `pkg/native/*_test.go` — unit gates for each new helper
* `pkg/cli/phase152_noc_test.go` — the no-C gate
* `.github/workflows/ci.yml` — one step for the gate
* `docs/release/v1.2.0-CHECKLIST.md`, `AGENTS.md` — status on completion
* a `stdlib/net` example or fixture for criterion 3, only if none is adequate

## 10. NFR impact

| NFR | Target | Increment |
|---|---|---|
| NFR-15 toolchain independence | *"no C compiler on the default path"* — currently a C compiler **is** required | **152**, then 154 for no-Go |
| NFR-2 reproducible builds | same discipline extended to native images | 152, 165 |
| Performance | direct emission already beats gcc+link; native stdlib helpers must not regress the measured 11–53 µs/program | 152 (measure) |

## 11. Risks

| Risk | Severity | Mitigation |
|---|---|---|
| **`sha256_hex` needs bitwise; native has none** | **high** | measure first (§11.1); add/carry path or land bitwise lowering before committing to the shape |
| Concat-in-loop unsoundness reaches a shipped program | high | refuse it as `pushInLoop` is refused, in the same change |
| "Deterministic reject" collapses into "undefined identifier" | medium | design the native net rejection explicitly; assert diagnostic text in the gate |
| No-C proof evaporates on Windows via PATHEXT | medium | stub both script and `.bat` forms; assert the resolved path is the stub |
| Scope creep into 151/154 (kcc-owned native, Go removal) | high | §6 non-scope; reject any 152 change to `kccOwnsNativeTargets` |
| 4 GB host cannot run the gate | medium | isolate subtests, one built CLI via `sync.Once`, no corpus |
| Stdlib rewrite changes C-path output | high | the Phase-114 both-engine golden must stay byte-identical; C path untouched |

### 11.1 The measurement that must come first

Before the shape of §5.1 is fixed, measure **whether the six builtins can be
expressed at all** on the native target. Specifically: does `sha256_hex` have a
native realisation without adding bitwise operators to `pkg/native`? If not, the
increment's largest single item changes shape, and that should be a recorded
decision rather than a surprise discovered mid-implementation.

---

## 12. Completion criteria

152 is complete when **all** hold:

1. `hello` builds and runs with `gcc`, `clang` and `cl` **proven** unavailable.
2. `stdlib_v2` builds and runs in that same environment, byte-identical to its
   existing both-engine golden.
3. One `std.net` program builds and runs there, with a documented deterministic
   rejection at the socket operation.
4. `--target c23` still registered, still builds and links, output unchanged.
5. Concat-in-loop is either refused or sound — no unsound arena bound ships.
6. `std.db` is classified explicitly: works natively, or carries a documented
   reason it cannot.
7. The Phase-114 both-engine corpus golden is unchanged (no C-path regression).
8. A gate exists **and** has its own CI step, with anti-vacuity assertions.
9. Unknown cross-targets still fail deterministically; no silent host fallback.
10. `docs/release/v1.2.0-CHECKLIST.md` and `AGENTS.md` record the closure.

**A version is not closed as "green except X."**

## 13. Verification strategy

* **Source-level first**, on this host: confirm each of §3's portable/blocked
  classifications by reading the stdlib and `pkg/native`; no execution needed.
* **One focused measurement** (§11.1) before committing to §5.1's shape.
* **The §8 gate**, run isolated per subtest, is the acceptance evidence.
* **Regression neighbours** after a `pkg/native` change, each isolated per the
  ~4 GB rule: Phase 148 CLI gate, Phase 150 native-target gate, `pkg/native`
  itself, and the 19-image legacy-ELF byte-identity differential.
* **Phase-114 corpus** is re-run only if a stdlib or `pkg/codegen` file changes,
  which §7 says it should not.
* Not run during 152 implementation: full corpus, full conformance, probes,
  bootstrap, full release QA.

---

## 14. Progress

### 152-A — native safety + capability boundary + no-C gate (DONE)

Scope was deliberately reduced from the original §5 after the §4.1 measurement
showed `split` cannot be implemented at all on the native target. **152-A
implements no new stdlib function.**

**1. `concatInLoop` — latent soundness defect CLOSED.**

The arena bound `sites * totalLiteralBytes` is a proof that requires each concat
site to execute **at most once**. A concat in a loop body runs once per
iteration, so the bound is not an upper bound. The rule now mirrors
`pushInLoop` exactly: `scanConcatSites` threads a loop-depth through its walkers
(`While`/`For`/`ForIn` bodies are `depth+1`; an `if` body is **not**, because a
conditional runs at most once per entry), and `emitStrConcat` refuses with a
`K145` naming the construct and the reason.

**The defect was measured, not inferred.** A scratch probe (added, run, deleted)
compiled the same program shape with the refusal disabled:

```kark
func main() {
    let i = 0
    while (i < 2000) { print("ab" + "cd"); i = i + 1 }
}
```

It printed ~250 lines of `abcd`, **corrupted several of them** (`nabcd`,
`a\nbcd`, `abc\nd` — the bump cursor overrunning live bytes), then died with
`2147483651` = `0xC0000003` STATUS_BREAKPOINT: the allocator's exhaustion trap,
hit at an arbitrary iteration.

**A pre-existing executed golden was a false negative.** `TestNativeStrConcatExec`
listed `in_loop` (`while (i < 3) { print("it" + "er") }`) as a passing case. Its
bound was `1 * 4 = 4` bytes, floored to `heapMinSize` = **1024**, while three
iterations need **12** — it passed because the floor happened to cover it, *not*
because the bound was sound. It is reclassified as a refusal, with the reason
recorded in place rather than silently deleted.

**2. `net_*` — capability boundary, deliberately not implemented.**

The native backend's only syscalls are `write` and `exit`. The seven socket
builtins (`net_connect`, `net_listen`, `net_accept`, `net_read`, `net_write`,
`net_close`, `net_last_error`) are rejected by name with `K145`, stating the
reason and pointing at `--target c23`. The check sits in `retKindVisit`, so it
fires during **kind inference** — before emission — which is why the first
diagnostic a `std.net` program meets is the socket boundary itself rather than
some later emission limit.

It is deliberately **not** the generic `undefined function` message: that cannot
distinguish "this target has no sockets" from "this program is invalid", which
is exactly what exit criterion 5 requires. The C23 path is untouched and keeps
the full `std.net` surface.

**3. The no-C gate.** `pkg/cli/phase152_noc_test.go`, plus
`pkg/native/phase152a_safety_test.go`. For `gcc`, `clang` and `cl` it asserts
resolution lands **inside the stub directory** and that the stub **fails when
run**. It **fails rather than skips** when absence cannot be proven — a gate that
skips on a missing precondition is not evidence. Windows PATHEXT is handled with
script **and** `.bat`/`.cmd` stub forms, because a bare script would be skipped
and the *real* compiler found, silently voiding the proof.

**Regressions:** `pkg/native` full suite green; the 19-image legacy-ELF
byte-identity differential at **zero drift**; Phase 148 + Phase 150D CLI gates
green.

### 152-B1 — native string-codec builtins (IMPLEMENTED, UNCOMMITTED)

Four of the five codecs listed as open above are now **implemented**. The
"Still open for 152-B" list below was written before this work landed and has
been corrected; the two functions it still describes as open are genuinely open
and their blocker/deferral decisions are **unchanged**.

| Function | Implementation | Helper | Allocates? |
|---|---|---|---|
| `trim` | `emitStringCodec` → `karkain_trim` | `emitTrimHelper` | **No** — a view (ptr/len adjust) |
| `hex_decode_bytes` | `emitStringCodec` → `karkain_hex_decode_bytes` | `emitHexDecodeHelper` | Yes — n/2 bytes, cannot be a view |
| `base64_encode_bytes` | `emitStringCodec` → `karkain_base64_encode_bytes` | `emitBase64EncodeHelper` | Yes |
| `base64_decode_bytes` | `emitStringCodec` → `karkain_base64_decode_bytes` | `emitBase64DecodeHelper` | Yes |

**Gating.** One flag per builtin (`usesTrim`, `usesHexDecode`,
`usesBase64Encode`, `usesBase64Decode`) set by a `scanStringCodecs` pre-pass, so
each helper is emitted **only** when the program actually calls it and every
other image stays byte-identical. `codecArenaBytes` and `codecTemp` (the
five-unit staging area per call site) are likewise reserved only on demand.
Arithmetic diagnostics are raised through `raiseCodecError` against the same
reporter 152-B0 introduced; only the two decoders need it (`trim` is a pure
scan and `base64_encode_bytes` always succeeds).

**Stdlib path.** The stdlib wrappers (`str_trim` → `trim`, `hex_decode` →
`hex_decode_bytes`, ...) reach the backend as plain calls to these names, so
`scanStringCodecs` matches on the builtin names, not the wrappers.

**Companion PE fix, required by this work (real defect, found by it).**
`pkg/native/pe.go`: `.idata`/`.reloc` RVAs were fixed constants
(`0x2000`/`0x3000`), which silently assumed `.text` never exceeded one 0x1000
page. Emitting two codec helpers **does** exceed it, and the two sections then
overlapped — the loader rejected the image as "not a valid Win32 application"
with **no** error from `buildReloc`, `ParsePE` or the build itself, because every
one of those checks was satisfied. `LinkPE` now derives both RVAs from the real
end of `.text` (`peAlignUp`), `buildIdataAt(idataRVA)` bakes that RVA into every
self-relative pointer in `.idata` so the two cannot disagree, and `ParsePE`
validates placement **structurally** (section-aligned, non-overlapping,
directories naming their own section) instead of against constants — comparing
against constants would have accepted the very image the check exists to
reject. `buildIdata()`/`peIdataRVA` survive only so `program.go`'s PE
`heapBase` expression keeps compiling; that expression is dead on the Windows
path (the branch returns before `heapBase` is used) and is deliberately not a
second source of truth.

**Evidence status — stated precisely, and deliberately not upgraded:**

* **Implementation present** — yes, for all four functions.
* **Tests present** — yes: `pkg/native/phase152b1_codec_test.go`, 13 test
  functions (trim; hex decode + rejects + message-differentiation; base64
  encode + binary + length; base64 decode + binary + lenient-padding + rejects
  + message-differentiation).
* **Tests executed and passing — measured, locally on Windows/amd64:**
  * focused gate `go test ./pkg/native/ -run 'TestPhase152B1_' -count=1 -v`
    → **97 PASS / 0 FAIL / 0 SKIP** across 13 test functions, `9.508s`;
  * full suite `go test ./pkg/native/ -count=1` → **PASS, `55.438s`, no FAIL**.
* **These are local Windows/amd64 results.** The dedicated CI step for this gate
  has been added (see below), but the **Linux `ubuntu-latest` execution of it is
  not yet independently verified** until CI actually runs it — where these tests
  take the ELF path rather than the PE path measured here. Recorded as an open
  measurement, **not** as a blocker: the neighbouring 152-A `pkg/native` step
  runs on the same runner, so the pattern is sound, but per evidence discipline
  that is inference and must not be recorded as verified.
* **Native execution evidence** — **none claimed for 152-B1** beyond the local
  Windows PE runs above. The reject tests' stderr is empty on this host by the
  152-B0 `GetStdHandle(STD_ERROR_HANDLE)` limitation, so they assert exit code,
  stdout route and interned text rather than captured stderr.
* **Byte-identity differential evidence** — **none claimed for 152-B1.** The
  19-image legacy-ELF zero-drift differential is the *incremental-150*
  invariant and is not evidence about these four functions.

#### 152-B1 — reconciliation note (2026-10-03)

A reconciliation pass established that this section was **stale in both
directions**: it listed the four implemented codecs as "no native
implementation yet", and described 152-B0 as uncommitted when it is committed as
`da1dc86`. Both are corrected above. No blocker decision was altered.

**Scope discipline.** 152-B1 is `pkg/native` plus one dedicated CI gate:
`program.go`, `pe.go`, `phase152b1_codec_test.go`, and a named step in
`.github/workflows/ci.yml` (`Run Increment 152-B1 native string-codec gate`,
`-run 'TestPhase152B1_'`). That step is required by the `AGENTS.md` companion
rule — *a gate is a test file plus a CI entry* — and was added in the same change
for exactly that reason. It deliberately touches **no** `src/compiler/*.kark`
file and therefore requires **no** KIR pin change. The working tree additionally
contains an **unauthorized** `_start` / `phase151a4` slice which is *not* part of
152-B1 and is excluded from it; that work has no roadmap authorization (no "151A
Step 4" exists in `PHASE-151-BASELINE.md`, `KARKAIN-VERSION-PLAN.md`,
`v1.2.0-CHECKLIST.md` or `AGENTS.md`), so the KIR pin in
`pkg/cli/phase122_pipeline_ownership_test.go` (11373 → 11377 → 11412) belongs to
that slice and to LH-2, **not** to 152-B1.

### Still open for 152-B (corrected)

* `split` — **blocked**, unchanged: its `string[]` return has no native
  representation (arrays are int-only), so string arrays are a prerequisite.
  No implementation exists.
* `sha256_hex` — **deferred**, unchanged: needs a bitwise strategy decision
  (§11.1). No implementation exists.
* Native map returns (`net_endpoint` returns a map literal) — a separate
  unsupported case, not part of the socket boundary.
* `stdlib_v2` as a whole still cannot run natively: it needs both `split` and
  `sha256_hex`.

### Unchanged by 152-A, deliberately

kcc native ownership (`kccOwnsNativeTargets` is still `false`), increment 151,
the C23 backend (`pkg/codegen` untouched), `src/compiler/native_*.kark`, any
1.3.0 stdlib module, and increment 154.

### 152-B0 — native runtime-error reporting (IMPLEMENTED, COMMITTED)

**Commit status corrected.** This entry previously read "IMPLEMENTED, NOT
COMMITTED". That was stale: 152-B0 is committed on `main` as **`da1dc86`**
(*feat(native): add runtime error reporting*). The shipped-work description
below is unchanged.

Prerequisite for 152-B1. The dependency report found the native target could not
express the Phase-100 contract at all: no message infrastructure existed, and
all six native `Int3` traps are internal invariant/bounds checks with no text
and no exit code. Array indexing and string slicing therefore already diverged
from the contract, independently of the codecs.

**Shipped:** `karkain_runtime_error(kind, file, line)` emitted inside the
existing helper architecture and gated on a `usesRuntimeError` pre-pass (an
intentional over-approximation: any index expression, because the pre-pass runs
before type inference and a missing reporter is a link-time failure while an
unused one only costs bytes). Source-filename plumbing via
`CompileProgramForOSSource`, using `filepath.Base` to mirror C23's
`sourceBaseC()` so both engines print the same location. The array-index trap
is replaced by the real diagnostic; Win64 handle selection is parameterised so
`STD_ERROR_HANDLE` is reachable at all (the Windows write path had hardcoded
`STD_OUTPUT_HANDLE`).

**Unresolved and deliberately untouched:** string-slice bounds. C23's
`karkain_slice` *clamps* out-of-range bounds instead of raising, so there is no
reference diagnostic for native to match. Native still traps. Settling this
requires an explicit language-semantics decision (should slicing clamp, matching
C23? — in which case the native trap is the bug) and is outside 152-B0. No new
roadmap phase was created for it.

**Two completion conditions are unproven on this host.**
`GetStdHandle(STD_ERROR_HANDLE)` returns `INVALID_HANDLE_VALUE` for these
minimal PE images while `GetStdHandle(STD_OUTPUT_HANDLE)` resolves through
byte-identical code. Measured four ways: Go `exec` with a piped stderr, with
an inherited stderr, `cmd /c image.exe 2>file` from a real console, and with a
sign-extended 64-bit immediate. The diagnostic is therefore emitted correctly
but cannot be *captured* from a PE image here; the Linux leg is where the text
is compared byte for byte. Recorded rather than worked around, because routing a
runtime error to stdout to make it visible would break the Phase-100 contract.
