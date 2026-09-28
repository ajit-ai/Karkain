# Increment 151C2 — the Mach-O PIE container writer in kcc: final report

*Verdict: **COMPLETE (Mach-O).*** Slice C2 of increment 151, per
`PHASE-151-BASELINE.md` §4. PE is **not** in this report; it is 151C3, and §5
explains why splitting there is the right call.

---

## 1. Why Mach-O is the subtle one of the three containers

ELF is a header and two program headers. Mach-O is a **load-command chain
sized by its own contents**: `__TEXT` contains the header, the header contains
the load commands, `__LINKEDIT` is placed after the body, and one extra
`LC_SEGMENT_64` (72 bytes) appears only when the program needs a writable
arena — which moves `.text` and therefore moves the very offsets the rebase
stream addresses. So the writer and the rebase encoder are **entangled**, where
in ELF the header count is a simple consequence of whether data is present.

## 2. The rebase stream is the point of a PIE

The kernel loads a PIE at a slide it chooses, so every absolute address baked
into the text is wrong until dyld adds the slide. The stream in
`LC_DYLD_INFO_ONLY` tells dyld which pointer slots to fix. A site the stream
misses is **not a crash** — it is an image that loads and then dereferences a
pointer nobody slid. That asymmetry is why the gate *decodes* the stream rather
than assuming it.

The stream was encoded with the `cur`/`last` distinction preserved:

* `cur` is the **cursor** — the address the stream will visit next, which dyld
  advances by 8 after every rebase;
* `last` is the **last slot actually emitted**.

Two sites exactly 8 apart are *adjacent pointer slots*, not a duplicate. The
correct stream repositions **once** and rebases twice. Conflating the two
positions silently **drops** the second site, and a negative delta (what a
genuine duplicate looks like) falls through the range check and encodes as a
bogus forward hop. 150D's golden table caught both; this port keeps them apart.

## 3. What landed

**`src/compiler/native_macho.kark`** (new):

* `natMachoLink` — the image: the 32-byte header, `__PAGEZERO` /
  `__TEXT` / (`__DATA`) / `__LINKEDIT`, `LC_DYLD_INFO_ONLY`,
  `LC_LOAD_DYLINKER`, `LC_MAIN`, then `.text`, `.rodata`, the page gap, and the
  rebase stream;
* `natMachoRebaseOpcodes` — the stream encoder, with `cur` and `last` tracked
  separately;
* `natAppendUleb` — LEB128, whose continuation bits dyld depends on;
* `natSortInts` — an insertion sort, because Karkain has no sort and a program
  has only tens of sites;
* `natMachoTextOffset` — a function, not a constant, for the same reason the
  ELF writer's header count is: the header grows by one load command;
* `natMachoSegCmd` — `LC_SEGMENT_64`, with the 16-byte fixed-width name written
  character by character through the compiler's existing `asciiVal`.

**`native-macho`** — kcc subcommand plus Go-side dispatch
(`KCCNativeMachOCommand`). The three container/encoder commands now share one
`kccSubcommand` helper, since they genuinely are the same shape: no input file,
no fallback, exit code passed through, and **refusal lines treated as data**
rather than as build failures, because the reference corpus deliberately
includes refusal cases.

## 4. Two real bugs, both the 151B constant class — and I repeated the mistake

### 4.1 Two mistyped load-command ids

I wrote `LC_DYLD_INFO_ONLY` as **2147484194** (it is `0x80000022` =
**2147483682**) and `LC_MAIN` as **2147484712** (it is `0x80000028` =
**2147483688**). Every other constant in the file was right.

The consequence was silent and *doubly* confusing, which is worth recording:

* the oracle reported **"no LC_MAIN"** — a structural complaint that reads as
  "kcc built a broken image";
* and a test that scanned for the rebase stream, finding no matching command,
  fell back to offset 0 and read the **whole image** instead of the stream —
  inventing a second, entirely spurious fault out of the first.

This is exactly the failure 151B found in the encoder's opcode table (`0xFF`
written as 597, `0x0F` as 21). I had a gate for the encoder's constants and
still wrote two by hand in the new file.

### 4.2 The guard, added this time rather than after the fact

`TestPhase151C2_MachOConstantsAreIndependentlyComputed` now asserts the header
fields **and** the set of load-command ids, so a mistyped constant fails the
test instead of producing a plausible image. The lesson from 151B is now
enforced at the point of writing new code rather than remembered afterwards.

## 5. One wrong expectation, in the test

`TestPhase151C2_AdjacentSitesProduceNoRedundantReposition` initially failed with
"2 SET_SEGMENT_AND_OFFSET opcodes, want 1". That was **not** a writer bug — it
was §4.1's second symptom (the scan read the whole image), and it resolved the
moment the constant was right. Recorded because the initial diagnosis pointed at
the rebase encoder, which was correct all along.

## 6. Evidence

**Gate** `pkg/cli/phase151c2_macho_test.go` — 6/6 PASS (37.7 s).

| test | what it proves |
|---|---|
| `MachOImagesByteIdenticalToOracle` | the contract: kcc's images equal `native.LinkMachO`'s for all four cases |
| `MachORefusalMatchesTheOracle` | both writers refuse an unsupported text offset, and kcc names the offset it wanted |
| `MachOIsARealPIE` | MH_PIE set; images pass the **oracle's** `ParseMachO`; the stream **decodes** to the file offsets it should slide; `__DATA` presence follows the arena |
| `AdjacentSitesProduceNoRedundantReposition` | one reposition and two `DO_REBASE` for an adjacent pair, stream ends `DONE` |
| `MachOConstantsAreIndependentlyComputed` | header fields and load-command ids |
| `MachOImagesAreNonEmpty` | the vacuity guard |

**Structural validation uses the ORACLE's parser and its stream decoder** —
`ParseMachO`, `MachORebaseSites`, `MachOHasDataSegment` — rather than a check
written here, because a check in the writer's own language would share its
assumptions.

**The reference cases are chosen to make the stream shapes real**, not
decorative:

| case | shape | why |
|---|---|---|
| `single` | 3 segments, 1 site | the pre-150D shape |
| `arena` | 4 segments, 2 sites | `.text` moves 72 bytes; the sites must move with it |
| `nosite` | arena, 0 sites | a PIE with no baked addresses still needs a valid `SET_TYPE`+`DONE` stream — this is the case that catches an encoder emitting nothing |
| `adjacent` | sites 8 apart | exercises the `d == 0` branch, where the cursor already points at the next site |

**Mutation-verified**: making the `d == 0` branch emit a redundant reposition
fails **two** layers — the byte differential (4106 vs 4103 bytes) and the
adjacent-site opcode count.

**Regressions green**, isolated per the ~4 GB host rule:

| suite | result |
|---|---|
| `go build ./...`, `go vet ./pkg/... ./cmd/...` | clean |
| `pkg/native`, `pkg/lexer`, `pkg/parser`, `pkg/sema`, `pkg/target` | ok |
| `pkg/codegen` | ok (60.5 s) |
| Phase 151 / 151B / 151C / 151C2 / 150D / 148 gates | ok |
| Phase 122 KIRContinuity | ok (265.3 s), pin re-pinned 10031 → 10264 |
| kcc self-check of `src/compiler/main.kark` | `[ok]` |

`pkg/codegen`'s `TestPhase107_CodegenSpawnJoin` failed once in a combined run
and passes in isolation (3.8 s) — the documented ~4 GB-host OOM class, not a
regression; this slice touches no codegen.

## 7. Boundaries — what is NOT done

* **PE is not ported.** It is 151C3. It is the one container that can be
  **executed** on the dev host (PE images run in increments 145–150), so
  landing it would let the "no direct execution proof" boundary — which this
  report and 151B's both have to state — be closed rather than restated. It is
  also the hardest, per the baseline's risk register: PEB bootstrap, IAT,
  DIR64 relocations, and 16-byte alignment discipline.
* **No execution proof, and none claimed.** The images are byte-identical to
  `native.LinkMachO`'s, and every structural and rebase claim is checked
  against the oracle. There is no Intel-mac runner, so every Mach-O claim in
  this repository has been structural since increment 149; this slice does not
  change that, and does not pretend to.
* The rebase sites are **supplied** by the reference cases rather than
  derived, because deriving them needs the value model (151A). What this
  increment proves is that the writer places whatever list it is given at the
  right file offsets and encodes it correctly.

## 8. Governance

No frozen increment is reopened. This adds a compiler module, a subcommand, a
shared dispatch helper, and a re-pinned KIR count. No emitted byte of an
existing golden moves. `kccOwnsNativeTargets` stays `false`.

**Next: 151C3 — PE**, then 151A (the value model), then 151D (flip
`kccOwnsNativeTargets` and re-run the 148/150 gates with the kcc engine
unpinned).
