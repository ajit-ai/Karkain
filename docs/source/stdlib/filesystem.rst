.. _stdlib-filesystem:

std.filesystem — File and directory operations
==============================================

:implemented:`Implemented` — importable on both engines.

``std.filesystem`` provides file and directory operations beyond ``std.io``,
using system calls. Platform-dependent behavior may vary.

Importing
---------

.. code-block:: karkain

   import std.filesystem

Module reference
----------------

``fs_read_file(path)``
   Reads the complete contents of a file into a string.
   Returns: ``string``

``fs_write_file(path, data)``
   Writes data to a file.
   Returns: ``int``

``fs_exists(path)``
   Returns ``true`` if the path exists.
   Returns: ``bool``

``fs_is_file(path)``
   Returns ``true`` if the path is a regular file.
   Returns: ``bool``

``fs_is_dir(path)``
   Returns ``true`` if the path is a directory.
   Returns: ``bool``

``fs_mkdir(path)``
   Creates a directory (and parents if needed).
   Returns: ``int``

``fs_remove(path)``
   Deletes a file or directory.
   Returns: ``int``

``fs_copy(src, dst)``
   Copies a file from src to dst.
   Returns: ``int``

``fs_move(src, dst)``
   Moves/renames a file or directory.
   Returns: ``int``

``fs_cwd()``
   Returns the current working directory.
   Returns: ``string``

``fs_temp_dir()``
   Returns the system temporary directory.
   Returns: ``string``

``fs_basename(path)``
   Returns the base name of a path.
   Returns: ``string``

``fs_dirname(path)``
   Returns the directory name of a path.
   Returns: ``string``

``fs_extension(path)``
   Returns the file extension.
   Returns: ``string``

Example
-------

.. code-block:: karkain

   import std.filesystem

   func main() {
       print(fs_exists("main.kark"))    // true
       print(fs_cwd())                  // current directory
       print(fs_temp_dir())             // temp directory
   }