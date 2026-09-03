package npu

import (
	"math"
	"strings"
	"testing"

	"karkain/pkg/tensor"
)

// ============================================================
// Fusion tests
// ============================================================

func TestFusionLinear(t *testing.T) {
	// x[4,3] @ w[3,5] => mm[4,5]; mm + bias[5] => add; add -> relu.
	x := tensor.NewNode("x", tensor.OpCreate, tensor.NewShape(4, 3), tensor.ElemF32)
	w := tensor.NewNode("w", tensor.OpCreate, tensor.NewShape(3, 5), tensor.ElemF32)
	mm := tensor.NewNode("mm", tensor.OpMatMul, tensor.NewShape(4, 5), tensor.ElemF32, "x", "w")
	bias := tensor.NewNode("bias", tensor.OpCreate, tensor.NewShape(5), tensor.ElemF32)
	add := tensor.NewNode("add", tensor.OpAdd, tensor.NewShape(4, 5), tensor.ElemF32, "mm", "bias")
	relu := tensor.NewNode("relu", tensor.OpRelu, tensor.NewShape(4, 5), tensor.ElemF32, "add")

	g := tensor.NewGraph()
	for _, n := range []*tensor.TensorNode{x, w, mm, bias, add, relu} {
		g.AddNode(n)
	}
	g.AddOutput(relu)

	fp := NewFusionPass()
	res := fp.Run(g)
	if len(res.Ops) != 1 {
		t.Fatalf("expected 1 fused op, got %d", len(res.Ops))
	}
	fused := res.Ops[0]
	if fused.Kind != FusedLinear {
		t.Errorf("expected FusedLinear, got %s", fused.Kind)
	}
	if len(fused.Nodes) != 3 {
		t.Errorf("expected 3 fused nodes, got %d", len(fused.Nodes))
	}
	// All 3 fused nodes should be removed from remainder.
	if len(res.Remainder) != 3 { // x, w, bias (matmul/add/relu covered)
		t.Errorf("expected 3 remainder (inputs), got %d", len(res.Remainder))
	}
	if !strings.Contains(res.Describe(), "matmul + add + relu") {
		t.Errorf("unexpected describe: %s", res.Describe())
	}
}

func TestFusionDisabled(t *testing.T) {
	x := tensor.NewNode("x", tensor.OpCreate, tensor.NewShape(4, 3), tensor.ElemF32)
	w := tensor.NewNode("w", tensor.OpCreate, tensor.NewShape(3, 5), tensor.ElemF32)
	mm := tensor.NewNode("mm", tensor.OpMatMul, tensor.NewShape(4, 5), tensor.ElemF32, "x", "w")
	bias := tensor.NewNode("bias", tensor.OpCreate, tensor.NewShape(5), tensor.ElemF32)
	add := tensor.NewNode("add", tensor.OpAdd, tensor.NewShape(4, 5), tensor.ElemF32, "mm", "bias")
	relu := tensor.NewNode("relu", tensor.OpRelu, tensor.NewShape(4, 5), tensor.ElemF32, "add")

	g := tensor.NewGraph()
	for _, n := range []*tensor.TensorNode{x, w, mm, bias, add, relu} {
		g.AddNode(n)
	}
	g.AddOutput(relu)

	fp := NewFusionPass()
	fp.Disable(FusedLinear)
	res := fp.Run(g)
	if len(res.Ops) != 0 {
		t.Fatalf("expected no fused ops when disabled, got %d", len(res.Ops))
	}
	if len(res.Remainder) != 6 {
		t.Errorf("expected all 6 nodes in remainder, got %d", len(res.Remainder))
	}
}

func TestFusionNoMatch(t *testing.T) {
	// Just an elementwise add+relu, no matmul prefix -> no fusion.
	a := tensor.NewNode("a", tensor.OpCreate, tensor.NewShape(4), tensor.ElemF32)
	b := tensor.NewNode("b", tensor.OpCreate, tensor.NewShape(4), tensor.ElemF32)
	add := tensor.NewNode("add", tensor.OpAdd, tensor.NewShape(4), tensor.ElemF32, "a", "b")
	relu := tensor.NewNode("relu", tensor.OpRelu, tensor.NewShape(4), tensor.ElemF32, "add")

	g := tensor.NewGraph()
	for _, n := range []*tensor.TensorNode{a, b, add, relu} {
		g.AddNode(n)
	}
	g.AddOutput(relu)

	fp := NewFusionPass()
	res := fp.Run(g)
	if len(res.Ops) != 0 {
		t.Errorf("expected no fusion for add+relu, got %d", len(res.Ops))
	}
	if len(res.Remainder) != 4 {
		t.Errorf("expected all 4 in remainder, got %d", len(res.Remainder))
	}
}

// ============================================================
// Memory planning tests
// ============================================================

func TestPlanMemoryRowMajor(t *testing.T) {
	x := tensor.NewNode("x", tensor.OpCreate, tensor.NewShape(4, 4), tensor.ElemF32) // 16*4=64B
	y := tensor.NewNode("y", tensor.OpRelu, tensor.NewShape(4, 4), tensor.ElemF32, "x")
	g := tensor.NewGraph()
	g.AddNode(x)
	g.AddNode(y)

	plan := PlanMemory(g, LayoutRowMajor)
	if plan.Layout != LayoutRowMajor {
		t.Error("expected row-major layout")
	}
	// 64 + 64 elements bytes = 128, plus back buffer 64 => 192
	if plan.ArenaSize != 192 {
		t.Errorf("expected arena 192, got %d", plan.ArenaSize)
	}
	if len(plan.Objects) != 2 {
		t.Errorf("expected 2 objects, got %d", len(plan.Objects))
	}
	if plan.DoubleBuf.NBuffers != 2 {
		t.Errorf("expected 2 double buffers, got %d", plan.DoubleBuf.NBuffers)
	}
}

func TestPlanMemoryDoubleBuf(t *testing.T) {
	big := tensor.NewNode("big", tensor.OpCreate, tensor.NewShape(100, 100), tensor.ElemF32, ) // 40000B
	g := tensor.NewGraph()
	g.AddNode(big)
	plan := PlanMemory(g, LayoutRowMajor)
	largest := 100 * 100 * 4
	if plan.Objects[0].Bytes != largest {
		t.Errorf("expected largest %d, got %d", largest, plan.Objects[0].Bytes)
	}
	if plan.DoubleBuf.BackOffset != largest {
		t.Errorf("expected back offset %d, got %d", largest, plan.DoubleBuf.BackOffset)
	}
}

func TestPickLayout(t *testing.T) {
	if PickLayout(4) != LayoutNHWC {
		t.Error("expected NHWC for rank 4")
	}
	if PickLayout(2) != LayoutRowMajor {
		t.Error("expected row-major for rank 2")
	}
}

func TestOverlapDetection(t *testing.T) {
	a := &MemObject{Offset: 0, Bytes: 64}
	b := &MemObject{Offset: 32, Bytes: 32}
	c := &MemObject{Offset: 64, Bytes: 32}
	if !checkOverlap(a, b) {
		t.Error("expected a,b to overlap")
	}
	if checkOverlap(a, c) {
		t.Error("expected a,c to NOT overlap")
	}
}

// ============================================================
// Quantization tests
// ============================================================

func TestQuantizeInt8(t *testing.T) {
	q := NewQuantizer(QuantInt8)
	qt, err := q.Quantize([]float64{1, 2, 3, 4}, 0, 0)
	if err != nil {
		t.Fatalf("quantize failed: %v", err)
	}
	if qt.Mode != QuantInt8 {
		t.Errorf("expected int8, got %s", qt.Mode)
	}
	if len(qt.Data) != 4 {
		t.Errorf("expected 4 data, got %d", len(qt.Data))
	}
	// Dequant should approximately reconstruct.
	dq := qt.Dequant()
	if len(dq) != 4 {
		t.Fatalf("expected 4 dequant, got %d", len(dq))
	}
	for i := 0; i < 4; i++ {
		if math.Abs(dq[i]-float64(i+1)) > 0.5 {
			t.Errorf("dequant[%d]=%f not close to %d", i, dq[i], i+1)
		}
	}
}

func TestQuantizeInt8PerChannel(t *testing.T) {
	w := []float64{
		1, 2, // row 0 scale = 2/127
		10, 20, // row 1 scale = 20/127
	}
	qt := quantizeInt8PerChannel(w, 2, 2)
	if qt.Scale.Count != 2 {
		t.Errorf("expected 2 scales, got %d", qt.Scale.Count)
	}
	// Different rows have different scales.
	if qt.Scale.Scale[0] == qt.Scale.Scale[1] {
		t.Error("expected different row scales")
	}
	// Larger row should quantize to near max int8 values.
	dq := qt.Dequant()
	if math.Abs(dq[2]-10) > 1 || math.Abs(dq[3]-20) > 1 {
		t.Errorf("per-channel dequant off: %v", dq)
	}
}

func TestQuantizeInt4Pack(t *testing.T) {
	q := NewQuantizer(QuantInt4)
	qt, err := q.Quantize([]float64{1, 2, 3, 4}, 0, 0)
	if err != nil {
		t.Fatalf("quantize failed: %v", err)
	}
	if qt.Mode != QuantInt4 {
		t.Errorf("expected int4, got %s", qt.Mode)
	}
	if len(qt.Data) != 2 {
		t.Errorf("expected 2 packed bytes for 4 values, got %d", len(qt.Data))
	}
	dq := qt.Dequant()
	for i, v := range dq {
		if math.Abs(v-float64(i+1)) > 1 {
			t.Errorf("int4 dequant[%d]=%f not close to %d", i, v, i+1)
		}
	}
}

func TestQuantizeMixed(t *testing.T) {
	q := NewQuantizer(QuantMixed)
	w := []float64{1, 2, 3, 4}
	qt, _ := q.Quantize(w, 2, 2)
	if qt.Scale.Count != 2 {
		t.Errorf("mixed should use per-channel for 2D weights, got %d scales", qt.Scale.Count)
	}
	// 1D tensor -> scalar-scale int8
	q1 := NewQuantizer(QuantMixed)
	bias := []float64{0.5, -0.5, 1.5}
	qt1, _ := q1.Quantize(bias, 0, 0)
	if qt1.Scale.Count != 1 {
		t.Errorf("mixed 1D should use scalar scale, got %d", qt1.Scale.Count)
	}
}

func TestQuantizeNone(t *testing.T) {
	q := NewQuantizer(QuantNone)
	qt, err := q.Quantize([]float64{1, 2, 3}, 0, 0)
	if err != nil {
		t.Fatalf("quantize failed: %v", err)
	}
	if qt.Mode != QuantNone {
		t.Errorf("expected none, got %s", qt.Mode)
	}
	if len(qt.Data) != 0 {
		t.Errorf("expected no data for none mode, got %d", len(qt.Data))
	}
}

func TestQuantizerUnknownMode(t *testing.T) {
	q := NewQuantizer(QuantMode("bogus"))
	_, err := q.Quantize([]float64{1}, 0, 0)
	if err == nil {
		t.Error("expected error for unknown mode")
	}
}

// ============================================================
// MLIR dialect + lowering tests
// ============================================================

func TestLowerToMLIR(t *testing.T) {
	x := tensor.NewNode("x", tensor.OpCreate, tensor.NewShape(3, 2), tensor.ElemF32)
	w := tensor.NewNode("w", tensor.OpCreate, tensor.NewShape(2, 4), tensor.ElemF32)
	mm := tensor.NewNode("mm", tensor.OpMatMul, tensor.NewShape(3, 4), tensor.ElemF32, "x", "w")
	z := tensor.NewNode("z", tensor.OpRelu, tensor.NewShape(3, 4), tensor.ElemF32, "mm")

	g := tensor.NewGraph()
	for _, n := range []*tensor.TensorNode{x, w, mm, z} {
		g.AddNode(n)
	}
	g.AddOutput(z)

	l := NewLowerToMLIR("main")
	mod, err := l.Lower(g)
	if err != nil {
		t.Fatalf("lower failed: %v", err)
	}
	text := mod.Emit()
	if !strings.Contains(text, "@main") {
		t.Error("expected func @main")
	}
	if !strings.Contains(text, "karkain.matmul") {
		t.Error("expected karkain.matmul op")
	}
	if !strings.Contains(text, "karkain.relu") {
		t.Error("expected karkain.relu op")
	}
	if !strings.Contains(text, "[3, 4]") {
		t.Error("expected shape attr [3, 4]")
	}
	if len(mod.Results) != 1 || mod.Results[0] != "z" {
		t.Errorf("expected result [z], got %v", mod.Results)
	}
}

func TestLowerToMLIRNoOutput(t *testing.T) {
	x := tensor.NewNode("x", tensor.OpCreate, tensor.NewShape(3), tensor.ElemF32)
	g := tensor.NewGraph()
	g.AddNode(x)
	l := NewLowerToMLIR("main")
	_, err := l.Lower(g)
	if err == nil {
		t.Error("expected error for graph with no outputs")
	}
}

func TestMLIREmitFormat(t *testing.T) {
	m := NewMLIRModule("run")
	m.Add(&MLIROp{
		Name:     "karkain.add",
		Operands: []string{"a", "b"},
		Result:   "c",
		Attrs:    map[string]string{"shape": "[4]"},
	})
	m.AddResult("c")
	text := m.Emit()
	if !strings.Contains(text, "%_c = karkain.add(%a, %b)") {
		t.Errorf("unexpected MLIR text:\n%s", text)
	}
	if !strings.Contains(text, "{shape = [4]}") {
		t.Errorf("expected attrs, got:\n%s", text)
	}
}

func TestFusionThenLower(t *testing.T) {
	// Fuse matmul+add+relu then lower the fused graph.
	x := tensor.NewNode("x", tensor.OpCreate, tensor.NewShape(4, 3), tensor.ElemF32)
	w := tensor.NewNode("w", tensor.OpCreate, tensor.NewShape(3, 5), tensor.ElemF32)
	mm := tensor.NewNode("mm", tensor.OpMatMul, tensor.NewShape(4, 5), tensor.ElemF32, "x", "w")
	bias := tensor.NewNode("bias", tensor.OpCreate, tensor.NewShape(5), tensor.ElemF32)
	add := tensor.NewNode("add", tensor.OpAdd, tensor.NewShape(4, 5), tensor.ElemF32, "mm", "bias")
	relu := tensor.NewNode("relu", tensor.OpRelu, tensor.NewShape(4, 5), tensor.ElemF32, "add")
	g := tensor.NewGraph()
	for _, n := range []*tensor.TensorNode{x, w, mm, bias, add, relu} {
		g.AddNode(n)
	}
	g.AddOutput(relu)

	fp := NewFusionPass()
	res := fp.Run(g)
	if len(res.Ops) != 1 {
		t.Fatalf("expected 1 fused op, got %d", len(res.Ops))
	}

	l := NewLowerToMLIR("fused")
	mod, err := l.Lower(g)
	if err != nil {
		t.Fatalf("lower fused failed: %v", err)
	}
	if !strings.Contains(mod.Emit(), "karkain.matmul") {
		t.Error("fused lower missing matmul")
	}
	if !strings.Contains(mod.Emit(), "karkain.relu") {
		t.Error("fused lower missing relu")
	}
}
