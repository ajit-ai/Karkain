package apple

import (
	"testing"

	"karkain/pkg/backend"
	"karkain/pkg/npu"
	"karkain/pkg/tensor"
)

func TestBackendInterface(t *testing.T) {
	b := New()
	var bk backend.Backend = b
	if bk.Name() != "npu-apple" {
		t.Errorf("expected npu-apple, got %s", bk.Name())
	}
	var nb npu.NPUBackend = b
	if nb.Vendor() != npu.VendorApple {
		t.Error("expected apple vendor")
	}
}

func TestCapabilities(t *testing.T) {
	b := New()
	if !b.Capabilities().SupportsOp(tensor.OpMatMul) {
		t.Error("expected matmul support")
	}
	if !b.Supports(tensor.OpSigmoid, tensor.ElemF32, tensor.NewShape(8)) {
		t.Error("expected sigmoid support")
	}
}

func TestCompile(t *testing.T) {
	b := New()
	g := tensor.NewGraph()
	x := tensor.NewNode("x", tensor.OpCreate, tensor.NewShape(4), tensor.ElemF32)
	y := tensor.NewNode("y", tensor.OpRelu, tensor.NewShape(4), tensor.ElemF32, "x")
	g.AddNode(x)
	g.AddNode(y)
	g.AddOutput(y)

	dev := npu.NPUDevice{Name: "ANE", Vendor: npu.VendorApple}
	prog, err := b.Compile(g, dev)
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}
	if prog.Vendor != npu.VendorApple {
		t.Error("expected apple program")
	}
	if string(prog.Blob) != "coreml:ANE" {
		t.Errorf("unexpected blob: %q", prog.Blob)
	}
}

func TestExecute(t *testing.T) {
	b := New()
	g := tensor.NewGraph()
	x := tensor.NewNode("x", tensor.OpCreate, tensor.NewShape(4), tensor.ElemF32)
	g.AddNode(x)
	g.AddOutput(x)
	res, err := b.Execute(g, nil)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if res.Metadata["npu_vendor"] != "apple" {
		t.Error("expected apple vendor metadata")
	}
}
