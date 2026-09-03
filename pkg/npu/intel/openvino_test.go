package intel

import (
	"testing"

	"karkain/pkg/backend"
	"karkain/pkg/npu"
	"karkain/pkg/tensor"
)

func TestBackendInterface(t *testing.T) {
	b := New()
	var bk backend.Backend = b
	if bk.Name() != "npu-intel" {
		t.Errorf("expected npu-intel, got %s", bk.Name())
	}
	var nb npu.NPUBackend = b
	if nb.Vendor() != npu.VendorIntel {
		t.Error("expected intel vendor")
	}
}

func TestCapabilities(t *testing.T) {
	b := New()
	caps := b.Capabilities()
	if !caps.SupportsOp(tensor.OpMatMul) {
		t.Error("expected matmul support")
	}
	if !caps.SupportsDtype(tensor.ElemF32) {
		t.Error("expected f32 support")
	}
}

func TestSupports(t *testing.T) {
	b := New()
	if !b.Supports(tensor.OpMatMul, tensor.ElemF32, tensor.NewShape(4, 4)) {
		t.Error("expected to support matmul")
	}
}

func TestDetectHardware(t *testing.T) {
	b := New()
	devs := b.DetectHardware()
	// Without opt-in env, detection is empty but must not panic.
	_ = devs
}

func TestCompile(t *testing.T) {
	b := New()
	x := tensor.NewNode("x", tensor.OpCreate, tensor.NewShape(4), tensor.ElemF32)
	y := tensor.NewNode("y", tensor.OpRelu, tensor.NewShape(4), tensor.ElemF32, "x")
	g := tensor.NewGraph()
	g.AddNode(x)
	g.AddNode(y)
	g.AddOutput(y)

	dev := npu.NPUDevice{Name: "Intel AI Boost", Vendor: npu.VendorIntel}
	prog, err := b.Compile(g, dev)
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}
	if prog.Vendor != npu.VendorIntel {
		t.Error("expected intel program")
	}
	if string(prog.Blob) != "openvino-ir:Intel AI Boost" {
		t.Errorf("unexpected blob: %q", prog.Blob)
	}
}

func TestExecute(t *testing.T) {
	b := New()
	x := tensor.NewNode("x", tensor.OpCreate, tensor.NewShape(4), tensor.ElemF32)
	y := tensor.NewNode("y", tensor.OpAdd, tensor.NewShape(4), tensor.ElemF32, "x", "x")
	g := tensor.NewGraph()
	g.AddNode(x)
	g.AddNode(y)
	g.AddOutput(y)

	res, err := b.Execute(g, nil)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if res.Metadata["npu_vendor"] != "intel" {
		t.Error("expected intel vendor metadata")
	}
}
