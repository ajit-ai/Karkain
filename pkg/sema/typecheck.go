package sema

import (
	"fmt"
	"strconv"
	"strings"

	"karkain/pkg/parser"
)

// This file implements compile-time checking of the type-first declaration
// surface: explicit primitive/array types on declarations, typed function
// parameters and returns, assignment compatibility and array element types.
// It is diagnostics-only: codegen is unchanged.

type typeScope map[string]string

func normalizeType(t string) string {
	// Normalise the element type through any number of array suffixes, so
	// `float64[]` and `double[]` (the migration's declaration spelling) are
	// the same type — matching the scalar `float64`/`double` equivalence.
	suffix := ""
	for strings.HasSuffix(t, "[]") {
		t = strings.TrimSuffix(t, "[]")
		suffix += "[]"
	}
	if t == "float64" {
		t = "double"
	}
	return t + suffix
}

func isIntegerType(t string) bool {
	switch t {
	case "byte", "short", "int", "long", "char":
		return true
	}
	return false
}

func isFloatType(t string) bool {
	return t == "float" || t == "double"
}

func integerFits(t, lit string) bool {
	v, err := strconv.ParseInt(strings.TrimSuffix(strings.TrimSuffix(lit, "n"), "N"), 10, 64)
	if err != nil {
		return true // unknown literal form: leave it to later stages
	}
	switch t {
	case "byte":
		return v >= 0 && v <= 255
	case "short":
		return v >= -32768 && v <= 32767
	case "int":
		return v >= -2147483648 && v <= 2147483647
	case "char":
		return v >= 0 && v <= 65535
	}
	return true
}

// inferType derives an explicit type from a literal or a typed expression.
// It never guesses a width from context: an empty result means "unknown".
func (r *Resolver) inferType(n parser.Node, scope typeScope) string {
	switch v := n.(type) {
	case *parser.IntLiteral:
		return "int"
	case *parser.Float64Literal:
		return "double"
	case *parser.BoolLiteral:
		return "bool"
	case *parser.StringLiteral:
		return "string"
	case *parser.Identifier:
		if t, ok := scope[v.Name]; ok {
			return t
		}
	case *parser.CallExpr:
		if t, ok := r.funcReturnTypes[v.Function]; ok {
			return t
		}
		switch v.Function {
		case "len":
			return "int"
		case "str":
			return "string"
		case "int":
			return "int"
		case "float", "float64":
			return "double"
		}
	case *parser.ArrayLiteral:
		if len(v.Elements) == 0 {
			return "unknown[]"
		}
		return r.inferType(v.Elements[0], scope) + "[]"
	case *parser.BinaryExpr:
		switch v.Operator {
		case "==", "!=", "<", ">", "<=", ">=", "&&", "||":
			return "bool"
		case "+":
			l := r.inferType(v.Left, scope)
			if normalizeType(l) == "string" {
				return "string"
			}
			return l
		default:
			return r.inferType(v.Left, scope)
		}
	case *parser.UnaryExpr:
		return r.inferType(v.Operand, scope)
	}
	return ""
}

func tfTypesCompatible(declared, actual string) bool {
	if declared == "" || actual == "" || strings.HasPrefix(actual, "unknown") {
		return true
	}
	d, a := normalizeType(declared), normalizeType(actual)
	if d == a {
		return true
	}
	// Value is the compiler's nominal heterogeneous-cell type: in the Karkain
	// runtime it is represented as an array of mixed cells, so it accepts any
	// array form. This is a compiler-internal type, not a user-facing any.
	isArr := func(t string) bool {
		return strings.HasSuffix(t, "[]") || t == "[]" || t == "array"
	}
	if d == "Value" && isArr(a) {
		return true
	}
	if a == "Value" && isArr(d) {
		return true
	}
	if isIntegerType(d) && isIntegerType(a) {
		return true
	}
	if isFloatType(d) && (isFloatType(a) || isIntegerType(a)) {
		return true // explicit numeric widening/narrowing at the literal site
	}
	return false
}

func (r *Resolver) typeError(n parser.Node, msg string) {
	line, col, end := 0, 0, 0
	if n != nil {
		line = parser.GetLine(n)
	}
	r.errors = append(r.errors, ResolveError{Line: line, Col: col, EndCol: end, Msg: "type error: " + msg})
}

func (r *Resolver) checkDecl(vd *parser.VarDeclStmt, scope typeScope) {
	if vd == nil || vd.Type == "" {
		return
	}
	t := vd.Type
	if strings.HasPrefix(t, "map[") && strings.Contains(t, "]") {
		close := strings.Index(t, "]")
		keyType := t[len("map["):close]
		valType := t[close+1:]
		if ml, ok := vd.Value.(*parser.MapLiteral); ok {
			for i, k := range ml.Keys {
				kt := r.inferType(k, scope)
				if kt != "" && !tfTypesCompatible(keyType, kt) {
					r.typeError(k, fmt.Sprintf("map key type '%s' is not compatible with declared key type '%s'", kt, keyType))
				}
				if i < len(ml.Values) {
					vt := r.inferType(ml.Values[i], scope)
					if vt != "" && !tfTypesCompatible(valType, vt) {
						r.typeError(ml.Values[i], fmt.Sprintf("map value type '%s' is not compatible with declared value type '%s'", vt, valType))
					}
				}
			}
		}
		scope[vd.Name] = t
		return
	}
	actual := r.inferType(vd.Value, scope)
	if strings.HasSuffix(t, "[]") {
		elem := strings.TrimSuffix(t, "[]")
		if arr, ok := vd.Value.(*parser.ArrayLiteral); ok {
			for _, e := range arr.Elements {
				at := r.inferType(e, scope)
				if at != "" && !tfTypesCompatible(elem, at) {
					r.typeError(e, fmt.Sprintf("array element type '%s' is not compatible with declared element type '%s'", at, elem))
				}
				if lit, ok := e.(*parser.IntLiteral); ok && isIntegerType(elem) && !integerFits(elem, lit.Value) {
					r.typeError(e, fmt.Sprintf("integer literal '%s' is outside the range of '%s'", lit.Value, elem))
				}
			}
		}
	} else if actual != "" && !tfTypesCompatible(t, actual) {
		r.typeError(vd.Value, fmt.Sprintf("cannot initialize '%s' with a value of type '%s'", t, actual))
	} else if lit, ok := vd.Value.(*parser.IntLiteral); ok && isIntegerType(t) && !integerFits(t, lit.Value) {
		r.typeError(vd.Value, fmt.Sprintf("integer literal '%s' is outside the range of '%s'", lit.Value, t))
	}
	scope[vd.Name] = t
}

func (r *Resolver) checkCallTypes(c *parser.CallExpr, scope typeScope) {
	if c == nil {
		return
	}
	params, ok := r.funcParamTypes[c.Function]
	if !ok || len(params) == 0 {
		return
	}
	for i, a := range c.Args {
		if i >= len(params) || params[i] == "" {
			continue
		}
		at := r.inferType(a, scope)
		if at != "" && !tfTypesCompatible(params[i], at) {
			r.typeError(a, fmt.Sprintf("argument %d of '%s' has type '%s', expected '%s'", i+1, c.Function, at, params[i]))
		}
	}
	for _, a := range c.Args {
		r.checkExprTypes(a, scope)
	}
}

func (r *Resolver) checkExprTypes(n parser.Node, scope typeScope) {
	switch v := n.(type) {
	case *parser.CallExpr:
		r.checkCallTypes(v, scope)
	case *parser.BinaryExpr:
		if v.Operator == "=" {
			if id, ok := v.Left.(*parser.Identifier); ok {
				if declared, ok := scope[id.Name]; ok {
					at := r.inferType(v.Right, scope)
					if at != "" && !tfTypesCompatible(declared, at) {
						r.typeError(v, fmt.Sprintf("cannot assign a value of type '%s' to '%s' of type '%s'", at, id.Name, declared))
					}
				}
			}
		}
		r.checkExprTypes(v.Left, scope)
		r.checkExprTypes(v.Right, scope)
	case *parser.UnaryExpr:
		r.checkExprTypes(v.Operand, scope)
	case *parser.ArrayLiteral:
		for _, e := range v.Elements {
			r.checkExprTypes(e, scope)
		}
	case *parser.IndexExpr:
		r.checkExprTypes(v.Left, scope)
		r.checkExprTypes(v.Index, scope)
	}
}

func (r *Resolver) checkStmtTypes(stmts []parser.Node, scope typeScope, retType string) {
	for _, s := range stmts {
		switch n := s.(type) {
		case *parser.VarDeclStmt:
			r.checkDecl(n, scope)
			r.checkExprTypes(n.Value, scope)
		case *parser.ReturnStmt:
			if retType != "" {
				at := r.inferType(n.Value, scope)
				if at != "" && !tfTypesCompatible(retType, at) {
					r.typeError(n.Value, fmt.Sprintf("return type '%s' is not compatible with declared return type '%s'", at, retType))
				}
			}
			r.checkExprTypes(n.Value, scope)
		case *parser.ExprStmt:
			r.checkExprTypes(n.Expression, scope)
		case *parser.PrintStmt:
			r.checkExprTypes(n.Value, scope)
		case *parser.BlockStmt:
			r.checkStmtTypes(n.Statements, scope, retType)
		case *parser.IfStmt:
			r.checkStmtTypes(n.Consequence, scope, retType)
			r.checkStmtTypes(n.Alternative, scope, retType)
		case *parser.WhileStmt:
			r.checkStmtTypes(n.Body, scope, retType)
		case *parser.ForStmt:
			r.checkStmtTypes(n.Body, scope, retType)
		case *parser.ForInStmt:
			r.checkStmtTypes(n.Body, scope, retType)
		}
	}
}

// checkExplicitTypes walks every function with an explicit type-first
// signature (or explicit local declaration) and reports incompatibilities.
func (r *Resolver) checkExplicitTypes(stmts []parser.Node) {
	for _, s := range stmts {
		if fn, ok := s.(*parser.FuncDecl); ok {
			scope := typeScope{}
			for i, p := range fn.Params {
				if i < len(fn.ParamTypes) && fn.ParamTypes[i] != "" {
					scope[p] = fn.ParamTypes[i]
				}
			}
			ret := ""
			if r.funcReturnTypes[fn.Name] != "" {
				ret = r.funcReturnTypes[fn.Name]
			} else if fn.ReturnType != "" {
				ret = fn.ReturnType
			}
			r.checkStmtTypes(fn.Body, scope, ret)
		} else if vd, ok := s.(*parser.VarDeclStmt); ok {
			r.checkDecl(vd, typeScope{})
		}
	}
}

