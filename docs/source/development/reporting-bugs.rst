=====================
Reporting Bugs
=====================

A good bug report lets a maintainer reproduce your problem without guessing
about your machine, your version, or your command. This page is the
canonical workflow for reporting a Karkain compiler or toolchain problem.
The GitHub issue templates (``.github/ISSUE_TEMPLATE/``) use exactly these
fields.

.. contents:: Sections
   :local:
   :depth: 1

Before you file
===============

1. Find the smallest command that reproduces the problem. Two lines of
   ``.kark`` are better than two hundred.
2. Run it on the **current** build of the toolchain:
   ``karkain run <file>`` with ``KARKAIN_ENGINE` unset (the default ``kcc``
   engine) and, if relevant, again with ``KARKAIN_ENGINE=go``.
3. Reproduce twice. If the result is nondeterministic, say so explicitly.

If you reproduce a security vulnerability, do **not** open a public issue —
see ``SECURITY.md``.

Collect the diagnostic information
==================================

Every report needs the following. None of these require private data.

.. list-table:: Required fields
   :widths: 30 70
   :header-rows: 1

   * - Field
     - How to gather it
   * - Karkain version
     - ``karkain --version`` (e.g. ``Karkain Compiler v1.0.0
       (windows/amd64, Stable Build)``).
   * - OS
     - e.g. ``Windows 11``, ``Ubuntu 24.04``; include the architecture.
   * - Architecture
     - ``karkain target`` prints the host triple, or use ``uname -m`` /
       ``$env:PROCESSOR_ARCHITECTURE``.
   * - Engine
     - The ``KARKAIN_ENGINE`` value you used, or "default (kcc)".
   * - Command
     - The exact ``karkain ...`` invocation.
   * - Source file
     - The minimal, self-contained ``.kark`` file that reproduces it.
   * - Expected result
     - What you expected to happen.
   * - Actual result
     - What happened instead: output, ``error[K...]`` diagnostics,
       ``runtime error: <kind> at <file>:<line>`` traces, exit codes,
       and whether the Go engine and ``kcc`` agree.

Do not include private information: no credentials, no tokens, no personal
paths beyond what is needed to reproduce, no unrelated project code.

The reproduction script
=======================

For engine-parity or diagnostic bugs, attach (or paste) the two-line script
that shows the divergence:

.. code-block:: text

   # shell / PowerShell
   karkain check repro.kark
   karkain run  repro.kark
   KARKAIN_ENGINE=go karkain run repro.kark
   echo Exit: $LASTEXITCODE   # or $?

For a panic or crash, add steps 1-3 from the compilation pipeline
(``go build ./cmd/karkain`` + the failing command) so the build identity is
reproducible.

Filing the issue
================

Open the issue at https://github.com/ajit-ai/Karkain/issues/new/choose and
pick **Bug Report**. The template collects the fields above in order. If you
are unsure whether the behavior is a bug or intended, open a **Documentation
Issue** or ask first — maintainers prefer a question over a mislabeled bug.

After you file
==============

* Add a comment if you discover the minimal reproduction case afterwards,
  or if behavior changes between two builds.
* If a maintainer asks for clarification, treat the issue as blocked until
  you answer; stale questions prolong the fix.
* Bugs filed against the current Beta line are fixed in the next Beta/RC
  patch and noted in :doc:`/release-notes`.

.. seealso::

   :doc:`/status/index` — the status vocabulary (what counts as a bug vs.
   a planned feature).
   :doc:`/reference/diagnostics` — the error-code contract.
   :doc:`/status/beta` — what the Beta stable core claims.
   :doc:`/getting-started/installation` — how to get the exact build you
   are testing.