# Corrective change — map key equality on the self-hosted engine (P1)

Date: 2026-10-01. Recorded per the Governance rule (`AGENTS.md`): this is a
**corrective change against the frozen baseline**, not a new increment and not
a re-opened phase. It is **P1** of `docs/audit/KARKAIN-1.1.0-MATURITY-MATRIX.md`
§12, prescribed by its §15.4: *"make map key equality correct on the
self-hosted engine, and put an int-keyed map into the pinned corpus so the
blind spot is closed. Nothing else."*

## 1. The owning layer

`src/compiler/codegen.kark`, `emitC11ValueRuntime` — the C runtime text the
self-hosted engine (kcc) emits into every program it compiles. The affected
path is the map runtime: the `struct Value` key storage plus `map_get`,
`map_set`, `karkain_hasKey`, `karkain_delete` and `karkain_mapKeys`.

The Go engine's sibling runtime (`pkg/codegen/codegen.go`) was **never
affected**: it has stored typed `Value` keys and compared them with
`values_equal` (`codegen.go:1475-1534`, `2306-2337`). The defect was a
divergence of the self-hosted engine from the reference engine.

## 2. The demonstrated failure (measured, not inferred)

kcc stored map keys as `char** keys` and compared them with
`strcmp(m.keys[i], k.strVal ? k.strVal : "")`. An integer key has
`strVal == 0`, so every integer key was coerced to the empty string: `m[1]`
and `m[2]` answered with the **same** (last-stored) value, `len(m)`
under-counted, and `hasKey`/`delete` were equally blind. Exit code 0, with
plausible-looking output — the matrix's "silently wrong answer on the default
engine" class, in a core collection type.

Measured against the reverted (pre-fix) tree — the mutation run in §5 —
`m = { 1: "one", 2: "two" }` read back as `m[1] → "two"`, `m[2] → "two"`,
`len(m) → 1` on kcc, where the Go engine reads `one / two / 2`.

Why no gate saw it: the pinned corpus's only map fixture
(`examples/01-fundamentals/09_maps.kark`) used **string** keys — precisely
the case the broken comparison happened to get right, because a `strdup`ed
key string compares correctly with `strcmp`. Matrix P7 ("whatever else
changes, the corpus must be able to fail") is the structural half of the same
finding, and this change closes it too (§6).

## 3. The change (7 insertions / 7 deletions, 3289 → 3289 lines)

1. `struct Value` field: `char** keys;` → `struct Value** keys;`.
2. The `values_equal` forward declaration **moves** (removed from after
   `map_set`'s old position, re-added before `map_get`) because `map_get` and
   `map_set` now call it; the added statement and the removed one cancel, so
   the file and the emitted statement count are unchanged.
3. `map_get` — linear scan on `is_truthy(values_equal(*m.keys[i], k))`.
4. `map_set` — same comparison; keys are now stored as heap copies of the
   `Value` (`kc = malloc; *kc = k`) instead of `strdup(key)`, so an integer
   key survives as an integer.
5. `karkain_hasKey` — same comparison.
6. `karkain_delete` — same comparison (the array-shift was already
   representation-agnostic).
7. `karkain_mapKeys` — pushes `*m.keys[i]` (the stored key Value) instead of
   `make_string(m.keys[i])`, so key iteration yields integers for integer
   keys, matching the Go engine.

The shape of the fix is deliberately "the engine's own value equality":
keys compare exactly as values compare everywhere else in the program, which
is what the Go oracle does and what makes the two engines agree by
construction rather than by a second hand-written rule.

## 4. Downstream behavior (stated explicitly, per the Governance rule)

* **Emitted bytes change** for every kcc-compiled program: the generated C
  runtime text of the six functions and the `struct Value` field differ from
  the pre-fix emission. The generated C is compiler-internal surface, not a
  published artifact, but this is emitted-bytes drift and is recorded as
  such.
* **Program output changes only where a map has non-string keys** — from
  wrong to correct. Maps with string keys (including struct field maps, the
  highest-volume consumer of the changed functions) produce identical
  output; pinned by the new gate's `string_keys_still_work` and
  `struct_fields_are_string_keyed_maps` cases and by the full Phase 114
  corpus (§7).
* **One pinned corpus golden moves**: `09_maps.kark` gains typed-key
  coverage and is re-pinned (§6). No other corpus golden moved.
* **KIR invariant**: re-measured, never hand-edited (§7).

## 5. The gate — `pkg/cli/p1_map_key_test.go`

A **differential**, not a golden: the defect was a *disagreement* between the
two engines, so a golden pinned to either engine alone would have kept
passing while the defect lived. Every case asserts both halves:

1. kcc output == the expected lines, AND
2. kcc output == the Go reference output.

This is what makes it a parity assertion rather than a self-consistent one.
The suite also carries `TestP1_MapKeyEquality_NotVacuous`, which states the
defect as a precondition (distinct integer keys must read back distinctly),
so the gate cannot silently become a test of nothing.

Cases (8/8 PASS, 42.2 s):

| Case | What it pins |
|---|---|
| `integer_keys_do_not_collide` | the original collision: two integer keys read back distinctly, `len` correct |
| `integer_keys_set_and_overwrite` | `map_set` + overwrite through insertion |
| `hasKey_on_integer_keys` | `karkain_hasKey` typed comparison |
| `string_keys_still_work` | **preservation**: the case the old code got right |
| `delete_on_mixed_keys` | `karkain_delete` with integer and string keys in one map |
| `struct_fields_are_string_keyed_maps` | the highest-volume consumer of the changed functions |
| `NotVacuous` | the defect as a precondition |

**Mutation-verified.** The pre-fix `codegen.kark` (HEAD's version) was
restored in place, a kcc was rebuilt from it through the real stage-1
pipeline, and the gate **failed** with exactly the matrix's recorded
signature:

```
p1_map_key_test.go:182: self-hosted kcc output:
 have = "two\ntwo\n1"
 want = "one\ntwo\n2"
p1_map_key_test.go:190: engine disagreement on the same source:
   kcc = "two\ntwo\n1"
    go = "one\ntwo\n2"
```

both assertion layers firing. The fixed file was then restored
(hash-verified) and the same case passed again. A gate whose failure cannot
be reproduced is not evidence; this one's can.

## 6. The corpus fixture (closes P7's blind spot)

`examples/01-fundamentals/09_maps.kark` — the pinned corpus's map fixture —
now exercises typed keys: an integer-keyed literal with read, insert,
overwrite and `len`; `map_contains_key` on present/absent integers;
`delete` on an integer key with a visible `len` change; and `map_keys`
iteration over integer keys, enumerated element-by-element like the existing
string-key loop.

The new golden was **derived by running both engines** on the edited file and
is byte-identical:

```
92 84 97 2 1 0 3 ana bob cam http https alt 3 11 20 1 0 1 80 443 8080
```

The Phase 114 gate pins it for both engines (`phase114Examples`,
`TestPhase114_CorpusExamples_GoEngine` and
`TestPhase114_CorpusExamples_KCCParity`). Docs carried the change:
`examples/01-fundamentals/README.md` (column 9), `examples/EXAMPLES.md`
(row + expected output) and `docs/source/examples/fundamentals.rst`.

CI gains its own step — *Run P1 map-key equality gate*
(`go test ./pkg/cli/ -run 'TestP1_'`), wired in the same change per the
"a gate is a test file *plus* a CI entry" rule.
