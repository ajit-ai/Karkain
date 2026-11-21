.. _stdlib-time:

std.time — Time and date utilities
=================================

:implemented:`Implemented` — importable on both engines.

``std.time`` provides time and date utilities using system calls.
Platform-dependent behavior may vary.

Importing
---------

.. code-block:: karkain

   import std.time

Module reference
----------------

``time_now()``
   Returns the current Unix timestamp as a string.
   Returns: ``string``

``time_now_ms()``
   Returns the current time in milliseconds.
   Returns: ``string``

``time_format(ts, fmt)``
   Formats a Unix timestamp using the given format string.
   Returns: ``string``

``time_today()``
   Returns today's date in YYYY-MM-DD format.
   Returns: ``string``

``time_str()``
   Returns the current time in HH:MM:SS format.
   Returns: ``string``

``time_year()``
   Returns the current year.
   Returns: ``string``

``time_month()``
   Returns the current month (01-12).
   Returns: ``string``

``time_day()``
   Returns the current day of month (01-31).
   Returns: ``string``

``time_weekday()``
   Returns the day of week.
   Returns: ``string``

``time_sleep(seconds)``
   Pauses execution for the given number of seconds.
   Returns: ``void``

``time_measure(cmd)``
   Returns the elapsed time for a command.
   Returns: ``string``

Example
-------

.. code-block:: karkain

   import std.time

   func main() {
       print(time_today())    // 2026-11-21
       print(time_year())     // 2026
       print(time_now())      // 1795224994
   }