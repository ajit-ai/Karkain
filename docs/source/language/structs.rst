.. _structs:

Structs
=======

Declaration
-----------

.. code-block:: kark

   struct Counter {
       value: int
   }

Fields are separated by ``;`` (not commas):

.. code-block:: kark

   struct Wallet {
       owner: string;
       balance: int
   }

Instantiation
-------------

.. code-block:: kark

   let c = Counter{ value: 0 }
   print(c.value)        // 0

Record idiom (methods)
----------------------

Karkain does not have a native ``method`` keyword.  The **record idiom**
passes the receiver as the first parameter to a top-level function:

.. code-block:: kark

   struct Counter {
       value: int
   }

   func deposit(acc, amount) {
       acc.value = acc.value + amount
   }

   let c = Counter{ value: 0 }
   deposit(c, 10)
   print(c.value)        // 10

The receiver and parameter types are **untyped** — the call
``deposit(c, 10)`` resolves the same as ``deposit(c, 10)`` with the
struct instance passed by reference (mutated in-place).

Internal representation
-----------------------

Structs are represented internally as **map-based containers** with
field names as string keys.  This means struct field access is
equivalent to map key lookup under the hood.
