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
	cfg           Config
	structRegistry map[string]*parser.StructDecl // name → decl
}

func New(cfg Config) *Generator {
	return &Generator{cfg: cfg, structRegistry: map[string]*parser.StructDecl{}}
}

func (g *Generator) GenerateAndCompile(prog *parser.Program, sourceFile string) error {
	cCode := g.generateCHeader()

	// First pass: collect all struct declarations and emit their C typedefs
	var structDefs strings.Builder
	for _, stmt := range prog.Statements {
		if sd, ok := stmt.(*parser.StructDecl); ok {
			g.structRegistry[sd.Name] = sd
			structDefs.WriteString(g.genStructDecl(sd))
		}
	}
	cCode += structDefs.String()

	// Second pass: emit function definitions
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
#include <math.h>
#include <ctype.h>

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

void map_delete(Value* m, Value* k) {
    if (!m || m->type != TYPE_MAP) return;
    for (int i = 0; i < m->mapVal.length; i++) {
        if (values_equal(m->mapVal.keys[i], k)) {
            for (int j = i; j < m->mapVal.length - 1; j++) {
                m->mapVal.keys[j]   = m->mapVal.keys[j+1];
                m->mapVal.values[j] = m->mapVal.values[j+1];
            }
            m->mapVal.length--;
            return;
        }
    }
}

Value* map_has(Value* m, Value* k) {
    if (!m || m->type != TYPE_MAP) return make_int(0);
    for (int i = 0; i < m->mapVal.length; i++) {
        if (values_equal(m->mapVal.keys[i], k)) return make_int(1);
    }
    return make_int(0);
}

Value* karkain_split(Value* s, Value* sep) {
    Value* arr = make_array();
    if (!s || s->type != TYPE_STRING || !sep || sep->type != TYPE_STRING) {
        return arr;
    }
    char* src = strdup(s->strVal);
    char* token = strtok(src, sep->strVal);
    while (token != NULL) {
        array_push(arr, make_string(token));
        token = strtok(NULL, sep->strVal);
    }
    free(src);
    return arr;
}

Value* karkain_contains(Value* s, Value* substr) {
    if (!s || s->type != TYPE_STRING || !substr || substr->type != TYPE_STRING) {
        return make_int(0);
    }
    return make_int(strstr(s->strVal, substr->strVal) != NULL);
}

Value* karkain_trim(Value* s) {
    if (!s || s->type != TYPE_STRING) {
        return make_string("");
    }
    char* start = s->strVal;
    while (*start && isspace((unsigned char)*start)) {
        start++;
    }
    char* end = start + strlen(start) - 1;
    while (end > start && isspace((unsigned char)*end)) {
        end--;
    }
    int len = (end >= start) ? (int)(end - start + 1) : 0;
    char* res = (char*)malloc(len + 1);
    memcpy(res, start, len);
    res[len] = '\0';
    Value* val = make_string(res);
    free(res);
    return val;
}

Value* karkain_sqrt(Value* x) {
    if (!x || x->type != TYPE_INT) return make_int(0);
    return make_int((long long)sqrt((double)x->intVal));
}

Value* karkain_pow(Value* x, Value* y) {
    if (!x || x->type != TYPE_INT || !y || y->type != TYPE_INT) return make_int(0);
    return make_int((long long)pow((double)x->intVal, (double)y->intVal));
}

Value* karkain_abs(Value* x) {
    if (!x || x->type != TYPE_INT) return make_int(0);
    return make_int(x->intVal < 0 ? -x->intVal : x->intVal);
}

`
}

// genStructDecl emits a C typedef struct for the given struct declaration.
func (g *Generator) genStructDecl(sd *parser.StructDecl) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("typedef struct %s {\n", sd.Name))
	for _, f := range sd.Fields {
		switch f.Type {
		case "int":
			sb.WriteString(fmt.Sprintf("    long long %s;\n", f.Name))
		case "string":
			sb.WriteString(fmt.Sprintf("    char* %s;\n", f.Name))
		default:
			// Unknown / nested struct type — use a Value* pointer for flexibility
			sb.WriteString(fmt.Sprintf("    Value* %s;\n", f.Name))
		}
	}
	sb.WriteString(fmt.Sprintf("} %s;\n\n", sd.Name))
	return sb.String()
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
		// Check if initialiser is a StructLiteral → emit typed pointer
		if sl, ok := node.Value.(*parser.StructLiteral); ok {
			return fmt.Sprintf("\t%s* %s = %s;\n", sl.TypeName, node.Name, g.genExpr(node.Value))
		}
		return fmt.Sprintf("\tValue* %s = %s;\n", node.Name, g.genExpr(node.Value))
	case *parser.AssignStmt:
		return fmt.Sprintf("\t%s = %s;\n", node.Name, g.genExpr(node.Value))
	case *parser.FieldAssignStmt:
		return fmt.Sprintf("\t%s->%s = %s;\n", g.genExpr(node.Object), node.Field, g.genStructFieldRHS(node.Field, node.Object, node.Value))
	case *parser.ReturnStmt:
		return fmt.Sprintf("\treturn %s;\n", g.genExpr(node.Value))
	case *parser.PrintStmt:
		return g.genPrintStmt(node)
	case *parser.ExprStmt:
		return fmt.Sprintf("\t%s;\n", g.genExpr(node.Expression))
	case *parser.DeleteStmt:
		return fmt.Sprintf("\tmap_delete(%s, %s);\n", g.genExpr(node.Map), g.genExpr(node.Key))
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

// genPrintStmt handles print — if the argument is a FieldAccess on a struct, emit the right printf.
func (g *Generator) genPrintStmt(node *parser.PrintStmt) string {
	if fa, ok := node.Value.(*parser.FieldAccess); ok {
		if fieldType := g.lookupFieldType(fa); fieldType != "" {
			switch fieldType {
			case "int":
				return fmt.Sprintf("\tprintf(\"%%lld\\n\", %s->%s); fflush(stdout);\n", g.genExpr(fa.Left), fa.Field)
			case "string":
				return fmt.Sprintf("\tprintf(\"%%s\\n\", %s->%s); fflush(stdout);\n", g.genExpr(fa.Left), fa.Field)
			}
		}
	}
	return fmt.Sprintf("\tprint_value(%s);\n", g.genExpr(node.Value))
}

// lookupFieldType resolves the declared C type of a struct field from the registry.
func (g *Generator) lookupFieldType(fa *parser.FieldAccess) string {
	var typeName string
	switch obj := fa.Left.(type) {
	case *parser.Identifier:
		// We don't have a variable-to-type mapping, so scan the registry for this field name
		_ = obj
		for _, sd := range g.structRegistry {
			for _, f := range sd.Fields {
				if f.Name == fa.Field {
					return f.Type
				}
			}
		}
	}
	_ = typeName
	return ""
}

// genStructFieldRHS generates the RHS for a field assignment, converting to the right C type.
func (g *Generator) genStructFieldRHS(fieldName string, obj parser.Node, val parser.Node) string {
	// Find the field type in the registry
	for _, sd := range g.structRegistry {
		for _, f := range sd.Fields {
			if f.Name == fieldName {
				switch f.Type {
				case "int":
					// If the value is an IntLiteral or expression returning Value*, unwrap it
					if il, ok := val.(*parser.IntLiteral); ok {
						return il.Value
					}
					return g.genExpr(val) + "->intVal"
				case "string":
					if sl, ok := val.(*parser.StringLiteral); ok {
						return fmt.Sprintf("%q", sl.Value)
					}
					return g.genExpr(val) + "->strVal"
				}
			}
		}
	}
	return g.genExpr(val)
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
		if n.Function == "delete" {
			return fmt.Sprintf("(map_delete(%s, %s), make_int(0))", g.genExpr(n.Args[0]), g.genExpr(n.Args[1]))
		}
		if n.Function == "hasKey" {
			return fmt.Sprintf("map_has(%s, %s)", g.genExpr(n.Args[0]), g.genExpr(n.Args[1]))
		}
		if n.Function == "push" {
			return fmt.Sprintf("(array_push(%s, %s), make_int(0))", g.genExpr(n.Args[0]), g.genExpr(n.Args[1]))
		}
		if n.Function == "readFile" {
			return fmt.Sprintf("karkain_readFile(%s)", g.genExpr(n.Args[0]))
		}
		if n.Function == "writeFile" {
			return fmt.Sprintf("karkain_writeFile(%s, %s)", g.genExpr(n.Args[0]), g.genExpr(n.Args[1]))
		}
		if n.Function == "split" {
			return fmt.Sprintf("karkain_split(%s, %s)", g.genExpr(n.Args[0]), g.genExpr(n.Args[1]))
		}
		if n.Function == "contains" {
			return fmt.Sprintf("karkain_contains(%s, %s)", g.genExpr(n.Args[0]), g.genExpr(n.Args[1]))
		}
		if n.Function == "trim" {
			return fmt.Sprintf("karkain_trim(%s)", g.genExpr(n.Args[0]))
		}
		if n.Function == "sqrt" {
			return fmt.Sprintf("karkain_sqrt(%s)", g.genExpr(n.Args[0]))
		}
		if n.Function == "pow" {
			return fmt.Sprintf("karkain_pow(%s, %s)", g.genExpr(n.Args[0]), g.genExpr(n.Args[1]))
		}
		if n.Function == "abs" {
			return fmt.Sprintf("karkain_abs(%s)", g.genExpr(n.Args[0]))
		}
		args := []string{}
		for _, arg := range n.Args {
			args = append(args, g.genExpr(arg))
		}
		return fmt.Sprintf("%s(%s)", n.Function, strings.Join(args, ", "))
	case *parser.FieldAccess:
		// Struct field read — detect the field type to emit the right accessor
		if fieldType := g.lookupFieldType(n); fieldType != "" {
			switch fieldType {
			case "int":
				return fmt.Sprintf("make_int(%s->%s)", g.genExpr(n.Left), n.Field)
			case "string":
				return fmt.Sprintf("make_string(%s->%s)", g.genExpr(n.Left), n.Field)
			}
		}
		// Fallback: raw pointer field access
		return fmt.Sprintf("%s->%s", g.genExpr(n.Left), n.Field)
	case *parser.StructLiteral:
		sd, ok := g.structRegistry[n.TypeName]
		if !ok {
			return "NULL"
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("((%s*)({ %s* _s = (%s*)calloc(1, sizeof(%s));", n.TypeName, n.TypeName, n.TypeName, n.TypeName))
		for _, f := range sd.Fields {
			if val, found := n.Fields[f.Name]; found {
				switch f.Type {
				case "int":
					if il, ok2 := val.(*parser.IntLiteral); ok2 {
						sb.WriteString(fmt.Sprintf(" _s->%s = %s;", f.Name, il.Value))
					} else {
						sb.WriteString(fmt.Sprintf(" _s->%s = %s->intVal;", f.Name, g.genExpr(val)))
					}
				case "string":
					if sl, ok2 := val.(*parser.StringLiteral); ok2 {
						sb.WriteString(fmt.Sprintf(" _s->%s = %q;", f.Name, sl.Value))
					} else {
						sb.WriteString(fmt.Sprintf(" _s->%s = %s->strVal;", f.Name, g.genExpr(val)))
					}
				default:
					sb.WriteString(fmt.Sprintf(" _s->%s = %s;", f.Name, g.genExpr(val)))
				}
			}
		}
		sb.WriteString(" _s; }))")
		return sb.String()
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
