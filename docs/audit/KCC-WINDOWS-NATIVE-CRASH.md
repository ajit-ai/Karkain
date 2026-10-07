# KCC Windows Native-Runtime Defect — `0xC0000005` in `kcc check`

**Status:** investigation CLOSED at this evidence level.
**Classification:** ROOT CAUSE MECHANISM STRONGLY NARROWED — the immediate
malformed-string producer is narrowed to `make_string()`/`strdup()`, but the runtime
allocation failure itself was **not directly observed**.

This is a **known KCC Windows native-runtime defect**, not an architectural failure.
KCC remains a core objective of the Karkain self-hosting roadmap and is neither
abandoned nor deprecated.

## 1. Symptom

```text
karkain check src/compiler/main.kark --engine kcc
  exit 3, stdout 0 bytes, stderr 0 bytes, ~146-226 s, ~1.9 GB peak working set
```

Invoked directly (`kcc.exe check src/compiler/main.kark`, bypassing the Go wrapper):

```text
exit -1073741819 == 0xC0000005 == ACCESS_VIOLATION
```

Reproducible 3/3 on the dev host — **deterministic, not intermittent**.

Distinct from the intermittent `kcc kir --verify` crash in `PHASE-151A-BASELINE.md`
§9a-1. That one is whole-tree KIR on a low-RAM host; this is the self-check.

## 2. Crash site

GDB 15.2 backtrace at the fault:

```text
#0  ucrtbase!strcmp+64
#1  binary_op()                        src/compiler/main.c:1086
#2  karkain_user_stripModuleImports()  src/compiler/main.kark:916
#3  karkain_user_assembleProject()
#4  karkain_user_checkFile()
#5  main()
```

```c
Value binary_op(Value left, const char* op, Value right) {
    if (left.type == TYPE_STRING && right.type == TYPE_STRING) {
        int c = strcmp(left.strVal, right.strVal);   // <-- left.strVal == NULL
```

`strcmp(NULL, valid)` dereferences NULL.

## 3. Proven facts

| Fact | Evidence |
|---|---|
| `sizeof(Value)==16`; `type`@+0, `strVal`@+8 | `src/compiler/main.c:256-289` |
| `binary_op` takes 4 register args: RCX, RDX=left `Value*`, R8=op, R9=right `Value*` | static disassembly |
| No ABI mismatch | prologue consumes exactly 4 args; both derefs at +0/+8 as predicted |
| `left.type==2`, `left.strVal==0` **on entry** to `binary_op` | GDB breakpoint at `binary_op` |
| Right operand is valid | R8 is the **operator string**, not a `Value`. The earlier "both operands malformed" reading was a mislabelling and is **withdrawn** |
| `left` is a caller stack local at `rbp-0x50` | live call site: `lea -0x50(%rbp),%rax ; mov %rax,%rdx` |
| That local is fully initialised | 12 paired `mov %rax,-0x50(%rbp)` / `mov %rdx,-0x48(%rbp)`; **no SSE aggregate stores**. The earlier "16-byte truncation" hypothesis is **falsified** |
| `array_get` bounds-checks both ends | `if (i < 0 \|\| i >= arr.arrVal.length) return make_int(0);` (`pkg/codegen/codegen.go:1506`) |
| `array_push` copies all 16 bytes | `Value* copy = malloc(sizeof(Value)); *copy = elem;` (`codegen.go:1221`) |
| `appendArray` is a pass-through | forwards to `array_push`; no conversion, no string special-casing |
| The malformed element is genuinely **in bounds** | GDB + the bounds check |
| Element originates at the `lines[i]` load | chain proven by explicit two-store copies, no intervening calls |

```text
lines[i] -> rbp+0x290/0x298 -> rbp+0x90/0x98 -> rbp+0x50/0x58 -> rbp-0x50/0x48
        -> binary_op -> strcmp(NULL, ...) -> 0xC0000005
```

## 4. Static producer chain

```text
sourceLines()   src/compiler/main.kark:826
  -> substr()          karkain_substr (src/compiler/main.c)
  -> make_string(buf)  pkg/codegen/codegen.go:1196
  -> strdup(buf)       <-- unchecked
  -> appendArray() / array_push()   faithful 16-byte copies
  -> lines[i]
```

Every `TYPE_STRING` `Value` is constructed by `make_string`:

```c
Value make_string(const char* s) {
    Value val;
    val.type = TYPE_STRING;
    val.strVal = strdup(s);      // return value never checked
    return val;
}
```

It is the **only** site that can yield `TYPE_STRING` + `strVal==NULL`, and every caller
on this path passes a non-NULL buffer. A NULL `strdup` result is therefore the only
statically consistent explanation.

## 5. NOT proven — explicitly NOT claimed

* **`strdup()` returning NULL was NOT directly observed.** An instrumented run was
  attempted but the `buildKCC` path was never exercised, so no marker was emitted.
* **No memory-exhaustion claim.** The ~1.9 GB peak is contextual only, not evidence.
* **No out-of-bounds claim.** `array_get` is proven bounds-safe.
* **No store-width claim.** Proven falsified.
* **Not attributable to 9g.** CI fails identically at `f2fa8f8`, `714adab`, `e1a6400`
  — all before/at 9g — and the faulting code is not 9g code.

## 6. Go / self-hosted parity findings (recorded, NOT fixed)

Independent of this crash; each would be a separate deliberate change.

| # | Finding | Self-hosted | Go side |
|---|---|---|---|
| A | `make_string` NULL-input guard | `strdup(s ? s : "")` guarded (`src/compiler/codegen.kark:732`) | `strdup(s)` unguarded (`pkg/codegen/codegen.go:1196`) |
| B | String indexing `s[i]` -> char | present | absent |

Finding A means the two engines do **not** emit equivalent C runtimes. Neither causes
this crash — `karkain_substr` passes a non-NULL `buf`.

**A NULL fallback must not be added merely to make the crash disappear.** Guarding
`binary_op` would turn a crash into silently comparing `""`, masking the producer.

## 7. The one decisive next diagnostic

Not performed; recorded so it is not lost.

```text
1. Record SHA of pkg/codegen/codegen.go.
2. Instrument ONLY make_string (L1196):
     if (s == NULL) { fprintf(stderr,"MAKE_STRING_NULL_INPUT\n"); fflush(stderr); }
     val.strVal = strdup(s);
     if (val.strVal == NULL) { fprintf(stderr,"MAKE_STRING_STRDUP_NULL len=%llu\n",
                                       (unsigned long long)strlen(s)); fflush(stderr); }
   (no fallback; behaviour otherwise unchanged)
3. TRIGGER THE REAL BUILD PATH - the step an earlier attempt missed:
     delete kcc.exe, then run `karkain kir ...`
   The staleness check fires buildKCC() -> `karkain build src/compiler/main.kark`
   with KARKAIN_ENGINE=go -> main.c -> gcc. Running `karkain build` while a
   non-stale kcc.exe exists does NOTHING.
4. VERIFY [byte-scan kcc.exe for "MAKE_STRING_"] BEFORE the 150-230 s run.
   This cheap assertion caused the earlier false negative.
5. Run `kcc.exe check src/compiler/main.kark` once; capture stderr + exit status.
6. Restore codegen.go (verify SHA), delete kcc.exe, rebuild clean.
```

`MAKE_STRING_STRDUP_NULL` confirms the producer. `MAKE_STRING_NULL_INPUT` redirects to
the caller. Neither means the hypothesis is wrong.

## 8. CI impact

`TestPhase146C_CheckCleanKCC` (`pkg/cli/phase146c_generics_test.go:145`) fails because it
invokes this self-check. **Phase 146 generics itself is healthy** — 19 of 20 tests pass,
including both engines' goldens, the demotion rule and every rejection case. Only the
compiler self-check assertion fails.

In CI the step terminates with `exit 143` (SIGTERM, ~34 s, no output), as did two other
concurrent whole-tree kcc jobs. That is **external infrastructure termination**, not a
Phase 146 or 9g regression, and is reported separately from the local `0xC0000005`.

## 9. Related diagnostic-fidelity defect (independent)

`karkain` collapses kcc's `0xC0000005` into `exit 3` with empty output, so a caller
cannot distinguish "the compiler rejected your program" from "the compiler crashed".
This is why the class stayed recorded as merely "unresolved" for so long. Not fixed
here; changing it would surface many previously hidden crashes at once and deserves a
deliberate decision.