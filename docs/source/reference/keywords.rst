.. _keywords:

=========
Keywords
=========

Karkain reserves the following words. They cannot be used as identifiers
(function names, variable names, field names, etc.).

Complete keyword list
=====================

.. list-table::
   :widths: 30 70
   :header-rows: 1

   * - Keyword
     - Purpose
   * - ``func``
     - Function declaration
   * - ``let``
     - Immutable variable declaration
   * - ``var``
     - Mutable variable declaration
   * - ``const``
     - Constant declaration (compile-time evaluated)
   * - ``if``
     - Conditional expression / statement
   * - ``else``
     - Alternate branch of ``if``
   * - ``while``
     - Loop (condition must be parenthesized)
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
     - Pattern matching
   * - ``struct``
     - Struct declaration
   * - ``enum``
     - Enum declaration
   * - ``import``
     - Module import
   * - ``public``
     - Public export modifier
   * - ``true``
     - Boolean literal
   * - ``false``
     - Boolean literal
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
