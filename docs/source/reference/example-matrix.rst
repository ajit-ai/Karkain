Example Capability Matrix
=========================

One-page summary of what exists in the Phase 114/116 example corpus and
which capability it demonstrates. Statuses follow the site vocabulary
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
     - ``examples/01-fundamentals/01_hello_world.kark``
   * - Variables & const
     - Implemented
     - both
     - ``01-fundamentals/02_variables``, ``03_constants``
   * - Functions & recursion
     - Implemented
     - both
     - ``01-fundamentals/04_functions``, ``02-algorithms/06_fibonacci``
   * - Conditionals & loops
     - Implemented
     - both
     - ``01-fundamentals/05_conditionals``, ``06_loops``
   * - Arrays & strings
     - Implemented
     - both
     - ``01-fundamentals/07_strings``, ``08_arrays``
   * - Maps
     - Implemented
     - both
     - ``01-fundamentals/09_maps``, ``05-data/01_word_frequency``
   * - Structs (records)
     - Implemented
     - both
     - ``01-fundamentals/10_structs``
   * - Match / Options
     - Implemented
     - both
     - ``01-fundamentals/11_match``
   * - Numeric casts
     - Implemented
     - both
     - ``01-fundamentals/12_casts``
   * - Search & sort
     - Implemented
     - both
     - ``02-algorithms/01–04, 09–10``
   * - Stacks, queues & DP
     - Implemented
     - both
     - ``02-algorithms/11_stack``, ``12_queue``, ``13_knapsack``
   * - File I/O
     - Implemented
     - both
     - ``03-systems/01_file_io``, ``05-data/*``
   * - Encoding & crypto
     - Implemented
     - both
     - ``14-security/*``
   * - Scientific numerics
     - Implemented
     - both
     - ``12-scientific-computing/*``
   * - Synthetic finance
     - Implemented
     - both
     - ``13-finance/*``
   * - AI / ML (hand-rolled)
     - Implemented
     - both
     - ``09-ai/*``, ``10-machine-learning/*``
   * - Concurrency
     - Experimental
     - Go only
     - ``08-concurrency/*``, ``examples/concurrency/pipeline``
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