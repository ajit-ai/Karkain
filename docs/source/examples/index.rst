Karkain by Example
==================

Real, runnable ``.kark`` programs that exercise the implemented language,
standard library and toolchain. Every snippet on these pages is taken from a
source file that exists in the repository and has been validated through the
real CLI (``karkain check`` / ``karkain run`` / ``karkain test``); the
Runnable categories are pinned byte-identical on both engines by
``pkg/cli/phase114_examples_test.go``.

Everything here follows the site's status vocabulary
(:doc:`/status/index`): a category is only presented as working when it is,
and planned capability is labeled ``Planned`` or ``Not Yet Implemented``
explicitly.

--------------- 

The 15-category framework
-------------------------

The Phase 114/116/123 corpus populates the framework with 52 ``.kark`` source
files (15 categories, 4 of them deliberately Planned — README only). See the
inventory ``examples/EXAMPLES.md`` for the authoritative file list.

.. list-table:: Example categories
   :widths: 22 48 30
   :header-rows: 1

   * - Category
     - Status
     - Examples
   * - :doc:`Fundamentals <fundamentals>`
     - :implemented:`Implemented`
     - ``examples/01-fundamentals/`` (12)
   * - :doc:`Algorithms <algorithms>`
     - :implemented:`Implemented`
     - ``examples/02-algorithms/`` (13) + ``examples/algorithms/`` (21)
   * - :doc:`Systems <systems>`
     - :implemented:`Implemented` (specific examples)
     - ``examples/03-systems/``, concurrency pipeline, WASM corpus (5)
   * - :doc:`Networking <networking>`
     - :not-implemented:`Not Yet Implemented`
     - —
   * - :doc:`Data <data>`
     - :implemented:`Implemented`
     - ``examples/05-data/`` (4)
   * - :doc:`Database <database>`
     - :not-implemented:`Not Yet Implemented`
     - —
   * - :doc:`Web <web>`
     - :not-implemented:`Not Yet Implemented`
     - —
   * - :doc:`Concurrency <concurrency>`
     - :experimental:`Experimental`
     - ``examples/08-concurrency/`` (2) + pipeline
   * - :doc:`AI <ai>`
     - :implemented:`Implemented` (arithmetic demos)
     - ``examples/09-ai/`` (2)
   * - :doc:`Machine Learning <machine-learning>`
     - :implemented:`Implemented` (arithmetic demos)
     - ``examples/10-machine-learning/`` (2)
   * - :doc:`Quantum <quantum>`
     - :not-implemented:`Not Yet Implemented` (infrastructure only)
     - —
   * - :doc:`Scientific Computing <scientific-computing>`
     - :implemented:`Implemented`
     - ``examples/12-scientific-computing/`` (4)
   * - :doc:`Finance <finance>`
     - :implemented:`Implemented` (synthetic data)
     - ``examples/13-finance/`` (4)
   * - :doc:`Security <security>`
     - :implemented:`Implemented`
     - ``examples/14-security/`` (4)
   * - :doc:`Developer Tools <developer-tools>`
     - :implemented:`Implemented`
     - ``examples/15-developer-tools/`` (2) + CLI suite

.. note::

   AI and Machine Learning are marked ``Implemented`` strictly in the sense of
   *hand-rolled arithmetic demonstrations* — there is **no** ``std.ai`` or ML
   framework surface. Quantum is infrastructure-only: parser wiring is
   explicitly part of its roadmap. See the individual pages for the exact,
   honest claim.

.. toctree::
   :maxdepth: 2
   :caption: Examples by Category

   fundamentals
   algorithms
   systems
   networking
   data
   database
   web
   concurrency
   ai
   machine-learning
   quantum
   scientific-computing
   finance
   security
   developer-tools

--------------- 

How to explore
--------------

The fastest path is the :doc:`/getting-started/first-program`, then run the
corpus:

.. code-block:: console

   $ karkain run examples/01-fundamentals/01_hello_world.kark
   $ powershell -ExecutionPolicy Bypass -File scripts\verify-examples.ps1

``scripts/verify-examples.ps1`` classifies every example by its declared
status (``Runnable`` / ``Experimental`` / ``Planned``) and runs the runnable
ones through the real CLI — 51 examples pass; 5 are deliberately skipped (the
test-mode runner and the four Planned categories).

Validation corpus
-----------------

Three checked-in corpora give these examples their weight:

* **Conformance** — ``conformance/`` contains 11 test files
  (``001_arithmetic_test.kark`` … ``011_namespace_test.kark``) with
  59 ``func test_*`` functions run through the real front end + C runtime on
  both engines.
* **Golden outputs** — the conformance and probes corpora assert
  byte-identical stdout on the Go front end and the self-hosted ``kcc``
  engine (e.g. ``pkg/cli/phase95_parity_test.go``).
* **Phase 114 corpus gate** — ``pkg/cli/phase114_examples_test.go`` pins every
  Runnable example to a golden output on both engines.

Example inventory (repository)
------------------------------

* ``examples/algorithms/`` — 21 algorithm programs (factorial, fibonacci,
  gcd, lcm, power, sieve, and more).
* ``examples/language_foundation/`` — 13 golden targets that round-trip the
  language surface on both engines.
* ``examples/concurrency/`` — ``pipeline/``, the Phase 107 concurrency
  runtime demo (spawn/join, channels, actors).
* ``examples/wasm/`` — the ``wasm32-wasi`` target corpus (Phases 108/123):
  ``hello.kark`` plus ``functions``, ``control_flow``, ``data`` and
  ``strings_builtin``. Go-engine production candidate, gated by
  ``pkg/cli/phase123_cli_test.go``.
* ``examples/stdlib_v2/`` — multi-module end-to-end program importing
  ``std.string``, ``std.collections``, ``std.encoding`` and ``std.crypto``
  (Phase 109).
* ``examples/testing/`` — ``std.testing`` module validation through
  ``karkain test`` (Phase 112).
* ``examples/profiling/`` — ``basic``, ``recursion`` and ``hotspot`` targets
  for ``karkain prof`` (Phase 110).

.. seealso::

   :doc:`/getting-started/project-layout` — where these directories live.
   :doc:`/status/index` — the status vocabulary used throughout this site.