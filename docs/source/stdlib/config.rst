.. _stdlib-config:

std.config — Configuration management
=====================================

:implemented:`Implemented` — importable on both engines.

``std.config`` provides configuration management utilities that read from
environment variables with sensible defaults.

Importing
---------

.. code-block:: karkain

   import std.config

Module reference
----------------

``config_get(key, default_val)``
   Returns the value of an env var, or a default if not set.
   Returns: ``string``

``config_has(key)``
   Returns ``true`` if the env var is set.
   Returns: ``bool``

``config_get_int(key, default_val)``
   Returns an integer config value, or default if not set.
   Returns: ``int``

``config_get_bool(key, default_val)``
   Returns a boolean config value, or default if not set.
   Returns: ``bool``

``config_require(key)``
   Returns the value of a required config, or prints an error if not set.
   Returns: ``string``

Example
-------

.. code-block:: karkain

   import std.config

   func main() {
       string host = config_get("HOST", "localhost")
       int port = config_get_int("PORT", 8080)
       print(host)
       print(port)
   }