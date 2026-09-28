# Increment 151C3 — PE32+ container writer in kcc: final report

*Verdict: **COMPLETE (PE32+).*** Slice C3 of increment 151, per
`PHASE-151-BASELINE.md` §4. This is the last of the three containers and the
one that matters most: it is the only container a Windows loader will accept
and run.

---

## 1. Implementation

| File | Purpose |
|---|---|
| `src/compiler/native_pe.kark` (new) | The self-hosted PE32+ serialiser: DOS header, PE signature, COFF header, PE32+ optional header, three section headers, the `.idata` body, `.reloc` DIR64 blocks, and the four patch-class resolutions. |
| `src/compiler/main.kark` | Adds the `native-pe` measurement subcommand. |
| `pkg/cli/native_encode.go` | Adds `KCCNativePECommand`, reusing the shared `kccSubcommand` helper. |
| `cmd/karkain/main.go` | Dispatches `native-pe` and accepts it as a command word. |
| `pkg/native/phase151c3_pe_test.go` (new) | The 151C3 gate, the independent structural oracle, and the mutation table. |
| `.github/workflows/ci.yml` | Adds the 151C3 step (see §8). |
| `pkg/cli/phase122_pipeline_ownership_test.go` | KIR pin re-pinned 10264 to 10530. |

## 2. PE layout

```text
0x000  DOS header (64 bytes)        MZ; e_lfanew at 0x3C
0x040  padding to 0x80
0x080  PE\0\0
0x084  COFF header (20)             Machine 0x8664, 3 sections,
                                     TimeDateStamp 0, Characteristics 0x22
0x098  Optional header (240, PE32+)  magic 0x20B, ImageBase 0x140000000,
                                     SectionAlignment 0x1000,
                                     FileAlignment 0x200, Subsystem 3,
                                     MajorSubsystemVersion 6,
                                     DllCharacteristics 0x160
0x188  Section table (3 x 40 = 120)
0x200  .text   R-X  code + rodata    headers land on EXACTLY 0x200
0x400  .idata  RW  imports + arena   must be writable: the loader writes
                                     resolved addresses into the IAT
0x600  .reloc  RS  DIR64 fixups      DYNAMIC_BASE is only honest with these
```

Three decisions worth stating:

* **`.idata` is not optional and must be writable.** Windows has no stable
  user-mode syscall ABI, so exit and output go through `kernel32.dll`
  imports; the loader resolves them by *writing* into the IAT slots, and a
  read-only `.idata` fails the load with `ERROR_BAD_EXE_FORMAT` (Phase 149,
  diagnosed with objdump). The arena rides inside `.idata` because it is
  already R/W, which avoids a fourth section and keeps the import
  directories pointing only at import structures.
* **`.reloc` is not decoration.** The host loader rebases the image even with
  no relocation table (Phase 149 measured a live base of `0x7FF6...` instead
  of the linked `0x140000000`), which makes every absolute `movabs` stale
  and faults immediately.
* **The headers occupy exactly 0x200** — `0x80 + 4 + 20 + 240 + 3*40 = 512`
  exactly. There is **zero slack**, so the writer refuses if its headers ever
  overflow that value rather than silently truncating. Recorded because it is
  a trap for any future header growth.

Only three data directories are claimed — import, base relocation, IAT — and
only those three are actually generated. Claiming a directory that carries no
data is a claim the loader could act on.

## 3. Self-hosting

The PE serialisation runs entirely inside the self-hosted engine. Evidence:

* `src/compiler/native_pe.kark` contains the whole serialiser and **no
  reference to `LinkPE` or any Go symbol**. The only mention of
  `pkg/native/pe.go` is a prose comment naming it as the thing being matched.
* The measurement path is `kcc native-pe` -> `natPECorpus` -> `natPELink`,
  all in `src/compiler`. There is no branch in the self-hosted path that
  delegates to Go, which is the specific failure mode increment 151 was
  opened to remove (the measured silent Go fallback).
* `kcc check src/compiler/main.kark` is `[ok]`: the compiler still accepts its
  own new source.
## 4. Tests — `pkg/native/phase151c3_pe_test.go`, 9 tests PASS

| test | what it proves |
|---|---|
| `PEByteDifferential` | the contract: kcc's image equals `native.LinkPE`'s, byte for byte, for all three cases |
| `PEStructuralOracle` | an independent validator accepts every emitted image |
| `PEEntryRVAIsIndependentlyComputed` | the entry RVA recomputed from the spec, and confirmed inside `.text` |
| `PEDeterminism` | two runs, identical bytes |
| `PERefusal` | unsupported text offset refuses on both sides, naming the offset |
| `PEMutationVerification` | 15 mutations, each rejected |
| `PERawExtentBeyondFile` | a section running past the file end is rejected |
| `PEConstantsAreIndependentlyComputed` | header fields read out of an emitted image at spec offsets |
| `PEExecutionStatus` | records what was and was not proved (see §7) |

### The independent structural oracle

`peRead` and `peValidate` are written from the PE/COFF specification, not from
the writer. They read every field at a hand-derived offset with
`le16`/`le32`/`le64` and recompute every relationship. They never call
`LinkPE`, `natPELink`, or any writer helper, and never read a writer
variable.

It checks: `MZ`; `e_lfanew`; `PE\0\0`; COFF machine, section count,
timestamp, optional size and characteristics; optional magic; `ImageBase`
(64-bit); both alignments; subsystem and its minimum version;
`DllCharacteristics`; `NumberOfRvaAndSizes`; the three claimed directories;
`SizeOfHeaders` covering the headers and file-aligned; each section's name,
RVA, characteristics and both alignments; raw extents inside the file;
`.text` not overlapping the headers; **no two sections overlapping**;
`SizeOfImage` covering the virtual high-water mark and section-aligned;
`BaseOfCode`; the entry inside `.text`; and `SizeOfCode` /
`SizeOfInitializedData` against the section table.

## 5. Differential validation

**Reference implementation: `pkg/native.LinkPE`** — increment 149's Go PE
writer, which is the oracle for the whole increment. Byte identity is an
established contract here, since that is what makes the two engines' images
indistinguishable, so the differential asserts full byte equality for all
three cases.

The gate lives in `package native` rather than `pkg/cli` because `LinkPE`
takes a `*Builder` whose `patches` / `ipatches` / `apatches` / `hpatches`
fields are **unexported**; a `pkg/cli` gate cannot construct a `Builder` with
a chosen patch set. All four patch classes are exercised, each at a
**distinct** `.text` site, so a last-writer-wins bug would be visible rather
than hidden.

The first draft of this test passed `rodata` to the `bare` case, which has
none, so the Go side computed `.text` VirtualSize 48 where kcc computed 40.
The reported byte difference was the test's fault, not the writer's. The
inputs are now per-case.

## 6. Mutation verification

Fifteen single-field mutations plus one raw-extent mutation, each of which the
independent validator **must reject**: corrupt `MZ`; corrupt `e_lfanew`;
corrupt `PE\0\0`; wrong machine; wrong section count; wrong optional magic;
entry RVA outside `.text`; entry RVA beyond the image; unaligned
`SectionAlignment`; unaligned `FileAlignment`; `SizeOfImage` too small;
`SizeOfHeaders` too small; unaligned section RVA; unaligned `SizeOfRawData`;
section overlap; raw extent past the end of the file.

The table is also **mutation-verified against the writer**: changing the entry
field to write the raw file offset instead of converting it to an RVA fails
the byte differential on all three cases.

### A real finding — in the gate, and it matters generally

Six of the thirteen first-draft mutations were **no-ops**. They poked a single
low byte at fields whose low byte was already `0`, or already equal to the
written value: `SectionAlignment` is `0x1000`, `FileAlignment` `0x200`,
`SizeOfImage` `0x3000`, `SizeOfHeaders` `0x200`, and the Machine field's low
byte is already `0x64`. Writing `0x00` changed nothing, and the validator
correctly accepted an *unmutated* image — which reads exactly like a
validator bug.

**A mutation that changes no bytes cannot demonstrate anything.** The table now
writes 32-bit values, and the trap is documented in the test so the next
mutation table in this repository does not repeat it.

Two further wrong expectations, both in the test and both corrected rather than
worked around:

* Three header fields were read with `le32` when they are 16-bit, so the
  assertions picked up the neighbouring field: `Machine` read `0x038664`,
  `SizeOfOptionalHeader` read `0x002200f0`, `Subsystem` read `0x01600003`.
  PE32+ mixes 2- and 4-byte header fields, so the width is part of the
  definition, not a detail of the check.
* `SizeOfImage` was asserted as the literal `0x3000` and the gate failed with
  `0x4000`: the real high-water mark is `.reloc`'s RVA plus its 16-byte block
  (12304), which rounds **up** to the next `0x1000` boundary. That value is
  now **recomputed from the section table** rather than asserted, because a
  literal there is a guess dressed as a requirement.

## 7. Execution — stated separately, and honestly

> PE structural validity was verified; execution of newly generated
> Phase-151C3 images was not established by this phase.

This is deliberate, and `TestPhase151C3_PEExecutionStatus` records it in code
so it cannot be quietly upgraded later. The reference `.text` is four bare
`mov` instructions: there is no entry stub, no syscall tail and no PEB
bootstrap. Those belong to `program.go` and to 151A's value model, not to a
container. There is therefore nothing here a Windows loader could usefully
run, and claiming execution would be false.

The execution evidence that DOES exist for PE images in this repository
belongs to increments 145-150, and those images were produced by the **Go**
writer. Substituting that evidence for 151C3 evidence is precisely the
conflation this section refuses.

## 8. CI

The gate is wired, and the step is named explicitly rather than left implicit:

```yaml
- name: Run Phase 151C3 PE container gate
  run: go test ./pkg/native/ -run 'TestPhase151C3' -count=1 -timeout 20m -v
```

The existing `go test ./pkg/native/ -count=1` step would also have picked
this up, but relying on that is exactly how four increment gates (151P0,
151B, 151C, 151C2) were shipped without ever executing in CI. The step is
therefore written out.

## 9. Refusal behaviour

One refusal, matching the Go oracle: an unsupported `.text` offset. The
message names both the value received and the value wanted, so a reader can
tell which caller computed the wrong offset. The writer also refuses
internally if its headers ever overflow `peTextOff` — the zero-slack
condition described in §2 — rather than emitting a truncated image.

No new general error architecture was introduced. The refusal is a returned
diagnostic string, the same shape 151B/151C/151C2 use, so the caller decides
what it means and the existing `error[K117]` convention still applies.

## 10. Limitations — documented, not defects

* **No execution claim.** See §7.
* **Structurally validated, not loader-validated.** Every field is checked
  against the specification by an independent reader, but no Windows loader
  has seen a 151C3-produced image.
* **The rebase sites are supplied, not derived.** The four patch lists are
  inputs. Deriving them from a real program is `program.go`'s job and lands
  with 151A's value model. What this phase proves is that the writer places
  whatever lists it is given at the right file offsets and encodes them
  correctly.
* **arm64 native is still out of scope**, as are PE delay-load, TLS, SEH,
  resources and code signing — all increment 149 boundaries, unchanged.

## 11. Governance

No frozen increment is reopened, and no phase is redefined. This adds a
compiler module, a subcommand, a gate, a CI step, and a re-pinned KIR count.
`VERSION` is unchanged at 1.1.0, Phase 152 is untouched, and no new phase
number is invented. `kccOwnsNativeTargets` stays `false`: 151C3 completes the
three containers but nothing yet routes a user program through them, which is
151A's value model and then 151D's dispatch.

The **Language Hardening Checkpoint** is not started. It is recorded in
`LANGUAGE-HARDENING-CHECKPOINT.md` as the gate that follows 151C3, and this
report is the only phase-status reference made to it.

**Increment 151 now has all four mechanisms the baseline asked for** — 151B
the encoder, 151C ELF, 151C2 Mach-O, 151C3 PE — all byte-identical to the Go
oracle. What remains inside 151 is 151A, the value model, which is what
actually produces the bytes these four serialise.
