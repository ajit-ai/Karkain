:orphan:

Developer Tools
===============

:implemented:`Implemented` — this category documents tools *for* Karkain
development, all of which are real and tested end-to-end.

* **``karkain fmt``** — token-level canonical formatter; idempotent
  ``fmt --check`` mode; recursive ``karkain fmt .`` (Phase 82/86).
* **``karkain lint``** — real lint pipeline with diagnostics; exits
  ``ExitLint`` (7) on findings.
* **``karkain debug``** — compiles and runs a program with opt-in function
  enter/leave trace emissions (Phase 112; Go engine).
* **``karkain prof``** — compiles and runs once with opt-in,
  aggregation-based instrumentation; text/json/folded reports (Phase 110;
  Go engine).
* **``karkain test``** — deterministic ``*_test.kark`` discovery and
  execution with ``--filter``; the native test foundation (KTF-001).
* **LSP server** — ``karkain lsp`` serves real language-server features
  (diagnostics, completion, hover, go-to-definition, formatting) on a shared
  ``cli.AnalyzeSource`` driver with the check command (Phase 82/83/86).
* **VS Code extension** — ``extension.js`` with check/compile/run/format
  commands and a grammar, validated by
  ``pkg/cli/vscode_extension_test.go``.

A combined multi-tool example lives in ``examples/showcase/15_devtools/``
(multi-file banking app, a ``karkain test`` suite, and a ``fmt``/``fmt
--check`` formatting demo).

See the tool pages for each command in detail:
:doc:`/tools/fmt`, :doc:`/tools/debug`, :doc:`/tools/profile`,
:doc:`/tools/test`, :doc:`/tools/lsp`.

.. note::

   ``karkain debug`` and ``karkain prof`` explicitly support the Go engine;
   the self-hosted ``kcc`` engine is not yet profiler/trace-aware (deferred
   boundaries, documented in their reports).

.. seealso::

   :doc:`/tools/index` — the full CLI surface.
   :doc:`/status/implemented` — related implemented-feature entries.