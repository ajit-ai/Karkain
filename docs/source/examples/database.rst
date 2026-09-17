Database
========

:implemented:`Implemented` — ``std.db`` provides an in-memory SQL-text
database engine callable from ``.kark`` on both engines, with file-backed
persistence through ``std.io`` text snapshots.

The ``examples/06-database/`` category contains two runnable examples
demonstrating CRUD operations and file-backed persistence.

.. list-table:: Examples
   :widths: 40 60

   * - ``01_db_crud.kark``
     - In-memory CRUD: create/insert/select/update/delete plus query results
       and error reporting.
   * - ``02_db_persist.kark``
     - File-backed persistence: open, save the catalog, close, reopen and
       re-query.

Implemented ``std.db`` surface
------------------------------

.. code-block:: kark

   std.db (in-memory, SQL-text engine):
     db_engine() -> engine name
     db_backend() -> backend name
     db_open(path?) -> database        # ":memory:" or a file snapshot path
     db_execute(db, sql) -> ok
     db_query(db, sql) -> rows         # SELECT results
     db_result(db) -> last rowset
     db_last_error(db) -> error text   # "" when the last call succeeded
     db_last_count(db) -> affected/row count
     db_create(db, sql)                # CREATE TABLE
     db_insert(db, sql)                # INSERT INTO
     db_select(db, sql)                # SELECT
     db_update(db, sql)                # UPDATE
     db_delete(db, sql)                # DELETE
     db_drop(db, sql)                  # DROP TABLE
     db_begin(db) -> snapshot
     db_commit(db)
     db_rollback(db, snapshot)
     db_prepare(db, sql) -> statement
     db_execute_stmt(db, statement)
     db_format(db) -> text
     db_save(db)                       # persist to the open path
     db_close(db)
     db_load(db, content)              # reload from text

The engine understands a small, honest SQL subset (CREATE TABLE, INSERT INTO,
SELECT, UPDATE, DELETE, DROP TABLE) over pipe-delimited text catalogs; real
indexed/multi-field DB engines are planned for a later phase. File-backed
persistence uses ``std.io`` text snapshots.

.. seealso::

   :doc:`/examples/data` — the adjacent, implemented file-based data category.