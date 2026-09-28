# Increment 151C — the ELF64 container writer in kcc: final report

*Verdict: **COMPLETE (ELF).*** Slice C of increment 151, per
`PHASE-151-BASELINE.md` §4. Mach-O and PE are **not** in this report; the
baseline's risk register said PE may need to land after ELF, and the evidence
below says the same thing is true of Mach-O. See §5.

---

## 1. Why the container could land before the value model

`native.Link` takes **already-encoded bytes** — `(text, rodata, data)` plus
two offsets — and knows nothing about functions, arrays, maps or strings. The
container is therefore *pure serialisation*, and it can be proven
byte-identical to the Go oracle using **fixed input bytes**, with no value
model anywhere in the picture. That is what made 151C reachable while 151A is
still open, and it is the reason the increment is smaller than it looks.

## 2. What landed

**`src/compiler/native_elf.kark`** (new) — the ELF64 container writer:

* `natELFLink` — the full image: identification bytes, the 64-byte header, one
  R+X `PT_LOAD`, an optional second R+W `PT_LOAD` for the arena, then `.text`,
  `.rodata`, a zero-filled gap to the page boundary, and the arena;
* `natAlignUp` — round up to a power of two, written as "add, then clear the
  low bits" (`x - (x & 0xFFF)`) because Karkain has no `&^`;
* `natPut16/32/64` — little-endian field writers, matching `put16/32/64`;
* `natHexDecode` — hex string to bytes, because Karkain has no byte string
  literal, so the test surface is hex in and hex out;
* one refusal: an unsupported `.text` offset.

**`native-elf`** — kcc subcommand plus Go-side dispatch
(`KCCNativeELFCommand`), same no-Go-fallback contract as 151B's
`native-encode`.

**`pkg/cli/phase151c_container_test.go`** — 5 tests.

### Layout, and the two decisions that matter

```
+----------------------------+  file 0 / vaddr 0x400000
| ELF header        (64 B)    |
| program header 1  (56 B)    |  R+X: .text and .rodata
| [program header 2 (56 B)]   |  R+W: the arena, only when there is one
| .text                      |
| .rodata                    |
| [zero-fill to a page]      |
| .data (the arena)          |
+----------------------------+
```

* **The header count follows the arena, nothing else.** A program with no
  heap gets *exactly one* `PT_LOAD` and `.text` immediately after it — the
  pre-150B layout, byte for byte. Preserving that is what keeps increment
  149's identity pins honest, and the `rodata` reference case exists to prove
  the count follows the *arena* and not the rodata.
* **The arena is page-aligned by OFFSET, not sized to a page.** Its file
  offset is rounded up so that `p_offset` and `p_vaddr` share the same residue
  modulo 4096, which is the requirement the ELF loader actually imposes. The
  R+X segment stops before the arena so the two mappings cannot overlap. Both
  are asserted in `TestPhase151C_ELFLayoutFacts`.

## 3. Three defects found, and one that was mine, in the test

### 3.1 `entryOffset` is a FILE offset, not a `.text`-relative one

This is the one worth reading twice. `natELFLink` writes
`e_entry = BaseAddr + entryOffset`, and the loader compares that against
`BaseAddr + textOffset` — so a `.text`-relative value lands *before* `.text`
and is rejected. `pkg/native/elf.go` says the same in its own comment
("entryLabel is resolved by the caller to a file offset before calling").

The first draft of the gate passed 15, meaning "15 bytes into `.text`", and
every image was rejected with `entry 0x40000f outside image`. The message
names the *writer*, so the mistake reads as "kcc emitted a bad entry" when in
fact the test asked for an entry outside `.text`. It is now documented in the
Karkain source, in the test constant, and here.

### 3.2 A wrong expectation about page-sized images

The same test asserted the arena image's **total size** was a multiple of
4096. It is not, and should not be: the arena is placed at a page-aligned
offset and its own length need not be a whole number of pages. The assertion
now checks the *offset* and the offset/vaddr residue agreement, which is the
real requirement.

### 3.3 A 47-byte "32-byte" arena fixture

The arena fixture's hex was 94 characters, so it decoded to 47 bytes, not the
32 its comment claimed. The image was therefore 4143 bytes where 4144 was
expected. No writer was involved — the count was off by my own arithmetic
when reading the fixture. It is now a clean 32 bytes, and the lesson from
151B applies: **measure, do not count by eye.**

## 4. Evidence

**Gate** `pkg/cli/phase151c_container_test.go` — 5/5 PASS (19.7 s).

| test | what it proves |
|---|---|
| `ELFImagesByteIdenticalToOracle` | the contract: kcc's images equal `native.Link`'s for all three cases |
| `ELFRefusalMatchesTheOracle` | both writers refuse an unsupported text offset, and kcc names it |
| `ELFStructuralParse` | kcc's images parse under the **oracle's** `Parse`, with per-case entry and text offset |
| `ELFLayoutFacts` | one vs two `PT_LOAD`s, no-arena size, page-aligned arena offset, offset/vaddr residue agreement, no segment overlap |
| `ELFImagesAreNonEmpty` | the vacuity guard |

**The gate is a differential, not a golden** — every expected image comes from
calling the real `native.Link` in the test, so the two writers are compared to
each other. A golden would pin kcc against a transcription and pass when both
copies are wrong the same way.

**Structural validation is done by the ORACLE's parser, not by kcc's own.**
A check written in the same language as the writer would share its
assumptions; `native.Parse` already validates increments 145–150's images, so
a kcc image that parses under it is independent confirmation.

**The layout facts are read out of the IMAGE**, not from the writer's
variables (`readPh64` decodes the bytes). A test that consulted the writer for
its own expected values would be circular.

**Mutation-verified**: removing the arena page-alignment fails
`ELFImagesByteIdenticalToOracle`.

**Regressions green**, isolated per the ~4 GB host rule:

| suite | result |
|---|---|
| `go build ./...`, `go vet ./pkg/... ./cmd/...` | clean |
| `pkg/native`, `pkg/lexer`, `pkg/parser`, `pkg/sema`, `pkg/codegen` | ok |
| Phase 151 / 151B / 151C / 150D / 148 gates | ok |
| Phase 122 KIRContinuity | ok (292.6 s), pin re-pinned 9889 → 10031 |
| kcc self-check of `src/compiler/main.kark` | `[ok]` |

The KIR pin moved 9889 → 10031 because `native_elf.kark` and its driver arm
are rendered by KIR.

## 5. Boundaries — what is NOT done

* **Mach-O and PE are not ported.** The baseline's risk register said PE is
  the hardest container to match byte-for-byte (PEB bootstrap, IAT, DIR64
  relocations, 16-byte alignment) and "may need to land after ELF". The
  evidence here supports the same for Mach-O: its rebase-opcode stream
  (150D) and `__DATA`/`__LINKEDIT` page-alignment rules are a second, larger
  layout. They are 151C2 and 151C3, not omissions.
* **No execution proof, and none claimed.** The images are byte-identical to
  `native.Link`'s, whose ELF images *are* executed by increment 145–150's
  Linux gates — so the argument is transitive and is stated that way. There is
  no Intel-mac or native-ELF runner on this host.
* The images here carry a **real 151B-encoded loop body**, not a whole
  program: no entry stub, no syscall tail, no PEB bootstrap. Those come with
  151A's value model and are what 151D flips on.

## 6. Governance

No frozen increment is reopened. This adds a compiler module, a subcommand,
and a re-pinned KIR count. No emitted byte of an existing golden moves: the
container builds into its own buffer and nothing else in the tree consumes it
yet. `kccOwnsNativeTargets` stays `false`.

**Next: 151C2 — Mach-O, then PE.** After those, 151A (the value model) is
what actually routes a user program through this writer, and 151D flips
`kccOwnsNativeTargets` and re-runs the 148/150 gates with the kcc engine
unpinned.
