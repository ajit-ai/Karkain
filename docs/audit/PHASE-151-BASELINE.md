# Phase 151 BASELINE — kcc native parity

Date: 2026-09-27. Version-plan slot: 1.2.0 "Sovereignty I", increment 151
(second of 150 → 151 → 152 → 154; 165 free-floating). Parent report:
`PHASE-150-BASELINE.md` §13. This file is the mandatory baseline written
**before** implementation, per `KARKAIN-VERSION-PLAN.md` §1.

## 1. Starting position (verified in-tree, not assumed)

* **Increment 150 is COMPLETE** (all eight slices, 150A–150D). `pkg/native`
  owns the full boxed Value model, register allocation, three OS containers
  (ELF / PE / Mach-O) and the three CLI targets
  (`native-x86_64-{linux,windows,macos}`).
* **kcc has ZERO native codegen.** Verified: no occurrence of `elf`, `pe`,
  `macho` or `x86` in `src/compiler/*.kark`. The self-hosted engine emits
  **C23 only**; there is no machine-code path to be made byte-identical yet.
* **kcc's whole command surface** (`src/compiler/main.kark`) is
  `check`/`build`/`run`/`test`/`kir`/`verifykir` — all C-emitting.
* **The seam 151 changes is a Go-side dispatch**, not kcc code. In
  `cmd/karkain/main.go` the kcc branch routes any target that is not
  `native`/`c23` to the **Go** engine, and `pkg/cli/commands.go` then
  dispatches `IsNativeTarget(cfg.Target)` to `cfreeBuildForOS`/`cfreeRunForOS`,
  which are Go-only and call `native.CompileProgramForOS`. So today
  `KARKAIN_ENGINE=kcc karkain build --target native-x86_64-linux` is a
  **silent Go fallback**: the user asked for kcc and got the Go backend. That
  is exactly the class of dishonesty the Phase-111 rule forbids, and removing
  it is 151's first job.
* **Error-code space:** the native backend owns exactly one code, `K145`
  (`error[K145]: ...` in `pkg/native/program.go`), used for every refusal and
  unsupported shape. `pkg/wasm` uses `K108`. The K1xx range documented in
  `stable-api.rst` currently ends at K115.
* **KIR pin 9574** is unaffected by native work (the native path does not
  consume KIR), but 151 **will** grow `src/compiler/*.kark`, so the pin must be
  re-measured and re-pinned in the same commit (the standing rule from 122).

### 1.1 The silent Go fallback — MEASURED, not inferred

The claim above was verified by running the real binary, not by reading code:

```text
$ KARKAIN_ENGINE=kcc karkain build hello.kark \
      --target native-x86_64-linux -o kcc.elf --verbose
=== [Verbose] Lexer Token Stream ===      <- GO lexer, not kcc
=== [Verbose] Parsed AST Statements count: 1 ===
=== [native-x86_64-linux] BUILD (no C compiler) ===   <- Go dispatch banner
native image: 351 bytes -> kcc.elf
```

The two images are **byte-identical**:

| Engine | SHA-256 |
|---|---|
| `KARKAIN_ENGINE=kcc` | `237f5f1a...4b13` |
| `KARKAIN_ENGINE=go` | `237f5f1a...4b13` |

The kcc run's hash is also the pre-existing `p145_int42` golden in
`nativeLegacyELF`, so the "kcc" image is literally the Go image.

**Consequence for 151's exit criteria:** the existing 148/150 gates cannot
distinguish kcc parity from Go fallback, because the fallback makes every
native build succeed and look byte-identical by construction. A gate that only
compares images would pass today without a line of kcc code. That is why §8
requires those gates re-run with the **kcc engine unpinned**, and why the
parity harness must assert *which engine* produced the image.


## 2. Contract (from `KARKAIN-VERSION-PLAN.md` §4, quoted)

| 151 | I | **kcc native parity** — the self-hosted engine selects and emits the same native targets | One corpus file per native feature byte-identical Go↔kcc; 148/150 gates re-run with the kcc engine un-pinned | new native features (that is 150's contract) |

## 3. What "parity" must mean here (and what it must not)

**Must:** for every native feature increment 150 delivered, `kcc` and the Go
engine produce **byte-identical images** for the same source and target. The
existing Phase-109/123 precedent is explicit — parity is asserted on golden
bytes, never on "both ran without error".

**Must not:**

* **No new native features.** Anything 150 could not do, 151 cannot add. A
  151 "improvement" only kcc can do is a parity break, not a parity fix.
* **No silent Go fallback.** If kcc cannot lower a construct it must be a

## 4. Slice plan (proposed)

* **151A — the native value model in kcc.** Port the 150A–150B surface to
  `src/compiler`: frame layout, the boxed kinds, arrays/`for-in`, floats, the
  arena, string ops, `push`, records, maps. Gate: one corpus file per feature,
  **byte-identical Go↔kcc**. The Go `pkg/native` is the oracle; kcc is the
  implementation under test.
* **151B — the encoder in kcc.** `Emitter` equivalents (rel32 backpatching,
  the imm64 patch lists) in Karkain. This is the subtle slice: the Go
  emitter's "resolve references eagerly" discipline is exactly what makes the
  images byte-identical, so the port must reproduce it, not improve on it.
* **151C — the three containers in kcc.** ELF, PE (with the PEB bootstrap and
  the Win64 boundary) and Mach-O PIE + rebase opcodes, matching
  `Link`/`LinkPE`/`LinkMachO` byte for byte. Gate: image SHA-256 equality per
  OS, plus structural parse.
* **151D — the dispatch + the no-fallback rule.** Route the three native
  targets to kcc in `main.go`; make any unsupported construct a loud K1xx;
  re-run the **148 and 150 gates with the kcc engine unpinned** so they can no
  longer pass by accident through the Go fallback.
* **Gate file:** `pkg/cli/phase151_kcc_native_test.go`.

## 5. The measurement that must come first

Before writing any codegen, 151 needs a **byte-parity harness**: given a source
and a target OS, compile with Go, compile with kcc, and compare image bytes.
Without it, every parity claim in this increment would be an assertion. The
harness is a small deliverable, and it is the thing that makes 151A–151C
verifiable rather than hopeful.

## 6. Risk register (stated now, not discovered later)

1. **154 remains the schedule risk, not 151.** 151 is a large but
   well-understood port. 154 needs `stage2 == stage3` with no Go toolchain,
   and bootstrap stage-2/3 currently OOM/SEGFAULTs on the ~4 GB dev host behind
   the K127 guard. That is a hardware gate, not an effort estimate.
2. **Bootstrapping kcc-from-kcc needs the Go toolchain.** 151 can be developed
   and gated with Go present (Go is the *oracle*, not the *implementation*).
   Only 154 removes that dependency.
3. **PE is the hardest container to match byte-for-byte** (PEB bootstrap, IAT,
   DIR64 relocations, 16-byte alignment). It may need to land after ELF.
4. **KIR pin drift is expected** and must be re-measured, not hand-edited.

## 7. Explicitly OUT OF SCOPE for 151

* New native features (increment 150's contract).
* arm64 native; PE delay-load / TLS / SEH / resources / signing.
* Mach-O **execution** (no Intel-mac runner exists; structural only).
* The native-split incremental cache (150D closed it as a gated, measured
  refusal — 151 does not reopen it).
* Removing the C path or the Go tree (154 / later).

## 8. Exit criteria

- [ ] Byte-parity harness exists and compares **image bytes**, not exit codes.
- [ ] One corpus file per 150 native feature, byte-identical Go↔kcc.
- [ ] All three containers match (ELF required; PE/Mach-O may ship in slices
      with their own parity evidence, never as a silent gap).
- [ ] The Phase 148 and Phase 150 gates pass with the **kcc engine unpinned**
      (proving no Go fallback is doing the work).
- [ ] An unsupported construct on the kcc path is a **loud** refusal, tested.
- [ ] KIR pin re-measured and re-pinned in the same commit.
- [ ] `go build ./...`, `go vet`, `pkg/native`, `pkg/cli` neighbours green.
- [ ] AGENTS.md record; commit `develop` → merge `main` → push (hard rule).

## 9. Progress

### 151-prelude — the byte-parity harness (DONE)

`pkg/cli/phase151_parity_test.go`. Written before any codegen, as §5 requires.

**The instrument, and the trap it avoids.** A parity test that compares image
bytes is **not evidence** here, because the silent Go fallback makes the bytes
match by construction. So every comparison in the harness pairs the byte
comparison with a **provenance** assertion: `kccProducedImage(out)` checks for a
kcc-native emission marker the Go fallback cannot imitate. When 151A lands it
becomes the switch that flips; until then it is honestly false.

**Self-test result** (the state 151A–151C must close):

```
CONFIRMED FALLBACK: the kcc leg emitted no kcc-native provenance;
its image 237f5f1ab09a is the Go image 237f5f1ab09a.
```

**Positive control** — `TestPhase151_HarnessComparesBytes` builds the same
source for two different containers and requires different hashes, so the
harness cannot silently stop comparing bytes. It was **mutation-verified**:
forcing the branch makes the test FAIL with the real digests
(`elf=237f5f1a…`, `pe=64916445…`), proving the gate is live rather than
vacuously green.

### 151A–151D — NOT STARTED


  **loud** refusal naming the construct, never a quiet hand-off to the Go
  engine. An invisible fallback is worse than an error.
* **No C-path change.** `--target c23` behaviour is byte-frozen by this
  increment; kcc's C23 output must not move while native is added.
