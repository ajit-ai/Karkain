.. _stdlib-process:

std.process — Process management
===============================

:implemented:`Implemented` — importable on both engines.

``std.process`` provides process management and execution utilities.

Importing
---------

.. code-block:: karkain

   import std.process

Module reference
----------------

``proc_run(cmd)``
   Executes a shell command and returns its exit status.
   Returns: ``int``

``proc_run_bg(cmd)``
   Executes a command in the background.
   Returns: ``int``

``proc_output(cmd)``
   Executes a command and returns its output.
   Returns: ``string``

``proc_exit(code)``
   Terminates the program with the given exit code.
   Returns: ``int``

``proc_is_running(name)``
   Checks if a process is running.
   Returns: ``bool``

``proc_pid()``
   Returns the current process ID.
   Returns: ``string``

``proc_kill(name)``
   Kills a process by name.
   Returns: ``int``

``proc_env(name)``
   Returns the value of an environment variable.
   Returns: ``string``

``proc_sleep(seconds)``
   Pauses execution for the given number of seconds.
   Returns: ``int``

``proc_which(cmd)``
   Returns the path of an executable.
   Returns: ``string``

Example
-------

.. code-block:: karkain

   import std.process

   func main() {
       print(proc_pid())        // current process ID
       print(proc_which("go"))  // path to go executable
   }