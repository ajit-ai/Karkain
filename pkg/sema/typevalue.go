// v1.2.26 (S1 foundation): minimal Type value representation.
//
// A Type is a structured value (Kind + element/arg types) instead of a raw
// string. This is the seed of the type-system foundation: it does not yet
// replace the string annotations (those remain for v1.x compatibility, per
// Decision D-4), but it gives the resolver a place to record and compare
// types without blocking future real type values.
package sema

import (
	"strings"

	"karkain/pkg/parser"
)

// TypeKind enumerates the primitive type kinds Karkain understands.
type TypeKind int

const (
	TypeUnknown TypeKind = iota
	TypeInt
	TypeFloat64
	TypeBool
	TypeString
	TypeArray
	TypeStruct
	TypeEnum
	TypeOption
	TypeResult
	TypeFunc
	TypeUnit
)

// Type is a minimal structured type value.
type Type struct {
	Kind    TypeKind
	Name    string   // struct/enum/function name or primitive spelling
	Elem    *Type    // element type for arrays/Option/Result
	Args    []*Type  // function params / generic args
	Return  *Type    // function return type
}

// String renders a Type back to its canonical spelling.
func (t *Type) String() string {
	if t == nil {
		return ""
	}
	switch t.Kind {
	case TypeInt:
		return "int"
	case TypeFloat64:
		return "float64"
	case TypeBool:
		return "bool"
	case TypeString:
		return "string"
	case TypeUnit:
		return "unit"
	case TypeArray:
		if t.Elem != nil {
			return t.Elem.String() + "[]"
		}
		return "[]"
	case TypeOption:
		if t.Elem != nil {
			return "Option<" + t.Elem.String() + ">"
		}
		return "Option"
	case TypeResult:
		if t.Elem != nil {
			return "Result<" + t.Elem.String() + ">"
		}
		return "Result"
	case TypeStruct, TypeEnum, TypeFunc:
		if t.Name != "" {
			return t.Name
		}
	}
	if t.Name != "" {
		return t.Name
	}
	return "unknown"
}

// ParseTypeString lowers a Karkain type-annotation string into a Type value.
// This is a best-effort, non-validating lowering used by S1; full validation
// and inference land in later slices.
func ParseTypeString(s string) *Type {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	switch s {
	case "int", "i32", "i64":
		return &Type{Kind: TypeInt, Name: s}
	case "float64", "f64", "float", "f32":
		return &Type{Kind: TypeFloat64, Name: s}
	case "bool":
		return &Type{Kind: TypeBool, Name: s}
	case "string":
		return &Type{Kind: TypeString, Name: s}
	case "unit", "void":
		return &Type{Kind: TypeUnit, Name: s}
	}
	if strings.HasSuffix(s, "[]") {
		elem := ParseTypeString(strings.TrimSuffix(s, "[]"))
		return &Type{Kind: TypeArray, Name: s, Elem: elem}
	}
	// Option/Result shapes: "Option", "Result" (full generics deferred).
	if s == "Option" {
		return &Type{Kind: TypeOption, Name: s}
	}
	if s == "Result" {
		return &Type{Kind: TypeResult, Name: s}
	}
	// Default: treat as a named type (struct/enum/function).
	return &Type{Kind: TypeStruct, Name: s}
}

// InferTypeValue attempts to infer a *Type from an expression node.
// Best-effort, non-validating: covers literals, struct literals, array
// literals, Option/Result constructors. Returns nil when unknown.
func InferTypeValue(node parser.Node) *Type {
	if node == nil {
		return nil
	}
	switch n := node.(type) {
	case *parser.IntLiteral:
		return &Type{Kind: TypeInt, Name: "int"}
	case *parser.Float64Literal:
		return &Type{Kind: TypeFloat64, Name: "float64"}
	case *parser.BoolLiteral:
		return &Type{Kind: TypeBool, Name: "bool"}
	case *parser.StringLiteral:
		return &Type{Kind: TypeString, Name: "string"}
	case *parser.StructLiteral:
		if n.TypeName != "" {
			return &Type{Kind: TypeStruct, Name: n.TypeName}
		}
	case *parser.ArrayLiteral:
		if len(n.Elements) > 0 {
			elem := InferTypeValue(n.Elements[0])
			return &Type{Kind: TypeArray, Name: "[]", Elem: elem}
		}
		return &Type{Kind: TypeArray, Name: "[]"}
	case *parser.OptionSomeExpr:
		return &Type{Kind: TypeOption, Name: "Option", Elem: InferTypeValue(n.Value)}
	case *parser.OptionNoneExpr:
		return &Type{Kind: TypeOption, Name: "Option"}
	case *parser.ResultOkExpr:
		return &Type{Kind: TypeResult, Name: "Result", Elem: InferTypeValue(n.Value)}
	case *parser.ResultErrExpr:
		return &Type{Kind: TypeResult, Name: "Result"}
	}
	return nil
}

// InferTypeValueWithContext is the bidirectional half of S7: an expected type
// refines expressions whose own shape is ambiguous (an empty array literal,
// some/ok constructors without an element annotation, an int literal used
// where a float is expected). It is METADATA ONLY — like InferTypeValue it
// never feeds Resolver.funcReturnTypes or the strict declaration checker
// (see the S4a corrective note in resolve.go). When the expression already
// has a concrete type, that type wins unless it is a numeric literal that the
// expected type legitimately widens.
func InferTypeValueWithContext(node parser.Node, expected *Type) *Type {
	if expected == nil {
		return InferTypeValue(node)
	}
	switch n := node.(type) {
	case *parser.ArrayLiteral:
		if expected.Kind == TypeArray {
			elem := expected.Elem
			if elem == nil && len(n.Elements) > 0 {
				elem = InferTypeValue(n.Elements[0])
			}
			name := expected.Name
			if name == "" {
				name = "[]"
			}
			return &Type{Kind: TypeArray, Name: name, Elem: elem}
		}
	case *parser.OptionSomeExpr:
		if expected.Kind == TypeOption {
			elem := expected.Elem
			if elem == nil {
				elem = InferTypeValue(n.Value)
			}
			return &Type{Kind: TypeOption, Name: "Option", Elem: elem}
		}
	case *parser.ResultOkExpr:
		if expected.Kind == TypeResult {
			elem := expected.Elem
			if elem == nil {
				elem = InferTypeValue(n.Value)
			}
			return &Type{Kind: TypeResult, Name: "Result", Elem: elem}
		}
	case *parser.StructLiteral:
		if n.TypeName == "" && expected.Kind == TypeStruct {
			return expected
		}
	case *parser.IntLiteral:
		// Numeric widening is a contextual decision, not a rewrite of the
		// expression: int -> float64 is metadata here; emitted code is
		// unchanged and the strict checker still uses the annotation string.
		if expected.Kind == TypeFloat64 {
			return expected
		}
	}
	if inferred := InferTypeValue(node); inferred != nil {
		return inferred
	}
	return expected
}

// Equal reports whether two Types are structurally equal.
func (a *Type) Equal(b *Type) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Kind != b.Kind || a.Name != b.Name {
		return false
	}
	if !a.Elem.Equal(b.Elem) {
		return false
	}
	if len(a.Args) != len(b.Args) {
		return false
	}
	for i := range a.Args {
		if !a.Args[i].Equal(b.Args[i]) {
			return false
		}
	}
	return a.Return.Equal(b.Return)
}