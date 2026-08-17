package codegen

import (
	"fmt"
	"karkain/pkg/parser"
	"strings"
)

type GPUGenerator struct {
	buf    strings.Builder
	errors []string
}

func NewGPUGenerator() *GPUGenerator {
	return &GPUGenerator{}
}

func (g *GPUGenerator) GenerateOpenCL(kernel *parser.KernelDeclStmt) (string, error) {
	g.buf.Reset()
	g.errors = nil

	g.buf.WriteString("__kernel void ")
	g.buf.WriteString(kernel.Name)
	g.buf.WriteString("(")

	params := []string{}
	for _, p := range kernel.Params {
		cType := g.mapKernelParamType(p.Type)
		params = append(params, fmt.Sprintf("__global %s* %s", cType, p.Name))
	}
	g.buf.WriteString(strings.Join(params, ", "))
	g.buf.WriteString(") {\n")

	for _, stmt := range kernel.Body {
		line := g.genKernelStmt(stmt)
		if line != "" {
			g.buf.WriteString("    " + line + "\n")
		}
	}

	g.buf.WriteString("}\n")

	if len(g.errors) > 0 {
		return "", fmt.Errorf("GPU codegen errors:\n%s", strings.Join(g.errors, "\n"))
	}
	return g.buf.String(), nil
}

func (g *GPUGenerator) genKernelStmt(stmt parser.Node) string {
	switch node := stmt.(type) {
	case *parser.VarDeclStmt:
		expr := g.genKernelExpr(node.Value)
		return fmt.Sprintf("%s %s = %s;", "int", node.Name, expr)
	case *parser.ExprStmt:
		return g.genKernelExpr(node.Expression) + ";"
	case *parser.BinaryExpr:
		return g.genKernelExpr(node) + ";"
	case *parser.BarrierStmt:
		return "barrier(CLK_LOCAL_MEM_FENCE);"
	default:
		g.errors = append(g.errors, fmt.Sprintf("unsupported statement type in kernel: %T", stmt))
		return ""
	}
}

func (g *GPUGenerator) genKernelExpr(node parser.Node) string {
	switch n := node.(type) {
	case *parser.IntLiteral:
		return n.Value
	case *parser.Float64Literal:
		return n.Value
	case *parser.Identifier:
		return n.Name
	case *parser.BinaryExpr:
		left := g.genKernelExpr(n.Left)
		right := g.genKernelExpr(n.Right)
		return fmt.Sprintf("%s %s %s", left, n.Operator, right)
	case *parser.GlobalIdExpr:
		return fmt.Sprintf("get_global_id(%d)", n.Dimension)
	case *parser.IndexExpr:
		left := g.genKernelExpr(n.Left)
		index := g.genKernelExpr(n.Index)
		return fmt.Sprintf("%s[%s]", left, index)
	case *parser.MatrixIndexExpr:
		matrix := g.genKernelExpr(n.Matrix)
		row := g.genKernelExpr(n.Row)
		return fmt.Sprintf("%s[%s]", matrix, row)
	case *parser.CallExpr:
		args := []string{}
		for _, arg := range n.Args {
			args = append(args, g.genKernelExpr(arg))
		}
		return fmt.Sprintf("%s(%s)", n.Function, strings.Join(args, ", "))
	default:
		g.errors = append(g.errors, fmt.Sprintf("unsupported expression type in kernel: %T", node))
		return "/* unsupported */"
	}
}

func (g *GPUGenerator) mapKernelParamType(kType string) string {
	switch kType {
	case "float", "float64":
		return "float"
	case "int":
		return "int"
	case "double":
		return "double"
	default:
		return "float"
	}
}
