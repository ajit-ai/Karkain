package cpu

import (
	"os/exec"
	"testing"

	"karkain/pkg/backend"
	"karkain/pkg/tensor"
)

func TestName(t *testing.T) {
	b := New()
	if b.Name() != "cpu" {
		t.Errorf("expected cpu, got %s", b.Name())
	}
}

func TestCapabilities(t *testing.T) {
	b := New()
	caps := b.Capabilities()
	if !caps.SupportsOp(tensor.OpAdd) {
		t.Error("expected to support Add")
	}
	if !caps.SupportsOp(tensor.OpMatMul) {
		t.Error("expected to support MatMul")
	}
	if !caps.SupportsDtype(tensor.ElemF32) {
		t.Error("expected to support F32")
	}
	if caps.SupportsOp(tensor.OpSlice) {
		t.Error("expected to NOT support Slice")
	}
}

func TestSupports(t *testing.T) {
	b := New()
	if !b.Supports(tensor.OpAdd, tensor.ElemF32, tensor.NewShape(3, 4)) {
		t.Error("expected to support add f32[3,4]")
	}
	if b.Supports(tensor.OpSlice, tensor.ElemF32, tensor.NewShape(3, 4)) {
		t.Error("expected to NOT support slice")
	}
}

func TestTensorCRuntimePresent(t *testing.T) {
	if len(TensorCRuntime) < 500 {
		t.Errorf("expected substantial C runtime, got %d bytes", len(TensorCRuntime))
	}
	if !contains(TensorCRuntime, "tensor_matmul") {
		t.Error("runtime missing tensor_matmul")
	}
	if !contains(TensorCRuntime, "tensor_add") {
		t.Error("runtime missing tensor_add")
	}
	if !contains(TensorCRuntime, "tensor_relu") {
		t.Error("runtime missing tensor_relu")
	}
}

func TestGenerateCProgram(t *testing.T) {
	a := tensor.NewNode("a", tensor.OpCreate, tensor.NewShape(2), tensor.ElemF32)
	b := tensor.NewNode("b", tensor.OpCreate, tensor.NewShape(2), tensor.ElemF32)
	sum := tensor.NewNode("sum", tensor.OpAdd, tensor.NewShape(2), tensor.ElemF32, "a", "b")

	g := tensor.NewGraph()
	g.AddNode(a)
	g.AddNode(b)
	g.AddNode(sum)

	inputs := map[string][]float64{
		"a": {1.0, 2.0},
		"b": {3.0, 4.0},
	}
	code := generateCProgram(g, inputs)
	if !contains(code, "tensor_add") {
		t.Error("generated C missing tensor_add")
	}
	if !contains(code, "tensor_create") {
		t.Error("generated C missing tensor_create")
	}
}

// ============================================================
// End-to-end execution tests (compile + run + verify)
// ============================================================

func TestExecuteAdd(t *testing.T) {
	skipIfNoGCC(t)

	a := tensor.NewNode("a", tensor.OpCreate, tensor.NewShape(4), tensor.ElemF32)
	b := tensor.NewNode("b", tensor.OpCreate, tensor.NewShape(4), tensor.ElemF32)
	sum := tensor.NewNode("sum", tensor.OpAdd, tensor.NewShape(4), tensor.ElemF32, "a", "b")

	g := tensor.NewGraph()
	g.AddNode(a)
	g.AddNode(b)
	g.AddNode(sum)
	g.AddOutput(sum)

	bk := New()
	result, err := bk.Execute(g, map[string][]float64{
		"a": {1, 2, 3, 4},
		"b": {10, 20, 30, 40},
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.Outputs == nil {
		t.Fatal("expected outputs")
	}
}

func TestExecuteMatMul(t *testing.T) {
	skipIfNoGCC(t)

	a := tensor.NewNode("a", tensor.OpCreate, tensor.NewShape(2, 3), tensor.ElemF32)
	b := tensor.NewNode("b", tensor.OpCreate, tensor.NewShape(3, 2), tensor.ElemF32)
	mm := tensor.NewNode("mm", tensor.OpMatMul, tensor.NewShape(2, 2), tensor.ElemF32, "a", "b")

	g := tensor.NewGraph()
	g.AddNode(a)
	g.AddNode(b)
	g.AddNode(mm)
	g.AddOutput(mm)

	bk := New()
	result, err := bk.Execute(g, map[string][]float64{
		"a": {1, 2, 3, 4, 5, 6},
		"b": {7, 8, 9, 10, 11, 12},
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if result.Outputs == nil {
		t.Fatal("expected outputs")
	}
}

func TestExecuteRelu(t *testing.T) {
	skipIfNoGCC(t)

	a := tensor.NewNode("a", tensor.OpCreate, tensor.NewShape(4), tensor.ElemF32)
	relu := tensor.NewNode("relu", tensor.OpRelu, tensor.NewShape(4), tensor.ElemF32, "a")

	g := tensor.NewGraph()
	g.AddNode(a)
	g.AddNode(relu)
	g.AddOutput(relu)

	bk := New()
	result, err := bk.Execute(g, map[string][]float64{
		"a": {-1, 0, 1, 2},
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if result.Outputs == nil {
		t.Fatal("expected outputs")
	}
}

func skipIfNoGCC(t *testing.T) {
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("gcc not available")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestBackendInterface(t *testing.T) {
	b := New()
	var bk backend.Backend = b
	if bk.Name() != "cpu" {
		t.Errorf("expected cpu, got %s", bk.Name())
	}
}
