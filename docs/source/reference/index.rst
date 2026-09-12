.. _reference:

==================
Language Reference
==================

The language reference is a precise, detailed description of every lexical
token, syntactic form, operator, keyword and type in Karkain. It is meant
for implementors and developers who need authoritative answers — not a
tutorial.

For a tutorial-oriented introduction, see :doc:`/language/index`.

.. toctree::
   :maxdepth: 2
   :caption: Reference

   syntax
   operators
   keywords
   stable-api
   types
   diagnostics
   compatibility
   example-matrix

-----------

How the reference is organized
==============================

- :doc:`syntax` — lexical structure and statements: identifiers,
  literals, comments, keywords and the full statement and expression
  grammar.
- :doc:`operators` — every built-in operator, its precedence and known
  semantics.
- :doc:`keywords` — the complete list of reserved words and
  type-keywords.
- :doc:`types` — the built-in type system: representation, inference and
  conversion.
- :doc:`diagnostics` — the error-code contract, JSON schema and exit
  codes.
- :doc:`compatibility` — engine parity matrix between the Go front end
  and the self-hosted ``kcc`` engine.
