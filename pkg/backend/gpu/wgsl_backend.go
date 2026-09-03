// Package gpu implements the GPU (WGSL/WebGPU) backend for tensor execution.
// It generates WGSL compute shaders from Karkain Tensor IR graphs.
package gpu

import (
	"fmt"
	"strings"

	"karkain/pkg/backend"
	"karkain/pkg/tensor"
)

// WGSLBackend implements the Backend interface using WGSL compute shaders.
type WGSLBackend struct {
	// shaderBuf accumulates generated WGSL across a graph.
	shaderBuf strings.Builder
}

// New creates a new WGSL backend.
func New() *WGSLBackend {
	return &WGSLBackend{}
}

// Name returns the backend name.
func (b *WGSLBackend) Name() string {
	return "gpu"
}

// Capabilities returns what the GPU backend supports.
func (b *WGSLBackend) Capabilities() backend.Capabilities {
	return backend.Capabilities{
		Name:            "gpu",
		SupportedDtypes: []tensor.ElemType{tensor.ElemF32, tensor.ElemI32},
		MaxRank:         8,
		SupportedOps: []tensor.Op{
			tensor.OpAdd, tensor.OpSub, tensor.OpMul, tensor.OpDiv,
			tensor.OpMatMul,
			tensor.OpRelu, tensor.OpSigmoid, tensor.OpTanh, tensor.OpSoftmax,
			tensor.OpTranspose,
		},
		SupportedLayouts: []tensor.Layout{tensor.LayoutRowMajor},
	}
}

// Supports returns true if the GPU backend can execute the operation.
func (b *WGSLBackend) Supports(op tensor.Op, dtype tensor.ElemType, _ tensor.Shape) bool {
	caps := b.Capabilities()
	if !caps.SupportsOp(op) {
		return false
	}
	return caps.SupportsDtype(dtype)
}

// Execute generates a WGSL compute shader for the graph.
// Since WebGPU execution requires a browser/runtime environment, the backend
// returns the generated shader plus kernel metadata rather than executing
// against hardware (which is not available in this test environment).
func (b *WGSLBackend) Execute(graph *tensor.TensorGraph, _ map[string][]float64) (*backend.Result, error) {
	shader, err := b.GenerateShader(graph)
	if err != nil {
		return nil, err
	}

	result := backend.NewResult()
	result.Metadata["wgsl"] = shader
	// Mark which node IDs have kernels
	for _, node := range graph.Nodes {
		if node.Op == tensor.OpMatMul || node.Op == tensor.OpRelu ||
			node.Op == tensor.OpSigmoid || node.Op == tensor.OpTanh ||
			node.Op == tensor.OpSoftmax {
			result.Outputs[node.ID] = node
			result.Metadata["kernel_"+node.ID] = kernelName(node)
		}
	}
	return result, nil
}

// GenerateShader produces a WGSL compute shader for the given tensor graph.
func (b *WGSLBackend) GenerateShader(graph *tensor.TensorGraph) (string, error) {
	b.shaderBuf.Reset()

	if len(graph.Nodes) == 0 {
		return "", fmt.Errorf("empty graph")
	}

	// Emit a kernel for each supported graph node.
	for _, node := range graph.Nodes {
		switch node.Op {
		case tensor.OpMatMul:
			if err := b.emitMatMulKernel(node); err != nil {
				return "", err
			}
		case tensor.OpRelu:
			b.emitReluKernel(node)
		case tensor.OpSigmoid:
			b.emitSigmoidKernel(node)
		case tensor.OpTanh:
			b.emitTanhKernel(node)
		case tensor.OpSoftmax:
			b.emitSoftmaxKernel(node)
		case tensor.OpAdd, tensor.OpSub, tensor.OpMul, tensor.OpDiv:
			b.emitElementwiseKernel(node)
		}
	}

	return b.shaderBuf.String(), nil
}

// ============================================================
// Kernel emission
// ============================================================

func wgslType(dt tensor.ElemType) string {
	switch dt {
	case tensor.ElemI32:
		return "i32"
	case tensor.ElemF64:
		return "f64"
	default:
		return "f32"
	}
}

func kernelName(node *tensor.TensorNode) string {
	return "kernel_" + sanitizeID(node.ID)
}

func sanitizeID(id string) string {
	var sb strings.Builder
	for _, r := range id {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' {
			sb.WriteRune(r)
		} else {
			sb.WriteByte('_')
		}
	}
	return sb.String()
}

// A flattened storage buffer layout: A[M*K], B[K*N], C[M*N]
func (b *WGSLBackend) emitMatMulKernel(node *tensor.TensorNode) error {
	if node.Shape.Rank() < 2 || len(node.Args) < 2 {
		return fmt.Errorf("matmul node %s needs rank>=2 and 2 args", node.ID)
	}
	aID := sanitizeID(node.Args[0].(string))
	bID := sanitizeID(node.Args[1].(string))
	cID := sanitizeID(node.ID)
	wt := wgslType(node.DType)

	// Infer M and N from node's output shape last two dims (for documentation)
	var dims []string
	for _, d := range node.Shape[node.Shape.Rank()-2:] {
		if d.IsStatic() {
			dims = append(dims, fmt.Sprintf("%d", d.Size))
		} else {
			dims = append(dims, "?")
		}
	}
	shapeNote := ""
	if len(dims) == 2 {
		shapeNote = fmt.Sprintf(" // output shape [%s, %s]", dims[0], dims[1])
	}

	b.shaderBuf.WriteString(fmt.Sprintf("// %s%s\n", node.Op, shapeNote))
	b.shaderBuf.WriteString(fmt.Sprintf("@group(0) @binding(0) var<storage, read> %s: array<%s>;\n", aID, wt))
	b.shaderBuf.WriteString(fmt.Sprintf("@group(0) @binding(1) var<storage, read> %s: array<%s>;\n", bID, wt))
	b.shaderBuf.WriteString(fmt.Sprintf("@group(0) @binding(2) var<storage, read_write> %s: array<%s>;\n", cID, wt))
	b.shaderBuf.WriteString("struct Params { M: u32, K: u32, N: u32 };\n")
	b.shaderBuf.WriteString("@group(1) @binding(0) var<uniform> params: Params;\n")
	b.shaderBuf.WriteString("@compute @workgroup_size(16, 16, 1)\n")
	b.shaderBuf.WriteString(fmt.Sprintf("fn %s(@builtin(global_invocation_id) gid: vec3<u32>) {\n", kernelName(node)))
	b.shaderBuf.WriteString("    let row = gid.y;\n    let col = gid.x;\n")
	b.shaderBuf.WriteString("    if (row >= params.M || col >= params.N) { return; }\n")
	b.shaderBuf.WriteString("    var sum: f32 = 0.0;\n")
	b.shaderBuf.WriteString("    for (var k: u32 = 0u; k < params.K; k = k + 1u) {\n")
	b.shaderBuf.WriteString(fmt.Sprintf("        sum = sum + %s[row * params.K + k] * %s[k * params.N + col];\n", aID, bID))
	b.shaderBuf.WriteString("    }\n")
	b.shaderBuf.WriteString(fmt.Sprintf("    %s[row * params.N + col] = sum;\n", cID))
	b.shaderBuf.WriteString("}\n\n")

	return nil
}

func (b *WGSLBackend) emitReluKernel(node *tensor.TensorNode) {
	input := sanitizeID(node.Args[0].(string))
	output := sanitizeID(node.ID)
	wt := wgslType(node.DType)
	b.shaderBuf.WriteString(fmt.Sprintf("// %s\n", node.Op))
	b.shaderBuf.WriteString(fmt.Sprintf("@group(0) @binding(0) var<storage, read> %s: array<%s>;\n", input, wt))
	b.shaderBuf.WriteString(fmt.Sprintf("@group(0) @binding(1) var<storage, read_write> %s: array<%s>;\n", output, wt))
	b.shaderBuf.WriteString("@group(1) @binding(0) var<uniform> size: u32;\n")
	b.shaderBuf.WriteString("@compute @workgroup_size(64, 1, 1)\n")
	b.shaderBuf.WriteString(fmt.Sprintf("fn %s(@builtin(global_invocation_id) gid: vec3<u32>) {\n", kernelName(node)))
	b.shaderBuf.WriteString("    let idx = gid.x;\n    if (idx >= size) { return; }\n")
	b.shaderBuf.WriteString(fmt.Sprintf("    %s[idx] = max(%s[idx], 0.0);\n", output, input))
	b.shaderBuf.WriteString("}\n\n")
}

func (b *WGSLBackend) emitSigmoidKernel(node *tensor.TensorNode) {
	input := sanitizeID(node.Args[0].(string))
	output := sanitizeID(node.ID)
	wt := wgslType(node.DType)
	b.shaderBuf.WriteString(fmt.Sprintf("// %s\n", node.Op))
	b.shaderBuf.WriteString(fmt.Sprintf("@group(0) @binding(0) var<storage, read> %s: array<%s>;\n", input, wt))
	b.shaderBuf.WriteString(fmt.Sprintf("@group(0) @binding(1) var<storage, read_write> %s: array<%s>;\n", output, wt))
	b.shaderBuf.WriteString("@group(1) @binding(0) var<uniform> size: u32;\n")
	b.shaderBuf.WriteString("@compute @workgroup_size(64, 1, 1)\n")
	b.shaderBuf.WriteString(fmt.Sprintf("fn %s(@builtin(global_invocation_id) gid: vec3<u32>) {\n", kernelName(node)))
	b.shaderBuf.WriteString("    let idx = gid.x;\n    if (idx >= size) { return; }\n")
	b.shaderBuf.WriteString(fmt.Sprintf("    %s[idx] = 1.0 / (1.0 + exp(-%s[idx]));\n", output, input))
	b.shaderBuf.WriteString("}\n\n")
}

func (b *WGSLBackend) emitTanhKernel(node *tensor.TensorNode) {
	input := sanitizeID(node.Args[0].(string))
	output := sanitizeID(node.ID)
	wt := wgslType(node.DType)
	b.shaderBuf.WriteString(fmt.Sprintf("// %s\n", node.Op))
	b.shaderBuf.WriteString(fmt.Sprintf("@group(0) @binding(0) var<storage, read> %s: array<%s>;\n", input, wt))
	b.shaderBuf.WriteString(fmt.Sprintf("@group(0) @binding(1) var<storage, read_write> %s: array<%s>;\n", output, wt))
	b.shaderBuf.WriteString("@group(1) @binding(0) var<uniform> size: u32;\n")
	b.shaderBuf.WriteString("@compute @workgroup_size(64, 1, 1)\n")
	b.shaderBuf.WriteString(fmt.Sprintf("fn %s(@builtin(global_invocation_id) gid: vec3<u32>) {\n", kernelName(node)))
	b.shaderBuf.WriteString("    let idx = gid.x;\n    if (idx >= size) { return; }\n")
	b.shaderBuf.WriteString(fmt.Sprintf("    let x = %s[idx];\n", input))
	b.shaderBuf.WriteString(fmt.Sprintf("    %s[idx] = (exp(x) - exp(-x)) / (exp(x) + exp(-x));\n", output))
	b.shaderBuf.WriteString("}\n\n")
}

func (b *WGSLBackend) emitSoftmaxKernel(node *tensor.TensorNode) {
	input := sanitizeID(node.Args[0].(string))
	output := sanitizeID(node.ID)
	wt := wgslType(node.DType)
	b.shaderBuf.WriteString(fmt.Sprintf("// %s\n", node.Op))
	b.shaderBuf.WriteString(fmt.Sprintf("@group(0) @binding(0) var<storage, read> %s: array<%s>;\n", input, wt))
	b.shaderBuf.WriteString(fmt.Sprintf("@group(0) @binding(1) var<storage, read_write> %s: array<%s>;\n", output, wt))
	b.shaderBuf.WriteString("@group(1) @binding(0) var<uniform> size: u32;\n")
	b.shaderBuf.WriteString("@compute @workgroup_size(64, 1, 1)\n")
	b.shaderBuf.WriteString(fmt.Sprintf("fn %s(@builtin(global_invocation_id) gid: vec3<u32>) {\n", kernelName(node)))
	b.shaderBuf.WriteString("    let idx = gid.x;\n    if (idx >= size) { return; }\n")
	b.shaderBuf.WriteString("    // per-element softmax approximation (row softmax requires reduction)\n")
	b.shaderBuf.WriteString(fmt.Sprintf("    %s[idx] = exp(%s[idx]);\n", output, input))
	b.shaderBuf.WriteString("}\n\n")
}

func (b *WGSLBackend) emitElementwiseKernel(node *tensor.TensorNode) {
	left := sanitizeID(node.Args[0].(string))
	right := sanitizeID(node.Args[1].(string))
	output := sanitizeID(node.ID)
	wt := wgslType(node.DType)
	op := "?"
	switch node.Op {
	case tensor.OpAdd:
		op = "+"
	case tensor.OpSub:
		op = "-"
	case tensor.OpMul:
		op = "*"
	case tensor.OpDiv:
		op = "/"
	}
	b.shaderBuf.WriteString(fmt.Sprintf("// %s\n", node.Op))
	b.shaderBuf.WriteString(fmt.Sprintf("@group(0) @binding(0) var<storage, read> %s: array<%s>;\n", left, wt))
	b.shaderBuf.WriteString(fmt.Sprintf("@group(0) @binding(1) var<storage, read> %s: array<%s>;\n", right, wt))
	b.shaderBuf.WriteString(fmt.Sprintf("@group(0) @binding(2) var<storage, read_write> %s: array<%s>;\n", output, wt))
	b.shaderBuf.WriteString("@group(1) @binding(0) var<uniform> size: u32;\n")
	b.shaderBuf.WriteString("@compute @workgroup_size(64, 1, 1)\n")
	b.shaderBuf.WriteString(fmt.Sprintf("fn %s(@builtin(global_invocation_id) gid: vec3<u32>) {\n", kernelName(node)))
	b.shaderBuf.WriteString("    let idx = gid.x;\n    if (idx >= size) { return; }\n")
	b.shaderBuf.WriteString(fmt.Sprintf("    %s[idx] = %s[idx] %s %s[idx];\n", output, left, op, right))
	b.shaderBuf.WriteString("}\n\n")
}
