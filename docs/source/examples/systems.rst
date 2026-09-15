Systems
=======

:implemented:`Implemented` for these specific examples — real, checked-in
systems programs demonstrate the file, concurrency and WASM surfaces of
Karkain:

* ``examples/03-systems/01_file_io.kark`` — the importable ``std.io`` module
  (write / append / read lines / exists / delete), byte-identical on **both**
  engines.
* ``examples/concurrency/pipeline/main.kark`` — the Phase 107 concurrency
  runtime (spawn/join, channels, actors), validated end-to-end by
  ``pkg/cli/phase107_concurrency_test.go`` (Go engine).
* ``examples/wasm/`` — the Phase 108 ``wasm32-wasi`` target hardened to a
  :production-candidate:`Production Candidate` in Phase 123, gated by
  ``pkg/cli/phase123_cli_test.go`` (five corpus programs: ``hello.kark``
  plus ``functions``, ``control_flow``, ``data``, ``strings_builtin``).

File I/O (new)
--------------

``examples/03-systems/01_file_io.kark`` creates ``corpus_demo.txt`` in its
working directory, writes lines, appends a line, reads them back, and deletes
the file — all through ``import std.io``:

.. code-block:: kark

   import std.io

   func main() {
       io_write_file("corpus_demo.txt", "alpha\nbeta\n")
       io_append_file("corpus_demo.txt", "gamma\n")
       let lines = io_read_lines("corpus_demo.txt")
       print(io_file_exists("corpus_demo.txt"))
       print(len(lines))
       let i = 0
       while (i < len(lines)) {
           print(lines[i])
           i = i + 1
       }
       io_delete_file("corpus_demo.txt")
       print(io_file_exists("corpus_demo.txt"))
   }

Expected output (verified, byte-identical on both engines):

.. code-block:: text

   true
   3
   alpha
   beta
   gamma
   true

Run it from anywhere — files are created in the current working directory:

.. code-block:: console

   $ karkain run examples/03-systems/01_file_io.kark

Concurrency pipeline
--------------------

``examples/concurrency/pipeline/main.kark`` is the end-to-end concurrency
demo: a spawned task joined for its result, a producer task streaming over a
channel (with deterministic close-drain), and an actor accumulating state
through three serialized messages.

Caveat: the concurrency language surface runs on the **Go engine**. kcc
parity is deferred, so the example is compiled and run with the Go front end
(``karkain run --engine go``).

.. code-block:: kark

   func work(n) {
   	return n * n
   }

   func producer(ch) {
   	chanSend(ch, 10)
   	chanSend(ch, 20)
   	chanSend(ch, 30)
   	chanClose(ch)
   }

   func counter(state, msg) {
   	return state + msg
   }

   func main() {
   	let t = spawn(work, 12)
   	println(join(t))

   	let ch = channel()
   	spawn(producer, ch)
   	println(receive(ch))
   	println(receive(ch))
   	println(receive(ch))
   	println(receive(ch))

   	let a = actor("counter", 0)
   	actorSend(a, 1)
   	actorSend(a, 2)
   	actorSend(a, 3)
   	wait_all()
   	println(actorState(a))
   	actorStop(a)

   	wait_all()
   }

Expected output (verified, byte-identical across repeated runs):

.. code-block:: text

   144
   10
   20
   30
   0
   6

Reading: ``144`` is ``spawn(work, 12)`` joined (12²); ``10 20 30`` are the
channel messages; ``0`` is the read after close + drain (deterministic
close); ``6`` is the actor state after ``counter(a,1) counter(a,2)
counter(a,3)``.

See :doc:`/language/concurrency` for the full concurrency surface and the
Phase 107 report under ``docs/audit/PHASE-107-CONCURRENCY-RUNTIME-FINAL-REPORT.md``.

WASM hello
----------

``examples/wasm/hello.kark`` is the ``wasm32-wasi`` target demo:

.. code-block:: kark

   func main() {
       print("hello wasmtime")
       print(42)
       print("done")
   }

Build it with the explicit cross/foreign target and run it under
``wasmtime`` (which must be on ``PATH``):

.. code-block:: console

   $ karkain build examples/wasm/hello.kark --target wasm32-wasi
   $ wasmtime run <output>.wasm

Expected output (verified byte-exact):

.. code-block:: text

   hello wasmtime
   42
   done

The emitted binary is deterministically byte-identical across builds. The
WASI boundary is implemented end-to-end: ``return N`` from ``main`` becomes
the wasmtime exit code, runtime errors (e.g. division by zero) print
``runtime error: <kind> at <file>:<line>`` to stderr and exit 1, and
``getArgs()`` returns the host CLI arguments from the WASI start bootstrap.
Since Phase 123 the target is a
:production-candidate:`Production Candidate` (Go engine,
``wasmtime``-gated) with a five-program example corpus:
``hello.kark`` plus ``examples/wasm/functions`` (typed functions),
``examples/wasm/control_flow`` (``for``/``while``/``if`` with postfix
``i++``/``n--``), ``examples/wasm/data`` (arrays) and
``examples/wasm/strings_builtin`` (string length). Every unsupported
construct — structs, maps, C-interop, concurrency, modules/imports,
stdlib — is rejected deterministically with ``error K108: feature '...' is
not supported for target wasm32-wasi`` and exit 3, never silently wrong.

See :doc:`/targets/cross-compilation` and :doc:`/status/scope` for the
production-candidate classification.

.. seealso::

   :doc:`/stdlib/io` — the importable I/O module.
   :doc:`/targets/host-targets` — targets supported on this host.
   :doc:`/tools/build` — the ``karkain build`` command and its target flag.