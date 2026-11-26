std.db
======

``std.db`` exposes two database layers:

1. the original pure-Karkain SQL-text engine (``db2`` / ``embedded``), which
   is deterministic and immutable-snapshot based;
2. v1.2.22's real SQLite driver (``db_sqlite_*``), available on the C23 path.

The SQL-text engine is backend-neutral and does not require an external
library. The SQLite driver vendors SQLite 3.46.1 in-tree.

SQL-text engine
---------------

Database value shape:

``{ path, tables, ok, error, count, cols, rows, tx }``

Supported SQL subset:

```text
CREATE TABLE name ( col1, col2, ... )
INSERT INTO name VALUES ( v1, v2, ... )
UPDATE name SET col = value WHERE col = value
DELETE FROM name WHERE col = value
SELECT * FROM name
SELECT col1, col2 FROM name
DROP TABLE name
```

Keywords are case-insensitive; identifiers are case-sensitive. Values are
typed by shape; there is no parameter binding, no joins, only one equality
WHERE clause, and pipe-delimited persistence cannot store ``|`` or newlines.

Functions:

``db_engine()`` / ``db_backend()`` / ``db_open(path)`` / ``db_close(db)`` /
``db_save(db)`` / ``db_load(content)`` / ``db_format(db)`` /
``db_execute(db, sql)`` / ``db_query(db, sql)`` / ``db_result(db)`` /
``db_last_error(db)`` / ``db_last_count(db)`` /
``db_create(db, name, cols)`` / ``db_insert(db, name, values)`` /
``db_select(db, name, cols, where)`` / ``db_update(db, name, col, value, where_col, where_value)`` /
``db_delete(db, name, col, value)`` / ``db_drop(db, name)`` /
``db_begin(db)`` / ``db_commit(db)`` / ``db_rollback(db, snapshot)`` /
``db_prepare(db, sql)`` / ``db_execute_stmt(db, stmt)``.

SQLite driver (v1.2.22)
-----------------------

The real driver is C23-only on both the Go front end and kcc. Native and WASM
have no FFI/SQLite linkage and refuse the surface by name.

``db_sqlite_open(path)``
    Open ``":memory:"`` or a file-backed database; returns a positive handle
    or 0 on failure.

``db_sqlite_exec(handle, sql)``
    Execute complete SQL text; returns 0 on success or a SQLite status code.

``db_sqlite_query(handle, sql)``
    Return an array of rows; each row is an array of integer/float/string
    cells.

``db_sqlite_close(handle)``
    Close the handle; returns 0 on success.

``db_sqlite_error()``
    Return the last ``sqlite:``-prefixed diagnostic, or empty on success.

Example
-------

.. code-block:: kark

   import std.db

   func main() {
       let h = db_sqlite_open(":memory:")
       db_sqlite_exec(h, "CREATE TABLE t (id INTEGER, name TEXT)")
       db_sqlite_exec(h, "INSERT INTO t VALUES (1, 'alice')")
       let rows = db_sqlite_query(h, "SELECT id, name FROM t")
       print(len(rows))
       print(rows[0][0])
       print(rows[0][1])
       print(db_sqlite_close(h))
   }

See ``examples/06-database/`` for SQL-text CRUD and persistence examples.
