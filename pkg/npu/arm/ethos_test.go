package arm

import (
	"testing"

	"karkain/pkg/backend"
	"karkain/pkg/npu"
	"karkain/pkg/tensor"
)

func TestBackendInterface(t *testing.T) {
	b := New()
	var bk backend.Backend = b
	if bk.Name() != "npu-arm" {
		t.Errorf("expected npu-arm, got %s", bk.Name())
	}
	var nb npu.NPUBackend = b
	if nb.Vendor() != npu.VendorArm {
		t.Error("expected arm vendor")
	}
}

func TestCapabilitiesEmbedded(t *testing.T) {
	b := New()
	caps := b.Capabilities()
	if caps.MaxRank != 4 {
		t.Errorf("expected embedded max rank 4, got %d", caps.MaxRank)
	}
	if !caps.SupportsOp(tensor.OpMatMul) {
		t.Error("expected matmul support")
	}
	if caps.SupportsDtype(tensor.ElemF64) {
		t.Error("embedded Ethos-U should not support f64")
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

	dev := npu.NPUDevice{Name: "Ethos-U65", Vendor: npu.VendorArm}
	prog, err := b.Compile(g, dev)
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}
	if prog.Vendor != npu.VendorArm {
		t.Error("expected arm program")
	}
	if string(prog.Blob) != "ethos-u:Ethos-U65" {
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
	if res.Metadata["npu_vendor"] != "arm" {
		t.Error("expected arm vendor metadata")
	}
}
