# NAT-IF baseline — a conditional branch in the kcc native driver

**Slice:** `NAT-IF` (unnumbered working label; the second v1.3.0 increment after
`NAT-CALL`).
**Branch:** `feature/v1.2.0-native-independence`.
**Status:** implemented and gated.
**Parent:** `NAT-CALL` (`docs/audit/NAT-CALL-BASELINE.md`).

---

## 1. The gap this slice closes

Measured on the tree this slice started from (`main` = `d0b67f1`): the native
driver lowered **no branch at all**. A program with a conditional was refused:

```kark
func main() {
    let x = 5
    if x > 3 {
        print(1)
    }
}
```

```
error[K145]: native target: `if` statement is not lowered by the native driver in `func main`
exit 3, no image written
```

The Go oracle builds the same program and prints `1`. So the gap was one of
*native lowering*, not of the language, the parser or the checker.

This is the first construct whose emitted bytes depend on a **label** rather
than only on a frame slot, which is why it needed its own byte-identity
evidence rather than a "it runs" assertion.

---

## 2. What this slice adds

| Construct | Before | After |
|---|---|---|
| `if <int comparison> { ... }` | refused | lowered, executes |
| `if <int comparison> { ... } else { ... }` | refused | lowered, executes |
| `else if` | refused | lowered (the parser produces a one-element alternative list) |
| a nested `if` | refused | lowered |
| a binding inside a branch | refused (wrongly) | lowered, with the slot the oracle gives it |
| `while` / `for` / `for-in` / `break` / `continue` | refused | **still refused** |
| a string or float comparison | refused | **still refused** |
| a comparison used as a value | refused | **still refused** |

### 2.1 The condition is an integer comparison

Six operators: `==`, `!=`, `<`, `<=`, `>`, `>=`. The jump opcode per operator is
the oracle's (`==` jumps on `Jnz`, `!=` on `Jz`, `<` on `Jge`, `<=` on `Jg`,
`>` on `Jle`, `>=` on `Jl`) — the condition **falls through** when the
comparison holds, so each operator jumps to the false label on the inverse
test.

The oracle has a string path and a float path in `emitCond` as well. Neither is
ported: the self-hosted driver's string layer is main-only, and it has no float
statement lowering yet. Both are refused by name.

### 2.2 The branch body is int-only, and one binding is not allowed

A branch may bind an **integer**, print an integer expression, assign a record
field, make a bare call, return an integer expression, or nest another `if`.

It may **not** bind a string. The reason is a property of this driver: the
rodata cursor is threaded per *statement list*, so a literal inside a branch
would need a cursor the recursion does not carry, and interning a literal
whose offset it could not keep would shift every later offset. An integer
binding needs no rodata entry, so it is allowed.

---

## 3. What the slice deliberately did NOT do

* **`while`, C-style `for`, `for-in`, `break`, `continue`.** They are the next
  increment. A loop also needs a bound the arena does not yet have, and the
  Phase-150C register allocator engages under control flow — see §6.
* **The Phase-150C register allocator in the self-hosted driver.** The Go
  oracle has one; the self-hosted driver keeps every local in its frame slot,
  which is the pre-150C path and is always correct. §6 pins the boundary.
* **A string or float comparison**, and a comparison used as a value.
* **A branch-scoped STRING binding** (§2.2).

---

## 4. The two facts that were wrong, and how they were caught

### 4.1 "A binding inside a branch has no frame slot" — WRONG

The first draft **refused** a `let` inside a branch, on the belief that the
layout pass walked only a function's top-level statements, so a branch-scoped
binding would resolve to frame offset 0 and overwrite the first local.

That belief was wrong. The oracle's `scanLets` **descends into a branch**
(`pkg/native/program.go:4674-4676`):

```go
case *parser.IfStmt:
    next = b.scanLets(n.Consequence, next)
    next = b.scanLets(n.Alternative, next)
```

The gate that caught it was the byte-identity differential: the oracle produced
an image for such a program and the self-hosted driver refused it. The
refusal was **withdrawn**, `natNativePlanDecls` was made to descend exactly as
the oracle does — allocating the branch's slot **at the point the `if` is
reached**, so the branch's binding is the MIDDLE slot of three — and the
agreement is now asserted from both directions by
`TestPhaseNATIf_TheBranchScopedBindingIsAgreedOnByBothEngines`.

A corrective change to the Go oracle was written first, on the same wrong
belief, and **reverted**: the oracle was right all along.

### 4.2 "A `for` post is not an assignment" — the row that moved

The NAT-CALL refusal table pinned "an `if` is not lowered" in two places. Both
rows became false. They were **replaced**, not deleted, because a refusal
table that quietly shrinks stops being evidence for the property it exists to
prove. The replacements are the next control-flow construct (`for-in`), which
is still outside the subset.

The same move was needed in `phase151d_elfnative_test.go` (whose `unsupported_if`
row became `unsupported_for_in`). Both moves are recorded in
`docs/audit/NAT-CALL-BASELINE.md` §8.6.

---

## 5. The gate

`pkg/cli/phase151g_if_test.go`, tests named `TestPhaseNATIf_*`, with its own
`ci.yml` step (*Run NAT-IF kcc native conditional-branch gate*). The filename
avoids the `_linux` / `_wasm` suffixes, which Go treats as build constraints
(the trap A1 hit, where a gate was silently not compiled).

Seven layers, each answering a question the others cannot:

1. **Executed PE goldens.** 19 programs, each compiled through the kcc native
   driver, written to disk and EXECUTED on this host. stdout is compared as
   **exact bytes** — never trimmed, because the Step 9b defect was a *missing
   byte* and trimming is exactly the tool that hides it. The exit code is
   asserted separately, because `return_in_branch` is observable only through
   it and `if_not_taken` only through the absence of output.
2. **A Linux ELF byte-identity DIFFERENTIAL** against the Go oracle, with the
   expected bytes produced by calling the oracle at test time. A golden would
   be wrong: for most of increment 151 the bytes were wrong, so a golden pins
   the divergence and calls it expected. The kcc side must also carry the
   self-hosted provenance marker, so a quiet fall-back to the Go backend would
   agree byte for byte and still fail here.
3. **A refusal table** — 6 rows, each asserting the exit code, that the
   message NAMES the construct, and that **no image was written**. A refusal
   that still emits is not a refusal.
4. **The branch-scoped binding, agreed on by both engines** (§4.1).
5. **The register-allocator boundary**, asserted from both sides (§6).
6. **Determinism and non-vacuity.** The same source twice → identical bytes;
   two programs whose conditions differ → different images (a driver that
   ignored the condition would pass the execution layer for any case whose
   expected output happened to match).
7. **The CI-wiring check** — a gate is a test file *plus* a `ci.yml` entry.

---

## 6. The register-allocator boundary, measured

The Go oracle's Phase-150C allocator engages only when a function contains
control flow **and** has more than `raMinLive` (= `allocRegCount + 1` = 5)
simultaneously-live int locals. The self-hosted driver has no allocator: every
local stays in its frame slot, which is the pre-150C path and is always
correct. So from five locals upward the two engines encode the same program
differently — an **encoding strategy** difference, not a wrong answer.

Measured on this host, not derived:

| locals + a branch | kcc | oracle | identical |
|---|---|---|---|
| 4 | 454 bytes | 454 bytes | **yes** |
| 5 | 469 bytes | 471 bytes | no |
| 6 | 484 bytes | 486 bytes | no |

`TestPhaseNATIf_TheRegisterAllocatorBoundaryIsPinned` asserts **both sides**:
four locals must stay identical (so a change that breaks byte identity for
small programs is caught here), and five must differ (so the gap cannot
quietly become an unknown one). The five-local case is in the execution corpus
as well, so "different bytes" is backed by "same behaviour".

---

## 7. Mutation verification

Four mutations, each restored **byte-identically** (the file's SHA-256 after
every restore: `145489AEB6AEDA59FFC4C8DCF583C6051A033AAE60248F8D65E1955190286F09`),
with the self-hosted source's mtime bumped so `kccStale` cannot reuse a mutated
binary.

| # | Mutation | Observed |
|---|---|---|
| M1 | the `<` jump changed from `Jge` to `Jle` | the execution layer fails on 4 cases, the byte-identity layer fails, and the allocator-boundary layer fails |
| M2 | the plan's descent into a branch removed | the execution layer fails, the byte-identity layer fails, and the branch-scoped-binding agreement fails |
| M3 | the comparison's left and right operands swapped | the execution layer fails on 13 cases — a comparison that compiles and computes the WRONG answer, which is the worst outcome for a native backend |
| M4 | the jump that crosses the `else` block removed | the execution layer fails on 4 cases (the consequence falls through into the alternative) and the byte-identity layer fails on 6 |

M3 is the instructive one: it produces a **valid, executing, wrong** program,
which is exactly the class of defect an execution-only gate cannot see and a
byte-identity gate can.

---

## 8. Record

### 8.1 Measured results

* **Gate:** `TestPhaseNATIf_*` — 7 layers, 19 corpus programs, 6 refusal rows.
  Green: `ok karkain/pkg/cli 29-60 s`.
* **Every corpus program executes** on this Windows host as a kcc-produced PE
  with exact stdout bytes.
* **Every corpus program is byte-identical** to the Go oracle's Linux ELF,
  except the one case marked `expectDiff` (the five-local one, §6), whose
  difference is **asserted** rather than skipped.
* **KIR pin:** 14385 (NAT-CALL) → 14477 (the `push()` slice, which moved the
  pin and did not re-pin it, leaving the tree red) → **14597** (this slice,
  +120). Measured with `karkain kir --verify src/compiler/kir.kark` in the
  gate's own invocation — `text=14597 verify=14597` — and never predicted.
* **Frozen gates re-run, all green:** `TestPhase151A9C_`/`9D_`/`9E_`/`9F_`/
  `9G_`/`9H_`, `TestPhase151B9_`, `TestPhaseD2_`/`D5_`/`D8_`,
  `TestPhase151D_`, `TestPhaseNATCall_`, `TestPushMain_`, and the whole
  `pkg/native` suite. Zero byte drift in the frozen corpora.

### 8.2 Defects the gates found

* The two §4 facts. Both were found by the **byte-identity differential**, not
  by inspection and not by the execution layer — the execution layer was
  perfectly happy with the refusal, because a refusal produces no output and
  the corpus expected none.
* The gate found two defects in **my own matrix** (see
  `docs/audit/NAT-IF-QUALIFICATION.md`): a status with no gate behind it, and
  a gate name that did not exist. Both are recorded in the matrix's own gate.

### 8.3 A defect the ENVIRONMENT caused, and how it was told apart from a code
defect

`TestPhase151A5_KIRPinHolds` failed with **no output and exit 3**, four times
in a row. The gate's own diagnostic attributes that to the known intermittent
`0xC0000005`, and the retry loop clears stale `kcc.exe` processes.

The real cause was different: **`kcc.exe` links `libgmp-10.dll`, and the
directory that provides it (`C:\msys64\ucrt64\bin`, which also provides
`gcc`) was not on `PATH`.** Running the binary directly gave
`0xC0000135` (`STATUS_DLL_NOT_FOUND`), and the CLI reports a failing `kcc` as
exit 3 with no output — indistinguishable, from the outside, from the
intermittent crash the retry loop is built for.

The discriminator is one command: run the binary with the DLL directory on
`PATH` and it prints its banner. A missing DLL is a **toolchain** problem; the
intermittent is a **compiler** problem, and the two have different fixes.

This is worth recording because the gate's diagnostic MISATTRIBUTED it: the
retry loop cannot fix a missing DLL, so a host with the wrong `PATH` will burn
four attempts and then blame the compiler.

### 8.4 The slice's own boundary, stated

**No execution evidence exists for the Linux ELF or the Mach-O image of this
slice.** This host is `windows/amd64` with no WSL and no Docker, and the
repository has no macOS runner. The Windows PE images are executed; the Linux
and macOS images are validated structurally by the Go oracle's own parsers and
compared byte for byte against the oracle. Every Mach-O claim in this
repository has been structural since increment 149, and this slice does not
change that.

### 8.5 Corrective changes this slice made to frozen gates

| Gate | Row that moved | Replacement |
|---|---|---|
| `phase151e_call_test.go` | `if` in a called function | `for-in` in a called function |
| `phase151e_call_test.go` | `if` in `func main` | `for-in` in `func main` |
| `phase151d_elfnative_test.go` | `unsupported_if` | `unsupported_for_in` |

Each is a row that asserted a refusal the driver no longer makes. They were
replaced rather than deleted, and each replacement is a construct that is still
outside the subset.

### 8.6 What the next slice should be

`while` and the C-style `for`, in that order, with the loop-label discipline
the oracle's `loops[]` stack carries (`break` targets the loop's end label,
`continue` targets the head for a `while` and the post label for a `for`).
Two things that slice must decide rather than inherit:

* the **arena bound** for an allocation inside a loop, which is unbounded and
  therefore cannot be proved the way a straight-line program's is;
* the **register allocator**, which the oracle engages under control flow and
  pressure, and which the self-hosted driver does not have. Either the driver
  grows one or the byte-identity claim states its boundary explicitly, as §6
  does here.
