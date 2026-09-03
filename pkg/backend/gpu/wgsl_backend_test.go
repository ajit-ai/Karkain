package gpu

import (
	"strings"
	"testing"

	"karkain/pkg/backend"
	"karkain/pkg/tensor"
)

func TestName(t *testing.T) {
	b := New()
	if b.Name() != "gpu" {
		t.Errorf("expected gpu, got %s", b.Name())
	}
}

func TestCapabilities(t *testing.T) {
	b := New()
	caps := b.Capabilities()
	if !caps.SupportsOp(tensor.OpMatMul) {
		t.Error("expected to support MatMul")
	}
	if !caps.SupportsOp(tensor.OpRelu) {
		t.Error("expected to support Relu")
	}
	if !caps.SupportsDtype(tensor.ElemF32) {
		t.Error("expected to support F32")
	}
	if caps.SupportsDtype(tensor.ElemF64) {
		t.Error("GPU backend only advertises f32/i32")
	}
	if caps.SupportsOp(tensor.OpSlice) {
		t.Error("expected to NOT support Slice")
	}
}

func TestSupports(t *testing.T) {
	b := New()
	if !b.Supports(tensor.OpMatMul, tensor.ElemF32, tensor.NewShape(4, 4)) {
		t.Error("expected to support matmul f32")
	}
	if b.Supports(tensor.OpSlice, tensor.ElemF32, tensor.NewShape(4, 4)) {
		t.Error("expected to NOT support slice")
	}
}

func TestGenerateMatMulShader(t *testing.T) {
	b := New()
	a := tensor.NewNode("A", tensor.OpCreate, tensor.NewShape(2, 3), tensor.ElemF32)
	bb := tensor.NewNode("B", tensor.OpCreate, tensor.NewShape(3, 4), tensor.ElemF32)
	mm := tensor.NewNode("mm", tensor.OpMatMul, tensor.NewShape(2, 4), tensor.ElemF32, "A", "B")

	g := tensor.NewGraph()
	g.AddNode(a)
	g.AddNode(bb)
	g.AddNode(mm)
	g.AddOutput(mm)

	shader, err := b.GenerateShader(g)
	if err != nil {
		t.Fatalf("generate shader failed: %v", err)
	}
	if !strings.Contains(shader, "@compute") {
		t.Error("shader missing @compute")
	}
	if !strings.Contains(shader, "kernel_mm") {
		t.Error("shader missing kernel_mm")
	}
	if !strings.Contains(shader, "params") {
		t.Error("shader missing params uniform")
	}
}

func TestGenerateReluShader(t *testing.T) {
	b := New()
	a := tensor.NewNode("a", tensor.OpCreate, tensor.NewShape(4), tensor.ElemF32)
	relu := tensor.NewNode("relu", tensor.OpRelu, tensor.NewShape(4), tensor.ElemF32, "a")

	g := tensor.NewGraph()
	g.AddNode(a)
	g.AddNode(relu)
	g.AddOutput(relu)

	shader, err := b.GenerateShader(g)
	if err != nil {
		t.Fatalf("generate shader failed: %v", err)
	}
	if !strings.Contains(shader, "max(") {
		t.Error("relu shader missing max")
	}
}

func TestGenerateElementwiseShader(t *testing.T) {
	b := New()
	a := tensor.NewNode("a", tensor.OpCreate, tensor.NewShape(4), tensor.ElemF32)
	bb := tensor.NewNode("b", tensor.OpCreate, tensor.NewShape(4), tensor.ElemF32)
	sum := tensor.NewNode("sum", tensor.OpAdd, tensor.NewShape(4), tensor.ElemF32, "a", "b")

	g := tensor.NewGraph()
	g.AddNode(a)
	g.AddNode(bb)
	g.AddNode(sum)
	g.AddOutput(sum)

	shader, err := b.GenerateShader(g)
	if err != nil {
		t.Fatalf("generate shader failed: %v", err)
	}
	if !strings.Contains(shader, "a[idx] + b[idx]") {
		t.Error("add shader missing element-wise add")
	}
}

func TestGenerateSoftmaxShader(t *testing.T) {
	b := New()
	a := tensor.NewNode("a", tensor.OpCreate, tensor.NewShape(4), tensor.ElemF32)
	sm := tensor.NewNode("sm", tensor.OpSoftmax, tensor.NewShape(4), tensor.ElemF32, "a")

	g := tensor.NewGraph()
	g.AddNode(a)
	g.AddNode(sm)
	g.AddOutput(sm)

	shader, err := b.GenerateShader(g)
	if err != nil {
		t.Fatalf("generate shader failed: %v", err)
	}
	if !strings.Contains(shader, "exp(") {
		t.Error("softmax shader missing exp")
	}
}

func TestExecuteGeneratesWGSL(t *testing.T) {
	b := New()
	// Build a combined forward graph: relu(A @ B)
	a := tensor.NewNode("A", tensor.OpCreate, tensor.NewShape(3, 2), tensor.ElemF32)
	bb := tensor.NewNode("B", tensor.OpCreate, tensor.NewShape(2, 4), tensor.ElemF32)
	mm := tensor.NewNode("mm", tensor.OpMatMul, tensor.NewShape(3, 4), tensor.ElemF32, "A", "B")
	z := tensor.NewNode("z", tensor.OpRelu, tensor.NewShape(3, 4), tensor.ElemF32, "mm")

	g := tensor.NewGraph()
	g.AddNode(a)
	g.AddNode(bb)
	g.AddNode(mm)
	g.AddNode(z)
	g.AddOutput(z)

	result, err := b.Execute(g, nil)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if result.Metadata["wgsl"] == "" {
		t.Error("expected wgsl metadata")
	}
	if _, ok := result.Outputs["mm"]; !ok {
		t.Error("expected matmul node registered as output/kernel")
	}
}

func TestExecuteEmptyGraph(t *testing.T) {
	b := New()
	g := tensor.NewGraph()
	_, err := b.Execute(g, nil)
	if err == nil {
		t.Error("expected error for empty graph")
	}
}

func TestBackendInterface(t *testing.T) {
	b := New()
	var bk backend.Backend = b
	if bk.Name() != "gpu" {
		t.Errorf("expected gpu, got %s", bk.Name())
	}
}

func TestWGSLTypeMapping(t *testing.T) {
	if wgslType(tensor.ElemF32) != "f32" {
		t.Error("expected f32")
	}
	if wgslType(tensor.ElemI32) != "i32" {
		t.Error("expected i32")
	}
	if wgslType(tensor.ElemF64) != "f64" {
		t.Error("expected f64 for f64")
	}
	if wgslType(tensor.ElemBool) != "f32" {
		t.Error("expected default f32 for bool")
	}
}
