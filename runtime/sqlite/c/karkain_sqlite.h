#ifndef KARKAIN_SQLITE_H
#define KARKAIN_SQLITE_H

// v1.2.22: minimal C23 binding over the vendored SQLite amalgamation. The
// generated preamble defines Value and the shared Value API. We declare only
// the SQLite C ABI symbols we call; the amalgamation is compiled separately.

Value karkain_sqlite_open(Value path);
Value karkain_sqlite_exec(Value handle, Value sql);
Value karkain_sqlite_query(Value handle, Value sql);
Value karkain_sqlite_close(Value handle);
Value karkain_sqlite_last_error(void);

#endif
