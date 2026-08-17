package sema

import (
	"fmt"
	"karkain/pkg/parser"
	"strings"
)

// TraitDef represents a declared trait with its required methods
type TraitDef struct {
	Name    string
	Methods map[string]TraitMethodSig
}

// TraitMethodSig is the signature of a required method in a trait
type TraitMethodSig struct {
	Params     []string // parameter types (simplified)
	ReturnType string
}

// ImplDef represents a concrete implementation of a trait for a type
type ImplDef struct {
	TraitName string
	ForType   string
	Methods   []string // method names implemented
}

// Monomorphizer performs type substitution and generic instantiation
type Monomorphizer struct {
	traits     map[string]*TraitDef    // trait name -> definition
	impls      map[string][]*ImplDef   // type name -> list of impls
	errors     []error
}

// NewMonomorphizer creates a new monomorphizer
func NewMonomorphizer() *Monomorphizer {
	return &Monomorphizer{
		traits: make(map[string]*TraitDef),
		impls:  make(map[string][]*ImplDef),
	}
}

// RegisterTrait registers a trait definition
func (m *Monomorphizer) RegisterTrait(trait *parser.TraitDeclStmt) {
	methods := make(map[string]TraitMethodSig)
	for _, method := range trait.Methods {
		paramTypes := make([]string, len(method.Params))
		for i, p := range method.Params {
			paramTypes[i] = p.Type
		}
		methods[method.Name] = TraitMethodSig{
			Params:     paramTypes,
			ReturnType: method.ReturnType,
		}
	}
	m.traits[trait.Name] = &TraitDef{
		Name:    trait.Name,
		Methods: methods,
	}
}

// RegisterImpl registers a trait implementation for a concrete type
func (m *Monomorphizer) RegisterImpl(impl *parser.ImplDeclStmt) {
	methodNames := make([]string, len(impl.Methods))
	for i, method := range impl.Methods {
		methodNames[i] = method.Name
	}
	m.impls[impl.ForType] = append(m.impls[impl.ForType], &ImplDef{
		TraitName: impl.TraitName,
		ForType:   impl.ForType,
		Methods:   methodNames,
	})
}

// CheckTraitConstraints verifies that a concrete type satisfies all trait constraints
// Returns an error if the type does not implement any required trait
func (m *Monomorphizer) CheckTraitConstraints(concreteType string, constraints []string) error {
	for _, traitName := range constraints {
		if err := m.checkSingleConstraint(concreteType, traitName); err != nil {
			return err
		}
	}
	return nil
}

// checkSingleConstraint checks if a type implements a single trait
func (m *Monomorphizer) checkSingleConstraint(concreteType, traitName string) error {
	// Verify the trait exists
	trait, ok := m.traits[traitName]
	if !ok {
		return fmt.Errorf("trait %q not found", traitName)
	}

	// Check if the type has an impl for this trait
	impls := m.impls[concreteType]
	for _, impl := range impls {
		if impl.TraitName == traitName {
			// Verify all required methods are implemented
			for methodName := range trait.Methods {
				found := false
				for _, m := range impl.Methods {
					if m == methodName {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf(
						"type %q implements trait %q but is missing required method %q",
						concreteType, traitName, methodName)
				}
			}
			return nil
		}
	}

	return fmt.Errorf(
		"type %q does not implement trait %q (constraint not satisfied)",
		concreteType, traitName)
}

// InstantiateGenericStruct creates a specialized struct from a generic template.
// For example, Vector<T> with T=int produces a StructDeclStmt named "Vector_int"
// with all type parameters replaced by concrete types.
func (m *Monomorphizer) InstantiateGenericStruct(
	tmpl *parser.StructDeclStmt,
	typeArgs map[string]string,
) (*parser.StructDeclStmt, error) {
	m.errors = nil

	// Build the specialized name: Vector<T> + T=int -> Vector_int
	specializedName := m.buildSpecializedName(tmpl.Name, tmpl.GenericParams, typeArgs)

	// Substitute field types
	fields := make([]parser.StructField, len(tmpl.Fields))
	for i, field := range tmpl.Fields {
		resolved, err := m.substituteType(field.Type, typeArgs)
		if err != nil {
			return nil, fmt.Errorf("struct %q field %q: %w", tmpl.Name, field.Name, err)
		}
		fields[i] = parser.StructField{
			Name: field.Name,
			Type: resolved,
		}
	}

	return &parser.StructDeclStmt{
		Name:   specializedName,
		Fields: fields,
	}, nil
}

// InstantiateGenericKernel creates a specialized kernel from a generic template.
// For example, kernel add<T>() with T=int produces a kernel named "add_int"
// with all type parameters replaced by concrete types.
func (m *Monomorphizer) InstantiateGenericKernel(
	tmpl *parser.KernelDeclStmt,
	typeArgs map[string]string,
) (*parser.KernelDeclStmt, error) {
	m.errors = nil

	// Check trait constraints first
	for _, gp := range tmpl.GenericParams {
		if concreteType, ok := typeArgs[gp.Name]; ok {
			if err := m.CheckTraitConstraints(concreteType, gp.Constraints); err != nil {
				return nil, err
			}
		}
	}

	specializedName := m.buildSpecializedName(tmpl.Name, tmpl.GenericParams, typeArgs)

	// Substitute parameter types
	params := make([]parser.Parameter, len(tmpl.Params))
	for i, p := range tmpl.Params {
		resolved, err := m.substituteType(p.Type, typeArgs)
		if err != nil {
			return nil, fmt.Errorf("kernel %q param %q: %w", tmpl.Name, p.Name, err)
		}
		params[i] = parser.Parameter{
			Name: p.Name,
			Type: resolved,
		}
	}

	// Substitute body types (var declarations with type annotations)
	body := m.substituteBody(tmpl.Body, typeArgs)

	return &parser.KernelDeclStmt{
		Name:       specializedName,
		Params:     params,
		Body:       body,
		WorkGroupX: tmpl.WorkGroupX,
		WorkGroupY: tmpl.WorkGroupY,
		WorkGroupZ: tmpl.WorkGroupZ,
	}, nil
}

// InstantiateGenericFunc creates a specialized function from a generic template
func (m *Monomorphizer) InstantiateGenericFunc(
	tmpl *parser.FuncDecl,
	typeArgs map[string]string,
) (*parser.FuncDecl, error) {
	m.errors = nil

	for _, gp := range tmpl.GenericParams {
		if concreteType, ok := typeArgs[gp.Name]; ok {
			if err := m.CheckTraitConstraints(concreteType, gp.Constraints); err != nil {
				return nil, err
			}
		}
	}

	specializedName := m.buildSpecializedName(tmpl.Name, tmpl.GenericParams, typeArgs)

	return &parser.FuncDecl{
		Name:   specializedName,
		Params: tmpl.Params,
		Body:   tmpl.Body,
	}, nil
}

// buildSpecializedName constructs the mangled name for a specialized generic.
// e.g., "Vector", [{T, []}], {T: "int"} -> "Vector_int"
func (m *Monomorphizer) buildSpecializedName(
	baseName string,
	genericParams []parser.GenericTypeParam,
	typeArgs map[string]string,
) string {
	if len(genericParams) == 0 {
		return baseName
	}

	argNames := make([]string, 0, len(genericParams))
	for _, gp := range genericParams {
		if concrete, ok := typeArgs[gp.Name]; ok {
			argNames = append(argNames, concrete)
		} else {
			argNames = append(argNames, gp.Name)
		}
	}
	return baseName + "_" + strings.Join(argNames, "_")
}

// substituteType replaces generic type parameters with concrete types in a type string
func (m *Monomorphizer) substituteType(typeStr string, typeArgs map[string]string) (string, error) {
	// Direct match: type parameter is the whole type
	if concrete, ok := typeArgs[typeStr]; ok {
		return concrete, nil
	}

	// Handle pointer types: *T
	if len(typeStr) > 1 && typeStr[0] == '*' {
		inner := typeStr[1:]
		substituted, err := m.substituteType(inner, typeArgs)
		if err != nil {
			return "", err
		}
		return "*" + substituted, nil
	}

	// Handle array types: []T
	if len(typeStr) > 2 && typeStr[:2] == "[]" {
		inner := typeStr[2:]
		substituted, err := m.substituteType(inner, typeArgs)
		if err != nil {
			return "", err
		}
		return "[]" + substituted, nil
	}

	return typeStr, nil
}

// substituteBody walks the body AST and substitutes type references in VarDeclStmt nodes
func (m *Monomorphizer) substituteBody(stmts []parser.Node, typeArgs map[string]string) []parser.Node {
	result := make([]parser.Node, len(stmts))
	for i, stmt := range stmts {
		result[i] = m.substituteNode(stmt, typeArgs)
	}
	return result
}

// substituteNode substitutes types in a single AST node
func (m *Monomorphizer) substituteNode(node parser.Node, typeArgs map[string]string) parser.Node {
	switch n := node.(type) {
	case *parser.VarDeclStmt:
		resolved, err := m.substituteType(n.Type, typeArgs)
		if err != nil {
			resolved = n.Type
		}
		return &parser.VarDeclStmt{
			Name:     n.Name,
			Value:    m.substituteNode(n.Value, typeArgs),
			Type:     resolved,
			IsMatrix: n.IsMatrix,
		}
	case *parser.BinaryExpr:
		return &parser.BinaryExpr{
			Left:     m.substituteNode(n.Left, typeArgs),
			Operator: n.Operator,
			Right:    m.substituteNode(n.Right, typeArgs),
		}
	case *parser.IndexExpr:
		return &parser.IndexExpr{
			Left:  m.substituteNode(n.Left, typeArgs),
			Index: m.substituteNode(n.Index, typeArgs),
		}
	case *parser.ExprStmt:
		return &parser.ExprStmt{
			Expression: m.substituteNode(n.Expression, typeArgs),
		}
	default:
		return node
	}
}

// GetErrors returns any errors from the last operation
func (m *Monomorphizer) GetErrors() []error {
	return m.errors
}
