Database
========

:not-implemented:`Not Yet Implemented` — there are no database APIs in the
Karkain language or standard library.

The category directory ``examples/06_database/`` documents only an intended
roadmap shape and carries no example source. File-based persistence is
already available through ``std.io`` — see :doc:`/examples/data` and
``examples/03_systems/01_file_io.kark``.

Intended future surface (roadmap only, **not** implemented):

.. code-block:: kark

   std.db:
     db_connect(dsn) -> connection
     db_execute(conn, sql) -> result
     db_query(conn, sql) -> rows
     db_close(conn)

The corpus must never imply unsupported database functionality exists.

.. seealso::

   :doc:`/examples/data` — the adjacent, implemented file-based data category.
   :doc:`/status/planned` — the roadmap vocabulary.