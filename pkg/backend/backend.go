// Package backend defines the backend abstraction for tensor execution.
// CPU is the reference backend — the correctness oracle before any accelerator.
package backend

import (
	"karkain/pkg/tensor"
)

// Capabilities describes what a backend supports.
type Capabilities struct {
	Name           string
	SupportedDtypes []tensor.ElemType
	MaxRank        int
	SupportedOps   []tensor.Op
	MemoryLimit    int64
	AlignmentReqs  int
	SupportedLayouts []tensor.Layout
}

// SupportsDtype returns true if the backend supports the given element type.
func (c Capabilities) SupportsDtype(dt tensor.ElemType) bool {
	for _, t := range c.SupportedDtypes {
		if t == dt {
			return true
		}
	}
	return false
}

// SupportsOp returns true if the backend supports the given operation.
func (c Capabilities) SupportsOp(op tensor.Op) bool {
	for _, o := range c.SupportedOps {
		if o == op {
			return true
		}
	}
	return false
}

// Supports fills in a tensor type.
func (c Capabilities) Supports(typ tensor.TensorType) bool {
	if !c.SupportsDtype(typ.Elem) {
		return false
	}
	if typ.Rank() > c.MaxRank {
		return false
	}
	return true
}

// Backend is the interface that all execution backends must implement.
type Backend interface {
	// Name returns the backend name (e.g., "cpu", "gpu", "npu").
	Name() string
	// Capabilities returns the backend's capabilities.
	Capabilities() Capabilities
	// Supports returns true if the backend can execute the operation.
	Supports(op tensor.Op, dtype tensor.ElemType, shape tensor.Shape) bool
	// Execute runs a tensor graph and returns the output tensors.
	Execute(graph *tensor.TensorGraph, inputs map[string][]float64) (*Result, error)
}

// Result represents the output of a backend execution.
type Result struct {
	Outputs  map[string]*tensor.TensorNode
	Values   map[string][]float64 // output ID -> computed values
	Shapes   map[string]int       // output ID -> rank
	Timings  map[string]float64   // op name -> execution time in ms
	Metadata map[string]string    // raw output lines
}

// NewResult creates an empty result.
func NewResult() *Result {
	return &Result{
		Outputs:  make(map[string]*tensor.TensorNode),
		Values:   make(map[string][]float64),
		Shapes:   make(map[string]int),
		Timings:  make(map[string]float64),
		Metadata: make(map[string]string),
	}
}
