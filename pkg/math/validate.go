package math

import (
	"fmt"
	"strings"
)

// ValidationError represents a single validation error.
type ValidationError struct {
	Node    Node
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error at %s: %s", e.Node.Op(), e.Message)
}

// Validate checks a Math IR graph for well-formedness.
// Returns a list of errors (empty if valid).
func Validate(g *Graph) []error {
	var errs []error
	seen := make(map[Node]bool)

	// Check inputs are variables
	for _, input := range g.Inputs {
		if input == nil {
			errs = append(errs, fmt.Errorf("nil input variable"))
			continue
		}
	}

	// Validate each node
	for name, node := range g.Nodes {
		if node == nil {
			errs = append(errs, fmt.Errorf("nil node %q", name))
			continue
		}
		if !seen[node] {
			seen[node] = true
			nodeErrs := validateNode(node)
			for _, e := range nodeErrs {
				errs = append(errs, fmt.Errorf("in node %q: %w", name, e))
			}
		}
	}

	// Validate outputs
	for i, out := range g.Outputs {
		if out == nil {
			errs = append(errs, fmt.Errorf("nil output %d", i))
			continue
		}
		nodeErrs := validateNode(out)
		for _, e := range nodeErrs {
			errs = append(errs, fmt.Errorf("in output %d: %w", i, e))
		}
	}

	return errs
}

// validateNode checks a single node for well-formedness.
func validateNode(node Node) []error {
	var errs []error

	switch n := node.(type) {
	case *IntConst:
		// Always valid
	case *FloatConst:
		// Check for NaN/Inf
		if isNaN(n.Value) {
			errs = append(errs, fmt.Errorf("constant is NaN"))
		}
		if isInf(n.Value) {
			errs = append(errs, fmt.Errorf("constant is Inf"))
		}
	case *BoolConst:
		// Always valid
	case *StringConst:
		// Always valid
	case *Variable:
		// Always valid
	case *Param:
		// Always valid
	case *BinaryOp:
		errs = append(errs, validateBinaryOp(n)...)
	case *UnaryOp:
		errs = append(errs, validateUnaryOp(n)...)
	case *FuncCall:
		errs = append(errs, validateFuncCall(n)...)
	case *Let:
		errs = append(errs, validateLet(n)...)
	}

	return errs
}

// validateBinaryOp checks a binary operation.
func validateBinaryOp(n *BinaryOp) []error {
	var errs []error

	// Check operand types
	if n.Left == nil || n.Right == nil {
		errs = append(errs, fmt.Errorf("binary op %s has nil operand", n.OpName))
		return errs
	}

	// Check type consistency
	leftType := n.Left.Type()
	rightType := n.Right.Type()

	// For arithmetic ops, both operands must be numeric
	switch n.OpName {
	case "+", "-", "*", "/", "%", "^":
		if !leftType.IsNumeric() {
			errs = append(errs, fmt.Errorf("left operand of %s is not numeric: %s", n.OpName, leftType))
		}
		if !rightType.IsNumeric() {
			errs = append(errs, fmt.Errorf("right operand of %s is not numeric: %s", n.OpName, rightType))
		}
		// Result type should match promoted type
		promoted := PromoteType(leftType, rightType)
		if n.Typ != promoted {
			errs = append(errs, fmt.Errorf("result type mismatch: expected %s, got %s", promoted, n.Typ))
		}
	}

	// For comparison ops, result must be Bool
	switch n.OpName {
	case "==", "!=", "<", ">", "<=", ">=":
		if n.Typ != Bool {
			errs = append(errs, fmt.Errorf("comparison result type must be bool, got %s", n.Typ))
		}
	}

	// For logical ops, both operands must be Bool
	switch n.OpName {
	case "&&", "||":
		if leftType != Bool {
			errs = append(errs, fmt.Errorf("left operand of %s is not bool: %s", n.OpName, leftType))
		}
		if rightType != Bool {
			errs = append(errs, fmt.Errorf("right operand of %s is not bool: %s", n.OpName, rightType))
		}
		if n.Typ != Bool {
			errs = append(errs, fmt.Errorf("logical op result type must be bool, got %s", n.Typ))
		}
	}

	// Division by zero check (for constants)
	if n.OpName == "/" || n.OpName == "%" {
		if isZeroConst(n.Right) {
			errs = append(errs, fmt.Errorf("division by constant zero"))
		}
	}

	return errs
}

// validateUnaryOp checks a unary operation.
func validateUnaryOp(n *UnaryOp) []error {
	var errs []error

	if n.Operand == nil {
		errs = append(errs, fmt.Errorf("unary op %s has nil operand", n.OpName))
		return errs
	}

	switch n.OpName {
	case "-":
		// Negation requires numeric operand
		if !n.Operand.Type().IsNumeric() {
			errs = append(errs, fmt.Errorf("cannot negate non-numeric type: %s", n.Operand.Type()))
		}
	case "!":
		// Logical NOT requires bool operand
		if n.Operand.Type() != Bool {
			errs = append(errs, fmt.Errorf("logical NOT requires bool operand, got %s", n.Operand.Type()))
		}
	}

	return errs
}

// validateFuncCall checks a function call.
func validateFuncCall(n *FuncCall) []error {
	var errs []error

	sig, ok := LookupFunc(n.Name)
	if !ok {
		errs = append(errs, fmt.Errorf("unknown function: %s", n.Name))
		return errs
	}

	// Check argument count
	if len(n.Args) < sig.MinArgs {
		errs = append(errs, fmt.Errorf("function %s requires at least %d arguments, got %d", n.Name, sig.MinArgs, len(n.Args)))
	}
	if sig.MaxArgs >= 0 && len(n.Args) > sig.MaxArgs {
		errs = append(errs, fmt.Errorf("function %s accepts at most %d arguments, got %d", n.Name, sig.MaxArgs, len(n.Args)))
	}

	// Check argument types
	if sig.ArgTypes != nil {
		for i, arg := range n.Args {
			if arg == nil {
				errs = append(errs, fmt.Errorf("nil argument %d to function %s", i, n.Name))
				continue
			}
			valid := false
			for _, allowed := range sig.ArgTypes {
				if arg.Type() == allowed {
					valid = true
					break
				}
			}
			if !valid {
				allowedNames := make([]string, len(sig.ArgTypes))
				for j, t := range sig.ArgTypes {
					allowedNames[j] = t.String()
				}
				errs = append(errs, fmt.Errorf("argument %d to %s has type %s, expected one of [%s]",
					i, n.Name, arg.Type(), strings.Join(allowedNames, ", ")))
			}
		}
	}

	return errs
}

// validateLet checks a let binding.
func validateLet(n *Let) []error {
	var errs []error

	if n.Name == "" {
		errs = append(errs, fmt.Errorf("let binding with empty name"))
	}
	if n.Expr == nil {
		errs = append(errs, fmt.Errorf("let binding %s has nil expression", n.Name))
	}
	if n.Body == nil {
		errs = append(errs, fmt.Errorf("let binding %s has nil body", n.Name))
	}

	return errs
}

// ============================================================
// Helper functions
// ============================================================

func isNaN(f float64) bool {
	return f != f
}

func isInf(f float64) bool {
	return f > 1e308 || f < -1e308
}

func isZeroConst(node Node) bool {
	switch n := node.(type) {
	case *IntConst:
		return n.Value == 0
	case *FloatConst:
		return n.Value == 0.0
	}
	return false
}
