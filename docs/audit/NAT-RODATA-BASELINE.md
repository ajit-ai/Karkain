# NAT-RODATA baseline - the reporter's read-only data: interning order and offsets

**Slice:** `NAT-RODATA` (unnumbered working label; the eighth v1.3.0 increment,
after `NAT-CALL`, `NAT-IF`, `NAT-QUAL`, `NAT-LOOP`, `NAT-IDX`, `NAT-FRAME` and
`NAT-STR-STAGE`).
**Branch:** `feature/v1.2.0-native-independence`.
**Status:** implemented and gated.

---

## 1. The premise, checked rather than inherited

The NAT-FRAME record left a residual of **TWO bytes** in an indexing
program's image and attributed it to the ORDER in which the reporter's strings
are interned:

> the oracle emits the reporter **helper** before the user function, so its
> strings are interned **first**; the driver appends them to the rodata list
> **after** the program's own literals.

**Measured, that reading is wrong for the program the residual was measured
on.** The rodata layout of that program is **byte-identical** between the two
engines:

```
kcc rodata: 1042:"runtime error: " 1057:" at " 1062:"array index out of range"
go  rodata: 1042:"runtime error: " 1057:" at " 1062:"array index out of range"
```

and the two differing bytes are the array-bounds call site's two **absolute
addresses**, which point at **different bytes**:

| | address | rodata offset | what is there |
|---|---|---|---|
| oracle | `0x400426` | 22 | `array index out of range` |
| driver | `0x400416` | 6 | the `i` of `runtime error: ` |
| oracle | `0x40043e` | 46 | `main.kark` |
| driver | `0x400417` | 7 | the `m` of `runtime error: ` |

The address difference (16 and 39 bytes) was the difference between the two
WRONG numbers and the two RIGHT ones. The record read a symptom of a
reference-resolution defect as an offset difference.

### 1.1 The oracle's interning, exactly

`pkg/native/program.go:2136-2148`:

```go
func (b *Builder) internRodata(s string) uint64 {
	if off, ok := b.strs[s]; ok {
		return off
	}
	off := uint64(len(b.rodata))
	b.rodata = append(b.rodata, s...)
	b.strs[s] = off
	return off
}

func (b *Builder) rodataRef(r Reg, s string) {
	b.patches = append(b.patches, addrPatch{pos: b.e.imm64Patch(r), roOff: b.internRodata(s)})
}
```

* **What is interned:** every string the image refers to.
* **The deduplication key:** the string's **CONTENT** (`b.strs` is a
  `map[string]uint64`). A repeated string occupies ONE entry.
* **The insertion order:** FIRST USE, during EMISSION. The offset is assigned
  at the moment of the first reference, so the section's layout is a function
  of the emission order.
* **Whether the order is source-order, discovery-order, map iteration or
  another rule:** **discovery order**, and it is fully deterministic - the
  emission order is fixed (helpers, then the user functions in source order,
  then the entry), and there is no map iteration anywhere in it. The map is
  only a lookup; the section is a byte slice.
* **Whether offsets are assigned during discovery or at final emission:**
  **during discovery** - `off := uint64(len(b.rodata))` at the moment of the
  first use.

The oracle's helper order (`program.go:2475-2521`) is: `print_int`,
`print_str`, `[print_float]`, `[alloc]`, `[map]`, `runtime_error`. So the
reporter's four strings are interned **before** any of the user's literals,
because the reporter helper is emitted before the user function.

### 1.2 The driver's interning, and the defect

`src/compiler/native_value.kark`, before this slice:

```kark
    let strs = ["-", "\n"]           // print_int's two
    ... append the program's literals ...
    if (usesRtErr == 1) {
        strs = appendArray(strs, "runtime error: ")
        strs = appendArray(strs, " at ")
        strs = appendArray(strs, ":")
        strs = appendArray(strs, "\n")
        strs = appendArray(strs, "array index out of range")
        strs = appendArray(strs, sourceBase)
        rtKindSlot = 2 + len(lits) + 4      // <-- a LIST POSITION
        srcOff = 2 + len(lits) + 5          // <-- a LIST POSITION
        srcLen = len(sourceBase)
    }
```

Two violations of the D5 baseline's stated design
(`PHASE-D5-BASELINE.md` section 8.3: *"The reporter's strings must be
appended in the order the oracle emits them (helpers first, then the call
site), and their offsets read back OUT of `natRodataSection`'s offset
array"*):

1. **The ORDER.** The reporter's four strings were appended AFTER the
   program's own literals, so they were interned after them - the opposite of
   the oracle's, whose helper is emitted first.
2. **The OFFSETS.** The call site's two references were the LIST POSITIONS
   `2 + len(lits) + 4` and `+ 5`, used as RODATA OFFSETS. A position counts
   ENTRIES and an offset counts BYTES; they differ as soon as any earlier
   string is longer than one byte, which every one of them is.

The second one is a **correctness** defect: the out-of-range diagnostic
pointed at the wrong text.

### 1.3 The other half of the design: the emitter's cursor

The driver handed the EMITTER the whole rodata offset array and started its
cursor at 2, because entries 0 and 1 were print_int's. That made the
emitter's cursor a function of the INTERNING ORDER: moving the reporter's
strings moved every literal's offset and with it the cursor.

---

## 2. What changed

`src/compiler/native_value.kark`, in `natNativeProgramOS`:

1. **The interning list is built in the ORACLE'S ORDER**: print_int's two, the
   reporter HELPER's four, the program's literals, and the call site's two.
2. **Every reporter-related offset is read back OUT of `natRodataSection`'s
   offset array** - the helper's four (as before) and now the call site's two
   as well. One mechanism, no arithmetic.
3. **The EMITTER gets the LITERALS' offsets and nothing else.** The literal
   offsets are collected into their own list, so the emitter's cursor starts
   at 0 and no longer depends on how many non-literal entries precede the
   literals. The linker owns the section's interning order; the emitter wants
   "the k-th literal's offset"; the two are separate facts now.

`natNativeLower`'s cursor starts at 0, and `natRodataOffsetFor`'s lookup drops
its `+ 2` (the same change, in the one other place that indexed the array).

Nothing else moved: the frame layout, the string staging area, the parser,
the arena bound and the reporter's code are untouched.

---

## 3. Differential evidence

Gate: `pkg/cli/phase151m_rodata_test.go` (`TestPhaseNATRodata_`), CI step
*Run NAT-RODATA native reporter rodata gate*.

### 3.1 The corpus

Nine programs: no reporter rodata at all; a program with a string and no
reporter; an array index with no literals; **two** index sites (the
reporter's strings are interned ONCE); an array index with a literal; two
distinct literals; the same literal bound twice; several functions; and a
literal bound AFTER the index site (the different discovery order).

A tenth case - a string in a CALLEE - is a refusal: the driver refuses it by
name, so the question of its interning order never arises there.

### 3.2 The recorded fields

Per fixture: the reporter's rodata entries, the deduplication count, the
insertion order, the assigned offsets, the rodata size, the section
boundaries, the total image size and the byte-identity result.

### 3.3 The measured result

Linux ELF, driver against oracle, after the correction:

| fixture | entries | deduped | insertion order | rodata size | image size | byte-identical |
|---|---|---|---|---|---|---|
| ints only, no reporter | 2 | 0 | `-`, `\n` | 2 | 351 | **yes** |
| string, no reporter | 3 | 0 | `-`, `\n`, `hello` | 7 | 376 | **yes** |
| array index, no literals | 7 | 0 | `-`, `\n`, `runtime error: `, ` at `, `:`, `array index out of range`, `probe.kark` | 56 | 1096 | **yes** |
| two index sites | 7 | 0 | as above | 56 | 1199 | **yes** |
| array index + a literal | 8 | 0 | `-`, `\n`, `runtime error: `, ` at `, `:`, `hello`, `array index out of range`, `probe.kark` | 58 | 1143 | **yes** |
| two distinct literals | 9 | 0 | ..., `alpha`, `beta`, `array index out of range`, `probe.kark` | 63 | 1184 | **yes** |
| the same literal twice | 8 | **1** | ..., `hello`, `array index out of range`, `probe.kark` | 58 | 1155 | **yes** |
| several functions | 7 | 0 | as the third row | 56 | 1083 | **yes** |
| a literal after the index site | 8 | 0 | ..., `hello`, `array index out of range`, `probe.kark` | 58 | 1140 | **no** (39 bytes) |

The same measurement BEFORE the correction:

| fixture | differing bytes | cause |
|---|---|---|
| array index, no literals | **2** | the call site's two wrong addresses |
| two index sites | **4** | two call sites, two wrong addresses each |
| array index + a literal | **31** | the two, plus the order |
| two distinct literals | **34** | the two, plus the order |
| a literal after the index site | **62** | the two, plus the order |
| no reporter | 0 | - |

### 3.4 The correctness claim, measured directly

The image is asked whether it references the address of the message and of the
file name. Before the correction it referenced neither - it referenced offsets
6 and 7, the middle of `runtime error: `. After it, it references both, and
the diagnostic reads `runtime error: array index out of range at main.kark:N`.

### 3.5 The PE is not a byte-identity target

The Windows entry stub and the loader-independent PEB bootstrap are SEPARATE
IMPLEMENTATIONS in the two engines - the driver's `natWinStub` against the
oracle's `emitWinBootstrap` - so the PE's `.text` is not the same bytes:
measured, the driver's `.text` virtual size is `0xa9c` and the oracle's
`0xa94`, eight bytes apart, and about a thousand bytes of the bootstrap
differ. That is why the byte-identity differential (Phase D2) compares the
LINUX ELF, where both engines share one entry stub, and why the PE is checked
here for what it can show: it is a valid PE, its strings are interned once
each, and the program RUNS.

---

## 4. What remains, isolated

### 4.1 The oracle interns the call site's strings at the first index site

The oracle's interning is by first use DURING emission, so the call site's
two strings are interned exactly where the first index expression is
lowered. The driver collects the literals up front and groups the call site's
two after them.

The two agree whenever the program's literals are all bound before its first
index site, which is every fixture but one. When a literal is bound AFTER an
index site, the oracle puts the call site's two before it and the driver
puts them after:

```
driver: - \n runtime error:  at : hello array index out of range probe.kark
oracle: - \n runtime error:  at : array index out of range probe.kark hello
```

39 bytes differ. It is a **LAYOUT** difference with **no correctness
consequence** - the diagnostic's text is identical in both engines, because
both point at the right bytes.

Closing it would mean re-deriving the interning order from the emitter's
walk (the split point is "how many literals the emitter had consumed when it
reached the first index site"), or interning at emission time as the oracle
does - which would put the read-only buffer and the intern map in the
emitter state. Both are a larger blast radius than this slice's area, and
neither is needed for correctness. It is pinned as a known difference by
`TestPhaseNATRodata_TheImagesAgree` and
`TestPhaseNATRodata_TheInterningOrderIsTheOracles`, which both FAIL if it
closes.

### 4.2 The known differences this slice does not touch

* the **25-byte field-assignment staging code** difference. It DOES affect
  the reporter rodata: the oracle's field write stages the pair through the
  area, so its `.text` is 25 bytes longer, its read-only section starts 25
  bytes later and every rodata address in that image differs. It is a
  difference in the CODE, not in the interning, and it is unchanged.
* the oracle's **name-table over-reservation**, the unsupported string
  equality and slicing, the absence of a register allocator, the arena-bound
  limitations and the parser limitations are all unchanged. None of them
  affects the reporter rodata: the name table changes the FRAME and the
  arena, the equality and the slicing are refused before any image exists,
  and the allocator and the arena bound are about code size and layout.

---

## 5. The gate

`pkg/cli/phase151m_rodata_test.go`, prefix `TestPhaseNATRodata_`:

| test | what it holds |
|------|---------------|
| `TheImagesAgree` | the byte-identity result per fixture, on the Linux ELF, with the one known difference asserted to DIFFER |
| `TheInterningOrderIsTheOracles` | the section's entries read out of each image, in order, with their offsets: the driver's order is the D5 rule's, the oracle's is the oracle's, and the two agree except where the discovery order interleaves |
| `TheCallSitePointsAtTheRightBytes` | the image references the address of the message and of the file name - the correctness claim, measured from the image rather than compared with the oracle |
| `TheDifferentialRecord` | the recorded fields, and the relations that must hold among them (the deduplication count, the section's size) |
| `TheOutputIsDeterministic` | two builds of the same program, byte-identical |
| `TheWindowsPERegression` | the PE is valid, its strings are interned once each, and the program RUNS |
| `TheRefusalsAreNamed` | a string in a callee is refused by name, with no image |
| `GateIsWiredInCI` | this gate has a `ci.yml` step |

Two existing gates moved with the correction, and both moved by FAILING first,
which is the signal each was built to give:

* `pkg/cli/phase151d_parity_test.go`: `array_index` was asserted to DIFFER by
  `TestPhaseD2_KnownNonParityIsStillNonParity` and failed when it became
  identical. It moved into the parity list, and the known-non-parity list is
  now EMPTY - so that gate was REMOVED rather than left with nothing to
  compare, and `TestPhaseD2_TheKnownNonParityListIsEmpty` records that every
  shape which was ever known to differ is still asserted identical.
* `pkg/cli/phase151k_frame_test.go`:
  `TestPhaseNATFrame_TheImagesAgreeOnEverythingButTheReporterStrings`
  asserted that the images differ in exactly TWO bytes and failed when they
  stopped differing. It is now `TestPhaseNATFrame_TheImagesAgreeEntirely`,
  which asserts that they agree in EVERY byte.

---

## 6. Mutation verification

Two focused mutations, each caught by a meaningful assertion and restored
byte-identically.

### 6.1 M1 - the call site's offsets become list positions again

`rtKindSlot` and `srcOff` were set back to `2 + len(lits) + 4` and `+ 5` -
the list positions - instead of being read out of the offset array.

**Caught by:** `TestPhaseNATRodata_TheCallSitePointsAtTheRightBytes` (the
image no longer references the message or the file name), and
`TestPhaseNATRodata_TheImagesAgree` (the array-index fixtures differ again,
by the same 2 and 4 bytes as before the correction).

### 6.2 M2 - the interning order is put back

The reporter helper's four strings were moved back AFTER the program's own
literals - the order the driver used before this slice.

**Caught by:** `TestPhaseNATRodata_TheInterningOrderIsTheOracles` (the
driver's order is no longer the D5 rule's) and
`TestPhaseNATRodata_TheImagesAgree` (every fixture with a literal differs
again).

Both mutations were reverted and the file verified SHA-256 identical to its
pre-mutation content.

---

## 7. What this slice does NOT do

- It does not close the oracle's "intern at the first index site" ordering
  (section 4.1). It is a layout difference with no correctness consequence,
  and closing it means changing the emitter's walk or the emitter state.
- It does not change the frame layout, the string staging area, the parser,
  the arena bound or the reporter's code.
- It does not make the PE byte-identical, because the Windows entry stub and
  the PEB bootstrap are separate implementations in the two engines.
- It does not touch the field-assignment staging difference, the oracle's
  name-table over-reservation, the string equality and slicing refusals, the
  register allocator, the arena bound or the parser limitations.

---

## 8. Record

### 8.1 Files

- `src/compiler/native_value.kark` - the interning list's order, the offsets
  read back out of the offset array, and the emitter's literal-only offset
  list.
- `pkg/cli/phase151m_rodata_test.go` - the gate.
- `pkg/cli/phase151d_parity_test.go` - the `array_index` shape moved into the
  parity list; the known-non-parity list is empty and its gate was removed.
- `pkg/cli/phase151k_frame_test.go` - the residual test inverted.
- `.github/workflows/ci.yml` - the gate's step.
- `docs/inventory/native-feature-matrix.json` - the rodata row.

### 8.2 Measured results

**The KIR pin moved 14938 -> 14946 (+8)**, measured with
`karkain kir --verify src/compiler/kir.kark` (`text=14946 verify=14946`) and
never predicted. The delta is small because most of this slice's source growth
is COMMENTS, which the KIR text does not render. The measurement took about 23
minutes on this host.

**Regressions**, each run ISOLATED (the ~4 GB host rule), with the actual
times on this host:

| gate | result | time |
|------|--------|------|
| `TestPhaseNATRodata_` (this slice's gate) | PASS | 15.9 s |
| `TestPhaseNATRodata_` + `TestPhaseD2_` + `TestPhaseNATFrame_` (after the mutations were restored) | ok | 87.0 s |
| the native family (RODATA, STR-STAGE, FRAME, IF, LOOP, IDX, QUAL, CALL, D2, D5, D8, 9C, 9D, 9E, 9F, 9G, 9H, B, KCCRun, P1, PushMain, 152-A) | ok | 518.2 s |
| `pkg/native` (the whole package) | ok | 57.6 s |
| `TestPhase151A5_KIRPinHolds` (the KIR pin) | PASS | 898.7 s, `text=14946 verify=14946` |
| `TestPhase122_PipelineOwnership` (the other KIR pin + KIRContinuity) | PASS | 2147.3 s (self-check 1162.6 s, KIRContinuity 934.4 s) |
| `go build ./...`, `go vet ./pkg/cli/` | clean | - |

See section 3 for the differential and section 6 for the mutations.

### 8.3 Next

The remaining native-path work is unchanged: the string equality and slice
lowerings, the field-assignment staging code, the reporter's rodata interning
position, the register allocator and the macOS execution evidence.
