Roadmap
=======

The canonical development roadmap lives in ``ROADMAP.md`` at the repository
root. This page summarizes the current state for documentation purposes.

.. contents:: Sections
   :local:
   :depth: 1

Completed phases (50–149)
-------------------------

Phases 50–149 are complete. Phases 128–142 shipped the **1.0.0 (Stable)**
line and the **1.1.0** line (Phases 132–149) is complete in-tree, with only
the owner tag/archive ceremony outstanding. A concise summary:

* **50–106** — language core, semantic model (borrow checker, escape
  analysis), SSA optimization pipeline, HIR, stdlib foundation, self-hosted
  compiler foundation, conformance/examples, diagnostic spans, native
  codegen/linker/DWARF, incremental compilation, SIMD, package manager,
  debugger, profile, error recovery.
* **107** — concurrency runtime (work-stealing scheduler, channels, actors).
* **108** — WASM target (``wasm32-wasi`` backend, wasmtime-gated).
* **109** — standard library v2 (importable ``std.string / collections /
  io / encoding / crypto / testing`` on both engines).
* **110** — profiling and diagnostics (``karkain prof``, text/json/folded
  reports).
* **111** — cross-compilation (``--target <triple>``, Karkain-owned target
  model, same-machine/foreign machine detection).
* **112** — programming language foundation additions (``const`` keyword,
  ``float()`` builtin, ``karkain debug`` trace, ``std.testing`` module,
  nested block comments).
* **113** — official documentation set (this site): 15-category examples
  framework, status model and contributor documentation.
* **114** — example corpus (50 ``.kark`` programs across 15 categories,
  pinned byte-identical on both engines by ``pkg/cli/phase114_examples_test.go``).
* **115** — developer preview readiness: LICENSE/CONTRIBUTING/CODE_OF_CONDUCT,
  honest README + docs, unified v0.115.0 identity. Superseded by the 1.0.0
  label.
* **116** — developer examples corpus and real-world programming corpus.
* **117** — Beta 1 readiness and hardening (``while`` parity, semantic
  gating, exit-code contract, kcc test runner, v0.117.0 identity).
* **118** — Beta 1 external validation and release-candidate readiness
  (RC READY verdict; release hygiene, issue templates, install scripts,
  rc-journey gate).
* **119** — language QA and public repository finalization: **Karkain
  1.0.0 (Stable)** label, v1.0.0 identity, superseded-material cleanup,
  QA battery, release documentation.
* **120** — GA envelope + KIR v1 text emitter (compiler-owned IR, `karkain kir`).
* **121** — compiler independence foundation (KIR structural self-verification).
* **122** — compiler pipeline ownership (flat project assembly inside kcc,
  marker-gated stdlib discovery).
* **123** — language core completion: enum/match byte-identical parity on both
  engines with checker validation.
* **124** — compute-target catalog (``cpu``/``simd``/``wasm32-wasi``/
  ``gpu-experimental``/``npu-experimental``/``quantum-experimental``).
* **125A** — standard-library networking/database/web slice + Windows Winsock
  link contract (``std.net``, ``std.http``, ``std.db``).
* **126** — standard-library numerics module (``std.numerics``, 40 funcs),
  corpus 59/59 byte-identical.
* **127** — bootstrap memory guard (``error[K127]`` on low-RAM hosts replaces
  the SEGFAULT class).
* **128** — compiler independence foundation and **129** — 4 GB bootstrap
  battle (progress heartbeats, CI memory cap, pagefile guidance).
* **130** — closures / ``fn`` capture mutation (write-through on both engines).
* **131** — reproducible self-host gate (stage-1 byte-reproducible; tamper
  divergence detected).
* **132** — standard-library freeze + SemVer and deprecation policy (10 modules
  frozen).
* **133** — first-class ``fn`` values (params, arrays, returns, error[K114]
  escape rejection).
* **134** — incremental compilation v2 (per-module translation units,
  content-keyed objects, ≤100 ms no-op rebuilds).
* **135** — local package registry (directory origin, immutable versions,
  digest-verified fetch).
* **136** — LSP v2 (semantic tokens, hover, go-to-definition, scoped +
  member completion).
* **137** — concurrency parity (``spawn``/``channel``/``actor`` byte-identical
  on both engines — the Go-only boundary closed).
* **138** — accelerator kernel surface (``@target(gpu)`` WGSL emission,
  compile-only guarantee).
* **139** — cross-compilation expansion + WASM components (``aarch64-windows``,
  ``riscv64-linux``, ``*-macos``, ``karkain wit``).
* **140** — debugger integration (``karkain dbg`` live gdb backtraces, VS Code
  debug templates).
* **141** — compiler performance & memory (SimplifyCFG; ``prof`` allocation
  counts; measured RSS table).
* **142** — **1.1.0** version identity, ``1.0.x`` LTS branch, release notes.
* **143** — seed-binary bootstrap closure (stages 2–3 run Go-free).
* **144** — toolchain sovereignty (manifest/workspace/registry resolution
  inside ``kcc``).
* **145** — native backend v1 (hand-encoded x86-64 + static ELF, no C
  compiler).
* **146** — generics v1 (explicit type args, monomorphization, ``std.generics``,
  then ``kcc`` parity).
* **147** — native execution P1 resolved (REX.W immediate misencoding;
  quarantine lifted, Linux execution green).
* **148** — ``--target native-x86_64-linux`` CLI path, control flow and the
  integer/string calling convention.
* **149** — PE + Mach-O writers, Win64 boundary and loader-independent PEB
  bootstrap (PE images execute on windows/amd64).

Key cross-cutting milestones
----------------------------

* **Phases 71–78**: Math/Tensor IR chain and the CPU/GPU/NPU backend
  abstraction — internal packages with no language surface yet released.
* **Phase 79**: compiler integrity audit and self-hosting readiness — audit
  of the full pipeline, IR layers and backend parity.
* **Phase 95–99**: the self-hosted ``kcc`` engine becomes the default
  compiler for ``check/build/run/test``, with bootstrap identity.

Current status
--------------

The project is **Karkain 1.1.0** (identity adopted at Phase 142; all 1.1.0 code
in-tree with only the owner tag/archive ceremony outstanding). The versioned
language specification lives in ``SPEC.md``; the authoritative status matrix
for the released scope is :doc:`/status/scope`, and the compatibility
guarantees are documented at :doc:`/status/compatibility`. Release notes are
tracked at :doc:`/release-notes`. Forward planning is tracked version-wise in
``docs/audit/KARKAIN-VERSION-PLAN.md``.

What ships in 1.1.0 today
-------------------------

* Importable standard library modules (``std.string``, ``std.collections``,
  ``std.io``, ``std.encoding``, ``std.crypto``, ``std.testing``,
  ``std.numerics``, ``std.net``, ``std.http``, ``std.db``, ``std.generics``)
  on both engines, frozen under the SemVer policy.
* First-class ``fn`` values (higher-order functions, arrays, factories,
  recursion, safe escape rejection via ``error[K114]``).
* Generics v1 (functions and structs, explicit type arguments, monomorphization,
  both engines).
* CLI surface: ``check``, ``build``, ``run``, ``test`` (``--filter``),
  ``transpile``, ``fmt``, ``lint``, ``dbg``, ``prof``, ``target``,
  ``pkg`` (local registry init/add/publish/fetch), ``workspace``, ``clean``,
  ``explain``, ``bench``, ``lsp`` (LSP v2 semantic tokens/hover/definition),
  ``kir`` (intermediate representation text + structural verification),
  ``wit`` (WASM component envelopes).
* Cross-compilation via ``--target <triple>`` (10 triples + ``wasm32-wasi``)
  with deterministic failure when a cross-linker is missing.
* Native backend (C-free): ``--target native-x86_64-linux`` writes static ELF64
  and executes green; PE32+ (Windows x86-64) and Mach-O writers in-tree with
  PE execution verified.
* Concurrency runtime (work-stealing scheduler, channels, actors) with full
  ``kcc`` parity (Phase 137).
* Runtime error model with source locations and stack traces.
* Incremental compilation v2 (per-module translation units, content-keyed
  objects, ≤100 ms no-op rebuilds).
* Accelerator kernel surface: ``@target(gpu)`` emits WGSL compute kernels with
  a compile-only guarantee.

Future: Version-Wise Roadmap
----------------------------

Forward development is organized by **version** with per-increment execution
(see ``docs/audit/KARKAIN-VERSION-PLAN.md`` for exit criteria and NFR targets):

* **1.2.0 "Sovereignty I" (open — next):** Native Value model (boxed Values,
  arrays, ``for-in``, floats, structs) + ``native-x86_64-windows``/``macos`` CLI
  targets (Increment 150), ``kcc`` native parity (151), native stdlib and no-C
  closure (152), ``kcc`` builds ``kcc`` self-bootstrap (154), and multi-arch
  Docker image on ``ghcr.io`` (165).
* **1.3.0 "Libraries & Foundation":** Essential OS modules (``std.path``,
  ``std.env``, ``std.time``, ``std.random``, ``std.json`` — Increment 155),
  promote/cut verdicts for historical modules (156), stdlib expansion (158),
  generics v2 (159), linear/move type enforcement (160), atomics + memory
  ordering (161).
* **1.4.0 "Tooling & Developer Experience":** DAP server (153), package
  manager git/semver resolution (157), test framework v2 (163), CLI polish
  (164), DWARF consumer (175), semantic analysis v2 (176).
* **1.5.0 "Platform Reach & Packaging":** Native packages (.deb, .rpm, MSI,
  Homebrew — 166–168), AArch64 native backend (169), freestanding / MCU target
  (170), WASM GC + component model (171), BSD targets (172).
* **1.6.0 "Performance & Polish":** Fast front-end / low-RAM kcc (162), native
  SSA optimization pipeline (174), public package registry client (179).
* **2.0.0 "Language Maturity":** Explicit struct layout control (the one
  breaking ABI change — 177), trait/interface system (178), error-handling
  idiom v2 (180).

The versioned phase descriptions remain in ``AGENTS.md`` at the repository
root. Current status is **Karkain 1.1.0 (in-tree complete)** with the public
release ceremony pending.

.. seealso::

   :doc:`/development/developer-preview` — the historical development-preview
   assessment and how the label changed.
   :doc:`/status/index` — the status vocabulary applied throughout this site.