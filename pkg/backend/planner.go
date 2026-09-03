package backend

import (
	"fmt"

	"karkain/pkg/tensor"
)

// Plan describes how a tensor graph is partitioned across backends.
type Plan struct {
	// ExecuteOrder is the execution order of planned subgraphs.
	ExecuteOrder []*PlannedGraph
	// Assignments maps each forward node ID to its backend name.
	Assignments map[string]string
	// Backends is the ordered fallback chain of available backends.
	Backends []Backend
}

// PlannedGraph is a subgraph assigned to a single backend.
type PlannedGraph struct {
	BackendName string
	Nodes       []*tensor.TensorNode // in topological order
	Outputs     []*tensor.TensorNode
}

// Planner partitions a tensor graph across available backends.
//
// Strategy (Phase 75 — deterministic, CPU-first):
//  1. For each node, find the first backend (in priority order) that Supports it.
//  2. CPU is always the final fallback and supports everything.
//  3. Group contiguous nodes assigned to the same backend into subgraphs.
type Planner struct {
	backends []Backend // ordered by priority (highest first)
}

// NewPlanner creates a planner with the given backends in priority order.
func NewPlanner(backends ...Backend) *Planner {
	return &Planner{backends: backends}
}

// Plan partitions the graph across backends.
func (p *Planner) Plan(g *tensor.TensorGraph) (*Plan, error) {
	plan := &Plan{
		ExecuteOrder: make([]*PlannedGraph, 0),
		Assignments:  make(map[string]string),
		Backends:     p.backends,
	}

	if len(p.backends) == 0 {
		return nil, fmt.Errorf("no backends available")
	}

	// Compute each node's backend
	nodeBackend := make(map[string]string)
	for _, node := range g.Nodes {
		assigned := false
		for _, bk := range p.backends {
			dt := node.DType
			if bk.Supports(node.Op, dt, node.Shape) {
				nodeBackend[node.ID] = bk.Name()
				plan.Assignments[node.ID] = bk.Name()
				assigned = true
				break
			}
		}
		if !assigned {
			// No backend supports it — error (shouldn't happen with CPU fallback)
			return nil, fmt.Errorf("no backend supports op %s with dtype %s shape %s",
				node.Op, node.DType, node.Shape)
		}
	}

	// Group contiguous same-backend nodes into subgraphs
	var current *PlannedGraph
	var currentBackend string
	for _, node := range g.Nodes {
		bkName := nodeBackend[node.ID]
		if current == nil || bkName != currentBackend {
			current = &PlannedGraph{BackendName: bkName}
			plan.ExecuteOrder = append(plan.ExecuteOrder, current)
			currentBackend = bkName
		}
		current.Nodes = append(current.Nodes, node)
	}

	// Assign outputs to their subgraphs
	for _, out := range g.Outputs {
		for _, pg := range plan.ExecuteOrder {
			for _, n := range pg.Nodes {
				if n.ID == out.ID {
					pg.Outputs = append(pg.Outputs, out)
					break
				}
			}
		}
	}

	return plan, nil
}

// NumBackends returns how many backends are in the priority order.
func (p *Planner) NumBackends() int {
	return len(p.backends)
}

// Backends returns the backends in priority order (highest first).
func (p *Planner) Backends() []Backend {
	return p.backends
}
