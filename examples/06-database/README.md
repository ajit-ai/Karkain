# 06 - Database

**Status: Runnable**

| Example | Status  | Notes |
|---------|---------|-------|
| `01_db_crud.kark` | Runnable | In-memory CRUD with SQL-text statements |
| `02_db_persist.kark` | Runnable | File-backed persistence, close/reopen round-trip |

## What is implemented

- `std.db` provides an in-memory SQL-text database engine: `db_open`,
  `db_engine`, `db_backend`, `db_execute`, `db_query`, `db_result`,
  `db_last_error`, `db_last_count`, `db_create`, `db_insert`, `db_select`,
  `db_update`, `db_delete`, `db_drop`, `db_begin`, `db_commit`, `db_rollback`,
  `db_prepare`, `db_execute_stmt`, `db_format`, `db_save`, `db_close` and
  `db_load`.
- The engine understands a small SQL subset (CREATE TABLE, INSERT INTO,
  SELECT, UPDATE, DELETE, DROP TABLE) over pipe-delimited text catalogs; real
  indexed/multi-field DB engines are planned for a later phase. `db_save`/
  `db_open` provide file-backed persistence via `std.io` text snapshots.

## Run

```powershell
karkain run examples/06-database/01_db_crud.kark
```