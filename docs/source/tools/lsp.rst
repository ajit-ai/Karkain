===
LSP
===

Karkain ships a real Language Server Protocol server exposed through
two equivalent commands.

Usage
=====

.. code-block:: console

    $ karkain lsp
    $ karkain language-server   # alias

The server speaks **JSON-RPC 2.0 over stdio** (``Content-Length``
framing per the LSP specification). It is invoked directly by an editor
or IDE client; there is no blocking network port.

Capabilities
------------

The server advertises and implements:

.. list-table::
   :widths: 34 66
   :header-rows: 1

   * - Method
     - Description
   * - ``initialize`` / ``initialized`` / ``shutdown`` / ``exit``
     - Standard LSP lifecycle
   * - ``textDocument/didOpen``
     - Register a document; diagnostics are pushed immediately
   * - ``textDocument/didChange``
     - Full-document sync; diagnostics are re-pushed on every change
   * - ``textDocument/didSave``
     - Text is retained on save
   * - ``textDocument/publishDiagnostics``
     - Server-initiated diagnostics push back to the client
   * - ``textDocument/completion``
     - Code completion (trigger characters ``.`` and ``:``)
   * - ``textDocument/hover``
     - Hover information
   * - ``textDocument/definition``
     - Go-to-definition
   * - ``textDocument/semanticTokens/full``
     - Lexer-driven semantic highlighting (keywords, strings, numbers,
       comments, operators, variables)
   * - ``textDocument/documentSymbol``
     - Document outline / symbols
   * - ``textDocument/formatting``
     - Format a document through the same canonicalizer as
       :doc:`fmt`

Diagnostics sync
----------------

On every ``textDocument/didOpen`` and ``textDocument/didChange``, the
server analyzes the document with the same engine-agnostic
``cli.AnalyzeSource`` driver used by :doc:`check <cli>` and publishes the
result as ``textDocument/publishDiagnostics``. Diagnostics use the real
lexer/parser columns and excerpts (source spans with ``endColumn``)
from the Phase-83 pipeline, so inline squiggles point at the true error
location.

The Go front-end engine is used for analysis regardless of the
``KARKAIN_ENGINE`` default, because LSP requires parse-AST access that
the self-hosted runner does not expose yet.

Client integration
------------------

The LSP capability is what powers the bundled **VS Code extension**:
``extension.js`` keeps its check/compile/run/format commands and additionally
starts a language client against ``karkain lsp`` for hover, go-to-definition,
completion and semantic highlighting. Configure the
extension to launch ``karkain lsp`` with stdio and the server reports
version ``karkain-lsp 1.1.0`` on ``initialize``.

.. code-block:: console

    $ karkain lsp            # block forever serving the protocol
    $ karkain language-server

.. seealso::

   :doc:`fmt` — the formatter the LSP uses for
   ``textDocument/formatting``.
   :doc:`cli` — ``ide info`` prints a machine-readable IDE contract.