Concurrency
===========

:experimental:`Experimental` — concurrency is **partially implemented**: the
Phase 107 runtime (work-stealing scheduler, tasks, channels, actors) works
end-to-end through the **Go front end only**. The default ``kcc`` engine has
parser/sema support for the keywords but no codegen parity yet, so these
examples must run with ``--engine go``.

Corpus: ``examples/08-concurrency/``

.. list-table:: examples/08-concurrency/
   :widths: 32 68
   :header-rows: 1

   * - File
     - Demonstrates
   * - ``01_parallel_sum.kark``
     - Work-grid: ``spawn`` one task per lane, ``join`` each result,
       accumulate (Σ 0²…9² = 285)
   * - ``02_channel_ping.kark``
     - Producer task streams messages over a channel; fixed-count reads with
       deterministic close-drain

Parallel sum (excerpt)
----------------------

.. code-block:: kark

   func square(i) {
       return i * i
   }

   func main() {
       let tasks = []
       let n = 0
       while (n < 10) {
           push(tasks, spawn(square, n))
           n = n + 1
       }
       let total = 0
       let i = 0
       while (i < len(tasks)) {
           total = total + join(tasks[i])
           i = i + 1
       }
       print(total)
       wait_all()
   }

Expected output (verified): ``285`` on repeated runs.

Run them through the Go engine:

.. code-block:: console

   $ karkain run examples/08-concurrency/01_parallel_sum.kark --engine go
   $ karkain run examples/08-concurrency/02_channel_ping.kark --engine go

Why experimental
----------------

* Concurrency runs only on the Go front end; ``kcc`` codegen parity is a
  documented post-107 boundary.
* Scheduler events are timing-dependent at the machine level (worker count,
  steal order), so only join-serialized or close-drain-deterministic output
  is pinned by gates.

.. seealso::

   :doc:`/language/concurrency` — the language surface and runtime
   semantics.
   :doc:`/status/experimental` — why this is labeled experimental.
   :doc:`/examples/systems` — the concurrency pipeline demo.