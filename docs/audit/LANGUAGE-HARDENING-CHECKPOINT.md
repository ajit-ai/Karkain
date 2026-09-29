# Language Hardening Checkpoint (post-151C3)

*Status:* **documented, not scheduled.** No increment number is assigned to
this checkpoint. It is a named gate in the sovereignty sequence, positioned
immediately after increment **151C3** (the PE container) and **before** any
further sovereignty/self-hosting increment.

Authorised by: the engineering assessment recorded in this repository's audit
corpus, and by the numbering model in `KARKAIN-VERSION-PLAN.md` §0, which
requires that a number be assigned only when an increment is scheduled and
baselined.

**Audit performed 2026-09-28** (increment 151C3 complete). Result:
`LANGUAGE-HARDENING-CHECKPOINT-FINAL-AUDIT.md`. The audit is
**verification only** and authorised no implementation; findings are
classified there as confirmed, fixed, stale, intentional, unverified, or
requiring an architectural decision.


---

## 1. Purpose

> Before advancing deeper into sovereignty/self-hosting work, Karkain will
> perform an evidence-driven verification of the current language
> implementation, semantic enforcement, backend parity, and developer-debugging
> foundations.

The checkpoint exists for one reason: **to prevent language-correctness work
from being performed informally or outside the roadmap.**

`KARKAIN_GAP_ANALYSIS.md` is a large and useful document, but it is an
*inventory of hypotheses and observations*. Its entries are not, by themselves,
authorisation to change code. The gap analysis identifies potential problems;
the roadmap authorises work. Keeping those two responsibilities separate is the
point of this checkpoint.

## 2. Scope discipline — read this before acting

* The items in §4 are **VERIFICATION TARGETS, not confirmed defects.** Nothing
  here asserts that any behaviour is currently broken. That is precisely what
  the verification is for.
* **No item in §4 is an implementation instruction.** See §6.
* This checkpoint is **documentation and verification first.** Individual fixes
  require a subsequent, explicitly authorised implementation slice.
* The checkpoint does **not** redefine Phase 152, and does not claim a place in
  the 1.2.0 closure conditions. Whether it becomes one is a decision for the
  version plan, not for this document.

## 3. Sequence position

```text
150 ──▶ 151 ──▶ 152 ──▶ 154
          │
          └─ 151P0  (complete)
             151A-1 (complete)
             151B   Native Encoder   (complete)
             151C   ELF Container    (complete)
             151C2  Mach-O Container (complete)
             151C3  PE Container     (next)
                  │
                  └─▶ Language Hardening Checkpoint
                        (this document; verification only)
```

Per-slice completion status is recorded in `AGENTS.md`, which is the
repository's completion record, and summarised in `KARKAIN-VERSION-PLAN.md` §8.
This document does not restate completion claims; it positions the gate.

## 4. Verification targets

Each area is a question to be answered with evidence, not a task to be
completed. A finding in any area may resolve to "correct", "fixed",
"intentional limitation", "stale", or "unable to reproduce" — and all of those
are successful outcomes of the verification.

### LH-A — Error handling

Verify:

* the semantics of the `?` operator as currently implemented on both engines;
* whether `Result` / `Option` propagation is functional end to end;
* the behaviour of runtime errors (diagnostic, exit status, whether the process
  or any frame can observe them);
* whether language-level error semantics are consistent with what the generated
  code actually does.

*Evidence base:* `KARKAIN_GAP_ANALYSIS.md` entry C5 and architectural entry A3
record observations about error handling. Those are **hypotheses to re-test**,
not established current state; the entry dates and the intervening increments
matter.

### LH-B — Native semantic enforcement

Verify whether the native (C-free) compilation path applies the same relevant
checks as the other compilation paths:

* parsing and syntax diagnostics;
* semantic analysis and name resolution;
* type checking;
* ownership and borrow/safety analysis.

*Evidence base:* determine the actual call sites rather than assuming, and
record which command surfaces (`build`, `run`, `check`, `lint`) and which
targets (`c23`, `native-*`, `wasm32-wasi`) run each pass on each engine.

### LH-C — Borrow / ownership correctness

Verify the current state of reported findings concerning:

* borrow lifetime and whether borrows expire;
* move tracking and whether moves are recorded and released;
* whether borrow diagnostics carry usable line/column information;
* how much of the documented ownership model is actually enforced, and on which
  backends.

*Evidence base:* `KARKAIN_GAP_ANALYSIS.md` entry C4. Do not assume historical
audit findings are still current — several are old, and some may have been
addressed by later work without the entry being updated.

### LH-D — Code-generation correctness

Determine the current status of previously reported BUG-* findings in the
struct, match, enum, index and typed-declaration code-generation paths.

Record each as exactly one of:

* **confirmed current defect** — reproduced today, with a minimal reproducer;
* **fixed** — no longer reproduces, with the gate or commit that fixed it;
* **historical / stale** — the finding refers to behaviour that no longer
  exists, or to a path that was replaced;
* **intentional limitation** — current behaviour, accepted by design, and it
  should be documented as a limitation rather than a bug;
* **unable to reproduce** — no reproducer found, recorded as unknown rather
  than as either a pass or a failure.

*This area must not be closed by assuming a BUG id still means an open defect.
An id in a list is a claim; a reproducer is evidence.*

### LH-E — Enum payload semantics

Verify whether enum constructor payloads are:

* preserved at construction;
* accessible through pattern matching;
* represented correctly and consistently in each backend.

*Evidence base:* `SPEC.md` §13 currently records payload handling as a known
gap. Re-test that statement rather than inheriting it.

### LH-F — Cross-backend semantic parity

The requirement is that **equivalent Karkain source programs have equivalent
observable semantics across supported backends** — same results, same exit
codes, same user-visible errors.

It is explicitly **not** a requirement that backends produce identical machine
code or identical internal representations. The native backend's frame-slot
model, the C backend's tagged values and the WASM backend's boxed cells are
different by design, and the existing byte-identity work between the Go and
self-hosted *front ends* is a separate concern from parity *across backends*.

Record, per feature, which backends are semantically equivalent and which are
compile-only, backend-specific, or unverified.

### LH-G — Whole-language capability ledger

Define the need for a single authoritative capability matrix, and produce it if
authorised. It should cover, where applicable:

```text
Feature | Parser | Semantic analysis | Ownership/borrow | C backend |
Native backend | WASM backend | Runtime | Tests/gates | Status |
Known limitations
```

The purpose is to make these states distinguishable, and currently they are not:

* implemented
* partially implemented
* parser-only
* backend-specific
* compile-only
* runtime-supported
* deferred
* known defect

*Evidence base:* the existing `docs/source/status/feature-matrix.rst` covers
generics v1 only. This area is about the whole language, and about which
backends enforce each row.

## 5. Developer Experience / Debugging

Debugging is tracked as a **separate, future track**, not as part of this
checkpoint and not as a language-hardening fix. See
`DEVELOPER-EXPERIENCE-DEBUGGING-TRACK.md` for DX-A through DX-D.

It is named here only so that the relationship is explicit: DX work depends on
the debug-information inventory (DX-A), and the semantic decisions taken in
LH-B and LH-C affect what a debugger can meaningfully show.

## 6. What the verification authorises

The checkpoint authorises **verification only**. Implementation work will be
authorised only after the evidence audit has determined, for each area:

1. which findings are still valid;
2. which are already fixed;
3. which are intentional limitations;
4. which require an architectural decision before they can even be specified;
5. which should become implementation slices.

Only then may new increments be proposed, and each will need its own baseline
note before work starts, per the numbering model.

**Do not derive implementation phases from this document.** No phase numbers
are invented here, and none should be inferred from it. `KARKAIN_GAP_ANALYSIS.md`
findings are not implementation authorisation.

## 7. Implementation authority (added 2026-09-28, after the audit)

> **A finding in the audit does not automatically authorise implementation.**
> The audit classifies and evidences; it does not authorise.

The audit (`LANGUAGE-HARDENING-CHECKPOINT-FINAL-AUDIT.md`) confirmed two
current kcc/backend parity defects. Those two — and only those two — are
authorised for implementation:

| Slice | Confirmed defect | Status |
|---|---|---|
| **LH-1** | kcc `?` propagation parity. The operator was parsed and then lowered to nothing. The Go engine propagates correctly. | **COMPLETE** |
| **LH-2** | kcc `match` binding parity. Match-arm bindings are rejected on the self-hosted engine with `error[K102] undefined identifier`, so the idiomatic way to consume a `Result` is unavailable there while it works on the Go engine. | **AUTHORIZED / NOT STARTED** |

Both are **implementation slices under this checkpoint, not new numbered
phases.** They carry no number, do not redefine any phase, and do not alter the
`1.2.0` closure conditions.

### LH-1 closure (implementation `5dcdd80`)

The audit's description of LH-1 was **partly inaccurate**, and the correction
matters for anyone re-deriving the fix:

- The construct is **not** carried by `NODE_PROPAGATE`. kcc's parser lowers
  postfix `?` to a `UnaryExpr` whose operator is `?`, so `NODE_PROPAGATE` (57)
  and its `Propagate` type name are **dead on the lowering path**. That is why
  searching `codegen.kark` for a `NODE_PROPAGATE` case finds nothing, and why
  adding one there would have changed nothing. The lowering belongs in the
  `UnaryExpr` branch.
- A **second, previously unrecorded** defect was the actual blocker: kcc had
  no *expression*-level `Ok`/`Err`/`Some`/`None`. Those four tokens were
  consumed only in `parseMatchExpr`'s arm-**pattern** position, so in
  expression position the constructor was dropped and only the parenthesised
  payload survived. A kcc-compiled program therefore could not build a
  `Result` or `Option` value at all, leaving `?` with nothing to inspect or
  propagate. This was verified with a program containing **no `?` anywhere**,
  where `return Ok(5)` produced `return mk_nil()` plus a dead trailing
  statement.

Evidence:

- Focused local gate: **11/11 PASS** (`TestLH1_`), covering successful
  propagation, error propagation, three-boundary nesting, `Option` `Some`/`None`,
  and `?` in call-argument, nested and arithmetic positions, on both engines.
- CI: `CI/CD` run **36502124716**, job **Test**, step *Run LH-1 kcc propagation
  parity gate* — **success**.
- The CI step is **non-vacuous**: it reports `ok karkain/pkg/cli` with **no**
  `[no tests to run]` marker, and a deliberately non-matching pattern is
  confirmed to print that marker.
- Mutation-verified: disabling the `?` lowering makes four independent tests
  fail and reproduces the original diagnostic verbatim.
- The whole-tree KIR pin moved 10530 → 10611, with `kir verify` passing.

**Not yet authorised.** Every other audit finding remains evidence only,
including: the absence of a whole-program type-checking pass on any backend;
borrow diagnostics discarding a line number at the render boundary; enum
payloads being discarded; the three-way memory model; the absence of a
security or capability model; and every item in the Developer Experience /
Debugging track. A runtime exception system (`try` / `catch` / `throw` /
`panic`) is **not** authorised and is a separate architectural decision.

Implementation proceeds **one slice at a time**: LH-2 is not to be started
together with LH-1 unless that is explicitly requested.

