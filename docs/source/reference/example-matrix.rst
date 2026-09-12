Example Capability Matrix
=========================

One-page summary of what exists in the Phase 114 example corpus and which
capability it demonstrates. Statuses follow the site vocabulary
(:doc:`/status/index`). "Engine" lists the engines that reproduce the pinned
output byte-identically (Go = Go front end; kcc = self-hosted compiler,
default).

.. list-table:: Example corpus capability matrix
   :widths: 18 14 14 54
   :header-rows: 1

   * - Capability
     - Status
     - Engine
     - Corpus
   * - Hello world / printing
     - Implemented
     - both
     - ``examples/01_fundamentals/01_hello_world.kark``
   * - Variables & const
     - Implemented
     - both
     - ``01_fundamentals/02_variables``, ``03_constants``
   * - Functions & recursion
     - Implemented
     - both
     - ``01_fundamentals/04_functions``, ``02_algorithms/06_fibonacci``
   * - Conditionals & loops
     - Implemented
     - both
     - ``01_fundamentals/05_conditionals``, ``06_loops``
   * - Arrays & strings
     - Implemented
     - both
     - ``01_fundamentals/07_strings``, ``08_arrays``
   * - Maps
     - Implemented
     - both
     - ``01_fundamentals/09_maps``, ``05_data/01_word_frequency``
   * - Structs (records)
     - Implemented
     - both
     - ``01_fundamentals/10_structs``
   * - Match / Options
     - Implemented
     - both
     - ``01_fundamentals/11_match``
   * - Numeric casts
     - Implemented
     - both
     - ``01_fundamentals/12_casts``
   * - Search & sort
     - Implemented
     - both
     - ``02_algorithms/01–04, 09–10``
   * - File I/O
     - Implemented
     - both
     - ``03_systems/01_file_io``, ``05_data/*``
   * - Encoding & crypto
     - Implemented
     - both
     - ``14_security/*``
   * - Scientific numerics
     - Implemented
     - both
     - ``12_scientific_computing/*``
   * - Synthetic finance
     - Implemented
     - both
     - ``13_finance/*``
   * - AI / ML (hand-rolled)
     - Implemented
     - both
     - ``09_ai/*``, ``10_machine_learning/*``
   * - Concurrency
     - Experimental
     - Go only
     - ``08_concurrency/*``, ``examples/concurrency/pipeline``
   * - WASM target
     - Experimental
     - Go only
     - ``examples/wasm/hello.kark``
   * - Networking / HTTP
     - Not Yet Implemented
     - —
     - —
   * - Database
     - Not Yet Implemented
     - —
     - —
   * - Web framework
     - Not Yet Implemented
     - —
     - —
   * - Quantum
     - Not Yet Implemented (infrastructure only)
     - —
     - —

.. seealso::

   :doc:`/examples/index` — per-category pages with excerpts and expected
   output.
   :doc:`/status/index` — the status vocabulary.