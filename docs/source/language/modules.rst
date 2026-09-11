.. _modules:

Modules
=======

Importing
---------

Standard-library modules are imported with a dotted path:

.. code-block:: kark

   import std.string
   import std.io

Local (project) modules are resolved by the dependency graph — see
``karkain.toml`` manifest and Phase 103 cross-module resolution.

Qualified calls
---------------

Functions imported from a module are called with the module prefix:

.. code-block:: kark

   import std.string

   let s = std.string.to_upper("hello")

The ``public`` modifier controls whether a function or type is exported
from its module:

.. code-block:: kark

   public func helper(): int {
       return 42
   }

   func internal(): int {
       return 0
   }

``internal`` is **not** visible from other modules (private by default).

Phase 103 cross-module resolution
----------------------------------

The resolver resolves qualified calls against the imported module's
export set.  Diagnostic messages cover:

* calling a private function from another module
* missing ``import`` statement
* undefined function on a module
* wrong-module target
* cross-module duplicate definition

.. note::

   The ``karkain test`` driver synthesises a per-file ``main`` and
   currently does **not** resolve cross-module imports during test
   compilation — a known limitation.
