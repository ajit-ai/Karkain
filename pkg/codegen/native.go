package codegen

import (
	"fmt"
	"karkain/pkg/diagnostics"
	"karkain/pkg/parser"
	"strings"
)

// NativeGenerator translates Karkain AST modules into C11/LLVM IR intermediate code
// for compilation to optimized native object files and executables.
type NativeGenerator struct {
	buf          strings.Builder
	errors       []string
	indent       int
	typeMap      map[string]string
	externFuncs  map[string]bool
	diagnostics  *CodegenDiagnostics
	symbolTable  *SymbolTable
	relocManager *RelocationManager
	debugBuilder *DebugInfoBuilder
	sourceFile   string
}

// NewNativeGenerator creates a new native code generator
func NewNativeGenerator() *NativeGenerator {
	return &NativeGenerator{
		typeMap:      make(map[string]string),
		externFuncs:  make(map[string]bool),
		diagnostics:  NewCodegenDiagnostics(""),
		symbolTable:  NewSymbolTable(),
		relocManager: NewRelocationManager(),
		debugBuilder: NewDebugInfoBuilder(),
	}
}

// GenerateModule translates a full Karkain program module into C11 source code
func (g *NativeGenerator) GenerateModule(prog *parser.Program, sourceFile string) (string, []diagnostics.Diagnostic, error) {
	g.buf.Reset()
	g.errors = nil
	g.indent = 0
	g.externFuncs = make(map[string]bool)
	g.sourceFile = sourceFile
	g.diagnostics = NewCodegenDiagnostics(sourceFile)

	g.emitPreamble()
	g.emitExternDeclarations()

	for _, stmt := range prog.Statements {
		switch node := stmt.(type) {
		case *parser.StructDeclStmt:
			g.emitStructDecl(node)
		case *parser.FuncDecl:
			g.emitFuncDecl(node)
		case *parser.ActorDeclStmt:
			g.emitActorDecl(node)
		case *parser.KernelDeclStmt:
			g.emitKernelHostStub(node)
		}
	}

	if len(g.errors) > 0 {
		diags := ConvertToDiagnostics(sourceFile, g.errors)
		return "", diags, fmt.Errorf("native codegen errors:\n%s", strings.Join(g.errors, "\n"))
	}

	// Add any diagnostics collected during generation
	diags := g.diagnostics.GetDiagnostics()
	return g.buf.String(), diags, nil
}

// GenerateObject builds a native object representation (sections, symbols,
// debug information) from an AST module. Backend emission fills the section
// payloads; symbols and source-to-address mappings are resolved here from the
// compiler structures, giving linkers and debuggers a well-defined path from
// Karkain source to native representation.
func (g *NativeGenerator) GenerateObject(prog *parser.Program, sourceFile string) (*Object, []diagnostics.Diagnostic, error) {
	g.sourceFile = sourceFile
	g.diagnostics = NewCodegenDiagnostics(sourceFile)

	obj := NewObject(sourceFile)
	obj.AddSection(".text", SectionTypeText, nil, 16)
	obj.AddSection(".rodata", SectionTypeROData, nil, 8)
	obj.AddSection(".data", SectionTypeData, nil, 8)

	st := NewSymbolTable()
	if err := st.CollectSymbolsFromAST(prog, obj); err != nil {
		diag := ReportSymbolDiagnostic(sourceFile, "program symbols", err)
		g.diagnostics.AddError(1, 1, diagnostics.CodeCodegen, diag.Message)
		return obj, g.diagnostics.GetDiagnostics(), err
	}
	if err := ValidateSymbolNames(obj.Symbols); err != nil {
		diag := ReportSymbolDiagnostic(sourceFile, "program symbols", err)
		g.diagnostics.AddError(1, 1, diagnostics.CodeCodegen, diag.Message)
		return obj, g.diagnostics.GetDiagnostics(), err
	}

	dib := NewDebugInfoBuilder()
	if err := dib.CollectDebugInfoFromAST(prog, sourceFile); err != nil {
		diag := ReportDebugDiagnostic(sourceFile, "program debug info", err)
		g.diagnostics.AddError(1, 1, diagnostics.CodeCodegen, diag.Message)
		return obj, g.diagnostics.GetDiagnostics(), err
	}
	obj.DebugInfo = dib.GetDebugInfo()

	return obj, g.diagnostics.GetDiagnostics(), nil
}

// emitPreamble writes the C11 header includes and type definitions
func (g *NativeGenerator) emitPreamble() {
	g.buf.WriteString(`#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <stdalign.h>
#include <math.h>

`)
}

// emitExternDeclarations writes extern bindings for runtime functions
func (g *NativeGenerator) emitExternDeclarations() {
	externs := []string{
		"extern void* karkain_gpu_sync(void* buffer, int direction);",
		"extern int karkain_actor_spawn(int actor_id, void* state);",
		"extern void karkain_actor_send(int actor_id, void* message);",
		"extern void* karkain_actor_recv(int actor_id);",
		"extern void karkain_scheduler_run(void);",
	}
	for _, ext := range externs {
		g.buf.WriteString(ext + "\n")
		g.externFuncs[strings.TrimSpace(ext)] = true
	}
	g.buf.WriteString("\n")
}

// emitStructDecl generates a C struct with aligned memory layout
func (g *NativeGenerator) emitStructDecl(node *parser.StructDeclStmt) {
	g.buf.WriteString(fmt.Sprintf("// Karkain struct: %s\n", node.Name))
	g.buf.WriteString(fmt.Sprintf("typedef struct __attribute__((aligned(16))) {\n"))
	for _, field := range node.Fields {
		cType := g.mapType(field.Type)
		g.buf.WriteString(fmt.Sprintf("    %s %s;\n", cType, field.Name))
	}
	g.buf.WriteString(fmt.Sprintf("} %s;\n\n", node.Name))
}

// emitFuncDecl generates a native C function from a Karkain function declaration
func (g *NativeGenerator) emitFuncDecl(fn *parser.FuncDecl) {
	retType := g.mapReturnType(fn.Name)
	params := g.mapFuncParams(fn.Params)

	g.buf.WriteString(fmt.Sprintf("%s %s(%s) {\n", retType, fn.Name, strings.Join(params, ", ")))

	g.indent++
	for _, stmt := range fn.Body {
		g.emitStmt(stmt)
	}
	g.indent--

	g.buf.WriteString("}\n\n")
}

// emitActorDecl generates a C struct and scheduling stubs for actors
func (g *NativeGenerator) emitActorDecl(node *parser.ActorDeclStmt) {
	g.buf.WriteString(fmt.Sprintf("// Actor: %s\n", node.Name))
	g.buf.WriteString(fmt.Sprintf("typedef struct {\n"))
	g.buf.WriteString("    int id;\n")
	g.buf.WriteString("    void* state;\n")
	g.buf.WriteString("    int is_running;\n")
	for _, param := range node.Params {
		g.buf.WriteString(fmt.Sprintf("    void* %s;\n", param))
	}
	g.buf.WriteString(fmt.Sprintf("} %sActor;\n\n", node.Name))

	// Generate actor init function
	g.buf.WriteString(fmt.Sprintf("void %s_init(%sActor* actor) {\n", node.Name, node.Name))
	g.indent++
	g.buf.WriteString("actor->id = -1;\n")
	g.buf.WriteString("actor->state = NULL;\n")
	g.buf.WriteString("actor->is_running = 1;\n")
	g.indent--
	g.buf.WriteString("}\n\n")
}

// emitKernelHostStub generates a host-side stub for GPU kernel launches
func (g *NativeGenerator) emitKernelHostStub(node *parser.KernelDeclStmt) {
	g.buf.WriteString(fmt.Sprintf("// Host stub for GPU kernel: %s\n", node.Name))
	g.buf.WriteString(fmt.Sprintf("void launch_%s(", node.Name))
	params := []string{}
	for _, p := range node.Params {
		params = append(params, fmt.Sprintf("void* %s", p.Name))
	}
	g.buf.WriteString(strings.Join(params, ", "))
	g.buf.WriteString(") {\n")
	g.indent++
	g.buf.WriteString(fmt.Sprintf("karkain_gpu_sync(NULL, 0); // sync before launch\n"))
	g.indent--
	g.buf.WriteString("}\n\n")
}

// emitStmt generates C code for a single statement
func (g *NativeGenerator) emitStmt(stmt parser.Node) {
	indent := strings.Repeat("    ", g.indent)
	switch node := stmt.(type) {
	case *parser.VarDeclStmt:
		cType := g.mapType(node.Type)
		if node.Type == "" {
			cType = "Value*"
		}
		g.buf.WriteString(fmt.Sprintf("%s%s %s = %s;\n", indent, cType, node.Name, g.genNativeExpr(node.Value)))
	case *parser.ReturnStmt:
		g.buf.WriteString(fmt.Sprintf("%sreturn %s;\n", indent, g.genNativeExpr(node.Value)))
	case *parser.IfStmt:
		g.buf.WriteString(fmt.Sprintf("%sif (%s) {\n", indent, g.genNativeExpr(node.Condition)))
		g.indent++
		for _, cStmt := range node.Consequence {
			g.emitStmt(cStmt)
		}
		g.indent--
		if len(node.Alternative) > 0 {
			g.buf.WriteString(fmt.Sprintf("%s} else {\n", indent))
			g.indent++
			for _, aStmt := range node.Alternative {
				g.emitStmt(aStmt)
			}
			g.indent--
		}
		g.buf.WriteString(fmt.Sprintf("%s}\n", indent))
	case *parser.WhileStmt:
		g.buf.WriteString(fmt.Sprintf("%swhile (%s) {\n", indent, g.genNativeExpr(node.Condition)))
		g.indent++
		for _, bodyStmt := range node.Body {
			g.emitStmt(bodyStmt)
		}
		g.indent--
		g.buf.WriteString(fmt.Sprintf("%s}\n", indent))
	case *parser.ForStmt:
		g.buf.WriteString(fmt.Sprintf("%sfor (", indent))
		if node.Init != nil {
			g.buf.WriteString(g.genNativeExpr(node.Init))
		}
		g.buf.WriteString("; ")
		if node.Condition != nil {
			g.buf.WriteString(g.genNativeExpr(node.Condition))
		}
		g.buf.WriteString("; ")
		if node.Post != nil {
			g.buf.WriteString(g.genNativeExpr(node.Post))
		}
		g.buf.WriteString(") {\n")
		g.indent++
		for _, bodyStmt := range node.Body {
			g.emitStmt(bodyStmt)
		}
		g.indent--
		g.buf.WriteString(fmt.Sprintf("%s}\n", indent))
	case *parser.ExprStmt:
		g.buf.WriteString(fmt.Sprintf("%s%s;\n", indent, g.genNativeExpr(node.Expression)))
	case *parser.PrintStmt:
		g.buf.WriteString(fmt.Sprintf("%sprint_value(%s);\n", indent, g.genNativeExpr(node.Value)))
	}
}

// genNativeExpr generates C expressions from AST nodes
func (g *NativeGenerator) genNativeExpr(node parser.Node) string {
	switch n := node.(type) {
	case *parser.IntLiteral:
		return n.Value
	case *parser.Float64Literal:
		return n.Value
	case *parser.StringLiteral:
		return fmt.Sprintf("make_string(%q)", unescapeKarkain(n.Value))
	case *parser.BoolLiteral:
		if n.Value {
			return "1"
		}
		return "0"
	case *parser.Identifier:
		return n.Name
	case *parser.BinaryExpr:
		left := g.genNativeExpr(n.Left)
		right := g.genNativeExpr(n.Right)
		if n.Operator == "=" {
			return fmt.Sprintf("%s = %s", left, right)
		}
		return fmt.Sprintf("(%s %s %s)", left, n.Operator, right)
	case *parser.UnaryExpr:
		operand := g.genNativeExpr(n.Operand)
		return fmt.Sprintf("(%s%s)", n.Operator, operand)
	case *parser.CallExpr:
		args := []string{}
		for _, arg := range n.Args {
			args = append(args, g.genNativeExpr(arg))
		}
		if n.IsCFunc {
			funcName := strings.TrimPrefix(n.Function, "C.")
			return fmt.Sprintf("%s(%s)", funcName, strings.Join(args, ", "))
		}
		return fmt.Sprintf("%s(%s)", n.Function, strings.Join(args, ", "))
	case *parser.IndexExpr:
		return fmt.Sprintf("%s[%s]", g.genNativeExpr(n.Left), g.genNativeExpr(n.Index))
	case *parser.DotExpr:
		return fmt.Sprintf("%s.%s", g.genNativeExpr(n.Left), n.Right)
	case *parser.ArrayLiteral:
		elements := []string{}
		for _, elem := range n.Elements {
			elements = append(elements, g.genNativeExpr(elem))
		}
		return fmt.Sprintf("({Value* _arr = make_array(); %s; _arr;})", strings.Join(elements, "; "))
	case *parser.MapLiteral:
		expr := "make_map()"
		for i := range n.Keys {
			expr = fmt.Sprintf("map_set(%s, %s, %s)", expr, g.genNativeExpr(n.Keys[i]), g.genNativeExpr(n.Values[i]))
		}
		return expr
	case *parser.StructLiteral:
		return g.genStructInit(n)
	case *parser.SpawnExpr:
		return fmt.Sprintf("karkain_actor_spawn(-1, NULL)")
	case *parser.SendExpr:
		return fmt.Sprintf("karkain_actor_send(-1, %s)", g.genNativeExpr(n.Message))
	case *parser.ReceiveStmt:
		return fmt.Sprintf("karkain_actor_recv(-1)")
	case *parser.GlobalIdExpr:
		return fmt.Sprintf("get_global_id(%d)", n.Dimension)
	}
	return "NULL"
}

// genStructInit generates a C struct initializer
func (g *NativeGenerator) genStructInit(node *parser.StructLiteral) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("(%s){", node.TypeName))
	for i, field := range node.Fields {
		if binExpr, ok := field.(*parser.BinaryExpr); ok {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(g.genNativeExpr(binExpr.Right))
		}
	}
	sb.WriteString("}")
	return sb.String()
}

// mapType maps Karkain types to C11 types
func (g *NativeGenerator) mapType(kType string) string {
	if kType == "" {
		return "Value*"
	}
	switch kType {
	case "int":
		return "int64_t"
	case "float", "float64":
		return "double"
	case "string":
		return "char*"
	case "bool":
		return "int"
	default:
		if strings.HasPrefix(kType, "*") {
			return g.mapType(strings.TrimPrefix(kType, "*")) + "*"
		}
		if strings.HasPrefix(kType, "[]") {
			return "Value*"
		}
		return kType + "*"
	}
}

// mapReturnType maps function names to their return types
func (g *NativeGenerator) mapReturnType(name string) string {
	if name == "main" {
		return "int"
	}
	return "Value*"
}

// mapFuncParams maps Karkain function parameters to C parameter declarations
func (g *NativeGenerator) mapFuncParams(params []string) []string {
	result := []string{}
	for _, p := range params {
		result = append(result, fmt.Sprintf("Value* %s", p))
	}
	return result
}

// EmitLLVMIR generates LLVM IR-compatible intermediate representation
func (g *NativeGenerator) EmitLLVMIR(prog *parser.Program) (string, error) {
	var ir strings.Builder

	ir.WriteString("target datalayout = \"e-m:w-p270:32:32-p271:32:32-p272:64:64-i64:64-f80:128-n8:16:32:64-S128\"\n")
	ir.WriteString("target triple = \"x86_64-pc-windows-msvc\"\n\n")

	ir.WriteString("%Value = type { i32, %union.ValueData }\n")
	ir.WriteString("%union.ValueData = type { i64 }\n\n")

	for _, stmt := range prog.Statements {
		if fn, ok := stmt.(*parser.FuncDecl); ok {
			ir.WriteString(g.emitLLVMFuncDecl(fn))
		}
	}

	return ir.String(), nil
}

// emitLLVMFuncDecl generates LLVM IR for a function declaration
func (g *NativeGenerator) emitLLVMFuncDecl(fn *parser.FuncDecl) string {
	retType := "i64"
	if fn.Name == "main" {
		retType = "i32"
	}

	params := []string{}
	for range fn.Params {
		params = append(params, "%Value*")
	}

	ir := fmt.Sprintf("define %s @%s(%s) {\n", retType, fn.Name, strings.Join(params, ", "))
	ir += "entry:\n"

	for _, stmt := range fn.Body {
		ir += g.emitLLVMStmt(stmt)
	}

	if fn.Name == "main" {
		ir += "  ret i32 0\n"
	} else {
		ir += "  ret i64 0\n"
	}
	ir += "}\n\n"

	return ir
}

// emitLLVMStmt generates LLVM IR for a single statement
func (g *NativeGenerator) emitLLVMStmt(stmt parser.Node) string {
	switch node := stmt.(type) {
	case *parser.ReturnStmt:
		return fmt.Sprintf("  ret i64 0\n")
	case *parser.VarDeclStmt:
		return fmt.Sprintf("  %%_%s = alloca %s\n", node.Name, g.mapType(node.Type))
	case *parser.IfStmt:
		return "  br i1 0, label %then, label %else\nthen:\n  br label %merge\nelse:\n  br label %merge\nmerge:\n"
	case *parser.WhileStmt:
		return "  br label %loop\nloop:\n  br i1 0, label %loop, label %exit\nexit:\n"
	case *parser.ForStmt:
		return "  br label %forInit\nforInit:\n  br label %forBody\nforBody:\n  br label %forPost\nforPost:\n  br i1 0, label %forBody, label %forExit\nforExit:\n"
	}
	return ""
}
