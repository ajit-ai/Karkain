// Package cpu implements the CPU reference backend for tensor execution.
// CPU is the correctness oracle — every accelerator must match its output.
package cpu

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"karkain/pkg/backend"
	"karkain/pkg/tensor"
)

// CPUBackend implements the Backend interface for CPU execution.
type CPUBackend struct {
	workdir string
}

// New creates a new CPU backend.
func New() *CPUBackend {
	return &CPUBackend{workdir: ""}
}

// Name returns the backend name.
func (b *CPUBackend) Name() string {
	return "cpu"
}

// Capabilities returns what the CPU backend supports.
func (b *CPUBackend) Capabilities() backend.Capabilities {
	return backend.Capabilities{
		Name:            "cpu",
		SupportedDtypes: []tensor.ElemType{tensor.ElemI32, tensor.ElemI64, tensor.ElemF32, tensor.ElemF64, tensor.ElemBool},
		MaxRank:         8,
		SupportedOps: []tensor.Op{
			tensor.OpCreate, tensor.OpCopy,
			tensor.OpAdd, tensor.OpSub, tensor.OpMul, tensor.OpDiv,
			tensor.OpMatMul,
			tensor.OpRelu, tensor.OpSigmoid, tensor.OpTanh, tensor.OpSoftmax,
			tensor.OpTranspose, tensor.OpReshape, tensor.OpReduceSum,
		},
		SupportedLayouts: []tensor.Layout{tensor.LayoutRowMajor},
	}
}

// Supports returns true if the CPU backend can execute the operation.
func (b *CPUBackend) Supports(op tensor.Op, dtype tensor.ElemType, shape tensor.Shape) bool {
	caps := b.Capabilities()
	if !caps.SupportsOp(op) {
		return false
	}
	if !caps.SupportsDtype(dtype) {
		return false
	}
	if shape.Rank() > caps.MaxRank {
		return false
	}
	return true
}

// Execute runs a tensor graph using the embedded C runtime.
// It compiles a small C program and runs it.
func (b *CPUBackend) Execute(graph *tensor.TensorGraph, inputs map[string][]float64) (*backend.Result, error) {
	tempDir := filepath.Join(os.TempDir(), "karkain-npu")
	os.MkdirAll(tempDir, 0777)

	// Generate C program
	cCode := generateCProgram(graph, inputs)

	// Write to temp file
	cFile := filepath.Join(tempDir, "kernel.c")
	exeFile := filepath.Join(tempDir, "kernel.exe")
	os.WriteFile(cFile, []byte(cCode), 0644)

	// Compile with gcc
	cmd := exec.Command("gcc", "-std=c2x", "-O2", "-lm", cFile, "-o", exeFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gcc compile failed: %v\n%s", err, out)
	}

	// Run
	runCmd := exec.Command(exeFile)
	runOut, err := runCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("execution failed: %v\n%s", err, runOut)
	}

	// Parse output
	result := parseOutput(string(runOut), graph)
	return result, nil
}

// SetWorkdir overrides the temporary working directory (for tests).
func (b *CPUBackend) SetWorkdir(dir string) {
	b.workdir = dir
}

// ============================================================
// C Program Generation
// ============================================================

func generateCProgram(graph *tensor.TensorGraph, inputs map[string][]float64) string {
	var sb strings.Builder

	// Header
	sb.WriteString("#include <stdio.h>\n#include <stdlib.h>\n")
	sb.WriteString(TensorCRuntime)
	sb.WriteString("\nint main(void) {\n")

	// C-legal variable name for each node ID
	sanitize := func(id string) string {
		var b strings.Builder
		for _, r := range id {
			if r == '-' || r == ':' {
				b.WriteByte('_')
			} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
				b.WriteRune(r)
			} else {
				b.WriteByte('_')
			}
		}
		return b.String()
	}
	varName := make(map[string]string)
	for _, node := range graph.Nodes {
		varName[node.ID] = sanitize(node.ID)
	}

	// Declare resolved node types for those that are Create (may hold data)
	// First: emit all Create nodes as tensor_create, and fill inputs.
	for _, node := range graph.Nodes {
		if node.Op != tensor.OpCreate {
			continue
		}
		v := varName[node.ID]
		// Emit shape array
		sb.WriteString(fmt.Sprintf("    int %s_shape[] = {", v))
		for i, d := range node.Shape {
			if i > 0 {
				sb.WriteString(", ")
			}
			if d.IsStatic() {
				sb.WriteString(fmt.Sprintf("%d", d.Size))
			} else {
				sb.WriteString("1")
			}
		}
		sb.WriteString("};\n")
		sb.WriteString(fmt.Sprintf("    Tensor* %s = tensor_create(%d, %s_shape, 0);\n", v, node.Shape.Rank(), v))

		// Fill input data
		if data, ok := inputs[node.ID]; ok {
			for i, val := range data {
				sb.WriteString(fmt.Sprintf("    %s->data[%d] = %g;\n", v, i, val))
			}
		}
	}

	// Emit all non-Create ops, resolving args by node ID to their variable names
	for _, node := range graph.Nodes {
		switch node.Op {
		case tensor.OpAdd, tensor.OpSub, tensor.OpMul, tensor.OpDiv,
			tensor.OpMatMul, tensor.OpRelu, tensor.OpSigmoid, tensor.OpTanh,
			tensor.OpSoftmax, tensor.OpTranspose:
			emitOpNamed(&sb, node, varName)
		}
	}

	// Print graph outputs for verification
	outputUses := make(map[string]bool)
	for _, out := range graph.Outputs {
		outputUses[out.ID] = true
	}
	for _, node := range graph.Nodes {
		if !outputUses[node.ID] {
			continue
		}
		v := varName[node.ID]
		if node.Op == tensor.OpCreate {
			// Create outputs directly print their data
			sb.WriteString(fmt.Sprintf("    printf(\"RESULT %s\", \"\");\n", node.ID))
			sb.WriteString(fmt.Sprintf("    for (int i = 0; i < %s->ndim; i++) printf(\"%%d,\", %s->shape[i]);\n", v, v))
			sb.WriteString(fmt.Sprintf("    printf(\"|%%zu|\", tensor_numel(%s));\n", v))
			sb.WriteString(fmt.Sprintf("    for (size_t i = 0; i < tensor_numel(%s); i++) printf(\"%%g \", %s->data[i]);\n", v, v))
			sb.WriteString("    printf(\"\\n\");\n")
		}
	}

	sb.WriteString("\n    return 0;\n}\n")
	return sb.String()
}

func emitOpNamed(sb *strings.Builder, node *tensor.TensorNode, varName map[string]string) {
	target := varName[node.ID]
	arg := func(idx int) string { return varName[node.Args[idx].(string)] }
	switch node.Op {
	case tensor.OpAdd:
		sb.WriteString(fmt.Sprintf("    Tensor* %s = tensor_add(%s, %s);\n", target, arg(0), arg(1)))
	case tensor.OpSub:
		sb.WriteString(fmt.Sprintf("    Tensor* %s = tensor_sub(%s, %s);\n", target, arg(0), arg(1)))
	case tensor.OpMul:
		sb.WriteString(fmt.Sprintf("    Tensor* %s = tensor_mul(%s, %s);\n", target, arg(0), arg(1)))
	case tensor.OpDiv:
		sb.WriteString(fmt.Sprintf("    Tensor* %s = tensor_div(%s, %s);\n", target, arg(0), arg(1)))
	case tensor.OpMatMul:
		sb.WriteString(fmt.Sprintf("    Tensor* %s = tensor_matmul(%s, %s);\n", target, arg(0), arg(1)))
	case tensor.OpRelu:
		sb.WriteString(fmt.Sprintf("    Tensor* %s = tensor_relu(%s);\n", target, arg(0)))
	case tensor.OpSigmoid:
		sb.WriteString(fmt.Sprintf("    Tensor* %s = tensor_sigmoid(%s);\n", target, arg(0)))
	case tensor.OpTanh:
		sb.WriteString(fmt.Sprintf("    Tensor* %s = tensor_tanh(%s);\n", target, arg(0)))
	case tensor.OpSoftmax:
		sb.WriteString(fmt.Sprintf("    Tensor* %s = tensor_softmax(%s);\n", target, arg(0)))
	case tensor.OpTranspose:
		sb.WriteString(fmt.Sprintf("    Tensor* %s = tensor_transpose(%s);\n", target, arg(0)))
	}
}

// parseOutput parses the C program output, extracting RESULT lines.
func parseOutput(output string, graph *tensor.TensorGraph) *backend.Result {
	result := backend.NewResult()
	for _, node := range graph.Nodes {
		result.Outputs[node.ID] = node
	}

	// Parse RESULT lines: "RESULT <id> <shape...,>|<numel>|<values...>"
	lines := strings.Split(output, "\n")
	var currentID string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "RESULT ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				currentID = parts[1]
				result.Metadata[currentID] = strings.TrimSpace(strings.TrimPrefix(line, "RESULT "+currentID))
			}
			continue
		}
		if currentID == "" {
			continue
		}
		// Shape line: "3,4,"
		if strings.HasSuffix(line, ",") && !strings.Contains(line, "|") {
			shapeStr := strings.TrimSuffix(line, ",")
			dims := strings.Split(shapeStr, ",")
			result.Shapes[currentID] = len(dims)
			continue
		}
		// Numel line: "|12|"
		if strings.HasPrefix(line, "|") && strings.Contains(line, "|") {
			continue
		}
		// Values line: "1 2 3 "
		fields := strings.Fields(line)
		if len(fields) > 0 {
			vals := make([]float64, 0, len(fields))
			for _, f := range fields {
				var v float64
				if _, err := fmt.Sscanf(f, "%g", &v); err == nil {
					vals = append(vals, v)
				}
			}
			if len(vals) > 0 {
				result.Values[currentID] = vals
				currentID = ""
			}
		}
	}

	return result
}

// TempDir returns the working dir for CPU backend compilation (for platform).
func TempDir() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("TEMP"), "karkain-npu")
	}
	return "/tmp/karkain-npu"
}
