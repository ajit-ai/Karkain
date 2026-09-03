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

	// Assign input data
	inputShapes := make(map[string]tensor.Shape)
	for _, node := range graph.Nodes {
		if node.Op == tensor.OpCreate {
			inputShapes[node.ID] = node.Shape
		}
	}
	for name, data := range inputs {
		shape, ok := inputShapes[name]
		if !ok {
			shape = tensor.NewShape(len(data))
		}
		// Emit shape array
		sb.WriteString(fmt.Sprintf("    int %s_shape[] = {", name))
		for i, d := range shape {
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
		sb.WriteString(fmt.Sprintf("    Tensor* %s = tensor_create(%d, %s_shape, 0);\n", name, shape.Rank(), name))
		for i, v := range data {
			sb.WriteString(fmt.Sprintf("    %s->data[%d] = %g;\n", name, i, v))
		}
	}

	// Execute graph ops
	counter := 0
	outputVars := make(map[string]string)
	for _, node := range graph.Nodes {
		switch node.Op {
		case tensor.OpAdd, tensor.OpSub, tensor.OpMul, tensor.OpDiv,
			tensor.OpMatMul, tensor.OpRelu, tensor.OpSigmoid, tensor.OpTanh,
			tensor.OpSoftmax, tensor.OpTranspose:
			next := counter
			emitOp(&sb, node, &counter)
			outputVars[node.ID] = fmt.Sprintf("var%d", next+1)
		}
	}

	// Print graph outputs for verification
	for _, out := range graph.Outputs {
		varName, ok := outputVars[out.ID]
		if !ok {
			continue
		}
		sb.WriteString(fmt.Sprintf("    printf(\"RESULT %s\", \"\");\n", out.ID))
		sb.WriteString(fmt.Sprintf("    for (int i = 0; i < %s->ndim; i++) printf(\"%%d,\", %s->shape[i]);\n", varName, varName))
		sb.WriteString(fmt.Sprintf("    printf(\"|%%zu|\", tensor_numel(%s));\n", varName))
		sb.WriteString(fmt.Sprintf("    for (size_t i = 0; i < tensor_numel(%s); i++) printf(\"%%g \", %s->data[i]);\n", varName, varName))
		sb.WriteString("    printf(\"\\n\");\n")
	}

	sb.WriteString("\n    return 0;\n}\n")
	return sb.String()
}

func emitOp(sb *strings.Builder, node *tensor.TensorNode, counter *int) {
	*counter++
	tmp := fmt.Sprintf("var%d", *counter)
	switch node.Op {
	case tensor.OpAdd:
		sb.WriteString(fmt.Sprintf("    Tensor* %s = tensor_add(%s, %s);\n", tmp, node.Args[0], node.Args[1]))
	case tensor.OpSub:
		sb.WriteString(fmt.Sprintf("    Tensor* %s = tensor_sub(%s, %s);\n", tmp, node.Args[0], node.Args[1]))
	case tensor.OpMul:
		sb.WriteString(fmt.Sprintf("    Tensor* %s = tensor_mul(%s, %s);\n", tmp, node.Args[0], node.Args[1]))
	case tensor.OpDiv:
		sb.WriteString(fmt.Sprintf("    Tensor* %s = tensor_div(%s, %s);\n", tmp, node.Args[0], node.Args[1]))
	case tensor.OpMatMul:
		sb.WriteString(fmt.Sprintf("    Tensor* %s = tensor_matmul(%s, %s);\n", tmp, node.Args[0], node.Args[1]))
	case tensor.OpRelu:
		sb.WriteString(fmt.Sprintf("    Tensor* %s = tensor_relu(%s);\n", tmp, node.Args[0]))
	case tensor.OpSigmoid:
		sb.WriteString(fmt.Sprintf("    Tensor* %s = tensor_sigmoid(%s);\n", tmp, node.Args[0]))
	case tensor.OpTanh:
		sb.WriteString(fmt.Sprintf("    Tensor* %s = tensor_tanh(%s);\n", tmp, node.Args[0]))
	case tensor.OpSoftmax:
		sb.WriteString(fmt.Sprintf("    Tensor* %s = tensor_softmax(%s);\n", tmp, node.Args[0]))
	case tensor.OpTranspose:
		sb.WriteString(fmt.Sprintf("    Tensor* %s = tensor_transpose(%s);\n", tmp, node.Args[0]))
	}
	node.Args = append(node.Args, "output_var") // placeholder
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
