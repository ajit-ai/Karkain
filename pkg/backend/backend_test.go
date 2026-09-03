package backend_test

import (
	"testing"

	"karkain/pkg/backend"
	cpu "karkain/pkg/backend/cpu"
	"karkain/pkg/tensor"
)

func TestPlanSingleBackend(t *testing.T) {
	bk := cpu.New()
	planner := backend.NewPlanner(bk)

	a := tensor.NewNode("a", tensor.OpCreate, tensor.NewShape(3), tensor.ElemF32)
	b := tensor.NewNode("b", tensor.OpCreate, tensor.NewShape(3), tensor.ElemF32)
	sum := tensor.NewNode("sum", tensor.OpAdd, tensor.NewShape(3), tensor.ElemF32, "a", "b")

	g := tensor.NewGraph()
	g.AddNode(a)
	g.AddNode(b)
	g.AddNode(sum)
	g.AddInput(a)
	g.AddInput(b)
	g.AddOutput(sum)

	plan, err := planner.Plan(g)
	if err != nil {
		t.Fatalf("plan failed: %v", err)
	}
	if len(plan.ExecuteOrder) != 1 {
		t.Fatalf("expected 1 subgraph, got %d", len(plan.ExecuteOrder))
	}
	if plan.Assignments["sum"] != "cpu" {
		t.Errorf("expected cpu, got %s", plan.Assignments["sum"])
	}
}

func TestPlanContiguousGrouping(t *testing.T) {
	bk := cpu.New()
	planner := backend.NewPlanner(bk)

	// a + b -> c, then c * d -> e : all cpu, should be 1 subgraph
	a := tensor.NewNode("a", tensor.OpCreate, tensor.NewShape(3), tensor.ElemF32)
	b := tensor.NewNode("b", tensor.OpCreate, tensor.NewShape(3), tensor.ElemF32)
	c := tensor.NewNode("c", tensor.OpAdd, tensor.NewShape(3), tensor.ElemF32, "a", "b")
	d := tensor.NewNode("d", tensor.OpCreate, tensor.NewShape(3), tensor.ElemF32)
	e := tensor.NewNode("e", tensor.OpMul, tensor.NewShape(3), tensor.ElemF32, "c", "d")

	g := tensor.NewGraph()
	for _, n := range []*tensor.TensorNode{a, b, c, d, e} {
		g.AddNode(n)
	}
	g.AddInput(a)
	g.AddInput(b)
	g.AddOutput(e)

	plan, err := planner.Plan(g)
	if err != nil {
		t.Fatalf("plan failed: %v", err)
	}
	if len(plan.ExecuteOrder) != 1 {
		t.Errorf("expected 1 subgraph for contiguous cpu nodes, got %d", len(plan.ExecuteOrder))
	}
}

func TestPlanNoBackends(t *testing.T) {
	planner := backend.NewPlanner()
	a := tensor.NewNode("a", tensor.OpCreate, tensor.NewShape(3), tensor.ElemF32)
	g := tensor.NewGraph()
	g.AddNode(a)
	_, err := planner.Plan(g)
	if err == nil {
		t.Error("expected error with no backends")
	}
}

func TestDispatcherExecuteAdd(t *testing.T) {
	bk := cpu.New()
	dispatcher := backend.NewDispatcher(bk)

	a := tensor.NewNode("a", tensor.OpCreate, tensor.NewShape(4), tensor.ElemF32)
	b := tensor.NewNode("b", tensor.OpCreate, tensor.NewShape(4), tensor.ElemF32)
	sum := tensor.NewNode("sum", tensor.OpAdd, tensor.NewShape(4), tensor.ElemF32, "a", "b")

	g := tensor.NewGraph()
	g.AddNode(a)
	g.AddNode(b)
	g.AddNode(sum)
	g.AddInput(a)
	g.AddInput(b)
	g.AddOutput(sum)

	result, err := dispatcher.Execute(g, map[string][]float64{
		"a": {1, 2, 3, 4},
		"b": {5, 6, 7, 8},
	})
	if err != nil {
		t.Fatalf("dispatch execute failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.Outputs == nil {
		t.Fatal("expected outputs")
	}
}

// mockBackend is a backend that supports only a subset of ops,
// for testing fallback and capability selection.
type mockBackend struct {
	name     string
	onlyRelu bool
}

func (m *mockBackend) Name() string               { return m.name }
func (m *mockBackend) Capabilities() backend.Capabilities {
	return backend.Capabilities{}
}
func (m *mockBackend) Supports(op tensor.Op, dt tensor.ElemType, _ tensor.Shape) bool {
	if m.onlyRelu {
		return op == tensor.OpRelu
	}
	return true
}
func (m *mockBackend) Execute(*tensor.TensorGraph, map[string][]float64) (*backend.Result, error) {
	return backend.NewResult(), nil
}

func TestDispatcherCapabilitySelection(t *testing.T) {
	// mock "npu" only supports relu; CPU supports everything.
	npu := &mockBackend{name: "npu", onlyRelu: true}
	cpubk := cpu.New()
	dispatcher := backend.NewDispatcher(npu, cpubk)

	// OpRelu node: should be assigned to npu
	a := tensor.NewNode("a", tensor.OpCreate, tensor.NewShape(3), tensor.ElemF32)
	relu := tensor.NewNode("relu", tensor.OpRelu, tensor.NewShape(3), tensor.ElemF32, "a")

	g := tensor.NewGraph()
	g.AddNode(a)
	g.AddNode(relu)
	g.AddInput(a)
	g.AddOutput(relu)

	plan, err := dispatcher.Planner().Plan(g)
	if err != nil {
		t.Fatalf("plan failed: %v", err)
	}
	if plan.Assignments["relu"] != "npu" {
		t.Errorf("expected relu assigned to npu, got %s", plan.Assignments["relu"])
	}
}

func TestDispatcherAddFallsBackToCPU(t *testing.T) {
	// npu only supports relu; add must fall back to CPU.
	npu := &mockBackend{name: "npu", onlyRelu: true}
	dispatcher := backend.NewDispatcher(npu, cpu.New())

	a := tensor.NewNode("a", tensor.OpCreate, tensor.NewShape(3), tensor.ElemF32)
	b := tensor.NewNode("b", tensor.OpCreate, tensor.NewShape(3), tensor.ElemF32)
	sum := tensor.NewNode("sum", tensor.OpAdd, tensor.NewShape(3), tensor.ElemF32, "a", "b")

	g := tensor.NewGraph()
	g.AddNode(a)
	g.AddNode(b)
	g.AddNode(sum)
	g.AddInput(a)
	g.AddInput(b)
	g.AddOutput(sum)

	plan, err := dispatcher.Planner().Plan(g)
	if err != nil {
		t.Fatalf("plan failed: %v", err)
	}
	if plan.Assignments["sum"] != "cpu" {
		t.Errorf("expected sum assigned to cpu, got %s", plan.Assignments["sum"])
	}
}

func TestDispatcherBackendFor(t *testing.T) {
	cpubk := cpu.New()
	dispatcher := backend.NewDispatcher(cpubk)
	bk, ok := dispatcher.BackendFor("cpu")
	if !ok {
		t.Fatal("expected cpu backend")
	}
	if bk.Name() != "cpu" {
		t.Errorf("expected cpu, got %s", bk.Name())
	}
	if _, ok := dispatcher.BackendFor("npu"); ok {
		t.Error("expected no npu backend")
	}
}

func TestDispatcherNumBackends(t *testing.T) {
	npu := &mockBackend{name: "npu"}
	dispatcher := backend.NewDispatcher(npu, cpu.New())
	if dispatcher.NumBackends() != 2 {
		t.Errorf("expected 2 backends, got %d", dispatcher.NumBackends())
	}
}

