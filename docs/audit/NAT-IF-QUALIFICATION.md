# NAT-QUAL baseline — the v1.2.0 native-independence qualification

**Slice:** `NAT-QUAL` (unnumbered working label; the third v1.3.0 increment,
after `NAT-CALL` and `NAT-IF`).
**Branch:** `feature/v1.2.0-native-independence`.
**Status:** implemented and gated.

This slice adds **no language construct**. It answers the question the v1.2.0
mission actually asks: *what does the Karkain-owned native path support, how is
that claim checked, and does that path really avoid C and Go?*

---

## 1. Why this slice exists

The native path was already largely implemented before this slice: increment
150 wrote the C-free containers, increment 151 made the self-hosted engine own
all three native targets, `NAT-CALL` taught it to call a user function, and
`NAT-IF` taught it to branch. What did **not** exist was any statement of
*which* constructs are supported, *on which* platforms, and *how* a reader can
check the claims.

Three artefacts were missing, and all three were missing in the same way: they
were claims without evidence behind them.

1. A feature matrix. The repository had
   `docs/architecture/KARKAIN-LANGUAGE-COMPLETENESS.md` (an L0–L10 completeness
   model over a GAP inventory) and `docs/inventory/compiler-dependencies.json`
   (a dependency inventory) — but the dependency inventory was **frozen at
   phase 144 (verified 2026-09-23)** and did not mention the native backend at
   all, and nothing stated which language constructs the native path lowers.
2. A platform qualification statement. The Mach-O claim is structural only, and
   that fact was recorded in a dozen places but never collected in one place
   where a reader could find it.
3. Process-level evidence of independence. The claim "the qualified native path
   does not invoke a C compiler or the Go toolchain" rested on a **provenance
   string** that the compiler prints about itself — which is the engine's own
   account of what it did, not an observation of what it did.

---

## 2. What this slice adds

### 2.1 A machine-verifiable feature matrix

`docs/inventory/native-feature-matrix.json` — one row per construct, with a
status, and the gate that backs it.

"Machine-verifiable" is a property of the file, not a description of it:
`TestPhaseNATQual_TheMatrixIsBackedByGates` reads the matrix and asserts that

* every status is one of the four declared statuses;
* every construct appears exactly once;
* every `supported` row names a gate **file that exists** and a **test
  pattern that file actually contains** — a row naming a gate that does not
  exist would otherwise read exactly like a row naming a gate that does;
* every `not_supported` row either names a gate or explains, in a note, why no
  gate exercises the refusal — a status with nothing behind it is a claim, not
  a fact;
* every `implemented_not_gated` row explains what is missing. That status
  exists so a gap can be named without being advertised as support.

The gate found **two defects in the matrix on its first run** — a row with a
status and no gate behind it, and a row naming a gate pattern that no test
carried. Both are recorded here because a matrix that is only checked by
reading it is a document.

### 2.2 A platform table that must match the code

`TestPhaseNATQual_ThePlatformTableMatchesTheCode` asserts that the matrix's
platform table agrees with the code's **own** ownership data
(`pkg/cli/kcc_native.go`, `kccNativeOwnedTargets`), that every target the
code says kcc owns appears in the table, and that every target lists an
emission status and an execution status. A table that disagrees with the code
is a table that will be wrong the moment the code changes.

The execution column is where honesty matters most:

| target | container | emission | execution |
|---|---|---|---|
| `native-x86_64-windows` | PE32+ | proven | **proven** on windows/amd64 — the produced images are executed and their stdout compared as exact bytes |
| `native-x86_64-linux` | ELF64, non-PIE | proven | **not proven on this host** (windows/amd64, no WSL, no Docker): byte identity against the oracle is proven; execution comes from the Linux CI leg |
| `native-x86_64-macos` | Mach-O 64-bit PIE with a rebase stream | proven | **not proven anywhere**: this repository has no macOS runner and no Intel-Mac host. Every Mach-O claim since increment 149 has been structural. |

### 2.3 Process-level independence evidence

`TestPhaseNATQual_TheNativePathInvokesNeitherCNorGo` — the layer that matters.

The method:

1. **Warm** the self-hosted compiler with an *untraced* build, so the
   compiler's own build (which legitimately uses `gcc`) is not inside the
   trace. The claim is about compiling a **user program**, not about how the
   compiler was built.
2. **Trace** a native build with PATH shims for `gcc` and `go` first on PATH.
   Each shim appends its arguments to a log and then **fails** — failing is
   deliberate, because a shim that succeeded would let the build continue with
   a fake compiler, and the log would still prove nothing about the build
   succeeding.
3. **Assert the log is EMPTY** for the native build.
4. **Positive control:** run the **legacy** build path with the same shims and
   assert the log is **NOT** empty — the legacy path is supposed to reach `gcc`.
   If the shims do not fire there, they cannot be trusted to stay silent on
   the native path, and step 3 would be a test that can pass without its
   mechanism working.

Step 4 is the part that makes step 3 evidence. A "the log was empty" assertion
on its own is satisfied by a shim that never fired, a `PATH` that was not
consulted, and a build that never looked for a compiler at all.

Measured on this host: the native build's log is empty, and the legacy build's
log contains the `gcc` invocation.

---

## 3. The gate

`pkg/cli/phase151h_qualification_test.go`, tests named `TestPhaseNATQual_*`,
with its own `ci.yml` step (*Run NAT-QUAL native independence qualification
gate*).

| Layer | Question | Evidence |
|---|---|---|
| `TheMatrixIsBackedByGates` | is every matrix claim checked? | the named gate file exists and contains the named test pattern |
| `ThePlatformTableMatchesTheCode` | does the platform table match the code? | compared against `kccNativeOwnedTargets` and the target registry |
| `TheNativePathInvokesNeitherCNorGo` | does the native path really avoid C and Go? | PATH shims, an empty log, and a positive control |

---

## 4. The dependency inventory, refreshed

`docs/inventory/compiler-dependencies.json` was written at phase 144 and
verified on 2026-09-23. It predates the entire native backend: it has no
`native` component, and it says of KIR that "KIR text does not yet feed the
backend" and that the ownership target is "KIR-consumption and kcc-native
test-driver assembly (Phase 145+)".

Both of those statements are now half-true in a way that matters:

* the native backend IS Karkain-owned (`src/compiler/native_emit.kark`,
  `native_value.kark`, `native_elf.kark`, `native_pe.kark`, `native_macho.kark`),
  and the Go side (`pkg/native`) is a **reference implementation** used for
  byte-identity differential and container validation;
* KIR is still **not** the backend's input. The native driver consumes the
  **AST** directly. Wiring KIR into the backend is not done, and the matrix
  says so under "known limitations" rather than leaving it to be discovered.

The refreshed inventory records the native component, the reference-vs-active
distinction for `pkg/native`, and the corrected KIR statement.

---

## 5. What this slice does NOT claim

* **It does not claim cross-platform execution.** Only the Windows PE is
  executed here, and that is stated in the matrix itself rather than in a
  footnote.
* **It does not claim the native path is complete.** The matrix lists the
  constructs that are refused, and the refusal table proves each one is
  refused *by name* and writes no image.
* **It does not claim the compiler is independent.** The **compiler's own
  build** still uses `gcc` (stage 1 emits C, `gcc` links `kcc.exe`), and the
  **bootstrap** (increment 154's self-hosting proof) is still carried out of
  v1.2.0 to v1.3.0 as UNMET. What is proven here is narrower and is stated
  precisely: *compiling a user program* to a native image invokes neither a C
  compiler nor the Go toolchain.
* **It does not change the language.** No syntax, no semantics, no stdlib
  surface moved.

---

## 6. Measured results

* **Gate:** `TestPhaseNATQual_*` — 3 layers. Green: `ok karkain/pkg/cli ~27 s`.
* **Matrix:** 32 feature rows, 3 platform rows, 5 known limitations, all
  schema-checked and back-checked against the code and the gates.
* **Independence:** the native build's shim log is **empty**; the legacy
  build's log is **not** empty (the `gcc` shim fired).
* **The matrix's own gate found two defects in the matrix** on its first run
  (a status with no gate, and a gate name that did not exist). Both were
  fixed in the matrix, not in the gate.
