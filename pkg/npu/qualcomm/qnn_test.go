package qualcomm

import (
	"testing"

	"karkain/pkg/backend"
	"karkain/pkg/npu"
	"karkain/pkg/tensor"
)

func TestBackendInterface(t *testing.T) {
	b := New()
	var bk backend.Backend = b
	if bk.Name() != "npu-qualcomm" {
		t.Errorf("expected npu-qualcomm, got %s", bk.Name())
	}
	var nb npu.NPUBackend = b
	if nb.Vendor() != npu.VendorQualcomm {
		t.Error("expected qualcomm vendor")
	}
}

func TestCapabilities(t *testing.T) {
	b := New()
	if !b.Capabilities().SupportsOp(tensor.OpMatMul) {
		t.Error("expected matmul support")
	}
	if !b.Supports(tensor.OpRelu, tensor.ElemF32, tensor.NewShape(8)) {
		t.Error("expected relu support")
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

	dev := npu.NPUDevice{Name: "Hexagon HTP", Vendor: npu.VendorQualcomm}
	prog, err := b.Compile(g, dev)
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}
	if prog.Vendor != npu.VendorQualcomm {
		t.Error("expected qualcomm program")
	}
	if string(prog.Blob) != "qnn:Hexagon HTP" {
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
	if res.Metadata["npu_vendor"] != "qualcomm" {
		t.Error("expected qualcomm vendor metadata")
	}
}
