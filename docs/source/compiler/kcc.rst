The Self-Hosted kcc Engine
==========================

The self-hosted compiler (``kcc``) is a Karkain-to-C23 transpiler
written entirely in Karkain. It compiles its own source code through a
bootstrap pipeline and is the default engine for all core CLI commands.

Source files
------------

The compiler lives in ``src/compiler/`` and consists of nine ``.kark``
source files plus two generated C artifacts:

.. list-table::
   :header-rows: 1
   :widths: 22 70

   * - File
     - Responsibility
   * - ``lexer.kark``
     - Tokenizer: keywords, literals, operators, identifiers
   * - ``parser.kark``
     - Recursive-descent parser, AST construction, macro expansion
   * - ``ast.kark``
     - AST node definitions and accessor helpers
   * - ``sema.kark``
     - Symbol tables, scope management, name resolution
   * - ``checker.kark``
     - Two-pass type checker (``collectDecls`` then ``checkStmt``/
       ``checkExpr``/``checkCall``/``checkIdent``)
   * - ``atypes.kark``
     - Conservative static type inference (``inferType``,
       ``primitiveTag``, ``typeDescription``, ``annotationMismatch``)
   * - ``codegen.kark``
     - C23 code emission (mirrors ``pkg/codegen`` output)
   * - ``main.kark``
     - Driver: ``check``, ``build``, ``run``, ``test``, ``lsp``
       commands; sibling assembly; test runner with per-test driver
       synthesis
   * - ``stdlib.kark``
     - Standard library builtins for the self-hosted engine
   * - ``kernel.c``
     - Generated C artifact (output of transpilation)
   * - ``runtime.c``
     - Generated C runtime (output of transpilation)

Assembly
--------

When the Go front end compiles ``kcc``, it assembles all ``.kark``
sources into a single C23 file (``src/compiler/main.c`` or
``main.c23``), which GCC links into the ``kcc`` binary. The assembly
step concatenates ``lexer.kark``, ``parser.kark``, ``ast.kark``,
``sema.kark``, ``checker.kark``, ``atypes.kark``, ``codegen.kark``,
``stdlib.kark``, and ``main.kark`` in dependency order, then appends
``main.kark`` last (it contains ``main()``). The file
``loadSourceWithSiblings()`` in ``main.kark`` mirrors this: it reads
all ``.kark`` files in the same directory and concatenates them, placing
the target file last.

Pipeline
--------

The self-hosted compiler implements the same four-stage pipeline as the
Go front end:

1. **Lex** — ``tokenize(source)`` produces a token array.
2. **Parse** — ``parse(tokens, source)`` produces an AST. Parser errors
   are reported via ``printParserErrors(state)``.
3. **Type check** — ``typeCheckProgram(ast)`` runs the two-pass checker.
   Errors surface as ``error[K1XX]`` diagnostics. When any are present
   the file is NOT reported as ``[ok]`` and the CLI maps the result to
   exit code 3.
4. **Codegen** — ``generate(ast, target, fileBaseName)`` produces C23
   source code which is written to a ``.c23`` file alongside the source.

CLI commands in ``main.kark``:

- ``check <file.kark>`` — lex → parse → type-check only.
- ``build <file.kark> [--target c23]`` — full pipeline to C23 output.
- ``run <file.kark>`` — transpile → ``gcc`` → execute → cleanup.
- ``test <path> [filter]`` — discover ``*_test.kark``, per-test driver
  synthesis, compile+run, structured summary output.
- ``lsp`` — placeholder for the self-hosted LSP.

CLI integration
---------------

**Package:** ``pkg/cli/kcc_engine.go``

The Go CLI routes commands through kcc via three entry points:

- ``KCCCheckCommand`` — assembles source (manifest dependencies +
  siblings), runs the Go-side multi-file syntax preflight, NPU target
  preflight, then invokes ``kcc check`` in a temp sandbox.
- ``KCCBuildCommand`` — same assembly + preflight, then ``kcc build``
  in a temp sandbox. Unless compile-only, links with GCC.
- ``KCCRunCommand`` — same assembly + preflight, then ``kcc run``
  (transpile → gcc → execute) in a temp sandbox.

The ``KCCRunCommand`` wraps kcc's run path because the self-hosted
engine shells out to ``gcc`` and ``system()``, which require the binary
to be on the host filesystem. The Go CLI manages sandbox creation and
cleanup.

Engine selection
~~~~~~~~~~~~~~~~

Since Phase 97 kcc is the **default engine**. ``EngineFromEnv()``
returns ``EngineKCC`` for an empty ``KARKAIN_ENGINE`` or any value
other than ``go``/``Go``/``GO``. The ``--engine go|kcc`` flag provides
explicit override.

Staleness detection
~~~~~~~~~~~~~~~~~~~

``kccStale()`` compares the modification time of ``kcc.exe`` against
every ``src/compiler/*.kark`` file. When any source is newer, the
binary is rebuilt via the bootstrap pipeline to prevent silent incorrect
output.

Phase history
-------------

.. list-table::
   :header-rows: 1
   :widths: 14 76

   * - Phase
     - Milestone
   * - 88
     - Self-hosted compiler foundation: lex/parse/AST/sema/codegen in
       Karkain, 7 representative test programs, build script
   * - 95
     - kcc is the primary engine for lex + parse + C codegen;
       ``KCCCheckCommand``/``KCCBuildCommand``/``KCCRunCommand`` wired
       into CLI
   * - 96
     - kcc owns the test runner: ``collectTestFiles``, per-test driver
       synthesis, ``--filter`` substring matching, Go-parity summary
       output
   * - 97
     - kcc is the default engine (``EngineFromEnv`` returns ``EngineKCC``);
       manifest dependency resolution wired into assembly pipeline
   * - 99
     - Type checker certified: ``checkFile`` → ``typeCheckProgram``
       produces clean output for the compiler's own sources; 14/14 error
       fixtures rejected; bootstrap identity verified
   * - 107
     - Bootstrap identity verified (stage2 == stage3 bitwise identical,
       SHA ``aff1d624d9e52c2d``)

Known limitations
-----------------

- **Stage-2 OOM on ~4 GB hosts** — The full compiler source tree
  (approximately 214 KB of ``.kark`` source) may exhaust memory during
  Stage 2 compilation. This is a documented environmental class, not a
  defect. The low-memory ``check`` path still passes.
- **String-array print differs** — kcc prints string arrays as ``[x]``
  while the Go engine prints ``["x"]``. This is an accepted divergence
  in the test output formatting, not a semantic difference.
- **PowerShell BOM issue** — PowerShell 5.1 ``Set-Content -Encoding UTF8``
  writes a BOM that breaks mid-assembly module loads. Standard library
  ``.kark`` files must be written as UTF-8 without BOM.
- **SSA optimization** — kcc generates C directly from AST without SSA
  lowering. Optimized builds use the Go front end.
- **DWARF / profiling / concurrency C helpers** — not emitted by kcc;
  these are Go-only features.
