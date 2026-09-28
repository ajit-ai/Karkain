# Increment 151B — the machine-code encoder in kcc: final report

*Verdict: **COMPLETE**.* Slice B of increment 151, per
`PHASE-151-BASELINE.md` §4. It is the "subtle slice" the baseline calls out:
the Go emitter's eager label resolution is exactly what makes byte-identical
images possible, so this port reproduces that discipline rather than improving
on it.

---

## 1. What landed

**`src/compiler/native_emit.kark`** (new) — the self-hosted byte encoder:

* a byte buffer carried as an array (Karkain has no structs, so the emitter
  state threads through one array with fixed slots);
* a label table with a linear-scan lookup;
* `rel32` fixups recorded as `(position, label)` pairs and resolved in one
  pass at finish, so instruction *lengths* are known at emission time while
  *targets* are not;
* `imm64` placeholders, whose position is recorded for a linker and never
  resolved by the encoder — filling absolute addresses is the linker's job;
* REX / ModRM computation mirroring `pkg/native/emit.go` exactly, including
  suppressing a redundant `0x40` prefix;
* two refusals, `error[K117]`: an undefined label and a duplicate label.

**`native-encode`** — a kcc subcommand plus Go-side dispatch
(`pkg/cli/native_encode.go`, `cmd/karkain/main.go`). It takes **no input
file**: the reference sequences are compiled into kcc, which is what makes the
comparison a comparison of two *implementations* rather than of two
invocations of one. There is deliberately **no Go fallback** — a Go encoder
producing the same bytes would make the gate pass while proving nothing about
kcc, which is the exact dishonesty increment 151 was opened to remove.

**`pkg/cli/phase151b_encoder_test.go`** — 7 tests.

## 2. Two real bugs found, both by decoding bytes by eye

Neither was caught by a test at the time, and both are now gate-pinned. They
are recorded in full because the pattern — an x86 encoder with hand-converted
constants and no differential — will recur.

### 2.1 Every constant in the table was wrong (three of them)

Karkain has no hex literals, so every x86 constant is a hand-written decimal.
The table was produced by converting decimal-*looking* strings as if they
were hex, which turned:

| intended | written | consequence |
|---|---|---|
| `0xFF` (255) | 597 = `0x255` | byte truncation and hex rendering wrong |
| `0x0F` (15) | 21 = `0x15` | hex nibble extraction wrong |
| `0x90` (144) | 90 = `0x5A` | the nop was not a nop |

The output was `4010445400141000000005541` for a `mov rax, imm64` — not x86
at all. It was caught by *reading* the bytes, not by any assertion, because
the differential against the oracle had not been written yet.

**Guard now in place:** `TestPhase151B_OpcodeConstantsAreIndependentlyComputed`
states seven constants as what `pkg/native` actually emits, so a mistyped
selector fails the test instead of reaching an image.

### 2.2 ModRM double-counted the mode field

```
natModrm(s, mod, reg, rm)  ->  natByte(s, 192 + (mod << 6) + (reg << 3) + rm)
```

`192` is `0xC0`, which **is** `mod=3` already shifted. Adding it on top of
`mod << 6` made `mod=3` produce `0xC0 + 0xC0 = 392`, which truncated to
`0x88` — a *memory* operand where a *register* operand was meant. `add rax,
rcx` encoded as `48 01 88` instead of `48 01 c8`.

This one is worth dwelling on: the `imm64` and `multi` sequences contain **no
ModRM at all**, so two thirds of the corpus was unaffected and a differential
over those sequences would have stayed green. Only the loop sequence exposed
it, and only by being decoded by eye.

**Guard now in place:** the differential covers the loop, and the mutation
was re-run to confirm the gate fails with exactly these bytes.

### 2.3 An oracle/native asymmetry, fixed at the source

`pkg/native.Emitter` had **no** `Nop()`, so the Go mirror of the `imm64`
sequence could not reproduce the two padding bytes. Rather than drop the
padding (which is load-bearing: a forward fixup whose displacement happens to
be zero produces identical bytes whether the fixup pass ran or not) a `Nop()`
was added to the Go emitter. A primitive present on only one side of a
bit-exact port is an asymmetry that would surface later anyway.

## 3. Evidence

**Gate** `pkg/cli/phase151b_encoder_test.go` — 7/7 PASS (21.0 s).

| test | what it proves |
|---|---|
| `EncoderByteIdenticalToOracle` | the contract: kcc's bytes equal the Go oracle's for all three sequences |
| `Rel32DisplacementsAreArithmeticallyCorrect` | displacements pinned from first principles, not from the oracle |
| `LoopSequenceDisassemblesToTheIntendedCode` | the loop *is* the intended x86, in stated bytes |
| `RefusalsMatchTheOracle` | both engines fail on an undefined label and a duplicate |
| `EncoderIsDeterministic` | two runs, identical output |
| `OpcodeConstantsAreIndependentlyComputed` | seven constants vs what `pkg/native` emits |
| `SequencesAreNotEmpty` | no differential over empty output (the vacuity guard) |

**The gate is a differential, not a golden.** Every expected value is produced
by calling the real `pkg/native.Emitter` in the test file, so the two
implementations are compared to each other. A golden would pin kcc against a
transcription of the Go code — the kind of test that passes when both copies
are wrong the same way, which is exactly bug 2.1.

**Four independent layers, deliberately**, because each catches a different
class of wrong:
1. differential vs the oracle (catches any byte difference);
2. arithmetic from offsets (catches a *shared* sign error);
3. the disassembly golden (catches a *shared* drift in the sequence definition);
4. the constants table (catches a mistyped selector).

**Mutation-verified**: reintroducing the ModRM double-count fails the
differential with exactly the bytes from §2.2.

**Regressions green**, all run isolated per the ~4 GB host rule:

| suite | result |
|---|---|
| `go build ./...`, `go vet ./pkg/... ./cmd/...` | clean |
| `pkg/lexer`, `pkg/parser`, `pkg/sema`, `pkg/codegen`, `pkg/compiler` | ok |
| `pkg/native` | ok (15.1 s) |
| Phase 151 / 151B / 150D / 148 gates | ok (18.0 s) |
| Phase 122 KIRContinuity | ok (286.0 s), pin re-pinned 9642 → 9889 |
| kcc self-check of `src/compiler/main.kark` | `[ok]` |

The KIR pin moved 9642 → 9889 because the new `native_emit.kark` module and
its driver arm are rendered by KIR; `kir verify` passes on the new count.

## 4. Boundaries — stated rather than papered over

* **No execution proof of the kcc-encoded bytes, and none claimed.** The
  encoder's output is byte-identical to `pkg/native`'s, whose images *are*
  executed on this Windows host by the increment 145–150 PE suites — so the
  correctness argument is transitive, and it is stated that way rather than
  dressed up as a direct run. Executing kcc-encoded bytes *directly* would
  need the PE PEB bootstrap, which is `program.go`'s job (151A/151C), not the
  encoder's.
* The encoder covers a representative opcode subset — `mov` (imm32/imm64),
  `add`/`sub` reg-reg, `jnz`, `jmp`, `call`, `nop`, `ret` — enough to prove
  the *mechanism* byte-for-byte. The remaining ~80 primitives of
  `pkg/native/emit.go` are mechanical once the mechanism is proven, and are
  151C/151A work.
* The imm64 placeholder's *position* is recorded in kcc but is not yet
  consumed by anything; the linker that would fill it is 151C.

## 5. Governance

This slice reopens no frozen increment. It adds a new compiler module, a new
subcommand, one primitive to `pkg/native` (`Nop`), and a re-pinned KIR count.
No emitted byte of an existing golden moves: the encoder emits into its own
buffer and nothing else in the tree consumes it yet.

**151C is the next slice** — the three containers (ELF, PE with the PEB
bootstrap and Win64 boundary, Mach-O PIE with rebase opcodes) in kcc, matching
`Link`/`LinkPE`/`LinkMachO` byte for byte. The baseline's risk register still
stands and is worth repeating: PE is the hardest container to match byte for
byte and may need to land after ELF.
