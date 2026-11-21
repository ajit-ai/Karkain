.. _stdlib-env:

std.env — Environment variables
===============================

:implemented:`Implemented` — importable on both engines.

``std.env`` provides environment variable access and process environment
utilities. Uses system calls; platform-dependent behavior may vary.

Importing
---------

.. code-block:: karkain

   import std.env

Module reference
----------------

``env_get(name)``
   Returns the value of an environment variable, or ``""`` if not set.
   Returns: ``string``

``env_set(name, value)``
   Sets an environment variable. Returns ``0`` on success.
   Returns: ``int``

``env_has(name)``
   Returns ``true`` if the environment variable is set.
   Returns: ``bool``

``env_get_or(name, default_val)``
   Returns the value of an env var, or a default if not set.
   Returns: ``string``

``env_home()``
   Returns the user's home directory.
   Returns: ``string``

``env_path()``
   Returns the PATH environment variable.
   Returns: ``string``

``env_temp_dir()``
   Returns the system temporary directory.
   Returns: ``string``

``env_user()``
   Returns the current username.
   Returns: ``string``

``env_shell()``
   Returns the default shell.
   Returns: ``string``

Example
-------

.. code-block:: karkain

   import std.env

   func main() {
       print(env_has("PATH"))     // true
       print(env_temp_dir())      // C:\Windows\Temp (on Windows)
       print(env_user())          // current username
   }