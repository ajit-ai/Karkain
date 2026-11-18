# NAT-STRTRIM — Native Trim as a Non-Allocating String View

**Increment:** NAT-STRTRIM (native `trim` / `str_trim`)
**Branch:** `feature/v1.3.0-natstdlib-string`
**Starting HEAD:** `ff137cabd81c3e747177e47ec508e6e33e177f8b` (`ff137ca docs: record the NAT-STRIDX commit and push hashes in the report`)
**Ending HEAD:** recorded in the final Git status below (commit appended after this report)
**Status of acceptance criteria:** implementation complete and measured; see *Status of acceptance criteria*.

---

## 1. What was implemented and the exact semantics

### 1.1 The verified semantic contract (established before any code)

The actual standard-library API was inspected, not assumed:

- `stdlib/string/string.kark:149-152` — `str_trim(s)` **delegates** to the builtin `trim(s)`;
  `str_trim_left` / `str_trim_right` (`stdlib/string/string.kark:325-348`) pin the whitespace
  set as byte comparisons against `" "`, `"\t"`, `"\n"`, `"\r"`.
- `pkg/codegen/codegen.go:2185-2198` (`karkain_trim`) — strips exactly those four bytes from
  **both** ends; internal whitespace is preserved; empty / all-whitespace inputs produce `""`;
  a non-string argument produces `""` in the C runtime (the native scan refuses it by name).
- `SPEC.md` (strings): strings are UTF-8 byte strings; `len()` and indexing are byte-wise;
  case conversion is ASCII-only.

Verified contract implemented on the native (kcc) path:

- Leading **and** trailing ASCII whitespace are removed; the set is exactly `{0x20, 0x09, 0x0a, 0x0d}`.
- Internal whitespace is preserved.
- Empty input → zero-length view; all-whitespace input → zero-length view; the view is never
  dereferenced when the length is zero.
- A view that trims nothing IS the source string (ptr/len unchanged) — not a copy of a trimmed
  string; it *is* the source re-armed.
- The result is `(ptr + start, trimmed_len)` of the source bytes — **no allocation, no copy**.
- UTF-8 high bytes (≥ 0x80) are preserved as bytes and are never treated as whitespace.

### 1.2 What this slice does (and does not) implement

Native `trim` / `str_trim` on the self-hosted kcc native driver, as a non-allocating string
view:

- `trim(s)` lowers to the same two-unit `(RDI=ptr, RSI=len)` string shape every string
  operation uses; the view is `(RBX + RCX, RAX - RCX + 1)` with the all-whitespace/empty case
  returning a zero-length view.
- The scans are register-only: leading scan (`RCX` index, `natCmpRegReg`/`natJz`/`natJge`,
`natIncReg`) and trailing scan (`RAX` index, `natCmpRegReg`/`natJl`/`natDecReg`/`natIncReg`),
joined through `natTrimGotoWs` (four-way byte test: 32, 9, 10, 13).
- `len(trim(s))` lowers only at positions that carry string-value context (direct print /
  return / callee return); nested-in-arithmetic positions are refused by name rather than
  emitting a wrong answer.

---

## 2. Implementation

All production changes are in the self-hosted Karkain compiler (`src/compiler/native_value.kark`);
no Go production source, no C runtime, no new dependency.

- `natTrimGotoWs(s, ch, advLbl)` — the four-way whitespace test + jump.
- `natEmitTrim(s, arg, stmts, plan, roffs, tbl, ictx)` — lowers the builtin; returns the view in
  `(RDI, RSI)`; handles the empty/all-whitespace zero-length case.
- `natEmitStrValue` gained a `trim` arm (built on `natEmitTrim`) plus the pre-existing `NODE_INDEX`
  routing to `natEmitStrIndex` (from NAT-STRIDX, unchanged by this slice).
- `natNativeStrKind` gained a `trim` arm (a trim is a string); `natNativeBuiltinRetKind` maps
  `trim` → string kind (2), so `let b = trim(s)` classifies and binds the view, and
  `str_trim`'s builtin return kind feeds the stdlib's callee return lowering.
- `natNativeScanTrim(call, plan, line, fns)` — scan: 1 argument; string literal / string
  variable / string index / nested trim accepted; everything else refused by name (int scalar,
  undeclared, concat, slice, record field, array index base).
- `natNativeScanLen` / print-and-return dispatch — `len(trim(s))` handled at direct-value
  positions; refused in arithmetic.
- Decl / let-binding dispatch — `let b = trim(s)` binds the two-unit view; `let b = trim(s) + "x"`
  refused (non-int init).

### 2.1 Interim fix landed during the increment (not trim scope, but required to keep the
### 2.2 driver compilable and the frozen gates honest)

While landing the trim slice, an interim guard in `natNativeScanCall` refused *every* argument
position containing a call; the frozen NAT-CALL corpus (`inc(inc(n))` — argument 0 is itself a
call) then failed. The guard was refined: argument index ≥ 1 containing a call is refused (the
spill area is reused across calls); argument index 0 containing a call is refused **only when the
argument kind is not int** (int args recurse through the int walker; string/array args cannot
stage through the two-unit spill). Frozen NAT-CALL gate re-run green afterwards.

---

## 3. Tests and gates

### 3.1 New focused gate: `pkg/cli/phase151t_strtrim_test.go` — `TestPhaseNATStrTrim_`

Six layers:

1. **`TestPhaseNATStrTrim_ProgramsExecuteWithExactBytes`** — corpus of valid cases, exact
   stdout bytes compared (never trimmed) and exit codes, through the kcc-native PE (windows).
2. **`TestPhaseNATStrTrim_MatchesTheCBackend`** — same sources through the C backend
   (`--engine go`), stdout CRLF-normalized; the multibyte case is `nativeOnly` (see §5).
3. **`TestPhaseNATStrTrim_RefusalsAreByName`** — refusal table; asserts the refusal text, that
   kcc did not accept, and that no image was written.
4. **`TestPhaseNATStrTrim_StrTrimWorks`** — end-to-end `import std.string` + `str_trim` through
   the real CLI (`karkain build --target native-x86_64-windows` with `KARKAIN_ENGINE=kcc`),
   provenance markers + `native.ParsePE` + exact run output.
5. **`TestPhaseNATStrTrim_Determinism`** — two builds → identical SHA; corpus ≥ 20; ≥ 8 distinct
   stdout shapes (anti-constant-emitter).
6. **`TestPhaseNATStrTrim_CIWiringIsPresent`** — `ci.yml` carries the step + pattern.

Valid cases executed (all exact bytes, all exit 0): `no_trim_needed`, `leading_whitespace`,
`trailing_whitespace`, `leading_and_trailing`, `internal_whitespace_preserved`, `empty_string`,
`all_whitespace`, `one_byte_string`, `one_byte_whitespace`, `raw_tab_and_cr`, `many_consecutive_whitespace`,
`repeated_trim_calls`, `len_of_trim`, `bind_view_then_print`, `concat_view_plus_literal`,
`concat_literal_plus_view`, `concat_view_plus_view`, `trim_returned_from_callee`,
`trim_from_param_returned`, `nested_trim_in_callee`, `utf8_high_bytes_native_only`.

### 3.2 CI

Added step (after the NAT-STRIDX step, purely additive):

```yaml
- name: Run NAT-STRTRIM kcc native trim gate
  run: go test ./pkg/cli/ -run 'TestPhaseNATStrTrim_' -count=1 -timeout 40m -v
```

Pattern `TestPhaseNATStrTrim_` is matched by no other step.

### 3.3 Regression gates run during the increment

| Gate | Result |
|---|---|
| `go test ./pkg/cli/ -run 'TestPhaseNATStrTrim_' -count=1 -timeout 35m` | **PASS** (119.7s after source restore) |
| `go test ./pkg/cli/ -run 'TestPhaseNAT(StrEq\|StrSlice\|Idx\|StrIdx\|StrTrim)' -count=1 -timeout 40m` | **PASS** (302.7s) |
| `go test ./pkg/cli/ -run 'TestPhaseNATCall'` | **PASS** (50.4s) after the arg0-guard refinement |
| `go test ./pkg/cli/ -run 'TestPhaseNATLoop'` | **FAIL** — see §6/§8 (reproduced on pristine source; not trim-caused) |

---

## 4. Oracle comparisons and native execution

- The **Go native oracle** (`pkg/native`) has no trim; the **C backend** (`karkain_trim`,
  `pkg/codegen/codegen.go:2185`) is the semantic reference for trim. The C-backend differential
  (layer 2) is therefore the parity layer for this increment, with CRLF normalization.
- Windows PE execution (kcc-produced images executed on the dev host): all applicable corpus
  cases byte-exact (`hi\n`, `a b\n`, `\n`, `a\n`, `hi\n2\n`, `hi!\n`, `!hi\n`, `hi\nhi\n`, …).
- End-to-end stdlib: `import std.string` + `str_trim` produced a kcc-owned PE that printed `hi`
  for `"  hi  "`; the Go engine produced the same.
- Linux ELF: **not executed** this increment (host is windows/amd64; no suitable runner). The
  C-backend differential is the substitute for observable behaviour.

## 5. Status of acceptance criteria

| # | Criterion | Status |
|---|---|---|
| 1 | Actual trim contract documented (before coding) | **Verified** (§1.1) |
| 2 | Native trim returns a valid non-allocating view | **Verified** (§2) |
| 3 | Exact output matches the oracle for supported cases | **Verified** (§4, gate layer 1/2) |
| 4 | Empty and all-whitespace cases safe | **Verified** (corpus) |
| 5 | String-view lifetime correct | **Verified** (§2; sources are rodata/arena-backed) |
| 6 | No external runtime dependency introduced | **Verified** |
| 7 | Focused tests pass | **Verified** (gate PASS) |
| 8 | Existing string regression gates remain intact | **Verified** for STR/SLICE/IDX family; **see** NAT-LOOP finding (§8) |

## 6. Known limitations

- **Bytes ≥ 0x80 do not survive the self-hosted rodata writer** (`asciiVal` has no byte source;
  whole-literal limitation, recorded in the NAT-STRIDX report). The multibyte trim case is
  therefore `nativeOnly` in the differential and asserts behaviour on the resident bytes.
- **KIR pin is stale** for this host (whole-tree `kir --verify` exceeds the ~4 GB host budget);
  deliberately not predicted (evidence discipline).
- **NAT-LOOP `multiple_loop_locals`** fails — see §8 (reproduced on pristine source; not trim-caused).

## 7. Git controls

- No merge to `develop`/`main`; no tag; no VERSION change; no release.
- Commit appended after this report; push to `origin` on `feature/v1.3.0-natstdlib-string`.

## 8. Final report fields

- Starting / ending branch and full commit hashes: starting `ff137ca…` (`ff137cabd81c3e747177e47ec508e6e33e177f8b`), ending recorded in final Git status.
- Verified / Failed / Skipped / Not Tested (explicit labels):

  - **Verified:** trim contract; non-allocating view; exact-byte PE execution; C-backend parity;
    refusals by name; determinism; stdlib end-to-end; NAT-STRTRIM gate; string-family gates.
  - **Failed:** NAT-LOOP `multiple_loop_locals` (native-ast exit 3221225477; ELF byte-diff) —
    **reproduced with the pristine `ff137ca` compiler source restored via `git show`**, so it is
    not attributable to this increment's trim implementation; separate triage required.
  - **Skipped:** Linux ELF execution / byte identity (no suitable runner); Sphinx docs build
    (no Sphinx on this host); `kir --verify` (host memory budget).
  - **Not Tested:** CI execution of the new step (billing-blocked); whole-tree KIR pin.

## 9. Final Git status

- Branch: `feature/v1.3.0-natstdlib-string`
- HEAD at report time: `ff137ca…` (commit appended after this report)
- Modified: `.github/workflows/ci.yml`, `src/compiler/native_value.kark`
- New: `pkg/cli/phase151t_strtrim_test.go`, `NAT-STRTRIM-IMPLEMENTATION-REPORT.md`
- Untracked user files preserved (KARKAIN_*.md, NATIVE-STRING-ARRAY-ARCHITECTURE-INVESTIGATION.md,
  STDLIB-V2-NATIVE-BLOCKER-ASSESSMENT.md, docs/audit/KARKAIN_V1_2_0_RELEASE_BASELINE.md,
  examples/compute/matmul.c, PERSONAL_THREE_TECH_PAGES_RECOVERY_REPORT.md).
