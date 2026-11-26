#include "karkain_sqlite.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

extern int sqlite3_open(const char* filename, void** ppDb);
extern int sqlite3_close(void* db);
extern int sqlite3_exec(void* db, const char* sql,
    int (*callback)(void*, int, char**, char**), void* arg, char** errmsg);
extern int sqlite3_prepare_v2(void* db, const char* sql, int nByte,
    void** ppStmt, const char** pzTail);
extern int sqlite3_step(void* stmt);
extern int sqlite3_finalize(void* stmt);
extern int sqlite3_column_count(void* stmt);
extern int sqlite3_column_type(void* stmt, int col);
extern long long sqlite3_column_int64(void* stmt, int col);
extern double sqlite3_column_double(void* stmt, int col);
extern const unsigned char* sqlite3_column_text(void* stmt, int col);
extern const char* sqlite3_errmsg(void* db);
extern void sqlite3_free(void* p);

#define KARKAIN_SQLITE_MAX_DB 64
#define SQLITE_OK_ 0
#define SQLITE_ROW_ 100
#define SQLITE_DONE_ 101
#define SQLITE_INTEGER_ 1
#define SQLITE_FLOAT_ 2

static char _karkain_sqlite_error[512] = "";
static void* _karkain_sqlite_db[KARKAIN_SQLITE_MAX_DB];
static int _karkain_sqlite_db_used = 0;

static void karkain_sqlite_err(const char* msg) {
    snprintf(_karkain_sqlite_error, sizeof(_karkain_sqlite_error), "%s", msg ? msg : "sqlite failure");
}

static void karkain_sqlite_copy_err(const char* msg) {
    if (!msg) return;
    snprintf(_karkain_sqlite_error, sizeof(_karkain_sqlite_error), "sqlite: %s", msg);
}

static void* karkain_sqlite_db_at(Value handle) {
    if (handle.type != TYPE_INT) return NULL;
    long long id = handle.intVal;
    if (id <= 0 || id > KARKAIN_SQLITE_MAX_DB) return NULL;
    return _karkain_sqlite_db[id - 1];
}

Value karkain_sqlite_last_error(void) {
    return make_string(_karkain_sqlite_error);
}

Value karkain_sqlite_open(Value path) {
    _karkain_sqlite_error[0] = '\0';
    if (path.type != TYPE_STRING) {
        karkain_sqlite_err("sqlite: path must be a string");
        return make_int(0);
    }
    if (_karkain_sqlite_db_used >= KARKAIN_SQLITE_MAX_DB) {
        karkain_sqlite_err("sqlite: handle table full");
        return make_int(0);
    }
    void* db = NULL;
    int rc = sqlite3_open(path.strVal ? path.strVal : "", &db);
    if (rc != SQLITE_OK_ || !db) {
        karkain_sqlite_copy_err(db ? sqlite3_errmsg(db) : "sqlite: open failed");
        if (db) sqlite3_close(db);
        return make_int(0);
    }
    int slot = _karkain_sqlite_db_used++;
    _karkain_sqlite_db[slot] = db;
    return make_int((long long)slot + 1);
}

Value karkain_sqlite_exec(Value handle, Value sql) {
    _karkain_sqlite_error[0] = '\0';
    void* db = karkain_sqlite_db_at(handle);
    if (!db || sql.type != TYPE_STRING) {
        karkain_sqlite_err("sqlite: invalid exec argument");
        return make_int(-1);
    }
    char* errmsg = NULL;
    int rc = sqlite3_exec(db, sql.strVal ? sql.strVal : "", NULL, NULL, &errmsg);
    if (rc != SQLITE_OK_) {
        karkain_sqlite_copy_err(errmsg ? errmsg : sqlite3_errmsg(db));
        if (errmsg) sqlite3_free(errmsg);
        return make_int((long long)rc);
    }
    return make_int(0);
}

Value karkain_sqlite_query(Value handle, Value sql) {
    _karkain_sqlite_error[0] = '\0';
    void* db = karkain_sqlite_db_at(handle);
    if (!db || sql.type != TYPE_STRING) {
        karkain_sqlite_err("sqlite: invalid query argument");
        return make_array();
    }
    void* stmt = NULL;
    int rc = sqlite3_prepare_v2(db, sql.strVal ? sql.strVal : "", -1, &stmt, NULL);
    if (rc != SQLITE_OK_ || !stmt) {
        karkain_sqlite_copy_err(sqlite3_errmsg(db));
        if (stmt) sqlite3_finalize(stmt);
        return make_array();
    }
    Value out = make_array();
    for (;;) {
        rc = sqlite3_step(stmt);
        if (rc == SQLITE_DONE_) break;
        if (rc != SQLITE_ROW_) {
            karkain_sqlite_copy_err(sqlite3_errmsg(db));
            sqlite3_finalize(stmt);
            return make_array();
        }
        Value row = make_array();
        int cols = sqlite3_column_count(stmt);
        int i;
        for (i = 0; i < cols; i++) {
            int ty = sqlite3_column_type(stmt, i);
            Value cell;
            if (ty == SQLITE_INTEGER_) cell = make_int(sqlite3_column_int64(stmt, i));
            else if (ty == SQLITE_FLOAT_) cell = make_float(sqlite3_column_double(stmt, i));
            else {
                const unsigned char* text = sqlite3_column_text(stmt, i);
                cell = make_string(text ? (const char*)text : "");
            }
            array_push(&row, cell);
        }
        array_push(&out, row);
    }
    sqlite3_finalize(stmt);
    return out;
}

Value karkain_sqlite_close(Value handle) {
    _karkain_sqlite_error[0] = '\0';
    void* db = karkain_sqlite_db_at(handle);
    if (!db) {
        karkain_sqlite_err("sqlite: invalid close handle");
        return make_int(-1);
    }
    int rc = sqlite3_close(db);
    if (rc != SQLITE_OK_) {
        karkain_sqlite_copy_err(sqlite3_errmsg(db));
        return make_int((long long)rc);
    }
    long long id = handle.intVal;
    _karkain_sqlite_db[id - 1] = NULL;
    return make_int(0);
}
