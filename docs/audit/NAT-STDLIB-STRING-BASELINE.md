# NAT-STDLIB-STRING baseline — string tranche, honest state after the 2026-11-15/16 session

**Milestone:** `NAT-STDLIB-STRING` (v1.3.0 continuation of NAT-STDLIB).
**Version:** 1.3.0 (NOT open; `VERSION` unchanged; no release claim follows).
**Branch:** `feature/v1.3.0-natstdlib-string` (from `199ee87`).
**Verdict today: PARTIAL.** Meaningful implementation landed and was qualified on
Windows/amd64 with exact byte evidence; mandatory criteria remain unmet and
stdlib_v2 does not execute. NO acceptance criterion is claimed met that was not
verified on this host.

This document supersedes the 2026-11-15 BLOCKED record, which is preserved in
Appendix A.

---

## 1. What was actually implemented (this tranche, under VERSION unchanged)

The self-hosted native value model (`src/compiler/native_value.kark` +
`src/compiler/native_emit.kark`) gained, all verified by executed PE images
with **exact** stdout bytes on this Windows/amd64 host:

1. **Program-wide callee literal interning.** The rodata literal collector now
   recurses into `if`/`while`/`for` bodies and walks **every emitted
   function** in emission order (the skip predicate mirrors the emitter's
   reachability skip). The old positional `roffs[sidx]` cursor is gone in
   favour of value-lookup `natRodataOffsetFor` — the cursor desynchronisation
   class that produced the "image offset 0 (MZ header)" garbage output and the
   Step-9g linker panic is closed by construction.
2. **String-return ABI.** A callee's string result lowers through `(RDI, RSI)`
   via `natEmitStrValue`, and the return dispatch is by value-kind
   **before** the int path. Literal returns from callees (`return "one"`)
   resolve correctly now that callees' literals are interned (§1).
3. **`let` of a string-returning call** in main binds from `(RDI, RSI)` into
   the two-slot string slot.
4. **String parameters** are homed as two units by the kind-aware homing
   already in place; the two-unit argument staging path now also
   **materialises a string literal argument directly** into the arg spill
   (`f("direct literal")` works), plumbed through new emitter string-context
   slots 17/18 (`natStStrStmts`/`natStStrRoffs`, `natSetNativeStringCtx`).
5. **Call-site kind inference** widened: a combined parameter/return-kind fixpoint
   over the call graph (replacing the parameter-only pass). Mixed string/int
   return detection still exists but now fires ONLY for reachable functions
   with a concrete call site — dead functions (e.g. `str_char_at` in the
   stdlib_v2 call graph) no longer abort the whole build on disjoint return
   arms. This removes the `str_char_at` false-positive that previously blocked
   the example.
6. **Builtin return-kind table** (`hex_encode_bytes`/`hex_decode_bytes`/
   `base64_*`/`sha256_hex`/`sha512_hex` → string; `int`/`len` → int):
   `return hex_decode_bytes(x)` no longer classifies as an int return.
7. **Arena bound extended to `if` branches** (concat/push sites inside branches
   are counted against the compile-time bound; loops still excluded — the loop
   scan refuses those constructs by name).

## 2. What was EXECUTED and verified this session (evidence classes listed)

**PE images executed on Windows/amd64, exact stdout bytes (`run | hexdump`):**

| Probe | Program | stdout |
|---|---|---|
| bind_call | callee returns a literal string | `karkain\n` |
| branch_lit | literal returned inside `if`/`else` | `one\nother\n` |
| callee_lit | callee returns a literal via print | `world\n` |
| callee_refarg | literal argument promoted to string param | `refarg\n` |
| callee_print_lit | callee prints a literal | `from callee\n` |
| callee_callee | call-chained string return across callees | `inner\n` |
| str_ret2 | print param then literal return | `karkain ok\n` |
| lit_local / lit_arg_bind / lit_arg_direct / lit_arg_let | local, bound and inline literal args | `direct literal\n` |

`NAT-CALL` gate re-qualified **after** the tranche: **PASS (33.7s)** with the
two previously-refused programs (a string argument, a callee returning a
literal) moved into an executing corpus `TestPhaseNATCall_StringProgramsExecute`
(7 subtests, exact `68690a`/`780a` etc.).

`NAT-CALLQual` gate re-qualified: **PASS (19.5s)** — two stale refusal pins
removed, two new corpus cases added, Linux ELF byte-identity differential still
green for the shared corpus (7 ELF pairs byte-identical to the oracle; the two
promoted-string programs are kcc-only, pinned).
`TestPhaseNATStdlib_ImportedCollectionsExecutes` now fails ONLY on this host
(§4 environmental). `TestPhaseNATStdlib_LinuxELFIsStructurallyValid` likewise.
All other NAT-STDLIB subtests pass isolated.

## 3. What is NOT done (exact gaps, in dependency order)

1. **The whole `examples/stdlib_v2` acceptance target does not execute
   natively.** Refusal today: `` `let parts` initialised from a non-integer
   expression`` (`str_split` wants a *string array* kind; the native value
   model has no string-array representation). The fixture is pinned by NAME in
   `TestPhaseNATStdlib_ImportedCollectionsNoImageThenReport`.
2. **The codec builtins have no native lowering** — `hex_decode_bytes`,
   `hex_encode_bytes`, `base64_encode_bytes`, `base64_decode_bytes`,
   `utf8_valid_bytes`, `sha256_hex`, `sha512_hex`. The *classification* of the
   builtins in the driver is fixed (§1.6) but no x86 emitter exists for any of
   them.
3. **Byte indexing `s[i]`** is classified as string-typed in the inference (so
   `str_char_at`'s literal/empty arms agree) but has **no lowering**;
   `str_to_upper`/`str_chars`/`str_trim_left` remain refused by name.
4. **String equality, slicing, `split`/`trim`/`substr`** — unimplemented.
   `std.collections` (int array `array_sum/max/min/contains/index_of`) remains
   the ONLY stdlib consumption proven end to end (exact bytes) — this is the
   NAT-STDLIB INT-ARRAY state, unchanged by the tranche.
5. **NAT-NET: not implemented.** No socket API on the native path; the Go
   native backend has none either (net is C-runtime-only today).
6. **Arena is still bump-only, compile-time bounded** — no heap allocator, no
   reclamation. Any loop that allocates a string remains unsupported by design.
7. **The arena bound for callees sharing the arena is right but coarse**:
   per-function site sums floored at 1024 each, which over-allocates
   multi-function programs. Safe direction; noted, not measured to byte size.

## 4. Host status and the environmental failure class (honest list)

This record was produced on a windows/amd64 host with **3.9 GB physical RAM**
running ~220–600 MiB free for most of the session. The documented ~4GB-class
unstable builds were observed, reproduced and quantified:

* `kcc` exits with `0xC0000005` (access violation) or `0xC0000135` (dll not
  found) on programs that build fine minutes apart — e.g. the exact `callee_len_loop`
  source built twice cleanly at ~500 MiB free and crashed under identical
  conditions later; `v1` passed once and failed once with no source change.
* Consistently failing today (consistent inside one session): **none asserted** —
  the failure set moves with memory pressure. Today's gate runs:
  - `NAT-CALL` ✓ PASS (33.7s)
  - `NAT-CALLQual` ✓ PASS (19.5s)
  - `NAT-STDLIB`: `ImportedCollectionsExecutes` and `LinuxELFIsStructurallyValid`
    both RED on `callee_len_loop` (and `sum_and_max`, flaky, passes isolated);
    subtests pass isolated consistently: `contains`, `empty_import`,
    `callee_idx_loop`, `ArgumentMismatchRefusals`, `ImportedModuleNeedsPruning`,
    `BuildsWithNoCCompiler` (PASS twice isolated).
  - The failures carry **no compiler diagnostic** — kcc dies with `0xC0000005`;
    CI is unavailable (account billing/payment restriction, all runs FAILURE).
* `kir --verify src/compiler/kir.kark` — the whole-tree KIR pin — **did not
  complete**: 900 s timeout at 494 MiB free (matches the §25 record of
  ~1.6 GB peak). KIR pin left at **15671**; `KIRPinHolds` will FAIL HONESTLY
  until measured. Do not trust any "verified" claim about it from this host.

## 5. Carried conditions status

Carried v1.2.0 conditions **2, 3, 5, 6** remain **UNMET**. Condition 2
(stdlib_v2 through the native path) is the one this milestone reaches for; it
remains open in full — the remaining blocker is §3.1/§3.2, not the string ABI.

## 6. Verdict

**PARTIAL.** The string ABI work is real, executed and reproducible on this
host; stdlib_v2 does not execute; NAT-NET is unimplemented; the Linux/macOS
branches exist in the driver (ELF is byte-identical to the oracle for the
shared corpus on this host's successful runs) but CI-level qualification is
blocked by the account billing restriction and host memory. Per the evidence
discipline this milestone is not COMPLETE, and it is not BLOCKED either —
the Windows/amd64 half of it is qualified.

**Next steps, in dependency order:**
1. Re-run the NAT-STDLIB gate on a host with >= 1.5 GiB free, including
   `callee_len_loop` (this host cannot carry it today).
2. Implement byte indexing and string equality (pure emission work on the
   existing primitives), then `str_char_at` reachable, then `str_to_upper`.
3. Implement the codecs natively (hex is trivial; base64 is ~30 instructions;
   sha256 is the largest — it needs shifts/masks, which exist).
4. Only then evaluate `examples/stdlib_v2` again; string arrays are the
   next-core representation work before `str_split` can execute.
5. NAT-NET begins after §3.1/§3.2/§3.3 — not attempted here.

---

## Appendix A — the 2026-11-15 BLOCKED record (historical, preserved)

<details>
<summary>Original 2026-11-15 text, verbatim restored for the audit trail</summary>

```
# NAT-STDLIB-STRING baseline — string tranche, honest outcome

**Verdict: BLOCKED — NOT COMPLETE.** The string ABI is inventoried and partially implemented in-tree, but
**no qualification test was run** and **the host memory capacity cannot carry a validation pass right now**.
No acceptance criterion was claimed met.

## 1. Preflight verification (2026-11-15 session)
* Tag `v1.2.0` = `e900c91` → `77d7069`, unchanged.
* Nine pre-existing untracked files preserved; nothing staged.

## 2. String analysis (measured, source-backed)
(table unchanged; key row: rodata literals collected from `main` only — callee crashes)

## 3. What was implemented (NOT qualified)
In-tree: return-kind inference to combined param/return fixpoint; string args staged as two-unit headers;
`let` of string call results bound in slots.

## 4. Why the work stops (host, not judgement)
* free RAM read **138 MiB**; kcc rebuilds ~2 GB transpile peak; repeated hard-crashes.

## 5. Exact remaining blockers (then)
1. String parameters/returns/ABI — implemented in WIP, unverified
2. String equality/slicing/byte indexing/`let name = <string call>` — not implemented
3. `trim`/`substr`/`split` builtins — not lowered
4. Hex/Base64/SHA-256 — not lowered
5. Array-of-string elements — not supported
6. Arena model redesign — required for loop-allocating strings
7. KIR re-measure after the ABI lands

## 7. Verdict (then)
**BLOCKED** — not COMPLETE, not PARTIAL (execution evidence is the entry bar).
```
</details>
