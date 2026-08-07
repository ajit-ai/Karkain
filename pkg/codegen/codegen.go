package codegen

import (
	"fmt"
	"karkain/pkg/parser"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type Config struct {
	OutputPath  string
	CompileOnly bool
	Verbose     bool
	RunAfter    bool
}

type Generator struct {
	cfg Config
}

func New(cfg Config) *Generator {
	return &Generator{cfg: cfg}
}

func (g *Generator) GenerateAndCompile(prog *parser.Program, sourceFile string) error {
	cCode := g.generateCHeader()

	for _, stmt := range prog.Statements {
		if fn, ok := stmt.(*parser.FuncDecl); ok {
			cCode += g.genFuncDecl(fn)
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

	if !g.cfg.CompileOnly {
		defer os.Remove(tmpCFile)
	}

	compiler, flags := g.detectCompiler(tmpCFile, exeFile)
	if compiler == "" {
		return fmt.Errorf("no supported C compiler found (GCC, Clang, or MSVC cl.exe required)")
	}

	if g.cfg.Verbose {
		fmt.Printf("=== [4] C COMPILER INVOCATION ===\n%s %s\n", compiler, strings.Join(flags, " "))
	}

	compileCmd := exec.Command(compiler, flags...)
	compileCmd.Stdout = os.Stdout
	compileCmd.Stderr = os.Stderr
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

typedef enum { TYPE_INT, TYPE_STRING, TYPE_ARRAY, TYPE_MAP } ValueType;

typedef struct Value {
    ValueType type;
    union {
        long long intVal;
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
    if (strcmp(op, "==") == 0) {
        if (left->type == TYPE_INT && right->type == TYPE_INT) return make_int(left->intVal == right->intVal);
        if (left->type == TYPE_STRING && right->type == TYPE_STRING) return make_int(strcmp(left->strVal, right->strVal) == 0);
    }
    return make_int(0);
}

int is_truthy(Value* v) {
    if (!v) return 0;
    if (v->type == TYPE_INT) return v->intVal != 0;
    if (v->type == TYPE_STRING) return strlen(v->strVal) > 0;
    if (v->type == TYPE_ARRAY) return v->arrVal.length > 0;
    if (v->type == TYPE_MAP) return v->mapVal.length > 0;
    return 0;
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
		return fmt.Sprintf("\tValue* %s = %s;\n", node.Name, g.genExpr(node.Value))
	case *parser.ReturnStmt:
		return fmt.Sprintf("\treturn %s;\n", g.genExpr(node.Value))
	case *parser.PrintStmt:
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
			res += " else {\n"
			for _, aStmt := range node.Alternative {
				res += "\t" + g.genStatement(aStmt)
			}
			res += "\t}"
		}
		res += "\n"
		return res
	}
	return ""
}

func (g *Generator) genExpr(node parser.Node) string {
	switch n := node.(type) {
	case *parser.StringLiteral:
		return fmt.Sprintf("make_string(%q)", n.Value)
	case *parser.IntLiteral:
		return fmt.Sprintf("make_int(%s)", n.Value)
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
		return fmt.Sprintf("binary_op(%s, %q, %s)", g.genExpr(n.Left), n.Operator, g.genExpr(n.Right))
	case *parser.CallExpr:
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
		args := []string{}
		for _, arg := range n.Args {
			args = append(args, g.genExpr(arg))
		}
		return fmt.Sprintf("%s(%s)", n.Function, strings.Join(args, ", "))
	}
	return "make_int(0)"
}

func (g *Generator) detectCompiler(cFile, exeFile string) (string, []string) {
	if _, err := exec.LookPath("gcc"); err == nil {
		return "gcc", []string{cFile, "-o", exeFile}
	}
	if _, err := exec.LookPath("clang"); err == nil {
		return "clang", []string{cFile, "-o", exeFile}
	}
	if _, err := exec.LookPath("cl"); err == nil {
		return "cl", []string{cFile, fmt.Sprintf("/Fe:%s", exeFile)}
	}
	return "", nil
}
