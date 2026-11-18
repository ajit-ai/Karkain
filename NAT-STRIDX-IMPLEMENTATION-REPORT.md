# NAT-STRIDX — Native String Byte Indexing, Implementation Report

**Increment:** NAT-STRIDX (Native string byte indexing)
**Branch:** `feature/v1.3.0-natstdlib-string`
**Starting HEAD:** `596a05c09758142791bff86c59c3c148ddaa3b6f` (`596a05c NAT-STRING-SLICE nil-start parity`)
**Ending HEAD:** recorded in the final Git status below (commit appended after this report)
**Status of acceptance criteria:** all pass, with the documented environmental limits recorded
under *Known limitations*. Nothing is marked complete that was not measured.

---

## 1. What was implemented and the exact semantics

`string_value[index]` is now lowered on the self-hosted (kcc) native path.

- **Indexing is BYTE-based, not code-point-based.** No Unicode conversion exists anywhere
  in the lowering. The bounds check compares the index against the string's byte length;
  the result is a **one-byte view** `(ptr + i, 1)` — zero copy, zero allocation, identical
  in shape to the `s[a:b]` slice view (`natEmitStrSlice`).
- **Result type:** a one-byte string. The literal `s[0]`, a variable binding
  (`let b = s[0]`), a return value (`return s[0]` in a callee), a print argument
  (`print(s[0])`) and a concatenation operand (`s[0] + s[1]`) all produce the established
  two-unit `(ptr, len)` string representation. `len(b)` on such a binding reports `1`.
- **Index type and validation:** the index is an *ordinary integer expression* — a literal,
  a variable, `len(s) - 1`, a call to a user function. It is validated by the shared integer
  expression walker (`natNativeScanIntExpr`), exactly as the array-index slice landed it.
- **Bounds:** `0 <= i < len(s)`. A negative index, an index equal to the length, a greater
  index, and any index into an empty string are all the same diagnostic: the Phase-100
  runtime-error reporter called with the kind
  `string index out of range`, `file:line`, on stderr, ending in `exit(1)` — the same
  contract the array path (`natArrayIndex`) and the C backend (`karkain_checked_get`,
  `pkg/codegen/codegen.go:2101`) establish. No silent wrong byte is ever produced.
- **Evaluation order:** the index expression is evaluated first, in the ARRAY path's order
  (`pkg/native/program.go:6320-6324`), then the base and the length are loaded.
- **Embedded NUL bytes are supported by the representation** (the pair carries its length;
  the view arithmetic never consults a terminator). The resident literal in this host even
  *contains* NULs as soon as a high byte is written, which is why the `héllo` case prints
  `00 0a` for the two positions where the two é bytes used to be (see *Known limitations*:
  bytes ≥ 0x80 do not survive the self-hosted rodata writer — a pre-existing platform
  limitation of `asciiVal`/`natAppendStr`, not of this slice).
- **Repeated indexing allocates nothing.** `print(s[1]); print(s[4]); print(s[1])` performs
  three register-pair constructions; no arena is reserved for a view, no copy is made.
- **A byte index is a string, not an integer.** It prints as itself, participates in
  concatenation, binds to locals and returns from callees; it is refused **BY NAME**
  everywhere an integer is required.

### 1.1 UTF-8 multibyte, concretely

`s[2]` of `"héllo"` prints NUL (`00 0a`) on this path, not `l`. That is *byte* indexing
proving itself: the é occupied two byte positions (1 and 2), not one. The Go engine prints
`c3 a9` (its literal pipeline is byte-faithful) and is **not** a valid reference for the
multibyte case; the divergence is recorded below, not papered over.

---

## 2. What this slice does NOT do (each refusal is by name, with no image)

| Shape | Refusal |
|---|---|
| `let n = s[0] + 1` (binding init is not an int expr) | `` `let n` initialised from a non-integer expression`` |
| `print(s[0] + 1)` | `` `+` needs two strings on the native target`` |
| `"abc"[0]` (literal base) | `string index target must be a variable` |
| `s[s[0]]`, `f()[0]`, `("a"+"b")[0]` (composed base) | `array index target must be a variable` (int path; the base is not a string variable) |
| `a[0]` scalar base | `index target must be an array (got int 'a')` (frozen NAT-IDX wording, unchanged) |
| view inside a string comparison | `an indexed operand in a string comparison is not lowered` (NAT-STREQ slice-1 boundary, unchanged) |
| view in a `let` (`let b = s[0]`) | **supported** — binds the one-byte view |
| `s[0] + "z"` concat view | **supported** — view through the arena |
| `return s[0]` in `main` | `return of a string from 'main' is not supported` (as with a literal) |
| slice of a view, `s[0][1:2]` | `string slicing of a string index is not lowered` (closes a pre-existing access-violation) |
| `return "a" + "a"` in a callee | `` `return` of a string concatenation is not lowered by the native driver`` (closes a measured access violation, `0xC0000005` in `natEmitStrValue`) |

Two **pre-existing defects found and fixed en route** (documented, not new scope):

1. **`return "a" + "a"` in a callee crashed the self-hosted compiler** with `0xC0000005`
   (`natNativeSlotOff <- natEmitStrValue`, measured). `natEmitStrValue` has no `BinaryExpr`
   arm; `natNativeValueKindOf` classifies a string concat as a string for the return
   dispatch, so the return path reached the crash. The scan had an arm for exactly this —
   but it was **dead code**, because a stray `return ""` preceded the BINARY block
   (the same mis-nesting class as the `natNativeScanStrSlice` brace slip). Fixed by moving
   that `return ""` below the BINARY block. Verified: the program now refuses with K145.
2. **`s[0][1:2]` crashed the scanner** the same way (`natNativeSlotOff` via `identName` on a
   non-identifier node). Refused by name.

---

## 3. Implementation

All work is in the self-hosted Karkain compiler; no Go production source, no C runtime, no
external dependency.

**`src/compiler/native_value.kark`** — the native driver. The pieces:

- `natNativeStrIndexSlot(plan, e)` — one predicate deciding "is this a *string* byte index":
  `e` must be a `NODE_INDEX` whose base is a plain variable whose slot kind is `natKindString()`.
  Used by the scan's kind split, by every emission dispatch, and by the arena accounting,
  so there is exactly one notion of the construct.
- `natNativeScanStrIndex(expr, line, fns, plan)` — scan: the base must be a string variable
  (literal base gets its own message); the index must be an int expression.
- `natExprHasStrIndex` / `natStmtHasStrIndex` / `natStmtHasStrIndexIn` /
  `natNativeUsesStringIndex` — the plan-based presence walkers (mirror of `natStmtHasIndex`,
  which is plan-free because an array index is defined structurally). Used **only** to decide
  whether the `"string index out of range"` kind string must be interned.
- `natEmitStrIndex(s, expr, plan, fns, binTemp, extrasBase, frame, rctx, line)` — the emitter.
  The index expression is lowered into RAX with the shared walker at depth+1, moved to RCX;
  base and length are loaded into RBX/RAX; bounds are checked exactly as `natArrayIndex`
  checks them (`test rcx,rcx; jns` / `cmp rcx,rax; jge fail`); the success path computes
  `(base + i, 1)` — `add rbx, rcx; mov rdi, rbx; mov rsi, 1`; the failure path calls the
  Phase-152-B0 reporter with the *string* kind string (`rctx[3]`), the basename
  (`rctx[1]`/`rctx[2]`) and the index's own line.
- `natEmitStrValue` gained the final `ictx` parameter (`[fns, binTemp, extrasBase, frame,
  rctx]`, built once in `natNativeLower`) and a `NODE_INDEX` arm routing to `natEmitStrIndex`.
  `natStrConcat` and `natEmitStrEqCond` thread `ictx` through to the operands, so a view can
  appear on either side of `+` (`s[0] + s[1]`, `s[0] + "z"`) and inside `print`.
- `natNativeValueKindOf` — the `NODE_INDEX` arm now classifies through the shared str-kind
  (`natNativeStrKind(e, 0, plan)`), so an index of any string-shaped base (ident today) is a
  string for the return dispatch.
- `natNativeStrKind` — new `NODE_INDEX` arm: a byte index of a string is a string; a base
  that is not a string is not; an unclassifiable base stays unknown. This is what makes
  `s[0] + "x"` classify as a concat and `print(s[0] + 1)` reach the concat scanner (which
  refuses the int operand).
- `natNativePlanDecls` — `let b = s[i]` infers the binding as a **string** slot (two units),
  so `print(b)` and `len(b)` see a string.
- `natNativeScanValue` — the `NODE_INDEX` arm: string base (validated as a byte index) or the
  historical int-walker path, unchanged for arrays and scalars. A **literal** base is refused
  with its own message before either arm sees it.
- `natNativeScanIntExpr` — an `NODE_INDEX` whose base is a string identifier is refused by
  name (`cannot appear in an integer expression`) instead of being lowered as an array element.
- `natNativeScanDecl` — `let b = <string index>` is accepted as a string binding; the arm sits
  before the catch-all non-integer refusal.
- `natNativeScanReturn` — a byte index in `main`'s return position is refused (exit status is
  an int); the BINARY block was made reachable (see §2).
- `natNativeScanStrSlice` — a string index as a slice base is refused by name (closes crash #2).
- Reporter rodata — the `"string index out of range"` kind string is interned **only when a
  string byte index exists** (per-function, plan-based detection over every function, with
  the same dead-callee conservatism as the reporter). Array-only programs produce byte-identical
  images to before this slice — that is what kept the frozen NAT-IDX gate green.
- `natNativeArenaBytes` — the compile-time arena bound now adds **exactly one byte per view
  operand** (`natNativeConcatViewBytes`), which the literal census could not see. Programs
  with no views compute the same size as before.
- `rctx` gained a fourth slot: `rctx[3]` is the rodata offset of the string-index kind string.
  The array path keeps reading `rctx[0..2]` and is unchanged.

**`pkg/cli/phase151s_stridx_test.go`** — the new gate `TestPhaseNATStrIdx_*` (layered:
execution / C-backend differential / bounds / refusals / determinism / CI wiring).

**`.github/workflows/ci.yml`** — one new step `Run NAT-STRIDX kcc native string byte indexing
gate` (`go test ./pkg/cli/ -run 'TestPhaseNATStrIdx_'`). Required by the gate-is-a-test-plus-CI-step rule.

---

## 4. File inventory

| File | Status | Purpose |
|---|---|---|
| `src/compiler/native_value.kark` | modified (+388/-52) | the implementation (§3) |
| `pkg/cli/phase151s_stridx_test.go` | new (519 lines) | the gate (§5) |
| `.github/workflows/ci.yml` | modified (+11) | CI step for the gate |
| `NAT-STRIDX-IMPLEMENTATION-REPORT.md` | new | this report |

**Not changed:** `VERSION`, `scripts/build-kcc.sh`, the NAT-STREQ gate and its CI step, the
NAT-STRSLICE gate and its CI step, the frozen NAT-IDX gate (`pkg/cli/phase151j_array_index_test.go`)
or its CI step, the Go compiler sources, the C runtime, `SPEC.md`, the two architecture
investigation reports, and every user-created untracked file in the repository.

Build artifacts (`kcc.exe`, `src/compiler/main.c`) are regenerated in place and are gitignored;
`main.c` is a stage-1 output of the Go engine, rebuilt here, never committed.

---

## 5. Verification

**Test command (isolated, per the ~4 GB host rule):**

```
$env:PATH = "C:\msys64\ucrt64\bin;$env:PATH"; $env:GOMEMLIMIT = '1GiB'
$env:KARKAIN_KCC = "F:\Codes\Git\Karkain\kcc.exe"
go test ./pkg/cli/ -run 'TestPhaseNATStrIdx_' -count=1 -timeout 35m
```

Result: **ok, 66.6s.** Layers:

| Layer | What it asserts | Result |
|---|---|---|
| `TestPhaseNATStrIdx_ProgramsExecuteWithExactBytes` | 14 programs compiled to PE, executed, stdout compared as **exact bytes** + exit code | PASS — Windows host, real execution |
| `TestPhaseNATStrIdx_MatchesTheCBackend` | 13 cases run through both engines; stdout compared (CRLF-normalized) | PASS — `utf8_multibyte_positions` skipped as native-only, loudly labelled |
| `TestPhaseNATStrIdx_OutOfRangeIsARuntimeError` | 6 bounds cases: non-zero exit + exact stdout produced *before* the failure | PASS — includes `a\nb\nc\n` in a loop that dies at index 3 |
| `TestPhaseNATStrIdx_RefusalsAreByName` | 8 refusal cases: no image, named construct | PASS — each refusal writes **no image** |
| `TestPhaseNATStrIdx_Determinism` | same source twice → identical image SHA-256 | PASS |
| `TestPhaseNATStrIdx_CIWiringIsPresent` | ci.yml runs `TestPhaseNATStrIdx_` under a `NAT-STRIDX` step | PASS |

**Oracle comparisons (Go C backend, `--engine go run`):** `a`/`c`/`b` (first/middle/last),
`b`/`c` (variable and `len`-based), `o` (`héllo` via `len-1` — both engines agree),
`a\nb\nc\n` (while and for loops), `ab`, `az`, `a\n1`, `b\ne\nb` — **byte-identical on every
case except the multibyte one**, which is native-only by design (§1.1).

**Multibyte executed bytes (native only):** `h\n\x00\n\x00\nl\n` for
`print(s[0..3])` of `"héllo"` — recorded as exact bytes, asserted exactly in the gate.

**PE vs Linux ELF:** PE images are **executed on this host** (Windows/amd64) and structurally
validated with the oracle's `native.ParsePE` before execution. **Linux ELF was not built or
executed by this gate** — this host is Windows, and the CI runner is billing-blocked (no green
CI URL exists for any of this). ELF byte-identity is **not applicable** here: the Go native
backend has no string indexing at all (`grep` for `emitStrIndex` in `pkg/native` is empty), so
there is no oracle image to compare against. The behavioural differential against the C backend
substitutes for it, as in NAT-IDX.

**Bounds diagnostics:** the Phase-152-B0 `GetStdHandle` limitation leaves the reporter's stderr
**empty** on this host, so the diagnostic text is not assertable here; the exit status and the
absence of a value *are* asserted. This is recorded, not assumed fixed.

**Mutation evidence:** flipping the string-index upper-bound jump (`Jge` → `Jle`) — the byte-level
bounds guarantee — was caught by `out_of_range_in_loop` (`stdout = , want 61 0a 62 0a 63 0a`).
A gate that asserted only "non-zero exit" would have **passed** under that mutation; the
before-failure-bytes assertion is what discriminates. The mutation was reverted and the image
rebuilt; the full suite re-passed.

**Two refusals close measured crashes** (`0xC0000005`), both now `error[K145]` with no image:
`return "a" + "a"` in a callee (`natNativeScanReturn`'s BINARY arm was dead code behind a stray
`return ""`) and `s[0][1:2]`.

**Regression gates re-run with the final binary (all on this host, isolated):**

```
go test ./pkg/cli/ -run 'TestPhaseNAT(StrEq|StrSlice|Idx|StrIdx)' -count=1
```
`ok  karkain/pkg/cli  244.906s` — NAT-STREQ (80.8s in isolation) ✓, NAT-STRSLICE (68.8s) ✓,
NAT-IDX (105.3s, frozen; array-only images byte-identical) ✓, NAT-STRIDX (66.6s) ✓.

**Host-environmental classes observed:** the ~600 KB stale compiler scratch in
`C:\Windows\Temp\opencode\` made the sibling assembler join ~500k garbage tokens and the
self-hosted compiler spin at 100% CPU indefinitely — that is what the "kcc hangs on s01" scare
actually was, and it was reproduced and then eliminated by moving the scratch out of sibling
scope; it was **not** a compiler defect. Two `.natreg.pristine*`/`original-native_value.kark`
scratch files and the string-WIP marker were archived to `archive-stale\` (not deleted).
No bytes of this repository's test suite rely on them.

**Not re-measured in this increment:** the whole-tree KIR pin (`15671`, stale since NAT-STREQ).
This change moves it again; re-pinning requires `kir --verify` over the ~20k-line tree, which
exceeds this ~4 GB host's safe budget. The pin is reported as stale, not predicted.
The two host-crash skips recorded for NAT-STREQ
(`eq_false_long_local_short_literal`, `eq_true_single_dash_local`) are preserved untouched and
are unreachable by this slice's corpus.

---

## 6. Dependency analysis

- **No new external dependencies** — no module, no C library, no runtime support library.
- **No new C runtime code** — the reporter used is the existing Phase-152-B0
  `karkain_runtime_error` path; the string-index kind string is interned in rodata only when
  a string-index site exists (measured: array-only images are byte-identical to before).
- **No Go production source changed** — the Go toolchain is untouched; `pkg/cli/phase151s_stridx_test.go`
  is test-only.
- The self-hosted front-end (`parser.kark`, `lexer.kark`, `ast.kark`, `checker.kark`) is
  **unchanged**; the slice is lowering-only.

---

## 7. Known limitations (recorded, not hidden)

1. **High bytes (≥ 0x80) do not survive the self-hosted rodata writer.** `natAppendStr` takes
   bytes via `asciiVal(substr(str, i, 1))`, and `asciiVal` has no byte source above 0x7E, so
   such bytes become `0x00`. This affects **every** literal, not just indexed ones
   (`print("héllo")` prints `h\x00\x00llo`). Fixing it needs a `byteAt` primitive in both
   engines — its own increment. The multibyte case therefore runs as a native-only golden and
   is explicitly excluded from the C differential.
2. **Embedded NULs in *source text* cannot be represented** — the self-hosted front-end reads
   the file through `strlen`, so a raw NUL terminates compilation. The representation supports
   NULs; the source path does not. The `héllo` case produces NUL bytes in the resident image
   and is the embedded-NUL evidence.
3. **Linux ELF was not executed** (Windows host; CI billing-blocked). The gate's PE execution
   and C-backend differential are the substitutes; CI wiring exists for when the runner is available.
4. **Bytes ≥ 0x80 in `s[i]` compare equal to `0x00`** (consequence of (1)). Native-only.
5. **`s[i]` is not sliceable, comparable, or bindable-as-concat in a `let`** — each refused by
   name (see §2). `let b = s[0]` (the bare view) is supported.
6. **View operands of a concat are accounted for exactly in the arena** (`natNativeConcatViewBytes`,
   +1 byte per view operand); programs with no views are byte-identical.
7. **`str_to_upper` remains unimplemented** — it needs its own increment on top of this one.
8. **The whole-tree KIR pin is stale** (see §5), and the two NAT-STREQ host-crash skips remain
   exactly as recorded.

---

## 8. Commit & push status

Committed to `feature/v1.3.0-natstdlib-string` as the final step of this increment
(the commit hash is printed in the final status section below). Not merged to `develop` or
`main`; no tag; no `VERSION` change; no release. This report is included in the commit.

---

## 9. Final Git status

`git status --short` at the end of the increment is printed in the turn that produced this
report; the two previously untracked investigation reports
(`NATIVE-STRING-ARRAY-ARCHITECTURE-INVESTIGATION.md`, `STDLIB-V2-NATIVE-BLOCKER-ASSESSMENT.md`)
and every user-created untracked file remain untouched and uncommitted, as do `VERSION` and
all protected gates.
