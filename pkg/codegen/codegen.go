package codegen

import (
	"context"
	"fmt"
	"io"
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
	DisableSSA  bool   // Phase 53: disable SSA IR pipeline (fallback to legacy emission)

	// Stdout / Stderr direct the executed program's output. When nil they
	// default to the process standard streams. Used by the test runner to
	// capture per-test output for structured reporting (KTF-001).
	Stdout io.Writer
	Stderr io.Writer
}

func NewConfig() Config {
	return Config{
		Target: "native", // Default to native target
	}
}

// userFuncC is the deterministic C symbol for a user-defined Karkain function.
// User code lives in the karkain_user_* namespace so Karkain functions can
// never collide with the runtime's karkain_* helpers (the C-library collision
// fix). `main` (the C entry point) and `getArgs` (the argument intrinsic) keep
// their canonical names. The mapping is injective because Karkain identifiers
// are already C-legal, so sanitizeC is an identity on them.
func userFuncC(name string) string {
	if name == "main" || name == "getArgs" {
		return name
	}
	return "karkain_user_" + name
}

type Generator struct {
	cfg              Config
	enumDecls        map[string]*parser.EnumDecl // Phase 45: tracked enum declarations
	lambdaCount      int                         // Phase 48: unique lambda naming
	lambdaBuf        strings.Builder             // Phase 48: lambda function definitions to inject
	closureVars      map[string]bool             // Phase 54: let-bound lambdas with captures
	lastClosureInit  string                      // Phase 54: env-instance init emitted at binding site
	quantumRegisters []string                    // Phase 14: declared quantum registers, in declaration order
	needsHTTP        bool                        // Phase 55b: track if http.get is used (strip stub otherwise)
	sourceFile       string                      // Phase 55b: source file for #line directives
	userFuncs        map[string]bool             // Phase 83: user-declared FuncDecl names (mangling + shadowing)
}

func New(cfg Config) *Generator {
	return &Generator{cfg: cfg, enumDecls: make(map[string]*parser.EnumDecl),
		closureVars: make(map[string]bool), userFuncs: make(map[string]bool)}
}

func (g *Generator) GenerateAndCompile(prog *parser.Program, sourceFile string) error {
	var sb strings.Builder

	// Pre-scan: detect http.get calls to conditionally include HTTP runtime
	g.needsHTTP = false
	g.sourceFile = strings.Replace(sourceFile, "\\", "/", -1)
	for _, stmt := range prog.Statements {
		g.scanNodeForHTTP(stmt)
	}
	if g.needsHTTP {
		sb.WriteString("#define KARKAIN_USE_HTTP\n")
	}
	// Pre-scan: collect user-declared function names for deterministic symbol
	// mangling and user-functions-first builtin shadowing.
	g.userFuncs = make(map[string]bool)
	for _, stmt := range prog.Statements {
		if fn, ok := stmt.(*parser.FuncDecl); ok && fn.Name != "main" && fn.Name != "getArgs" {
			g.userFuncs[fn.Name] = true
		}
	}

	sb.WriteString(g.generateCHeader())

	// Add AVX2 and scalar matrix multiplication kernels
	sb.WriteString(g.genAVX2MatrixMul("rowsA", "colsA", "colsB"))
	sb.WriteString(g.genScalarMatrixMul())

	// Emit #line directive for source mapping if debug mode is enabled
	if g.cfg.Debug {
		// Convert backslashes to forward slashes for cross-platform compatibility
		cleanSourceFile := strings.Replace(sourceFile, "\\", "/", -1)
		fmt.Fprintf(&sb, "#line 1 \"%s\"\n", cleanSourceFile)
	}

	// Inject C import blocks - check if they exist first
	if len(prog.CImports) > 0 {
		for _, cImport := range prog.CImports {
			if cImport.Content != "" && !strings.Contains(cImport.Content, "Phase 11") {
				sb.WriteString(cImport.Content)
				sb.WriteByte('\n')
			}
		}
	}

	// Phase 45: Generate enum type declarations before functions
	for _, stmt := range prog.Statements {
		if enum, ok := stmt.(*parser.EnumDecl); ok {
			g.enumDecls[enum.Name] = enum
			sb.WriteString(g.genEnumDecl(enum))
		}
	}

	// Phase 50: Generate struct type declarations before functions
	for _, stmt := range prog.Statements {
		if st, ok := stmt.(*parser.StructDeclStmt); ok {
			sb.WriteString(g.genStructDecl(st))
		}
	}

	// Phase 40: Generate forward declarations for all functions
	// This enables cross-file references when multiple .kark files are concatenated
	for _, stmt := range prog.Statements {
		if fn, ok := stmt.(*parser.FuncDecl); ok {
			params := []string{}
			for _, p := range fn.Params {
				params = append(params, "Value "+p)
			}
			retType := "Value"
			if fn.Name == "main" {
				retType = "int"
				params = []string{"int _karkain_argc", "char** _karkain_argv"}
			}
			fmt.Fprintf(&sb, "%s %s(%s);\n", retType, userFuncC(fn.Name), strings.Join(params, ", "))
		}
	}
	sb.WriteByte('\n')

	// Generate all function declarations
	for _, stmt := range prog.Statements {
		if fn, ok := stmt.(*parser.FuncDecl); ok {
			// Phase 53: SSA IR pipeline â€” lower, optimize, verify, emit.
			// Falls back to legacy emission on any lowering/verification failure.
			if !g.cfg.DisableSSA {
				if out, ok2 := g.emitFunctionViaIR(prog, fn); ok2 {
					sb.WriteString(out)
					continue
				}
			}
			sb.WriteString(g.genFuncDecl(fn))
		}
	}

	// Phase 18: Generate GPU kernel declarations and host launchers
	for _, stmt := range prog.Statements {
		if kernel, ok := stmt.(*parser.KernelDeclStmt); ok {
			sb.WriteString(g.genKernelDecl(kernel))
		}
	}

	cCode := sb.String()

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

	// CompileOnly: write C source and return (used by bootstrap to get C output)
	if g.cfg.CompileOnly {
		return nil
	}

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
		runCmd.Stdout = g.cfg.Stdout
		if runCmd.Stdout == nil {
			runCmd.Stdout = os.Stdout
		}
		runCmd.Stderr = g.cfg.Stderr
		if runCmd.Stderr == nil {
			runCmd.Stderr = os.Stderr
		}
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
#include <gmp.h>

// Phase 70: Atomics (memory orderings, atomic load/store/rmw).
#include <stdatomic.h>
// Phase 70: Portable SIMD intrinsics surface (vector lane types).
#if defined(__x86_64__) || defined(__i386__) || defined(_M_X64) || defined(_M_IX86)
#include <immintrin.h>
#define KARKAIN_SIMD_SSE 1
#else
#define KARKAIN_SIMD_SSE 0
typedef float float4 __attribute__((vector_size(16)));
typedef float float8 __attribute__((vector_size(32)));
typedef double double4 __attribute__((vector_size(32)));
#endif

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

// Windows <windows.h> defines min/max as function-like macros, which collides
// with Karkain user functions named min/max in the generated C. Undefine them
// so Value max(...) / Value min(...) declarations compile cleanly.
#ifdef min
#undef min
#endif
#ifdef max
#undef max
#endif

// Phase 14: Self-hosting directory listing (POSIX dir iteration, available in
// MinGW and all POSIX toolchains).
#include <dirent.h>

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
#ifdef KARKAIN_USE_HTTP
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
#endif

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

typedef enum { TYPE_INT, TYPE_FLOAT64, TYPE_STRING, TYPE_ARRAY, TYPE_MAP, TYPE_BOOL, TYPE_BIGINT, TYPE_BIGFLOAT, TYPE_OPTION, TYPE_RESULT } ValueType;

typedef struct Value {
    ValueType type;
    union {
        long long intVal;
        double floatVal;
        char* strVal;
        mpz_t bigIntVal;
        mpf_t bigFloatVal;
        struct {
            struct Value** items;
            int length;
        } arrVal;
        struct {
            struct Value** keys;
            struct Value** values;
            int length;
        } mapVal;
        struct {
            int tag;           // 0=None, 1=Some
            struct Value* inner;
        } optVal;
        struct {
            int tag;           // 0=Ok, 1=Err
            struct Value* okVal;
            struct Value* errVal;
        } resVal;
    };
} Value;

// Phase 45: Built-in Result tagged union
typedef enum { Result_Tag_default, Result_Tag_Ok, Result_Tag_Err } Result_Tag;
typedef struct { Result_Tag tag; union { Value* Ok; Value* Err; }; } Result;

static inline Result Result_make_Ok(Value val) { Result r; r.tag = Result_Tag_Ok; Value* h = (Value*)malloc(sizeof(Value)); *h = val; r.Ok = h; return r; }
static inline Result Result_make_Err(Value val) { Result r; r.tag = Result_Tag_Err; Value* h = (Value*)malloc(sizeof(Value)); *h = val; r.Err = h; return r; }
#define Result_Ok_ok ((Result){ .tag = Result_Tag_Ok })
#define Result_Err_err ((Result){ .tag = Result_Tag_Err })

// ============================================================
// Phase 49: Value Representation & Allocation Model
// ============================================================
// Allocation classification:
//   VAL_IMMEDIATE â€” int, float, bool: value stored inline in Value union, no heap
//   VAL_HEAP      â€” string, array, map, bigint, bigfloat: data on heap
//   VAL_REF       â€” &T / &mut T: zero-cost pointer, references another Value
typedef enum { VAL_IMMEDIATE, VAL_HEAP, VAL_REF } ValueClass;

static inline ValueClass value_class(Value v) {
    switch (v.type) {
        case TYPE_INT:
        case TYPE_FLOAT64:
        case TYPE_BOOL:
            return VAL_IMMEDIATE;
        case TYPE_STRING:
        case TYPE_ARRAY:
        case TYPE_MAP:
        case TYPE_BIGINT:
        case TYPE_BIGFLOAT:
        case TYPE_OPTION:
        case TYPE_RESULT:
            return VAL_HEAP;
    }
    return VAL_IMMEDIATE;
}

// Phase 52: Option/Result constructors â€” take Value, heap-allocate inner copy for lifetime, return Value
static inline Value option_some(Value inner) {
    Value v; v.type = TYPE_OPTION; v.optVal.tag = 1;
    Value* h = (Value*)malloc(sizeof(Value)); *h = inner; v.optVal.inner = h; return v;
}
static inline Value option_none(void) {
    Value v; v.type = TYPE_OPTION; v.optVal.tag = 0; v.optVal.inner = 0; return v;
}
static inline Value result_ok(Value inner) {
    Value v; v.type = TYPE_RESULT; v.resVal.tag = 0;
    Value* h = (Value*)malloc(sizeof(Value)); *h = inner; v.resVal.okVal = h; v.resVal.errVal = 0; return v;
}
static inline Value result_err(Value err) {
    Value v; v.type = TYPE_RESULT; v.resVal.tag = 1; v.resVal.okVal = 0;
    Value* h = (Value*)malloc(sizeof(Value)); *h = err; v.resVal.errVal = h; return v;
}

// Stack-allocated primitive constructors (no malloc, for use in generated C)
// These create Value structs directly on the C stack as compound literals.
static inline Value mk_int(long long v) {
    Value val; val.type = TYPE_INT; val.intVal = v; return val;
}
static inline Value mk_float(double v) {
    Value val; val.type = TYPE_FLOAT64; val.floatVal = v; return val;
}
static inline Value mk_bool_val(int v) {
    Value val; val.type = TYPE_BOOL; val.intVal = v ? 1 : 0; return val;
}
static inline Value mk_nil(void) {
    Value val; val.type = TYPE_INT; val.intVal = 0; return val;
}

// Phase 52: Value-by-Value Runtime API â€” ALL constructors return Value (stack-allocated struct).
// Heap-backed data (strdup'd strings, array/map element storage, GMP state) remains on the heap,
// but the Value wrapper itself lives on the C stack â€” no malloc for primitives.
// Container storage heap-copies elements on insert; mutation functions take Value*.

// Layout assertion: Value must fit in a single cache line for efficient stack passing
_Static_assert(sizeof(Value) <= 64, "Value must fit in a 64-byte cache line");

// Phase 52: Value Representation & Allocation Model
// Small integer pool: values -128..127 never malloc (covers most literals, loop counters, booleans)
// BUG-6 fix: pool entries are returned as COPIES (Value by value), so callers can never
// mutate shared pool state â€” each caller owns its own Value.
Value make_int(long long v) {
    if (v >= -128 && v <= 127) {
        static Value pool[256];
        static int pool_init = 0;
        if (!pool_init) {
            for (int i = 0; i < 256; i++) {
                pool[i].type = TYPE_INT;
                pool[i].intVal = i - 128;
            }
            pool_init = 1;
        }
        Value out = pool[v + 128];
        return out;
    }
    Value val;
    val.type = TYPE_INT;
    val.intVal = v;
    return val;
}

Value make_float(double v) {
    Value val;
    val.type = TYPE_FLOAT64;
    val.floatVal = v;
    return val;
}

Value make_bigint(const char* s) {
    Value val;
    val.type = TYPE_BIGINT;
    mpz_init(val.bigIntVal);
    mpz_set_str(val.bigIntVal, s, 10);
    return val;
}

Value make_bigint_from_long(long long v) {
    Value val;
    val.type = TYPE_BIGINT;
    mpz_init(val.bigIntVal);
    mpz_set_si(val.bigIntVal, v);
    return val;
}

Value make_bigfloat(const char* s) {
    Value val;
    val.type = TYPE_BIGFLOAT;
    mpf_init(val.bigFloatVal);
    mpf_set_str(val.bigFloatVal, s, 10);
    return val;
}

Value make_bigfloat_from_double(double v) {
    Value val;
    val.type = TYPE_BIGFLOAT;
    mpf_init(val.bigFloatVal);
    mpf_set_d(val.bigFloatVal, v);
    return val;
}

// Karkain argument API (getArgs) backing store. Populated by the generated C
// entry point from argc/argv so Karkain code can read real process arguments.
static int _karkain_gargc = 0;
static char** _karkain_gargv = NULL;

Value make_string(const char* s) {
    Value val;
    val.type = TYPE_STRING;
    val.strVal = strdup(s);
    return val;
}

Value make_array() {
    Value val;
    val.type = TYPE_ARRAY;
    val.arrVal.items = NULL;
    val.arrVal.length = 0;
    return val;
}

Value make_map() {
    Value val;
    val.type = TYPE_MAP;
    val.mapVal.keys = NULL;
    val.mapVal.values = NULL;
    val.mapVal.length = 0;
    return val;
}

// Mutation takes Value* (pointer to caller's stack variable); element is heap-copied for storage.
void array_push(Value* arr, Value elem) {
    if (!arr || arr->type != TYPE_ARRAY) return;
    Value* copy = (Value*)malloc(sizeof(Value));
    *copy = elem;
    arr->arrVal.length++;
    arr->arrVal.items = (Value**)realloc(arr->arrVal.items, sizeof(Value*) * arr->arrVal.length);
    arr->arrVal.items[arr->arrVal.length - 1] = copy;
}

int values_equal(Value a, Value b) {
    if (a.type != b.type) return 0;
    if (a.type == TYPE_INT) return a.intVal == b.intVal;
    if (a.type == TYPE_BOOL) return a.intVal == b.intVal;
    if (a.type == TYPE_FLOAT64) {
        double diff = a.floatVal - b.floatVal;
        return (diff > -1e-9 && diff < 1e-9);
    }
    if (a.type == TYPE_STRING) return strcmp(a.strVal, b.strVal) == 0;
    if (a.type == TYPE_ARRAY) {
        if (a.arrVal.length != b.arrVal.length) return 0;
        for (int i = 0; i < a.arrVal.length; i++) {
            if (!values_equal(*a.arrVal.items[i], *b.arrVal.items[i])) return 0;
        }
        return 1;
    }
    if (a.type == TYPE_MAP) {
        if (a.mapVal.length != b.mapVal.length) return 0;
        for (int i = 0; i < a.mapVal.length; i++) {
            int found = 0;
            for (int j = 0; j < b.mapVal.length; j++) {
                if (values_equal(*a.mapVal.keys[i], *b.mapVal.keys[j]) &&
                    values_equal(*a.mapVal.values[i], *b.mapVal.values[j])) {
                    found = 1; break;
                }
            }
            if (!found) return 0;
        }
        return 1;
    }
    return 0;
}

// Phase 55: slicing with bounds checking; negative end counts from length, -1 = open
Value karkain_slice(Value v, Value s, Value e) {
    long long start = (s.type == TYPE_INT) ? s.intVal : 0;
    long long end = (e.type == TYPE_INT) ? e.intVal : -1;
    if (v.type == TYPE_STRING) {
        long long n = (long long)strlen(v.strVal);
        if (end < 0) end += n + 1;
        if (start < 0) start = 0;
        if (end > n) end = n;
        if (start >= end || start >= n) return make_string("");
        long long len = end - start;
        char* buf = (char*)malloc((size_t)len + 1);
        memcpy(buf, v.strVal + start, (size_t)len);
        buf[len] = 0;
        Value r = make_string(buf);
        free(buf);
        return r;
    }
    if (v.type == TYPE_ARRAY) {
        long long n = (long long)v.arrVal.length;
        if (end < 0) end += n + 1;
        if (start < 0) start = 0;
        if (end > n) end = n;
        Value out = make_array();
        if (start < end) {
            for (long long i = start; i < end; i++) array_push(&out, *v.arrVal.items[i]);
        }
        return out;
    }
    return make_int(0);
}

// By-value helpers for the self-hosted compiler (replaces stale pointer-based
// runtime.c versions so main.c is fully self-contained).
Value karkain_substr(Value str, Value start, Value length) {
    if (str.type != TYPE_STRING || start.type != TYPE_INT || length.type != TYPE_INT) return make_string("");
    int s = (int)start.intVal;
    int l = (int)length.intVal;
    int slen = (int)strlen(str.strVal);
    if (s < 0) s = 0;
    if (s >= slen) return make_string("");
    if (s + l > slen) l = slen - s;
    if (l < 0) l = 0;
    char* buf = (char*)malloc(l + 1);
    memcpy(buf, str.strVal + s, (size_t)l);
    buf[l] = '\0';
    Value r = make_string(buf);
    free(buf);
    return r;
}

Value karkain_str(Value v) {
    if (v.type == TYPE_STRING) return make_string(v.strVal);
    if (v.type == TYPE_INT) {
        char buf[64];
        snprintf(buf, sizeof(buf), "%lld", v.intVal);
        return make_string(buf);
    }
    if (v.type == TYPE_FLOAT64) {
        char buf[64];
        snprintf(buf, sizeof(buf), "%g", v.floatVal);
        return make_string(buf);
    }
    if (v.type == TYPE_BOOL) return make_string(v.intVal ? "true" : "false");
    if (v.type == TYPE_ARRAY) return make_string("[array]");
    if (v.type == TYPE_MAP) return make_string("[map]");
    return make_string("");
}

Value karkain_int(Value v) {
    if (v.type == TYPE_INT) return make_int(v.intVal);
    if (v.type == TYPE_FLOAT64) return make_int((long long)v.floatVal);
    if (v.type == TYPE_STRING) return make_int(atoll(v.strVal));
    if (v.type == TYPE_BOOL) return make_int(v.intVal);
    return make_int(0);
}

Value karkain_system(Value cmd) {
    if (cmd.type != TYPE_STRING) return make_int(-1);
    int result = system(cmd.strVal);
    return make_int(result);
}

Value karkain_removeFile(Value path) {
    if (path.type != TYPE_STRING) return make_int(0);
    int result = remove(path.strVal);
    return make_int(result == 0 ? 1 : 0);
}

static FILE* _karkain_files[256];
static int _karkain_file_count = 0;

Value openFile(Value path) {
    if (path.type != TYPE_STRING) return make_string("");
    FILE* f = fopen(path.strVal, "rb");
    if (!f) return make_string("");
    fclose(f);
    return make_string(path.strVal);
}

Value readLine(Value handle) {
    if (handle.type != TYPE_STRING || strlen(handle.strVal) == 0) return make_string("");
    int idx = -1;
    int i;
    for (i = 0; i < _karkain_file_count; i++) {
        if (_karkain_files[i] != NULL) { idx = i; break; }
    }
    if (idx == -1) {
        if (_karkain_file_count >= 256) return make_string("");
        idx = _karkain_file_count;
        _karkain_files[idx] = fopen(handle.strVal, "rb");
        _karkain_file_count++;
    } else if (_karkain_files[idx] == NULL) {
        _karkain_files[idx] = fopen(handle.strVal, "rb");
    }
    if (!_karkain_files[idx]) return make_string("");
    char buf[4096];
    if (fgets(buf, sizeof(buf), _karkain_files[idx]) == NULL) return make_string("");
    int len = (int)strlen(buf);
    while (len > 0 && (buf[len - 1] == '\n' || buf[len - 1] == '\r')) buf[--len] = '\0';
    return make_string(buf);
}

Value closeFile(Value handle) {
    (void)handle;
    int i;
    for (i = 0; i < _karkain_file_count; i++) {
        if (_karkain_files[i] != NULL) { fclose(_karkain_files[i]); _karkain_files[i] = NULL; break; }
    }
    return make_int(1);
}

Value createFile(Value path) {
    if (path.type != TYPE_STRING) return make_string("");
    FILE* f = fopen(path.strVal, "wb");
    if (f) fclose(f);
    return make_string(path.strVal);
}

Value writeToFile(Value handle, Value content) {
    if (handle.type != TYPE_STRING || content.type != TYPE_STRING) return make_int(0);
    FILE* f = fopen(handle.strVal, "wb");
    if (!f) return make_int(0);
    fputs(content.strVal, f);
    fclose(f);
    return make_int(1);
}

Value removeFile(Value path) {
    return karkain_removeFile(path);
}

// Phase 55: formatting â€” fmt("x={} y={}", a, b); {} consumes next arg in order
#include <stdarg.h>
Value karkain_fmt(Value fstr, int count, ...) {
    if (fstr.type != TYPE_STRING) return make_string("");
    const char* f = fstr.strVal;
    size_t cap = strlen(f) + (size_t)(count > 0 ? count : 0) * 32 + 16;
    char* buf = (char*)malloc(cap);
    size_t pos = 0;
    va_list ap;
    va_start(ap, count);
    while (*f && pos + 64 < cap) {
        if (f[0] == '{' && f[1] == '}') {
            f += 2;
            if (count-- <= 0) continue;
            Value v = va_arg(ap, Value);
            char tmp[48];
            const char* piece = "";
            if (v.type == TYPE_STRING) piece = v.strVal;
            else if (v.type == TYPE_FLOAT64) { snprintf(tmp, sizeof(tmp), "%g", v.floatVal); piece = tmp; }
            else if (v.type == TYPE_INT || v.type == TYPE_BOOL) { snprintf(tmp, sizeof(tmp), "%lld", v.intVal); piece = tmp; }
            size_t pl = strlen(piece);
            if (pos + pl + 1 >= cap) break;
            memcpy(buf + pos, piece, pl);
            pos += pl;
            continue;
        }
        buf[pos++] = *f++;
    }
    va_end(ap);
    buf[pos] = 0;
    Value r = make_string(buf);
    free(buf);
    return r;
}

// Mutation takes Value*; key and value are heap-copied for storage. Returns void.
void map_set(Value* m, Value k, Value v) {
    if (!m || m->type != TYPE_MAP) return;
    for (int i = 0; i < m->mapVal.length; i++) {
        if (values_equal(*m->mapVal.keys[i], k)) {
            Value* vc = (Value*)malloc(sizeof(Value));
            *vc = v;
            m->mapVal.values[i] = vc;
            return;
        }
    }
    Value* kc = (Value*)malloc(sizeof(Value));
    *kc = k;
    Value* vc2 = (Value*)malloc(sizeof(Value));
    *vc2 = v;
    m->mapVal.length++;
    m->mapVal.keys = (Value**)realloc(m->mapVal.keys, sizeof(Value*) * m->mapVal.length);
    m->mapVal.values = (Value**)realloc(m->mapVal.values, sizeof(Value*) * m->mapVal.length);
    m->mapVal.keys[m->mapVal.length - 1] = kc;
    m->mapVal.values[m->mapVal.length - 1] = vc2;
}

Value map_get(Value m, Value k) {
    if (m.type != TYPE_MAP) return make_int(0);
    for (int i = 0; i < m.mapVal.length; i++) {
        if (values_equal(*m.mapVal.keys[i], k)) {
            return *m.mapVal.values[i];
        }
    }
    return make_int(0);
}

Value array_get(Value arr, Value idx) {
    if (arr.type == TYPE_MAP) return map_get(arr, idx);
    if (arr.type != TYPE_ARRAY || idx.type != TYPE_INT) return make_int(0);
    int i = (int)idx.intVal;
    if (i < 0 || i >= arr.arrVal.length) return make_int(0);
    return *arr.arrVal.items[i];
}

// String indexing: s[i] returns the character at position i as a one-character
// string (empty string when out of bounds). Enables char-level algorithms such
// as Levenshtein distance without requiring an explicit split into an array.
Value string_get(Value s, long long i) {
    if (s.type != TYPE_STRING || s.strVal == NULL) return make_string("");
    long long n = (long long)strlen(s.strVal);
    if (i < 0 || i >= n) return make_string("");
    char buf[2] = { s.strVal[i], '\0' };
    return make_string(buf);
}

Value array_or_string_get(Value container, Value idx) {
    if (container.type == TYPE_STRING) return string_get(container, idx.type == TYPE_INT ? idx.intVal : -1);
    return array_get(container, idx);
}

// Phase 50: Index assignment â€” sets element at index for both arrays and maps
// Mutation takes Value*; the value is heap-copied for storage. Returns void.
void index_set(Value* container, Value idx, Value val) {
    if (!container) return;
    if (container->type == TYPE_MAP) { map_set(container, idx, val); return; }
    if (container->type == TYPE_ARRAY && idx.type == TYPE_INT) {
        int i = (int)idx.intVal;
        if (i >= 0 && i < container->arrVal.length) {
            Value* copy = (Value*)malloc(sizeof(Value));
            *copy = val;
            container->arrVal.items[i] = copy;
            return;
        }
    }
}

Value karkain_len(Value v) {
    if (v.type == TYPE_ARRAY) return make_int(v.arrVal.length);
    if (v.type == TYPE_MAP) return make_int(v.mapVal.length);
    if (v.type == TYPE_STRING) return make_int(strlen(v.strVal));
    return make_int(0);
}

Value karkain_readFile(Value path) {
    if (path.type != TYPE_STRING) return make_string("");
    FILE* f = fopen(path.strVal, "rb");
    if (!f) return make_string("");
    fseek(f, 0, SEEK_END);
    long sz = ftell(f);
    fseek(f, 0, SEEK_SET);
    char* buf = (char*)malloc(sz + 1);
    fread(buf, 1, sz, f);
    fclose(f);
    buf[sz] = '\0';
    Value res = make_string(buf);
    free(buf);
    return res;
}

Value karkain_writeFile(Value path, Value content) {
    if (path.type != TYPE_STRING) return make_int(0);
    FILE* f = fopen(path.strVal, "wb");
    if (!f) return make_int(0);
    const char* str = (content.type == TYPE_STRING) ? content.strVal : "";
    fputs(str, f);
    fclose(f);
    return make_int(1);
}

void print_value(Value v) {
    if (v.type == TYPE_INT) {
        printf("%lld\n", v.intVal);
    } else if (v.type == TYPE_BOOL) {
        printf("%s\n", v.intVal ? "true" : "false");
    } else if (v.type == TYPE_FLOAT64) {
        printf("%g\n", v.floatVal);
    } else if (v.type == TYPE_STRING) {
        printf("%s\n", v.strVal);
    } else if (v.type == TYPE_BIGINT) {
        mpz_out_str(stdout, 10, v.bigIntVal);
        printf("\n");
    } else if (v.type == TYPE_BIGFLOAT) {
        mp_exp_t exp;
        char* str = mpf_get_str(NULL, &exp, 10, 0, v.bigFloatVal);
        int len = strlen(str);
        if (exp <= 0) {
            printf("0.");
            for (int i = 0; i < -exp; i++) printf("0");
            printf("%s\n", str);
        } else if (exp >= len) {
            printf("%s", str);
            for (int i = len; i < exp; i++) printf("0");
            printf("\n");
        } else {
            for (int i = 0; i < exp; i++) printf("%c", str[i]);
            printf(".");
            for (int i = exp; i < len; i++) printf("%c", str[i]);
            printf("\n");
        }
        void (*freefunc)(void *, size_t);
        mp_get_memory_functions(NULL, NULL, &freefunc);
        freefunc(str, len + 1);
    } else if (v.type == TYPE_ARRAY) {
        printf("[");
        for (int i = 0; i < v.arrVal.length; i++) {
            if (i > 0) printf(", ");
            Value* item = v.arrVal.items[i];
            if (item->type == TYPE_STRING) printf("\"%s\"", item->strVal);
            else if (item->type == TYPE_INT) printf("%lld", item->intVal);
        }
        printf("]\n");
    } else if (v.type == TYPE_MAP) {
        printf("{");
        for (int i = 0; i < v.mapVal.length; i++) {
            if (i > 0) printf(", ");
            Value* k = v.mapVal.keys[i];
            Value* item = v.mapVal.values[i];
            if (k->type == TYPE_STRING) printf("\"%s\": ", k->strVal);
            else printf("%lld: ", k->intVal);
            if (item->type == TYPE_STRING) printf("\"%s\"", item->strVal);
            else printf("%lld", item->intVal);
        }
        printf("}\n");
    } else if (v.type == TYPE_OPTION) {
        if (v.optVal.tag == 0) printf("None\n");
        else { printf("Some("); print_value(*v.optVal.inner); printf(")\n"); }
    } else if (v.type == TYPE_RESULT) {
        if (v.resVal.tag == 0) { printf("Ok("); print_value(*v.resVal.okVal); printf(")\n"); }
        else { printf("Err("); print_value(*v.resVal.errVal); printf(")\n"); }
    }
    fflush(stdout);
}

Value binary_op(Value left, const char* op, Value right) {
    // Phase 55: string comparison operators (ordering + equality)
    if (left.type == TYPE_STRING && right.type == TYPE_STRING) {
        int c = strcmp(left.strVal, right.strVal);
        if (strcmp(op, "==") == 0) return make_int(c == 0);
        if (strcmp(op, "!=") == 0) return make_int(c != 0);
        if (strcmp(op, "<") == 0) return make_int(c < 0);
        if (strcmp(op, ">") == 0) return make_int(c > 0);
        if (strcmp(op, "<=") == 0) return make_int(c <= 0);
        if (strcmp(op, ">=") == 0) return make_int(c >= 0);
    }
    // String concatenation
    if (strcmp(op, "+") == 0 && left.type == TYPE_STRING && right.type == TYPE_STRING) {
        size_t len = strlen(left.strVal) + strlen(right.strVal);
        char* buf = (char*)malloc(len + 1);
        strcpy(buf, left.strVal);
        strcat(buf, right.strVal);
        Value result = make_string(buf);
        free(buf);
        return result;
    }
    // Float64 arithmetic
    if (left.type == TYPE_FLOAT64 || right.type == TYPE_FLOAT64) {
        double l = (left.type == TYPE_FLOAT64) ? left.floatVal : (double)left.intVal;
        double r = (right.type == TYPE_FLOAT64) ? right.floatVal : (double)right.intVal;
        if (strcmp(op, "+") == 0) return make_float(l + r);
        if (strcmp(op, "-") == 0) return make_float(l - r);
        if (strcmp(op, "*") == 0) return make_float(l * r);
        if (strcmp(op, "/") == 0) return make_float(r != 0.0 ? l / r : 0.0);
        if (strcmp(op, "%") == 0) return make_float(r != 0.0 ? fmod(l, r) : 0.0);
        if (strcmp(op, ">") == 0) return make_int(l > r);
        if (strcmp(op, "<") == 0) return make_int(l < r);
        if (strcmp(op, ">=") == 0) return make_int(l >= r);
        if (strcmp(op, "<=") == 0) return make_int(l <= r);
        if (strcmp(op, "==") == 0) return make_int(l == r);
        if (strcmp(op, "!=") == 0) return make_int(l != r);
    }
    // BigFloat arithmetic (promotes int/bigint to bigfloat)
    if (left.type == TYPE_BIGFLOAT || right.type == TYPE_BIGFLOAT) {
        mpf_t l, r;
        mpf_init(l); mpf_init(r);
        if (left.type == TYPE_BIGFLOAT) mpf_set(l, left.bigFloatVal);
        else if (left.type == TYPE_BIGINT) mpf_set_z(l, left.bigIntVal);
        else if (left.type == TYPE_INT) mpf_set_si(l, left.intVal);
        else mpf_set_d(l, left.floatVal);
        if (right.type == TYPE_BIGFLOAT) mpf_set(r, right.bigFloatVal);
        else if (right.type == TYPE_BIGINT) mpf_set_z(r, right.bigIntVal);
        else if (right.type == TYPE_INT) mpf_set_si(r, right.intVal);
        else mpf_set_d(r, right.floatVal);
        Value result;
        if (strcmp(op, "+") == 0) { mpf_add(l, l, r); result = make_bigfloat_from_double(0); mpf_set(result.bigFloatVal, l); }
        else if (strcmp(op, "-") == 0) { mpf_sub(l, l, r); result = make_bigfloat_from_double(0); mpf_set(result.bigFloatVal, l); }
        else if (strcmp(op, "*") == 0) { mpf_mul(l, l, r); result = make_bigfloat_from_double(0); mpf_set(result.bigFloatVal, l); }
        else if (strcmp(op, "/") == 0) { mpf_div(l, l, r); result = make_bigfloat_from_double(0); mpf_set(result.bigFloatVal, l); }
        else if (strcmp(op, ">") == 0) result = make_int(mpf_cmp(l, r) > 0);
        else if (strcmp(op, "<") == 0) result = make_int(mpf_cmp(l, r) < 0);
        else if (strcmp(op, ">=") == 0) result = make_int(mpf_cmp(l, r) >= 0);
        else if (strcmp(op, "<=") == 0) result = make_int(mpf_cmp(l, r) <= 0);
        else if (strcmp(op, "==") == 0) result = make_int(mpf_cmp(l, r) == 0);
        else if (strcmp(op, "!=") == 0) result = make_int(mpf_cmp(l, r) != 0);
        else result = make_int(0);
        mpf_clear(l); mpf_clear(r);
        return result;
    }
    // BigInt arithmetic
    if (left.type == TYPE_BIGINT || right.type == TYPE_BIGINT) {
        mpz_t l, r;
        mpz_init(l); mpz_init(r);
        if (left.type == TYPE_BIGINT) mpz_set(l, left.bigIntVal);
        else mpz_set_si(l, left.intVal);
        if (right.type == TYPE_BIGINT) mpz_set(r, right.bigIntVal);
        else mpz_set_si(r, right.intVal);
        Value result;
        if (strcmp(op, "+") == 0) { result = make_bigint_from_long(0); mpz_add(result.bigIntVal, l, r); }
        else if (strcmp(op, "-") == 0) { result = make_bigint_from_long(0); mpz_sub(result.bigIntVal, l, r); }
        else if (strcmp(op, "*") == 0) { result = make_bigint_from_long(0); mpz_mul(result.bigIntVal, l, r); }
        else if (strcmp(op, "/") == 0) { result = make_bigint_from_long(0); mpz_tdiv_q(result.bigIntVal, l, r); }
        else if (strcmp(op, "%") == 0) { result = make_bigint_from_long(0); mpz_tdiv_r(result.bigIntVal, l, r); }
        else if (strcmp(op, ">") == 0) result = make_int(mpz_cmp(l, r) > 0);
        else if (strcmp(op, "<") == 0) result = make_int(mpz_cmp(l, r) < 0);
        else if (strcmp(op, ">=") == 0) result = make_int(mpz_cmp(l, r) >= 0);
        else if (strcmp(op, "<=") == 0) result = make_int(mpz_cmp(l, r) <= 0);
        else if (strcmp(op, "==") == 0) result = make_int(mpz_cmp(l, r) == 0);
        else if (strcmp(op, "!=") == 0) result = make_int(mpz_cmp(l, r) != 0);
        else result = make_int(0);
        mpz_clear(l); mpz_clear(r);
        return result;
    }
    // Int arithmetic
    if (strcmp(op, "+") == 0 && left.type == TYPE_INT && right.type == TYPE_INT) {
        return make_int(left.intVal + right.intVal);
    }
    if (strcmp(op, "-") == 0 && left.type == TYPE_INT && right.type == TYPE_INT) {
        return make_int(left.intVal - right.intVal);
    }
    if (strcmp(op, "*") == 0 && left.type == TYPE_INT && right.type == TYPE_INT) {
        return make_int(left.intVal * right.intVal);
    }
    if (strcmp(op, "/") == 0 && left.type == TYPE_INT && right.type == TYPE_INT) {
        return make_int(right.intVal != 0 ? left.intVal / right.intVal : 0);
    }
    if (strcmp(op, "%") == 0 && left.type == TYPE_INT && right.type == TYPE_INT) {
        return make_int(right.intVal != 0 ? left.intVal % right.intVal : 0);
    }
    if (strcmp(op, ">") == 0 && left.type == TYPE_INT && right.type == TYPE_INT) {
        return make_int(left.intVal > right.intVal);
    }
    if (strcmp(op, "<") == 0 && left.type == TYPE_INT && right.type == TYPE_INT) {
        return make_int(left.intVal < right.intVal);
    }
    if (strcmp(op, ">=") == 0 && left.type == TYPE_INT && right.type == TYPE_INT) {
        return make_int(left.intVal >= right.intVal);
    }
    if (strcmp(op, "<=") == 0 && left.type == TYPE_INT && right.type == TYPE_INT) {
        return make_int(left.intVal <= right.intVal);
    }
    if (strcmp(op, "!=") == 0) {
        return make_int(!values_equal(left, right));
    }
    if (strcmp(op, "==") == 0) {
        return make_int(values_equal(left, right));
    }
    return make_int(0);
}

int is_truthy(Value v) {
    if (v.type == TYPE_INT) return v.intVal != 0;
    if (v.type == TYPE_BOOL) return v.intVal != 0;
    if (v.type == TYPE_FLOAT64) return v.floatVal != 0.0;
    if (v.type == TYPE_STRING) return strlen(v.strVal) > 0;
    if (v.type == TYPE_ARRAY) return v.arrVal.length > 0;
    if (v.type == TYPE_MAP) return v.mapVal.length > 0;
    if (v.type == TYPE_BIGINT) return mpz_cmp_si(v.bigIntVal, 0) != 0;
    if (v.type == TYPE_BIGFLOAT) return mpf_cmp_d(v.bigFloatVal, 0.0) != 0;
    if (v.type == TYPE_OPTION) return v.optVal.tag != 0;
    if (v.type == TYPE_RESULT) return v.resVal.tag == 0;
    return 0;
}

// KTF-001: Native assertion foundation. These runtime functions back the
// language-level assert / assert_eq / assert_ne builtins. On failure they emit
// structured diagnostics to stderr and terminate with a non-zero exit status so
// a test runner can detect a genuine failure (rather than silently continuing).
void karkain_assert_report(const char* kind, const char* expr, const char* loc) {
    fprintf(stderr, "%s: %s", kind, expr);
    if (loc != NULL && loc[0] != '\0') fprintf(stderr, "  [%s]", loc);
    fprintf(stderr, "\n");
    fflush(stderr);
}

void karkain_assert(Value cond, const char* expr, const char* loc) {
    if (!is_truthy(cond)) {
        karkain_assert_report("assertion failed", expr, loc);
        exit(1);
    }
}

void karkain_assert_eq(Value actual, Value expected, const char* expr, const char* loc) {
    if (!values_equal(actual, expected)) {
        karkain_assert_report("assertion failed: assert_eq", expr, loc);
        Value _a = karkain_str(actual);
        Value _e = karkain_str(expected);
        fprintf(stderr, "  expected: %s\n  actual:   %s\n", _e.strVal ? _e.strVal : "", _a.strVal ? _a.strVal : "");
        fflush(stderr);
        exit(1);
    }
}

void karkain_assert_ne(Value actual, Value expected, const char* expr, const char* loc) {
    if (values_equal(actual, expected)) {
        karkain_assert_report("assertion failed: assert_ne", expr, loc);
        Value _a = karkain_str(actual);
        fprintf(stderr, "  unexpectedly equal (actual): %s\n", _a.strVal ? _a.strVal : "");
        fflush(stderr);
        exit(1);
    }
}

// Phase 11: Raw pointer operations
void* karkain_alloc(size_t count, size_t size) {
    return malloc(count * size);
}

void karkain_free(void* ptr) {
    free(ptr);
}

// Phase 10: String standard library functions
Value karkain_trim(Value str) {
    if (str.type != TYPE_STRING) return make_string("");
    char* s = str.strVal;
    char* start = s;
    char* end = s + strlen(s) - 1;
    while (start <= end && (*start == ' ' || *start == '\t' || *start == '\n' || *start == '\r')) start++;
    while (end >= start && (*end == ' ' || *end == '\t' || *end == '\n' || *end == '\r')) end--;
    size_t len = (end >= start) ? (end - start + 1) : 0;
    char* buf = (char*)malloc(len + 1);
    if (len > 0) memcpy(buf, start, len);
    buf[len] = '\0';
    Value res = make_string(buf);
    free(buf);
    return res;
}

Value karkain_contains(Value haystack, Value needle) {
    if (haystack.type != TYPE_STRING || needle.type != TYPE_STRING) return make_int(0);
    return make_int(strstr(haystack.strVal, needle.strVal) != NULL);
}

Value karkain_split(Value str, Value delim) {
    if (str.type != TYPE_STRING || delim.type != TYPE_STRING) return make_array();
    Value arr = make_array();
    char* s = strdup(str.strVal);
    char* d = delim.strVal;
    char* token = strtok(s, d);
    while (token != NULL) {
        array_push(&arr, make_string(token));
        token = strtok(NULL, d);
    }
    free(s);
    return arr;
}

// Phase 10: Math standard library functions
Value karkain_sqrt(Value v) {
    if (v.type != TYPE_INT) return make_int(0);
    return make_int((long long)sqrt((double)v.intVal));
}

Value karkain_abs(Value v) {
    if (v.type != TYPE_INT) return make_int(0);
    return make_int(v.intVal >= 0 ? v.intVal : -v.intVal);
}

Value karkain_pow(Value base, Value exp) {
    if (base.type != TYPE_INT || exp.type != TYPE_INT) return make_int(0);
    long long result = 1;
    long long b = base.intVal;
    long long e = exp.intVal;
    while (e > 0) {
        if (e & 1) result *= b;
        b *= b;
        e >>= 1;
    }
    return make_int(result);
}

// Phase 10: Array append function â€” mutates arr and returns it
Value karkain_appendArray(Value* arr, Value elem) {
    if (!arr || arr->type != TYPE_ARRAY) return mk_nil();
    array_push(arr, elem);
    return *arr;
}

// Phase 52: Boolean â€” compound literal, never malloc, no shared state
Value make_bool(int v) {
    return v ? (Value){ .type = TYPE_BOOL, .intVal = 1 } : (Value){ .type = TYPE_BOOL, .intVal = 0 };
}

// Phase 88: readLineEOF — reads a line and returns [line, isEOF] array.
// Used by the self-hosted compiler's file reading loop.
Value readLineEOF(Value handle) {
    Value r = make_array();
    if (handle.type != TYPE_STRING || strlen(handle.strVal) == 0) {
        array_push(&r, make_string("")); array_push(&r, make_bool(1)); return r;
    }
    int idx = -1;
    int i;
    for (i = 0; i < _karkain_file_count; i++) {
        if (_karkain_files[i] != NULL) { idx = i; break; }
    }
    if (idx == -1) {
        if (_karkain_file_count >= 256) { array_push(&r, make_string("")); array_push(&r, make_bool(1)); return r; }
        idx = _karkain_file_count;
        _karkain_files[idx] = fopen(handle.strVal, "rb");
        _karkain_file_count++;
    } else if (_karkain_files[idx] == NULL) {
        _karkain_files[idx] = fopen(handle.strVal, "rb");
    }
    if (!_karkain_files[idx]) { array_push(&r, make_string("")); array_push(&r, make_bool(1)); return r; }
    char buf[4096];
    if (fgets(buf, sizeof(buf), _karkain_files[idx]) == NULL) {
        array_push(&r, make_string("")); array_push(&r, make_bool(1)); return r;
    }
    int len = (int)strlen(buf);
    while (len > 0 && (buf[len - 1] == '\n' || buf[len - 1] == '\r')) buf[--len] = '\0';
    array_push(&r, make_string(buf));
    array_push(&r, make_bool(0));
    return r;
}

// listFiles returns the names of entries in a directory as a Value array.
// Mirrors Go's os.ReadDir for the self-hosted compiler's sibling gathering.
Value karkain_listFiles(Value dir) {
    Value arr = make_array();
    const char* dname = dir.type == TYPE_STRING ? (dir.strVal ? dir.strVal : "") : "";
    DIR* d = opendir(dname);
    if (!d) return arr;
    struct dirent* e;
    while ((e = readdir(d)) != NULL) {
        if (e->d_name[0] == '.') continue; // skip "." and ".."
        array_push(&arr, make_string(e->d_name));
    }
    closedir(d);
    return arr;
}

// Phase 19: Map hasKey and delete
Value karkain_hasKey(Value m, Value k) {
    if (m.type != TYPE_MAP) return make_int(0);
    for (int i = 0; i < m.mapVal.length; i++) {
        if (values_equal(*m.mapVal.keys[i], k)) {
            return make_int(1);
        }
    }
    return make_int(0);
}

void karkain_delete(Value* m, Value k) {
    if (!m || m->type != TYPE_MAP) return;
    for (int i = 0; i < m->mapVal.length; i++) {
        if (values_equal(*m->mapVal.keys[i], k)) {
            for (int j = i; j < m->mapVal.length - 1; j++) {
                m->mapVal.keys[j] = m->mapVal.keys[j + 1];
                m->mapVal.values[j] = m->mapVal.values[j + 1];
            }
            m->mapVal.length--;
            return;
        }
    }
}

// Phase 70: atomic compare-and-swap helper.
//   int karkain_atomic_cas(_Atomic int* p, int expected, int desired, memory_order o)
//   returns 1 if the exchange occurred, else 0 (and no swap).
int karkain_atomic_cas(volatile _Atomic(int)* p, int expected, int desired, memory_order o) {
    return atomic_compare_exchange_strong_explicit(p, &expected, desired, o, memory_order_relaxed);
}

// Phase 19: Negate unary operator
Value karkain_negate(Value v) {
    if (v.type == TYPE_INT) return make_int(-v.intVal);
    if (v.type == TYPE_FLOAT64) return make_float(-v.floatVal);
    return make_int(0);
}

// Phase 19: Logical NOT operator
Value karkain_not(Value v) {
    return make_int(is_truthy(v) ? 0 : 1);
}

// Phase 55b: Checked arithmetic â€” returns option_some(result) on success, option_none() on overflow
Value karkain_add_checked(Value a, Value b) {
    if (a.type == TYPE_INT && b.type == TYPE_INT) {
        long long r;
        if (__builtin_add_overflow(a.intVal, b.intVal, &r)) return option_none();
        return option_some(make_int(r));
    }
    return option_some(binary_op(a, "+", b));
}
Value karkain_sub_checked(Value a, Value b) {
    if (a.type == TYPE_INT && b.type == TYPE_INT) {
        long long r;
        if (__builtin_sub_overflow(a.intVal, b.intVal, &r)) return option_none();
        return option_some(make_int(r));
    }
    return option_some(binary_op(a, "-", b));
}
Value karkain_mul_checked(Value a, Value b) {
    if (a.type == TYPE_INT && b.type == TYPE_INT) {
        long long r;
        if (__builtin_mul_overflow(a.intVal, b.intVal, &r)) return option_none();
        return option_some(make_int(r));
    }
    return option_some(binary_op(a, "*", b));
}
`
}

// Phase 55b: pre-scan AST for http.get calls to conditionally include HTTP runtime
func (g *Generator) scanNodeForHTTP(node parser.Node) {
	if node == nil || g.needsHTTP {
		return
	}
	switch n := node.(type) {
	case *parser.CallExpr:
		if n.Function == "http.get" {
			g.needsHTTP = true
			return
		}
		for _, a := range n.Args {
			g.scanNodeForHTTP(a)
		}
	case *parser.FuncDecl:
		for _, s := range n.Body {
			g.scanNodeForHTTP(s)
		}
	case *parser.IfStmt:
		g.scanNodeForHTTP(n.Condition)
		for _, s := range n.Consequence {
			g.scanNodeForHTTP(s)
		}
		for _, s := range n.Alternative {
			g.scanNodeForHTTP(s)
		}
	case *parser.WhileStmt:
		g.scanNodeForHTTP(n.Condition)
		for _, s := range n.Body {
			g.scanNodeForHTTP(s)
		}
	case *parser.ForStmt:
		g.scanNodeForHTTP(n.Init)
		g.scanNodeForHTTP(n.Condition)
		g.scanNodeForHTTP(n.Post)
		for _, s := range n.Body {
			g.scanNodeForHTTP(s)
		}
	case *parser.ForInStmt:
		g.scanNodeForHTTP(n.Iter)
		for _, s := range n.Body {
			g.scanNodeForHTTP(s)
		}
	case *parser.VarDeclStmt:
		g.scanNodeForHTTP(n.Value)
	case *parser.ReturnStmt:
		g.scanNodeForHTTP(n.Value)
	case *parser.ExprStmt:
		g.scanNodeForHTTP(n.Expression)
	case *parser.PrintStmt:
		g.scanNodeForHTTP(n.Value)
	case *parser.BinaryExpr:
		g.scanNodeForHTTP(n.Left)
		g.scanNodeForHTTP(n.Right)
	case *parser.UnaryExpr:
		g.scanNodeForHTTP(n.Operand)
	case *parser.LambdaExpr:
		for _, s := range n.Body {
			g.scanNodeForHTTP(s)
		}
	case *parser.MatchExpr:
		g.scanNodeForHTTP(n.Value)
		for _, arm := range n.Arms {
			g.scanNodeForHTTP(arm.Body)
		}
	case *parser.IndexExpr:
		g.scanNodeForHTTP(n.Left)
		g.scanNodeForHTTP(n.Index)
	case *parser.SliceExpr:
		g.scanNodeForHTTP(n.Target)
		g.scanNodeForHTTP(n.Start)
		g.scanNodeForHTTP(n.End)
	case *parser.DotExpr:
		g.scanNodeForHTTP(n.Left)
	case *parser.ArrayLiteral:
		for _, e := range n.Elements {
			g.scanNodeForHTTP(e)
		}
	case *parser.MapLiteral:
		for _, k := range n.Keys {
			g.scanNodeForHTTP(k)
		}
		for _, v := range n.Values {
			g.scanNodeForHTTP(v)
		}
	case *parser.StructLiteral:
		for _, v := range n.Fields {
			g.scanNodeForHTTP(v)
		}
	}
}

func (g *Generator) genFuncDecl(fn *parser.FuncDecl) string {
	params := []string{}
	for _, p := range fn.Params {
		params = append(params, "Value "+p)
	}

	// Phase 54: closure conversion â€” capturing lambdas take an env struct of
	// pointers to the captured variables (mutable, shared with enclosing scope).
	closureDefs, closureUndefs := "", ""
	if len(fn.Captures) > 0 {
		envT := "ClosureEnv_" + sanitizeC(fn.Name)
		g.closureVars[fn.Name] = true
		var fields strings.Builder
		for _, cap := range fn.Captures {
			fmt.Fprintf(&fields, "\tValue* %s;\n", cap)
			closureDefs += fmt.Sprintf("#define %s (*_env->%s)\n", cap, cap)
			closureUndefs += fmt.Sprintf("#undef %s\n", cap)
		}
		holder := "_genv_" + sanitizeC(fn.Name)
		inits := ""
		for _, cap := range fn.Captures {
			inits += fmt.Sprintf("_e.%s = &%s; ", cap, cap)
		}
		g.lambdaBuf.WriteString(fmt.Sprintf(
			"typedef struct {\n%s} %s;\nstatic %s* %s;\n\n", fields.String(), envT, envT, holder))
		params = append([]string{envT + "* _env"}, params...)
		g.lastClosureInit = fmt.Sprintf("\t{ static %s _e; %s%s = &_e; }\n", envT, inits, holder)
	} else {
		g.lastClosureInit = ""
	}

	retType := "Value"
	fnName := userFuncC(fn.Name)
	if fnName == "main" {
		retType = "int"
	}

	// Karkain argument API: getArgs() is an intrinsic whose body is supplied by
	// the codegen, returning the argc/argv captured by the generated C entry
	// point. Both the Go bootstrap backend and the self-hosted emitC11 backend
	// must emit this same canonical implementation for parity. The function is
	// still declared in Karkain source (so the parser/self-hosted driver can
	// dispatch on it), but the declared body is replaced here at emission.
	if fnName == "getArgs" {
		var gb strings.Builder
		gb.WriteString("Value getArgs(void) {\n")
		gb.WriteString("\tValue _args = make_array();\n")
		gb.WriteString("\tint _i;\n")
		gb.WriteString("\tfor (_i = 0; _i < _karkain_gargc; _i++) {\n")
		gb.WriteString("\t\tarray_push(&_args, make_string(_karkain_gargv[_i]));\n")
		gb.WriteString("\t}\n")
		gb.WriteString("\treturn _args;\n")
		gb.WriteString("}\n\n")
		return gb.String()
	}

	// Phase 48: Generate body first to collect any lambda definitions
	g.lambdaBuf.Reset()
	var bodySb strings.Builder
	for _, stmt := range fn.Body {
		bodySb.WriteString(g.genStatement(stmt))
	}

	// Now build the full output: lambda defs + function
	var sb strings.Builder
	if g.lambdaBuf.Len() > 0 {
		sb.WriteString(g.lambdaBuf.String())
		g.lambdaBuf.Reset()
	}
	// Generated C entry point receives real argv so Karkain's getArgs() can
	// expose the actual process arguments (not a placeholder).
	sig := strings.Join(params, ", ")
	if fnName == "main" && sig == "" {
		sig = "int _karkain_argc, char** _karkain_argv"
	}
	fmt.Fprintf(&sb, "%s %s(%s) {\n", retType, fnName, sig)

	// Phase 14: Initialize quantum runtime in main
	if fnName == "main" {
		sb.WriteString("\t_karkain_gargc = _karkain_argc; _karkain_gargv = _karkain_argv;\n")
		sb.WriteString("\tquantum_init();\n")
	}

	sb.WriteString(closureDefs)
	sb.WriteString(bodySb.String())
	sb.WriteString(closureUndefs)

	if fnName == "main" {
		sb.WriteString("\treturn 0;\n")
	} else {
		sb.WriteString("\treturn make_int(0);\n")
	}
	sb.WriteString("}\n\n")
	return sb.String()
}

// Phase 42: match expression codegen
func (g *Generator) genMatchExpr(node *parser.MatchExpr) string {
	valueExpr := g.genExpr(node.Value)
	var sb strings.Builder
	sb.WriteString("({ ")
	sb.WriteString(fmt.Sprintf("Value _match_val = %s; ", valueExpr))
	sb.WriteString("Value _match_result = _match_val; ")

	for i, arm := range node.Arms {
		cond := ""
		binding := ""
		switch arm.Pattern.Type {
		case "Some":
			if arm.Pattern.Binding != "" {
				binding = arm.Pattern.Binding
			}
			cond = "(_match_val.type == TYPE_OPTION && _match_val.optVal.tag == 1)"
		case "None":
			cond = "(_match_val.type == TYPE_OPTION && _match_val.optVal.tag == 0)"
		case "Ok":
			if arm.Pattern.Binding != "" {
				binding = arm.Pattern.Binding
			}
			cond = "(_match_val.type == TYPE_RESULT && _match_val.resVal.tag == 0)"
		case "Err":
			if arm.Pattern.Binding != "" {
				binding = arm.Pattern.Binding
			}
			cond = "(_match_val.type == TYPE_RESULT && _match_val.resVal.tag == 1)"
		case "literal":
			litExpr := g.mapLiteralToC(arm.Pattern.Value)
			cond = fmt.Sprintf("is_truthy(binary_op(_match_val, \"==\", %s))", litExpr)
		case "wildcard":
			cond = "1"
		case "binding":
			cond = "1"
			binding = arm.Pattern.Binding
		case "enum_variant":
			parts := strings.SplitN(arm.Pattern.Binding, ".", 2)
			if len(parts) == 2 {
				enumName, variantName := parts[0], parts[1]
				tagIdx := g.getEnumVariantIndex(enumName, variantName)
				cond = fmt.Sprintf("is_truthy(binary_op(_match_val, \"==\", make_int(%d)))", tagIdx)
			} else {
				cond = "1"
			}
		}

		prefix := " "
		if i > 0 {
			prefix = " else "
		}

		var bodyBlock strings.Builder
		bodyBlock.WriteString("{ ")

		if binding != "" {
			if arm.Pattern.Type == "Some" {
				bodyBlock.WriteString(fmt.Sprintf("Value %s = *_match_val.optVal.inner; ", binding))
			} else if arm.Pattern.Type == "Ok" {
				bodyBlock.WriteString(fmt.Sprintf("Value %s = *_match_val.resVal.okVal; ", binding))
			} else if arm.Pattern.Type == "Err" {
				bodyBlock.WriteString(fmt.Sprintf("Value %s = *_match_val.resVal.errVal; ", binding))
			} else {
				bodyBlock.WriteString(fmt.Sprintf("Value %s = _match_val; ", binding))
			}
		}

		g.genArmBody(arm.Body, &bodyBlock)

		bodyBlock.WriteString("}")
		sb.WriteString(fmt.Sprintf("%sif (%s) %s", prefix, cond, bodyBlock.String()))
	}
	sb.WriteString(" _match_result; })")
	return sb.String()
}

func (g *Generator) genArmBody(body parser.Node, sb *strings.Builder) {
	switch b := body.(type) {
	case *parser.ExprStmt:
		sb.WriteString(fmt.Sprintf("_match_result = %s; ", g.genExpr(b.Expression)))
	case *parser.BlockStmt:
		for j, stmt := range b.Statements {
			g.genStatementTo(sb, stmt)
			if j == len(b.Statements)-1 {
				if exprStmt, ok := stmt.(*parser.ExprStmt); ok {
					sb.WriteString(fmt.Sprintf("_match_result = %s; ", g.genExpr(exprStmt.Expression)))
				}
			}
		}
	case *parser.PrintStmt:
		sb.WriteString(fmt.Sprintf("print_value(%s); ", g.genExpr(b.Value)))
	default:
		sb.WriteString(fmt.Sprintf("_match_result = %s; ", g.genExpr(body)))
	}
}

func (g *Generator) genStatementTo(sb *strings.Builder, stmt parser.Node) {
	if g.cfg.Debug {
		if line := parser.GetLine(stmt); line > 0 {
			fmt.Fprintf(sb, "#line %d \"%s\"\n", line, g.sourceFile)
		}
	}
	switch node := stmt.(type) {
	case *parser.VarDeclStmt:
		if node.IsMatrix {
			sb.WriteString(g.genMatrixDecl(node))
		} else {
			sb.WriteString(fmt.Sprintf("Value %s = %s; ", node.Name, g.genExpr(node.Value)))
		}
	case *parser.PrintStmt:
		sb.WriteString(fmt.Sprintf("print_value(%s); ", g.genExpr(node.Value)))
	case *parser.ExprStmt:
		sb.WriteString(fmt.Sprintf("%s; ", g.genExpr(node.Expression)))
	case *parser.ReturnStmt:
		sb.WriteString(fmt.Sprintf("return %s; ", g.genExpr(node.Value)))
	default:
		sb.WriteString(g.genStatement(stmt))
	}
}

// Phase 42: SIMD intrinsic codegen
func (g *Generator) genSIMDExpr(node *parser.SIMDBuiltinExpr) string {
	var left, right string
	if len(node.Args) >= 1 {
		left = g.genExpr(node.Args[0])
	}
	if len(node.Args) >= 2 {
		right = g.genExpr(node.Args[1])
	}

	// Phase 70: lane-vector construction / load intrinsics operate on real C
	// SIMD types (not the scalar Value union).
	switch node.Op {
	case "splat":
		// @simd_splat(v, lanes) → broadcast scalar into a lane vector.
		arg := left
		if arg == "" {
			return "make_int(0)"
		}
		if KarkainSIMDSSE() {
			return fmt.Sprintf("_mm_set1_ps((float)(%s))", arg)
		}
		return fmt.Sprintf("(float4){(%s),(%s),(%s),(%s)}", arg, arg, arg, arg)
	case "load":
		// @simd_load(arr, offset) → load 4 consecutive f32 from a buffer.
		if left == "" || right == "" {
			return "make_int(0)"
		}
		if KarkainSIMDSSE() {
			return fmt.Sprintf("_mm_loadu_ps((const float*)&(%s)[%s])", left, right)
		}
		return fmt.Sprintf("(*(float4*)&(%s)[%s])", left, right)
	case "store":
		// @simd_store(arr, offset, v) → store a lane vector back to a buffer.
		third := ""
		if len(node.Args) >= 3 {
			third = g.genExpr(node.Args[2])
		}
		return fmt.Sprintf("(*((float4*)&(%s)[%s]) = (%s))", left, right, third)
	}

	// Scalar fallback for the legacy @simd_add/mul/sub/div ops.
	if len(node.Args) < 2 {
		return "make_int(0)"
	}
	switch node.Op {
	case "add":
		return fmt.Sprintf("binary_op(%s, \"+\", %s)", left, right)
	case "mul":
		return fmt.Sprintf("binary_op(%s, \"*\", %s)", left, right)
	case "sub":
		return fmt.Sprintf("binary_op(%s, \"-\", %s)", left, right)
	case "div":
		return fmt.Sprintf("binary_op(%s, \"/\", %s)", left, right)
	default:
		return fmt.Sprintf("binary_op(%s, \"%s\", %s)", left, node.Op, right)
	}
}

// KarkainSIMDSSE reports whether the generated C can use _mm_* intrinsics.
func KarkainSIMDSSE() bool { return true }

// simdVectorCType maps a "[N]f32"-style lane-vector type to a C type.
//   [4]f32 -> __m128 (SSE)
//   [8]f32 -> __m256 (AVX)
//   [2]f64 -> __m128d
//   [4]f64 -> __m256d
// Unknown combos fall back to an aligned float array so code still compiles.
func simdVectorCType(typeStr string) string {
	lanes, elem, ok := parser.ParseSIMDVectorType(typeStr)
	if !ok || lanes <= 0 {
		return "void*"
	}
	switch elem {
	case "f32", "float32":
		switch lanes {
		case 2:
			return "__m64"
		case 4:
			return "__m128"
		case 8:
			return "__m256"
		}
		return fmt.Sprintf("float[%d]", lanes)
	case "f64", "float64":
		switch lanes {
		case 2:
			return "__m128d"
		case 4:
			return "__m256d"
		}
		return fmt.Sprintf("double[%d]", lanes)
	case "i32", "int32":
		switch lanes {
		case 4:
			return "__m128i"
		case 8:
			return "__m256i"
		}
		return fmt.Sprintf("int32_t[%d]", lanes)
	case "i64", "int64":
		switch lanes {
		case 2:
			return "__m128i"
		case 4:
			return "__m256i"
		}
		return fmt.Sprintf("int64_t[%d]", lanes)
	}
	return fmt.Sprintf("%s[%d]", elem, lanes)
}

// memoryOrderC maps a Karkain ordering name to a C memory_order_* value.
func memoryOrderC(order string) string {
	switch order {
	case "relaxed":
		return "memory_order_relaxed"
	case "acquire":
		return "memory_order_acquire"
	case "release":
		return "memory_order_release"
	case "acq_rel":
		return "memory_order_acq_rel"
	default:
		return "memory_order_seq_cst"
	}
}

// genAtomicExpr emits C11 stdatomic operations with explicit memory orderings.
// Surface (ordering defaults to seq_cst when the trailing string is omitted):
//   @atomic_load(ptr, [order])        -> atomic_load(ptr, order)
//   @atomic_store(ptr, v, [order])    -> atomic_store(ptr, v, order)
//   @atomic_fetch_add(ptr, v, [order])-> atomic_fetch_add(ptr, v, order)
//   @atomic_fetch_sub(ptr, v, [order])-> atomic_fetch_sub(ptr, v, order)
//   @atomic_cas(ptr, cmp, v, [order]) -> atomic_compare_exchange...
func (g *Generator) genAtomicExpr(node *parser.AtomicOp) string {
	order := memoryOrderC(node.Order)
	n := len(node.Args)
	gen := func(i int) string {
		if i < n {
			return g.genExpr(node.Args[i])
		}
		return "0"
	}
	ptr := gen(0)
	switch node.Op {
	case "load":
		return fmt.Sprintf("atomic_load_explicit((volatile _Atomic(int)*(%s)), %s)", ptr, order)
	case "store":
		return fmt.Sprintf("atomic_store_explicit((volatile _Atomic(int)*(%s)), (int)(%s), %s)", ptr, gen(1), order)
	case "fetch_add":
		return fmt.Sprintf("atomic_fetch_add_explicit((volatile _Atomic(int)*(%s)), (int)(%s), %s)", ptr, gen(1), order)
	case "fetch_sub":
		return fmt.Sprintf("atomic_fetch_sub_explicit((volatile _Atomic(int)*(%s)), (int)(%s), %s)", ptr, gen(1), order)
	case "exchange":
		return fmt.Sprintf("atomic_exchange_explicit((volatile _Atomic(int)*(%s)), (int)(%s), %s)", ptr, gen(1), order)
	case "cas":
		// cmp is a Value holding a pointer to expected; simplify to a known
		// comparison helper used by the runtime.
		return fmt.Sprintf("karkain_atomic_cas((volatile _Atomic(int)*(%s)), (int)(%s), (int)(%s), %s)", ptr, gen(1), gen(2), order)
	default:
		return "0"
	}
}

// Phase 45: Get the integer tag index for an enum variant
func (g *Generator) getEnumVariantIndex(enumName, variantName string) int {
	if enumDecl, ok := g.enumDecls[enumName]; ok {
		for i, v := range enumDecl.Variants {
			if v.Name == variantName {
				return i + 1 // tags start at 1
			}
		}
	}
	return 0
}

func (g *Generator) genStatement(stmt parser.Node) string {
	return g.genStatementInner(stmt)
}

func (g *Generator) genStatementInner(stmt parser.Node) (result string) {
	if g.cfg.Debug {
		defer func() {
			if line := parser.GetLine(stmt); line > 0 {
				result = fmt.Sprintf("#line %d \"%s\"\n%s", line, g.sourceFile, result)
			}
		}()
	}
	switch node := stmt.(type) {
	case *parser.VarDeclStmt:
		if node.IsMatrix {
			return g.genMatrixDecl(node)
		}
		// Phase 48/54: let x = fn(...) â†’ emit as named function declaration
		if fn, ok := node.Value.(*parser.FuncDecl); ok {
			out := g.genFuncDecl(fn)
			return out + g.lastClosureInit // Phase 54: env instance init after closure def
		}
		// Phase 70: `let x [N]f32 = ...` â€” fixed-lane SIMD vector variable.
		if node.IsSIMD {
			cType := simdVectorCType(node.Type)
			alignAttr := ""
			if node.Align > 0 {
				alignAttr = fmt.Sprintf(" _Alignas(%d)", node.Align)
			}
			return fmt.Sprintf("\t%s%s %s = %s;\n", cType, alignAttr, node.Name, g.genExpr(node.Value))
		}
		// Phase 70: cache-line alignment attribute on a plain scalar/Value var.
		if node.Align > 0 {
			return fmt.Sprintf("\t_Alignas(%d) Value %s = %s;\n", node.Align, node.Name, g.genExpr(node.Value))
		}
		// Check if this is a typed declaration
		if node.Type != "" {
			// Phase 52: All declarations use Value (by value, stack-allocated)
			return fmt.Sprintf("\tValue %s = %s;\n", node.Name, g.genExpr(node.Value))
		}
		// For dynamic types, use Value wrapper
		return fmt.Sprintf("\tValue %s = %s;\n", node.Name, g.genExpr(node.Value))
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
		var res strings.Builder
		fmt.Fprintf(&res, "\tif (is_truthy(%s)) {\n", g.genExpr(node.Condition))
		for _, cStmt := range node.Consequence {
			res.WriteString("\t")
			res.WriteString(g.genStatement(cStmt))
		}
		res.WriteString("\t}")
		if len(node.Alternative) > 0 {
			if len(node.Alternative) == 1 {
				if elseIf, ok := node.Alternative[0].(*parser.IfStmt); ok {
					elseIfStr := g.genStatement(elseIf)
					res.WriteString(" else ")
					res.WriteString(strings.TrimPrefix(elseIfStr, "\t"))
				} else {
					res.WriteString(" else {\n")
					for _, aStmt := range node.Alternative {
						res.WriteString("\t")
						res.WriteString(g.genStatement(aStmt))
					}
					res.WriteString("\t}")
				}
			} else {
				res.WriteString(" else {\n")
				for _, aStmt := range node.Alternative {
					res.WriteString("\t")
					res.WriteString(g.genStatement(aStmt))
				}
				res.WriteString("\t}")
			}
		}
		res.WriteString("\n")
		return res.String()
	case *parser.WhileStmt:
		var res strings.Builder
		fmt.Fprintf(&res, "\twhile (is_truthy(%s)) {\n", g.genExpr(node.Condition))
		for _, bodyStmt := range node.Body {
			res.WriteString("\t")
			res.WriteString(g.genStatement(bodyStmt))
		}
		res.WriteString("\t}\n")
		return res.String()
	case *parser.AllocExpr:
		countExpr := g.mapLiteralToC(node.Count)
		cType := g.mapKarkainTypeToC(node.Type)
		return fmt.Sprintf("(%s*)malloc(%s * sizeof(%s))", cType, countExpr, cType)
	case *parser.FreeExpr:
		return fmt.Sprintf("free(%s)", g.genExpr(node.Ptr))
	case *parser.AddressOf:
		return fmt.Sprintf("&(%s)", g.genExpr(node.Operand))
	case *parser.QRegDeclStmt:
		// Phase 14: Generate quantum register declaration; track it for gate/measure resolution
		g.quantumRegisters = append(g.quantumRegisters, node.Name)
		qubitsExpr := g.mapLiteralToC(node.Qubits)
		return fmt.Sprintf("\tQuantumRegister %s;\n\tqreg_init(&%s, %s);\n", node.Name, node.Name, qubitsExpr)
	case *parser.GateApplyStmt:
		// Phase 14: Generate gate application calls
		targetReg, targetIdx := g.resolveQubitOperand(node.Target)

		switch node.Gate {
		case "H":
			return fmt.Sprintf("\tgate_h(&%s, %s);\n", targetReg, targetIdx)
		case "X":
			return fmt.Sprintf("\tgate_x(&%s, %s);\n", targetReg, targetIdx)
		case "Y":
			return fmt.Sprintf("\tgate_y(&%s, %s);\n", targetReg, targetIdx)
		case "Z":
			return fmt.Sprintf("\tgate_z(&%s, %s);\n", targetReg, targetIdx)
		case "CNOT":
			if node.Control != nil {
				controlReg, controlIdx := g.resolveQubitOperand(node.Control)
				if controlReg == targetReg {
					return fmt.Sprintf("\tgate_cnot(&%s, %s, %s);\n", targetReg, controlIdx, targetIdx)
				}
				return fmt.Sprintf("\t// Cross-register CNOT (%s[%s] -> %s[%s]) not yet supported\n", controlReg, controlIdx, targetReg, targetIdx)
			}
			return fmt.Sprintf("\tgate_cnot(&%s, %s, %s);\n", targetReg, "0", targetIdx)
		case "CZ":
			controlReg, controlIdx := g.resolveQubitOperand(node.Control)
			return fmt.Sprintf("\t// CZ gate (%s[%s], %s[%s]) not yet supported\n", controlReg, controlIdx, targetReg, targetIdx)
		default:
			return fmt.Sprintf("\t// Unknown gate: %s\n", node.Gate)
		}
	case *parser.MeasureExpr:
		// Phase 14: Generate measure call in statement context
		regName, idx := g.resolveQubitOperand(node.Qubit)
		return fmt.Sprintf("\tmeasure(&%s, %s);\n", regName, idx)
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
	case *parser.EnumDecl:
		g.enumDecls[node.Name] = node
		return g.genEnumDecl(node)
	case *parser.ForStmt:
		return g.genForStmt(node)
	case *parser.ForInStmt:
		return g.genForInStmt(node)
	case *parser.BreakStmt:
		return "\tbreak;\n"
	case *parser.ContinueStmt:
		return "\tcontinue;\n"
	}
	return ""
}

// genAssertCall lowers a KTF-001 assertion builtin call (assert/assert_eq/
// assert_ne) to its C runtime check. A trailing string literal argument is
// treated as a user message; otherwise the call expression text is used as the
// diagnostic label. The optional message/expression is passed as the `loc` C
// argument, which the runtime prints alongside the failure.
func (g *Generator) genAssertCall(n *parser.CallExpr) string {
	label := assertLabel(n.Function)
	minArgs := 1
	if n.Function == "assert_eq" || n.Function == "assert_ne" {
		minArgs = 2
	}
	if len(n.Args) < minArgs {
		return fmt.Sprintf("({ fprintf(stderr, \"assertion failed: %s (missing arguments)\\n\"); exit(1); (Value){0}; })", n.Function)
	}
	msg := ""
	valueArgs := n.Args[:minArgs]
	if len(n.Args) > minArgs {
		if sl, ok := n.Args[minArgs].(*parser.StringLiteral); ok && isStringLit(n.Args[minArgs]) {
			msg = sl.Value
		}
	}
	if msg == "" {
		msg = label
	}
	gen := make([]string, 0, len(valueArgs))
	for _, a := range valueArgs {
		gen = append(gen, g.genExpr(a))
	}
	return fmt.Sprintf("karkain_%s(%s, \"%s\", \"\")",
		n.Function, strings.Join(gen, ", "), escapeCString(msg))
}

func assertLabel(fn string) string {
	switch fn {
	case "assert_eq":
		return "assert_eq(actual, expected)"
	case "assert_ne":
		return "assert_ne(actual, expected)"
	default:
		return "assert(condition)"
	}
}

func isStringLit(n parser.Node) bool {
	_, ok := n.(*parser.StringLiteral)
	return ok
}

func escapeCString(s string) string {
	r := strings.ReplaceAll(s, `\`, `\\`)
	r = strings.ReplaceAll(r, `"`, `\"`)
	r = strings.ReplaceAll(r, "\n", `\n`)
	return r
}

func (g *Generator) genExpr(node parser.Node) string {
	switch n := node.(type) {
	case *parser.StringLiteral:
		return fmt.Sprintf("make_string(%q)", unescapeKarkain(n.Value))
	case *parser.IntLiteral:
		return fmt.Sprintf("make_int(%s)", n.Value)
	case *parser.BigIntLiteral:
		return fmt.Sprintf("make_bigint(\"%s\")", strings.TrimRight(strings.TrimRight(n.Value, "n"), "N"))
	case *parser.Float64Literal:
		return fmt.Sprintf("make_float(%s)", n.Value)
	case *parser.BigFloatLiteral:
		return fmt.Sprintf("make_bigfloat(\"%s\")", strings.TrimRight(strings.TrimRight(n.Value, "b"), "B"))
	case *parser.BoolLiteral:
		if n.Value {
			return "make_int(1)"
		}
		return "make_int(0)"
	case *parser.Identifier:
		return n.Name
	case *parser.ArrayLiteral:
		var sb strings.Builder
		sb.WriteString("({ Value _arr = make_array(); ")
		for _, elem := range n.Elements {
			sb.WriteString(fmt.Sprintf("array_push(&_arr, %s); ", g.genExpr(elem)))
		}
		sb.WriteString("_arr; })")
		return sb.String()
	case *parser.MapLiteral:
		var sb strings.Builder
		sb.WriteString("({ Value _m = make_map(); ")
		for i := 0; i < len(n.Keys); i++ {
			sb.WriteString(fmt.Sprintf("map_set(&_m, %s, %s); ", g.genExpr(n.Keys[i]), g.genExpr(n.Values[i])))
		}
		sb.WriteString("_m; })")
		return sb.String()
	case *parser.IndexExpr:
		return fmt.Sprintf("array_or_string_get(%s, %s)", g.genExpr(n.Left), g.genExpr(n.Index))
	case *parser.SliceExpr:
		end := "make_int(-1)"
		if n.End != nil {
			end = g.genExpr(n.End)
		}
		return fmt.Sprintf("karkain_slice(%s, %s, %s)", g.genExpr(n.Target), g.genExpr(n.Start), end)
	case *parser.LambdaExpr:
		g.lambdaCount++
		name := fmt.Sprintf("_lambda_%d", g.lambdaCount)
		params := []string{}
		for _, p := range n.Params {
			params = append(params, "Value "+p)
		}
		var body strings.Builder
		for _, stmt := range n.Body {
			body.WriteString(g.genStatement(stmt))
		}
		// GCC nested function: define as a static function inside the enclosing scope
		fmt.Fprintf(&g.lambdaBuf, "Value %s(%s) {\n%s\treturn make_int(0);\n}\n\n", name, strings.Join(params, ", "), body.String())
		return name

	case *parser.BinaryExpr:
		if n.Operator == "=" {
			// Handle assignment specially
			left := g.genExpr(n.Left)
			right := g.genExpr(n.Right)

			// Phase 52: Index assignment â€” m[k] = v â†’ index_set(&m, k, v) (mutates variable directly)
			if idxExpr, ok := n.Left.(*parser.IndexExpr); ok {
				index := g.genExpr(idxExpr.Index)
				if ident, ok := idxExpr.Left.(*parser.Identifier); ok {
					return fmt.Sprintf("index_set(&%s, %s, %s)", ident.Name, index, right)
				}
				target := g.genExpr(idxExpr.Left)
				return fmt.Sprintf("({ Value _iset_tgt = %s; index_set(&_iset_tgt, %s, %s); _iset_tgt; })", target, index, right)
			}

			// Phase 52: Dot assignment â€” p.name = v â†’ map_set(&p, "name", v) (mutates variable directly)
			if dotExpr, ok := n.Left.(*parser.DotExpr); ok {
				if ident, ok := dotExpr.Left.(*parser.Identifier); ok {
					return fmt.Sprintf("map_set(&%s, make_string(%q), %s)", ident.Name, dotExpr.Right, right)
				}
				target := g.genExpr(dotExpr.Left)
				return fmt.Sprintf("({ Value _ms_tgt = %s; map_set(&_ms_tgt, make_string(%q), %s); _ms_tgt; })", target, dotExpr.Right, right)
			}

			// If the left side is a matrix index, we need proper type handling
			if _, ok := n.Left.(*parser.MatrixIndexExpr); ok {
				// For matrix assignment, convert the right side to proper C literal
				rightC := g.mapLiteralToC(n.Right)
				return fmt.Sprintf("%s = %s", left, rightC)
			}

			return fmt.Sprintf("%s = %s", left, right)
		}
		// Phase 81: the `%` operator shares the single authoritative modulo path
		// (binary_op), matching +,-,*,/ and the SSA emitter. Legacy karkain_mod
		// produced identical C-style results but created a second, redundant
		// code path with semantic-drift risk.
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
		// Phase 83: user functions shadow language builtins. A user `func sqrt`
		// (or readFile/push/spawn/...) must dispatch to the mangled user symbol,
		// never the runtime helper it would otherwise collide with. Capturing
		// closures (which need an implicit env argument) are handled below.
		if g.userFuncs[n.Function] && !g.closureVars[n.Function] {
			args := []string{}
			for _, arg := range n.Args {
				args = append(args, g.genExpr(arg))
			}
			return fmt.Sprintf("%s(%s)", userFuncC(n.Function), strings.Join(args, ", "))
		}
		if n.Function == "len" {
			return fmt.Sprintf("karkain_len(%s)", g.genExpr(n.Args[0]))
		}
		if n.Function == "fmt" && len(n.Args) > 0 { // Phase 55: string formatting
			parts := []string{g.genExpr(n.Args[0]), fmt.Sprintf("%d", len(n.Args)-1)}
			for _, a := range n.Args[1:] {
				parts = append(parts, g.genExpr(a))
			}
			return fmt.Sprintf("karkain_fmt(%s)", strings.Join(parts, ", "))
		}
		if n.Function == "readFile" {
			return fmt.Sprintf("karkain_readFile(%s)", g.genExpr(n.Args[0]))
		}
		if n.Function == "writeFile" {
			// CORRECT (Two separate calls to g.genExpr)
			return fmt.Sprintf("karkain_writeFile(%s, %s)", g.genExpr(n.Args[0]), g.genExpr(n.Args[1]))
		}
		if n.Function == "http.get" {
			g.needsHTTP = true
			return fmt.Sprintf("http_get(%s)", g.genExpr(n.Args[0]))
		}
		if n.Function == "substr" {
			return fmt.Sprintf("karkain_substr(%s, %s, %s)", g.genExpr(n.Args[0]), g.genExpr(n.Args[1]), g.genExpr(n.Args[2]))
		}
		if n.Function == "str" {
			return fmt.Sprintf("karkain_str(%s)", g.genExpr(n.Args[0]))
		}
		if n.Function == "int" {
			return fmt.Sprintf("karkain_int(%s)", g.genExpr(n.Args[0]))
		}
		if n.Function == "system" {
			return fmt.Sprintf("karkain_system(%s)", g.genExpr(n.Args[0]))
		}
		if n.Function == "openFile" {
			return fmt.Sprintf("openFile(%s)", g.genExpr(n.Args[0]))
		}
		if n.Function == "readLine" {
			return fmt.Sprintf("readLine(%s)", g.genExpr(n.Args[0]))
		}
		if n.Function == "readLineEOF" {
			return fmt.Sprintf("readLineEOF(%s)", g.genExpr(n.Args[0]))
		}
		if n.Function == "listFiles" {
			return fmt.Sprintf("karkain_listFiles(%s)", g.genExpr(n.Args[0]))
		}
		if n.Function == "closeFile" {
			return fmt.Sprintf("closeFile(%s)", g.genExpr(n.Args[0]))
		}
		if n.Function == "createFile" {
			return fmt.Sprintf("createFile(%s)", g.genExpr(n.Args[0]))
		}
		if n.Function == "writeToFile" {
			return fmt.Sprintf("writeToFile(%s, %s)", g.genExpr(n.Args[0]), g.genExpr(n.Args[1]))
		}
		if n.Function == "removeFile" {
			return fmt.Sprintf("removeFile(%s)", g.genExpr(n.Args[0]))
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
		if n.Function == "assert" || n.Function == "assert_eq" || n.Function == "assert_ne" {
			// KTF-001: native assertions. Optional trailing string is a message;
			// otherwise the call expression text is used as the diagnostic label.
			return g.genAssertCall(n)
		}
		if n.Function == "pow" {
			return fmt.Sprintf("karkain_pow(%s, %s)", g.genExpr(n.Args[0]), g.genExpr(n.Args[1]))
		}
		if n.Function == "appendArray" || n.Function == "push" {
			// Mutating builtin: pass first arg by pointer when it's a variable
			if ident, ok := n.Args[0].(*parser.Identifier); ok {
				return fmt.Sprintf("karkain_appendArray(&%s, %s)", ident.Name, g.genExpr(n.Args[1]))
			}
			return fmt.Sprintf("({ Value _ap_arr = %s; karkain_appendArray(&_ap_arr, %s); _ap_arr; })", g.genExpr(n.Args[0]), g.genExpr(n.Args[1]))
		}
		if n.Function == "hasKey" {
			return fmt.Sprintf("karkain_hasKey(%s, %s)", g.genExpr(n.Args[0]), g.genExpr(n.Args[1]))
		}
		if n.Function == "delete" {
			// Mutating builtin: pass first arg by pointer when it's a variable
			if ident, ok := n.Args[0].(*parser.Identifier); ok {
				return fmt.Sprintf("karkain_delete(&%s, %s)", ident.Name, g.genExpr(n.Args[1]))
			}
			return fmt.Sprintf("({ Value _del_m = %s; karkain_delete(&_del_m, %s); _del_m; })", g.genExpr(n.Args[0]), g.genExpr(n.Args[1]))
		}
		if n.Function == "mod" {
			return fmt.Sprintf("binary_op(%s, \"%%\", %s)", g.genExpr(n.Args[0]), g.genExpr(n.Args[1]))
		}
		if n.Function == "add_checked" {
			return fmt.Sprintf("karkain_add_checked(%s, %s)", g.genExpr(n.Args[0]), g.genExpr(n.Args[1]))
		}
		if n.Function == "sub_checked" {
			return fmt.Sprintf("karkain_sub_checked(%s, %s)", g.genExpr(n.Args[0]), g.genExpr(n.Args[1]))
		}
		if n.Function == "mul_checked" {
			return fmt.Sprintf("karkain_mul_checked(%s, %s)", g.genExpr(n.Args[0]), g.genExpr(n.Args[1]))
		}
		// Phase 47: Method calls â€” obj.method(args) â†’ method(obj, args)
		if strings.Contains(n.Function, ".") {
			parts := strings.SplitN(n.Function, ".", 2)
			receiver := parts[0]
			method := parts[1]
			args := []string{receiver}
			for _, arg := range n.Args {
				args = append(args, g.genExpr(arg))
			}
			return fmt.Sprintf("%s(%s)", method, strings.Join(args, ", "))
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
		// Phase 54: closure variables carry an implicit env argument
		if g.closureVars[n.Function] {
			return fmt.Sprintf("%s(_genv_%s%s)", userFuncC(n.Function), sanitizeC(n.Function),
				func() string {
					if len(args) > 0 {
						return ", " + strings.Join(args, ", ")
					}
					return ""
				}())
		}
		if g.userFuncs[n.Function] {
			return fmt.Sprintf("%s(%s)", userFuncC(n.Function), strings.Join(args, ", "))
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
	case *parser.BorrowExpr:
		// Phase 41: &x and &mut x â€” references are transparent pointers in C
		// The borrow checker validates safety at compile time, zero cost at runtime
		return g.genExpr(n.Operand)
	case *parser.MoveExpr:
		// Phase 41: move(x) â€” in C, just pass the value
		// Move semantics are enforced at compile time, zero cost at runtime
		return g.genExpr(n.Operand)
	case *parser.PropagateExpr:
		// Phase 50: expr? — error/option propagation operator
		// If the operand is Result Err or Option None, propagate it as the
		// function's return value; otherwise unwrap the Ok/Some payload.
		operand := g.genExpr(n.Operand)
		return fmt.Sprintf("(({Value _r = %s; if (_r.type == TYPE_RESULT && _r.resVal.tag == 1) return _r; if (_r.type == TYPE_OPTION && _r.optVal.tag == 0) return _r; (_r.type == TYPE_RESULT ? *_r.resVal.okVal : (_r.type == TYPE_OPTION ? *_r.optVal.inner : _r)); }))", operand)
	case *parser.EnumVariantExpr:
		// Phase 45: EnumName.Variant or EnumName.Variant(payload)
		if n.Value != nil {
			val := g.genExpr(n.Value)
			return fmt.Sprintf("%s_make_%s(%s)", n.EnumName, n.Variant, val)
		}
		return fmt.Sprintf("%s_%s", n.EnumName, n.Variant)
	case *parser.RawAccessExpr:
		// Phase 41: @raw(addr) read or @raw(addr, val) write
		// Address is a raw integer, not a Value* â€” use mapLiteralToC for raw C value
		addr := g.mapLiteralToC(n.Address)
		if n.Value != nil {
			val := g.mapLiteralToC(n.Value)
			return fmt.Sprintf("(*((volatile unsigned long long*)(%s)) = (unsigned long long)(%s))", addr, val)
		}
		return fmt.Sprintf("(*((volatile unsigned long long*)(%s)))", addr)
	// Phase 50: Option<T> and Result<T,E>
	case *parser.OptionSomeExpr:
		return fmt.Sprintf("option_some(%s)", g.genExpr(n.Value))
	case *parser.OptionNoneExpr:
		return "option_none()"
	case *parser.ResultOkExpr:
		return fmt.Sprintf("result_ok(%s)", g.genExpr(n.Value))
	case *parser.ResultErrExpr:
		return fmt.Sprintf("result_err(%s)", g.genExpr(n.Error))
	case *parser.MatchExpr:
		return g.genMatchExpr(n)
	case *parser.SIMDBuiltinExpr:
		return g.genSIMDExpr(n)
	case *parser.AtomicOp:
		return g.genAtomicExpr(n)
	case *parser.AllocExpr:
		countExpr := g.genExpr(n.Count)
		cType := g.mapKarkainTypeToC(n.Type)
		return fmt.Sprintf("(%s*)malloc(%s * sizeof(%s))", cType, countExpr, cType)
	case *parser.FreeExpr:
		return fmt.Sprintf("free(%s)", g.genExpr(n.Ptr))
	case *parser.DotExpr:
		// Phase 50: Struct field access via map_get â€” structs are stored as maps internally
		left := g.genExpr(n.Left)
		return fmt.Sprintf("map_get(%s, make_string(%q))", left, n.Right)
	case *parser.MeasureExpr:
		// Phase 14: Generate measure expression â€” yields a classical bit as Value
		regName, idx := g.resolveQubitOperand(n.Qubit)
		return fmt.Sprintf("make_int(measure(&%s, %s))", regName, idx)
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
		flags := []string{cFile, "-o", exeFile, "-std=c2x", "-O0", "-lgmp"}
		if runtime.GOOS == "windows" {
			flags = []string{cFile, "-o", exeFile, "-mconsole", "-std=c2x", "-O0", "-lgmp"}
		}
		if g.cfg.Debug {
			flags = append(flags, "-g")
		}
		return cc, flags
	}

	// Check for gcc first
	if runtime.GOOS == "windows" {
		if _, err := exec.LookPath("gcc"); err == nil {
			flags := []string{cFile, "-o", exeFile, "-mconsole", "-std=c2x", "-O0", "-lgmp"}
			if g.cfg.Debug {
				flags = append(flags, "-g")
			}
			return "gcc", flags
		}
	} else {
		if _, err := exec.LookPath("gcc"); err == nil {
			flags := []string{cFile, "-o", exeFile, "-std=c2x", "-O0", "-lgmp"}
			if g.cfg.Debug {
				flags = append(flags, "-g")
			}
			return "gcc", flags
		}
	}
	// Check for clang
	if _, err := exec.LookPath("clang"); err == nil {
		flags := []string{cFile, "-o", exeFile, "-std=c2x", "-O0", "-lgmp"}
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
	const int64_t %s_cols = %s;
`, stmt.Name, matrixDecl.DataType, cType, stmt.Name, stmt.Name, cType, rows, cols, cType, stmt.Name, cType, rows, cols, cType, stmt.Name, stmt.Name, rows, cols, cType, stmt.Name, cols)

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
	case "bigint":
		return "mpz_t"
	case "bigfloat":
		return "mpf_t"
	default:
		// Phase 41: Handle reference types â€” &T and &mut T map to Value* in C
		// References are transparent pointers; the borrow checker enforces safety at compile time
		if strings.HasPrefix(karkainType, "&mut ") {
			return "Value*"
		}
		if strings.HasPrefix(karkainType, "&") {
			return "Value*"
		}
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
	case *parser.BigIntLiteral:
		return fmt.Sprintf("make_bigint(\"%s\")", strings.TrimRight(strings.TrimRight(n.Value, "n"), "N"))
	case *parser.Float64Literal:
		return n.Value
	case *parser.BigFloatLiteral:
		return fmt.Sprintf("make_bigfloat(\"%s\")", strings.TrimRight(strings.TrimRight(n.Value, "b"), "B"))
	case *parser.Identifier:
		return n.Name
	case *parser.BinaryExpr:
		// Handle simple binary expressions that might be assignments
		return g.genExpr(node)
	default:
		return g.genExpr(node)
	}
}

// Phase 50: Struct declaration â€” structs are stored as maps internally
// The typedef is kept for documentation; actual data is map-based
func (g *Generator) genStructDecl(node *parser.StructDeclStmt) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("// struct %s { ", node.Name))
	for i, field := range node.Fields {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("%s %s", field.Type, field.Name))
	}
	sb.WriteString(" }\n")
	return sb.String()
}

// Phase 50: Generate C tagged union for enum declaration
func (g *Generator) genEnumDecl(node *parser.EnumDecl) string {
	var sb strings.Builder

	for i, v := range node.Variants {
		if v.Payload != "" {
			// Payload variant — generate a constructor function.
			// Naming must match the call site `genEnumVariantExpr` which emits
			// `%s_make_%s(%s)` (e.g. `Color_make_Red(payload)`), consistent with
			// the built-in Result constructors `Result_make_Ok`/`Result_make_Err`.
			sb.WriteString(fmt.Sprintf("Value %s_make_%s(Value payload) {\n", node.Name, v.Name))
			sb.WriteString(fmt.Sprintf("    (void)payload;\n"))
			sb.WriteString(fmt.Sprintf("    return make_int(%d);\n", i+1))
			sb.WriteString(fmt.Sprintf("}\n"))
		} else {
			// Unit variant: integer tag constant
			sb.WriteString(fmt.Sprintf("#define %s_%s make_int(%d)\n", node.Name, v.Name, i+1))
		}
	}

	sb.WriteString("\n")
	return sb.String()
}

// Phase 19: Generate C for loop from Karkain for statement
func (g *Generator) genForStmt(node *parser.ForStmt) string {
	var sb strings.Builder
	sb.WriteString("\tfor (")
	if node.Init != nil {
		initStmt := g.genStatement(node.Init)
		sb.WriteString(strings.TrimRight(initStmt, "\n"))
	}
	sb.WriteString("; ")
	if node.Condition != nil {
		fmt.Fprintf(&sb, "is_truthy(%s)", g.genExpr(node.Condition))
	}
	sb.WriteString("; ")
	if node.Post != nil {
		sb.WriteString(g.genExpr(node.Post))
	}
	sb.WriteString(") {\n")
	for _, bodyStmt := range node.Body {
		sb.WriteString("\t\t")
		sb.WriteString(g.genStatement(bodyStmt))
	}
	sb.WriteString("\t}\n")
	return sb.String()
}

// Phase 47: for-in loop codegen â€” generates C for loop over array elements
func (g *Generator) genForInStmt(node *parser.ForInStmt) string {
	var sb strings.Builder
	iterExpr := g.genExpr(node.Iter)
	if node.KeyName != "" {
		// Phase 52: Map iteration â€” for k, v in map { ... }
		sb.WriteString(fmt.Sprintf("\t{ Value _iter = %s; ", iterExpr))
		sb.WriteString(fmt.Sprintf("int _len = (_iter.type == TYPE_MAP) ? _iter.mapVal.length : 0; "))
		sb.WriteString(fmt.Sprintf("for (int _i = 0; _i < _len; _i++) { "))
		sb.WriteString(fmt.Sprintf("Value %s = *_iter.mapVal.keys[_i]; ", node.KeyName))
		sb.WriteString(fmt.Sprintf("Value %s = *_iter.mapVal.values[_i]; ", node.VarName))
		for _, bodyStmt := range node.Body {
			sb.WriteString(g.genStatement(bodyStmt))
		}
		sb.WriteString("} }")
	} else {
		// Phase 52: Array iteration
		sb.WriteString(fmt.Sprintf("\t{ Value _iter = %s; ", iterExpr))
		sb.WriteString(fmt.Sprintf("int _len = (_iter.type == TYPE_ARRAY) ? _iter.arrVal.length : 0; "))
		sb.WriteString(fmt.Sprintf("for (int _i = 0; _i < _len; _i++) { "))
		sb.WriteString(fmt.Sprintf("Value %s = *_iter.arrVal.items[_i]; ", node.VarName))
		for _, bodyStmt := range node.Body {
			sb.WriteString(g.genStatement(bodyStmt))
		}
		sb.WriteString("} }")
	}
	sb.WriteString("\n")
	return sb.String()
}

// Phase 52: Generate C struct literal â€” structs are stored as maps internally
func (g *Generator) genStructLiteral(node *parser.StructLiteral) string {
	var sb strings.Builder
	sb.WriteString("({ Value _s = make_map(); ")
	for _, field := range node.Fields {
		if binExpr, ok := field.(*parser.BinaryExpr); ok {
			if ident, ok := binExpr.Left.(*parser.Identifier); ok {
				sb.WriteString(fmt.Sprintf("map_set(&_s, make_string(%q), %s); ", ident.Name, g.genExpr(binExpr.Right)))
			}
		}
	}
	sb.WriteString("_s; })")
	return sb.String()
}

// Phase 14: Get the quantum register name
// Phase 14: Resolve a qubit operand to (registerName, rawIndex).
// Accepts register-index form qr[0] and bare index form 0.
// Bare indices resolve against the most recently declared register.
func (g *Generator) resolveQubitOperand(node parser.Node) (string, string) {
	if node == nil {
		return g.lastQuantumRegister(), "0"
	}
	if idxExpr, ok := node.(*parser.IndexExpr); ok {
		reg := g.lastQuantumRegister()
		if ident, ok := idxExpr.Left.(*parser.Identifier); ok {
			reg = ident.Name
		}
		if intLit, ok := idxExpr.Index.(*parser.IntLiteral); ok {
			return reg, intLit.Value
		}
		return reg, g.mapLiteralToC(idxExpr.Index)
	}
	if intLit, ok := node.(*parser.IntLiteral); ok {
		return g.lastQuantumRegister(), intLit.Value
	}
	return g.lastQuantumRegister(), g.mapLiteralToC(node)
}

func (g *Generator) lastQuantumRegister() string {
	if len(g.quantumRegisters) == 0 {
		return "qr"
	}
	return g.quantumRegisters[len(g.quantumRegisters)-1]
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
