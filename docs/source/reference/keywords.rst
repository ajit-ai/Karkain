.. _keywords:

=========
Keywords
=========

Karkain reserves the following words. They cannot be used as identifiers
(function names, variable names, field names, etc.). This table is
authoritative against the lexers (``pkg/lexer/lexer.go``,
``src/compiler/lexer.kark``); a word is a keyword only if a lexer emits
it as one.

Complete keyword list
=====================

.. list-table::
   :widths: 30 70
   :header-rows: 1

   * - Keyword
     - Purpose
   * - ``func``
     - Function declaration
   * - ``fn``
     - Function declaration / lambda closure
   * - ``let``
     - Immutable variable declaration
   * - ``var``
     - Mutable variable declaration
   * - ``const``
     - Constant declaration (compile-time evaluated)
   * - ``type``
     - Type declaration
   * - ``struct``
     - Struct declaration
   * - ``enum``
     - Enum declaration
   * - ``linear``
     - Linear (exactly-once) type modifier
   * - ``packed``
     - Padding-free struct modifier
   * - ``if``
     - Conditional expression / statement
   * - ``else``
     - Alternate branch of ``if``
   * - ``while``
     - Loop (documented form ``while (cond)``)
   * - ``for``
     - Loop (C-style or ``for-in``)
   * - ``in``
     - Iteration in ``for-in``
   * - ``return``
     - Return from function
   * - ``break``
     - Exit innermost loop
   * - ``continue``
     - Skip to next iteration
   * - ``match``
     - Pattern matching (arms use ``=>``)
   * - ``import``
     - Module import
   * - ``public``
     - Public export modifier
   * - ``true``
     - Boolean literal
   * - ``false``
     - Boolean literal
   * - ``Some`` / ``None``
     - ``Option`` constructors (capitalized)
   * - ``Ok`` / ``Err``
     - ``Result`` constructors (capitalized)
   * - ``print`` / ``println``
     - Output (same token)
   * - ``alloc`` / ``free`` / ``addr``
     - Raw memory operations
   * - ``mut`` / ``raw`` / ``move``
     - Borrow and ownership modifiers
   * - ``spawn``
     - Spawn a concurrent task
   * - ``receive``
     - Receive from a channel
   * - ``channel``
     - Create a channel
   * - ``actor``
     - Create an actor
   * - ``send``
     - Send a message on a channel (reserved)
   * - ``matrix``
     - Matrix declaration
   * - ``kernel`` / ``device`` / ``global_id`` / ``barrier``
     - GPU compute surface (reserved)
   * - ``qreg`` / ``gate`` / ``measure``
     - Quantum surface (reserved)
   * - ``macro`` / ``quote`` / ``unquote`` / ``comptime``
     - Metaprogramming surface (reserved)

.. note::

   These look like keywords but are **not**: ``nil`` (use ``None``),
   ``assert`` / ``assert_eq`` / ``assert_ne`` / ``test`` / ``wait_all``
   (builtins and test-runner conventions), ``actorSend`` /
   ``actorState`` / ``setActorState`` / ``actorStop`` (library
   functions). Only the table above is reserved.

Type keywords
=============

These are reserved as type names and cannot be used as identifiers:

.. list-table::
   :widths: 20 30
   :header-rows: 1

   * - Keyword
     - Meaning
   * - ``int``
     - 64-bit signed integer
   * - ``float64``
     - 64-bit IEEE-754 float
   * - ``bool``
     - Boolean
   * - ``string``
     - UTF-8 byte string
   * - ``array``
     - Dynamic array type
   * - ``bigint``
     - Arbitrary-precision integer (GMP-backed)
   * - ``bigfloat``
     - Arbitrary-precision float (GMP-backed)
