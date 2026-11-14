# NAT-STDLIB baseline — native standard-library qualification for `stdlib_v2`

**Milestone:** `NAT-STDLIB` — v1.3.0 §1 item 2; carried v1.2.0 condition 2.
**Version:** 1.3.0 (NOT open; `VERSION` unchanged; no release claim follows).
**Branch:** `feature/v1.3.0-natstdlib` (created from `feature/v1.3.0-natcall` at `aa45970`; `main` verified at `b5dcd7f`).
**Starting commit:** the NAT-CALL branch head `aa45970` — the brief's expected base; v1.2.0 tag `e900c91` → `77d7069` unchanged.
**Final commit:** pinned after the qualification commit lands (hash recorded in `AGENTS.md` and the engineering report).
**Verdict up front:** **PARTIAL.** The int-array slice of the native
standard-library story is implemented and executed. `examples/stdlib_v2` does NOT
compile through the native path, will not, and this milestone does not pretend
otherwise: the binding of a call result in `main` (`let name = str_to_upper(...)`)
is refused BY NAME with `K145`, because the string runtime this needs does not
exist on the native target. The carried v1.2.0 condition 2 is therefore still
UNMET in full.

---

## 1. The measured starting position

`examples/stdlib_v2/main.kark` is the acceptance target. Imports four modules and
relies on: string trimming/casing/concat/split, `len` on an array of strings,
array_max/sum/contains, hex/base64 codecs and sha256 — none of which the native
driver lowered. The measured starting refusal, on the committed compiler
(`develop`'s lineage):

```
karkain build --target native-x86_64-windows examples/stdlib_v2/main.kark
error[K145]: native target: `let name` initialised from a non-integer expression
harmless on the Go engine, fatal here: main's `let` arm could not hold a call
result, and the string runtime below the line was not coming.
```

None of the native compilers owns an stdlib runtime: `src/compiler/native_value.kark`
lowered int/array/record/push/branch/loop shapes only. The standard library is written
in canonical `.kark` over runtime builtins, and those builtins have no native
emission at all.

## 2. What this milestone delivers

An int-array slice, end to end and executed:

1. **Array parameters.** A parameter the call sites pass an array to is inferred
   as an array — the parser discards parameter annotations, so an argument at a
   call site is the only kind evidence available. Kinds are computed by fixpoint
   over every call site in the program (at most one promotion per parameter per
   pass, 8 passes max). A parameter that receives BOTH an array and a non-array
   argument across the program is refused BY NAME with the function and the
   parameter. An array parameter homes as two frame units and receives exactly
   one argument shape — an identifier whose slot is an array; anything else
   (array literal, expression, int literal) is refused BY NAME.
2. **`len(arr)` inside a called function** — the len-as-callee refusal is gone;
   len routes to the driver's own array count load, like the oracle.
3. **`while (i < len(arr))` in a loop condition, in main and in a callee** — a
   PRE-EXISTING defect is fixed here: `len` in a condition routed into the
   user-call lowering and the PE linker crashed with an out-of-bounds write.
   Dedicated print/return arms handled len; the condition path did not, which
   is why no gate caught it.
4. **Bool literals** (`true`/`false` lower to 1/0).
5. **Reachability skipping at emit time.** An unmodified module carries
   functions the program never calls, whose bodies may use constructs the
   driver refuses. A dead function the driver cannot lower is **skipped**
   (never scanned for refusal, never emitted); a dead function it CAN lower is
   emitted byte-identically (this preserves the frozen NAT-STR-STAGE frame
   pin). A **reachable** callee that cannot be lowered remains a refusal,
   first and loudest.
6. **Runtime-error reporter for indexes inside CALLEES** — the reporter gate
   previously scanned only main's body; a callee's index recorded a rodata
   reference against a section with no reporter strings and the PE linker
   crashed. Program-wide now.
7. Everything works with **no C toolchain** on the tested path (152-A failing
   stubs), and the rebuilt self-hosted engine has **no Go runtime** on the
   native path either.

## 3. `examples/stdlib_v2` status (the acceptance line)

**NOT compiling, NOT claimed.** The native refusal, verbatim:

```
error[K145]: native target: `let name` initialised from a non-integer expression
(main.kark line 745). ... `let` of an integer expression inside a called function;
`print` and `return` of an integer literal, a local, `a[i]`, `len(a)`, ...
```

The first blocker, in assembly order:

| Statement | Required native capability | State |
|---|---|---|
| `let name = str_to_upper(str_trim("  karkain  "))` | string params/returns, string equality, byte indexing, string concat in callees, main `let`-from-call | NONE of it exists |
| `print(str_concat("hello ", name))` | concat in main + string print (some exists) | partial shapes exist in main; string locals from calls do not |
| `hex_decode`/`base64_decode`/`sha256` | hex/base64/sha256 builtins | NOT lowered by this driver |
| `str_split(...)`/join over `parts` | split builtin + arrays of strings + print of a string array element | NOT lowered |

No acceptance criterion for the whole example is met. The example is the one
source where `std.string`, `std.encoding`, `std.crypto` all appear, and the
string runtime is the same missing capability for every one of them.

## 4. Support matrix (module × function × verdict × evidence)

| Module | Public function (as used) | Verdict | Evidence |
|---|---|---|---|
| std.collections | `array_sum` | NATIVE-PASS | executed PE: `26`; `import std.collections` compiles and runs; PhaseNATStdlib gate |
| std.collections | `array_max` | NATIVE-PASS | executed PE: `9`; same gate |
| std.collections | `array_contains` | NATIVE-PASS | executed PE: `1` then `0`; bool literal lowering |
| std.collections | `array_min`, `array_index_of` | NATIVE-PASS (by the same mechanism) | gated through the shared infer/lower path; not individually executed in this milestone |
| std.collections | `array_max_2` (returns array literal) | UNSUPPORTED | refused BY NAME (`return` of an array is not lowered) |
| std.collections | `array_contains_str`, `array_index_of_str` | UNSUPPORTED | string parameters and comparisons not lowered |
| std.collections | `map_keys`, `map_values`, `map_merge`, `map_filter_*`, `map_keys_of` | UNSUPPORTED | maps and the `map_keys_of` builtin are not lowered; dead in the acceptance program and skipped at emit time |
| std.string | every function (`str_trim`, `str_to_upper`, `str_concat`, …) | UNSUPPORTED | no string params/returns, no string equality, no byte indexing (`s[i]`), no string slicing, no concat in callees |
| std.encoding | `hex_encode_bytes`, `hex_decode_bytes`, `base64_*` | UNSUPPORTED | hex/base64 codec builtins not emitted by the driver |
| std.crypto | `sha256_hex`, `sha256`, … | UNSUPPORTED | sha256 builtin not emitted; no implementation of it natively |
| std.testing/others | — | UNKNOWN/not probed | out of scope |

**Both-engine status:** the Go oracle does not build these programs either
(`error[K145]: unsupported return expression *parser.ArrayLiteral`) — the native
stdlib consumption is kcc-only today, and byte identity with the oracle is
unconditionally claimed nowhere in this milestone.

## 5. Native build command and target

```
karkain build --engine kcc --target native-x86_64-windows --target native-x86_64-linux <file.kark>
```

- Windows/amd64: **EXECUTED** PE on this host (evidence in the gate).
- Linux/amd64: **BUILD + structural parse only** on this host (no runner);
  execution evidence is Linux-CI-bound and that CI is billing-blocked as of
  2026-11-14.
- macOS: unchanged — structural only, no runner, no execution claims.

## 6. The no-C-compiler proof

The gate shadows `gcc`, `clang` and `cl` with failing stubs on PATH (152-A's
mechanism), asserts each stub resolves and fails, then builds
`import std.collections` + an int-array program with
`--engine kcc --target native-x86_64-windows` and RUNS it. It prints
`32360a390a` (26, 9). Result: PASS. This proves the tested native path does not
reach a C compiler. It does NOT prove the whole Karkain development toolchain is
C-free — building the self-hosted compiler still needs gcc (stage 1 link).

## 7. Focused tests, counts, durations

```
go test ./pkg/cli/ -run 'TestPhaseNATStdlib_' -count=1 -timeout 30m
ok  15.1s   (9 tests / 20 subtests, PASS)
```

Regression gates re-run on this host, all PASS: NAT-CALL (35.6s), NAT-CALL-QUAL
(16.2s), NAT-IF (14.4s), NAT-LOOP (77.1s), NAT-IDX (62.7s), NAT-STR-STAGE
(30.0s), NAT-RODATA (16.7s), NAT-FRAME (15.0s), NAT-REG (already green), D2/D5/D8
(parity + diagnostics gates), NAT-QUAL (7.5s), Phase 151 family (97.8s).

**CI:** as of 2026-11-14 GitHub Actions is billing/payment-restricted; the
dedicated *Run NAT-STDLIB native standard-library gate* step is wired in ci.yml
and its wiring is asserted in the gate, but it has NOT run in CI. CI status is
BLOCKED / NOT EXECUTED, not PASS.

## 8. Defects found and fixed inside this milestone

1. `while (i < len(arr))` — len in a condition routed to user-call lowering;
   the PE linker crashed. Fixed at the single point every position shares.
2. Runtime-error reporter for an index inside a callee — program-wide gate;
   the PE linker crashed before. Same shape of defect as NAT-STR-STAGE's
   program-wide staging fix.
3. Reachability discipline — my first cut PRUNED dead functions from the list,
   which silently broke a frozen NAT-STR-STAGE pin (it expects a supported-but-
   dead helper to emit; wantFuncs: 2). Replaced by emit-time skip keyed on
   `(unreachable AND unsupported)`, keeping supported dead functions byte-
   identical to the oracle.
4. `callsInNode`'s accumulation of found calls restarted per position, which
   discarded every call found in one position when a later position was
   reached — an `if` kept no callees it mentioned in its condition.

## 9. KIR pin

`kir.kark`: **15671** (measured `text=15671 verify=15671` on this host after all
the slice's edits; 15243 before). Re-pinned in `phase122_pipeline_ownership_test.go`
and `phase151a5_win_test.go`. Never predicted.

## 10. Remaining Go/C/bootstrap dependencies and deferred work

- No Go runtime is linked into native executables (they are self-contained
  ELF/PE/Mach-O); the compiler's own bootstrap still needs gcc.
- `import "C"` hang: pre-existing, untouched, not a prerequisite here.
- The string runtime is the next tranche: string params/returns, equality,
  slicing, byte indexing, concat in callees, `let name = <call>` in main, the
  hex/base64/sha256/sha256 emit/benchmarks — one tranche per reachable user.
- `std.testing`, `std.io`, `std.net` — NAT-NET's scope, untouched.
- 154/NAT-BOOTSTRAP: untouched.

## 11. Honest milestone verdict

**PARTIAL — and conditional 2 remains UNMET.** What this milestone makes true:
an int-array slice of the native stdlib is implemented, executed and gated
(unmodified `std.collections` int functions run natively). What it does not
make true: `examples/stdlib_v2` building natively, any of std.string/encoding/
crypto in native form, or `stage2 == stage3`/self-hosting closure — all are
named here so nobody reading one line of this report accepts the next as done.
