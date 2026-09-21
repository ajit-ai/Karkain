package sema

import (
	"fmt"
	"karkain/pkg/parser"
)

// KernelAnalyzer performs static semantic analysis on GPU kernel declarations
// to prevent invalid compute operations at compile time
type KernelAnalyzer struct {
	errors       []error
	currentFunc  string
	seenFuncs    map[string]bool
}

// NewKernelAnalyzer creates a new kernel analyzer
func NewKernelAnalyzer() *KernelAnalyzer {
	return &KernelAnalyzer{
		seenFuncs: make(map[string]bool),
	}
}

// AnalyzeKernel validates a kernel declaration and returns any semantic errors
func (a *KernelAnalyzer) AnalyzeKernel(kernel *parser.KernelDeclStmt) []error {
	a.errors = nil
	a.seenFuncs = make(map[string]bool)
	a.currentFunc = kernel.Name

	a.validateParams(kernel)
	a.analyzeBody(kernel.Body)

	return a.errors
}

// validateParams ensures kernel parameters are scalar or array-pointer types
func (a *KernelAnalyzer) validateParams(kernel *parser.KernelDeclStmt) {
	for _, p := range kernel.Params {
		if !a.isValidKernelParamType(p.Type) {
			a.errors = append(a.errors, fmt.Errorf(
				"kernel '%s': parameter '%s' has invalid type '%s'; only scalar and array-pointer types allowed",
				kernel.Name, p.Name, p.Type))
		}
	}
}

// isValidKernelParamType checks if a type is valid for kernel parameters
func (a *KernelAnalyzer) isValidKernelParamType(t string) bool {
	switch t {
	case "int", "float", "float64", "double", "bool":
		return true
	default:
		// Allow array types (denoted by [] prefix or pointer types)
		if len(t) > 2 && t[:2] == "[]" {
			return true
		}
		if len(t) > 1 && t[0] == '*' {
			return true
		}
		return false
	}
}

// analyzeBody recursively analyzes statements in the kernel body
func (a *KernelAnalyzer) analyzeBody(stmts []parser.Node) {
	for _, stmt := range stmts {
		a.analyzeStatement(stmt)
	}
}

// analyzeStatement checks a single statement for forbidden operations
func (a *KernelAnalyzer) analyzeStatement(stmt parser.Node) {
	switch node := stmt.(type) {
	case *parser.ExprStmt:
		a.analyzeExpression(node.Expression)
	case *parser.VarDeclStmt:
		a.analyzeExpression(node.Value)
	case *parser.IfStmt:
		a.analyzeExpression(node.Condition)
		a.analyzeBody(node.Consequence)
		a.analyzeBody(node.Alternative)
	case *parser.WhileStmt:
		a.analyzeExpression(node.Condition)
		a.analyzeBody(node.Body)
	case *parser.ForStmt:
		a.analyzeStatement(node.Init)
		a.analyzeExpression(node.Condition)
		a.analyzeStatement(node.Post)
		a.analyzeBody(node.Body)
	case *parser.ReturnStmt:
		a.analyzeExpression(node.Value)
	case *parser.PrintStmt:
		a.analyzeExpression(node.Value)
	case *parser.SpawnExpr:
		a.errors = append(a.errors, fmt.Errorf(
			"kernel '%s': actor spawn operation forbidden inside GPU kernel", a.currentFunc))
	case *parser.ReceiveStmt:
		a.errors = append(a.errors, fmt.Errorf(
			"kernel '%s': channel receive operation forbidden inside GPU kernel", a.currentFunc))
	case *parser.SendExpr:
		a.errors = append(a.errors, fmt.Errorf(
			"kernel '%s': channel send operation forbidden inside GPU kernel", a.currentFunc))
	case *parser.ActorDeclStmt:
		a.errors = append(a.errors, fmt.Errorf(
			"kernel '%s': actor declaration forbidden inside GPU kernel", a.currentFunc))
	case *parser.BarrierStmt:
		// Barriers are allowed in kernels
	case *parser.KernelDeclStmt:
		a.errors = append(a.errors, fmt.Errorf(
			"kernel '%s': nested kernel declaration forbidden", a.currentFunc))
	}
}

// analyzeExpression checks expressions for forbidden operations
func (a *KernelAnalyzer) analyzeExpression(expr parser.Node) {
	switch node := expr.(type) {
	case *parser.CallExpr:
		a.analyzeCallExpr(node)
	case *parser.IndirectCallExpr:
		// Phase 133: analyze the computed callee and arguments; indirect
		// calls are not GPU-kernel compatible (rejected downstream).
		a.analyzeExpression(node.Target)
		for _, arg := range node.Args {
			a.analyzeExpression(arg)
		}
	case *parser.BinaryExpr:
		a.analyzeExpression(node.Left)
		a.analyzeExpression(node.Right)
	case *parser.UnaryExpr:
		a.analyzeExpression(node.Operand)
	case *parser.IndexExpr:
		a.analyzeExpression(node.Left)
		a.analyzeExpression(node.Index)
	case *parser.ArrayLiteral:
		for _, elem := range node.Elements {
			a.analyzeExpression(elem)
		}
	case *parser.MapLiteral:
		for i := range node.Keys {
			a.analyzeExpression(node.Keys[i])
			a.analyzeExpression(node.Values[i])
		}
	case *parser.SendExpr:
		a.errors = append(a.errors, fmt.Errorf(
			"kernel '%s': channel send operation forbidden inside GPU kernel", a.currentFunc))
	case *parser.SpawnExpr:
		a.errors = append(a.errors, fmt.Errorf(
			"kernel '%s': actor spawn operation forbidden inside GPU kernel", a.currentFunc))
	case *parser.Identifier, *parser.IntLiteral, *parser.Float64Literal,
		*parser.StringLiteral, *parser.BoolLiteral:
		// Literals and identifiers are safe
	case *parser.GlobalIdExpr:
		// GPU built-in, allowed
	}
}

// analyzeCallExpr validates function calls for forbidden operations
func (a *KernelAnalyzer) analyzeCallExpr(call *parser.CallExpr) {
	// Forbid heap allocation
	if call.Function == "alloc" || call.Function == "malloc" {
		a.errors = append(a.errors, fmt.Errorf(
			"kernel '%s': heap allocation ('%s') forbidden inside GPU kernel",
			a.currentFunc, call.Function))
		return
	}

	// Forbid dynamic memory operations
	if call.Function == "free" || call.Function == "realloc" {
		a.errors = append(a.errors, fmt.Errorf(
			"kernel '%s': dynamic memory operation ('%s') forbidden inside GPU kernel",
			a.currentFunc, call.Function))
		return
	}

	// Forbid file I/O
	if call.Function == "readFile" || call.Function == "writeFile" {
		a.errors = append(a.errors, fmt.Errorf(
			"kernel '%s': file I/O operation ('%s') forbidden inside GPU kernel",
			a.currentFunc, call.Function))
		return
	}

	// Forbid network operations
	if call.Function == "http.get" || call.Function == "http.post" {
		a.errors = append(a.errors, fmt.Errorf(
			"kernel '%s': network operation ('%s') forbidden inside GPU kernel",
			a.currentFunc, call.Function))
		return
	}

	// Forbid actor spawn
	if call.Function == "spawn" {
		a.errors = append(a.errors, fmt.Errorf(
			"kernel '%s': actor spawn operation forbidden inside GPU kernel",
			a.currentFunc))
		return
	}

	// Recursion check
	if call.Function == a.currentFunc {
		a.errors = append(a.errors, fmt.Errorf(
			"kernel '%s': recursive call forbidden inside GPU kernel",
			a.currentFunc))
		return
	}

	// Analyze arguments
	for _, arg := range call.Args {
		a.analyzeExpression(arg)
	}
}
