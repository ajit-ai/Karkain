.. _stdlib-log:

std.log — Structured logging
============================

:implemented:`Implemented` — importable on both engines.

``std.log`` provides structured logging utilities with level prefixes
and optional timestamps.

Importing
---------

.. code-block:: karkain

   import std.log

Module reference
----------------

``log_info(msg)``
   Prints an info-level message.
   Returns: ``void``

``log_warn(msg)``
   Prints a warning-level message.
   Returns: ``void``

``log_error(msg)``
   Prints an error-level message.
   Returns: ``void``

``log_debug(msg)``
   Prints a debug-level message.
   Returns: ``void``

``log_format(level, msg, timestamp)``
   Formats a log entry with level, timestamp, and message.
   Returns: ``string``

``log_with_time(level, msg)``
   Prints a log entry with an automatic timestamp.
   Returns: ``void``

Example
-------

.. code-block:: karkain

   import std.log

   func main() {
       log_info("application started")
       log_warn("low memory")
       log_error("connection failed")
       log_with_time("INFO", "request processed")
   }