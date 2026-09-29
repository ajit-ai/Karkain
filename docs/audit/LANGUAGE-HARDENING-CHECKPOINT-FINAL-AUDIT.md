# Language Hardening Checkpoint — Final Evidence Audit

*Checkpoint:* post-151C3 Language Hardening Checkpoint, defined in
`LANGUAGE-HARDENING-CHECKPOINT.md` (areas LH-A to LH-G).
*Date:* 2026-09-28. *Repository:* `develop` @ `266695c`.

---

## 1. Scope

This was an **evidence-only audit**. No compiler code, `.kark` source, Go
source, test, or CI file was modified, and no bug was fixed. The only file
written is this report.

Every finding below is classified with one of:

```text
CONFIRMED CURRENT | FIXED | HISTORICAL / STALE
INTENTIONAL LIMITATION | UNABLE TO VERIFY | ARCHITECTURAL DECISION REQUIRED
```

Method: read the current source, trace real call sites, and reproduce where
practical. **No defect was inferred from a prior document saying one existed.**
That rule mattered — the single largest outcome of this audit is that several
long-standing audit claims are no longer true, and one widely-repeated claim of
mine was wrong (LH-B).

## 2. Repository state

| | |
|---|---|
| Branch | `develop` |
| HEAD | `266695c` — `feat: implement self-hosted PE container for 151C3` |
| Working tree | clean except `docs/source/development/index.rst`, the pre-existing user change, preserved and never staged |
| Increments complete | 150A-150D, 151D-first, 151A-1, 151P0, 151B, 151C, 151C2, 151C3 |

## 3. Error model (LH-A)

### The `?` operator — NOT a no-op, on the reference engine

`KARKAIN_GAP_ANALYSIS.md` C5 states *"`?` is a no-op (BUG-4)"*. That claim is
**wrong for the Go engine today**.

Full trace: `TokenQuestion` (lexer.go:107) -> `&PropagateExpr{Operand: left}`
(parser.go:1372-1375) -> `codegen.go:4053-4058`, which emits real C
propagation: a `Result` with `tag == 1` or an `Option` with `tag == 0` causes
an early `return`, otherwise the payload is unwrapped.

**Reproduction** (`outer` calls `inner(x)?`):

```text
$ KARKAIN_ENGINE=go karkain run q2.kark
Ok:11
Err:negative
```

`inner(5)` returns `Ok(10)`; `?` unwraps to 10, `+1` gives `Ok:11`.
`inner(-3)` returns `Err("negative")`; `?` propagates it. Both correct.
`TestBugFix_OptionPropagation` in `pkg/cli/bugfix_e2e_test.go` also passes and
covers BUG-4.

**Classification: HISTORICAL / STALE** (the "no-op" claim).

### `?` on the self-hosted engine — a real parity defect

`?` **parses** on kcc: `TK_QUESTION` exists (lexer.kark:69, scanned at 580),
`NODE_PROPAGATE` exists (ast.kark), the parser builds it (parser.kark:1502),
and `tokenName` reports `"Propagate"`.

But `src/compiler/codegen.kark` contains **no case for `Propagate`** (grep for
`Propagate` returns nothing). The expression therefore lowers to nothing.

**Reproduction:**

```text
$ KARKAIN_ENGINE=kcc karkain run q3.kark
gcc link failed: exit status 1
q3.c23:348:1: error: expected expression before ';' token
  348 | ;{ Value _karkain_fret = mk_nil()  ; karkain_frame_leave(); return _karkain_fret; }
```

**Classification: CONFIRMED CURRENT.** The failure is **loud** (a build
error), not a silent wrong answer: an empty expression statement precedes the
return wrapper.

### Runtime failures are fatal and unobservable

There is **no** `try`, `catch`, `throw`, `panic` or `recover` anywhere in the
parser. Runtime failures route to `karkain_runtime_error`, which prints a
diagnostic plus a frame stack and calls `exit(1)`.

**Reproduction** (identical on both engines):

```text
$ karkain run rt.kark
before
runtime error: integer division by zero at rt.kark:4
  stack:
    main (rt.kark:4)
exit=1
```

`before` printed, then the process died. `Result` cannot observe this: the
process is already gone.

**Classification: CONFIRMED CURRENT** — the error model is **partially
functional**. `Result`/`Option` and `?` work for *explicit* fallible returns;
*implicit* runtime failures (div-by-zero, index out of range, invalid call,
and by extension I/O, DB and network failures raised in the runtime) are fatal
and uncatchable.

### A separate parity divergence found while testing this

`match` arm bindings (`Ok(n) => ...`) are rejected by kcc with
`error[K102] undefined identifier 'n'`, while the same program is accepted and
correct on the Go engine. This is a `match` defect, not a `?` defect, but it
means the idiomatic way of *consuming* a `Result` is currently unavailable on
the self-hosted engine.

**Classification: CONFIRMED CURRENT** (LH-F relevant).
## 4. Native semantic enforcement (LH-B)

**This corrects a finding I published earlier.** I previously reported that
the C-free native path applies no semantic or borrow checking. That was based
on grepping `pkg/cli/cfree_target.go` alone and was **wrong**: the checks run
in the *caller*, before the target dispatch.

Traced call sites in `pkg/cli/commands.go`:

| Pass | `RunCommand` | `BuildCommand` | Native dispatch |
|---|---|---|---|
| parse + syntax diagnostics | 55 | 145 | — |
| borrow check (`runBorrowCheck`) | **61** | **150** | — |
| semantic preflight (`runSemanticPreflight`) | **72** | **159** | — |
| native dispatch (`IsNativeTarget`) | 88 | 174 | — |

Both checks execute **before** the native branch is taken.

**Reproduction** — the same source, two targets:

| source defect | `--target c23` | `--target native-x86_64-windows` |
|---|---|---|
| `print(undefinedName)` | `error[K002] undefined identifier` exit 3 | `error[K002] undefined identifier` exit 3 |
| move-then-use (`let y = move(x); print(x)`) | — | `Borrow check failed: use of moved value: 'x'` exit 3 |

| Check | C path | Native path | WASM path | Evidence |
|---|---|---|---|---|
| Parsing | yes | **yes** | yes | shared `parseSourceWithErrors` |
| Syntax diagnostics | yes | **yes** | yes | commands.go:55 / 145, before dispatch |
| Semantic preflight (K002 etc.) | yes | **yes** | yes | commands.go:72 / 159; reproduced |
| Borrow / ownership | yes | **yes** | yes | commands.go:61 / 150; reproduced |
| Whole-program type checking | not present as a distinct pass | not present | not present | `pkg/sema` has `resolve.go` + domain checkers but no single whole-program type checker (see §14) |

**Classification: my earlier claim — HISTORICAL / STALE (and wrong).**
The "native path skips safety checks" concern does not hold. The real LH-B
finding is narrower: **no backend has a whole-program type-checking pass**, so
the residual gap is language-wide, not native-specific.

## 5. Borrow / ownership (LH-C)

### BUG-5, "moves never expire"

`KARKAIN_GAP_ANALYSIS.md` C4 cites `borrow_checker.go:74-85` as evidence of a
move leak. That region today is `scope.lookupOwn` and the `BorrowChecker`
struct — it does not contain the claimed code. The line reference is
**stale**.

The scenario itself has a dedicated regression test, which passes:

```text
--- PASS: TestPhase51_Bug5DoubleMoveAcrossScopes
--- PASS: TestPhase51_MovePersistsAcrossScope
```

All 31 borrow/ownership tests pass, including move, double-move, borrow
aliasing, borrow expiry, and shadowing groups.

**Classification: FIXED (or stale).** The cited evidence no longer exists and
the scenario's test is green. This audit did not attempt to construct a
fresh move-leak case, so "fixed" is asserted on the strength of the existing
regression coverage plus the moved code.

### `BorrowError.Line` always 0

Half true, and the true half matters more than the false half.

* **False:** errors are constructed *with* a line, e.g.
  `BorrowError{Message: ..., Line: entry.line}` (borrow_checker.go:211-214),
  and 12 construction sites populate `Line`.
* **Confirmed:** the CLI render path discards it. Every render site does
  `msg += "  " + e.Message + "\n"` and never `e.Line` (commands.go, six
  call sites of `runBorrowCheck`).

**Reproduction:**

```text
$ karkain build mv.kark --target native-x86_64-windows
Borrow check failed:
  use of moved value: 'x'
```

The violation is on line 4; the message carries no line.

**Classification: CONFIRMED CURRENT** — the information is computed and then
thrown away at the display boundary.

## 6. Code generation (LH-D)

`ROADMAP.md` carries a status table (lines 428-438) recording **BUG-1, BUG-2,
BUG-3, BUG-7 and BUG-8 as FIXED**, "audited in PHASE 79", with regression
coverage in `pkg/cli/bugfix_e2e_test.go`. `KARKAIN_GAP_ANALYSIS.md` C6 says
the same five are "all OPEN". The two documents contradict each other.

Decisive evidence — the cited regression file exists and passes:

```text
--- PASS: TestBugFix_OptionPropagation   (BUG-4 / `?`)
--- PASS: TestBugFix_OptionUnwrap        (BUG-2 / Option)
--- PASS: TestBugFix_EnumPayloadConstructor (BUG-7)
--- PASS: TestBugFix_TypedColonDecl      (BUG-8)
ok  	karkain/pkg/cli	11.909s
```

| Bug | Construct | Current status | Evidence |
|---|---|---|---|
| BUG-1 | Struct codegen | **FIXED** | ROADMAP table; no named regression test found for structs specifically |
| BUG-2 | Option/Result match arms | **FIXED** | `TestBugFix_OptionPropagation`, `TestBugFix_OptionUnwrap` pass |
| BUG-3 | Index assignment | **FIXED** per ROADMAP | **UNABLE TO VERIFY**: no dedicated regression test located in this pass |
| BUG-7 | Enum payloads emitted undefined `_make_` | **FIXED as to the crash** | `TestBugFix_EnumPayloadConstructor` passes; the constructor is now defined. See §7 for what it does with the payload |
| BUG-8 | Typed declarations mixed representations | **FIXED** | `TestBugFix_TypedColonDecl` passes |

**Classification: HISTORICAL / STALE** for C6 as a whole ("all OPEN"). BUG-3
is **UNABLE TO VERIFY** — its evidence is a documentation row only.

## 7. Enum payloads (LH-E)

Payloads are **accepted, type-checked, and then explicitly discarded** — on
both engines.

Go engine, `pkg/codegen/codegen.go:4264-4267`:

```c
Value Shape_make_Circle(Value payload) {
    (void)payload;
    return make_int(1);
}
```

Self-hosted engine, `src/compiler/codegen.kark:2283`, byte-for-byte the same
shape, described in its own comment as "Phase 123 ... `(void)payload; return
make_int(i+1);`".

So a payload variant:

* **is** stored at the syntax and AST level (the `Name:payload` entry is
  parsed, and a `_make_` constructor is emitted and callable);
* **is not** preserved in the runtime value — the returned `Value` is an
  integer tag;
* **cannot** be recovered by pattern matching, because the payload is gone
  before any match runs;
* is **consistent across backends** — both discard it identically.

`SPEC.md` §13 records this as a Phase 123 gap: "Payloads are recorded but
semantically dropped at construction (the tag is observable); payload
destructuring in match patterns is not part of the language."

**Classification: INTENTIONAL LIMITATION, documented — and CONFIRMED CURRENT
as a semantic gap.** It is a deliberate Phase 123 decision, stated in the spec
and mirrored consistently in both backends. It is *not* a crash and *not* a
divergence; a user who writes a payload variant gets a value that silently
carries only its tag.

## 8. Backend parity (LH-F)

| Aspect | Status | Evidence |
|---|---|---|
| Byte-identity Go vs kcc, encoder + 3 containers | **proven** | 151B/151C/151C2/151C3 gates, all wired into CI |
| Corpus parity Go vs kcc (61 programs) | **proven in CI** | `TestPhase114_CorpusExamples_GoEngine` / `_KCCParity` |
| `?` propagation | **DIVERGENT** | Go functional; kcc parses but does not lower it (§3) |
| `match` arm bindings | **DIVERGENT** | Go correct; kcc `error[K102]` (§3) |
| Runtime errors | **parity** | identical diagnostic + exit 1 on both engines (§3) |
| Div-by-zero / OOB / invalid call | **parity** | Phase 100 gates; reproduced for div-by-zero here |
| Native (C-free) target semantics | **Go-only** | the native backend is `pkg/native`; kcc native parity is 151A/151D, not yet routed |
| Mach-O | **structural only** | no Intel-mac runner exists; structural validation only since increment 149 |
| WASM | **parity + executable** | Phase 108 gates; wasmtime is installed on this host and CI installs it |
| Enum payload semantics | **parity (both discard)** | §7 |

## 9. Whole-language capability ledger (LH-G)

`docs/source/status/feature-matrix.rst` exists and is exemplary in structure,
but its title and content are **generics-v1 only**: "Generics v1 — Feature
Matrix", 13 rows, all about Phase 146. Its own header says it is the
"per-feature ledger for the Phase 146 generics track".

So there is **no whole-language ledger today**. What a whole-language ledger
would need, and where each column already has a home:

| Column | Exists today? | Source |
|---|---|---|
| Feature | partially | feature-matrix.rst (generics only) |
| Syntax / Parser | no | `SPEC.md` §13 has a partial known-gaps table |
| Sema / type checking | no | `pkg/sema` checkers exist but are not enumerated per feature |
| Ownership / borrow | no | `SPEC.md` §5 describes intent, not per-feature enforcement |
| C backend | partially | `SPEC.md` §14 capability summary (Y/N per feature, both engines) |
| Native backend | **no** | nothing maps features to `pkg/native` support |
| WASM backend | **no** | `pkg/wasm/backend.go` `scanUnsupported` knows the boundary in code, not in a ledger |
| Runtime | no | — |
| Tests / gates | partially | gates exist and are named per phase, but not indexed per feature |
| Status vocabulary | no | the repo needs the 8 states named in the checkpoint (implemented / partial / parser-only / backend-specific / compile-only / runtime-supported / deferred / known defect) |
| Known limitations | partially | `SPEC.md` §13, `KARKAIN_GAP_ANALYSIS.md` |

**Classification: ARCHITECTURAL DECISION REQUIRED.** The information largely
exists in code and in per-phase reports; what is missing is a single index
with a controlled vocabulary. Building it is a documentation decision, not a
code change.

## 10. Debugging (DX-A to DX-D)

Searched the whole repository for DAP and debugger implementation.

| Capability | Current status | Evidence |
|---|---|---|
| `karkain dbg` command | **exists** | `pkg/cli/dbg.go`; 4 tests in `phase140_debug_test.go` |
| Debugger used | **gdb, batch mode only** | dbg.go:20-21, 28 |
| lldb | **unwired on this host** | dbg.go:30-31 ("same batch shape, unwired on this host") |
| Interactive operation | **no** | batch file, `rbreak` regex + `bt`/`continue` |
| Breakpoints | **regex over symbol names only** (`karkain_user_*`), capped at `dbgMaxSteps = 10` | dbg.go:21, 33-36 |
| Stepping | **no** — the walk reports, it does not drive | dbg.go:30-31 |
| Stack inspection | **partial** — backtrace with file:line, renames the namespace | dbg.go, `karkain_dbg trace:` |
| Variable inspection | **no** | no expression evaluation anywhere in dbg.go |
| Expression evaluation | **no** | — |
| Source mapping | **partial** | DWARF *is* generated (`pkg/codegen/dwarf.go`: `NewDwarfEmitter`, `Emit`, `.debug_info`/`.debug_abbrev`/`.debug_line`) with a self-hosted reader `dwarf_parse.go` |
| Native images | **not debuggable via dbg** | documented boundary: stays on the C path |
| kcc engine | **not trace-aware** | dbg.go:19-20 ("not yet trace-aware, the same explicit boundary as Phase 112 tracing") |
| **DAP** | **absent** | no `DebugAdapter`, no `adapterCapabilities` anywhere; the only `dap` substring hits are NPU "adapter" in `npu_compiler.go` |
| VS Code integration | commands only | check/compile/run/format + a debug command; no DAP client |
| Missing gdb | clean `ExitEnv` with an install hint, never a fake trace | dbg.go:28 |

**Minimum architectural prerequisites for a future DAP server** (recorded, not
scheduled — no implementation is proposed here):

1. A debug-information inventory (DX-A): which backends emit what, and
   whether the self-hosted engine's output is equivalent to the Go engine's.
2. A defined debug model (DX-B): how a Karkain frame maps to KIR, to generated
   code, to an image, and from an address back to source.
3. A decision on the adapter seam: front an external debugger, or instrument
   the runtime — different answers give different architectures.
4. Per-backend coverage, because the answers differ: DWARF exists for the C
   path, kcc is not trace-aware, and native images are not on this path at all.

**Classification: CONFIRMED CURRENT.** Debugging is batch-trace only, and
DAP does not exist.

## 11. Feedback latency

| Observation | Evidence |
|---|---|
| KIR pin must be re-measured on every `src/compiler` change | The pin moved 9574 -> 9642 -> 9889 -> 10031 -> 10264 -> **10530** across increments 151P0, 151B, 151C, 151C2, 151C3 in this session alone; each move is a gate failure that must be re-measured and annotated |
| `kcc` self-build is minutes on this host | Repeated `karkain build src/compiler/main.kark --target c23` invocations exceed the interactive tool timeout and had to be re-run |
| Broad gates exceed Go's default timeout | Phase 114's two legs in one invocation `panic: test timed out after 10m0s`; each leg passes alone (GoEngine 452s, KCCParity 568s) |
| Host memory limits the combined run | `TestPhase107_CodegenSpawnJoin` failed in a combined `pkg/codegen` run and passed in isolation at 3.8s — the documented ~4 GB-host class |
| Focused execution is available and used | This audit ran only targeted tests (`-run` patterns) plus a small `pkg/sema`/`pkg/cli` selection, per §16 |
| Bootstrap stage 2/3 cannot run here | `error[K127]` guard, ~4 GB host; recorded in `PHASE-127`/`129` records |

**Classification: CONFIRMED CURRENT** — the development feedback problem is
real and is documented in the increment records rather than inferred.

## 12. Memory model

`SPEC.md` §5 states one intent: "safe by default, unsafe with `@raw`", with
ownership, move semantics, a borrow checker, and raw access. What the backends
actually do differs:

| Backend | Allocation | Ownership enforcement | Free |
|---|---|---|---|
| C | tagged `Value`, `malloc` via the runtime | borrow checker at CLI level | yes |
| Native (C-free) | frame slots + **bump arena**, no GC | borrow checker at CLI level | **no** — bump-only |
| WASM | boxed cells, a linear-memory heap | borrow checker at CLI level | no GC |

There is therefore **one enforcement layer and three allocation strategies**.

**Classification: BACKEND-SPECIFIC.** The documentation describes one coherent
model; the tree implements three allocation strategies behind it. No
recommendation is made here.

## 13. Security / ecosystem / WASM

**Security.** No capability, effect or sandbox layer exists in the language or
the toolchain. Authority comes from what the program calls: `@raw` reads and
writes arbitrary addresses by design, `import "C"` is arbitrary FFI, and
`stdlib/io`, `stdlib/db` and `stdlib/net` perform real filesystem, database and
socket I/O. A search of `SPEC.md` for "sandbox / capability / permission"
returns only version tables and the unrelated "Capability summary" section
heading. `KARKAIN_GAP_ANALYSIS.md` A6 lists capability-based security as
"undecided ... don't build yet".

**Ecosystem.** The registry is a local-directory implementation (Phase 135);
Phase 135 records that http is explicitly refused. There are no third-party
packages in the tree.

**WASM.** `SPEC.md` §13 records wasm32-wasi as "Compile-only, no runtime
execution". That row is **stale**: `pkg/cli/wasm.go` locates `wasmtime`
(`findWasmtime`, line 81) and runs it, CI installs wasmtime (ci.yml:84-87), and
`wasmtime` is installed on this host at `~/bin/wasmtime.exe`. Phase 108 records
byte-exact E2E goldens. So WASM is executable in the supported workflow.

**Classification:** security **CONFIRMED CURRENT** (no authority model);
ecosystem **CONFIRMED CURRENT** (local-only, no packages); WASM table row
**HISTORICAL / STALE**.

## 14. Historical / stale / intentional — explicitly separated

These are findings from older documents that do **not** represent current
behaviour, or that are deliberate limitations. They are listed apart from
§3-§13 so that a reader cannot mistake an old claim for a live defect.

| Prior claim | Source | Current reality |
|---|---|---|
| "`?` is a no-op (BUG-4)" | GAP_ANALYSIS C5 | **False.** `?` propagates correctly on the Go engine; reproduced, and `TestBugFix_OptionPropagation` passes |
| "BUG-1/2/3/7/8 all OPEN" | GAP_ANALYSIS C6 | **False for 1, 2, 7, 8** (ROADMAP table + 4 passing regressions). BUG-3 unverifiable here |
| "moves never expire (BUG-5)", evidence `borrow_checker.go:74-85` | GAP_ANALYSIS C4 | **Stale.** That code region is unrelated today; the scenario's test passes |
| "`BorrowError.Line` always 0" | GAP_ANALYSIS C4 | **Half stale.** Lines *are* populated; they are dropped at render |
| "the native path skips sema/borrow checks" | **my own earlier assessment** | **False.** Both run before dispatch at commands.go:61/72; reproduced |
| wasm32-wasi is "compile-only" | SPEC §13 | **Stale.** wasmtime is used by the CLI, installed in CI, present on this host |
| enum payloads are dropped | SPEC §13 | **Accurate** — a deliberate Phase 123 limitation, consistent across backends |
| Mach-O execution | increment 149 onward | **Intentional** — no Intel-mac runner exists; structural only |
| 151C3 PE execution | increment 151C3 | **Intentional** — structural only; recorded in `TestPhase151C3_PEExecutionStatus` |
| kcc native parity not started | — | **Stale.** 151B/151C/151C2/151C3 are complete and byte-identical; only 151A (value model) and 151D (dispatch) remain |

Two further GAP_ANALYSIS items were **not verified in this pass** and are
recorded as **UNABLE TO VERIFY** rather than repeated: **C1** (parser runaway
memory; a guard is mentioned at parser.go:424 but was not reproduced) and
**C3** (no symbol table / undefined-name detection — contradicted in practice,
since `error[K002]` is produced and enforced, but the specific claim was not
traced). **C2** ("no whole-program type checker") is **PARTIALLY STALE**:
`pkg/sema` now contains `resolve.go` plus actor, coroutine, ffi, kernel,
monomorph, npu and quantum checkers; whether these compose into a
whole-program type check was not determined here.

## 15. Release status — recorded, not adjudicated

* What the repository calls 1.1.0: **"Stable"**, in `VERSION`, banners and
  `stable-api.rst`.
* Known current defects established by this audit: `?` and `match` bindings
  diverge on the self-hosted engine (§3, §8); borrow diagnostics carry no line
  number (§5); no backend has a whole-program type-checking pass (§4).
* What the version plan requires for 1.1.0 (`KARKAIN-VERSION-PLAN.md` §3.1):
  *"only the owner ceremony: tag `v1.1.0` on current `main`, CI archives +
  checksums, `verify-install`/`rc-journey` against the shipped binary, docs
  marker flip."*

The documented 1.1.0 closure conditions are **ceremony-only**; they contain no
defect-free or soundness condition, so nothing in the plan is currently
violated by the findings above. Whether "Stable" is the right label given them
is a project decision, and this audit does not make it.

## 16. Evidence limitations

* **BUG-3** rests on a documentation row; no dedicated regression test was
  located in this pass.
* **GAP_ANALYSIS C1 and C3** were not reproduced or refuted.
* **Whole-program type checking** was not determined either way.
* **kcc's `?` divergence** was shown to fail at C compilation. Whether the
  missing `Propagate` case is the sole cause was not isolated — no other kcc
  code path was disabled to confirm.
* **No test was executed for the 151B/151C/151C2/151C3 gates** in this audit;
  their parity is cited from this session's own runs and from CI configuration,
  not re-measured here.
* **CI results were not re-run.** No claim below rests on an unverified run;
  where CI matters, the workflow configuration was read instead.
* **Ownership enforcement per backend** was traced at the CLI level only. A
  program that bypasses `RunCommand`/`BuildCommand` (for example a direct API
  caller) was not examined.
* **The `match` arm-binding divergence** was observed once and attributed to
  `match`; the precise cause was not traced.

## 17. Conclusion

After 151C3, the following is known about the current Karkain language
implementation, on repository evidence:

**Sound and verified.** The compiler is genuinely self-hosting enough to
serialise machine code: an x86-64 encoder and all three executable containers
(ELF, Mach-O PIE, PE32+) are now implemented in Karkain and byte-identical to
the Go oracle, with CI-wired gates, independent structural oracles and
mutation verification. The C and self-hosted engines are byte-identical across
a 61-program corpus. Runtime-error diagnostics are identical across engines.

**Partially functional.** The error model works for *explicit* fallible
returns — `Result`, `Option` and `?` are correct on the Go engine — but
*implicit* runtime failures are fatal and unobservable, and the self-hosted
engine cannot yet lower `?` or bind `match` arms.

**Not established.** A whole-language type check on any backend; whole-language
parity of `?` and `match`; a source-level debugging story (batch traces only,
no DAP); one coherent memory model (three allocation strategies); any
authority/security model; and a third-party ecosystem.

**Corrected.** Three long-standing audit claims are no longer true — `?` is not
a no-op, BUG-1/2/7/8 are fixed, and the native path does apply borrow and
semantic checks. Two of those corrections were to claims I made myself.

This audit deliberately does **not** state which of these should be addressed
next, or in what order. That decision belongs to the project owner.

---

## Post-audit authorization status (added 2026-09-28)

This audit remains the historical evidence of the checkpoint. Its findings are
unchanged and are not superseded by the note below.

```text
LH-1  (kcc `?` propagation parity)   AUTHORIZED / NOT STARTED
LH-2  (kcc `match` binding parity)    AUTHORIZED / NOT STARTED
All other audit findings             NOT CURRENTLY AUTHORIZED
```

The project owner authorized LH-1 and LH-2 only, as implementation slices
under the Language Hardening Checkpoint — not as new numbered phases. Nothing
in this audit was implemented, and the remaining findings stay evidence until
separately approved.

---

## LH-1 status update (2026-09-29)

The note above is retained as the authorization record. LH-1 has since been
implemented and closed:

```text
LH-1  (kcc `?` propagation parity)   COMPLETE
LH-2  (kcc `match` binding parity)    AUTHORIZED / NOT STARTED
All other audit findings             NOT CURRENTLY AUTHORIZED
```

```text
Implementation:   5dcdd80  feat: fix kcc question propagation
Focused local gate: 11/11 PASS
CI:                CI/CD run 36502124716, job Test,
                   step "Run LH-1 kcc propagation parity gate" -- success
```

**§3 of this audit described LH-1 inaccurately, and the error is recorded here
rather than quietly dropped.** The audit located the defect by searching
`codegen.kark` for a `NODE_PROPAGATE` case. That search was correct but its
conclusion was not: kcc's parser lowers postfix `?` to a `UnaryExpr` whose
operator is `?`, so `NODE_PROPAGATE` (57) and the `Propagate` type name are
**dead on the lowering path**. A case added for `NODE_PROPAGATE` would never
have executed.

The audit also did not record the defect that actually prevented the operator
from working. kcc had no **expression-level** `Ok`/`Err`/`Some`/`None`; those
tokens were consumed only in `parseMatchExpr`'s arm-pattern position, so in
expression position the constructor was dropped and only the parenthesised
payload survived. With no way to construct a `Result` or `Option` value, `?`
had nothing to inspect or propagate. This was isolated with a program
containing no `?` at all. Both defects were required for the operator's
semantics, and both are gate-pinned.

