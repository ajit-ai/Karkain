# NAT-STR-STAGE baseline - the native string staging area: scope and coverage

**Slice:** `NAT-STR-STAGE` (unnumbered working label; the seventh v1.3.0
increment, after `NAT-CALL`, `NAT-IF`, `NAT-QUAL`, `NAT-LOOP`, `NAT-IDX` and
`NAT-FRAME`).
**Branch:** `feature/v1.2.0-native-independence`.
**Status:** implemented and gated.

---

## 1. The premise, traced through both engines

The recorded discrepancy was a **frame-size difference of exactly 2560 bytes**
between the two engines, with two causes that had never been separated: a
**SCOPE** difference and a **COVERAGE** difference.

### 1.1 What the oracle reserves, and where

`pkg/native/program.go:4454`, inside the per-function `layout`:

```go
	if b.usesConcat || b.usesStrEq || b.usesStrSlice || b.usesPush || b.usesStrField {
		b.strTemp = next
		next += 8*5*maxBinDepth
	}
```

Three facts about that line decide everything:

1. **The five flags are properties of the PROGRAM, not of a function.**
   `scanValueUsage` (`pkg/native/program.go:873`) and `scanStructUsage`
   (`pkg/native/program.go:615`) both walk `prog.Statements`, which holds the
   top-level `FuncDecl`s. `b.usesConcat` is set from `scanConcatSites(prog)`,
   `b.usesStrEq`/`b.usesStrSlice` from `scanStringViewOps(prog, strNames)`,
   `b.usesPush` from `scanPushSites(prog)` and `b.usesStrField` from
   `scanStructUsage(prog)`.

2. **The reservation lands in EVERY function's layout.** `layout` is called
   once per function (`b.emitFunc` -> `layout`), and the reservation is
   unconditional on which function is being laid out. A callee in a program
   whose `main` concatenates therefore reserves the area too, even though the
   callee never touches a string.

3. **The area is 2560 bytes**: `8 * 5 * maxBinDepth`, with `maxBinDepth` = 64.

### 1.2 What the driver reserved, and where

`src/compiler/native_value.kark`, in `natNativeEmitFunc` (the per-function
emitter):

```kark
    let usesStr = 0
    if (natNativeConcatSites(body, plan) > 0) { usesStr = 1 }
    if (natNativePushCount(body) > 0) { usesStr = 1 }
```

and a **dead copy** of the same computation at the program level, which
nothing read.

So the driver's flag was:

- **per function**, where the oracle's is per program; and
- **two operations** wide (a concat site, a push site), where the oracle's is
  five.

### 1.3 The two divergences, measured

Measured on the **Linux ELF** target. The target matters: the Windows frame is
rounded up to a multiple of 16 (`if next % 16 != 0 { next += 8 }`), which
quantises an 8-byte difference away and cannot see a 2560-byte one at all.

The frame-size immediates are read out of each generated image (the long form
`48 81 EC`), one per user function, in source order:

| program | driver frames | oracle frames | difference |
|---------|---------------|---------------|------------|
| `helper() int` + `main` concatenating | `[608, 3168]` | `[3168, 3168]` | 2560 in the helper |
| `helper() int` + `main` pushing | `[608, 3208]` | `[3168, 3208]` | 2560 in the helper |
| `s.label = "changed"` | `[640]` | `[3200]` | 2560 in main |

Every difference is the same 2560 bytes, and each one is the driver's own
scope or coverage, not the oracle's.

---

## 2. What changed

`src/compiler/native_value.kark`:

1. **`natNativeProgramUsesStr(fns, tbl)`** - the PROGRAM-WIDE flag, computed
   once. It walks every top-level function, plans each one, and asks
   `natNativeStrOpIn` whether it performs a string operation.

2. **`natNativeStrOpIn(stmts, plan)`** - the five-operation predicate, in the
   oracle's own order: a concatenation, a string equality, a string slice, a
   `push()`, a string field assignment.

   The concatenation and the `push()` arms **reuse** the detectors the arena
   bound already uses (`natNativeConcatSites`, `natNativePushCount`), so there
   is one notion of "where a concat site is" rather than two collectors that
   could disagree - the Step 9g defect class.

   The equality, slice and field-assignment arms are new detectors. Each is
   **unreachable from the driver's accepted subset today** (a string equality,
   a string slice and a string field assignment from anything but a literal are
   all refused by name), and they are implemented anyway because the
   reservation is a layout contract with the oracle: when one of them is
   lowered, the area has to be reserved already or the frame would move at
   that point and every image with it.

3. **`natNativeEmitFunc`** takes the program-wide flag as a parameter and uses
   it in place of the per-function computation. This is the same shape as the
   map-stage flag that NAT-FRAME established.

4. The **dead** program-level copy of the per-function computation is deleted.

**What was NOT done.** The driver does not duplicate the oracle's staging
implementation. The oracle's string-field write stages the `(pointer, length)`
pair through the area (`pkg/native/program.go:6236-6250`); the driver's writes
the pair directly into the field area. The **frame** matches the oracle's
exactly; the **code** does not, and the field-assignment image is 25 bytes
shorter on the driver (measured: 443 against 468).

---

## 3. Differential evidence

Gate: `pkg/cli/phase151l_strstage_test.go` (`TestPhaseNATStrStage_`), CI step
*Run NAT-STR-STAGE native string-staging-area gate*.

### 3.1 The corpus

Eleven programs, one per shape the brief names: one function with one string,
one function with several strings, several functions with independent strings,
several functions with the same string, nested calls with strings, strings
alongside arrays and integer expressions, no strings at all, a mixed program
(a push, a concatenation and an array index), a string field assignment, a
`push()` on its own, and one whose `main` is **not** the last function (which
is what catches an aggregation that keeps only the last function's answer).

### 3.2 The recorded fields

Per case, the differential records: the total string-stage allocation, the
allocation's **scope** (which functions reserve it), the **string offsets**
(the file offsets of the program's distinctive literal in each image), the
**function count**, the **image size**, the **section boundaries** (the
program-header count and the `.text` offset, read with the oracle's own
`native.Parse`), the **frame-size immediates**, and the **byte-identity
result** (the number of differing bytes).

### 3.3 What the frames are now

Measured on the Linux ELF target, driver against oracle:

| case | frames (driver) | frames (oracle) | equal |
|------|-----------------|-----------------|-------|
| one string, no operation | `[624]` | `[624]` | yes |
| one function, several strings, concat | `[3200]` | `[3200]` | yes |
| several functions, independent strings | `[608, 624]` | `[608, 624]` | yes |
| several functions, same string, concat | `[3168, 3200]` | `[3168, 3200]` | yes |
| nested calls with a concat | `[3176, 3176, 3184]` | `[3176, 3176, 3184]` | yes |
| strings with an array and integer expressions | `[3232]` | `[3232]` | yes |
| no strings | `[608]` | `[608]` | yes |
| mixed push, concat and array | `[3232]` | `[3232]` | yes |
| string field assignment | `[3200]` | `[3200]` | yes |
| `push()` only | `[3216]` | `[3216]` | yes |
| `main` is not the last function | `[3184, 3168]` | `[3184, 3168]` | yes |

### 3.4 The scope, as a difference rather than an equality

The same helper appears in two programs that differ only by `main` doing a
concatenation. Because the oracle's flag is program-wide, the **helper's**
frame grows by exactly the staging area, even though the helper never touches
a string:

```
with a concat:    [3168, 3184]
without a concat: [ 608,  624]
difference:       [2560, 2560]
```

A per-function flag would leave the helper's frame unchanged. That is what the
assertion rules out.

### 3.5 The coverage, measured on both engines

For each of the oracle's five operations, the oracle's frame for the operation
is compared with the oracle's frame for a twin that performs no string
operation but has the **same bindings** - so the difference is exactly the
staging area and nothing else. The driver's frame is measured the same way
where the driver lowers the operation, and the driver is required to REFUSE
(by name, with no image) the two operations it does not lower.

---

## 4. What remains, isolated

### 4.1 The oracle's program-wide NAME table

**This is the one difference that remains, and it is the oracle's
over-approximation rather than the driver's.**

The oracle's string inference is **name-based** and **program-wide**:
`scanConcatSites` builds a table of names bound to a string and repeats its
walk until the table stops changing (`pkg/native/program.go:1180-1182`). A
name bound to a string **anywhere** in the program is therefore a string
**everywhere**, shadowing included.

So in this program:

```
func h(a int) int { return a + a }
func main() { let a = "x" print(a) print(h(3)) }
```

the oracle classifies the helper's `a + a` as a string **concatenation site**,
reserves the staging area in both functions, and allocates an arena. Its own
**emission** disagrees with its scan: the same program builds, runs, and
prints `x` and `6` - an integer addition (measured on the oracle's own PE).

The driver does not reproduce that, and the reason is not convenience. The
oracle's scan over-approximates its own emission; copying the
over-approximation would mean allocating an arena for a program that allocates
nothing, and classifying an integer addition as a string concatenation. The
driver follows the emission.

Measured: the oracle reserves 2560 bytes in both frames of that program; the
driver reserves none. Pinned by
`TestPhaseNATStrStage_TheOracleNameQuirkIsNotReproduced`.

### 4.2 The field write's staging code

The oracle's string-field write stages the pair through the area and the
driver's does not, so the field-assignment image is **25 bytes shorter** on the
driver (443 against 468) and its rodata starts at a different file offset
(436 against 461). The frame matches exactly. This is a deliberate
non-duplication of the oracle's implementation, not a defect.

### 4.3 The reporter's rodata interning order

Unchanged from NAT-FRAME: the two engines intern the runtime-error reporter's
strings in a different order, so two absolute rodata addresses inside the
reporter differ. That is why the differential records the byte-identity
result rather than claiming it.

---

## 5. The gate

`pkg/cli/phase151l_strstage_test.go`, prefix `TestPhaseNATStrStage_`:

| test | what it holds |
|------|---------------|
| `TheFramesMatchTheOracle` | the frame-size immediates of EVERY function, read out of each image, equal element for element, on the Linux target, for all ten cases |
| `TheStageIsReservedInEveryFunction` | the scope, as a difference of exactly 2560 in every frame between a program with a string operation and its twin without one |
| `TheCoverageIsTheOracleFiveOperations` | the five operations, measured on the oracle and on the driver, with the driver's two refusals required to name the construct and produce no image |
| `TheDifferentialRecord` | the recorded fields, and the relations that must hold among them |
| `TheProgramsStillRun` | the behavioural half: every case executed as a PE on this host, stdout compared **exactly** (never trimmed - the Step 9b defect was a missing byte) |
| `TheOutputIsDeterministic` | two builds of the same program, byte-identical |
| `TheOracleNameQuirkIsNotReproduced` | the remaining divergence, measured rather than hidden |
| `TheRefusalsAreNamed` | the constructs the driver declines, refused by name with no image |
| `GateIsWiredInCI` | this gate has a `ci.yml` step |

Two existing gates moved with the correction:

- `pkg/cli/phase151k_frame_test.go`: the string-assignment case joined
  `natFrameCases`, so the parity assertion covers it directly, and
  `TestPhaseNATFrame_TheStringStageIsStillDivergent` became
  `TestPhaseNATFrame_TheStringStageIsReservedProgramWide` - it asserts the
  CLOSED state on both axes rather than recording a divergence that no longer
  exists.
- `pkg/cli/phase151a9d_array_test.go`: the shared frame rule
  (`nat9dExpectFrame`) gained a string-stage term, in the oracle's layout
  position (between the bin temporary area and the map-stage unit), and the
  9H record gate's `string_field_assign` case now expects the staging area.

---

## 6. Mutation verification

Two focused mutations, each caught by a meaningful assertion and restored
byte-identically.

### 6.1 M1 - restore per-function allocation

The per-function computation was put back in `natNativeEmitFunc`:

```kark
    let usesStr = 0
    if (natNativeConcatSites(body, plan) > 0) { usesStr = 1 }
    if (natNativePushCount(body) > 0) { usesStr = 1 }
```

**Caught by:** `TestPhaseNATStrStage_TheStageIsReservedInEveryFunction` (the
helper's frame stops growing by 2560), and
`TestPhaseNATStrStage_TheFramesMatchTheOracle/several_functions_same_string`
and `/nested_calls_with_strings` (the callee's frame is 2560 bytes short
against the oracle's). The NAT-FRAME gate's
`TheStringStageIsReservedProgramWide` fails on the same axis.

### 6.2 M2 - corrupt the program-wide calculation

The aggregation was corrupted so that it keeps only the LAST function's answer
instead of "any function's answer":

```kark
    any = natNativeStrOpIn(fbody, fplan)      // overwrites
```

instead of

```kark
    if (natNativeStrOpIn(fbody, fplan) == 1) { any = 1 }
```

**Caught by:** the corpus case whose string operation is in a function that is
**not** the last one - `main_is_not_the_last_function` - where the corrupted
aggregation reports "no string operation" and the frames lose the staging
area. Measured: the driver's frames `[624, 608]` against the oracle's
`[3184, 3168]`. The scope and coverage cases, whose string work is in the
last function, stay green, which is what makes the mutation a SCOPED one
rather than a blanket failure.

Both mutations were reverted and the file verified SHA-256 identical to its
pre-mutation content.

---

## 7. What this slice does NOT do

- It does not change the driver's string-field WRITE. The oracle stages the
  pair through the area and the driver does not; the frame matches, the code
  does not, and the oracle's implementation is not duplicated.
- It does not lower a string equality or a string slice. Both are still
  refused by name; the arms exist so the reservation is complete when they
  are.
- It does not change the reporter's rodata interning order.
- It does not touch register allocation, the arena bound, the reporter, the
  container writers or the parsers.
- It does not close the oracle's name-based over-approximation; it records it.

---

## 8. Record

### 8.1 Files

- `src/compiler/native_value.kark` - `natNativeProgramUsesStr`,
  `natNativeStrOpIn`, `natNativeStrEqSites`/`In`,
  `natNativeStrSliceSites`/`In`, `natNativeStrFieldSites`,
  `natNativeStrLitIn`; `natNativeEmitFunc` takes the program-wide flag; the
  dead program-level copy is deleted.
- `pkg/cli/phase151l_strstage_test.go` - the gate.
- `pkg/cli/phase151k_frame_test.go` - the string-assignment case joins
  `natFrameCases`; the divergence test becomes an agreement test; the
  per-function frame reader `natFrameAllFrames` is added.
- `pkg/cli/phase151a9d_array_test.go`, `pkg/cli/phase151a9e_len_test.go`,
  `pkg/cli/phase151a9f_string_test.go`, `pkg/cli/phase151a9h_records_test.go` -
  the shared frame rule gains a string-stage term.
- `.github/workflows/ci.yml` - the gate's step.
- `docs/inventory/native-feature-matrix.json` - the string-stage row becomes
  `supported`; the limitation is restated as the oracle's name table and the
  field write's staging code.
- `pkg/cli/phase122_pipeline_ownership_test.go`, `pkg/cli/phase151a5_win_test.go`
  - the whole-tree KIR pin re-pinned.

### 8.2 Measured results

**The KIR pin moved 14841 -> 14938 (+97)**, measured with
`karkain kir --verify src/compiler/kir.kark` (`text=14938 verify=14938`) and
never predicted. The measurement took about 25 minutes on this host.

**Regressions**, each run ISOLATED (the ~4 GB host rule), with the actual
times on this host:

| gate | result | time |
|------|--------|------|
| `TestPhaseNATStrStage_` (this slice's gate) | PASS | 100.7 s |
| `TestPhaseNATFrame_` (the inverted divergence test + the new case) | PASS | in the family run |
| `TestPhase151A9D_` / `9E_` / `9F_` / `9H_` (the shared frame rule) | PASS | in the family run |
| `TestPhaseNATQual_` (the matrix row) | PASS | 31.4 s |
| `TestPhaseNATStrStage_` + `TestPhaseNATFrame_` + `TestPhaseNATIf_` + `TestPhaseNATLoop_` + `TestPhaseNATIdx_` + `TestPhaseNATQual_` + `TestPhaseNATCall_` | ok | 247.4 s |
| `TestPhase122_PipelineOwnership` (the KIR pin + KIRContinuity) | PASS | 2513.8 s (self-check 1297.0 s, KIRContinuity 1172.8 s) |
| `TestPhase151A5_KIRPinHolds` (the second KIR pin) | PASS | 1629.2 s, `text=14938 verify=14938` |
| `pkg/native` (the whole package) | ok | 54.5 s |
| `go build ./...`, `go vet ./pkg/cli/` | clean | - |

The two KIR-pin measurements are the long poles on this host (about 25 and 27
minutes); everything else is minutes.

See section 3 for the frames and section 6 for the mutations.

### 8.3 Next

The remaining native-path work is unchanged: the string equality and slice
lowerings, the string-field write's staging (only if byte-identity with the
oracle's code is wanted rather than only its frame), the reporter's rodata
interning order, and the register allocator.
