# 06 — Database

**Status: Planned**

No database APIs exist in the Karkain language or stdlib.

| Example | Status  | Notes |
|---------|---------|-------|
| (none)  | Planned | See below |

## Intended API surface (roadmap — NOT implemented)

```
std.db:
  db_connect(dsn) -> connection
  db_execute(conn, sql) -> result        # parameterized once implemented
  db_query(conn, sql) -> rows
  db_close(conn)
```

The corpus must never imply unsupported database functionality exists.
Until a database driver runtime ships, this category documents the intended
shape only, and no `.kark` example is provided.

## Parallels today

Data persistence available today is file-based through `std.io`
(see `examples/05_data` and `examples/03_systems/01_file_io.kark`).

## Run

Nothing to run yet. See `docs/source/examples/database.rst`.