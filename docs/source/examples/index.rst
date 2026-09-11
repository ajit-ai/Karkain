Karkain by Example
==================

Real, runnable ``.kark`` programs that exercise the implemented language,
standard library and toolchain. Every snippet on these pages is taken from a
source file that exists in the repository and has been validated through the
real CLI (``karkain check`` / ``karkain run`` / ``karkain test``).

Everything here follows the site's status vocabulary
(:doc:`/status/index`): a category is only presented as working when it is,
and planned capability is labeled ``Planned`` or ``Not Yet Implemented``
explicitly.

---------------

The 15-category framework
=========================

Examples are organized into 15 categories. Phase 113 establishes the
framework and the pages; Phase 114 will populate the remaining categories
with validated examples.

.. list-table:: Example categories
   :widths: 22 48 30
   :header-rows: 1

   * - Category
     - Status
     - Examples
   * - :doc:`Fundamentals <fundamentals>`
     - :implemented:`Implemented`
     - ``examples/language_foundation/`` (13 targets)
   * - :doc:`Algorithms <algorithms>`
     - :implemented:`Implemented`
     - ``examples/algorithms/`` (21 programs)
   * - :doc:`Systems <systems>`
     - :implemented:`Implemented` (specific examples)
     - ``examples/concurrency/pipeline``, ``examples/wasm/hello``
   * - :doc:`Networking <networking>`
     - :not-implemented:`Not Yet Implemented`
     - —
   * - :doc:`Data <data>`
     - :not-implemented:`Not Yet Implemented`
     - —
   * - :doc:`Database <database>`
     - :not-implemented:`Not Yet Implemented`
     - —
   * - :doc:`Web <web>`
     - :not-implemented:`Not Yet Implemented`
     - —
   * - :doc:`Concurrency <concurrency>`
     - :experimental:`Experimental`
     - ``examples/concurrency/pipeline``
   * - :doc:`AI <ai>`
     - :not-implemented:`Not Yet Implemented`
     - —
   * - :doc:`Machine Learning <machine-learning>`
     - :not-implemented:`Not Yet Implemented`
     - —
   * - :doc:`Quantum <quantum>`
     - :not-implemented:`Not Yet Implemented`
     - —
   * - :doc:`Scientific Computing <scientific-computing>`
     - :not-implemented:`Not Yet Implemented`
     - —
   * - :doc:`Finance <finance>`
     - :not-implemented:`Not Yet Implemented`
     - —
   * - :doc:`Security <security>`
     - :not-implemented:`Not Yet Implemented`
     - —
   * - :doc:`Developer Tools <developer-tools>`
     - :implemented:`Implemented`
     - ``karkain fmt``, ``lint``, ``debug``, ``prof``, LSP, VS Code

.. note::

   Remaining categories will be populated in Phase 114.

Only the three implemented categories have pages in the table of contents
below; the category placeholder pages (``networking``, ``data``, ``database``,
``web``, ``concurrency``, ``ai``, ``machine-learning``, ``quantum``,
``scientific-computing``, ``finance``, ``security``, ``developer-tools``)
exist to document status honestly and will gain examples as the capability
they describe is implemented.

.. toctree::
   :maxdepth: 2
   :caption: Examples by Category

   fundamentals
   algorithms
   systems

---------------

Validation corpus
=================

Two checked-in corpora give these examples their weight:

* **Conformance** — ``conformance/`` contains 11 test files
  (``001_arithmetic_test.kark`` … ``011_namespace_test.kark``) with
  59 ``func test_*`` functions run through the real front end + C runtime on
  both engines.
* **Golden outputs** — the conformance and probes corpora assert
  byte-identical stdout on the Go front end and the self-hosted ``kcc``
  engine (e.g. ``pkg/cli/phase95_parity_test.go``).

Example inventory (repository)
==============================

* ``examples/algorithms/`` — 21 algorithm programs (factorial, fibonacci,
  gcd, lcm, power, sieve, and more).
* ``examples/language_foundation/`` — 13 golden targets that round-trip the
  language surface on both engines: variables, functions, recursion, arrays,
  strings, control flow, structs, maps, types, match, methods, modules, and a
  multi-file application layout.
* ``examples/concurrency/`` — ``pipeline/``, the Phase 107 concurrency
  runtime demo (spawn/join, channels, actors).
* ``examples/wasm/`` — ``hello.kark``, the ``wasm32-wasi`` target demo
  (Phase 108).
* ``examples/stdlib_v2/`` — multi-module end-to-end program importing
  ``std.string``, ``std.collections``, ``std.encoding`` and ``std.crypto``,
  including the ``sha256("karkain")`` digest check (Phase 109).
* ``examples/testing/`` — ``std.testing`` module validation through
  ``karkain test`` (Phase 112).
* ``examples/profiling/`` — ``basic``, ``recursion`` and ``hotspot`` targets
  for ``karkain prof`` (Phase 110).

.. seealso::

   :doc:`/getting-started/project-layout` — where these directories live.
   :doc:`/status/index` — the status vocabulary used throughout this site.