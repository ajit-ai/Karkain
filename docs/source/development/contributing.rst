Contributing
============

Welcome to the Karkain contributor guide. This page is the minimum needed to
produce a correct, mergeable pull request.

.. contents:: Sections
   :local:
   :depth: 1

Fork, branch, commit, PR
------------------------

1. **Fork** the repository on GitHub.
2. **Create a branch** off ``develop`` — descriptive names preferred, e.g.
   ``feature/phase-114-examples``.
3. **Write code** in ``.kark`` files, Go packages, or documentation (RST under
   ``docs/source/``).
4. **Commit** — one logical change per commit; the commit message should
   describe *what* changed and *why*, not how.
5. **Open a PR** against ``develop``. The PR description should state the
   feature boundary, what was tested, and any honest limitations.

Branch workflow (hard rule)
---------------------------

After **every** Phase completion and successful test run:

1. Commit all changes to ``develop``.
2. Merge ``develop`` into ``main``.
3. Push both branches to origin.

This ensures ``main`` always reflects the latest working state. This is a
hard rule — never skip this step.

Running the test suite
----------------------

Run the full test suite before committing. **All tests must pass.**

.. code-block:: console

   go test ./pkg/lexer/... ./pkg/parser/... ./pkg/codegen/... ./pkg/pm/... -count=1

On the documented 4 GB host, run single-package tests sequentially:

.. code-block:: console

   GOMAXPROCS=1 GOGC=60 go test ./pkg/lexer/... -count=1
   GOMAXPROCS=1 GOGC=60 go test ./pkg/parser/... -count=1

.. note::

   The CLI test suite (``go test ./pkg/cli/...``) is large; run it in a
   dedicated session rather than as part of a fast commit cycle.

Code style
----------

* **No comments** in source unless explicitly asked for them.
* Use the ``.kark`` extension for all Karkain source files.
* Follow existing patterns — check ``pkg/`` package style, the conformance
  convention, and the example structure before inventing new ones.
* The language is authored in Karkain itself (``src/compiler/*.kark``) and in
  Go (``pkg/**/``); follow whichever applies to your change.

Self-hosting boundary
---------------------

Changes to ``src/compiler/*.kark`` are part of the self-hosted compiler.
Modify them with care: the Go codegen path and the self-hosted ``kcc``
engine must agree. Test with both engines explicitly:

.. code-block:: console

   KARKAIN_ENGINE=go  go test ./pkg/cli/... -run TestPhase99_Selfhosted -count=1
   KARKAIN_ENGINE=kcc go test ./pkg/cli/... -run TestPhase99_Selfhosted -count=1

Language surface changes
------------------------

New keywords, builtins or runtime helpers need changes in multiple places at
once — a minimal parity map:

* **Lexer**: ``pkg/lexer/lexer.go`` (token type) and
  ``src/compiler/lexer.kark`` (self-hosted mirror).
* **Parser**: ``pkg/parser/parser.go`` and
  ``src/compiler/parser.kark``.
* **Semantic**: ``pkg/sema/resolve.go`` and
  ``src/compiler/checker.kark``.
* **Codegen**: ``pkg/codegen/codegen.go`` and
  ``src/compiler/codegen.kark``.

All additions must pass the full regression suite on **both** engines before
committing.

Security rules
--------------

* Never commit secrets, keys or credentials.
* Never log or expose secrets in generated code.
* Follow the security rules in the repository root ``AGENTS.md``.

.. seealso::

   :doc:`testing` — how the test suite is structured and how to run it.
   :doc:`architecture` — repository layout and package map.