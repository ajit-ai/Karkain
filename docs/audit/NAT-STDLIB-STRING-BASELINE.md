# NAT-STDLIB-STRING baseline — string tranche, honest outcome

**Milestone:** `NAT-STDLIB-STRING` (v1.3.0 continuation of NAT-STDLIB).
**Version:** 1.3.0 (NOT open; `VERSION` unchanged; no release claim follows).
**Branch:** `feature/v1.3.0-natstdlib-string` (created from verified `feature/v1.3.0-natstdlib` at `199ee87`).
**Verdict: BLOCKED — NOT COMPLETE.** The string ABI is inventoried and partially implemented in-tree, but
**no qualification test was run** and **the host memory capacity cannot carry a validation pass right now**.
No acceptance criterion was claimed met.

---

## 1. Preflight verification (this session)

* `feature/v1.3.0-natcall` = `aa45970`, `feature/v1.3.0-natstdlib` = `199ee87` — matched the brief; no divergence.
* Tag `v1.2.0` = `e900c91` → `77d7069`, unchanged.
* Nine pre-existing untracked files preserved; nothing staged.
* `docs/audit/NAT-STDLIB-BASELINE.md`, v1.3.0 checklist, current native driver source inspected.
* CI: the three runs created on 2026-11-14 (`43556707796`, `43556713438`, `43556713444`) are all
  `failure` — the billing/payment restriction is still active; no CI execution of any new gate is expected.

## 2. String analysis (measured, source-backed)

| Concern | Measured state |
|---|---|
| Representation | `(ptr, len)` pair in two adjacent 8-byte slots (Phase 148 ABI); string params home 2 units (`natNativeHomeParams` is kind-aware already) |
| Literal storage | `.rodata` emitted once by interning; **the rodata offset list is collected from `main`'s body only** — a callee that binds or prints a string literal has no rodata slots and crashes (`natEmitStrValue` indexes off the end) — recorded repeatedly in the driver's own history |
| Frame layout | kind-aware string slots are already supported (`natKindString`, 16 bytes); `natNativePlanDecls`, `natNativePlanBytes` handle them |
| Allocation | heap **arena**, bump-only, **compile-time bounded** (`natNativeArenaBytes`): every allocation site must execute at most once. **Loops that allocate are rejected** — `str_to_upper`/`str_repeat`/any loop `+` cannot be supported by this memory model |
| Ownership | none at language level; the driver owns lifetime via the arena, no reclamation |
| `import std.string` | the whole module is assembled and scanned; unreachable unsupported callees are emit-time skipped (landed in NAT-STDLIB); `str_to_upper` IS a string-user, so it is reachable for the acceptance example |

**Consequences for the acceptance target:** `examples/stdlib_v2` requires `str_trim` (C builtin, not lowered),
`str_to_upper` (string runtime + byte indexing + equality), `str_split` (array of strings, not supported),
`str_concat` chain, `hex_decode`/`base64_decode`/`sha256` (codecs, not lowered). The string tranche alone
closes a minority of those blockers; the codecs and the string-array representation are separate trenches.

## 3. What was implemented this session (NOT qualified)

In the working tree the native driver gained: a return-kind inference (`int`/`string`) pass over the call graph
(exceptions reported BY NAME); string arguments at call sites are staged like two-unit headers; a combined
parameter/return-kind fixpoint replaces the parameter-only pass; string call results bind into `let` slots.

* The self-hosted compiler **rebuilt and linked `kcc.exe`** with these changes and emitted a valid image for
  a trivial program (`coll_empty.kark` → 4608 bytes). That is NOT qualification evidence — it is only the
  compiler accepting its own sources.
* **No gate, probe or acceptance test was executed.** NAT-CALL/NAT-STDLIB regression gates were NOT re-run.
* KIR pin was NOT re-measured (would require a ~1.6 GB-peak `kir --verify` run on this host).
* The string-ABI work-in-progress was preserved at `C:\Windows\Temp\opencode\native_value-string-wip.kark`
  (one file) rather than committed, because unverified work does not belong on the branch.

## 4. Why the work stops here (host, not engineering judgement)

This host (windows/amd64, 3.9 GB physical) repeatedly collapses under its own memory limits during this milestone:

* `exit 1073807364` / `exit 3221225794` — Get-Content / PS host hard-crashes in this exact session.
* `kcc.exe` rebuilds take the transpile peak (~2 GB) when they fit at all; the last one succeeded, several others did not.
* free physical RAM read **138 MiB** after the last crash — less than a single `go build ./cmd/karkain` peak.

The milestone demands execution evidence. Producing bytes that were never executed would violate the evidence
discipline that governs every other claim in this repository.

## 5. Exact remaining blockers, in dependency order

1. String parameters/returns/ABI — implemented in WIP, **unverified** (needs a >=1.5 GiB-free host or a runner).
2. String equality/slicing/byte indexing/`let name = <string call>` — not implemented; required by `str_to_upper`.
3. `trim`/`substr`/`split` builtins — not lowered on native (C builtins; would need native helpers).
4. Hex/Base64/SHA-256 codecs — not lowered; no path exists today.
5. Array-of-string elements — not supported at all; required by `parts := str_split(...)`.
6. Arena model redesign or a dynamic runtime allocator — required for ANY loop that allocates strings.
7. KIR re-measure after the ABI work lands (`kir --verify`, ~25 minutes, ~1.6 GB peak).

## 6. Carried conditions status

Carried v1.2.0 conditions **2, 3, 5, 6** remain **UNMET** and unchanged. Condition 2 is the one this milestone
reaches for; it remains open in full.

## 7. Verdict

**BLOCKED** — the string tranche is not qualified and cannot honestly be called COMPLETE or even PARTIAL
(execution evidence is the entry bar for PARTIAL). NAT-STDLIB remains at its prior status (INT-ARRAY slice, PARTIAL).

**Recommended next milestone:** re-approach the string tranche on a host (or runner) with >=1.5 GiB free physical,
starting from the preserved work-in-progress, first by validating the ABI inference with the NAT-CALL/NAT-STDLIB
regression gates, then by a string-focused gate, then by the acceptance path. Codec work must follow, not precede,
the validated ABI. NAT-NET remains blocked behind NAT-STDLIB regardless.
