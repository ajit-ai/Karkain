package codegen

import (
	"fmt"
	"karkain/pkg/parser"
	"strings"
)

type WGSLGenerator struct {
	buf         strings.Builder
	errors      []string
	bindingIdx  int
	groupIdx    int
	paramTypes  map[string]string
}

func NewWGSLGenerator() *WGSLGenerator {
	return &WGSLGenerator{
		paramTypes: make(map[string]string),
	}
}

func (g *WGSLGenerator) GenerateWGSL(kernel *parser.KernelDeclStmt) (string, error) {
	g.buf.Reset()
	g.errors = nil
	g.bindingIdx = 0
	g.paramTypes = make(map[string]string)

	for _, p := range kernel.Params {
		g.paramTypes[p.Name] = p.Type
	}

	workgroupX := kernel.WorkGroupX
	if workgroupX == 0 {
		workgroupX = 64
	}
	workgroupY := kernel.WorkGroupY
	if workgroupY == 0 {
		workgroupY = 1
	}
	workgroupZ := kernel.WorkGroupZ
	if workgroupZ == 0 {
		workgroupZ = 1
	}

	for _, p := range kernel.Params {
		wgslType := g.mapWGSLType(p.Type)
		g.buf.WriteString(fmt.Sprintf("@group(%d) @binding(%d) var<storage, read_write> %s: array<%s>;\n",
			g.groupIdx, g.bindingIdx, p.Name, wgslType))
		g.bindingIdx++
	}

	if len(kernel.Params) > 0 {
		g.buf.WriteString("\n")
	}

	g.buf.WriteString(fmt.Sprintf("@compute @workgroup_size(%d, %d, %d)\n", workgroupX, workgroupY, workgroupZ))
	g.buf.WriteString(fmt.Sprintf("fn %s(", kernel.Name))

	params := []string{}
	for _, p := range kernel.Params {
		params = append(params, fmt.Sprintf("%s: u32", p.Name+"_idx"))
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
		return "", fmt.Errorf("WGSL codegen errors:\n%s", strings.Join(g.errors, "\n"))
	}
	return g.buf.String(), nil
}

func (g *WGSLGenerator) genKernelStmt(stmt parser.Node) string {
	switch node := stmt.(type) {
	case *parser.VarDeclStmt:
		expr := g.genKernelExpr(node.Value)
		return fmt.Sprintf("var %s: i32 = %s;", node.Name, expr)
	case *parser.ExprStmt:
		return g.genKernelExpr(node.Expression) + ";"
	case *parser.BinaryExpr:
		return g.genKernelExpr(node) + ";"
	case *parser.BarrierStmt:
		return "workgroupBarrier();"
	default:
		g.errors = append(g.errors, fmt.Sprintf("unsupported statement type in WGSL kernel: %T", stmt))
		return ""
	}
}

func (g *WGSLGenerator) genKernelExpr(node parser.Node) string {
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
		op := n.Operator
		if op == "=" {
			return fmt.Sprintf("%s = %s", left, right)
		}
		return fmt.Sprintf("%s %s %s", left, op, right)
	case *parser.GlobalIdExpr:
		return g.mapGlobalId(n.Dimension)
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
		g.errors = append(g.errors, fmt.Sprintf("unsupported expression type in WGSL kernel: %T", node))
		return "/* unsupported */"
	}
}

func (g *WGSLGenerator) mapGlobalId(dimension int) string {
	switch dimension {
	case 0:
		return "global_id.x"
	case 1:
		return "global_id.y"
	case 2:
		return "global_id.z"
	default:
		return "global_id.x"
	}
}

func (g *WGSLGenerator) mapWGSLType(kType string) string {
	// Strip array prefix if present (e.g., []int -> int)
	elemType := kType
	if len(elemType) > 2 && elemType[:2] == "[]" {
		elemType = elemType[2:]
	}

	switch elemType {
	case "float", "float64":
		return "f32"
	case "int":
		return "i32"
	case "double":
		return "f64"
	default:
		return "f32"
	}
}
