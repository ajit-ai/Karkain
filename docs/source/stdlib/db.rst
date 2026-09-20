std.db
======

Backend-neutral database layer with SQL subset support.

Phase 125A added a functional database layer where every operation returns a
NEW Database value (immutable snapshots). The current backend is ``db2`` (a
small embedded, deterministic file/line store).

.. note::

   The persistence format is a simple deterministic text format:
   ``#karkain-db-v1`` followed by ``c`` table definitions and ``r`` rows.

Database value shape
---------------------

``{ path, tables, ok, error, count, cols, rows, tx }``

* ``path`` — file path or ``:memory:``
* ``tables`` — map: table name → ``{ cols, rows }``
* ``ok`` — last operation succeeded (1 = yes, 0 = no)
* ``error`` — last error message ("" on success)
* ``count`` — last operation affected/returned row count
* ``cols`` — last SELECT column names
* ``rows`` — last SELECT rows (decoded values)
* ``tx`` — 1 inside a read-committed transaction, else 0

SQL subset
----------

Keywords are case-insensitive; table/column names are not.

``CREATE TABLE name ( col1, col2, ... )``
``INSERT INTO name VALUES ( v1, v2, ... )``
``UPDATE name SET col = value WHERE col = value``
``DELETE FROM name WHERE col = value``
``SELECT * FROM name``
``SELECT col1, col2 FROM name``
``DROP TABLE name``

.. note::

   Values are typed by shape at insert time. No parameter binding, no joins,
   one WHERE clause. Values may not contain ``|`` or newlines (pipe-delimited
   format).

Core functions
--------------

``db_engine()`` 
    Report current backend name
``db_backend(name)`` 
    Select backend (returns new Database value)
``db_open(path)`` 
    Open database (returns Database value)
``db_close(db)`` 
    Close database
``db_save(db)`` 
    Save database to file
``db_load(path)`` 
    Load database from file

Table operations
----------------

``db_create_table(db, name, cols)`` 
    Create table
``db_drop_table(db, name)`` 
    Drop table
``db_table_exists(db, name)`` 
    Check if table exists
``db_list_tables(db)`` 
    List table names

Data operations
----------------

``db_insert(db, name, values)`` 
    Insert row(s)
``db_select(db, name, cols, where)`` 
    SELECT query
``db_update(db, name, set, where)`` 
    UPDATE query
``db_delete(db, name, where)`` 
    DELETE query
``db_query(db, sql)`` 
    Execute raw SQL query

Transaction support
-------------------

``db_begin(db)`` 
    Begin transaction
``db_commit(db)`` 
    Commit transaction
``db_rollback(db)`` 
    Rollback transaction

Utility functions
-----------------

``db_format_row(values)`` 
    Format values as pipe-delimited row
``db_parse_row(line)`` 
    Parse pipe-delimited row
``db_format_sql(sql)`` 
    Format SQL for persistence
``db_last_error(db)`` 
    Get last error message
``db_last_count(db)`` 
    Get last operation row count

Example
-------

.. code-block:: kark

   import std.db

   func main() {
       let db = db_open(":memory:")
       db = db_create_table(db, "users", ["id", "name"])
       db = db_insert(db, "users", ["1", "alice"])
       db = db_save(db)
   }

See ``examples/06-database/`` for CRUD and persistence examples.
