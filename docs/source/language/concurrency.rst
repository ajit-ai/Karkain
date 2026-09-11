.. _concurrency:

Concurrency
===========

.. versionadded:: Phase 107

Karkain provides concurrency primitives backed by a **work-stealing
task scheduler** and a C runtime (``runtime/concurrency/c/``).

.. note::

   Concurrency is implemented and tested on the **Go front end**.
   The self-hosted kcc engine already lexes and parses the keywords;
   full codegen parity is a documented post-107 boundary.

Tasks — ``spawn`` / ``join`` / ``wait_all``
--------------------------------------------

``spawn(fn, args...)``
  Launches *fn* as an asynchronous task.

``wait_all()``
  Blocks until every spawned task has completed.

.. code-block:: kark

   func work(n): int {
       return n * 2
   }

   spawn(work, 10)
   spawn(work, 20)
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

``actor(handler, initialState)``
  Creates an actor with the given handler function and initial state.

``actorSend(actor, message)``
  Sends a message to the actor's mailbox.

``actorState(actor)``
  Returns the actor's current state.

``setActorState(actor, newState)``
  Replaces the actor's state.

``actorStop(actor)``
  Stops the actor.

.. code-block:: kark

   func counter(msg, state): int {
       return state + 1
   }

   let a = actor(counter, 0)
   actorSend(a, "inc")
   actorSend(a, "inc")
   wait_all()
   print(actorState(a))  // 2

Scheduler internals
-------------------

* Fixed worker pool, one thread per worker.
* Bounded grab queue with mutex + condvar.
* Pending-task drain on shutdown — no busy-spin.
* Actors are serialised dispatcher jobs over a mailbox with a state cell.
