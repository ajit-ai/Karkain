.. _errors:

Errors
======

Karkain distinguishes two kinds of error:

* **Recoverable** conditions are values, not exceptions. A function returns an
  ``Option`` or a ``Result``; the caller inspects it with ``match`` or
  propagates it with ``?``. These are decided at check time.
* **Unrecoverable** conditions abort the program at runtime and report a source
  location (see `Runtime errors`_ below).

Result and Option
-----------------

Karkain has no null and no exceptions. The two carrier types are part of the
language itself — they are not standard-library wrappers, and they are written
**without type arguments**:

.. code-block:: kark

   func first(xs: int[]) -> Option {
       if len(xs) == 0 { return None }
       return Some(xs[0])
   }

   func parse(s: string) -> Result {
       if s == "" { return Err("empty") }
       return Ok(42)
   }

``Option``
  ``Some(v)`` for a present value, ``None`` for absence. Written bare as
  ``Option``.

``Result``
  ``Ok(v)`` for success, ``Err(e)`` for failure. Written bare as ``Result``.

``std.core`` provides the predicates ``is_some`` / ``is_none`` / ``is_ok`` /
``is_err``. The constructors themselves are language forms, and both engines
lower them to the same value layout (``optVal.tag``: 0 = ``None``, 1 =
``Some``; ``resVal.tag``: 0 = ``Ok``, 1 = ``Err``).

Consuming a value with ``match``
--------------------------------

``match`` destructures the payload and **binds it to a name** that is in scope
inside that arm only:

.. code-block:: kark

   let r = parse("42")
   match r {
       Ok(n)  => { print(n) },
       Err(e) => { print(e) },
   }

.. code-block:: kark

   let o = first([1, 2, 3])
   match o {
       Some(v) => { print(v) },
       None    => { print("empty") },
   }

Bind the scrutinee to a local first. ``match parse("42") { ... }`` — a call
expression directly in scrutinee position — is a pre-existing Go parser
limitation and does not parse there; it works when the value is a variable.

An arm body is a block, a ``print``, or a bare expression. All three forms are
semantically checked, with the arm's binding in scope, on both engines. The
binding does **not** escape its arm — referring to it after the ``match`` is an
error.

Enum arms are different: they are tag-only and carry no binding. See
:ref:`control-flow` for ``match`` as a control-flow construct, and :ref:`enums`
for enum patterns.

Exhaustiveness
~~~~~~~~~~~~~~

The Go front end enforces exhaustiveness for ``Option`` and ``Result``: a match
must cover both ``Some`` and ``None``, or both ``Ok`` and ``Err``, otherwise it
reports ``non-exhaustive match on Result: missing pattern(s): ...``.

The self-hosted kcc checker does **not** implement this check, so an incomplete
match passes ``kcc check``. It is not a crash: if no arm matches, the match
evaluates to the *matched value itself*, because that is what the generated code
initialises the result to before testing any arm. Write exhaustive matches
anyway — an unmatched arm silently yielding the scrutinee is easy to mistake for
a correct result.

Error propagation with ``?``
----------------------------

``?`` unwraps a ``Result``/``Option`` or returns early. The operand is evaluated
exactly once:

.. code-block:: kark

   func double_it(s: string) -> Result {
       let v = parse(s)?
       return Ok(v * 2)
   }

If the operand is ``Err`` or ``None`` it becomes the enclosing function's return
value immediately; otherwise the ``Ok``/``Some`` payload is unwrapped and the
whole expression evaluates to it. Equivalently, ``expr?`` behaves like
``match expr { Ok(v) => v, Err(e) => return Err(e) }``.

``?`` must appear inside a function that itself returns a ``Result`` or an
``Option``. Using it in ``main`` is a compile error **on both engines** — that
is parity, not a gap.

Runtime errors
--------------

Runtime errors report a **source location** and exit with code 1:

.. code-block:: text

   runtime error: division by zero at main.kark:5
   runtime error: index out of range at main.kark:12
   runtime error: string index out of range at main.kark:18

Checked operations
~~~~~~~~~~~~~~~~~~

The following operations are bounds-checked and produce a runtime error
on failure:

* **Division / modulo by zero**
* **Array index out of range**
* **String index out of range**

Unchecked operations silently return zero (pre-Phase-100 behaviour).

Stack traces (Phase 101)
~~~~~~~~~~~~~~~~~~~~~~~~

When a runtime error occurs the engine prints a **stack trace** showing
the call chain up to ``KARKAIN_MAX_FRAMES`` (128) deep:

.. code-block:: text

   runtime error: division by zero at main.kark:3
   stack:
     inner:2
     outer:5
     main:9

The stack is recorded by ``karkain_frame_enter`` /
``karkain_frame_leave`` calls emitted around every function body.

Cross-engine parity
-------------------

Both the Go front end and the self-hosted kcc engine produce **identical**
runtime-error messages and stack traces for the same input, and identical
semantics for ``Result``/``Option``, ``match`` arm bindings and ``?``
propagation.