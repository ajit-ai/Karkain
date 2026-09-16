===
Fmt
===

``karkain fmt`` applies the deterministic, token-level canonical
formatter to ``.kark`` files.

Usage
=====

.. code-block:: console

    $ karkain fmt <file.kark|dir> [--check]

- ``file`` — format a single file in place.
- ``dir`` — format all ``.kark`` files in the directory recursively.
- ``--check`` — verify canonical formatting without rewriting; exits
  non-zero if any file would change.

How it works
------------

The formatter is deliberately conservative (contract-level, not a
sophisticated AST pretty-printer): it preserves token text and order,
line structure, leading indentation, blank lines and comments, and only
canonicalizes inter-token spacing, trailing whitespace and line endings.

Because tokens are never re-ordered or re-spelled, formatting cannot
change semantics, and it is **idempotent by construction** — running it
twice produces the same output.

.. code-block:: console

    # Format a single file in place
    $ karkain fmt hello.kark

    # Format every .kark file below the current directory
    $ karkain fmt .

    # CI gate: fail if anything is not yet canonical
    $ karkain fmt . --check
    error: 1 file(s) are not formatted
    $ echo $?   # exits non-zero

CI usage
--------

``--check`` is intended as a CI gate: run it over the whole source tree
and fail the job when it reports unformatted files. The exit code is
non-zero when any file would change.

Exit codes
----------

- ``0`` — every file is (or became) canonical
- ``1`` — ``--check`` found files that need formatting, or an I/O error

.. seealso::

   :doc:`/reference/index` — the language reference.
   :doc:`/language/index` — the language guide.