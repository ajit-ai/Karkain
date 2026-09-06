package ssa

import "fmt"

// Dominance tree computation for SSA IR.
// Uses the iterative algorithm (Cooper, Harvey, Kennedy) for computing
// immediate dominators.

// DomTree — dominance tree for a function.
type DomTree struct {
	// IDom maps each block name to its immediate dominator block name.
	// The entry block's immediate dominator is itself.
	IDom map[string]string

	// DomChildren maps each block name to the list of blocks it immediately dominates.
	DomChildren map[string][]string
}

// predecessors returns the list of blocks that jump/branch to the given block.
func predecessors(fn *Function, blockName string) []string {
	var preds []string
	for _, other := range fn.Blocks {
		for _, in := range other.Instrs {
			if in.Op == OpBr {
				if in.ThenTarget == blockName || in.ElseTarget == blockName {
					preds = append(preds, other.Name)
				}
			} else if in.Op == OpJmp {
				if in.JmpTarget == blockName {
					preds = append(preds, other.Name)
				}
			}
		}
	}
	return preds
}

// BuildDominatorTree computes the dominance tree for a function.
// Uses the iterative algorithm from Cooper et al. "A Simple, Fast Dominance Algorithm."
func BuildDominatorTree(fn *Function) *DomTree {
	if len(fn.Blocks) == 0 {
		return &DomTree{IDom: make(map[string]string), DomChildren: make(map[string][]string)}
	}

	blockIndex := make(map[string]int)
	for i, b := range fn.Blocks {
		blockIndex[b.Name] = i
	}

	// Initialize: entry block dominates itself
	idom := make(map[string]string)
	entryName := fn.Entry.Name
	idom[entryName] = entryName

	// Set all other blocks to undefined
	for _, b := range fn.Blocks {
		if b.Name != entryName {
			idom[b.Name] = ""
		}
	}

	changed := true
	for changed {
		changed = false
		for _, b := range fn.Blocks {
			if b.Name == entryName {
				continue
			}

			preds := predecessors(fn, b.Name)
			if len(preds) == 0 {
				continue
			}

			// Find first predecessor with known dominator
			newIDom := ""
			for _, p := range preds {
				if idom[p] != "" {
					newIDom = p
					break
				}
			}

			if newIDom == "" {
				continue
			}

			// Intersect with other predecessors
			for _, p := range preds {
				if idom[p] == "" || p == newIDom {
					continue
				}
				newIDom = intersect(blockIndex, idom, p, newIDom)
			}

			if idom[b.Name] != newIDom {
				idom[b.Name] = newIDom
				changed = true
			}
		}
	}

	// Build children map
	children := make(map[string][]string)
	for name, dom := range idom {
		if name != dom {
			children[dom] = append(children[dom], name)
		}
	}

	return &DomTree{IDom: idom, DomChildren: children}
}

// intersect finds the common dominator of two blocks using DFS numbering.
func intersect(blockIndex map[string]int, idom map[string]string, b1, b2 string) string {
	finger1 := b1
	finger2 := b2

	for finger1 != finger2 {
		i1, ok1 := blockIndex[finger1]
		i2, ok2 := blockIndex[finger2]
		if !ok1 || !ok2 {
			break
		}
		for i1 > i2 {
			finger1 = idom[finger1]
			i1 = blockIndex[finger1]
		}
		for i2 > i1 {
			finger2 = idom[finger2]
			i2 = blockIndex[finger2]
		}
	}

	return finger1
}

// Dominates returns true if block `a` dominates block `b`.
func (dt *DomTree) Dominates(a, b string) bool {
	if a == b {
		return true
	}
	cur := b
	seen := make(map[string]bool)
	for cur != "" && cur != a {
		if seen[cur] {
			break
		}
		seen[cur] = true
		next, ok := dt.IDom[cur]
		if !ok || next == cur {
			break
		}
		cur = next
	}
	return cur == a
}

// StrictlyDominates returns true if block `a` strictly dominates block `b`.
func (dt *DomTree) StrictlyDominates(a, b string) bool {
	return a != b && dt.Dominates(a, b)
}

// DominanceFrontier returns the dominance frontier of a block.
func (dt *DomTree) DominanceFrontier(fn *Function, blockName string) map[string]bool {
	frontier := make(map[string]bool)

	for _, b := range fn.Blocks {
		preds := predecessors(fn, b.Name)
		for _, pred := range preds {
			if dt.Dominates(blockName, pred) && !dt.StrictlyDominates(blockName, b.Name) {
				frontier[b.Name] = true
			}
		}
	}

	return frontier
}

// DomTreeDepth returns the depth of a block in the dominance tree.
func (dt *DomTree) DomTreeDepth(name string) int {
	depth := 0
	cur := name
	seen := make(map[string]bool)
	for {
		if seen[cur] {
			break
		}
		seen[cur] = true
		next, ok := dt.IDom[cur]
		if !ok || next == cur {
			break
		}
		cur = next
		depth++
		if depth > 1000 {
			break
		}
	}
	return depth
}

// IsDominatedBy returns true if all paths from entry to `b` must go through `a`.
func (dt *DomTree) IsDominatedBy(b, a string) bool {
	return dt.Dominates(a, b)
}

// String returns a human-readable representation of the dominance tree.
func (dt *DomTree) String() string {
	s := "DomTree:\n"
	for name, dom := range dt.IDom {
		s += fmt.Sprintf("  %s -> %s\n", name, dom)
	}
	return s
}
