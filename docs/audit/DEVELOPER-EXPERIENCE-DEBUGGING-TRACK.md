# Developer Experience / Debugging Track

*Status:* **future work, documented not scheduled.** No increment number is
assigned. This is a separate track from the Language Hardening Checkpoint
(`LANGUAGE-HARDENING-CHECKPOINT.md`) and is deliberately **not** folded into
it: debugging is tooling and architecture work, not language correctness.

Nothing in this document is implemented, scheduled, or committed to by its
presence here.

---

## 1. Purpose

Karkain currently has a debugging story that is real but narrow. This track
exists to define what the full story should be, and to record the areas that
must be investigated before it can be designed.

Two constraints shape everything below:

* **Do not over-claim.** The existing `karkain dbg` command is a real feature
  with real limits. This document does not replace it and does not disparage it;
  it records what is not yet covered.
* **The vocabulary and the architecture are separate questions.** A debug
  adapter protocol server is an integration decision; whether the compiler
  emits the information such a server needs is a compiler question. DX-C depends
  on DX-A.

## 2. What exists today (recorded, not evaluated)

For accuracy, the current state includes at least:

* a `dbg` command that runs a compiled program under an external debugger in a
  batch/batch-file mode and renders a Karkain-level backtrace;
* a DWARF-emitting path and a self-hosted DWARF reader, with readelf-style text
  output;
* the backtrace format carrying file and line information.

The authoritative detail is in the increment-104/112/140 records in
`AGENTS.md` and in `docs/native-codegen.md`. This section deliberately does not
restate specific limits, because doing so from memory is exactly how
documentation drifts. **DX-A is the first task precisely so that the inventory
is measured rather than remembered.**

## 3. Investigation areas

### DX-A — Debug information inventory

**Determine what source-location and debug information Karkain currently
generates**, per backend and per engine.

Deliverable: an inventory, not an implementation. It should record, for each
backend, which of the following are produced and at what fidelity:

* function/section boundaries and their source mapping;
* line-number tables;
* variable and type information;
* inlined-frame information;
* whether the information survives each container writer (ELF, PE, Mach-O);
* whether the self-hosted engine's output is equivalent to the Go engine's.

This is the input to every other area here. A debugger cannot show what the
compiler does not emit.

### DX-B — Debug model

**Define the required mapping** through the pipeline:

```text
Karkain source
      ↓
KIR
      ↓
generated code
      ↓
executable
```

and specify the source-level experience that must follow from it:

* breakpoints — by source line, and how they map onto machine-level sites;
* stack frames — which frames are visible and how they are named;
* variables — which values can be inspected, and at what fidelity;
* source locations — how a runtime address resolves back to file and line;
* stepping — statement-level, line-level, and frame-level expectations.

This is a design document. It requires architectural decisions, and it should
state which decisions are open.

### DX-C — DAP architecture

**Record the Debug Adapter Protocol as the intended standard integration
direction**, conditional on the architecture supporting it.

The objective is eventually to allow Karkain debugging through standard IDE and
debug clients, rather than requiring users to work through the current
command-line workflow or a bespoke extension command.

This area should establish:

* whether a DAP server is the right seam, given DX-B's model;
* what the adapter fronts — an external debugger, an in-process runtime, or a
  hybrid;
* how it interacts with the self-hosted engine, whose debugging story may
  differ from the Go engine's;
* what the migration path is for the existing `dbg` workflow, and whether it is
  retained.

**No DAP server exists in the repository today.** This is recorded as intended
direction, not as implemented functionality.

### DX-D — Native debugging

**Document the requirements for debugging natively produced Karkain
executables.**

Native images differ from the C-transpiled path in ways that matter to a
debugger: container and entry-point conventions, symbol and relocation
handling, and a different set of backends per host OS.

This area should record:

* which information a native image carries, per container, and what is missing;
* what a debugger needs from the backend for breakpoints and stepping to work;
* per-OS realities, including which platforms have any execution host at all;
* the honest position on platforms where no execution proof exists, so that
  structural validation and executed validation are not conflated.

**Do not claim this is implemented.** For the native path this is the least
mature area, and this document is where that fact belongs.

## 4. Relationship to the Language Hardening Checkpoint

The two tracks are independent but related:

* the checkpoint (LH-A to LH-G) is about whether the language is **correct and
  consistently enforced**;
* this track is about whether a developer can **observe** what the compiler
  produced.

They share one dependency direction only: **LH-B and LH-C affect DX-B.** If the
native path does not run the same semantic and borrow passes as the C path, then
what a debugger can meaningfully show differs per backend, and DX-B has to
describe that difference rather than assume one uniform model.

That is the whole of the coupling. Neither track blocks the other from being
investigated, and neither is authorised to start implementing.

## 5. Authorisation

As with the checkpoint: **this document authorises investigation only.** No
increment number is assigned, no phase is redefined, and no implementation work
is implied. Proposing implementation slices requires an explicit decision, and
each proposed slice would need its own baseline note under the numbering model
in `KARKAIN-VERSION-PLAN.md` §0.
