KIR (Karkain IR)
================

KIR is the Karkain-owned compiler intermediate representation. It is the
first compiler component written entirely in Karkain, owned by the
self-hosted compiler, and serialized as a deterministic **text v1**
format.

Ownership
---------

The emitter lives in ``src/compiler/kir.kark`` (``kirEmit`` and its
helpers). It is:

1. actual ``.kark`` source compiled by the Karkain toolchain,
2. invoked through the real compiler path (the self-hosted ``kcc``),
3. gated by the Phase 120 test suite (``pkg/cli/phase120_kir_test.go``),
4. exercised end-to-end by ``kcc kir`` and the CLI ``karkain kir``.

The Go repository only routes to it: ``pkg/cli/kir.go``
(``KCCKirCommand``) is a thin gateway that runs the Go-side syntax
preflight, assembles the unit, and dispatches to the self-hosted engine.
There is no Go KIR emitter.

Interface
---------

::

   karkain kir <file.kark>
   karkain kir --verify <file.kark>
   kcc kir <file>
   kcc verifykir <file>

The command parses the file with the self-hosted parser and renders the
AST as KIR text. Parse errors surface before emission; success prints the
KIR lines followed by a path-less confirmation:

::

   [ok] kir text: N lines

``--verify`` (Phase 121) runs the compiler-owned structural verifier
instead of dumping the text and prints a distinct verdict:

::

   [ok] kir text: N lines
   [ok] kir verify: N lines ok

Structural verification (Phase 121)
-----------------------------------

``kirVerify`` in ``src/compiler/kir.kark`` checks every emitted line
against the KIR v1 structural contract:

- the header line is exactly ``KIR v1``;
- the source line names exactly the file basename;
- indentation is two spaces per depth level;
- nesting depth never jumps forward by more than one level;
- every statement line ends `` line: <decimal>`` except bare ``block``
  introducers and the unknown-node fallback (``stmt <type>``).

The verifier is written in Karkain using only language builtins and the
in-tree user helpers — no Go front-end, external IR, or library
participates in the proof. It runs in two places:

- ``checkFile``: the DEFAULT kcc check path executes it after the Phase 99
  type checker — silent on success, ``error[K121]`` + no ``[ok]`` on
  drift (the CLI maps that to exit 3);
- ``verifykir`` / ``karkain kir --verify``: the standalone, deterministic
  command form.

This turns KIR from a CLI artifact (Phase 120) into an engine-internal
invariant: any future emitter-vs-contract drift becomes a hard, visible
error on every accepted ``check`` instead of silent divergence. With the
current emitter no valid source can fail the verifier — the point.

Determinism contract
--------------------

KIR output is **byte-identical** across runs and sandboxes:

- the header carries only the file basename (``source: <name>.kark``) —
  never the sandbox path,
- the confirmation line is path-less,
- every line is produced by a single ``print``, so identical input means
  identical bytes (enforced by the Phase 120 ``ByteDeterminism`` gate).

Format (v1)
-----------

One line per statement at an indentation equal to the block depth (two
spaces per level). Expressions are fully parenthesized; empty
constructs render as ``_``.

.. list-table::
   :header-rows: 1
   :widths: 18 82

   * - Construct
     - KIR text
   * - Program
     - ``KIR v1`` / ``source: <basename>`` / ``import <name> line: N`` /
       ``cimport (<len> bytes) line: N``
   * - Function
     - ``func <name> (params: a, b) line: N`` followed by the body depth+1
   * - Kernel
     - ``kernel <name> (params: a, b) line: N`` followed by the body
   * - Struct
     - ``struct <name> (x:int, y:int) line: N``
   * - Enum
     - ``enum <name> (Red:, Green:i32) line: N``
   * - Variable
     - ``var/let/const <name> = <expr> line: N``
   * - Return / Print
     - ``return [<expr>] line: N`` / ``print <expr> line: N``
   * - Expression stmt
     - ``<expr> line: N``
   * - Block
     - ``block`` followed by body depth+1
   * - If
     - ``if <expr> line: N``, body depth+1, then ``else line: N``, body
   * - While
     - ``while <expr> line: N`` followed by body
   * - For
     - ``for <init>; <cond>; <post> line: N`` (``_`` when a clause is empty)
   * - For-in
     - ``forin <var> in <iter> line: N`` followed by body
   * - Break / Continue
     - ``break line: N`` / ``continue line: N``
   * - Alloc / Free / GlobalId
     - ``alloc <type> <expr> line: N`` / ``free <ptr> line: N`` /
       ``global_id <dim> line: N``
   * - Binary / Unary
     - ``(<op> l r)`` / ``(<op> e)``
   * - Call
     - ``(call <name> <arg>...)``
   * - Index / Member / Slice
     - ``(index l i)`` / ``(member obj field)`` / ``(slice l s e)``
   * - Literals
     - ``(array e ...)`` / ``(map k=v ...)`` / ``(struct Name f=v ...)`` /
       ``"<string>"`` / int / float / bool raw values
   * - Lambda / Closure
     - ``(lambda (params: ...) body=N)`` /
       ``(closure <name> (params: ...) body=N)``
   * - Match
     - ``(match <value> (<type> <binding> <pattern> line: N) ...)``
   * - Unknown
     - ``(expr <type>)`` / ``stmt <type>`` (never silently dropped)

Boundaries
----------

- KIR text never feeds the build/run backend yet. Phase 121 makes the
  emitter an engine-internal verified invariant on the default check path;
  consuming generated KIR as the input to code generation is tracked as a
  post-121 work item.
- The self-hosted ``kcc kir`` path is the reference; there is no Go
  engine fallback for ``karkain kir`` (``--verify`` included).
- Line numbers in the text are those produced by the self-hosted
  lexer/parser on the assembled unit; they are stable and deterministic
  for a given input but are not asserted as file-absolute in the gate.