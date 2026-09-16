.. _stdlib-io:

std.io — File and stream I/O
============================

:implemented:`Implemented` — importable on both engines, byte-identical.

``std.io`` provides standard file and stream routines built on Karkain's
runtime file builtins: ``readFile``, ``writeFile``, ``openFile``, ``readLine``,
``readLineEOF``, ``createFile``, ``writeToFile``, ``removeFile``, ``listFiles``,
``closeFile`` and ``system``. There is no libc interop: everything available
here works through the documented builtin boundary on both engines.

All functions are implemented in pure canonical Karkain with untyped
parameters.

Importing
---------

.. code-block:: karkain

   import std.io

Module reference
----------------

Reading
~~~~~~~

``io_read_all(path)``
   Reads the complete contents of ``path`` into a string. Returns ``""`` if
   the file does not exist or cannot be read.
   Returns: ``string``

``io_read_line(path)``
   Reads one line (newline stripped) from ``path``. Repeated calls on the
   same path continue from the shared runtime file handle; returns ``""`` at
   end of file.
   Returns: ``string``

``io_read_lines(path)``
   Reads every line from ``path`` into an array of strings. Newlines and
   carriage returns are stripped.
   Returns: ``string[]``

``io_list_dir(path)``
   Lists the names of the entries in directory ``path`` (files and
   subdirectories, excluding ``.`` and ``..``).
   Returns: ``string[]``

Writing
~~~~~~~

``io_write_file(path, data)``
   Writes ``data`` to ``path``, replacing any existing content. Returns ``1``
   on success, ``0`` on failure.
   Returns: ``int``

``io_append_file(path, data)``
   Appends ``data`` to ``path``. Implemented as read-modify-write because the
   runtime has no append-mode builtin; for large files prefer building the
   full content and writing once. Returns ``1`` on success.
   Returns: ``int``

``io_create_file(path)``
   Creates an empty file at ``path``. Returns ``path`` on success or ``""``
   if the file could not be created.
   Returns: ``string``

File lifecycle
~~~~~~~~~~~~~~

``io_file_exists(path)``
   Returns ``true`` if ``path`` names an existing regular file that can be
   opened for reading.
   Returns: ``bool``

``io_delete_file(path)``
   Deletes the file at ``path``. Returns ``true`` on success.
   Returns: ``bool``

``io_close(path)``
   Closes the shared runtime file handle for ``path`` (a no-op if not open).
   Returns: the result of the underlying ``closeFile``.

Process
~~~~~~~

``io_run(command)``
   Executes a shell command and returns its exit status. Delegates to the
   ``system`` builtin.
   Returns: ``int``

Windows note
-------------

Call ``io_close`` before ``io_delete_file`` on Windows, or the operating
system retains the open handle and the delete fails (the file stays locked).
The same applies before removing files you have just read or written.

Example
-------

.. code-block:: karkain

   import std.io

   func main() {
       io_write_file("data.txt", "line one\nline two\n")
       println(io_file_exists("data.txt"))   // true
       println(io_read_all("data.txt"))      // line one\nline two\n
       println(len(io_read_lines("data.txt"))) // 2
       io_close("data.txt")                  // release the handle first
       io_delete_file("data.txt")            // guaranteed on Windows
       println(io_file_exists("data.txt"))   // false
   }