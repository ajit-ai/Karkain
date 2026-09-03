package math

import (
	"fmt"
	"strings"
)

// Print returns a human-readable string representation of a Math IR node.
func Print(node Node) string {
	if node == nil {
		return "<nil>"
	}
	return printNode(node, 0)
}

func printNode(node Node, depth int) string {
	indent := strings.Repeat("  ", depth)
	switch n := node.(type) {
	case *IntConst:
		return fmt.Sprintf("%s%d : %s", indent, n.Value, n.Typ)
	case *FloatConst:
		return fmt.Sprintf("%s%g : %s", indent, n.Value, n.Typ)
	case *BoolConst:
		return fmt.Sprintf("%s%v : bool", indent, n.Value)
	case *StringConst:
		return fmt.Sprintf("%s%q : string", indent, n.Value)
	case *Variable:
		return fmt.Sprintf("%s%s : %s", indent, n.Name, n.Typ)
	case *Param:
		return fmt.Sprintf("%sparam %s : %s", indent, n.Name, n.Typ)
	case *BinaryOp:
		return fmt.Sprintf("%s%s [%s]\n%s\n%s",
			indent, n.OpName, n.Typ,
			printNode(n.Left, depth+1),
			printNode(n.Right, depth+1))
	case *UnaryOp:
		return fmt.Sprintf("%s%s [%s]\n%s",
			indent, n.OpName, n.Typ,
			printNode(n.Operand, depth+1))
	case *FuncCall:
		args := make([]string, len(n.Args))
		for i, arg := range n.Args {
			args[i] = printNode(arg, depth+1)
		}
		return fmt.Sprintf("%s%s [%s]\n%s",
			indent, n.Name, n.Typ,
			strings.Join(args, "\n"))
	case *Let:
		return fmt.Sprintf("%slet %s : %s\n%s\n%s",
			indent, n.Name, n.Typ,
			printNode(n.Expr, depth+1),
			printNode(n.Body, depth+1))
	default:
		return fmt.Sprintf("%s<unknown: %T>", indent, node)
	}
}

// PrintFlat returns a single-line expression string.
func PrintFlat(node Node) string {
	if node == nil {
		return "<nil>"
	}
	switch n := node.(type) {
	case *IntConst:
		return fmt.Sprintf("%d", n.Value)
	case *FloatConst:
		return fmt.Sprintf("%g", n.Value)
	case *BoolConst:
		if n.Value {
			return "true"
		}
		return "false"
	case *StringConst:
		return fmt.Sprintf("%q", n.Value)
	case *Variable:
		return n.Name
	case *Param:
		return n.Name
	case *BinaryOp:
		return fmt.Sprintf("(%s %s %s)", PrintFlat(n.Left), n.OpName, PrintFlat(n.Right))
	case *UnaryOp:
		return fmt.Sprintf("(%s%s)", n.OpName, PrintFlat(n.Operand))
	case *FuncCall:
		args := make([]string, len(n.Args))
		for i, arg := range n.Args {
			args[i] = PrintFlat(arg)
		}
		return fmt.Sprintf("%s(%s)", n.Name, strings.Join(args, ", "))
	case *Let:
		return fmt.Sprintf("let %s = %s in %s", n.Name, PrintFlat(n.Expr), PrintFlat(n.Body))
	default:
		return fmt.Sprintf("<?%T>", node)
	}
}

// PrintC returns a C23 expression string for the node.
func PrintC(node Node) string {
	if node == nil {
		return "/* nil */"
	}
	switch n := node.(type) {
	case *IntConst:
		return fmt.Sprintf("%d", n.Value)
	case *FloatConst:
		return fmt.Sprintf("%g", n.Value)
	case *BoolConst:
		if n.Value {
			return "1"
		}
		return "0"
	case *StringConst:
		return fmt.Sprintf("%q", n.Value)
	case *Variable:
		return n.Name
	case *Param:
		return n.Name
	case *BinaryOp:
		left := PrintC(n.Left)
		right := PrintC(n.Right)
		return fmt.Sprintf("(%s %s %s)", left, n.OpName, right)
	case *UnaryOp:
		operand := PrintC(n.Operand)
		if n.OpName == "!" {
			return fmt.Sprintf("(!%s)", operand)
		}
		return fmt.Sprintf("(-%s)", operand)
	case *FuncCall:
		args := make([]string, len(n.Args))
		for i, arg := range n.Args {
			args[i] = PrintC(arg)
		}
		// Map to C math functions
		cName := mapFuncToC(n.Name)
		return fmt.Sprintf("%s(%s)", cName, strings.Join(args, ", "))
	case *Let:
		// C doesn't have let; inline the expression
		return PrintC(n.Body)
	default:
		return fmt.Sprintf("/* unknown:%T */", node)
	}
}

// mapFuncToC maps Karkain math function names to C math.h equivalents.
func mapFuncToC(name string) string {
	switch name {
	case "sqrt":
		return "sqrt"
	case "exp":
		return "exp"
	case "log":
		return "log"
	case "log2":
		return "log2"
	case "log10":
		return "log10"
	case "sin":
		return "sin"
	case "cos":
		return "cos"
	case "tan":
		return "tan"
	case "asin":
		return "asin"
	case "acos":
		return "acos"
	case "atan":
		return "atan"
	case "atan2":
		return "atan2"
	case "abs":
		return "abs"
	case "floor":
		return "floor"
	case "ceil":
		return "ceil"
	case "round":
		return "round"
	case "pow":
		return "pow"
	case "min":
		return "fmin"
	case "max":
		return "fmax"
	case "sign":
		return "signbit"
	default:
		return name
	}
}

// PrintGraph prints an entire graph in a readable format.
func PrintGraph(g *Graph) string {
	var sb strings.Builder
	sb.WriteString("=== Math IR Graph ===\n")
	sb.WriteString(fmt.Sprintf("Nodes: %d, Inputs: %d, Outputs: %d\n\n", g.NumNodes(), len(g.Inputs), len(g.Outputs)))

	for _, input := range g.Inputs {
		sb.WriteString(fmt.Sprintf("Input: %s : %s\n", input.Name, input.Typ))
	}
	sb.WriteString("\n")

	for name, node := range g.Nodes {
		sb.WriteString(fmt.Sprintf("%s = %s\n", name, PrintFlat(node)))
	}

	if len(g.Outputs) > 0 {
		sb.WriteString("\nOutputs:\n")
		for _, out := range g.Outputs {
			sb.WriteString(fmt.Sprintf("  %s\n", PrintFlat(out)))
		}
	}

	return sb.String()
}
