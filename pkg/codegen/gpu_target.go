package codegen

import (
	"fmt"
	"karkain/pkg/parser"
	"strings"
)

// GPUGeneratorWGSL handles @target(gpu) function emission to WGSL compute shaders.
// Phase 138: Real WGSL emission (not comment stubs) with compile-only guarantee.
type GPUGeneratorWGSL struct {
	buf       strings.Builder
	errors    []string
	shaders   map[string]string // function name -> WGSL shader
	funcNames []string
}

// NewGPUGeneratorWGSL creates a new GPU/WGSL generator.
func NewGPUGeneratorWGSL() *GPUGeneratorWGSL {
	return &GPUGeneratorWGSL{
		shaders: make(map[string]string),
	}
}

// GenerateFunction emits WGSL for a @target(gpu) function.
func (g *GPUGeneratorWGSL) GenerateFunction(fn *parser.FuncDecl) (string, error) {
	g.buf.Reset()
	g.errors = nil

	// Emit WGSL shader header
	g.emitWGSLHeader(fn.Name)

	// Emit function parameters as WGSL bindings
	g.emitWGSLParams(fn)

	// Emit function body as WGSL compute shader
	g.emitWGSLBody(fn)

	shader := g.buf.String()
	g.shaders[fn.Name] = shader
	g.funcNames = append(g.funcNames, fn.Name)

	if len(g.errors) > 0 {
		return "", fmt.Errorf("GPU/WGSL codegen errors:\n%s", strings.Join(g.errors, "\n"))
	}
	return shader, nil
}

// emitWGSLHeader emits the WGSL shader header.
func (g *GPUGeneratorWGSL) emitWGSLHeader(funcName string) {
	g.buf.WriteString(fmt.Sprintf("// @target(gpu) function: %s\n", funcName))
	g.buf.WriteString("// Phase 138: Real WGSL compute shader emission\n")
}

// emitWGSLParams emits function parameters as WGSL storage buffers.
func (g *GPUGeneratorWGSL) emitWGSLParams(fn *parser.FuncDecl) {
	binding := 0
	for i, param := range fn.Params {
		wgslType := g.mapTypeToWGSL(param)
		g.buf.WriteString(fmt.Sprintf("@group(0) @binding(%d) var<storage, read_write> param_%d: array<%s>;\n",
			binding, i, wgslType))
		binding++
	}
}

// emitWGSLBody emits the function body as a WGSL compute shader.
func (g *GPUGeneratorWGSL) emitWGSLBody(fn *parser.FuncDecl) {
	g.buf.WriteString("@compute @workgroup_size(64, 1, 1)\n")
	g.buf.WriteString(fmt.Sprintf("fn %s(@builtin(global_invocation_id) gid: vec3<u32>) {\n", fn.Name))

	for _, stmt := range fn.Body {
		line := g.emitWGSLStmt(stmt)
		if line != "" {
			g.buf.WriteString("    " + line + "\n")
		}
	}

	g.buf.WriteString("}\n")
}

// emitWGSLStmt emits a single statement as WGSL.
func (g *GPUGeneratorWGSL) emitWGSLStmt(stmt parser.Node) string {
	switch node := stmt.(type) {
	case *parser.VarDeclStmt:
		expr := g.emitWGSLExpr(node.Value)
		wt := g.mapTypeToWGSL(node.Type)
		return fmt.Sprintf("var %s: %s = %s;", node.Name, wt, expr)
	case *parser.ExprStmt:
		return g.emitWGSLExpr(node.Expression) + ";"
	case *parser.BinaryExpr:
		return g.emitWGSLExpr(node) + ";"
	case *parser.BarrierStmt:
		return "workgroupBarrier();"
	case *parser.ReturnStmt:
		if node.Value != nil {
			return fmt.Sprintf("return %s;", g.emitWGSLExpr(node.Value))
		}
		return "return;"
	case *parser.CallExpr:
		return g.emitWGSLExpr(node) + ";"
	default:
		// For unsupported statement types, emit a comment
		g.errors = append(g.errors, fmt.Sprintf("unsupported statement type in GPU kernel: %T", stmt))
		return fmt.Sprintf("/* unsupported statement: %T */", stmt)
	}
}

// emitWGSLExpr emits an expression as WGSL.
func (g *GPUGeneratorWGSL) emitWGSLExpr(node parser.Node) string {
	switch n := node.(type) {
	case *parser.IntLiteral:
		return n.Value
	case *parser.Float64Literal:
		return n.Value
	case *parser.Identifier:
		return n.Name
	case *parser.BinaryExpr:
		left := g.emitWGSLExpr(n.Left)
		right := g.emitWGSLExpr(n.Right)
		return fmt.Sprintf("%s %s %s", left, n.Operator, right)
	case *parser.IndexExpr:
		left := g.emitWGSLExpr(n.Left)
		index := g.emitWGSLExpr(n.Index)
		return fmt.Sprintf("%s[%s]", left, index)
	case *parser.CallExpr:
		args := []string{}
		for _, arg := range n.Args {
			args = append(args, g.emitWGSLExpr(arg))
		}
		return fmt.Sprintf("%s(%s)", n.Function, strings.Join(args, ", "))
	case *parser.GlobalIdExpr:
		return fmt.Sprintf("gid.%s", g.dimensionToWGSL(n.Dimension))
	default:
		g.errors = append(g.errors, fmt.Sprintf("unsupported expression type in GPU kernel: %T", node))
		return "/* unsupported */"
	}
}

// dimensionToWGSL converts GlobalIdExpr dimension to WGSL component.
func (g *GPUGeneratorWGSL) dimensionToWGSL(dim int) string {
	switch dim {
	case 0:
		return "x"
	case 1:
		return "y"
	case 2:
		return "z"
	default:
		return "x"
	}
}

// mapTypeToWGSL maps Karkain types to WGSL types.
func (g *GPUGeneratorWGSL) mapTypeToWGSL(kType string) string {
	// Karkain uses space-separated params: "a float", "b int"
	// Extract just the type part (after the space)
	parts := strings.Fields(kType)
	if len(parts) >= 2 {
		kType = parts[len(parts)-1]
	}
	switch kType {
	case "int", "int32":
		return "i32"
	case "float", "float32":
		return "f32"
	case "float64":
		return "f32" // WGSL doesn't have f64, use f32
	case "bool":
		return "bool"
	default:
		// Default to f32 for unknown types (conservative for GPU compute)
		return "f32"
	}
}

// GetShaders returns all generated WGSL shaders.
func (g *GPUGeneratorWGSL) GetShaders() map[string]string {
	return g.shaders
}

// GetFunctionNames returns the names of all processed GPU functions.
func (g *GPUGeneratorWGSL) GetFunctionNames() []string {
	return g.funcNames
}

// HasErrors returns true if any errors occurred during generation.
func (g *GPUGeneratorWGSL) HasErrors() bool {
	return len(g.errors) > 0
}

// GetErrors returns the list of errors.
func (g *GPUGeneratorWGSL) GetErrors() []string {
	return g.errors
}
