.. _concurrency:

Concurrency
===========

.. versionadded:: Phase 107

Karkain provides concurrency primitives backed by a **work-stealing
task scheduler** and a C runtime (``runtime/concurrency/c/``).

.. note::

   Concurrency is implemented and tested on **both engines** — the Go
   front end and the self-hosted kcc engine — with byte-identical golden
   output (Phase 137). Task cancellation and timed joins (``cancel``,
   ``join_timeout``) are available on both engines as well.

Tasks — ``spawn`` / ``join`` / ``wait_all``
--------------------------------------------

``spawn(fn, args...)``
  Launches *fn* as an asynchronous task.

``join(task)``
  Blocks until *task* finishes and returns its result value.

``wait_all()``
  Blocks until every spawned task has completed.

.. code-block:: kark

   func work(n): int {
       return n * 2
   }

   let t = spawn(work, 10)
   print(join(t))        // 20
   spawn(work, 20)
   wait_all()

Task control — ``cancel`` / ``join_timeout``
-----------------------------------------------

``cancel(task)``
  Requests cancellation and returns ``true`` when the request was
  accepted. It returns ``false`` for a handle that is already finished,
  already cancelled, or not a task, so repeated calls are safe.

  * A task that has not started yet (**queued**) is skipped entirely: its
    body never runs and joining it reports the cancelled status ``-4``
    (``KARKAIN_TASK_CANCELLED``).
  * A task that is already **running** is not interrupted. Cancellation
    is cooperative and non-preemptive: the running body completes
    normally and its result still stands. Code that must observe
    cancellation does so explicitly (for example by checking a channel
    or a flag), exactly as with any cooperative runtime.

``join_timeout(task, ms)``
  Waits up to *ms* milliseconds for *task* to finish and returns ``true``
  when it completed inside the budget, ``false`` on timeout.

  * A timeout **does not cancel** the task. The task keeps running and
    remains joinable — a later ``join(task)`` or ``join_timeout`` still
    observes completion and its value.
  * ``0`` is a non-blocking completion check.
  * A negative timeout is rejected rather than treated as an infinite
    wait: a literal negative constant is a compile-time diagnostic, and a
    computed negative value makes ``join_timeout`` return ``false``. A
    handle that is not a task is rejected safely and also yields ``false``.
  * Waiting is deadline-based on a monotonic clock; the runtime sleeps on
    the task's condition variable instead of spinning.

.. code-block:: kark

   func worker(gate): int {
       let v = receive(gate)
       return v
   }

   let gate = channel()
   let t = spawn(worker, gate)

   print(join_timeout(t, 10))   // false — still blocked
   chanSend(gate, 7)
   print(join_timeout(t, 5000)) // true  — completed in budget
   print(join(t))               // 7
   wait_all()

Channels
--------

``channel()``
  Creates a new channel (bounded or unbounded).

``chanSend(ch, value)``
  Sends *value* into the channel.

``chanClose(ch)``
  Closes the channel.

``receive(ch)``
  Blocks until a value is available on the channel.

.. code-block:: kark

   let ch = channel()

   func producer():
       chanSend(ch, 42)
       chanClose(ch)

   spawn(producer)

   let val = receive(ch)
   print(val)            // 42

Actors
------

Actors encapsulate state behind a serialised message dispatcher:

``actor(handlerName, initialState)``
  Creates an actor with the given handler function and initial state.
  The handler is passed as a **quoted function name** — ``actor("counter",
  0)`` — and receives ``(state, message)``. The unquoted form
  ``actor(counter, 0)`` is a compile-time diagnostic on both engines.

``actorSend(actor, message)``
  Sends a message to the actor's mailbox.

``actorState(actor)``
  Returns the actor's current state.

``setActorState(actor, newState)``
  Replaces the actor's state.

``actorStop(actor)``
  Stops the actor.

.. code-block:: kark

   func counter(state, msg) {
       return state + msg
   }

   let a = actor("counter", 0)
   actorSend(a, 1)
   actorSend(a, 2)
   wait_all()
   print(actorState(a))  // 3

Scheduler internals
-------------------

* Fixed worker pool, one thread per worker.
* Bounded grab queue with mutex + condvar.
* Pending-task drain on shutdown — no busy-spin.
* Actors are serialised dispatcher jobs over a mailbox with a state cell.
* Timed joins wait on the task's condition variable against a monotonic
  deadline; the remaining budget is recomputed after every wakeup, so a
  spurious wakeup cannot extend a timeout and no wait spins.
* Cancellation is a single atomic request flag. A queued task checks it
  before running and skips its body; a finished task ignores it. Nothing is
  killed mid-execution.
