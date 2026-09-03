package backend

import (
	"fmt"

	"karkain/pkg/tensor"
)

// Dispatcher executes a planned tensor graph across backends with a
// graceful fallback chain: if the preferred backend fails, execution
// falls back to the next available backend (ultimately CPU).
type Dispatcher struct {
	backends map[string]Backend
	planner  *Planner
}

// NewDispatcher creates a dispatcher with the given backends in priority order.
func NewDispatcher(backends ...Backend) *Dispatcher {
	bm := make(map[string]Backend)
	for _, bk := range backends {
		bm[bk.Name()] = bk
	}
	return &Dispatcher{
		backends: bm,
		planner:  NewPlanner(backends...),
	}
}

// BackendFor returns the backend registered under the given name.
func (d *Dispatcher) BackendFor(name string) (Backend, bool) {
	bk, ok := d.backends[name]
	return bk, ok
}

// Planner returns the planner used by this dispatcher.
func (d *Dispatcher) Planner() *Planner {
	return d.planner
}

// NumBackends returns the number of registered backends.
func (d *Dispatcher) NumBackends() int {
	return len(d.backends)
}

// Execute runs a tensor graph, dispatching each subgraph to its assigned
// backend. If a subgraph backend fails at runtime, it falls back to the
// next backend that supports it (finally CPU).
func (d *Dispatcher) Execute(g *tensor.TensorGraph, inputs map[string][]float64) (*Result, error) {
	plan, err := d.planner.Plan(g)
	if err != nil {
		return nil, err
	}

	overall := NewResult()

	// Execute subgraphs in order
	for _, pg := range plan.ExecuteOrder {
		sub := subgraphFromPlan(pg)

		// Build the execution order with fallback
		var lastErr error
		executed := false
		for _, bk := range d.priorityList() {
			if !allSupported(bk, sub) {
				continue
			}
			result, err := bk.Execute(sub, inputs)
			if err == nil {
				mergeResult(overall, result)
				executed = true
				break
			}
			lastErr = err
		}
		if !executed {
			return nil, fmt.Errorf("subgraph for backend %s failed and no fallback succeeded: %w",
				pg.BackendName, lastErr)
		}
	}

	return overall, nil
}

// priorityList returns the backends ordered deterministically (priority order).
func (d *Dispatcher) priorityList() []Backend {
	return d.planner.Backends()
}

func allSupported(bk Backend, g *tensor.TensorGraph) bool {
	for _, node := range g.Nodes {
		if node.Op == tensor.OpCreate {
			continue
		}
		if !bk.Supports(node.Op, node.DType, node.Shape) {
			return false
		}
	}
	return true
}

func subgraphFromPlan(pg *PlannedGraph) *tensor.TensorGraph {
	g := tensor.NewGraph()
	for _, n := range pg.Nodes {
		g.AddNode(n)
	}
	for _, out := range pg.Outputs {
		g.AddOutput(out)
	}
	return g
}

func mergeResult(dst, src *Result) {
	for k, v := range src.Outputs {
		dst.Outputs[k] = v
	}
	for k, v := range src.Values {
		dst.Values[k] = v
	}
	for k, v := range src.Shapes {
		dst.Shapes[k] = v
	}
	for k, v := range src.Timings {
		dst.Timings[k] = v
	}
	for k, v := range src.Metadata {
		dst.Metadata[k] = v
	}
}
