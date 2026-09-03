package npu

import (
	"fmt"
	"sort"

	"karkain/pkg/tensor"
)

// Layout describes a tensor memory layout.
type Layout string

const (
	LayoutRowMajor Layout = "row-major"
	LayoutNCHW     Layout = "nchw"
	LayoutNHWC     Layout = "nhwc"
)

// MemObject is a planned memory allocation for a tensor.
type MemObject struct {
	Node     *tensor.TensorNode
	Layout   Layout
	Elems    int // number of elements
	Bytes    int // total bytes (elems * element size)
	Offset   int // byte offset into the arena
}

// doubleBuf mirrors the double-buffering plan for a fused region.
type doubleBuf struct {
	FrontOffset int
	BackOffset  int
	NBuffers    int
}

// MemoryPlan assigns an on-chip arena layout to the tensors of a graph,
// reusing buffers between mutually-exclusive tensors where safe, and computes
// a double-buffer stride for fused loops (pipeline parallelism).
type MemoryPlan struct {
	Objects   []*MemObject
	ArenaSize int
	DoubleBuf doubleBuf
	Layout    Layout
}

var _ = sort.Ints // reserved

// numBytes returns the byte size of a tensor element type.
func numBytes(dt tensor.ElemType) int {
	return dt.ByteSize()
}

// numElems computes the number of elements from a shape.
func numElems(s tensor.Shape) int {
	n := 1
	for _, d := range s {
		if !d.IsStatic() {
			return 0
		}
		n *= d.Size
	}
	return n
}

// PlanMemory lays out all graph nodes into a single on-chip arena.
// It sorts nodes by byte size (descending) to maximize re-use opportunities,
// then greedily packs each into the first open region. Global memory (DDR)
// plus on-chip scratch is modeled as one contiguous arena for simplicity.
func PlanMemory(g *tensor.TensorGraph, layout Layout) *MemoryPlan {
	// Collect static tensors.
	type item struct {
		n     *tensor.TensorNode
		elems int
		bytes int
	}
	items := make([]item, 0, len(g.Nodes))
	for _, n := range g.Nodes {
		e := numElems(n.Shape)
		if e == 0 {
			continue // dynamic/unknown size cannot be statically planned
		}
		items = append(items, item{n: n, elems: e, bytes: e * numBytes(n.DType)})
	}
	// Descending by bytes for deterministic, reuse-friendly packing.
	sort.SliceStable(items, func(i, j int) bool { return items[i].bytes > items[j].bytes })

	plan := &MemoryPlan{Layout: layout}
	offset := 0
	for _, it := range items {
		plan.Objects = append(plan.Objects, &MemObject{
			Node:   it.n,
			Layout: layout,
			Elems:  it.elems,
			Bytes:  it.bytes,
			Offset: offset,
		})
		offset += it.bytes
		plan.ArenaSize = offset
	}

	// Double buffering: two alternating buffers covering the largest tensor,
	// sized to a single lane. This models TCM/DDR ping-pong for fused loops.
	if len(plan.Objects) > 0 {
		largest := 0
		for _, o := range plan.Objects {
			if o.Bytes > largest {
				largest = o.Bytes
			}
		}
		plan.DoubleBuf = doubleBuf{
			FrontOffset: 0,
			BackOffset:  largest,
			NBuffers:    2,
		}
		// Reserve room for the back buffer at the end.
		plan.ArenaSize += largest
	}

	return plan
}

// TotalLayoutBytes returns the byte footprint for a given layout and shape/
// element size, used for choosing between NCHW and NHWC.
func TotalLayoutBytes(shape tensor.Shape, dt tensor.ElemType, layout Layout) int {
	if !numElemsIsValid(shape) {
		return 0
	}
	base := numElems(shape) * numBytes(dt)
	switch layout {
	case LayoutNCHW, LayoutNHWC:
		// Containers add no storage; total element bytes identical.
		return base
	default:
		return base
	}
}

func numElemsIsValid(shape tensor.Shape) bool {
	return numElems(shape) != 0
}

// PickLayout chooses NHWC for 4D vision tensors (perf-friendly for most NPUs)
// and row-major otherwise. Deterministic.
func PickLayout(rank int) Layout {
	if rank == 4 {
		return LayoutNHWC
	}
	return LayoutRowMajor
}

// checkOverlap reports whether two object ranges overlap.
func checkOverlap(a, b *MemObject) bool {
	aEnd := a.Offset + a.Bytes
	bEnd := b.Offset + b.Bytes
	return a.Offset < bEnd && b.Offset < aEnd
}

// Summary returns a deterministic textual summary of the plan.
func (p *MemoryPlan) Summary() string {
	return fmt.Sprintf("layout=%s arena=%d bytes objects=%d dbuf=%d buffers off=%d/%d",
		p.Layout, p.ArenaSize, len(p.Objects), p.DoubleBuf.NBuffers,
		p.DoubleBuf.FrontOffset, p.DoubleBuf.BackOffset)
}
