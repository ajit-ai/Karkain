Data
====

:implemented:`Implemented` — a compact data-handling corpus lives under
``examples/05-data/``: word frequency, CSV-style aggregation, binary/text
payload round-trips and a text token-statistics pipeline. Every file here
runs byte-identically on **both** engines (Go front end and the self-hosted
``kcc`` engine) and is gated by ``pkg/cli/phase114_examples_test.go``.

Data-storage (databases) remains a separate planned category:
:doc:`/examples/database`.

.. list-table:: examples/05-data/
   :widths: 30 70
   :header-rows: 1

   * - File
     - Demonstrates
   * - ``01_word_frequency.kark``
     - Tokenize a text and tally counts in a ``std.collections`` map
       (``map_contains_key`` / ``map_keys`` + ``array_sum``)
   * - ``02_csv_aggregate.kark``
     - CSV-style parse via ``str_split`` / ``str_to_int``, then aggregate
       with ``array_sum`` / ``array_max``
   * - ``03_payload_roundtrip.kark``
     - hex / base64 / UTF-8 byte round-trips through ``std.encoding``
   * - ``04_token_stats.kark``
     - tokenize text with ``std.string``, then filter, transform and
       aggregate in one deterministic pass

Token statistics
----------------

``examples/05-data/04_token_stats.kark`` — a small text-processing pipeline
(token count, long-word filter, per-word character totals):

.. literalinclude:: /../../examples/05-data/04_token_stats.kark
   :language: kark

Expected output (verified on both engines):

.. code-block:: text

   9
   3
   35

Word frequency
--------------

Excerpt from ``examples/05-data/01_word_frequency.kark``:

.. code-block:: kark

   func main() {
       let words = ["karkain", "data", "karkain", "data", "hello"]
       ...
   }

Run any of them directly:

.. code-block:: console

   $ karkain run examples/05-data/01_word_frequency.kark

.. seealso::

   :doc:`/stdlib/collections` and :doc:`/stdlib/encoding` — the stdlib modules
   these examples exercise.
   :doc:`/examples/database` — the not-yet-implemented storage category.