/* ============================================================
 * Karkain Runtime - C runtime for the self-hosted compiler
 * Provides Value type system and all runtime functions needed
 * to compile and run the compiler itself.
 * Compiles with: gcc -std=c99 -o output runtime.c -lm
 * ============================================================ */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <math.h>
#include <time.h>
#include <ctype.h>

/* ============================================================
 * Value Type System
 * ============================================================ */

typedef enum {
    TYPE_INT = 0,
    TYPE_FLOAT64 = 1,
    TYPE_STRING = 2,
    TYPE_ARRAY = 3,
    TYPE_MAP = 4,
    TYPE_BOOL = 5
} ValueType;

typedef struct Value {
    ValueType type;
    union {
        long long intVal;
        double floatVal;
        char* strVal;
        struct {
            struct Value** items;
            int length;
        } arrVal;
        struct {
            struct Value** keys;
            struct Value** values;
            int length;
        } mapVal;
    };
} Value;

/* Forward declarations for internal use */
int values_equal(Value* a, Value* b);
int is_truthy(Value* v);
Value* map_get(Value* m, Value* k);

/* ============================================================
 * Constructor Functions
 * ============================================================ */

Value* make_int(long long v) {
    Value* val = (Value*)malloc(sizeof(Value));
    val->type = TYPE_INT;
    val->intVal = v;
    return val;
}

Value* make_float(double v) {
    Value* val = (Value*)malloc(sizeof(Value));
    val->type = TYPE_FLOAT64;
    val->floatVal = v;
    return val;
}

Value* make_string(const char* s) {
    Value* val = (Value*)malloc(sizeof(Value));
    val->type = TYPE_STRING;
    val->strVal = strdup(s ? s : "");
    return val;
}

Value* make_bool(int v) {
    Value* val = (Value*)malloc(sizeof(Value));
    val->type = TYPE_BOOL;
    val->intVal = v ? 1 : 0;
    return val;
}

Value* make_array(void) {
    Value* val = (Value*)malloc(sizeof(Value));
    val->type = TYPE_ARRAY;
    val->arrVal.items = NULL;
    val->arrVal.length = 0;
    return val;
}

Value* make_map(void) {
    Value* val = (Value*)malloc(sizeof(Value));
    val->type = TYPE_MAP;
    val->mapVal.keys = NULL;
    val->mapVal.values = NULL;
    val->mapVal.length = 0;
    return val;
}

/* ============================================================
 * Array Operations
 * ============================================================ */

void array_push(Value* arr, Value* elem) {
    if (!arr || arr->type != TYPE_ARRAY) return;
    arr->arrVal.length++;
    arr->arrVal.items = (Value**)realloc(arr->arrVal.items, sizeof(Value*) * arr->arrVal.length);
    arr->arrVal.items[arr->arrVal.length - 1] = elem;
}

Value* array_get(Value* arr, int idx) {
    if (!arr) return make_int(0);
    if (arr->type == TYPE_MAP) {
        return map_get(arr, make_int(idx));
    }
    if (arr->type != TYPE_ARRAY) return make_int(0);
    if (idx < 0 || idx >= arr->arrVal.length) return make_int(0);
    return arr->arrVal.items[idx];
}

Value* array_length(Value* arr) {
    if (!arr) return make_int(0);
    if (arr->type == TYPE_ARRAY) return make_int(arr->arrVal.length);
    if (arr->type == TYPE_MAP) return make_int(arr->mapVal.length);
    if (arr->type == TYPE_STRING) return make_int((long long)strlen(arr->strVal));
    return make_int(0);
}

/* ============================================================
 * Map Operations
 * ============================================================ */

void map_set(Value* m, Value* k, Value* v) {
    if (!m || m->type != TYPE_MAP) return;
    int i;
    for (i = 0; i < m->mapVal.length; i++) {
        if (values_equal(m->mapVal.keys[i], k)) {
            m->mapVal.values[i] = v;
            return;
        }
    }
    m->mapVal.length++;
    m->mapVal.keys = (Value**)realloc(m->mapVal.keys, sizeof(Value*) * m->mapVal.length);
    m->mapVal.values = (Value**)realloc(m->mapVal.values, sizeof(Value*) * m->mapVal.length);
    m->mapVal.keys[m->mapVal.length - 1] = k;
    m->mapVal.values[m->mapVal.length - 1] = v;
}

Value* map_get(Value* m, Value* k) {
    if (!m || m->type != TYPE_MAP) return make_int(0);
    int i;
    for (i = 0; i < m->mapVal.length; i++) {
        if (values_equal(m->mapVal.keys[i], k)) {
            return m->mapVal.values[i];
        }
    }
    return make_int(0);
}

Value* map_has(Value* m, Value* k) {
    if (!m || m->type != TYPE_MAP) return make_int(0);
    int i;
    for (i = 0; i < m->mapVal.length; i++) {
        if (values_equal(m->mapVal.keys[i], k)) {
            return make_int(1);
        }
    }
    return make_int(0);
}

/* ============================================================
 * Values Equality
 * ============================================================ */

int values_equal(Value* a, Value* b) {
    if (!a || !b) return 0;
    if (a->type != b->type) return 0;
    if (a->type == TYPE_INT) return a->intVal == b->intVal;
    if (a->type == TYPE_BOOL) return a->intVal == b->intVal;
    if (a->type == TYPE_FLOAT64) return a->floatVal == b->floatVal;
    if (a->type == TYPE_STRING) return strcmp(a->strVal, b->strVal) == 0;
    if (a->type == TYPE_ARRAY) {
        if (a->arrVal.length != b->arrVal.length) return 0;
        int i;
        for (i = 0; i < a->arrVal.length; i++) {
            if (!values_equal(a->arrVal.items[i], b->arrVal.items[i])) return 0;
        }
        return 1;
    }
    return 0;
}

/* ============================================================
 * Is Truthy
 * ============================================================ */

int is_truthy(Value* v) {
    if (!v) return 0;
    if (v->type == TYPE_INT) return v->intVal != 0;
    if (v->type == TYPE_BOOL) return v->intVal != 0;
    if (v->type == TYPE_FLOAT64) return v->floatVal != 0.0;
    if (v->type == TYPE_STRING) return strlen(v->strVal) > 0;
    if (v->type == TYPE_ARRAY) return v->arrVal.length > 0;
    if (v->type == TYPE_MAP) return v->mapVal.length > 0;
    return 0;
}

/* ============================================================
 * Polymorphic print_value
 * ============================================================ */

void print_value(Value* v) {
    if (!v) {
        printf("null\n");
        fflush(stdout);
        return;
    }
    if (v->type == TYPE_INT) {
        printf("%lld\n", v->intVal);
    } else if (v->type == TYPE_BOOL) {
        printf("%s\n", v->intVal ? "true" : "false");
    } else if (v->type == TYPE_FLOAT64) {
        printf("%g\n", v->floatVal);
    } else if (v->type == TYPE_STRING) {
        printf("%s\n", v->strVal);
    } else if (v->type == TYPE_ARRAY) {
        printf("[");
        int i;
        for (i = 0; i < v->arrVal.length; i++) {
            if (i > 0) printf(", ");
            Value* item = v->arrVal.items[i];
            if (!item) {
                printf("null");
            } else if (item->type == TYPE_STRING) {
                printf("\"%s\"", item->strVal);
            } else if (item->type == TYPE_INT) {
                printf("%lld", item->intVal);
            } else if (item->type == TYPE_BOOL) {
                printf("%s", item->intVal ? "true" : "false");
            } else if (item->type == TYPE_FLOAT64) {
                printf("%g", item->floatVal);
            } else {
                printf("...");
            }
        }
        printf("]\n");
    } else if (v->type == TYPE_MAP) {
        printf("{");
        int i;
        for (i = 0; i < v->mapVal.length; i++) {
            if (i > 0) printf(", ");
            Value* k = v->mapVal.keys[i];
            Value* item = v->mapVal.values[i];
            if (k->type == TYPE_STRING) printf("\"%s\": ", k->strVal);
            else if (k->type == TYPE_INT) printf("%lld: ", k->intVal);
            else printf("?: ");
            if (!item) {
                printf("null");
            } else if (item->type == TYPE_STRING) {
                printf("\"%s\"", item->strVal);
            } else if (item->type == TYPE_INT) {
                printf("%lld", item->intVal);
            } else if (item->type == TYPE_BOOL) {
                printf("%s", item->intVal ? "true" : "false");
            } else if (item->type == TYPE_FLOAT64) {
                printf("%g", item->floatVal);
            } else {
                printf("...");
            }
        }
        printf("}\n");
    }
    fflush(stdout);
}

/* ============================================================
 * Polymorphic binary_op
 * ============================================================ */

Value* binary_op(Value* left, const char* op, Value* right) {
    if (!left || !right) return make_int(0);

    /* String concatenation */
    if (strcmp(op, "+") == 0 && left->type == TYPE_STRING && right->type == TYPE_STRING) {
        size_t len = strlen(left->strVal) + strlen(right->strVal);
        char* buf = (char*)malloc(len + 1);
        strcpy(buf, left->strVal);
        strcat(buf, right->strVal);
        Value* result = make_string(buf);
        free(buf);
        return result;
    }

    /* Float64 arithmetic (promotes int to float when mixed) */
    if (left->type == TYPE_FLOAT64 || right->type == TYPE_FLOAT64) {
        double l = (left->type == TYPE_FLOAT64) ? left->floatVal : (double)left->intVal;
        double r = (right->type == TYPE_FLOAT64) ? right->floatVal : (double)right->intVal;
        if (strcmp(op, "+") == 0) return make_float(l + r);
        if (strcmp(op, "-") == 0) return make_float(l - r);
        if (strcmp(op, "*") == 0) return make_float(l * r);
        if (strcmp(op, "/") == 0) return make_float(r != 0.0 ? l / r : 0.0);
        if (strcmp(op, "%") == 0) return make_float(r != 0.0 ? fmod(l, r) : 0.0);
        if (strcmp(op, ">") == 0) return make_int(l > r ? 1 : 0);
        if (strcmp(op, "<") == 0) return make_int(l < r ? 1 : 0);
        if (strcmp(op, ">=") == 0) return make_int(l >= r ? 1 : 0);
        if (strcmp(op, "<=") == 0) return make_int(l <= r ? 1 : 0);
        if (strcmp(op, "==") == 0) return make_int(l == r ? 1 : 0);
        if (strcmp(op, "!=") == 0) return make_int(l != r ? 1 : 0);
    }

    /* Int arithmetic */
    if (left->type == TYPE_INT && right->type == TYPE_INT) {
        if (strcmp(op, "+") == 0) return make_int(left->intVal + right->intVal);
        if (strcmp(op, "-") == 0) return make_int(left->intVal - right->intVal);
        if (strcmp(op, "*") == 0) return make_int(left->intVal * right->intVal);
        if (strcmp(op, "/") == 0) return make_int(right->intVal != 0 ? left->intVal / right->intVal : 0);
        if (strcmp(op, "%") == 0) return make_int(right->intVal != 0 ? left->intVal % right->intVal : 0);
        if (strcmp(op, ">") == 0) return make_int(left->intVal > right->intVal ? 1 : 0);
        if (strcmp(op, "<") == 0) return make_int(left->intVal < right->intVal ? 1 : 0);
        if (strcmp(op, ">=") == 0) return make_int(left->intVal >= right->intVal ? 1 : 0);
        if (strcmp(op, "<=") == 0) return make_int(left->intVal <= right->intVal ? 1 : 0);
        if (strcmp(op, "==") == 0) return make_int(left->intVal == right->intVal ? 1 : 0);
        if (strcmp(op, "!=") == 0) return make_int(left->intVal != right->intVal ? 1 : 0);
    }

    /* String comparison */
    if (left->type == TYPE_STRING && right->type == TYPE_STRING) {
        if (strcmp(op, "==") == 0) return make_int(strcmp(left->strVal, right->strVal) == 0 ? 1 : 0);
        if (strcmp(op, "!=") == 0) return make_int(strcmp(left->strVal, right->strVal) != 0 ? 1 : 0);
    }

    /* Logical operators (truthiness-based) */
    if (strcmp(op, "&&") == 0) return make_int(is_truthy(left) && is_truthy(right) ? 1 : 0);
    if (strcmp(op, "||") == 0) return make_int(is_truthy(left) || is_truthy(right) ? 1 : 0);

    return make_int(0);
}

/* ============================================================
 * karkain_len - polymorphic length
 * ============================================================ */

Value* karkain_len(Value* v) {
    return array_length(v);
}

/* ============================================================
 * File I/O
 * ============================================================ */

Value* karkain_readFile(Value* path) {
    if (!path || path->type != TYPE_STRING) return make_string("");
    FILE* f = fopen(path->strVal, "rb");
    if (!f) return make_string("");
    fseek(f, 0, SEEK_END);
    long sz = ftell(f);
    fseek(f, 0, SEEK_SET);
    if (sz <= 0) { fclose(f); return make_string(""); }
    char* buf = (char*)malloc(sz + 1);
    size_t read = fread(buf, 1, sz, f);
    fclose(f);
    buf[read] = '\0';
    Value* res = make_string(buf);
    free(buf);
    return res;
}

Value* karkain_writeFile(Value* path, Value* content) {
    if (!path || path->type != TYPE_STRING) return make_int(0);
    FILE* f = fopen(path->strVal, "wb");
    if (!f) return make_int(0);
    const char* str = (content && content->type == TYPE_STRING) ? content->strVal : "";
    fputs(str, f);
    fclose(f);
    return make_int(1);
}

/* ============================================================
 * String Operations
 * ============================================================ */

Value* karkain_substr(Value* str, Value* start, Value* length) {
    if (!str || str->type != TYPE_STRING) return make_string("");
    if (!start || start->type != TYPE_INT) return make_string("");
    if (!length || length->type != TYPE_INT) return make_string("");
    int s = (int)start->intVal;
    int l = (int)length->intVal;
    int slen = (int)strlen(str->strVal);
    if (s < 0) s = 0;
    if (s >= slen) return make_string("");
    if (s + l > slen) l = slen - s;
    if (l < 0) l = 0;
    char* buf = (char*)malloc(l + 1);
    memcpy(buf, str->strVal + s, l);
    buf[l] = '\0';
    Value* res = make_string(buf);
    free(buf);
    return res;
}

Value* karkain_str(Value* v) {
    if (!v) return make_string("");
    if (v->type == TYPE_STRING) return make_string(v->strVal);
    if (v->type == TYPE_INT) {
        char buf[64];
        snprintf(buf, sizeof(buf), "%lld", v->intVal);
        return make_string(buf);
    }
    if (v->type == TYPE_FLOAT64) {
        char buf[64];
        snprintf(buf, sizeof(buf), "%g", v->floatVal);
        return make_string(buf);
    }
    if (v->type == TYPE_BOOL) return make_string(v->intVal ? "true" : "false");
    if (v->type == TYPE_ARRAY) return make_string("[array]");
    if (v->type == TYPE_MAP) return make_string("[map]");
    return make_string("");
}

Value* karkain_int(Value* v) {
    if (!v) return make_int(0);
    if (v->type == TYPE_INT) return make_int(v->intVal);
    if (v->type == TYPE_FLOAT64) return make_int((long long)v->floatVal);
    if (v->type == TYPE_STRING) return make_int(atoll(v->strVal));
    if (v->type == TYPE_BOOL) return make_int(v->intVal);
    return make_int(0);
}

/* ============================================================
 * System Command Execution
 * ============================================================ */

Value* karkain_system(Value* cmd) {
    if (!cmd || cmd->type != TYPE_STRING) return make_int(-1);
    int result = system(cmd->strVal);
    return make_int(result);
}

/* ============================================================
 * File Operations
 * ============================================================ */

Value* karkain_removeFile(Value* path) {
    if (!path || path->type != TYPE_STRING) return make_int(0);
    int result = remove(path->strVal);
    return make_int(result == 0 ? 1 : 0);
}

Value* karkain_fileExists(Value* path) {
    if (!path || path->type != TYPE_STRING) return make_int(0);
    FILE* f = fopen(path->strVal, "rb");
    if (f) {
        fclose(f);
        return make_int(1);
    }
    return make_int(0);
}

/* ============================================================
 * Built-in functions used by the self-hosted compiler
 * ============================================================ */

/* openFile - opens a file, returns a handle as a string (path) or "" on failure */
Value* openFile(Value* path) {
    if (!path || path->type != TYPE_STRING) return make_string("");
    FILE* f = fopen(path->strVal, "rb");
    if (!f) return make_string("");
    fclose(f);
    /* Return the path as a handle; readLine will re-open it */
    return make_string(path->strVal);
}

/* readLine - reads one line from a file handle */
static FILE* _karkain_files[256];
static int _karkain_file_count = 0;

Value* readLine(Value* handle) {
    if (!handle || handle->type != TYPE_STRING) return make_string("");
    if (strlen(handle->strVal) == 0) return make_string("");

    /* Find existing file handle or open new one */
    int idx = -1;
    int i;
    for (i = 0; i < _karkain_file_count; i++) {
        if (_karkain_files[i] != NULL) {
            idx = i;
            break;
        }
    }
    if (idx == -1) {
        if (_karkain_file_count >= 256) return make_string("");
        idx = _karkain_file_count;
        _karkain_files[idx] = fopen(handle->strVal, "rb");
        _karkain_file_count++;
    } else if (_karkain_files[idx] == NULL) {
        _karkain_files[idx] = fopen(handle->strVal, "rb");
    }

    if (!_karkain_files[idx]) return make_string("");

    char buf[4096];
    if (fgets(buf, sizeof(buf), _karkain_files[idx]) == NULL) {
        return make_string("");
    }
    /* Remove trailing newline */
    int len = (int)strlen(buf);
    while (len > 0 && (buf[len - 1] == '\n' || buf[len - 1] == '\r')) {
        buf[--len] = '\0';
    }
    return make_string(buf);
}

/* closeFile - closes a file handle */
Value* closeFile(Value* handle) {
    if (!handle || handle->type != TYPE_STRING) return make_int(0);
    int i;
    for (i = 0; i < _karkain_file_count; i++) {
        if (_karkain_files[i] != NULL) {
            fclose(_karkain_files[i]);
            _karkain_files[i] = NULL;
            break;
        }
    }
    return make_int(1);
}

/* createFile - creates/truncates a file for writing, returns path */
Value* createFile(Value* path) {
    if (!path || path->type != TYPE_STRING) return make_string("");
    FILE* f = fopen(path->strVal, "wb");
    if (f) fclose(f);
    return make_string(path->strVal);
}

/* writeToFile - writes content to a file handle */
Value* writeToFile(Value* handle, Value* content) {
    if (!handle || handle->type != TYPE_STRING) return make_int(0);
    if (!content || content->type != TYPE_STRING) return make_int(0);
    FILE* f = fopen(handle->strVal, "wb");
    if (!f) return make_int(0);
    fputs(content->strVal, f);
    fclose(f);
    return make_int(1);
}

/* removeFile - removes a file */
Value* removeFile(Value* path) {
    return karkain_removeFile(path);
}

/* getArgs - returns command line arguments as an array */
Value* getArgs(void) {
    /* Placeholder: returns a basic args array.
     * In a real build, this would use argc/argv from main.
     * The self-hosted compiler uses this to get ["karkain", ...] */
    Value* args = make_array();
    array_push(args, make_string("karkain"));
    return args;
}

/* appendArray - appends element to array, returns the array */
Value* appendArray(Value* arr, Value* elem) {
    if (!arr || arr->type != TYPE_ARRAY) return make_array();
    array_push(arr, elem);
    return arr;
}

/* ============================================================
 * Forward Declarations for Compiler Functions
 * These match the signatures generated by the self-hosted compiler.
 * All functions take Value* args and return Value*, except main.
 * ============================================================ */

/* ast.kar */
Value* createLocation(Value* line, Value* col, Value* offset);
Value* locLine(Value* loc);
Value* locCol(Value* loc);
Value* locOffset(Value* loc);
Value* createToken(Value* kind, Value* literal, Value* loc);
Value* tokenKind(Value* tok);
Value* tokenLiteral(Value* tok);
Value* tokenLoc(Value* tok);
Value* createProgramNode(Value* statements);
Value* createFuncDeclNode(Value* name, Value* params, Value* returnType, Value* body, Value* loc);
Value* createKernelDeclNode(Value* name, Value* params, Value* body, Value* workgroup, Value* loc);
Value* createVarDeclNode(Value* varType, Value* name, Value* value, Value* loc);
Value* createLetDeclNode(Value* name, Value* value, Value* loc);
Value* createStructDeclNode(Value* name, Value* fields, Value* loc);
Value* createIdentNode(Value* name, Value* loc);
Value* createIntLiteralNode(Value* value, Value* loc);
Value* createFloatLiteralNode(Value* value, Value* loc);
Value* createStringLiteralNode(Value* value, Value* loc);
Value* createBoolLiteralNode(Value* value, Value* loc);
Value* createBinaryExprNode(Value* op, Value* left, Value* right, Value* loc);
Value* createUnaryExprNode(Value* op, Value* operand, Value* loc);
Value* createCallNode(Value* funcName, Value* args, Value* loc);
Value* createIndexNode(Value* left, Value* index, Value* loc);
Value* createMemberNode(Value* object, Value* member, Value* loc);
Value* createPrintNode(Value* expr, Value* loc);
Value* createIfNode(Value* condition, Value* thenBranch, Value* elseBranch, Value* loc);
Value* createWhileNode(Value* condition, Value* body, Value* loc);
Value* createForNode(Value* init, Value* condition, Value* post, Value* body, Value* loc);
Value* createReturnNode(Value* expr, Value* loc);
Value* createExprStmtNode(Value* expr, Value* loc);
Value* createArrayLiteralNode(Value* elements, Value* loc);
Value* createMapLiteralNode(Value* keys, Value* values, Value* loc);
Value* getNodeType(Value* node);
Value* funcName(Value* node);
Value* funcParams(Value* node);
Value* funcReturnType(Value* node);
Value* funcBody(Value* node);
Value* kernelName(Value* node);
Value* kernelParams(Value* node);
Value* kernelBody(Value* node);
Value* kernelWorkgroup(Value* node);
Value* varDeclType(Value* node);
Value* varDeclName(Value* node);
Value* varDeclValue(Value* node);
Value* letDeclName(Value* node);
Value* letDeclValue(Value* node);
Value* identName(Value* node);
Value* literalValue(Value* node);
Value* binaryOp(Value* node);
Value* binaryLeft(Value* node);
Value* binaryRight(Value* node);
Value* unaryOp(Value* node);
Value* unaryOperand(Value* node);
Value* callFunc(Value* node);
Value* callArgs(Value* node);
Value* printExpr(Value* node);
Value* ifCondition(Value* node);
Value* ifThen(Value* node);
Value* ifElse(Value* node);
Value* whileCondition(Value* node);
Value* whileBody(Value* node);
Value* forInit(Value* node);
Value* forCondition(Value* node);
Value* forPost(Value* node);
Value* forBody(Value* node);
Value* returnExpr(Value* node);
Value* exprStmtExpr(Value* node);
Value* programStatements(Value* node);
Value* structName(Value* node);
Value* structFields(Value* node);
Value* indexLeft(Value* node);
Value* indexExpr(Value* node);
Value* memberObject(Value* node);
Value* memberField(Value* node);
Value* arrayElements(Value* node);
Value* mapKeys(Value* node);
Value* mapValues(Value* node);

/* lexer.kar */
Value* tokenize(Value* source);

/* parser.kar */
Value* createParserState(Value* tokens);
Value* hasParserErrors(Value* state);
Value* printParserErrors(Value* state);
Value* parse(Value* tokens);

/* sema.kar */
Value* analyze(Value* ast);

/* codegen.kar */
Value* generate(Value* ast, Value* target);
Value* generateForwardDecls(Value* ast);

/* main.kar */
Value* readFile(Value* path);
Value* checkFile(Value* path);
Value* buildFile(Value* path, Value* target);
Value* runFile(Value* path);
Value* runTests(Value* path);
Value* startLSP(void);
void printUsage(void);
Value* replaceExtension(Value* path, Value* newExt);
Value* printAST(Value* node, Value* indent);
int kar_main(void);

/* ============================================================
 * Entry Point
 * ============================================================ */

int main(int argc, char* argv[]) {
    return kar_main();
}
