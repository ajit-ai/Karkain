.. _stdlib-os:

std.os — OS metrics and process enumeration
============================================

v1.2.19 adds a real OS metric and process surface on the C23 path. Both the
Go front end and the self-hosted ``kcc`` engine compile the same canonical
Karkain wrappers in ``stdlib/os/os.kark``.

.. note::

   The native and WASM targets have no OS process/metric syscalls. They refuse
   these names by the existing stdlib/platform boundaries; see
   :doc:`../targets/host-targets`.

Functions
---------

``os_current_pid()``
    Current process ID (int).

``os_cpu_total()``
    Number of online CPUs (int, at least 1).

``os_memory_total_mb()``
    Total physical memory in MiB (int).

``os_memory_available_mb()``
    Currently available physical memory in MiB (int). Returns -1 with an
    error on macOS.

``os_uptime()``
    Monotonic system uptime in milliseconds (int).

``os_processes()``
    Array of ``"<pid>:<name>"`` strings, sorted, capped at 64 entries, and
    guaranteed to contain this process.

``os_error()``
    Last OS failure string (empty on success).

Platform notes
--------------

Windows uses Win32 (``kernel32``, Toolhelp) and needs no new link library.
Linux uses ``/proc/meminfo``, ``/proc/<pid>/comm``, ``sysconf`` and
``clock_gettime``. macOS reports total memory and uptime, but available memory
is -1 and only the current process is listed; there is no ``/proc``.

The values are host-dependent, so gates assert predicates (positive PID and
uptime, nonnegative bounded memory, self-containing sorted list), never
golden numbers.

Example
-------

.. code-block:: kark

   import std.os

   func main() {
       print(os_current_pid())
       print(os_cpu_total())
       print(os_memory_total_mb())
       let list = os_processes()
       print(len(list))
   }
