package codegen

import (
	"fmt"
	"karkain/pkg/sema"
	"strings"
)

// TensorWGSLGenerator generates WGSL compute shaders from autodiff computation graphs
type TensorWGSLGenerator struct {
	buf       strings.Builder
	errors    []string
	bindingIdx int
	groupIdx   int
}

// NewTensorWGSLGenerator creates a new tensor WGSL generator
func NewTensorWGSLGenerator() *TensorWGSLGenerator {
	return &TensorWGSLGenerator{}
}

// GenerateMatmulKernel generates a WGSL compute shader for matrix multiplication
// A[M,K] x B[K,N] -> C[M,N]
func (g *TensorWGSLGenerator) GenerateMatmulKernel(
	name string,
	M, K, N int,
	elemType string,
) (string, error) {
	g.buf.Reset()
	g.errors = nil
	g.bindingIdx = 0

	wgslType := mapTensorElemType(elemType)

	// Buffer declarations
	g.buf.WriteString(fmt.Sprintf("@group(0) @binding(0) var<storage, read> A: array<%s>;\n", wgslType))
	g.buf.WriteString(fmt.Sprintf("@group(0) @binding(1) var<storage, read> B: array<%s>;\n", wgslType))
	g.buf.WriteString(fmt.Sprintf("@group(0) @binding(2) var<storage, read_write> C: array<%s>;\n", wgslType))
	g.buf.WriteString("\n")

	// Uniform parameters
	g.buf.WriteString(fmt.Sprintf("struct Params { M: u32, K: u32, N: u32 };\n"))
	g.buf.WriteString(fmt.Sprintf("@group(1) @binding(0) var<uniform> params: Params;\n"))
	g.buf.WriteString("\n")

	// Workgroup tile dimensions
	g.buf.WriteString("const TILE: u32 = 16u;\n")
	g.buf.WriteString(fmt.Sprintf("var<workgroup> tileA: array<array<%s, 16>, 16>;\n", wgslType))
	g.buf.WriteString(fmt.Sprintf("var<workgroup> tileB: array<array<%s, 16>, 16>;\n", wgslType))
	g.buf.WriteString("\n")

	// Compute shader
	g.buf.WriteString("@compute @workgroup_size(16, 16, 1)\n")
	g.buf.WriteString(fmt.Sprintf("fn %s(@builtin(global_invocation_id) gid: vec3<u32>,\n", name))
	g.buf.WriteString("      @builtin(local_invocation_id) lid: vec3<u32>) {\n")
	g.buf.WriteString("    let row = gid.y;\n")
	g.buf.WriteString("    let col = gid.x;\n")
	g.buf.WriteString("    let numTiles = (params.K + TILE - 1u) / TILE;\n")
	g.buf.WriteString("\n")
	g.buf.WriteString("    var sum: " + wgslType + " = 0.0;\n")
	g.buf.WriteString("\n")
	g.buf.WriteString("    for (var t: u32 = 0u; t < numTiles; t = t + 1u) {\n")
	g.buf.WriteString("        let tileCol = t * TILE + lid.x;\n")
	g.buf.WriteString("        let tileRow = t * TILE + lid.y;\n")
	g.buf.WriteString("\n")
	g.buf.WriteString("        if (row < params.M && tileCol < params.K) {\n")
	g.buf.WriteString(fmt.Sprintf("            tileA[lid.y][lid.x] = A[row * params.K + tileCol];\n"))
	g.buf.WriteString("        } else {\n")
	g.buf.WriteString(fmt.Sprintf("            tileA[lid.y][lid.x] = 0.0;\n"))
	g.buf.WriteString("        }\n")
	g.buf.WriteString("\n")
	g.buf.WriteString("        if (tileRow < params.K && col < params.N) {\n")
	g.buf.WriteString(fmt.Sprintf("            tileB[lid.y][lid.x] = B[tileRow * params.N + col];\n"))
	g.buf.WriteString("        } else {\n")
	g.buf.WriteString(fmt.Sprintf("            tileB[lid.y][lid.x] = 0.0;\n"))
	g.buf.WriteString("        }\n")
	g.buf.WriteString("\n")
	g.buf.WriteString("        workgroupBarrier();\n")
	g.buf.WriteString("\n")
	g.buf.WriteString("        for (var k: u32 = 0u; k < TILE; k = k + 1u) {\n")
	g.buf.WriteString("            sum = sum + tileA[lid.y][k] * tileB[k][lid.x];\n")
	g.buf.WriteString("        }\n")
	g.buf.WriteString("\n")
	g.buf.WriteString("        workgroupBarrier();\n")
	g.buf.WriteString("    }\n")
	g.buf.WriteString("\n")
	g.buf.WriteString("    if (row < params.M && col < params.N) {\n")
	g.buf.WriteString("        C[row * params.N + col] = sum;\n")
	g.buf.WriteString("    }\n")
	g.buf.WriteString("}\n")

	if len(g.errors) > 0 {
		return "", fmt.Errorf("tensor WGSL errors: %s", strings.Join(g.errors, "\n"))
	}
	return g.buf.String(), nil
}

// GenerateReluKernel generates a WGSL compute shader for element-wise ReLU
func (g *TensorWGSLGenerator) GenerateReluKernel(name string, elemType string) string {
	wgslType := mapTensorElemType(elemType)

	g.buf.Reset()
	g.buf.WriteString(fmt.Sprintf("@group(0) @binding(0) var<storage, read> input: array<%s>;\n", wgslType))
	g.buf.WriteString(fmt.Sprintf("@group(0) @binding(1) var<storage, read_write> output: array<%s>;\n", wgslType))
	g.buf.WriteString(fmt.Sprintf("@group(1) @binding(0) var<uniform> size: u32;\n"))
	g.buf.WriteString("\n")
	g.buf.WriteString("@compute @workgroup_size(64, 1, 1)\n")
	g.buf.WriteString(fmt.Sprintf("fn %s(@builtin(global_invocation_id) gid: vec3<u32>) {\n", name))
	g.buf.WriteString("    let idx = gid.x;\n")
	g.buf.WriteString("    if (idx >= size) { return; }\n")
	g.buf.WriteString(fmt.Sprintf("    output[idx] = max(input[idx], 0.0);\n"))
	g.buf.WriteString("}\n")

	return g.buf.String()
}

// GenerateTensorGraph generates WGSL for a full autodiff computation graph
func (g *TensorWGSLGenerator) GenerateTensorGraph(graph *sema.ADGraph) (string, error) {
	g.buf.Reset()
	g.errors = nil
	g.bindingIdx = 0

	if len(graph.Outputs) == 0 {
		return "", fmt.Errorf("no output nodes in graph")
	}

	// Emit input buffer declarations
	for i, input := range graph.Inputs {
		wgslType := mapTensorElemType("f32")
		g.buf.WriteString(fmt.Sprintf("@group(0) @binding(%d) var<storage, read> %s: array<%s>; // input shape %s\n",
			i, input.Name, wgslType, input.Shape.String()))
		g.bindingIdx++
	}

	g.buf.WriteString("\n")

	// Emit intermediate storage for each non-input, non-output op node
	outputNode := graph.Outputs[0]
	g.buf.WriteString(fmt.Sprintf("@group(0) @binding(%d) var<storage, read_write> output: array<f32>; // output shape %s\n",
		g.bindingIdx, outputNode.Shape.String()))
	g.bindingIdx++

	g.buf.WriteString("\n")

	// Emit compute function
	g.buf.WriteString("@compute @workgroup_size(64, 1, 1)\n")
	g.buf.WriteString("fn tensor_graph_main(@builtin(global_invocation_id) gid: vec3<u32>) {\n")
	g.buf.WriteString("    let idx = gid.x;\n")

	// Walk nodes in forward order, emitting operations
	for _, node := range graph.Nodes {
		if node.Op == "input" {
			continue
		}
		g.emitNodeOp(node)
	}

	g.buf.WriteString("}\n")

	if len(g.errors) > 0 {
		return "", fmt.Errorf("tensor graph WGSL errors: %s", strings.Join(g.errors, "\n"))
	}
	return g.buf.String(), nil
}

// emitNodeOp emits WGSL code for a single DAG node
func (g *TensorWGSLGenerator) emitNodeOp(node *sema.ADNode) {
	switch node.Op {
	case "matmul":
		g.buf.WriteString(fmt.Sprintf("    // matmul: %s shape %s\n", node.Name, node.Shape.String()))
		g.buf.WriteString(fmt.Sprintf("    // inputs: %s, %s\n",
			node.Inputs[0].Name, node.Inputs[1].Name))
	case "relu":
		g.buf.WriteString(fmt.Sprintf("    // relu: %s (element-wise)\n", node.Name))
	case "softmax":
		g.buf.WriteString(fmt.Sprintf("    // softmax: %s (axis=%d)\n", node.Name, node.Metadata["axis"]))
	case "conv2d":
		g.buf.WriteString(fmt.Sprintf("    // conv2d: %s shape %s\n", node.Name, node.Shape.String()))
	case "transpose":
		g.buf.WriteString(fmt.Sprintf("    // transpose: %s (dims %d,%d)\n",
			node.Name, node.Metadata["dim0"], node.Metadata["dim1"]))
	case "matmul_grad_a", "matmul_grad_b":
		g.buf.WriteString(fmt.Sprintf("    // gradient: %s shape %s\n", node.Name, node.Shape.String()))
	case "relu_grad":
		g.buf.WriteString(fmt.Sprintf("    // relu_grad: %s\n", node.Name))
	case "softmax_grad":
		g.buf.WriteString(fmt.Sprintf("    // softmax_grad: %s\n", node.Name))
	case "conv2d_grad_input", "conv2d_grad_kernel":
		g.buf.WriteString(fmt.Sprintf("    // conv2d gradient: %s\n", node.Name))
	case "transpose_grad":
		g.buf.WriteString(fmt.Sprintf("    // transpose_grad: %s\n", node.Name))
	default:
		g.errors = append(g.errors, fmt.Sprintf("unknown op in tensor graph: %s", node.Op))
	}
}

// mapTensorElemType maps element type strings to WGSL type strings
func mapTensorElemType(elemType string) string {
	switch strings.ToLower(elemType) {
	case "f16":
		return "f32" // WGSL doesn't have f16 in all implementations, use f32
	case "f32", "float", "float32":
		return "f32"
	case "f64", "float64", "double":
		return "f64"
	case "i32", "int":
		return "i32"
	case "u32", "uint":
		return "u32"
	default:
		return "f32"
	}
}
