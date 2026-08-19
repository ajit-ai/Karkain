package codegen

import (
	"context"
	"fmt"
	"karkain/pkg/parser"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type Config struct {
	OutputPath  string
	CompileOnly bool
	Verbose     bool
	RunAfter    bool
	Debug       bool   // Add debug flag for DWARF symbols
	Target      string // Target architecture (native, wasm32-wasi)
}

func NewConfig() Config {
	return Config{
		Target: "native", // Default to native target
	}
}

type Generator struct {
	cfg Config
}

func New(cfg Config) *Generator {
	return &Generator{cfg: cfg}
}

func (g *Generator) GenerateAndCompile(prog *parser.Program, sourceFile string) error {
	cCode := g.generateCHeader()

	// Add AVX2 and scalar matrix multiplication kernels
	cCode += g.genAVX2MatrixMul("rowsA", "colsA", "colsB")
	cCode += g.genScalarMatrixMul()

	// Emit #line directive for source mapping if debug mode is enabled
	if g.cfg.Debug {
		// Convert backslashes to forward slashes for cross-platform compatibility
		cleanSourceFile := strings.Replace(sourceFile, "\\", "/", -1)
		cCode += fmt.Sprintf("#line 1 \"%s\"\n", cleanSourceFile)
	}

	// Inject C import blocks - check if they exist first
	if len(prog.CImports) > 0 {
		for _, cImport := range prog.CImports {
			if cImport.Content != "" && !strings.Contains(cImport.Content, "Phase 11") {
				cCode += cImport.Content + "\n"
			}
		}
	}

	// Phase 40: Generate forward declarations for all functions
	// This enables cross-file references when multiple .kar files are concatenated
	for _, stmt := range prog.Statements {
		if fn, ok := stmt.(*parser.FuncDecl); ok {
			params := []string{}
			for _, p := range fn.Params {
				params = append(params, "Value* "+p)
			}
			retType := "Value*"
			if fn.Name == "main" {
				retType = "int"
			}
			cCode += fmt.Sprintf("%s %s(%s);\n", retType, fn.Name, strings.Join(params, ", "))
		}
	}
	cCode += "\n"

	// Generate all function declarations
	for _, stmt := range prog.Statements {
		if fn, ok := stmt.(*parser.FuncDecl); ok {
			cCode += g.genFuncDecl(fn)
		}
	}

	// Phase 18: Generate GPU kernel declarations and host launchers
	for _, stmt := range prog.Statements {
		if kernel, ok := stmt.(*parser.KernelDeclStmt); ok {
			cCode += g.genKernelDecl(kernel)
		}
	}

	if g.cfg.Verbose {
		fmt.Println("=== [3] GENERATED C99 SOURCE CODE ===")
		fmt.Println(cCode)
		fmt.Println("=====================================")
	}

	tmpCFile := strings.TrimSuffix(sourceFile, filepath.Ext(sourceFile)) + ".c"
	exeFile := g.cfg.OutputPath

	if exeFile == "" {
		baseName := strings.TrimSuffix(sourceFile, filepath.Ext(sourceFile))
		if runtime.GOOS == "windows" {
			exeFile = baseName + ".exe"
		} else {
			exeFile = baseName
		}
	}

	err := os.WriteFile(tmpCFile, []byte(cCode), 0644)
	if err != nil {
		return fmt.Errorf("failed to write C source file: %w", err)
	}

	// Don't remove the C file for debugging
	// if !g.cfg.CompileOnly {
	// 	defer os.Remove(tmpCFile)
	// }

	compiler, flags := g.detectCompiler(tmpCFile, exeFile)
	if compiler == "" {
		return fmt.Errorf("no supported C compiler found (GCC, Clang, or MSVC cl.exe required)")
	}

	if g.cfg.Verbose {
		fmt.Printf("=== [4] C COMPILER INVOCATION ===\n%s %s\n", compiler, strings.Join(flags, " "))
	}

	// Set a 10-second timeout for the compilation process
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	compileCmd := exec.CommandContext(ctx, compiler, flags...)
	compileCmd.Stdout = os.Stdout
	compileCmd.Stderr = os.Stderr
	if runtime.GOOS == "windows" {
		// On Windows, use cmd.exe to handle shell escaping
		compileCmd = exec.CommandContext(ctx, "cmd", append([]string{"/C", compiler}, flags...)...)
	}
	if err := compileCmd.Run(); err != nil {
		return fmt.Errorf("C compilation failed: %w", err)
	}

	if g.cfg.RunAfter {
		if g.cfg.Verbose {
			fmt.Println("=== [5] EXECUTING PROGRAM ===")
		}

		runPath := exeFile
		if !strings.Contains(runPath, `\`) && !strings.Contains(runPath, "/") {
			runPath = "." + string(os.PathSeparator) + runPath
		}

		runCmd := exec.Command(runPath)
		runCmd.Stdout = os.Stdout
		runCmd.Stderr = os.Stderr
		runErr := runCmd.Run()

		if g.cfg.OutputPath == "" {
			os.Remove(exeFile)
		}

		return runErr
	}

	return nil
}

func (g *Generator) generateCHeader() string {
	return `#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <stdalign.h>
#include <math.h>
#include <time.h>

// Phase 15: WASI compatibility and HTTP Runtime (cross-platform socket abstraction)
#ifdef __wasi__
#include <unistd.h>
#else
#ifdef _WIN32
#include <winsock2.h>
#include <windows.h>
#pragma comment(lib, "ws2_32.lib")
#else
#include <sys/socket.h>
#include <netinet/in.h>
#include <arpa/inet.h>
#include <unistd.h>
#define closesocket close
#endif
#endif

// Phase 14: Quantum Runtime (inline for single-file compilation)
typedef struct {
    double real;
    double imag;
} Complex;

typedef struct {
    int num_qubits;
    Complex state[1024];
    int size;
} QuantumRegister;

Complex complex_add(Complex a, Complex b) {
    return (Complex){a.real + b.real, a.imag + b.imag};
}

Complex complex_sub(Complex a, Complex b) {
    return (Complex){a.real - b.real, a.imag - b.imag};
}

Complex complex_mul(Complex a, Complex b) {
    return (Complex){a.real * b.real - a.imag * b.imag, a.real * b.imag + a.imag * b.real};
}

Complex complex_scale(Complex a, double s) {
    return (Complex){a.real * s, a.imag * s};
}

double complex_abs_sq(Complex a) {
    return a.real * a.real + a.imag * a.imag;
}

void qreg_init(QuantumRegister* qr, int num_qubits) {
    qr->num_qubits = num_qubits;
    qr->size = 1 << num_qubits;
    for (int i = 0; i < qr->size; i++) {
        qr->state[i] = (Complex){0.0, 0.0};
    }
    qr->state[0] = (Complex){1.0, 0.0};
}

void gate_h(QuantumRegister* qr, int target) {
    int mask = 1 << target;
    int half_size = qr->size / 2;
    for (int i = 0; i < half_size; i++) {
        int i0 = i;
        int i1 = i ^ mask;
        Complex a = qr->state[i0];
        Complex b = qr->state[i1];
        double inv_sqrt2 = 1.0 / sqrt(2.0);
        qr->state[i0] = complex_scale(complex_add(a, b), inv_sqrt2);
        qr->state[i1] = complex_scale(complex_sub(a, b), inv_sqrt2);
    }
}

void gate_x(QuantumRegister* qr, int target) {
    int mask = 1 << target;
    for (int i = 0; i < qr->size; i++) {
        int j = i ^ mask;
        if (i < j) {
            Complex temp = qr->state[i];
            qr->state[i] = qr->state[j];
            qr->state[j] = temp;
        }
    }
}

void gate_cnot(QuantumRegister* qr, int control, int target) {
    int control_mask = 1 << control;
    int target_mask = 1 << target;
    for (int i = 0; i < qr->size; i++) {
        if (i & control_mask) {
            int j = i ^ target_mask;
            Complex temp = qr->state[i];
            qr->state[i] = qr->state[j];
            qr->state[j] = temp;
        }
    }
}

int measure(QuantumRegister* qr, int target) {
    int mask = 1 << target;
    double prob0 = 0.0;
    for (int i = 0; i < qr->size; i++) {
        if ((i & mask) == 0) {
            prob0 += complex_abs_sq(qr->state[i]);
        }
    }
    int result;
    double r = (double)rand() / RAND_MAX;
    if (r < prob0) {
        result = 0;
        double norm = sqrt(prob0);
        for (int i = 0; i < qr->size; i++) {
            if ((i & mask) == 0) {
                qr->state[i] = complex_scale(qr->state[i], 1.0 / norm);
            } else {
                qr->state[i] = (Complex){0.0, 0.0};
            }
        }
    } else {
        result = 1;
        double norm = sqrt(1.0 - prob0);
        for (int i = 0; i < qr->size; i++) {
            if ((i & mask) != 0) {
                qr->state[i] = complex_scale(qr->state[i], 1.0 / norm);
            } else {
                qr->state[i] = (Complex){0.0, 0.0};
            }
        }
    }
    return result;
}

void quantum_init() {
    srand(time(NULL));
}

// Phase 15: HTTP Runtime (cross-platform socket abstraction)
// HTTP response structure
typedef struct {
    int status_code;
    char* body;
    int body_len;
} HttpResponse;

// HTTP client functions
HttpResponse* http_get(const char* url) {
    HttpResponse* response = (HttpResponse*)malloc(sizeof(HttpResponse));
    response->status_code = 200;
    response->body = strdup("HTTP GET response (placeholder)");
    response->body_len = strlen(response->body);
    return response;
}

void http_response_free(HttpResponse* response) {
    if (response) {
        if (response->body) free(response->body);
        free(response);
    }
}

// Phase 16: Actor Runtime (atomic MPMC mailboxes)
typedef struct {
    void* data;
    size_t size;
    int sender_id;
} ActorMessage;

typedef struct {
    ActorMessage* buffer;
    size_t capacity;
    volatile size_t head;
    volatile size_t tail;
    volatile size_t count;
} MPMCQueue;

typedef struct {
    int id;
    MPMCQueue* mailbox;
    int is_running;
} Actor;

// Phase 16: RPC Runtime (TCP wire protocol)
typedef enum {
    RPC_SPAWN,
    RPC_SEND,
    RPC_STOP,
    RPC_PING
} RPCMessageType;

typedef struct {
    RPCMessageType type;
    int actor_id;
    int sender_id;
    size_t data_size;
    char data[1024];
} RPCMessage;

typedef struct {
    int node_id;
    int port;
    int socket_fd;
    int is_running;
} RPCNode;

// Phase 14: SIMD support detection
#ifdef __AVX2__
#include <immintrin.h>
#define HAS_AVX2 1
#else
#define HAS_AVX2 0
#endif

typedef enum { TYPE_INT, TYPE_FLOAT64, TYPE_STRING, TYPE_ARRAY, TYPE_MAP, TYPE_BOOL } ValueType;

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
    val->strVal = strdup(s);
    return val;
}

Value* make_array() {
    Value* val = (Value*)malloc(sizeof(Value));
    val->type = TYPE_ARRAY;
    val->arrVal.items = NULL;
    val->arrVal.length = 0;
    return val;
}

Value* make_map() {
    Value* val = (Value*)malloc(sizeof(Value));
    val->type = TYPE_MAP;
    val->mapVal.keys = NULL;
    val->mapVal.values = NULL;
    val->mapVal.length = 0;
    return val;
}

void array_push(Value* arr, Value* elem) {
    if (!arr || arr->type != TYPE_ARRAY) return;
    arr->arrVal.length++;
    arr->arrVal.items = (Value**)realloc(arr->arrVal.items, sizeof(Value*) * arr->arrVal.length);
    arr->arrVal.items[arr->arrVal.length - 1] = elem;
}

int values_equal(Value* a, Value* b) {
    if (!a || !b || a->type != b->type) return 0;
    if (a->type == TYPE_INT) return a->intVal == b->intVal;
    if (a->type == TYPE_STRING) return strcmp(a->strVal, b->strVal) == 0;
    return 0;
}

Value* map_set(Value* m, Value* k, Value* v) {
    if (!m || m->type != TYPE_MAP) return m;
    for (int i = 0; i < m->mapVal.length; i++) {
        if (values_equal(m->mapVal.keys[i], k)) {
            m->mapVal.values[i] = v;
            return m;
        }
    }
    m->mapVal.length++;
    m->mapVal.keys = (Value**)realloc(m->mapVal.keys, sizeof(Value*) * m->mapVal.length);
    m->mapVal.values = (Value**)realloc(m->mapVal.values, sizeof(Value*) * m->mapVal.length);
    m->mapVal.keys[m->mapVal.length - 1] = k;
    m->mapVal.values[m->mapVal.length - 1] = v;
    return m;
}

Value* map_get(Value* m, Value* k) {
    if (!m || m->type != TYPE_MAP) return make_int(0);
    for (int i = 0; i < m->mapVal.length; i++) {
        if (values_equal(m->mapVal.keys[i], k)) {
            return m->mapVal.values[i];
        }
    }
    return make_int(0);
}

Value* array_get(Value* arr, Value* idx) {
    if (!arr) return make_int(0);
    if (arr->type == TYPE_MAP) return map_get(arr, idx);
    if (arr->type != TYPE_ARRAY || idx->type != TYPE_INT) return make_int(0);
    int i = (int)idx->intVal;
    if (i < 0 || i >= arr->arrVal.length) return make_int(0);
    return arr->arrVal.items[i];
}

Value* karkain_len(Value* v) {
    if (!v) return make_int(0);
    if (v->type == TYPE_ARRAY) return make_int(v->arrVal.length);
    if (v->type == TYPE_MAP) return make_int(v->mapVal.length);
    if (v->type == TYPE_STRING) return make_int(strlen(v->strVal));
    return make_int(0);
}

Value* karkain_readFile(Value* path) {
    if (!path || path->type != TYPE_STRING) return make_string("");
    FILE* f = fopen(path->strVal, "rb");
    if (!f) return make_string("");
    fseek(f, 0, SEEK_END);
    long sz = ftell(f);
    fseek(f, 0, SEEK_SET);
    char* buf = (char*)malloc(sz + 1);
    fread(buf, 1, sz, f);
    fclose(f);
    buf[sz] = '\0';
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

void print_value(Value* v) {
    if (!v) return;
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
        for (int i = 0; i < v->arrVal.length; i++) {
            if (i > 0) printf(", ");
            Value* item = v->arrVal.items[i];
            if (item->type == TYPE_STRING) printf("\"%s\"", item->strVal);
            else if (item->type == TYPE_INT) printf("%lld", item->intVal);
        }
        printf("]\n");
    } else if (v->type == TYPE_MAP) {
        printf("{");
        for (int i = 0; i < v->mapVal.length; i++) {
            if (i > 0) printf(", ");
            Value* k = v->mapVal.keys[i];
            Value* item = v->mapVal.values[i];
            if (k->type == TYPE_STRING) printf("\"%s\": ", k->strVal);
            else printf("%lld: ", k->intVal);
            if (item->type == TYPE_STRING) printf("\"%s\"", item->strVal);
            else printf("%lld", item->intVal);
        }
        printf("}\n");
    }
    fflush(stdout);
}

Value* binary_op(Value* left, const char* op, Value* right) {
    if (!left || !right) return make_int(0);
    // String concatenation
    if (strcmp(op, "+") == 0 && left->type == TYPE_STRING && right->type == TYPE_STRING) {
        size_t len = strlen(left->strVal) + strlen(right->strVal);
        char* buf = (char*)malloc(len + 1);
        strcpy(buf, left->strVal);
        strcat(buf, right->strVal);
        Value* result = make_string(buf);
        free(buf);
        return result;
    }
    // Float64 arithmetic
    if (left->type == TYPE_FLOAT64 || right->type == TYPE_FLOAT64) {
        double l = (left->type == TYPE_FLOAT64) ? left->floatVal : (double)left->intVal;
        double r = (right->type == TYPE_FLOAT64) ? right->floatVal : (double)right->intVal;
        if (strcmp(op, "+") == 0) return make_float(l + r);
        if (strcmp(op, "-") == 0) return make_float(l - r);
        if (strcmp(op, "*") == 0) return make_float(l * r);
        if (strcmp(op, "/") == 0) return make_float(r != 0.0 ? l / r : 0.0);
        if (strcmp(op, ">") == 0) return make_int(l > r);
        if (strcmp(op, "<") == 0) return make_int(l < r);
        if (strcmp(op, ">=") == 0) return make_int(l >= r);
        if (strcmp(op, "<=") == 0) return make_int(l <= r);
        if (strcmp(op, "==") == 0) return make_int(l == r);
        if (strcmp(op, "!=") == 0) return make_int(l != r);
    }
    // Int arithmetic
    if (strcmp(op, "+") == 0 && left->type == TYPE_INT && right->type == TYPE_INT) {
        return make_int(left->intVal + right->intVal);
    }
    if (strcmp(op, "-") == 0 && left->type == TYPE_INT && right->type == TYPE_INT) {
        return make_int(left->intVal - right->intVal);
    }
    if (strcmp(op, "*") == 0 && left->type == TYPE_INT && right->type == TYPE_INT) {
        return make_int(left->intVal * right->intVal);
    }
    if (strcmp(op, "/") == 0 && left->type == TYPE_INT && right->type == TYPE_INT) {
        return make_int(right->intVal != 0 ? left->intVal / right->intVal : 0);
    }
    if (strcmp(op, ">") == 0 && left->type == TYPE_INT && right->type == TYPE_INT) {
        return make_int(left->intVal > right->intVal);
    }
    if (strcmp(op, "<") == 0 && left->type == TYPE_INT && right->type == TYPE_INT) {
        return make_int(left->intVal < right->intVal);
    }
    if (strcmp(op, ">=") == 0 && left->type == TYPE_INT && right->type == TYPE_INT) {
        return make_int(left->intVal >= right->intVal);
    }
    if (strcmp(op, "<=") == 0 && left->type == TYPE_INT && right->type == TYPE_INT) {
        return make_int(left->intVal <= right->intVal);
    }
    if (strcmp(op, "!=") == 0) {
        if (left->type == TYPE_INT && right->type == TYPE_INT) return make_int(left->intVal != right->intVal);
        if (left->type == TYPE_STRING && right->type == TYPE_STRING) return make_int(strcmp(left->strVal, right->strVal) != 0);
    }
    if (strcmp(op, "==") == 0) {
        if (left->type == TYPE_INT && right->type == TYPE_INT) return make_int(left->intVal == right->intVal);
        if (left->type == TYPE_STRING && right->type == TYPE_STRING) return make_int(strcmp(left->strVal, right->strVal) == 0);
    }
    return make_int(0);
}

int is_truthy(Value* v) {
    if (!v) return 0;
    if (v->type == TYPE_INT) return v->intVal != 0;
    if (v->type == TYPE_FLOAT64) return v->floatVal != 0.0;
    if (v->type == TYPE_STRING) return strlen(v->strVal) > 0;
    if (v->type == TYPE_ARRAY) return v->arrVal.length > 0;
    if (v->type == TYPE_MAP) return v->mapVal.length > 0;
    return 0;
}

// Phase 11: Raw pointer operations
void* karkain_alloc(size_t count, size_t size) {
    return malloc(count * size);
}

void karkain_free(void* ptr) {
    free(ptr);
}

// Phase 10: String standard library functions
Value* karkain_trim(Value* str) {
    if (!str || str->type != TYPE_STRING) return make_string("");
    char* s = str->strVal;
    char* start = s;
    char* end = s + strlen(s) - 1;
    while (start <= end && (*start == ' ' || *start == '\t' || *start == '\n' || *start == '\r')) start++;
    while (end >= start && (*end == ' ' || *end == '\t' || *end == '\n' || *end == '\r')) end--;
    size_t len = (end >= start) ? (end - start + 1) : 0;
    char* buf = (char*)malloc(len + 1);
    if (len > 0) memcpy(buf, start, len);
    buf[len] = '\0';
    Value* res = make_string(buf);
    free(buf);
    return res;
}

Value* karkain_contains(Value* haystack, Value* needle) {
    if (!haystack || !needle || haystack->type != TYPE_STRING || needle->type != TYPE_STRING) return make_int(0);
    return make_int(strstr(haystack->strVal, needle->strVal) != NULL);
}

Value* karkain_split(Value* str, Value* delim) {
    if (!str || !delim || str->type != TYPE_STRING || delim->type != TYPE_STRING) return make_array();
    Value* arr = make_array();
    char* s = strdup(str->strVal);
    char* d = delim->strVal;
    char* token = strtok(s, d);
    while (token != NULL) {
        array_push(arr, make_string(token));
        token = strtok(NULL, d);
    }
    free(s);
    return arr;
}

// Phase 10: Math standard library functions
Value* karkain_sqrt(Value* v) {
    if (!v || v->type != TYPE_INT) return make_int(0);
    return make_int((long long)sqrt((double)v->intVal));
}

Value* karkain_abs(Value* v) {
    if (!v || v->type != TYPE_INT) return make_int(0);
    return make_int(v->intVal >= 0 ? v->intVal : -v->intVal);
}

Value* karkain_pow(Value* base, Value* exp) {
    if (!base || !exp || base->type != TYPE_INT || exp->type != TYPE_INT) return make_int(0);
    long long result = 1;
    long long b = base->intVal;
    long long e = exp->intVal;
    while (e > 0) {
        if (e & 1) result *= b;
        b *= b;
        e >>= 1;
    }
    return make_int(result);
}

// Phase 10: Array append function
Value* karkain_appendArray(Value* arr, Value* elem) {
    if (!arr || arr->type != TYPE_ARRAY) return make_array();
    array_push(arr, elem);
    return arr;
}

// Phase 19: Boolean support
Value* make_bool(int v) {
    Value* val = (Value*)malloc(sizeof(Value));
    val->type = TYPE_BOOL;
    val->intVal = v ? 1 : 0;
    return val;
}

// Phase 19: Map hasKey and delete
Value* karkain_hasKey(Value* m, Value* k) {
    if (!m || m->type != TYPE_MAP) return make_int(0);
    for (int i = 0; i < m->mapVal.length; i++) {
        if (values_equal(m->mapVal.keys[i], k)) {
            return make_int(1);
        }
    }
    return make_int(0);
}

Value* karkain_delete(Value* m, Value* k) {
    if (!m || m->type != TYPE_MAP) return m;
    for (int i = 0; i < m->mapVal.length; i++) {
        if (values_equal(m->mapVal.keys[i], k)) {
            for (int j = i; j < m->mapVal.length - 1; j++) {
                m->mapVal.keys[j] = m->mapVal.keys[j + 1];
                m->mapVal.values[j] = m->mapVal.values[j + 1];
            }
            m->mapVal.length--;
            return m;
        }
    }
    return m;
}

// Phase 19: Modulo operator
Value* karkain_mod(Value* a, Value* b) {
    if (!a || !b) return make_int(0);
    if (a->type == TYPE_INT && b->type == TYPE_INT) {
        if (b->intVal == 0) return make_int(0);
        return make_int(a->intVal % b->intVal);
    }
    if (a->type == TYPE_FLOAT64 || b->type == TYPE_FLOAT64) {
        double l = (a->type == TYPE_FLOAT64) ? a->floatVal : (double)a->intVal;
        double r = (b->type == TYPE_FLOAT64) ? b->floatVal : (double)b->intVal;
        if (r == 0.0) return make_float(0.0);
        return make_float(fmod(l, r));
    }
    return make_int(0);
}

// Phase 19: Negate unary operator
Value* karkain_negate(Value* v) {
    if (!v) return make_int(0);
    if (v->type == TYPE_INT) return make_int(-v->intVal);
    if (v->type == TYPE_FLOAT64) return make_float(-v->floatVal);
    return make_int(0);
}

// Phase 19: Logical NOT operator
Value* karkain_not(Value* v) {
    return make_int(is_truthy(v) ? 0 : 1);
}
`
}

func (g *Generator) genFuncDecl(fn *parser.FuncDecl) string {
	params := []string{}
	for _, p := range fn.Params {
		params = append(params, "Value* "+p)
	}

	retType := "Value*"
	fnName := fn.Name
	if fnName == "main" {
		retType = "int"
	}

	out := fmt.Sprintf("%s %s(%s) {\n", retType, fnName, strings.Join(params, ", "))

	// Phase 14: Initialize quantum runtime in main
	if fnName == "main" {
		out += "\tquantum_init();\n"
	}

	for _, stmt := range fn.Body {
		out += g.genStatement(stmt)
	}

	if fnName == "main" {
		out += "\treturn 0;\n"
	} else {
		out += "\treturn make_int(0);\n"
	}
	out += "}\n\n"
	return out
}

func (g *Generator) genStatement(stmt parser.Node) string {
	switch node := stmt.(type) {
	case *parser.VarDeclStmt:
		if node.IsMatrix {
			return g.genMatrixDecl(node)
		}
		// Check if this is an unboxed type declaration
		if node.Type != "" {
			cType := g.mapKarkainTypeToC(node.Type)
			// For unboxed types, convert the value to raw C literal
			valueExpr := g.mapLiteralToC(node.Value)
			return fmt.Sprintf("\t%s %s = %s;\n", cType, node.Name, valueExpr)
		}
		// For dynamic types, use Value* wrapper
		return fmt.Sprintf("\tValue* %s = %s;\n", node.Name, g.genExpr(node.Value))
	case *parser.ReturnStmt:
		return fmt.Sprintf("\treturn %s;\n", g.genExpr(node.Value))
	case *parser.PrintStmt:
		// Check if this is a matrix index expression
		if matrixIdx, ok := node.Value.(*parser.MatrixIndexExpr); ok {
			matrixExpr := g.genExpr(matrixIdx.Matrix)
			rowC := "0"
			colC := "0"
			if rowLit, ok := matrixIdx.Row.(*parser.IntLiteral); ok {
				rowC = rowLit.Value
			}
			if colLit, ok := matrixIdx.Col.(*parser.IntLiteral); ok {
				colC = colLit.Value
			}
			if matrixIdent, ok := matrixIdx.Matrix.(*parser.Identifier); ok {
				colsVar := matrixIdent.Name + "_cols"
				return fmt.Sprintf("\tprintf(\"%%f\\n\", %s[(%s * %s + %s)]);\n", matrixExpr, rowC, colsVar, colC)
			}
		}
		// Check if the value is a CallExpr with C function result
		if callExpr, ok := node.Value.(*parser.CallExpr); ok {
			if callExpr.IsCFunc {
				// C function results should be printed as raw values
				return fmt.Sprintf("\tprintf(\"%%f\\n\", %s);\n", g.genExpr(node.Value))
			}
		}
		return fmt.Sprintf("\tprint_value(%s);\n", g.genExpr(node.Value))
	case *parser.ExprStmt:
		return fmt.Sprintf("\t%s;\n", g.genExpr(node.Expression))
	case *parser.IfStmt:
		res := fmt.Sprintf("\tif (is_truthy(%s)) {\n", g.genExpr(node.Condition))
		for _, cStmt := range node.Consequence {
			res += "\t" + g.genStatement(cStmt)
		}
		res += "\t}"
		if len(node.Alternative) > 0 {
			if len(node.Alternative) == 1 {
				if elseIf, ok := node.Alternative[0].(*parser.IfStmt); ok {
					// else if -> else if(...)
					elseIfStr := g.genStatement(elseIf)
					res += " else " + strings.TrimPrefix(elseIfStr, "\t")
				} else {
					res += " else {\n"
					for _, aStmt := range node.Alternative {
						res += "\t" + g.genStatement(aStmt)
					}
					res += "\t}"
				}
			} else {
				res += " else {\n"
				for _, aStmt := range node.Alternative {
					res += "\t" + g.genStatement(aStmt)
				}
				res += "\t}"
			}
		}
		res += "\n"
		return res
	case *parser.WhileStmt:
		res := fmt.Sprintf("\twhile (is_truthy(%s)) {\n", g.genExpr(node.Condition))
		for _, bodyStmt := range node.Body {
			res += "\t" + g.genStatement(bodyStmt)
		}
		res += "\t}\n"
		return res
	case *parser.AllocExpr:
		countExpr := g.mapLiteralToC(node.Count)
		cType := g.mapKarkainTypeToC(node.Type)
		return fmt.Sprintf("(%s*)malloc(%s * sizeof(%s))", cType, countExpr, cType)
	case *parser.FreeExpr:
		return fmt.Sprintf("free(%s)", g.genExpr(node.Ptr))
	case *parser.AddressOf:
		return fmt.Sprintf("&(%s)", g.genExpr(node.Operand))
	case *parser.QRegDeclStmt:
		// Phase 14: Generate quantum register declaration
		qubitsExpr := g.mapLiteralToC(node.Qubits)
		return fmt.Sprintf("\tQuantumRegister %s;\n\tqreg_init(&%s, %s);\n", node.Name, node.Name, qubitsExpr)
	case *parser.GateApplyStmt:
		// Phase 14: Generate gate application calls
		qrName := g.getQuantumRegisterName()

		switch node.Gate {
		case "H":
			targetExpr := g.mapLiteralToC(node.Target)
			return fmt.Sprintf("\tgate_h(&%s, %s);\n", qrName, targetExpr)
		case "X":
			targetExpr := g.mapLiteralToC(node.Target)
			return fmt.Sprintf("\tgate_x(&%s, %s);\n", qrName, targetExpr)
		case "CNOT":
			controlExpr := ""
			if controlLit, ok := node.Control.(*parser.IntLiteral); ok {
				controlExpr = controlLit.Value
			} else {
				controlExpr = g.mapLiteralToC(node.Control)
			}
			targetExpr := ""
			if targetLit, ok := node.Target.(*parser.IntLiteral); ok {
				targetExpr = targetLit.Value
			} else {
				targetExpr = g.mapLiteralToC(node.Target)
			}
			return fmt.Sprintf("\tgate_cnot(&%s, %s, %s);\n", qrName, controlExpr, targetExpr)
		default:
			return fmt.Sprintf("\t// Unknown gate: %s\n", node.Gate)
		}
	case *parser.MeasureExpr:
		// Phase 14: Generate measure call in statement context
		// For statement context, we generate the call without storing
		qrName := g.getQuantumRegisterName()
		targetExpr := ""
		if intLit, ok := node.Qubit.(*parser.IntLiteral); ok {
			targetExpr = intLit.Value
		} else {
			targetExpr = g.mapLiteralToC(node.Qubit)
		}
		return fmt.Sprintf("\tmeasure(&%s, %s);\n", qrName, targetExpr)
	case *parser.ActorDeclStmt:
		// Phase 16: Generate actor declaration
		return fmt.Sprintf("\t// Actor %s (declaration placeholder)\n", node.Name)
	case *parser.ReceiveStmt:
		// Phase 16: Generate receive statement
		channelExpr := g.genExpr(node.Channel)
		if node.VarName != "" {
			return fmt.Sprintf("\t// receive(%s) -> %s (placeholder)\n", channelExpr, node.VarName)
		}
		return fmt.Sprintf("\t// receive(%s) (placeholder)\n", channelExpr)
	case *parser.KernelDeclStmt:
		return g.genKernelDecl(node)
	case *parser.BarrierStmt:
		return "\tbarrier(CLK_LOCAL_MEM_FENCE);\n"
	case *parser.StructDeclStmt:
		return g.genStructDecl(node)
	case *parser.ForStmt:
		return g.genForStmt(node)
	}
	return ""
}

func (g *Generator) genExpr(node parser.Node) string {
	switch n := node.(type) {
	case *parser.StringLiteral:
		return fmt.Sprintf("make_string(%q)", n.Value)
	case *parser.IntLiteral:
		return fmt.Sprintf("make_int(%s)", n.Value)
	case *parser.Float64Literal:
		return fmt.Sprintf("make_float(%s)", n.Value)
	case *parser.BoolLiteral:
		if n.Value {
			return "make_int(1)"
		}
		return "make_int(0)"
	case *parser.Identifier:
		return n.Name
	case *parser.ArrayLiteral:
		var sb strings.Builder
		sb.WriteString("({ Value* _arr = make_array(); ")
		for _, elem := range n.Elements {
			sb.WriteString(fmt.Sprintf("array_push(_arr, %s); ", g.genExpr(elem)))
		}
		sb.WriteString("_arr; })")
		return sb.String()
	case *parser.MapLiteral:
		expr := "make_map()"
		for i := 0; i < len(n.Keys); i++ {
			expr = fmt.Sprintf("map_set(%s, %s, %s)", expr, g.genExpr(n.Keys[i]), g.genExpr(n.Values[i]))
		}
		return expr
	case *parser.IndexExpr:
		return fmt.Sprintf("array_get(%s, %s)", g.genExpr(n.Left), g.genExpr(n.Index))
	case *parser.BinaryExpr:
		if n.Operator == "=" {
			// Handle assignment specially
			left := g.genExpr(n.Left)
			right := g.genExpr(n.Right)

			// If the left side is a matrix index, we need proper type handling
			if _, ok := n.Left.(*parser.MatrixIndexExpr); ok {
				// For matrix assignment, convert the right side to proper C literal
				rightC := g.mapLiteralToC(n.Right)
				return fmt.Sprintf("%s = %s", left, rightC)
			}

			return fmt.Sprintf("%s = %s", left, right)
		}
		if n.Operator == "%" {
			return fmt.Sprintf("karkain_mod(%s, %s)", g.genExpr(n.Left), g.genExpr(n.Right))
		}
		if n.Operator == "&&" {
			return fmt.Sprintf("make_int(is_truthy(%s) && is_truthy(%s))", g.genExpr(n.Left), g.genExpr(n.Right))
		}
		if n.Operator == "||" {
			return fmt.Sprintf("make_int(is_truthy(%s) || is_truthy(%s))", g.genExpr(n.Left), g.genExpr(n.Right))
		}
		return fmt.Sprintf("binary_op(%s, %q, %s)", g.genExpr(n.Left), n.Operator, g.genExpr(n.Right))
	case *parser.CallExpr:
		if n.IsCFunc {
			// Handle C function calls (e.g., C.sqrt) - enforce C. namespace
			args := []string{}
			for _, arg := range n.Args {
				argExpr := g.genExpr(arg)
				// Extract double values from Value wrapper for C functions
				if floatLit, ok := arg.(*parser.Float64Literal); ok {
					args = append(args, floatLit.Value) // Pass raw double value
				} else if intLit, ok := arg.(*parser.IntLiteral); ok {
					args = append(args, intLit.Value) // Pass raw int value
				} else {
					args = append(args, argExpr)
				}
			}
			// Extract the function name after "C." - enforces C. namespace
			funcName := strings.TrimPrefix(n.Function, "C.")
			return fmt.Sprintf("%s(%s)", funcName, strings.Join(args, ", "))
		}
		if n.Function == "len" {
			return fmt.Sprintf("karkain_len(%s)", g.genExpr(n.Args[0]))
		}
		if n.Function == "readFile" {
			return fmt.Sprintf("karkain_readFile(%s)", g.genExpr(n.Args[0]))
		}
		if n.Function == "writeFile" {
			// CORRECT (Two separate calls to g.genExpr)
			return fmt.Sprintf("karkain_writeFile(%s, %s)", g.genExpr(n.Args[0]), g.genExpr(n.Args[1]))
		}
		if n.Function == "http.get" {
			return fmt.Sprintf("http_get(%s)", g.genExpr(n.Args[0]))
		}
		if n.Function == "trim" {
			return fmt.Sprintf("karkain_trim(%s)", g.genExpr(n.Args[0]))
		}
		if n.Function == "contains" {
			return fmt.Sprintf("karkain_contains(%s, %s)", g.genExpr(n.Args[0]), g.genExpr(n.Args[1]))
		}
		if n.Function == "split" {
			return fmt.Sprintf("karkain_split(%s, %s)", g.genExpr(n.Args[0]), g.genExpr(n.Args[1]))
		}
		if n.Function == "sqrt" {
			return fmt.Sprintf("karkain_sqrt(%s)", g.genExpr(n.Args[0]))
		}
		if n.Function == "abs" {
			return fmt.Sprintf("karkain_abs(%s)", g.genExpr(n.Args[0]))
		}
		if n.Function == "pow" {
			return fmt.Sprintf("karkain_pow(%s, %s)", g.genExpr(n.Args[0]), g.genExpr(n.Args[1]))
		}
		if n.Function == "appendArray" {
			return fmt.Sprintf("karkain_appendArray(%s, %s)", g.genExpr(n.Args[0]), g.genExpr(n.Args[1]))
		}
		if n.Function == "hasKey" {
			return fmt.Sprintf("karkain_hasKey(%s, %s)", g.genExpr(n.Args[0]), g.genExpr(n.Args[1]))
		}
		if n.Function == "delete" {
			return fmt.Sprintf("karkain_delete(%s, %s)", g.genExpr(n.Args[0]), g.genExpr(n.Args[1]))
		}
		if n.Function == "mod" {
			return fmt.Sprintf("karkain_mod(%s, %s)", g.genExpr(n.Args[0]), g.genExpr(n.Args[1]))
		}
		if n.Function == "spawn" {
			// Phase 16: Generate spawn expression
			actorName := n.Function
			if len(n.Args) > 0 {
				actorName = g.genExpr(n.Args[0])
			}
			return fmt.Sprintf("// spawn(%s) (placeholder)", actorName)
		}
		args := []string{}
		for _, arg := range n.Args {
			args = append(args, g.genExpr(arg))
		}
		return fmt.Sprintf("%s(%s)", n.Function, strings.Join(args, ", "))
	case *parser.MatrixIndexExpr:
		// Generate row-major offset calculation: (row * cols + col)
		matrixExpr := g.genExpr(n.Matrix)
		// For matrix indexing, we need raw C arithmetic, not Value wrappers
		// Check if row and col are literals
		rowC := ""
		colC := ""
		if rowLit, ok := n.Row.(*parser.IntLiteral); ok {
			rowC = rowLit.Value
		} else if rowIdent, ok := n.Row.(*parser.Identifier); ok {
			rowC = rowIdent.Name
		} else {
			rowC = "0" // fallback
		}

		if colLit, ok := n.Col.(*parser.IntLiteral); ok {
			colC = colLit.Value
		} else if colIdent, ok := n.Col.(*parser.Identifier); ok {
			colC = colIdent.Name
		} else {
			colC = "0" // fallback
		}

		// Use the stored dimension metadata
		if matrixIdent, ok := n.Matrix.(*parser.Identifier); ok {
			colsVar := matrixIdent.Name + "_cols"
			return fmt.Sprintf("%s[(%s * %s + %s)]", matrixExpr, rowC, colsVar, colC)
		}
		// Fallback placeholder
		return fmt.Sprintf("%s[(%s * 2 + %s)]", matrixExpr, rowC, colC)
	case *parser.AddressOf:
		return fmt.Sprintf("&(%s)", g.genExpr(n.Operand))
	case *parser.Dereference:
		return fmt.Sprintf("*(%s)", g.genExpr(n.Operand))
	case *parser.AllocExpr:
		countExpr := g.genExpr(n.Count)
		cType := g.mapKarkainTypeToC(n.Type)
		return fmt.Sprintf("(%s*)malloc(%s * sizeof(%s))", cType, countExpr, cType)
	case *parser.FreeExpr:
		return fmt.Sprintf("free(%s)", g.genExpr(n.Ptr))
	case *parser.DotExpr:
		// Handle dot expressions for C struct access
		left := g.genExpr(n.Left)
		return fmt.Sprintf("%s.%s", left, n.Right)
	case *parser.MeasureExpr:
		// Phase 14: Generate measure expression
		qrName := g.getQuantumRegisterName()
		// Extract raw int value for C function
		targetExpr := ""
		if intLit, ok := n.Qubit.(*parser.IntLiteral); ok {
			targetExpr = intLit.Value
		} else {
			targetExpr = g.mapLiteralToC(n.Qubit)
		}
		return fmt.Sprintf("measure(&%s, %s)", qrName, targetExpr)
	case *parser.SpawnExpr:
		// Phase 16: Generate spawn expression
		return fmt.Sprintf("// spawn(%s) (placeholder)", n.ActorName)
	case *parser.SendExpr:
		// Phase 16: Generate send expression
		channelExpr := g.genExpr(n.Channel)
		messageExpr := g.genExpr(n.Message)
		return fmt.Sprintf("// %s <- %s (send placeholder)", channelExpr, messageExpr)
	case *parser.GlobalIdExpr:
		return fmt.Sprintf("get_global_id(%d)", n.Dimension)
	case *parser.UnaryExpr:
		operand := g.genExpr(n.Operand)
		if n.Operator == "-" {
			return fmt.Sprintf("karkain_negate(%s)", operand)
		}
		if n.Operator == "!" {
			return fmt.Sprintf("karkain_not(%s)", operand)
		}
		return operand
	case *parser.StructLiteral:
		return g.genStructLiteral(n)
	}
	return "make_int(0)"
}

func (g *Generator) detectCompiler(cFile, exeFile string) (string, []string) {
	// Phase 15: Handle WASM/WASI target
	if g.cfg.Target == "wasm32-wasi" {
		// For WASM/WASI, use clang with appropriate flags
		if _, err := exec.LookPath("clang"); err == nil {
			flags := []string{cFile, "-o", exeFile, "--target=wasm32-wasi", "-O2"}
			if g.cfg.Debug {
				flags = append(flags, "-g")
			}
			return "clang", flags
		}
		// Fallback to any clang-like compiler
		if cc := os.Getenv("CC"); cc != "" {
			flags := []string{cFile, "-o", exeFile, "--target=wasm32-wasi", "-O2"}
			if g.cfg.Debug {
				flags = append(flags, "-g")
			}
			return cc, flags
		}
		return "", nil
	}

	// Check if CC environment variable is set
	if cc := os.Getenv("CC"); cc != "" {
		flags := []string{cFile, "-o", exeFile, "-std=c99", "-O0"}
		if runtime.GOOS == "windows" {
			flags = []string{cFile, "-o", exeFile, "-mconsole", "-std=c99", "-O0"}
		}
		if g.cfg.Debug {
			flags = append(flags, "-g")
		}
		return cc, flags
	}

	// Check for gcc first
	if runtime.GOOS == "windows" {
		if _, err := exec.LookPath("gcc"); err == nil {
			flags := []string{cFile, "-o", exeFile, "-mconsole", "-std=c99", "-O0"}
			if g.cfg.Debug {
				flags = append(flags, "-g")
			}
			return "gcc", flags
		}
	} else {
		if _, err := exec.LookPath("gcc"); err == nil {
			flags := []string{cFile, "-o", exeFile, "-std=c99", "-O0"}
			if g.cfg.Debug {
				flags = append(flags, "-g")
			}
			return "gcc", flags
		}
	}
	// Check for clang
	if _, err := exec.LookPath("clang"); err == nil {
		flags := []string{cFile, "-o", exeFile, "-std=c99", "-O0"}
		if g.cfg.Debug {
			flags = append(flags, "-g")
		}
		return "clang", flags
	}
	// Check for MSVC cl.exe
	if runtime.GOOS == "windows" {
		if _, err := exec.LookPath("cl"); err == nil {
			flags := []string{cFile, "/Fe:" + exeFile, "/nologo", "/O0"}
			if g.cfg.Debug {
				flags = append(flags, "/Zi")
			}
			return "cl", flags
		}
	}
	return "", nil
}

// Phase 11: Matrix declaration generation
func (g *Generator) genMatrixDecl(stmt *parser.VarDeclStmt) string {
	matrixDecl, ok := stmt.Value.(*parser.MatrixDecl)
	if !ok {
		return ""
	}

	// Evaluate rows and cols as literal integers if possible
	rows := "2"
	cols := "2"

	if rowLit, ok := matrixDecl.Rows.(*parser.IntLiteral); ok {
		rows = rowLit.Value
	}
	if colLit, ok := matrixDecl.Cols.(*parser.IntLiteral); ok {
		cols = colLit.Value
	}

	cType := g.mapKarkainTypeToC(matrixDecl.DataType)

	// Generate 64-byte aligned contiguous array allocation
	// Use aligned_alloc on POSIX, _aligned_malloc on Windows
	decl := fmt.Sprintf(`
	// Matrix declaration: %s (%s) - 64-byte aligned for SIMD
	%s* %s;
#ifdef _WIN32
	%s = (%s*)_aligned_malloc(%s * %s * sizeof(%s), 64);
#else
	%s = (%s*)aligned_alloc(64, %s * %s * sizeof(%s));
#endif
	if (!%s) {
		fprintf(stderr, "Matrix allocation failed\\n");
		exit(1);
	}
	memset(%s, 0, %s * %s * sizeof(%s));
	// Store matrix dimensions for row-major indexing
	const int64_t %s_rows = %s;
	const int64_t %s_cols = %s;
`, stmt.Name, matrixDecl.DataType, cType, stmt.Name, stmt.Name, cType, rows, cols, cType, stmt.Name, cType, rows, cols, cType, stmt.Name, stmt.Name, rows, cols, cType, stmt.Name, rows, stmt.Name, cols)

	return decl
}

// Phase 11: Map Karkain types to C types (unboxed native types)
func (g *Generator) mapKarkainTypeToC(karkainType string) string {
	switch karkainType {
	case "int":
		return "int64_t"
	case "float64":
		return "double"
	case "string":
		return "char*"
	case "bool":
		return "int"
	default:
		// Handle pointer types
		if strings.HasPrefix(karkainType, "*") {
			baseType := strings.TrimPrefix(karkainType, "*")
			return g.mapKarkainTypeToC(baseType) + "*"
		}
		return "void*" // Fallback
	}
}

// Phase 11: Map Karkain literal values to C literal values
func (g *Generator) mapLiteralToC(node parser.Node) string {
	switch n := node.(type) {
	case *parser.IntLiteral:
		return n.Value
	case *parser.Float64Literal:
		return n.Value
	case *parser.Identifier:
		return n.Name
	case *parser.BinaryExpr:
		// Handle simple binary expressions that might be assignments
		return g.genExpr(node)
	default:
		return g.genExpr(node)
	}
}

// Phase 19: Generate C struct declaration from Karkain struct type
func (g *Generator) genStructDecl(node *parser.StructDeclStmt) string {
	out := fmt.Sprintf("typedef struct {\n")
	for _, field := range node.Fields {
		cType := g.mapKarkainTypeToC(field.Type)
		out += fmt.Sprintf("    %s %s;\n", cType, field.Name)
	}
	out += fmt.Sprintf("} %s;\n\n", node.Name)
	return out
}

// Phase 19: Generate C for loop from Karkain for statement
func (g *Generator) genForStmt(node *parser.ForStmt) string {
	out := "\tfor ("
	if node.Init != nil {
		out += g.genStatement(node.Init)
		// Strip trailing newline from init statement
		out = strings.TrimRight(out, "\n")
	}
	out += "; "
	if node.Condition != nil {
		out += fmt.Sprintf("is_truthy(%s)", g.genExpr(node.Condition))
	}
	out += "; "
	if node.Post != nil {
		postExpr := g.genExpr(node.Post)
		// For expression statements like i = i + 1, strip the trailing semicolon
		out += postExpr
	}
	out += ") {\n"
	for _, bodyStmt := range node.Body {
		out += "\t\t" + g.genStatement(bodyStmt)
	}
	out += "\t}\n"
	return out
}

// Phase 19: Generate C struct literal initialization
func (g *Generator) genStructLiteral(node *parser.StructLiteral) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("({%s _s; ", node.TypeName))
	for _, field := range node.Fields {
		if binExpr, ok := field.(*parser.BinaryExpr); ok {
			if ident, ok := binExpr.Left.(*parser.Identifier); ok {
				sb.WriteString(fmt.Sprintf("_s.%s = %s; ", ident.Name, g.genExpr(binExpr.Right)))
			}
		}
	}
	sb.WriteString(fmt.Sprintf("_s; })"))
	return sb.String()
}

// Phase 14: Get the quantum register name
func (g *Generator) getQuantumRegisterName() string {
	return "qr"
}

// Phase 18: Generate GPU kernel declaration and host launcher
func (g *Generator) genKernelDecl(kernel *parser.KernelDeclStmt) string {
	gen := NewGPUGenerator()
	openclSrc, err := gen.GenerateOpenCL(kernel)
	if err != nil {
		return fmt.Sprintf("\t// GPU kernel generation error: %s\n", err.Error())
	}

	escaped := strings.ReplaceAll(openclSrc, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
	escaped = strings.ReplaceAll(escaped, "\n", "\\n\"\n\"")

	result := fmt.Sprintf("\n// Phase 18: GPU kernel '%s' - OpenCL source\n", kernel.Name)
	result += fmt.Sprintf("static const char* %s_opencl_source =\n", kernel.Name)
	result += fmt.Sprintf("\"%s\";\n\n", escaped)

	result += GenerateHostLauncher(kernel)

	return result
}

// Phase 14: Generate AVX2 matrix multiplication kernel
func (g *Generator) genAVX2MatrixMul(rowsA, colsA, colsB string) string {
	return fmt.Sprintf(`
// AVX2 matrix multiplication kernel (256-bit FMA)
#if HAS_AVX2
void matrix_mul_avx2(double* A, double* B, double* C, int64_t rowsA, int64_t colsA, int64_t colsB) {
    int64_t i, j, k;
    int64_t simd_width = 4; // 256-bit AVX2 = 4 doubles
    
    for (i = 0; i < rowsA; i++) {
        for (j = 0; j < colsB; j += simd_width) {
            __m256d sum = _mm256_setzero_pd();
            
            for (k = 0; k < colsA; k++) {
                __m256d a_vec = _mm256_set1_pd(A[i * colsA + k]);
                __m256d b_vec = _mm256_loadu_pd(&B[k * colsB + j]);
                sum = _mm256_fmadd_pd(a_vec, b_vec, sum);
            }
            
            _mm256_storeu_pd(&C[i * colsB + j], sum);
        }
        
        // Handle remaining columns
        for (j = (colsB / simd_width) * simd_width; j < colsB; j++) {
            double sum = 0.0;
            for (k = 0; k < colsA; k++) {
                sum += A[i * colsA + k] * B[k * colsB + j];
            }
            C[i * colsB + j] = sum;
        }
    }
}
#endif
`)
}

// Phase 14: Generate fallback scalar matrix multiplication
func (g *Generator) genScalarMatrixMul() string {
	return `
// Scalar matrix multiplication (fallback)
void matrix_mul_scalar(double* A, double* B, double* C, int64_t rowsA, int64_t colsA, int64_t colsB) {
    int64_t i, j, k;
    for (i = 0; i < rowsA; i++) {
        for (j = 0; j < colsB; j++) {
            double sum = 0.0;
            for (k = 0; k < colsA; k++) {
                sum += A[i * colsA + k] * B[k * colsB + j];
            }
            C[i * colsB + j] = sum;
        }
    }
}
`
}
